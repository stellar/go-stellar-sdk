// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package historyarchive

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

type scanCheckpointFastReq struct {
	category   string
	pathprefix string
}

type scanCheckpointSlowReq struct {
	category   string
	checkpoint uint32
}

func (arch *Archive) ScanCheckpoints(opts *CommandOptions) error {
	state, e := arch.GetRootHAS()
	if e != nil {
		return e
	}
	opts.Range = opts.Range.clamp(state.Range(), arch.checkpointManager)

	log.Printf("Scanning checkpoint files in range: %s", opts.Range)

	if arch.backend.CanListFiles() {
		return arch.ScanCheckpointsFast(opts)
	} else {
		return arch.ScanCheckpointsSlow(opts)
	}
}

func (arch *Archive) ScanCheckpointsSlow(opts *CommandOptions) error {

	if opts.Concurrency == 0 {
		return errors.New("zero concurrency")
	}

	var errs uint32
	tick := makeTicker(func(_ uint) {
		arch.ReportCheckpointStats()
	})

	var wg sync.WaitGroup
	wg.Add(opts.Concurrency)

	req := make(chan scanCheckpointSlowReq)

	cats := Categories()
	go func() {
		for _, cat := range cats {
			if opts.SkipOptional && !categoryRequired(cat) {
				continue
			}
			for chk := range opts.Range.GenerateCheckpoints(arch.checkpointManager) {
				req <- scanCheckpointSlowReq{category: cat, checkpoint: chk}
			}
		}
		close(req)
	}()

	for i := 0; i < opts.Concurrency; i++ {
		go func() {
			for {
				r, ok := <-req
				if !ok {
					break
				}
				exists, err := arch.CategoryCheckpointExists(r.category, r.checkpoint)
				if err != nil {
					panic(err)
				}
				tick <- true
				arch.NoteCheckpointFile(r.category, r.checkpoint, exists)
				if exists && opts.Verify {
					atomic.AddUint32(&errs,
						noteError(arch.VerifyCategoryCheckpoint(r.category,
							r.checkpoint)))
				}
			}
			wg.Done()
		}()
	}

	wg.Wait()
	close(tick)
	log.Printf("Checkpoint files scanned with %d errors", errs)
	arch.ReportCheckpointStats()
	if errs != 0 {
		return fmt.Errorf("%d errors scanning checkpoints", errs)
	}
	return nil
}

func (arch *Archive) ScanCheckpointsFast(opts *CommandOptions) error {

	if opts.Concurrency == 0 {
		return errors.New("zero concurrency")
	}

	var errs uint32
	tick := makeTicker(func(_ uint) {
		arch.ReportCheckpointStats()
	})

	var wg sync.WaitGroup
	wg.Add(opts.Concurrency)

	req := make(chan scanCheckpointFastReq)

	cats := Categories()
	go func() {
		for _, cat := range cats {
			if opts.SkipOptional && !categoryRequired(cat) {
				continue
			}
			for _, pth := range RangePaths(opts.Range) {
				req <- scanCheckpointFastReq{category: cat, pathprefix: pth}
			}
		}
		close(req)
	}()

	for i := 0; i < opts.Concurrency; i++ {
		go func() {
			for {
				r, ok := <-req
				if !ok {
					break
				}
				ch, es := arch.ListCategoryCheckpoints(r.category, r.pathprefix)
				for n := range ch {
					if n < opts.Range.Low || n > opts.Range.High {
						continue
					}
					tick <- true
					arch.NoteCheckpointFile(r.category, n, true)
					if opts.Verify {
						atomic.AddUint32(&errs,
							noteError(arch.VerifyCategoryCheckpoint(r.category, n)))
					}
				}
				atomic.AddUint32(&errs, drainErrors(es))
			}
			wg.Done()
		}()
	}

	wg.Wait()
	close(tick)
	log.Printf("Checkpoint files scanned with %d errors", errs)
	arch.ReportCheckpointStats()
	if errs != 0 {
		return fmt.Errorf("%d errors scanning checkpoints", errs)
	}
	return nil
}

func (arch *Archive) Scan(opts *CommandOptions) error {
	e1 := arch.ScanCheckpoints(opts)
	if e1 != nil {
		return e1
	}
	e2 := arch.ScanBuckets(opts)
	if e2 != nil {
		return e2
	}
	return nil
}

func (arch *Archive) ScanAllBuckets() error {
	log.Printf("Scanning all buckets, and those referenced by range")

	tick := makeTicker(func(_ uint) {
		arch.ReportBucketStats()
	})

	allBuckets, ech := arch.ListAllBucketHashes()

	for b := range allBuckets {
		arch.NoteExistingBucket(b)
		tick <- true
	}

	errs := drainErrors(ech)
	if errs != 0 {
		return fmt.Errorf("%d errors while scanning all buckets", errs)
	}
	return nil
}

func (arch *Archive) ScanBuckets(opts *CommandOptions) error {

	if opts.Concurrency == 0 {
		return errors.New("zero concurrency")
	}

	var errs uint32

	// First scan _all_ buckets if we can (and should -- if asked to look at the
	// entire range); if not, we'll do an exists-check on each bucket as we
	// go. But this is faster when we can do it.
	doList := arch.backend.CanListFiles()
	has, err := arch.GetRootHAS()
	if err == nil {
		fullRange := arch.checkpointManager.MakeRange(0, has.CurrentLedger)
		doList = doList && opts.Range.SizeInCheckPoints(arch.checkpointManager) == fullRange.SizeInCheckPoints(arch.checkpointManager)
	} else {
		log.Print("Error retrieving root archive state, possibly corrupt archive:", err)
		log.Print("Continuing and will do an exists-check on each bucket as we go, this will be slower")
	}
	if doList {
		errs += noteError(arch.ScanAllBuckets())
	} else {
		log.Printf("Scanning buckets for %d checkpoints", opts.Range.SizeInCheckPoints(arch.checkpointManager))
	}

	// Grab the set of checkpoints we have HASs for, to read references.
	arch.mutex.Lock()
	hists := arch.checkpointFiles["history"]
	seqs := make([]uint32, 0, len(hists))
	for k, present := range hists {
		if present {
			seqs = append(seqs, k)
		}
	}
	arch.mutex.Unlock()

	var wg sync.WaitGroup
	wg.Add(opts.Concurrency)

	tick := makeTicker(func(_ uint) {
		arch.ReportBucketStats()
	})

	// Make a bunch of goroutines that pull each HAS and enumerate
	// its buckets into a channel. These are the _referenced_ buckets.
	req := make(chan uint32)
	go func() {
		for _, seq := range seqs {
			req <- seq
		}
		close(req)
	}()
	for i := 0; i < opts.Concurrency; i++ {
		go func() {
			for {
				ix, ok := <-req
				if !ok {
					break
				}
				has, e := arch.GetCheckpointHAS(ix)
				atomic.AddUint32(&errs, noteError(e))
				buckets, err := has.Buckets()
				if err != nil {
					panic(err)
				}
				for _, bucket := range buckets {
					new := arch.NoteReferencedBucket(bucket)
					if !new {
						continue
					}

					if !doList || opts.Verify {
						exists, err := arch.BucketExists(bucket)
						if err != nil {
							panic(err)
						}
						if exists {
							if !doList {
								arch.NoteExistingBucket(bucket)
							}
							if opts.Verify {
								n := uint32(0)
								if opts.Thorough {
									n = noteError(arch.VerifyBucketEntries(bucket))
								} else {
									n = noteError(arch.VerifyBucketHash(bucket))
								}
								atomic.AddUint32(&errs, n)
								if n != 0 {
									arch.mutex.Lock()
									arch.invalidBuckets++
									arch.mutex.Unlock()
								}
							}
						}
					}
				}
				tick <- true
			}
			wg.Done()
		}()
	}

	wg.Wait()
	arch.ReportBucketStats()
	close(tick)
	if errs != 0 {
		return fmt.Errorf("%d errors while scanning buckets", errs)
	}
	return nil
}

func (arch *Archive) ClearCachedInfo() {
	arch.mutex.Lock()
	defer arch.mutex.Unlock()
	for _, cat := range Categories() {
		arch.checkpointFiles[cat] = make(map[uint32]bool)
	}
	arch.allBuckets = make(map[Hash]bool)
	arch.referencedBuckets = make(map[Hash]bool)
}

func (arch *Archive) ReportCheckpointStats() {
	arch.mutex.Lock()
	defer arch.mutex.Unlock()
	s := make([]string, 0)
	for _, cat := range Categories() {
		tab := arch.checkpointFiles[cat]
		s = append(s, fmt.Sprintf("%d %s", len(tab), cat))
	}
	log.Printf("Archive: %s", strings.Join(s, ", "))
}

func (arch *Archive) ReportBucketStats() {
	arch.mutex.Lock()
	defer arch.mutex.Unlock()
	log.Printf("Archive: %d buckets total, %d referenced",
		len(arch.allBuckets), len(arch.referencedBuckets))
}

func (arch *Archive) NoteCheckpointFile(cat string, chk uint32, present bool) {
	arch.mutex.Lock()
	defer arch.mutex.Unlock()
	arch.checkpointFiles[cat][chk] = present
}

func (arch *Archive) NoteExistingBucket(bucket Hash) {
	arch.mutex.Lock()
	defer arch.mutex.Unlock()
	arch.allBuckets[bucket] = true
}

func (arch *Archive) NoteReferencedBucket(bucket Hash) bool {
	arch.mutex.Lock()
	defer arch.mutex.Unlock()
	_, mapEntryExists := arch.referencedBuckets[bucket]
	if mapEntryExists {
		return false
	}
	arch.referencedBuckets[bucket] = true
	return true
}

func (arch *Archive) CheckCheckpointFilesMissing(opts *CommandOptions) map[string][]uint32 {
	arch.mutex.Lock()
	defer arch.mutex.Unlock()
	missing := make(map[string][]uint32)
	for _, cat := range Categories() {
		missing[cat] = make([]uint32, 0)
		for ix := range opts.Range.GenerateCheckpoints(arch.checkpointManager) {
			fileExists := arch.checkpointFiles[cat][ix]
			if !fileExists {
				missing[cat] = append(missing[cat], ix)
			}
		}
	}
	return missing
}

func (arch *Archive) CheckBucketsMissing() map[Hash]bool {
	arch.mutex.Lock()
	defer arch.mutex.Unlock()
	missing := make(map[Hash]bool)
	for k := range arch.referencedBuckets {
		bucketExists := arch.allBuckets[k]
		if !bucketExists {
			missing[k] = true
		}
	}
	return missing
}

func (arch *Archive) ReportMissing(opts *CommandOptions) (bool, error) {

	log.Printf("Examining checkpoint files for gaps")
	missingCheckpointFiles := arch.CheckCheckpointFilesMissing(opts)
	log.Printf("Examining buckets referenced by checkpoints")
	missingBuckets := arch.CheckBucketsMissing()

	missingCheckpoints := false
	for cat, missing := range missingCheckpointFiles {
		if !categoryRequired(cat) {
			if len(missing) > 0 {
				s := fmtRangeList(missing, arch.checkpointManager)
				if opts.SkipOptional {
					log.Warnf("Skipped optional %s files (%d): %s", cat, len(missing), s)
				} else {
					log.Warnf("Missing optional %s files (%d): %s", cat, len(missing), s)
				}
			}
			continue
		}
		if len(missing) != 0 {
			s := fmtRangeList(missing, arch.checkpointManager)
			missingCheckpoints = true
			log.Errorf("Missing %s files (%d): %s", cat, len(missing), s)
		}
	}

	if !missingCheckpoints {
		log.Printf("No checkpoint files missing in range %s", opts.Range)
	}

	for bucket := range missingBuckets {
		log.Errorf("Missing bucket: %s", bucket)
	}

	if len(missingBuckets) == 0 {
		log.Printf("No missing buckets referenced in range %s", opts.Range)
	}

	return missingCheckpoints || len(missingBuckets) != 0, nil
}
