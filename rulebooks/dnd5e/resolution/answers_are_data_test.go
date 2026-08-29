// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEntriesReturnAnswersNotSheets is the D11 pin: nothing an entry hands back
// carries a ToData().
//
// # Why ToData is the thing being matched
//
// It is the marker of a SHEET. A live character or monster can serialize itself,
// because it is the thing the repository stores; an answer cannot, because it is
// a number somebody derived. So a type with a ToData() crossing this boundary
// means the seam stopped holding records and started holding sheets — and a
// caller given one has a serialization affordance outside the keeper discipline,
// which is exactly the probe that produced this rule ("if we return character
// doesn't that have a ToData() on it?").
//
// # Outputs only, and inputs deliberately not
//
// Records go IN. That is the whole shape: the seam fetches records and only
// records, hands them down, and takes answers back. character.Data has no
// ToData of its own — it IS the data — but even if a record type grew one, an
// input carrying it would be correct. The direction is what matters, so the
// direction is what this walks.
//
// # It walks the whole shape, not the top level
//
// A struct field, a pointer, a slice element, a map value: each is a way to
// smuggle a sheet one level down where a top-level check would not look. The
// walk is recursive and cycle-safe, so nesting is not an escape.
func TestEntriesReturnAnswersNotSheets(t *testing.T) {
	outputs := []struct {
		entry string
		typ   reflect.Type
	}{
		{"ProjectCharacter", reflect.TypeOf(ProjectCharacterOutput{})},
		{"Standing", reflect.TypeOf(StandingOutput{})},
		{"Preflight", reflect.TypeOf(PreflightOutput{})},
		{"Resolve", reflect.TypeOf(Output{})},
	}

	for _, out := range outputs {
		seen := map[reflect.Type]bool{}
		offenders := serializableTypesIn(out.typ, seen, nil)

		require.Empty(t, offenders,
			"%sOutput reaches a type with a ToData(), which is what a SHEET has and an "+
				"ANSWER does not. A caller handed one can serialize a live object the "+
				"keeper is supposed to own. Send numbers instead: %v",
			out.entry, offenders)
	}
}

// serializableTypesIn returns the paths within t that reach a type declaring a
// ToData method, on the type or on a pointer to it.
func serializableTypesIn(t reflect.Type, seen map[reflect.Type]bool, path []string) []string {
	if t == nil || seen[t] {
		return nil
	}
	seen[t] = true

	if hasToData(t) {
		return []string{pathOf(path) + " (" + t.String() + ")"}
	}

	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return serializableTypesIn(t.Elem(), seen, path)
	case reflect.Map:
		return serializableTypesIn(t.Elem(), seen, path)
	case reflect.Struct:
		var found []string
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			found = append(found, serializableTypesIn(field.Type, seen, append(path, field.Name))...)
		}

		return found
	default:
		return nil
	}
}

// hasToData reports whether t, or a pointer to t, declares ToData.
func hasToData(t reflect.Type) bool {
	if _, ok := t.MethodByName("ToData"); ok {
		return true
	}
	if t.Kind() != reflect.Pointer {
		if _, ok := reflect.PointerTo(t).MethodByName("ToData"); ok {
			return true
		}
	}

	return false
}

func pathOf(path []string) string {
	if len(path) == 0 {
		return "<the output itself>"
	}
	out := path[0]
	for _, p := range path[1:] {
		out += "." + p
	}

	return out
}
