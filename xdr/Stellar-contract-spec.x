// Copyright 2022 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

// The contract Contractspec XDR is highly experimental, incomplete, and still being
// iterated on. Breaking changes expected.

% #include "xdr/Stellar-types.h"
% #include "xdr/Stellar-contract.h"
namespace stellar
{

const SC_SPEC_DOC_LIMIT = 1024;

enum SCSpecType
{
    SC_SPEC_TYPE_VAL = 0,

    // Types with no parameters.
    SC_SPEC_TYPE_BOOL = 1,
    SC_SPEC_TYPE_VOID = 2,
    SC_SPEC_TYPE_ERROR = 3,
    SC_SPEC_TYPE_U32 = 4,
    SC_SPEC_TYPE_I32 = 5,
    SC_SPEC_TYPE_U64 = 6,
    SC_SPEC_TYPE_I64 = 7,
    SC_SPEC_TYPE_TIMEPOINT = 8,
    SC_SPEC_TYPE_DURATION = 9,
    SC_SPEC_TYPE_U128 = 10,
    SC_SPEC_TYPE_I128 = 11,
    SC_SPEC_TYPE_U256 = 12,
    SC_SPEC_TYPE_I256 = 13,
    SC_SPEC_TYPE_BYTES = 14,
    SC_SPEC_TYPE_STRING = 16,
    SC_SPEC_TYPE_SYMBOL = 17,
    SC_SPEC_TYPE_ADDRESS = 19,
    SC_SPEC_TYPE_MUXED_ADDRESS = 20,

    // Types with parameters.
    SC_SPEC_TYPE_OPTION = 1000,
    SC_SPEC_TYPE_RESULT = 1001,
    SC_SPEC_TYPE_VEC = 1002,
    SC_SPEC_TYPE_MAP = 1004,
    SC_SPEC_TYPE_TUPLE = 1005,
    SC_SPEC_TYPE_BYTES_N = 1006,

    // User defined types.
    SC_SPEC_TYPE_UDT = 2000
};

struct SCSpecTypeOption
{
    SCSpecTypeDef valueType;
};

struct SCSpecTypeResult
{
    SCSpecTypeDef okType;
    SCSpecTypeDef errorType;
};

struct SCSpecTypeVec
{
    SCSpecTypeDef elementType;
};

struct SCSpecTypeMap
{
    SCSpecTypeDef keyType;
    SCSpecTypeDef valueType;
};

struct SCSpecTypeTuple
{
    SCSpecTypeDef valueTypes<12>;
};

struct SCSpecTypeBytesN
{
    uint32 n;
};

struct SCSpecTypeUDT
{
    string name<60>;
};

union SCSpecTypeDef switch (SCSpecType type)
{
case SC_SPEC_TYPE_VAL:
case SC_SPEC_TYPE_BOOL:
case SC_SPEC_TYPE_VOID:
case SC_SPEC_TYPE_ERROR:
case SC_SPEC_TYPE_U32:
case SC_SPEC_TYPE_I32:
case SC_SPEC_TYPE_U64:
case SC_SPEC_TYPE_I64:
case SC_SPEC_TYPE_TIMEPOINT:
case SC_SPEC_TYPE_DURATION:
case SC_SPEC_TYPE_U128:
case SC_SPEC_TYPE_I128:
case SC_SPEC_TYPE_U256:
case SC_SPEC_TYPE_I256:
case SC_SPEC_TYPE_BYTES:
case SC_SPEC_TYPE_STRING:
case SC_SPEC_TYPE_SYMBOL:
case SC_SPEC_TYPE_ADDRESS:
case SC_SPEC_TYPE_MUXED_ADDRESS:
    void;
case SC_SPEC_TYPE_OPTION:
    SCSpecTypeOption option;
case SC_SPEC_TYPE_RESULT:
    SCSpecTypeResult result;
case SC_SPEC_TYPE_VEC:
    SCSpecTypeVec vec;
case SC_SPEC_TYPE_MAP:
    SCSpecTypeMap map;
case SC_SPEC_TYPE_TUPLE:
    SCSpecTypeTuple tuple;
case SC_SPEC_TYPE_BYTES_N:
    SCSpecTypeBytesN bytesN;
case SC_SPEC_TYPE_UDT:
    SCSpecTypeUDT udt;
};

struct SCSpecUDTStructFieldV0
{
    string doc<SC_SPEC_DOC_LIMIT>;
    string name<30>;
    SCSpecTypeDef type;
};

struct SCSpecUDTStructV0
{
    string doc<SC_SPEC_DOC_LIMIT>;
    string lib<80>;
    string name<60>;
    SCSpecUDTStructFieldV0 fields<>;
};

struct SCSpecUDTUnionCaseVoidV0
{
    string doc<SC_SPEC_DOC_LIMIT>;
    string name<60>;
};

struct SCSpecUDTUnionCaseTupleV0
{
    string doc<SC_SPEC_DOC_LIMIT>;
    string name<60>;
    SCSpecTypeDef type<>;
};

enum SCSpecUDTUnionCaseV0Kind
{
    SC_SPEC_UDT_UNION_CASE_VOID_V0 = 0,
    SC_SPEC_UDT_UNION_CASE_TUPLE_V0 = 1
};

union SCSpecUDTUnionCaseV0 switch (SCSpecUDTUnionCaseV0Kind kind)
{
case SC_SPEC_UDT_UNION_CASE_VOID_V0:
    SCSpecUDTUnionCaseVoidV0 voidCase;
case SC_SPEC_UDT_UNION_CASE_TUPLE_V0:
    SCSpecUDTUnionCaseTupleV0 tupleCase;
};

struct SCSpecUDTUnionV0
{
    string doc<SC_SPEC_DOC_LIMIT>;
    string lib<80>;
    string name<60>;
    SCSpecUDTUnionCaseV0 cases<>;
};

struct SCSpecUDTEnumCaseV0
{
    string doc<SC_SPEC_DOC_LIMIT>;
    string name<60>;
    uint32 value;
};

struct SCSpecUDTEnumV0
{
    string doc<SC_SPEC_DOC_LIMIT>;
    string lib<80>;
    string name<60>;
    SCSpecUDTEnumCaseV0 cases<>;
};

struct SCSpecUDTErrorEnumCaseV0
{
    string doc<SC_SPEC_DOC_LIMIT>;
    string name<60>;
    uint32 value;
};

struct SCSpecUDTErrorEnumV0
{
    string doc<SC_SPEC_DOC_LIMIT>;
    string lib<80>;
    string name<60>;
    SCSpecUDTErrorEnumCaseV0 cases<>;
};

struct SCSpecFunctionInputV0
{
    string doc<SC_SPEC_DOC_LIMIT>;
    string name<30>;
    SCSpecTypeDef type;
};

struct SCSpecFunctionV0
{
    string doc<SC_SPEC_DOC_LIMIT>;
    SCSymbol name;
    SCSpecFunctionInputV0 inputs<>;
    SCSpecTypeDef outputs<1>;
};

enum SCSpecEventParamLocationV0
{
    SC_SPEC_EVENT_PARAM_LOCATION_DATA = 0,
    SC_SPEC_EVENT_PARAM_LOCATION_TOPIC_LIST = 1
};

struct SCSpecEventParamV0
{
    string doc<SC_SPEC_DOC_LIMIT>;
    string name<30>;
    SCSpecTypeDef type;
    SCSpecEventParamLocationV0 location;
};

enum SCSpecEventDataFormat
{
    SC_SPEC_EVENT_DATA_FORMAT_SINGLE_VALUE = 0,
    SC_SPEC_EVENT_DATA_FORMAT_VEC = 1,
    SC_SPEC_EVENT_DATA_FORMAT_MAP = 2
};

struct SCSpecEventV0
{
    string doc<SC_SPEC_DOC_LIMIT>;
    string lib<80>;
    SCSymbol name;
    SCSymbol prefixTopics<2>;
    SCSpecEventParamV0 params<>;
    SCSpecEventDataFormat dataFormat;
};

enum SCSpecEntryKind
{
    SC_SPEC_ENTRY_FUNCTION_V0 = 0,
    SC_SPEC_ENTRY_UDT_STRUCT_V0 = 1,
    SC_SPEC_ENTRY_UDT_UNION_V0 = 2,
    SC_SPEC_ENTRY_UDT_ENUM_V0 = 3,
    SC_SPEC_ENTRY_UDT_ERROR_ENUM_V0 = 4,
    SC_SPEC_ENTRY_EVENT_V0 = 5
};

union SCSpecEntry switch (SCSpecEntryKind kind)
{
case SC_SPEC_ENTRY_FUNCTION_V0:
    SCSpecFunctionV0 functionV0;
case SC_SPEC_ENTRY_UDT_STRUCT_V0:
    SCSpecUDTStructV0 udtStructV0;
case SC_SPEC_ENTRY_UDT_UNION_V0:
    SCSpecUDTUnionV0 udtUnionV0;
case SC_SPEC_ENTRY_UDT_ENUM_V0:
    SCSpecUDTEnumV0 udtEnumV0;
case SC_SPEC_ENTRY_UDT_ERROR_ENUM_V0:
    SCSpecUDTErrorEnumV0 udtErrorEnumV0;
case SC_SPEC_ENTRY_EVENT_V0:
    SCSpecEventV0 eventV0;
};

}
