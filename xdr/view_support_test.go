package xdr

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// ------ safeUint32ToInt ------

func TestSafeUint32ToInt(t *testing.T) {
	// Normal values round-trip.
	n, err := safeUint32ToInt(0, 0)
	require.NoError(t, err)
	require.Equal(t, 0, n)
	n, err = safeUint32ToInt(1_000_000, 17)
	require.NoError(t, err)
	require.Equal(t, 1_000_000, n)

	// maxInt32 is the boundary value — always safely representable as int on both
	// 32- and 64-bit platforms.
	n, err = safeUint32ToInt(uint32(maxInt32), 0)
	require.NoError(t, err)
	require.Equal(t, maxInt32, n)
	// uint32 > maxInt32 overflows int on 32-bit platforms. The check must reject
	// regardless of the runtime width — an error here catches silent truncation.
	_, err = safeUint32ToInt(uint32(maxInt32)+1, 42)
	var vErr *ViewError
	require.True(t, errors.As(err, &vErr))
	require.Equal(t, ViewErrShortBuffer, vErr.Kind)
	require.Equal(t, uint32(42), vErr.Offset)
}

// ------ BoolView ------

func TestBoolView_Value(t *testing.T) {
	mk := func(v uint32) BoolView {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], v)
		return BoolView(b[:])
	}

	got, err := mk(0).Value()
	require.NoError(t, err)
	require.False(t, got)
	got, err = mk(1).Value()
	require.NoError(t, err)
	require.True(t, got)

	for _, bad := range []uint32{2, 3, 0xFFFFFFFF} {
		_, err = mk(bad).Value()
		var vErr *ViewError
		require.True(t, errors.As(err, &vErr), "value %d", bad)
		require.Equal(t, ViewErrBadBoolValue, vErr.Kind)
	}

	// Truncated buffer.
	_, err = BoolView([]byte{0, 0}).Value()
	var vErr *ViewError
	require.True(t, errors.As(err, &vErr))
	require.Equal(t, ViewErrShortBuffer, vErr.Kind)
}

// ------ Scalar views, truncated ------

func TestScalarViews_Truncated(t *testing.T) {
	shortBuf := []byte{0, 0, 0}

	_, err := Int32View(shortBuf).Value()
	assertShortBuffer(t, err)
	_, err = Uint32View(shortBuf).Value()
	assertShortBuffer(t, err)
	_, err = Float32View(shortBuf).Value()
	assertShortBuffer(t, err)

	shortBuf8 := make([]byte, 7)
	_, err = Int64View(shortBuf8).Value()
	assertShortBuffer(t, err)
	_, err = Uint64View(shortBuf8).Value()
	assertShortBuffer(t, err)
	_, err = Float64View(shortBuf8).Value()
	assertShortBuffer(t, err)
}

func TestScalarViews_RoundTrip(t *testing.T) {
	// 4-byte types
	b4 := make([]byte, 4)
	neg32 := int32(-42)
	binary.BigEndian.PutUint32(b4, uint32(neg32))
	i32, err := Int32View(b4).Value()
	require.NoError(t, err)
	require.Equal(t, int32(-42), i32)

	binary.BigEndian.PutUint32(b4, 0xdeadbeef)
	u32, err := Uint32View(b4).Value()
	require.NoError(t, err)
	require.Equal(t, uint32(0xdeadbeef), u32)

	// 8-byte types
	b8 := make([]byte, 8)
	neg64 := int64(-1)
	binary.BigEndian.PutUint64(b8, uint64(neg64))
	i64, err := Int64View(b8).Value()
	require.NoError(t, err)
	require.Equal(t, int64(-1), i64)

	binary.BigEndian.PutUint64(b8, 0x0123456789abcdef)
	u64, err := Uint64View(b8).Value()
	require.NoError(t, err)
	require.Equal(t, uint64(0x0123456789abcdef), u64)
}

// ------ VarOpaqueView ------

func TestVarOpaqueView(t *testing.T) {
	mk := func(length int, payload []byte, padded int) VarOpaqueView {
		b := make([]byte, 4+padded)
		binary.BigEndian.PutUint32(b[:4], uint32(length))
		copy(b[4:], payload)
		return VarOpaqueView(b)
	}

	t.Run("empty", func(t *testing.T) {
		payload, err := mk(0, nil, 0).Value()
		require.NoError(t, err)
		require.Empty(t, payload)
	})

	t.Run("round-trip with padding", func(t *testing.T) {
		// length 5 → 3 bytes of zero padding
		payload, err := mk(5, []byte("hello"), 8).Value()
		require.NoError(t, err)
		require.Equal(t, []byte("hello"), payload)
	})

	t.Run("truncated length header", func(t *testing.T) {
		_, err := VarOpaqueView([]byte{0, 0, 0}).Value()
		assertShortBuffer(t, err)
	})

	t.Run("truncated payload", func(t *testing.T) {
		b := []byte{0, 0, 0, 5, 'h', 'i'} // claims 5 bytes, only 2 present
		_, err := VarOpaqueView(b).Value()
		assertShortBuffer(t, err)
	})

	t.Run("non-zero padding", func(t *testing.T) {
		// length 1, payload "a", padding must be 0x00 0x00 0x00 — set one to 0xff
		b := []byte{0, 0, 0, 1, 'a', 0, 0xff, 0}
		_, err := VarOpaqueView(b).Value()
		var vErr *ViewError
		require.True(t, errors.As(err, &vErr))
		require.Equal(t, ViewErrNonZeroPadding, vErr.Kind)
	})
}

// ------ arrayViewCount ------

func TestArrayViewCount(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		b := []byte{0, 0, 0, 3}
		n, err := arrayViewCount(b, 0)
		require.NoError(t, err)
		require.Equal(t, 3, n)
	})

	t.Run("truncated length header", func(t *testing.T) {
		_, err := arrayViewCount([]byte{0, 0}, 0)
		assertShortBuffer(t, err)
	})

	t.Run("exceeds schema max", func(t *testing.T) {
		b := []byte{0, 0, 0, 10}
		_, err := arrayViewCount(b, 5)
		var vErr *ViewError
		require.True(t, errors.As(err, &vErr))
		require.Equal(t, ViewErrArrayCountExceedsMax, vErr.Kind)
	})

	t.Run("maxCount=0 disables bound check", func(t *testing.T) {
		b := []byte{0x7f, 0xff, 0xff, 0xff}
		_, err := arrayViewCount(b, 0)
		require.NoError(t, err)
	})
}

// ------ Try / TryVoid / Must ------

func TestTry_Success(t *testing.T) {
	result, err := Try(func() int { return 42 })
	require.NoError(t, err)
	require.Equal(t, 42, result)
}

func TestTry_CatchesViewError(t *testing.T) {
	sentinel := viewErrShortBuffer(7, "test sentinel")
	_, err := Try(func() int {
		panic(sentinel)
	})
	require.Equal(t, sentinel, err)
}

func TestTry_RePanicsNonViewError(t *testing.T) {
	// A non-ViewError panic (e.g., programmer error) must propagate.
	require.PanicsWithValue(t, "unrelated panic", func() {
		_, _ = Try(func() int {
			panic("unrelated panic")
		})
	})
}

func TestTryVoid_Success(t *testing.T) {
	called := false
	err := TryVoid(func() { called = true })
	require.NoError(t, err)
	require.True(t, called)
}

func TestTryVoid_CatchesViewError(t *testing.T) {
	sentinel := viewErrBadBoolValue(0, 2)
	err := TryVoid(func() { panic(sentinel) })
	require.Equal(t, sentinel, err)
}

func TestMust_PanicsOnError(t *testing.T) {
	sentinel := viewErrShortBuffer(0, "boom")
	require.PanicsWithValue(t, sentinel, func() {
		_ = must(0, sentinel)
	})
}

func TestMust_ReturnsValueOnSuccess(t *testing.T) {
	require.Equal(t, 99, must(99, nil))
}

// ------ ViewError formatting ------

func TestViewError_Error(t *testing.T) {
	e := &ViewError{Kind: ViewErrShortBuffer, Offset: 42, Detail: "need 8 bytes"}
	require.Equal(t, "xdr view: short buffer at offset 42: need 8 bytes", e.Error())
}

func TestViewErrorKind_String_CoversAllKinds(t *testing.T) {
	// Every kind must have a non-empty, non-"unknown" label. Protects against
	// adding a new kind and forgetting to extend the String() switch.
	for k := ViewErrShortBuffer; k <= ViewErrNonZeroPadding; k++ {
		s := k.String()
		require.NotEmpty(t, s)
		require.NotEqual(t, "unknown error", s, "kind %d", k)
	}
}

// ------ helpers ------

func assertShortBuffer(t *testing.T, err error) {
	t.Helper()
	var vErr *ViewError
	require.True(t, errors.As(err, &vErr), "expected *ViewError, got %T: %v", err, err)
	require.Equal(t, ViewErrShortBuffer, vErr.Kind)
}
