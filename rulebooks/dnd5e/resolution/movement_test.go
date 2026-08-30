// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// everyoneSwings answers with one attack for anybody who asks.
type everyoneSwings struct{ asked []string }

func (e *everyoneSwings) AttackFor(reactorID string) (combatActions.Definition, bool) {
	e.asked = append(e.asked, reactorID)
	return validMeleeDefinition(), true
}

// nobodySwings is the empty-handed caster: it answers, and the answer is no.
type nobodySwings struct{ asked []string }

func (n *nobodySwings) AttackFor(reactorID string) (combatActions.Definition, bool) {
	n.asked = append(n.asked, reactorID)
	return combatActions.Definition{}, false
}

type MovementTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestMovementSuite(t *testing.T) { suite.Run(t, new(MovementTestSuite)) }

func (s *MovementTestSuite) SetupTest() { s.ctx = context.Background() }

func (s *MovementTestSuite) world() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Mover: encounter.RefusingMover{},
		Announcer: quietAnnouncer{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: wolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 1}},
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 1}},
			{ID: "zara", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

// runStep drives one movement interaction on a bus the test holds, so it can
// subscribe the way a condition would.
func (s *MovementTestSuite) runStep(
	in *MovementInput, listen func(context.Context, events.EventBus),
) (*Output, error) {
	machine, err := NewMovement(in)
	s.Require().NoError(err)

	bus := events.NewEventBus()
	if listen != nil {
		listen(s.ctx, bus)
	}

	return resolveOn(s.ctx, &Input{
		World: s.world(),
		Participants: []Participant{
			{Character: probeSheet(heroID)}, {Character: probeSheet(wolfID)},
			{Character: probeSheet("alice")}, {Character: probeSheet("zara")},
		},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Roller:  dice.NewRoller(),
		Machine: machine,
	}, newSurface(bus))
}

func (s *MovementTestSuite) stepInput() *MovementInput {
	return &MovementInput{
		Mover: wolfID, MoverKind: "monster",
		From: spatial.Position{X: 2, Y: 1}, To: spatial.Position{X: 5, Y: 1},
		Reactions: &everyoneSwings{}, Roller: dice.NewRoller(),
	}
}

// subscribeMovement attaches a MovementChain subscriber that mutates nothing,
// which is how a condition that only WATCHES a step behaves.
func watchSteps(seen *[]dnd5eEvents.MovementChainEvent) func(context.Context, events.EventBus) {
	return func(ctx context.Context, bus events.EventBus) {
		_, _ = dnd5eEvents.MovementChain.On(bus).SubscribeWithChain(ctx,
			func(_ context.Context, e *dnd5eEvents.MovementChainEvent,
				c chain.Chain[*dnd5eEvents.MovementChainEvent],
			) (chain.Chain[*dnd5eEvents.MovementChainEvent], error) {
				*seen = append(*seen, *e)
				return c, nil
			})
	}
}

// THE POINT OF THE WHOLE MACHINE, and the acceptance test named in
// rpg-project#316's Done-when: something that wants to notice a step subscribes
// to MovementChain and is heard, with no change to the caller to make it so.
//
// Before this machine the live walk published nothing at all, so a condition
// with a perfect predicate and its own green suite could not fire in a real
// game — which is exactly what happened to the opportunity attack.
func (s *MovementTestSuite) TestAStepReachesEverythingListeningForOne() {
	var seen []dnd5eEvents.MovementChainEvent
	_, err := s.runStep(s.stepInput(), watchSteps(&seen))
	s.Require().NoError(err)

	s.Require().Len(seen, 1, "one step, one publish")
	s.Equal(wolfID, seen[0].EntityID)
	s.Equal("monster", seen[0].EntityType)
	s.EqualValues(2, seen[0].FromPosition.X)
	s.EqualValues(5, seen[0].ToPosition.X)
}

// The sixth uninstalled registry (rpg-toolkit#1251's family). gamectx
// IsReactionReady fails closed, so until this was installed every reaction
// condition was gated behind a map nobody supplied — the reason the opportunity
// attack's own suite is green while the game's is not is that each of those
// tests installs one by hand.
func (s *MovementTestSuite) TestEveryParticipantIsReadiedForTheFreeReactions() {
	readied := map[string]bool{}
	_, err := s.runStep(s.stepInput(), func(ctx context.Context, bus events.EventBus) {
		_, _ = dnd5eEvents.MovementChain.On(bus).SubscribeWithChain(ctx,
			func(inner context.Context, _ *dnd5eEvents.MovementChainEvent,
				c chain.Chain[*dnd5eEvents.MovementChainEvent],
			) (chain.Chain[*dnd5eEvents.MovementChainEvent], error) {
				oa := refs.Conditions.OpportunityAttack().String()
				readied[heroID] = gamectx.IsReactionReady(inner, heroID, oa)
				readied[wolfID] = gamectx.IsReactionReady(inner, wolfID, oa)
				readied["shield"] = gamectx.IsReactionReady(inner, heroID, refs.Spells.Shield().String())
				return c, nil
			})
	})
	s.Require().NoError(err)

	s.True(readied[heroID], "a free reaction is readied by being in the interaction")
	s.True(readied[wolfID], "for monsters too — the gate is not a character rule")
	s.False(readied["shield"], "a COSTED reaction is not opted into on the player's behalf")
}

// A reaction resolves inside this interaction, on its own bus and over its own
// cast, rather than being handed back for the caller to run separately.
func (s *MovementTestSuite) TestATriggeredReactionSwingsWithinTheSameInteraction() {
	out, err := s.runStep(s.stepInput(), triggerFrom(heroID, wolfID))
	s.Require().NoError(err)

	moved, ok := out.Outcome.(MovementOutcome)
	s.Require().True(ok, "a movement interaction reports a movement outcome")
	s.Require().Len(moved.Reactions, 1)

	got := moved.Reactions[0]
	s.Equal(heroID, got.ReactorID)
	s.Equal(wolfID, got.Against, "the reaction answers the mover")
	s.Equal(refs.Conditions.OpportunityAttack().String(), got.ConditionRef)
	s.Equal(heroID, got.Struck.AttackerID, "the strike really ran; this is its outcome")
	s.Equal(wolfID, got.Struck.TargetID)
	s.Positive(got.Struck.Total, "a strike that never rolled is not a strike")
}

// preventOAsInAStage adds OA prevention the way DisengagingCondition really
// does it: as a CHAIN STAGE, not from the subscriber.
//
// That distinction is the whole point of the test below. A stage runs during
// Execute, which is after every subscriber has already been and gone — so a
// subscriber that tried to read IsOAPrevented() for itself would read an empty
// field every time.
func preventOAsInAStage(holder string) func(context.Context, events.EventBus) {
	return func(ctx context.Context, bus events.EventBus) {
		_, _ = dnd5eEvents.MovementChain.On(bus).SubscribeWithChain(ctx,
			func(_ context.Context, _ *dnd5eEvents.MovementChainEvent,
				c chain.Chain[*dnd5eEvents.MovementChainEvent],
			) (chain.Chain[*dnd5eEvents.MovementChainEvent], error) {
				err := c.Add(combat.StageConditions, "disengaging",
					func(_ context.Context, e *dnd5eEvents.MovementChainEvent,
					) (*dnd5eEvents.MovementChainEvent, error) {
						e.OAPreventionSources = append(e.OAPreventionSources,
							dnd5eEvents.MovementModifierSource{
								Name: "Disengaging", SourceType: "condition",
								SourceRef: refs.Conditions.Disengaging(), EntityID: holder,
							})
						return e, nil
					})
				return c, err
			})
	}
}

// runStepWithCast is runStep with the participants supplied, so a test can
// seat REAL conditions on the fold instead of standing in for them.
func (s *MovementTestSuite) runStepWithCast(in *MovementInput, cast []Participant) (*Output, error) {
	machine, err := NewMovement(in)
	s.Require().NoError(err)

	return resolveOn(s.ctx, &Input{
		World:        s.world(),
		Participants: cast,
		Initiative:   orderAsGiven{}, TurnDriver: passDriver{},
		Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Roller:  dice.NewRoller(),
		Machine: machine,
	}, newSurface(events.NewEventBus()))
}

// reactorSheet is a probe sheet that can actually react: an economy with a
// reaction in it, which is what canReact reads, plus whatever conditions the
// test wants seated.
func reactorSheet(id string, blobs ...json.RawMessage) *character.Data {
	data := probeSheet(id)
	data.ActionEconomy = &character.ActionEconomyData{
		TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
		ReactionsRemaining: 1, MovementRemaining: 30,
	}
	data.Conditions = append(data.Conditions, blobs...)
	return data
}

// TestBothRealConditionsOnOneRealFold is the pair that found the bug, run at
// the layer that fixes it.
//
// Everything here is the production article: the real
// OpportunityAttackCondition on the hero, the real DisengagingCondition on the
// mover, both seated by attachAll out of ordinary character data, folded by the
// ordinary machine. No hand-published event, no stand-in for a publish.
//
// That matters because hand-feeding is exactly how the bug hid. The condition
// suite's own prevention case fed a pre-populated event straight to the
// subscriber — a path production never runs — and passed for an unrelated
// reason while the check it claimed to cover was dead. This test cannot pass
// for an unrelated reason: remove the machine's filter and it fails.
func (s *MovementTestSuite) TestBothRealConditionsOnOneRealFold() {
	oaBlob, err := conditions.NewOpportunityAttackCondition(heroID).ToJSON()
	s.Require().NoError(err)
	disengageBlob, err := conditions.NewDisengagingCondition(wolfID).ToJSON()
	s.Require().NoError(err)

	s.Run("without Disengage the hero swings", func() {
		out, rerr := s.runStepWithCast(s.stepInput(), []Participant{
			{Character: reactorSheet(heroID, oaBlob)},
			{Character: reactorSheet(wolfID)},
		})
		s.Require().NoError(rerr)

		moved, ok := out.Outcome.(MovementOutcome)
		s.Require().True(ok)
		s.False(moved.OAPrevented, "nothing prevented anything")
		s.Require().Len(moved.Reactions, 1,
			"the real condition fired on a real fold — no publish stood in for it")
		s.Equal(heroID, moved.Reactions[0].ReactorID)
		s.Equal(refs.Conditions.OpportunityAttack().String(), moved.Reactions[0].ConditionRef)
	})

	s.Run("with Disengage the outcome is consistent with itself", func() {
		out, rerr := s.runStepWithCast(s.stepInput(), []Participant{
			{Character: reactorSheet(heroID, oaBlob)},
			{Character: reactorSheet(wolfID, disengageBlob)},
		})
		s.Require().NoError(rerr)

		moved, ok := out.Outcome.(MovementOutcome)
		s.Require().True(ok)

		// THE RECONCILIATION, asserted as a pair. The old code could report
		// both of these at once — prevention true AND a swing — because the
		// condition's own check was dead and nothing else looked. Either
		// assertion alone would still pass in that world; together they are
		// the statement.
		s.True(moved.OAPrevented, "the fold says opportunity attacks were prevented")
		s.Empty(moved.Reactions, "and none swung, which is what prevented has to mean")
	})
}

// bothAttach installs two subscribers on the one bus runStep hands over.
func bothAttach(a, b func(context.Context, events.EventBus)) func(context.Context, events.EventBus) {
	return func(ctx context.Context, bus events.EventBus) { a(ctx, bus); b(ctx, bus) }
}

// TestAPreventedOpportunityAttackDoesNotSwing is Disengage actually working,
// and it is here rather than in the condition because THIS is the only layer
// that can enforce it.
//
// The ordering makes it so: prevention is written by a chain STAGE and read
// after Execute, while a reaction condition publishes its trigger from a
// SUBSCRIBER, strictly earlier. OpportunityAttackCondition used to check
// IsOAPrevented() itself and could never have seen anything — a check that read
// as load-bearing and was dead, found by the first end-to-end walk of both real
// conditions through one real fold (rpg-project#316).
//
// So the machine drops the trigger after the fold, where the answer is
// complete, and the condition no longer pretends to.
func (s *MovementTestSuite) TestAPreventedOpportunityAttackDoesNotSwing() {
	out, err := s.runStep(s.stepInput(),
		bothAttach(triggerFrom(heroID, wolfID), preventOAsInAStage(wolfID)))
	s.Require().NoError(err)

	moved, ok := out.Outcome.(MovementOutcome)
	s.Require().True(ok)
	s.True(moved.OAPrevented, "the fold says opportunity attacks were prevented")
	s.Empty(moved.Reactions,
		"and so none swung — the outcome must not report prevention AND a swing")
}

// TestPreventionIsPreciseToOpportunityAttacks: Disengage stops opportunity
// attacks, which is exactly as narrow as the rule is. A reaction of another
// kind to the same step is untouched — Shield is the one coming.
func (s *MovementTestSuite) TestPreventionIsPreciseToOpportunityAttacks() {
	otherKind := func(ctx context.Context, bus events.EventBus) {
		_, _ = dnd5eEvents.MovementChain.On(bus).SubscribeWithChain(ctx,
			func(inner context.Context, e *dnd5eEvents.MovementChainEvent,
				c chain.Chain[*dnd5eEvents.MovementChainEvent],
			) (chain.Chain[*dnd5eEvents.MovementChainEvent], error) {
				err := dnd5eEvents.ReactionTriggerTopic.On(bus).Publish(inner,
					dnd5eEvents.ReactionTriggerEvent{
						ReactorID:    heroID,
						ConditionRef: refs.Spells.Shield().String(),
						TriggerKind:  dnd5eEvents.TriggerKindPostHit,
						SourceEntity: wolfID,
						Payload:      *e,
					})
				return c, err
			})
	}

	out, err := s.runStep(s.stepInput(), bothAttach(otherKind, preventOAsInAStage(wolfID)))
	s.Require().NoError(err)

	moved, ok := out.Outcome.(MovementOutcome)
	s.Require().True(ok)
	s.True(moved.OAPrevented)
	s.Require().Len(moved.Reactions, 1,
		"an OA prevention must not silence a reaction that is not an OA")
	s.Equal(refs.Spells.Shield().String(), moved.Reactions[0].ConditionRef)
}

// triggerFrom publishes a reaction trigger during the fold, which is exactly
// what OpportunityAttackCondition does when its predicate matches — the
// condition itself is not used here because this module cannot seat one, and
// standing in for its PUBLISH is what keeps this a test of the machine.
func triggerFrom(reactor, mover string) func(context.Context, events.EventBus) {
	return func(ctx context.Context, bus events.EventBus) {
		_, _ = dnd5eEvents.MovementChain.On(bus).SubscribeWithChain(ctx,
			func(inner context.Context, e *dnd5eEvents.MovementChainEvent,
				c chain.Chain[*dnd5eEvents.MovementChainEvent],
			) (chain.Chain[*dnd5eEvents.MovementChainEvent], error) {
				err := dnd5eEvents.ReactionTriggerTopic.On(bus).Publish(inner,
					dnd5eEvents.ReactionTriggerEvent{
						ReactorID:    reactor,
						ConditionRef: refs.Conditions.OpportunityAttack().String(),
						TriggerKind:  dnd5eEvents.TriggerKindMovementOA,
						SourceEntity: mover,
						Payload:      *e,
					})
				return c, err
			})
	}
}

// A reactor with nothing to swing is an ANSWER, not a failure. The step still
// completes and the walk still happened.
func (s *MovementTestSuite) TestAReactorWithNoAttackIsSkippedNotFailed() {
	in := s.stepInput()
	empty := &nobodySwings{}
	in.Reactions = empty

	out, err := s.runStep(in, triggerFrom(heroID, wolfID))
	s.Require().NoError(err, "an empty-handed reactor must not break the walk")

	moved := out.Outcome.(MovementOutcome)
	s.Empty(moved.Reactions, "nothing was swung, so nothing is reported as swung")
	s.Equal([]string{heroID}, empty.asked, "the capability was still asked, per reactor")
}

// Disengage reads from out here: the fold says opportunity attacks were
// suppressed for this step, and the caller learns it without knowing what
// Disengage is.
func (s *MovementTestSuite) TestASuppressedStepReportsItsSuppression() {
	out, err := s.runStep(s.stepInput(), func(ctx context.Context, bus events.EventBus) {
		_, _ = dnd5eEvents.MovementChain.On(bus).SubscribeWithChain(ctx,
			func(_ context.Context, _ *dnd5eEvents.MovementChainEvent,
				c chain.Chain[*dnd5eEvents.MovementChainEvent],
			) (chain.Chain[*dnd5eEvents.MovementChainEvent], error) {
				err := c.Add(combat.StageConditions, "test_disengage",
					func(_ context.Context, e *dnd5eEvents.MovementChainEvent) (*dnd5eEvents.MovementChainEvent, error) {
						e.OAPreventionSources = append(e.OAPreventionSources,
							dnd5eEvents.MovementModifierSource{Name: "Test Disengage", SourceType: "condition", EntityID: wolfID})
						return e, nil
					})
				return c, err
			})
	})
	s.Require().NoError(err)

	s.True(out.Outcome.(MovementOutcome).OAPrevented,
		"the outcome is read off the FOLDED event, not echoed from the input")
}

// Identical inputs must produce identical stories (C8), so two reactors to one
// step are answered in a fixed order rather than in subscriber order.
func (s *MovementTestSuite) TestTwoReactorsAreAnsweredInADeterministicOrder() {
	swings := &everyoneSwings{}
	in := s.stepInput()
	in.Reactions = swings

	out, err := s.runStep(in, func(ctx context.Context, bus events.EventBus) {
		// Published deliberately in reverse-alphabetical order.
		triggerFrom("zara", wolfID)(ctx, bus)
		triggerFrom("alice", wolfID)(ctx, bus)
	})
	s.Require().NoError(err)

	moved := out.Outcome.(MovementOutcome)
	s.Require().Len(moved.Reactions, 2)
	s.Equal("alice", moved.Reactions[0].ReactorID)
	s.Equal("zara", moved.Reactions[1].ReactorID)
}

// Wiring being wrong is refused at the door, before the world is loaded — and a
// movement with no way to resolve a reaction is wiring being wrong, not a free
// movement. Accepting one would publish triggers and drop them, which looks
// exactly like an opportunity attack that never fired.
func (s *MovementTestSuite) TestMalformedMovementsAreRefusedAtTheDoor() {
	here := spatial.Position{X: 2, Y: 1}
	there := spatial.Position{X: 5, Y: 1}

	cases := map[string]*MovementInput{
		"no input at all": nil,
		"no mover": {
			From: here, To: there, Reactions: &everyoneSwings{}, Roller: dice.NewRoller(),
		},
		"a step that goes nowhere": {
			Mover: wolfID, From: here, To: here,
			Reactions: &everyoneSwings{}, Roller: dice.NewRoller(),
		},
		"no way to resolve a reaction": {
			Mover: wolfID, From: here, To: there, Roller: dice.NewRoller(),
		},
		"no roller": {
			Mover: wolfID, From: here, To: there, Reactions: &everyoneSwings{},
		},
	}

	for name, in := range cases {
		s.Run(name, func() {
			machine, err := NewMovement(in)
			s.Require().Error(err)
			s.Nil(machine, "a refused movement hands back no machine to drive")
		})
	}
}

// A machine constructed from a reused input must keep announcing the step it
// was built for, the same guarantee NewActivation makes about its ref.
func (s *MovementTestSuite) TestTheStepIsCopiedNotBorrowed() {
	in := s.stepInput()
	machine, err := NewMovement(in)
	s.Require().NoError(err)

	in.To = spatial.Position{X: 9, Y: 9}

	step, err := machine.Start(s.ctx, nil)
	s.Require().NoError(err)
	s.Contains(step.(Gather).Name(), "to (5,1)",
		"changing the caller's struct must not change what this machine announces")
}

// The outcome must READ the folded event, endpoints included, not echo the
// input back. They are the same values today — nothing in the rulebook moves a
// step's endpoints — which is exactly what lets an echo survive the whole suite
// while being wrong. The same warning is written into runWalk's loop one layer
// up, about the same mistake.
func (s *MovementTestSuite) TestTheOutcomeReportsWhereTheStepACTUALLYWent() {
	out, err := s.runStep(s.stepInput(), func(ctx context.Context, bus events.EventBus) {
		_, _ = dnd5eEvents.MovementChain.On(bus).SubscribeWithChain(ctx,
			func(_ context.Context, _ *dnd5eEvents.MovementChainEvent,
				c chain.Chain[*dnd5eEvents.MovementChainEvent],
			) (chain.Chain[*dnd5eEvents.MovementChainEvent], error) {
				// A shove, a slide, a door that opens onto a different cell.
				err := c.Add(combat.StageConditions, "test_shove",
					func(_ context.Context, e *dnd5eEvents.MovementChainEvent) (*dnd5eEvents.MovementChainEvent, error) {
						e.ToPosition = dnd5eEvents.Position{X: 7, Y: 3}
						return e, nil
					})
				return c, err
			})
	})
	s.Require().NoError(err)

	moved := out.Outcome.(MovementOutcome)
	s.Equal(spatial.Position{X: 7, Y: 3}, moved.To,
		"a modifier moved the step and the outcome must say so, not repeat the request")
	s.Equal(spatial.Position{X: 2, Y: 1}, moved.From, "the origin was untouched and still reads back")
}
