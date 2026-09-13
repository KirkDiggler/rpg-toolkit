// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

func (s *CastActionTestSuite) TestFreeSpellStillRestrictsSameTurnReactionWithoutRefillingIt() {
	caster := baneCaster(1, 2)
	caster.ActionEconomy.BonusActionsRemaining = 1
	caster.ActionEconomy.ReactionsRemaining = 1
	d := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.HealingWord})
	roll := &countingCastRoller{facedRoller: facedRoller{other: 4}}
	out, err := s.spellAttempt(caster, d, &Cost{PayerID: bardID, SpellTurn: "run/1/bard"}, roll)
	s.Require().NoError(err)
	caster = s.fixtures().sheet(out, bardID)
	s.Equal(2, caster.Resources[resources.SpellSlotLevel1].Current, "nil price is free, not untracked")
	d.Cast.Casting.Time = combat.SpellCastingReaction
	price := &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionReaction: 1}}
	roll.calls = 0
	out, err = s.spellAttempt(caster, d, &Cost{PayerID: bardID, Profile: price, SpellTurn: "run/1/bard"}, roll)
	s.ErrorIs(err, combat.ErrBonusActionSpell)
	s.Nil(out)
	s.Zero(roll.calls)
	out, err = s.spellAttempt(caster, d, &Cost{PayerID: bardID, Profile: price, SpellTurn: "run/1/hero"}, roll)
	s.Require().NoError(err)
	caster = s.fixtures().sheet(out, bardID)
	s.Zero(caster.ActionEconomy.ReactionsRemaining)
	roll.calls = 0
	out, err = s.spellAttempt(caster, d, &Cost{PayerID: bardID, Profile: price, SpellTurn: "run/1/wolf"}, roll)
	s.ErrorIs(err, ErrCannotPay)
	s.Nil(out)
	s.Zero(roll.calls)
	s.Equal("run/1/hero", caster.ActionEconomy.Spellcasting.Turn, "failed payment must not record a cast")
}

func (s *CastActionTestSuite) TestRangedHealingProjectionIncludesSelfDyingAndUntypedRecipients() {
	f := s.fixtures()
	run, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: f.world(),
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encounter.RefusingAnnouncer{}})
	s.Require().NoError(err)
	room, err := run.Canvas()
	s.Require().NoError(err)
	wolf := f.wolfData()
	wolf.Ref = nil
	input := &RangedHealingTargetsInput{Encounter: run, RangeFeet: 60,
		HealingTargetsInput: HealingTargetsInput{Room: room, CasterID: bardID,
			Candidates:   []string{bardID, heroID, wolfID},
			Participants: []Participant{{Character: baneCaster(1, 2)}, {Character: f.saver(0)}, {Monster: wolf}}}}
	answers, err := RangedHealingTargets(s.ctx, input)
	s.Require().NoError(err)
	s.Equal(map[string]bool{bardID: true, heroID: true, wolfID: true}, answers)
	input.RangeFeet = 5
	answers, err = RangedHealingTargets(s.ctx, input)
	s.Require().NoError(err)
	s.False(answers[heroID])
	s.True(answers[bardID])
}

func (s *CastActionTestSuite) spellAttempt(caster *character.Data, definition *combatActions.Definition, cost *Cost, roll *countingCastRoller) (*Output, error) {
	f := s.fixtures()
	machine, err := NewAction(&ActionInput{Definition: *definition, AttackerID: bardID, TargetIDs: []string{bardID}, Roller: roll})
	s.Require().NoError(err)
	return Resolve(s.ctx, &Input{World: f.world(), Participants: []Participant{{Character: caster}, {Character: f.saver(14)}, {Monster: f.wolfData()}}, Machine: machine,
		Cost: cost, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Roller: dice.NewRoller()})
}

func (s *CastActionTestSuite) TestSpellRestrictionSurvivesReloadInBothOrders() {
	for _, pair := range [][2]spells.Spell{{spells.HealingWord, spells.CureWounds}, {spells.CureWounds, spells.HealingWord},
		{spells.HealingWord, spells.SacredFlame}, {spells.SacredFlame, spells.HealingWord}} {
		s.Run(string(pair[0])+" then "+string(pair[1]), func() {
			caster := baneCaster(1, 2)
			caster.ActionEconomy.BonusActionsRemaining = 1
			for index, id := range pair {
				ch, err := character.Load(s.ctx, caster)
				s.Require().NoError(err)
				d := ch.CastDefinition(id)
				roll := &countingCastRoller{facedRoller: facedRoller{d20: 20, other: 4}}
				before, err := json.Marshal(caster)
				s.Require().NoError(err)
				out, err := s.spellAttempt(caster, d, &Cost{PayerID: bardID, Profile: d.Cost, SpellTurn: "run/1/bard"}, roll)
				blocked := index == 1 && pair[0] != spells.SacredFlame && pair[1] != spells.SacredFlame
				if blocked {
					s.ErrorIs(err, combat.ErrBonusActionSpell)
					s.Nil(out)
					s.Zero(roll.calls)
				} else {
					s.Require().NoError(err)
					stored := s.fixtures().sheet(out, bardID)
					encoded, marshalErr := json.Marshal(stored)
					s.Require().NoError(marshalErr)
					var loaded character.Data
					s.Require().NoError(json.Unmarshal(encoded, &loaded))
					s.Equal("run/1/bard", loaded.ActionEconomy.Spellcasting.Turn)
					caster = &loaded
				}
				if blocked {
					after, marshalErr := json.Marshal(caster)
					s.Require().NoError(marshalErr)
					s.Equal(before, after)
				}
			}
		})
	}
}

func (s *CastActionTestSuite) TestCostedCastRequiresCompleteDeclaration() {
	for _, missing := range []string{"turn", "classification", "payer"} {
		s.Run(missing, func() {
			caster := baneCaster(1, 2)
			caster.ActionEconomy.BonusActionsRemaining = 1
			d := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.HealingWord})
			cost := &Cost{PayerID: bardID, Profile: d.Cost, SpellTurn: "turn"}
			switch missing {
			case "turn":
				cost.SpellTurn = ""
			case "classification":
				d.Cast.Casting = nil
			case "payer":
				cost.PayerID = heroID
			}
			roll := &countingCastRoller{facedRoller: facedRoller{other: 4}}
			out, err := s.spellAttempt(caster, d, cost, roll)
			s.ErrorIs(err, ErrBadCost)
			s.Nil(out)
			s.Zero(roll.calls)
			s.Equal(2, caster.Resources[resources.SpellSlotLevel1].Current)
		})
	}
}

func (s *CastActionTestSuite) TestHealingWordHealsDyingRecipientBeyondTouch() {
	f := s.fixtures()
	caster := baneCaster(1, 2)
	caster.ActionEconomy.BonusActionsRemaining = 1
	target := f.saver(0)
	ch, err := character.Load(s.ctx, caster)
	s.Require().NoError(err)
	definition := ch.CastDefinition(spells.HealingWord)
	roll := &countingCastRoller{facedRoller: facedRoller{other: 4}}
	machine, err := NewAction(&ActionInput{Definition: *definition, AttackerID: bardID, TargetIDs: []string{heroID}, Roller: roll})
	s.Require().NoError(err)
	out, err := Resolve(s.ctx, &Input{World: f.world(), Participants: []Participant{{Character: caster}, {Character: target}, {Monster: f.wolfData()}}, Machine: machine,
		Cost:       &Cost{SpellTurn: "scene/round-1/bard", PayerID: bardID, Profile: definition.Cost},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Roller: dice.NewRoller()})
	s.Require().NoError(err)
	heal := s.castOutcome(out).Targets[0].Applied[0]
	s.Equal(ImposedHealing, heal.Kind)
	s.Equal(7, heal.Amount)
	s.Equal(0, heal.Before)
	s.Equal(7, heal.After)
	s.Equal(1, roll.calls)
	paid := f.sheet(out, bardID)
	s.Equal(1, paid.ActionEconomy.ActionsRemaining)
	s.Zero(paid.ActionEconomy.BonusActionsRemaining)
	s.Equal(1, paid.Resources[resources.SpellSlotLevel1].Current)
	s.Equal(7, f.sheet(out, heroID).HitPoints)
	encoded, err := json.Marshal(out.Outcome)
	s.Require().NoError(err)
	s.Contains(string(encoded), "healing-word")
}

func (s *CastActionTestSuite) TestHealingWordRejectsUnseenOrOutOfRangeBeforePayment() {
	for _, reason := range []string{"stale sight", "out of range", "wall"} {
		s.Run(reason, func() {
			f := s.fixtures()
			world := f.world()
			caster := baneCaster(1, 2)
			caster.ActionEconomy.BonusActionsRemaining = 1
			ch, err := character.Load(s.ctx, caster)
			s.Require().NoError(err)
			definition := ch.CastDefinition(spells.HealingWord)
			switch reason {
			case "stale sight":
				// Persisted testimony still names channels: EncounterData's
				// field is perception.Data, whose own Intel is the store
				// underneath. perception.Holding collapsed CurrentVia into a
				// bool for READERS; the persistence shape kept the list.
				holding := world.Perception.Intel.Holdings[bardID][heroID]
				holding.CurrentVia = nil
				world.Perception.Intel.Holdings[bardID][heroID] = holding
			case "out of range":
				definition.Cast.RangeFeet = 5
			case "wall":
				world.Field.Walls = append(world.Field.Walls, encounter.BoundaryData{From: encounter.PositionData{X: 2, Y: 1}, To: encounter.PositionData{X: 3, Y: 1}, BlocksMovement: true, BlocksLineOfSight: true})
			}
			roll := &countingCastRoller{facedRoller: facedRoller{other: 4}}
			machine, err := NewAction(&ActionInput{Definition: *definition, AttackerID: bardID, TargetIDs: []string{heroID}, Roller: roll})
			s.Require().NoError(err)
			out, err := Resolve(s.ctx, &Input{World: world, Participants: []Participant{{Character: caster}, {Character: f.saver(1)}, {Monster: f.wolfData()}}, Machine: machine,
				Cost:       &Cost{SpellTurn: "scene/round-1/bard", PayerID: bardID, Profile: definition.Cost},
				Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Roller: dice.NewRoller()})
			s.ErrorIs(err, ErrOutOfRange)
			s.Nil(out)
			s.Zero(roll.calls)
			s.Equal(2, caster.Resources[resources.SpellSlotLevel1].Current)
			s.Equal(1, caster.ActionEconomy.BonusActionsRemaining)
		})
	}
}
