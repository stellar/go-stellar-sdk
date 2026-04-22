package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestPlanViews_MiniXDR(t *testing.T) {
	data, err := os.ReadFile("testdata/mini.json")
	if err != nil {
		t.Fatalf("reading IR: %v", err)
	}
	var ir IR
	if err = json.Unmarshal(data, &ir); err != nil {
		t.Fatalf("parsing IR: %v", err)
	}
	gen, err := NewGenerator(&ir)
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}

	plan, err := gen.PlanViews()
	if err != nil {
		t.Fatalf("PlanViews: %v", err)
	}

	var structs, unions, enums, typedefs, inlines int
	for _, e := range plan.Entries {
		switch e.(type) {
		case *StructViewPlan:
			structs++
		case *UnionViewPlan:
			unions++
		case *EnumViewPlan:
			enums++
		case *TypedefViewPlan:
			typedefs++
		case *InlineTypePlan:
			inlines++
		}
	}

	if structs != 2 {
		t.Errorf("structs = %d, want 2", structs)
	}
	if unions != 3 {
		t.Errorf("unions = %d, want 3", unions)
	}
	if enums != 1 {
		t.Errorf("enums = %d, want 1", enums)
	}
}

func TestPlanViews_StructFields(t *testing.T) {
	ir := &IR{
		Definitions: []DefWrap{
			{Kind: DKStruct, Struct: &StructDef{
				Name: "Foo",
				Fields: []StructField{
					{Name: "x", Type: TypeRef{Kind: TRInt}},
					{Name: "y", Type: TypeRef{Kind: TRUnsignedHyper}},
				},
			}, FixedSize: ptrSize(12)},
		},
	}
	gen, err := NewGenerator(ir)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := gen.PlanViews()
	if err != nil {
		t.Fatal(err)
	}

	if len(plan.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d entries", len(plan.Entries))
	}
	sp, ok := plan.Entries[0].(*StructViewPlan)
	if !ok {
		t.Fatalf("expected *StructViewPlan, got %T", plan.Entries[0])
	}
	if sp.ViewTypeName != "FooView" {
		t.Errorf("ViewTypeName = %q, want FooView", sp.ViewTypeName)
	}
	if sp.Fields[0].FieldName != "X" {
		t.Errorf("Fields[0].FieldName = %q, want X", sp.Fields[0].FieldName)
	}
	if sp.Fields[0].ViewType.GoType != "Int32View" {
		t.Errorf("Fields[0].GoType = %q, want Int32View", sp.Fields[0].ViewType.GoType)
	}
	if sp.Fields[1].ViewType.GoType != "Uint64View" {
		t.Errorf("Fields[1].GoType = %q, want Uint64View", sp.Fields[1].ViewType.GoType)
	}
}

func TestPlanViews_UnionArms(t *testing.T) {
	ir := &IR{
		Definitions: []DefWrap{
			{Kind: DKEnum, Enum: &EnumDef{
				Name:    "Tag",
				Members: []EnumMember{{Name: "TAG_A", Value: 0}, {Name: "TAG_B", Value: 1}},
			}},
			{Kind: DKUnion, Union: &UnionDef{
				Name:         "MyUnion",
				Discriminant: StructField{Name: "tag", Type: TypeRef{Kind: TRRef, Name: "Tag"}},
				Arms: []UnionArm{
					{Cases: []UnionCase{{Value: 0, Name: "TAG_A"}}, Name: ""},
					{Cases: []UnionCase{{Value: 1, Name: "TAG_B"}}, Name: "bValue", Type: &TypeRef{Kind: TRInt}},
				},
			}},
		},
	}
	gen, err := NewGenerator(ir)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := gen.PlanViews()
	if err != nil {
		t.Fatal(err)
	}

	var up *UnionViewPlan
	for _, e := range plan.Entries {
		if u, ok := e.(*UnionViewPlan); ok {
			up = u
			break
		}
	}
	if up == nil {
		t.Fatal("no union entry found")
	}

	if up.ViewTypeName != "MyUnionView" {
		t.Errorf("ViewTypeName = %q, want MyUnionView", up.ViewTypeName)
	}
	if len(up.Arms) != 2 {
		t.Fatalf("len(Arms) = %d, want 2", len(up.Arms))
	}
	if up.Arms[0].ViewType != nil {
		t.Error("Arms[0] should be void")
	}
	if up.Arms[1].ViewType == nil || up.Arms[1].ViewType.GoType != "Int32View" {
		t.Errorf("Arms[1].GoType = %v, want Int32View", up.Arms[1].ViewType)
	}
	if up.Arms[1].ArmName != "BValue" {
		t.Errorf("Arms[1].ArmName = %q, want BValue", up.Arms[1].ArmName)
	}
}

func TestPlanViews_InlineTypes(t *testing.T) {
	ir := &IR{
		Definitions: []DefWrap{
			{Kind: DKStruct, Struct: &StructDef{
				Name: "Foo",
				Fields: []StructField{
					{Name: "data", Type: TypeRef{Kind: TROpaqueVar, MaxSize: ptrU64(256)}},
				},
			}},
		},
	}
	gen, err := NewGenerator(ir)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := gen.PlanViews()
	if err != nil {
		t.Fatal(err)
	}

	if len(plan.Entries) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(plan.Entries))
	}
	ip, ok := plan.Entries[0].(*InlineTypePlan)
	if !ok {
		t.Fatal("first entry should be *InlineTypePlan")
	}
	if ip.Name != "FooDataOpaqueView" {
		t.Errorf("InlineType.Name = %q, want FooDataOpaqueView", ip.Name)
	}
	sp2 := plan.Entries[1].(*StructViewPlan)
	if sp2.Fields[0].ViewType.GoType != "FooDataOpaqueView" {
		t.Errorf("field GoType = %q, want FooDataOpaqueView", sp2.Fields[0].ViewType.GoType)
	}
}
