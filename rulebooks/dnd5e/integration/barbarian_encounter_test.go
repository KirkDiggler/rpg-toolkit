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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
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
	lookup     *combatantLookup
	room       spatial.Room

	barbarian *character.Character
	goblin    *monster.Monster
	greataxe  *weapons.Weapon
}

// combatantLookup provides combatant lookup for tests
type combatantLookup struct {
	combatants map[string]combat.Combatant
}

func newCombatantLookup() *combatantLookup {
	return &combatantLookup{combatants: make(map[string]combat.Combatant)}
}

func (l *combatantLookup) Add(c combat.Combatant) {
	l.combatants[c.GetID()] = c
}

func (l *combatantLookup) Get(id string) (combat.Combatant, error) {
	if c, ok := l.combatants[id]; ok {
		return c, nil
	}
	return nil, nil
}

func (s *BarbarianEncounterSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.bus = events.NewEventBus()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
	s.lookup = newCombatantLookup()
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
	s.lookup = newCombatantLookup()

	// Create barbarian and goblin
	s.barbarian = s.createLevel1Barbarian()
	s.goblin = s.createGoblin()
	s.greataxe = s.createGreataxe()

	s.lookup.Add(s.barbarian)
	s.lookup.Add(s.goblin)

	// Set up context with combatant lookup for ResolveAttack
	s.ctx = combat.WithCombatantLookup(context.Background(), s.lookup)

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
	weapon, _ := weapons.GetByID(weapons.Greataxe)
	return &weapon
}

// =============================================================================
// LEVEL 1: RAGE FEATURE TESTS
// =============================================================================

func (s *BarbarianEncounterSuite) TestRage_ActivationAndDamageBonus() {
	s.Run("Activating rage consumes a use and adds +2 melee damage", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  BARBARIAN RAGE: Activation and Damage Bonus                     ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")
		s.T().Logf("  Barbarian: %s (Level 1, STR +3, Prof +2)", s.barbarian.GetName())
		s.T().Logf("  Target: Goblin Scout (AC 13, HP 7)")
		s.T().Log("")

		// Check initial rage uses
		rageUses := s.barbarian.GetResource(resources.RageCharges).Current()
		s.Equal(2, rageUses, "Level 1 barbarian should have 2 rage uses")
		s.T().Logf("  Initial Rage uses: %d/2", rageUses)

		// Activate rage
		rage := s.barbarian.GetFeature("rage")
		s.Require().NotNil(rage, "Barbarian should have Rage feature")

		s.T().Log("")
		s.T().Log("→ Grog roars and ENTERS A RAGE!")
		err := rage.Activate(s.ctx, s.barbarian, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)

		// Verify rage was consumed
		rageUsesAfter := s.barbarian.GetResource(resources.RageCharges).Current()
		s.Equal(1, rageUsesAfter, "Rage use should be consumed")
		s.T().Logf("  Rage uses remaining: %d/2", rageUsesAfter)

		// Verify raging condition is active
		charConditions := s.barbarian.GetConditions()
		s.Require().Len(charConditions, 1, "Should have Raging condition")
		ragingCond, ok := charConditions[0].(*conditions.RagingCondition)
		s.Require().True(ok, "Condition should be RagingCondition")
		s.Equal(2, ragingCond.DamageBonus, "Level 1 rage should give +2 damage")
		s.T().Log("  ✓ Raging condition active (+2 damage, B/P/S resistance)")
		s.T().Log("")

		// Attack with rage active
		s.T().Log("→ Grog swings his greataxe!")

		// Mock dice: attack roll 15 (hits), damage roll 8
		s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil).Times(1)
		s.mockRoller.EXPECT().RollN(s.ctx, 1, 12).Return([]int{8}, nil).Times(1)

		result, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
			AttackerID: s.barbarian.GetID(),
			TargetID:   s.goblin.GetID(),
			Weapon:     s.greataxe,
			EventBus:   s.bus,
			Roller:     s.mockRoller,
		})
		s.Require().NoError(err)
		s.True(result.Hit, "Attack should hit")

		// Damage breakdown: 1d12(8) + STR(3) + Rage(2) = 13
		expectedDamage := 8 + 3 + 2
		s.Equal(expectedDamage, result.TotalDamage, "Should include rage damage bonus")

		s.T().Logf("  Attack: 1d20(%d) + STR(%d) + Prof(%d) = %d vs AC 13 → HIT!", 15, 3, 2, 20)
		s.T().Log("  Damage breakdown:")
		s.T().Logf("    1d12 greataxe:  %d", 8)
		s.T().Logf("    + STR modifier: %d", 3)
		s.T().Logf("    + Rage bonus:   %d", 2)
		s.T().Logf("    = Total:        %d damage", expectedDamage)
		s.T().Log("")
		s.T().Log("✓ Rage correctly adds +2 damage to melee attacks")
	})
}

func (s *BarbarianEncounterSuite) TestRage_ContinuesWhenBarbarianHitsEnemy() {
	s.Run("Rage continues when barbarian successfully attacks", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  BARBARIAN RAGE: Continues When Landing Attacks                  ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// Activate rage
		rage := s.barbarian.GetFeature("rage")
		err := rage.Activate(s.ctx, s.barbarian, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)
		s.T().Log("→ Grog enters a rage!")

		// Track if rage ends
		var rageEnded bool
		removedTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
		_, err = removedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			if event.ConditionRef == "dnd5e:conditions:raging" {
				rageEnded = true
			}
			return nil
		})
		s.Require().NoError(err)

		// TURN 1: Attack hits
		s.T().Log("")
		s.T().Log("─── ROUND 1 ───")
		s.T().Log("→ Grog attacks the goblin")

		s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil).Times(1)
		s.mockRoller.EXPECT().RollN(s.ctx, 1, 12).Return([]int{6}, nil).Times(1)

		result, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
			AttackerID: s.barbarian.GetID(),
			TargetID:   s.goblin.GetID(),
			Weapon:     s.greataxe,
			EventBus:   s.bus,
			Roller:     s.mockRoller,
		})
		s.Require().NoError(err)
		s.True(result.Hit)
		s.T().Logf("  Attack hits for %d damage", result.TotalDamage)

		// End turn 1
		turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
		err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
			CharacterID: s.barbarian.GetID(),
			Round:       1,
		})
		s.Require().NoError(err)
		s.T().Log("→ End of Grog's turn")

		// Verify rage continues
		s.False(rageEnded, "Rage should NOT end when barbarian hit an enemy")
		s.T().Log("  ✓ Rage continues (attacked enemy this turn)")

		s.T().Log("")
		s.T().Log("✓ Rage correctly continues when barbarian lands attacks")
	})
}

func (s *BarbarianEncounterSuite) TestRage_ContinuesWhenBarbarianTakesDamage() {
	s.Run("Rage continues when barbarian takes damage", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  BARBARIAN RAGE: Continues When Taking Damage                    ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// Activate rage
		rage := s.barbarian.GetFeature("rage")
		err := rage.Activate(s.ctx, s.barbarian, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)
		s.T().Log("→ Grog enters a rage!")

		// Track if rage ends
		var rageEnded bool
		removedTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
		_, err = removedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			if event.ConditionRef == "dnd5e:conditions:raging" {
				rageEnded = true
			}
			return nil
		})
		s.Require().NoError(err)

		// TURN 1: Barbarian doesn't attack but takes damage
		s.T().Log("")
		s.T().Log("─── ROUND 1 ───")
		s.T().Log("→ Grog holds his action (doesn't attack)")
		s.T().Log("→ Goblin stabs Grog!")

		// Publish damage received event (goblin hit the barbarian)
		damageTopic := dnd5eEvents.DamageReceivedTopic.On(s.bus)
		err = damageTopic.Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
			TargetID: s.barbarian.GetID(),
			SourceID: s.goblin.GetID(),
			Amount:   5,
		})
		s.Require().NoError(err)
		s.T().Logf("  Grog takes 5 damage")

		// End turn 1
		turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
		err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
			CharacterID: s.barbarian.GetID(),
			Round:       1,
		})
		s.Require().NoError(err)
		s.T().Log("→ End of Grog's turn")

		// Verify rage continues
		s.False(rageEnded, "Rage should NOT end when barbarian took damage")
		s.T().Log("  ✓ Rage continues (took damage this turn)")

		s.T().Log("")
		s.T().Log("✓ Rage correctly continues when barbarian takes damage")
	})
}

func (s *BarbarianEncounterSuite) TestRage_EndsWithNoCombatActivity() {
	s.Run("Rage ends when barbarian neither attacks nor takes damage", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  BARBARIAN RAGE: Ends Without Combat Activity                    ║")
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

		// TURN 1: No combat activity
		s.T().Log("")
		s.T().Log("─── ROUND 1 ───")
		s.T().Log("→ Grog moves around but doesn't attack")
		s.T().Log("→ No enemies attack Grog")

		// End turn 1 with NO combat activity
		turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
		err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
			CharacterID: s.barbarian.GetID(),
			Round:       1,
		})
		s.Require().NoError(err)
		s.T().Log("→ End of Grog's turn")

		// Verify rage ended
		s.Equal("no_combat_activity", rageEndReason, "Rage should end due to no combat activity")
		s.T().Log("  ✗ Rage ends (no attacks made, no damage taken)")

		s.T().Log("")
		s.T().Log("✓ Rage correctly ends when neither attacking nor taking damage")
	})
}

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
				CharacterID: s.barbarian.GetID(),
				Round:       round,
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

func (s *BarbarianEncounterSuite) TestRage_ResistanceHalvesPhysicalDamage() {
	s.Run("Rage resistance halves bludgeoning/piercing/slashing damage", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  BARBARIAN RAGE: Resistance to Physical Damage                   ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		initialHP := s.barbarian.GetHitPoints()
		s.T().Logf("  Grog's HP: %d/%d", initialHP, s.barbarian.GetMaxHitPoints())

		// Activate rage
		rage := s.barbarian.GetFeature("rage")
		err := rage.Activate(s.ctx, s.barbarian, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)
		s.T().Log("")
		s.T().Log("→ Grog enters a rage!")
		s.T().Log("  ✓ Resistance to bludgeoning, piercing, slashing")
		s.T().Log("")

		// Deal slashing damage to the barbarian
		incomingDamage := 10
		s.T().Logf("→ Goblin deals %d slashing damage to Grog", incomingDamage)

		output, err := combat.DealDamage(s.ctx, &combat.DealDamageInput{
			Target:     s.barbarian,
			AttackerID: s.goblin.GetID(),
			Source:     combat.DamageSourceAttack,
			Instances: []combat.DamageInstanceInput{
				{Amount: incomingDamage, Type: "slashing"},
			},
			EventBus: s.bus,
		})
		s.Require().NoError(err)

		// Resistance halves damage: 10 / 2 = 5
		expectedDamage := incomingDamage / 2
		s.Equal(expectedDamage, output.TotalDamage, "Resistance should halve physical damage")

		s.T().Log("")
		s.T().Log("  Damage calculation:")
		s.T().Logf("    Incoming damage:    %d", incomingDamage)
		s.T().Logf("    × Rage resistance:  0.5")
		s.T().Logf("    = Damage taken:     %d", expectedDamage)
		s.T().Log("")
		s.T().Logf("  Grog's HP: %d → %d", initialHP, output.CurrentHP)
		s.T().Log("")
		s.T().Log("✓ Rage resistance correctly halves physical damage")
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
		_, _ = appliedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
			if event.Type == dnd5eEvents.ConditionRaging {
				rageActive = true
			}
			return nil
		})

		removedTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
		_, _ = removedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			if event.ConditionRef == "dnd5e:conditions:raging" {
				rageActive = false
				rageEndReason = event.Reason
			}
			return nil
		})

		turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)

		// ─── ROUND 1 ───
		s.T().Log("═══════════════════════════════════════════════════════════════════")
		s.T().Log("  ROUND 1")
		s.T().Log("═══════════════════════════════════════════════════════════════════")

		// Grog activates rage (bonus action) and attacks
		s.T().Log("→ Grog's turn:")
		s.T().Log("  [Bonus Action] Grog enters a RAGE!")
		rage := s.barbarian.GetFeature("rage")
		err := rage.Activate(s.ctx, s.barbarian, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)
		s.True(rageActive, "Rage should be active")
		s.T().Logf("    Rage uses: %d/2", s.barbarian.GetResource(resources.RageCharges).Current())

		// Attack goblin
		s.T().Log("  [Action] Attack with greataxe")
		s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(18, nil).Times(1)
		s.mockRoller.EXPECT().RollN(s.ctx, 1, 12).Return([]int{10}, nil).Times(1)

		result, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
			AttackerID: s.barbarian.GetID(),
			TargetID:   s.goblin.GetID(),
			Weapon:     s.greataxe,
			EventBus:   s.bus,
			Roller:     s.mockRoller,
		})
		s.Require().NoError(err)
		s.True(result.Hit)
		s.T().Logf("    Roll: 1d20(%d)+5 = %d vs AC 13 → HIT!", 18, 23)
		s.T().Logf("    Damage: 1d12(%d)+3+2(rage) = %d", 10, result.TotalDamage)

		// Goblin is likely dead (7 HP vs 15 damage)
		s.T().Log("")
		if s.goblin.GetHitPoints() <= 0 {
			s.T().Log("  💀 The goblin falls!")
		} else {
			s.T().Logf("  Goblin HP: %d/%d", s.goblin.GetHitPoints(), s.goblin.GetMaxHitPoints())
		}

		// End round 1
		_ = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
			CharacterID: s.barbarian.GetID(),
			Round:       1,
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
		s.T().Logf("  Goblin: DEFEATED")
		s.T().Logf("  Rage status: Active")
		s.T().Logf("  Rage uses remaining: %d/2", s.barbarian.GetResource(resources.RageCharges).Current())
		s.T().Log("")

		// Simulate rage ending (no more enemies)
		s.T().Log("─── After combat ───")
		s.T().Log("→ With no enemies left, Grog doesn't attack")
		_ = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
			CharacterID: s.barbarian.GetID(),
			Round:       2,
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
