// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// placement_test.go pins what this composition says about WHERE somebody is
// (rpg-toolkit#1040). Every answer is dungeon-absolute: the composition keeps
// rooms, and projects the absolute geometry so its caller sees one map
// (rpg-project#227).
//
// Every fixture here anchors its rooms AWAY from the origin, and that is the
// whole design of the file. A room at (0,0) makes local and absolute the same
// number, so a test built on one passes identically against an implementation
// that never projects at all — which is exactly why the module's existing
// suites went green on this change without noticing it.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type PlacementSuite struct {
	suite.Suite
	enc *encounter.Encounter
}

func TestPlacementSuite(t *testing.T) {
	suite.Run(t, new(PlacementSuite))
}

// hallOrigin and vaultOrigin are deliberately non-zero and different from each
// other, so that a projection using the wrong room's origin is as visible as
// one that forgets to project.
var (
	hallOrigin  = spatial.Position{X: 30, Y: 10}
	vaultOrigin = spatial.Position{X: 38, Y: 10}
)

// gate is the open door joining the two regions, standing in the crossing
// from the hall's own [7,4] — authored absolute [37,14] — to the vault's
// [0,4] — [38,14] — which are adjacent (W3).
var gate = openDoorway("gate", 37, 14, 38, 14)

func (s *PlacementSuite) SetupTest() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{
				rectRegion("hall", int(hallOrigin.X), int(hallOrigin.Y), 8, 8),
				rectRegion("vault", int(vaultOrigin.X), int(vaultOrigin.Y), 8, 8),
			},
			// The seam is walled, leaving the gate's row open: with one canvas
			// the two regions would otherwise share an open seam, and this
			// fixture's scenes need the ogre out of sight until somebody walks
			// through (rpg-toolkit#1106).
			Walls: seamWallRows(37, 10, 8, 14),
			Doors: []encounter.DoorInput{gate},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: hallOrigin.Add(spatial.Position{X: 2, Y: 3})},
			{ID: "ogre", Kind: encounter.KindMonster, Position: vaultOrigin.Add(spatial.Position{X: 5, Y: 1})},
		},
		Endings:   []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	s.enc = enc
}

// absolute is the FIXTURE's own arithmetic — the anchor above plus a
// room-local cell — used as the cross-check throughout: what the composition
// reports must be what the authored layout says, computed here rather than
// asked of the composition, so the two cannot agree by sharing a bug.
//
// It used to call the composition's Absolute bridge, which is gone: the rooms
// are compiled onto one canvas at construction now, so there is nothing left to
// project at runtime and nothing to ask (rpg-toolkit#1106).
func absoluteIn(origin, local spatial.Position) spatial.Position {
	seat := local.Add(origin)
	return cellAt(int(seat.X), int(seat.Y))
}

// TestTheRosterSaysWhereEverybodyStands is #933's half 2: a caller reads
// placement without serializing the aggregate to get at two floats.
func (s *PlacementSuite) TestTheRosterSaysWhereEverybodyStands() {
	members, err := s.enc.Members()
	s.Require().NoError(err)
	s.Require().Len(members, 2)

	// Sorted by ID, so alice then ogre.
	s.Equal(encounter.MemberID("alice"), members[0].ID)
	s.Equal(cellAt(32, 13), members[0].Position, "hall-local (2,3) anchored at (30,10)")
	s.Equal(absoluteIn(hallOrigin, spatial.Position{X: 2, Y: 3}), members[0].Position,
		"the roster and the authored layout must agree")

	s.Equal(encounter.MemberID("ogre"), members[1].ID)
	s.Equal(cellAt(43, 11), members[1].Position, "vault-local (5,1) anchored at (38,10)")
	s.Equal(absoluteIn(vaultOrigin, spatial.Position{X: 5, Y: 1}), members[1].Position,
		"and for the other room's anchor too")
}

// TestTheRosterAgreesWithTheMap crosses the roster against the map, the two
// reads a client renders together. A member reported at a cell no region holds
// is a member drawn outside the chamber they are standing in, and a member
// whose reported region disagrees with the cell under them is the dual state
// deriving it was supposed to end.
func (s *PlacementSuite) TestTheRosterAgreesWithTheMap() {
	members, err := s.enc.Members()
	s.Require().NoError(err)

	for _, member := range members {
		owner, ok := s.enc.RegionAt(member.Position)
		s.Require().True(ok, "%s stands at %v, which is not a cell of any region on the map",
			member.ID, member.Position)
		s.Equal(member.Region, owner, "%s is reported in %q but their cell belongs to %q",
			member.ID, member.Region, owner)
	}
}

// TestJoiningReportsAbsolutePlacement: the verb that places somebody answers in
// the same space the roster does. A join that reported local would hand a host
// a coordinate it could only fix by knowing about rooms.
func (s *PlacementSuite) TestJoiningReportsAbsolutePlacement() {
	out, err := s.enc.Join(&encounter.JoinInput{
		Member: "bob", Kind: encounter.KindPlayer,
		Cell: absoluteIn(vaultOrigin, spatial.Position{X: 1, Y: 6}),
	})
	s.Require().NoError(err)

	s.Equal(cellAt(39, 16), out.Member.Position, "vault-local (1,6) anchored at (38,10)")
	s.Equal(absoluteIn(vaultOrigin, spatial.Position{X: 1, Y: 6}), out.Member.Position)
}

// beatAt decodes the story beat a verb reported, found by the sequence the verb
// itself returned rather than by taking the last entry.
//
// Not fussiness: a doorway crossing can put two sides in sight of each other
// and start a fight, whose own beat lands AFTER the crossing's. Reading "the
// last beat" would silently assert against a bubble-formed payload — which is
// exactly what it did on the first run of this file.
func (s *PlacementSuite) beatAt(member encounter.MemberID, seq uint64) map[string]interface{} {
	entries, err := s.enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)

	for _, entry := range entries {
		if entry.Seq != seq {
			continue
		}
		var payload map[string]interface{}
		s.Require().NoError(json.Unmarshal(entry.Payload, &payload))
		return payload
	}
	s.Require().Fail("no beat at seq", "%d is not in %s's story", seq, member)
	return nil
}

// TestTheMovedBeatSpeaksAbsolute.
//
// This beat is the one that was not merely a dialect mismatch but unresolvable:
// it carried a room-local cell and NO room, so two members standing in
// different rooms could report the same "position" and mean cells at opposite
// ends of the dungeon.
func (s *PlacementSuite) TestTheMovedBeatSpeaksAbsolute() {
	moved, err := s.enc.Step(&encounter.StepInput{
		Member: "alice", To: absoluteIn(hallOrigin, spatial.Position{X: 3, Y: 3})})
	s.Require().NoError(err)

	payload := s.beatAt("alice", moved.Seq)
	s.Equal("moved", payload["beat"])
	at := cellAt(33, 13)
	s.Equal(map[string]interface{}{"x": at.X, "y": at.Y}, payload["position"],
		"hall-local (3,3) anchored at (30,10), as an axial cell")
}

// TestTheCrossingBeatSpeaksAbsoluteAndNamesNoRoom.
//
// Two assertions, and the second is the point: the room key is GONE, not merely
// projected alongside. A beat that still named a room would leave the concept
// crossing the seam in the one place a client actually reads.
//
// The beat says "moved", like every other step. A crossing stopped being a
// second kind of movement when the field stopped being a set of rooms
// (rpg-toolkit#1106); what survives of it is the doorway's NAME, alongside.
func (s *PlacementSuite) TestTheCrossingBeatSpeaksAbsoluteAndNamesNoRoom() {
	// Walk to the doorway first, one cell at a time, in dungeon-absolute cells.
	_, err := s.enc.Step(&encounter.StepInput{
		Member: "alice", To: absoluteIn(hallOrigin, spatial.Position{X: 3, Y: 4})})
	s.Require().NoError(err)
	for _, x := range []float64{4, 5, 6, 7} {
		_, err = s.enc.Step(&encounter.StepInput{
			Member: "alice", To: absoluteIn(hallOrigin, spatial.Position{X: x, Y: 4})})
		s.Require().NoError(err)
	}

	// Standing in the opening she can see the ogre, and that IS a fight — the
	// doorway is a window now (rpg-toolkit#1106). She breaks off before
	// slipping through, and the break-off must succeed: if no bubble had
	// formed, the premise of this walk changed and the test should say so
	// rather than tolerate it.
	_, err = s.enc.Dissolve(&encounter.DissolveInput{Member: "alice"})
	s.Require().NoError(err, "the walk to the threshold puts her in the ogre's sight")

	crossed, err := s.enc.Step(&encounter.StepInput{
		Member: "alice", To: absoluteIn(vaultOrigin, spatial.Position{X: 0, Y: 4})})
	s.Require().NoError(err)
	s.Require().Len(crossed.Doors, 1)
	s.Equal(encounter.DoorID("gate"), crossed.Doors[0].ID)

	payload := s.beatAt("alice", crossed.Seq)
	s.Equal("moved", payload["beat"])
	s.Equal([]any{"gate"}, payload["doors"], "the door is narrated, not the room")
	farSide := cellAt(38, 14)
	s.Equal(map[string]interface{}{"x": farSide.X, "y": farSide.Y}, payload["position"],
		"vault-local (0,4) anchored at (38,10) — the far side of the doorway, as an axial cell")
	s.NotContains(payload, "room", "rooms are the composition's own business")
}

// TestTheDoorwayKissesInAbsoluteSpace is not a pin on this change so much as
// the reason the change is coherent: the two endpoint cells are ADJACENT once
// projected, which is what makes a crossing an ordinary step on one map rather
// than a jump between two coordinate systems.
func (s *PlacementSuite) TestTheDoorwayKissesInAbsoluteSpace() {
	atlas, err := s.enc.Atlas()
	s.Require().NoError(err)
	s.Require().Len(atlas.Doorways, 1)

	doorway := atlas.Doorways[0]
	s.Equal(cellAt(37, 14), doorway.From)
	s.Equal(cellAt(38, 14), doorway.To)
	s.Equal(float64(1), adjacencyDistance(doorway.From, doorway.To), "one step apart on the dungeon map")
}

// TestTheOutcomeSpeaksAbsolute (rpg-toolkit#1068) closes the last room-local
// report on this surface.
//
// An outcome is the one placement report a host reads AFTER the encounter is
// over, when it has nothing left to cross-check against — no roster call, no
// further beats. It carried a room-local cell while every other read had
// already flipped, so a party that finished in a room anchored anywhere but
// the origin was drawn at cells belonging to whatever room happens to sit
// there.
func (s *PlacementSuite) TestTheOutcomeSpeaksAbsolute() {
	ended, err := s.enc.End(&encounter.EndInput{Ending: "done"})
	s.Require().NoError(err)

	placed := map[encounter.MemberID]spatial.Position{}
	for _, m := range ended.Outcome.Members {
		placed[m.ID] = m.Position
	}
	s.Require().Len(placed, 2)
	s.Equal(cellAt(32, 13), placed["alice"], "hall-local (2,3) anchored at (30,10)")
	s.Equal(cellAt(43, 11), placed["ogre"], "vault-local (5,1) anchored at (38,10)")

	s.Equal(absoluteIn(hallOrigin, spatial.Position{X: 2, Y: 3}), placed["alice"],
		"the outcome and the authored layout must agree")
	s.Equal(absoluteIn(vaultOrigin, spatial.Position{X: 5, Y: 1}), placed["ogre"],
		"and for the other room's anchor too")
}

// TestTheOutcomeAgreesWithTheRoster is the cross-check that makes the flip
// mean something: the last thing a host hears about where somebody stands must
// be the same cell as the last roster read, not the same numbers in a
// different frame.
func (s *PlacementSuite) TestTheOutcomeAgreesWithTheRoster() {
	roster, err := s.enc.Members()
	s.Require().NoError(err)
	standing := map[encounter.MemberID]spatial.Position{}
	for _, m := range roster {
		standing[m.ID] = m.Position
	}

	ended, err := s.enc.End(&encounter.EndInput{Ending: "done"})
	s.Require().NoError(err)

	for _, m := range ended.Outcome.Members {
		s.Equal(standing[m.ID], m.Position, "%s finished where the roster last put them", m.ID)
	}
}

// TestAClosedEncounterKeepsAnsweringAbsolute: Status re-reads the stored
// outcome rather than rebuilding it, so it is its own path and its own pin.
func (s *PlacementSuite) TestAClosedEncounterKeepsAnsweringAbsolute() {
	_, err := s.enc.End(&encounter.EndInput{Ending: "done"})
	s.Require().NoError(err)

	status, err := s.enc.Status()
	s.Require().NoError(err)
	s.Require().NotNil(status.Outcome)

	placed := map[encounter.MemberID]spatial.Position{}
	for _, m := range status.Outcome.Members {
		placed[m.ID] = m.Position
	}
	s.Equal(cellAt(32, 13), placed["alice"])
	s.Equal(cellAt(43, 11), placed["ogre"])
}

// TestAnExitReportsAbsolutePlacement. Exit builds its departing member's
// outcome itself — from the spatial room, not from buildMemberOutcomes — so
// projecting the shared path and leaving this one alone would flip everything
// except the report the leaver actually gets.
func (s *PlacementSuite) TestAnExitReportsAbsolutePlacement() {
	left, err := s.enc.Exit(&encounter.ExitInput{Member: "alice"})
	s.Require().NoError(err)

	s.Equal(cellAt(32, 13), left.Outcome.Position,
		"hall-local (2,3) anchored at (30,10)")
	s.Equal(absoluteIn(hallOrigin, spatial.Position{X: 2, Y: 3}), left.Outcome.Position)
}

// TestTheOutcomeSurvivesARoundTripStillAbsolute.
//
// Persistence is where a frame flip goes wrong quietly: a loader that stores
// the cell it was given and hands it back unchanged passes every live test in
// this file and still returns a different frame after a save/load, because the
// blob was written in one dialect and read in another.
func (s *PlacementSuite) TestTheOutcomeSurvivesARoundTripStillAbsolute() {
	_, err := s.enc.End(&encounter.EndInput{Ending: "done"})
	s.Require().NoError(err)

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: s.enc.ToData(), Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)

	status, err := reloaded.Status()
	s.Require().NoError(err)
	s.Require().NotNil(status.Outcome)

	placed := map[encounter.MemberID]spatial.Position{}
	for _, m := range status.Outcome.Members {
		placed[m.ID] = m.Position
	}
	s.Equal(cellAt(32, 13), placed["alice"], "still on the dungeon map after a save")
	s.Equal(cellAt(43, 11), placed["ogre"])
}
