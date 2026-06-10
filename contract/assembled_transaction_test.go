package contract

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockRPCClient is the package's test double for RPCClient.
type mockRPCClient struct {
	mock.Mock
}

var _ RPCClient = (*mockRPCClient)(nil)

func (m *mockRPCClient) SimulateTransaction(
	ctx context.Context,
	req protocol.SimulateTransactionRequest) (protocol.SimulateTransactionResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(protocol.SimulateTransactionResponse), args.Error(1)
}

func (m *mockRPCClient) LoadAccount(ctx context.Context, addr string) (txnbuild.Account, error) {
	args := m.Called(ctx, addr)
	acct, _ := args.Get(0).(txnbuild.Account)
	return acct, args.Error(1)
}

// helpers ---------------------------------------------------------------

// newTestInvokeOp builds an InvokeHostFunction op invoking `bump(amount: 7)`
// on a deterministic contract ID. The auth + soroban data are placeholders
// that Simulate will overwrite.
func newTestInvokeOp(t *testing.T, source string) *txnbuild.InvokeHostFunction {
	t.Helper()
	var cid xdr.ContractId
	for i := range cid {
		cid[i] = byte(i + 1)
	}
	args := xdr.ScVec{
		{Type: xdr.ScValTypeScvU32, U32: u32ptr(7)},
	}
	return &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: xdr.ScAddress{
					Type:       xdr.ScAddressTypeScAddressTypeContract,
					ContractId: &cid,
				},
				FunctionName: "bump",
				Args:         args,
			},
		},
		SourceAccount: source,
		Ext: xdr.TransactionExt{
			V: 1,
			SorobanData: &xdr.SorobanTransactionData{
				Resources: xdr.SorobanResources{
					Instructions:  100,
					DiskReadBytes: 100,
					WriteBytes:    100,
				},
				ResourceFee: 1_000,
			},
		},
	}
}

func u32ptr(v xdr.Uint32) *xdr.Uint32 { return &v }

// canonical SorobanTransactionData the fake simulator returns.
func cannedSorobanData(t *testing.T) (xdr.SorobanTransactionData, string) {
	t.Helper()
	data := xdr.SorobanTransactionData{
		Resources: xdr.SorobanResources{
			Instructions:  9_000_000,
			DiskReadBytes: 4_096,
			WriteBytes:    2_048,
		},
		ResourceFee: 7_777_777,
	}
	b64, err := xdr.MarshalBase64(data)
	require.NoError(t, err)
	return data, b64
}

// canned auth entry: SourceAccount credentials, function-call invocation.
func cannedAuthEntry(t *testing.T) (xdr.SorobanAuthorizationEntry, string) {
	t.Helper()
	var cid xdr.ContractId
	for i := range cid {
		cid[i] = byte(i + 1)
	}
	entry := xdr.SorobanAuthorizationEntry{
		Credentials: xdr.SorobanCredentials{
			Type: xdr.SorobanCredentialsTypeSorobanCredentialsSourceAccount,
		},
		RootInvocation: xdr.SorobanAuthorizedInvocation{
			Function: xdr.SorobanAuthorizedFunction{
				Type: xdr.SorobanAuthorizedFunctionTypeSorobanAuthorizedFunctionTypeContractFn,
				ContractFn: &xdr.InvokeContractArgs{
					ContractAddress: xdr.ScAddress{
						Type:       xdr.ScAddressTypeScAddressTypeContract,
						ContractId: &cid,
					},
					FunctionName: "bump",
				},
			},
		},
	}
	b64, err := xdr.MarshalBase64(entry)
	require.NoError(t, err)
	return entry, b64
}

// cannedWriteFootprintData returns SorobanTransactionData whose ReadWrite
// footprint touches one ledger key, so IsReadCall reports false (a write call).
func cannedWriteFootprintData(t *testing.T) (xdr.SorobanTransactionData, string) {
	t.Helper()
	var cid xdr.ContractId
	for i := range cid {
		cid[i] = byte(i + 1)
	}
	key := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractData,
		ContractData: &xdr.LedgerKeyContractData{
			Contract:   xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeContract, ContractId: &cid},
			Key:        xdr.ScVal{Type: xdr.ScValTypeScvLedgerKeyContractInstance},
			Durability: xdr.ContractDataDurabilityPersistent,
		},
	}
	data := xdr.SorobanTransactionData{
		Resources: xdr.SorobanResources{
			Footprint: xdr.LedgerFootprint{ReadWrite: []xdr.LedgerKey{key}},
		},
		ResourceFee: 1_000,
	}
	b64, err := xdr.MarshalBase64(data)
	require.NoError(t, err)
	return data, b64
}

// canned return value: scvU32(42).
func cannedReturnValue(t *testing.T) (xdr.ScVal, string) {
	t.Helper()
	v := xdr.ScVal{Type: xdr.ScValTypeScvU32, U32: u32ptr(42)}
	b64, err := xdr.MarshalBase64(v)
	require.NoError(t, err)
	return v, b64
}

func newAssembleParams(t *testing.T, rpc RPCClient) AssembleParams {
	t.Helper()
	kp := keypair.MustRandom()
	acct := txnbuild.NewSimpleAccount(kp.Address(), 42)
	return AssembleParams{
		RPC:               rpc,
		NetworkPassphrase: network.TestNetworkPassphrase,
		BaseFee:           txnbuild.MinBaseFee,
		SourceAccount:     &acct,
		Op:                newTestInvokeOp(t, kp.Address()),
		Preconditions:     txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()},
	}
}

// constructor sanity ----------------------------------------------------

func TestNewAssembledTransaction_PopulatesMethodAndArgs(t *testing.T) {
	rpc := &mockRPCClient{}
	at, err := NewAssembledTransaction(newAssembleParams(t, rpc))
	require.NoError(t, err)
	require.NotNil(t, at.Built)
	assert.Equal(t, "bump", at.Method)
	require.Len(t, at.Args, 1)
	assert.Equal(t, xdr.ScValTypeScvU32, at.Args[0].Type)
	assert.Equal(t, DefaultResourceFeeMultiplier, at.resourceFeeMultiplier)

	// No simulation has run yet.
	assert.Nil(t, at.Simulation)
	assert.Nil(t, at.AuthEntries)
	assert.Nil(t, at.ReturnValue)

	// Built tx wraps the host function we passed in.
	ops := at.Built.Operations()
	require.Len(t, ops, 1)
	_, ok := ops[0].(*txnbuild.InvokeHostFunction)
	assert.True(t, ok, "operation should be *InvokeHostFunction")
}

func TestNewAssembledTransaction_RejectsMissingFields(t *testing.T) {
	good := newAssembleParams(t, &mockRPCClient{})

	cases := []struct {
		name    string
		mutate  func(*AssembleParams)
		wantSub string
	}{
		{"nil rpc", func(p *AssembleParams) { p.RPC = nil }, "RPC is required"},
		{"empty network", func(p *AssembleParams) { p.NetworkPassphrase = "" }, "NetworkPassphrase"},
		{"nil source", func(p *AssembleParams) { p.SourceAccount = nil }, "SourceAccount"},
		{"nil op", func(p *AssembleParams) { p.Op = nil }, "Op is required"},
		{"low base fee", func(p *AssembleParams) { p.BaseFee = 1 }, "MinBaseFee"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := good
			tc.mutate(&p)
			_, err := NewAssembledTransaction(p)
			require.Error(t, err)
			var e *Error
			require.True(t, errors.As(err, &e))
			assert.Equal(t, KindInvalidArgs, e.Kind)
			assert.Contains(t, e.Error(), tc.wantSub)
		})
	}
}

func TestNewAssembledTransaction_DefaultResourceFeeMultiplier(t *testing.T) {
	p := newAssembleParams(t, &mockRPCClient{})
	p.ResourceFeeMultiplier = 0
	at, err := NewAssembledTransaction(p)
	require.NoError(t, err)
	assert.Equal(t, DefaultResourceFeeMultiplier, at.resourceFeeMultiplier)

	p.ResourceFeeMultiplier = 2.0
	at, err = NewAssembledTransaction(p)
	require.NoError(t, err)
	assert.Equal(t, 2.0, at.resourceFeeMultiplier)
}

func TestNewAssembledTransaction_DefaultsZeroTimeBounds(t *testing.T) {
	// Preconditions is documented optional: a zero-value Preconditions must
	// build successfully, defaulting TimeBounds to an infinite timeout (txnbuild
	// otherwise rejects a TimeBounds not built via a factory method).
	p := newAssembleParams(t, &mockRPCClient{})
	p.Preconditions = txnbuild.Preconditions{}
	at, err := NewAssembledTransaction(p)
	require.NoError(t, err)
	tb := at.Built.Timebounds()
	assert.Equal(t, int64(0), tb.MinTime)
	assert.Equal(t, txnbuild.TimeoutInfinite, tb.MaxTime)

	// An explicitly constructed TimeBounds is preserved, not overwritten.
	p.Preconditions = txnbuild.Preconditions{TimeBounds: txnbuild.NewTimebounds(5, 10)}
	at, err = NewAssembledTransaction(p)
	require.NoError(t, err)
	tb = at.Built.Timebounds()
	assert.Equal(t, int64(5), tb.MinTime)
	assert.Equal(t, int64(10), tb.MaxTime)
}

// Simulate happy path --------------------------------------------------

func TestAssembledTransaction_Simulate_HappyPath(t *testing.T) {
	wantData, dataB64 := cannedSorobanData(t)
	_, authB64 := cannedAuthEntry(t)
	_, retB64 := cannedReturnValue(t)

	rpc := &mockRPCClient{}
	rpc.On("SimulateTransaction", mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{
		TransactionDataXDR: dataB64,
		MinResourceFee:     1_000_000,
		Results: []protocol.SimulateHostFunctionResult{
			{
				AuthXDR:        &[]string{authB64},
				ReturnValueXDR: &retB64,
			},
		},
		LatestLedger: 123,
	}, nil)
	at, err := NewAssembledTransaction(newAssembleParams(t, rpc))
	require.NoError(t, err)
	require.NoError(t, at.Simulate(context.Background()))

	// Simulator was called once, with a non-empty encoded tx.
	rpc.AssertNumberOfCalls(t, "SimulateTransaction", 1)
	rpc.AssertCalled(t, "SimulateTransaction", mock.Anything,
		mock.MatchedBy(func(req protocol.SimulateTransactionRequest) bool { return req.Transaction != "" }))

	// State mutated.
	require.NotNil(t, at.Simulation)
	require.Len(t, at.AuthEntries, 1)
	require.NotNil(t, at.ReturnValue)
	assert.Equal(t, xdr.ScValTypeScvU32, at.ReturnValue.Type)
	assert.Equal(t, xdr.Uint32(42), *at.ReturnValue.U32)

	// SorobanData on the underlying op should reflect the simulation, with the
	// simulated resource fee written verbatim (default multiplier is 1.0, no pad).
	require.NotNil(t, at.op.Ext.SorobanData)
	assert.Equal(t, wantData.Resources.Instructions, at.op.Ext.SorobanData.Resources.Instructions)
	assert.Equal(t, wantData.Resources.WriteBytes, at.op.Ext.SorobanData.Resources.WriteBytes)
	wantFee := xdr.Int64(1_000_000)
	assert.Equal(t, wantFee, at.op.Ext.SorobanData.ResourceFee)

	// And op.Auth was replaced by the entry from the simulation result.
	require.Len(t, at.op.Auth, 1)
	assert.Equal(t, xdr.SorobanCredentialsTypeSorobanCredentialsSourceAccount, at.op.Auth[0].Credentials.Type)

	// Built tx was rebuilt: envelope Ext.SorobanData carries the simulated fee.
	env := at.Built.ToXDR()
	require.NotNil(t, env.V1)
	require.NotNil(t, env.V1.Tx.Ext.SorobanData)
	assert.Equal(t, wantFee, env.V1.Tx.Ext.SorobanData.ResourceFee)
}

func TestAssembledTransaction_Simulate_ResourceFeePadOptIn(t *testing.T) {
	_, dataB64 := cannedSorobanData(t)
	rpc := &mockRPCClient{}
	rpc.On("SimulateTransaction", mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{
		TransactionDataXDR: dataB64,
		MinResourceFee:     1_000_000,
	}, nil)
	p := newAssembleParams(t, rpc)
	p.ResourceFeeMultiplier = 2.0
	at, err := NewAssembledTransaction(p)
	require.NoError(t, err)
	require.NoError(t, at.Simulate(context.Background()))

	// Opting into a pad scales the simulated fee; the default path leaves it
	// verbatim (see Simulate_HappyPath).
	require.NotNil(t, at.op.Ext.SorobanData)
	assert.Equal(t, xdr.Int64(2_000_000), at.op.Ext.SorobanData.ResourceFee)
}

func TestAssembledTransaction_Simulate_Idempotent(t *testing.T) {
	_, dataB64 := cannedSorobanData(t)
	rpc := &mockRPCClient{}
	rpc.On("SimulateTransaction", mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{
		TransactionDataXDR: dataB64,
		MinResourceFee:     1_000,
	}, nil)
	at, err := NewAssembledTransaction(newAssembleParams(t, rpc))
	require.NoError(t, err)
	require.NoError(t, at.Simulate(context.Background()))
	require.NoError(t, at.Simulate(context.Background()))
	rpc.AssertNumberOfCalls(t, "SimulateTransaction", 2)
}

// Simulate error paths -------------------------------------------------

func TestAssembledTransaction_Simulate_RPCErrorWrapsSentinel(t *testing.T) {
	rpcErr := errors.New("connection refused")
	rpc := &mockRPCClient{}
	rpc.On("SimulateTransaction", mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{}, rpcErr)
	at, err := NewAssembledTransaction(newAssembleParams(t, rpc))
	require.NoError(t, err)

	err = at.Simulate(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSimulationFailed), "should match ErrSimulationFailed sentinel")
	assert.True(t, errors.Is(err, rpcErr), "should preserve underlying cause")
	assert.Nil(t, at.Simulation)
}

func TestAssembledTransaction_Simulate_ResponseErrorWrapsSentinel(t *testing.T) {
	rpc := &mockRPCClient{}
	rpc.On("SimulateTransaction", mock.Anything, mock.Anything).Return(
		protocol.SimulateTransactionResponse{Error: "host fn trapped"}, nil)
	at, err := NewAssembledTransaction(newAssembleParams(t, rpc))
	require.NoError(t, err)

	err = at.Simulate(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSimulationFailed))
	assert.Contains(t, err.Error(), "host fn trapped")
}

func TestAssembledTransaction_Simulate_RestorePreambleFailsExplicitly(t *testing.T) {
	rpc := &mockRPCClient{}
	rpc.On("SimulateTransaction", mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{
		RestorePreamble: &protocol.RestorePreamble{MinResourceFee: 100},
	}, nil)
	at, err := NewAssembledTransaction(newAssembleParams(t, rpc))
	require.NoError(t, err)

	err = at.Simulate(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSimulationFailed))
	assert.Contains(t, err.Error(), "restore preamble")
	assert.Nil(t, at.Simulation)
	assert.False(t, at.IsReadCall())
}

func TestAssembledTransaction_Simulate_BadTransactionDataB64(t *testing.T) {
	rpc := &mockRPCClient{}
	rpc.On("SimulateTransaction", mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{
		TransactionDataXDR: "!!not-base64!!",
		MinResourceFee:     1,
	}, nil)
	at, err := NewAssembledTransaction(newAssembleParams(t, rpc))
	require.NoError(t, err)

	err = at.Simulate(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSimulationFailed))
	var e *Error
	require.True(t, errors.As(err, &e))
	assert.Contains(t, e.Details, "SorobanTransactionData")
}

func TestAssembledTransaction_Simulate_BadAuthB64(t *testing.T) {
	_, dataB64 := cannedSorobanData(t)
	bad := "!!not-b64!!"
	rpc := &mockRPCClient{}
	rpc.On("SimulateTransaction", mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{
		TransactionDataXDR: dataB64,
		Results: []protocol.SimulateHostFunctionResult{
			{AuthXDR: &[]string{bad}},
		},
	}, nil)
	at, err := NewAssembledTransaction(newAssembleParams(t, rpc))
	require.NoError(t, err)

	err = at.Simulate(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSimulationFailed))
}

func TestAssembledTransaction_Simulate_BadReturnValueB64(t *testing.T) {
	_, dataB64 := cannedSorobanData(t)
	bad := "!!not-b64!!"
	rpc := &mockRPCClient{}
	rpc.On("SimulateTransaction", mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{
		TransactionDataXDR: dataB64,
		Results: []protocol.SimulateHostFunctionResult{
			{ReturnValueXDR: &bad},
		},
	}, nil)
	at, err := NewAssembledTransaction(newAssembleParams(t, rpc))
	require.NoError(t, err)

	err = at.Simulate(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSimulationFailed))
	var e *Error
	require.True(t, errors.As(err, &e))
	assert.Contains(t, e.Details, "simulation result")
}

func TestAssembledTransaction_Simulate_MissingTransactionDataFails(t *testing.T) {
	rpc := &mockRPCClient{}
	rpc.On("SimulateTransaction", mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{
		// No TransactionDataXDR, no Error, no RestorePreamble.
		MinResourceFee: 1_000,
	}, nil)
	at, err := NewAssembledTransaction(newAssembleParams(t, rpc))
	require.NoError(t, err)

	err = at.Simulate(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSimulationFailed))
	var e *Error
	require.True(t, errors.As(err, &e))
	assert.Contains(t, e.Details, "transaction data")
	assert.Nil(t, at.Simulation, "no state should be folded in on failure")
}

func TestCalculateResourceFee(t *testing.T) {
	cases := []struct {
		name       string
		min        int64
		multiplier float64
		want       int64
	}{
		{"multiplier 1 is verbatim", 1_000, 1.0, 1_000},
		{"multiplier below 1 never reduces below the minimum", 1_000, 0.5, 1_000},
		{"fractional multiplier rounds up", 3, 1.1, 4}, // 3 * 1.1 = 3.3 -> 4
		{"whole multiplier is exact", 1_000_000, 2.0, 2_000_000},
		{"overflow clamps to MaxInt64", math.MaxInt64, 2.0, math.MaxInt64},
		{"product of exactly 2^63 clamps to MaxInt64", 1 << 62, 2.0, math.MaxInt64},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, calculateResourceFee(tc.min, tc.multiplier))
		})
	}
}

// Result branches ------------------------------------------------------

func TestResult_NilReceiver(t *testing.T) {
	var at *AssembledTransaction
	_, err := at.Result()
	require.Error(t, err)
	var ce *Error
	require.True(t, errors.As(err, &ce))
	assert.Equal(t, KindInvalidArgs, ce.Kind)
}

func TestResult_BeforeSimulate_NotYetSimulated(t *testing.T) {
	at, err := NewAssembledTransaction(newAssembleParams(t, &mockRPCClient{}))
	require.NoError(t, err)
	_, err = at.Result()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotYetSimulated))
}

// A void-returning read call omits the ScVal entirely; Result decodes it to
// ScvVoid rather than erroring on the nil ReturnValue.
func TestResult_ReadCall_VoidReturn(t *testing.T) {
	_, dataB64 := cannedSorobanData(t) // empty RW footprint → read call
	rpc := &mockRPCClient{}
	rpc.On("SimulateTransaction", mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{
		TransactionDataXDR: dataB64,
		MinResourceFee:     1_000,
		// No Results → ReturnValue stays nil.
	}, nil)
	at, err := NewAssembledTransaction(newAssembleParams(t, rpc))
	require.NoError(t, err)
	require.NoError(t, at.Simulate(context.Background()))
	require.True(t, at.IsReadCall())
	require.Nil(t, at.ReturnValue)

	res, err := at.Result()
	require.NoError(t, err)
	scv, ok := res.(xdr.ScVal)
	require.True(t, ok)
	assert.Equal(t, xdr.ScValTypeScvVoid, scv.Type, "void-returning read call decodes to ScvVoid")
}

// A write call's result lives in on-chain meta, which needs the unsupported
// submit path; Result reports KindNotYetSimulated even though simulation ran.
func TestResult_WriteCall_RequiresSubmission(t *testing.T) {
	_, dataB64 := cannedWriteFootprintData(t) // non-empty RW footprint → write call
	rpc := &mockRPCClient{}
	rpc.On("SimulateTransaction", mock.Anything, mock.Anything).Return(protocol.SimulateTransactionResponse{
		TransactionDataXDR: dataB64,
		MinResourceFee:     1_000,
	}, nil)
	at, err := NewAssembledTransaction(newAssembleParams(t, rpc))
	require.NoError(t, err)
	require.NoError(t, at.Simulate(context.Background()))
	require.False(t, at.IsReadCall(), "non-empty ReadWrite footprint is a write call")

	_, err = at.Result()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotYetSimulated))
	assert.Contains(t, err.Error(), "submission")
}
