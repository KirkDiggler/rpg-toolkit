// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	mock_combat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/mock"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// IsDownSuite pins the standing question against real sheets: every state a
// test asserts on is reached by driving ApplyDamage (or the healing the sheet
// listens for), never by hand-setting hit points, so the answer is a read of
// what the sheet actually says.
type IsDownSuite struct {
	suite.Suite

	ctx       context.Context
	bus       events.EventBus
	ctrl      *gomock.Controller
	fighter   *character.Character
	skeleton  *monster.Monster
	fighterHP int
}

func TestIsDownSuite(t *testing.T) {
	suite.Run(t, new(IsDownSuite))
}

func (s *IsDownSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.ctrl = gomock.NewController(s.T())
	s.fighterHP = 12
	s.fighter = s.createFighter()
	s.skeleton = s.createSkeleton()
}

func (s *IsDownSuite) TearDownTest() {
	if s.fighter != nil {
		_ = s.fighter.Cleanup(s.ctx)
	}
	s.ctrl.Finish()
}

// createFighter loads a level 1 fighter attached to the bus, so the sheet
// listens for healing the way a live sheet does.
func (s *IsDownSuite) createFighter() *character.Character {
	data := &character.Data{
		ID:               "fighter-1",
		PlayerID:         "player-1",
		Name:             "Alice",
		Level:            1,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:    s.fighterHP,
		MaxHitPoints: s.fighterHP,
		ArmorClass:   16,
	}

	char, err := character.LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)
	s.Require().NotNil(char)

	return char
}

func (s *IsDownSuite) createSkeleton() *monster.Monster {
	return monster.New(monster.Config{
		ID:   "skeleton-1",
		Name: "Skeleton",
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 14,
			abilities.CON: 15,
			abilities.INT: 6,
			abilities.WIS: 8,
			abilities.CHA: 5,
		},
		AC: 13,
		HP: 13,
	})
}

// hit drives real damage through the combatant's own ApplyDamage.
func (s *IsDownSuite) hit(c combat.Combatant, amount int) *combat.ApplyDamageResult {
	return c.ApplyDamage(s.ctx, &combat.ApplyDamageInput{
		Instances: []combat.DamageInstance{{Amount: amount, Type: "slashing"}},
	})
}

// An untouched sheet is standing — for a character and for a monster.
func (s *IsDownSuite) TestUndamagedCombatantsAreStanding() {
	s.Require().Positive(s.fighter.GetHitPoints())
	s.Require().Positive(s.skeleton.GetHitPoints())

	s.False(combat.IsDown(s.fighter), "an undamaged character is standing")
	s.False(combat.IsDown(s.skeleton), "an undamaged monster is standing")
}

// Damage that leaves hit points behind leaves the combatant standing.
func (s *IsDownSuite) TestDamagedButNotAtZeroIsStanding() {
	result := s.hit(s.fighter, s.fighter.GetHitPoints()-1)
	s.Require().Positive(result.CurrentHP, "this hit must leave the fighter above zero")
	s.Require().False(result.DroppedToZero)

	s.False(combat.IsDown(s.fighter))
}

// Exactly-lethal damage: the sheet lands on zero and the combatant reports down.
func (s *IsDownSuite) TestCharacterAtZeroIsDown() {
	result := s.hit(s.fighter, s.fighter.GetHitPoints())
	s.Require().Zero(result.CurrentHP, "this hit must land the fighter exactly on zero")
	s.Require().True(result.DroppedToZero)

	s.True(combat.IsDown(s.fighter))
}

// The same question, asked of a monster: nothing about the answer is
// character-specific.
func (s *IsDownSuite) TestMonsterAtZeroIsDown() {
	result := s.hit(s.skeleton, s.skeleton.GetHitPoints())
	s.Require().Zero(result.CurrentHP)
	s.Require().True(result.DroppedToZero)

	s.True(combat.IsDown(s.skeleton))
}

// Overkill floors at zero on both real sheets, and down is still down.
func (s *IsDownSuite) TestOverkillIsDown() {
	result := s.hit(s.skeleton, s.skeleton.GetHitPoints()*3)
	s.Require().Zero(result.CurrentHP, "the sheet floors overkill at zero")

	s.True(combat.IsDown(s.skeleton))
}

// A combatant whose sheet reports below the floor is down. Neither real sheet
// can produce this state — both floor at zero in ApplyDamage — so the reading
// combatant is a stand-in for any future sheet that keeps negative hit points.
// It pins the answer as "at or below zero", not "exactly zero".
func (s *IsDownSuite) TestBelowZeroHitPointsIsDown() {
	belowFloor := mock_combat.NewMockCombatant(s.ctrl)
	belowFloor.EXPECT().GetHitPoints().Return(-7).AnyTimes()

	s.True(combat.IsDown(belowFloor))
}

// The standing question is a pull read, not a stored flag: heal the sheet the
// way the world heals it and the answer changes with the sheet.
func (s *IsDownSuite) TestHealedCombatantStandsAgain() {
	dropped := s.hit(s.fighter, s.fighter.GetHitPoints())
	s.Require().Zero(dropped.CurrentHP)
	s.Require().True(combat.IsDown(s.fighter), "the fighter must be down before healing")

	healing := dnd5eEvents.HealingReceivedTopic.On(s.bus)
	err := healing.Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: s.fighter.GetID(),
		Amount:   5,
		Roll:     5,
		Source:   "potion",
	})
	s.Require().NoError(err)
	s.Require().Positive(s.fighter.GetHitPoints(), "the healing must have reached the sheet")

	s.False(combat.IsDown(s.fighter), "a healed combatant is standing again")
}

// And the reverse: a healed combatant taken back to zero reports down again on
// the next ask. Any route to zero is noticed; nothing latches.
func (s *IsDownSuite) TestDownAgainAfterHealingAndAnotherHit() {
	s.hit(s.fighter, s.fighter.GetHitPoints())

	healing := dnd5eEvents.HealingReceivedTopic.On(s.bus)
	err := healing.Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: s.fighter.GetID(),
		Amount:   5,
		Roll:     5,
		Source:   "potion",
	})
	s.Require().NoError(err)
	s.Require().False(combat.IsDown(s.fighter), "the fighter must be standing before the second hit")

	again := s.hit(s.fighter, s.fighter.GetHitPoints())
	s.Require().Zero(again.CurrentHP)

	s.True(combat.IsDown(s.fighter))
}
