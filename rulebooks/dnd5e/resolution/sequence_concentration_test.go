// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// twoClawSequence is a multiattack over the suite's own claw: two plain
// swings, neither penalised. Disadvantage is left off on purpose — it is a
// different rule with its own tests, and a pair of d20 faces in the script
// here would only make the save rolls harder to read.
var twoClawSequence = combatActions.Definition{
	Ref:  core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "claw-multiattack"},
	Name: "Multiattack",
	Sequence: &combatActions.SequenceProfile{
		Steps: []combatActions.SequenceStep{
			{Action: *refs.Weapons.Greatsword()},
			{Action: *refs.Weapons.Greatsword()},
		},
	},
}

// multiattack runs the two-claw sequence at the hero on a bus the scene can
// listen to — [ConcentrationTestSuite.strike]'s sibling, one profile arm over.
func (s *ConcentrationTestSuite) multiattack(
	hero *character.Data, component combatActions.Definition, roller dice.Roller, bus events.EventBus,
) (*Output, error) {
	fixtures := s.fixtures()

	machine, err := NewAction(&ActionInput{
		Definition: twoClawSequence,
		AttackerID: wolfID,
		TargetID:   heroID,
		Components: []combatActions.Definition{twoClawSequence, component},
		Roller:     roller,
	})
	s.Require().NoError(err)

	return resolveOn(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		Equipment:    noHandsAreObserved{},
		World:        fixtures.world(),
		Participants: []Participant{{Character: hero}, {Monster: fixtures.wolfData()}},
		Machine:      machine,
	}, newSurface(bus))
}

func (s *ConcentrationTestSuite) sequenced(out *Output) SequenceOutcome {
	outcome, ok := out.Outcome.(SequenceOutcome)
	s.Require().True(ok, "a sequence produces a SequenceOutcome")

	return outcome
}

// TestEachSwingOfAMultiattackRunsItsOwnConcentrationCheck is Kirk's ruling
// (2026-09-20) as a test: a check is a roll made against ONE blow, so two
// blows against a concentrating defender are two checks, in order, each
// recorded on the swing that forced it.
//
// The saves are scripted to be MADE, so the hold survives both and the scene
// is about the pairing rather than about the break.
func (s *ConcentrationTestSuite) TestEachSwingOfAMultiattackRunsItsOwnConcentrationCheck() {
	// Per swing: the claw's d20, its damage, then the save's d20. CON +2
	// against the DC 10 floor is made on a 15 and on an 18, so the hold
	// survives to be checked a second time.
	roller := &sequenceRoller{
		singles: []int{straightRoll, 15, straightRoll, 18},
		pair:    []int{6, 6},
	}

	out, err := s.multiattack(
		s.fixtures().saver(40, s.holding(heroID, wolfID)...),
		claw("1d6"), roller, events.NewEventBus(),
	)
	s.Require().NoError(err)

	sequence := s.sequenced(out)
	s.Require().Len(sequence.Steps, 2, "a miss-free multiattack swings twice")

	for index, step := range sequence.Steps {
		s.Require().Len(step.Strike.FollowUps, 1, "swing %d forced one check", index)
		s.Equal(heroID, step.Strike.FollowUps[0].SaverID, "swing %d", index)
		s.Equal(abilities.CON, step.Strike.FollowUps[0].Ability, "swing %d", index)

		s.Require().Len(step.ConcentrationChecks, 1,
			"swing %d records the check IT forced, not the action's", index)
		s.Equal(refs.Spells.TrueStrike().String(), step.ConcentrationChecks[0].Spell.Ref, "swing %d", index)
		s.True(step.ConcentrationChecks[0].Save.Succeeded, "swing %d", index)
		s.Empty(step.ConcentrationBreaks, "swing %d was survived", index)
	}

	// THE ORDER IS THE POINT, and two different faces are what make it
	// visible: a record that folded both onto one beat, or swapped them,
	// could not produce 17 then 20.
	s.Equal(17, sequence.Steps[0].ConcentrationChecks[0].Save.Total, "15 + CON 2, against the first swing")
	s.Equal(20, sequence.Steps[1].ConcentrationChecks[0].Save.Total, "18 + CON 2, against the second")

	// And the interaction-level lists are EMPTY, so a consumer cannot read
	// the same save twice by reading both places.
	s.Empty(out.ConcentrationChecks, "a sequence's checks ride its steps")
	s.Empty(out.ConcentrationBreaks)
}

// TestAHoldBrokenByTheFirstSwingIsGoneBeforeTheSecondLands is the other half
// of the ruling, and the roller is the assertion: only three single faces are
// scripted — two claws and ONE save — so a machine that asked a broken hold
// for a second check would exhaust the script and fail rather than roll
// something plausible.
func (s *ConcentrationTestSuite) TestAHoldBrokenByTheFirstSwingIsGoneBeforeTheSecondLands() {
	bus := events.NewEventBus()
	removals := s.removalLog(bus)

	roller := &sequenceRoller{
		singles: []int{straightRoll, straightRoll, straightRoll},
		pair:    []int{6, 6},
	}

	out, err := s.multiattack(
		s.fixtures().saver(40, s.holding(heroID, wolfID)...),
		claw("1d6"), roller, bus,
	)
	s.Require().NoError(err)

	sequence := s.sequenced(out)
	s.Require().Len(sequence.Steps, 2, "the defender is still standing, so both swings land")

	first := sequence.Steps[0]
	s.Require().Len(first.Strike.FollowUps, 1)
	s.False(first.Strike.FollowUps[0].Save.Result.Success, "3 + CON 2 does not reach the DC 10 floor")
	s.Empty(first.ConcentrationChecks, "a check that failed is recorded as the break it caused")
	s.Require().Len(first.ConcentrationBreaks, 1, "the hold ended on the swing that ended it")

	broke := first.ConcentrationBreaks[0]
	s.Equal(encounter.MemberID(heroID), broke.Caster)
	s.Equal(refs.Spells.TrueStrike().String(), broke.Spell.Ref)
	s.Equal(conditions.ConcentrationEndedDamage, broke.Reason)
	s.Require().NotNil(broke.Save, "a damage break carries the check it failed")
	s.False(broke.Save.Succeeded)

	// THE SECOND SWING HAPPENED AND FOUND NOTHING TO CHECK. That is what
	// "the break is visible before the second swing resolves" means: the hold
	// was already off the sheet, so no follow-up was produced and no face was
	// drawn for one.
	second := sequence.Steps[1]
	s.Empty(second.Strike.FollowUps, "there was no hold left to check")
	s.Empty(second.ConcentrationChecks)
	s.Empty(second.ConcentrationBreaks, "one break, on one swing, recorded once")
	s.Empty(roller.singles, "the script was spent exactly: two claws and ONE save")

	// The strip itself, in publication order, before the second swing's own
	// damage went anywhere.
	s.Require().Len(*removals, 2)
	s.Equal(concentratingAddress(heroID).ConditionRef, (*removals)[0].ConditionRef)
	s.Equal(trueStrikeAddress(heroID).ConditionRef, (*removals)[1].ConditionRef)

	s.Empty(out.ConcentrationChecks, "a sequence's record rides its steps")
	s.Empty(out.ConcentrationBreaks)
}
