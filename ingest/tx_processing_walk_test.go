package ingest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// offsetIn returns the offset of v within base; v must reslice base.
func offsetIn(t *testing.T, base, v []byte) int {
	t.Helper()
	off := cap(base) - cap(v)
	require.True(t, off >= 0 && off+len(v) <= len(base), "view is not a slice of the buffer")
	if len(v) > 0 {
		require.True(t, &base[off] == &v[0], "view is not a slice of the buffer")
	}
	return off
}

// txProcessingElements returns the ledger's TxProcessing elements with the
// generated accessors.
func txProcessingElements(t *testing.T, view xdr.LedgerCloseMetaView) [][]byte {
	t.Helper()
	var out [][]byte
	switch view.MustV() {
	case 0:
		for _, e := range view.MustV0().MustTxProcessing().MustAll() {
			out = append(out, e)
		}
	case 1:
		for _, e := range view.MustV1().MustTxProcessing().MustAll() {
			out = append(out, e)
		}
	case 2:
		for _, e := range view.MustV2().MustTxProcessing().MustAll() {
			out = append(out, e)
		}
	default:
		t.Fatalf("unsupported LCM version")
	}
	return out
}

func TestExtractLedgerTxParts_ElementSpans(t *testing.T) {
	for _, version := range []int32{0, 1, 2} {
		t.Run(fmt.Sprintf("lcmV%d", version), func(t *testing.T) {
			v4 := sorobanTx(t, "x")
			v4.meta = equivalenceV4Meta([]int{2, 0}, 1, 1, true)
			txs := []txWithHash{
				sorobanTx(t, "a"),
				feeBumpTx(t, vMetaV3Soroban([]xdr.ContractEvent{vContractEvent("fb")})),
				v4,
				txV0(t, nil, 5),
			}
			raw, err := buildLCM(t, version, 9120, 1_700_000_000, txs, true).MarshalBinary()
			require.NoError(t, err)
			view := xdr.LedgerCloseMetaView(raw)
			parts, err := ExtractLedgerTxParts(view)
			require.NoError(t, err)

			elems := txProcessingElements(t, view)
			require.Len(t, parts, len(elems))
			hashAt := 0
			if version == 2 {
				hashAt = xdrWord
			}
			for i, part := range parts {
				assert.Equal(t, offsetIn(t, raw, elems[i]), part.ElemStart, "tx %d", i)
				assert.Equal(t, part.ElemStart+len(elems[i]), part.ElemEnd, "tx %d", i)
				if i > 0 {
					assert.Equal(t, parts[i-1].ElemEnd, part.ElemStart, "elements %d and %d must tile", i-1, i)
				}
				elem := raw[part.ElemStart:part.ElemEnd]
				assert.Equal(t, part.Hash[:], elem[hashAt:hashAt+32], "tx %d", i)
				for name, v := range map[string][]byte{"Result": part.Result, "Meta": part.Meta} {
					off := offsetIn(t, raw, v)
					assert.GreaterOrEqual(t, off, part.ElemStart, "tx %d %s", i, name)
					assert.LessOrEqual(t, off+len(v), part.ElemEnd, "tx %d %s", i, name)
				}
			}
		})
	}
}
