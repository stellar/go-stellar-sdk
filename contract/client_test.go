package contract

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testContractID returns a deterministic strkey-encoded contract id ("C...")
// for client-construction tests.
func testContractID(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	cid, err := strkey.Encode(strkey.VersionByteContract, raw)
	require.NoError(t, err)
	return cid
}

// ----------------------------------------------------------------------
// New
// ----------------------------------------------------------------------

func TestNew_DefaultsBaseFee(t *testing.T) {
	cid := testContractID(t)
	c, err := New(cid, &fakeSimulator{}, network.TestNetworkPassphrase)
	require.NoError(t, err)
	assert.Equal(t, cid, c.ContractID())
	assert.Equal(t, network.TestNetworkPassphrase, c.network)
	assert.Equal(t, int64(txnbuild.MinBaseFee), c.baseFee, "baseFee defaults to MinBaseFee")
}

func TestNew_WithBaseFee(t *testing.T) {
	cid := testContractID(t)
	c, err := New(cid, &fakeSimulator{}, network.TestNetworkPassphrase, WithBaseFee(500))
	require.NoError(t, err)
	assert.Equal(t, int64(500), c.baseFee)
}

func TestNew_RejectsInvalidArgs(t *testing.T) {
	cid := testContractID(t)
	net := network.TestNetworkPassphrase
	testMuxedAccount := "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK"

	cases := []struct {
		name    string
		build   func() (*Client, error)
		wantSub string
	}{
		{"nil RPC", func() (*Client, error) {
			return New(cid, nil, net)
		}, "RPC is required"},
		{"empty network", func() (*Client, error) {
			return New(cid, &fakeSimulator{}, "")
		}, "network passphrase is required"},
		{"bad contract id", func() (*Client, error) {
			return New("not-a-contract", &fakeSimulator{}, net)
		}, "contract id"},
		{"account (G) strkey rejected", func() (*Client, error) {
			return New(keypair.MustRandom().Address(), &fakeSimulator{}, net)
		}, "not a contract"},
		{"muxed (M) strkey rejected", func() (*Client, error) {
			return New(testMuxedAccount, &fakeSimulator{}, net)
		}, "not a contract"},
		{"invalid source strkey", func() (*Client, error) {
			return New(cid, &fakeSimulator{}, net, WithDefaultSource("not-a-strkey"))
		}, "not a valid ed25519"},
		{"base fee below minimum", func() (*Client, error) {
			return New(cid, &fakeSimulator{}, net, WithBaseFee(1))
		}, "below MinBaseFee"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := tc.build()
			require.Error(t, err)
			assert.Nil(t, c)
			var ce *Error
			require.True(t, errors.As(err, &ce))
			assert.Equal(t, KindInvalidArgs, ce.Kind)
			assert.Contains(t, ce.Error(), tc.wantSub)
		})
	}
}

// ----------------------------------------------------------------------
// Invoke
// ----------------------------------------------------------------------

// cannedInvokeRPC returns a fakeSimulator whose SimulateTransaction response
// is a minimally valid success (read call: SourceAccount-only auth).
func cannedInvokeRPC(t *testing.T) *fakeSimulator {
	t.Helper()
	_, dataB64 := cannedSorobanData(t)
	_, retB64 := cannedReturnValue(t)
	return &fakeSimulator{
		resp: protocol.SimulateTransactionResponse{
			TransactionDataXDR: dataB64,
			MinResourceFee:     500_000,
			Results: []protocol.SimulateHostFunctionResult{
				{ReturnValueXDR: &retB64},
			},
		},
	}
}

func TestInvoke_AcceptsRawScVals(t *testing.T) {
	cid := testContractID(t)
	rpc := cannedInvokeRPC(t)

	c, err := New(cid, rpc, network.TestNetworkPassphrase)
	require.NoError(t, err)

	rawArgs := []xdr.ScVal{{Type: xdr.ScValTypeScvU32, U32: u32ptr(11)}}
	at, err := c.Invoke(context.Background(), "anything", rawArgs)
	require.NoError(t, err, "raw []xdr.ScVal must work")
	require.Len(t, at.Args, 1)
	assert.Equal(t, uint32(11), uint32(*at.Args[0].U32))
}

func TestInvoke_BuildsHostFunctionAndSimulates(t *testing.T) {
	cid := testContractID(t)
	rpc := cannedInvokeRPC(t)

	c, err := New(cid, rpc, network.TestNetworkPassphrase)
	require.NoError(t, err)

	args := []xdr.ScVal{{Type: xdr.ScValTypeScvU32, U32: u32ptr(7)}}
	at, err := c.Invoke(context.Background(), "bump", args)
	require.NoError(t, err)
	require.NotNil(t, at)

	assert.Equal(t, "bump", at.Method)
	require.Len(t, at.Args, 1)
	assert.Equal(t, xdr.ScValTypeScvU32, at.Args[0].Type)

	// The transaction must carry an InvokeHostFunction op targeting our
	// contract id.
	require.NotNil(t, at.Built)
	ops := at.Built.Operations()
	require.Len(t, ops, 1)
	op, ok := ops[0].(*txnbuild.InvokeHostFunction)
	require.True(t, ok)
	ic := op.HostFunction.InvokeContract
	require.NotNil(t, ic)
	require.NotNil(t, ic.ContractAddress.ContractId)

	expectedAddr, err := xdr.ScAddressFromStrkey(cid)
	require.NoError(t, err)
	assert.Equal(t, expectedAddr, ic.ContractAddress)
	assert.Equal(t, xdr.ScSymbol("bump"), ic.FunctionName)

	// Simulate ran exactly once.
	assert.Equal(t, 1, rpc.calls)
}

// KindInvalidArgs is the slice's most-produced error class; callers must be
// able to classify it with errors.Is against the package sentinel, not only by
// unwrapping with errors.As and reading .Kind. (Regression test for finding M2.)
func TestInvalidArgs_MatchesSentinel(t *testing.T) {
	c, err := New(testContractID(t), &fakeSimulator{}, network.TestNetworkPassphrase)
	require.NoError(t, err)

	_, err = c.Invoke(context.Background(), "", nil) // empty method → KindInvalidArgs
	require.Error(t, err)

	assert.True(t, errors.Is(err, ErrInvalidArgs),
		"an invalid-args error must match the ErrInvalidArgs sentinel")
	assert.False(t, errors.Is(err, ErrSimulationFailed),
		"an invalid-args error must not match an unrelated sentinel")
}

// TestInvoke_NoSource_UsesSyntheticAccount proves Invoke needs no configured
// source: it simulates against the synthetic null account without a
// LoadAccount round-trip, which is all a read-only call needs.
func TestInvoke_NoSource_UsesSyntheticAccount(t *testing.T) {
	cid := testContractID(t)
	rpc := cannedInvokeRPC(t)
	c, err := New(cid, rpc, network.TestNetworkPassphrase)
	require.NoError(t, err)

	at, err := c.Invoke(context.Background(), "bump", []xdr.ScVal{})
	require.NoError(t, err)
	assert.Equal(t, 0, rpc.loadAcctCalls, "no source must not trigger LoadAccount")
	assert.Equal(t, 1, rpc.calls, "simulation runs once")

	require.NotNil(t, at.Built)
	ops := at.Built.Operations()
	require.Len(t, ops, 1)
	op, ok := ops[0].(*txnbuild.InvokeHostFunction)
	require.True(t, ok)
	assert.Equal(t, nullAccount, op.SourceAccount, "synthetic source is the null account")
}

func TestInvoke_EmptyMethodRejected(t *testing.T) {
	cid := testContractID(t)
	c, err := New(cid, &fakeSimulator{}, network.TestNetworkPassphrase)
	require.NoError(t, err)
	_, err = c.Invoke(context.Background(), "", nil)
	require.Error(t, err)
	var ce *Error
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, KindInvalidArgs, ce.Kind)
}

func TestInvoke_InvalidMethodRejectedBeforeLoadAccount(t *testing.T) {
	cid := testContractID(t)
	kp := keypair.MustRandom()

	cases := []struct {
		name   string
		method string
	}{
		{"invalid character", "bad-name"},
		{"too long", strings.Repeat("a", 33)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rpc := cannedInvokeRPC(t)
			c, err := New(cid, rpc, network.TestNetworkPassphrase, WithDefaultSource(kp.Address()))
			require.NoError(t, err)

			_, err = c.Invoke(context.Background(), tc.method, []xdr.ScVal{})
			require.Error(t, err)
			var ce *Error
			require.True(t, errors.As(err, &ce))
			assert.Equal(t, KindInvalidArgs, ce.Kind)
			assert.Equal(t, 0, rpc.loadAcctCalls)
			assert.Equal(t, 0, rpc.calls)
		})
	}
}

// WithDefaultSource resolves the live account via LoadAccount and bumps the
// sequence exactly once before building.
func TestInvoke_WithDefaultSourceLoadsAccountAndBumpsSeq(t *testing.T) {
	cid := testContractID(t)
	rpc := cannedInvokeRPC(t)

	kp := keypair.MustRandom()
	c, err := New(cid, rpc, network.TestNetworkPassphrase, WithDefaultSource(kp.Address()))
	require.NoError(t, err)

	_, err = c.Invoke(context.Background(), "bump", []xdr.ScVal{})
	require.NoError(t, err)
	assert.Equal(t, 1, rpc.loadAcctCalls, "WithDefaultSource must LoadAccount exactly once")
	assert.Equal(t, kp.Address(), rpc.gotLoadAddr)
}

// seqErrAccount is a txnbuild.Account whose sequence cannot be incremented,
// used to exercise resolveSource's IncrementSequenceNumber error branch (a
// real SimpleAccount never fails this).
type seqErrAccount struct{ addr string }

func (a seqErrAccount) GetAccountID() string              { return a.addr }
func (a seqErrAccount) GetSequenceNumber() (int64, error) { return 0, nil }
func (a seqErrAccount) IncrementSequenceNumber() (int64, error) {
	return 0, errors.New("sequence overflow")
}

// Source-resolution failures happen before any simulation, so they are
// classified KindSourceAccountFailed — not KindSimulationFailed. A caller
// branching on errors.Is(err, ErrSimulationFailed) must not catch them.
// (Regression test for review finding M1.)
func TestInvoke_SourceResolutionFailsBeforeSimulate(t *testing.T) {
	cid := testContractID(t)
	kp := keypair.MustRandom()

	cases := []struct {
		name    string
		rpc     *fakeSimulator
		wantSub string
	}{
		{
			name:    "LoadAccount error",
			rpc:     &fakeSimulator{loadAcctErr: errors.New("rpc unreachable")},
			wantSub: "load source account",
		},
		{
			name:    "sequence increment error",
			rpc:     &fakeSimulator{loadAcctResp: seqErrAccount{addr: kp.Address()}},
			wantSub: "increment source sequence",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := New(cid, tc.rpc, network.TestNetworkPassphrase, WithDefaultSource(kp.Address()))
			require.NoError(t, err)

			_, err = c.Invoke(context.Background(), "bump", []xdr.ScVal{})
			require.Error(t, err)

			// errors.Is matches on Kind: this is a source-account failure...
			assert.True(t, errors.Is(err, ErrSourceAccountFailed),
				"source-resolution failure must match ErrSourceAccountFailed")
			// ...and explicitly NOT a simulation failure (the M1 misclassification).
			assert.False(t, errors.Is(err, ErrSimulationFailed),
				"source-resolution failure must not be classified as a simulation failure")

			var ce *Error
			require.True(t, errors.As(err, &ce))
			assert.Equal(t, KindSourceAccountFailed, ce.Kind)
			assert.Contains(t, ce.Error(), tc.wantSub)

			// The failure precedes simulation, so simulate must never run.
			assert.Equal(t, 0, tc.rpc.calls, "simulate must not run when source resolution fails")
			assert.Equal(t, 1, tc.rpc.loadAcctCalls, "LoadAccount is attempted exactly once")
		})
	}
}

// ----------------------------------------------------------------------
// End-to-end acceptance: read short-circuit
// ----------------------------------------------------------------------

// TestInvoke_ReadCall_ShortCircuits proves a read call returns the simulated
// value and never submits.
func TestInvoke_ReadCall_ShortCircuits(t *testing.T) {
	cid := testContractID(t)
	rpc := cannedInvokeRPC(t) // SourceAccount-only auth + empty ReadWrite → read call

	c, err := New(cid, rpc, network.TestNetworkPassphrase)
	require.NoError(t, err)
	at, err := c.Invoke(context.Background(), "balance", []xdr.ScVal{})
	require.NoError(t, err)
	require.True(t, at.IsReadCall(), "SourceAccount-only auth + empty RW footprint is a read call")

	// The RPC interface has no send path; Result reads the simulated value directly.
	res, err := at.Result()
	require.NoError(t, err)
	scv, ok := res.(xdr.ScVal)
	require.True(t, ok)
	assert.Equal(t, xdr.Uint32(42), *scv.U32)
}

// ----------------------------------------------------------------------
// Per-call InvokeOption side effects on the built transaction
// ----------------------------------------------------------------------

// WithResourceFeeMultiplier scales the simulated resource fee written onto the op.
// cannedInvokeRPC reports MinResourceFee = 500_000.
func TestInvoke_WithResourceFeeMultiplier_ScalesFee(t *testing.T) {
	cid := testContractID(t)
	rpc := cannedInvokeRPC(t)
	c, err := New(cid, rpc, network.TestNetworkPassphrase)
	require.NoError(t, err)

	at, err := c.Invoke(context.Background(), "bump", []xdr.ScVal{}, WithResourceFeeMultiplier(2.0))
	require.NoError(t, err)
	require.NotNil(t, at.op.Ext.SorobanData)
	assert.Equal(t, xdr.Int64(1_000_000), at.op.Ext.SorobanData.ResourceFee, "multiplier scales the simulated fee")
}

// A non-positive multiplier is ignored: the simulated fee is written verbatim.
func TestInvoke_WithResourceFeeMultiplier_NonPositiveIgnored(t *testing.T) {
	cid := testContractID(t)
	rpc := cannedInvokeRPC(t)
	c, err := New(cid, rpc, network.TestNetworkPassphrase)
	require.NoError(t, err)

	at, err := c.Invoke(context.Background(), "bump", []xdr.ScVal{}, WithResourceFeeMultiplier(-1))
	require.NoError(t, err)
	require.NotNil(t, at.op.Ext.SorobanData)
	assert.Equal(t, xdr.Int64(500_000), at.op.Ext.SorobanData.ResourceFee, "non-positive multiplier leaves the fee verbatim")
}

// WithMemo attaches the given memo to the built transaction.
func TestInvoke_WithMemo_AttachedToBuiltTx(t *testing.T) {
	cid := testContractID(t)
	rpc := cannedInvokeRPC(t)
	c, err := New(cid, rpc, network.TestNetworkPassphrase)
	require.NoError(t, err)

	at, err := c.Invoke(context.Background(), "bump", []xdr.ScVal{}, WithMemo(txnbuild.MemoText("hi")))
	require.NoError(t, err)
	require.NotNil(t, at.Built)
	assert.Equal(t, txnbuild.MemoText("hi"), at.Built.Memo())
}

// WithTimeBounds applies to the built tx; a zero min/max leaves that bound open
// (MaxTime == TimeoutInfinite), exercising every IsZero branch of the option.
func TestInvoke_WithTimeBounds_AppliedToBuiltTx(t *testing.T) {
	cid := testContractID(t)
	cases := []struct {
		name             string
		min, max         time.Time
		wantMin, wantMax int64
	}{
		{"both set", time.Unix(1000, 0), time.Unix(2000, 0), 1000, 2000},
		{"open lower", time.Time{}, time.Unix(2000, 0), 0, 2000},
		{"open upper", time.Unix(1000, 0), time.Time{}, 1000, txnbuild.TimeoutInfinite},
		{"both open", time.Time{}, time.Time{}, 0, txnbuild.TimeoutInfinite},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rpc := cannedInvokeRPC(t)
			c, err := New(cid, rpc, network.TestNetworkPassphrase)
			require.NoError(t, err)

			at, err := c.Invoke(context.Background(), "bump", []xdr.ScVal{}, WithTimeBounds(tc.min, tc.max))
			require.NoError(t, err)
			tb := at.Built.Timebounds()
			assert.Equal(t, tc.wantMin, tb.MinTime)
			assert.Equal(t, tc.wantMax, tb.MaxTime)
		})
	}
}

// A per-call WithSource overrides the client default; LoadAccount is driven with
// the override address.
func TestInvoke_WithSource_OverridesDefault(t *testing.T) {
	cid := testContractID(t)
	rpc := cannedInvokeRPC(t)
	defaultKp := keypair.MustRandom()
	overrideKp := keypair.MustRandom()
	c, err := New(cid, rpc, network.TestNetworkPassphrase, WithDefaultSource(defaultKp.Address()))
	require.NoError(t, err)

	_, err = c.Invoke(context.Background(), "bump", []xdr.ScVal{}, WithSource(overrideKp.Address()))
	require.NoError(t, err)
	assert.Equal(t, overrideKp.Address(), rpc.gotLoadAddr, "per-call WithSource overrides the client default")
}

// An invalid per-call WithSource is rejected before any network round-trip:
// resolveSource validates the strkey before it calls LoadAccount.
func TestInvoke_WithSource_InvalidStrkeyRejectedBeforeLoadAccount(t *testing.T) {
	cid := testContractID(t)
	rpc := cannedInvokeRPC(t)
	c, err := New(cid, rpc, network.TestNetworkPassphrase)
	require.NoError(t, err)

	_, err = c.Invoke(context.Background(), "bump", []xdr.ScVal{}, WithSource("not-a-strkey"))
	require.Error(t, err)
	var ce *Error
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, KindInvalidArgs, ce.Kind)
	assert.Contains(t, ce.Error(), "not a valid ed25519")
	assert.Equal(t, 0, rpc.loadAcctCalls, "invalid WithSource must fail before LoadAccount")
	assert.Equal(t, 0, rpc.calls, "invalid WithSource must fail before simulate")
}

// A nil/uninitialized client is reported, not panicked on.
func TestInvoke_NilClientRejected(t *testing.T) {
	var c *Client
	_, err := c.Invoke(context.Background(), "bump", nil)
	require.Error(t, err)
	var ce *Error
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, KindInvalidArgs, ce.Kind)
}

// Invoke propagates a simulation failure from the underlying transaction.
func TestInvoke_PropagatesSimulationError(t *testing.T) {
	cid := testContractID(t)
	rpc := &fakeSimulator{err: errors.New("rpc down")}
	c, err := New(cid, rpc, network.TestNetworkPassphrase)
	require.NoError(t, err)

	_, err = c.Invoke(context.Background(), "bump", []xdr.ScVal{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSimulationFailed))
}
