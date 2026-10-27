package main

import "fmt"

// ViewKind identifies the kind of a ViewType.
type ViewKind string

const (
	VKScalar   ViewKind = "scalar"
	VKOpaque   ViewKind = "opaque"
	VKArray    ViewKind = "array"
	VKOptional ViewKind = "optional"
	VKNamed    ViewKind = "named" // struct, union, or enum referenced by name
)

// OpaqueViewType holds fields specific to opaque types.
type OpaqueViewType struct {
	RawSize uint32 // Unpadded byte size (> 0 = fixed opaque).
	MaxLen  uint32 // Schema max length (> 0 = bounded variable opaque, 0 = unbounded).
}

// ArrayViewType holds fields specific to array types.
type ArrayViewType struct {
	Element *ViewType // Element type.
	Count   uint32    // Fixed element count (> 0 = fixed, 0 = variable).
	MaxLen  uint32    // Schema max length (variable arrays only, 0 = unbounded).
}

// OptionalViewType holds fields specific to optional types.
type OptionalViewType struct {
	Element *ViewType
}

// NamedViewType holds fields specific to named type references (struct/union/enum).
type NamedViewType struct {
	XDRName string // Original XDR type name for definition lookup.
}

func ptrSize(n uint32) *uint32 { return &n }

// ViewType describes a resolved XDR type for view code generation.
// Kind + exactly one per-kind pointer is set.
type ViewType struct {
	Kind      ViewKind
	GoType    string  // Go type name used everywhere: return types, method calls, casts.
	fixedSize *uint32 // nil if variable-size.

	Opaque   *OpaqueViewType
	Array    *ArrayViewType
	Optional *OptionalViewType
	Named    *NamedViewType
}

func (vt *ViewType) FixedSize() (uint32, bool) {
	if vt == nil || vt.fixedSize == nil {
		return 0, false
	}
	return *vt.fixedSize, true
}

// NeedsConcreteType reports whether this ViewType requires a dedicated concrete
// type to be emitted (as opposed to using an existing named type or alias).
func (vt *ViewType) NeedsConcreteType() bool {
	switch vt.Kind {
	case VKArray, VKOptional:
		return true
	case VKOpaque:
		return vt.Opaque.RawSize > 0 || vt.Opaque.MaxLen > 0
	}
	return false
}

// SubView returns a Go expression that constructs a sub-view starting at offExpr.
func (vt *ViewType) SubView(receiverVar, offExpr string) string {
	return fmt.Sprintf("%s(%s[%s:])", vt.GoType, receiverVar, offExpr)
}
