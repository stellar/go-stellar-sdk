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

// Returns the associated end ledger for a given ledgerSeq based on the datastore
// configuration for LedgersPerFile and a ceiling of MaxUint32.
func (ec DataStoreSchema) GetSequenceNumberEndBoundary(ledgerSeq uint32) uint32 {
	end64 := uint64(ec.GetSequenceNumberStartBoundary(ledgerSeq)) + uint64(ec.LedgersPerFile) - 1
	if end64 > uint64(math.MaxUint32) {
		end64 = uint64(math.MaxUint32)
	}
	return uint32(end64)
}

// GetObjectKeyFromSequenceNumber generates the object key name from the ledger sequence number based on configuration.
func (ec DataStoreSchema) GetObjectKeyFromSequenceNumber(ledgerSeq uint32) string {
	var objectKey string

	if ec.FilesPerPartition > 1 {
		partitionSize := ec.LedgersPerFile * ec.FilesPerPartition
		partitionStart := (ledgerSeq / partitionSize) * partitionSize

		end64 := uint64(partitionStart) + uint64(partitionSize) - 1
		if end64 > uint64(math.MaxUint32) {
			end64 = uint64(math.MaxUint32)
		}
		partitionEnd := uint32(end64)

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
