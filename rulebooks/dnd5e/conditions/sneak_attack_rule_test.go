// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type sneakAttackRuleSuite struct{ suite.Suite }

func TestSneakAttackRuleSuite(t *testing.T) { suite.Run(t, new(sneakAttackRuleSuite)) }

// rogueFrame is the rogue's shortsword attack on the goblin, with advantage
// known false and no pairs. Complete says whether the pairs are exhaustive.
func rogueFrame(complete bool) contributions.Frame {
	return contributions.Frame{
		Actor:  "rogue",
		Target: contributions.Known("goblin"),
		Action: contributions.ActionFacts{
			Roll:       contributions.Known(contributions.RollKindAttack),
			Ability:    contributions.Known(abilities.DEX),
			Melee:      contributions.Known(true),
			WeaponPool: contributions.Known(true),
			Advantage:  contributions.Known(false),
		},
		Complete: complete,
	}
}

func knownPair(from, to string, distance float64, stance contributions.Stance) contributions.PairFacts {
	return contributions.PairFacts{
		From: from, To: to,
		DistanceCells: contributions.Known(distance),
		Stance:        contributions.Known(stance),
	}
}

func (s *sneakAttackRuleSuite) assess(rule sneakAttackRule, frame contributions.Frame) contributions.Answer {
	out, err := rule.AssessAction(&contributions.AssessActionInput{Frame: frame})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Require().NoError(out.Answer.Decision.Validate())
	s.Equal(contributions.ContributesNow, out.Answer.Participation)
	if out.Answer.Decision.Applicability != contributions.Applies {
		s.Empty(out.Answer.Benefit)
	}
	return out.Answer
}

func rogueRule() sneakAttackRule { return sneakAttackRule{owner: "rogue", dice: 1} }

func (s *sneakAttackRuleSuite) TestSneakAttackRuleAppliesWithKnownAdvantage() {
	frame := rogueFrame(false)
	frame.Action.Advantage = contributions.Known(true)

	answer := s.assess(rogueRule(), frame)

	s.Equal(contributions.Applies, answer.Decision.Applicability)
	s.Equal("+1d6 damage", answer.Benefit)
}

func (s *sneakAttackRuleSuite) TestSneakAttackRuleAppliesWithEnemyOfTargetAdjacent() {
	frame := rogueFrame(false)
	frame.Pairs = []contributions.PairFacts{knownPair("goblin", "ally", 1.0, contributions.StanceHostile)}

	answer := s.assess(sneakAttackRule{owner: "rogue", dice: 3}, frame)

	s.Equal(contributions.Applies, answer.Decision.Applicability)
	s.Equal("Another enemy of the target is within 5 feet", answer.Decision.Reason)
	s.Equal("+3d6 damage", answer.Benefit)
}

func (s *sneakAttackRuleSuite) TestSneakAttackRuleNeutralAdjacentDoesNotQualify() {
	incomplete := rogueFrame(false)
	incomplete.Pairs = []contributions.PairFacts{knownPair("goblin", "npc", 1.0, contributions.StanceNeutral)}
	s.Equal(contributions.Depends, s.assess(rogueRule(), incomplete).Decision.Applicability)

	complete := rogueFrame(true)
	complete.Pairs = incomplete.Pairs
	answer := s.assess(rogueRule(), complete)
	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("No advantage and no other enemy of the target within 5 feet", answer.Decision.Reason)
}

func (s *sneakAttackRuleSuite) TestSneakAttackRuleIgnoresActorAdjacency() {
	frame := rogueFrame(true)
	frame.Pairs = []contributions.PairFacts{knownPair("goblin", "rogue", 1.0, contributions.StanceHostile)}

	answer := s.assess(rogueRule(), frame)

	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability,
		"the attacker beside its own target is not another enemy")
}

func (s *sneakAttackRuleSuite) TestSneakAttackRuleDependsWithoutProofWhenIncomplete() {
	for name, pairs := range map[string][]contributions.PairFacts{
		"no sightings": nil,
		"far enemy":    {knownPair("goblin", "ally", 3.0, contributions.StanceHostile)},
		"adjacent, stance unknown": {{
			From: "goblin", To: "ally", DistanceCells: contributions.Known(1.0),
		}},
	} {
		frame := rogueFrame(false)
		frame.Pairs = pairs
		answer := s.assess(rogueRule(), frame)
		s.Equal(contributions.Depends, answer.Decision.Applicability, name)
		s.Equal("Needs advantage or another enemy of the target within 5 feet", answer.Decision.Reason, name)
	}

	unknownAdvantage := rogueFrame(true)
	unknownAdvantage.Action.Advantage = contributions.Unknown[bool]()
	s.Equal(contributions.Depends, s.assess(rogueRule(), unknownAdvantage).Decision.Applicability,
		"an unknown advantage is never read as false")

	openPair := rogueFrame(true)
	openPair.Pairs = []contributions.PairFacts{{
		From: "goblin", To: "ally", Stance: contributions.Known(contributions.StanceHostile),
	}}
	s.Equal(contributions.Depends, s.assess(rogueRule(), openPair).Decision.Applicability,
		"a hostile creature at an unknown distance could still qualify")
}

func (s *sneakAttackRuleSuite) TestSneakAttackRuleDoesNotApplyWhenCompleteWithoutAdvantageOrEnemy() {
	frame := rogueFrame(true)
	frame.Pairs = []contributions.PairFacts{
		knownPair("goblin", "ally", 1.0, contributions.StanceAllied),
		knownPair("goblin", "fighter", 2.0, contributions.StanceHostile),
	}

	answer := s.assess(rogueRule(), frame)

	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("No advantage and no other enemy of the target within 5 feet", answer.Decision.Reason)
}

func (s *sneakAttackRuleSuite) TestSneakAttackRuleUsedThisTurn() {
	frame := rogueFrame(true)
	frame.Action.Advantage = contributions.Known(true)

	answer := s.assess(sneakAttackRule{owner: "rogue", usedThisTurn: true, dice: 1}, frame)

	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Already used this turn", answer.Decision.Reason)
}

// The Dexterity test is rpg-toolkit#1929, kept on purpose: a finesse weapon
// swung with Strength should qualify by RAW and does not here.
func (s *sneakAttackRuleSuite) TestSneakAttackRuleKeepsDexOnlyDefect() {
	frame := rogueFrame(true)
	frame.Action.Ability = contributions.Known(abilities.STR)
	frame.Action.Advantage = contributions.Known(true)

	answer := s.assess(rogueRule(), frame)

	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Sneak Attack requires a Dexterity attack", answer.Decision.Reason)
}

func (s *sneakAttackRuleSuite) TestSneakAttackRuleDependsWithoutTarget() {
	frame := rogueFrame(false)
	frame.Target = contributions.Unknown[string]()
	frame.Action.Advantage = contributions.Unknown[bool]()

	answer := s.assess(rogueRule(), frame)

	s.Equal(contributions.Depends, answer.Decision.Applicability)
	s.Equal("Depends on the target", answer.Decision.Reason)
}

func (s *sneakAttackRuleSuite) TestSneakAttackRuleOtherActorAndNonWeapon() {
	other := rogueFrame(true)
	other.Actor = "fighter"
	s.Equal(contributions.DoesNotApply, s.assess(rogueRule(), other).Decision.Applicability)

	spell := rogueFrame(true)
	spell.Action.WeaponPool = contributions.Known(false)
	spell.Action.Advantage = contributions.Known(true)
	s.Equal(contributions.DoesNotApply, s.assess(rogueRule(), spell).Decision.Applicability)
}

func (s *sneakAttackRuleSuite) TestSneakAttackRuleRefusesInvalidFrame() {
	for _, in := range []*contributions.AssessActionInput{nil, {}} {
		out, err := rogueRule().AssessAction(in)
		s.Error(err)
		s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
		s.Nil(out)
	}
}

func (s *sneakAttackRuleSuite) TestSneakAttackSourceImportsNoGameContext() {
	file, err := parser.ParseFile(token.NewFileSet(), "sneak_attack.go", nil, parser.ImportsOnly)
	s.Require().NoError(err)
	for _, spec := range file.Imports {
		path, unquoteErr := strconv.Unquote(spec.Path.Value)
		s.Require().NoError(unquoteErr)
		s.NotContains(path, "gamectx", "Sneak Attack answers from the frame, not the game context")
		s.NotContains(path, "tools/spatial")
	}
}

// sneakAttackHandlerSuite drives the real damage chain with no game context.
type sneakAttackHandlerSuite struct {
	suite.Suite
	bus       events.EventBus
	condition *SneakAttackCondition
	roller    *countingRoller
}

// countingRoller rolls fixed faces and counts the dice it was asked for.
type countingRoller struct{ rollNCalls int }

func (r *countingRoller) Roll(context.Context, int) (int, error) { return 3, nil }

func (r *countingRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	r.rollNCalls++
	faces := make([]int, count)
	for i := range faces {
		faces[i] = 3
	}
	return faces, nil
}

func TestSneakAttackHandlerSuite(t *testing.T) { suite.Run(t, new(sneakAttackHandlerSuite)) }

func (s *sneakAttackHandlerSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.roller = &countingRoller{}
	s.condition = NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue", Level: 1, Roller: s.roller})
	s.Require().NoError(s.condition.Apply(context.Background(), s.bus))
}

func (s *sneakAttackHandlerSuite) fold(frame contributions.Frame) (*dnd5eEvents.DamageChainEvent, *dnd5eEvents.DamageChainEvent, error) {
	ctx := context.Background() // deliberately no room and no cast
	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: "rogue", TargetID: "goblin", WeaponDamageType: damage.Piercing,
		Components: []dnd5eEvents.DamageComponent{{
			Source:     dnd5eEvents.DamageSourceWeapon,
			Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			DamageType: damage.Piercing,
		}},
		Frame: frame,
	}
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.DamageChain.On(s.bus).PublishWithChain(ctx, event, chain)
	if err != nil {
		return event, nil, err
	}
	result, err := modified.Execute(ctx, event)
	return event, result, err
}

func (s *sneakAttackHandlerSuite) TestSneakAttackHandlerNeedsNoGameContext() {
	frame := rogueFrame(true)
	frame.Pairs = []contributions.PairFacts{knownPair("goblin", "fighter", 1.0, contributions.StanceHostile)}

	_, result, err := s.fold(frame)

	s.Require().NoError(err)
	s.Require().Len(result.Components, 2)
	s.Equal("Sneak Attack", result.Components[1].Roll.Source.Name)
	s.Equal([]int{3}, result.Components[1].Roll.Dice.FinalRolls)
	s.True(s.condition.UsedThisTurn)
	s.Equal(1, s.roller.rollNCalls)
}

func (s *sneakAttackHandlerSuite) TestSneakAttackHandlerNoOpWhenRuleDoesNotApply() {
	_, result, err := s.fold(rogueFrame(true))

	s.Require().NoError(err)
	s.Len(result.Components, 1)
	s.False(s.condition.UsedThisTurn)
	s.Zero(s.roller.rollNCalls)
}

func (s *sneakAttackHandlerSuite) TestSneakAttackHandlerFailsWhenRuleDepends() {
	event, result, err := s.fold(rogueFrame(false))

	s.Require().Error(err)
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
	s.Nil(result)
	s.Len(event.Components, 1)
	s.False(s.condition.UsedThisTurn, "a rule that cannot answer spends nothing")
	s.Zero(s.roller.rollNCalls)
}

func (s *sneakAttackHandlerSuite) TestSneakAttackHandlerRejectsZeroFrame() {
	_, _, err := s.fold(contributions.Frame{})

	s.Require().Error(err)
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
	s.False(s.condition.UsedThisTurn)
}
