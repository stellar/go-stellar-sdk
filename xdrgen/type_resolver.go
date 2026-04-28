package main

import (
	"fmt"
	"math"
)

// TypeResolver maps definition names to their DefWrap entries and provides
// type resolution methods.
type TypeResolver map[string]*DefWrap

// NewTypeResolver creates a resolver from an IR.
func NewTypeResolver(ir *IR) TypeResolver {
	r := make(TypeResolver, len(ir.Definitions))
	for i := range ir.Definitions {
		d := &ir.Definitions[i]
		r[d.Name()] = d
	}
	return r
}

// resolveTypeRef follows typedef chains to the underlying type. Errors on a
// cyclic chain (e.g., `typedef A B; typedef B A;`) — the IR producer should
// reject these earlier, but checking here makes codegen robust to malformed
// input rather than infinite-looping.
func (r TypeResolver) resolveTypeRef(t *TypeRef) (*TypeRef, error) {
	visited := make(map[string]bool)
	for t.Kind == TRRef {
		if visited[t.Name] {
			return nil, fmt.Errorf("cyclic typedef chain through %q", t.Name)
		}
		visited[t.Name] = true
		def, ok := r[t.Name]
		if !ok || def.Kind != DKTypedef || def.Typedef == nil {
			break
		}
		t = &def.Typedef.Type
	}
	return t, nil
}

// ResolveViewType resolves a TypeRef into a ViewType, following typedef chains
// and renaming GoType to match typedef aliases.
func (r TypeResolver) ResolveViewType(t *TypeRef) (*ViewType, error) {
	if t.Kind != TRRef {
		return r.BuildViewType(t)
	}
	resolved, err := r.resolveTypeRef(t)
	if err != nil {
		return nil, fmt.Errorf("resolving %q: %w", t.Name, err)
	}
	vt, err := r.BuildViewType(resolved)
	if err != nil {
		return nil, fmt.Errorf("resolving %q: %w", t.Name, err)
	}
	if want := GoTypeName(t.Name) + "View"; vt.GoType != want {
		copy := *vt
		copy.GoType = want
		vt = &copy
	}
	return vt, nil
}

var scalarViewTypes = map[string]struct {
	size uint32
	name string
}{
	TRInt:           {4, "Int32View"},
	TRUnsignedInt:   {4, "Uint32View"},
	TRHyper:         {8, "Int64View"},
	TRUnsignedHyper: {8, "Uint64View"},
	TRBool:          {4, "BoolView"},
	TRFloat:         {4, "Float32View"},
	TRDouble:        {8, "Float64View"},
}

// sizeVal converts an XDR size/count value (encoded in the JSON IR as uint64
// for permissive parsing) to uint32. Errors if the value would overflow,
// which would indicate a malformed IR — XDR sizes are uint32 per RFC 4506.
func sizeVal(p *uint64) (uint32, error) {
	if p == nil {
		return 0, nil
	}
	if *p > math.MaxUint32 {
		return 0, fmt.Errorf("size %d exceeds uint32 max", *p)
	}
	return uint32(*p), nil
}

// BuildViewType maps a resolved TypeRef to a ViewType.
func (r TypeResolver) BuildViewType(resolved *TypeRef) (*ViewType, error) {
	if s, ok := scalarViewTypes[resolved.Kind]; ok {
		return &ViewType{Kind: VKScalar, fixedSize: ptrSize(s.size), GoType: s.name}, nil
	}
	switch resolved.Kind {
	case TROpaqueFixed:
		raw, err := sizeVal(resolved.Size)
		if err != nil {
			return nil, fmt.Errorf("opaque fixed size: %w", err)
		}
		padded := (raw + 3) &^ 3
		return &ViewType{Kind: VKOpaque, fixedSize: ptrSize(padded), GoType: "[]byte",
			Opaque: &OpaqueViewType{RawSize: raw}}, nil
	case TROpaqueVar, TRString:
		maxLen, err := sizeVal(resolved.MaxSize)
		if err != nil {
			return nil, fmt.Errorf("opaque var max size: %w", err)
		}
		return &ViewType{Kind: VKOpaque, GoType: "VarOpaqueView",
			Opaque: &OpaqueViewType{MaxLen: maxLen}}, nil
	case TRRef:
		def, ok := r[resolved.Name]
		if !ok {
			return nil, fmt.Errorf("unknown type %q", resolved.Name)
		}
		vn := GoTypeName(resolved.Name) + "View"
		switch def.Kind {
		case DKStruct, DKUnion:
			return &ViewType{Kind: VKNamed, fixedSize: def.FixedSize, GoType: vn,
				Named: &NamedViewType{XDRName: resolved.Name}}, nil
		case DKEnum:
			return &ViewType{Kind: VKNamed, fixedSize: ptrSize(4), GoType: vn,
				Named: &NamedViewType{XDRName: resolved.Name}}, nil
		default:
			return nil, fmt.Errorf("unhandled DefKind %q for ref %q", def.Kind, resolved.Name)
		}
	case TRArray:
		elem, err := r.ResolveViewType(resolved.Element)
		if err != nil {
			return nil, fmt.Errorf("array element: %w", err)
		}
		count, err := sizeVal(resolved.Count)
		if err != nil {
			return nil, fmt.Errorf("array count: %w", err)
		}
		var fixed *uint32
		if efs, ok := elem.FixedSize(); ok {
			fixed = ptrSize(efs * count)
		}
		return &ViewType{Kind: VKArray, fixedSize: fixed, GoType: "[]byte",
			Array: &ArrayViewType{Element: elem, Count: count}}, nil
	case TRVarArray:
		elem, err := r.ResolveViewType(resolved.Element)
		if err != nil {
			return nil, fmt.Errorf("var array element: %w", err)
		}
		maxLen, err := sizeVal(resolved.MaxCount)
		if err != nil {
			return nil, fmt.Errorf("var array max count: %w", err)
		}
		return &ViewType{Kind: VKArray, GoType: "[]byte",
			Array: &ArrayViewType{Element: elem, MaxLen: maxLen}}, nil
	case TROptional:
		elem, err := r.ResolveViewType(resolved.Element)
		if err != nil {
			return nil, fmt.Errorf("optional element: %w", err)
		}
		return &ViewType{Kind: VKOptional, GoType: elem.GoType + "Opt",
			Optional: &OptionalViewType{Element: elem}}, nil
	}
	return nil, fmt.Errorf("unhandled TypeRef kind %q", resolved.Kind)
}
