package loadtest

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/klauspost/compress/zstd"
	"golang.org/x/mod/semver"

	"github.com/stellar/go-stellar-sdk/historyarchive"
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type Options struct {
	// inputs
	ConfigPath     string
	OutputPath     string
	FixturesPath   string
	CoreBinaryPath string     // optional, will be looked up in PATH if not set
	WorkDirPath    string     // optional
	Logger         *log.Entry // optional
}

type Results struct {
	PreBenchmarkCheckpoint uint32
	CountLedgers           int
	CountFixtures          int
}

// ApplyLoad runs stellar-core's apply-load against the supplied config and writes
// benchmark ledgers + fixture entries to OutputPath / FixturesPath.
//
// Required: ConfigPath, OutputPath, FixturesPath. Optional: CoreBinaryPath
// (looked up in PATH), Logger, WorkDirPath (temp dir if unspecified).
// The supplied config's [HISTORY] commands must publish to a `history/`
// subdirectory of the work dir.
func ApplyLoad(ctx context.Context, opts Options) (Results, error) {
	var results Results
	createdWorkDir := (opts.WorkDirPath == "")

	if err := resolveOptions(&opts); err != nil {
		return Results{}, fmt.Errorf("invalid options: %w", err)
	}
	if createdWorkDir {
		defer func() {
			if err := os.RemoveAll(opts.WorkDirPath); err != nil {
				opts.Logger.Warnf("failed to cleanup temporary work directory: %v", err)
			}
		}()
	}
	cfg, err := parseConfig(opts.ConfigPath)
	if err != nil {
		return Results{}, fmt.Errorf("failed to parse config: %w", err)
	}

	if results.PreBenchmarkCheckpoint, err = run(ctx, opts, cfg); err != nil {
		return Results{}, fmt.Errorf("failed to run stellar-core commands: %w", err)
	}

	// Verify fixtures completeness before writing anything
	if err = verifyFixturesCompleteness(ctx, cfg, opts, results.PreBenchmarkCheckpoint); err != nil {
		return Results{}, fmt.Errorf("fixture completeness verification failed: %w", err)
	}

	// Stream benchmark ledgers (after the pre-benchmark checkpoint) to output file.
	// Setup ledgers are excluded because they would conflict with the fixtures.
	if results.CountLedgers, err = streamLedgersToFile(ctx, cfg, opts, results.PreBenchmarkCheckpoint); err != nil {
		return Results{}, fmt.Errorf("failed to stream ledgers to file: %w", err)
	} else if results.CountLedgers == 0 {
		return Results{}, fmt.Errorf("no benchmark ledgers found to write to file")
	}

	if results.CountFixtures, err = streamFixturesToFile(ctx, cfg, opts, results.PreBenchmarkCheckpoint); err != nil {
		return Results{}, fmt.Errorf("failed to stream fixtures to file: %w", err)
	} else if results.CountFixtures == 0 {
		return Results{}, fmt.Errorf("no benchmark fixtures found to write to file")
	}

	return results, nil
}

// resolveOptions checks that required options are set and valid, and fills in defaults for optional ones.
func resolveOptions(opts *Options) error {
	if opts.ConfigPath == "" {
		return fmt.Errorf("configPath is required")
	}
	if opts.OutputPath == "" || opts.FixturesPath == "" {
		return fmt.Errorf("both outputPath and fixturesPath are required")
	}

	if opts.CoreBinaryPath == "" {
		var err error
		opts.CoreBinaryPath, err = exec.LookPath("stellar-core")
		if err != nil {
			return err
		}
	}
	coreVersion, err := ledgerbackend.CoreBuildVersion(opts.CoreBinaryPath)
	if err != nil {
		return err
	}
	if semver.Compare(semver.Major(coreVersion), "v22") < 0 {
		return fmt.Errorf("stellar-core %s does not support apply-load, need v22 or higher", coreVersion)
	}

	if opts.Logger == nil {
		opts.Logger = log.New()
	}

	opts.Logger.Infof("Using stellar-core: %s %s", opts.CoreBinaryPath, coreVersion)
	opts.Logger.Infof("Using config: %s", opts.ConfigPath)

	if opts.WorkDirPath == "" {
		var err error
		opts.WorkDirPath, err = os.MkdirTemp("", "apply-load-workdir-*")
		if err != nil {
			return err
		}
	}
	return nil
}

// run executes the stellar-core apply-load command and captures the pre-benchmark checkpoint from its output.
func run(ctx context.Context, opts Options, cfg applyLoadConfig) (uint32, error) {
	// Copy config to work dir (apply-load writes files relative to config location)
	destConfigPath := filepath.Join(opts.WorkDirPath, "apply-load.cfg")
	if err := copyFile(opts.ConfigPath, destConfigPath); err != nil {
		return 0, err
	}

	if _, err := runCore(ctx, opts, destConfigPath, "new-db"); err != nil {
		return 0, err
	}
	if _, err := runCore(ctx, opts, destConfigPath, "new-hist", cfg.HistoryArchiveName); err != nil {
		return 0, err
	}
	opts.Logger.Infof("Initialized history archive: %s", cfg.HistoryArchiveName)

	output, err := runCore(ctx, opts, destConfigPath, "apply-load")
	if err != nil {
		return 0, err
	}

	// Parse pre-benchmark checkpoint from stellar-core's stdout.
	// The config sets LOG_FILE_PATH="" so logs go to stdout.
	preBenchmarkCheckpoint, err := parsePreBenchmarkCheckpoint(string(output))
	if err != nil {
		return 0, err
	}
	opts.Logger.Infof("Pre-benchmark checkpoint: ledger %d", preBenchmarkCheckpoint)

	return preBenchmarkCheckpoint, nil
}

// runCore invokes a stellar-core subcommand against the copy of the config in the work dir.
func runCore(ctx context.Context, opts Options, configPath string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, opts.CoreBinaryPath, append(args, "--conf", configPath)...)
	cmd.Dir = opts.WorkDirPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("stellar-core %s failed:\n%s\n%w", strings.Join(args, " "), out, err)
	}
	return out, nil
}

func streamLedgersToFile(
	ctx context.Context,
	cfg applyLoadConfig,
	opts Options,
	preBenchmarkCheckpoint uint32,
) (int, error) {
	metadataPath, err := resolveMetadataPath(opts.WorkDirPath, cfg.MetadataOutputStream)
	if err != nil {
		return 0, err
	}
	inFile, err := os.Open(metadataPath)
	if err != nil {
		return 0, err
	}

	// Note: xdr.Stream closes the underlying reader when ReadOne hits EOF or error
	stream := xdr.NewStream(inFile)
	defer stream.Close()

	outFile, err := os.Create(opts.OutputPath)
	if err != nil {
		return 0, err
	}
	defer outFile.Close()

	writer, err := zstd.NewWriter(outFile)
	if err != nil {
		return 0, err
	}
	defer writer.Close()

	count := 0
	skipped := 0

	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		var ledger xdr.LedgerCloseMeta
		if err := stream.ReadOne(&ledger); err == io.EOF {
			break
		} else if err != nil {
			return 0, err
		}

		// Skip setup ledgers (ledgers up to and including the pre-benchmark checkpoint).
		// Only include benchmark ledgers which operate on the fixture state.
		if ledger.LedgerSequence() <= preBenchmarkCheckpoint {
			skipped++
			continue
		}

		if err := xdr.MarshalFramed(writer, ledger); err != nil {
			return 0, err
		}
		count++
	}

	opts.Logger.Infof("Wrote %d benchmark ledgers, skipped %d setup ledgers", count, skipped)
	return count, nil
}

func streamFixturesToFile(
	ctx context.Context,
	cfg applyLoadConfig,
	opts Options,
	preBenchmarkCheckpoint uint32,
) (int, error) {
	checkpointReader, err := openCheckpointReader(ctx, opts.WorkDirPath, cfg.NetworkPassphrase, preBenchmarkCheckpoint)
	if err != nil {
		return 0, err
	}
	defer checkpointReader.Close()

	// Compute root account to filter it out (exists in any network with this passphrase)
	rootAccountID := keypair.Root(cfg.NetworkPassphrase).Address()

	outFile, err := os.Create(opts.FixturesPath)
	if err != nil {
		return 0, err
	}
	defer outFile.Close()

	writer, err := zstd.NewWriter(outFile)
	if err != nil {
		return 0, err
	}
	defer writer.Close()

	count := 0
	skipped := 0
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		change, err := checkpointReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}

		if change.Post != nil {
			entry := change.Post

			// Skip protocol-level entries that would conflict with existing DB state:
			// 1. Config settings - exist in any network
			// 2. Root account - derived from network passphrase, created at genesis
			if entry.Data.Type == xdr.LedgerEntryTypeConfigSetting {
				skipped++
				continue
			}
			if entry.Data.Type == xdr.LedgerEntryTypeAccount {
				if entry.Data.Account.AccountId.Address() == rootAccountID {
					skipped++
					continue
				}
			}

			if err := xdr.MarshalFramed(writer, entry); err != nil {
				return 0, err
			}
			count++
		}
	}

	opts.Logger.Infof("Wrote %d entries, skipped %d protocol entries", count, skipped)
	return count, nil
}

func encodeKey(e *xdr.LedgerEntry) (string, error) {
	k, err := e.LedgerKey()
	if err != nil {
		return "", err
	}
	return k.MarshalBinaryBase64()
}

func verifyFixturesCompleteness(ctx context.Context, cfg applyLoadConfig, opts Options, preBenchmarkCheckpoint uint32) error {
	knownKeys, err := loadFixtureKeys(ctx, cfg, opts.WorkDirPath, preBenchmarkCheckpoint)
	if err != nil {
		return err
	}
	opts.Logger.Infof("Loaded %d fixture keys into verification set", len(knownKeys))
	return replayAndVerify(ctx, cfg, opts, preBenchmarkCheckpoint, knownKeys)
}

// loadFixtureKeys returns the set of ledger entry keys present at preBenchmarkCheckpoint.
func loadFixtureKeys(
	ctx context.Context,
	cfg applyLoadConfig,
	workDir string,
	preBenchmarkCheckpoint uint32,
) (map[string]bool, error) {
	knownKeys := make(map[string]bool)
	checkpointReader, err := openCheckpointReader(ctx, workDir, cfg.NetworkPassphrase, preBenchmarkCheckpoint)
	if err != nil {
		return nil, err
	}
	defer checkpointReader.Close()

	for {
		var change ingest.Change
		if change, err = checkpointReader.Read(); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if change.Post != nil {
			keyB64, err := encodeKey(change.Post)
			if err != nil {
				return nil, err
			}
			knownKeys[keyB64] = true
		}
	}
	return knownKeys, nil
}

// replayAndVerify streams the benchmark ledgers and asserts every Pre referenced
// exists in knownKeys, mutating knownKeys to track Post adds and deletes.
func replayAndVerify(
	ctx context.Context,
	cfg applyLoadConfig,
	opts Options,
	preBenchmarkCheckpoint uint32,
	knownKeys map[string]bool,
) error {
	metadataPath, err := resolveMetadataPath(opts.WorkDirPath, cfg.MetadataOutputStream)
	if err != nil {
		return err
	}
	file, err := os.Open(metadataPath)
	if err != nil {
		return err
	}
	stream := xdr.NewStream(file)
	defer stream.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var ledger xdr.LedgerCloseMeta
		if err := stream.ReadOne(&ledger); err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		// Skip setup ledgers
		if ledger.LedgerSequence() <= preBenchmarkCheckpoint {
			continue
		}

		changeReader, err := ingest.NewLedgerChangeReaderFromLedgerCloseMeta(cfg.NetworkPassphrase, ledger)
		if err != nil {
			return err
		}

		for {
			change, err := changeReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}

			// If the change has a Pre state, the entry must already exist in our known set
			if change.Pre != nil {
				keyB64, err := encodeKey(change.Pre)
				if err != nil {
					return err
				}
				if !knownKeys[keyB64] {
					return fmt.Errorf("ledger key (ledger: %d, type: %v) not found in known set: %s",
						ledger.LedgerSequence(), change.Pre.Data.Type, keyB64)
				}
			}

			// Update our known set based on the Post state
			if change.Post != nil {
				// Entry exists after this change - add/keep in set
				keyB64, err := encodeKey(change.Post)
				if err != nil {
					return err
				}
				knownKeys[keyB64] = true
			} else if change.Pre != nil {
				// Entry was deleted - remove from set
				keyB64, err := encodeKey(change.Pre)
				if err != nil {
					return err
				}
				delete(knownKeys, keyB64)
			}
		}
		if err := changeReader.Close(); err != nil {
			return err
		}
	}
	return nil
}

// resolveMetadataPath resolves cfg.MetadataOutputStream against the work dir, rejecting
// absolute paths or `..` traversals that would escape it.
func resolveMetadataPath(workDir, metadataOutputStream string) (string, error) {
	cleanWorkDir := filepath.Clean(workDir)
	p := filepath.Join(cleanWorkDir, metadataOutputStream)
	if p != cleanWorkDir && !strings.HasPrefix(p, cleanWorkDir+string(filepath.Separator)) {
		return "", fmt.Errorf("METADATA_OUTPUT_STREAM %q escapes work dir", metadataOutputStream)
	}
	return p, nil
}

func parsePreBenchmarkCheckpoint(output string) (uint32, error) {
	// This parses a purpose-built log message from stellar-core's apply-load command.
	// It is the intended interface for communicating the pre-benchmark checkpoint boundary.
	re := regexp.MustCompile(`Published final checkpoint before benchmark: ledger (\d+)`)
	matches := re.FindStringSubmatch(output)
	if matches == nil {
		return 0, fmt.Errorf("could not find 'Published final checkpoint before benchmark' in stellar-core output")
	}

	ledger, err := strconv.ParseUint(matches[1], 10, 32)
	if err != nil {
		return 0, err
	}

	return uint32(ledger), nil
}

type applyLoadConfig struct {
	NetworkPassphrase    string
	MetadataOutputStream string
	HistoryArchiveName   string
}

func parseConfig(configPath string) (applyLoadConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return applyLoadConfig{}, err
	}

	var raw map[string]any
	err = toml.Unmarshal(data, &raw)
	if err != nil {
		return applyLoadConfig{}, err
	}

	passphrase, ok := raw["NETWORK_PASSPHRASE"].(string)
	if !ok {
		return applyLoadConfig{}, fmt.Errorf("NETWORK_PASSPHRASE not found in config")
	}

	metadataStream, ok := raw["METADATA_OUTPUT_STREAM"].(string)
	if !ok {
		return applyLoadConfig{}, fmt.Errorf("METADATA_OUTPUT_STREAM not found in config")
	}

	history, ok := raw["HISTORY"].(map[string]any)
	if !ok {
		return applyLoadConfig{}, fmt.Errorf("HISTORY section not found in config")
	}
	if len(history) != 1 {
		return applyLoadConfig{}, fmt.Errorf("expected exactly one history archive in config")
	}

	var archiveName string
	for name := range history {
		archiveName = name
	}

	return applyLoadConfig{
		NetworkPassphrase:    passphrase,
		MetadataOutputStream: metadataStream,
		HistoryArchiveName:   archiveName,
	}, nil
}

func openCheckpointReader(
	ctx context.Context,
	workDir, networkPassphrase string,
	checkpointLedger uint32,
) (ingest.ChangeReader, error) {
	archivePath := filepath.Join(workDir, "history")
	archive, err := historyarchive.Connect(
		"file://"+archivePath,
		historyarchive.ArchiveOptions{
			NetworkPassphrase: networkPassphrase,
		},
	)
	if err != nil {
		return nil, err
	}

	checkpointReader, err := ingest.NewCheckpointChangeReader(ctx, archive, checkpointLedger)
	if err != nil {
		return nil, err
	}

	return checkpointReader, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return err
	}
	return nil
}
