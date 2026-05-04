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

	"github.com/BurntSushi/toml"
	"github.com/klauspost/compress/zstd"
	"github.com/stellar/go-stellar-sdk/historyarchive"
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type ApplyLoad struct {
	// Inputs (set in New)
	logger         *log.Entry
	coreBinaryPath string
	configPath     string
	cfg            applyLoadConfig
	outputPath     string // optional
	fixturesPath   string // optional
	workDir        string

	preBenchmarkCheckpoint uint32
}

// NewApplyLoad creates a new ApplyLoad instance with the given parameters.
// If outputPath and fixturesPath are both set, ledgers and fixtures are written there after running apply-load.
// If workDirPath is not set, a temporary directory will be created for stellar-core's working directory.
func NewApplyLoad(
	logger *log.Entry,
	coreBinaryPath, configPath, outputPath, fixturesPath, workDirPath string,
) (*ApplyLoad, error) {
	cfg, err := parseConfig(configPath)
	if err != nil {
		return nil, err
	}

	if coreBinaryPath == "" {
		if coreBinaryPath, err = exec.LookPath("stellar-core"); err != nil {
			return nil, fmt.Errorf("stellar-core binary unspecified and not found in PATH: %w", err)
		}
	}
	coreVersion, err := ledgerbackend.CoreBuildVersion(coreBinaryPath)
	if err != nil {
		return nil, fmt.Errorf("error getting stellar-core version: %w", err)
	}
	logger.Infof("Using stellar-core: %s %s", coreBinaryPath, coreVersion)
	logger.Infof("Using config: %s", configPath)

	if (outputPath != "") != (fixturesPath != "") {
		return nil, fmt.Errorf("outputPath and fixturesPath must both be set or both be empty")
	}

	if workDirPath == "" {
		var err error
		workDirPath, err = os.MkdirTemp("", "apply-load-workdir-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temporary work directory: %w", err)
		}
	}

	if logger == nil {
		logger = log.New()
	}

	return &ApplyLoad{
		logger:         logger,
		coreBinaryPath: coreBinaryPath,
		configPath:     configPath,
		cfg:            cfg,
		outputPath:     outputPath,
		fixturesPath:   fixturesPath,
		workDir:        workDirPath,
	}, nil
}

// Cleanup removes the working directory and all its contents. Should be called after RunApplyLoadAndWrite.
func (a *ApplyLoad) Cleanup() error {
	if a.workDir == "" {
		return nil
	}
	return os.RemoveAll(a.workDir)
}

// RunApplyLoadAndWrite runs the apply-load process and writes outputs to files if paths are set.
func (a *ApplyLoad) RunApplyLoadAndWrite(ctx context.Context) error {
	// Run apply-load
	if err := a.run(); err != nil {
		return err
	}

	// Verify fixtures completeness before writing anything
	if err := a.verifyFixturesCompleteness(ctx); err != nil {
		return fmt.Errorf("fixture completeness verification failed: %w", err)
	}

	// Stream ledgers to output file or just count them for verification
	// Only include benchmark ledgers (after the pre-benchmark checkpoint),
	// as setup ledgers would conflict with the fixtures.
	if a.outputPath != "" {
		if count, err := a.streamLedgersToFile(); err != nil {
			return fmt.Errorf("error streaming ledgers to file: %w", err)
		} else if count == 0 {
			return fmt.Errorf("no benchmark ledgers found to write to file")
		}
	}

	// Stream fixtures to output file
	if a.fixturesPath != "" {
		if _, err := a.streamFixturesToFile(ctx); err != nil {
			return fmt.Errorf("error streaming fixtures to file: %w", err)
		}
	}
	return nil
}

// run executes the stellar-core apply-load command and captures the pre-benchmark checkpoint from its output.
func (a *ApplyLoad) run() error {
	// Copy config to work dir (apply-load writes files relative to config location)
	destConfigPath := filepath.Join(a.workDir, "apply-load.cfg")
	if err := copyFile(a.configPath, destConfigPath); err != nil {
		return fmt.Errorf("failed to copy config file: %w", err)
	}

	// Step 1: Initialize database
	newDBCmd := exec.Command(a.coreBinaryPath, "new-db", "--conf", destConfigPath)
	newDBCmd.Dir = a.workDir
	output, err := newDBCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("new-db failed:\noutput: %s\nerror: %w", string(output), err)
	}

	// Step 2: Initialize history archive
	newHistCmd := exec.Command(a.coreBinaryPath, "new-hist", a.cfg.HistoryArchiveName, "--conf", destConfigPath)
	newHistCmd.Dir = a.workDir
	output, err = newHistCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("new-hist failed:\noutput: %s\nerror: %w", string(output), err)
	}
	a.logger.Infof("Initialized history archive: %s\n", a.cfg.HistoryArchiveName)

	// Step 3: Execute stellar-core apply-load
	applyLoadCmd := exec.Command(a.coreBinaryPath, "apply-load", "--conf", destConfigPath)
	applyLoadCmd.Dir = a.workDir
	output, err = applyLoadCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apply-load failed:\noutput: %s\nerror: %w", string(output), err)
	}

	// Parse pre-benchmark checkpoint from stellar-core output.
	// The config sets LOG_FILE_PATH="" so logs go to stdout.
	if a.preBenchmarkCheckpoint, err = parsePreBenchmarkCheckpoint(string(output)); err != nil {
		return fmt.Errorf("failed to parse pre-benchmark checkpoint: %w", err)
	}
	a.logger.Infof("Pre-benchmark checkpoint: ledger %d", a.preBenchmarkCheckpoint)

	return nil
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
		return 0, fmt.Errorf("failed to parse ledger number: %w", err)
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

func (a *ApplyLoad) streamLedgersToFile() (int, error) {
	metadataPath := filepath.Join(a.workDir, a.cfg.MetadataOutputStream)
	inFile, err := os.Open(metadataPath)
	if err != nil {
		return 0, err
	}
	defer inFile.Close()

	// Note: xdr.Stream closes the underlying reader when ReadOne hits EOF or error
	stream := xdr.NewStream(inFile)

	outFile, err := os.Create(a.outputPath)
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
		var ledger xdr.LedgerCloseMeta
		if err := stream.ReadOne(&ledger); err == io.EOF {
			break
		} else if err != nil {
			return 0, err
		}

		// Skip setup ledgers (ledgers up to and including the pre-benchmark checkpoint).
		// Only include benchmark ledgers which operate on the fixture state.
		if ledger.LedgerSequence() <= a.preBenchmarkCheckpoint {
			skipped++
			continue
		}

		if err := xdr.MarshalFramed(writer, ledger); err != nil {
			return 0, err
		}
		count++
	}

	a.logger.Infof("Wrote %d benchmark ledgers, skipped %d setup ledgers", count, skipped)
	return count, nil
}

func (a *ApplyLoad) streamFixturesToFile(ctx context.Context) (int, error) {
	checkpointReader, err := openCheckpointReader(ctx, a.workDir, a.cfg.NetworkPassphrase, a.preBenchmarkCheckpoint)
	if err != nil {
		return 0, err
	}
	defer checkpointReader.Close()

	// Compute root account to filter it out (exists in any network with this passphrase)
	rootAccountID := keypair.Root(a.cfg.NetworkPassphrase).Address()

	outFile, err := os.Create(a.fixturesPath)
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

	a.logger.Infof("Wrote %d entries, skipped %d protocol entries", count, skipped)
	return count, nil
}

func (a *ApplyLoad) verifyFixturesCompleteness(ctx context.Context) error {
	if a.fixturesPath == "" {
		return nil // no fixtures to verify
	}
	metadataPath := filepath.Join(a.workDir, a.cfg.MetadataOutputStream)

	// Step 1: Load all ledger entry keys from fixtures into a set
	knownKeys := make(map[string]bool)
	checkpointReader, err := openCheckpointReader(ctx, a.workDir, a.cfg.NetworkPassphrase, a.preBenchmarkCheckpoint)
	if err != nil {
		return err
	}
	defer checkpointReader.Close()

	fixtureCount := 0
	for {
		var change ingest.Change
		if change, err = checkpointReader.Read(); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if change.Post != nil {
			var key xdr.LedgerKey
			key, err = change.Post.LedgerKey()
			if err != nil {
				return err
			}
			var keyB64 string
			keyB64, err = key.MarshalBinaryBase64()
			if err != nil {
				return err
			}
			knownKeys[keyB64] = true
			fixtureCount++
		}
	}
	a.logger.Infof("Loaded %d fixture keys into verification set", fixtureCount)

	// Step 2: Stream through generated ledgers and verify each change
	file, err := os.Open(metadataPath)
	if err != nil {
		return err
	}
	defer file.Close()
	stream := xdr.NewStream(file)

	for {
		var ledger xdr.LedgerCloseMeta
		if err := stream.ReadOne(&ledger); err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		// Skip setup ledgers
		if ledger.LedgerSequence() <= a.preBenchmarkCheckpoint {
			continue
		}

		// Extract changes from this ledger
		changeReader, err := ingest.NewLedgerChangeReaderFromLedgerCloseMeta(a.cfg.NetworkPassphrase, ledger)
		if err != nil {
			return fmt.Errorf("error creating change reader: %w", err)
		}

		for {
			change, err := changeReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("error reading change: %w", err)
			}

			// If the change has a Pre state, the entry must already exist in our known set
			if change.Pre != nil {
				key, err := change.Pre.LedgerKey()
				if err != nil {
					return fmt.Errorf("error getting ledger key: %w", err)
				}
				keyB64, err := key.MarshalBinaryBase64()
				if err != nil {
					return fmt.Errorf("error marshalling ledger key: %w", err)
				}
				if !knownKeys[keyB64] {
					return fmt.Errorf("ledger key not found in known set: %s", keyB64)
				}
			}

			// Update our known set based on the Post state
			if change.Post != nil {
				// Entry exists after this change - add/keep in set
				key, err := change.Post.LedgerKey()
				if err != nil {
					return fmt.Errorf("error getting ledger key: %w", err)
				}
				keyB64, err := key.MarshalBinaryBase64()
				if err != nil {
					return fmt.Errorf("error marshalling ledger key: %w", err)
				}
				knownKeys[keyB64] = true
			} else if change.Pre != nil {
				// Entry was deleted - remove from set
				key, err := change.Pre.LedgerKey()
				if err != nil {
					return fmt.Errorf("error getting ledger key: %w", err)
				}
				keyB64, err := key.MarshalBinaryBase64()
				if err != nil {
					return fmt.Errorf("error marshalling ledger key: %w", err)
				}
				delete(knownKeys, keyB64)
			}
		}
		if err := changeReader.Close(); err != nil {
			return fmt.Errorf("error closing change reader: %w", err)
		}
	}
	return nil
}

func openCheckpointReader(ctx context.Context, workDir, networkPassphrase string, checkpointLedger uint32) (ingest.ChangeReader, error) {
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
