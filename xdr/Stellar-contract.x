// Copyright 2022 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

% #include "xdr/Stellar-types.h"
namespace stellar
{

typedef opaque SCBytes<>;
typedef string SCString<>;
const SCSYMBOL_LIMIT = 32;
typedef string SCSymbol<SCSYMBOL_LIMIT>;

// We fix a maximum of 128 value types in the system for two reasons: we want to
// keep the codes relatively small (<= 8 bits) when bit-packing values into a
// u64 at the environment interface level, so that we keep many bits for
// payloads (small strings, small numeric values, object handles); and then we
// actually want to go one step further and ensure (for code-size) that our
// codes fit in a single ULEB128-code byte, which means we can only use 7 bits.
//
// We also reserve several type codes from this space because we want to _reuse_
// the SCValType codes at the environment interface level (or at least not
// exceed its number-space) but there are more types at that level, assigned to
// optimizations/special case representations of values abstract at this level.

enum SCValType
{
    SCV_BOOL = 0,
    SCV_VOID = 1,
    SCV_ERROR = 2,

    // 32 bits is the smallest type in WASM or XDR; no need for u8/u16.
    SCV_U32 = 3,
    SCV_I32 = 4,

    // 64 bits is naturally supported by both WASM and XDR also.
    SCV_U64 = 5,
    SCV_I64 = 6,

    // Time-related u64 subtypes with their own functions and formatting.
    SCV_TIMEPOINT = 7,
    SCV_DURATION = 8,

    // 128 bits is naturally supported by Rust and we use it for Soroban
    // fixed-point arithmetic prices / balances / similar "quantities". These
    // are represented in XDR as a pair of 2 u64s.
    SCV_U128 = 9,
    SCV_I128 = 10,

    // 256 bits is the size of sha256 output, ed25519 keys, and the EVM machine
    // word, so for interop use we include this even though it requires a small
    // amount of Rust guest and/or host library code.
    SCV_U256 = 11,
    SCV_I256 = 12,

    // Bytes come in 3 flavors, 2 of which have meaningfully different
    // formatting and validity-checking / domain-restriction.
    SCV_BYTES = 13,
    SCV_STRING = 14,
    SCV_SYMBOL = 15,

    // Vecs and maps are just polymorphic containers of other ScVals.
    SCV_VEC = 16,
    SCV_MAP = 17,

    // Address is the universal identifier for contracts and classic
    // accounts.
    SCV_ADDRESS = 18,

    // The following are the internal SCVal variants that are not
    // exposed to the contracts. 
    SCV_CONTRACT_INSTANCE = 19,

    // SCV_LEDGER_KEY_CONTRACT_INSTANCE and SCV_LEDGER_KEY_NONCE are unique
    // symbolic SCVals used as the key for ledger entries for a contract's
    // instance and an address' nonce, respectively.
    SCV_LEDGER_KEY_CONTRACT_INSTANCE = 20,
    SCV_LEDGER_KEY_NONCE = 21,

    SCV_EXECUTABLE_TAG = 22
};

enum SCErrorType
{
    SCE_CONTRACT = 0,          // Contract-specific, user-defined codes.
    SCE_WASM_VM = 1,           // Errors while interpreting WASM bytecode.
    SCE_CONTEXT = 2,           // Errors in the contract's host context.
    SCE_STORAGE = 3,           // Errors accessing host storage.
    SCE_OBJECT = 4,            // Errors working with host objects.
    SCE_CRYPTO = 5,            // Errors in cryptographic operations.
    SCE_EVENTS = 6,            // Errors while emitting events.
    SCE_BUDGET = 7,            // Errors relating to budget limits.
    SCE_VALUE = 8,             // Errors working with host values or SCVals.
    SCE_AUTH = 9               // Errors from the authentication subsystem.
};

enum SCErrorCode
{
    SCEC_ARITH_DOMAIN = 0,      // Some arithmetic was undefined (overflow, divide-by-zero).
    SCEC_INDEX_BOUNDS = 1,      // Something was indexed beyond its bounds.
    SCEC_INVALID_INPUT = 2,     // User provided some otherwise-bad data.
    SCEC_MISSING_VALUE = 3,     // Some value was required but not provided.
    SCEC_EXISTING_VALUE = 4,    // Some value was provided where not allowed.
    SCEC_EXCEEDED_LIMIT = 5,    // Some arbitrary limit -- gas or otherwise -- was hit.
    SCEC_INVALID_ACTION = 6,    // Data was valid but action requested was not.
    SCEC_INTERNAL_ERROR = 7,    // The host detected an error in its own logic.
    SCEC_UNEXPECTED_TYPE = 8,   // Some type wasn't as expected.
    SCEC_UNEXPECTED_SIZE = 9    // Something's size wasn't as expected.
};

// Smart contract errors are split into a type (SCErrorType) and a code. When an
// error is of type SCE_CONTRACT it carries a user-defined uint32 code that
// Soroban assigns no specific meaning to. In all other cases, the type
// specifies a subsystem of the Soroban host where the error originated, and the
// accompanying code is an SCErrorCode, each of which specifies a slightly more
// precise class of errors within that subsystem.
//
// Error types and codes are not maximally precise; there is a tradeoff between
// precision and flexibility in the implementation, and the granularity here is
// chosen to be adequate for most purposes while not placing a burden on future
// system evolution and maintenance. When additional precision is needed for
// debugging, Soroban can be run with diagnostic events enabled.

union SCError switch (SCErrorType type)
{
case SCE_CONTRACT:
    uint32 contractCode;
case SCE_WASM_VM:
case SCE_CONTEXT:
case SCE_STORAGE:
case SCE_OBJECT:
case SCE_CRYPTO:
case SCE_EVENTS:
case SCE_BUDGET:
case SCE_VALUE:
case SCE_AUTH:
    SCErrorCode code;
};

struct UInt128Parts {
    uint64 hi;
    uint64 lo;
};

// A signed int128 has a high sign bit and 127 value bits. We break it into a
// signed high int64 (that carries the sign bit and the high 63 value bits) and
// a low unsigned uint64 that carries the low 64 bits. This will sort in
// generated code in the same order the underlying int128 sorts.
struct Int128Parts {
    int64 hi;
    uint64 lo;
};

struct UInt256Parts {
    uint64 hi_hi;
    uint64 hi_lo;
    uint64 lo_hi;
    uint64 lo_lo;
};

// A signed int256 has a high sign bit and 255 value bits. We break it into a
// signed high int64 (that carries the sign bit and the high 63 value bits) and
// three low unsigned `uint64`s that carry the lower bits. This will sort in
// generated code in the same order the underlying int256 sorts.
struct Int256Parts {
    int64 hi_hi;
    uint64 hi_lo;
    uint64 lo_hi;
    uint64 lo_lo;
};

enum ContractExecutableType
{
    CONTRACT_EXECUTABLE_WASM = 0,
    CONTRACT_EXECUTABLE_STELLAR_ASSET = 1,
    CONTRACT_EXECUTABLE_EXTERNAL_REF = 2
};

enum SCAddressType
{
    SC_ADDRESS_TYPE_ACCOUNT = 0,
    SC_ADDRESS_TYPE_CONTRACT = 1,
    SC_ADDRESS_TYPE_MUXED_ACCOUNT = 2,
    SC_ADDRESS_TYPE_CLAIMABLE_BALANCE = 3,
    SC_ADDRESS_TYPE_LIQUIDITY_POOL = 4
};

struct MuxedEd25519Account
{
    uint64 id;
    uint256 ed25519;
};


union SCAddress switch (SCAddressType type)
{
case SC_ADDRESS_TYPE_ACCOUNT:
    AccountID accountId;
case SC_ADDRESS_TYPE_CONTRACT:
    ContractID contractId;
case SC_ADDRESS_TYPE_MUXED_ACCOUNT:
    MuxedEd25519Account muxedAccount;
case SC_ADDRESS_TYPE_CLAIMABLE_BALANCE:
    ClaimableBalanceID claimableBalanceId;
case SC_ADDRESS_TYPE_LIQUIDITY_POOL:
    PoolID liquidityPoolId;
};

struct ContractExecutableExternalRef {
    SCAddress executable_owner;
    SCString tag;
};

union ContractExecutable switch (ContractExecutableType type)
{
case CONTRACT_EXECUTABLE_WASM:
    Hash wasm_hash;
case CONTRACT_EXECUTABLE_STELLAR_ASSET:
    void;
case CONTRACT_EXECUTABLE_EXTERNAL_REF:
    ContractExecutableExternalRef external_ref;
};

%struct SCVal;
%struct SCMapEntry;

typedef SCVal SCVec<>;
typedef SCMapEntry SCMap<>;

struct SCNonceKey {
    int64 nonce;
};

struct SCContractInstance {
    ContractExecutable executable;
    SCMap* storage;
};

union SCVal switch (SCValType type)
{

case SCV_BOOL:
    bool b;
case SCV_VOID:
    void;
case SCV_ERROR:
    SCError error;

case SCV_U32:
    uint32 u32;
case SCV_I32:
    int32 i32;

case SCV_U64:
    uint64 u64;
case SCV_I64:
    int64 i64;
case SCV_TIMEPOINT:
    TimePoint timepoint;
case SCV_DURATION:
    Duration duration;

case SCV_U128:
    UInt128Parts u128;
case SCV_I128:
    Int128Parts i128;

case SCV_U256:
    UInt256Parts u256;
case SCV_I256:
    Int256Parts i256;

case SCV_BYTES:
    SCBytes bytes;
case SCV_STRING:
    SCString str;
case SCV_SYMBOL:
    SCSymbol sym;

// Vec and Map are recursive so need to live
// behind an option, due to xdrpp limitations.
case SCV_VEC:
    SCVec *vec;
case SCV_MAP:
    SCMap *map;

case SCV_ADDRESS:
    SCAddress address;

// Special SCVals reserved for system-constructed contract-data
// ledger keys, not generally usable elsewhere.
case SCV_CONTRACT_INSTANCE:
    SCContractInstance instance;
case SCV_LEDGER_KEY_CONTRACT_INSTANCE:
    void;
case SCV_LEDGER_KEY_NONCE:
    SCNonceKey nonce_key;

case SCV_EXECUTABLE_TAG:
    SCString executable_tag;
};

struct SCMapEntry
{
    SCVal key;
    SCVal val;
};

}
