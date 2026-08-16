// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// ConvertTestSuite guards the converter layer against the one failure it is
// actually prone to.
//
// Encapsulation costs a translation layer: every value the composition returns
// is rewritten into a type this package owns, so hosts never recompile when an
// inner module changes. That trade is worth making, but a hand-written
// converter does not fail loudly. It fails by silently dropping a field when an
// inner type grows a new one — the projection still compiles, still returns
// something shaped correctly, and quietly omits data the host needed. Nothing
// downstream notices, because nothing downstream knows the field ever existed.
//
// So the completeness check is structural rather than per-field: for each
// projected pair, every field on the inner type must have a counterpart on the
// outer one, or be listed below with a reason. When the composition adds a
// field, this test fails and forces the decision — carry it, or justify
// dropping it — at the moment it is cheap.
type ConvertTestSuite struct {
	suite.Suite
}

// projectedPairs is every inner type this package translates, paired with the
// type it becomes.
var projectedPairs = []struct {
	name  string
	inner any
	outer any
}{
	{"Atlas", encounter.Atlas{}, session.Atlas{}},
	// The composition's per-room footprint is checked against the FLAT map it
	// is folded into (rpg-toolkit#1042), not against a type of its own — there
	// is no longer a room-shaped thing on this side. Pairing it this way keeps
	// every field of a room accounted for: the ones that survive match by
	// name, and the four that describe a room AS a room have to be justified
	// below rather than quietly disappearing with the type that held them.
	{"AtlasRoom folded into Atlas", encounter.AtlasRoom{}, session.Atlas{}},
	{"AtlasBoundary", encounter.AtlasBoundary{}, session.AtlasBoundary{}},
	{"AtlasDoorway", encounter.AtlasDoorway{}, session.AtlasDoorway{}},
	{"Status", encounter.Status{}, session.Status{}},
	{"Outcome", encounter.Outcome{}, session.Outcome{}},
	{"Member", encounter.Member{}, session.Member{}},
	{"MemberOutcome", encounter.MemberOutcome{}, session.MemberOutcome{}},
	{"Sighting", intel.Holding{}, session.Sighting{}},
	{"StoryEntry", record.Entry{}, session.StoryEntry{}},
}

// omitted lists inner fields deliberately not carried across, each with the
// reason. Anything absent from a projection and absent from here is a bug, not
// a decision.
var omitted = map[string]string{
	// A record entry names every viewer a beat was addressed to. Returning that
	// would tell one player which other members exist and were present —
	// including members they have never perceived and rooms they have never
	// entered. The audience is a delivery rule, not story content.
	"record.Entry.Audience": "delivery rule, not story content — naming it leaks unperceived members",

	// The seam speaks ONE MAP (rpg-project#227). The composition keeps rooms
	// and projects the absolute geometry out of them; by the time a map
	// reaches a client the decomposition has done its job.
	"encounter.Atlas.Rooms":  "the decomposition itself — folded into the flat Cells/Occluders/Boundaries",
	"encounter.AtlasRoom.ID": "a room id, which is exactly what the one-map seam stops carrying",
	"encounter.AtlasRoom.Origin": "an anchor exists to project room-local coordinates, and nothing " +
		"on this side has one left to project",
	"encounter.AtlasRoom.Width":  "a room's span; the map's extent is the extent of Cells",
	"encounter.AtlasRoom.Height": "a room's span; the map's extent is the extent of Cells",

	// Placement answers a cell, not a chamber (rpg-toolkit#1046). The room is
	// the composition's own decomposition, and a caller that wanted it would
	// be reconstructing the frame the reshape removed.
	"encounter.Member.Room":        "a room id; the seam reports the cell instead",
	"encounter.MemberOutcome.Room": "a room id; Position is projected onto the map instead",

	// A doorway's endpoints kept their meaning and lost their qualifier: on one
	// map there is no second pair of From/To fields naming rooms to
	// distinguish these from.
	"encounter.AtlasDoorway.From":     "the source ROOM id; the doorway now carries only its two cells",
	"encounter.AtlasDoorway.To":       "the destination ROOM id; same",
	"encounter.AtlasDoorway.FromCell": "renamed to From, now that no room id competes for the name",
	"encounter.AtlasDoorway.ToCell":   "renamed to To, for the same reason",
}

// TestEveryInnerFieldIsCarriedOrJustified is the completeness check.
func (s *ConvertTestSuite) TestEveryInnerFieldIsCarriedOrJustified() {
	for _, pair := range projectedPairs {
		s.Run(pair.name, func() {
			innerType := reflect.TypeOf(pair.inner)
			outerFields := fieldNames(pair.outer)

			for i := 0; i < innerType.NumField(); i++ {
				field := innerType.Field(i)
				if !field.IsExported() {
					continue
				}
				if _, ok := outerFields[field.Name]; ok {
					continue
				}
				key := innerType.String() + "." + field.Name
				reason, justified := omitted[key]
				s.True(justified,
					"%s is not carried into %T and is not listed as a deliberate omission — "+
						"either project it or record why it is dropped",
					key, pair.outer)
				s.NotEmpty(reason, "%s must record WHY it is omitted", key)
			}
		})
	}
}

// TestOmissionsStillApply keeps the exception list from outliving its reasons.
//
// A justified omission for a field that no longer exists is a stale comment
// pretending to be a decision, and it makes the list less trustworthy every
// time someone reads it.
func (s *ConvertTestSuite) TestOmissionsStillApply() {
	known := map[string]bool{}
	for _, pair := range projectedPairs {
		t := reflect.TypeOf(pair.inner)
		for i := 0; i < t.NumField(); i++ {
			known[t.String()+"."+t.Field(i).Name] = true
		}
	}

	for key := range omitted {
		s.True(known[key],
			"%s is listed as a deliberate omission but no longer exists on any projected "+
				"type — remove the entry rather than leaving a reason for a field that is gone",
			key)
	}
}

// TestCompletenessCheckCanActuallyFail is the meta-pin, for the same reason the
// boundary test has one: a completeness check that cannot fail is a green light
// nobody should trust.
func (s *ConvertTestSuite) TestCompletenessCheckCanActuallyFail() {
	type innerWithNewField struct {
		Seq     uint64
		Payload []byte
		Added   string // the composition grew a field and nobody projected it
	}
	type outerMissingIt struct {
		Seq     uint64
		Payload []byte
	}

	outerFields := fieldNames(outerMissingIt{})
	innerType := reflect.TypeOf(innerWithNewField{})

	var missing []string
	for i := 0; i < innerType.NumField(); i++ {
		if _, ok := outerFields[innerType.Field(i).Name]; !ok {
			missing = append(missing, innerType.Field(i).Name)
		}
	}

	s.Equal([]string{"Added"}, missing,
		"the detector must notice a field the projection forgot")
}

func fieldNames(v any) map[string]struct{} {
	t := reflect.TypeOf(v)
	out := make(map[string]struct{}, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		out[t.Field(i).Name] = struct{}{}
	}
	return out
}

func TestConvertSuite(t *testing.T) {
	suite.Run(t, new(ConvertTestSuite))
}
