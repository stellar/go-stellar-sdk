package main

import "fmt"

// emitStructViewFromPlan emits a struct view type from a pre-computed plan.
func emitStructViewFromPlan(f *GeneratedFile, sp *StructViewPlan) {
	g := f.Use("viewTypeName", sp.ViewTypeName)
	g.L("type $viewTypeName struct{ view }")
	emitParse(f, sp.ViewTypeName)

	slow := sp.FixedWireSize == nil
	if slow {
		emitTidConst(f, sp.ViewTypeName, sp.Tid)
	}

	if sp.FixedWireSize != nil {
		emitFixedSizeMethods(f, sp.ViewTypeName, *sp.FixedWireSize)
		g = g.Set("fixedSize", *sp.FixedWireSize)
	} else {
		g.L("func size$viewTypeName(d []byte, depth int) (int, error) {")
		g.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
		g.L("	off := int64(0)")
		emitSizeTraversal(f, sp.Fields, "0")
		g.L("	return int(off), nil")
		g.L("}")
		g.L("func (v $viewTypeName) size(depth int) (int, error) { return size$viewTypeName(v.d, depth) }")
		emitStructSizeResume(f, sp)
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
	emitPublicMethods(f, sp.ViewTypeName, slow)

	// Fields() lazy memoizing bundle.
	emitStructFields(f, sp)
}

// emitStructSizeResume emits the walk-assisted sizing body for a variable-size
// struct: a record covering exactly this node returns its extent in O(1);
// records inside it let child sizing resume past already-resolved bytes; on
// completion the node's own span is recorded (records bubble upward).
//
// When the struct has a resolve()-backed bundle (a frontier writer), the body
// also consults the frontier stack after a leaf-record miss: an entry for this
// node restarts the field loop at (idx, off) instead of 0, so fields the
// bundle already sized are never re-walked. Each field's advance is guarded by
// its index; frontier idx values land only on field boundaries (they originate
// from resolve(i)).
func emitStructSizeResume(f *GeneratedFile, sp *StructViewPlan) {
	g := f.Use("viewTypeName", sp.ViewTypeName)
	// A frontier entry for this struct can only exist if its bundle has a
	// resolve helper (the writer); without one, skip the consult entirely.
	dynamic := len(staticBoundaries(sp.Fields)) < len(sp.Fields)
	g.L("// sizeResume is size() with walk assistance: a record covering exactly this")
	g.L("// node returns its extent in O(1); records inside it resume sizing past the")
	g.L("// already-resolved bytes; on completion the node's own span is recorded.")
	if dynamic {
		g.L("// A frontier entry left by this node's bundle restarts the field loop at the")
		g.L("// resolved boundary, skipping every field the bundle already sized.")
	}
	g.L("func (v $viewTypeName) sizeResume(depth int) (int, error) {")
	g.L("	if v.w != nil {")
	g.L("		if end, ok := v.w.hit(tid$viewTypeName, v.off, v.off+int64(len(v.d))); ok { return int(end - v.off), nil }")
	g.L("	}")
	g.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
	g.L("	off := int64(0)")
	if dynamic {
		g.Set("n", len(sp.Fields)).Block(`
			fidx := 0
			if v.w != nil {
				if fi, fo, ok := v.w.frontier(tid$viewTypeName, v.off, int64(len(v.d))); ok && int(fi) < $n {
					fidx, off = int(fi), fo
				}
			}
		`)
	}
	for i := range sp.Fields {
		vt := sp.Fields[i].ViewType
		h := g.Set("i", i)
		if fs, ok := vt.FixedSize(); ok {
			if dynamic {
				h.Set("fs", fs).L("	if fidx <= $i { off += $fs }")
			} else {
				h.Set("fs", fs).L("	off += $fs")
			}
			continue
		}
		if dynamic {
			h.L("	if fidx <= $i {")
		}
		emitResumeAdvance(f, vt, sp.Fields[i].IsVoidCase0, "depth + 1", "0", false)
		if dynamic {
			h.L("	}")
		}
	}
	g.L(`	if off > int64(len(v.d)) { return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data") }`)
	g.L("	if v.w != nil { v.w.record(tid$viewTypeName, v.off, v.off+off) }")
	g.L("	return int(off), nil")
	g.L("}")
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

// emitStructFields emits the XFields lazy memoizing bundle: the bundle type,
// the Fields() constructor, one pointer-receiver accessor per field, and (when
// any field start is dynamic) the unexported resolve helper. Field starts that
// are compile-time constants (the fixed-size prefix) are folded at generation
// time; dynamic starts resolve on first access — from the memo, from a walk
// record via sizeResume, or by sizing — and stay memoized in the bundle.
func emitStructFields(f *GeneratedFile, sp *StructViewPlan) {
	fieldsType := GoTypeName(sp.XDRName) + "Fields"
	n := len(sp.Fields)
	static := staticBoundaries(sp.Fields)
	dynamic := len(static) < n // some field start needs runtime resolution

	g := f.Use("viewTypeName", sp.ViewTypeName, "fieldsType", fieldsType, "n", n)

	g.L("// $fieldsType is the lazy memoizing field bundle for $viewTypeName: field start")
	g.L("// offsets resolve on first access and stay memoized in the bundle (the bundle")
	g.L("// IS the memo), so reading every field in order costs one pass and revisits")
	g.L("// are free. Use through a pointer; the zero value is a safe empty bundle.")
	if dynamic {
		g.L("type $fieldsType struct {")
		g.L("	v    $viewTypeName")
		g.L("	offs [$n]int32 // resolved field starts relative to v.off; -1 = unresolved")
		g.L("}")
		// offs literal: constants for the static prefix, -1 beyond.
		lit := make([]string, n)
		for k := range lit {
			if k < len(static) {
				lit[k] = fmt.Sprint(static[k])
			} else {
				lit[k] = "-1"
			}
		}
		h := g.Set("offsLit", joinComma(lit))
		h.L("// Fields returns the lazy field bundle for this view. Nothing is resolved up front.")
		h.L("func (v $viewTypeName) Fields() ($fieldsType, error) {")
		h.L("	return $fieldsType{v: v, offs: [$n]int32{$offsLit}}, nil")
		h.L("}")
	} else {
		g.L("type $fieldsType struct {")
		g.L("	v $viewTypeName")
		g.L("}")
		g.L("// Fields returns the lazy field bundle for this view. Nothing is resolved up front.")
		g.L("func (v $viewTypeName) Fields() ($fieldsType, error) {")
		g.L("	return $fieldsType{v: v}, nil")
		g.L("}")
	}

	// Accessors.
	for k, fp := range sp.Fields {
		h := g.Set("fieldName", fp.FieldName).Set("fieldType", fp.ViewType.GoType).Set("k", k)
		h.L("// $fieldName returns the $fieldName field's sub-view, resolving and memoizing")
		h.L("// the starts of any preceding unresolved fields on the way.")
		if k < len(static) {
			// Static start: compile-time constant offset, bounds-checked.
			h = h.Set("start", static[k])
			h.L("func (f *$fieldsType) $fieldName() ($fieldType, error) {")
			if static[k] > 0 {
				h.L(`	if $start > int64(len(f.v.d)) { return $fieldType{}, viewErrShortBuffer($start, "field offset exceeds data") }`)
			}
			h.L("	return $fieldType{f.v.sub($start)}, nil")
			h.L("}")
			continue
		}
		h.Block(`
			func (f *$fieldsType) $fieldName() ($fieldType, error) {
				off := int64(f.offs[$k])
				if off < 0 {
					var err error
					if off, err = f.resolve($k); err != nil { return $fieldType{}, err }
				}
				return $fieldType{f.v.sub(off)}, nil
			}
		`)
	}

	if dynamic {
		emitStructResolve(f, sp, fieldsType, static)
	}
}

// emitStructResolve emits the unexported resolve helper: it computes the start
// of field i (only called for dynamic starts), resolving and memoizing every
// unresolved boundary before it. Each step comes from the memo, from a walk
// record via sizeResume, or from a blind size walk; memoized offsets are
// always bounds-checked before storing, so accessors can slice without
// re-checking.
//
// Every successful resolve notes the frontier for this node — "resolved
// through field i, which starts at off" — so a later sizeResume of the SAME
// struct (an iterator advance, a Raw() after consumption) restarts its field
// loop there instead of re-walking the fields the bundle already sized.
func emitStructResolve(f *GeneratedFile, sp *StructViewPlan, fieldsType string, static []uint32) {
	n := len(sp.Fields)
	g := f.Use("fieldsType", fieldsType, "viewTypeName", sp.ViewTypeName, "startConst", static[len(static)-1])
	g.L("func (f *$fieldsType) resolve(i int) (int64, error) {")
	g.L("	v := &f.v")
	g.L("	off := int64($startConst)")
	for b := len(static); b <= n-1; b++ {
		// Step: compute boundary b (start of field b) by advancing past field b-1.
		fld := sp.Fields[b-1]
		h := g.Set("b", b)
		h.L("	if o := f.offs[$b]; o >= 0 { off = int64(o) } else {")
		if fs, ok := fld.ViewType.FixedSize(); ok {
			h.Set("fs", fs).L("		off += $fs")
			h.L(`		if off > int64(len(v.d)) || off > maxViewOff { return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data") }`)
		} else {
			emitResumeAdvance(f, fld.ViewType, fld.IsVoidCase0, "0", "0", true)
		}
		h.L("		f.offs[$b] = int32(off)")
		h.L("	}")
		if b < n-1 {
			h.L("	if i == $b {")
			h.L("		if v.w != nil { v.w.noteFrontier(tid$viewTypeName, v.off, $b, off) }")
			h.L("		return off, nil")
			h.L("	}")
		}
	}
	g.Set("last", n-1).L("	if v.w != nil { v.w.noteFrontier(tid$viewTypeName, v.off, $last, off) }")
	g.L("	return off, nil")
	g.L("}")
}
