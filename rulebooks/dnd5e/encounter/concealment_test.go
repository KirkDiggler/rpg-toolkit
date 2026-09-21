// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// concealment_test.go is THE PRIMITIVE'S OWN DOOR (rpg-project#490,
// concealment.go): every sentence a malformed concealment earns, the
// reveal cause the noun added, the passive tell it carries unread, and the
// two tombstones that refuse a blob from the dialect of two flags.
//
// The scenes that are about what a secret DOES — search, the masquerade, the
// beat, the probe and move laws — are conceal_test.go's and
// conceallaw_test.go's, re-seated on this primitive. What is here is what a
// concealment IS.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type ConcealmentSuite struct {
	suite.Suite
}

func TestConcealmentSuite(t *testing.T) {
	suite.Run(t, new(ConcealmentSuite))
}

// vaultCheck is the find check every scene here authors unless it is about
// the check itself.
func vaultCheck() []encounter.CheckApproach {
	return []encounter.CheckApproach{{Ability: "perception", DC: 15}}
}

// twoRoomField is a visible hall and a second room behind a shut door, with a
// holdable idol standing in the hall — enough floor, doors and props for a
// concealment to name something of each kind.
func (s *ConcealmentSuite) twoRoomField() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion("hall", 0, 0, 3, 4),
			rectRegion("vault", 3, 0, 3, 4),
		},
		Walls: seamWallExcept(2, 4, 1),
		Doors: []encounter.DoorInput{{
			ID: "panel", Edges: doorEdgesAcross(2, 1), State: encounter.DoorIsClosed(),
		}},
		Props: []encounter.PropInput{holdableProp("idol", "dnd5e:props:idol", spatial.Position{X: 1, Y: 3})},
	}
}

func (s *ConcealmentSuite) setup(field encounter.FieldInput, members ...encounter.MemberInput) error {
	if len(members) == 0 {
		members = []encounter.MemberInput{{
			ID: core.EntityID("walker"), Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0},
		}}
	}
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Field:   field,
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})

	return err
}

// TestAConcealmentIsRefusedForWhatItCannotBe is every sentence the primitive
// adds, each earning ErrBadConcealment and each naming the defect.
//
// THE THREE OVERLAPS NAME BOTH DECLARATIONS (R4). An author looking at one of
// two colliding secrets cannot fix it without being told which other one it
// collides with, and a refusal that named only the second would send them
// hunting.
func (s *ConcealmentSuite) TestAConcealmentIsRefusedForWhatItCannotBe() {
	cells := rectCells(3, 0, 3, 4)
	for _, tc := range []struct {
		name  string
		hides []encounter.ConcealmentInput
		says  string
	}{
		{"no id", []encounter.ConcealmentInput{{Checks: vaultCheck(), Cells: cells}}, "has no id"},
		{"two secrets sharing an id", []encounter.ConcealmentInput{
			{ID: "vault", Checks: vaultCheck(), Cells: cells[:2]},
			{ID: "vault", Checks: vaultCheck(), Cells: cells[2:]},
		}, "duplicate concealment"},
		{"no checks", []encounter.ConcealmentInput{{ID: "vault", Cells: cells}},
			"lists no way to find it"},
		{"a check nothing has to beat", []encounter.ConcealmentInput{{
			ID: "vault", Checks: []encounter.CheckApproach{{Ability: "perception"}}, Cells: cells,
		}}, "nothing has to beat"},
		{"a notice with no way through it", []encounter.ConcealmentInput{{
			ID: "vault", Checks: vaultCheck(), Notice: []encounter.CheckApproach{}, Cells: cells,
		}}, "declares a notice with no way through it"},
		{"hides nothing", []encounter.ConcealmentInput{{ID: "vault", Checks: vaultCheck()}},
			"hides nothing"},
		{"a cell this field does not have", []encounter.ConcealmentInput{{
			ID: "vault", Checks: vaultCheck(), Cells: []spatial.Position{{X: 40, Y: 40}},
		}}, "is not floor this field has"},
		{"one cell in two secrets", []encounter.ConcealmentInput{
			{ID: "alcove", Checks: vaultCheck(), Cells: cells[:1]},
			{ID: "vault", Checks: vaultCheck(), Cells: cells[:1]},
		}, `concealments "alcove" and "vault" both hide the cell`},
		{"one door in two secrets", []encounter.ConcealmentInput{
			{ID: "alcove", Checks: vaultCheck(), Doors: []encounter.DoorID{"panel"}},
			{ID: "vault", Checks: vaultCheck(), Doors: []encounter.DoorID{"panel"}},
		}, `concealments "alcove" and "vault" both hide door "panel"`},
		{"one prop in two secrets", []encounter.ConcealmentInput{
			{ID: "alcove", Checks: vaultCheck(), Props: []encounter.PropID{"idol"}},
			{ID: "vault", Checks: vaultCheck(), Props: []encounter.PropID{"idol"}},
		}, `concealments "alcove" and "vault" both hide prop "idol"`},
		{"a prop this field does not declare", []encounter.ConcealmentInput{{
			ID: "vault", Checks: vaultCheck(), Props: []encounter.PropID{"bookcase"},
		}}, `hides prop "bookcase", which this field does not declare`},
	} {
		s.Run(tc.name, func() {
			field := s.twoRoomField()
			field.Concealments = tc.hides
			err := s.setup(field)
			s.Require().ErrorIs(err, encounter.ErrBadConcealment)
			s.Contains(err.Error(), tc.says)
		})
	}

	// A DOOR IT NAMES THAT THE FIELD DOES NOT DECLARE is the OTHER sentinel,
	// for [validateIntelTargets]' reason one noun over: the secret is fine
	// and the thing it points at is missing.
	s.Run("a door this field does not declare", func() {
		field := s.twoRoomField()
		field.Concealments = []encounter.ConcealmentInput{{
			ID: "vault", Checks: vaultCheck(), Doors: []encounter.DoorID{"cellar-hatch"},
		}}
		err := s.setup(field)
		s.Require().ErrorIs(err, encounter.ErrNoDoor)
		s.Contains(err.Error(), "cellar-hatch")
	})
}

// TestTheNoticeIsCarriedAndUnread is [ConcealmentInput.Notice]'s contract
// (E6, slice 2): validated at the door, persisted, read back — and read by
// nothing in this build. NIL IS NOT EMPTY, which is what the refusal above
// pins from the other side.
func (s *ConcealmentSuite) TestTheNoticeIsCarriedAndUnread() {
	notice := []encounter.CheckApproach{{Ability: "investigation", DC: 12}}
	field := s.twoRoomField()
	field.Concealments = []encounter.ConcealmentInput{{
		ID: "vault", Checks: vaultCheck(), Notice: notice, Cells: rectCells(3, 0, 3, 4),
	}}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Field: field,
		Members: []encounter.MemberInput{{
			ID: core.EntityID("walker"), Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0},
		}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	data := enc.ToData()
	s.Require().Len(data.Field.Concealments, 1)
	s.Equal([]encounter.CheckApproachData{{Ability: "investigation", DC: 12}}, data.Field.Concealments[0].Notice)

	back, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:      data,
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
	})
	s.Require().NoError(err)
	s.Equal(data.Field.Concealments, back.ToData().Field.Concealments, "round-trips unchanged")

	// A secret whose author declared no passive tell writes NO KEY AT ALL —
	// the difference this build carries so the slice that reads it can tell
	// "said nothing" from "said nothing beats it".
	plain := s.twoRoomField()
	plain.Concealments = []encounter.ConcealmentInput{{
		ID: "vault", Checks: vaultCheck(), Cells: rectCells(3, 0, 3, 4),
	}}
	s.Require().NoError(s.setup(plain))
}

// TestSteppingOntoHiddenFloorRevealsIt is THE REVEAL CAUSE THE NOUN ADDED
// (rpg-project#490, E3). A concealment need not have a door at all — cells
// with no door are a secret alcove (R8) — so "crossed a hidden door" could
// not be the whole of the crossing cause any more. Walking ONTO the floor is
// perceiving the secret as directly as perception gets.
func (s *ConcealmentSuite) TestSteppingOntoHiddenFloorRevealsIt() {
	walker := core.EntityID("walker")
	field := encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion("hall", 0, 0, 3, 4),
			rectRegion("alcove", 3, 0, 1, 4),
		},
		// NO DOOR AND NO WALL between the two: the alcove is a secret
		// alcove, and the boundary a non-knower sees is the synthesized one
		// the masquerade stands on every bare visible/hidden adjacency.
		Concealments: []encounter.ConcealmentInput{{
			ID: "alcove", Checks: vaultCheck(), Cells: rectCells(3, 0, 1, 4),
		}},
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Field: field,
		Members: []encounter.MemberInput{
			{ID: walker, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	blind, err := enc.AtlasFor(walker)
	s.Require().NoError(err)
	s.NotContains(blind.Cells, cellAt(3, 1), "the alcove's floor is not theirs yet")
	_, masked := hasBoundary(blind, spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1})
	s.True(masked, "and what they see there is a wall — the synthesized one, on a bare adjacency")

	// AND THE WALL IS THE PICTURE, NOT THE GEOMETRY. The author drew no wall,
	// so the canvas has none and the step is legal — which is R8 exactly:
	// "reveal the concealment and the boundary opens where the author left
	// the gap". Walking into the gap IS the revealing.
	_, err = enc.Step(&encounter.StepInput{Member: walker, To: cellAt(3, 1)})
	s.Require().NoError(err)

	reveals := s.revealsFor(enc, walker)
	s.Require().Len(reveals, 1, "stepping onto it taught the walker")
	s.Equal("alcove", reveals[0]["concealment"])
	s.Len(reveals[0]["cells"], 4, "the whole column rides the beat, not just the cell they stood on")
	s.Empty(reveals[0]["doors"], "a secret alcove has no door to name")

	after, err := enc.AtlasFor(walker)
	s.Require().NoError(err)
	s.Contains(after.Cells, cellAt(3, 1), "and the floor is theirs")
	_, stillMasked := hasBoundary(after, spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1})
	s.False(stillMasked, "with the wall that was never there gone from the picture too")
}

// TestAnOccupantOfAnAlcoveKnowsItFromFrameOne is the presence cause asked of
// a DOORLESS secret: the occupancy sweep reads the concealment's cells, not a
// region's flag, so a secret with no door and no region of its own still
// pierces for whoever stands on it.
func (s *ConcealmentSuite) TestAnOccupantOfAnAlcoveKnowsItFromFrameOne() {
	lurking := core.EntityID("lurking")
	field := encounter.FieldInput{
		Canvas:  pointyCanvas(),
		Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 4, 4)},
		// A SLICE OF ONE ROOM, which the retired region flag could not say
		// at all: hiding meant hiding a whole region.
		Concealments: []encounter.ConcealmentInput{{
			ID: "alcove", Checks: vaultCheck(), Cells: rectCells(3, 0, 1, 4),
		}},
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Field: field,
		Members: []encounter.MemberInput{
			{ID: lurking, Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 1}},
			{ID: core.EntityID("outside"), Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	s.Require().Len(s.revealsFor(enc, lurking), 1, "the occupant's story opens with it")

	mine, err := enc.AtlasFor(lurking)
	s.Require().NoError(err)
	s.Contains(mine.Cells, cellAt(3, 1), "they have the floor under their feet")

	// THE REGION IS TRIMMED FOR EVERYONE ELSE, not dropped: the hall is one
	// room and only a slice of it is a secret, so the outsider's entry lists
	// the cells they can see and no more.
	theirs, err := enc.AtlasFor(core.EntityID("outside"))
	s.Require().NoError(err)
	s.Require().Len(theirs.Regions, 1, "the hall is still their room")
	s.Len(theirs.Regions[0].Cells, 12, "with the alcove's column trimmed out of it")
	s.Len(mine.Regions[0].Cells, 16, "and whole for the one who knows")
}

// TestTheRevealCarriesTheRoomsTheSecretWasCutOutOf is the `regions` key on
// the beat (rpg-project#490 E4, rpg-api-protos#352).
//
// A concealment hides CELLS, and those cells sit inside an authored region —
// so a non-knower's region entry is the authored one with the hidden cells
// taken out, and withheld entirely when none survive. A reveal does not only
// add floor: it restores the room that floor belongs to. A recipient handed
// cells with no region to file them under would be holding floor with no
// name, no lighting and no archetype.
//
// A REPLACEMENT, NOT A DIFFERENCE — [Atlas.Sealed]'s law on this beat. The
// entry comes back LARGER, so a difference could only ever say "here is a
// room you already have"; the whole entry as it now stands is the news.
func (s *ConcealmentSuite) TestTheRevealCarriesTheRoomsTheSecretWasCutOutOf() {
	walker := core.EntityID("walker")

	s.Run("a room that was wholly a secret comes back whole", func() {
		field := s.twoRoomField()
		field.Concealments = []encounter.ConcealmentInput{{
			ID: "vault", Checks: vaultCheck(), Cells: rectCells(3, 0, 3, 4),
			Doors: []encounter.DoorID{"panel"},
		}}
		enc := s.play(field, findsEverything{},
			encounter.MemberInput{ID: walker, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}})

		blind, err := enc.AtlasFor(walker)
		s.Require().NoError(err)
		s.Require().Len(blind.Regions, 1, "the vault has no entry at all for a non-knower")

		_, err = enc.Search(&encounter.SearchInput{Member: walker, Region: "hall"})
		s.Require().NoError(err)

		reveals := s.revealsFor(enc, walker)
		s.Require().Len(reveals, 1)
		rooms, ok := reveals[0]["regions"].([]any)
		s.Require().True(ok)
		s.Require().Len(rooms, 1, "one room was cut out of, so one room comes back")
		room, ok := rooms[0].(map[string]any)
		s.Require().True(ok)
		s.Equal("vault", room["id"])
		s.Equal("vault", room["name"])
		s.Equal(testArchetype, room["archetype"], "with the facts a client dresses it with")
		s.NotNil(room["lighting"])
		s.Len(room["cells"], 12, "and every cell of it")
	})

	s.Run("a room only PART of which was a secret comes back whole too", func() {
		field := encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 4, 4)},
			Concealments: []encounter.ConcealmentInput{{
				ID: "alcove", Checks: vaultCheck(), Cells: rectCells(3, 0, 1, 4),
			}},
		}
		enc := s.play(field, findsEverything{},
			encounter.MemberInput{ID: walker, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}})

		blind, err := enc.AtlasFor(walker)
		s.Require().NoError(err)
		s.Require().Len(blind.Regions, 1, "the hall is still their room")
		s.Require().Len(blind.Regions[0].Cells, 12, "with the alcove's column trimmed out of it")

		_, err = enc.Search(&encounter.SearchInput{Member: walker, Region: "hall"})
		s.Require().NoError(err)

		reveals := s.revealsFor(enc, walker)
		s.Require().Len(reveals, 1)
		rooms, ok := reveals[0]["regions"].([]any)
		s.Require().True(ok)
		s.Require().Len(rooms, 1)
		room, ok := rooms[0].(map[string]any)
		s.Require().True(ok)
		s.Equal("hall", room["id"])
		s.Len(room["cells"], 16,
			"THE WHOLE ENTRY, not the four cells that arrived — a replacement, exactly as `sealed` is")
	})

	s.Run("a secret that hides no floor names no room", func() {
		field := s.twoRoomField()
		field.Concealments = []encounter.ConcealmentInput{{
			ID: "panel-secret", Checks: vaultCheck(), Props: []encounter.PropID{"idol"},
		}}
		enc := s.play(field, findsEverything{},
			encounter.MemberInput{ID: walker, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}})

		_, err := enc.Search(&encounter.SearchInput{Member: walker, Region: "hall"})
		s.Require().NoError(err)

		reveals := s.revealsFor(enc, walker)
		s.Require().Len(reveals, 1)
		s.Empty(reveals[0]["regions"], "a hidden bookcase cuts no room out of anything")
		s.NotEmpty(reveals[0]["props"], "and the thing itself is the whole of the news")
	})
}

// play builds a live encounter the scenes above act on, with the resolver the
// scene is about.
func (s *ConcealmentSuite) play(
	field encounter.FieldInput, resolver encounter.CheckResolver, members ...encounter.MemberInput,
) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: resolver, Witness: nobodyPerceives{},
		Field:   field,
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// TestABlobWithARetiredRegionFlagIsRefusedByName is the region tombstone
// ([encounter.RegionData.Concealed], rpg-project#490) — [DoorData.Concealed]'s
// twin, and the standing fail-loudly precedent (rpg-toolkit#1053/#1068): a
// changed shape gets a detectable name so the old dialect lands nowhere.
func (s *ConcealmentSuite) TestABlobWithARetiredRegionFlagIsRefusedByName() {
	field := s.twoRoomField()
	field.Concealments = []encounter.ConcealmentInput{{
		ID: "vault", Checks: vaultCheck(), Cells: rectCells(3, 0, 3, 4),
	}}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Field: field,
		Members: []encounter.MemberInput{{
			ID: core.EntityID("walker"), Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0},
		}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	data := enc.ToData()
	s.Require().NotEmpty(data.Field.Regions)
	data.Field.Regions[1].Concealed = []byte("true")

	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:      data,
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
	})
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Contains(err.Error(), "the flag this build does not speak")
}

// revealsFor is one member's concealment reveals, decoded.
func (s *ConcealmentSuite) revealsFor(enc *encounter.Encounter, member core.EntityID) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	out := make([]map[string]any, 0)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == encounter.BeatConcealmentRevealed {
			out = append(out, beat)
		}
	}

	return out
}
