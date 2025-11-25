package datastore

import (
	"fmt"
	"math"

	"github.com/stellar/go-stellar-sdk/support/compressxdr"
)

type DataStoreSchema struct {
	LedgersPerFile    uint32 `toml:"ledgers_per_file"`
	FilesPerPartition uint32 `toml:"files_per_partition"`
	FileExtension     string // Optional – for backward (zstd) compatibility only
}

func (ec DataStoreSchema) GetSequenceNumberStartBoundary(ledgerSeq uint32) uint32 {
	if ec.LedgersPerFile == 0 {
		return 0
	}
	return (ledgerSeq / ec.LedgersPerFile) * ec.LedgersPerFile
}

func (ec DataStoreSchema) GetSequenceNumberEndBoundary(ledgerSeq uint32) uint32 {
	return ec.GetSequenceNumberStartBoundary(ledgerSeq) + ec.LedgersPerFile - 1
}

// GetObjectKeyFromSequenceNumber generates the object key name from the ledger sequence number based on configuration.
func (ec DataStoreSchema) GetObjectKeyFromSequenceNumber(ledgerSeq uint32) string {
	var objectKey string

	if ec.FilesPerPartition > 1 {
		partitionSize := ec.LedgersPerFile * ec.FilesPerPartition
		partitionStart := (ledgerSeq / partitionSize) * partitionSize
		partitionEnd := partitionStart + partitionSize - 1

		objectKey = fmt.Sprintf("%08X--%d-%d/", math.MaxUint32-partitionStart, partitionStart, partitionEnd)
	}

	fileStart := ec.GetSequenceNumberStartBoundary(ledgerSeq)
	fileEnd := ec.GetSequenceNumberEndBoundary(ledgerSeq)
	objectKey += fmt.Sprintf("%08X--%d", math.MaxUint32-fileStart, fileStart)

	// Multiple ledgers per file
	if fileStart != fileEnd {
		objectKey += fmt.Sprintf("-%d", fileEnd)
	}

	if ec.FileExtension == "" {
		ec.FileExtension = compressxdr.DefaultCompressor.Name()
	}

	objectKey += fmt.Sprintf(".xdr.%s", ec.FileExtension)

	return objectKey
}
