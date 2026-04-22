package main

import "fmt"

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

// resolveTypeRef follows typedef chains to the underlying type.
func (r TypeResolver) resolveTypeRef(t *TypeRef) *TypeRef {
	for t.Kind == TRRef {
		def, ok := r[t.Name]
		if !ok || def.Kind != DKTypedef || def.Typedef == nil {
			break
		}
		t = &def.Typedef.Type
	}
	return t
}

// ResolveViewType resolves a TypeRef into a ViewType, following typedef chains
// and renaming GoType to match typedef aliases.
func (r TypeResolver) ResolveViewType(t *TypeRef) (*ViewType, error) {
	if t.Kind != TRRef {
		return r.BuildViewType(t)
	}
	resolved := r.resolveTypeRef(t)
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

func sizeVal(p *uint64) uint32 {
	if p == nil {
		return 0
	}
	return uint32(*p)
}

// BuildViewType maps a resolved TypeRef to a ViewType.
func (r TypeResolver) BuildViewType(resolved *TypeRef) (*ViewType, error) {
	if s, ok := scalarViewTypes[resolved.Kind]; ok {
		return &ViewType{Kind: VKScalar, fixedSize: ptrSize(s.size), GoType: s.name}, nil
	}
	switch resolved.Kind {
	case TROpaqueFixed:
		raw := sizeVal(resolved.Size)
		padded := (raw + 3) &^ 3
		return &ViewType{Kind: VKOpaque, fixedSize: ptrSize(padded), GoType: "[]byte",
			Opaque: &OpaqueViewType{RawSize: raw}}, nil
	case TROpaqueVar, TRString:
		return &ViewType{Kind: VKOpaque, GoType: "VarOpaqueView",
			Opaque: &OpaqueViewType{MaxLen: sizeVal(resolved.MaxSize)}}, nil
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
		count := sizeVal(resolved.Count)
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
		return &ViewType{Kind: VKArray, GoType: "[]byte",
			Array: &ArrayViewType{Element: elem, MaxLen: sizeVal(resolved.MaxCount)}}, nil
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
