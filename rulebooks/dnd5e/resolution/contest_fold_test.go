// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// ContestFoldTestSuite proves a contest's damage folds on the damage chain the
// way a strike's does (rpg-toolkit#1965 tier 1 #1): the target's immunity,
// resistance and vulnerability reach spell damage, and the caster's
// weapon-only rules do not ride a save.
//
// Every number is stated, not compared: a fold that did nothing deals the
// dice, and the dice are scripted so that differs from every answer below.
type ContestFoldTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestContestFoldSuite(t *testing.T) {
	suite.Run(t, new(ContestFoldTestSuite))
}

func (s *ContestFoldTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// fixtures reuses the damage suite's world, sheets and rollers: this is the
// same contest, folded.
func (s *ContestFoldTestSuite) fixtures() *ContestDamageTestSuite {
	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(s.T())
	fixtures.ctx = s.ctx

	return fixtures
}

// creature is the monster in the wolf's place on the board, carrying the given
// traits, with WIS 10 so its save is the d20 alone.
func (s *ContestFoldTestSuite) creature(traits ...json.RawMessage) *monster.Data {
	data := s.fixtures().wolfData()
	data.AbilityScores = shared.AbilityScores{
		abilities.STR: 10, abilities.DEX: 10, abilities.CON: 10,
		abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 10,
	}
	data.Conditions = traits

	return data
}

func (s *ContestFoldTestSuite) raging() json.RawMessage {
	raw, err := (&conditions.RagingCondition{
		CharacterID: heroID, Source: "rage",
	}).ToJSON()
	s.Require().NoError(err)

	return raw
}

// contest builds a damage-only contest from the bard against the saver, gated
// on Wisdom with the given save effect. Every damage die shows psychicFace (3).
func contest(saverID string, onSuccess saves.SaveEffect, pools []damage.Damage, d20 int) Machine {
	gate := mockeryGate()
	gate.OnSuccess = onSuccess

	return NewContest(&ContestInput{
		Gate:       gate,
		SaverID:    saverID,
		Damage:     pools,
		SourceName: mockeryName,
		Cause:      mockedCause(),
		Roller:     facedRoller{d20: d20, other: psychicFace},
	})
}

func (s *ContestFoldTestSuite) resolve(machine Machine, participants ...Participant) ContestOutcome {
	out, err := s.resolveErr(machine, participants...)
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(ContestOutcome)
	s.Require().True(ok, "a contest produces a ContestOutcome")

	return outcome
}

func (s *ContestFoldTestSuite) resolveErr(machine Machine, participants ...Participant) (*Output, error) {
	return Resolve(s.ctx, &Input{
		World:        s.fixtures().world(),
		Participants: participants,
		Machine:      machine,
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
	})
}

// dealt is the one damage the contest imposed.
func (s *ContestFoldTestSuite) dealt(outcome ContestOutcome) ImposedEffect {
	for _, imposed := range outcome.Imposed {
		if imposed.Kind == ImposedDamage {
			return imposed
		}
	}
	s.Require().Fail("the contest imposed no damage")

	return ImposedEffect{}
}

// THE HEADLINE. A creature immune to poison takes nothing from a poison spell
// it failed to save against. Before the fold it took the dice.
func (s *ContestFoldTestSuite) TestAPoisonImmuneSaverTakesNothingFromASpell() {
	outcome := s.resolve(
		contest(wolfID, saves.Negated, []damage.Damage{{Dice: "2d4", Type: damage.Poison}}, straightRoll),
		Participant{Monster: s.creature(monstertraits.MustImmunityJSON(wolfID, damage.Poison))},
		Participant{Character: s.fixtures().saver(14)},
	)
	s.Require().False(outcome.Succeeded, "+0 on a 3 is 3 against DC 13")

	dealt := s.dealt(outcome)
	s.Equal(0, dealt.Amount, "immunity: nothing gets through")
	s.Equal(0, dealt.Requested, "and the trace says why, rather than asking for six")
	s.Equal(11, dealt.After, "the creature's hit points did not move")

	s.Require().NotNil(dealt.Calculation)
	s.Equal(0, dealt.Calculation.Total)
	immune := componentLabelled(dealt.Calculation, "immune")
	s.Require().NotNil(immune, "the immunity is a line on the trace a record can show")
	s.Require().NotNil(immune.Modifier)
	s.Equal(-6, *immune.Modifier, "cancelling the six the dice rolled")
	s.Equal(refs.MonsterTraits.Immunity().String(), immune.Source.Ref.String())
}

// Rage resists bludgeoning whatever dealt it, a spell included.
func (s *ContestFoldTestSuite) TestARagingSaverTakesHalfASpellsBludgeoning() {
	outcome := s.resolve(
		contest(heroID, saves.Negated, []damage.Damage{{Dice: "2d4", Type: damage.Bludgeoning}}, straightRoll),
		Participant{Character: s.fixtures().saver(14, s.raging())},
		Participant{Monster: s.fixtures().wolfData()},
	)
	s.Require().False(outcome.Succeeded)

	dealt := s.dealt(outcome)
	s.Equal(3, dealt.Amount, "six bludgeoning, resisted to three")
	s.Equal(3, dealt.Requested)
	s.Equal(11, dealt.After, "14 - 3")

	resisted := componentLabelled(dealt.Calculation, "resisted")
	s.Require().NotNil(resisted, "the resistance is on the trace, named for the rule that resisted")
	s.Require().NotNil(resisted.Modifier)
	s.Equal(-3, *resisted.Modifier)
	s.Equal(refs.Conditions.Raging().String(), resisted.Source.Ref.String())
}

// THE ORDER. A made save against a Half gate halves the damage roll, and the
// target's multipliers apply after it — resistance and vulnerability come
// "after all other modifiers to damage". Vulnerability is the probe because
// halving and doubling do not commute under rounding down, where halving and
// resisting do: nine halved is four, doubled is eight; nine doubled is
// eighteen, halved is nine.
func (s *ContestFoldTestSuite) TestAHalfGateHalvesBeforeTheMultipliersApply() {
	outcome := s.resolve(
		contest(wolfID, saves.Half, []damage.Damage{{Dice: "3d4", Type: damage.Bludgeoning}}, 20),
		Participant{Monster: s.creature(monstertraits.MustVulnerabilityJSON(wolfID, damage.Bludgeoning))},
		Participant{Character: s.fixtures().saver(14)},
	)
	s.Require().True(outcome.Succeeded, "+0 on a 20 is 20 against DC 13")

	dealt := s.dealt(outcome)
	s.Equal(8, dealt.Amount, "nine, halved to four, doubled to eight")
	s.Equal(8, dealt.Requested)
	s.Equal(8, dealt.Calculation.Total)

	halving := componentLabelled(dealt.Calculation, halvedBySaveLabel)
	s.Require().NotNil(halving)
	s.Equal(-5, *halving.Modifier, "the half is taken of the rolled nine")
	vulnerable := componentLabelled(dealt.Calculation, "vulnerable")
	s.Require().NotNil(vulnerable)
	s.Equal(4, *vulnerable.Modifier, "and the doubling of the halved four")
}

// The caster's Sneak Attack is a weapon rule. It is asked by the same fold and
// answers DoesNotApply, because the frame KNOWS there is no weapon pool — an
// unknown pool would answer Depends and fail the fold.
//
// The caster holds a rogue level, because a Sneak Attack holder with no rogue
// levels is refused before the pool is read (rpg-project#538): the contest's
// frame carries the instigator's class levels from its own sheet, which is
// what lets the rule reach the pool question at all. (A bard/rogue multiclass
// would read better; the level record refuses multiclassing today, R2.4.)
func (s *ContestFoldTestSuite) TestTheCastersSneakAttackDoesNotRideASave() {
	sneak, err := conditions.NewSneakAttackCondition(conditions.SneakAttackInput{MemberID: bardID}).ToJSON()
	s.Require().NoError(err)
	bard := s.fixtures().bard(1)
	bard.ClassID = classes.Rogue
	bard.Conditions = []json.RawMessage{sneak}

	outcome := s.resolve(
		contest(heroID, saves.Negated, psychic(), straightRoll),
		Participant{Character: s.fixtures().saver(14)},
		Participant{Monster: s.fixtures().wolfData()},
		Participant{Character: bard},
	)

	dealt := s.dealt(outcome)
	s.Equal(psychicFace, dealt.Amount, "the d4 and nothing else")
	for _, component := range dealt.Components {
		if component.Roll.Source.Ref != nil {
			s.NotEqual(refs.Conditions.SneakAttack().String(), component.Roll.Source.Ref.String(),
				"Sneak Attack adds to weapon attacks, not to a save's damage")
			s.NotEqual(refs.Features.SneakAttack().String(), component.Roll.Source.Ref.String(),
				"under either of its refs")
		}
	}
}

// A contest's damage folds as somebody's. With no instigator there is no
// actor to frame it under, so it is refused at the door, before the dice.
func (s *ContestFoldTestSuite) TestContestDamageWithNoInstigatorIsRefusedAtTheDoor() {
	cause := mockedCause()
	cause.InstigatorID = ""
	machine := NewContest(&ContestInput{
		Gate:        mockeryGate(),
		SaverID:     heroID,
		Application: combatActions.ConditionApplication{},
		Damage:      psychic(),
		SourceName:  mockeryName,
		Cause:       cause,
		Roller:      facedRoller{d20: straightRoll, other: psychicFace},
	})

	_, err := s.resolveErr(machine,
		Participant{Character: s.fixtures().saver(14)},
		Participant{Monster: s.fixtures().wolfData()},
	)
	s.Require().ErrorIs(err, ErrBadAction)
	s.Require().ErrorContains(err, "instigator")
}

// The trace line names the multiplier the settlement decided by, not one
// whose truncated product happens to match. With a total of 1, a resistance
// folded before an immunity also reproduces 0 (int(1 * 0.5) == 0); the line
// must still read immune, sourced to the immunity. Driven directly because
// today's content cannot put both on one creature.
func (s *ContestFoldTestSuite) TestAMultiplierLineNamesTheFactorThatWasApplied() {
	one := 1
	dealt := []dnd5eEvents.DamageComponent{{
		Source: dnd5eEvents.DamageSourceSpell,
		Roll: dnd5eEvents.RollComponent{
			Source:   dnd5eEvents.RollSource{Ref: refs.Spells.DissonantWhispers(), Name: mockeryName},
			Modifier: &one,
		},
		DamageType: damage.Bludgeoning,
	}}
	settlement, err := combat.SettleDamage(&combat.SettleDamageInput{
		Dealt: dealt,
		Multipliers: []dnd5eEvents.DamageMultiplier{
			{
				Category:   dnd5eEvents.DamageSourceCondition,
				Source:     dnd5eEvents.RollSource{Ref: refs.Conditions.Raging(), Name: "Raging"},
				DamageType: damage.Bludgeoning,
				Factor:     dnd5eEvents.DamageFactorResistance,
			},
			{
				Category:   dnd5eEvents.DamageSourceMonsterTrait,
				Source:     dnd5eEvents.RollSource{Ref: refs.MonsterTraits.Immunity(), Name: "Immunity"},
				DamageType: damage.Bludgeoning,
				Factor:     dnd5eEvents.DamageFactorImmunity,
			},
		},
	})
	s.Require().NoError(err)
	trace := receivedTrace(dealt, nil, settlement)
	s.Require().Len(trace, 2, "the dealt line and one multiplier line for the one type")

	line := trace[1]
	s.Equal("immune", line.Roll.Source.Label, "immunity is what the settlement applied")
	s.Equal(refs.MonsterTraits.Immunity().String(), line.Roll.Source.Ref.String())
	s.Require().NotNil(line.Roll.Modifier)
	s.Equal(-1, *line.Roll.Modifier)
	s.Nil(line.Multiplier, "a raw multiplier never leaves resolution")
}
