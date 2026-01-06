package historyarchive

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func makeHeader(ts time.Time) xdr.LedgerHeaderHistoryEntry {
	var h xdr.LedgerHeaderHistoryEntry
	h.Header.ScpValue.CloseTime = xdr.TimePoint(uint64(ts.Unix()))
	return h
}

func TestLedgerForTime(t *testing.T) {
	m := &MockArchive{}
	base := time.Unix(1_600_000_000, 0).UTC()

	// Ledger timeline:
	// seq=2 → base
	// seq=3 → base+5s
	// seq=4 → base+10s
	m.On("GetRootHAS").
		Return(HistoryArchiveState{CurrentLedger: 4}, nil)

	m.On("GetLedgerHeader", uint32(2)).
		Return(makeHeader(base), nil)
	m.On("GetLedgerHeader", uint32(3)).
		Return(makeHeader(base.Add(5*time.Second)), nil)
	m.On("GetLedgerHeader", uint32(4)).
		Return(makeHeader(base.Add(10*time.Second)), nil)

	// Target: halfway between ledger 2 and 3
	target := base.Add(3 * time.Second)

	got, err := LedgerForTime(m, target)
	assert.NoError(t, err)
	assert.Equal(t, uint32(3), got)

	m.AssertExpectations(t)
}

func TestLedgerRangeForTimes(t *testing.T) {
	m := &MockArchive{}
	base := time.Unix(1_600_000_000, 0).UTC()

	// Ledger timeline:
	// 2 → base
	// 3 → base+5s
	// 4 → base+10s
	m.On("GetRootHAS").Return(HistoryArchiveState{CurrentLedger: 4}, nil)

	m.On("GetLedgerHeader", uint32(2)).Return(makeHeader(base), nil)
	m.On("GetLedgerHeader", uint32(3)).Return(makeHeader(base.Add(5*time.Second)), nil)
	m.On("GetLedgerHeader", uint32(4)).Return(makeHeader(base.Add(10*time.Second)), nil)

	start := base                    // => ledger 2
	end := base.Add(7 * time.Second) // between 3 and 4 → ledger 4

	startSeq, endSeq, err := LedgerRangeForTimespan(m, start, end)
	assert.NoError(t, err)
	assert.Equal(t, uint32(2), startSeq)
	assert.Equal(t, uint32(4), endSeq)

	m.AssertExpectations(t)
}

func TestLedgerForTime_RootError(t *testing.T) {
	m := &MockArchive{}

	base := time.Unix(1_600_000_000, 0).UTC()
	m.On("GetRootHAS").Return(HistoryArchiveState{}, fmt.Errorf("boom"))
	m.On("GetLedgerHeader", uint32(2)).Return(makeHeader(base), nil)

	_, err := LedgerForTime(m, time.Now())
	assert.Error(t, err)
}

func TestLedgerForTime_HeaderError(t *testing.T) {
	m := &MockArchive{}
	base := time.Unix(1_600_000_000, 0).UTC()

	m.On("GetRootHAS").Return(HistoryArchiveState{CurrentLedger: 4}, nil)

	// begin and end headers okay
	m.On("GetLedgerHeader", uint32(2)).Return(makeHeader(base), nil)
	m.On("GetLedgerHeader", uint32(4)).Return(makeHeader(base.Add(10*time.Second)), nil)
	// but middle one blows up
	m.On("GetLedgerHeader", uint32(3)).Return(xdr.LedgerHeaderHistoryEntry{}, fmt.Errorf("boom"))

	_, err := LedgerForTime(m, base.Add(6*time.Second))
	assert.Error(t, err)

	m.AssertExpectations(t)
}

func TestLedgerRangeForTimes_SameStartEnd(t *testing.T) {
	m := &MockArchive{}
	base := time.Unix(1_600_000_000, 0).UTC()

	m.On("GetRootHAS").Return(HistoryArchiveState{CurrentLedger: 3}, nil)
	m.On("GetLedgerHeader", uint32(2)).Return(makeHeader(base), nil)
	m.On("GetLedgerHeader", uint32(3)).Return(makeHeader(base.Add(5*time.Second)), nil)

	// Exactly equal to ledger 3 close time
	ts := base.Add(5 * time.Second)

	startSeq, endSeq, err := LedgerRangeForTimespan(m, ts, ts)
	assert.NoError(t, err)
	assert.Equal(t, uint32(3), startSeq)
	assert.Equal(t, uint32(3), endSeq)

	m.AssertExpectations(t)
}
