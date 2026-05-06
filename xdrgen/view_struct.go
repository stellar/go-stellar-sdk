package main

// emitStructViewFromPlan emits a struct view type from a pre-computed plan.
func emitStructViewFromPlan(f *GeneratedFile, sp *StructViewPlan) {
	g := f.Use("viewTypeName", sp.ViewTypeName)
	g.L("type $viewTypeName []byte")

	if sp.FixedWireSize != nil {
		emitFixedSizeMethods(f, sp.ViewTypeName, *sp.FixedWireSize)
		g = g.Set("fixedSize", *sp.FixedWireSize)
	} else {
		g.L("func (v $viewTypeName) size(depth int) (int, error) {")
		g.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
		g.L("	off := int64(0)")
		emitSizeTraversal(f, sp.Fields, "size(depth + 1)", "0")
		g.L("	return int(off), nil")
		g.L("}")
	}

	// Field accessors
	for i, fp := range sp.Fields {
		vt := fp.ViewType
		h := g.Set("fieldName", fp.FieldName).Set("fieldType", vt.GoType).Set("subView", vt.SubView("v", "0"))
		h.L("func (v $viewTypeName) $fieldName() ($fieldType, error) {")
		if sp.FixedWireSize != nil {
			h.L(`	if len(v) < $fixedSize { return nil, viewErrShortBuffer(0, "need $fixedSize bytes") }`)
		}
		if i == 0 {
			h.L("	return $subView, nil")
		} else {
			h = h.Set("subView", vt.SubView("v", "off"))
			h.L("	off := int64(0)")
			emitSizeTraversal(f, sp.Fields[:i], "size(0)", "nil")
			h.L("	return $subView, nil")
		}
		h.L("}")
		h.L("func (v $viewTypeName) Must$fieldName() $fieldType { return must(v.$fieldName()) }")
	}

	// valid
	g.L("func (v $viewTypeName) valid(depth int) (int, error) {")
	if sp.FixedWireSize != nil {
		g.L(`	if len(v) < $fixedSize { return 0, viewErrShortBuffer(0, "need $fixedSize bytes") }`)
	} else {
		g.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
	}
	g.L("	off := int64(0)")
	emitValidTraversal(f, sp.Fields)
	g.L("	return int(off), nil")
	g.L("}")
	emitPublicMethods(f, sp.ViewTypeName)
}
