package ledgerbackend

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type MockRPCClient struct {
	mock.Mock
}

func (m *MockRPCClient) GetLedgers(ctx context.Context, req protocol.GetLedgersRequest) (protocol.GetLedgersResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(protocol.GetLedgersResponse), args.Error(1)
}

func (m *MockRPCClient) GetHealth(ctx context.Context) (protocol.GetHealthResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(protocol.GetHealthResponse), args.Error(1)
}

func setupRPCTest(t *testing.T) (*RPCLedgerBackend, *MockRPCClient) {
	mockClient := new(MockRPCClient)
	backend := &RPCLedgerBackend{
		client:     mockClient,
		bufferSize: rpcBackendDefaultBufferSize,
		closed:     make(chan struct{}),
	}
	backend.initBuffer()
	return backend, mockClient
}

func TestRPCGetLedger(t *testing.T) {
	rpcBackend, mockClient := setupRPCTest(t)
	ctx := context.Background()
	sequence := uint32(12345)

	mockClient.On("GetHealth", ctx).Return(protocol.GetHealthResponse{
		LatestLedger: sequence + 10,
	}, nil)

	lcm := xdr.LedgerCloseMeta{
		V: 0,
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(sequence),
				},
			},
		},
	}

	encodedLCM, err := xdr.MarshalBase64(lcm)
	assert.NoError(t, err)

	mockSuccessResponse := protocol.GetLedgersResponse{
		Ledgers: []protocol.LedgerInfo{
			{
				Sequence:       sequence,
				LedgerMetadata: encodedLCM,
			},
		},
		LatestLedger: sequence + 10,
	}

	mockMissingLedgerResponse := protocol.GetLedgersResponse{
		Ledgers:      []protocol.LedgerInfo{},
		LatestLedger: sequence + 10,
	}

	expectedReq := protocol.GetLedgersRequest{
		StartLedger: sequence,
		Pagination: &protocol.LedgerPaginationOptions{
			Limit: uint(rpcBackendDefaultBufferSize),
		},
	}
	mockClient.On("GetLedgers", ctx, expectedReq).Return(mockSuccessResponse, nil).Once()

	preparedRange := Range{from: sequence, to: sequence + 10, bounded: true}
	rpcBackend.PrepareRange(ctx, preparedRange)

	// Test ledger found response
	actualLCM, err := rpcBackend.GetLedger(ctx, sequence)
	assert.NoError(t, err)
	assert.Equal(t, sequence, uint32(actualLCM.V0.LedgerHeader.Header.LedgerSeq))

	// Test requesteed ledger is not contiguous, ascending from last invocation
	_, err = rpcBackend.GetLedger(ctx, sequence)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requested ledger 12345 is not the expected ledger 12346")

	// Test requested ledger is outside of prepared range
	_, err = rpcBackend.GetLedger(ctx, sequence+50)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requested ledger 12395 is outside prepared range")

	// Test requested ledger is in valid range of rpc but was missing from response
	notFoundSequnce := sequence + 1
	expectedReq.StartLedger = notFoundSequnce
	mockClient.On("GetLedgers", ctx, expectedReq).Return(mockMissingLedgerResponse, nil).Once()
	_, err = rpcBackend.GetLedger(ctx, notFoundSequnce)
	var missingErr *RPCLedgerMissingError
	assert.ErrorAs(t, err, &missingErr)
	assert.Equal(t, notFoundSequnce, missingErr.Sequence)

	// Test rpc error response
	expectedErr := fmt.Errorf("rpc error")
	mockClient.On("GetLedgers", ctx, expectedReq).Return(protocol.GetLedgersResponse{}, expectedErr).Once()
	_, err = rpcBackend.GetLedger(ctx, sequence+1)
	assert.Contains(t, err.Error(), expectedErr.Error())

	// Verify Closed  backend
	err = rpcBackend.Close()
	assert.NoError(t, err)

	_, err = rpcBackend.GetLedger(ctx, sequence)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RPCLedgerBackend is closed")
}

func TestRPCBackendImplementsInterface(t *testing.T) {
	var rpcBackend interface{} = (*RPCLedgerBackend)(nil)
	_, ok := rpcBackend.(LedgerBackend)
	assert.True(t, ok, "RPCLedgerBackend should implement LedgerBackend interface")
}

func TestNewRPCLedgerBackend(t *testing.T) {

	t.Run("uses default buffer size when 0 provided", func(t *testing.T) {
		opts := RPCLedgerBackendOptions{
			RPCServerURL: "http://localhost:8000",
			BufferSize:   0,
		}
		backend := NewRPCLedgerBackend(opts)
		assert.Equal(t, rpcBackendDefaultBufferSize, backend.bufferSize)
		assert.NotNil(t, backend.client)
	})

	t.Run("uses provided buffer size", func(t *testing.T) {
		opts := RPCLedgerBackendOptions{
			RPCServerURL: "http://localhost:8000",
			BufferSize:   20,
		}
		backend := NewRPCLedgerBackend(opts)
		assert.Equal(t, opts.BufferSize, backend.bufferSize)
		assert.NotNil(t, backend.client)
	})
}

func TestGetLedgerWaitsForLatest(t *testing.T) {
	rpcBackend, mockClient := setupRPCTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	requestedSequence := uint32(100)

	rpcGetLedgersRequest := protocol.GetLedgersRequest{
		StartLedger: requestedSequence,
		Pagination: &protocol.LedgerPaginationOptions{
			Limit: uint(rpcBackendDefaultBufferSize),
		},
	}

	// this gets used on Prepared Range call and first call to GetLedger
	// indicates requested ledger is beyond rpc latest
	mockClient.On("GetHealth", ctx).Return(protocol.GetHealthResponse{
		LatestLedger: requestedSequence - 1,
	}, nil).Twice()

	// Setup call to GetLedger return the requested ledger
	lcm := xdr.LedgerCloseMeta{
		V: 0,
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(requestedSequence),
				},
			},
		},
	}
	encodedLCM, err := xdr.MarshalBase64(lcm)
	assert.NoError(t, err)

	getResponse := protocol.GetLedgersResponse{
		LatestLedger: requestedSequence,
		Ledgers: []protocol.LedgerInfo{
			{
				Sequence:       requestedSequence,
				LedgerMetadata: encodedLCM,
			},
		},
	}
	// called by GetLedger on second try, indicates rpc latest has advanced beyond requested
	mockClient.On("GetHealth", ctx).Return(protocol.GetHealthResponse{
		LatestLedger: requestedSequence + 10,
	}, nil).Once()
	// now it attempts GetLedgers call
	mockClient.On("GetLedgers", ctx, rpcGetLedgersRequest).Return(getResponse, nil).Once()

	preparedRange := Range{from: requestedSequence, to: requestedSequence + 10, bounded: true}
	assert.NoError(t, rpcBackend.PrepareRange(ctx, preparedRange))

	startTime := time.Now()
	// first call to GetLedger gets health status indicating requested ledger is beyond roc latest
	// triggers retry logic
	actualLCM, err := rpcBackend.GetLedger(ctx, requestedSequence)
	duration := time.Since(startTime)

	assert.NoError(t, err)
	assert.Equal(t, requestedSequence, uint32(actualLCM.V0.LedgerHeader.Header.LedgerSeq))

	// Verify timing - GetLedger should have waited one retry interval and then refetched ledgers from rpc on second call
	assert.GreaterOrEqual(t, duration.Seconds(), float64(rpcBackendDefaultWaitIntervalSeconds))
}

func TestGetLedgerContextTimeoutInterrupt(t *testing.T) {
	rpcBackend, mockClient := setupRPCTest(t)
	sequence := uint32(100)
	background := context.Background()
	// Create second context with short timeout
	ctx, cancel := context.WithTimeout(background, 100*time.Millisecond)
	defer cancel()

	mockClient.On("GetHealth", mock.Anything).Return(protocol.GetHealthResponse{
		LatestLedger: sequence - 1,
	}, nil).Twice()

	// Prepare the range first, it doesn't mind the beyond latest response
	preparedRange := Range{from: sequence, to: sequence + 10, bounded: true}
	rpcBackend.PrepareRange(background, preparedRange)

	// Call GetLedger using a timeout ctx, it will retry after pause due to beyond latest
	// and run into the context deadline exceeded
	_, err := rpcBackend.GetLedger(ctx, sequence)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestGetLedgerClosedInterrupt(t *testing.T) {
	rpcBackend, mockClient := setupRPCTest(t)
	sequence := uint32(100)
	ctx := context.Background()

	healthResponse := protocol.GetHealthResponse{
		LatestLedger: sequence - 1,
	}

	// prepare range calls it
	mockClient.On("GetHealth", ctx).Return(healthResponse, nil).Once()

	// get ledger calls it again,
	// and artificallly simulate backedn being closed by caller
	mockClient.On("GetHealth", ctx).
		Run(func(args mock.Arguments) {
			assert.NoError(t, rpcBackend.Close())
		}).Return(healthResponse, nil).Once()

	// Prepare the range first, it doesn't mind the beyond latest response
	preparedRange := Range{from: sequence, to: sequence + 10, bounded: true}
	rpcBackend.PrepareRange(ctx, preparedRange)

	// Call GetLedger it will attempt roc retry and detect closed after
	_, err := rpcBackend.GetLedger(ctx, sequence)
	assert.ErrorContains(t, err, "RPCLedgerBackend is closed")
}

func TestPrepareRange(t *testing.T) {
	t.Run("bounded range", func(t *testing.T) {
		backend, mockClient := setupRPCTest(t)
		ctx := context.Background()
		start := uint32(150)

		mockClient.On("GetHealth", ctx).Return(protocol.GetHealthResponse{
			LatestLedger: start + 10,
		}, nil).Once()
		expectedReq := protocol.GetLedgersRequest{
			StartLedger: start,
			Pagination: &protocol.LedgerPaginationOptions{
				Limit: uint(rpcBackendDefaultBufferSize),
			},
		}
		mockResponse := protocol.GetLedgersResponse{
			LatestLedger: start + 10,
			Ledgers:      []protocol.LedgerInfo{generateRPCInfo(start)},
		}
		mockClient.On("GetLedgers", ctx, expectedReq).Return(mockResponse, nil)

		err := backend.PrepareRange(ctx, Range{from: start, to: start + 30, bounded: true})
		assert.NoError(t, err)
		assert.Equal(t, uint32(150), backend.nextLedger)

		// Second prepare should fail
		err = backend.PrepareRange(ctx, Range{from: start, to: start + 30, bounded: true})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already prepared")
	})

	t.Run("unbounded range", func(t *testing.T) {
		backend, mockClient := setupRPCTest(t)
		ctx := context.Background()
		start := uint32(150)

		mockClient.On("GetHealth", ctx).Return(protocol.GetHealthResponse{
			LatestLedger: start + 10,
		}, nil).Once()

		expectedReq := protocol.GetLedgersRequest{
			StartLedger: start,
			Pagination: &protocol.LedgerPaginationOptions{
				Limit: uint(rpcBackendDefaultBufferSize),
			},
		}
		mockResponse := protocol.GetLedgersResponse{
			LatestLedger: start + 10,
			Ledgers:      []protocol.LedgerInfo{generateRPCInfo(start)},
		}
		mockClient.On("GetLedgers", ctx, expectedReq).Return(mockResponse, nil)

		err := backend.PrepareRange(ctx, Range{from: 150, bounded: false})
		assert.NoError(t, err)
		assert.Equal(t, uint32(150), backend.nextLedger)
	})

	t.Run("error when RPC returns error", func(t *testing.T) {
		backend, mockClient := setupRPCTest(t)
		ctx := context.Background()

		expectedErr := fmt.Errorf("rpc server side reported error")
		start := uint32(150)

		mockClient.On("GetHealth", ctx).Return(protocol.GetHealthResponse{
			LatestLedger: start + 10,
		}, nil).Once()

		expectedReq := protocol.GetLedgersRequest{
			StartLedger: start,
			Pagination: &protocol.LedgerPaginationOptions{
				Limit: uint(rpcBackendDefaultBufferSize),
			},
		}

		mockClient.On("GetLedgers", ctx, expectedReq).Return(protocol.GetLedgersResponse{}, expectedErr).Once()

		err := backend.PrepareRange(ctx, Range{from: start, to: start + 10, bounded: true})
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns error when closed", func(t *testing.T) {
		backend, _ := setupRPCTest(t)
		ctx := context.Background()

		// Close the backend
		err := backend.Close()
		assert.NoError(t, err)

		// Verify checkClosed returns error
		err = backend.PrepareRange(ctx, Range{from: 100, to: 105, bounded: true})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "RPCLedgerBackend is closed")
	})
}

func TestIsPrepared(t *testing.T) {
	t.Run("returns false when backend is not prepared", func(t *testing.T) {
		backend, _ := setupRPCTest(t)

		prepared, err := backend.IsPrepared(context.Background(), Range{from: 100, to: 200, bounded: true})
		assert.NoError(t, err)
		assert.False(t, prepared, "should return false when no range is prepared")
	})

	t.Run("returns true when ranges match exactly", func(t *testing.T) {
		backend, _ := setupRPCTest(t)
		ctx := context.Background()

		// establish a prepared range
		start := uint32(150)
		targetRange := Range{from: start, to: start + 10, bounded: true}
		backend.preparedRange = &targetRange

		prepared, err := backend.IsPrepared(ctx, targetRange)
		assert.NoError(t, err)
		assert.True(t, prepared, "should return true for exact range match")
	})

	t.Run("returns false when ranges differ", func(t *testing.T) {
		backend, _ := setupRPCTest(t)
		ctx := context.Background()

		start := uint32(150)
		backend.preparedRange = &Range{from: start, to: start + 10, bounded: true}

		// Test different range variations
		testCases := []struct {
			name   string
			range_ Range
		}{
			{"different from", Range{from: start - 10, to: start + 10, bounded: true}},
			{"different to", Range{from: start, to: start + 50, bounded: true}},
			{"different bounded", Range{from: start, bounded: false}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				prepared, err := backend.IsPrepared(ctx, tc.range_)
				assert.NoError(t, err)
				assert.False(t, prepared, "should return false when ranges don't match exactly")
			})
		}
	})

	t.Run("returns error when backend is closed", func(t *testing.T) {
		backend, _ := setupRPCTest(t)
		assert.NoError(t, backend.Close())

		prepared, err := backend.IsPrepared(context.Background(), Range{from: 100, to: 200})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "closed")
		assert.False(t, prepared)
	})
}

func TestRPCBackendGetLatestLedgerSequence(t *testing.T) {
	t.Run("returns error when closed", func(t *testing.T) {
		backend, _ := setupRPCTest(t)
		assert.NoError(t, backend.Close())

		seq, err := backend.GetLatestLedgerSequence(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "RPCLedgerBackend is closed")
		assert.Equal(t, uint32(0), seq)
	})

	t.Run("returns error when not prepared", func(t *testing.T) {
		backend, _ := setupRPCTest(t)

		seq, err := backend.GetLatestLedgerSequence(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be prepared")
		assert.Equal(t, uint32(0), seq)
	})

	t.Run("returns 0 when buffer empty", func(t *testing.T) {
		backend, _ := setupRPCTest(t)

		backend, mockClient := setupRPCTest(t)
		ctx := context.Background()
		start := uint32(150)

		mockClient.On("GetHealth", ctx).Return(protocol.GetHealthResponse{
			LatestLedger: start - 1,
		}, nil).Once()

		// establish a prepared, but was beyond rpc latest, so empty buffer state
		err := backend.PrepareRange(ctx, Range{from: start, to: start + 10, bounded: true})
		assert.NoError(t, err)

		// any attempts to get latest ledger should be zero
		seq, err := backend.GetLatestLedgerSequence(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, uint32(0), seq)
	})

	t.Run("returns greatest ledger from buffer", func(t *testing.T) {
		backend, _ := setupRPCTest(t)

		backend, mockClient := setupRPCTest(t)
		ctx := context.Background()
		start := uint32(150)

		mockClient.On("GetHealth", ctx).Return(protocol.GetHealthResponse{
			LatestLedger: start + 10,
		}, nil).Once()

		expectedReq := protocol.GetLedgersRequest{
			StartLedger: start,
			Pagination: &protocol.LedgerPaginationOptions{
				Limit: uint(rpcBackendDefaultBufferSize),
			},
		}
		ledgerInfos := []protocol.LedgerInfo{}
		for i := start; i <= start+4; i++ {
			ledgerInfos = append(ledgerInfos, generateRPCInfo(i))
		}
		mockResponse := protocol.GetLedgersResponse{
			LatestLedger: start + 10,
			Ledgers:      ledgerInfos,
		}
		mockClient.On("GetLedgers", ctx, expectedReq).Return(mockResponse, nil)

		err := backend.PrepareRange(ctx, Range{from: start, to: start + 10, bounded: true})

		seq, err := backend.GetLatestLedgerSequence(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, uint32(154), seq)
	})
}

func generateRPCInfo(sequence uint32) protocol.LedgerInfo {
	lcm := xdr.LedgerCloseMeta{
		V: 0,
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(sequence),
				},
			},
		},
	}
	encodedLCM, _ := xdr.MarshalBase64(lcm)
	return protocol.LedgerInfo{
		Sequence:       sequence,
		LedgerMetadata: encodedLCM,
	}
}
