// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombatType "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The door refuses a status its user already holds, before anything is charged
// (rpg-project#1131, slice 8c). The rule is the rules module's
// (features.RefuseHeldStatus); the door asks it for whoever handed the
// description out.

// removedOn counts the condition-removed facts a bus carries.
func (s *ActivationTestSuite) removedOn(bus events.EventBus) *int {
	count := 0
	_, err := dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, _ dnd5eEvents.ConditionRemovedEvent) error {
			count++
			return nil
		})
	s.Require().NoError(err)

	return &count
}

// TEST 3. When it is gone she can again.
func (s *ActivationTestSuite) TestAUserWhoNoLongerHoldsTheStatusRagesAgain() {
	definition := s.rageAtTheDoorDescription()
	saved := s.aBarbarianWhoRagedLastTurn(definition)
	saved.Conditions = nil

	out, err := s.ragesAtTheDoor(events.NewEventBus(), saved, definition, 2)

	s.Require().NoError(err)
	again := dirty(out, heroID)
	s.Equal([]string{refs.Conditions.Raging().String()}, conditionRefs(again))
	s.Zero(again.Resources[resources.RageCharges].Current)
}

// TEST 4. Only a status is asked: a raging user's heal on its user, and a
// description that is only its price, are not refused for the status she holds.
func (s *ActivationTestSuite) TestOnlyADescriptionThatPutsAStatusIsAskedWhetherItIsHeld() {
	raging, err := ragingForTest(s.T(), activationFighterID).ToJSON()
	s.Require().NoError(err)

	s.Run("a heal on its user", func() {
		data := s.fighterAt(1, 4, 12)
		data.Conditions = []json.RawMessage{raging}

		out, err := s.resolveEffect(data, s.secondWindDescription(data), events.NewEventBus(), s.wolfParticipant())

		s.Require().NoError(err, "a heal is not asked")
		s.Equal(11, dirty(out, activationFighterID).HitPoints)
	})

	s.Run("a description that is only its price", func() {
		data := s.surgeFighter()
		data.Conditions = []json.RawMessage{raging}
		definition := s.actionSurgeDescription(data)
		_, puts := definition.StatusOnUser()
		s.Require().False(puts)

		out, err := s.resolveEffect(data, definition, events.NewEventBus(), s.wolfParticipant())

		s.Require().NoError(err, "a price is not asked")
		s.Equal(2, dirty(out, activationFighterID).ActionEconomy.ActionsRemaining)
	})
}

const (
	raiderID      = "raider-1"
	raiderLets    = `[{"use":"rage","with":{"damage_bonus":2},"uses":{"max":2,"refills":{"on":["long rest"]}}}]`
	raiderStr     = 16
	raiderHP      = 20
	longswordRoll = 5
)

// raider is a creature whose stat block lets it rage with a damage bonus of 2
// and two uses, carrying a longsword, built by the rules' own FromTemplate; its
// description is the one the creature's own kit hands out.
func (s *ActivationTestSuite) raider() (*monster.Data, *combatActions.Definition) {
	built, err := monster.FromTemplate(&monster.FromTemplateInput{
		ID:  raiderID,
		Ref: &core.Ref{Module: "dnd5e", Type: "monsters", ID: "raider"},
		Template: monster.Template{
			Base:      refs.Monsters.Human(),
			Name:      "Raider",
			Abilities: map[abilities.Ability]int{abilities.STR: raiderStr},
			Actions:   []weapons.WeaponID{weapons.Longsword},
			Lets:      json.RawMessage(raiderLets),
		},
		Base: monsters.Human,
	})
	s.Require().NoError(err)
	granted, err := built.GrantedAbility(&monster.GrantedAbilityInput{Grant: "rage"})
	s.Require().NoError(err)
	data := built.ToData()
	data.HitPoints, data.MaxHitPoints = raiderHP, raiderHP

	return data, granted.Definition
}

func (s *ActivationTestSuite) raiderWorld() encounter.EncounterData {
	return s.world(
		encounter.MemberInput{ID: raiderID, Kind: encounter.KindMonster, Position: spatial.Position{X: 1, Y: 1}},
		encounter.MemberInput{ID: wolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 1}},
	)
}

func (s *ActivationTestSuite) raiderRages(
	data *monster.Data, definition *combatActions.Definition, bus events.EventBus, turn int,
) (*Output, error) {
	machine, err := NewAction(&ActionInput{Definition: *definition, AttackerID: raiderID})
	s.Require().NoError(err)
	cost := &Cost{PayerID: raiderID, Profile: definition.Cost, Turn: &Turn{Number: turn, Speed: 30}}

	return s.resolveOnBus(bus, machine, s.raiderWorld(), []Participant{{Monster: data}, s.wolfParticipant()}, cost)
}

func (s *ActivationTestSuite) dirtyRaider(out *Output) *monster.Data {
	for _, sheet := range out.DirtyMonsters {
		if sheet.ID == raiderID {
			return sheet
		}
	}
	s.FailNow("the raider came back clean")

	return nil
}

func raiderConditions(data *monster.Data) []string {
	out := make([]string, 0, len(data.Conditions))
	for _, raw := range data.Conditions {
		out = append(out, string(raw))
	}

	return out
}

// TEST 5. A creature at the door: the first use lands; the second, with its
// bonus action back, is refused as the barbarian's is, and its count is
// exactly what it was.
func (s *ActivationTestSuite) TestACreatureWhoAlreadyRagesIsRefusedItsRageAtTheDoor() {
	data, definition := s.raider()
	bus := events.NewEventBus()
	applied := s.appliedOn(bus)

	first, err := s.raiderRages(data, definition, bus, 1)

	s.Require().NoError(err)
	sheet := s.dirtyRaider(first)
	s.Equal(0, sheet.SlotsLeft(coreCombatType.ActionBonus), "its bonus action is spent")
	s.Equal(1, sheet.Left["rage"], "one of the two uses is spent")
	s.Equal([]appliedFact{{
		Target: raiderID, Source: dnd5eEvents.ConditionSourceFeature, Ref: refs.Conditions.Raging().String(),
	}}, *applied, "told as an activation's condition-applied")
	s.Require().Len(sheet.Conditions, 1)

	sheet.Economy = combat.NewActionEconomy()
	before := raiderConditions(sheet)
	bus = events.NewEventBus()
	applied = s.appliedOn(bus)
	removed := s.removedOn(bus)

	out, err := s.raiderRages(sheet, definition, bus, 2)

	s.Require().ErrorIs(err, ErrActivationRefused)
	s.Contains(err.Error(), "already raging")
	s.Nil(out)
	s.Equal(1, sheet.Left["rage"], "the count is unchanged by the refusal")
	s.Equal(1, sheet.SlotsLeft(coreCombatType.ActionBonus), "its bonus action is untouched")
	s.Equal(before, raiderConditions(sheet), "the same instance")
	s.Empty(*applied)
	s.Zero(*removed)
}

// TEST 6. A raging creature's status does what it says: its hit carries the
// damage_bonus, and a slashing hit on it is halved. A claim being checked.
func (s *ActivationTestSuite) TestARagingCreatureHitsHarderAndTakesSlashingHalved() {
	data, definition := s.raider()
	first, err := s.raiderRages(data, definition, events.NewEventBus(), 1)
	s.Require().NoError(err)
	raging := s.dirtyRaider(first)
	longsword := raging.Actions[0]

	swing := func(sheet *monster.Data) int {
		out, err := s.resolveOnBus(events.NewEventBus(), NewStrike(&StrikeInput{
			AttackerID: raiderID, TargetID: wolfID, Definition: longsword,
			Roller: &actionRoller{singles: []int{18}, damage: [][]int{{longswordRoll}}},
		}), s.raiderWorld(), []Participant{{Monster: sheet}, s.wolfParticipant()}, nil)
		s.Require().NoError(err)
		struck := out.Outcome.(StrikeOutcome)
		s.Require().True(struck.Hit)

		return struck.Damage
	}

	calm, _ := s.raider()
	withoutRage := swing(calm)
	withRage := swing(raging)
	s.T().Logf("longsword damage: %d calm, %d raging (die %d, STR %d)", withoutRage, withRage, longswordRoll, raiderStr)
	s.Equal(withoutRage+2, withRage, "the raging status adds its damage_bonus of 2")

	slash := func(sheet *monster.Data) int {
		out, err := s.resolveOnBus(events.NewEventBus(), NewStrike(&StrikeInput{
			AttackerID: wolfID, TargetID: raiderID, Definition: claw("1d6"),
			Roller: &actionRoller{singles: []int{18}, damage: [][]int{{6}}},
		}), s.raiderWorld(), []Participant{{Monster: sheet}, s.wolfParticipant()}, nil)
		s.Require().NoError(err)
		struck := out.Outcome.(StrikeOutcome)
		s.Require().True(struck.Hit)

		return struck.Damage
	}
	s.T().Logf("slashing 6 on the raider: %d calm, %d raging", slash(calm), slash(raging))
	s.Equal(6, slash(calm), "unhurt by anything but the blow")
	s.Equal(3, slash(raging), "a slashing hit on a raging creature is halved")
}
