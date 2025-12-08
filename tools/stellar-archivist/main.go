// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/historyarchive"
	"github.com/stellar/go-stellar-sdk/support/errors"
)

const checkpointFrequency = uint32(64)

func status(a string, opts *Options) {
	arch := historyarchive.MustConnect(a, opts.ConnectOpts)
	state, err := arch.GetRootHAS()
	if err != nil {
		log.Fatal(errors.Wrap(err, "Error getting HAS"))
	}
	buckets, err := state.Buckets()
	if err != nil {
		log.Fatal(errors.Wrap(err, "Error getting buckets"))
	}
	summ, nz, err := state.LevelSummary()
	if err != nil {
		log.Fatal(errors.Wrap(err, "Error getting level summary"))
	}
	fmt.Printf("\n")
	fmt.Printf("       Archive: %s\n", a)
	fmt.Printf("        Server: %s\n", state.Server)
	fmt.Printf(" CurrentLedger: %d (0x%8.8x)\n", state.CurrentLedger, state.CurrentLedger)
	fmt.Printf("CurrentBuckets: %s (%d nonzero levels)\n", summ, nz)
	fmt.Printf(" Newest bucket: %s\n", buckets[0])
	fmt.Printf("\n")
}

type Options struct {
	Low         int
	High        uint32
	Last        int
	Recent      bool
	Profile     bool
	Debug       bool
	Trace       bool
	CommandOpts historyarchive.CommandOptions
	ConnectOpts historyarchive.ArchiveOptions
}

func (opts *Options) SetRange(srcArch *historyarchive.Archive, dstArch *historyarchive.Archive) {
	checkpointMgr := historyarchive.NewCheckpointManager(checkpointFrequency)
	if srcArch != nil {

		// If we got a src and dst archive and were passed --recent, we extract
		// the range as the sequence-difference between the two.
		if dstArch != nil && opts.Recent {
			srcState, e1 := srcArch.GetRootHAS()
			dstState, e2 := dstArch.GetRootHAS()
			if e1 == nil && e2 == nil {
				low := dstState.CurrentLedger
				high := srcState.CurrentLedger
				opts.CommandOpts.Range =
					checkpointMgr.MakeRange(low, high)
				return
			}

			// If we got a src, no dst, and a --last N flag, we extract the
			// range as the last N ledgers in the src archive.
		} else if opts.Last != -1 {
			state, e := srcArch.GetRootHAS()
			if e == nil {
				low := uint32(0)
				if state.CurrentLedger > uint32(opts.Last) {
					low = state.CurrentLedger - uint32(opts.Last)
				}
				opts.CommandOpts.Range =
					checkpointMgr.MakeRange(low, state.CurrentLedger)
				return
			}
		}
	}

	// Otherwise we fall back to the provided low and high, which further
	// default to the entire numeric range of a uint32.
	opts.CommandOpts.Range =
		checkpointMgr.MakeRange(uint32(opts.Low), uint32(opts.High))

}

func (opts *Options) MaybeProfile() {
	if opts.Profile {
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}
}

func (opts *Options) SetupLogging() {
	if opts.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("set logging to DEBUG level")
	}
	if opts.Trace {
		log.SetLevel(log.TraceLevel)
		log.Trace("set logging to TRACE level")
	}
}

func logArchive(a string, opts *Options) {
	arch := historyarchive.MustConnect(a, opts.ConnectOpts)
	opts.SetRange(arch, nil)
	arch.Log(&opts.CommandOpts)
}

func scan(a string, opts *Options) {
	arch := historyarchive.MustConnect(a, opts.ConnectOpts)
	opts.SetRange(arch, nil)
	e1 := arch.Scan(&opts.CommandOpts)
	missing, e2 := arch.ReportMissing(&opts.CommandOpts)
	invalid, e3 := arch.ReportInvalid(&opts.CommandOpts)
	if e1 != nil {
		log.Fatal(e1)
	}
	if e2 != nil {
		log.Fatal(e2)
	}
	if e3 != nil {
		log.Fatal(e3)
	}
	if missing {
		log.Fatal("Some objects were missing")
	}
	if invalid {
		log.Fatal("Some objects were invalid")
	}
}

func mirror(src string, dst string, opts *Options) {
	srcArch := historyarchive.MustConnect(src, opts.ConnectOpts)
	dstArch := historyarchive.MustConnect(dst, opts.ConnectOpts)
	opts.SetRange(srcArch, dstArch)
	log.Printf("mirroring %v -> %v\n", src, dst)
	e := historyarchive.Mirror(srcArch, dstArch, &opts.CommandOpts)
	if e != nil {
		log.Fatal(e)
	}
}

func repair(src string, dst string, opts *Options) {
	srcArch := historyarchive.MustConnect(src, opts.ConnectOpts)
	dstArch := historyarchive.MustConnect(dst, opts.ConnectOpts)
	opts.SetRange(srcArch, dstArch)
	log.Printf("repairing %v -> %v\n", src, dst)
	e := historyarchive.Repair(srcArch, dstArch, &opts.CommandOpts)
	if e != nil {
		log.Fatal(e)
	}
}

func main() {

	var opts Options
	opts.ConnectOpts.CheckpointFrequency = checkpointFrequency

	rootCmd := &cobra.Command{
		Use:   "stellar-archivist",
		Short: "inspect stellar history archive",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(0)
		},
	}

	rootCmd.PersistentFlags().IntVar(
		&opts.Low,
		"low",
		0,
		"first ledger to act on",
	)

	rootCmd.PersistentFlags().Uint32Var(
		&opts.High,
		"high",
		uint32(0xffffffff),
		"last ledger to act on",
	)

	rootCmd.PersistentFlags().BoolVarP(
		&opts.Recent,
		"recent",
		"r",
		false,
		"act on ledger-range difference between achives",
	)

	rootCmd.PersistentFlags().IntVar(
		&opts.Last,
		"last",
		-1,
		"number of recent ledgers to act on",
	)

	rootCmd.PersistentFlags().IntVarP(
		&opts.CommandOpts.Concurrency,
		"concurrency",
		"c",
		32,
		"number of files to operate on concurrently",
	)

	rootCmd.PersistentFlags().StringVar(
		&opts.ConnectOpts.S3Region,
		"s3region",
		"us-east-1",
		"S3 region to connect to",
	)

	rootCmd.PersistentFlags().StringVar(
		&opts.ConnectOpts.S3Endpoint,
		"s3endpoint",
		"",
		"S3 endpoint to use",
	)

	rootCmd.PersistentFlags().BoolVarP(
		&opts.CommandOpts.DryRun,
		"dryrun",
		"n",
		false,
		"describe file-writes, but do not perform any",
	)

	rootCmd.PersistentFlags().BoolVarP(
		&opts.CommandOpts.Force,
		"force",
		"f",
		false,
		"overwrite existing files",
	)

	rootCmd.PersistentFlags().BoolVar(
		&opts.CommandOpts.Verify,
		"verify",
		false,
		"verify file contents",
	)

	rootCmd.PersistentFlags().BoolVar(
		&opts.CommandOpts.Thorough,
		"thorough",
		false,
		"decode and re-encode all buckets",
	)

	rootCmd.PersistentFlags().BoolVar(
		&opts.CommandOpts.SkipOptional,
		"skip-optional",
		false,
		"skip optional (SCP) checkpoint files",
	)

	rootCmd.PersistentFlags().BoolVar(
		&opts.Profile,
		"profile",
		false,
		"collect and serve profile locally",
	)

	rootCmd.PersistentFlags().BoolVar(
		&opts.Debug,
		"debug",
		false,
		"set log level to DEBUG",
	)

	rootCmd.PersistentFlags().BoolVar(
		&opts.Trace,
		"trace",
		false,
		"set log level to TRACE",
	)

	rootCmd.AddCommand(&cobra.Command{
		Use: "status",
		Run: func(cmd *cobra.Command, args []string) {
			opts.SetupLogging()
			status(firstArg(args), &opts)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use: "log",
		Run: func(cmd *cobra.Command, args []string) {
			opts.SetupLogging()
			opts.MaybeProfile()
			logArchive(firstArg(args), &opts)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use: "scan",
		Run: func(cmd *cobra.Command, args []string) {
			opts.SetupLogging()
			opts.MaybeProfile()
			scan(firstArg(args), &opts)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use: "mirror",
		Run: func(cmd *cobra.Command, args []string) {
			opts.SetupLogging()
			opts.MaybeProfile()
			src, dst := srcDst(args)
			mirror(src, dst, &opts)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use: "repair",
		Run: func(cmd *cobra.Command, args []string) {
			opts.SetupLogging()
			opts.MaybeProfile()
			src, dst := srcDst(args)
			repair(src, dst, &opts)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use: "dumpxdr",
		Run: func(cmd *cobra.Command, args []string) {
			opts.SetupLogging()
			err := historyarchive.DumpXdrAsJson(args)
			if err != nil {
				log.Fatal(err)
			}
		},
	})

	rootCmd.Execute()
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}

	return args[0]
}

func srcDst(args []string) (string, string) {
	if len(args) != 2 {
		log.Fatal("require exactly 2 arguments")
	}

	src := args[0]
	dst := args[1]
	if src == "" || dst == "" {
		log.Fatal("require exactly 2 arguments")
	}

	return src, dst
}
