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

	// Fields() bundle + locate helper.
	emitStructFields(f, sp)
}

// emitStructFields emits the FooFields bundle type, the unexported locate helper
// (one walk capturing every field's trimmed extent), and the Fields() method.
// For fixed-size structs the offsets are compile-time constants, so locate is
// straight-line with no walk; for variable-size structs it reuses the size
// traversal logic, capturing offsets instead of discarding them.
func emitStructFields(f *GeneratedFile, sp *StructViewPlan) {
	fieldsType := GoTypeName(sp.XDRName) + "Fields"
	locateFn := "locate" + GoTypeName(sp.XDRName)
	g := f.Use("viewTypeName", sp.ViewTypeName, "fieldsType", fieldsType, "locateFn", locateFn)

	// Bundle type: View (whole node, trimmed) + one trimmed sub-view per field.
	g.L("// $fieldsType is the located form of $viewTypeName: every field trimmed to its exact wire extent, all found in one walk.")
	g.L("type $fieldsType struct {")
	g.L("	View $viewTypeName")
	for _, fp := range sp.Fields {
		g.Set("bundleName", fieldsBundleFieldName(fp.FieldName)).Set("fieldType", fp.ViewType.GoType).
			L("	$bundleName $fieldType")
	}
	g.L("}")

	// locate helper
	g.L("func $locateFn(v $viewTypeName) ($fieldsType, error) {")
	g.L("	var f $fieldsType")
	if sp.FixedWireSize != nil {
		g = g.Set("fixedSize", *sp.FixedWireSize)
		g.L(`	if len(v) < $fixedSize { return f, viewErrShortBuffer(0, "need $fixedSize bytes") }`)
		// All fields are fixed-size; emit constant offsets.
		var off uint32
		for _, fp := range sp.Fields {
			fs, _ := fp.ViewType.FixedSize()
			h := g.Set("bundleName", fieldsBundleFieldName(fp.FieldName)).
				Set("fieldType", fp.ViewType.GoType).
				Set("start", off).Set("end", off+fs)
			h.L("	f.$bundleName = $fieldType(v[$start:$end])")
			off += fs
		}
		g.Set("total", *sp.FixedWireSize).L("	f.View = $viewTypeName(v[:$total])")
	} else {
		g.L("	off := int64(0)")
		emitLocateTraversal(f, sp.Fields)
		g.L("	f.View = $viewTypeName(v[:off])")
	}
	g.L("	return f, nil")
	g.L("}")

	// Fields() method. If a struct has a field whose own accessor is already named
	// Fields (which must stay untouched), escape this method to Fields_ to avoid a
	// method-set collision, mirroring the View_ escape.
	methodName := "Fields"
	for _, fp := range sp.Fields {
		if fp.FieldName == "Fields" {
			methodName = "Fields_"
			break
		}
	}
	h := g.Set("methodName", methodName)
	h.L("// $methodName locates every field of this node in a single walk, each trimmed to its exact wire extent.")
	h.L("func (v $viewTypeName) $methodName() ($fieldsType, error) { return $locateFn(v) }")
}
