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
	starts    []dnd5eEvents.TurnStartEvent
	ends      []dnd5eEvents.TurnEndEvent
	fightEnds []dnd5eEvents.CombatEndEvent
	order     []string
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
	_, _ = dnd5eEvents.CombatEndTopic.On(bus).Subscribe(ctx,
		func(_ context.Context, e dnd5eEvents.CombatEndEvent) error {
			h.fightEnds = append(h.fightEnds, e)
			h.order = append(h.order, "fight:"+e.SubjectID)
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
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Mover: encounter.RefusingMover{},
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

// TestAFightEndingReachesEveryoneItEndedFor — the composition announces a
// fight's ending once per member, and every one of those reaches the bus.
//
// The run is what a real dissolve produces: the last turn boundary the clock
// crossed, then the ending, for everybody. A subscriber deciding whether an
// ending is its own (dnd5e/conditions.RagingCondition.onCombatEnd) needs its
// own name to arrive, which is what this asserts.
func (s *BoundaryTestSuite) TestAFightEndingReachesEveryoneItEndedFor() {
	out, h, err := s.runBoundary([]encounter.Boundary{
		{Kind: encounter.CombatEnded, Subject: "alice", Round: 4},
		{Kind: encounter.CombatEnded, Subject: "bob", Round: 4},
		{Kind: encounter.CombatEnded, Subject: "goblin-7", Round: 4},
	})
	s.Require().NoError(err)

	s.Equal([]string{"fight:alice", "fight:bob", "fight:goblin-7"}, h.order,
		"one ending per member, in the order the composition crossed them")
	s.Require().IsType(BoundaryOutcome{}, out.Outcome)
	s.Equal(3, out.Outcome.(BoundaryOutcome).Announced)
}

// TestAFightEndingRidesTheSameRunAsTheTurnBoundaries — the kinds are not
// separate mechanisms and must not become two.
//
// Mixed is the real case: nothing stops a composition from crossing a turn
// boundary and a fight ending in one advance, and causal order across the
// whole run is the promise BoundaryInput makes.
func (s *BoundaryTestSuite) TestAFightEndingRidesTheSameRunAsTheTurnBoundaries() {
	_, h, err := s.runBoundary([]encounter.Boundary{
		{Kind: encounter.TurnEnded, Subject: "alice", Round: 2},
		{Kind: encounter.CombatEnded, Subject: "alice", Round: 2},
		{Kind: encounter.CombatEnded, Subject: "goblin-7", Round: 2},
	})
	s.Require().NoError(err)
	s.Equal([]string{"end:alice", "fight:alice", "fight:goblin-7"}, h.order)
}

// TestAFightsEndingCarriesItsSubjectAndNothingElse pins the deliberate
// asymmetry with the turn events.
//
// CombatEndEvent has no Round, and that is a decision rather than an oversight:
// the round a fight ended on is not a coordinate anyone can use afterwards,
// because play/clock's Turn.Dissolve sets the round back to zero and the next
// fight starts again at 1. A subscriber storing it would be storing a number
// from a clock that no longer exists — which is the exact bug the slice this
// belongs to exists to make impossible.
func (s *BoundaryTestSuite) TestAFightsEndingCarriesItsSubjectAndNothingElse() {
	_, h, err := s.runBoundary([]encounter.Boundary{
		{Kind: encounter.CombatEnded, Subject: "goblin-7", Round: 9},
	})
	s.Require().NoError(err)
	s.Require().Len(h.fightEnds, 1)
	s.Equal(dnd5eEvents.CombatEndEvent{SubjectID: "goblin-7"}, h.fightEnds[0],
		"the whole event is the subject; a round here would outlive the clock it came from")
}

// TestTheTableCoversEveryKindThisBuildKnows guards the seal from the inside.
//
// boundaryTopics is a sealed lookup precisely so a kind it does not know is
// refused at the door rather than silently publishing nothing
// (TestAnUnknownKindIsRefusedRatherThanPublishedAsNothing is the other half).
// This is the half that fails when the table falls BEHIND: a kind added to the
// composition and not added here.
//
// Honest about its own limit: this cannot enumerate the composition's consts
// from another module, so it will not fail the moment encounter grows a fourth
// kind. What fails then is the refusal above, at the first announcement that
// carries it — loudly, in the session suite, rather than silently forever.
// This test's job is narrower and still worth doing: it stops the table being
// trimmed.
func (s *BoundaryTestSuite) TestTheTableCoversEveryKindThisBuildKnows() {
	for _, kind := range []encounter.BoundaryKind{
		encounter.TurnStarted, encounter.TurnEnded, encounter.CombatEnded,
	} {
		_, ok := boundaryTopics[kind]
		s.True(ok, "boundaryTopics has no entry for %q, so announcing one would be refused", kind)
	}
	s.Len(boundaryTopics, 3, "a fourth entry here needs a fourth kind above and a test with it")
}
