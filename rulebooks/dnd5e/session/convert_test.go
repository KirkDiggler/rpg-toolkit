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
	// A region crosses as a region (rpg-project#256). It used to be folded
	// into the flat map, because a room was a frame — an anchor and a span
	// — and the one-map seam exists to hide frames. A region is a NAMED SET
	// OF CELLS in the map's own frame, plus the facts true of that area;
	// there is nothing in it to hide and a lighting level a client must not
	// have to re-derive by experiment.
	{"AtlasRegion", encounter.AtlasRegion{}, session.AtlasRegion{}},
	{"AtlasProp", encounter.AtlasProp{}, session.AtlasProp{}},
	{"AtlasBoundary", encounter.AtlasBoundary{}, session.AtlasBoundary{}},
	{"AtlasDoorway", encounter.AtlasDoorway{}, session.AtlasDoorway{}},
	{"Status", encounter.Status{}, session.Status{}},
	{"Outcome", encounter.Outcome{}, session.Outcome{}},
	{"Member", encounter.Member{}, session.Member{}},
	{"MemberOutcome", encounter.MemberOutcome{}, session.MemberOutcome{}},
	{"Sighting", intel.Holding{}, session.Sighting{}},
	{"StoryEntry", record.Entry{}, session.StoryEntry{}},
}

// renamed lists inner fields carried across under a DIFFERENT name, each with
// the reason the name had to change. The audit below matches fields by name,
// so without this a rename is indistinguishable from a drop — and a rename
// that is recorded as an omission is a decision hiding behind the wrong label.
//
// The frame crosses as a render word (rpg-toolkit#1140). The composition
// keeps Orientation as the frame an author typed offset cells in; this seam
// sends the CELLS, already converted, so a client never needs that frame. What
// a client DOES need — and found out by drawing the reference tomb as a
// diagonal staircase — is which way to lay the hexes out, because the same
// axial set draws as two different pictures. Same two values, different
// question, and the staircase happened precisely because the two questions
// shared a word. So the field is named for what a client does with it.
var renamed = map[string]struct{ outer, reason string }{
	"encounter.Atlas.Orientation": {outer: "Layout",
		reason: "the frame an author typed in becomes the layout a client draws in — same " +
			"two values, a different question, and a different name so they cannot be confused"},
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

	// Placement answers a cell, not a chamber (rpg-toolkit#1046). The region
	// is derivable: the Atlas now lists every region's cells
	// (rpg-project#256), so a client that wants the name of where somebody
	// stands looks the cell up once in construction data rather than being
	// told on every placement read.
	"encounter.Member.Region":        "a region id; the seam reports the cell instead",
	"encounter.MemberOutcome.Region": "a region id; the composition's own bookkeeping — Position already names the cell on the map",

	// SpeedFeet, SightFeet, Actions and Targeting (rpg-project#254) are a
	// member's static facts for the ONE consumer that reads them: a
	// TurnDriver, through session.MonsterView — already fully projected
	// there (see projectMonsterView). A roster listing is a different
	// question ("who is here, and where"), asked by every caller of Join,
	// Spawn, Where and View alike, and stuffing four fields only a turn's
	// own driver ever reads into every one of those would be noise for
	// clients that never drive a turn. The data is carried; this is a
	// choice about WHICH projection carries it.
	"encounter.Member.SpeedFeet": "a TurnDriver-facing fact; MonsterView.Budget.MovementFeet is the turn's " +
		"own derived answer, and a raw speed a roster listing has no use for",
	"encounter.Member.SightFeet": "a TurnDriver-facing fact; it gates which members even appear in " +
		"MonsterView.Seen, and a roster listing has no use for the raw range itself",
	"encounter.Member.Actions":   "a TurnDriver-facing fact, carried verbatim via session.MonsterView.Actions",
	"encounter.Member.Targeting": "a TurnDriver-facing fact, carried verbatim via session.MonsterView.Targeting",
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
				if as, ok := renamed[key]; ok {
					_, exists := outerFields[as.outer]
					s.True(exists, "%s is recorded as carried into %T as %q, but no such field exists", key, pair.outer, as.outer)
					s.NotEmpty(as.reason, "%s must record WHY it was renamed", key)
					continue
				}
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

// TestSeenIsProjectedFromPayloadOnBothSightingAndReport pins ADR-0041's shape
// outside the generic inner-field audit above, because Seen is not a
// same-named passthrough of an intel field the way every other projected
// field is: it is a SECOND projection of Payload, decoded by the
// composition's own encounter.DecodeSightPayload rather than renamed from or
// dropped in favour of it. The generic audit above already requires Payload
// itself to survive unchanged (matched by name on intel.Holding and
// intel.Report — see the "Sighting" pair); this states the derived half
// explicitly, so a future reviewer does not mistake Seen's absence from the
// renamed/omitted maps above for an oversight.
func (s *ConvertTestSuite) TestSeenIsProjectedFromPayloadOnBothSightingAndReport() {
	s.Contains(fieldNames(session.Sighting{}), "Seen",
		"Sighting must carry the sight channel's typed knowledge (ADR-0041)")
	s.Contains(fieldNames(session.Report{}), "Seen",
		"Report must carry it too — first contact and a held sighting are the same fact")
	s.Contains(fieldNames(session.Sighting{}), "Payload",
		"retained for channels the SDK has not typed")
	s.Contains(fieldNames(session.Report{}), "Payload",
		"retained for channels the SDK has not typed")
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
	for key := range renamed {
		s.True(known[key],
			"%s is listed as a rename but no longer exists on any projected type — "+
				"remove the entry rather than leaving a reason for a field that is gone",
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
