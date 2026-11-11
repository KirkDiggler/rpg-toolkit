// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
)

// jsonNames is every JSON name a struct type's fields carry.
func jsonNames(t reflect.Type) []string {
	out := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		out = append(out, name)
	}
	return out
}

// TestPieces_EveryFieldHasAPiece holds the split to EncounterData's fields:
// every field is carried by exactly one piece, under the name the rest
// writes it with. A field added to EncounterData and to no piece would be
// lost on every save in silence; this fails instead. Members, the journal,
// perception's holdings and the sight sweeps are the per-key pieces; the
// rest are the head, roster and knowledge pieces' fields. Perception is
// intel's Data verbatim with one field, holdings, which the observer pieces
// carry whole; a second field in either would be lost the same way.
func TestPieces_EveryFieldHasAPiece(t *testing.T) {
	carried := []string{"members", "journal", "perception", "sight_sweeps"}
	for _, piece := range []any{headPiece{}, rosterPiece{}, knowledgePiece{}} {
		carried = append(carried, jsonNames(reflect.TypeOf(piece))...)
	}
	slices.Sort(carried)
	require.Equal(t, len(slices.Compact(slices.Clone(carried))), len(carried), "no field in two pieces: %v", carried)
	fields := jsonNames(reflect.TypeOf(EncounterData{}))
	slices.Sort(fields)
	require.Equal(t, fields, carried, "every EncounterData field is carried by one piece")

	require.Equal(t, []string{"intel"}, jsonNames(reflect.TypeOf(perception.Data{})), "perception is intel's Data alone")
	require.Equal(t, []string{"holdings"}, jsonNames(reflect.TypeOf(intel.Data{})), "intel's Data is its holdings alone")
	for _, piece := range []any{headPiece{}, rosterPiece{}, knowledgePiece{}} {
		pt := reflect.TypeOf(piece)
		et := reflect.TypeOf(EncounterData{})
		for i := 0; i < pt.NumField(); i++ {
			f, ok := et.FieldByName(pt.Field(i).Name)
			require.True(t, ok, "%s.%s is an EncounterData field", pt.Name(), pt.Field(i).Name)
			require.Equal(t, f.Type, pt.Field(i).Type, "%s.%s has EncounterData's type", pt.Name(), f.Name)
			require.Equal(t, f.Tag.Get("json"), pt.Field(i).Tag.Get("json"), "%s.%s has EncounterData's tag", pt.Name(), f.Name)
		}
	}
}
