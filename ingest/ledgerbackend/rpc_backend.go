package ledgerbackend

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	rpc "github.com/stellar/go-stellar-sdk/clients/rpcclient"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const rpcBackendDefaultBufferSize uint32 = 10
const rpcBackendDefaultWaitIntervalSeconds uint32 = 2

type RPCLedgerMissingError struct {
	Sequence uint32
}

func (e *RPCLedgerMissingError) Error() string {
	return fmt.Sprintf("ledger %d was not present on rpc", e.Sequence)
}

type rpcLedgerBeyondLatestError struct{}

func (e rpcLedgerBeyondLatestError) Error() string {
	return "ledger is not available on the RPC server yet"
}

// The minimum required RPC client methods used by RPCLedgerBackend.
type RPCLedgerGetter interface {
	GetLedgers(ctx context.Context, req protocol.GetLedgersRequest) (protocol.GetLedgersResponse, error)
	GetHealth(ctx context.Context) (protocol.GetHealthResponse, error) // <-- Added
}

type RPCLedgerBackend struct {
	client             RPCLedgerGetter
	buffer             map[uint32]xdr.LedgerCloseMeta
	bufferSize         uint32
	preparedRange      *Range
	nextLedger         uint32
	latestBufferLedger atomic.Uint32
	closed             chan struct{}
	closedOnce         sync.Once
	bufferLock         sync.RWMutex
}

type RPCLedgerBackendOptions struct {
	// Required, URL of the Stellar RPC server
	RPCServerURL string

	// Optional, size of the ledger retrieval buffer to use with RPC server requests.
	// if not set, defaults to 10
	BufferSize uint32

	// Optional, custom HTTP client to use for RPC requests.
	// If nil, the default http.Client will be used.
	HttpClient *http.Client
}

// NewRPCLedgerBackend creates a new RPCLedgerBackend instance that fetches ledger data
// from a Stellar RPC server.
//
// Parameters:
//   - options: RPCLedgerBackendOptions
//
// Returns:
//   - *RPCLedgerBackend: A new backend instance ready for use
func NewRPCLedgerBackend(options RPCLedgerBackendOptions) *RPCLedgerBackend {
	backend := &RPCLedgerBackend{
		closed:     make(chan struct{}),
		client:     rpc.NewClient(options.RPCServerURL, options.HttpClient),
		bufferSize: options.BufferSize,
	}

	if backend.bufferSize == 0 {
		backend.bufferSize = rpcBackendDefaultBufferSize
	}
	backend.initBuffer()
	return backend
}

// GetLatestLedgerSequence returns the latest ledger sequence currently loaded by internal buffer.
func (b *RPCLedgerBackend) GetLatestLedgerSequence(ctx context.Context) (sequence uint32, err error) {
	if err := b.checkClosed(); err != nil {
		return 0, err
	}

	if b.preparedRange == nil {
		return 0, fmt.Errorf("RPCLedgerBackend must be prepared before calling GetLatestLedgerSequence")
	}

	return b.latestBufferLedger.Load(), nil
}

// GetLedger queries the RPC server for a specific ledger sequence and returns the meta data.
// If the requested ledger is not immediately available but is beyond the latest ledger in the RPC server,
// it will block and retry until either:
//   - The ledger becomes available
//   - The context is cancelled or times out
//   - The backend is closed
//   - An error occurs while fetching the ledger
//
// The caller can control the maximum wait time by setting a timeout or deadline on the provided context.
// Or by invoking the Close method from another goroutine.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - sequence: The ledger sequence number to retrieve meta data for
//
// Returns:
//   - xdr.LedgerCloseMeta: The ledger meta data if found
//   - error:  if ledger sequence is not avaialble from the RPC server,
//     or context is cancelled or times out.
func (b *RPCLedgerBackend) GetLedger(ctx context.Context, sequence uint32) (xdr.LedgerCloseMeta, error) {
	b.bufferLock.Lock()
	defer b.bufferLock.Unlock()

	if err := b.checkClosed(); err != nil {
		return xdr.LedgerCloseMeta{}, err
	}

	if b.preparedRange == nil {
		return xdr.LedgerCloseMeta{}, fmt.Errorf("RPCLedgerBackend must be prepared before calling GetLedger")
	}

	if sequence < b.preparedRange.from || (b.preparedRange.bounded && sequence > b.preparedRange.to) {
		return xdr.LedgerCloseMeta{}, fmt.Errorf("requested ledger %d is outside prepared range [%d, %d]",
			sequence, b.preparedRange.from, b.preparedRange.to)
	}

	if sequence != b.nextLedger {
		return xdr.LedgerCloseMeta{}, fmt.Errorf("requested ledger %d is not the expected ledger %d", sequence, b.nextLedger)
	}

	for {
		lcm, err := b.getBufferedLedger(ctx, sequence)
		if err == nil {
			b.nextLedger = sequence + 1
			return lcm, nil
		}

		var beyondErr *rpcLedgerBeyondLatestError
		if !(errors.As(err, &beyondErr)) {
			return xdr.LedgerCloseMeta{}, err
		}

		select {
		case <-b.closed:
			return xdr.LedgerCloseMeta{}, fmt.Errorf("RPCLedgerBackend is closed: %w", err)
		case <-ctx.Done():
			return xdr.LedgerCloseMeta{}, ctx.Err()
		case <-time.After(time.Duration(rpcBackendDefaultWaitIntervalSeconds) * time.Second):
			continue
		}
	}
}

// PrepareRange initiates retrieval of requested ledger range.
// It does minimal validation of data on RPC up front.
// It wil check if starting point of range is withing current historical retention window of the RPC server.
// It cannot gaurantee ledgers within historical ranges will be available when requested later by GetLedger.
// See Also: GetLedger for more details on how the RPCLedgerBackend handles ledger availability.
func (b *RPCLedgerBackend) PrepareRange(ctx context.Context, ledgerRange Range) error {
	b.bufferLock.Lock()
	defer b.bufferLock.Unlock()

	if err := b.checkClosed(); err != nil {
		return err
	}

	if b.preparedRange != nil {
		return fmt.Errorf("RPCLedgerBackend is already prepared with range [%d, %d]", b.preparedRange.from, b.preparedRange.to)
	}

	_, err := b.getBufferedLedger(ctx, ledgerRange.from)
	if err != nil {
		// beyond latest is handled later in GetLedger
		var beyondErr *rpcLedgerBeyondLatestError
		if !(errors.As(err, &beyondErr)) {
			return err
		}
	}

	b.nextLedger = ledgerRange.from
	b.preparedRange = &ledgerRange
	return nil
}

func (b *RPCLedgerBackend) IsPrepared(ctx context.Context, ledgerRange Range) (bool, error) {
	b.bufferLock.RLock()
	defer b.bufferLock.RUnlock()

	if err := b.checkClosed(); err != nil {
		return false, err
	}

	if b.preparedRange == nil {
		return false, nil
	}

	rangesMatch := b.preparedRange.from == ledgerRange.from &&
		b.preparedRange.bounded == ledgerRange.bounded &&
		(!b.preparedRange.bounded || b.preparedRange.to == ledgerRange.to)

	return rangesMatch, nil
}

// Close cleans up the RPCLedgerBackend resources and closes the backend.
// It will halt any ongoing GerLedger calls that may be in progress on other goroutines.
func (b *RPCLedgerBackend) Close() error {
	b.closedOnce.Do(func() {
		close(b.closed)
	})
	return nil
}

func (b *RPCLedgerBackend) checkClosed() error {
	select {
	case <-b.closed:
		return fmt.Errorf("RPCLedgerBackend is closed")
	default:
		return nil
	}
}

func (b *RPCLedgerBackend) initBuffer() {
	b.buffer = make(map[uint32]xdr.LedgerCloseMeta)
}

func (b *RPCLedgerBackend) getBufferedLedger(ctx context.Context, sequence uint32) (xdr.LedgerCloseMeta, error) {
	// Check if ledger is in buffer
	if lcm, exists := b.buffer[sequence]; exists {
		return lcm, nil
	}

	// Check if requested ledger is beyond the RPC retention window using GetHealth
	health, err := b.client.GetHealth(ctx)
	if err != nil {
		return xdr.LedgerCloseMeta{}, fmt.Errorf("failed to get health from RPC: %w", err)
	}
	if sequence > health.LatestLedger {
		return xdr.LedgerCloseMeta{}, &rpcLedgerBeyondLatestError{}
	}

	// attempt to fetch a small batch from RPC starting from the requested sequence
	req := protocol.GetLedgersRequest{
		StartLedger: sequence,
		Pagination: &protocol.LedgerPaginationOptions{
			Limit: uint(b.bufferSize),
		},
	}

	ledgers, err := b.client.GetLedgers(ctx, req)
	if err != nil {
		return xdr.LedgerCloseMeta{}, err
	}

	b.initBuffer()

	// Populate buffer with new ledgers
	for _, ledger := range ledgers.Ledgers {
		var lcm xdr.LedgerCloseMeta
		if err := xdr.SafeUnmarshalBase64(ledger.LedgerMetadata, &lcm); err != nil {
			return xdr.LedgerCloseMeta{}, fmt.Errorf("failed to unmarshal ledger %d: %w", ledger.Sequence, err)
		}
		b.buffer[ledger.Sequence] = lcm
	}

	latestSeq := uint32(0)
	if size := len(ledgers.Ledgers); size > 0 {
		latestSeq = ledgers.Ledgers[size-1].Sequence
	}
	b.latestBufferLedger.Store(latestSeq)

	// Check if requested ledger is in new buffer
	if lcm, exists := b.buffer[sequence]; exists {
		return lcm, nil
	}

	return xdr.LedgerCloseMeta{}, &RPCLedgerMissingError{Sequence: sequence}
}
