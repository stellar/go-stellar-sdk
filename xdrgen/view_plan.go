package main

import "fmt"

// ViewPlan holds the resolved type information for all view definitions,
// in IR order. It is computed from the IR without any code emission,
// making it testable independently of the emitters.
type ViewPlan struct {
	Entries []ViewPlanEntry
}

// ViewPlanEntry is implemented by each plan type.
type ViewPlanEntry interface {
	planEntry()
}

type StructViewPlan struct {
	ViewTypeName  string
	FixedWireSize *uint32
	Fields        []FieldPlan
}

func (*StructViewPlan) planEntry() {}

type FieldPlan struct {
	FieldName   string
	ViewType    *ViewType
	IsVoidCase0 bool
}

type UnionViewPlan struct {
	ViewTypeName  string
	FixedWireSize *uint32
	DiscName      string
	DiscViewType  *ViewType
	Arms          []UnionArmPlan
}

func (*UnionViewPlan) planEntry() {}

type UnionArmPlan struct {
	ArmName   string
	ViewType  *ViewType
	CaseExprs []string
}

type EnumViewPlan struct {
	ViewTypeName string
	EnumName     string
	CaseNames    []string
}

func (*EnumViewPlan) planEntry() {}

type TypedefViewPlan struct {
	AliasName string
	ViewType  *ViewType
}

func (*TypedefViewPlan) planEntry() {}

type InlineTypePlan struct {
	Name     string
	ViewType *ViewType
}

func (*InlineTypePlan) planEntry() {}

// PlanViews computes the ViewPlan for the entire IR.
func (g *Generator) PlanViews() (*ViewPlan, error) {
	plan := &ViewPlan{}

	for _, def := range g.allDefs {
		switch def.Kind {
		case DKStruct:
			if err := g.planStruct(plan, def.Struct); err != nil {
				return nil, err
			}
		case DKUnion:
			if err := g.planUnion(plan, def.Union); err != nil {
				return nil, err
			}
		case DKEnum:
			g.planEnum(plan, def.Enum)
		case DKTypedef:
			if err := g.planTypedef(plan, def.Typedef); err != nil {
				return nil, err
			}
		case DKConst:
		}
	}
	return plan, nil
}

func inlineTypeName(containerName, fieldName string, vt *ViewType) string {
	suffix := "View"
	if vt.Kind == VKOpaque {
		suffix = "OpaqueView"
	}
	if vt.Kind == VKOptional {
		suffix = "OptView"
	}
	return GoTypeName(containerName) + GoTypeName(fieldName) + suffix
}

func nameInlineType(containerName, fieldName string, vt *ViewType) (*ViewType, *InlineTypePlan) {
	if !vt.NeedsConcreteType() {
		return vt, nil
	}
	name := inlineTypeName(containerName, fieldName, vt)
	result := *vt
	result.GoType = name
	return &result, &InlineTypePlan{Name: name, ViewType: vt}
}

func (g *Generator) planStruct(plan *ViewPlan, s *StructDef) error {
	sp := StructViewPlan{
		ViewTypeName:  GoTypeName(s.Name) + "View",
		FixedWireSize: g.TypeResolver[s.Name].FixedSize,
	}

	for _, f := range s.Fields {
		vt, err := g.ResolveViewType(&f.Type)
		if err != nil {
			return fmt.Errorf("struct %s field %s: %w", s.Name, f.Name, err)
		}
		isInline := f.Type.Kind != TRRef
		if isInline {
			var inlinePlan *InlineTypePlan
			vt, inlinePlan = nameInlineType(s.Name, f.Name, vt)
			if inlinePlan != nil {
				plan.Entries = append(plan.Entries, inlinePlan)
			}
		}
		sp.Fields = append(sp.Fields, FieldPlan{
			FieldName:   GoTypeName(f.Name),
			ViewType:    vt,
			IsVoidCase0: g.isVoidCase0Union(vt),
		})
	}

	plan.Entries = append(plan.Entries, &sp)
	return nil
}

func (g *Generator) planUnion(plan *ViewPlan, u *UnionDef) error {
	for _, arm := range u.Arms {
		if len(arm.Cases) == 0 {
			return fmt.Errorf("union %s: XDR default arms not yet supported", u.Name)
		}
	}

	xdrName := GoTypeName(u.Name)

	discVT, err := g.ResolveViewType(&u.Discriminant.Type)
	if err != nil {
		return fmt.Errorf("union %s discriminant: %w", u.Name, err)
	}

	up := UnionViewPlan{
		ViewTypeName:  xdrName + "View",
		FixedWireSize: g.TypeResolver[u.Name].FixedSize,
		DiscName:      GoTypeName(u.Discriminant.Name),
		DiscViewType:  discVT,
	}

	for i, arm := range u.Arms {
		var caseExprs []string
		for j := range arm.Cases {
			expr, ceErr := g.caseValueExpr(u, j, &arm)
			if ceErr != nil {
				return ceErr
			}
			caseExprs = append(caseExprs, expr)
		}

		armName := GoTypeName(arm.Name)
		if armName == "" {
			armName = fmt.Sprintf("Arm%d", i)
		}

		var vt *ViewType
		if arm.Type != nil {
			vt, err = g.ResolveViewType(arm.Type)
			if err != nil {
				return fmt.Errorf("union %s arm %s: %w", u.Name, armName, err)
			}
			if arm.Type.Kind != TRRef {
				var inlinePlan *InlineTypePlan
				vt, inlinePlan = nameInlineType(xdrName, armName, vt)
				if inlinePlan != nil {
					plan.Entries = append(plan.Entries, inlinePlan)
				}
			}
		}

		up.Arms = append(up.Arms, UnionArmPlan{
			ArmName:   armName,
			ViewType:  vt,
			CaseExprs: caseExprs,
		})
	}

	plan.Entries = append(plan.Entries, &up)
	return nil
}

func (g *Generator) planEnum(plan *ViewPlan, e *EnumDef) {
	enumName := GoTypeName(e.Name)
	caseNames := make([]string, len(e.Members))
	for i, m := range e.Members {
		caseNames[i] = enumName + GoTypeName(m.Name)
	}
	plan.Entries = append(plan.Entries, &EnumViewPlan{
		ViewTypeName: enumName + "View",
		EnumName:     enumName,
		CaseNames:    caseNames,
	})
}

func (g *Generator) planTypedef(plan *ViewPlan, td *TypedefDef) error {
	vt, err := g.ResolveViewType(&td.Type)
	if err != nil {
		return fmt.Errorf("typedef %s: %w", td.Name, err)
	}
	aliasName := GoTypeName(td.Name) + "View"

	if vt.NeedsConcreteType() && aliasName != vt.GoType {
		plan.Entries = append(plan.Entries, &InlineTypePlan{
			Name:     aliasName,
			ViewType: vt,
		})
		result := *vt
		result.GoType = aliasName
		vt = &result
	}

	plan.Entries = append(plan.Entries, &TypedefViewPlan{
		AliasName: aliasName,
		ViewType:  vt,
	})
	return nil
}

// isVoidCase0Union checks if a ViewType is a union whose case 0 is void.
func (g *Generator) isVoidCase0Union(vt *ViewType) bool {
	if vt.Named == nil {
		return false
	}
	def, ok := g.TypeResolver[vt.Named.XDRName]
	if !ok || def.Kind != DKUnion {
		return false
	}
	for _, arm := range def.Union.Arms {
		for _, c := range arm.Cases {
			if c.Value == 0 {
				return arm.Type == nil
			}
		}
	}
	return false
}

// caseValueExpr returns a Go expression for a union case value, cast to int32.
func (g *Generator) caseValueExpr(u *UnionDef, caseIdx int, arm *UnionArm) (string, error) {
	c := arm.Cases[caseIdx]
	if c.Name == "" {
		return fmt.Sprintf("int32(%d)", c.Value), nil
	}
	discType, err := g.resolveTypeRef(&u.Discriminant.Type)
	if err != nil {
		return "", err
	}
	if discType.Kind == TRRef {
		return fmt.Sprintf("int32(%s%s)", GoTypeName(discType.Name), GoTypeName(c.Name)), nil
	}
	return fmt.Sprintf("int32(%s)", GoTypeName(c.Name)), nil
}
