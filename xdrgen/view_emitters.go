package main

import "fmt"

// This file contains shared emit helpers and concrete view type emitters.
// We generate per-type code rather than using Go generics because Go's shape
// stenciling compiles all struct-shaped view types into shared function bodies
// with indirect dictionary dispatch, adding ~25% overhead on traversal-heavy
// paths. Every emitted body is concrete.

// --- Shared emit helpers (used by struct, union, enum, array, opaque, optional emitters) ---

// emitParse emits the public ParseXView constructor: the single *walk
// allocation for the whole parse tree happens here and nowhere else.
func emitParse(f *GeneratedFile, viewTypeName string) {
	f.Use("viewTypeName", viewTypeName).Block(`
		// Parse$viewTypeName wraps b (untrusted XDR bytes) in a $viewTypeName sharing one
		// fresh walk across the whole parse tree. Nothing is validated up front; every
		// access bounds-checks and validates what it reads.
		func Parse$viewTypeName(b []byte) $viewTypeName {
			return $viewTypeName{view{d: b, w: newWalk()}}
		}
	`)
}

// emitTidConst emits the walk type-identity constant for a slow-sized type.
func emitTidConst(f *GeneratedFile, viewTypeName string, tid int) {
	f.Use("viewTypeName", viewTypeName, "tid", tid).
		L("const tid$viewTypeName = $tid // walk record type identity")
}

// emitPublicMethods emits Raw(), Copy(), and ValidateFull() methods for a view
// type. slow types (O(n) sizing) route Raw/Copy through sizeResume, so a
// fully-consumed subtree trims in O(1) and the resolved extent is recorded
// into the walk for the enclosing traversal to consume.
func emitPublicMethods(f *GeneratedFile, viewTypeName string, slow bool) {
	sizeCall := "v.size(0)"
	if slow {
		sizeCall = "v.sizeResume(0)"
	}
	g := f.Use("viewTypeName", viewTypeName, "sizeCall", sizeCall)
	g.Block(`
		// Raw returns the exact wire bytes for this view, trimmed from the open-ended window.
		func (v $viewTypeName) Raw() ([]byte, error) { return v.trimmed($sizeCall) }
		// Copy returns an independent, detached copy of this view that does not alias the original bytes.
		func (v $viewTypeName) Copy() ($viewTypeName, error) {
			nv, err := v.copied($sizeCall)
			return $viewTypeName{nv}, err
		}
		// ValidateFull checks that this view is well-formed: bounds, schema constraints, and depth limits.
		func (v $viewTypeName) ValidateFull() error { _, err := v.valid(0); return err }
	`)
}

// emitValueBasedValid emits a valid() that delegates to Value() for schema
// validation, then returns size(). Used by enums, fixed opaque, bounded opaque.
func emitValueBasedValid(f *GeneratedFile, typeName string) {
	f.Use("typeName", typeName).Block(`
		func (v $typeName) valid(_ int) (int, error) {
			if _, err := v.Value(); err != nil { return 0, err }
			return v.size(0)
		}
	`)
}

func emitFixedSizeMethods(f *GeneratedFile, viewTypeName string, size uint32) {
	g := f.Use("viewTypeName", viewTypeName, "size", size)
	g.L("func (v $viewTypeName) size(_ int) (int, error) { return $size, nil }")
}

// emitSizeResumeAlias emits the trivial sizeResume for variable-size types
// whose size() is O(1): there is no interior walk to resume, so the walk
// record is neither consulted nor written.
func emitSizeResumeAlias(f *GeneratedFile, viewTypeName string) {
	f.Use("viewTypeName", viewTypeName).
		L("func (v $viewTypeName) sizeResume(depth int) (int, error) { return v.size(depth) }")
}

// emitSizeTraversal emits code that advances `off` past fields [0, end) for
// blind size paths. Fixed-size fields emit `off += N` (the compiler folds
// consecutive additions). Void-case-0 unions are inlined for the common
// extension-point pattern.
func emitSizeTraversal(f *GeneratedFile, fields []FieldPlan, call, errReturn string) {
	g := f.Use("call", call, "errReturn", errReturn)
	for i := range fields {
		vt := fields[i].ViewType
		if fs, ok := vt.FixedSize(); ok {
			g.Set("fs", fs).L("	off += $fs")
			continue
		}
		g.L(`	if off > int64(len(v.d)) { return $errReturn, viewErrShortBuffer(uint32(off), "field offset exceeds data") }`)
		h := g.Set("fieldType", vt.GoType)
		if fields[i].IsVoidCase0 {
			h.Block(`
					{ d := v.d[off:]
					if len(d) >= 4 && binary.BigEndian.Uint32(d[:4]) == 0 {
						off += 4
					} else {
						sz, err := $fieldType{view{d: d}}.$call
						if err != nil { return $errReturn, err }
						off += int64(sz)
					} }
			`)
			continue
		}
		h.Block(`
				{ sz, err := $fieldType{view{d: v.d[off:]}}.$call
				if err != nil { return $errReturn, err }
				off += int64(sz)
				if off > int64(len(v.d)) { return $errReturn, viewErrShortBuffer(uint32(off), "field offset exceeds data") } }
		`)
	}
	g.L(`	if off > int64(len(v.d)) { return $errReturn, viewErrShortBuffer(uint32(off), "field offset exceeds data") }`)
}

// emitResumeAdvance emits the walk-gated advance over one variable-size field
// or element of type vt at relative offset `off` (int64) within receiver `v`:
// if the walk's record could still lie at or ahead of this position
// (w.recStart >= absolute position), the child is sized through its
// sizeResume — which consumes an exact-match record in O(1) and catches up
// from interior records — otherwise through its blind size(). Every advance
// is bounds-checked; overflowGuard additionally rejects offsets that cannot
// be memoized as int32.
func emitResumeAdvance(f *GeneratedFile, vt *ViewType, isVoidCase0 bool, childDepth, errReturn string, overflowGuard bool) {
	guard := ""
	if overflowGuard {
		guard = " || off > maxViewOff"
	}
	g := f.Use("fieldType", vt.GoType, "childDepth", childDepth, "errReturn", errReturn, "guard", guard)
	g.L(`	if off > int64(len(v.d)) { return $errReturn, viewErrShortBuffer(uint32(off), "field offset exceeds data") }`)
	if isVoidCase0 {
		g.Block(`
			{ d := v.d[off:]
			var sz int
			var err error
			if len(d) >= 4 && binary.BigEndian.Uint32(d[:4]) == 0 {
				sz = 4
			} else if v.w != nil && v.w.recStart >= v.off+off {
				sz, err = $fieldType{v.sub(off)}.sizeResume($childDepth)
			} else {
				sz, err = $fieldType{view{d: d}}.size($childDepth)
			}
			if err != nil { return $errReturn, err }
			off += int64(sz)
			if off > int64(len(v.d))$guard { return $errReturn, viewErrShortBuffer(uint32(off), "field offset exceeds data") } }
		`)
		return
	}
	g.Block(`
		{ var sz int
		var err error
		if v.w != nil && v.w.recStart >= v.off+off {
			sz, err = $fieldType{v.sub(off)}.sizeResume($childDepth)
		} else {
			sz, err = $fieldType{view{d: v.d[off:]}}.size($childDepth)
		}
		if err != nil { return $errReturn, err }
		off += int64(sz)
		if off > int64(len(v.d))$guard { return $errReturn, viewErrShortBuffer(uint32(off), "field offset exceeds data") } }
	`)
}

// emitValidTraversal emits code that advances `off` past all fields for the valid() path.
func emitValidTraversal(f *GeneratedFile, fields []FieldPlan) {
	g := f.Use()
	for i := range fields {
		g.Set("fieldType", fields[i].ViewType.GoType).Block(`
				{ sz, err := $fieldType{view{d: v.d[off:]}}.valid(depth + 1)
				if err != nil { return 0, err }
				off += int64(sz)
				if off > int64(len(v.d)) { return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data") } }
		`)
	}
}

// --- Concrete view type emitters (arrays, optionals, opaque) ---

// emitConcreteViewType emits a concrete view type (array, opaque, or optional)
// with the given name. The plan phase determines which types need emission;
// this is the dispatch point for the actual code generation.
func emitConcreteViewType(f *GeneratedFile, ip *InlineTypePlan) error {
	switch ip.ViewType.Kind {
	case VKArray:
		return emitArrayType(f, ip)
	case VKOpaque:
		emitOpaqueType(f, ip)
	case VKOptional:
		emitOptionalType(f, ip)
	}
	return nil
}

// arrayMinElemW returns the count-validation divisor (the element's clamped
// minimum wire size) for an array view. It returns an error when the
// element computes a zero minimum, which would make the O(1) count bound
// unsound. A zero-size fixed element (e.g. opaque[0]) is rejected loudly upstream
// at BuildViewType, so it never reaches here with a zero computed minimum.
func arrayMinElemW(ip *InlineTypePlan) (uint32, error) {
	if ip.ElemMinWidthComputed == 0 {
		return 0, fmt.Errorf("array view %s: element type %s has zero minimum wire size; "+
			"count validation bound would be unsound", ip.Name, ip.ViewType.Array.Element.GoType)
	}
	return ip.ElemMinWidth, nil
}

// emitArrayType generates a complete concrete array view type.
// Fixed vs variable count is determined by vt.Array.Count (> 0 = fixed).
// Fixed vs variable elements is determined by vt.Array.Element.FixedSize().
func emitArrayType(f *GeneratedFile, ip *InlineTypePlan) error {
	typeName := ip.Name
	vt := ip.ViewType
	elemType := vt.Array.Element.GoType
	elemSize, isFixedElem := vt.Array.Element.FixedSize()
	isVarCount := vt.Array.Count == 0
	slow := !isFixedElem

	startOff := 0
	if isVarCount {
		startOff = 4
	}

	var countExpr any = vt.Array.Count
	if isVarCount {
		countExpr = "count"
	}

	minElemW, err := arrayMinElemW(ip)
	if err != nil {
		return err
	}

	g := f.Use(
		"typeName", typeName,
		"elemType", elemType,
		"elemSize", elemSize,
		"minElemW", minElemW,
		"startOff", startOff,
		"countExpr", countExpr,
		"maxLen", vt.Array.MaxLen,
		"count", vt.Array.Count,
	)

	g.L("type $typeName struct{ view }")
	emitParse(f, typeName)
	if slow {
		emitTidConst(f, typeName, ip.Tid)
	}

	// Len — O(1) validated count. For variable-count arrays the wire count is
	// checked against BOTH the schema maximum and the remaining buffer (the OOM
	// guard): a standalone Len() must never return an unvalidated wire count.
	if isVarCount {
		g.Block(`
			// Len returns the element count (O(1), from the count prefix), validated
			// against the schema bound and the remaining buffer before use.
			func (v $typeName) Len() (uint32, error) {
				n, err := arrayViewCountChecked(v.d, $maxLen, $minElemW)
				return uint32(n), err
			}
		`)
	} else {
		g.L("// Len returns the fixed element count of this array.")
		g.L("func (v $typeName) Len() (uint32, error) { return $count, nil }")
	}

	// size — fixed-element arrays use O(1) shortcuts
	if isFixedElem && isVarCount {
		g.Block(`
			func (v $typeName) size(depth int) (int, error) {
				if depth > maxDepth { return 0, viewErrMaxDepth(0) }
				// Cheap unvalidated count: the total-vs-buffer check below bounds work
				// to O(buffer) on a bogus count, so the up-front min-size check is
				// redundant here. The OOM guard (for preallocation) lives in Len()/All().
				count, err := arrayViewCount(v.d, $maxLen)
				if err != nil { return 0, err }
				total := int64(4) + int64(count)*int64($elemSize)
				if total > int64(len(v.d)) { return 0, viewErrArrayCountExceedsData(4, count, len(v.d)-4) }
				return int(total), nil
			}
		`)
	} else if isFixedElem {
		g = g.Set("totalSize", elemSize*vt.Array.Count)
		g.L("func (v $typeName) size(_ int) (int, error) { return $totalSize, nil }")
	}

	// size (variable-element only) + valid — shared via call parameter
	methods := []struct{ name, call string }{{"valid", "valid(depth + 1)"}}
	if !isFixedElem {
		methods = append([]struct{ name, call string }{{"size", "size(depth + 1)"}}, methods...)
	}
	for _, m := range methods {
		h := g.Set("method", m.name).Set("call", m.call)
		h.L("func (v $typeName) $method(depth int) (int, error) {")
		h.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
		if isVarCount {
			// size()/valid() read the cheap unvalidated count: arrayTraverse's
			// per-element bounds check caps work at O(buffer) on a bogus count, so the
			// up-front min-size check is redundant on this hot recursive path. The OOM
			// guard (for preallocation) lives in Len()/All().
			h.L("	count, err := arrayViewCount(v.d, $maxLen)")
			h.L("	if err != nil { return 0, err }")
		}
		h.L("	return arrayTraverse(v.d, $countExpr, $startOff, func(d []byte) (int, error) { return $elemType{view{d: d}}.$call })")
		h.L("}")
	}

	// sizeResume — walk-assisted sizing for O(n) arrays; trivial alias otherwise.
	if slow {
		g.L("// sizeResume is size() with walk assistance: a record covering exactly this")
		g.L("// array returns its extent in O(1); records inside it resume element sizing")
		g.L("// past already-resolved bytes; a frontier entry left by All() restarts the")
		g.L("// element loop at the last yielded element (so a loop broken early still")
		g.L("// resumes); on completion the array's span is recorded.")
		g.L("func (v $typeName) sizeResume(depth int) (int, error) {")
		g.L("	if v.w != nil {")
		g.L("		if end, ok := v.w.hit(tid$typeName, v.off, v.off+int64(len(v.d))); ok { return int(end - v.off), nil }")
		g.L("	}")
		g.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
		if isVarCount {
			g.L("	count, err := arrayViewCount(v.d, $maxLen)")
			g.L("	if err != nil { return 0, err }")
		}
		g.L("	off := int64($startOff)")
		g.L("	k := 0")
		g.L("	if v.w != nil {")
		g.L("		if fi, fo, ok := v.w.frontier(tid$typeName, v.off, int64(len(v.d))); ok && int(fi) < $countExpr {")
		g.L("			k, off = int(fi), fo")
		g.L("		}")
		g.L("	}")
		g.L("	for ; k < $countExpr; k++ {")
		emitResumeAdvance(f, vt.Array.Element, false, "depth + 1", "0", false)
		g.L("	}")
		g.L("	if v.w != nil { v.w.record(tid$typeName, v.off, v.off+off) }")
		g.L("	return int(off), nil")
		g.L("}")
	} else {
		emitSizeResumeAlias(f, typeName)
	}

	// All — the per-type iterator: yields elements in wire order with in-band,
	// unskippable errors. Elements are open-ended sub-views (extent stays lazy).
	g.L("// All iterates this array's elements in wire order. Elements yield with a nil")
	g.L("// error; on the first malformed element it yields (zero view, error) once and")
	g.L("// stops, so errors are in-band and cannot be skipped.")
	if slow {
		g.L("// Advancing past an element consumes walk records left by fully consuming the")
		g.L("// element's interior (O(1) instead of a re-walk), each yielded element notes")
		g.L("// the array's frontier (so a loop broken early resumes where it stopped), and")
		g.L("// completing the iteration records the whole array's span for the enclosing")
		g.L("// traversal.")
	}
	g.L("func (v $typeName) All() iter.Seq2[$elemType, error] {")
	g.L("	return func(yield func($elemType, error) bool) {")
	if isVarCount {
		g.L("		count, err := arrayViewCountChecked(v.d, $maxLen, $minElemW)")
		g.L("		if err != nil { yield($elemType{}, err); return }")
	}
	g.L("		off := int64($startOff)")
	g.L("		for k := 0; k < $countExpr; k++ {")
	if isFixedElem {
		g.L(`			if off+$elemSize > int64(len(v.d)) { yield($elemType{}, viewErrShortBuffer(uint32(off), "need $elemSize bytes")); return }`)
		g.L("			if !yield($elemType{v.sub(off)}, nil) { return }")
		g.L("			off += $elemSize")
	} else {
		g.L(`			if off >= int64(len(v.d)) { yield($elemType{}, viewErrShortBuffer(uint32(off), "element offset exceeds data")); return }`)
		g.L("			elem := $elemType{v.sub(off)}")
		g.L("			// Note frontier progress before yielding: elements 0..k-1 span")
		g.L("			// [array start, off), so a consumer that stops here leaves the")
		g.L("			// array resumable at element k. k == 0 carries no progress (and")
		g.L("			// idx > 0 is what distinguishes a live slot), so it is not noted.")
		g.L("			if v.w != nil && k > 0 { v.w.noteFrontier(tid$typeName, v.off, int32(k), off) }")
		g.L("			if !yield(elem, nil) { return }")
		g.L("			var sz int")
		g.L("			var err error")
		g.L("			// Advance through sizeResume when a record lies ahead OR any")
		g.L("			// frontier entry exists — a bundle-only consumer (no Raw/All on")
		g.L("			// the interior) leaves frontier progress but no leaf record, and")
		g.L("			// the element's own entry is what spares re-sizing its fields.")
		g.L("			if v.w != nil && (v.w.recStart >= elem.off || v.w.frLive) {")
		g.L("				sz, err = elem.sizeResume(0)")
		g.L("			} else {")
		g.L("				sz, err = $elemType{view{d: v.d[off:]}}.size(0)")
		g.L("			}")
		g.L("			if err != nil { yield($elemType{}, err); return }")
		g.L("			off += int64(sz)")
	}
	g.L("		}")
	if slow {
		g.L("		if v.w != nil { v.w.record(tid$typeName, v.off, v.off+off) }")
	}
	g.L("	}")
	g.L("}")

	emitPublicMethods(f, typeName, slow)
	return nil
}

// emitOptionalType generates a concrete optional view type.
func emitOptionalType(f *GeneratedFile, ip *InlineTypePlan) {
	typeName := ip.Name
	vt := ip.ViewType
	innerType := vt.Optional.Element.GoType
	_, fixedInner := vt.Optional.Element.FixedSize()
	slow := !fixedInner

	g := f.Use("typeName", typeName, "innerType", innerType)
	g.L("type $typeName struct{ view }")
	emitParse(f, typeName)
	if slow {
		emitTidConst(f, typeName, ip.Tid)
	}

	g.Block(`
		// Unwrap reads the presence flag: (zero, false, nil) when absent, the inner
		// view and true when present, an error when the flag is malformed.
		func (v $typeName) Unwrap() ($innerType, bool, error) {
			if len(v.d) < 4 { return $innerType{}, false, viewErrShortBuffer(0, "need 4 bytes for optional flag") }
			flag := binary.BigEndian.Uint32(v.d[:4])
			switch flag {
			case 0: return $innerType{}, false, nil
			case 1: return $innerType{v.sub(4)}, true, nil
			default: return $innerType{}, false, viewErrBadBoolValue(0, flag)
			}
		}
	`)
	for _, m := range []struct{ name, call string }{{"size", "size(depth + 1)"}, {"valid", "valid(depth + 1)"}} {
		g.Set("method", m.name).Set("call", m.call).Block(`
			func (v $typeName) $method(depth int) (int, error) {
				if depth > maxDepth { return 0, viewErrMaxDepth(0) }
				if len(v.d) < 4 { return 0, viewErrShortBuffer(0, "need 4 bytes for optional flag") }
				flag := binary.BigEndian.Uint32(v.d[:4])
				switch flag {
				case 0: return 4, nil
				case 1:
					sz, err := $innerType{view{d: v.d[4:]}}.$call
					if err != nil { return 0, err }
					return 4 + sz, nil
				default: return 0, viewErrBadBoolValue(0, flag)
				}
			}
		`)
	}
	if slow {
		g.Block(`
			// sizeResume is size() with walk assistance; see the struct sizeResume contract.
			func (v $typeName) sizeResume(depth int) (int, error) {
				if v.w != nil {
					if end, ok := v.w.hit(tid$typeName, v.off, v.off+int64(len(v.d))); ok { return int(end - v.off), nil }
				}
				if depth > maxDepth { return 0, viewErrMaxDepth(0) }
				if len(v.d) < 4 { return 0, viewErrShortBuffer(0, "need 4 bytes for optional flag") }
				flag := binary.BigEndian.Uint32(v.d[:4])
				switch flag {
				case 0: return 4, nil
				case 1:
					var sz int
					var err error
					if v.w != nil && v.w.recStart >= v.off+4 {
						sz, err = $innerType{v.sub(4)}.sizeResume(depth + 1)
					} else {
						sz, err = $innerType{view{d: v.d[4:]}}.size(depth + 1)
					}
					if err != nil { return 0, err }
					if v.w != nil { v.w.record(tid$typeName, v.off, v.off+int64(4+sz)) }
					return 4 + sz, nil
				default: return 0, viewErrBadBoolValue(0, flag)
				}
			}
		`)
	} else {
		emitSizeResumeAlias(f, typeName)
	}
	emitPublicMethods(f, typeName, slow)
}

// emitOpaqueType generates a concrete opaque view type.
// Fixed (RawSize > 0): constant size, padding validation. Value() returns the
// schema Go type the struct decoder produces — a typed [N]byte array (Hash,
// [4]byte, ...) BY VALUE (a copy), not an aliasing []byte.
// Variable bounded (MaxLen > 0): delegates to VarOpaqueView, enforces max length;
// Value() returns an aliasing []byte.
func emitOpaqueType(f *GeneratedFile, ip *InlineTypePlan) {
	typeName := ip.Name
	vt := ip.ViewType
	g := f.Use("typeName", typeName)
	g.L("type $typeName struct{ view }")
	emitParse(f, typeName)

	if vt.Opaque.RawSize > 0 {
		paddedSize, _ := vt.FixedSize()
		valGoType := ip.OpaqueValueGoType
		if valGoType == "" {
			valGoType = fmt.Sprintf("[%d]byte", vt.Opaque.RawSize)
		}
		h := g.Set("paddedSize", paddedSize).Set("rawSize", vt.Opaque.RawSize).Set("valGoType", valGoType)
		if vt.Opaque.RawSize%4 != 0 {
			h = h.Set("padLen", paddedSize-vt.Opaque.RawSize)
			h.Block(`
				func (v $typeName) Value() ($valGoType, error) {
					var out $valGoType
					if len(v.d) < $paddedSize { return out, viewErrShortBuffer(0, "need $paddedSize bytes") }
					if !bytes.Equal(v.d[$rawSize:$paddedSize], zeroPad[:$padLen]) {
						return out, viewErrNonZeroPadding($rawSize)
					}
					copy(out[:], v.d[:$rawSize])
					return out, nil
				}
			`)
		} else {
			h.Block(`
				func (v $typeName) Value() ($valGoType, error) {
					var out $valGoType
					if len(v.d) < $paddedSize { return out, viewErrShortBuffer(0, "need $paddedSize bytes") }
					copy(out[:], v.d[:$rawSize])
					return out, nil
				}
			`)
		}
		h.L("func (v $typeName) size(_ int) (int, error) { return $paddedSize, nil }")
		emitValueBasedValid(f, typeName)
	} else {
		g = g.Set("maxLen", vt.Opaque.MaxLen)
		g.Block(`
			func (v $typeName) Value() ([]byte, error) {
				val, err := VarOpaqueView{v.view}.Value()
				if err != nil { return nil, err }
				if len(val) > $maxLen { return nil, viewErrOpaqueExceedsMax(0, uint32(len(val)), $maxLen) }
				return val, nil
			}
		`)
		g.L("func (v $typeName) size(depth int) (int, error) { return VarOpaqueView{v.view}.size(depth) }")
		emitValueBasedValid(f, typeName)
		emitSizeResumeAlias(f, typeName)
	}

	emitPublicMethods(f, typeName, false)
}

// emitEnumViewFromPlan emits an enum view type.
func emitEnumViewFromPlan(f *GeneratedFile, ep *EnumViewPlan) {
	p := f.Use("viewName", ep.ViewTypeName, "enumName", ep.EnumName, "caseNames", joinComma(ep.CaseNames))
	p.L("type $viewName struct{ view }")
	emitParse(f, ep.ViewTypeName)
	p.Block(`
		func (v $viewName) Value() ($enumName, error) {
			if len(v.d) < 4 { return 0, viewErrShortBuffer(0, "need 4 bytes") }
			val := $enumName(int32(binary.BigEndian.Uint32(v.d[:4])))
			switch val {
			case $caseNames:
				return val, nil
			default:
				return 0, viewErrUnknownDiscriminant(0, int32(val))
			}
		}
		func (v $viewName) size(_ int) (int, error) { return 4, nil }
	`)
	emitValueBasedValid(f, ep.ViewTypeName)
	emitPublicMethods(f, ep.ViewTypeName, false)
}

// emitTypedefViewFromPlan emits a typedef alias plus its Parse constructor.
func emitTypedefViewFromPlan(f *GeneratedFile, tp *TypedefViewPlan) {
	if tp.ViewType.GoType == tp.AliasName {
		return
	}
	p := f.Use("aliasName", tp.AliasName, "goType", tp.ViewType.GoType)
	p.L("type $aliasName = $goType")
	emitParse(f, tp.AliasName)
}
