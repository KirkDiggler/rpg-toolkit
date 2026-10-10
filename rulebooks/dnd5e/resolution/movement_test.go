// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// everyoneSwings answers with one attack for anybody who asks.
type everyoneSwings struct{ asked []string }

func (e *everyoneSwings) AttackFor(reactorID string) (combatActions.Definition, ReactionAnswer) {
	e.asked = append(e.asked, reactorID)
	return validMeleeDefinition(), ReactionSwing
}

// nobodySwings is the empty-handed caster: it answers, and the answer is no.
type nobodySwings struct{ asked []string }

func (n *nobodySwings) AttackFor(reactorID string) (combatActions.Definition, ReactionAnswer) {
	n.asked = append(n.asked, reactorID)
	return combatActions.Definition{}, ReactionNone
}

// reactorReactions is how many reactions a reactor in these scenes holds:
// TWO, so a bill that charged twice reads differently from one that charged
// once, and every billing assertion says how much was spent.
const reactorReactions = 2

// reactorSheet is a probe in a fight, with reactions on its meter for the door
// to bill.
func reactorSheet(id string) *character.Data {
	sheet := probeSheet(id)
	sheet.ActionEconomy = &character.ActionEconomyData{
		TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1, ReactionsRemaining: reactorReactions,
	}
	return sheet
}

// reactionsLeft reads a character's reaction meter off the sheets an output
// handed back; a sheet that came back clean was never billed.
func reactionsLeft(out *Output, id string) int {
	for _, sheet := range out.DirtyCharacters {
		if sheet.ID == id && sheet.ActionEconomy != nil {
			return sheet.ActionEconomy.ReactionsRemaining
		}
	}
	return reactorReactions
}

type MovementTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestMovementSuite(t *testing.T) { suite.Run(t, new(MovementTestSuite)) }

func (s *MovementTestSuite) SetupTest() { s.ctx = context.Background() }

func (s *MovementTestSuite) world() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
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
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Actors: encounter.Actors{
				Striker:   noAttacksExpected{},
				Mover:     encounter.RefusingMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

// runStep drives one movement interaction on a bus the test holds, so it can
// subscribe the way a condition would.
//
// reactors are the members in a fight: each gets one reaction to spend, which
// is also what lets the real opportunity attack condition offer them a swing.
// Everybody else carries no economy and so is never offered one.
func (s *MovementTestSuite) runStep(
	in *MovementInput, listen func(context.Context, events.EventBus), reactors ...string,
) (*Output, error) {
	machine, err := NewMovement(in)
	s.Require().NoError(err)

	return s.runMachine(machine, listen, reactors...)
}

// runMachine is [MovementTestSuite.runStep] over a machine already built — a
// resumed one, say — in the same world and cast.
func (s *MovementTestSuite) runMachine(
	machine Machine, listen func(context.Context, events.EventBus), reactors ...string,
) (*Output, error) {
	bus := events.NewEventBus()
	if listen != nil {
		listen(s.ctx, bus)
	}

	return resolveOn(s.ctx, &Input{
		World: s.world(),
		Participants: func() []Participant {
			fighting := map[string]bool{}
			for _, id := range reactors {
				fighting[id] = true
			}
			var cast []Participant
			for _, id := range []string{heroID, wolfID, "alice", "zara"} {
				sheet := probeSheet(id)
				if fighting[id] {
					sheet = reactorSheet(id)
				}
				cast = append(cast, Participant{Character: sheet})
			}
			return cast
		}(),
		Machine: machine,
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
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
	out, err := s.runStep(s.stepInput(), nil, heroID)
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

// TestAnOpportunityAttackSwingsAsOne: the reaction the movement machine runs is
// an opportunity attack, and its attack-roll frame says so — the fact Reckless
// Attack reads to refuse it. This machine is the flag's only production
// writer.
func (s *MovementTestSuite) TestAnOpportunityAttackSwingsAsOne() {
	var frames []contributions.Frame
	out, err := s.runStep(s.stepInput(), func(ctx context.Context, bus events.EventBus) {
		_, _ = dnd5eEvents.AttackChain.On(bus).SubscribeWithChain(ctx,
			func(_ context.Context, e dnd5eEvents.AttackChainEvent,
				c chain.Chain[dnd5eEvents.AttackChainEvent],
			) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
				frames = append(frames, e.Frame.Clone())
				return c, nil
			})
	}, heroID)
	s.Require().NoError(err)

	moved, ok := out.Outcome.(MovementOutcome)
	s.Require().True(ok)
	s.Require().Len(moved.Reactions, 1, "precondition: the opportunity attack swung")
	s.Require().Len(frames, 1)
	s.Equal(heroID, frames[0].Actor)
	s.Equal(contributions.Known(true), frames[0].Action.Opportunity)
}

// triggerFrom publishes a reaction trigger during the fold, which is exactly
// what OpportunityAttackCondition does when its predicate matches. It stands
// in for the condition where a scene needs a trigger from a member who is in
// no fight (no economy, so the real condition stays silent): the tests that
// decide whether to swing and never bill.
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

// THE BILL. The reaction is spent when it is TAKEN, through the one door,
// once a strike has actually resolved — exactly one reaction, off the
// reactor's own meter.
//
// It has to come from here because only here is the answer complete. The
// condition's trigger is published from a chain SUBSCRIBER, strictly before
// the fold's prevention flag is written and before the ReactionAttacks
// capability has been asked, so a condition that charged at publish time
// charged for every reaction those two went on to decline. The next three
// tests are exactly those declines (rpg-project#392 R1).
func (s *MovementTestSuite) TestAReactionThatSwingsIsBilledOnce() {
	out, err := s.runStep(s.stepInput(), nil, heroID)
	s.Require().NoError(err)
	moved := out.Outcome.(MovementOutcome)
	s.Require().Len(moved.Reactions, 1, "the swing is the thing being billed")

	s.Equal(reactorReactions-1, reactionsLeft(out, heroID), "one swing, exactly one reaction, off the reactor's meter")
	s.Equal(refs.Conditions.OpportunityAttack().String(), moved.Reactions[0].ConditionRef,
		"the reaction names its source, which the bill no longer carries")
}

// THE ALLY CASE, and the bug this ruling exists to fix. A reactor the caller
// declines to swing for — an ally the mover is not hostile to, an unarmed
// caster — is never billed, so a friend walking past a fighter no longer costs
// the fighter their reaction.
func (s *MovementTestSuite) TestARefusedReactorIsNeverBilled() {
	empty := &nobodySwings{}
	in := s.stepInput()
	in.Reactions = empty

	out, err := s.runStep(in, nil, heroID)
	s.Require().NoError(err)

	s.Require().Equal([]string{heroID}, empty.asked, "the capability was asked and said no")
	s.Empty(out.Outcome.(MovementOutcome).Reactions)
	s.Equal(reactorReactions, reactionsLeft(out, heroID), "a reaction the capability refused costs its reactor nothing")
}

// Disengage is free for the reactors it silences. A prevented opportunity
// attack is dropped before the capability is even asked, so there is nothing
// to bill.
func (s *MovementTestSuite) TestASuppressedStepBillsNobody() {
	out, err := s.runStep(s.stepInput(), preventOA(wolfID), heroID)
	s.Require().NoError(err)

	s.Equal(reactorReactions, reactionsLeft(out, heroID), "a suppressed reaction never happened, so nobody pays for it")
}

// Two reactors, two bills, each off its own reactor's meter. A bill that did
// not name its payer would spend the wrong member's reaction.
func (s *MovementTestSuite) TestEachReactorIsBilledForTheirOwnSwing() {
	out, err := s.runStep(s.stepInput(), nil, "zara", "alice")
	s.Require().NoError(err)

	s.Equal(reactorReactions-1, reactionsLeft(out, "alice"))
	s.Equal(reactorReactions-1, reactionsLeft(out, "zara"))
	s.Equal(reactorReactions, reactionsLeft(out, heroID), "nobody else's meter moves")
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

// A SUPPRESSED STEP PROVOKES NOTHING, which is the half of Disengage that was
// missing. The suppression is written from a chain STAGE and so lands during
// Execute; the opportunity attack's predicate is a SUBSCRIBER and runs strictly
// earlier, so it publishes its trigger into a fold that does not yet say
// "prevented" and cannot read it. This machine is the first place the answer is
// complete, so it is where the trigger is dropped — the division of labour
// conditions.OpportunityAttackCondition's own doc has always described
// (rpg-project#316).
//
// Reported AND enforced: OAPrevented still says so, and nothing swings.
func (s *MovementTestSuite) TestASuppressedStepProvokesNothing() {
	swings := &everyoneSwings{}
	in := s.stepInput()
	in.Reactions = swings

	out, err := s.runStep(in, func(ctx context.Context, bus events.EventBus) {
		triggerFrom(heroID, wolfID)(ctx, bus)
		preventOA(wolfID)(ctx, bus)
	})
	s.Require().NoError(err)

	moved := out.Outcome.(MovementOutcome)
	s.True(moved.OAPrevented, "the step still reports that it was suppressed")
	s.Empty(moved.Reactions, "a prevented opportunity attack does not roll")
	s.Empty(swings.asked, "and the capability is never even asked what it would swing")
}

// preventOA adds the chain stage Disengaging adds: opportunity attacks against
// mover are suppressed for this step.
func preventOA(mover string) func(context.Context, events.EventBus) {
	return func(ctx context.Context, bus events.EventBus) {
		_, _ = dnd5eEvents.MovementChain.On(bus).SubscribeWithChain(ctx,
			func(_ context.Context, _ *dnd5eEvents.MovementChainEvent,
				c chain.Chain[*dnd5eEvents.MovementChainEvent],
			) (chain.Chain[*dnd5eEvents.MovementChainEvent], error) {
				err := c.Add(combat.StageConditions, "test_disengage",
					func(_ context.Context, e *dnd5eEvents.MovementChainEvent) (*dnd5eEvents.MovementChainEvent, error) {
						e.OAPreventionSources = append(e.OAPreventionSources,
							dnd5eEvents.MovementModifierSource{
								Name: "Test Disengage", SourceType: "condition", EntityID: mover,
							})
						return e, nil
					})
				return c, err
			})
	}
}

// Identical inputs must produce identical stories (C8), so two reactors to one
// step are answered in a fixed order rather than in subscriber order.
func (s *MovementTestSuite) TestTwoReactorsAreAnsweredInADeterministicOrder() {
	swings := &everyoneSwings{}
	in := s.stepInput()
	in.Reactions = swings

	// Named deliberately in reverse-alphabetical order.
	out, err := s.runStep(in, nil, "zara", "alice")
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

// A FORCED STEP PROVOKES NOTHING, and the fold says what forced it.
//
// The push is the least permissive directive there is (rpg-project#431 §3):
// being thrown across a room is not walking out of somebody's reach, so the
// opportunity attack never fires. The suppression is seeded BEFORE the publish
// rather than written from a stage, because there is no condition here to
// write it — the cause is an effect resolution was handed, and the machine is
// the only thing that knows the step was not chosen.
func (s *MovementTestSuite) TestAForcedStepProvokesNothingAndTheFoldNamesWhatForcedIt() {
	swings := &everyoneSwings{}
	in := s.stepInput()
	in.Reactions = swings
	in.ForcedBy = refs.Spells.Thunderwave()

	var seen []dnd5eEvents.MovementChainEvent
	out, err := s.runStep(in, func(ctx context.Context, bus events.EventBus) {
		watchSteps(&seen)(ctx, bus)
		triggerFrom(heroID, wolfID)(ctx, bus)
	})
	s.Require().NoError(err)

	moved := out.Outcome.(MovementOutcome)
	s.True(moved.OAPrevented, "a shove reports its suppression exactly as Disengage does")
	s.Empty(moved.Reactions, "nothing swings at a creature that was pushed")
	s.Empty(swings.asked, "and the capability is never even asked what it would swing")

	s.Require().NotEmpty(seen, "the step is still announced; only the triggers are dropped")
	source := preventionSourceFor(seen[0].OAPreventionSources, refs.Spells.Thunderwave())
	s.Require().NotNil(source, "a subscriber can read WHAT forced the step, not merely that something did")
	s.Equal(refs.Spells.Thunderwave().String(), source.Name,
		"resolution holds no spell table, so the ref is the only honest name it can give (ADR-0045)")
	s.Equal("forced movement", source.SourceType)
	s.Equal(string(in.Mover), source.EntityID, "the source protects the mover's step, as Disengaging's does")
}

// preventionSourceFor finds the prevention source raised by one ref, or nil.
// By membership rather than by index: what matters is that the cause is in
// there, not that it is the only thing in there — a disengaging creature that
// is then shoved has two, and both are true.
func preventionSourceFor(
	sources []dnd5eEvents.MovementModifierSource, ref *core.Ref,
) *dnd5eEvents.MovementModifierSource {
	for i, source := range sources {
		if source.SourceRef != nil && source.SourceRef.String() == ref.String() {
			return &sources[i]
		}
	}

	return nil
}
