%#include "xdr/Stellar-types.h"

namespace stellar {
// General “Soroban execution lane” settings
struct ConfigSettingContractExecutionLanesV0
{
    // maximum number of Soroban transactions per ledger
    uint32 ledgerMaxTxCount;
};

// "Compute" settings for contracts (instructions and memory).
struct ConfigSettingContractComputeV0
{
    // Maximum instructions per ledger
    int64 ledgerMaxInstructions;
    // Maximum instructions per transaction
    int64 txMaxInstructions;
    // Cost of 10000 instructions
    int64 feeRatePerInstructionsIncrement;

    // Memory limit per transaction. Unlike instructions, there is no fee
    // for memory, just the limit.
    uint32 txMemoryLimit;
};

// Settings for running the contract transactions in parallel.
struct ConfigSettingContractParallelComputeV0
{
    // Maximum number of clusters with dependent transactions allowed in a
    // stage of parallel tx set component.
    // This effectively sets the lower bound on the number of physical threads
    // necessary to effectively apply transaction sets in parallel.
    uint32 ledgerMaxDependentTxClusters;
};

// Ledger access settings for contracts.
struct ConfigSettingContractLedgerCostV0
{
    // Maximum number of disk entry read operations per ledger
    uint32 ledgerMaxDiskReadEntries;
    // Maximum number of bytes of disk reads that can be performed per ledger
    uint32 ledgerMaxDiskReadBytes;
    // Maximum number of ledger entry write operations per ledger
    uint32 ledgerMaxWriteLedgerEntries;
    // Maximum number of bytes that can be written per ledger
    uint32 ledgerMaxWriteBytes;

    // Maximum number of disk entry read operations per transaction
    uint32 txMaxDiskReadEntries;
    // Maximum number of bytes of disk reads that can be performed per transaction
    uint32 txMaxDiskReadBytes;
    // Maximum number of ledger entry write operations per transaction
    uint32 txMaxWriteLedgerEntries;
    // Maximum number of bytes that can be written per transaction
    uint32 txMaxWriteBytes;

    int64 feeDiskReadLedgerEntry;  // Fee per disk ledger entry read
    int64 feeWriteLedgerEntry;     // Fee per ledger entry write

    int64 feeDiskRead1KB;          // Fee for reading 1KB disk

    // The following parameters determine the write fee per 1KB.
    // Rent fee grows linearly until soroban state reaches this size
    int64 sorobanStateTargetSizeBytes;
    // Fee per 1KB rent when the soroban state is empty
    int64 rentFee1KBSorobanStateSizeLow;
    // Fee per 1KB rent when the soroban state has reached `sorobanStateTargetSizeBytes`
    int64 rentFee1KBSorobanStateSizeHigh;
    // Rent fee multiplier for any additional data past the first `sorobanStateTargetSizeBytes`
    uint32 sorobanStateRentFeeGrowthFactor;
};

// Ledger access settings for contracts.
struct ConfigSettingContractLedgerCostExtV0
{
    // Maximum number of RO+RW entries in the transaction footprint.
    uint32 txMaxFootprintEntries;
    // Fee per 1 KB of data written to the ledger.
    // Unlike the rent fee, this is a flat fee that is charged for any ledger
    // write, independent of the type of the entry being written.
    int64 feeWrite1KB;
};

// Historical data (pushed to core archives) settings for contracts.
struct ConfigSettingContractHistoricalDataV0
{
    int64 feeHistorical1KB; // Fee for storing 1KB in archives
};

// Contract event-related settings.
struct ConfigSettingContractEventsV0
{
    // Maximum size of events that a contract call can emit.
    uint32 txMaxContractEventsSizeBytes;
    // Fee for generating 1KB of contract events.
    int64 feeContractEvents1KB;
};

// Bandwidth related data settings for contracts.
// We consider bandwidth to only be consumed by the transaction envelopes, hence
// this concerns only transaction sizes.
struct ConfigSettingContractBandwidthV0
{
    // Maximum sum of all transaction sizes in the ledger in bytes
    uint32 ledgerMaxTxsSizeBytes;
    // Maximum size in bytes for a transaction
    uint32 txMaxSizeBytes;

    // Fee for 1 KB of transaction size
    int64 feeTxSize1KB;
};

enum ContractCostType {
    // Cost of running 1 wasm instruction
    WasmInsnExec = 0,
    // Cost of allocating a slice of memory (in bytes)
    MemAlloc = 1,
    // Cost of copying a slice of bytes into a pre-allocated memory
    MemCpy = 2,
    // Cost of comparing two slices of memory
    MemCmp = 3,
    // Cost of a host function dispatch, not including the actual work done by
    // the function nor the cost of VM invocation machinary
    DispatchHostFunction = 4,
    // Cost of visiting a host object from the host object storage. Exists to 
    // make sure some baseline cost coverage, i.e. repeatly visiting objects
    // by the guest will always incur some charges.
    VisitObject = 5,
    // Cost of serializing an xdr object to bytes
    ValSer = 6,
    // Cost of deserializing an xdr object from bytes
    ValDeser = 7,
    // Cost of computing the sha256 hash from bytes
    ComputeSha256Hash = 8,
    // Cost of computing the ed25519 pubkey from bytes
    ComputeEd25519PubKey = 9,
    // Cost of verifying ed25519 signature of a payload.
    VerifyEd25519Sig = 10,
    // Cost of instantiation a VM from wasm bytes code.
    VmInstantiation = 11,
    // Cost of instantiation a VM from a cached state.
    VmCachedInstantiation = 12,
    // Cost of invoking a function on the VM. If the function is a host function,
    // additional cost will be covered by `DispatchHostFunction`.
    InvokeVmFunction = 13,
    // Cost of computing a keccak256 hash from bytes.
    ComputeKeccak256Hash = 14,
    // Cost of decoding an ECDSA signature computed from a 256-bit prime modulus
    // curve (e.g. secp256k1 and secp256r1)
    DecodeEcdsaCurve256Sig = 15,
    // Cost of recovering an ECDSA secp256k1 key from a signature.
    RecoverEcdsaSecp256k1Key = 16,
    // Cost of int256 addition (`+`) and subtraction (`-`) operations
    Int256AddSub = 17,
    // Cost of int256 multiplication (`*`) operation
    Int256Mul = 18,
    // Cost of int256 division (`/`) operation
    Int256Div = 19,
    // Cost of int256 power (`exp`) operation
    Int256Pow = 20,
    // Cost of int256 shift (`shl`, `shr`) operation
    Int256Shift = 21,
    // Cost of drawing random bytes using a ChaCha20 PRNG
    ChaCha20DrawBytes = 22,

    // Cost of parsing wasm bytes that only encode instructions.
    ParseWasmInstructions = 23,
    // Cost of parsing a known number of wasm functions.
    ParseWasmFunctions = 24,
    // Cost of parsing a known number of wasm globals.
    ParseWasmGlobals = 25,
    // Cost of parsing a known number of wasm table entries.
    ParseWasmTableEntries = 26,
    // Cost of parsing a known number of wasm types.
    ParseWasmTypes = 27,
    // Cost of parsing a known number of wasm data segments.
    ParseWasmDataSegments = 28,
    // Cost of parsing a known number of wasm element segments.
    ParseWasmElemSegments = 29,
    // Cost of parsing a known number of wasm imports.
    ParseWasmImports = 30,
    // Cost of parsing a known number of wasm exports.
    ParseWasmExports = 31,
    // Cost of parsing a known number of data segment bytes.
    ParseWasmDataSegmentBytes = 32,

    // Cost of instantiating wasm bytes that only encode instructions.
    InstantiateWasmInstructions = 33,
    // Cost of instantiating a known number of wasm functions.
    InstantiateWasmFunctions = 34,
    // Cost of instantiating a known number of wasm globals.
    InstantiateWasmGlobals = 35,
    // Cost of instantiating a known number of wasm table entries.
    InstantiateWasmTableEntries = 36,
    // Cost of instantiating a known number of wasm types.
    InstantiateWasmTypes = 37,
    // Cost of instantiating a known number of wasm data segments.
    InstantiateWasmDataSegments = 38,
    // Cost of instantiating a known number of wasm element segments.
    InstantiateWasmElemSegments = 39,
    // Cost of instantiating a known number of wasm imports.
    InstantiateWasmImports = 40,
    // Cost of instantiating a known number of wasm exports.
    InstantiateWasmExports = 41,
    // Cost of instantiating a known number of data segment bytes.
    InstantiateWasmDataSegmentBytes = 42,

    // Cost of decoding a bytes array representing an uncompressed SEC-1 encoded
    // point on a 256-bit elliptic curve
    Sec1DecodePointUncompressed = 43,
    // Cost of verifying an ECDSA Secp256r1 signature
    VerifyEcdsaSecp256r1Sig = 44,

    // Cost of encoding a BLS12-381 Fp (base field element)
    Bls12381EncodeFp = 45,
    // Cost of decoding a BLS12-381 Fp (base field element)
    Bls12381DecodeFp = 46,
    // Cost of checking a G1 point lies on the curve
    Bls12381G1CheckPointOnCurve = 47,
    // Cost of checking a G1 point belongs to the correct subgroup
    Bls12381G1CheckPointInSubgroup = 48,
    // Cost of checking a G2 point lies on the curve
    Bls12381G2CheckPointOnCurve = 49,
    // Cost of checking a G2 point belongs to the correct subgroup
    Bls12381G2CheckPointInSubgroup = 50,
    // Cost of converting a BLS12-381 G1 point from projective to affine coordinates
    Bls12381G1ProjectiveToAffine = 51,
    // Cost of converting a BLS12-381 G2 point from projective to affine coordinates
    Bls12381G2ProjectiveToAffine = 52,
    // Cost of performing BLS12-381 G1 point addition
    Bls12381G1Add = 53,
    // Cost of performing BLS12-381 G1 scalar multiplication
    Bls12381G1Mul = 54,
    // Cost of performing BLS12-381 G1 multi-scalar multiplication (MSM)
    Bls12381G1Msm = 55,
    // Cost of mapping a BLS12-381 Fp field element to a G1 point
    Bls12381MapFpToG1 = 56,
    // Cost of hashing to a BLS12-381 G1 point
    Bls12381HashToG1 = 57,
    // Cost of performing BLS12-381 G2 point addition
    Bls12381G2Add = 58,
    // Cost of performing BLS12-381 G2 scalar multiplication
    Bls12381G2Mul = 59,
    // Cost of performing BLS12-381 G2 multi-scalar multiplication (MSM)
    Bls12381G2Msm = 60,
    // Cost of mapping a BLS12-381 Fp2 field element to a G2 point
    Bls12381MapFp2ToG2 = 61,
    // Cost of hashing to a BLS12-381 G2 point
    Bls12381HashToG2 = 62,
    // Cost of performing BLS12-381 pairing operation
    Bls12381Pairing = 63,
    // Cost of converting a BLS12-381 scalar element from U256
    Bls12381FrFromU256 = 64,
    // Cost of converting a BLS12-381 scalar element to U256
    Bls12381FrToU256 = 65,
    // Cost of performing BLS12-381 scalar element addition/subtraction
    Bls12381FrAddSub = 66,
    // Cost of performing BLS12-381 scalar element multiplication
    Bls12381FrMul = 67,
    // Cost of performing BLS12-381 scalar element exponentiation
    Bls12381FrPow = 68,
    // Cost of performing BLS12-381 scalar element inversion
    Bls12381FrInv = 69,

    // Cost of encoding a BN254 Fp (base field element)
    Bn254EncodeFp = 70,
    // Cost of decoding a BN254 Fp (base field element)
    Bn254DecodeFp = 71,
    // Cost of checking a G1 point lies on the curve
    Bn254G1CheckPointOnCurve = 72,
    // Cost of checking a G2 point lies on the curve
    Bn254G2CheckPointOnCurve = 73,
    // Cost of checking a G2 point belongs to the correct subgroup
    Bn254G2CheckPointInSubgroup = 74,
    // Cost of converting a BN254 G1 point from projective to affine coordinates
    Bn254G1ProjectiveToAffine = 75,
    // Cost of performing BN254 G1 point addition
    Bn254G1Add = 76,
    // Cost of performing BN254 G1 scalar multiplication
    Bn254G1Mul = 77,
    // Cost of performing BN254 pairing operation
    Bn254Pairing = 78,
    // Cost of converting a BN254 scalar element from U256
    Bn254FrFromU256 = 79,
    // Cost of converting a BN254 scalar element to U256
    Bn254FrToU256 = 80,
    // // Cost of performing BN254 scalar element addition/subtraction
    Bn254FrAddSub = 81,
    // Cost of performing BN254 scalar element multiplication
    Bn254FrMul = 82,
    // Cost of performing BN254 scalar element exponentiation
    Bn254FrPow = 83,
     // Cost of performing BN254 scalar element inversion
    Bn254FrInv = 84
};

struct ContractCostParamEntry {
    // use `ext` to add more terms (e.g. higher order polynomials) in the future
    ExtensionPoint ext;

    int64 constTerm;
    int64 linearTerm;
};

struct StateArchivalSettings {
    uint32 maxEntryTTL;
    uint32 minTemporaryTTL;
    uint32 minPersistentTTL;

    // rent_fee = wfee_rate_average / rent_rate_denominator_for_type
    int64 persistentRentRateDenominator;
    int64 tempRentRateDenominator;

    // max number of entries that emit archival meta in a single ledger
    uint32 maxEntriesToArchive;

    // Number of snapshots to use when calculating average live Soroban State size
    uint32 liveSorobanStateSizeWindowSampleSize;

    // How often to sample the live Soroban State size for the average, in ledgers
    uint32 liveSorobanStateSizeWindowSamplePeriod;

    // Maximum number of bytes that we scan for eviction per ledger
    uint32 evictionScanSize;

    // Lowest BucketList level to be scanned to evict entries
    uint32 startingEvictionScanLevel;
};

struct EvictionIterator {
    uint32 bucketListLevel;
    bool isCurrBucket;
    uint64 bucketFileOffset;
};

struct ConfigSettingSCPTiming {
    uint32 ledgerTargetCloseTimeMilliseconds;
    uint32 nominationTimeoutInitialMilliseconds;
    uint32 nominationTimeoutIncrementMilliseconds;
    uint32 ballotTimeoutInitialMilliseconds;
    uint32 ballotTimeoutIncrementMilliseconds;
};

// limits the ContractCostParams size to 20kB
const CONTRACT_COST_COUNT_LIMIT = 1024;

typedef ContractCostParamEntry ContractCostParams<CONTRACT_COST_COUNT_LIMIT>;

// Identifiers of all the network settings.
enum ConfigSettingID
{
    CONFIG_SETTING_CONTRACT_MAX_SIZE_BYTES = 0,
    CONFIG_SETTING_CONTRACT_COMPUTE_V0 = 1,
    CONFIG_SETTING_CONTRACT_LEDGER_COST_V0 = 2,
    CONFIG_SETTING_CONTRACT_HISTORICAL_DATA_V0 = 3,
    CONFIG_SETTING_CONTRACT_EVENTS_V0 = 4,
    CONFIG_SETTING_CONTRACT_BANDWIDTH_V0 = 5,
    CONFIG_SETTING_CONTRACT_COST_PARAMS_CPU_INSTRUCTIONS = 6,
    CONFIG_SETTING_CONTRACT_COST_PARAMS_MEMORY_BYTES = 7,
    CONFIG_SETTING_CONTRACT_DATA_KEY_SIZE_BYTES = 8,
    CONFIG_SETTING_CONTRACT_DATA_ENTRY_SIZE_BYTES = 9,
    CONFIG_SETTING_STATE_ARCHIVAL = 10,
    CONFIG_SETTING_CONTRACT_EXECUTION_LANES = 11,
    CONFIG_SETTING_LIVE_SOROBAN_STATE_SIZE_WINDOW = 12,
    CONFIG_SETTING_EVICTION_ITERATOR = 13,
    CONFIG_SETTING_CONTRACT_PARALLEL_COMPUTE_V0 = 14,
    CONFIG_SETTING_CONTRACT_LEDGER_COST_EXT_V0 = 15,
    CONFIG_SETTING_SCP_TIMING = 16
};

union ConfigSettingEntry switch (ConfigSettingID configSettingID)
{
case CONFIG_SETTING_CONTRACT_MAX_SIZE_BYTES:
    uint32 contractMaxSizeBytes;
case CONFIG_SETTING_CONTRACT_COMPUTE_V0:
    ConfigSettingContractComputeV0 contractCompute;
case CONFIG_SETTING_CONTRACT_LEDGER_COST_V0:
    ConfigSettingContractLedgerCostV0 contractLedgerCost;
case CONFIG_SETTING_CONTRACT_HISTORICAL_DATA_V0:
    ConfigSettingContractHistoricalDataV0 contractHistoricalData;
case CONFIG_SETTING_CONTRACT_EVENTS_V0:
    ConfigSettingContractEventsV0 contractEvents;
case CONFIG_SETTING_CONTRACT_BANDWIDTH_V0:
    ConfigSettingContractBandwidthV0 contractBandwidth;
case CONFIG_SETTING_CONTRACT_COST_PARAMS_CPU_INSTRUCTIONS:
    ContractCostParams contractCostParamsCpuInsns;
case CONFIG_SETTING_CONTRACT_COST_PARAMS_MEMORY_BYTES:
    ContractCostParams contractCostParamsMemBytes;
case CONFIG_SETTING_CONTRACT_DATA_KEY_SIZE_BYTES:
    uint32 contractDataKeySizeBytes;
case CONFIG_SETTING_CONTRACT_DATA_ENTRY_SIZE_BYTES:
    uint32 contractDataEntrySizeBytes;
case CONFIG_SETTING_STATE_ARCHIVAL:
    StateArchivalSettings stateArchivalSettings;
case CONFIG_SETTING_CONTRACT_EXECUTION_LANES:
    ConfigSettingContractExecutionLanesV0 contractExecutionLanes;
case CONFIG_SETTING_LIVE_SOROBAN_STATE_SIZE_WINDOW:
    uint64 liveSorobanStateSizeWindow<>;
case CONFIG_SETTING_EVICTION_ITERATOR:
    EvictionIterator evictionIterator;
case CONFIG_SETTING_CONTRACT_PARALLEL_COMPUTE_V0:
    ConfigSettingContractParallelComputeV0 contractParallelCompute;
case CONFIG_SETTING_CONTRACT_LEDGER_COST_EXT_V0:
    ConfigSettingContractLedgerCostExtV0 contractLedgerCostExt;
case CONFIG_SETTING_SCP_TIMING:
    ConfigSettingSCPTiming contractSCPTiming;
};
}
