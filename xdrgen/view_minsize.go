package main

// xdrWordSize is the XDR basic block size in bytes: every XDR-encoded item is a
// multiple of four bytes (RFC 4506 section 3). It is the minimum wire size of a
// count word, a presence flag, a discriminant, and an enum.
const xdrWordSize = 4

// Minimum-wire-size pass for array count validation.
//
// minWireSize computes the smallest number of wire bytes any value of a type can
// occupy. It is used to validate array counts against the remaining buffer: an
// array of N elements needs at least N*minElemW bytes.
//
// Rules:
//   - scalars: their fixed size (4 or 8)
//   - fixed opaque / fixed array: their exact padded size
//   - variable-length prefix (var opaque, string, var array): 4 (the count word
//     alone, zero elements)
//   - optional: 4 (the absence flag alone)
//   - struct: sum of field minimums
//   - union: 4 (discriminant) + the minimum over all arms (a void arm is 0)
//   - enum: 4
//
// Recursion through named types only bottoms out through optionals and
// variable-length containers, whose 4-byte minimum is independent of the inner
// type; a cycle therefore always passes through such a node. The visiting guard
// returns 0 on a back-edge, which is correct because the enclosing
// optional/var-container contributes its own 4 regardless.

// minWireSize returns the minimum wire size of a resolved ViewType. visiting
// tracks the named types currently on the recursion stack to break cycles.
func (g *Generator) minWireSize(vt *ViewType, visiting map[string]bool) uint32 {
	if vt == nil {
		return 0
	}
	if fs, ok := vt.FixedSize(); ok {
		return fs
	}
	switch vt.Kind {
	case VKScalar:
		// Fixed-size scalars are handled above; this is unreachable in practice.
		return xdrWordSize
	case VKOpaque:
		// Fixed opaque is fixed-size (handled above). Variable/bounded opaque and
		// strings have only the 4-byte length prefix as a minimum.
		return xdrWordSize
	case VKArray:
		// Fixed-count arrays are fixed-size (handled above). Variable arrays have
		// only the 4-byte count prefix as a minimum (zero elements).
		return xdrWordSize
	case VKOptional:
		// Just the 4-byte presence flag (absent).
		return xdrWordSize
	case VKNamed:
		return g.minWireSizeNamed(vt.Named.XDRName, visiting)
	}
	return xdrWordSize
}

// minWireSizeNamed computes the minimum wire size of a named struct/union/enum/typedef.
func (g *Generator) minWireSizeNamed(xdrName string, visiting map[string]bool) uint32 {
	def, ok := g.TypeResolver[xdrName]
	if !ok {
		return xdrWordSize
	}
	// Fixed-size named types short-circuit to their exact size.
	if def.FixedSize != nil {
		return *def.FixedSize
	}
	if visiting[xdrName] {
		// Back-edge: an enclosing optional/var-container bounds this at 4.
		return 0
	}
	visiting[xdrName] = true
	defer delete(visiting, xdrName)

	switch def.Kind {
	case DKStruct:
		return g.minWireSizeStruct(def.Struct, visiting)
	case DKUnion:
		// 4-byte discriminant plus the minimum over all arms.
		return xdrWordSize + g.minWireSizeUnionArms(def.Union, visiting)
	case DKEnum:
		return xdrWordSize
	case DKTypedef:
		tvt, err := g.ResolveViewType(&def.Typedef.Type)
		if err != nil {
			return xdrWordSize
		}
		return g.minWireSize(tvt, visiting)
	}
	return xdrWordSize
}

// minWireSizeStruct sums the minimum wire sizes of a struct's fields.
func (g *Generator) minWireSizeStruct(def *StructDef, visiting map[string]bool) uint32 {
	var total uint32
	for i := range def.Fields {
		fvt, err := g.ResolveViewType(&def.Fields[i].Type)
		if err != nil {
			// Resolution errors are surfaced elsewhere; treat as 0 here.
			continue
		}
		total += g.minWireSize(fvt, visiting)
	}
	return total
}

// minWireSizeUnionArms returns the minimum wire size over all of a union's arms
// (a void arm contributes 0), excluding the discriminant word.
func (g *Generator) minWireSizeUnionArms(def *UnionDef, visiting map[string]bool) uint32 {
	var armMin uint32
	first := true
	for ai := range def.Arms {
		arm := &def.Arms[ai]
		var m uint32
		if arm.Type != nil {
			avt, err := g.ResolveViewType(arm.Type)
			if err == nil {
				m = g.minWireSize(avt, visiting)
			}
		}
		if first || m < armMin {
			armMin = m
			first = false
		}
	}
	return armMin
}

// minElemWidth returns the clamped (floor 1) minimum wire size of an array
// element type, used as the divisor in count validation. The second return
// value is the unclamped computed minimum, so callers can assert it is non-zero.
func (g *Generator) minElemWidth(elem *ViewType) (clamped, computed uint32) {
	computed = g.minWireSize(elem, map[string]bool{})
	clamped = computed
	if clamped < 1 {
		clamped = 1
	}
	return clamped, computed
}
