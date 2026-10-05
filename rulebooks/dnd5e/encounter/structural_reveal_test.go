// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// structural_reveal_test.go is P2E's witness (rpg-project#169): the existing
// room_revealed and concealment_revealed beats gain optional structural_walls
// and structural_doors carrying NEW OR CHANGED fixed layout rows by identity.
//
// The contract the beat exists to keep is the same one the segments keep: a
// recipient's cached atlas, plus what the beat says, IS what AtlasFor now
// answers — and NOTHING MORE. An old payload never learns about a secret
// discovered later, and a legacy beat with no structural change is byte-identical.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type StructuralRevealSuite struct {
	suite.Suite
}

func TestStructuralRevealSuite(t *testing.T) {
	suite.Run(t, new(StructuralRevealSuite))
}

// structuralRevealWall builds one authored structural wall whose single
// opening binds an existing footprint door.
func structuralRevealWall(wallID, doorPlacedID string, doorID encounter.DoorID) encounter.StructuralWallInput {
	return encounter.StructuralWallInput{
		ID: encounter.PropID(wallID), Ref: "dnd5e:env:test:wall",
		From: spatial.Point{X: 0, Y: 0}, To: spatial.Point{X: 10, Y: 0},
		Height: 3, Thickness: 0.3, Elevation: 0,
		Openings: []encounter.StructuralOpeningInput{{
			ID: wallID + "-gap", Position: 5, Width: 2,
			Door: &encounter.StructuralDoorBindingInput{
				PlacedID: encounter.PropID(doorPlacedID),
				DoorID:   doorID,
				Ref:      "dnd5e:env:test:door",
				From:     spatial.Point{X: 4, Y: 0},
				To:       spatial.Point{X: 6, Y: 0},
			},
		}},
	}
}

// structuralRevealField is three regions in a row: a visible hall, a room
// behind a shut edge door, and a farther region behind a second shut door.
// Each of the two inner regions carries its own structural wall and bound
// footprint door, so a reveal can prove it delivered one region's layout and
// nothing of the next.
func structuralRevealField() encounter.FieldInput {
	doorA := coveredBox(1, centreOf(cellAt(5, 1)))
	doorB := coveredBox(1, centreOf(cellAt(9, 1)))

	return encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion("hall", 0, 0, 3, 5),
			rectRegion("room", 3, 0, 4, 5),
			rectRegion("beyond", 7, 0, 3, 5),
		},
		Walls: append(seamWallExcept(2, 5, 1), seamWallExcept(6, 5, 1)...),
		Doors: []encounter.DoorInput{
			{ID: "entry-door", Edges: doorEdgesAcross(2, 1), State: encounter.DoorIsClosed()},
			{ID: "far-door", Edges: doorEdgesAcross(6, 1), State: encounter.DoorIsClosed()},
			{ID: "vault/gate-a", Placement: &doorA, State: encounter.DoorIsClosed()},
			{ID: "vault/gate-b", Placement: &doorB, State: encounter.DoorIsClosed()},
		},
		Placed: []encounter.PlacedPropInput{
			placed("wall-a", coveredBox(6, centreOf(cellAt(4, 1))), false, false),
			placed("door-a", doorA, false, false),
			placed("wall-b", coveredBox(6, centreOf(cellAt(8, 1))), false, false),
			placed("door-b", doorB, false, false),
		},
		StructuralWalls: []encounter.StructuralWallInput{
			structuralRevealWall("wall-a", "door-a", "vault/gate-a"),
			structuralRevealWall("wall-b", "door-b", "vault/gate-b"),
		},
	}
}

// hiddenDoorRevealField is one room whose structural wall is visible but whose
// bound footprint door is an authored secret: before the find the wall has no
// cut, and the find must UPDATE that same wall and add the door.
func hiddenDoorRevealField() encounter.FieldInput {
	doorPlacement := coveredBox(1, centreOf(cellAt(2, 1)))

	return encounter.FieldInput{
		Canvas:  pointyCanvas(),
		Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 5, 5)},
		Concealments: []encounter.ConcealmentInput{{
			ID:     "secret",
			Checks: vaultCheck(),
			Cells:  []spatial.Position{cellAt(4, 4)},
			Doors:  []encounter.DoorID{"vault/gate"},
			Props:  []encounter.PropID{"door-presence"},
		}},
		Doors: []encounter.DoorInput{{ID: "vault/gate", Placement: &doorPlacement, State: encounter.DoorIsClosed()}},
		Placed: []encounter.PlacedPropInput{
			placed("wall-presence", coveredBox(6, centreOf(cellAt(1, 1))), false, false),
			placed("door-presence", doorPlacement, false, false),
		},
		StructuralWalls: []encounter.StructuralWallInput{
			structuralRevealWall("wall-presence", "door-presence", "vault/gate"),
		},
	}
}

// hiddenWallRevealField is hiddenDoorRevealField with the WALL hidden and the
// door unlisted: the door is already the recipient's before the secret opens,
// so the reveal must carry the wall and NOT duplicate the door.
func hiddenWallRevealField() encounter.FieldInput {
	field := hiddenDoorRevealField()
	field.Concealments[0].Props = []encounter.PropID{"wall-presence"}
	field.Concealments[0].Doors = nil

	return field
}

// legacyRevealField is two ordinary regions with a shut edge door and no
// structural layout at all — the legacy payload case.
func legacyRevealField() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion("hall", 0, 0, 3, 5),
			rectRegion("room", 3, 0, 4, 5),
		},
		Walls: seamWallExcept(2, 5, 1),
		Doors: []encounter.DoorInput{{ID: "entry-door", Edges: doorEdgesAcross(2, 1), State: encounter.DoorIsClosed()}},
	}
}

func (s *StructuralRevealSuite) open(field encounter.FieldInput, resolver encounter.CheckResolver) *encounter.Encounter {
	s.T().Helper()
	// The finder sees far; the peer does not. A reveal is scoped to the
	// observer who earned it, and an obstructed peer must not be handed it.
	sight := &sightList{fallback: 30, reach: map[encounter.MemberID]int{"peer": 1}}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: sight, Equipment: noHandsAreObserved{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{},
		Announcer: quietAnnouncer{}, CheckResolver: resolver, Witness: nobodyPerceives{},
		Field: field,
		Members: []encounter.MemberInput{
			{ID: "finder", Kind: encounter.KindPlayer, Position: cellAt(1, 1)},
			// In the hall's far corner: the reveal is the opener's, and an
			// obstructed peer must not receive it.
			{ID: "peer", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 4}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

func (s *StructuralRevealSuite) reload(enc *encounter.Encounter) *encounter.Encounter {
	s.T().Helper()
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
	})
	s.Require().NoError(err)

	return loaded
}

// structuralBeats reads one member's decoded beats of a kind.
func (s *StructuralRevealSuite) structuralBeats(enc *encounter.Encounter, member encounter.MemberID, kind string) []map[string]any {
	s.T().Helper()
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	out := make([]map[string]any, 0)
	for _, entry := range story {
		beat := map[string]any{}
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == kind {
			out = append(out, beat)
		}
	}

	return out
}

// structuralBeatPayloads reads one member's raw payload bytes of a kind.
func (s *StructuralRevealSuite) structuralBeatPayloads(enc *encounter.Encounter, member encounter.MemberID, kind string) [][]byte {
	s.T().Helper()
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	out := make([][]byte, 0)
	for _, entry := range story {
		beat := map[string]any{}
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == kind {
			out = append(out, entry.Payload)
		}
	}

	return out
}

// wallRowIDs is the ids of a beat's structural_walls, authored order.
func wallRowIDs(body map[string]any) []string {
	raw, ok := body["structural_walls"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, row := range raw {
		out = append(out, row.(map[string]any)["id"].(string))
	}

	return out
}

// doorRowIDs is the ids of a beat's structural_doors, authored order.
func doorRowIDs(body map[string]any) []string {
	raw, ok := body["structural_doors"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, row := range raw {
		out = append(out, row.(map[string]any)["id"].(string))
	}

	return out
}

// --- The fixed layout a reveal delivers ---

func (s *StructuralRevealSuite) TestOpeningIntoAnUnknownRegionCarriesOnlyThatRegionsLayout() {
	enc := s.open(structuralRevealField(), findsNothing{})
	before, err := enc.AtlasFor("finder")
	s.Require().NoError(err)
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door", Actor: "finder"})
	s.Require().NoError(err)

	beats := s.structuralBeats(enc, "finder", encounter.BeatRoomRevealed)
	s.Require().Len(beats, 1, "the opener learns the room once")
	s.Equal([]string{"wall-a"}, wallRowIDs(beats[0]), "the newly permitted room's wall, by id")
	s.Equal([]string{"vault/gate-a"}, doorRowIDs(beats[0]), "and its independently permitted door")
	s.assertRevealPatchMatchesFresh(enc, before, beats[0])

	s.Empty(s.structuralBeats(enc, "peer", encounter.BeatRoomRevealed),
		"the obstructed peer is not handed the layout")
}

func (s *StructuralRevealSuite) TestAThirdRegionStaysAbsentFromTheOpener() {
	enc := s.open(structuralRevealField(), findsNothing{})
	_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door", Actor: "finder"})
	s.Require().NoError(err)

	beats := s.structuralBeats(enc, "finder", encounter.BeatRoomRevealed)
	s.Require().Len(beats, 1)
	s.NotContains(wallRowIDs(beats[0]), "wall-b", "the farther region's wall is not delivered early")
	s.NotContains(doorRowIDs(beats[0]), "vault/gate-b")

	// The author truth still has both, and the farther region is simply not
	// the opener's yet.
	full, err := enc.Atlas()
	s.Require().NoError(err)
	s.Len(full.StructuralWalls, 2)
	view, err := enc.AtlasFor("finder")
	s.Require().NoError(err)
	for _, w := range view.StructuralWalls {
		s.NotEqual(encounter.PropID("wall-b"), w.ID)
	}
}

func (s *StructuralRevealSuite) TestSearchUpdatesAnExistingWallAndAddsItsNewDoor() {
	enc := s.open(hiddenDoorRevealField(), findsEverything{})

	before, err := enc.AtlasFor("finder")
	s.Require().NoError(err)
	s.Require().Len(before.StructuralWalls, 1)
	s.Empty(before.StructuralWalls[0].Openings, "the door identity is withheld before the find")
	s.Empty(before.StructuralDoors)

	_, err = enc.Search(&encounter.SearchInput{Member: "finder", Region: "hall"})
	s.Require().NoError(err)

	beats := s.structuralBeats(enc, "finder", encounter.BeatConcealmentRevealed)
	s.Require().Len(beats, 1, "the secret opens once")
	body := beats[0]

	// THE SAME WALL ID, UPDATED: its newly permitted cut is delivered whole.
	s.Equal([]string{"wall-presence"}, wallRowIDs(body))
	wallRow := body["structural_walls"].([]any)[0].(map[string]any)
	s.Equal("wall-presence-gap", wallRow["openings"].([]any)[0].(map[string]any)["id"])
	s.Equal([]string{"vault/gate"}, doorRowIDs(body))

	s.assertRevealPatchMatchesFresh(enc, before, body)
}

func (s *StructuralRevealSuite) TestRevealingAHiddenWallDoesNotDuplicateAnAlreadyKnownDoor() {
	enc := s.open(hiddenWallRevealField(), findsEverything{})

	before, err := enc.AtlasFor("finder")
	s.Require().NoError(err)
	s.Empty(before.StructuralWalls, "the wall is withheld with the secret")
	s.Require().Len(before.StructuralDoors, 1, "the unlisted door is already the recipient's")

	_, err = enc.Search(&encounter.SearchInput{Member: "finder", Region: "hall"})
	s.Require().NoError(err)

	beats := s.structuralBeats(enc, "finder", encounter.BeatConcealmentRevealed)
	s.Require().Len(beats, 1)
	body := beats[0]
	s.Equal([]string{"wall-presence"}, wallRowIDs(body), "the parent wall arrives")
	_, hasDoors := body["structural_doors"]
	s.False(hasDoors, "an unchanged, already-known independent door is not duplicated")

	s.assertRevealPatchMatchesFresh(enc, before, body)
}

// assertRevealPatchMatchesFresh applies the beat's structural rows by id onto
// the recipient's prior atlas and requires the result to be the fresh answer —
// the patch/atlas agreement in the language of the fixed layout.
func (s *StructuralRevealSuite) assertRevealPatchMatchesFresh(enc *encounter.Encounter, before encounter.Atlas, body map[string]any) {
	s.T().Helper()
	after, err := enc.AtlasFor("finder")
	s.Require().NoError(err)

	patched := applyStructuralRows(s.T(), before, body)
	s.Equal(after.StructuralWalls, patched.StructuralWalls, "patched walls are the fresh walls")
	s.Equal(after.StructuralDoors, patched.StructuralDoors, "patched doors are the fresh doors")

	// IDEMPOTENT: applying the same rows a second time changes nothing.
	twice := applyStructuralRows(s.T(), patched, body)
	s.Equal(patched.StructuralWalls, twice.StructuralWalls)
	s.Equal(patched.StructuralDoors, twice.StructuralDoors)
}

// applyStructuralRows upserts a beat's structural rows by id onto a copy of the
// recipient's prior atlas, restoring the identity order the atlas uses.
func applyStructuralRows(t *testing.T, before encounter.Atlas, body map[string]any) encounter.Atlas {
	t.Helper()
	out := before
	wallIndex := map[encounter.PropID]int{}
	for i, w := range out.StructuralWalls {
		wallIndex[w.ID] = i
	}
	if raw, ok := body["structural_walls"].([]any); ok {
		for _, row := range raw {
			w := wallFromRow(t, row)
			if i, known := wallIndex[w.ID]; known {
				out.StructuralWalls[i] = w
				continue
			}
			wallIndex[w.ID] = len(out.StructuralWalls)
			out.StructuralWalls = append(out.StructuralWalls, w)
		}
	}
	doorIndex := map[encounter.DoorID]int{}
	for i, d := range out.StructuralDoors {
		doorIndex[d.ID] = i
	}
	if raw, ok := body["structural_doors"].([]any); ok {
		for _, row := range raw {
			d := doorFromRow(t, row)
			if i, known := doorIndex[d.ID]; known {
				out.StructuralDoors[i] = d
				continue
			}
			doorIndex[d.ID] = len(out.StructuralDoors)
			out.StructuralDoors = append(out.StructuralDoors, d)
		}
	}
	sortStructuralForTest(out.StructuralWalls, out.StructuralDoors)

	return out
}

func sortStructuralForTest(walls []encounter.AtlasStructuralWall, doors []encounter.AtlasStructuralDoor) {
	for i := 1; i < len(walls); i++ {
		for j := i; j > 0 && walls[j].ID < walls[j-1].ID; j-- {
			walls[j], walls[j-1] = walls[j-1], walls[j]
		}
	}
	for i := 1; i < len(doors); i++ {
		for j := i; j > 0 && doors[j].ID < doors[j-1].ID; j-- {
			doors[j], doors[j-1] = doors[j-1], doors[j]
		}
	}
}

func wallFromRow(t *testing.T, raw any) encounter.AtlasStructuralWall {
	t.Helper()
	m := raw.(map[string]any)
	w := encounter.AtlasStructuralWall{
		ID:        encounter.PropID(m["id"].(string)),
		Ref:       m["ref"].(string),
		From:      pointFromRow(t, m["from"]),
		To:        pointFromRow(t, m["to"]),
		Height:    m["height"].(float64),
		Thickness: m["thickness"].(float64),
		Elevation: m["elevation"].(float64),
	}
	for _, o := range m["openings"].([]any) {
		om := o.(map[string]any)
		w.Openings = append(w.Openings, encounter.AtlasStructuralOpening{
			ID: om["id"].(string), Position: om["position"].(float64), Width: om["width"].(float64),
		})
	}

	return w
}

func doorFromRow(t *testing.T, raw any) encounter.AtlasStructuralDoor {
	t.Helper()
	m := raw.(map[string]any)

	return encounter.AtlasStructuralDoor{
		ID:        encounter.DoorID(m["id"].(string)),
		Ref:       m["ref"].(string),
		From:      pointFromRow(t, m["from"]),
		To:        pointFromRow(t, m["to"]),
		Height:    m["height"].(float64),
		Thickness: m["thickness"].(float64),
		Elevation: m["elevation"].(float64),
	}
}

func pointFromRow(t *testing.T, raw any) spatial.Point {
	t.Helper()
	m := raw.(map[string]any)

	return spatial.Point{X: m["x"].(float64), Y: m["y"].(float64)}
}

// --- Fixed rows carry only fixed layout ---

func (s *StructuralRevealSuite) TestStructuralRowsCarryNoStateOrPrivateAssociation() {
	enc := s.open(hiddenDoorRevealField(), findsEverything{})
	_, err := enc.Search(&encounter.SearchInput{Member: "finder", Region: "hall"})
	s.Require().NoError(err)

	beats := s.structuralBeats(enc, "finder", encounter.BeatConcealmentRevealed)
	s.Require().Len(beats, 1)
	body := beats[0]

	wallRow := body["structural_walls"].([]any)[0].(map[string]any)
	s.Equal(map[string]bool{
		"id": true, "ref": true, "from": true, "to": true,
		"height": true, "thickness": true, "elevation": true, "openings": true,
	}, keySet(wallRow))
	openingRow := wallRow["openings"].([]any)[0].(map[string]any)
	s.Equal(map[string]bool{"id": true, "position": true, "width": true}, keySet(openingRow))

	doorRow := body["structural_doors"].([]any)[0].(map[string]any)
	s.Equal(map[string]bool{
		"id": true, "ref": true, "from": true, "to": true,
		"height": true, "thickness": true, "elevation": true,
	}, keySet(doorRow))
	for _, forbidden := range []string{"state", "lock", "placed_id", "door_id", "parent", "concealed"} {
		s.NotContains(wallRow, forbidden)
		s.NotContains(openingRow, forbidden)
		s.NotContains(doorRow, forbidden)
	}
}

func keySet(m map[string]any) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}

	return out
}

// --- Legacy and replay ---

func (s *StructuralRevealSuite) TestALegacyRevealAddsNoStructuralKeys() {
	enc := s.open(legacyRevealField(), findsNothing{})
	_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door", Actor: "finder"})
	s.Require().NoError(err)

	beats := s.structuralBeats(enc, "finder", encounter.BeatRoomRevealed)
	s.Require().Len(beats, 1)
	s.NotContains(beats[0], "structural_walls", "a field with no structural layout writes no key")
	s.NotContains(beats[0], "structural_doors")
}

func (s *StructuralRevealSuite) TestAnEarlierPayloadNeverAcquiresLaterSecretIDs() {
	enc := s.open(structuralRevealField(), findsNothing{})
	_, err := enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door", Actor: "finder"})
	s.Require().NoError(err)

	first := s.structuralBeatPayloads(enc, "finder", encounter.BeatRoomRevealed)
	s.Require().Len(first, 1)
	s.NotContains(string(first[0]), "vault/gate-b", "the later region's door is not in the first payload")

	// A later discovery must not rewrite the earlier payload. The finder
	// walks into the room and opens the second door, so the farther region is
	// revealed as a genuinely later event. The room's own footprint door is
	// opened world-side first, because it stands across the walk.
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: "vault/gate-a"})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: "finder", To: spatial.Position{X: 6, Y: 1}})
	s.Require().NoError(err)
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: "far-door", Actor: "finder"})
	s.Require().NoError(err)
	again := s.structuralBeatPayloads(enc, "finder", encounter.BeatRoomRevealed)
	s.Require().Len(again, 2)
	s.Equal(string(first[0]), string(again[0]), "an old event is not enriched by later knowledge")

	// Nor does a reload.
	reloaded := s.reload(enc)
	afterLoad := s.structuralBeatPayloads(reloaded, "finder", encounter.BeatRoomRevealed)
	s.Require().Len(afterLoad, 2)
	s.Equal(string(first[0]), string(afterLoad[0]))
	s.NotContains(string(afterLoad[0]), "vault/gate-b")
}
