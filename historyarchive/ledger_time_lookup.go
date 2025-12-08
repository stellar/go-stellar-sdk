package historyarchive

import (
	"fmt"
	"sort"
	"time"

	"github.com/stellar/go-stellar-sdk/xdr"
)

const firstLedgerWithRealTime int64 = 2 // ledger 1 has closeTime = 0 (Unix epoch)

// LedgerForTime returns the smallest ledger sequence L such that
// closeTime(L) >= target. If target is before the first ledger, it
// returns the genesis ledger. If after the latest, it returns the
// latest ledger.
func LedgerForTime(archive ArchiveInterface, target time.Time) (int64, error) {
	root, err := archive.GetRootHAS()
	if err != nil {
		return 0, fmt.Errorf("getting root HAS: %w", err)
	}

	target = target.UTC()

	// 1) Get bounds: [beginSeq, endSeq]
	beginSeq := firstLedgerWithRealTime

	beginHdr, err := archive.GetLedgerHeader(uint32(beginSeq))
	if err != nil {
		return 0, fmt.Errorf("getting header for begin ledger %d: %w", beginSeq, err)
	}
	beginTime := closeTimeFromHeader(beginHdr)

	endSeq := int64(root.CurrentLedger)
	if endSeq < beginSeq {
		return 0, fmt.Errorf("invalid archive: CurrentLedger=%d < %d", endSeq, beginSeq)
	}

	endHdr, err := archive.GetLedgerHeader(uint32(endSeq))
	if err != nil {
		return 0, fmt.Errorf("getting header for end ledger %d: %w", endSeq, err)
	}
	endTime := closeTimeFromHeader(endHdr)

	// 2) Clamp to [beginTime, endTime]
	if target.Before(beginTime) {
		return beginSeq, nil
	}
	if target.After(endTime) {
		return endSeq, nil
	}

	// 3) Binary search over [beginSeq, endSeq]
	lo := beginSeq
	hi := endSeq
	n := int(hi - lo + 1)

	var errCached error

	idx := sort.Search(n, func(i int) bool {
		if errCached != nil {
			return false
		}

		seq := lo + int64(i)
		hdr, err := archive.GetLedgerHeader(uint32(seq))
		if err != nil {
			errCached = err
			return false
		}

		ct := closeTimeFromHeader(hdr)
		// First ledger whose closeTime >= target (inclusive)
		return !ct.Before(target)
	})

	if errCached != nil {
		return 0, errCached
	}
	if idx == n {
		return 0, fmt.Errorf("no ledger found for time %v", target)
	}

	return lo + int64(idx), nil
}

// LedgerRangeForTimes returns [startLedger, endLedger] such that:
//
//   - startLedger is the first ledger whose closeTime >= startTime
//   - endLedger   is the first ledger whose closeTime >= endTime
//
// startTime must be <= endTime.
func LedgerRangeForTimes(archive ArchiveInterface, startTime, endTime time.Time) (int64, int64, error) {
	startTime = startTime.UTC()
	endTime = endTime.UTC()

	if startTime.After(endTime) {
		return 0, 0, fmt.Errorf("startTime must be <= endTime")
	}

	startSeq, err := LedgerForTime(archive, startTime)
	if err != nil {
		return 0, 0, fmt.Errorf("finding start ledger: %w", err)
	}

	endSeq, err := LedgerForTime(archive, endTime)
	if err != nil {
		return 0, 0, fmt.Errorf("finding end ledger: %w", err)
	}

	return startSeq, endSeq, nil
}

// closeTimeFromHeader converts the ledger header's CloseTime (TimePoint seconds)
// into a UTC time.Time.
func closeTimeFromHeader(h xdr.LedgerHeaderHistoryEntry) time.Time {
	sec := int64(h.Header.ScpValue.CloseTime)
	return time.Unix(sec, 0).UTC()
}
