package main

import "fmt"

// This file contains shared emit helpers and concrete view type emitters.
// We generate per-type code rather than using Go generics because Go's shape
// stenciling compiles all struct-shaped view types into shared function bodies
// with indirect dictionary dispatch, adding ~25% overhead on traversal-heavy
// paths. Every emitted body is concrete.

// --- Shared emit helpers (used by struct, union, enum, array, opaque, optional emitters) ---

// emitParse emits the public ParseXView constructor — the single *walk
// allocation for the whole parse tree happens here and nowhere else — and its
// detached twin, which attaches no walk at all: the identical API runs with
// every record/frontier bookkeeping branch off, at thin-engine speed, for
// point-read workloads that would never amortize the fusion state. Zero
// allocations.
func emitParse(f *GeneratedFile, viewTypeName string) {
	f.Use("viewTypeName", viewTypeName).Block(`
		// Parse$viewTypeName wraps b (untrusted XDR bytes) in a $viewTypeName. Nothing is
		// validated up front; every access bounds-checks and validates what it reads.
		// Allocation-free.
		func Parse$viewTypeName(b []byte) $viewTypeName {
			return $viewTypeName{view{d: b}}
		}
	`)
}

// emitPublicMethods emits Raw(), Copy(), and ValidateFull() methods for a view
// type. slow types (O(n) sizing) route Raw/Copy through sizeResume, so a
// fully-consumed subtree trims in O(1) and the resolved extent is recorded
// into the walk for the enclosing traversal to consume.
func emitPublicMethods(f *GeneratedFile, viewTypeName string, slow bool) {
	_ = slow
	g := f.Use("viewTypeName", viewTypeName)
	g.Block(`
		// Raw returns the exact wire bytes for this view. Views delivered by
		// iterators and the Walk are already trimmed to their exact extent, so Raw
		// is a slice operation (the exact check precedes any sizing — arguments
		// are not evaluated for it); open-ended views (accessor/arm results) are
		// sized first.
		func (v $viewTypeName) Raw() ([]byte, error) {
			if v.exact {
				return v.d, nil
			}
			return v.trimmed(v.size(0))
		}
		// MustRaw is Raw panicking with the *ViewError sentinel (recover via Try*).
		func (v $viewTypeName) MustRaw() []byte {
			raw, err := v.Raw()
			if err != nil { mustView(err) }
			return raw
		}
		// Copy returns an independent, detached copy of this view that does not alias the original bytes.
		func (v $viewTypeName) Copy() ($viewTypeName, error) {
			if v.exact {
				c := make([]byte, len(v.d))
				copy(c, v.d)
				return $viewTypeName{view{d: c, exact: true}}, nil
			}
			nv, err := v.copied(v.size(0))
			return $viewTypeName{nv}, err
		}
		// ValidateFull checks that this view is well-formed: bounds, schema constraints, and depth limits.
		func (v $viewTypeName) ValidateFull() error { _, err := v.valid(0); return err }
	`)
}

// emitValueBasedValid emits a valid() that delegates to Value() for schema
// validation, then returns size(). Used by enums, fixed opaque, bounded
// opaque. The thin-engine form wraps the method (Value() lives on the view;
// these are O(1) leaves, so the one view construction is not a hot cost).
func emitValueBasedValid(f *GeneratedFile, typeName string) {
	f.Use("typeName", typeName).Block(`
		func valid$typeName(d []byte, depth int) (int, error) { return $typeName{view{d: d}}.valid(depth) }
		func (v $typeName) valid(_ int) (int, error) {
			if _, err := v.Value(); err != nil { return 0, err }
			return v.size(0)
		}
	`)
}

// PLAN A.5 (thin engine, fat surface): the blind sizing/validation engine is
// emitted as package-level functions over bare (d []byte, depth int) — the
// baseline named-[]byte call shape, with plain reslicing and no view
// construction — because it runs on millions of tiny leaf nodes. The fat view
// surface (methods, bundles, iterators, walk/frontier/sizeResume) is the
// rarely-called control plane; its blind fallbacks delegate to the thin
// engine.

func emitFixedSizeMethods(f *GeneratedFile, viewTypeName string, size uint32) {
	g := f.Use("viewTypeName", viewTypeName, "size", size)
	g.L("func size$viewTypeName(_ []byte, _ int) (int, error) { return $size, nil }")
	g.L("func (v $viewTypeName) size(depth int) (int, error) { return size$viewTypeName(v.d, depth) }")
}

// emitSizeTraversal emits thin-engine code that advances `off` past fields
// [0, end) for the blind size path, over a bare `d []byte` in scope.
// Fixed-size fields emit `off += N` (the compiler folds consecutive
// additions). Void-case-0 unions are inlined for the common extension-point
// pattern.
func emitSizeTraversal(f *GeneratedFile, fields []FieldPlan, errReturn string) {
	g := f.Use("errReturn", errReturn)
	for i := range fields {
		vt := fields[i].ViewType
		if fs, ok := vt.FixedSize(); ok {
			g.Set("fs", fs).L("	off += $fs")
			continue
		}
		g.L(`	if off > int64(len(d)) { return $errReturn, viewErrShortBuffer(uint32(off), "field offset exceeds data") }`)
		h := g.Set("fieldType", vt.GoType)
		if fields[i].IsVoidCase0 {
			h.Block(`
					{ fd := d[off:]
					if len(fd) >= 4 && binary.BigEndian.Uint32(fd[:4]) == 0 {
						off += 4
					} else {
						sz, err := size$fieldType(fd, depth+1)
						if err != nil { return $errReturn, err }
						off += int64(sz)
					} }
			`)
			continue
		}
		h.Block(`
				{ sz, err := size$fieldType(d[off:], depth+1)
				if err != nil { return $errReturn, err }
				off += int64(sz)
				if off > int64(len(d)) { return $errReturn, viewErrShortBuffer(uint32(off), "field offset exceeds data") } }
		`)
	}
	g.L(`	if off > int64(len(d)) { return $errReturn, viewErrShortBuffer(uint32(off), "field offset exceeds data") }`)
}

// emitValidTraversal emits thin-engine code that advances `off` past all
// fields for the valid() path, over a bare `d []byte` in scope.
func emitValidTraversal(f *GeneratedFile, fields []FieldPlan) {
	g := f.Use()
	for i := range fields {
		g.Set("fieldType", fields[i].ViewType.GoType).Block(`
				{ sz, err := valid$fieldType(d[off:], depth+1)
				if err != nil { return 0, err }
				off += int64(sz)
				if off > int64(len(d)) { return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data") } }
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

	// size — fixed-element arrays use O(1) shortcuts (thin engine)
	if isFixedElem && isVarCount {
		g.Block(`
			func size$typeName(d []byte, depth int) (int, error) {
				if depth > maxDepth { return 0, viewErrMaxDepth(0) }
				// Cheap unvalidated count: the total-vs-buffer check below bounds work
				// to O(buffer) on a bogus count, so the up-front min-size check is
				// redundant here. The OOM guard (for preallocation) lives in Len()/All().
				count, err := arrayViewCount(d, $maxLen)
				if err != nil { return 0, err }
				total := int64(4) + int64(count)*int64($elemSize)
				if total > int64(len(d)) { return 0, viewErrArrayCountExceedsData(4, count, len(d)-4) }
				return int(total), nil
			}
		`)
	} else if isFixedElem {
		g = g.Set("totalSize", elemSize*vt.Array.Count)
		g.L("func size$typeName(_ []byte, _ int) (int, error) { return $totalSize, nil }")
	}

	// size (variable-element only) + valid — thin engine, shared via prefix
	methods := []string{"valid"}
	if !isFixedElem {
		methods = append([]string{"size"}, methods...)
	}
	for _, m := range methods {
		h := g.Set("fn", m)
		h.L("func $fn$typeName(d []byte, depth int) (int, error) {")
		h.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
		if isVarCount {
			// size()/valid() read the cheap unvalidated count: arrayTraverse's
			// per-element bounds check caps work at O(buffer) on a bogus count, so the
			// up-front min-size check is redundant on this hot recursive path. The OOM
			// guard (for preallocation) lives in Len()/All().
			h.L("	count, err := arrayViewCount(d, $maxLen)")
			h.L("	if err != nil { return 0, err }")
		}
		h.L("	return arrayTraverse(d, $countExpr, $startOff, func(fd []byte) (int, error) { return $fn$elemType(fd, depth+1) })")
		h.L("}")
	}
	g.Block(`
		func (v $typeName) size(depth int) (int, error) { return size$typeName(v.d, depth) }
		func (v $typeName) valid(depth int) (int, error) { return valid$typeName(v.d, depth) }
	`)

	// All — the per-type iterator: yields elements in wire order, each sized
	// BEFORE the yield and TRIMMED to its exact extent (element Raw() is a
	// slice operation). On a malformed element it yields (zero view, error)
	// once and stops. NOTE: as with any iter.Seq2, a consumer that ranges
	// without binding the second variable silently discards errors — use
	// MustAll for the blessed single-variable form.
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
		g.L("			if !yield($elemType{view{d: v.d[off : off+$elemSize], exact: true}}, nil) { return }")
		g.L("			off += $elemSize")
	} else {
		g.L(`			if off >= int64(len(v.d)) { yield($elemType{}, viewErrShortBuffer(uint32(off), "element offset exceeds data")); return }`)
		g.L("			sz, err := size$elemType(v.d[off:], 0)")
		g.L("			if err != nil { yield($elemType{}, err); return }")
		g.L("			if !yield($elemType{view{d: v.d[off : off+int64(sz)], exact: true}}, nil) { return }")
		g.L("			off += int64(sz)")
	}
	g.L("		}")
	g.L("	}")
	g.L("}")
	// Scanner — the scanner-idiom iterator (Next/Cur/Err), per-iterator local
	// position only: a plain struct stepper with no closures and no
	// range-over-func machinery, for hot scan loops.
	g.Block(`
		// $typeNameScanner iterates $typeName with the scanner idiom: for
		// sc.Next() { sc.Cur() ... }; sc.Err(). Cur views are exact-extent.
		type $typeNameScanner struct {
			d   []byte
			off int64
			n   int
			i   int
			err error
			cur $elemType
		}
		// Scan returns a scanner over this array's elements.
		func (v $typeName) Scan() $typeNameScanner {
			sc := $typeNameScanner{d: v.d}
	`)
	if isVarCount {
		g.Block(`
			n, err := arrayViewCountChecked(v.d, $maxLen, $minElemW)
			if err != nil {
				sc.err = err
				return sc
			}
			sc.n = n
		`)
	} else {
		g.L("	sc.n = $count")
	}
	g.Set("scanStart", startOff).Block(`
			sc.off = $scanStart
			return sc
		}
		// Next advances to the next element; false at the end or on error (check Err).
		func (sc *$typeNameScanner) Next() bool {
			if sc.err != nil || sc.i >= sc.n {
				return false
			}
	`)
	if isFixedElem {
		g.Block(`
			if sc.off+$elemSize > int64(len(sc.d)) {
				sc.err = viewErrShortBuffer(uint32(sc.off), "need $elemSize bytes")
				return false
			}
			sc.cur = $elemType{view{d: sc.d[sc.off : sc.off+$elemSize], exact: true}}
			sc.off += $elemSize
		`)
	} else {
		g.Block(`
			if sc.off >= int64(len(sc.d)) {
				sc.err = viewErrShortBuffer(uint32(sc.off), "element offset exceeds data")
				return false
			}
			sz, err := size$elemType(sc.d[sc.off:], 0)
			if err != nil {
				sc.err = err
				return false
			}
			sc.cur = $elemType{view{d: sc.d[sc.off : sc.off+int64(sz)], exact: true}}
			sc.off += int64(sz)
		`)
	}
	g.Block(`
			sc.i++
			return true
		}
		// Cur returns the element Next positioned on (exact extent).
		func (sc *$typeNameScanner) Cur() $elemType { return sc.cur }
		// Err returns the sticky error that stopped Next, if any.
		func (sc *$typeNameScanner) Err() error { return sc.err }
	`)

	// MustAll — the blessed single-variable iteration: panics with the
	// *ViewError sentinel on a malformed element (recover via Try*).
	g.L("// MustAll is All with in-band errors converted to a Must panic (the *ViewError")
	g.L("// sentinel, recovered by Try*): the blessed single-variable range form.")
	g.L("func (v $typeName) MustAll() iter.Seq[$elemType] {")
	g.L("	return func(yield func($elemType) bool) {")
	g.L("		for ev, err := range v.All() {")
	g.L("			if err != nil { mustView(err) }")
	g.L("			if !yield(ev) { return }")
	g.L("		}")
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

	g.Block(`
		// Unwrap reads the presence flag: (zero, false, nil) when absent, the inner
		// view and true when present, an error when the flag is malformed.
		func (v $typeName) Unwrap() ($innerType, bool, error) {
			if len(v.d) < 4 { return $innerType{}, false, viewErrShortBuffer(0, "need 4 bytes for optional flag") }
			flag := binary.BigEndian.Uint32(v.d[:4])
			switch flag {
			case 0: return $innerType{}, false, nil
			case 1: return $innerType{view{d: v.d[4:]}}, true, nil
			default: return $innerType{}, false, viewErrBadBoolValue(0, flag)
			}
		}
		// MustUnwrap is Unwrap panicking with the *ViewError sentinel (recover via Try*).
		func (v $typeName) MustUnwrap() ($innerType, bool) {
			iv, present, err := v.Unwrap()
			if err != nil { mustView(err) }
			return iv, present
		}
	`)
	for _, m := range []string{"size", "valid"} {
		g.Set("fn", m).Block(`
			func $fn$typeName(d []byte, depth int) (int, error) {
				if depth > maxDepth { return 0, viewErrMaxDepth(0) }
				if len(d) < 4 { return 0, viewErrShortBuffer(0, "need 4 bytes for optional flag") }
				flag := binary.BigEndian.Uint32(d[:4])
				switch flag {
				case 0: return 4, nil
				case 1:
					sz, err := $fn$innerType(d[4:], depth+1)
					if err != nil { return 0, err }
					return 4 + sz, nil
				default: return 0, viewErrBadBoolValue(0, flag)
				}
			}
			func (v $typeName) $fn(depth int) (int, error) { return $fn$typeName(v.d, depth) }
		`)
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
		h.L("func size$typeName(_ []byte, _ int) (int, error) { return $paddedSize, nil }")
		h.L("func (v $typeName) size(depth int) (int, error) { return size$typeName(v.d, depth) }")
		h.Block(`
			// MustValue is Value panicking with the *ViewError sentinel (recover via Try*).
			func (v $typeName) MustValue() $valGoType {
				val, err := v.Value()
				if err != nil { mustView(err) }
				return val
			}
		`)
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
		g.L("func size$typeName(d []byte, depth int) (int, error) { return sizeVarOpaqueView(d, depth) }")
		g.L("func (v $typeName) size(depth int) (int, error) { return size$typeName(v.d, depth) }")
		g.Block(`
			// MustValue is Value panicking with the *ViewError sentinel (recover via Try*).
			func (v $typeName) MustValue() []byte {
				val, err := v.Value()
				if err != nil { mustView(err) }
				return val
			}
		`)
		emitValueBasedValid(f, typeName)
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
		func size$viewName(_ []byte, _ int) (int, error) { return 4, nil }
		func (v $viewName) size(depth int) (int, error) { return size$viewName(v.d, depth) }
		// MustValue is Value panicking with the *ViewError sentinel (recover via Try*).
		func (v $viewName) MustValue() $enumName {
			val, err := v.Value()
			if err != nil { mustView(err) }
			return val
		}
	`)
	emitValueBasedValid(f, ep.ViewTypeName)
	emitPublicMethods(f, ep.ViewTypeName, false)
}

// emitTypedefViewFromPlan emits a typedef alias plus its Parse constructor,
// and thin-engine wrappers under the alias name (field types may reference
// the alias; functions cannot be aliased, so one-line delegates stand in).
func emitTypedefViewFromPlan(f *GeneratedFile, tp *TypedefViewPlan) {
	if tp.ViewType.GoType == tp.AliasName {
		return
	}
	p := f.Use("aliasName", tp.AliasName, "goType", tp.ViewType.GoType)
	p.L("type $aliasName = $goType")
	p.L("func size$aliasName(d []byte, depth int) (int, error) { return size$goType(d, depth) }")
	p.L("func valid$aliasName(d []byte, depth int) (int, error) { return valid$goType(d, depth) }")
	emitParse(f, tp.AliasName)
}
