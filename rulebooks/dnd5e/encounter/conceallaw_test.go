// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// conceallaw_test.go pins the projection and refusal laws of wave 1b
// (rpg-toolkit#1371) — the never-authored yardstick, the masquerade wall
// and its height, presence piercing, the still-hidden-neighbour boundary,
// the probe law, the move law, and the unlock's applied route. The fixture
// is conceal_test.go's.
type ConcealLawSuite struct {
	suite.Suite

	witness *scriptedWitness
}

func TestConcealLawSuite(t *testing.T) {
	suite.Run(t, new(ConcealLawSuite))
}

func (s *ConcealLawSuite) SetupTest() {
	s.witness = &scriptedWitness{perceivers: map[encounter.DoorID][]encounter.MemberID{}}
}

func (s *ConcealLawSuite) open(resolver encounter.CheckResolver, inVault bool) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: resolver, Witness: s.witness,
		Field:   concealField(),
		Members: partyMembers(inVault),
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// openTwin builds the honestly-authored twin — same cast, no secrets —
// whose Atlas is what a non-knower's AtlasFor must equal.
func (s *ConcealLawSuite) openTwin() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field:   twinField(),
		Members: partyMembers(false),
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// TestANonKnowersAtlasIsAnHonestlyAuthoredTwin is the whole absence law in
// one pin: a non-knower's member-scoped atlas equals — struct for struct,
// in the same sort order — the Atlas of a dungeon HONESTLY AUTHORED as what
// they believe: no vault (cells, region, props, every touching boundary,
// border walls included, all gone), no doors, and a real wall at the
// veil-door's crossing whose height is the surrounding run's own authored
// 2.0, because a mask at any other height would be a visible notch exactly
// where the secret is. The expectation is COMPUTED by authoring the twin,
// not echoed from the projection.
func (s *ConcealLawSuite) TestANonKnowersAtlasIsAnHonestlyAuthoredTwin() {
	scoped, err := s.open(findsNothing{}, false).AtlasFor(seeker)
	s.Require().NoError(err)

	twin, err := s.openTwin().Atlas()
	s.Require().NoError(err)

	s.Equal(twin, scoped, "byte-identical to never-authored")

	mask, present := hasBoundary(scoped, spatial.Position{X: 4, Y: concealRow}, spatial.Position{X: 5, Y: concealRow})
	s.Require().True(present, "the veil-door's crossing reads as wall, not hole")
	s.Equal(seamHeight, mask.Height, "at the neighbouring authored run's height")
	s.True(mask.BlocksMovement)
	s.True(mask.BlocksLineOfSight)
}

// TestPresencePiercesFromFrameOne: a member standing inside a concealed
// region perceives it from the first frame — their atlas carries it and
// their story opens with the region reveal — while everyone outside stays
// blind, and the room's own concealed door remains a separate, unfound
// knowledge moment: absent from the occupant's door list, masked as wall in
// their geometry (both its sides are spaces the occupant can see).
func (s *ConcealLawSuite) TestPresencePiercesFromFrameOne() {
	enc := s.open(findsNothing{}, true)

	reveals := s.beatsForLaw(enc, lurker, "region_revealed")
	s.Require().Len(reveals, 1, "the occupant's story opens with the reveal")

	atlas, err := enc.AtlasFor(lurker)
	s.Require().NoError(err)
	holdsVault := false
	for _, r := range atlas.Regions {
		if r.ID == vaultRegion {
			holdsVault = true
		}
	}
	s.True(holdsVault, "the occupant's atlas carries the vault")

	doors, err := enc.DoorsFor(lurker)
	s.Require().NoError(err)
	s.False(doorsListed(doors, vaultDoor), "occupying the room is not finding its door")
	_, masked := hasBoundary(atlas, spatial.Position{X: 8, Y: concealRow}, spatial.Position{X: 9, Y: concealRow})
	s.True(masked, "which is masked as wall between two spaces the occupant sees")

	// THE BEAT IS THE ATLAS'S OWN PATCH (PR #1373 review, Minor 1): its
	// boundary list equals, entry for entry, what the occupant's AtlasFor
	// answers at the vault's cells — the mask at their own still-unfound
	// door seam included. Expected computed from the atlas, decoded side
	// normalized through the same JSON both travel as.
	vaultCells := map[string]bool{}
	for col := 9; col < 12; col++ {
		for row := 0; row < 8; row++ {
			c := cellAt(col, row)
			vaultCells[posKey(c.X, c.Y)] = true
		}
	}
	expected := make([]map[string]any, 0)
	for _, b := range atlas.Boundaries {
		if !vaultCells[posKey(b.From.X, b.From.Y)] && !vaultCells[posKey(b.To.X, b.To.Y)] {
			continue
		}
		expected = append(expected, map[string]any{
			"from":                 map[string]any{"x": b.From.X, "y": b.From.Y},
			"to":                   map[string]any{"x": b.To.X, "y": b.To.Y},
			"blocks_movement":      b.BlocksMovement,
			"blocks_line_of_sight": b.BlocksLineOfSight,
			"height":               b.Height,
		})
	}
	decoded, ok := reveals[0]["boundaries"].([]any)
	s.Require().True(ok)
	got := make([]map[string]any, 0, len(decoded))
	for _, b := range decoded {
		got = append(got, b.(map[string]any))
	}
	s.Require().ElementsMatch(expected, got,
		"the reveal beat and the member-scoped atlas answer with the same boundaries — mask included")
	maskOnBeat := false
	edge := doorEdgesAcross(8, concealRow)[0]
	for _, b := range got {
		from := b["from"].(map[string]any)
		to := b["to"].(map[string]any)
		if (from["x"] == edge.From.X && from["y"] == edge.From.Y && to["x"] == edge.To.X && to["y"] == edge.To.Y) ||
			(from["x"] == edge.To.X && from["y"] == edge.To.Y && to["x"] == edge.From.X && to["y"] == edge.From.Y) {
			maskOnBeat = true
		}
	}
	s.True(maskOnBeat, "the patch carries the mask at the occupant's own unfound door seam")

	outsiderAtlas, err := enc.AtlasFor(seeker)
	s.Require().NoError(err)
	for _, r := range outsiderAtlas.Regions {
		s.Require().NotEqual(encounter.RegionID(vaultRegion), r.ID, "everyone outside begins blind")
	}
}

// TestABoundaryWithAStillHiddenNeighbourStaysWithheld — the Wave 1b
// interpretation pin: reveal is the member-scoped answer, not a literal
// every-touching-boundary sweep. Two concealed rooms share a wall; knowing
// one must not disclose the other, so the shared wall stays withheld from
// the atlas AND from the region reveal's own beat.
func (s *ConcealLawSuite) TestABoundaryWithAStillHiddenNeighbourStaysWithheld() {
	inner := core.EntityID("inner")
	field := encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion("antechamber", 0, 0, 3, 4),
			func() encounter.RegionInput {
				r := rectRegion("crypt-a", 3, 0, 3, 4)
				r.Concealed = true
				return r
			}(),
			func() encounter.RegionInput {
				r := rectRegion("crypt-b", 6, 0, 3, 4)
				r.Concealed = true
				return r
			}(),
		},
		// The antechamber|crypt-a seam is walled except the door row; the
		// crypt-a|crypt-b seam is FULLY walled — a crossing wholly inside
		// hidden space is nobody's business but the author's.
		Walls: append(seamWallExcept(2, 4, 1), seamWallRows(5, 0, 4)...),
		Doors: []encounter.DoorInput{{
			ID: "crypt-door", Edges: doorEdgesAcross(2, 1),
			State: encounter.DoorIsClosed(), Concealed: vaultFind(),
		}},
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: s.witness,
		Field: field,
		Members: []encounter.MemberInput{
			{ID: seeker, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: inner, Kind: encounter.KindPlayer, Position: spatial.Position{X: 4, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	// Presence pierced crypt-a for its occupant; crypt-b stays hidden.
	atlas, err := enc.AtlasFor(inner)
	s.Require().NoError(err)
	holdsA, holdsB := false, false
	for _, r := range atlas.Regions {
		holdsA = holdsA || r.ID == "crypt-a"
		holdsB = holdsB || r.ID == "crypt-b"
	}
	s.True(holdsA, "the occupant knows their own room")
	s.False(holdsB, "and nothing about the neighbour")

	_, shared := hasBoundary(atlas, spatial.Position{X: 5, Y: 1}, spatial.Position{X: 6, Y: 1})
	s.False(shared, "the shared wall stays withheld — it touches a still-hidden room")
	_, own := hasBoundary(atlas, spatial.Position{X: 2, Y: 0}, spatial.Position{X: 3, Y: 0})
	s.True(own, "while the revealed room's other border walls arrive")

	// And the region reveal's own beat kept the same silence.
	reveals := s.beatsForLaw(enc, inner, "region_revealed")
	s.Require().Len(reveals, 1)
	hiddenCells := map[string]bool{}
	for col := 6; col < 9; col++ {
		for row := 0; row < 4; row++ {
			c := cellAt(col, row)
			hiddenCells[posKey(c.X, c.Y)] = true
		}
	}
	boundaries, ok := reveals[0]["boundaries"].([]any)
	s.Require().True(ok)
	s.Require().NotEmpty(boundaries, "the reveal carries the room's boundaries")
	for _, b := range boundaries {
		bd := b.(map[string]any)
		for _, end := range []string{"from", "to"} {
			p := bd[end].(map[string]any)
			s.Require().False(hiddenCells[posKey(p["x"].(float64), p["y"].(float64))],
				"no boundary on the beat touches the still-hidden neighbour")
		}
	}
}

// TestAProbedConcealedDoorAnswersNotFound — the probe law: everywhere a
// door id is spoken, a concealed unfound door answers byte-identically to a
// door that does not exist. Never a locked/closed/DC-naming refusal, which
// would confirm existence to a guessed id. A knower gets the door's real
// answers back.
func (s *ConcealLawSuite) TestAProbedConcealedDoorAnswersNotFound() {
	enc := s.open(findsEverything{}, false)

	swap := func(err error) string {
		return strings.ReplaceAll(err.Error(), "no-such-door", veilDoor)
	}

	s.Run("open", func() {
		_, probed := enc.OpenDoor(&encounter.OpenDoorInput{Door: veilDoor, Actor: buddy})
		s.Require().ErrorIs(probed, encounter.ErrNoDoor)
		_, ghost := enc.OpenDoor(&encounter.OpenDoorInput{Door: "no-such-door", Actor: buddy})
		s.Equal(swap(ghost), probed.Error())
	})
	s.Run("unlock", func() {
		_, probed := enc.Unlock(&encounter.UnlockInput{Door: veilDoor, Actor: buddy, Beaten: true})
		s.Require().ErrorIs(probed, encounter.ErrNoDoor)
		_, ghost := enc.Unlock(&encounter.UnlockInput{Door: "no-such-door", Actor: buddy, Beaten: true})
		s.Equal(swap(ghost), probed.Error())
	})
	s.Run("close", func() {
		_, probed := enc.CloseDoor(&encounter.CloseDoorInput{Door: veilDoor, Actor: buddy})
		s.Require().ErrorIs(probed, encounter.ErrNoDoor)
		_, ghost := enc.CloseDoor(&encounter.CloseDoorInput{Door: "no-such-door", Actor: buddy})
		s.Equal(swap(ghost), probed.Error())
	})

	s.Run("a knower gets the real door", func() {
		_, err := enc.Search(&encounter.SearchInput{Member: seeker, Region: hallRegion})
		s.Require().NoError(err)
		_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: veilDoor, Actor: seeker})
		s.Require().NoError(err, "found, the door answers as itself")
	})
}

// TestAMoveRefusedAtAConcealedCrossingRefusesLikeAWall — the move law: the
// refusal is byte-identical to the honestly-authored twin's wall at the
// same crossing, computed by making that exact step in the twin. Found,
// the same step refuses by the door's own name instead.
func (s *ConcealLawSuite) TestAMoveRefusedAtAConcealedCrossingRefusesLikeAWall() {
	enc := s.open(findsEverything{}, false)
	twin := s.openTwin()

	// Walk to the door's near cell in both worlds, then push on the secret.
	for _, e := range []*encounter.Encounter{enc, twin} {
		_, err := e.Step(&encounter.StepInput{Member: seeker, To: cellAt(4, concealRow)})
		s.Require().NoError(err)
	}

	_, refused := enc.Step(&encounter.StepInput{Member: seeker, To: cellAt(5, concealRow)})
	s.Require().Error(refused)
	s.Require().ErrorIs(refused, encounter.ErrBadPlacement, "the ordinary no-crossing refusal")
	s.Require().NotErrorIs(refused, encounter.ErrDoorShut, "never the door's own sentence")
	s.NotContains(refused.Error(), veilDoor, "and never its name")

	_, walled := twin.Step(&encounter.StepInput{Member: seeker, To: cellAt(5, concealRow)})
	s.Require().Error(walled)
	s.Equal(walled.Error(), refused.Error(), "byte-identical to the twin's authored wall")

	// Found, the door answers as itself.
	_, err := enc.Search(&encounter.SearchInput{Member: seeker, Region: hallRegion})
	s.Require().NoError(err)
	_, named := enc.Step(&encounter.StepInput{Member: seeker, To: cellAt(5, concealRow)})
	s.Require().ErrorIs(named, encounter.ErrDoorShut)
	s.Contains(named.Error(), veilDoor, "a knower is told what stopped them")
}

// TestUnlockCarriesTheAppliedRoute — the applied-route half of the
// multi-approach ruling arriving at Unlock: the input names the one route
// the resolver applied, it must be an authored one, and the beat carries
// its DC so the story can say "17 vs DC 12" for the route actually faced.
func (s *ConcealLawSuite) TestUnlockCarriesTheAppliedRoute() {
	routes := []encounter.CheckApproach{
		{Ability: "str", DC: 15},
		{Ability: "dex", Tool: "dnd5e:item:thieves-tools", DC: 12},
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field: doorField(3, encounter.DoorIsLocked(encounter.Lock{Approaches: routes}), "two-way-lock", 1),
		Members: []encounter.MemberInput{
			{ID: nessa, Kind: encounter.KindPlayer, Position: nessaCell},
			{ID: orin, Kind: encounter.KindPlayer, Position: orinCell},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	_, err = enc.Unlock(&encounter.UnlockInput{
		Door: "two-way-lock", Actor: nessa, Beaten: true, Total: 14,
		Applied: encounter.CheckApproach{Ability: "wis", DC: 10},
	})
	s.Require().ErrorIs(err, encounter.ErrBadDoor, "a route the lock does not list is refused")

	out, err := enc.Unlock(&encounter.UnlockInput{
		Door: "two-way-lock", Actor: nessa, Beaten: true, Total: 14, Applied: routes[1],
	})
	s.Require().NoError(err)
	s.Equal(routes[1], out.Applied, "the answer names the route taken")
	s.Equal(routes, out.Approaches, "beside the whole authored list")

	beats := s.beatsForLaw(enc, nessa, "door")
	s.Require().NotEmpty(beats)
	last := beats[len(beats)-1]
	s.Equal(float64(routes[1].DC), last["dc"], "the beat carries the APPLIED route's DC")
	applied, ok := last["applied"].(map[string]any)
	s.Require().True(ok)
	s.Equal("dex", applied["ability"])
}

// beatsForLaw is conceal_test.go's beatsFor, local to this suite.
func (s *ConcealLawSuite) beatsForLaw(enc *encounter.Encounter, member core.EntityID, kind string) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	beats := make([]map[string]any, 0)
	for _, entry := range story {
		beat := map[string]any{}
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == kind {
			beats = append(beats, beat)
		}
	}
	return beats
}

// posKey names an absolute cell for set membership in assertions.
func posKey(x, y float64) string {
	return fmt.Sprintf("%g,%g", x, y)
}
