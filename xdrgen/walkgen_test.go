package main

// The maintenance property the schema-derived walk emitter exists for
// (walk-derivation-design.md §4.5): subscribing a NEW schema node — the
// protocol-upgrade case — costs exactly one spec attachment line (+ a
// manifest name), with traversal, pruning masks, context threading, and
// callback signatures all derived.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

func generateMini(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/mini.json")
	if err != nil {
		t.Fatal(err)
	}
	var ir IR
	if err := jsonUnmarshal(data, &ir); err != nil {
		t.Fatal(err)
	}
	gen, err := NewGenerator(&ir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := gen.GenerateViews()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestWalkSpec_NewPositionIsOneSpecLine extends the mini root's spec with a
// position on a schema node it never touched (Mixed.Count) and asserts the
// derived output gains the position, its mask bit, and its fire site —
// no traversal code was written.
func TestWalkSpec_NewPositionIsOneSpecLine(t *testing.T) {
	base := generateMini(t)
	if !strings.Contains(base, "func WalkOptionalEntry(") {
		t.Fatal("mini golden must derive the OptionalEntry walker")
	}
	if strings.Contains(base, "EntryCount") {
		t.Fatal("baseline must not contain the hypothetical position")
	}

	// The upgrade: ONE attachment line + the manifest name.
	saved := walkRoots
	defer func() { walkRoots = saved }()
	mod := make([]walkRootSpec, len(walkRoots))
	copy(mod, walkRoots)
	for i := range mod {
		if mod[i].Root != "OptionalEntry" {
			continue
		}
		mod[i].Positions = append(append([]string{}, mod[i].Positions...), "EntryCount")
		att := map[attachKey]walkAttach{}
		for k, v := range mod[i].Attach {
			att[k] = v
		}
		att[attachKey{"MixedView", "Count"}] = walkAttach{FieldView: "EntryCount"}
		mod[i].Attach = att
	}
	walkRoots = mod

	got := generateMini(t)
	flat := strings.Join(strings.Fields(got), " ") // collapse gofmt alignment
	for _, want := range []string{
		"OptionalEntryPosEntryCount",         // manifest const derived
		"EntryCount func(v Int32View) error", // signature derived from schema + scope
		"if w.EntryCount != nil {",           // fire site derived into the traversal
		`"EntryCount",`,                      // runtime manifest entry
	} {
		if !strings.Contains(flat, want) {
			t.Fatalf("derived output missing %q after a one-line spec upgrade", want)
		}
	}

	// And the reshape tripwire: an attachment to a nonexistent node fails
	// generation loudly.
	for i := range mod {
		if mod[i].Root == "OptionalEntry" {
			mod[i].Attach[attachKey{"MixedView", "NoSuchField"}] = walkAttach{FieldView: "EntryCount"}
		}
	}
	data, _ := os.ReadFile("testdata/mini.json")
	var ir2 IR
	_ = jsonUnmarshal(data, &ir2)
	gen, _ := NewGenerator(&ir2)
	if _, err := gen.GenerateViews(); err == nil || !strings.Contains(err.Error(), "field not found in plan") {
		t.Fatalf("expected the reshape tripwire, got %v", err)
	}
}
