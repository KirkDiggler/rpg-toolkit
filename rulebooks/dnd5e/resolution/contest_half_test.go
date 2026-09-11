// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// ContestHalfTestSuite drives what a MADE save costs, which until this slice
// was nothing at all.
//
// A gate that says Half is a gate whose success branch still delivers: half the
// damage, rounded down, through the same report and the same follow-ups the
// failure branch runs. The halving is a COMPONENT of the roll rather than a
// flag on it, so the trace that reaches a record explains the number it carries
// and every guard between here and the wire holds by construction.
type ContestHalfTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestContestHalfSuite(t *testing.T) {
	suite.Run(t, new(ContestHalfTestSuite))
}

func (s *ContestHalfTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// fixtures reuses the damage suite's world, sheets and rollers, for the reason
// the move suite does: this is another branch of the same contest, not a
// second interaction.
func (s *ContestHalfTestSuite) fixtures() *ContestDamageTestSuite {
	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(s.T())
	fixtures.ctx = s.ctx

	return fixtures
}

// holds reaches the concentration suite's fixture for a sheet already holding
// a spell, so the scene that proves a made save still owes a check does not
// re-derive the hold's JSON.
func (s *ContestHalfTestSuite) holds() *ConcentrationTestSuite {
	fixtures := &ConcentrationTestSuite{}
	fixtures.SetT(s.T())
	fixtures.ctx = s.ctx

	return fixtures
}

const (
	// whisperName is what the player reads on the damage roll, and what the
	// halving component is sourced to.
	whisperName = "Dissonant Whispers"

	// whisperDice is the pool, scripted below as 4, 5 and 4 so the sum is odd
	// and the rounding is visible.
	whisperDice = "3d6"
)

// whisperProfile is a damage-only cast gated on Wisdom with whatever save
// effect the scene declares.
//
// Built here rather than read from the content table for the reason every other
// profile in this package's tests is (ADR-0045): resolution must not know WHICH
// spell it holds. What it reads is a gate, a pool and a word.
func whisperProfile(onSuccess saves.SaveEffect) *combatActions.CastProfile {
	return &combatActions.CastProfile{
		RangeFeet:  60,
		Target:     combatActions.CastTargetOneCreature,
		MinTargets: 1,
		MaxTargets: 1,
		Save: &saves.SaveGate{
			Abilities:  []abilities.Ability{abilities.WIS},
			DC:         saves.DCStatic(spellSaveDC),
			OnSuccess:  onSuccess,
			Recurrence: saves.RecurrenceNone,
		},
		Damage: []damage.Damage{{Dice: whisperDice, Type: damage.Psychic}},
	}
}

func whisperDefinition(onSuccess saves.SaveEffect) combatActions.Definition {
	return combatActions.Definition{
		Ref:  *refs.Spells.DissonantWhispers(),
		Name: whisperName,
		Cost: oneAction(),
		Cast: whisperProfile(onSuccess),
	}
}

// whisper builds the machine the cast door builds, with the save scripted to
// the given d20 and the pool to the given faces.
func (s *ContestHalfTestSuite) whisper(onSuccess saves.SaveEffect, d20 int, faces ...int) Machine {
	machine, err := NewAction(&ActionInput{
		Definition: whisperDefinition(onSuccess),
		AttackerID: bardID,
		TargetIDs:  []string{heroID},
		Roller:     &sequenceRoller{singles: []int{d20}, pair: faces},
	})
	s.Require().NoError(err)

	return machine
}

func (s *ContestHalfTestSuite) castOutcome(out *Output) CastOutcome {
	outcome, ok := out.Outcome.(CastOutcome)
	s.Require().True(ok, "a cast produces a CastOutcome")

	return outcome
}

// componentLabelled finds the one roll component carrying the given label,
// which is how the halving is addressed rather than by its index: the trace's
// SHAPE is what this suite asserts, and an index would pin its length too.
func componentLabelled(
	calculation *dnd5eEvents.RollCalculation, label string,
) *dnd5eEvents.RollComponent {
	for i := range calculation.Components {
		if calculation.Components[i].Source.Label == label {
			return &calculation.Components[i]
		}
	}

	return nil
}

// dirtied names the sheets that came back to be saved, which is how a scene
// asserts that a delivery of nothing touched nobody: the caster is always
// dirty here, because casting costs an action.
func dirtied(out *Output) []string {
	ids := make([]string, 0, len(out.DirtyCharacters))
	for _, data := range out.DirtyCharacters {
		ids = append(ids, data.ID)
	}

	return ids
}

// diceSubtotal is everything the dice in a trace came to, which is the number
// the halving is measured against.
func diceSubtotal(calculation *dnd5eEvents.RollCalculation) int {
	total := 0
	for _, component := range calculation.Components {
		if component.Dice != nil {
			total += component.Dice.Subtotal
		}
	}

	return total
}

// THE HEADLINE. A made save against a Half gate is not a negation: it is half
// the damage, rounded down, with the halving visible in the trace as one more
// additive component the total is built from.
func (s *ContestHalfTestSuite) TestAMadeSaveAgainstAHalfGateDealsHalfRoundedDown() {
	fixtures := s.fixtures()

	out, err := fixtures.resolve(
		fixtures.saver(14), s.whisper(saves.Half, advantageRoll, 4, 5, 4), castCost(), fixtures.bard(1))
	s.Require().NoError(err)

	target := s.castOutcome(out).Targets[0]
	s.Require().NotNil(target.Save)
	s.Require().True(target.Save.Succeeded, "WIS +1 on an 18 is 19 against DC 13")
	s.Equal([]ImposedEffectKind{ImposedDamage}, kindsOf(target.Applied),
		"a made save still costs the damage, and nothing else")

	dealt := target.Applied[0]
	s.Equal(6, dealt.Amount, "thirteen halved and rounded down")
	s.Equal(6, dealt.Requested, "and the dice agree with what the sheet took")
	s.Require().NotNil(dealt.Calculation)
	s.Equal(6, dealt.Calculation.Total)
	s.Equal(13, diceSubtotal(dealt.Calculation), "the faces are still the faces that were rolled")

	halving := componentLabelled(dealt.Calculation, halvedBySaveLabel)
	s.Require().NotNil(halving, "the halving is a component the trace carries, not a flag on it")
	s.Require().NotNil(halving.Modifier)
	s.Equal(-7, *halving.Modifier, "the rounded half's complement, so the total IS the half")
	s.Nil(halving.Dice, "it subtracts nothing physical: no die was rerolled or dropped")
	s.Require().NotNil(halving.Source.Ref)
	s.Equal(refs.Spells.DissonantWhispers().String(), halving.Source.Ref.String(),
		"sourced to the spell that halved it")
	s.Equal(whisperName, halving.Source.Name)

	s.Require().NoError(dnd5eEvents.ValidateRollCalculation(dealt.Calculation),
		"the rulebook's own validator is what a record will run")
	s.Equal(8, fixtures.sheet(out, heroID).HitPoints, "14 - 6, applied exactly once")
}

// The discriminator. Same machine, same dice, one different word on the gate —
// and a Negated gate still delivers nothing on a success.
func (s *ContestHalfTestSuite) TestAMadeSaveAgainstANegatedGateStillDeliversNothing() {
	fixtures := s.fixtures()

	out, err := fixtures.resolve(
		fixtures.saver(14), s.whisper(saves.Negated, advantageRoll, 4, 5, 4), castCost(), fixtures.bard(1))
	s.Require().NoError(err)

	target := s.castOutcome(out).Targets[0]
	s.Require().NotNil(target.Save)
	s.Require().True(target.Save.Succeeded)
	s.Empty(target.Applied, "negated is still negated: a made save costs nothing at all")
	s.NotContains(dirtied(out), heroID, "and the saver's sheet was never written to")
}

// The scene that fails if the success branch returns Done instead of running
// the same continuation the failure branch does. Half is still damage taken,
// and damage taken is still what a concentrating creature answers for.
func (s *ContestHalfTestSuite) TestHalfDamageStillOwesAConcentrationCheck() {
	fixtures := s.fixtures()
	holds := s.holds()

	machine, err := NewAction(&ActionInput{
		Definition: whisperDefinition(saves.Half),
		AttackerID: bardID,
		TargetIDs:  []string{heroID},
		// singles: the spell's own save, which the hero MAKES, then the
		// concentration check, which the hero also makes. pair: the 3d6.
		Roller: &sequenceRoller{singles: []int{advantageRoll, 18}, pair: []int{4, 5, 4}},
	})
	s.Require().NoError(err)

	out, err := resolveOn(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		Equipment: noHandsAreObserved{},
		World:     fixtures.world(),
		Participants: []Participant{
			{Character: fixtures.saver(40, holds.holding(heroID, wolfID)...)},
			{Monster: fixtures.wolfData()},
			{Character: fixtures.bard(1)},
		},
		Machine: machine,
		Cost:    castCost(),
	}, newSurface(events.NewEventBus()))
	s.Require().NoError(err)

	outcome := s.castOutcome(out)
	s.Require().NotNil(outcome.Targets[0].Save)
	s.Require().True(outcome.Targets[0].Save.Succeeded, "the save was MADE, and half still landed")

	s.Require().Len(outcome.FollowUps, 1, "half the damage is still damage taken")
	followUp := outcome.FollowUps[0]
	s.Equal(heroID, followUp.SaverID)
	s.Equal(abilities.CON, followUp.Ability)
	s.Require().NotNil(followUp.Save.Result)
	s.Equal(conditions.ConcentrationDCFloor, followUp.Save.Result.DC, "six psychic asks for the floor")
	s.True(followUp.Save.Result.Success)
	s.Require().Len(out.ConcentrationChecks, 1, "and the roll that kept it is the record")
}

// Half of a condition means nothing, so a contest that would deliver one is
// refused at the door rather than resolved into something nobody declared.
//
// Driven at the contest rather than through a profile because content's own
// validator refuses the same combination first; this pins that the contest
// does not depend on having been reached through one.
func (s *ContestHalfTestSuite) TestAHalfGateWithAConditionIsRefusedAtTheContest() {
	fixtures := s.fixtures()
	gate := mockeryGate()
	gate.OnSuccess = saves.Half

	_, err := fixtures.resolve(fixtures.saver(14), NewContest(&ContestInput{
		Gate:        gate,
		SaverID:     heroID,
		Application: prone(),
		Damage:      psychic(),
		SourceName:  mockeryName,
		Cause:       mockedCause(),
		Roller:      facedRoller{d20: advantageRoll, other: psychicFace},
	}), nil, nil)

	s.Require().ErrorIs(err, ErrBadGate)
	s.Contains(err.Error(), "damage only")
}

// The edge the design flagged and nobody had run: a pool small enough that its
// half rounds to nothing.
//
// It is a real zero rather than a refusal or a silent drop. The halving
// component cancels the die exactly, so the trace totals zero, combat.FinalDamage
// drops a group that nets zero, and the guard that the trace explains the number
// compares zero with zero and holds. The saver takes nothing and the record
// still shows the die that was rolled and why it came to nothing.
func (s *ContestHalfTestSuite) TestAPoolThatHalvesToNothingIsAnHonestZero() {
	fixtures := s.fixtures()
	definition := whisperDefinition(saves.Half)
	definition.Cast.Damage = []damage.Damage{{Dice: "1d6", Type: damage.Psychic}}

	machine, err := NewAction(&ActionInput{
		Definition: definition,
		AttackerID: bardID,
		TargetIDs:  []string{heroID},
		Roller:     &sequenceRoller{singles: []int{advantageRoll}, pair: []int{1}},
	})
	s.Require().NoError(err)

	out, err := fixtures.resolve(fixtures.saver(14), machine, castCost(), fixtures.bard(1))
	s.Require().NoError(err)

	target := s.castOutcome(out).Targets[0]
	s.Require().True(target.Save.Succeeded)
	s.Equal([]ImposedEffectKind{ImposedDamage}, kindsOf(target.Applied),
		"the delivery happened; what it delivered was nothing")

	dealt := target.Applied[0]
	s.Zero(dealt.Amount, "half of one, rounded down")
	s.Zero(dealt.Requested, "and the trace says the same, which is what keeps the guard true")
	s.Require().NotNil(dealt.Calculation)
	s.Zero(dealt.Calculation.Total)
	s.Equal(1, diceSubtotal(dealt.Calculation), "the die that was rolled is still on the record")
	s.NotContains(dirtied(out), heroID,
		"and nobody lost a hit point: zero instances reach the sheet, so it is never written to")
}
