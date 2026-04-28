package main

import "testing"

func testResolver(defs []DefWrap) TypeResolver {
	return NewTypeResolver(&IR{Definitions: defs})
}

func mustBuild(t *testing.T, r TypeResolver, ref *TypeRef) *ViewType {
	t.Helper()
	vt, err := r.BuildViewType(ref)
	if err != nil {
		t.Fatalf("BuildViewType(%v): %v", ref.Kind, err)
	}
	return vt
}

func mustResolve(t *testing.T, r TypeResolver, ref *TypeRef) *ViewType {
	t.Helper()
	vt, err := r.ResolveViewType(ref)
	if err != nil {
		t.Fatalf("ResolveViewType(%v): %v", ref.Kind, err)
	}
	return vt
}

func ptrU64(n uint64) *uint64 { return &n }

func TestBuildViewType_Scalars(t *testing.T) {
	r := testResolver(nil)
	tests := []struct {
		kind     string
		wantType string
		wantSize uint32
	}{
		{TRInt, "Int32View", 4},
		{TRUnsignedInt, "Uint32View", 4},
		{TRHyper, "Int64View", 8},
		{TRUnsignedHyper, "Uint64View", 8},
		{TRBool, "BoolView", 4},
		{TRFloat, "Float32View", 4},
		{TRDouble, "Float64View", 8},
	}
	for _, tt := range tests {
		vt := mustBuild(t, r, &TypeRef{Kind: tt.kind})
		if vt.GoType != tt.wantType {
			t.Errorf("BuildViewType(%s).GoType = %q, want %q", tt.kind, vt.GoType, tt.wantType)
		}
		fs, ok := vt.FixedSize()
		if !ok || fs != tt.wantSize {
			t.Errorf("BuildViewType(%s).FixedSize = (%d, %v), want (%d, true)", tt.kind, fs, ok, tt.wantSize)
		}
	}
}

func TestBuildViewType_OpaqueFixed(t *testing.T) {
	r := testResolver(nil)
	tests := []struct {
		rawSize, wantPad, wantRaw uint32
	}{
		{32, 32, 32}, {5, 8, 5}, {1, 4, 1}, {4, 4, 4}, {33, 36, 33},
	}
	for _, tt := range tests {
		vt := mustBuild(t, r, &TypeRef{Kind: TROpaqueFixed, Size: ptrU64(uint64(tt.rawSize))})
		fs, ok := vt.FixedSize()
		if !ok || fs != tt.wantPad {
			t.Errorf("OpaqueFixed(%d).FixedSize = (%d, %v), want (%d, true)", tt.rawSize, fs, ok, tt.wantPad)
		}
		if vt.Opaque.RawSize != tt.wantRaw {
			t.Errorf("OpaqueFixed(%d).RawSize = %d, want %d", tt.rawSize, vt.Opaque.RawSize, tt.wantRaw)
		}
	}
}

func TestBuildViewType_OpaqueVar(t *testing.T) {
	r := testResolver(nil)
	vt := mustBuild(t, r, &TypeRef{Kind: TROpaqueVar})
	if _, ok := vt.FixedSize(); ok {
		t.Error("OpaqueVar: expected variable size")
	}
	if vt.Opaque.MaxLen != 0 {
		t.Errorf("OpaqueVar.MaxLen = %d, want 0", vt.Opaque.MaxLen)
	}

	vt = mustBuild(t, r, &TypeRef{Kind: TROpaqueVar, MaxSize: ptrU64(256)})
	if vt.Opaque.MaxLen != 256 {
		t.Errorf("OpaqueVar(256).MaxLen = %d, want 256", vt.Opaque.MaxLen)
	}
}

func TestBuildViewType_String(t *testing.T) {
	r := testResolver(nil)
	vt := mustBuild(t, r, &TypeRef{Kind: TRString})
	if vt.GoType != "VarOpaqueView" {
		t.Errorf("String.GoType = %q, want VarOpaqueView", vt.GoType)
	}
}

func TestBuildViewType_Ident_Struct(t *testing.T) {
	r := testResolver([]DefWrap{
		{Kind: DKStruct, Struct: &StructDef{Name: "Foo"}, FixedSize: ptrSize(12)},
	})
	vt := mustBuild(t, r, &TypeRef{Kind: TRRef, Name: "Foo"})
	if vt.GoType != "FooView" {
		t.Errorf("GoType = %q, want FooView", vt.GoType)
	}
	if fs, ok := vt.FixedSize(); !ok || fs != 12 {
		t.Errorf("FixedSize = (%d, %v), want (12, true)", fs, ok)
	}
	if vt.Named.XDRName != "Foo" {
		t.Errorf("XDRName = %q, want Foo", vt.Named.XDRName)
	}
}

func TestBuildViewType_Ident_Enum(t *testing.T) {
	r := testResolver([]DefWrap{
		{Kind: DKEnum, Enum: &EnumDef{Name: "Color"}},
	})
	vt := mustBuild(t, r, &TypeRef{Kind: TRRef, Name: "Color"})
	if fs, ok := vt.FixedSize(); !ok || fs != 4 {
		t.Errorf("FixedSize = (%d, %v), want (4, true)", fs, ok)
	}
}

func TestBuildViewType_Ident_VariableUnion(t *testing.T) {
	r := testResolver([]DefWrap{
		{Kind: DKUnion, Union: &UnionDef{Name: "MyUnion"}},
	})
	vt := mustBuild(t, r, &TypeRef{Kind: TRRef, Name: "MyUnion"})
	if _, ok := vt.FixedSize(); ok {
		t.Error("variable union should not have fixed size")
	}
}

func TestBuildViewType_FixedArray(t *testing.T) {
	r := testResolver(nil)
	vt := mustBuild(t, r, &TypeRef{
		Kind: TRArray, Count: ptrU64(5), Element: &TypeRef{Kind: TRInt},
	})
	if fs, ok := vt.FixedSize(); !ok || fs != 20 {
		t.Errorf("FixedSize = (%d, %v), want (20, true)", fs, ok)
	}
	if vt.Array.Count != 5 {
		t.Errorf("Count = %d, want 5", vt.Array.Count)
	}
	if vt.Array.Element.GoType != "Int32View" {
		t.Errorf("Element.GoType = %q, want Int32View", vt.Array.Element.GoType)
	}
}

func TestBuildViewType_VarArray(t *testing.T) {
	r := testResolver(nil)
	vt := mustBuild(t, r, &TypeRef{
		Kind: TRVarArray, MaxCount: ptrU64(100), Element: &TypeRef{Kind: TRInt},
	})
	if _, ok := vt.FixedSize(); ok {
		t.Error("VarArray should not have fixed size")
	}
	if vt.Array.MaxLen != 100 {
		t.Errorf("MaxLen = %d, want 100", vt.Array.MaxLen)
	}
}

func TestBuildViewType_Optional(t *testing.T) {
	r := testResolver(nil)
	vt := mustBuild(t, r, &TypeRef{Kind: TROptional, Element: &TypeRef{Kind: TRInt}})
	if vt.Kind != VKOptional {
		t.Errorf("Kind = %q, want VKOptional", vt.Kind)
	}
	if vt.Optional.Element.GoType != "Int32View" {
		t.Errorf("Element.GoType = %q, want Int32View", vt.Optional.Element.GoType)
	}
}

func TestBuildViewType_UnknownKind(t *testing.T) {
	r := testResolver(nil)
	_, err := r.BuildViewType(&TypeRef{Kind: "nonsense"})
	if err == nil {
		t.Error("expected error for unknown TypeRef kind")
	}
}

func TestBuildViewType_UnknownRef(t *testing.T) {
	r := testResolver(nil)
	_, err := r.BuildViewType(&TypeRef{Kind: TRRef, Name: "DoesNotExist"})
	if err == nil {
		t.Error("expected error for unknown ref")
	}
}

func TestResolveViewType_TypedefChain(t *testing.T) {
	r := testResolver([]DefWrap{
		{Kind: DKTypedef, Typedef: &TypedefDef{
			Name: "Hash", Type: TypeRef{Kind: TROpaqueFixed, Size: ptrU64(32)},
		}},
		{Kind: DKTypedef, Typedef: &TypedefDef{
			Name: "AccountId", Type: TypeRef{Kind: TRRef, Name: "Hash"},
		}},
	})
	vt := mustResolve(t, r, &TypeRef{Kind: TRRef, Name: "AccountId"})
	if vt.GoType != "AccountIdView" {
		t.Errorf("GoType = %q, want AccountIdView", vt.GoType)
	}
	if fs, ok := vt.FixedSize(); !ok || fs != 32 {
		t.Errorf("FixedSize = (%d, %v), want (32, true)", fs, ok)
	}
}

func TestResolveViewType_TypedefToStruct(t *testing.T) {
	r := testResolver([]DefWrap{
		{Kind: DKStruct, Struct: &StructDef{Name: "Inner"}, FixedSize: ptrSize(8)},
		{Kind: DKTypedef, Typedef: &TypedefDef{
			Name: "Outer", Type: TypeRef{Kind: TRRef, Name: "Inner"},
		}},
	})
	vt := mustResolve(t, r, &TypeRef{Kind: TRRef, Name: "Outer"})
	if vt.GoType != "OuterView" {
		t.Errorf("GoType = %q, want OuterView", vt.GoType)
	}
}

func testGenerator(defs []DefWrap) *Generator {
	gen, _ := NewGenerator(&IR{Definitions: defs})
	return gen
}

func TestIsVoidCase0Union(t *testing.T) {
	defs := []DefWrap{
		{Kind: DKUnion, Union: &UnionDef{
			Name:         "ExtPoint",
			Discriminant: StructField{Name: "v", Type: TypeRef{Kind: TRInt}},
			Arms:         []UnionArm{{Cases: []UnionCase{{Value: 0}}, Name: ""}},
		}, FixedSize: ptrSize(4)},
		{Kind: DKUnion, Union: &UnionDef{
			Name:         "NonVoid",
			Discriminant: StructField{Name: "v", Type: TypeRef{Kind: TRInt}},
			Arms:         []UnionArm{{Cases: []UnionCase{{Value: 0}}, Name: "val", Type: &TypeRef{Kind: TRInt}}},
		}, FixedSize: ptrSize(8)},
	}
	g := testGenerator(defs)
	r := g.TypeResolver

	vt := mustResolve(t, r, &TypeRef{Kind: TRRef, Name: "ExtPoint"})
	if !g.isVoidCase0Union(vt) {
		t.Error("ExtPoint should be void-case-0 union")
	}

	vt2 := mustResolve(t, r, &TypeRef{Kind: TRRef, Name: "NonVoid"})
	if g.isVoidCase0Union(vt2) {
		t.Error("NonVoid should not be void-case-0 union")
	}

	vt3, _ := r.BuildViewType(&TypeRef{Kind: TRInt})
	if g.isVoidCase0Union(vt3) {
		t.Error("scalar should not be void-case-0 union")
	}
}

func TestCaseValueExpr(t *testing.T) {
	defs := []DefWrap{
		{Kind: DKEnum, Enum: &EnumDef{Name: "MyEnum", Members: []EnumMember{{Name: "MY_ENUM_FOO", Value: 0}}}},
	}
	g := testGenerator(defs)

	u := &UnionDef{
		Name:         "U",
		Discriminant: StructField{Name: "v", Type: TypeRef{Kind: TRRef, Name: "MyEnum"}},
	}
	arm := &UnionArm{Cases: []UnionCase{{Value: 0, Name: "MY_ENUM_FOO"}, {Value: 5}}}

	got, err := g.caseValueExpr(u, 0, arm)
	if err != nil {
		t.Fatalf("caseValueExpr(ident): %v", err)
	}
	if got != "int32(MyEnumMyEnumFoo)" {
		t.Errorf("CaseValueExpr(ident) = %q, want %q", got, "int32(MyEnumMyEnumFoo)")
	}

	got, err = g.caseValueExpr(u, 1, arm)
	if err != nil {
		t.Fatalf("caseValueExpr(literal): %v", err)
	}
	if got != "int32(5)" {
		t.Errorf("CaseValueExpr(literal) = %q, want %q", got, "int32(5)")
	}
}

func TestNewGenerator_DuplicateNames(t *testing.T) {
	ir := &IR{
		Definitions: []DefWrap{
			{Kind: DKStruct, Struct: &StructDef{Name: "Foo"}},
			{Kind: DKStruct, Struct: &StructDef{Name: "Foo"}},
		},
	}
	_, err := NewGenerator(ir)
	if err == nil {
		t.Error("expected error for duplicate definition names")
	}
}
