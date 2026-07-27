package main

// emitUnionView emits a union view type from a pre-computed plan.
func emitUnionView(f *GeneratedFile, up *UnionViewPlan) error {
	g := f.Use("viewTypeName", up.ViewTypeName)
	g.L("type $viewTypeName struct{ view }")
	emitParse(f, up.ViewTypeName)

	if up.FixedWireSize != nil {
		emitFixedSizeMethods(f, up.ViewTypeName, *up.FixedWireSize)
		g = g.Set("fixedSize", *up.FixedWireSize)
	} else {
		g.L("func size$viewTypeName(d []byte, depth int) (int, error) {")
		g.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
		emitUnionArmSwitch(f, up.Arms, "size", false)
		g.L("}")
		g.L("func (v $viewTypeName) size(depth int) (int, error) { return size$viewTypeName(v.d, depth) }")
	}

	// Discriminant accessor — returns the DECODED discriminant value, not a leaf
	// view. Enum discriminants decode + validate against the known case set
	// (matching the enum leaf view); int discriminants decode to int32 with NO
	// validation so default arms stay reachable; bool discriminants decode to
	// bool.
	emitDiscriminantAccessor(f, up)

	// Arm accessors: Arm<Name> validates the discriminant and enters the arm.
	for _, ai := range up.Arms {
		if ai.ViewType == nil {
			continue
		}
		h := g.Set("armName", ai.ArmName).Set("armType", ai.ViewType.GoType).
			Set("caseExprs", joinComma(ai.CaseExprs)).
			Set("firstCase", ai.CaseExprs[0])
		h.Block(`
			// Arm$armName validates that the discriminant selects the $armName arm and
			// returns the arm's sub-view (O(1): only the 4-byte discriminant is read).
			func (v $viewTypeName) Arm$armName() ($armType, error) {
				if len(v.d) < 4 { return $armType{}, viewErrShortBuffer(0, "need 4 bytes for discriminant") }
				disc := int32(binary.BigEndian.Uint32(v.d[:4]))
				switch disc {
				case $caseExprs:
				default: return $armType{}, viewErrWrongDiscriminant(0, disc, $firstCase)
				}
				return $armType{view{d: v.d[4:]}}, nil
			}
			// MustArm$armName is Arm$armName panicking with the *ViewError sentinel (recover via Try*).
			func (v $viewTypeName) MustArm$armName() $armType {
				av, err := v.Arm$armName()
				if err != nil { mustView(err) }
				return av
			}
		`)
	}

	// valid — thin engine function + method delegate
	g.L("func valid$viewTypeName(d []byte, depth int) (int, error) {")
	if up.FixedWireSize != nil {
		g.L(`	if len(d) < $fixedSize { return 0, viewErrShortBuffer(0, "need $fixedSize bytes") }`)
	} else {
		g.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
	}
	emitUnionArmSwitch(f, up.Arms, "valid", up.FixedWireSize != nil)
	g.L("}")
	g.L("func (v $viewTypeName) valid(depth int) (int, error) { return valid$viewTypeName(v.d, depth) }")
	emitPublicMethods(f, up.ViewTypeName)
	return nil
}

// emitDiscriminantAccessor emits the decoded-discriminant accessor.
func emitDiscriminantAccessor(f *GeneratedFile, up *UnionViewPlan) {
	g := f.Use("viewTypeName", up.ViewTypeName, "discName", up.DiscName, "discGoType", up.DiscGoType)
	defer g.Block(`
		// Must$discName is $discName panicking with the *ViewError sentinel (recover via Try*).
		func (v $viewTypeName) Must$discName() $discGoType {
			dv, err := v.$discName()
			if err != nil { mustView(err) }
			return dv
		}
	`)
	switch up.DiscKind {
	case DiscEnum:
		g.Set("caseNames", joinComma(up.DiscCaseNames)).Block(`
			// $discName returns the decoded discriminant (O(1)), validated against the
			// known case set.
			func (v $viewTypeName) $discName() ($discGoType, error) {
				if len(v.d) < 4 { return 0, viewErrShortBuffer(0, "need 4 bytes for discriminant") }
				val := $discGoType(int32(binary.BigEndian.Uint32(v.d[:4])))
				switch val {
				case $caseNames:
					return val, nil
				default:
					return 0, viewErrUnknownDiscriminant(0, int32(val))
				}
			}
		`)
	case DiscInt:
		g.Block(`
			// $discName returns the decoded discriminant (O(1)). It is not validated
			// against the case set, so unknown discriminants stay observable.
			func (v $viewTypeName) $discName() ($discGoType, error) {
				if len(v.d) < 4 { return 0, viewErrShortBuffer(0, "need 4 bytes for discriminant") }
				return int32(binary.BigEndian.Uint32(v.d[:4])), nil
			}
		`)
	case DiscUint:
		g.Block(`
			// $discName returns the decoded discriminant (O(1)). It is not validated
			// against the case set, so unknown discriminants stay observable.
			func (v $viewTypeName) $discName() ($discGoType, error) {
				if len(v.d) < 4 { return 0, viewErrShortBuffer(0, "need 4 bytes for discriminant") }
				return binary.BigEndian.Uint32(v.d[:4]), nil
			}
		`)
	case DiscBool:
		g.Block(`
			// $discName returns the decoded bool discriminant (O(1)).
			func (v $viewTypeName) $discName() ($discGoType, error) {
				if len(v.d) < 4 { return false, viewErrShortBuffer(0, "need 4 bytes for discriminant") }
				flag := binary.BigEndian.Uint32(v.d[:4])
				switch flag {
				case 0: return false, nil
				case 1: return true, nil
				default: return false, viewErrBadBoolValue(0, flag)
				}
			}
		`)
	}
}

// emitUnionArmSwitch emits the discriminant switch for the thin-engine union
// size/valid bodies (over a bare `d []byte` in scope). fn is the thin-engine
// function prefix ("size" or "valid").
func emitUnionArmSwitch(f *GeneratedFile, arms []UnionArmPlan, fn string, boundsChecked bool) {
	g := f.Use("fn", fn)
	if !boundsChecked {
		g.L(`	if len(d) < 4 { return 0, viewErrShortBuffer(0, "need 4 bytes for discriminant") }`)
	}
	g.L("	disc := int32(binary.BigEndian.Uint32(d[:4]))")
	g.L("	switch disc {")
	for _, ai := range arms {
		h := g.Set("caseExprs", joinComma(ai.CaseExprs))
		h.L("	case $caseExprs:")
		if ai.ViewType == nil {
			h.L("		return 4, nil")
		} else {
			h = h.Set("armType", ai.ViewType.GoType)
			h.L("		sz, err := $fn$armType(d[4:], depth+1)")
			h.L("		if err != nil { return 0, err }")
			h.L(`		if 4 + sz > len(d) { return 0, viewErrShortBuffer(4, "arm exceeds data") }`)
			h.L("		return 4 + sz, nil")
		}
	}
	g.L("	default: return 0, viewErrUnknownDiscriminant(0, disc)")
	g.L("	}")
}
