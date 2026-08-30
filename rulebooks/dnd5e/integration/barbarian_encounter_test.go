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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ============================================================================
// BARBARIAN ENCOUNTER TEST SUITE
// Level 1 Barbarian Features:
//   - Rage (2 uses, +2 damage, B/P/S resistance, ends if no combat activity)
//   - Unarmored Defense (AC = 10 + DEX + CON)
// ============================================================================

type BarbarianEncounterSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	bus        events.EventBus
	mockRoller *mock_dice.MockRoller
	room       spatial.Room

	barbarian *character.Character
	goblin    *monster.Monster
	greataxe  *weapons.Weapon
}

func (s *BarbarianEncounterSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.bus = events.NewEventBus()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
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

func (s *BarbarianEncounterSuite) SetupSubTest() {
	// Fresh event bus for each subtest
	s.bus = events.NewEventBus()

	// Create barbarian and goblin
	s.barbarian = s.createLevel1Barbarian()
	s.goblin = s.createGoblin()
	s.greataxe = s.createGreataxe()

	// Set up context with combatant lookup for encounter fixtures.
	s.ctx = context.Background()

	// Place in room - adjacent for melee
	_ = s.room.PlaceEntity(s.barbarian, spatial.Position{X: 2, Y: 2})
	_ = s.room.PlaceEntity(s.goblin, spatial.Position{X: 3, Y: 2})
}

func (s *BarbarianEncounterSuite) TearDownSubTest() {
	_ = s.room.RemoveEntity(s.barbarian.GetID())
	_ = s.room.RemoveEntity(s.goblin.GetID())
	if s.barbarian != nil {
		_ = s.barbarian.Cleanup(s.ctx)
	}
}

func (s *BarbarianEncounterSuite) TearDownTest() {
	s.ctrl.Finish()
}

// =============================================================================
// CHARACTER CREATION HELPERS
// =============================================================================

func (s *BarbarianEncounterSuite) createLevel1Barbarian() *character.Character {
	// Level 1 Barbarian with standard array:
	// STR 16 (+3), DEX 14 (+2), CON 16 (+3), INT 8 (-1), WIS 10 (+0), CHA 12 (+1)
	// Unarmored Defense: 10 + 2 + 3 = 15 AC
	// HP: 12 + 3 = 15
	// Rage: 2 uses, +2 damage, resistance to B/P/S
	data := &character.Data{
		ID:               "grog-barbarian",
		PlayerID:         "player-1",
		Name:             "Grog the Destroyer",
		Level:            1,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Barbarian,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, // +3
			abilities.DEX: 14, // +2
			abilities.CON: 16, // +3
			abilities.INT: 8,  // -1
			abilities.WIS: 10, // +0
			abilities.CHA: 12, // +1
		},
		HitPoints:    15, // 12 base + 3 CON
		MaxHitPoints: 15,
		ArmorClass:   15, // Unarmored: 10 + DEX(2) + CON(3)
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Athletics:    shared.Proficient,
			skills.Intimidation: shared.Proficient,
		},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
			abilities.CON: shared.Proficient,
		},
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.RageCharges: {
				Current:   2,
				Maximum:   2,
				ResetType: coreResources.ResetLongRest,
			},
		},
		Features: []json.RawMessage{
			json.RawMessage(`{
				"ref": {"module": "dnd5e", "type": "features", "id": "rage"},
				"id": "rage",
				"name": "Rage",
				"level": 1
			}`),
		},
	}

	char, err := character.LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)

	// Add combat abilities
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewAttack("attack")))

	return char
}

func (s *BarbarianEncounterSuite) createGoblin() *monster.Monster {
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

func (s *BarbarianEncounterSuite) createGreataxe() *weapons.Weapon {
	weapon, err := weapons.GetByID(weapons.Greataxe)
	s.Require().NoError(err)
	return &weapon
}

// =============================================================================
// LEVEL 1: RAGE FEATURE TESTS
// =============================================================================

func (s *BarbarianEncounterSuite) TestRage_EndsAfter10Turns() {
	s.Run("Rage ends after 10 turns (1 minute)", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  BARBARIAN RAGE: Duration Limit (10 rounds = 1 minute)           ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// Activate rage
		rage := s.barbarian.GetFeature("rage")
		err := rage.Activate(s.ctx, s.barbarian, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)
		s.T().Log("→ Grog enters a rage!")

		// Track rage end
		var rageEndReason string
		removedTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
		_, err = removedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			if event.ConditionRef == "dnd5e:conditions:raging" {
				rageEndReason = event.Reason
			}
			return nil
		})
		s.Require().NoError(err)

		turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
		damageTopic := dnd5eEvents.DamageReceivedTopic.On(s.bus)

		// Simulate 10 turns with combat activity each turn
		for round := 1; round <= 10; round++ {
			// Take damage to maintain rage
			err = damageTopic.Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
				TargetID: s.barbarian.GetID(),
				SourceID: s.goblin.GetID(),
				Amount:   1,
			})
			s.Require().NoError(err)

			// End turn
			err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
				SubjectID: s.barbarian.GetID(),
				Round:     round,
			})
			s.Require().NoError(err)

			if round < 10 {
				s.Empty(rageEndReason, "Rage should continue before round 10")
			}
		}

		s.T().Log("")
		s.T().Logf("→ 10 rounds of combat pass (1 minute)")
		s.T().Log("→ End of round 10")

		// Verify rage ended
		s.Equal("duration_expired", rageEndReason, "Rage should end due to duration")
		s.T().Log("  ✗ Rage ends (1 minute duration expired)")

		s.T().Log("")
		s.T().Log("✓ Rage correctly ends after 10 rounds")
	})
}

// =============================================================================
// LEVEL 1: UNARMORED DEFENSE TESTS
// =============================================================================

func (s *BarbarianEncounterSuite) TestUnarmoredDefense_ACCalculation() {
	s.Run("Unarmored Defense: AC = 10 + DEX + CON", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  BARBARIAN UNARMORED DEFENSE: AC Calculation                     ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// Barbarian stats: DEX 14 (+2), CON 16 (+3)
		// Unarmored Defense: 10 + 2 + 3 = 15
		expectedAC := 15

		actualAC := s.barbarian.AC()
		s.Equal(expectedAC, actualAC, "Unarmored Defense should calculate AC correctly")

		s.T().Logf("  Ability Scores:")
		s.T().Logf("    DEX: 14 (+2)")
		s.T().Logf("    CON: 16 (+3)")
		s.T().Log("")
		s.T().Log("  Unarmored Defense:")
		s.T().Log("    Base:          10")
		s.T().Log("    + DEX mod:      2")
		s.T().Log("    + CON mod:      3")
		s.T().Logf("    = AC:          %d", expectedAC)
		s.T().Log("")
		s.T().Log("✓ Unarmored Defense AC calculated correctly")
	})
}

// =============================================================================
// MULTI-TURN ENCOUNTER SCENARIO
// =============================================================================

func (s *BarbarianEncounterSuite) TestEncounter_MultiTurnCombat() {
	s.Run("Full encounter: Barbarian vs Goblin over multiple turns", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  BARBARIAN ENCOUNTER: Multi-Turn Combat Scenario                 ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")
		s.T().Logf("  COMBATANTS:")
		s.T().Logf("    Grog the Destroyer - Level 1 Barbarian")
		s.T().Logf("      HP: %d/%d, AC: %d", s.barbarian.GetHitPoints(), s.barbarian.GetMaxHitPoints(), s.barbarian.AC())
		s.T().Logf("      Weapon: Greataxe (1d12 slashing)")
		s.T().Logf("      Rage uses: %d/2", s.barbarian.GetResource(resources.RageCharges).Current())
		s.T().Log("")
		s.T().Logf("    Goblin Scout")
		s.T().Logf("      HP: %d/%d, AC: %d", s.goblin.GetHitPoints(), s.goblin.GetMaxHitPoints(), s.goblin.AC())
		s.T().Log("")

		// Track rage status
		var rageActive bool
		var rageEndReason string

		appliedTopic := dnd5eEvents.ConditionAppliedTopic.On(s.bus)
		_, err := appliedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
			if event.Type == dnd5eEvents.ConditionRaging {
				rageActive = true
			}
			return nil
		})
		s.Require().NoError(err)

		removedTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
		_, err = removedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			if event.ConditionRef == "dnd5e:conditions:raging" {
				rageActive = false
				rageEndReason = event.Reason
			}
			return nil
		})
		s.Require().NoError(err)

		turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)

		// ─── ROUND 1 ───
		s.T().Log("═══════════════════════════════════════════════════════════════════")
		s.T().Log("  ROUND 1")
		s.T().Log("═══════════════════════════════════════════════════════════════════")

		// Grog activates rage (bonus action) and attacks
		s.T().Log("→ Grog's turn:")
		s.T().Log("  [Bonus Action] Grog enters a RAGE!")
		rage := s.barbarian.GetFeature("rage")
		err = rage.Activate(s.ctx, s.barbarian, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)
		s.True(rageActive, "Rage should be active")
		s.T().Logf("    Rage uses: %d/2", s.barbarian.GetResource(resources.RageCharges).Current())

		postRoll := &dnd5eEvents.PostAttackRollEvent{
			AttackerID: s.barbarian.GetID(), TargetID: s.goblin.GetID(),
			OriginalAC: s.goblin.AC(), AttackRoll: 18, AttackBonus: 5,
			TotalAttack: 23, WouldHit: true,
		}
		postChain := events.NewStagedChain[*dnd5eEvents.PostAttackRollEvent](combat.ModifierStages)
		postTopic := dnd5eEvents.PostAttackRollChain.On(s.bus)
		modifiedPost, err := postTopic.PublishWithChain(s.ctx, postRoll, postChain)
		s.Require().NoError(err)
		_, err = modifiedPost.Execute(s.ctx, postRoll)
		s.Require().NoError(err)

		// Publish the canonical typed damage fold. Raging observes the fold and
		// records this as combat activity while contributing its +2 component.
		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID:      s.barbarian.GetID(),
			TargetID:        s.goblin.GetID(),
			AbilityUsed:     abilities.STR,
			AbilityModifier: 3,
			IsMelee:         true,
			Components: []dnd5eEvents.DamageComponent{{
				Source: dnd5eEvents.DamageSourceWeapon, Properties: []damage.Property{damage.AddsAttackAbilityModifier}, OriginalDiceRolls: []int{10}, FinalDiceRolls: []int{10}, DamageType: damage.Slashing,
			}, {
				Source: dnd5eEvents.DamageSourceAbility, FlatBonus: 3, DamageType: damage.Slashing,
			}},
		}
		damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		damageTopic := dnd5eEvents.DamageChain.On(s.bus)
		modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, damageChain)
		s.Require().NoError(err)
		finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
		s.Require().NoError(err)
		var abilityBonus, rageBonus int
		for _, component := range finalEvent.Components {
			switch component.Source {
			case dnd5eEvents.DamageSourceAbility:
				abilityBonus += component.FlatBonus
			case dnd5eEvents.DamageSourceCondition:
				rageBonus += component.FlatBonus
			}
		}
		s.Equal(3, abilityBonus, "typed ability component should contribute +3")
		s.Equal(2, rageBonus, "raging condition should contribute +2")
		_, foldedDamage := combat.FinalDamage(finalEvent.Components)
		s.Equal(15, foldedDamage, "typed damage fold should total weapon 10 + ability 3 + rage 2")
		s.T().Logf("    Roll: 1d20(%d)+5 = %d vs AC 13 → HIT!", 18, 23)
		s.T().Logf("    Damage fold: weapon 1d12(%d) + ability +%d + rage +%d = %d", 10, abilityBonus, rageBonus, foldedDamage)

		s.T().Log("")
		s.T().Logf("  Goblin HP remains %d/%d; this scenario verifies the canonical fold", s.goblin.GetHitPoints(), s.goblin.GetMaxHitPoints())

		// End round 1
		_ = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
			SubjectID: s.barbarian.GetID(),
			Round:     1,
		})

		s.True(rageActive, "Rage should continue (attacked this turn)")
		s.T().Log("")
		s.T().Log("  ✓ Rage continues (dealt damage)")

		s.T().Log("")
		s.T().Log("═══════════════════════════════════════════════════════════════════")
		s.T().Log("  COMBAT SUMMARY")
		s.T().Log("═══════════════════════════════════════════════════════════════════")
		s.T().Logf("  Rounds: 1")
		s.T().Logf("  Grog HP: %d/%d", s.barbarian.GetHitPoints(), s.barbarian.GetMaxHitPoints())
		s.T().Logf("  Goblin HP: %d/%d", s.goblin.GetHitPoints(), s.goblin.GetMaxHitPoints())
		s.T().Logf("  Rage status: Active")
		s.T().Logf("  Rage uses remaining: %d/2", s.barbarian.GetResource(resources.RageCharges).Current())
		s.T().Log("")

		// Simulate rage ending (no more enemies)
		s.T().Log("─── After combat ───")
		s.T().Log("→ With no enemies left, Grog doesn't attack")
		_ = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
			SubjectID: s.barbarian.GetID(),
			Round:     2,
		})
		s.False(rageActive, "Rage should end with no combat activity")
		s.Equal("no_combat_activity", rageEndReason)
		s.T().Log("  Grog's rage subsides (no combat activity)")
		s.T().Log("")
		s.T().Log("✓ Encounter completed successfully")
	})
}

func TestBarbarianEncounterSuite(t *testing.T) {
	suite.Run(t, new(BarbarianEncounterSuite))
}
