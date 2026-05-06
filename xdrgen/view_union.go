package main

// emitUnionViewFromPlan emits a union view type from a pre-computed plan.
func emitUnionViewFromPlan(f *GeneratedFile, up *UnionViewPlan) {
	g := f.Use("viewTypeName", up.ViewTypeName)
	g.L("type $viewTypeName []byte")

	if up.FixedWireSize != nil {
		emitFixedSizeMethods(f, up.ViewTypeName, *up.FixedWireSize)
		g = g.Set("fixedSize", *up.FixedWireSize)
	} else {
		g.L("func (v $viewTypeName) size(depth int) (int, error) {")
		g.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
		emitUnionArmSwitch(f, up.Arms, "size(depth + 1)", false)
		g.L("}")
	}

	// Discriminant accessor
	h := g.Set("discName", up.DiscName).Set("discType", up.DiscViewType.GoType)
	h.Block(`
		func (v $viewTypeName) $discName() ($discType, error) {
			if len(v) < 4 { return nil, viewErrShortBuffer(0, "need 4 bytes for discriminant") }
			return $discType(v[:4]), nil
		}
		func (v $viewTypeName) Must$discName() $discType { return must(v.$discName()) }
	`)

	// Arm accessors
	for _, ai := range up.Arms {
		if ai.ViewType == nil {
			continue
		}
		h := g.Set("armName", ai.ArmName).Set("armType", ai.ViewType.GoType).
			Set("caseExprs", joinComma(ai.CaseExprs)).
			Set("firstCase", ai.CaseExprs[0]).
			Set("subView", ai.ViewType.SubView("v", "4"))
		h.Block(`
			func (v $viewTypeName) $armName() ($armType, error) {
				if len(v) < 4 { return nil, viewErrShortBuffer(0, "need 4 bytes for discriminant") }
				disc := int32(binary.BigEndian.Uint32(v[:4]))
				switch disc {
				case $caseExprs:
				default: return nil, viewErrWrongDiscriminant(0, disc, $firstCase)
				}
				return $subView, nil
			}
			func (v $viewTypeName) Must$armName() $armType { return must(v.$armName()) }
		`)
	}

	// valid
	g.L("func (v $viewTypeName) valid(depth int) (int, error) {")
	if up.FixedWireSize != nil {
		g.L(`	if len(v) < $fixedSize { return 0, viewErrShortBuffer(0, "need $fixedSize bytes") }`)
	} else {
		g.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
	}
	emitUnionArmSwitch(f, up.Arms, "valid(depth + 1)", up.FixedWireSize != nil)
	g.L("}")
	emitPublicMethods(f, up.ViewTypeName)
}

// emitUnionArmSwitch emits the discriminant switch for union size()/valid().
func emitUnionArmSwitch(f *GeneratedFile, arms []UnionArmPlan, call string, boundsChecked bool) {
	g := f.Use("call", call)
	if !boundsChecked {
		g.L(`	if len(v) < 4 { return 0, viewErrShortBuffer(0, "need 4 bytes for discriminant") }`)
	}
	g.L("	disc := int32(binary.BigEndian.Uint32(v[:4]))")
	g.L("	switch disc {")
	for _, ai := range arms {
		h := g.Set("caseExprs", joinComma(ai.CaseExprs))
		h.L("	case $caseExprs:")
		if ai.ViewType == nil {
			h.L("		return 4, nil")
		} else {
			h = h.Set("armType", ai.ViewType.GoType)
			h.L("		sz, err := $armType(v[4:]).$call")
			h.L("		if err != nil { return 0, err }")
			h.L(`		if 4 + sz > len(v) { return 0, viewErrShortBuffer(4, "arm exceeds data") }`)
			h.L("		return 4 + sz, nil")
		}
	}
	g.L("	default: return 0, viewErrUnknownDiscriminant(0, disc)")
	g.L("	}")
}
