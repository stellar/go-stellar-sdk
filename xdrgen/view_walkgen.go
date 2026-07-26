package main

import "fmt"

// Tier-2 Walk emission (two-tier visitor design): per-root WalkX drivers with
// position-keyed callbacks over the thin engine, mirroring the stage-1 spike
// (per-event granularity, adopted by the pre-committed ladder).
//
// Position enumeration is TABLE-DRIVEN per root (flagged deviation from
// fully-schema-derived enumeration): each root declares its subscribable
// positions here, and emission cross-checks the schema at generation time by
// resolving every array type the walker traverses from the plan — a renamed
// or reshaped node fails generation loudly (the protocol-upgrade tripwire the
// T-1 manifest test complements at run time).

// walkRootSpec declares one walk root.
type walkRootSpec struct {
	RootName string // XDR definition name, e.g. "LedgerCloseMeta"
	// Array types whose count bounds the walker needs, resolved from the
	// plan at emission time: name -> template var.
	CountBounds map[string]string
	// Positions in fire order (the manifest).
	Positions []string
}

var walkRoots = []walkRootSpec{{
	RootName: "LedgerCloseMeta",
	CountBounds: map[string]string{
		"LedgerCloseMetaV0TxProcessingView":          "tpMinW0",
		"LedgerCloseMetaV2TxProcessingView":          "tpMinW2",
		"TransactionMetaV4OperationsView":            "opsMinW",
		"SorobanTransactionMetaEventsView":           "cevMinW",
		"TransactionMetaV4EventsView":                "tevMinW",
		"SorobanTransactionMetaDiagnosticEventsView": "devMinW",
	},
	Positions: []string{
		"TxProcessingBegin", "ResultPair", "MetaVersion", "V4Ops",
		"OpEventsBegin", "OpEvent", "TxEventsBegin", "TxEvent",
		"DiagEventsBegin", "DiagEvent", "TxMeta",
	},
}}

// emitWalkRoots emits every configured walk root, resolving schema-derived
// constants from the plan.
func emitWalkRoots(f *GeneratedFile, plan *ViewPlan) error {
	byName := map[string]*InlineTypePlan{}
	rootPresent := map[string]bool{}
	for _, e := range plan.Entries {
		switch t := e.(type) {
		case *InlineTypePlan:
			byName[t.Name] = t
		case *UnionViewPlan:
			rootPresent[t.ViewTypeName] = true
		case *StructViewPlan:
			rootPresent[t.ViewTypeName] = true
		}
	}
	for _, spec := range walkRoots {
		if !rootPresent[spec.RootName+"View"] {
			// The root type is not part of this schema (e.g. the golden mini
			// corpus): nothing to emit. A PRESENT root with missing interior
			// types still hard-errors below — that is the reshape tripwire.
			continue
		}
		g := f.Use("root", spec.RootName)
		for typeName, tmplVar := range spec.CountBounds {
			ip, ok := byName[typeName]
			if !ok {
				return fmt.Errorf("walk root %s: array type %s not found in plan "+
					"(schema reshape? update walkRoots)", spec.RootName, typeName)
			}
			w, err := arrayMinElemW(ip)
			if err != nil {
				return err
			}
			g = g.Set(tmplVar, w)
		}
		if spec.RootName == "LedgerCloseMeta" {
			emitLedgerCloseMetaWalk(g, spec)
		} else {
			return fmt.Errorf("walk root %s: no emitter registered", spec.RootName)
		}
	}
	return nil
}

// emitLedgerCloseMetaWalk emits the LedgerCloseMeta walker: position consts +
// manifest, the callback struct with its reachability mask, and the driver.
// The driver prunes unsubscribed subtrees via thin size skips; truncation
// stops round up to element/array advance boundaries (scope-derived: the walk
// validates exactly what it traverses and owes nothing past TxProcessing).
func emitLedgerCloseMetaWalk(g Printer, spec walkRootSpec) {
	// Position constants + manifest.
	g.L("// $root walk positions (generated manifest, fire order).")
	g.L("const (")
	for i, pos := range spec.Positions {
		h := g.Set("pos", pos).Set("i", i)
		if i == 0 {
			h.L("	LedgerCloseMetaPos$pos uint8 = iota")
		} else {
			h.L("	LedgerCloseMetaPos$pos")
		}
	}
	g.L(")")
	g.L("")
	g.L("// LedgerCloseMetaWalkPositions is the generated position manifest for WalkLedgerCloseMeta.")
	g.L("var LedgerCloseMetaWalkPositions = []string{")
	for _, pos := range spec.Positions {
		g.Set("pos", pos).L("	\"$pos\",")
	}
	g.L("}")

	g.Block(`
		// LedgerCloseMetaWalk is the position-keyed subscription set for
		// WalkLedgerCloseMeta. nil fields are unsubscribed: subtrees containing no
		// subscribed positions are skipped via the thin sizing engine. Event-group
		// construction is version-discriminating by construction: OpEventsBegin
		// fires at most once for a V3 meta (opIdx 0, gated ONLY on SorobanMeta
		// presence) and once per V4 op (count from nOps) — never an arm-merged op
		// count. Any callback may return ErrStopWalk to stop the walk cleanly.
		type LedgerCloseMetaWalk struct {
			// TxProcessingBegin fires once with the validated TxProcessing count
			// (output presizing).
			TxProcessingBegin func(count int) error
			// ResultPair delivers TxProcessing[txIdx].Result as an exact-extent view.
			ResultPair func(txIdx int, pair TransactionResultPairView) error
			// MetaVersion delivers the tx's TransactionMeta discriminant.
			MetaVersion func(txIdx int, v int32) error
			// V4Ops fires before a V4 meta's operations with the op count.
			V4Ops func(txIdx, nOps int) error
			// OpEventsBegin fires once per contract-event group with its count.
			OpEventsBegin func(txIdx, opIdx, count int) error
			// OpEvent delivers one contract event (exact extent).
			OpEvent func(txIdx, opIdx, evIdx int, ev ContractEventView) error
			// TxEventsBegin fires once per V4 meta with the top-level event count.
			TxEventsBegin func(txIdx, count int) error
			// TxEvent delivers one V4 top-level transaction event (exact extent).
			TxEvent func(txIdx, evIdx int, ev TransactionEventView) error
			// DiagEventsBegin fires once per meta carrying diagnostics (V3
			// soroban-present or V4) with the diagnostic-event count.
			DiagEventsBegin func(txIdx, count int) error
			// DiagEvent delivers one diagnostic event (exact extent).
			DiagEvent func(txIdx, evIdx int, ev DiagnosticEventView) error
			// TxMeta delivers the tx's whole TransactionMeta as an exact-extent
			// view AFTER its interior positions fired (the walk just measured it,
			// so Raw() on the delivered view is a slice operation).
			TxMeta func(txIdx int, meta TransactionMetaView) error
		}

		// mask returns the subscription's position-reachability mask (bit i =
		// LedgerCloseMetaPos value i is subscribed).
		func (w *LedgerCloseMetaWalk) mask() uint64 {
			var m uint64
			if w.TxProcessingBegin != nil { m |= 1 << LedgerCloseMetaPosTxProcessingBegin }
			if w.ResultPair != nil { m |= 1 << LedgerCloseMetaPosResultPair }
			if w.MetaVersion != nil { m |= 1 << LedgerCloseMetaPosMetaVersion }
			if w.V4Ops != nil { m |= 1 << LedgerCloseMetaPosV4Ops }
			if w.OpEventsBegin != nil { m |= 1 << LedgerCloseMetaPosOpEventsBegin }
			if w.OpEvent != nil { m |= 1 << LedgerCloseMetaPosOpEvent }
			if w.TxEventsBegin != nil { m |= 1 << LedgerCloseMetaPosTxEventsBegin }
			if w.TxEvent != nil { m |= 1 << LedgerCloseMetaPosTxEvent }
			if w.DiagEventsBegin != nil { m |= 1 << LedgerCloseMetaPosDiagEventsBegin }
			if w.DiagEvent != nil { m |= 1 << LedgerCloseMetaPosDiagEvent }
			if w.TxMeta != nil { m |= 1 << LedgerCloseMetaPosTxMeta }
			return m
		}

		// Subtree reachability masks: which positions live under each pruned node.
		const (
			lcmMaskMeta = 1<<LedgerCloseMetaPosMetaVersion | 1<<LedgerCloseMetaPosV4Ops |
				1<<LedgerCloseMetaPosOpEventsBegin | 1<<LedgerCloseMetaPosOpEvent |
				1<<LedgerCloseMetaPosTxEventsBegin | 1<<LedgerCloseMetaPosTxEvent |
				1<<LedgerCloseMetaPosDiagEventsBegin | 1<<LedgerCloseMetaPosDiagEvent |
				1<<LedgerCloseMetaPosTxMeta
			lcmMaskOpEvents  = 1<<LedgerCloseMetaPosOpEventsBegin | 1<<LedgerCloseMetaPosOpEvent
			lcmMaskTxEvents  = 1<<LedgerCloseMetaPosTxEventsBegin | 1<<LedgerCloseMetaPosTxEvent
			lcmMaskDiagEvents = 1<<LedgerCloseMetaPosDiagEventsBegin | 1<<LedgerCloseMetaPosDiagEvent
		)

		// WalkLedgerCloseMeta drives w over one LedgerCloseMeta in wire order. A
		// zero subscription returns immediately, validating nothing. Truncation
		// stops round up to element/array advance boundaries: the walk errors
		// exactly where a thin size or count read fails, and validates nothing
		// past the last field it owes a subscriber (scope-derived semantics).
		func WalkLedgerCloseMeta(v LedgerCloseMetaView, w *LedgerCloseMetaWalk) error {
			err := walkLedgerCloseMetaMasked(v.d, w, w.mask())
			if err == ErrStopWalk {
				return nil
			}
			return err
		}

		func walkLedgerCloseMetaMasked(d []byte, w *LedgerCloseMetaWalk, m uint64) error {
			if m == 0 {
				return nil
			}
			if len(d) < 4 {
				return viewErrShortBuffer(0, "need 4 bytes for discriminant")
			}
			disc := int32(binary.BigEndian.Uint32(d[:4]))
			off := int64(4)
			var minElemW int
			switch disc {
			case 0:
				sz, err := sizeLedgerHeaderHistoryEntryView(d[off:], 1)
				if err != nil {
					return err
				}
				off += int64(sz)
				if sz, err = sizeTransactionSetView(d[off:], 1); err != nil {
					return err
				}
				off += int64(sz)
				minElemW = $tpMinW0
			case 1, 2:
				sz, err := sizeLedgerCloseMetaExtView(d[off:], 1)
				if err != nil {
					return err
				}
				off += int64(sz)
				if sz, err = sizeLedgerHeaderHistoryEntryView(d[off:], 1); err != nil {
					return err
				}
				off += int64(sz)
				if sz, err = sizeGeneralizedTransactionSetView(d[off:], 1); err != nil {
					return err
				}
				off += int64(sz)
				minElemW = $tpMinW0
				if disc == 2 {
					minElemW = $tpMinW2
				}
			default:
				return viewErrUnknownDiscriminant(0, disc)
			}
			if off > int64(len(d)) {
				return viewErrShortBuffer(uint32(off), "field offset exceeds data")
			}
			count, err := arrayViewCountChecked(d[off:], 0, minElemW)
			if err != nil {
				return err
			}
			if w.TxProcessingBegin != nil {
				if err := w.TxProcessingBegin(count); err != nil {
					return err
				}
			}
			off += 4
			for tx := 0; tx < count; tx++ {
				if off > int64(len(d)) {
					return viewErrShortBuffer(uint32(off), "element offset exceeds data")
				}
				if disc == 2 {
					off += 4 // TransactionResultMetaV1's leading ExtensionPoint
					if off > int64(len(d)) {
						return viewErrShortBuffer(uint32(off), "field offset exceeds data")
					}
				}
				sz, err := sizeTransactionResultPairView(d[off:], 2)
				if err != nil {
					return err
				}
				if w.ResultPair != nil {
					if err := w.ResultPair(tx, TransactionResultPairView{view{d: d[off : off+int64(sz)], exact: true}}); err != nil {
						return err
					}
				}
				off += int64(sz)
				if sz, err = sizeLedgerEntryChangesView(d[off:], 2); err != nil {
					return err
				}
				off += int64(sz)
				if m&lcmMaskMeta == 0 {
					if sz, err = sizeTransactionMetaView(d[off:], 2); err != nil {
						return err
					}
					off += int64(sz)
				} else {
					msz, err := walkLCMTransactionMeta(d[off:], tx, w, m)
					if err != nil {
						return err
					}
					if w.TxMeta != nil {
						if err := w.TxMeta(tx, TransactionMetaView{view{d: d[off : off+msz], exact: true}}); err != nil {
							return err
						}
					}
					off += msz
				}
				if disc == 2 {
					if off > int64(len(d)) {
						return viewErrShortBuffer(uint32(off), "field offset exceeds data")
					}
					if sz, err = sizeLedgerEntryChangesView(d[off:], 2); err != nil {
						return err
					}
					off += int64(sz)
				}
				if off > int64(len(d)) {
					return viewErrShortBuffer(uint32(off), "element exceeds data")
				}
			}
			return nil
		}

		func walkLCMTransactionMeta(d []byte, tx int, w *LedgerCloseMetaWalk, m uint64) (int64, error) {
			if len(d) < 4 {
				return 0, viewErrShortBuffer(0, "need 4 bytes for discriminant")
			}
			v := int32(binary.BigEndian.Uint32(d[:4]))
			if w.MetaVersion != nil {
				if err := w.MetaVersion(tx, v); err != nil {
					return 0, err
				}
			}
			off := int64(4)
			switch v {
			case 0:
				sz, err := sizeTransactionMetaOperationsView(d[off:], 3)
				if err != nil {
					return 0, err
				}
				off += int64(sz)
			case 1:
				sz, err := sizeTransactionMetaV1View(d[off:], 3)
				if err != nil {
					return 0, err
				}
				off += int64(sz)
			case 2:
				sz, err := sizeTransactionMetaV2View(d[off:], 3)
				if err != nil {
					return 0, err
				}
				off += int64(sz)
			case 3:
				off += 4 // ExtensionPoint
				if off > int64(len(d)) {
					return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data")
				}
				sz, err := sizeLedgerEntryChangesView(d[off:], 4)
				if err != nil {
					return 0, err
				}
				off += int64(sz)
				if sz, err = sizeTransactionMetaV3OperationsView(d[off:], 4); err != nil {
					return 0, err
				}
				off += int64(sz)
				if sz, err = sizeLedgerEntryChangesView(d[off:], 4); err != nil {
					return 0, err
				}
				off += int64(sz)
				if off+4 > int64(len(d)) {
					return 0, viewErrShortBuffer(uint32(off), "need 4 bytes for optional flag")
				}
				flag := binary.BigEndian.Uint32(d[off : off+4])
				off += 4
				switch flag {
				case 0:
					// SorobanMeta absent: no event group (presence, not op
					// count, gates the group).
				case 1:
					sz, err := sizeSorobanTransactionMetaExtView(d[off:], 5)
					if err != nil {
						return 0, err
					}
					off += int64(sz)
					evsz, err := walkLCMContractEvents(d[off:], tx, 0, w, m)
					if err != nil {
						return 0, err
					}
					off += evsz
					if sz, err = sizeScValView(d[off:], 5); err != nil {
						return 0, err
					}
					off += int64(sz)
					dvsz, err := walkLCMDiagEvents(d[off:], tx, w, m)
					if err != nil {
						return 0, err
					}
					off += dvsz
				default:
					return 0, viewErrBadBoolValue(uint32(off-4), flag)
				}
			case 4:
				off += 4 // ExtensionPoint
				if off > int64(len(d)) {
					return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data")
				}
				sz, err := sizeLedgerEntryChangesView(d[off:], 4)
				if err != nil {
					return 0, err
				}
				off += int64(sz)
				if off > int64(len(d)) {
					return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data")
				}
				nOps, err := arrayViewCountChecked(d[off:], 0, $opsMinW)
				if err != nil {
					return 0, err
				}
				if w.V4Ops != nil {
					if err := w.V4Ops(tx, nOps); err != nil {
						return 0, err
					}
				}
				off += 4
				for op := 0; op < nOps; op++ {
					off += 4 // OperationMetaV2's ExtensionPoint
					if off > int64(len(d)) {
						return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data")
					}
					if sz, err = sizeLedgerEntryChangesView(d[off:], 5); err != nil {
						return 0, err
					}
					off += int64(sz)
					evsz, err := walkLCMContractEvents(d[off:], tx, op, w, m)
					if err != nil {
						return 0, err
					}
					off += evsz
				}
				if sz, err = sizeLedgerEntryChangesView(d[off:], 4); err != nil {
					return 0, err
				}
				off += int64(sz)
				if sz, err = sizeTransactionMetaV4SorobanMetaOptView(d[off:], 4); err != nil {
					return 0, err
				}
				off += int64(sz)
				evsz, err := walkLCMTransactionEvents(d[off:], tx, w, m)
				if err != nil {
					return 0, err
				}
				off += evsz
				dvsz, err := walkLCMDiagEvents(d[off:], tx, w, m)
				if err != nil {
					return 0, err
				}
				off += dvsz
			default:
				return 0, viewErrUnknownDiscriminant(0, v)
			}
			if off > int64(len(d)) {
				return 0, viewErrShortBuffer(uint32(off), "meta exceeds data")
			}
			return off, nil
		}

		func walkLCMContractEvents(d []byte, tx, op int, w *LedgerCloseMetaWalk, m uint64) (int64, error) {
			if m&lcmMaskOpEvents == 0 {
				sz, err := sizeSorobanTransactionMetaEventsView(d, 5)
				return int64(sz), err
			}
			count, err := arrayViewCountChecked(d, 0, $cevMinW)
			if err != nil {
				return 0, err
			}
			if w.OpEventsBegin != nil {
				if err := w.OpEventsBegin(tx, op, count); err != nil {
					return 0, err
				}
			}
			off := int64(4)
			for k := 0; k < count; k++ {
				if off > int64(len(d)) {
					return 0, viewErrShortBuffer(uint32(off), "element offset exceeds data")
				}
				sz, err := sizeContractEventView(d[off:], 6)
				if err != nil {
					return 0, err
				}
				if w.OpEvent != nil {
					if err := w.OpEvent(tx, op, k, ContractEventView{view{d: d[off : off+int64(sz)], exact: true}}); err != nil {
						return 0, err
					}
				}
				off += int64(sz)
			}
			return off, nil
		}

		func walkLCMDiagEvents(d []byte, tx int, w *LedgerCloseMetaWalk, m uint64) (int64, error) {
			if m&lcmMaskDiagEvents == 0 {
				sz, err := sizeSorobanTransactionMetaDiagnosticEventsView(d, 5)
				return int64(sz), err
			}
			count, err := arrayViewCountChecked(d, 0, $devMinW)
			if err != nil {
				return 0, err
			}
			if w.DiagEventsBegin != nil {
				if err := w.DiagEventsBegin(tx, count); err != nil {
					return 0, err
				}
			}
			off := int64(4)
			for k := 0; k < count; k++ {
				if off > int64(len(d)) {
					return 0, viewErrShortBuffer(uint32(off), "element offset exceeds data")
				}
				sz, err := sizeDiagnosticEventView(d[off:], 6)
				if err != nil {
					return 0, err
				}
				if w.DiagEvent != nil {
					if err := w.DiagEvent(tx, k, DiagnosticEventView{view{d: d[off : off+int64(sz)], exact: true}}); err != nil {
						return 0, err
					}
				}
				off += int64(sz)
			}
			return off, nil
		}

		func walkLCMTransactionEvents(d []byte, tx int, w *LedgerCloseMetaWalk, m uint64) (int64, error) {
			if m&lcmMaskTxEvents == 0 {
				sz, err := sizeTransactionMetaV4EventsView(d, 5)
				return int64(sz), err
			}
			count, err := arrayViewCountChecked(d, 0, $tevMinW)
			if err != nil {
				return 0, err
			}
			if w.TxEventsBegin != nil {
				if err := w.TxEventsBegin(tx, count); err != nil {
					return 0, err
				}
			}
			off := int64(4)
			for k := 0; k < count; k++ {
				if off > int64(len(d)) {
					return 0, viewErrShortBuffer(uint32(off), "element offset exceeds data")
				}
				sz, err := sizeTransactionEventView(d[off:], 6)
				if err != nil {
					return 0, err
				}
				if w.TxEvent != nil {
					if err := w.TxEvent(tx, k, TransactionEventView{view{d: d[off : off+int64(sz)], exact: true}}); err != nil {
						return 0, err
					}
				}
				off += int64(sz)
			}
			return off, nil
		}
	`)
}
