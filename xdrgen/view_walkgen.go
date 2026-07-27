package main

// Tier-2 Walk emission, SCHEMA-DERIVED (walk-derivation-design.md): a walk
// root is a declarative spec — positions attached to concrete
// (ViewTypeName, FieldName) coordinates — and everything else (traversal
// order, descent/prune decisions, count validation, context-arg threading,
// element-boundary stops, scope-derived truncation) derives from the same
// plan the size/valid emitters consume. A protocol upgrade costs a spec
// line plus fixtures; there is no hand-written traversal to edit.

import (
	"fmt"
	"sort"
	"strings"
)

type attachKey struct{ Type, Field string } // Field "" = the type itself

// walkAttach states everything positions can do at one schema coordinate.
type walkAttach struct {
	CountPos  string            // array field: fired once with validated count
	ElemView  string            // array field: per-element exact-view delivery
	BindArg   string            // array field: context arg = element index
	FieldView string            // struct field: exact-view delivery (pre-descent)
	PostWalk  string            // struct field: exact-view delivery post-subtree
	DiscPos   string            // union (Field ""): discriminant delivery, int32
	SetArgs   map[string]string // literal context values entering this subtree
}

type walkRootSpec struct {
	Root      string
	Callbacks string
	SubRoots  []string
	Args      []string
	Positions []string
	Attach    map[attachKey]walkAttach
}

var walkRoots = []walkRootSpec{{
	Root:      "LedgerCloseMeta",
	Callbacks: "LedgerCloseMetaWalk",
	SubRoots:  []string{"TransactionMeta"},
	Args:      []string{"txIdx", "opIdx", "evIdx"},
	Positions: []string{
		"TxProcessingBegin", "ResultPair", "MetaVersion", "V4Ops",
		"OpEventsBegin", "OpEvent", "TxEventsBegin", "TxEvent",
		"DiagEventsBegin", "DiagEvent", "TxMeta",
	},
	Attach: map[attachKey]walkAttach{
		{"LedgerCloseMetaV0View", "TxProcessing"}: {CountPos: "TxProcessingBegin", BindArg: "txIdx"},
		{"LedgerCloseMetaV1View", "TxProcessing"}: {CountPos: "TxProcessingBegin", BindArg: "txIdx"},
		{"LedgerCloseMetaV2View", "TxProcessing"}: {CountPos: "TxProcessingBegin", BindArg: "txIdx"},

		{"TransactionResultMetaView", "Result"}:              {FieldView: "ResultPair"},
		{"TransactionResultMetaView", "TxApplyProcessing"}:   {PostWalk: "TxMeta"},
		{"TransactionResultMetaV1View", "Result"}:            {FieldView: "ResultPair"},
		{"TransactionResultMetaV1View", "TxApplyProcessing"}: {PostWalk: "TxMeta"},

		{"TransactionMetaView", ""}: {DiscPos: "MetaVersion"},

		{"SorobanTransactionMetaView", "Events"}: {
			CountPos: "OpEventsBegin", ElemView: "OpEvent", BindArg: "evIdx",
			SetArgs: map[string]string{"opIdx": "0"},
		},
		{"SorobanTransactionMetaView", "DiagnosticEvents"}: {CountPos: "DiagEventsBegin", ElemView: "DiagEvent", BindArg: "evIdx"},

		{"TransactionMetaV4View", "Operations"}:       {CountPos: "V4Ops", BindArg: "opIdx"},
		{"OperationMetaV2View", "Events"}:             {CountPos: "OpEventsBegin", ElemView: "OpEvent", BindArg: "evIdx"},
		{"TransactionMetaV4View", "Events"}:           {CountPos: "TxEventsBegin", ElemView: "TxEvent", BindArg: "evIdx"},
		{"TransactionMetaV4View", "DiagnosticEvents"}: {CountPos: "DiagEventsBegin", ElemView: "DiagEvent", BindArg: "evIdx"},
	},
}, {
	// Golden-schema demonstration root (emitted only when the mini schema is
	// the input): derives a small walker over OptionalEntry, proving the
	// spec-line-only maintenance property on a second, unrelated schema.
	Root:      "OptionalEntry",
	Callbacks: "OptionalEntryWalk",
	Args:      []string{},
	Positions: []string{"EntryVersion", "EntryPayload"},
	Attach: map[attachKey]walkAttach{
		{"OptionalEntryView", ""}: {DiscPos: "EntryVersion"},
		{"MixedView", "Payload"}:  {FieldView: "EntryPayload"},
	},
}}

// argRef is a context arg in scope at an emission site: its spec name and
// the Go expression carrying it (a parameter name or a literal).
type argRef struct{ name, expr string }

type posSig struct {
	args    []string // spec arg names, spec order
	payload string   // "count int" | "v int32" | "ev <Type>" | "m <Type>"
}

type walkEmitter struct {
	f       *GeneratedFile
	spec    walkRootSpec
	structs map[string]*StructViewPlan
	unions  map[string]*UnionViewPlan
	inlines map[string]*InlineTypePlan
	aliases map[string]string
	posBit  map[string]int
	reach   map[string]uint64
	inProg  map[string]bool
	sigs    map[string]posSig
	fnFor   map[string]string // GoType -> emitted walk fn name
	fnScope map[string]string // GoType -> canonical scope (arg names joined)
	queue   []queued
}

type queued struct {
	goType string
	scope  []argRef
	full   bool
}

// emitWalkRoots emits every configured walk root whose root type exists in
// the schema plan (absent roots skip; present roots with unresolvable spec
// keys hard-error — the reshape tripwire).
func emitWalkRoots(f *GeneratedFile, plan *ViewPlan) error {
	structs := map[string]*StructViewPlan{}
	unions := map[string]*UnionViewPlan{}
	inlines := map[string]*InlineTypePlan{}
	aliases := map[string]string{}
	for _, e := range plan.Entries {
		switch t := e.(type) {
		case *StructViewPlan:
			structs[t.ViewTypeName] = t
		case *UnionViewPlan:
			unions[t.ViewTypeName] = t
		case *InlineTypePlan:
			inlines[t.Name] = t
		case *TypedefViewPlan:
			aliases[t.AliasName] = t.ViewType.GoType
		}
	}
	for _, spec := range walkRoots {
		rootView := spec.Root + "View"
		if structs[rootView] == nil && unions[rootView] == nil {
			continue
		}
		we := &walkEmitter{
			f: f, spec: spec,
			structs: structs, unions: unions, inlines: inlines, aliases: aliases,
			posBit: map[string]int{}, reach: map[string]uint64{}, inProg: map[string]bool{},
			sigs: map[string]posSig{}, fnFor: map[string]string{}, fnScope: map[string]string{},
		}
		if err := we.run(); err != nil {
			return fmt.Errorf("walk root %s: %w", spec.Root, err)
		}
	}
	return nil
}

func (we *walkEmitter) run() error {
	if err := we.validateSpec(); err != nil {
		return err
	}
	for i, p := range we.spec.Positions {
		we.posBit[p] = i
	}
	// Reachability fixpoint (twice covers attachment-free cycles like ScVal).
	for pass := 0; pass < 2; pass++ {
		we.inProg = map[string]bool{}
		we.reachOf(we.spec.Root + "View")
	}
	if err := we.emitFns(); err != nil {
		return err
	}
	we.emitCallbacksAndWrappers()
	return nil
}

// validateSpec: every attach key must resolve in the plan; every referenced
// position must be in the manifest; every arg must be declared.
func (we *walkEmitter) validateSpec() error {
	inManifest := map[string]bool{}
	for _, p := range we.spec.Positions {
		inManifest[p] = true
	}
	declaredArg := map[string]bool{}
	for _, a := range we.spec.Args {
		declaredArg[a] = true
	}
	for k, a := range we.spec.Attach {
		if k.Field == "" {
			if we.unions[k.Type] == nil {
				return fmt.Errorf("attach key %v: union not found in plan (schema reshape? update the spec)", k)
			}
		} else {
			sp := we.structs[k.Type]
			if sp == nil {
				return fmt.Errorf("attach key %v: struct not found in plan (schema reshape? update the spec)", k)
			}
			found := false
			for _, fp := range sp.Fields {
				if fp.FieldName == k.Field {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("attach key %v: field not found in plan (schema reshape? update the spec)", k)
			}
		}
		for _, pos := range []string{a.CountPos, a.ElemView, a.FieldView, a.PostWalk, a.DiscPos} {
			if pos != "" && !inManifest[pos] {
				return fmt.Errorf("attach key %v: position %q not in the manifest", k, pos)
			}
		}
		if a.BindArg != "" && !declaredArg[a.BindArg] {
			return fmt.Errorf("attach key %v: BindArg %q not declared", k, a.BindArg)
		}
		for name := range a.SetArgs {
			if !declaredArg[name] {
				return fmt.Errorf("attach key %v: SetArgs %q not declared", k, name)
			}
		}
	}
	return nil
}

func (we *walkEmitter) resolve(goType string) string {
	for {
		next, ok := we.aliases[goType]
		if !ok || next == goType {
			return goType
		}
		goType = next
	}
}

func (we *walkEmitter) attachBits(k attachKey) uint64 {
	a, ok := we.spec.Attach[k]
	if !ok {
		return 0
	}
	var m uint64
	for _, pos := range []string{a.CountPos, a.ElemView, a.FieldView, a.PostWalk, a.DiscPos} {
		if pos != "" {
			m |= 1 << we.posBit[pos]
		}
	}
	return m
}

// reachOf computes the subscribable-position mask under a type (memoized).
func (we *walkEmitter) reachOf(goType string) uint64 {
	goType = we.resolve(goType)
	if m, ok := we.reach[goType]; ok && !we.inProg[goType] {
		return m
	}
	if we.inProg[goType] {
		return we.reach[goType] // cycle: current estimate
	}
	we.inProg[goType] = true
	var m uint64
	switch {
	case we.structs[goType] != nil:
		sp := we.structs[goType]
		for _, fp := range sp.Fields {
			m |= we.attachBits(attachKey{goType, fp.FieldName})
			m |= we.viewTypeReach(fp.ViewType)
		}
	case we.unions[goType] != nil:
		up := we.unions[goType]
		m |= we.attachBits(attachKey{goType, ""})
		for _, arm := range up.Arms {
			if arm.ViewType != nil {
				m |= we.viewTypeReach(arm.ViewType)
			}
		}
	case we.inlines[goType] != nil:
		m |= we.viewTypeReach(we.inlines[goType].ViewType)
	}
	we.inProg[goType] = false
	we.reach[goType] = m
	return m
}

func (we *walkEmitter) viewTypeReach(vt *ViewType) uint64 {
	switch vt.Kind {
	case VKArray:
		return we.viewTypeReach(vt.Array.Element)
	case VKOptional:
		return we.viewTypeReach(vt.Optional.Element)
	default:
		return we.reachOf(vt.GoType)
	}
}

// recordSig registers/validates a position's derived signature.
func (we *walkEmitter) recordSig(pos string, scope []argRef, payload string) error {
	names := make([]string, len(scope))
	for i, a := range scope {
		names[i] = a.name
	}
	sig := posSig{args: names, payload: payload}
	if prev, ok := we.sigs[pos]; ok {
		if strings.Join(prev.args, ",") != strings.Join(sig.args, ",") || prev.payload != sig.payload {
			return fmt.Errorf("position %s: inconsistent derived signatures (%v/%s vs %v/%s)",
				pos, prev.args, prev.payload, sig.args, sig.payload)
		}
		return nil
	}
	we.sigs[pos] = sig
	return nil
}

func scopeKey(scope []argRef) string {
	names := make([]string, len(scope))
	for i, a := range scope {
		names[i] = a.name
	}
	return strings.Join(names, ",")
}

// fnParams renders a walk fn's parameter list for the scope.
func fnParams(scope []argRef) string {
	var b strings.Builder
	for _, a := range scope {
		fmt.Fprintf(&b, "%s int, ", a.name)
	}
	return b.String()
}

func fnArgs(scope []argRef) string {
	var b strings.Builder
	for _, a := range scope {
		fmt.Fprintf(&b, "%s, ", a.expr)
	}
	return b.String()
}

// walkFnName derives the emitted function name for a composite type.
func (we *walkEmitter) walkFnName(goType string) string {
	return "walk" + we.spec.Root + "_" + strings.TrimSuffix(goType, "View")
}

// need registers a composite type for walk-fn emission under a scope
// (parameters = the scope's arg NAMES) and returns its fn name.
func (we *walkEmitter) need(goType string, scope []argRef, full bool) (string, error) {
	goType = we.resolve(goType)
	name := we.walkFnName(goType)
	key := scopeKey(scope) + fmt.Sprintf("|full=%v", full)
	if prev, ok := we.fnScope[goType]; ok {
		if prev != key {
			return "", fmt.Errorf("type %s walked under differing scope/mode (%q vs %q)", goType, prev, key)
		}
		return name, nil
	}
	we.fnScope[goType] = key
	we.fnFor[goType] = name
	params := make([]argRef, len(scope))
	for i, a := range scope {
		params[i] = argRef{a.name, a.name} // inside the fn, args are params
	}
	we.queue = append(we.queue, queued{goType, params, full})
	return name, nil
}

func (we *walkEmitter) emitFns() error {
	rootView := we.spec.Root + "View"
	if _, err := we.need(rootView, nil, false); err != nil {
		return err
	}
	for _, sub := range we.spec.SubRoots {
		subView := sub + "View"
		// Sub-roots reuse the walk fn generated for their type wherever it
		// sits in the tree; require it to exist after root emission.
		defer func(sv string) {}(subView)
	}
	for len(we.queue) > 0 {
		q := we.queue[0]
		we.queue = we.queue[1:]
		var err error
		switch {
		case we.structs[q.goType] != nil:
			err = we.emitStructFn(q)
		case we.unions[q.goType] != nil:
			err = we.emitUnionFn(q)
		case we.inlines[q.goType] != nil:
			err = we.emitInlineFn(q)
		default:
			err = fmt.Errorf("cannot walk type %s (not a composite in the plan)", q.goType)
		}
		if err != nil {
			return err
		}
	}
	for _, sub := range we.spec.SubRoots {
		if we.fnFor[sub+"View"] == "" {
			return fmt.Errorf("sub-root %s: type is never reached from the root walk", sub)
		}
	}
	return nil
}

// sizeCall renders the thin-engine skip for a view type at d[off:].
func sizeCall(vt *ViewType) string {
	return "size" + vt.GoType + "(d[off:], depth+1)"
}

// emitStructFn emits the walk fn for a struct: fields in order; top mode
// stops after the last reach/attachment-bearing field; full mode (array
// elements and everything below them) walks to the end so the caller can
// advance by the returned extent.
func (we *walkEmitter) emitStructFn(q queued) error {
	sp := we.structs[q.goType]
	g := we.f.Use("fn", we.fnFor[q.goType], "cb", we.spec.Callbacks, "params", fnParams(q.scope))
	g.L("func $fn(d []byte, $paramsw *$cb, m uint64, depth int) (int64, error) {")
	g.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
	g.L("	off := int64(0)")
	last := -1
	for i, fp := range sp.Fields {
		if we.attachBits(attachKey{q.goType, fp.FieldName}) != 0 || we.viewTypeReach(fp.ViewType) != 0 {
			last = i
		}
	}
	end := len(sp.Fields)
	if !q.full && last >= 0 {
		end = last + 1
	}
	for i := 0; i < end; i++ {
		fp := sp.Fields[i]
		a := we.spec.Attach[attachKey{q.goType, fp.FieldName}]
		scope := q.scope
		for _, name := range sortedKeys(a.SetArgs) {
			scope = appendArg(scope, we.spec.Args, argRef{name, a.SetArgs[name]})
		}
		fieldFull := q.full || i != end-1
		if err := we.emitFieldAdvance(fp, a, scope, q, fieldFull); err != nil {
			return err
		}
	}
	g.L("	if off > int64(len(d)) { return 0, viewErrShortBuffer(uint32(off), \"field offset exceeds data\") }")
	g.L("	return off, nil")
	g.L("}")
	return nil
}

// emitFieldAdvance emits one field's advance (with any attached firing).
func (we *walkEmitter) emitFieldAdvance(fp FieldPlan, a walkAttach, scope []argRef, q queued, fieldFull bool) error {
	g := we.f.Use("cb", we.spec.Callbacks)
	vt := fp.ViewType
	if fs, ok := vt.FixedSize(); ok && a.FieldView == "" && a.PostWalk == "" {
		g.Set("fs", fs).L("	off += $fs")
		g.L("	if off > int64(len(d)) { return 0, viewErrShortBuffer(uint32(off), \"field offset exceeds data\") }")
		return nil
	}
	// Array field with walk attachments: inline array walk.
	if vt.Kind == VKArray && (a.CountPos != "" || a.ElemView != "" || a.BindArg != "") {
		return we.emitArrayField(fp, a, scope, q)
	}
	if a.FieldView != "" {
		if err := we.recordSig(a.FieldView, scope, "v "+vt.GoType); err != nil {
			return err
		}
		h := g.Set("sz", sizeCall(vt)).Set("pos", a.FieldView).Set("args", fnArgs(scope)).Set("T", vt.GoType)
		h.Block(`
			if off > int64(len(d)) { return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data") }
			{ sz, err := $sz
			if err != nil { return 0, err }
			if w.$pos != nil {
				if err := w.$pos($args$T{view{d: d[off : off+int64(sz)], exact: true}}); err != nil { return 0, err }
			}
			off += int64(sz) }
		`)
		return nil
	}
	reach := we.viewTypeReach(vt)
	if reach == 0 && a.PostWalk == "" {
		g.Set("sz", sizeCall(vt)).Block(`
			if off > int64(len(d)) { return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data") }
			{ sz, err := $sz
			if err != nil { return 0, err }
			off += int64(sz) }
		`)
		return nil
	}
	// Composite descent with prune branch (+ optional PostWalk delivery). The
	// child walks in full mode unless it is the LAST field a top-mode walk
	// touches (its extent is then owed to no one — scope-derived truncation).
	inner, err := we.descendTarget(vt, scope, fieldFull)
	if err != nil {
		return err
	}
	h := g.Set("mask", fmt.Sprintf("0x%x", reach)).Set("walk", inner).
		Set("sz", sizeCall(vt)).Set("args", fnArgs(scope))
	h.L("	if off > int64(len(d)) { return 0, viewErrShortBuffer(uint32(off), \"field offset exceeds data\") }")
	h.L("	{")
	h.L("		var sz int64")
	h.L("		var err error")
	if reach != 0 {
		h.L("		if m&$mask != 0 {")
		h.L("			sz, err = $walk(d[off:], $argsw, m, depth+1)")
		h.L("		} else {")
		h.L("			var szi int")
		h.L("			szi, err = $sz")
		h.L("			sz = int64(szi)")
		h.L("		}")
	} else {
		h.L("		var szi int")
		h.L("		szi, err = $sz")
		h.L("		sz = int64(szi)")
	}
	h.L("		if err != nil { return 0, err }")
	if a.PostWalk != "" {
		if err := we.recordSig(a.PostWalk, scope, "v "+vt.GoType); err != nil {
			return err
		}
		h.Set("pw", a.PostWalk).Set("T", vt.GoType).Block(`
			if w.$pw != nil {
				if err := w.$pw($args$T{view{d: d[off : off+sz], exact: true}}); err != nil { return 0, err }
			}
		`)
	}
	h.L("		off += sz")
	h.L("	}")
	return nil
}

// descendTarget resolves how to walk INTO a field's view type: arrays and
// optionals get inline-callable fns; named composites get their walk fn.
func (we *walkEmitter) descendTarget(vt *ViewType, scope []argRef, full bool) (string, error) {
	if (vt.Kind == VKArray || vt.Kind == VKOptional) && we.inlines[vt.GoType] == nil {
		return "", fmt.Errorf("inline plan missing for %s", vt.GoType)
	}
	return we.need(vt.GoType, scope, full)
}

// emitArrayField emits an attachment-bearing array field inline: validated
// count, CountPos, element loop with BindArg, ElemView delivery or element
// descent.
func (we *walkEmitter) emitArrayField(fp FieldPlan, a walkAttach, scope []argRef, q queued) error {
	vt := fp.ViewType
	ip := we.inlines[vt.GoType]
	if ip == nil {
		return fmt.Errorf("array field %s.%s: inline plan missing for %s", q.goType, fp.FieldName, vt.GoType)
	}
	minW, err := arrayMinElemW(ip)
	if err != nil {
		return err
	}
	g := we.f.Use("cb", we.spec.Callbacks, "maxLen", vt.Array.MaxLen, "minW", minW)
	g.L("	if off > int64(len(d)) { return 0, viewErrShortBuffer(uint32(off), \"field offset exceeds data\") }")
	g.L("	{")
	g.L("		ad := d[off:]")
	g.L("		count, err := arrayViewCountChecked(ad, $maxLen, $minW)")
	g.L("		if err != nil { return 0, err }")
	if a.CountPos != "" {
		if err := we.recordSig(a.CountPos, scope, "count int"); err != nil {
			return err
		}
		g.Set("pos", a.CountPos).Set("args", fnArgs(scope)).Block(`
			if w.$pos != nil {
				if err := w.$pos($argscount); err != nil { return 0, err }
			}
		`)
	}
	elemScope := scope
	loopVar := "k"
	if a.BindArg != "" {
		elemScope = appendArg(scope, we.spec.Args, argRef{a.BindArg, "k"})
	}
	g.Set("k", loopVar).L("		aoff := int64(4)")
	g.L("		for k := 0; k < count; k++ {")
	g.L("			if aoff >= int64(len(ad)) { return 0, viewErrShortBuffer(uint32(aoff), \"element offset exceeds data\") }")
	elem := vt.Array.Element
	if a.ElemView != "" {
		if err := we.recordSig(a.ElemView, elemScope, "ev "+elem.GoType); err != nil {
			return err
		}
		g.Set("esz", "size"+elem.GoType+"(ad[aoff:], depth+1)").Set("pos", a.ElemView).
			Set("eargs", fnArgs(elemScope)).Set("ET", elem.GoType).Block(`
			sz, err := $esz
			if err != nil { return 0, err }
			if w.$pos != nil {
				if err := w.$pos($eargs$ET{view{d: ad[aoff : aoff+int64(sz)], exact: true}}); err != nil { return 0, err }
			}
			aoff += int64(sz)
		`)
	} else {
		inner, err := we.need(elem.GoType, elemScope, true)
		if err != nil {
			return err
		}
		g.Set("walk", inner).Set("eargs", fnArgs(elemScope)).Block(`
			sz, err := $walk(ad[aoff:], $eargsw, m, depth+1)
			if err != nil { return 0, err }
			aoff += sz
		`)
	}
	g.L("		}")
	g.L("		off += aoff")
	g.L("	}")
	return nil
}

// emitUnionFn emits the walk fn for a union: discriminant read (+DiscPos),
// then per-arm descend-or-skip with unknown-discriminant errors matching the
// thin engine.
func (we *walkEmitter) emitUnionFn(q queued) error {
	up := we.unions[q.goType]
	g := we.f.Use("fn", we.fnFor[q.goType], "cb", we.spec.Callbacks, "params", fnParams(q.scope))
	g.L("func $fn(d []byte, $paramsw *$cb, m uint64, depth int) (int64, error) {")
	g.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
	g.L("	if len(d) < 4 { return 0, viewErrShortBuffer(0, \"need 4 bytes for discriminant\") }")
	g.L("	disc := int32(binary.BigEndian.Uint32(d[:4]))")
	a := we.spec.Attach[attachKey{q.goType, ""}]
	if a.DiscPos != "" {
		if err := we.recordSig(a.DiscPos, q.scope, "v int32"); err != nil {
			return err
		}
		g.Set("pos", a.DiscPos).Set("args", fnArgs(q.scope)).Block(`
			if w.$pos != nil {
				if err := w.$pos($argsdisc); err != nil { return 0, err }
			}
		`)
	}
	g.L("	off := int64(4)")
	g.L("	switch disc {")
	for _, arm := range up.Arms {
		h := g.Set("cases", joinComma(arm.CaseExprs))
		h.L("	case $cases:")
		if arm.ViewType == nil {
			continue
		}
		reach := we.viewTypeReach(arm.ViewType)
		if reach == 0 {
			h.Set("sz", sizeCall(arm.ViewType)).Block(`
				sz, err := $sz
				if err != nil { return 0, err }
				off += int64(sz)
			`)
			continue
		}
		inner, err := we.descendTarget(arm.ViewType, q.scope, q.full)
		if err != nil {
			return err
		}
		h.Set("mask", fmt.Sprintf("0x%x", reach)).Set("walk", inner).
			Set("sz", sizeCall(arm.ViewType)).Set("args", fnArgs(q.scope)).Block(`
			var sz int64
			var err error
			if m&$mask != 0 {
				sz, err = $walk(d[off:], $argsw, m, depth+1)
			} else {
				var szi int
				szi, err = $sz
				sz = int64(szi)
			}
			if err != nil { return 0, err }
			off += sz
		`)
	}
	g.L("	default:")
	g.L("		return 0, viewErrUnknownDiscriminant(0, disc)")
	g.L("	}")
	g.L("	if off > int64(len(d)) { return 0, viewErrShortBuffer(uint32(off), \"arm exceeds data\") }")
	g.L("	return off, nil")
	g.L("}")
	return nil
}

// emitInlineFn emits walk fns for arrays (element descent without
// attachments? — only reachable when descended via a composite field) and
// optionals (flag + inner descent).
func (we *walkEmitter) emitInlineFn(q queued) error {
	ip := we.inlines[q.goType]
	vt := ip.ViewType
	g := we.f.Use("fn", we.fnFor[q.goType], "cb", we.spec.Callbacks, "params", fnParams(q.scope))
	switch vt.Kind {
	case VKOptional:
		inner := vt.Optional.Element
		reach := we.viewTypeReach(inner)
		g.L("func $fn(d []byte, $paramsw *$cb, m uint64, depth int) (int64, error) {")
		g.L("	if depth > maxDepth { return 0, viewErrMaxDepth(0) }")
		g.L("	if len(d) < 4 { return 0, viewErrShortBuffer(0, \"need 4 bytes for optional flag\") }")
		g.L("	flag := binary.BigEndian.Uint32(d[:4])")
		g.L("	switch flag {")
		g.L("	case 0:")
		g.L("		return 4, nil")
		g.L("	case 1:")
		if reach == 0 {
			g.Set("sz", "size"+inner.GoType+"(d[4:], depth+1)").Block(`
				sz, err := $sz
				if err != nil { return 0, err }
				return 4 + int64(sz), nil
			`)
		} else {
			inn, err := we.descendTarget(inner, q.scope, q.full)
			if err != nil {
				return err
			}
			g.Set("walk", inn).Set("args", fnArgs(q.scope)).Block(`
				sz, err := $walk(d[4:], $argsw, m, depth+1)
				if err != nil { return 0, err }
				return 4 + sz, nil
			`)
		}
		g.L("	default:")
		g.L("		return 0, viewErrBadBoolValue(0, flag)")
		g.L("	}")
		g.L("}")
		return nil
	case VKArray:
		minW, err := arrayMinElemW(ip)
		if err != nil {
			return err
		}
		elem := vt.Array.Element
		inner, err := we.need(elem.GoType, q.scope, true)
		if err != nil {
			return err
		}
		g = g.Set("maxLen", vt.Array.MaxLen).Set("minW", minW).Set("walk", inner).Set("args", fnArgs(q.scope))
		g.Block(`
			func $fn(d []byte, $paramsw *$cb, m uint64, depth int) (int64, error) {
				if depth > maxDepth { return 0, viewErrMaxDepth(0) }
				count, err := arrayViewCountChecked(d, $maxLen, $minW)
				if err != nil { return 0, err }
				off := int64(4)
				for k := 0; k < count; k++ {
					if off >= int64(len(d)) { return 0, viewErrShortBuffer(uint32(off), "element offset exceeds data") }
					sz, err := $walk(d[off:], $argsw, m, depth+1)
					if err != nil { return 0, err }
					off += sz
				}
				return off, nil
			}
		`)
		return nil
	}
	return fmt.Errorf("inline walk for %s: unsupported kind %s", q.goType, vt.Kind)
}

// emitCallbacksAndWrappers emits position consts, the manifest, the callback
// struct with mask(), and the Walk wrappers (root + sub-roots).
func (we *walkEmitter) emitCallbacksAndWrappers() {
	g := we.f.Use("cb", we.spec.Callbacks, "root", we.spec.Root)
	g.L("// $root walk positions (generated manifest, fire order).")
	g.L("const (")
	for i, pos := range we.spec.Positions {
		h := g.Set("pos", pos)
		if i == 0 {
			h.L("	$rootPos$pos uint8 = iota")
		} else {
			h.L("	$rootPos$pos")
		}
	}
	g.L(")")
	g.L("")
	g.L("// $rootWalkPositions is the generated position manifest for Walk$root.")
	g.L("var $rootWalkPositions = []string{")
	for _, pos := range we.spec.Positions {
		g.Set("pos", pos).L("	\"$pos\",")
	}
	g.L("}")
	g.L("")
	g.L("// $cb is the position-keyed subscription set for Walk$root: nil fields are")
	g.L("// unsubscribed, and subtrees containing no subscribed positions are skipped")
	g.L("// via the thin sizing engine. Any callback may return ErrStopWalk to stop")
	g.L("// the walk cleanly. Derived from the schema plan and the walk spec.")
	g.L("type $cb struct {")
	for _, pos := range we.spec.Positions {
		sig := we.sigs[pos]
		var params []string
		for _, a := range sig.args {
			params = append(params, a+" int")
		}
		if sig.payload != "" {
			parts := strings.SplitN(sig.payload, " ", 2)
			params = append(params, parts[0]+" "+parts[1])
		}
		g.Set("pos", pos).Set("sig", strings.Join(params, ", ")).L("	$pos func($sig) error")
	}
	g.L("}")
	g.L("")
	g.L("// mask returns the subscription's position-reachability mask.")
	g.L("func (w *$cb) mask() uint64 {")
	g.L("	var m uint64")
	for _, pos := range we.spec.Positions {
		g.Set("pos", pos).L("	if w.$pos != nil { m |= 1 << $rootPos$pos }")
	}
	g.L("	return m")
	g.L("}")
	roots := append([]string{we.spec.Root}, we.spec.SubRoots...)
	for _, r := range roots {
		h := g.Set("subRoot", r).Set("rfn", we.fnFor[r+"View"]).Set("rT", r+"View")
		scope, _, _ := strings.Cut(we.fnScope[r+"View"], "|")
		zeros := ""
		if scope != "" {
			for range strings.Split(scope, ",") {
				zeros += "0, "
			}
		}
		h = h.Set("zeros", zeros)
		h.Block(`
			// Walk$subRoot drives w over one $subRoot in wire order. A zero subscription returns
			// immediately, validating nothing. Truncation stops round up to element/
			// array advance boundaries, and the walk validates nothing past the last
			// field it owes a subscriber. ErrStopWalk from any callback stops the
			// walk cleanly (returns nil); any other error aborts verbatim.
			func Walk$subRoot(v $rT, w *$cb) error {
				m := w.mask()
				if m == 0 {
					return nil
				}
				_, err := $rfn(v.d, $zerosw, m, 0)
				if err == ErrStopWalk {
					return nil
				}
				return err
			}
		`)
	}
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// appendArg inserts a context arg keeping the spec's declared arg order.
func appendArg(scope []argRef, order []string, a argRef) []argRef {
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	out := append(append([]argRef{}, scope...), a)
	sort.SliceStable(out, func(i, j int) bool { return pos[out[i].name] < pos[out[j].name] })
	return out
}
