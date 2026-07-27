package main

import "fmt"

// Tier-1 struct emission (two-tier visitor design): a plain thin view with
// per-field lazy accessors. Accessing field k locates it by sizing the
// preceding variable-size fields through the thin engine (status-quo
// semantics — no memo, no shared traversal state); the returned sub-view is
// open-ended (its own extent stays lazy). Must* accessors panic with the
// *ViewError sentinel for Try-chained navigation.

// emitStructView emits a struct view type from a pre-computed plan.
func emitStructView(f *GeneratedFile, sp *StructViewPlan) error {
	if sp.FixedWireSize != nil && *sp.FixedWireSize == 0 {
		return fmt.Errorf("struct view %s: zero-width navigable node (empty struct); "+
			"a zero-width node cannot advance a traversal", sp.ViewTypeName)
	}
	g := f.Use("viewTypeName", sp.ViewTypeName)
	g.L("type $viewTypeName struct{ view }")
	emitParse(f, sp.ViewTypeName)

	if sp.FixedWireSize != nil {
		emitFixedSizeMethods(f, sp.ViewTypeName, *sp.FixedWireSize)
		g = g.Set("fixedSize", *sp.FixedWireSize)
	} else {
		g.L("func size$viewTypeName(d []byte, depth int) (int, error) {")
		g.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
		g.L("	off := int64(0)")
		emitSizeTraversal(f, sp.Fields, "depth+1", "0", true)
		g.L("	return int(off), nil")
		g.L("}")
		g.L("func (v $viewTypeName) size(depth int) (int, error) { return size$viewTypeName(v.d, depth) }")
	}

	// valid — thin engine function + method delegate
	g.L("func valid$viewTypeName(d []byte, depth int) (int, error) {")
	if sp.FixedWireSize != nil {
		g.L(`	if len(d) < $fixedSize { return 0, viewErrShortBuffer(0, "need $fixedSize bytes") }`)
	} else {
		g.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
	}
	g.L("	off := int64(0)")
	emitValidTraversal(f, sp.Fields)
	g.L("	return int(off), nil")
	g.L("}")
	g.L("func (v $viewTypeName) valid(depth int) (int, error) { return valid$viewTypeName(v.d, depth) }")
	emitPublicMethods(f, sp.ViewTypeName)

	emitStructAccessors(f, sp)
	return nil
}

// staticBoundaries returns the compile-time-constant field start offsets:
// entry k is the start of field k, present for k = 0..m where fields 0..k-1
// are all fixed-size. Entry 0 (offset 0) is always present. At most
// len(fields) entries are returned.
func staticBoundaries(fields []FieldPlan) []uint32 {
	offs := []uint32{0}
	var off uint32
	for i := 0; i+1 < len(fields); i++ {
		fs, ok := fields[i].ViewType.FixedSize()
		if !ok {
			break
		}
		off += fs
		offs = append(offs, off)
	}
	return offs
}

// emitStructAccessors emits one lazy accessor per field (plus its Must*
// twin): field starts in the fixed-size prefix fold to constants; later
// fields locate by sizing the preceding fields from the last static boundary
// through the thin engine on every access (tier-1 semantics: no memo).
func emitStructAccessors(f *GeneratedFile, sp *StructViewPlan) {
	static := staticBoundaries(sp.Fields)
	g := f.Use("viewTypeName", sp.ViewTypeName)
	for k, fp := range sp.Fields {
		h := g.Set("fieldName", fp.FieldName).Set("fieldType", fp.ViewType.GoType).Set("k", k)
		h.L("// $fieldName returns the $fieldName field's sub-view (open-ended: its extent")
		h.L("// stays lazy), locating it by sizing any preceding variable-size fields.")
		if k < len(static) {
			h = h.Set("start", static[k])
			h.L("func (v $viewTypeName) $fieldName() ($fieldType, error) {")
			if static[k] > 0 {
				h.L(`	if $start > int64(len(v.d)) { return $fieldType{}, viewErrShortBuffer($start, "field offset exceeds data") }`)
			}
			h.L("	return $fieldType{view{d: v.d[$start:]}}, nil")
			h.L("}")
		} else {
			h = h.Set("startConst", static[len(static)-1])
			h.L("func (v $viewTypeName) $fieldName() ($fieldType, error) {")
			h.L("	d := v.d")
			h.L("	off := int64($startConst)")
			emitSizeTraversal(f, sp.Fields[len(static)-1:k], "1", fp.ViewType.GoType+"{}", static[len(static)-1] == 0)
			h.Block(`
				return $fieldType{view{d: d[off:]}}, nil
				}
			`)
		}
		h.Block(`
			// Must$fieldName is $fieldName panicking with the *ViewError sentinel (recover via Try*).
			func (v $viewTypeName) Must$fieldName() $fieldType {
				fv, err := v.$fieldName()
				if err != nil { mustView(err) }
				return fv
			}
		`)
	}
}
