package main

import "fmt"

// This file contains shared emit helpers and concrete view type emitters.
// We generate per-type code rather than using Go generics because Go's shape
// stenciling compiles all ~[]byte types into a single shared function body with
// indirect dictionary dispatch, adding ~25% overhead on traversal-heavy paths.

// --- Shared emit helpers (used by struct, union, enum, array, opaque, optional emitters) ---

// emitPublicMethods emits Raw(), Copy(), and ValidateFull() methods for a view type.
func emitPublicMethods(f *GeneratedFile, viewTypeName string) {
	g := f.Use("viewTypeName", viewTypeName)
	g.Block(`
		// Raw returns the exact wire bytes for this view, trimmed from the fat slice.
		func (v $viewTypeName) Raw() ([]byte, error) { return viewRaw(v) }
		// Copy returns an independent copy of this view that does not alias the original bytes.
		func (v $viewTypeName) Copy() ($viewTypeName, error) { return viewCopy(v) }
		// ValidateFull checks that this view is well-formed: bounds, schema constraints, and depth limits.
		func (v $viewTypeName) ValidateFull() error { return validate(v) }
		func (v $viewTypeName) MustRaw() []byte { return must(v.Raw()) }
		func (v $viewTypeName) MustCopy() $viewTypeName { return must(v.Copy()) }
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

// emitSizeTraversal emits code that advances `off` past fields [0, end) for size/accessor paths.
// Fixed-size fields emit `off += N` (the compiler folds consecutive additions).
// Void-case-0 unions are inlined for the common extension-point pattern.
func emitSizeTraversal(f *GeneratedFile, fields []FieldPlan, call, errReturn string) {
	g := f.Use("call", call, "errReturn", errReturn)
	for i := range fields {
		vt := fields[i].ViewType
		if fs, ok := vt.FixedSize(); ok {
			g.Set("fs", fs).L("	off += $fs")
			continue
		}
		g.L(`	if off > int64(len(v)) { return $errReturn, viewErrShortBuffer(uint32(off), "field offset exceeds data") }`)
		h := g.Set("fieldType", vt.GoType)
		if fields[i].IsVoidCase0 {
			h.Block(`
					{ d := []byte(v)[off:]
					if len(d) >= 4 && binary.BigEndian.Uint32(d[:4]) == 0 {
						off += 4
					} else {
						sz, err := $fieldType(d).$call
						if err != nil { return $errReturn, err }
						off += int64(sz)
					} }
			`)
			continue
		}
		h.Block(`
				{ sz, err := $fieldType(v[off:]).$call
				if err != nil { return $errReturn, err }
				off += int64(sz)
				if off > int64(len(v)) { return $errReturn, viewErrShortBuffer(uint32(off), "field offset exceeds data") } }
		`)
	}
	g.L(`	if off > int64(len(v)) { return $errReturn, viewErrShortBuffer(uint32(off), "field offset exceeds data") }`)
}

// fieldsBundleFieldName returns the Go field name to use for a struct field
// inside its Fields bundle, escaping the reserved bundle field "View".
func fieldsBundleFieldName(name string) string {
	if name == "View" {
		return "View_"
	}
	return name
}

// emitLocateTraversal emits the body of a variable-size struct's locate helper:
// one walk over all fields capturing each field's trimmed extent into the bundle
// `f`, mirroring emitSizeTraversal's advance logic but keeping the offsets.
// On entry `off` is int64(0); on exit `off` is the node's total extent, and
// every f.<Field> has been set to a trimmed sub-view v[start:end].
func emitLocateTraversal(f *GeneratedFile, fields []FieldPlan) {
	g := f.Use()
	for i := range fields {
		vt := fields[i].ViewType
		bundleName := fieldsBundleFieldName(fields[i].FieldName)
		h := g.Set("bundleName", bundleName).Set("fieldType", vt.GoType)
		if fs, ok := vt.FixedSize(); ok {
			// Fixed-size field: extent is a compile-time constant.
			h.Set("fs", fs).Block(`
					if off+$fs > int64(len(v)) { return f, viewErrShortBuffer(uint32(off), "field offset exceeds data") }
					f.$bundleName = $fieldType(v[off : off+$fs])
					off += $fs
			`)
			continue
		}
		h.L(`	if off > int64(len(v)) { return f, viewErrShortBuffer(uint32(off), "field offset exceeds data") }`)
		if fields[i].IsVoidCase0 {
			h.Block(`
					{ d := []byte(v)[off:]
					var fsz int64
					if len(d) >= 4 && binary.BigEndian.Uint32(d[:4]) == 0 {
						fsz = 4
					} else {
						sz, err := $fieldType(d).size(0)
						if err != nil { return f, err }
						fsz = int64(sz)
					}
					if off+fsz > int64(len(v)) { return f, viewErrShortBuffer(uint32(off), "field offset exceeds data") }
					f.$bundleName = $fieldType(v[off : off+fsz])
					off += fsz }
			`)
			continue
		}
		h.Block(`
				{ sz, err := $fieldType(v[off:]).size(0)
				if err != nil { return f, err }
				fsz := int64(sz)
				if off+fsz > int64(len(v)) { return f, viewErrShortBuffer(uint32(off), "field offset exceeds data") }
				f.$bundleName = $fieldType(v[off : off+fsz])
				off += fsz }
		`)
	}
	g.L(`	if off > int64(len(v)) { return f, viewErrShortBuffer(uint32(off), "field offset exceeds data") }`)
}

// emitValidTraversal emits code that advances `off` past fields [0, end) for the valid() path.
func emitValidTraversal(f *GeneratedFile, fields []FieldPlan) {
	g := f.Use()
	for i := range fields {
		g.Set("fieldType", fields[i].ViewType.GoType).Block(`
				{ sz, err := $fieldType(v[off:]).valid(depth + 1)
				if err != nil { return 0, err }
				off += int64(sz)
				if off > int64(len(v)) { return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data") } }
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
		emitOptionalType(f, ip.Name, ip.ViewType)
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
		"iterSeq2", "iter.Seq2",
		"iterSeq", "iter.Seq",
	)

	g.L("type $typeName []byte")

	if isVarCount {
		// Count() validates the wire count against BOTH the schema maximum and the
		// remaining buffer (arrayViewCountChecked) — a standalone Count() on an
		// unbounded array must not return an unvalidated 4-byte wire count.
		g.L("func (v $typeName) Count() (int, error) { return arrayViewCountChecked([]byte(v), $maxLen, $minElemW) }")
	} else {
		g.L("func (v $typeName) Len() int { return $count }")
	}

	// size — fixed-element arrays use O(1) shortcuts
	if isFixedElem && isVarCount {
		g.Block(`
			func (v $typeName) size(depth int) (int, error) {
				if depth > maxDepth { return 0, viewErrMaxDepth(0) }
				// Cheap unvalidated count: the total-vs-buffer check below bounds work
				// to O(buffer) on a bogus count, so the up-front min-size check is
				// redundant here. The OOM guard (for preallocation) lives in Count()/All().
				count, err := arrayViewCount([]byte(v), $maxLen)
				if err != nil { return 0, err }
				total := int64(4) + int64(count)*int64($elemSize)
				if total > int64(len(v)) { return 0, viewErrArrayCountExceedsData(4, count, len(v)-4) }
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
			// guard (for preallocation) lives in Count()/All().
			h.L("	count, err := arrayViewCount([]byte(v), $maxLen)")
			h.L("	if err != nil { return 0, err }")
		}
		h.L("	return arrayTraverse([]byte(v), $countExpr, $startOff, func(d []byte) (int, error) { return $elemType(d).$call })")
		h.L("}")
	}

	// At
	if isVarCount {
		if isFixedElem {
			g.Block(`
				func (v $typeName) At(i int) ($elemType, error) {
					var zero $elemType
					count, err := arrayViewCount([]byte(v), $maxLen)
					if err != nil { return zero, err }
					if i < 0 || i >= count { return zero, viewErrIndexOutOfRange(0, i, count) }
					off64 := int64($startOff) + int64(i)*int64($elemSize)
					if off64+int64($elemSize) > int64(len(v)) { return zero, viewErrShortBuffer(uint32(off64), "need $elemSize bytes") }
					return $elemType(v[int(off64):]), nil
				}
			`)
		} else {
			g.Block(`
				func (v $typeName) At(i int) ($elemType, error) {
					var zero $elemType
					count, err := arrayViewCount([]byte(v), $maxLen)
					if err != nil { return zero, err }
					if i < 0 || i >= count { return zero, viewErrIndexOutOfRange(0, i, count) }
					off, err := arrayTraverse([]byte(v), i, $startOff, func(d []byte) (int, error) { return $elemType(d).size(0) })
					if err != nil { return zero, err }
					if off >= len(v) { return zero, viewErrShortBuffer(uint32(off), "element offset exceeds data") }
					return $elemType(v[off:]), nil
				}
			`)
		}
	} else {
		if isFixedElem {
			g.Block(`
				func (v $typeName) At(i int) ($elemType, error) {
					var zero $elemType
					if i < 0 || i >= $count { return zero, viewErrIndexOutOfRange(0, i, $count) }
					off64 := int64($startOff) + int64(i)*int64($elemSize)
					if off64+int64($elemSize) > int64(len(v)) { return zero, viewErrShortBuffer(uint32(off64), "need $elemSize bytes") }
					return $elemType(v[int(off64):]), nil
				}
			`)
		} else {
			g.Block(`
				func (v $typeName) At(i int) ($elemType, error) {
					var zero $elemType
					if i < 0 || i >= $count { return zero, viewErrIndexOutOfRange(0, i, $count) }
					off, err := arrayTraverse([]byte(v), i, $startOff, func(d []byte) (int, error) { return $elemType(d).size(0) })
					if err != nil { return zero, err }
					if off >= len(v) { return zero, viewErrShortBuffer(uint32(off), "element offset exceeds data") }
					return $elemType(v[off:]), nil
				}
			`)
		}
	}

	// Iter
	g.L("func (v $typeName) Iter() $iterSeq2[$elemType, error] {")
	g.L("	return func(yield func($elemType, error) bool) {")
	g.L("		var zero $elemType")
	if isVarCount {
		g.L("		count, err := arrayViewCountChecked([]byte(v), $maxLen, $minElemW)")
		g.L("		if err != nil { yield(zero, err); return }")
	}
	g.L("		off := int64($startOff)")
	g.L("		for k := 0; k < $countExpr; k++ {")
	if isFixedElem {
		g.L(`			if off+int64($elemSize) > int64(len(v)) { yield(zero, viewErrShortBuffer(uint32(off), "need $elemSize bytes")); return }`)
	} else {
		g.L(`			if off >= int64(len(v)) { yield(zero, viewErrShortBuffer(uint32(off), "element offset exceeds data")); return }`)
	}
	g.L("			if !yield($elemType(v[int(off):]), nil) { return }")
	if isFixedElem {
		g.L("			off += int64($elemSize)")
	} else {
		g.L("			sz, err := $elemType(v[int(off):]).size(0)")
		g.L("			if err != nil { yield(zero, err); return }")
		g.L("			off += int64(sz)")
	}
	g.L("		}")
	g.L("	}")
	g.L("}")

	// All — materialize all elements as exact-extent views.
	// Unlike Iter (fat slices for lazy navigation), All returns slices trimmed
	// to each element's wire extent — []byte(elem) is safe to use directly.
	g.L("func (v $typeName) All() ([]$elemType, error) {")
	if isVarCount {
		g.L("	count, err := arrayViewCountChecked([]byte(v), $maxLen, $minElemW)")
		g.L("	if err != nil { return nil, err }")
	}
	g.L("	result := make([]$elemType, 0, $countExpr)")
	g.L("	off := int64($startOff)")
	g.L("	for k := 0; k < $countExpr; k++ {")
	if isFixedElem {
		g.L(`		if off+int64($elemSize) > int64(len(v)) { return nil, viewErrShortBuffer(uint32(off), "need $elemSize bytes") }`)
		g.L("		result = append(result, $elemType(v[int(off):int(off)+$elemSize]))")
		g.L("		off += int64($elemSize)")
	} else {
		g.L(`		if off >= int64(len(v)) { return nil, viewErrShortBuffer(uint32(off), "element offset exceeds data") }`)
		g.L("		elem := $elemType(v[int(off):])")
		g.L("		sz, err := elem.size(0)")
		g.L("		if err != nil { return nil, err }")
		g.L(`		if int(off)+sz > len(v) { return nil, viewErrShortBuffer(uint32(off), "element extends beyond data") }`)
		g.L("		result = append(result, elem[:sz])")
		g.L("		off += int64(sz)")
	}
	g.L("	}")
	g.L("	return result, nil")
	g.L("}")

	// Must methods
	if isVarCount {
		g.L("func (v $typeName) MustCount() int { return must(v.Count()) }")
	}
	g.L("func (v $typeName) MustAt(i int) $elemType { return must(v.At(i)) }")
	g.L("func (v $typeName) MustAll() []$elemType { return must(v.All()) }")

	g.Block(`
		func (v $typeName) MustIter() $iterSeq[$elemType] {
			return func(yield func($elemType) bool) {
				for elem, err := range v.Iter() {
					if err != nil { panic(err) }
					if !yield(elem) { return }
				}
			}
		}
	`)

	emitPublicMethods(f, typeName)
	return nil
}

// emitOptionalType generates a concrete optional view type.
func emitOptionalType(f *GeneratedFile, typeName string, vt *ViewType) {
	innerType := vt.Optional.Element.GoType
	g := f.Use("typeName", typeName, "innerType", innerType)
	g.Block(`
		type $typeName []byte

		func (o $typeName) Unwrap() ($innerType, bool, error) {
			var zero $innerType
			if len(o) < 4 { return zero, false, viewErrShortBuffer(0, "need 4 bytes for optional flag") }
			flag := binary.BigEndian.Uint32(o[:4])
			switch flag {
			case 0: return zero, false, nil
			case 1: return $innerType(o[4:]), true, nil
			default: return zero, false, viewErrBadBoolValue(0, flag)
			}
		}
		func (o $typeName) MustUnwrap() ($innerType, bool) { return must2(o.Unwrap()) }
	`)
	for _, m := range []struct{ name, call string }{{"size", "size(depth + 1)"}, {"valid", "valid(depth + 1)"}} {
		g.Set("method", m.name).Set("call", m.call).Block(`
			func (o $typeName) $method(depth int) (int, error) {
				if depth > maxDepth { return 0, viewErrMaxDepth(0) }
				if len(o) < 4 { return 0, viewErrShortBuffer(0, "need 4 bytes for optional flag") }
				flag := binary.BigEndian.Uint32(o[:4])
				switch flag {
				case 0: return 4, nil
				case 1:
					sz, err := $innerType(o[4:]).$call
					if err != nil { return 0, err }
					return 4 + sz, nil
				default: return 0, viewErrBadBoolValue(0, flag)
				}
			}
		`)
	}
	emitPublicMethods(f, typeName)
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
	g.L("type $typeName []byte")

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
					if len(v) < $paddedSize { return out, viewErrShortBuffer(0, "need $paddedSize bytes") }
					if !bytes.Equal([]byte(v)[$rawSize:$paddedSize], zeroPad[:$padLen]) {
						return out, viewErrNonZeroPadding($rawSize)
					}
					copy(out[:], []byte(v)[:$rawSize])
					return out, nil
				}
			`)
		} else {
			h.Block(`
				func (v $typeName) Value() ($valGoType, error) {
					var out $valGoType
					if len(v) < $paddedSize { return out, viewErrShortBuffer(0, "need $paddedSize bytes") }
					copy(out[:], []byte(v)[:$rawSize])
					return out, nil
				}
			`)
		}
		h.L("func (v $typeName) size(_ int) (int, error) { return $paddedSize, nil }")
		emitValueBasedValid(f, typeName)
		h.L("func (v $typeName) MustValue() $valGoType { return must(v.Value()) }")
	} else {
		g = g.Set("maxLen", vt.Opaque.MaxLen)
		g.Block(`
			func (v $typeName) Value() ([]byte, error) {
				val, err := VarOpaqueView(v).Value()
				if err != nil { return nil, err }
				if len(val) > $maxLen { return nil, viewErrOpaqueExceedsMax(0, uint32(len(val)), $maxLen) }
				return val, nil
			}
		`)
		g.L("func (v $typeName) size(depth int) (int, error) { return VarOpaqueView(v).size(depth) }")
		emitValueBasedValid(f, typeName)
		g.L("func (v $typeName) MustValue() []byte { return must(v.Value()) }")
	}

	emitPublicMethods(f, typeName)
}

// emitEnumViewFromPlan emits an enum view type.
func emitEnumViewFromPlan(f *GeneratedFile, ep *EnumViewPlan) {
	p := f.Use("viewName", ep.ViewTypeName, "enumName", ep.EnumName, "caseNames", joinComma(ep.CaseNames))
	p.Block(`
		type $viewName []byte

		func (v $viewName) Value() ($enumName, error) {
			if len(v) < 4 { return 0, viewErrShortBuffer(0, "need 4 bytes") }
			val := $enumName(int32(binary.BigEndian.Uint32(v[:4])))
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
	p.L("func (v $viewName) MustValue() $enumName { return must(v.Value()) }")
	emitPublicMethods(f, ep.ViewTypeName)
}

// emitTypedefViewFromPlan emits a typedef alias.
func emitTypedefViewFromPlan(f *GeneratedFile, tp *TypedefViewPlan) {
	if tp.ViewType.GoType == tp.AliasName {
		return
	}
	p := f.Use("aliasName", tp.AliasName, "goType", tp.ViewType.GoType)
	p.L("type $aliasName = $goType")
}
