// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// CellKnowledgeSuite is THE CELL IS THE UNIT OF KNOWLEDGE (rpg-project#586
// R1, R2, R4, R5) on ordinary rooms, not concealment flags: discovery must not
// depend on an author marking every unexplored room as a secret, and it must
// not teach a room because one of its cells was seen.
//
// The fixture is three rooms in a row — entry, room, beyond — joined by two
// shut edge doors on row 1, with an alcove of walls around b's corner cell and
// a scenery cell in the room. Sight is per member and settable.
type CellKnowledgeSuite struct {
	suite.Suite
	enc   *encounter.Encounter
	sight *sightList
}

func TestCellKnowledgeSuite(t *testing.T) { suite.Run(t, new(CellKnowledgeSuite)) }

func (s *CellKnowledgeSuite) SetupTest() {
	s.sight = &sightList{fallback: 20}
	s.start()
}

func (s *CellKnowledgeSuite) capabilities() encounter.Capabilities {
	return encounter.Capabilities{
		Sight:         s.sight,
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        zeroSheets{},
		Participation: everyoneStanding{},
		Initiative:    orderAsGiven{},
		Driver:        passDriver{},
		CheckResolver: findsNothing{},
		Witness:       nobodyPerceives{},
		Actors: encounter.Actors{
			Striker:   passStriker{},
			Mover:     quietMover{},
			Announcer: quietAnnouncer{},
		},
	}
}

func (s *CellKnowledgeSuite) field() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion("entry", 0, 0, 3, 5),
			rectRegion("room", 3, 0, 4, 5),
			rectRegion("beyond", 7, 0, 3, 5),
		},
		Scenery: []spatial.Position{{X: 5, Y: 5}},
		Walls: append(append(seamWallExcept(2, 5, 1), seamWallExcept(6, 5, 1)...),
			wall(0, 4, 0, 3), wall(0, 4, 1, 4)),
		Doors: []encounter.DoorInput{
			{ID: "entry-door", Edges: doorEdgesAcross(2, 1), State: encounter.DoorIsClosed()},
			{ID: "far-door", Edges: doorEdgesAcross(6, 1), State: encounter.DoorIsClosed()},
		},
		Exits: []encounter.FieldExit{{ID: "hidden-exit", At: cellAt(8, 1)}},
	}
}

// start builds the encounter from the suite's field with a at the entry
// door's west edge and b in the walled alcove, unless members are given.
func (s *CellKnowledgeSuite) start(members ...encounter.MemberInput) {
	s.startWith(s.field(), members...)
}

func (s *CellKnowledgeSuite) startWith(field encounter.FieldInput, members ...encounter.MemberInput) {
	if len(members) == 0 {
		members = []encounter.MemberInput{
			{ID: "a", Kind: encounter.KindPlayer, Position: cellAt(2, 1)},
			{ID: "b", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 4}},
		}
	}
	var err error
	s.enc, err = encounter.NewEncounter(&encounter.SetupInput{
		Field:        field,
		Members:      members,
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: s.capabilities(),
	})
	s.Require().NoError(err)
}

func (s *CellKnowledgeSuite) reload() {
	bytes, err := json.Marshal(s.enc.ToData())
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(bytes, &data))
	s.enc, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: s.capabilities()})
	s.Require().NoError(err)
}

func (s *CellKnowledgeSuite) atlas(member encounter.MemberID) encounter.Atlas {
	out, err := s.enc.AtlasFor(member)
	s.Require().NoError(err)
	return out
}

func (s *CellKnowledgeSuite) visible(member encounter.MemberID) []spatial.Position {
	out, err := s.enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: member})
	s.Require().NoError(err)
	s.Require().NotNil(out.Cells, "never nil")
	return out.Cells
}

// regionCells is one region's cells as a recipient's atlas carries them, or
// nil when the region is absent from it.
func regionCells(atlas encounter.Atlas, id encounter.RegionID) []spatial.Position {
	for _, r := range atlas.Regions {
		if r.ID == id {
			return r.Cells
		}
	}
	return nil
}

func cellSet(cells []spatial.Position) map[spatial.Position]bool {
	out := make(map[spatial.Position]bool, len(cells))
	for _, c := range cells {
		out[c] = true
	}
	return out
}

func (s *CellKnowledgeSuite) full() encounter.Atlas {
	out, err := s.enc.Atlas()
	s.Require().NoError(err)
	return out
}

func (s *CellKnowledgeSuite) member(id encounter.MemberID) spatial.Position {
	members, err := s.enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.Require().Fail("no such member", string(id))
	return spatial.Position{}
}

func (s *CellKnowledgeSuite) cellsRevealed(member encounter.MemberID) []map[string]json.RawMessage {
	story, err := s.enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	out := []map[string]json.RawMessage{}
	for _, entry := range story {
		var beat map[string]json.RawMessage
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if string(beat["beat"]) == `"`+encounter.BeatCellsRevealed+`"` {
			out = append(out, beat)
		}
	}
	return out
}

// TestSightLearnsOnlyReachedCells: a reach of one hex teaches the cells one
// hex away and nothing else of the room the observer stands in — and the cell
// across the shut door, one hex away too, is behind a wall and stays unknown.
func (s *CellKnowledgeSuite) TestSightLearnsOnlyReachedCells() {
	s.sight.reach = map[encounter.MemberID]int{"a": 1}
	s.start()
	a := s.member("a")

	want := []spatial.Position{}
	for _, cell := range regionCells(s.full(), "entry") {
		if s.enc.Distance(a, cell) <= 1 {
			want = append(want, cell)
		}
	}
	s.Require().Less(len(want), len(regionCells(s.full(), "entry")), "the fixture leaves part of the room out of reach")

	atlas := s.atlas("a")
	s.Equal(want, regionCells(atlas, "entry"), "the region entry carries exactly the reached cells")
	s.Nil(regionCells(atlas, "room"), "a cell behind a wall is unknown until seen, however close")
	s.NotContains(atlas.Cells, cellAt(3, 1), "the far edge of the shut door is not floor a has seen")
	s.Equal(want, s.visible("a"), "seen now is the same reached cells")
}

// TestABlindMemberSeesOnlyTheCellTheyStandOn: no sight still knows its floor.
func (s *CellKnowledgeSuite) TestABlindMemberSeesOnlyTheCellTheyStandOn() {
	s.sight.reach = map[encounter.MemberID]int{"a": 0}
	s.start()
	a := s.member("a")
	s.Equal([]spatial.Position{a}, s.visible("a"))
	s.Equal([]spatial.Position{a}, regionCells(s.atlas("a"), "entry"))
}

// TestRememberedCellsSurviveLossOfSightAndReload: what the open door showed
// stays on the map when the door shuts, drops out of seen-now, and survives a
// JSON round trip.
func (s *CellKnowledgeSuite) TestRememberedCellsSurviveLossOfSightAndReload() {
	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door", Actor: "a"})
	s.Require().NoError(err)
	learned := s.atlas("a")
	room := regionCells(learned, "room")
	s.Require().NotEmpty(room, "the open door showed a some of the room")
	s.Less(len(room), len(regionCells(s.full(), "room")), "but not all of it: sight through a doorway is a slice")
	seen := cellSet(s.visible("a"))
	for _, cell := range room {
		s.True(seen[cell], "while the door stands open, a sees what it learned through it")
	}

	_, err = s.enc.CloseDoor(&encounter.CloseDoorInput{Door: "entry-door", Actor: "a"})
	s.Require().NoError(err)
	s.Equal(learned, s.atlas("a"), "memory is never forgotten within the run")
	seen = cellSet(s.visible("a"))
	for _, cell := range room {
		if cell != cellAt(3, 1) {
			s.False(seen[cell], "behind the shut door the room is remembered, not seen")
		}
	}

	s.reload()
	s.Equal(learned, s.atlas("a"))
	s.Nil(regionCells(s.atlas("b"), "room"), "and memory is per seat")
}

// TestVisibleCellsIsPerMemberAndSubsetOfAtlas: two seats in one room see
// different cells, a seat in another room sees none of theirs, and every
// answer is inside its own atlas.
func (s *CellKnowledgeSuite) TestVisibleCellsIsPerMemberAndSubsetOfAtlas() {
	s.start(
		encounter.MemberInput{ID: "a", Kind: encounter.KindPlayer, Position: cellAt(2, 1)},
		encounter.MemberInput{ID: "b", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 4}},
		encounter.MemberInput{ID: "c", Kind: encounter.KindPlayer, Position: cellAt(5, 2)},
	)
	answers := map[encounter.MemberID][]spatial.Position{}
	for _, id := range []encounter.MemberID{"a", "b", "c"} {
		answers[id] = s.visible(id)
		known := cellSet(s.atlas(id).Cells)
		for _, cell := range answers[id] {
			s.True(known[cell], "%s sees %v, which is not on %s's own map", id, cell, id)
		}
	}
	s.NotEqual(answers["a"], answers["b"], "two seats in one room see different cells")
	inRoom := cellSet(answers["c"])
	for _, id := range []encounter.MemberID{"a", "b"} {
		for _, cell := range answers[id] {
			s.False(inRoom[cell], "c, behind a shut door, shares no seen cell with %s", id)
		}
	}
}

// TestVisibleCellsExcludesSightAreaAndConcealment: an obscuring area drops
// cells from seen-now and leaves them remembered; a concealed cell is never
// seen.
func (s *CellKnowledgeSuite) TestVisibleCellsExcludesSightAreaAndConcealment() {
	field := s.field()
	field.Concealments = []encounter.ConcealmentInput{{ID: "nook", Checks: vaultCheck(), Cells: []spatial.Position{{X: 0, Y: 0}}}}
	s.startWith(field)
	nook := cellAt(0, 0)
	s.NotContains(s.visible("a"), nook, "a concealed cell is never seen")
	s.NotContains(s.atlas("a").Cells, nook, "nor known")

	before := s.atlas("a")
	far := cellAt(0, 2)
	s.Require().Contains(s.visible("a"), far)
	s.Require().NoError(s.enc.AddSightArea(&encounter.SightAreaInput{
		ID: "fog", SourceID: "spell", Center: cellAt(1, 2), RadiusFeet: 5,
	}))
	s.NotContains(s.visible("a"), far, "the area between a and the cell drops it from seen-now")
	s.Equal(before, s.atlas("a"), "while it stays on a's map")
	s.Contains(s.visible("a"), s.member("a"), "and a still stands somewhere")
}

// TestVisibleCellsRefusesAStranger: ErrNotMember and ErrNilInput by name.
func (s *CellKnowledgeSuite) TestVisibleCellsRefusesAStranger() {
	_, err := s.enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: "nobody"})
	s.ErrorIs(err, encounter.ErrNotMember)
	_, err = s.enc.VisibleCellsFor(nil)
	s.ErrorIs(err, encounter.ErrNilInput)
}

// sheetless is a Sight capability that, once armed, refuses any question
// naming a member it has no sheet for — what the session's sheet seam does for
// a character whose record is gone or a world NPC with no stored sheet.
type sheetless struct {
	inner   encounter.Sight
	missing map[encounter.MemberID]bool
	armed   bool
}

func (s *sheetless) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	if s.armed {
		for _, id := range members {
			if s.missing[id] {
				return nil, fmt.Errorf("character %s: no such character", id)
			}
		}
	}
	return s.inner.Sight(members)
}

// TestVisibleCellsAsksOnlyTheViewersSight: seen-now for one seat needs that
// seat's range alone, so another member whose sight cannot be answered does
// not fail the read — a player without a sheet, or a world NPC.
func (s *CellKnowledgeSuite) TestVisibleCellsAsksOnlyTheViewersSight() {
	for _, other := range []encounter.MemberInput{
		{ID: "b", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 4}},
		{ID: "npc", Kind: encounter.KindWorld, Position: cellAt(1, 3)},
	} {
		s.Run(string(other.Kind), func() {
			sight := &sheetless{inner: &sightList{fallback: 20}, missing: map[encounter.MemberID]bool{other.ID: true}}
			enc, err := encounter.NewEncounter(&encounter.SetupInput{
				Field: s.field(),
				Members: []encounter.MemberInput{
					{ID: "a", Kind: encounter.KindPlayer, Position: cellAt(2, 1)}, other,
				},
				Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
				Capabilities: func() encounter.Capabilities {
					c := s.capabilities()
					c.Sight = sight
					return c
				}(),
			})
			s.Require().NoError(err)
			want, err := enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: "a"})
			s.Require().NoError(err)

			sight.armed = true
			got, err := enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: "a"})
			s.Require().NoError(err, "another member's missing sheet is not a's read's business")
			s.Equal(want.Cells, got.Cells)
			_, err = enc.AtlasFor("a")
			s.Require().NoError(err, "the atlas asks no sight at all")

			_, err = enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: other.ID})
			s.Require().Error(err, "the member whose sight cannot be answered is still refused for its own read")
		})
	}
}

// TestOpeningTeachesOnlyTheDoorEdge: a blind opener learns the door's two
// edge cells and nothing of the room beyond.
func (s *CellKnowledgeSuite) TestOpeningTeachesOnlyTheDoorEdge() {
	s.sight.reach = map[encounter.MemberID]int{"a": 0}
	s.start()
	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door", Actor: "a"})
	s.Require().NoError(err)
	atlas := s.atlas("a")
	s.Equal([]spatial.Position{cellAt(3, 1)}, regionCells(atlas, "room"), "the far edge cell, and no more")
	s.Equal([]spatial.Position{cellAt(2, 1)}, regionCells(atlas, "entry"), "and the near one a stands on")
	s.Nil(regionCells(s.atlas("b"), "room"), "nobody else learns from a's door")
}

// TestAWorldSideOpeningTeachesOnlyWhatObserversSee: no actor, no edge lesson.
func (s *CellKnowledgeSuite) TestAWorldSideOpeningTeachesOnlyWhatObserversSee() {
	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door"})
	s.Require().NoError(err)
	s.NotEmpty(regionCells(s.atlas("a"), "room"), "a watches the door open and sees through it")
	s.Nil(regionCells(s.atlas("b"), "room"), "b's alcove hides the doorway")
}

// TestUnseenDoorIsAbsentAndSeenDoorHasASighting: before any edge cell is
// known the far door is absent from the atlas and has no sighting; the sight
// pass that learns its edge brings both.
func (s *CellKnowledgeSuite) TestUnseenDoorIsAbsentAndSeenDoorHasASighting() {
	doors := func(atlas encounter.Atlas) map[encounter.DoorID]bool {
		out := map[encounter.DoorID]bool{}
		for _, d := range atlas.Doorways {
			out[d.Door] = true
		}
		return out
	}
	sighted := func() map[encounter.DoorID]bool {
		out := map[encounter.DoorID]bool{}
		got, err := s.enc.DoorSightings(&encounter.ViewInput{Member: "a"})
		s.Require().NoError(err)
		for _, d := range got {
			out[d.Door.ID] = true
		}
		return out
	}
	s.True(doors(s.atlas("a"))["entry-door"], "a door on the edge of a known cell is presented")
	s.False(doors(s.atlas("a"))["far-door"], "a door with no known cell on any edge is absent")
	s.False(sighted()["far-door"])

	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door", Actor: "a"})
	s.Require().NoError(err)
	s.Require().Contains(s.atlas("a").Cells, cellAt(6, 1), "the fixture's sightline reaches the far door's edge")
	s.True(doors(s.atlas("a"))["far-door"], "first sight reveals the door as it is")
	s.True(sighted()["far-door"], "and the same pass sighted it")
}

// TestTheAtlasWithholdsAWallTouchingOnlyUnknownCells: the seam between the
// room and beyond borders nothing a knows; the seam a stands at does.
func (s *CellKnowledgeSuite) TestTheAtlasWithholdsAWallTouchingOnlyUnknownCells() {
	known := cellSet(s.atlas("a").Cells)
	full := s.full()
	presented := map[encounter.AtlasBoundary]bool{}
	for _, b := range s.atlas("a").Boundaries {
		presented[b] = true
	}
	sawNear, sawFar := false, false
	for _, b := range full.Boundaries {
		touchesKnown := known[b.From] || known[b.To]
		s.Equal(touchesKnown, presented[b], "boundary %v-%v", b.From, b.To)
		sawNear = sawNear || touchesKnown
		sawFar = sawFar || !touchesKnown
	}
	s.True(sawNear && sawFar, "the fixture has walls of both kinds")
}

// TestUnexploredCellsSynthesizeNoWall: unknown ordinary floor is not a wall.
// Every boundary a's partial map carries is an authored one.
func (s *CellKnowledgeSuite) TestUnexploredCellsSynthesizeNoWall() {
	s.sight.reach = map[encounter.MemberID]int{"a": 1}
	s.start()
	authored := map[encounter.AtlasBoundary]bool{}
	for _, b := range s.full().Boundaries {
		authored[b] = true
	}
	atlas := s.atlas("a")
	s.Require().Less(len(regionCells(atlas, "entry")), len(regionCells(s.full(), "entry")), "a's map ends inside the room")
	for _, b := range atlas.Boundaries {
		s.True(authored[b], "boundary %v-%v was synthesized at the edge of the known", b.From, b.To)
	}
}

// TestCellsRevealedBeatIsOnePerPassAndRecipientScoped: one beat per learning
// observer per pass, audienced to that observer, carrying partial regions of
// only the newly known cells — and nothing for an unchanged recheck.
func (s *CellKnowledgeSuite) TestCellsRevealedBeatIsOnePerPassAndRecipientScoped() {
	s.Empty(s.cellsRevealed("a"), "first sight is the scene-opened snapshot, not a beat")
	before := s.atlas("a")
	had := cellSet(before.Cells)

	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door"})
	s.Require().NoError(err)
	beats := s.cellsRevealed("a")
	s.Require().Len(beats, 1, "one pass, one beat")
	s.Empty(s.cellsRevealed("b"), "recipient-scoped: b learned nothing and is told nothing")

	var regions []struct {
		ID    string
		Cells []spatial.Position
	}
	s.Require().NoError(json.Unmarshal(beats[0]["regions"], &regions))
	s.Require().NotEmpty(regions)
	after := s.atlas("a")
	for _, r := range regions {
		s.Less(len(r.Cells), len(regionCells(s.full(), encounter.RegionID(r.ID))), "region %s is partial", r.ID)
		for _, cell := range r.Cells {
			s.False(had[cell], "only newly known cells ride the beat")
			s.Contains(regionCells(after, encounter.RegionID(r.ID)), cell)
		}
	}
	s.NotContains(string(beats[0]["doorways"]), `"state"`, "door state is not fixed layout")
	s.NotContains(string(beats[0]["exits"]), "hidden-exit")

	journal := s.enc.ToData().Journal
	_, err = s.enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{"a"}})
	s.Require().NoError(err)
	s.Equal(journal, s.enc.ToData().Journal, "unchanged sight appends no discovery history")
	s.Len(s.cellsRevealed("a"), 1, "and tells nothing")

	atlas := s.atlas("a")
	atlas.Cells[0].X = 999
	atlas.Regions[0].Cells[0].X = 999
	s.NotEqual(atlas.Cells, s.atlas("a").Cells, "the atlas is a copy")
	s.NotEqual(atlas.Regions, s.atlas("a").Regions)
}

// TestCellsRevealedSendsEachRecordOnce: every list on the beat is a
// difference against the recipient's own prior map (rpg-api-protos
// 69a68f3e). Opening the entry door as its actor writes two beats — the
// door's edge, then what the open door shows — and nothing either carries was
// already a's: not the doorway a already held, not a frontier wall a already
// had, and not anything the first beat sent.
func (s *CellKnowledgeSuite) TestCellsRevealedSendsEachRecordOnce() {
	before := s.atlas("a")
	held := map[string]bool{}
	for _, d := range before.Doorways {
		held[fmt.Sprintf("door %s %v %v", d.Door, d.From, d.To)] = true
	}
	for _, b := range before.Boundaries {
		held[fmt.Sprintf("wall %v %v", b.From, b.To)] = true
	}
	for _, c := range before.Cells {
		held[fmt.Sprintf("cell %v", c)] = true
	}
	s.Require().True(held[fmt.Sprintf("door entry-door %v %v", cellAt(2, 1), cellAt(3, 1))], "a already holds the entry doorway")

	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: "entry-door", Actor: "a"})
	s.Require().NoError(err)
	beats := s.cellsRevealed("a")
	s.Require().NotEmpty(beats)
	for i, beat := range beats {
		var body struct {
			Doorways []struct {
				Door     string
				From, To spatial.Position
			}
			Boundaries []struct{ From, To spatial.Position }
			Scenery    []spatial.Position
			Regions    []struct{ Cells []spatial.Position }
		}
		raw, err := json.Marshal(beat)
		s.Require().NoError(err)
		s.Require().NoError(json.Unmarshal(raw, &body))
		keys := []string{}
		for _, d := range body.Doorways {
			keys = append(keys, fmt.Sprintf("door %s %v %v", d.Door, d.From, d.To))
		}
		for _, b := range body.Boundaries {
			keys = append(keys, fmt.Sprintf("wall %v %v", b.From, b.To))
		}
		for _, c := range body.Scenery {
			keys = append(keys, fmt.Sprintf("cell %v", c))
		}
		for _, r := range body.Regions {
			for _, c := range r.Cells {
				keys = append(keys, fmt.Sprintf("cell %v", c))
			}
		}
		for _, key := range keys {
			s.False(held[key], "beat %d re-sends %s, which the recipient already holds", i, key)
			held[key] = true
		}
	}
	after := s.atlas("a")
	for _, d := range after.Doorways {
		s.True(held[fmt.Sprintf("door %s %v %v", d.Door, d.From, d.To)], "and the patches add up to the fresh map")
	}
	for _, b := range after.Boundaries {
		s.True(held[fmt.Sprintf("wall %v %v", b.From, b.To)], "every wall too")
	}
}

// TestInvalidCellKnowledgeIsRejectedOnLoad: the load boundary refuses a cell
// off the floor, an undecodable payload, a widened audience, a foreign subject
// or a foreign actor; a retired whole-room fact loads and teaches nothing.
func (s *CellKnowledgeSuite) TestInvalidCellKnowledgeIsRejectedOnLoad() {
	for _, change := range []string{"cell", "payload", "empty", "audience", "subject", "actor"} {
		s.Run(change, func() {
			data := s.enc.ToData()
			s.Require().NotEmpty(data.Journal.Facts)
			fact := &data.Journal.Facts[0]
			s.Require().Equal("known:cells", fact.Kind)
			switch change {
			case "cell":
				fact.Detail = `{"cells":[{"x":99,"y":99}]}`
			case "payload":
				fact.Detail = `not json`
			case "empty":
				fact.Detail = `{"cells":[]}`
			case "audience":
				fact.Audience = []string{"a", "b"}
			case "subject":
				fact.Subject = "other"
			case "actor":
				fact.Actor = "other"
			}
			_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: s.capabilities()})
			s.ErrorIs(err, encounter.ErrInvalidData)
		})
	}

	s.Run("a retired room fact", func() {
		before := s.atlas("a")
		data := s.enc.ToData()
		data.Journal.Facts = append(data.Journal.Facts, encounter.FactData{
			Kind: "known:room:beyond", Actor: "a", Subject: "a", Audience: []string{"a"},
		})
		loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: s.capabilities()})
		s.Require().NoError(err, "an old save still loads")
		after, err := loaded.AtlasFor("a")
		s.Require().NoError(err)
		s.Equal(before, after, "and its room fact teaches nothing")
	})
}
