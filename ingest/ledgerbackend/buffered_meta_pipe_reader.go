package ledgerbackend

import (
	"bufio"
	"io"
	"time"

	"github.com/pkg/errors"
	xdr3 "github.com/stellar/go-xdr/xdr3"

	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const (
	// The constants below define sizes of metaPipeBufferSize (binary) and
	// ledgerReadAheadBufferSize (in ledgers). In general:
	//
	//   metaPipeBufferSize >=
	//    ledgerReadAheadBufferSize * max over networks (average ledger size in bytes)
	//
	// so that meta pipe buffer always have binary data that can be unmarshaled into
	// ledger buffer.
	// After checking a few latest ledgers in pubnet and testnet the average size
	// is: 100,000 and 5,000 bytes respectively.

	// metaPipeBufferSize defines the meta pipe buffer size. We need at least
	// a couple MB to ensure there are at least a few ledgers captive core can
	// unmarshal into read-ahead buffer while waiting for client to finish
	// processing previous ledgers.
	metaPipeBufferSize = 10 * 1024 * 1024
	// ledgerReadAheadBufferSize defines the size (in ledgers) of read ahead
	// buffer that stores unmarshalled ledgers. This is especially important in
	// an online mode when GetLedger calls are not blocking. In such case, clients
	// usually wait for a specific time duration before checking if the ledger is
	// available. When catching up and small buffer this can increase the overall
	// time because ledgers are not available.
	ledgerReadAheadBufferSize = 20
)

type metaResult struct {
	*xdr.LedgerCloseMeta
	err error
}

// bufferedLedgerMetaReader is responsible for buffering meta pipe data in a
// fast and safe manner and unmarshaling it into XDR objects.
//
// It solves the following issues:
//
//   - Decouples buffering from stellarCoreRunner so it can focus on running core.
//   - Decouples unmarshaling and buffering of LedgerCloseMeta's from CaptiveCore.
//   - By adding buffering it allows unmarshaling the ledgers available in Stellar-Core
//     while previous ledger are being processed.
//   - Limits memory usage in case of large ledgers are closed by the network.
//
// Internally, it keeps two buffers: bufio.Reader with binary ledger data and
// buffered channel with unmarshaled xdr.LedgerCloseMeta objects ready for
// processing. The first buffer removes overhead time connected to reading from
// a file. The second buffer allows unmarshaling binary data into XDR objects
// (which can be a bottleneck) while clients are processing previous ledgers.
//
// Finally, when a large ledger (larger than binary buffer) is closed it waits
// until xdr.LedgerCloseMeta objects channel is empty. This prevents memory
// exhaustion when network closes a series a large ledgers.
type bufferedLedgerMetaReader struct {
	r       *bufio.Reader
	c       chan metaResult
	decoder *xdr3.Decoder
}

// newBufferedLedgerMetaReader creates a new meta reader that will shutdown
// when stellar-core terminates.
func newBufferedLedgerMetaReader(reader io.Reader) *bufferedLedgerMetaReader {
	r := bufio.NewReaderSize(reader, metaPipeBufferSize)
	return &bufferedLedgerMetaReader{
		c:       make(chan metaResult, ledgerReadAheadBufferSize),
		r:       r,
		decoder: xdr3.NewDecoder(r),
	}
}

// readLedgerMetaFromPipe unmarshalls the next ledger from meta pipe.
// It can block for two reasons:
//   - Meta pipe buffer is full so it will wait until it refills.
//   - The next ledger available in the buffer exceeds the meta pipe buffer size.
//     In such case the method will block until LedgerCloseMeta buffer is empty.
func (b *bufferedLedgerMetaReader) readLedgerMetaFromPipe() (*xdr.LedgerCloseMeta, error) {
	frameLength, err := xdr.ReadFrameLength(b.decoder)
	if err != nil {
		return nil, errors.Wrap(err, "error reading frame length")
	}

	for frameLength > metaPipeBufferSize && len(b.c) > 0 {
		// Wait for LedgerCloseMeta buffer to be cleared to minimize memory usage.
		<-time.After(time.Second)
	}

	var xlcm xdr.LedgerCloseMeta
	_, err = xlcm.DecodeFrom(b.decoder, xdr3.DecodeDefaultMaxDepth)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshaling framed LedgerCloseMeta")
	}
	return &xlcm, nil
}

func (b *bufferedLedgerMetaReader) getChannel() <-chan metaResult {
	return b.c
}

// Start starts a loop that reads binary ledger data into internal buffers.
// The function returns when it encounters an error (including io.EOF).
func (b *bufferedLedgerMetaReader) start() {
	printBufferOccupation := time.NewTicker(5 * time.Second)
	defer printBufferOccupation.Stop()
	defer close(b.c)

	for {
		select {
		case <-printBufferOccupation.C:
			log.Debug("captive core read-ahead buffer occupation:", len(b.c))
		default:
		}

		meta, err := b.readLedgerMetaFromPipe()
		if err != nil {
			b.c <- metaResult{nil, err}
			return
		}

		b.c <- metaResult{meta, nil}
	}
}
