// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

type BoundaryTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestBoundarySuite(t *testing.T) { suite.Run(t, new(BoundaryTestSuite)) }

func (s *BoundaryTestSuite) SetupTest() { s.ctx = context.Background() }

// heard records every turn boundary that reached the bus, in the order it
// arrived — which is the only thing these tests are about.
type heard struct {
	starts []dnd5eEvents.TurnStartEvent
	ends   []dnd5eEvents.TurnEndEvent
	order  []string
}

func (h *heard) listen(ctx context.Context, bus events.EventBus) {
	_, _ = dnd5eEvents.TurnStartTopic.On(bus).Subscribe(ctx,
		func(_ context.Context, e dnd5eEvents.TurnStartEvent) error {
			h.starts = append(h.starts, e)
			h.order = append(h.order, "start:"+e.SubjectID)
			return nil
		})
	_, _ = dnd5eEvents.TurnEndTopic.On(bus).Subscribe(ctx,
		func(_ context.Context, e dnd5eEvents.TurnEndEvent) error {
			h.ends = append(h.ends, e)
			h.order = append(h.order, "end:"+e.SubjectID)
			return nil
		})
}

// runBoundary drives a boundary interaction on a bus this test holds, so it can
// hear what was published. resolveOn exists for exactly this.
func (s *BoundaryTestSuite) runBoundary(crossed []encounter.Boundary) (*Output, *heard, error) {
	machine, err := NewBoundary(&BoundaryInput{Crossed: crossed})
	s.Require().NoError(err)

	h := &heard{}
	bus := events.NewEventBus()
	h.listen(s.ctx, bus)

	out, err := resolveOn(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Character: probeSheet(heroID)}},
		Initiative:   orderAsGiven{}, TurnDriver: passDriver{},
		Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Roller:  dice.NewRoller(),
		Machine: machine,
	}, newSurface(bus))
	return out, h, err
}

func (s *BoundaryTestSuite) world() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{},
		Announcer: quietAnnouncer{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc.ToData()
}

// TestEveryCrossingReachesTheBusInOrder is the point of the whole machine: a
// clock advance's boundaries arrive on the interaction's own bus, in the causal
// order the composition crossed them, where every attached effect hears them.
func (s *BoundaryTestSuite) TestEveryCrossingReachesTheBusInOrder() {
	out, h, err := s.runBoundary([]encounter.Boundary{
		{Kind: encounter.TurnEnded, Subject: "alice", Round: 1},
		{Kind: encounter.TurnStarted, Subject: "bob", Round: 1},
		{Kind: encounter.TurnEnded, Subject: "bob", Round: 1},
		{Kind: encounter.TurnStarted, Subject: "alice", Round: 2},
	})
	s.Require().NoError(err)

	s.Equal([]string{"end:alice", "start:bob", "end:bob", "start:alice"}, h.order,
		"causal order survives the trip from the composition to the bus")

	// The machine ITSELF attaches nothing: every registration on the way in
	// belongs to a participant's own sheet-keeper, which is what R3 attaching
	// everyone already produces. A boundary interaction only publishes.
	for _, h := range out.Hooks {
		s.Equal(heroID, h.Participant, "every hook belongs to a participant, none to the machine")
	}
	s.Require().IsType(BoundaryOutcome{}, out.Outcome)
	s.Equal(4, out.Outcome.(BoundaryOutcome).Announced)
}

// TestTheRoundRidesOnTheTurn pins the other half of Kirk's ruling from this
// side of the seam: there is no round event, and the round is not lost — it
// arrives as a field on the turn boundary that follows the wrap.
func (s *BoundaryTestSuite) TestTheRoundRidesOnTheTurn() {
	_, h, err := s.runBoundary([]encounter.Boundary{
		{Kind: encounter.TurnEnded, Subject: "alice", Round: 1},
		{Kind: encounter.TurnStarted, Subject: "alice", Round: 2},
	})
	s.Require().NoError(err)

	s.Require().Len(h.ends, 1)
	s.Require().Len(h.starts, 1)
	s.Equal(1, h.ends[0].Round, "the turn that ended belonged to round 1")
	s.Equal(2, h.starts[0].Round, "and the one that started belongs to round 2")
}

// TestTheSubjectSurvivesTheTranslation — a monster's id reaches the event
// unchanged. The field it lands in is called SubjectID precisely so this is not
// a lie (rpg-toolkit#1258).
func (s *BoundaryTestSuite) TestTheSubjectSurvivesTheTranslation() {
	_, h, err := s.runBoundary([]encounter.Boundary{
		{Kind: encounter.TurnEnded, Subject: "goblin-7", Round: 3},
	})
	s.Require().NoError(err)
	s.Require().Len(h.ends, 1)
	s.Equal("goblin-7", h.ends[0].SubjectID)
}

// TestAnUnknownKindIsRefusedRatherThanPublishedAsNothing is the load-bearing
// refusal. The composition's kind set and this package's topic table are the
// same set by construction; if one grows, a switch with a default would
// silently publish nothing and the new boundary would be as inert as
// TurnStartTopic has been all along.
func (s *BoundaryTestSuite) TestAnUnknownKindIsRefusedRatherThanPublishedAsNothing() {
	_, err := NewBoundary(&BoundaryInput{Crossed: []encounter.Boundary{
		{Kind: encounter.BoundaryKind("round_ended"), Subject: "alice", Round: 1},
	}})
	s.Require().ErrorIs(err, ErrBadBoundary)
}

// TestAnEmptyAdvanceIsNotAnInteraction — running one would load every sheet,
// attach every effect, publish nothing, and leave a resolution in the record
// that no boundary explains.
func (s *BoundaryTestSuite) TestAnEmptyAdvanceIsNotAnInteraction() {
	_, err := NewBoundary(&BoundaryInput{})
	s.Require().ErrorIs(err, ErrBadBoundary)

	_, err = NewBoundary(nil)
	s.Require().ErrorIs(err, ErrNilInput)
}

// TestASubjectlessCrossingIsRefused — both kinds are about somebody, and a
// crossing with no subject would publish an event every condition's own guard
// silently ignores. Inert, and indistinguishable from not having happened.
func (s *BoundaryTestSuite) TestASubjectlessCrossingIsRefused() {
	_, err := NewBoundary(&BoundaryInput{Crossed: []encounter.Boundary{
		{Kind: encounter.TurnEnded, Round: 1},
	}})
	s.Require().ErrorIs(err, ErrBadBoundary)
}

// TestTheInputIsCopied — a caller that reuses its slice cannot change what this
// machine will publish after it was built.
func (s *BoundaryTestSuite) TestTheInputIsCopied() {
	crossed := []encounter.Boundary{{Kind: encounter.TurnEnded, Subject: "alice", Round: 1}}
	machine, err := NewBoundary(&BoundaryInput{Crossed: crossed})
	s.Require().NoError(err)

	crossed[0].Subject = "somebody-else"

	h := &heard{}
	bus := events.NewEventBus()
	h.listen(s.ctx, bus)
	_, err = resolveOn(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Character: probeSheet(heroID)}},
		Initiative:   orderAsGiven{}, TurnDriver: passDriver{},
		Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Roller:  dice.NewRoller(),
		Machine: machine,
	}, newSurface(bus))
	s.Require().NoError(err)

	s.Require().Len(h.ends, 1)
	s.Equal("alice", h.ends[0].SubjectID)
}
