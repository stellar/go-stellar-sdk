package ledgerbackend

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/pelletier/go-toml"
)

var (
	//go:embed configs/captive-core-pubnet.cfg
	PubnetDefaultConfig []byte

	//go:embed configs/captive-core-testnet.cfg
	TestnetDefaultConfig []byte

	//go:embed configs/captive-core-futurenet.cfg
	FuturenetDefaultConfig []byte

	defaultBucketListDBPageSize uint = 12
)

const (
	defaultHTTPPort      = 11626
	defaultFailureSafety = -1

	// if LOG_FILE_PATH is omitted stellar core actually defaults to "stellar-core.log"
	// however, we are overriding this default for captive core
	defaultLogFilePath = "" // by default we disable logging to a file
)

var validQuality = map[string]bool{
	"CRITICAL": true,
	"HIGH":     true,
	"MEDIUM":   true,
	"LOW":      true,
}

// Validator represents a [[VALIDATORS]] entry in the captive core toml file.
type Validator struct {
	Name       string `toml:"NAME"`
	Quality    string `toml:"QUALITY,omitempty"`
	HomeDomain string `toml:"HOME_DOMAIN"`
	PublicKey  string `toml:"PUBLIC_KEY"`
	Address    string `toml:"ADDRESS,omitempty"`
	History    string `toml:"HISTORY,omitempty"`
}

// HomeDomain represents a [[HOME_DOMAINS]] entry in the captive core toml file.
type HomeDomain struct {
	HomeDomain string `toml:"HOME_DOMAIN"`
	Quality    string `toml:"QUALITY"`
}

// History represents a [HISTORY] table in the captive core toml file.
type History struct {
	Get string `toml:"get"`
	// should we allow put and mkdir for captive core?
	Put   string `toml:"put,omitempty"`
	Mkdir string `toml:"mkdir,omitempty"`
}

// QuorumSet represents a [QUORUM_SET] table in the captive core toml file.
type QuorumSet struct {
	ThresholdPercent int      `toml:"THRESHOLD_PERCENT"`
	Validators       []string `toml:"VALIDATORS"`
}

type captiveCoreTomlValues struct {
	Database string `toml:"DATABASE,omitempty"`
	// we cannot omitempty because the empty string is a valid configuration for LOG_FILE_PATH
	// and the default is stellar-core.log
	LogFilePath   string `toml:"LOG_FILE_PATH"`
	BucketDirPath string `toml:"BUCKET_DIR_PATH,omitempty"`
	// we cannot omitempty because 0 is a valid configuration for HTTP_PORT
	// and the default is 11626
	HTTPPort          uint     `toml:"HTTP_PORT"`
	PublicHTTPPort    bool     `toml:"PUBLIC_HTTP_PORT,omitempty"`
	NodeNames         []string `toml:"NODE_NAMES,omitempty"`
	NetworkPassphrase string   `toml:"NETWORK_PASSPHRASE,omitempty"`
	PeerPort          uint     `toml:"PEER_PORT,omitempty"`
	// we cannot omitempty because 0 is a valid configuration for FAILURE_SAFETY
	// and the default is -1
	FailureSafety                         int                  `toml:"FAILURE_SAFETY"`
	UnsafeQuorum                          bool                 `toml:"UNSAFE_QUORUM,omitempty"`
	RunStandalone                         bool                 `toml:"RUN_STANDALONE,omitempty"`
	ArtificiallyAccelerateTimeForTesting  bool                 `toml:"ARTIFICIALLY_ACCELERATE_TIME_FOR_TESTING,omitempty"`
	PreferredPeers                        []string             `toml:"PREFERRED_PEERS,omitempty"`
	PreferredPeerKeys                     []string             `toml:"PREFERRED_PEER_KEYS,omitempty"`
	PreferredPeersOnly                    bool                 `toml:"PREFERRED_PEERS_ONLY,omitempty"`
	HomeDomains                           []HomeDomain         `toml:"HOME_DOMAINS,omitempty"`
	Validators                            []Validator          `toml:"VALIDATORS,omitempty"`
	HistoryEntries                        map[string]History   `toml:"-"`
	QuorumSetEntries                      map[string]QuorumSet `toml:"-"`
	BackfillRestoreMeta                   *bool                `toml:"BACKFILL_RESTORE_META,omitempty"`
	BucketListDBPageSizeExp               *uint                `toml:"BUCKETLIST_DB_INDEX_PAGE_SIZE_EXPONENT,omitempty"`
	BucketListDBCutoff                    *uint                `toml:"BUCKETLIST_DB_INDEX_CUTOFF,omitempty"`
	EnableSorobanDiagnosticEvents         *bool                `toml:"ENABLE_SOROBAN_DIAGNOSTIC_EVENTS,omitempty"`
	TestingMinimumPersistentEntryLifetime *uint                `toml:"TESTING_MINIMUM_PERSISTENT_ENTRY_LIFETIME,omitempty"`
	TestingSorobanHighLimitOverride       *bool                `toml:"TESTING_SOROBAN_HIGH_LIMIT_OVERRIDE,omitempty"`
	EnableDiagnosticsForTxSubmission      *bool                `toml:"ENABLE_DIAGNOSTICS_FOR_TX_SUBMISSION,omitempty"`
	EnableEmitSorobanTransactionMetaExtV1 *bool                `toml:"EMIT_SOROBAN_TRANSACTION_META_EXT_V1,omitempty"`
	EnableEmitLedgerCloseMetaExtV1        *bool                `toml:"EMIT_LEDGER_CLOSE_META_EXT_V1,omitempty"`
	HTTPQueryPort                         *uint                `toml:"HTTP_QUERY_PORT,omitempty"`
	QueryThreadPoolSize                   *uint                `toml:"QUERY_THREAD_POOL_SIZE,omitempty"`
	QuerySnapshotLedgers                  *uint                `toml:"QUERY_SNAPSHOT_LEDGERS,omitempty"`
	BucketListDBMemoryForCaching          *uint                `toml:"BUCKETLIST_DB_MEMORY_FOR_CACHING,omitempty"`
	EnableEmitClassicEvents               *bool                `toml:"EMIT_CLASSIC_EVENTS,omitempty"`
	EnableBackfillStellarAssetEvents      *bool                `toml:"BACKFILL_STELLAR_ASSET_EVENTS,omitempty"`
	OverrideEvictionParamsForTesting      *bool                `toml:"OVERRIDE_EVICTION_PARAMS_FOR_TESTING,omitempty"`
	TestingStartingEvictionScanLevel      *uint                `toml:"TESTING_STARTING_EVICTION_SCAN_LEVEL,omitempty"`
	TestingMaxEntriesToArchive            *uint                `toml:"TESTING_MAX_ENTRIES_TO_ARCHIVE,omitempty"`
	CatchupComplete                       bool                 `toml:"CATCHUP_COMPLETE,omitempty"`
}

// QuorumSetIsConfigured returns true if there is a quorum set defined in the configuration.
func (c *captiveCoreTomlValues) QuorumSetIsConfigured() bool {
	return len(c.QuorumSetEntries) > 0 || len(c.Validators) > 0
}

// HistoryIsConfigured returns true if the history archive locations are configured.
func (c *captiveCoreTomlValues) HistoryIsConfigured() bool {
	if len(c.HistoryEntries) > 0 {
		return true
	}
	for _, v := range c.Validators {
		if v.History != "" {
			return true
		}
	}
	return false
}

type placeholders struct {
	labels map[string]string
	count  int
}

func (p *placeholders) newPlaceholder(key string) string {
	if p.labels == nil {
		p.labels = map[string]string{}
	}
	placeHolder := fmt.Sprintf("__placeholder_label_%d__", p.count)
	p.count++
	p.labels[placeHolder] = key
	return placeHolder
}

func (p *placeholders) get(placeholder string) (string, bool) {
	if p.labels == nil {
		return "", false
	}
	val, ok := p.labels[placeholder]
	return val, ok
}

// CaptiveCoreToml represents a parsed captive core configuration.
type CaptiveCoreToml struct {
	captiveCoreTomlValues
	tree              *toml.Tree
	tablePlaceholders *placeholders
}

// flattenTables will transform a given toml text by flattening all nested tables
// whose root can be found in `rootNames`.
//
// In the TOML spec dotted keys represents nesting. So we flatten the table key by replacing each table
// path with a placeholder. For example:
//
// text := `[QUORUM_SET.a.b.c]
//
//	THRESHOLD_PERCENT=67
//	VALIDATORS=["a","b"]`
//
// flattenTables(text, []string{"QUORUM_SET"}) ->
//
// `[__placeholder_label_0__]
// THRESHOLD_PERCENT=67
// VALIDATORS=["a","b"]`
func flattenTables(text string, rootNames []string) (string, *placeholders) {
	orExpression := strings.Join(rootNames, "|")
	re := regexp.MustCompile(`\[(` + orExpression + `)(\..+)?\]`)

	tablePlaceHolders := &placeholders{}

	flattened := re.ReplaceAllStringFunc(text, func(match string) string {
		insideBrackets := match[1 : len(match)-1]
		return "[" + tablePlaceHolders.newPlaceholder(insideBrackets) + "]"
	})
	return flattened, tablePlaceHolders
}

// unflattenTables is the inverse of flattenTables, it restores the
// text back to its original form by replacing all placeholders with their
// original values.
func unflattenTables(text string, tablePlaceHolders *placeholders) string {
	re := regexp.MustCompile(`\[.*\]`)

	return re.ReplaceAllStringFunc(text, func(match string) string {
		insideBrackets := match[1 : len(match)-1]
		original, ok := tablePlaceHolders.get(insideBrackets)
		if !ok {
			return match
		}
		return "[" + original + "]"
	})
}

// AddExamplePubnetQuorum adds example pubnet validators to toml file
func (c *CaptiveCoreToml) AddExamplePubnetValidators() {
	c.captiveCoreTomlValues.Validators = []Validator{
		{
			Name:       "sdf_1",
			HomeDomain: "stellar.org",
			PublicKey:  "GCGB2S2KGYARPVIA37HYZXVRM2YZUEXA6S33ZU5BUDC6THSB62LZSTYH",
			Address:    "core-live-a.stellar.org:11625",
			History:    "curl -sf https://history.stellar.org/prd/core-live/core_live_001/{0} -o {1}",
		},
		{
			Name:       "sdf_2",
			HomeDomain: "stellar.org",
			PublicKey:  "GCM6QMP3DLRPTAZW2UZPCPX2LF3SXWXKPMP3GKFZBDSF3QZGV2G5QSTK",
			Address:    "core-live-b.stellar.org:11625",
			History:    "curl -sf https://history.stellar.org/prd/core-live/core_live_002/{0} -o {1}",
		},
		{
			Name:       "sdf_3",
			HomeDomain: "stellar.org",
			PublicKey:  "GABMKJM6I25XI4K7U6XWMULOUQIQ27BCTMLS6BYYSOWKTBUXVRJSXHYQ",
			Address:    "core-live-c.stellar.org:11625",
			History:    "curl -sf https://history.stellar.org/prd/core-live/core_live_003/{0} -o {1}",
		},
	}
}

// Marshal serializes the CaptiveCoreToml into a toml document.
func (c *CaptiveCoreToml) Marshal() ([]byte, error) {
	var sb strings.Builder
	sb.WriteString("# Generated file, do not edit\n")
	encoder := toml.NewEncoder(&sb)
	if err := encoder.Encode(c.captiveCoreTomlValues); err != nil {
		return nil, errors.Wrap(err, "could not encode toml file")
	}

	if len(c.HistoryEntries) > 0 {
		if err := encoder.Encode(c.HistoryEntries); err != nil {
			return nil, errors.Wrap(err, "could not encode history entries")
		}
	}

	if len(c.QuorumSetEntries) > 0 {
		if err := encoder.Encode(c.QuorumSetEntries); err != nil {
			return nil, errors.Wrap(err, "could not encode quorum set")
		}
	}

	return []byte(unflattenTables(sb.String(), c.tablePlaceholders)), nil
}

func unmarshalTreeNode(t *toml.Tree, key string, dest interface{}) error {
	tree, ok := t.Get(key).(*toml.Tree)
	if !ok {
		return fmt.Errorf("unexpected key %v", key)
	}
	return tree.Unmarshal(dest)
}

func (c *CaptiveCoreToml) unmarshal(data []byte, strict bool) error {
	quorumSetEntries := map[string]QuorumSet{}
	historyEntries := map[string]History{}
	// The toml library has trouble with nested tables so we need to flatten all nested
	// QUORUM_SET and HISTORY tables as a workaround.
	// In Marshal() we apply the inverse process to unflatten the nested tables.
	flattened, tablePlaceholders := flattenTables(string(data), []string{"QUORUM_SET", "HISTORY"})

	tree, err := toml.Load(flattened)
	if err != nil {
		return err
	}

	for _, key := range tree.Keys() {
		originalKey, ok := tablePlaceholders.get(key)
		if !ok {
			continue
		}

		switch {
		case strings.HasPrefix(originalKey, "QUORUM_SET"):
			var qs QuorumSet
			if err = unmarshalTreeNode(tree, key, &qs); err != nil {
				return err
			}
			quorumSetEntries[key] = qs
		case strings.HasPrefix(originalKey, "HISTORY"):
			var h History
			if err = unmarshalTreeNode(tree, key, &h); err != nil {
				return err
			}
			historyEntries[key] = h
		}
		if err = tree.Delete(key); err != nil {
			return err
		}
	}

	var body captiveCoreTomlValues
	if withoutPlaceHolders, err := tree.Marshal(); err != nil {
		return err
	} else if err = toml.NewDecoder(bytes.NewReader(withoutPlaceHolders)).Strict(strict).Decode(&body); err != nil {
		if message := err.Error(); strings.HasPrefix(message, "undecoded keys") {
			return errors.New(strings.Replace(
				message,
				"undecoded keys",
				"these fields are not supported by captive core",
				1,
			))
		}
		return err
	}

	c.tree = tree
	c.captiveCoreTomlValues = body
	c.tablePlaceholders = tablePlaceholders
	c.QuorumSetEntries = quorumSetEntries
	c.HistoryEntries = historyEntries
	return nil
}

type HTTPQueryServerParams struct {
	Port            uint16
	ThreadPoolSize  uint16
	SnapshotLedgers uint16
}

// CaptiveCoreTomlParams defines captive core configuration provided by Horizon flags.
type CaptiveCoreTomlParams struct {
	// NetworkPassphrase is the Stellar network passphrase used by captive core when connecting to the Stellar network.
	NetworkPassphrase string
	// HistoryArchiveURLs are a list of history archive urls.
	HistoryArchiveURLs []string
	// HTTPPort is the TCP port to listen for requests (defaults to 0, which disables the HTTP server).
	HTTPPort *uint
	// PeerPort is the TCP port to bind to for connecting to the Stellar network
	// (defaults to 11625). It may be useful for example when there's >1 Stellar-Core
	// instance running on a machine.
	PeerPort *uint
	// LogPath is the (optional) path in which to store Core logs, passed along
	// to Stellar Core's LOG_FILE_PATH.
	LogPath *string
	// Strict is a flag which, if enabled, rejects Stellar Core toml fields which are not supported by captive core.
	Strict bool
	// the path to the core binary, used to introspect core at runtime, determine some toml capabilities
	CoreBinaryPath string
	// Enforce EnableSorobanDiagnosticEvents and EnableDiagnosticsForTxSubmission when not disabled explicitly
	EnforceSorobanDiagnosticEvents bool
	// Enforce EnableSorobanTransactionMetaExtV1 when not disabled explicitly
	EnforceSorobanTransactionMetaExtV1 bool
	// Emits unified events for all operations
	EmitUnifiedEvents bool
	// Enable backfilled events
	EmitUnifiedEventsBeforeProtocol22 bool
	// Fast HTTP Query Server parameters
	HTTPQueryServerParams *HTTPQueryServerParams
	// CoreBuildVersionFn is a function that returns the build version of the stellar-core binary.
	CoreBuildVersionFn CoreBuildVersionFunc
	// CoreProtocolVersionFn is a function that returns the protocol version of the stellar-core binary.
	CoreProtocolVersionFn CoreProtocolVersionFunc
	// BackfillRestoreMeta is the value assigned to BACKFILL_RESTORE_META in the captive core toml file.
	// If omitted we will default BACKFILL_RESTORE_META to true.
	BackfillRestoreMeta *bool
	// This will enable a series of core configurations to generate the most verbose meta possible.
	EmitVerboseMeta bool
}

// NewCaptiveCoreTomlFromFile constructs a new CaptiveCoreToml instance by merging configuration
// from the toml file located at `configPath` and the configuration provided by `params`.
func NewCaptiveCoreTomlFromFile(configPath string, params CaptiveCoreTomlParams) (*CaptiveCoreToml, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, errors.Wrap(err, "could not load toml path")
	}
	return NewCaptiveCoreTomlFromData(data, params)
}

// NewCaptiveCoreTomlFromData constructs a new CaptiveCoreToml instance by merging configuration
// from the toml data  and the configuration provided by `params`.
func NewCaptiveCoreTomlFromData(data []byte, params CaptiveCoreTomlParams) (*CaptiveCoreToml, error) {
	var captiveCoreToml CaptiveCoreToml

	if err := captiveCoreToml.unmarshal(data, params.Strict); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal captive core toml")
	}
	// disallow setting BUCKET_DIR_PATH through a file since it can cause multiple
	// running captive-core instances to clash
	if params.Strict && captiveCoreToml.BucketDirPath != "" {
		return nil, errors.New("could not unmarshal captive core toml: setting BUCKET_DIR_PATH is disallowed for Captive Core, use CAPTIVE_CORE_STORAGE_PATH instead")
	}

	if err := captiveCoreToml.validate(params); err != nil {
		return nil, errors.Wrap(err, "invalid captive core toml")
	}

	if len(captiveCoreToml.HistoryEntries) > 0 {
		log.Warnf(
			"Configuring captive core with history archive from %s",
			params.HistoryArchiveURLs,
		)
	}

	captiveCoreToml.setDefaults(params)
	return &captiveCoreToml, nil
}

// NewCaptiveCoreToml constructs a new CaptiveCoreToml instance based off
// the configuration in `params`.
func NewCaptiveCoreToml(params CaptiveCoreTomlParams) (*CaptiveCoreToml, error) {
	var captiveCoreToml CaptiveCoreToml
	var err error

	captiveCoreToml.tablePlaceholders = &placeholders{}
	captiveCoreToml.tree, err = toml.TreeFromMap(map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	captiveCoreToml.setDefaults(params)
	return &captiveCoreToml, nil
}

func (c *CaptiveCoreToml) clone() (*CaptiveCoreToml, error) {
	data, err := c.Marshal()
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal toml")
	}
	var cloned CaptiveCoreToml
	if err = cloned.unmarshal(data, false); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal captive core toml")
	}
	return &cloned, nil
}

// CatchupToml returns a new CaptiveCoreToml instance based off the existing
// instance with some modifications which are suitable for running
// the catchup command on captive core.
func (c *CaptiveCoreToml) CatchupToml() (*CaptiveCoreToml, error) {
	offline, err := c.clone()
	if err != nil {
		return nil, errors.Wrap(err, "could not clone toml")
	}

	offline.RunStandalone = true
	offline.UnsafeQuorum = true
	offline.PublicHTTPPort = false
	offline.HTTPPort = 0
	offline.FailureSafety = 0

	if !c.QuorumSetIsConfigured() {
		// Add a fictional quorum -- necessary to convince core to start up;
		// but not used at all for our purposes. Pubkey here is just random.
		offline.QuorumSetEntries = map[string]QuorumSet{
			"QUORUM_SET": {
				ThresholdPercent: 100,
				Validators:       []string{"GCZBOIAY4HLKAJVNJORXZOZRAY2BJDBZHKPBHZCRAIUR5IHC2UHBGCQR"},
			},
		}
	}
	return offline, nil
}

// coreVersion helper struct identify a core version and provides the
// utilities to compare the version ( i.e. minor + major pair ) to a predefined
// version.
type coreVersion struct {
	major int
	minor int
}

// greaterThanOrEqual compares the core version to a version specific. If unable
// to make the decision, the result is always "false", leaning toward the
// common denominator.
func (c coreVersion) greaterThanOrEqual(other coreVersion) bool {
	if c.major == 0 && c.minor == 0 {
		return false
	}
	return (c.major == other.major && c.minor >= other.minor) || (c.major > other.major)
}

func (c *CaptiveCoreToml) checkCoreVersion(params CaptiveCoreTomlParams) coreVersion {
	if params.CoreBinaryPath == "" {
		return coreVersion{}
	}

	getCoreVersion := params.CoreBuildVersionFn
	if getCoreVersion == nil {
		getCoreVersion = CoreBuildVersion
	}

	versionRaw, err := getCoreVersion(params.CoreBinaryPath)
	if err != nil {
		return coreVersion{}
	}

	var version [2]int

	re := regexp.MustCompile(`\D*(\d*)\.(\d*).*`)
	versionStr := re.FindStringSubmatch(versionRaw)
	if err == nil && len(versionStr) == 3 {
		for i := 1; i < len(versionStr); i++ {
			val, err := strconv.Atoi((versionStr[i]))
			if err != nil {
				break
			}
			version[i-1] = val
		}
	}

	return coreVersion{
		major: version[0],
		minor: version[1],
	}
}

func getCoreProtocolVersion(params CaptiveCoreTomlParams) (uint, error) {
	if params.CoreProtocolVersionFn != nil {
		return params.CoreProtocolVersionFn(params.CoreBinaryPath)
	}
	return CoreProtocolVersion(params.CoreBinaryPath)
}

var (
	minVersionForBucketlistCaching                = coreVersion{major: 22, minor: 2}
	minProtocolVersionForBackfillRestoreMeta uint = 23
	minProtocolVersionForUnifiedEvents       uint = 23
)

func (c *CaptiveCoreToml) setDefaults(params CaptiveCoreTomlParams) {
	if !c.tree.Has("DATABASE") {
		c.Database = "sqlite3://stellar.db"
	}

	if !c.tree.Has("BACKFILL_RESTORE_META") {
		if params.BackfillRestoreMeta != nil {
			c.BackfillRestoreMeta = params.BackfillRestoreMeta
		} else {
			protocolVersion, err := getCoreProtocolVersion(params)
			if err != nil {
				log.Warnf("Error getting core protocol version: %v", err)
			} else if protocolVersion >= minProtocolVersionForBackfillRestoreMeta {
				defaultVal := true
				c.BackfillRestoreMeta = &defaultVal
			}
		}
	}

	if !c.tree.Has("BUCKETLIST_DB_INDEX_PAGE_SIZE_EXPONENT") {
		c.BucketListDBPageSizeExp = &defaultBucketListDBPageSize
	}

	if !c.tree.Has("NETWORK_PASSPHRASE") {
		c.NetworkPassphrase = params.NetworkPassphrase
	}

	if def := c.tree.Has("HTTP_PORT"); !def && params.HTTPPort != nil {
		c.HTTPPort = *params.HTTPPort
	} else if !def && params.HTTPPort == nil {
		c.HTTPPort = defaultHTTPPort
	}

	if def := c.tree.Has("PEER_PORT"); !def && params.PeerPort != nil {
		c.PeerPort = *params.PeerPort
	}

	if def := c.tree.Has("LOG_FILE_PATH"); !def && params.LogPath != nil {
		c.LogFilePath = *params.LogPath
	} else if !def && params.LogPath == nil {
		c.LogFilePath = defaultLogFilePath
	}

	if !c.tree.Has("FAILURE_SAFETY") {
		c.FailureSafety = defaultFailureSafety
	}
	if !c.HistoryIsConfigured() {
		c.HistoryEntries = map[string]History{}
		for i, val := range params.HistoryArchiveURLs {
			name := fmt.Sprintf("HISTORY.h%d", i)
			c.HistoryEntries[c.tablePlaceholders.newPlaceholder(name)] = History{
				Get: fmt.Sprintf("curl -sf %s/{0} -o {1}", strings.TrimSuffix(val, "/")),
			}
		}
	}

	if params.EmitVerboseMeta || params.EnforceSorobanDiagnosticEvents {
		enforceOption(&c.EnableSorobanDiagnosticEvents)
		enforceOption(&c.EnableDiagnosticsForTxSubmission)
	}
	if params.EmitVerboseMeta || params.EnforceSorobanTransactionMetaExtV1 {
		enforceOption(&c.EnableEmitSorobanTransactionMetaExtV1)
	}
	if params.EmitVerboseMeta {
		enforceOption(&c.EnableEmitLedgerCloseMetaExtV1)
	}

	// enable event emission
	// as long as the running Core binary supports the feature.
	if params.EmitVerboseMeta || params.EmitUnifiedEvents {
		protocolVersion, err := getCoreProtocolVersion(params)
		if err != nil {
			log.Warnf("Error getting core protocol version: %v", err)
		}
		if protocolVersion < minProtocolVersionForUnifiedEvents {
			log.Warnf(
				"Core supported protocol version too low: %d < %d, "+
					"unified events flag has no effect.",
				protocolVersion, minProtocolVersionForUnifiedEvents)
		} else {
			enforceOption(&c.EnableEmitClassicEvents)
		}
	}

	// enable classic events to be generated prior to p22
	// as long as the running Core binary supports the feature.
	if params.EmitVerboseMeta || params.EmitUnifiedEventsBeforeProtocol22 {
		protocolVersion, err := getCoreProtocolVersion(params)
		if err != nil {
			log.Warnf("Error getting core protocol version: %v", err)
		}
		if protocolVersion < minProtocolVersionForUnifiedEvents {
			log.Warnf(
				"Core supported protocol version too low: %d < %d, "+
					"flag for backfilled classic events before p22 has no effect.",
				protocolVersion, minProtocolVersionForUnifiedEvents)
		} else {
			enforceOption(&c.EnableBackfillStellarAssetEvents)
		}
	}

	if params.HTTPQueryServerParams != nil {
		port := uint(params.HTTPQueryServerParams.Port)
		c.HTTPQueryPort = &port

		ledgers := uint(params.HTTPQueryServerParams.SnapshotLedgers)
		c.QuerySnapshotLedgers = &ledgers

		poolSize := uint(params.HTTPQueryServerParams.ThreadPoolSize)
		c.QueryThreadPoolSize = &poolSize
	}

	if !c.tree.Has("BUCKETLIST_DB_MEMORY_FOR_CACHING") &&
		c.checkCoreVersion(params).greaterThanOrEqual(minVersionForBucketlistCaching) {
		// set BUCKETLIST_DB_MEMORY_FOR_CACHING to 0 to disable allocation of
		// memory for caching entries in BucketListDB.
		// If we do not set BUCKETLIST_DB_MEMORY_FOR_CACHING, core will apply
		// the default value which may result in additional 2-3 GB of memory usage
		// in captive core
		var disable uint = 0
		c.BucketListDBMemoryForCaching = &disable
	}
}

func enforceOption(opt **bool) {
	if *opt == nil {
		// We are generating the file from scratch or the user didn't explicitly oppose the option.
		// Enforce it.
		t := true
		*opt = &t
	}
	if !**opt {
		// The user opposed the option, but there is no need to pass it on
		*opt = nil
	}
}

func (c *CaptiveCoreToml) validate(params CaptiveCoreTomlParams) error {
	if def := c.tree.Has("NETWORK_PASSPHRASE"); def && c.NetworkPassphrase != params.NetworkPassphrase {
		return fmt.Errorf(
			"NETWORK_PASSPHRASE in captive core config file: %s does not match passed configuration (%s)",
			c.NetworkPassphrase,
			params.NetworkPassphrase,
		)
	}

	if def := c.tree.Has("HTTP_PORT"); def && params.HTTPPort != nil && c.HTTPPort != *params.HTTPPort {
		return fmt.Errorf(
			"HTTP_PORT in captive core config file: %d does not match passed configuration (%d)",
			c.HTTPPort,
			*params.HTTPPort,
		)
	}

	if def := c.tree.Has("PEER_PORT"); def && params.PeerPort != nil && c.PeerPort != *params.PeerPort {
		return fmt.Errorf(
			"PEER_PORT in captive core config file: %d does not match passed configuration (%d)",
			c.PeerPort,
			*params.PeerPort,
		)
	}

	if def := c.tree.Has("LOG_FILE_PATH"); def && params.LogPath != nil && c.LogFilePath != *params.LogPath {
		return fmt.Errorf(
			"LOG_FILE_PATH in captive core config file: %s does not match passed configuration (%s)",
			c.LogFilePath,
			*params.LogPath,
		)
	}

	if def := c.tree.Has("BACKFILL_RESTORE_META"); def && params.BackfillRestoreMeta != nil && *c.BackfillRestoreMeta != *params.BackfillRestoreMeta {
		return fmt.Errorf(
			"BACKFILL_RESTORE_META in captive core config file: %v does not match passed configuration (%v)",
			*c.BackfillRestoreMeta,
			*params.BackfillRestoreMeta,
		)
	}

	if params.HTTPQueryServerParams != nil {
		if c.HTTPQueryPort != nil && *c.HTTPQueryPort != uint(params.HTTPQueryServerParams.Port) {
			return fmt.Errorf(
				"HTTP_QUERY_PORT in captive core config file: %d does not match passed configuration (%d)",
				c.PeerPort,
				*params.PeerPort,
			)
		}

		if c.QueryThreadPoolSize != nil && *c.QueryThreadPoolSize != uint(params.HTTPQueryServerParams.ThreadPoolSize) {
			return fmt.Errorf(
				"QUERY_THREADPOOL_SIZE in captive core config file: %d does not match passed configuration (%d)",
				c.PeerPort,
				*params.PeerPort,
			)
		}

		if c.QuerySnapshotLedgers != nil && *c.QuerySnapshotLedgers != uint(params.HTTPQueryServerParams.SnapshotLedgers) {
			return fmt.Errorf(
				"QUERY_SNAPSHOT_LEDGERS in captive core config file: %d does not match passed configuration (%d)",
				c.PeerPort,
				*params.PeerPort,
			)
		}
	}

	homeDomainSet := map[string]HomeDomain{}
	for _, hd := range c.HomeDomains {
		if _, ok := homeDomainSet[hd.HomeDomain]; ok {
			return fmt.Errorf(
				"found duplicate home domain in captive core configuration: %s",
				hd.HomeDomain,
			)
		}
		if hd.HomeDomain == "" {
			return fmt.Errorf(
				"found invalid home domain entry which is missing a HOME_DOMAIN value",
			)
		}
		if hd.Quality == "" {
			return fmt.Errorf(
				"found invalid home domain entry which is missing a QUALITY value: %s",
				hd.HomeDomain,
			)
		}
		if !validQuality[hd.Quality] {
			return fmt.Errorf(
				"found invalid home domain entry which has an invalid QUALITY value: %s",
				hd.HomeDomain,
			)
		}
		homeDomainSet[hd.HomeDomain] = hd
	}

	names := map[string]bool{}
	for _, v := range c.Validators {
		if names[v.Name] {
			return fmt.Errorf(
				"found duplicate validator in captive core configuration: %s",
				v.Name,
			)
		}
		if v.Name == "" {
			return fmt.Errorf(
				"found invalid validator entry which is missing a NAME value: %s",
				v.Name,
			)
		}
		if v.HomeDomain == "" {
			return fmt.Errorf(
				"found invalid validator entry which is missing a HOME_DOMAIN value: %s",
				v.Name,
			)
		}
		if v.PublicKey == "" {
			return fmt.Errorf(
				"found invalid validator entry which is missing a PUBLIC_KEY value: %s",
				v.Name,
			)
		}
		if _, err := xdr.AddressToAccountId(v.PublicKey); err != nil {
			return fmt.Errorf(
				"found invalid validator entry which has an invalid PUBLIC_KEY : %s",
				v.Name,
			)
		}
		if v.Quality == "" {
			if _, ok := homeDomainSet[v.HomeDomain]; !ok {
				return fmt.Errorf(
					"found invalid validator entry which is missing a QUALITY value: %s",
					v.Name,
				)
			}
		} else if !validQuality[v.Quality] {
			return fmt.Errorf(
				"found invalid validator entry which has an invalid QUALITY value: %s",
				v.Name,
			)
		}

		names[v.Name] = true
	}

	if len(c.Database) > 0 && !strings.HasPrefix(c.Database, "sqlite3://") {
		return fmt.Errorf("invalid DATABASE parameter: %s, for captive core config, must be valid sqlite3 db url", c.Database)
	}

	return nil
}
