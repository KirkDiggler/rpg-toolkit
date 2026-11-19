// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

// contenthash_internal_test.go proves the content hash cannot miss a part of
// the content (rpg-project#1001): every place the types' JSON would drop
// state is spelled by the hash's own encoding, and what the encoding cannot
// spell is refused.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// jsonDrop is one place the types' JSON loses state: an interface holding a
// struct with no exported field (it encodes as {}), or an unexported or
// `json:"-"` field holding data. token is what the hash's encoding must
// write for it.
type jsonDrop struct{ name, token string }

// jsonDrops is every place in v the types' JSON would lose state.
func jsonDrops(v reflect.Value) []jsonDrop {
	found := map[string]jsonDrop{}
	var walk func(v reflect.Value, depth int)
	walk = func(v reflect.Value, depth int) {
		if depth > canonicalDepth || !v.IsValid() {
			return
		}
		switch v.Kind() {
		case reflect.Interface, reflect.Pointer:
			if v.IsNil() {
				return
			}
			if elem := v.Elem(); v.Kind() == reflect.Interface && elem.Kind() == reflect.Struct && !hasExported(elem.Type()) {
				name := "interface " + v.Type().String() + " holding " + elem.Type().String()
				found[name] = jsonDrop{name, "<" + typeName(elem.Type()) + ">"}
			}
			walk(v.Elem(), depth+1)
		case reflect.Struct:
			t := v.Type()
			for i := 0; i < t.NumField(); i++ {
				f, fv := t.Field(i), v.Field(i)
				hidden := !f.IsExported() || f.Tag.Get("json") == "-"
				if hidden && !fv.IsZero() {
					name := "field " + t.String() + "." + f.Name
					found[name] = jsonDrop{name, typeName(t) + "{"}
				}
				if f.IsExported() {
					walk(fv, depth+1)
				}
			}
		case reflect.Map:
			iter := v.MapRange()
			for iter.Next() {
				walk(iter.Value(), depth+1)
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i), depth+1)
			}
		}
	}
	walk(v, 0)
	out := make([]jsonDrop, 0, len(found))
	for _, d := range found {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// hasExported reports whether a struct type has an exported field.
func hasExported(t reflect.Type) bool {
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).IsExported() {
			return true
		}
	}
	return false
}

// TestNothingHashesAsNothing walks the compiled form of every content file
// but the castle (whose drops the smaller files all hold):
// each place the types' JSON drops state is written by the hash's encoding,
// and the places found are exactly these, so a new one is reviewed, not
// slipped in.
func TestNothingHashesAsNothing(t *testing.T) {
	files, err := filepath.Glob("testdata/*.yaml")
	require.NoError(t, err)
	names := map[string]bool{}
	for _, f := range files {
		if filepath.Base(f) == "world-builder-v4-castle.yaml" {
			continue // its drops are all in smaller files, and its cost is the budget's
		}
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		c, err := Load(raw)
		if err != nil {
			continue // a refused file compiles to nothing to hash
		}
		var canon bytes.Buffer
		require.NoError(t, canonicalEncode(&canon, reflect.ValueOf(c), "Compiled", 0), f)
		for _, d := range jsonDrops(reflect.ValueOf(c)) {
			require.Contains(t, canon.String(), d.token, "%s: the hash spells %s", filepath.Base(f), d.name)
			names[d.name] = true
		}
	}
	var found []string
	for n := range names {
		found = append(found, n)
	}
	sort.Strings(found)
	require.Equal(t, []string{
		"field encounter.doorLocked.lock",
		"interface encounter.DoorState holding encounter.doorClosed",
		"interface encounter.DoorState holding encounter.doorLocked",
		"interface encounter.DoorState holding encounter.doorOpen",
		"interface encounter.Orientation holding encounter.pointyTop",
		"interface encounter.Void holding encounter.voidOpaque",
		"interface encounter.Void holding encounter.voidTransparent",
	}, found, "every place the types' JSON drops state in today's content")
}

// TestTheEncodingSpellsWhatJSONDrops: a value's concrete type, an
// unexported field and a `json:"-"` field each change the encoding; what it
// cannot spell is refused.
func TestTheEncodingSpellsWhatJSONDrops(t *testing.T) {
	type a struct{ N int }
	type b struct{ N int }
	type hidden struct {
		n    int
		Skip int `json:"-"`
	}
	type holder struct{ V any }
	enc := func(v any) string {
		t.Helper()
		var buf bytes.Buffer
		require.NoError(t, canonicalEncode(&buf, reflect.ValueOf(v), "v", 0))
		return buf.String()
	}
	ja, _ := json.Marshal(holder{a{1}})
	jb, _ := json.Marshal(holder{b{1}})
	require.Equal(t, string(ja), string(jb), "precondition: JSON cannot tell two types of one shape apart")
	require.NotEqual(t, enc(holder{a{1}}), enc(holder{b{1}}), "the encoding can")
	require.NotEqual(t, enc(hidden{n: 1}), enc(hidden{n: 2}), "an unexported field is hashed")
	require.NotEqual(t, enc(hidden{Skip: 1}), enc(hidden{Skip: 2}), "a json:\"-\" field is hashed")
	require.NotEqual(t, enc(encounter.CanvasInput{Void: encounter.VoidIsOpaque()}), enc(encounter.CanvasInput{Void: encounter.VoidIsTransparent()}))
	require.NotEqual(t, enc(encounter.CanvasInput{Orientation: encounter.HexesArePointyTop()}),
		enc(encounter.CanvasInput{Orientation: encounter.HexesAreFlatTop()}))

	var buf bytes.Buffer
	err := canonicalEncode(&buf, reflect.ValueOf(holder{func() {}}), "v", 0)
	require.ErrorContains(t, err, "a func cannot be hashed")
	require.True(t, strings.Contains(enc(holder{}), "nil"), "an empty interface is nil, not nothing")
}
