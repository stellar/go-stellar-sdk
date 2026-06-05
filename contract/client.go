package contract

import (
	"context"
	"time"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Client invokes functions on a single Soroban contract.
type Client struct {
	// contractID is the strkey contract id (`C...`) this client targets.
	contractID string
	// rpc is the Stellar RPC transport used to simulate and submit
	// transactions. *rpcclient.Client satisfies it.
	rpc RPCClient
	// network is the network passphrase transactions are signed against.
	network string

	// contractAddr is contractID decoded once at construction.
	contractAddr xdr.ScAddress

	// sourceAddr is the optional default source account set by WithDefaultSource.
	// When unset, Invoke simulates against a synthetic null account so read-only
	// calls need no real, funded source.
	sourceAddr string
	// baseFee is the per-operation base fee in stroops.
	baseFee int64
}

// clientConfig accumulates ClientOption mutations before construction.
type clientConfig struct {
	sourceAddr string
	baseFee    int64
}

// ClientOption is the functional-option type accepted by New.
type ClientOption func(*clientConfig)

// WithDefaultSource sets the default source account from an ed25519 account
// strkey. A per-call WithSource overrides it.
func WithDefaultSource(addr string) ClientOption {
	return func(c *clientConfig) { c.sourceAddr = addr }
}

// WithBaseFee sets the per-operation base fee in stroops. Defaults to txnbuild.MinBaseFee
// when omitted.
func WithBaseFee(stroops int64) ClientOption {
	return func(c *clientConfig) { c.baseFee = stroops }
}

// New constructs a Client targeting contractID.
func New(contractID string, rpc RPCClient, network string, opts ...ClientOption) (*Client, error) {
	cfg := &clientConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	if rpc == nil {
		return nil, invalidArgsf("New: RPC is required")
	}
	if network == "" {
		return nil, invalidArgsf("New: network passphrase is required")
	}
	addr, err := xdr.ScAddressFromStrkey(contractID)
	if err != nil {
		return nil, invalidArgsf("New: contract id %q: %v", contractID, err)
	}

	if cfg.sourceAddr != "" {
		if err := validateSourceAddr("WithDefaultSource", cfg.sourceAddr); err != nil {
			return nil, err
		}
	}

	baseFee := cfg.baseFee
	if baseFee == 0 {
		baseFee = txnbuild.MinBaseFee
	} else if baseFee < txnbuild.MinBaseFee {
		return nil, invalidArgsf("New: base fee %d below MinBaseFee %d", baseFee, txnbuild.MinBaseFee)
	}

	return &Client{
		contractID:   contractID,
		rpc:          rpc,
		network:      network,
		contractAddr: addr,
		sourceAddr:   cfg.sourceAddr,
		baseFee:      baseFee,
	}, nil
}

// ContractID returns the contract id this client targets,
// fixed at construction.
func (c *Client) ContractID() string {
	return c.contractID
}

// Invoke builds an AssembledTransaction for a call to method on the client's
// contract, simulates it, and returns it ready to sign and send.
//
// args is the positional argument list passed to the contract function; nil
// (or an empty slice) means no arguments. Marshaling native Go values into
// arguments will be supported once a contract Spec can be bound; until then,
// callers build the ScVals directly (e.g. with the xdr.Scv* helpers).
//
// A source account is optional: WithDefaultSource (or per-call WithSource) sets one;
// without it, Invoke simulates against a synthetic null account, which is all
// a read-only call needs. Per-call InvokeOption values override client
// defaults.
func (c *Client) Invoke(
	ctx context.Context,
	method string,
	args []xdr.ScVal,
	opts ...InvokeOption,
) (*AssembledTransaction, error) {
	if c == nil || c.rpc == nil {
		return nil, invalidArgsf("Invoke: client not initialized")
	}
	if method == "" {
		return nil, invalidArgsf("Invoke: method is required")
	}
	methodSym, err := xdr.ScvSymbol(method)
	if err != nil {
		return nil, invalidArgsf("Invoke: method %q: %v", method, err)
	}

	icfg := invokeConfig{
		baseFee:               c.baseFee,
		resourceFeeMultiplier: 0, // 0 = let AssembleParams pick DefaultResourceFeeMultiplier
		sourceAddr:            c.sourceAddr,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&icfg)
		}
	}

	source, err := c.resolveSource(ctx, &icfg)
	if err != nil {
		return nil, err
	}

	op := &txnbuild.InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: c.contractAddr,
				FunctionName:    *methodSym.Sym,
				Args:            args,
			},
		},
		SourceAccount: source.GetAccountID(),
	}

	preconditions := txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()}
	if icfg.hasTimeBounds {
		preconditions.TimeBounds = txnbuild.NewTimebounds(icfg.tbMin, icfg.tbMax)
	}

	at, err := NewAssembledTransaction(AssembleParams{
		RPC:                   c.rpc,
		NetworkPassphrase:     c.network,
		BaseFee:               icfg.baseFee,
		SourceAccount:         source,
		Op:                    op,
		Memo:                  icfg.memo,
		Preconditions:         preconditions,
		ResourceFeeMultiplier: icfg.resourceFeeMultiplier,
	})
	if err != nil {
		return nil, err
	}

	if err := at.Simulate(ctx); err != nil {
		return nil, err
	}
	return at, nil
}

// nullAccount is the canonical all-zero ed25519 account ("the impossible
// account"). resolveSource uses it as a synthetic source for read-only
// simulations that have no real source.
const nullAccount = "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF"

// resolveSource returns Invoke's source account. Without a configured source,
// it uses the synthetic null account for read-only simulations; otherwise it
// validates and loads the live account. LoadAccount returns the current
// sequence number, so this bumps it once for the transaction.
func (c *Client) resolveSource(ctx context.Context, icfg *invokeConfig) (txnbuild.Account, error) {
	if icfg.sourceAddr == "" {
		acct := txnbuild.NewSimpleAccount(nullAccount, 0)
		return &acct, nil
	}
	if err := validateSourceAddr("WithSource", icfg.sourceAddr); err != nil {
		return nil, err
	}
	acct, err := c.rpc.LoadAccount(ctx, icfg.sourceAddr)
	if err != nil {
		return nil, &Error{Kind: KindSourceAccountFailed, Details: "Invoke: load source account", cause: err}
	}
	if _, err := acct.IncrementSequenceNumber(); err != nil {
		return nil, &Error{Kind: KindSourceAccountFailed, Details: "Invoke: increment source sequence", cause: err}
	}
	return acct, nil
}

// InvokeOption is a per-call option for Invoke. Each overrides the client's
// default for a single invocation, winning over ClientOption defaults.
type InvokeOption func(*invokeConfig)

// invokeConfig is the receiver for InvokeOption mutations.
type invokeConfig struct {
	baseFee               int64
	resourceFeeMultiplier float64
	memo                  txnbuild.Memo
	tbMin                 int64
	tbMax                 int64
	hasTimeBounds         bool
	// sourceAddr mirrors the Client default; a per-call WithSource overrides it.
	sourceAddr string
}

// WithResourceFeeMultiplier sets the per-call multiplier applied to the simulated
// resource fee, letting a caller pad it for headroom against simulate-to-submit
// drift. Defaults to DefaultResourceFeeMultiplier (1.0).
func WithResourceFeeMultiplier(f float64) InvokeOption {
	return func(c *invokeConfig) {
		if f > 0 {
			c.resourceFeeMultiplier = f
		}
	}
}

// WithMemo attaches a txnbuild.Memo to the transaction Invoke builds.
func WithMemo(m txnbuild.Memo) InvokeOption {
	return func(c *invokeConfig) { c.memo = m }
}

// WithTimeBounds sets the transaction's time bounds. Pass the zero value of
// time.Time for min or max to leave that bound open. The default is the
// txnbuild infinite timeout.
func WithTimeBounds(min, max time.Time) InvokeOption {
	return func(c *invokeConfig) {
		c.hasTimeBounds = true
		if min.IsZero() {
			c.tbMin = 0
		} else {
			c.tbMin = min.Unix()
		}
		if max.IsZero() {
			c.tbMax = 0
		} else {
			c.tbMax = max.Unix()
		}
	}
}

// WithSource overrides the default source account for a single invocation. It
// must be an ed25519 account strkey, loaded via LoadAccount at Invoke time; for
// a muxed or custom-sequenced source, build the transaction with txnbuild directly.
func WithSource(addr string) InvokeOption {
	return func(c *invokeConfig) { c.sourceAddr = addr }
}

// validateSourceAddr reports an *Error (KindInvalidArgs) unless addr is a
// valid ed25519 account strkey.
func validateSourceAddr(who, addr string) error {
	if strkey.IsValidEd25519PublicKey(addr) {
		return nil
	}
	return invalidArgsf("%s: %q is not a valid ed25519 account (G...) strkey", who, addr)
}
