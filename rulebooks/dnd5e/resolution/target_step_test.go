// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// TargetStepTestSuite proves every damage source delivers through the one
// target step: a strike's blow and a contest's damage carry the same trace,
// the save's halving sits before the target's answers, and a target answer
// that rewrites what it was dealt is refused.
type TargetStepTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestTargetStepSuite(t *testing.T) {
	suite.Run(t, new(TargetStepTestSuite))
}

func (s *TargetStepTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *TargetStepTestSuite) fold() *ContestFoldTestSuite {
	fold := &ContestFoldTestSuite{ctx: s.ctx}
	fold.SetT(s.T())
	return fold
}

// swing is a weapon attack with no ability contribution: the only thing its
// damage depends on is the scripted dice.
func swing(dice string, damageType damage.Type) combatActions.Definition {
	definition := claw(dice)
	definition.Attack.Damage = []damage.Damage{{Dice: dice, Type: damageType}}
	return definition
}

// strike resolves one swing on the given bus (nil for a fresh one).
func (s *TargetStepTestSuite) strike(
	attackerID, targetID string, definition combatActions.Definition, faces []int,
	bus events.EventBus, participants ...Participant,
) (*Output, error) {
	if bus == nil {
		bus = events.NewEventBus()
	}
	return resolveOn(s.ctx, &Input{
		World:        s.fold().fixtures().world(),
		Participants: participants,
		Machine: NewStrike(&StrikeInput{
			AttackerID: attackerID, TargetID: targetID, Definition: definition,
			Roller: &sequenceRoller{singles: []int{straightRoll}, pair: faces},
		}),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Roller:     dice.NewRoller(),
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Actors:     Actors,
		},
	}, newSurface(bus))
}

func (s *TargetStepTestSuite) struck(out *Output, err error) StrikeOutcome {
	s.Require().NoError(err)
	struck, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok, "a strike produces a StrikeOutcome")
	s.Require().True(struck.Hit)
	return struck
}

// traceLine is what one line of a trace is, stripped of what differs between
// a sword and a spell: the dealt dice are "dealt", and every other line is
// its label, the rule it names and its amount.
type traceLine struct {
	Role       string
	Rule       string
	DamageType damage.Type
	Amount     int
}

// shapeOf reads a trace into its lines, and fails if any line carries a raw
// multiplier — that never leaves resolution.
func (s *TargetStepTestSuite) shapeOf(trace []dnd5eEvents.DamageComponent) (lines []traceLine, total int) {
	for _, component := range trace {
		s.Nil(component.Multiplier, "a raw multiplier component never leaves resolution")
		total += component.Total()
		if component.Roll.Dice != nil {
			lines = append(lines, traceLine{Role: "dealt", DamageType: component.DamageType, Amount: component.Total()})
			continue
		}
		s.Require().NotNil(component.Roll.Source.Ref, "every answer line names its rule")
		lines = append(lines, traceLine{
			Role:       component.Roll.Source.Label,
			Rule:       component.Roll.Source.Ref.String(),
			DamageType: component.DamageType,
			Amount:     component.Total(),
		})
	}
	return lines, total
}

// THE DONE-WHEN. Fire on a fire-immune creature and slashing on a raging
// barbarian: a strike and a contest carry the same trace — the dealt dice and
// one named multiplier line for the type — and it totals what was taken.
func (s *TargetStepTestSuite) TestAStrikeAndAContestCarryTheSameTrace() {
	fold := s.fold()
	immune := func() Participant {
		return Participant{Monster: fold.creature(monstertraits.MustImmunityJSON(wolfID, damage.Fire))}
	}

	// Fire, 2d4 showing 3 and 3: six dealt, immunity takes all six.
	strikeImmune := s.struck(s.strike(heroID, wolfID, swing("2d4", damage.Fire), []int{3, 3}, nil,
		immune(), Participant{Character: fold.fixtures().saver(14)}))
	contestImmune := fold.dealt(fold.resolve(
		contest(wolfID, saves.Negated, []damage.Damage{{Dice: "2d4", Type: damage.Fire}}, straightRoll),
		immune(), Participant{Character: fold.fixtures().saver(14)}))

	wantImmune := []traceLine{
		{Role: "dealt", DamageType: damage.Fire, Amount: 6},
		{Role: "immune", Rule: refs.MonsterTraits.Immunity().String(), DamageType: damage.Fire, Amount: -6},
	}
	strikeLines, strikeTotal := s.shapeOf(strikeImmune.DamageComponents)
	contestLines, contestTotal := s.shapeOf(contestImmune.Components)
	s.Equal(wantImmune, strikeLines)
	s.Equal(wantImmune, contestLines)
	s.Equal(strikeImmune.Damage, strikeTotal, "the strike's trace totals what was taken")
	s.Equal(contestImmune.Amount, contestTotal, "the contest's trace totals what was taken")
	s.Zero(strikeTotal)

	// Slashing, 2d4 showing 3 and 3: six dealt, Rage resists three.
	strikeRaging := s.struck(s.strike(wolfID, heroID, swing("2d4", damage.Slashing), []int{3, 3}, nil,
		Participant{Character: fold.fixtures().saver(14, fold.raging())},
		Participant{Monster: fold.fixtures().wolfData()}))
	contestRaging := fold.dealt(fold.resolve(
		contest(heroID, saves.Negated, []damage.Damage{{Dice: "2d4", Type: damage.Slashing}}, straightRoll),
		Participant{Character: fold.fixtures().saver(14, fold.raging())},
		Participant{Monster: fold.fixtures().wolfData()}))

	wantRaging := []traceLine{
		{Role: "dealt", DamageType: damage.Slashing, Amount: 6},
		{Role: "resisted", Rule: refs.Conditions.Raging().String(), DamageType: damage.Slashing, Amount: -3},
	}
	strikeLines, strikeTotal = s.shapeOf(strikeRaging.DamageComponents)
	contestLines, contestTotal = s.shapeOf(contestRaging.Components)
	s.Equal(wantRaging, strikeLines)
	s.Equal(wantRaging, contestLines)
	s.Equal(3, strikeRaging.Damage)
	s.Equal(strikeRaging.Damage, strikeTotal)
	s.Equal(contestRaging.Amount, contestTotal)
}

// THE ORDER. A made save against Half on a raging barbarian halves the rolled
// nine to four, and Rage resists the halved four to two. Totals cannot show
// the order — halving and resisting commute under rounding down — so the
// lines do: the halving is half of the NINE, and the resistance is half of
// the FOUR, after it.
func (s *TargetStepTestSuite) TestAMadeSaveHalvesBeforeTheResistance() {
	fold := s.fold()
	outcome := fold.resolve(
		contest(heroID, saves.Half, []damage.Damage{{Dice: "3d4", Type: damage.Slashing}}, 20),
		Participant{Character: fold.fixtures().saver(14, fold.raging())},
		Participant{Monster: fold.fixtures().wolfData()},
	)
	s.Require().True(outcome.Succeeded)

	lines, total := s.shapeOf(fold.dealt(outcome).Components)
	s.Equal([]traceLine{
		{Role: "dealt", DamageType: damage.Slashing, Amount: 9},
		{Role: halvedBySaveLabel, Rule: refs.Conditions.Prone().String(), DamageType: damage.Slashing, Amount: -5},
		{Role: "resisted", Rule: refs.Conditions.Raging().String(), DamageType: damage.Slashing, Amount: -2},
	}, lines)
	s.Equal(2, total)
	s.Equal(2, fold.dealt(outcome).Amount)
}

// A target answer that rewrites what it was dealt is refused, and nothing is
// applied: a target answers by appending, never by handing back a different
// blow.
func (s *TargetStepTestSuite) TestAnAnswerThatAltersADealtComponentIsRefused() {
	bus := events.NewEventBus()
	_, err := dnd5eEvents.IncomingDamageChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, _ *dnd5eEvents.IncomingDamageEvent,
			c chain.Chain[*dnd5eEvents.IncomingDamageEvent],
		) (chain.Chain[*dnd5eEvents.IncomingDamageEvent], error) {
			return c, c.Add(combat.StageFinal, "rewrites the blow",
				func(_ context.Context, e *dnd5eEvents.IncomingDamageEvent) (*dnd5eEvents.IncomingDamageEvent, error) {
					dealt := e.Dealt()
					one := 1
					dealt[0].Roll.Modifier = &one
					return dnd5eEvents.NewIncomingDamageEvent(dnd5eEvents.IncomingDamageInput{
						TargetID: e.TargetID(), SourceID: e.SourceID(), Dealt: dealt, Frame: e.Frame(),
					})
				})
		})
	s.Require().NoError(err)

	fold := s.fold()
	out, err := s.strike(wolfID, heroID, swing("2d4", damage.Slashing), []int{3, 3}, bus,
		Participant{Character: fold.fixtures().saver(14)},
		Participant{Monster: fold.fixtures().wolfData()})
	s.Require().ErrorIs(err, dnd5eEvents.ErrTargetAnswerAltered)
	s.Nil(out, "nothing is applied or saved")
}

// stepDirect runs the target step itself on bus, over the wolf's 2d4 slashing
// (3 and 3) at the hero, and returns what the hero received.
func (s *TargetStepTestSuite) stepDirect(bus events.EventBus) (receivedDamage, error) {
	fixtures := s.fold().fixtures()
	surf := newSurface(bus)
	cast, err := attachAll(s.ctx, surf, &attachAllInput{
		Participants: []Participant{{Character: fixtures.saver(14)}, {Monster: fixtures.wolfData()}},
		Roller:       dice.NewRoller(),
	})
	s.Require().NoError(err)
	defer func() { s.Require().NoError(surf.teardown(s.ctx)) }()

	rolled, err := rollContestDamage(s.ctx, []damage.Damage{{Dice: "2d4", Type: damage.Slashing}},
		facedRoller{d20: straightRoll, other: psychicFace}, mockedCause(), mockeryName)
	s.Require().NoError(err)

	var received receivedDamage
	_, err = receiveDamage(s.ctx, bus, targetStepInput{
		Dealt: dnd5eEvents.NewDamageChainEvent(dnd5eEvents.DamageChainInput{
			AttackerID: wolfID, TargetID: heroID, Components: rolled,
			Frame: contestDamageFrame(cast, wolfID, heroID),
		}),
		Cast:     cast,
		Cause:    mockedCause(),
		Roller:   dice.NewRoller(),
		Received: func(r receivedDamage) { received = r },
		Then:     func(context.Context) (Step, error) { return Done{}, nil },
	})
	return received, err
}

// incomingHandler subscribes one handler to the incoming fold. publish runs
// on the event the step hands subscribers; modify is the handler's chain
// stage, nil for none.
func (s *TargetStepTestSuite) incomingHandler(
	bus events.EventBus, publish func(*dnd5eEvents.IncomingDamageEvent),
	modify func(context.Context, *dnd5eEvents.IncomingDamageEvent) (*dnd5eEvents.IncomingDamageEvent, error),
) {
	_, err := dnd5eEvents.IncomingDamageChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, e *dnd5eEvents.IncomingDamageEvent,
			c chain.Chain[*dnd5eEvents.IncomingDamageEvent],
		) (chain.Chain[*dnd5eEvents.IncomingDamageEvent], error) {
			if publish != nil {
				publish(e)
			}
			if modify == nil {
				return c, nil
			}
			return c, c.Add(combat.StageFinal, "test answer", modify)
		})
	s.Require().NoError(err)
}

// resistance is a resistance to slashing from Rage.
func resistance() dnd5eEvents.DamageMultiplier {
	return dnd5eEvents.DamageMultiplier{
		Category:   dnd5eEvents.DamageSourceCondition,
		Source:     dnd5eEvents.RollSource{Ref: refs.Conditions.Raging(), Name: "Raging"},
		DamageType: damage.Slashing,
		Factor:     dnd5eEvents.DamageFactorResistance,
	}
}

// reduction is a fixed reduction to slashing.
func reduction(amount int) dnd5eEvents.DamageReduction {
	return dnd5eEvents.DamageReduction{
		Category:   dnd5eEvents.DamageSourceCondition,
		Source:     dnd5eEvents.RollSource{Ref: refs.Conditions.Raging(), Name: "Raging"},
		DamageType: damage.Slashing,
		Modifier:   amount,
	}
}

// THE CLONE. A handler that writes on the event it was handed — rather than
// answering in the fold — changes a clone nobody settles from: the hero takes
// the six it was dealt. Were the step to publish what it sent, the sent
// event would gain the answer the fold never saw, and CheckUnaltered would
// refuse.
func (s *TargetStepTestSuite) TestAHandlerCannotRewriteWhatWasSent() {
	bus := events.NewEventBus()
	s.incomingHandler(bus, func(e *dnd5eEvents.IncomingDamageEvent) {
		e.Multipliers = append(e.Multipliers, resistance())
	}, nil)
	received, err := s.stepDirect(bus)
	s.Require().NoError(err)
	s.Equal(6, received.Requested, "the write landed on a clone")
	s.Equal(6, received.Applied.TotalDamage)
}

// A reduction is a line of its own, and a reduction that sinks its type
// below zero is floored back to it on a line named for that reduction: six
// dealt, ten reduced, four floored, nothing taken.
func (s *TargetStepTestSuite) TestAReductionFloorsItsTypeAtZero() {
	bus := events.NewEventBus()
	s.incomingHandler(bus, nil, func(_ context.Context, e *dnd5eEvents.IncomingDamageEvent) (*dnd5eEvents.IncomingDamageEvent, error) {
		e.Reductions = append(e.Reductions, reduction(-10))
		return e, nil
	})
	received, err := s.stepDirect(bus)
	s.Require().NoError(err)

	lines, total := s.shapeOf(received.Trace)
	raging := refs.Conditions.Raging().String()
	s.Equal([]traceLine{
		{Role: "dealt", DamageType: damage.Slashing, Amount: 6},
		{Role: reducedLabel, Rule: raging, DamageType: damage.Slashing, Amount: -10},
		{Role: flooredLabel, Rule: raging, DamageType: damage.Slashing, Amount: 4},
	}, lines)
	s.Zero(total)
	s.Zero(received.Requested)
	s.Zero(received.Applied.TotalDamage)
}

// A reduction that does not sink its type carries its own line and no floor:
// six dealt, two reduced, four taken.
func (s *TargetStepTestSuite) TestAReductionIsALineOfItsOwn() {
	bus := events.NewEventBus()
	s.incomingHandler(bus, nil, func(_ context.Context, e *dnd5eEvents.IncomingDamageEvent) (*dnd5eEvents.IncomingDamageEvent, error) {
		e.Reductions = append(e.Reductions, reduction(-2))
		return e, nil
	})
	received, err := s.stepDirect(bus)
	s.Require().NoError(err)

	lines, total := s.shapeOf(received.Trace)
	s.Equal([]traceLine{
		{Role: "dealt", DamageType: damage.Slashing, Amount: 6},
		{Role: reducedLabel, Rule: refs.Conditions.Raging().String(), DamageType: damage.Slashing, Amount: -2},
	}, lines)
	s.Equal(4, total)
	s.Equal(4, received.Applied.TotalDamage)
}

// A malformed answer is a rule that cannot answer, not a bad world or a bad
// action: the defect is the answering rule's.
func (s *TargetStepTestSuite) TestAMalformedAnswerIsARuleThatCannotAnswer() {
	bus := events.NewEventBus()
	s.incomingHandler(bus, nil, func(_ context.Context, e *dnd5eEvents.IncomingDamageEvent) (*dnd5eEvents.IncomingDamageEvent, error) {
		malformed := resistance()
		malformed.Factor = 0.25
		e.Multipliers = append(e.Multipliers, malformed)
		return e, nil
	})
	_, err := s.stepDirect(bus)
	s.Require().ErrorIs(err, dnd5eEvents.ErrMalformedTargetAnswer)
	s.Require().ErrorIs(err, contributions.ErrRuleCannotAnswer)
	s.NotErrorIs(err, ErrBadWorld)
	s.NotErrorIs(err, ErrBadAction)
}
