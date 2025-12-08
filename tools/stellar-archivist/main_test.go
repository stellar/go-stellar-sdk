// Copyright 2019 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/historyarchive"
	"github.com/stretchr/testify/assert"
)

func TestLastOption(t *testing.T) {
	src_arch := historyarchive.MustConnect("mock://test", historyarchive.ArchiveOptions{CheckpointFrequency: 64})
	assert.NotEqual(t, nil, src_arch)

	var src_has historyarchive.HistoryArchiveState
	src_has.CurrentLedger = uint32(0xbf)
	cmd_opts := &historyarchive.CommandOptions{Force: true}
	src_arch.PutRootHAS(src_has, cmd_opts)

	var opts Options
	opts.Last = 10
	opts.SetRange(src_arch, nil)
	assert.Equal(t, uint32(0x7f), opts.CommandOpts.Range.Low)
	assert.Equal(t, uint32(0xbf), opts.CommandOpts.Range.High)
}

func TestRecentOption(t *testing.T) {
	src_arch := historyarchive.MustConnect("mock://test1", historyarchive.ArchiveOptions{CheckpointFrequency: 64})
	dst_arch := historyarchive.MustConnect("mock://test2", historyarchive.ArchiveOptions{CheckpointFrequency: 64})
	assert.NotEqual(t, nil, src_arch)
	assert.NotEqual(t, nil, dst_arch)

	var src_has, dst_has historyarchive.HistoryArchiveState
	src_has.CurrentLedger = uint32(0xbf)
	dst_has.CurrentLedger = uint32(0x3f)
	cmd_opts := &historyarchive.CommandOptions{Force: true}
	src_arch.PutRootHAS(src_has, cmd_opts)
	dst_arch.PutRootHAS(dst_has, cmd_opts)

	var opts Options
	opts.Recent = true
	opts.SetRange(src_arch, dst_arch)
	assert.Equal(t, uint32(0x3f), opts.CommandOpts.Range.Low)
	assert.Equal(t, uint32(0xbf), opts.CommandOpts.Range.High)
}
