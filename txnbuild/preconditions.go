package txnbuild

import (
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Preconditions is a container for all transaction preconditions.
type Preconditions struct {
	// Transaction is only valid during a certain time range (units are seconds).
	TimeBounds TimeBounds
	// Transaction is valid for ledger numbers n such that minLedger <= n <
	// maxLedger (if maxLedger == 0, then only minLedger is checked)
	LedgerBounds *LedgerBounds
	// If nil, the transaction is only valid when sourceAccount's sequence
	// number "N" is seqNum - 1. Otherwise, valid when N satisfies minSeqNum <=
	// N < tx.seqNum.
	MinSequenceNumber *int64
	// Transaction is valid if the current ledger time is at least
	// minSequenceNumberAge greater than the source account's seqTime (units are
	// seconds).
	MinSequenceNumberAge uint64
	// Transaction is valid if the current ledger number is at least
	// minSequenceNumberLedgerGap greater than the source account's seqLedger.
	MinSequenceNumberLedgerGap uint32
	// Transaction is valid if there is a signature corresponding to every
	// Signer in this array, even if the signature is not otherwise required by
	// the source account or operations.
	ExtraSigners []string
}

// Validate ensures that all enabled preconditions are valid.
func (cond *Preconditions) Validate() error {
	var err error

	if err = cond.TimeBounds.Validate(); err != nil {
		return err
	}

	if err = cond.LedgerBounds.Validate(); err != nil {
		return err
	}

	if len(cond.ExtraSigners) > 2 {
		return errors.New("only 2 extra signers allowed")
	}

	return nil
}

// BuildXDR will create a precondition structure that varies depending on
// whether or not there are additional preconditions besides timebounds (which
// are required).
func (cond *Preconditions) BuildXDR() (xdr.Preconditions, error) {
	xdrCond := xdr.Preconditions{}
	xdrTimeBounds := xdr.TimeBounds{
		MinTime: xdr.TimePoint(cond.TimeBounds.MinTime),
		MaxTime: xdr.TimePoint(cond.TimeBounds.MaxTime),
	}

	// Only build PRECOND_V2 structure if we need to
	if cond.hasV2Conditions() {
		xdrPrecond := xdr.PreconditionsV2{
			TimeBounds:      &xdrTimeBounds,
			MinSeqAge:       xdr.Duration(cond.MinSequenceNumberAge),
			MinSeqLedgerGap: xdr.Uint32(cond.MinSequenceNumberLedgerGap),
		}

		if len(cond.ExtraSigners) > 0 {
			xdrPrecond.ExtraSigners = make([]xdr.SignerKey, len(cond.ExtraSigners))
			for i, signer := range cond.ExtraSigners {
				signerKey := xdr.SignerKey{}
				if err := signerKey.SetAddress(signer); err != nil {
					return xdr.Preconditions{}, errors.Wrap(err, "invalid signer")
				}
				xdrPrecond.ExtraSigners[i] = signerKey
			}
		}

		// micro-optimization: if the ledgerbounds will always succeed, omit them
		if cond.LedgerBounds != nil && !(cond.LedgerBounds.MinLedger == 0 &&
			cond.LedgerBounds.MaxLedger == 0) {
			xdrPrecond.LedgerBounds = &xdr.LedgerBounds{
				MinLedger: xdr.Uint32(cond.LedgerBounds.MinLedger),
				MaxLedger: xdr.Uint32(cond.LedgerBounds.MaxLedger),
			}
		}

		if cond.MinSequenceNumber != nil {
			seqNum := xdr.SequenceNumber(*cond.MinSequenceNumber)
			xdrPrecond.MinSeqNum = &seqNum
		}

		xdrCond.Type = xdr.PreconditionTypePrecondV2
		xdrCond.V2 = &xdrPrecond
	} else {
		xdrCond.Type = xdr.PreconditionTypePrecondTime
		xdrCond.TimeBounds = &xdrTimeBounds
	}

	return xdrCond, nil
}

// FromXDR fills in the precondition structure from an xdr.Precondition.
func (cond *Preconditions) FromXDR(precondXdr xdr.Preconditions) error {
	*cond = Preconditions{} // reset existing values

	switch precondXdr.Type {
	case xdr.PreconditionTypePrecondNone:
	case xdr.PreconditionTypePrecondTime:
		cond.TimeBounds = NewTimebounds(
			int64(precondXdr.MustTimeBounds().MinTime),
			int64(precondXdr.MustTimeBounds().MaxTime),
		)

	case xdr.PreconditionTypePrecondV2:
		inner := precondXdr.MustV2()

		if inner.TimeBounds != nil {
			cond.TimeBounds = NewTimebounds(
				int64(inner.TimeBounds.MinTime),
				int64(inner.TimeBounds.MaxTime),
			)
		}

		if inner.LedgerBounds != nil {
			cond.LedgerBounds = &LedgerBounds{
				MinLedger: uint32(inner.LedgerBounds.MinLedger),
				MaxLedger: uint32(inner.LedgerBounds.MaxLedger),
			}
		}

		if inner.MinSeqNum != nil {
			minSeqNum := int64(*inner.MinSeqNum)
			cond.MinSequenceNumber = &minSeqNum
		}

		cond.MinSequenceNumberAge = uint64(inner.MinSeqAge)
		cond.MinSequenceNumberLedgerGap = uint32(inner.MinSeqLedgerGap)
		if len(inner.ExtraSigners) > 0 {
			cond.ExtraSigners = make([]string, len(inner.ExtraSigners))
			for i, signerKey := range inner.ExtraSigners {
				signer, err := signerKey.GetAddress()
				if err != nil {
					return err
				}
				cond.ExtraSigners[i] = signer
			}
		}

	default:
		return errors.New("unsupported precondition type: " +
			precondXdr.Type.String())
	}

	return nil
}

// hasV2Conditions determines whether or not this has conditions on top of
// the (required) timebound precondition.
func (cond *Preconditions) hasV2Conditions() bool {
	return (cond.LedgerBounds != nil ||
		cond.MinSequenceNumber != nil ||
		cond.MinSequenceNumberAge > 0 ||
		cond.MinSequenceNumberLedgerGap > 0 ||
		len(cond.ExtraSigners) > 0)
}
