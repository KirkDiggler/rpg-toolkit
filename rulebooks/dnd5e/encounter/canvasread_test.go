// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// canvasread_test.go is THE MAP, HANDED OUT TO BE READ (rpg-toolkit#1114).
//
// The composition has held one canvas since #1106 and told nobody. Everything
// it published about the map was a DESCRIPTION of it — the Atlas's regions and
// walls, a Member's cell, a region name — each correct, none of them the thing
// itself. A caller that needed to ask the map a question had to rebuild one out
// of those descriptions, and rpg-toolkit#1090's slice found what that costs: a
// reconstruction is a second implementation of the field's geometry, kept in
// step by a comment rather than by the compiler, and the copy that exists today
// has no walls in it at all.
//
// So the map is handed out. READ-ONLY, because it is the live one: what this
// returns is the encounter's own room, not a snapshot of it, and a caller that
// could place or move an entity on it would be moving members behind every
// verb's back — sight would not refresh, no beat would be written, and the
// blob would disagree with the world the next load produced.
//
// Three claims, and each of them is a way that could go wrong:
//
//   - IT IS LIVE. A snapshot would satisfy every read in this file except one,
//     and would quietly answer yesterday's question forever.
//   - IT REFUSES TO BE WRITTEN, BY NAME. A no-op mutator is worse than an
//     absent one: it reports success and changes nothing, which is the failure
//     shape this composition has paid for repeatedly.
//   - IT AGREES WITH THE ENCOUNTER ABOUT SIGHT. That is the question the
//     caller this exists for actually asks, and the walls are the half a
//     reconstruction gets wrong.
type CanvasReadSuite struct {
	suite.Suite

	enc *encounter.Encounter
}

func TestCanvasReadSuite(t *testing.T) {
	suite.Run(t, new(CanvasReadSuite))
}

// SetupTest opens the reference tomb from canvas_test.go — three walled
// chambers, two doorways on one row, pillars in the hall. The walls are the
// point: a map with nothing in the way agrees with any other map.
func (s *CanvasReadSuite) SetupTest() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: tombField(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: tombEntrance,
				Position: spatial.Position{X: 5, Y: float64(tombDoorRow)}},
			{ID: bob, Kind: encounter.KindPlayer, Room: tombHall,
				Position: spatial.Position{X: 0, Y: float64(tombDoorRow)}},
			{ID: carol, Kind: encounter.KindPlayer, Room: tombEntrance,
				Position: spatial.Position{X: 5, Y: float64(tombDoorRow - 1)}},
			{ID: dave, Kind: encounter.KindPlayer, Room: tombHall,
				Position: spatial.Position{X: 0, Y: float64(tombDoorRow - 1)}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	s.enc = enc
}

func (s *CanvasReadSuite) canvas() spatial.Room {
	canvas, err := s.enc.Canvas()
	s.Require().NoError(err)
	s.Require().NotNil(canvas)

	return canvas
}

// THE CANVAS IS THE LIVE MAP, and this is the test a snapshot fails.
//
// The canvas is taken FIRST, then a member walks, and then the canvas taken
// before the walk is asked where they are. A copy would still be holding the
// old cell and would answer confidently — which is why "returns the map" and
// "returns a picture of the map" cannot be told apart by any other read in this
// file.
//
// It matters because of what the caller is: an interaction installs this as the
// world its rules read positions out of, and a rule reading a stale world gives
// a wrong answer rather than no answer.
func (s *CanvasReadSuite) TestTheCanvasIsTheLiveMapNotACopy() {
	before := s.canvas()

	from, ok := before.GetEntityPosition(string(bob))
	s.Require().True(ok)

	to := tombAt(tombHallOrigin, 1, tombDoorRow)
	out, err := s.enc.Step(&encounter.StepInput{Member: bob, To: to})
	s.Require().NoError(err)
	s.Require().Equal(to, out.Stepped.To)
	s.Require().NotEqual(from, to, "the fixture has to actually move somebody")

	now, ok := before.GetEntityPosition(string(bob))
	s.Require().True(ok, "bob is still on the map")
	s.Equal(to, now, "the canvas taken before the step reports where bob is NOW")
}

// A REFUSAL, BY NAME, AND A REAL ONE. Both halves are asserted, because either
// on its own is a defect wearing the other's clothes: an error that did not
// actually prevent the write would be a lie, and a silent prevention would be
// the no-op this composition keeps deleting.
func (s *CanvasReadSuite) TestTheCanvasRefusesEveryWrite() {
	canvas := s.canvas()
	empty := tombAt(tombChamberOrigin, 4, 4)

	s.Run("place", func() {
		err := canvas.PlaceEntity(&readOnlyProbe{id: "intruder"}, empty)
		s.Require().ErrorIs(err, encounter.ErrReadOnly)
		s.Require().ErrorContains(err, "PlaceEntity", "the refusal names the verb refused")

		_, placed := canvas.GetEntityPosition("intruder")
		s.False(placed, "and nothing was placed")
	})

	s.Run("a nil entity is refused, not dereferenced", func() {
		// spatial.BasicRoom answers a nil entity with "entity cannot be nil".
		// A view in front of it that panicked instead would be harder to call
		// than the thing it protects, which is backwards for a read-only view.
		var err error
		s.Require().NotPanics(func() { err = canvas.PlaceEntity(nil, empty) })
		s.Require().ErrorIs(err, encounter.ErrReadOnly)
		s.Require().ErrorContains(err, "PlaceEntity")
	})

	s.Run("move", func() {
		was, ok := canvas.GetEntityPosition(string(alice))
		s.Require().True(ok)

		err := canvas.MoveEntity(string(alice), empty)
		s.Require().ErrorIs(err, encounter.ErrReadOnly)
		s.Require().ErrorContains(err, "MoveEntity")

		now, ok := canvas.GetEntityPosition(string(alice))
		s.Require().True(ok)
		s.Equal(was, now, "and alice did not move")
	})

	s.Run("remove", func() {
		err := canvas.RemoveEntity(string(alice))
		s.Require().ErrorIs(err, encounter.ErrReadOnly)
		s.Require().ErrorContains(err, "RemoveEntity")

		_, stillThere := canvas.GetEntityPosition(string(alice))
		s.True(stillThere, "and alice is still on the map")
	})
}

// A refused write must not reach the ENCOUNTER either — the reads above are
// taken through the same view that refused, so on their own they would pass for
// a view that wrote to a copy of the world and hid it.
func (s *CanvasReadSuite) TestARefusedWriteNeverReachesTheEncounter() {
	canvas := s.canvas()

	members, err := s.enc.Members()
	s.Require().NoError(err)

	_ = canvas.PlaceEntity(&readOnlyProbe{id: "intruder"}, tombAt(tombChamberOrigin, 4, 4))
	_ = canvas.MoveEntity(string(alice), tombAt(tombChamberOrigin, 5, 5))
	_ = canvas.RemoveEntity(string(dave))

	after, err := s.enc.Members()
	s.Require().NoError(err)
	s.Equal(members, after, "the roster and every cell in it are untouched")
}

// SIGHT AGREES, which is the question the caller this exists for asks.
//
// The encounter decides who sees whom on this canvas — within reach, and the
// ray unblocked (rebuildPercepts) — and the fixture gives everybody unlimited
// reach, so a sighting IS "the geometry admits it". Asking the handed-out map
// the same question about every pair has to give the same answer, and the tomb
// has walls and pillars in it so that both answers occur.
func (s *CanvasReadSuite) TestTheCanvasAnswersSightTheWayTheEncounterDoes() {
	canvas := s.canvas()

	members, err := s.enc.Members()
	s.Require().NoError(err)
	s.Require().Len(members, 4)

	blocked, clear := 0, 0
	for _, observer := range members {
		for _, subject := range members {
			if observer.ID == subject.ID {
				continue
			}

			isBlocked := canvas.IsLineOfSightBlocked(observer.Position, subject.Position)
			s.Equal(!isBlocked, s.holds(observer.ID, subject.ID),
				"the handed-out map and the encounter disagree about whether %s can see %s",
				observer.ID, subject.ID)

			if isBlocked {
				blocked++
			} else {
				clear++
			}
		}
	}

	s.Positive(blocked, "the tomb's walls block somebody")
	s.Positive(clear, "and the doorway lets somebody through")
}

// holds reports whether an observer holds anything at all on a subject —
// canvas_test.go's helper, which belongs to its own suite.
func (s *CanvasReadSuite) holds(observer, subject core.EntityID) bool {
	view, err := s.enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range view {
		if h.Subject == intel.Subject(subject) {
			return true
		}
	}

	return false
}

// An encounter with no canvas says so rather than handing back a nil map.
// Construction forbids one — both seams compile a canvas or fail — so this is
// only reachable through the zero value, which is exactly the case [Grid] and
// [Atlas] answer for too: a nil spatial.Room is not an absent answer, it is a
// wrong one that panics at the first read.
func TestACanvaslessEncounterSaysSo(t *testing.T) {
	canvas, err := (&encounter.Encounter{}).Canvas()
	require.ErrorIs(t, err, encounter.ErrNoField)
	require.Nil(t, canvas)
}

// readOnlyProbe is something to try to place. It is deliberately not a
// spatial.Placeable: the refusal never reaches spatial's placement rules, so
// nothing about whether this entity COULD be placed is ever consulted. Its ID
// is read, to name it in the refusal.
type readOnlyProbe struct {
	id string
}

func (p *readOnlyProbe) GetID() string            { return p.id }
func (p *readOnlyProbe) GetType() core.EntityType { return "probe" }
