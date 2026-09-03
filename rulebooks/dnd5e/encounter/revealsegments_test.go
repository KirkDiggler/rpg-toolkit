// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// revealsegments_test.go is THE REVEAL CARRIES THE WALLS THE ROOM IS DRAWN
// WITH (rpg-toolkit#1480, wall-geometry design §5.2 as amended, C19).
//
// A client draws walls from SEGMENTS now, not by chaining crossings back into
// runs. So a reveal beat that carries a room's boundaries and not its segments
// hands the recipient a room with no walls — which is the tell the masquerade
// exists to remove, arriving at the moment the secret opens.
//
// The beat is a PATCH to a cached atlas, and the pin that matters is that the
// patch and the atlas agree: apply what the beat says to what the recipient
// had, and you have what AtlasFor now answers, byte for byte. Anything less is
// two computations of one truth, which is how the two learn to disagree.

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type RevealSegmentsSuite struct {
	suite.Suite

	witness *scriptedWitness
}

func TestRevealSegmentsSuite(t *testing.T) {
	suite.Run(t, new(RevealSegmentsSuite))
}

// The finder perceives the panel, so opening it is what reveals the room —
// "perceived its door open", the ordinary way a secret opens in play.
func (s *RevealSegmentsSuite) SetupTest() {
	s.witness = &scriptedWitness{
		perceivers: map[encounter.DoorID][]encounter.MemberID{"panel": {finder}},
	}
}

// findThenOpen is how the vault opens: search until the panel is found, then
// open it. Finding a CLOSED door reveals the door alone; the room arrives when
// somebody who can see it watches it swing.
func (s *RevealSegmentsSuite) findThenOpen(enc *encounter.Encounter) {
	s.T().Helper()
	_, err := enc.Search(&encounter.SearchInput{Member: finder, Region: "hall"})
	s.Require().NoError(err)
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: "panel", Actor: finder})
	s.Require().NoError(err)
}

const finder = core.EntityID("finder")

// vaultField is a visible hall and a concealed vault behind a concealed door,
// with three walls: the seam the door hides in, and TWO walls wholly inside
// the vault, which are the ones a non-knower must not be shown and a knower
// must be.
//
// Each room carries a sealed cell, so the reveal has both halves of the thing
// to carry and something to get wrong about each.
func (s *RevealSegmentsSuite) vaultField() encounter.FieldInput {
	const rows = 6

	return encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion("hall", 0, 0, 4, rows),
			func() encounter.RegionInput {
				r := rectRegion("vault", 4, 0, 4, rows)
				r.Concealed = true
				return r
			}(),
		},
		Walls: withHeight(seamWallExcept(3, rows, 1), 2),
		// ONE SEALED CELL EACH SIDE. The vault's is what the reveal must
		// carry; the hall's is what it must not — a patch for one room that
		// listed another room's cells would have the recipient file them
		// under the room they just found.
		Sealed: []spatial.Position{{X: 1, Y: 3}, {X: 6, Y: 2}},
		Doors: []encounter.DoorInput{{
			ID:        "panel",
			Edges:     doorEdgesAcross(3, 1),
			State:     encounter.DoorIsClosed(),
			Concealed: []encounter.CheckApproach{{Ability: "perception", DC: 15}},
		}},
		Segments: []encounter.SegmentInput{
			{
				Name: "the seam", Height: 2,
				From: encounter.AxialPointF{Q: 4, R: -0.5},
				To:   encounter.AxialPointF{Q: 1, R: 5.5},
				Footprint: []spatial.Position{
					{X: 4, Y: 0}, {X: 3, Y: 1}, {X: 4, Y: 2},
					{X: 3, Y: 3}, {X: 4, Y: 4}, {X: 3, Y: 5},
				},
				DoorIDs: []encounter.DoorID{"panel"},
			},
			{
				Name: "the vault's spine", Height: 3,
				From:      encounter.AxialPointF{Q: 6, R: 0.5},
				To:        encounter.AxialPointF{Q: 5, R: 2.5},
				Footprint: []spatial.Position{{X: 6, Y: 1}, {X: 6, Y: 2}},
			},
			{
				Name:      "the vault's alcove",
				From:      encounter.AxialPointF{Q: 6, R: 3.5},
				To:        encounter.AxialPointF{Q: 5, R: 5.5},
				Footprint: []spatial.Position{{X: 6, Y: 4}, {X: 6, Y: 5}},
			},
		},
	}
}

func (s *RevealSegmentsSuite) open(resolver encounter.CheckResolver) *encounter.Encounter {
	s.T().Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: resolver, Witness: s.witness,
		Field: s.vaultField(),
		Members: []encounter.MemberInput{
			{ID: finder, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// beats reads one member's story, decoded.
func (s *RevealSegmentsSuite) beats(enc *encounter.Encounter, member core.EntityID, kind string) []map[string]any {
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

// segKeys names each wall in an atlas by its two ends.
//
// THE ENDS ARE A SEGMENT'S IDENTITY ON THE WIRE. [encounter.AtlasSegment]
// carries no name and no footprint on purpose (rpg-toolkit#1477): a segment
// that named its doors or the cells it stood on would say through the back
// door what the doorway list withholds. So a test about which walls a
// recipient has asks the same question a client does — where does this line
// run.
func segKeys(segments []encounter.AtlasSegment) []string {
	out := make([]string, 0, len(segments))
	for _, seg := range segments {
		out = append(out, segKey(seg.From, seg.To))
	}

	return out
}

func segKey(from, to encounter.AxialPointF) string {
	return fmt.Sprintf("%g,%g -> %g,%g", from.Q, from.R, to.Q, to.R)
}

// The three walls of [RevealSegmentsSuite.vaultField], by their ends.
var (
	theSeam   = segKey(encounter.AxialPointF{Q: 4, R: -0.5}, encounter.AxialPointF{Q: 1, R: 5.5})
	theSpine  = segKey(encounter.AxialPointF{Q: 6, R: 0.5}, encounter.AxialPointF{Q: 5, R: 2.5})
	theAlcove = segKey(encounter.AxialPointF{Q: 6, R: 3.5}, encounter.AxialPointF{Q: 5, R: 5.5})
)

// bodySegKey reads a beat's segment entry back into the same key.
func bodySegKey(raw any) string {
	seg := raw.(map[string]any)
	from := seg["from"].(map[string]any)
	to := seg["to"].(map[string]any)

	return fmt.Sprintf("%g,%g -> %g,%g",
		from["q"].(float64), from["r"].(float64), to["q"].(float64), to["r"].(float64))
}

// TestBeforeTheRevealTheWallIsWholeAndTheRoomIsNotThere is C19 restated at the
// point this issue is about: the non-knower gets the seam entire, with no gap
// and no sign of the room behind it, and none of the vault's own walls.
func (s *RevealSegmentsSuite) TestBeforeTheRevealTheWallIsWholeAndTheRoomIsNotThere() {
	enc := s.open(findsNothing{})

	blind, err := enc.AtlasFor(finder)
	s.Require().NoError(err)

	s.Equal([]string{theSeam}, segKeys(blind.Segments),
		"the seam presents; the walls inside the vault are withheld with it")
	s.Empty(blind.Doorways, "and no doorway cuts it — the whole wall, C19")
	for _, r := range blind.Regions {
		s.NotEqual(encounter.RegionID("vault"), r.ID)
	}
}

// TestTheRevealCarriesTheRoomsWallsAndItsSealedCells is the issue's own test.
func (s *RevealSegmentsSuite) TestTheRevealCarriesTheRoomsWallsAndItsSealedCells() {
	enc := s.open(findsEverything{})

	s.findThenOpen(enc)

	reveals := s.beats(enc, finder, "region_revealed")
	s.Require().Len(reveals, 1, "the vault entered the finder's knowledge once")
	body := reveals[0]

	// The walls the recipient did not have and now does — the two inside the
	// vault, and not the seam they could already see.
	segments, ok := body["segments"].([]any)
	s.Require().True(ok, "the reveal carries the room's walls, or the room has none")
	var revealed []string
	byWall := map[string]any{}
	for _, raw := range segments {
		revealed = append(revealed, bodySegKey(raw))
		byWall[bodySegKey(raw)] = raw.(map[string]any)["height"]
	}
	s.ElementsMatch([]string{theSpine, theAlcove}, revealed,
		"the walls inside the room, and not the seam they already had")

	// Heights ride along: a client draws a raised wall raised.
	s.Equal(3.0, byWall[theSpine], "the authored height, carried")
	s.Equal(0.0, byWall[theAlcove], "and not-authored stays not-authored")

	// The room's sealed cells: a cell that keeps its room and loses its feet.
	sealed, ok := body["sealed"].([]any)
	s.Require().True(ok, "the reveal says which of the room's cells nobody stands on")
	s.Require().Len(sealed, 1, "the vault's own sealed cell, and not the hall's")
	cell := sealed[0].(map[string]any)
	want := cellAt(6, 2) // authored [6,2]; the atlas answers in absolute axial
	s.Equal(want.X, cell["x"], "the cell inside the room being revealed")
	s.Equal(want.Y, cell["y"])

	// The doorway the mask was hiding.
	s.Require().Contains(body, "boundaries")
}

// TestTheRevealAndTheAtlasAgree is the pin the whole beat exists to keep: what
// the recipient had, plus what the beat says, IS what AtlasFor now answers.
func (s *RevealSegmentsSuite) TestTheRevealAndTheAtlasAgree() {
	enc := s.open(findsEverything{})

	before, err := enc.AtlasFor(finder)
	s.Require().NoError(err)

	s.findThenOpen(enc)

	after, err := enc.AtlasFor(finder)
	s.Require().NoError(err)

	reveals := s.beats(enc, finder, "region_revealed")
	s.Require().Len(reveals, 1)

	// SEGMENTS: what they had, plus what the beat added, is what they have.
	patched := append([]string(nil), segKeys(before.Segments)...)
	for _, raw := range reveals[0]["segments"].([]any) {
		patched = append(patched, bodySegKey(raw))
	}
	s.ElementsMatch(segKeys(after.Segments), patched,
		"the cached atlas plus the patch is the atlas — no wall invented, none missed")

	// SEALED: every cell the beat names is one the atlas now calls sealed.
	nowSealed := map[spatial.Position]bool{}
	for _, c := range after.Sealed {
		nowSealed[c] = true
	}
	for _, raw := range reveals[0]["sealed"].([]any) {
		cell := raw.(map[string]any)
		at := spatial.Position{X: cell["x"].(float64), Y: cell["y"].(float64)}
		s.Contains(nowSealed, at, "the beat names a cell the atlas agrees is sealed")
	}

	s.Greater(len(after.Segments), len(before.Segments), "the reveal really did add walls")
}
