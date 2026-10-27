// Package main implements an XDR-to-Go code generator.
// It reads a JSON IR produced by the Rust XDR parser (rs-stellar-xdr)
// and generates Go zero-copy view types.
package main

import (
	"encoding/json"
	"fmt"
)

// IR discriminator constants use snake_case to match the Rust serde output.

// TypeRefKind constants for XDR type references.
const (
	TRInt           = "int"
	TRUnsignedInt   = "unsigned_int"
	TRHyper         = "hyper"
	TRUnsignedHyper = "unsigned_hyper"
	TRFloat         = "float"
	TRDouble        = "double"
	TRBool          = "bool"
	TROpaqueFixed   = "opaque_fixed"
	TROpaqueVar     = "opaque_var"
	TRString        = "string"
	TRRef           = "ref"
	TROptional      = "optional"
	TRArray         = "array"
	TRVarArray      = "var_array"
)

// DefKind constants for definition types.
const (
	DKStruct  = "struct"
	DKUnion   = "union"
	DKEnum    = "enum"
	DKTypedef = "typedef"
	DKConst   = "const"
)

// IR represents the complete intermediate representation from the Rust parser.
type IR struct {
	Version          int       `json:"version"`
	ResolvedFeatures []string  `json:"resolved_features"`
	Files            []XdrFile `json:"files"`
	Definitions      []DefWrap `json:"definitions"`
}

// XdrFile metadata.
type XdrFile struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

// DefWrap is a JSON-deserialized definition. The Rust IR uses serde's
// internally-tagged encoding: {"kind": "struct", "name": "Foo", ...}.
type DefWrap struct {
	Kind      string  `json:"kind"`
	FixedSize *uint32 `json:"fixed_size"`
	Struct    *StructDef
	Union     *UnionDef
	Enum      *EnumDef
	Typedef   *TypedefDef
	Const     *ConstDef
}

// UnmarshalJSON handles the internally-tagged Definition format.
func (d *DefWrap) UnmarshalJSON(data []byte) error {
	var peek struct {
		Kind      string  `json:"kind"`
		FixedSize *uint32 `json:"fixed_size"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return fmt.Errorf("DefWrap: %w", err)
	}
	d.Kind = peek.Kind
	d.FixedSize = peek.FixedSize
	switch peek.Kind {
	case DKStruct:
		d.Struct = &StructDef{}
		return json.Unmarshal(data, d.Struct)
	case DKUnion:
		d.Union = &UnionDef{}
		return json.Unmarshal(data, d.Union)
	case DKEnum:
		d.Enum = &EnumDef{}
		return json.Unmarshal(data, d.Enum)
	case DKTypedef:
		d.Typedef = &TypedefDef{}
		return json.Unmarshal(data, d.Typedef)
	case DKConst:
		d.Const = &ConstDef{}
		return json.Unmarshal(data, d.Const)
	default:
		return fmt.Errorf("DefWrap: unknown kind %q", peek.Kind)
	}
}

// Name returns the definition's name. Panics if Kind is not one of the
// enumerated DK* constants — this is a contract violation indicating either
// corrupted IR input or a schema evolution that added a kind without updating
// this switch. Callers should assume any DefWrap produced by UnmarshalJSON
// has a recognized Kind; if you construct DefWrap manually, set Kind first.
func (d *DefWrap) Name() string {
	switch d.Kind {
	case DKStruct:
		return d.Struct.Name
	case DKUnion:
		return d.Union.Name
	case DKEnum:
		return d.Enum.Name
	case DKTypedef:
		return d.Typedef.Name
	case DKConst:
		return d.Const.Name
	}
	panic(fmt.Sprintf("DefWrap.Name: unknown kind %q", d.Kind))
}

// StructDef is an XDR struct definition.
type StructDef struct {
	Name   string        `json:"name"`
	Fields []StructField `json:"fields"`
}

// StructField is a field within a struct or a union discriminant.
type StructField struct {
	Name string  `json:"name"`
	Type TypeRef `json:"type"`
}

// UnionDef is an XDR union definition.
type UnionDef struct {
	Name         string      `json:"name"`
	Discriminant StructField `json:"discriminant"`
	Arms         []UnionArm  `json:"arms"`
}

// UnionArm is a single arm of a union (one or more cases).
type UnionArm struct {
	Cases []UnionCase `json:"cases"`
	Name  string      `json:"name"`
	Type  *TypeRef    `json:"type,omitempty"`
}

// UnionCase is a single case value within a union arm.
type UnionCase struct {
	Value int64  `json:"value"`
	Name  string `json:"name,omitempty"`
}

// EnumDef is an XDR enum definition.
type EnumDef struct {
	Name         string       `json:"name"`
	MemberPrefix string       `json:"member_prefix"`
	Members      []EnumMember `json:"members"`
}

// EnumMember is a value within an enum.
type EnumMember struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

// TypedefDef is an XDR typedef definition.
type TypedefDef struct {
	Name string  `json:"name"`
	Type TypeRef `json:"type"`
}

// ConstDef is an XDR const definition.
type ConstDef struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

// TypeRef represents an XDR type reference.
// Flat struct with kind discriminator — standard json.Unmarshal handles it.
type TypeRef struct {
	Kind     string   `json:"kind"`
	Name     string   `json:"name,omitempty"`      // for "ref" kind
	Size     *uint64  `json:"size,omitempty"`      // for "opaque_fixed"
	MaxSize  *uint64  `json:"max_size,omitempty"`  // for "opaque_var", "string"
	Element  *TypeRef `json:"element,omitempty"`   // for "array", "var_array", "optional"
	Count    *uint64  `json:"count,omitempty"`     // for "array"
	MaxCount *uint64  `json:"max_count,omitempty"` // for "var_array"
}
