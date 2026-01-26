// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package integration provides comprehensive encounter-level integration tests
// that demonstrate how each class's features work in combat scenarios.
// These tests serve as both verification AND documentation for toolkit integrators.
package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ============================================================================
// ROGUE ENCOUNTER TEST SUITE
// Level 1 Rogue Features:
//   - Sneak Attack (1d6 extra damage with advantage OR ally adjacent to target)
//   - Expertise (double proficiency on 2 skills) - choice-based, not combat-relevant
//   - Thieves' Cant (language) - not combat-relevant
//   - Proficiencies (light armor, simple weapons, hand crossbows, longswords, rapiers, shortswords)
// ============================================================================

type RogueEncounterSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	bus        events.EventBus
	mockRoller *mock_dice.MockRoller
	lookup     *integrationLookup
	room       spatial.Room
	registry   *gamectx.BasicCharacterRegistry

	rogue  *mockRogueCharacter
	ally   *mockAllyCharacter
	goblin *monster.Monster
	rapier *weapons.Weapon
}

// mockRogueCharacter implements the interfaces needed for rogue testing
type mockRogueCharacter struct {
	id               string
	name             string
	level            int
	abilityScores    shared.AbilityScores
	proficiencyBonus int
	hitPoints        int
	maxHitPoints     int
	armorClass       int
	conditions       []dnd5eEvents.ConditionBehavior
}

func (m *mockRogueCharacter) GetID() string                                  { return m.id }
func (m *mockRogueCharacter) GetType() core.EntityType                       { return "character" }
func (m *mockRogueCharacter) GetName() string                                { return m.name }
func (m *mockRogueCharacter) GetLevel() int                                  { return m.level }
func (m *mockRogueCharacter) AbilityScores() shared.AbilityScores            { return m.abilityScores }
func (m *mockRogueCharacter) ProficiencyBonus() int                          { return m.proficiencyBonus }
func (m *mockRogueCharacter) GetHitPoints() int                              { return m.hitPoints }
func (m *mockRogueCharacter) GetMaxHitPoints() int                           { return m.maxHitPoints }
func (m *mockRogueCharacter) AC() int                                        { return m.armorClass }
func (m *mockRogueCharacter) IsDirty() bool                                  { return false }
func (m *mockRogueCharacter) MarkClean()                                     {}
func (m *mockRogueCharacter) GetConditions() []dnd5eEvents.ConditionBehavior { return m.conditions }

func (m *mockRogueCharacter) ApplyDamage(_ context.Context, input *combat.ApplyDamageInput) *combat.ApplyDamageResult {
	if input == nil {
		return &combat.ApplyDamageResult{CurrentHP: m.hitPoints, PreviousHP: m.hitPoints}
	}
	previousHP := m.hitPoints
	totalDamage := 0
	for _, instance := range input.Instances {
		totalDamage += instance.Amount
	}
	m.hitPoints -= totalDamage
	if m.hitPoints < 0 {
		m.hitPoints = 0
	}
	return &combat.ApplyDamageResult{TotalDamage: totalDamage, CurrentHP: m.hitPoints, PreviousHP: previousHP}
}

// mockAllyCharacter represents another party member for testing ally-adjacent sneak attack
type mockAllyCharacter struct {
	id   string
	name string
}

func (m *mockAllyCharacter) GetID() string            { return m.id }
func (m *mockAllyCharacter) GetType() core.EntityType { return "character" }
func (m *mockAllyCharacter) GetName() string          { return m.name }

func TestRogueEncounterSuite(t *testing.T) {
	suite.Run(t, new(RogueEncounterSuite))
}

func (s *RogueEncounterSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.bus = events.NewEventBus()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
	s.lookup = newIntegrationLookup()
	s.ctx = context.Background()

	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{ID: "combat-room", Type: "combat", Grid: grid})
}

func (s *RogueEncounterSuite) SetupSubTest() {
	s.bus = events.NewEventBus()
	s.lookup = newIntegrationLookup()

	s.rogue = s.createLevel1Rogue()
	s.ally = s.createAlly()
	s.goblin = s.createGoblin()
	s.rapier = s.createRapier()

	s.lookup.Add(s.rogue)
	s.lookup.Add(s.goblin)

	s.registry = gamectx.NewBasicCharacterRegistry()
	scores := &gamectx.AbilityScores{
		Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 12, Wisdom: 10, Charisma: 14,
	}
	s.registry.AddAbilityScores(s.rogue.GetID(), scores)

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{CharacterRegistry: s.registry})
	s.ctx = combat.WithCombatantLookup(context.Background(), s.lookup)
	s.ctx = gamectx.WithGameContext(s.ctx, gameCtx)
	s.ctx = gamectx.WithRoom(s.ctx, s.room)

	_ = s.room.PlaceEntity(s.rogue, spatial.Position{X: 2, Y: 2})
	_ = s.room.PlaceEntity(s.goblin, spatial.Position{X: 3, Y: 2})
	_ = s.room.PlaceEntity(s.ally, spatial.Position{X: 8, Y: 8}) // Far away by default
}

func (s *RogueEncounterSuite) TearDownSubTest() {
	_ = s.room.RemoveEntity(s.rogue.GetID())
	_ = s.room.RemoveEntity(s.goblin.GetID())
	_ = s.room.RemoveEntity(s.ally.GetID())
}

func (s *RogueEncounterSuite) TearDownTest() {
	s.ctrl.Finish()
}

// =============================================================================
// CHARACTER CREATION
// =============================================================================

func (s *RogueEncounterSuite) createLevel1Rogue() *mockRogueCharacter {
	return &mockRogueCharacter{
		id: "rogue-1", name: "Shadow", level: 1, proficiencyBonus: 2,
		hitPoints: 10, maxHitPoints: 10, armorClass: 14,
		abilityScores: shared.AbilityScores{
			abilities.STR: 10, abilities.DEX: 16, abilities.CON: 14,
			abilities.INT: 12, abilities.WIS: 10, abilities.CHA: 14,
		},
		conditions: []dnd5eEvents.ConditionBehavior{},
	}
}

func (s *RogueEncounterSuite) createAlly() *mockAllyCharacter {
	return &mockAllyCharacter{id: "fighter-ally", name: "Tank"}
}

func (s *RogueEncounterSuite) createGoblin() *monster.Monster {
	return monster.New(monster.Config{
		ID: "goblin-1", Name: "Goblin", AC: 15, HP: 7,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8, abilities.DEX: 14, abilities.CON: 10,
			abilities.INT: 10, abilities.WIS: 8, abilities.CHA: 8,
		},
	})
}

func (s *RogueEncounterSuite) createRapier() *weapons.Weapon {
	weapon, _ := weapons.GetByID(weapons.Rapier)
	return &weapon
}

// =============================================================================
// SNEAK ATTACK TESTS
// =============================================================================

func (s *RogueEncounterSuite) TestSneakAttack_WithAdvantage_AddsDamage() {
	s.Run("Sneak Attack triggers with advantage", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  ROGUE SNEAK ATTACK: With Advantage                              ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		// Apply Sneak Attack condition
		sneakAttack := conditions.NewSneakAttackCondition(conditions.SneakAttackInput{
			CharacterID: s.rogue.GetID(),
			Level:       1,
			Roller:      s.mockRoller,
		})
		err := sneakAttack.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = sneakAttack.Remove(s.ctx, s.bus) }()

		// Expect 1d6 sneak attack roll
		s.mockRoller.EXPECT().RollN(gomock.Any(), 1, 6).Return([]int{4}, nil)

		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID:   s.rogue.GetID(),
			TargetID:     s.goblin.GetID(),
			DamageType:   damage.Piercing,
			AbilityUsed:  abilities.DEX, // Rapier is finesse
			HasAdvantage: true,
			Components: []dnd5eEvents.DamageComponent{
				{Source: dnd5eEvents.DamageSourceWeapon, OriginalDiceRolls: []int{6}, FinalDiceRolls: []int{6}, DamageType: damage.Piercing},
			},
		}

		// Execute through damage chain
		damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		damageTopic := dnd5eEvents.DamageChain.On(s.bus)
		modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, damageChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
		s.Require().NoError(err)

		// Verify sneak attack damage was added
		var sneakDice []int
		for _, comp := range finalEvent.Components {
			if comp.Source == dnd5eEvents.DamageSourceFeature {
				sneakDice = comp.FinalDiceRolls
				break
			}
		}
		s.Require().NotNil(sneakDice, "Should have sneak attack component")
		s.Equal([]int{4}, sneakDice, "Sneak attack should add 1d6 (rolled 4)")

		s.T().Log("✓ Sneak Attack correctly triggers with advantage")
	})
}

func (s *RogueEncounterSuite) TestSneakAttack_WithAllyAdjacent_AddsDamage() {
	s.Run("Sneak Attack triggers with ally adjacent to target", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  ROGUE SNEAK ATTACK: Ally Adjacent to Target                     ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		// Move ally adjacent to goblin
		_ = s.room.RemoveEntity(s.ally.GetID())
		_ = s.room.PlaceEntity(s.ally, spatial.Position{X: 4, Y: 2}) // Adjacent to goblin at (3,2)

		sneakAttack := conditions.NewSneakAttackCondition(conditions.SneakAttackInput{
			CharacterID: s.rogue.GetID(),
			Level:       1,
			Roller:      s.mockRoller,
		})
		err := sneakAttack.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = sneakAttack.Remove(s.ctx, s.bus) }()

		s.mockRoller.EXPECT().RollN(gomock.Any(), 1, 6).Return([]int{5}, nil)

		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID:   s.rogue.GetID(),
			TargetID:     s.goblin.GetID(),
			DamageType:   damage.Piercing,
			AbilityUsed:  abilities.DEX,
			HasAdvantage: false, // No advantage!
			Components: []dnd5eEvents.DamageComponent{
				{Source: dnd5eEvents.DamageSourceWeapon, OriginalDiceRolls: []int{5}, FinalDiceRolls: []int{5}, DamageType: damage.Piercing},
			},
		}

		damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		damageTopic := dnd5eEvents.DamageChain.On(s.bus)
		modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, damageChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
		s.Require().NoError(err)

		var hasSneakAttack bool
		for _, comp := range finalEvent.Components {
			if comp.Source == dnd5eEvents.DamageSourceFeature {
				hasSneakAttack = true
				break
			}
		}
		s.True(hasSneakAttack, "Should trigger sneak attack when ally is adjacent to target")

		s.T().Log("✓ Sneak Attack correctly triggers with ally adjacent")
	})
}

func (s *RogueEncounterSuite) TestSneakAttack_NoAdvantageNoAlly_NoDamage() {
	s.Run("Sneak Attack does NOT trigger without advantage or ally", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  ROGUE SNEAK ATTACK: No Advantage, No Ally                       ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		// Ally is far away (default position at 8,8)
		sneakAttack := conditions.NewSneakAttackCondition(conditions.SneakAttackInput{
			CharacterID: s.rogue.GetID(),
			Level:       1,
			Roller:      s.mockRoller,
		})
		err := sneakAttack.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = sneakAttack.Remove(s.ctx, s.bus) }()

		// NO dice roll expected

		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID:   s.rogue.GetID(),
			TargetID:     s.goblin.GetID(),
			DamageType:   damage.Piercing,
			AbilityUsed:  abilities.DEX,
			HasAdvantage: false,
			Components: []dnd5eEvents.DamageComponent{
				{Source: dnd5eEvents.DamageSourceWeapon, OriginalDiceRolls: []int{4}, FinalDiceRolls: []int{4}, DamageType: damage.Piercing},
			},
		}

		damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		damageTopic := dnd5eEvents.DamageChain.On(s.bus)
		modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, damageChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
		s.Require().NoError(err)

		// Should only have weapon damage, no sneak attack
		s.Len(finalEvent.Components, 1, "Should only have weapon damage, no sneak attack")

		s.T().Log("✓ Sneak Attack correctly denied without advantage or ally")
	})
}

func (s *RogueEncounterSuite) TestSneakAttack_OncePerTurn() {
	s.Run("Sneak Attack only triggers once per turn", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  ROGUE SNEAK ATTACK: Once Per Turn                               ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		sneakAttack := conditions.NewSneakAttackCondition(conditions.SneakAttackInput{
			CharacterID: s.rogue.GetID(),
			Level:       1,
			Roller:      s.mockRoller,
		})
		err := sneakAttack.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = sneakAttack.Remove(s.ctx, s.bus) }()

		// First attack - sneak attack should trigger
		s.mockRoller.EXPECT().RollN(gomock.Any(), 1, 6).Return([]int{3}, nil)

		damageEvent1 := &dnd5eEvents.DamageChainEvent{
			AttackerID: s.rogue.GetID(), TargetID: s.goblin.GetID(),
			DamageType: damage.Piercing, AbilityUsed: abilities.DEX, HasAdvantage: true,
			Components: []dnd5eEvents.DamageComponent{{Source: dnd5eEvents.DamageSourceWeapon}},
		}

		chain1 := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		topic := dnd5eEvents.DamageChain.On(s.bus)
		modChain1, _ := topic.PublishWithChain(s.ctx, damageEvent1, chain1)
		finalEvent1, _ := modChain1.Execute(s.ctx, damageEvent1)
		s.Len(finalEvent1.Components, 2, "First attack should have sneak attack")

		// Second attack (same turn) - sneak attack should NOT trigger
		damageEvent2 := &dnd5eEvents.DamageChainEvent{
			AttackerID: s.rogue.GetID(), TargetID: s.goblin.GetID(),
			DamageType: damage.Piercing, AbilityUsed: abilities.DEX, HasAdvantage: true,
			Components: []dnd5eEvents.DamageComponent{{Source: dnd5eEvents.DamageSourceWeapon}},
		}

		chain2 := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		modChain2, _ := topic.PublishWithChain(s.ctx, damageEvent2, chain2)
		finalEvent2, _ := modChain2.Execute(s.ctx, damageEvent2)
		s.Len(finalEvent2.Components, 1, "Second attack should NOT have sneak attack")

		s.T().Log("✓ Sneak Attack correctly limited to once per turn")
	})
}

func (s *RogueEncounterSuite) TestSneakAttack_ResetsOnTurnEnd() {
	s.Run("Sneak Attack resets on turn end", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  ROGUE SNEAK ATTACK: Resets on Turn End                          ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		sneakAttack := conditions.NewSneakAttackCondition(conditions.SneakAttackInput{
			CharacterID: s.rogue.GetID(),
			Level:       1,
			Roller:      s.mockRoller,
		})
		err := sneakAttack.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = sneakAttack.Remove(s.ctx, s.bus) }()

		topic := dnd5eEvents.DamageChain.On(s.bus)

		// First attack uses sneak attack
		s.mockRoller.EXPECT().RollN(gomock.Any(), 1, 6).Return([]int{3}, nil)
		damageEvent1 := &dnd5eEvents.DamageChainEvent{
			AttackerID: s.rogue.GetID(), TargetID: s.goblin.GetID(),
			AbilityUsed: abilities.DEX, HasAdvantage: true,
			Components: []dnd5eEvents.DamageComponent{{Source: dnd5eEvents.DamageSourceWeapon}},
		}
		chain1 := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		modChain1, _ := topic.PublishWithChain(s.ctx, damageEvent1, chain1)
		_, _ = modChain1.Execute(s.ctx, damageEvent1)

		// End the turn
		turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
		_ = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{CharacterID: s.rogue.GetID()})

		// Next turn - sneak attack should work again
		s.mockRoller.EXPECT().RollN(gomock.Any(), 1, 6).Return([]int{6}, nil)
		damageEvent2 := &dnd5eEvents.DamageChainEvent{
			AttackerID: s.rogue.GetID(), TargetID: s.goblin.GetID(),
			AbilityUsed: abilities.DEX, HasAdvantage: true,
			Components: []dnd5eEvents.DamageComponent{{Source: dnd5eEvents.DamageSourceWeapon}},
		}
		chain2 := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		modChain2, _ := topic.PublishWithChain(s.ctx, damageEvent2, chain2)
		finalEvent2, _ := modChain2.Execute(s.ctx, damageEvent2)

		s.Len(finalEvent2.Components, 2, "Sneak attack should work again after turn end")

		s.T().Log("✓ Sneak Attack correctly resets on turn end")
	})
}

func (s *RogueEncounterSuite) TestSneakAttack_RequiresFinesseOrRanged() {
	s.Run("Sneak Attack requires finesse (DEX) attack", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  ROGUE SNEAK ATTACK: Requires Finesse/Ranged                     ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		sneakAttack := conditions.NewSneakAttackCondition(conditions.SneakAttackInput{
			CharacterID: s.rogue.GetID(),
			Level:       1,
			Roller:      s.mockRoller,
		})
		err := sneakAttack.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = sneakAttack.Remove(s.ctx, s.bus) }()

		// STR attack - should NOT trigger sneak attack even with advantage
		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID:   s.rogue.GetID(),
			TargetID:     s.goblin.GetID(),
			AbilityUsed:  abilities.STR, // Not DEX!
			HasAdvantage: true,
			Components:   []dnd5eEvents.DamageComponent{{Source: dnd5eEvents.DamageSourceWeapon}},
		}

		chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		topic := dnd5eEvents.DamageChain.On(s.bus)
		modChain, _ := topic.PublishWithChain(s.ctx, damageEvent, chain)
		finalEvent, _ := modChain.Execute(s.ctx, damageEvent)

		s.Len(finalEvent.Components, 1, "STR attack should not trigger sneak attack")

		s.T().Log("✓ Sneak Attack correctly requires finesse/ranged attack")
	})
}

func (s *RogueEncounterSuite) TestSneakAttack_ScalesWithLevel() {
	s.Run("Sneak Attack scales: 2d6 at level 3", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  ROGUE SNEAK ATTACK: Level Scaling (2d6 at L3)                   ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		sneakAttack := conditions.NewSneakAttackCondition(conditions.SneakAttackInput{
			CharacterID: s.rogue.GetID(),
			Level:       3, // Level 3 = 2d6
			Roller:      s.mockRoller,
		})
		err := sneakAttack.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = sneakAttack.Remove(s.ctx, s.bus) }()

		// Expect 2d6 roll
		s.mockRoller.EXPECT().RollN(gomock.Any(), 2, 6).Return([]int{4, 5}, nil)

		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID: s.rogue.GetID(), TargetID: s.goblin.GetID(),
			AbilityUsed: abilities.DEX, HasAdvantage: true,
			Components: []dnd5eEvents.DamageComponent{{Source: dnd5eEvents.DamageSourceWeapon}},
		}

		chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		topic := dnd5eEvents.DamageChain.On(s.bus)
		modChain, _ := topic.PublishWithChain(s.ctx, damageEvent, chain)
		finalEvent, _ := modChain.Execute(s.ctx, damageEvent)

		var sneakDice []int
		for _, comp := range finalEvent.Components {
			if comp.Source == dnd5eEvents.DamageSourceFeature {
				sneakDice = comp.FinalDiceRolls
				break
			}
		}
		s.Equal([]int{4, 5}, sneakDice, "Level 3 should roll 2d6 for sneak attack")

		s.T().Log("✓ Sneak Attack correctly scales with level")
	})
}
