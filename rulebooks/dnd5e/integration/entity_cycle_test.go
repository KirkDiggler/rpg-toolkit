// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package integration provides comprehensive encounter-level integration tests
// that demonstrate how each class's features work in combat scenarios.
// These tests serve as both verification AND documentation for toolkit integrators.
package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ============================================================================
// ENTITY CYCLE SPIKE TEST
// Validates: Load -> Execute (ResolveAttack) -> ApplyDamage -> Harvest (ToData) -> Save
//
// This spike test confirms the full entity state cycle works end-to-end.
// It answers: Can we load a character, deal damage, and get the updated
// state back out via ToData() with the correct HP and dirty flag?
// ============================================================================

type EntityCycleSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	bus        events.EventBus
	mockRoller *mock_dice.MockRoller
	lookup     *integrationLookup
	room       spatial.Room

	fighter  *character.Character
	goblin   *monster.Monster
	scimitar *weapons.Weapon
}

func (s *EntityCycleSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.bus = events.NewEventBus()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
	s.lookup = newIntegrationLookup()
	s.ctx = context.Background()

	// Create spatial room for movement
	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{
		Width:  10,
		Height: 10,
	})
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "combat-room",
		Type: "combat",
		Grid: grid,
	})
}

func (s *EntityCycleSuite) SetupSubTest() {
	// Fresh event bus for each subtest
	s.bus = events.NewEventBus()
	s.lookup = newIntegrationLookup()

	s.fighter = s.createLevel1Fighter()
	s.goblin = s.createGoblin()
	s.scimitar = s.createScimitar()

	s.lookup.Add(s.fighter)
	s.lookup.Add(s.goblin)

	// Set up context with combatant lookup for ResolveAttack
	s.ctx = combat.WithCombatantLookup(context.Background(), s.lookup)

	// Place in room - adjacent for melee
	_ = s.room.PlaceEntity(s.fighter, spatial.Position{X: 2, Y: 2})
	_ = s.room.PlaceEntity(s.goblin, spatial.Position{X: 3, Y: 2})
}

func (s *EntityCycleSuite) TearDownSubTest() {
	_ = s.room.RemoveEntity(s.fighter.GetID())
	_ = s.room.RemoveEntity(s.goblin.GetID())
	if s.fighter != nil {
		_ = s.fighter.Cleanup(s.ctx)
	}
}

func (s *EntityCycleSuite) TearDownTest() {
	s.ctrl.Finish()
}

// =============================================================================
// CHARACTER CREATION HELPERS
// =============================================================================

func (s *EntityCycleSuite) createLevel1Fighter() *character.Character {
	// Level 1 Fighter with standard array:
	// STR 16 (+3), DEX 14 (+2), CON 14 (+2), INT 10 (+0), WIS 12 (+1), CHA 8 (-1)
	// HP: 10 + 2 = 12
	// AC: 16 (chain mail)
	data := &character.Data{
		ID:               "test-fighter",
		PlayerID:         "player-1",
		Name:             "Test Fighter",
		Level:            1,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, // +3
			abilities.DEX: 14, // +2
			abilities.CON: 14, // +2
			abilities.INT: 10, // +0
			abilities.WIS: 12, // +1
			abilities.CHA: 8,  // -1
		},
		HitPoints:    12, // 10 base + 2 CON
		MaxHitPoints: 12,
		ArmorClass:   16, // Chain mail
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Athletics:  shared.Proficient,
			skills.Perception: shared.Proficient,
		},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
			abilities.CON: shared.Proficient,
		},
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{},
	}

	char, err := character.LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)

	// Add combat abilities
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewAttack("attack")))

	return char
}

func (s *EntityCycleSuite) createGoblin() *monster.Monster {
	return monster.New(monster.Config{
		ID:   "goblin-1",
		Name: "Goblin Scout",
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8,  // -1
			abilities.DEX: 14, // +2
			abilities.CON: 10, // +0
			abilities.INT: 10, // +0
			abilities.WIS: 8,  // -1
			abilities.CHA: 8,  // -1
		},
		AC: 13,
		HP: 7,
	})
}

func (s *EntityCycleSuite) createScimitar() *weapons.Weapon {
	weapon, err := weapons.GetByID(weapons.Scimitar)
	s.Require().NoError(err)
	return &weapon
}

// =============================================================================
// SPIKE TEST: Load -> Execute -> ApplyDamage -> Harvest -> Save
// =============================================================================

func (s *EntityCycleSuite) TestEntityCycle_DamageAndHarvest() {
	s.Run("Full cycle: monster attacks character, character state reflects damage via ToData", func() {
		s.T().Log("============================================================")
		s.T().Log("  SPIKE TEST: Entity Load/Fire/Harvest/Save Cycle")
		s.T().Log("============================================================")
		s.T().Log("")

		// Step 1: Verify initial state
		initialHP := s.fighter.GetHitPoints()
		s.Equal(12, initialHP, "Fighter should start with 12 HP")
		s.False(s.fighter.IsDirty(), "Fighter should NOT be dirty after loading")

		s.T().Logf("  Initial state: Fighter HP=%d/%d, IsDirty=%v",
			initialHP, s.fighter.GetMaxHitPoints(), s.fighter.IsDirty())

		// Step 2: Resolve attack from goblin -> fighter
		// Mock dice: attack roll 18 (hits AC 16), damage roll 4 (1d6 scimitar)
		s.T().Log("")
		s.T().Log("  Step 2: Goblin attacks fighter with scimitar")

		s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(18, nil).Times(1)
		s.mockRoller.EXPECT().RollN(s.ctx, 1, 6).Return([]int{4}, nil).Times(1)

		attackResult, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
			AttackerID: s.goblin.GetID(),
			TargetID:   s.fighter.GetID(),
			Weapon:     s.scimitar,
			EventBus:   s.bus,
			Roller:     s.mockRoller,
		})
		s.Require().NoError(err)
		s.True(attackResult.Hit, "Attack should hit (18+4 = 22 vs AC 16)")

		// Goblin: DEX +2 + prof +2 = +4 attack bonus
		// Damage: 1d6(4) + DEX(2) = 6
		expectedDamage := 4 + 2
		s.Equal(expectedDamage, attackResult.TotalDamage, "Damage should be 1d6(4) + DEX(2) = 6")

		s.T().Logf("  Attack: 1d20(%d)+4 = %d vs AC 16 -> HIT", 18, 22)
		s.T().Logf("  Damage: 1d6(%d)+2 = %d slashing", 4, expectedDamage)

		// Step 3: Apply damage to the fighter via DealDamage (full event chain)
		s.T().Log("")
		s.T().Log("  Step 3: Apply damage to fighter via DealDamage")

		dealResult, err := combat.DealDamage(s.ctx, &combat.DealDamageInput{
			Target:     s.fighter,
			AttackerID: s.goblin.GetID(),
			Source:     combat.DamageSourceAttack,
			Instances: []combat.DamageInstanceInput{
				{Amount: attackResult.TotalDamage, Type: attackResult.DamageType},
			},
			EventBus: s.bus,
		})
		s.Require().NoError(err)

		expectedHP := initialHP - expectedDamage
		s.Equal(expectedHP, dealResult.CurrentHP, "Fighter HP should decrease by damage dealt")
		s.Equal(expectedDamage, dealResult.TotalDamage, "Total damage should match")
		s.False(dealResult.DroppedToZero, "Fighter should not be at 0 HP")

		s.T().Logf("  Fighter HP: %d -> %d (took %d damage)", initialHP, dealResult.CurrentHP, dealResult.TotalDamage)

		// Step 4: Assert IsDirty is true
		s.T().Log("")
		s.T().Log("  Step 4: Check dirty flag")
		s.True(s.fighter.IsDirty(), "Fighter MUST be dirty after taking damage")
		s.T().Logf("  IsDirty = %v (CORRECT: character was modified)", s.fighter.IsDirty())

		// Step 5: Harvest via ToData() and verify HP decreased
		s.T().Log("")
		s.T().Log("  Step 5: Harvest state via ToData()")
		data := s.fighter.ToData()
		s.Require().NotNil(data, "ToData() must not return nil")
		s.Equal(expectedHP, data.HitPoints, "ToData().HitPoints should reflect damage taken")
		s.Equal(12, data.MaxHitPoints, "ToData().MaxHitPoints should be unchanged")
		s.Equal("test-fighter", data.ID, "ToData().ID should match")

		s.T().Logf("  ToData().HitPoints = %d (expected %d)", data.HitPoints, expectedHP)
		s.T().Logf("  ToData().MaxHitPoints = %d (expected 12)", data.MaxHitPoints)

		// Step 6: Verify MarkClean resets dirty flag (simulating save completion)
		s.T().Log("")
		s.T().Log("  Step 6: Simulate save by calling MarkClean()")
		s.fighter.MarkClean()
		s.False(s.fighter.IsDirty(), "After MarkClean, IsDirty should be false")
		s.T().Logf("  After MarkClean(): IsDirty = %v", s.fighter.IsDirty())

		s.T().Log("")
		s.T().Log("  RESULT: Full Load/Fire/Harvest/Save cycle works correctly")
		s.T().Log("============================================================")
	})
}

func (s *EntityCycleSuite) TestEntityCycle_DropToZeroHP() {
	s.Run("Drop character to 0 HP and check for unconscious condition in ToData", func() {
		s.T().Log("============================================================")
		s.T().Log("  SPIKE TEST: Drop to 0 HP -> Unconscious Condition Check")
		s.T().Log("============================================================")
		s.T().Log("")

		initialHP := s.fighter.GetHitPoints()
		s.T().Logf("  Fighter HP: %d/%d", initialHP, s.fighter.GetMaxHitPoints())

		// Deal enough damage to drop to 0 HP (12 damage to a 12 HP character)
		s.T().Log("")
		s.T().Log("  Dealing 12 damage to drop fighter to 0 HP")

		dealResult, err := combat.DealDamage(s.ctx, &combat.DealDamageInput{
			Target:     s.fighter,
			AttackerID: s.goblin.GetID(),
			Source:     combat.DamageSourceAttack,
			Instances: []combat.DamageInstanceInput{
				{Amount: 12, Type: "slashing"},
			},
			EventBus: s.bus,
		})
		s.Require().NoError(err)

		s.Equal(0, dealResult.CurrentHP, "Fighter should be at 0 HP")
		s.True(dealResult.DroppedToZero, "DroppedToZero should be true")
		s.Equal(0, s.fighter.GetHitPoints(), "GetHitPoints() should return 0")

		s.T().Logf("  Fighter HP: %d -> %d", initialHP, dealResult.CurrentHP)
		s.T().Logf("  DroppedToZero: %v", dealResult.DroppedToZero)

		// Check IsDirty
		s.True(s.fighter.IsDirty(), "Fighter MUST be dirty after dropping to 0 HP")

		// Harvest via ToData and check for unconscious condition
		s.T().Log("")
		s.T().Log("  Harvesting state via ToData()...")
		data := s.fighter.ToData()
		s.Require().NotNil(data)
		s.Equal(0, data.HitPoints, "ToData().HitPoints should be 0")

		// Check if unconscious condition is present
		s.T().Log("")
		s.T().Log("  Checking for unconscious condition in ToData().Conditions...")

		unconsciousFound := false
		for _, rawCond := range data.Conditions {
			var peek struct {
				Ref struct {
					ID string `json:"id"`
				} `json:"ref"`
			}
			if err := json.Unmarshal(rawCond, &peek); err == nil {
				s.T().Logf("  Found condition: ref.id=%q", peek.Ref.ID)
				if peek.Ref.ID == "unconscious" {
					unconsciousFound = true
				}
			}
		}

		if unconsciousFound {
			s.T().Log("")
			s.T().Log("  FINDING: Unconscious condition IS auto-applied when HP drops to 0")
			s.T().Log("  This means the event chain handles the transition automatically")
		} else {
			s.T().Log("")
			s.T().Logf("  FINDING: Unconscious condition is NOT auto-applied when HP drops to 0")
			s.T().Logf("  Conditions found: %d", len(data.Conditions))
			s.T().Log("  The caller (game server) must detect DroppedToZero and apply")
			s.T().Log("  the unconscious condition separately. This is a gap in the")
			s.T().Log("  current toolkit that needs to be addressed.")
		}

		// Document the finding either way - this is a spike test
		s.T().Log("")
		s.T().Logf("  RESULT: ToData().HitPoints = %d, Conditions count = %d, Unconscious auto-applied = %v",
			data.HitPoints, len(data.Conditions), unconsciousFound)
		s.T().Log("============================================================")
	})
}

func (s *EntityCycleSuite) TestEntityCycle_RoundTrip() {
	s.Run("Load from Data, take damage, ToData, reload from Data - HP persists", func() {
		s.T().Log("============================================================")
		s.T().Log("  SPIKE TEST: Full Round-Trip (Load -> Damage -> Save -> Reload)")
		s.T().Log("============================================================")
		s.T().Log("")

		// Deal some damage
		_, err := combat.DealDamage(s.ctx, &combat.DealDamageInput{
			Target:     s.fighter,
			AttackerID: s.goblin.GetID(),
			Source:     combat.DamageSourceAttack,
			Instances: []combat.DamageInstanceInput{
				{Amount: 5, Type: "slashing"},
			},
			EventBus: s.bus,
		})
		s.Require().NoError(err)
		s.Equal(7, s.fighter.GetHitPoints(), "Fighter should have 7 HP after 5 damage")

		// Harvest state
		data := s.fighter.ToData()
		s.Equal(7, data.HitPoints, "ToData() should show 7 HP")

		s.T().Logf("  After damage: HP = %d, saving state...", data.HitPoints)

		// Simulate save + reload by creating a new character from the harvested data
		// Need to clean up old character first
		_ = s.fighter.Cleanup(s.ctx)

		newBus := events.NewEventBus()
		reloaded, err := character.LoadFromData(s.ctx, data, newBus)
		s.Require().NoError(err)
		s.Require().NotNil(reloaded)

		s.Equal(7, reloaded.GetHitPoints(), "Reloaded character should have 7 HP")
		s.Equal(12, reloaded.GetMaxHitPoints(), "Reloaded character should have 12 max HP")
		s.False(reloaded.IsDirty(), "Freshly loaded character should NOT be dirty")

		s.T().Logf("  After reload: HP = %d/%d, IsDirty = %v",
			reloaded.GetHitPoints(), reloaded.GetMaxHitPoints(), reloaded.IsDirty())

		// Clean up reloaded character
		_ = reloaded.Cleanup(s.ctx)
		// Prevent TearDownSubTest from double-cleaning
		s.fighter = reloaded

		s.T().Log("")
		s.T().Log("  RESULT: Full round-trip preserves HP state correctly")
		s.T().Log("============================================================")
	})
}

func TestEntityCycleSuite(t *testing.T) {
	suite.Run(t, new(EntityCycleSuite))
}
