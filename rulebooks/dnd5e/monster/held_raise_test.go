// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type monsterHeldRaiseSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
	m   *monster.Monster
}

func TestMonsterHeldRaiseSuite(t *testing.T) { suite.Run(t, new(monsterHeldRaiseSuite)) }

func (s *monsterHeldRaiseSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	m, err := monster.Load(s.ctx, &monster.Data{ID: "ogre-1", Name: "Ogre", HitPoints: 20, MaxHitPoints: 20, ArmorClass: 11})
	s.Require().NoError(err)
	s.Require().NoError(m.SheetKeeper().Apply(s.ctx, s.bus))
	s.m = m
}

// raise gives the monster a recipient half holding one hit point maximum part.
func (s *monsterHeldRaiseSuite) raise(application string, amount string) {
	config := json.RawMessage(`{"application_id":"` + application + `","address_source_id":"` + application + `","role":"recipient",` +
		`"bindings":{"name":"Feasted","owner":{"member_id":"cook","condition_ref":"` + refs.Conditions.LinkedEffects().String() + `","source_id":"` + application + `"},` +
		`"member_key":"ogre-1","parts":[{"key":"feast","kind":"hit_point_maximum","on_terminal":"part","amount":{"fixed":` + amount + `}}]}}`)
	made, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.LinkedEffects().String(), MemberID: "ogre-1",
		Origin: &dnd5eEvents.ConditionOrigin{Ref: *refs.Conditions.LinkedEffects(), EntityID: "cook"}, Config: config})
	s.Require().NoError(err)
	s.Require().NoError(dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.ConditionAppliedEvent{Target: s.m, Condition: made.Condition}))
}

func (s *monsterHeldRaiseSuite) end(application string) {
	s.Require().NoError(dnd5eEvents.ConditionRemovedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID: "ogre-1", ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: application, ApplicationID: application, Reason: "run_ended"}))
}

// reload saves the monster and loads it onto a fresh bus the way a host does.
func (s *monsterHeldRaiseSuite) reload() {
	m, err := monstertraits.LoadMonster(s.ctx, s.m.ToData())
	s.Require().NoError(err)
	s.bus = events.NewEventBus()
	s.Require().NoError(monstertraits.AttachMonster(s.ctx, m, s.bus, nil))
	s.m = m
}

func (s *monsterHeldRaiseSuite) heal(amount int) {
	s.Require().NoError(dnd5eEvents.HealingReceivedTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.HealingReceivedEvent{TargetID: "ogre-1", Amount: amount, Source: "fixture"}))
}

// Catches a monster reader left on the stored field: the getter, MaxHP, the
// percent and the heal cap must each use the raised number, and gaining the
// raise must not heal.
func (s *monsterHeldRaiseSuite) TestARaiseShowsInEveryReader() {
	s.raise("application:feast", "5")
	s.Equal(25, s.m.GetMaxHitPoints())
	s.Equal(25, s.m.MaxHP())
	s.Equal(20, s.m.HP(), "gaining a raise does not heal")
	s.Equal(80, s.m.HPPercent(), "20 of 25")

	s.heal(50)
	s.Equal(25, s.m.HP(), "healing to full reaches the raised maximum, and no further")
	s.Equal(100, s.m.HPPercent())
}

// Catches a sum that keeps only one raise.
func (s *monsterHeldRaiseSuite) TestTwoRaisesAdd() {
	s.raise("application:feast", "2")
	s.raise("application:stew", "3")
	s.Equal(25, s.m.GetMaxHitPoints())
}

// Catches a removal that leaves current above the new maximum, lowers the
// base, or does not mark the sheet changed; and one that ends the other raise.
func (s *monsterHeldRaiseSuite) TestEndingARaiseCapsCurrentAndKeepsTheBase() {
	s.raise("application:feast", "2")
	s.raise("application:stew", "3")
	s.heal(5)
	s.Require().Equal(25, s.m.HP())
	s.reload()
	s.Require().False(s.m.IsDirty(), "a freshly loaded sheet is clean, so the mark below is the removal's")

	s.end("application:stew")
	s.Equal(22, s.m.GetMaxHitPoints(), "the other raise is still held")
	s.Equal(22, s.m.HP(), "current is capped at the new maximum")
	s.True(s.m.IsDirty())
	s.Equal(20, s.m.ToData().MaxHitPoints, "the base is unchanged")
}

// Catches a save that writes the effective maximum as the base, or a load
// that drops the raise.
func (s *monsterHeldRaiseSuite) TestSaveAndReloadKeepTheBaseAndTheRaise() {
	s.raise("application:feast", "2")
	s.heal(2)
	data := s.m.ToData()
	s.Equal(20, data.MaxHitPoints, "the stored number is the base")
	s.Equal(22, data.HitPoints)

	s.reload()
	s.Equal(22, s.m.GetMaxHitPoints(), "the raise is read again from the held condition")
	s.Equal(22, s.m.HP())
	s.Equal(20, s.m.ToData().MaxHitPoints)
}

// Catches a monster's Initiative that skips the rule, reads the wrong ability,
// or reads a stat block with no scores as Dexterity 0 (-5).
func (s *monsterHeldRaiseSuite) TestInitiativeAddsDexterityByRule() {
	quick, err := monster.Load(s.ctx, &monster.Data{ID: "wolf-1", Name: "Wolf", HitPoints: 11, MaxHitPoints: 11, ArmorClass: 13,
		AbilityScores: shared.AbilityScores{abilities.DEX: 15, abilities.STR: 12}})
	s.Require().NoError(err)
	out, err := quick.Initiative()
	s.Require().NoError(err)
	s.Equal([]conditions.InitiativeContribution{{Source: "Dexterity", Amount: 2}}, out.Contributions)

	blank, err := s.m.Initiative()
	s.Require().NoError(err)
	s.Empty(blank.Contributions, "the ogre fixture states no ability scores")
}

// Catches damage that reads the raised maximum as a floor or ceiling: damage
// is unchanged by a raise.
func (s *monsterHeldRaiseSuite) TestDamageIsUnchanged() {
	s.raise("application:feast", "2")
	s.m.ApplyDamage(s.ctx, &combat.ApplyDamageInput{Instances: []combat.DamageInstance{{Amount: 5, Type: "slashing"}}})
	s.Equal(15, s.m.HP())
}
