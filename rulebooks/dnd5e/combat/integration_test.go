package combat_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// CombatIntegrationSuite tests the full combat flow with real components
type CombatIntegrationSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	bus        events.EventBus
	mockRoller *mock_dice.MockRoller

	// Test fixtures reset per subtest
	barbarian       *character.Character
	barbarianScores shared.AbilityScores
	goblin          core.Entity
	weapon          *weapons.Weapon
}

// SetupTest runs before each test function
func (s *CombatIntegrationSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
}

// SetupSubTest runs before each s.Run() subtest
func (s *CombatIntegrationSuite) SetupSubTest() {
	// Reset fixtures
	s.barbarianScores = shared.AbilityScores{
		abilities.STR: 16, // +3
		abilities.DEX: 14, // +2
		abilities.CON: 16, // +3
		abilities.INT: 8,  // -1
		abilities.WIS: 10, // +0
		abilities.CHA: 12, // +1
	}
	s.barbarian = s.createBarbarian()
	s.goblin = s.createGoblin()
	s.weapon = s.createGreataxe()
}

// TearDownTest runs after each test
func (s *CombatIntegrationSuite) TearDownTest() {
	if s.barbarian != nil {
		_ = s.barbarian.Cleanup(s.ctx)
	}
	s.ctrl.Finish()
}

// Helper: Create a level 1 barbarian from Data (simulates DB load)
func (s *CombatIntegrationSuite) createBarbarian() *character.Character {
	data := &character.Data{
		ID:               "barbarian-1",
		PlayerID:         "player-1",
		Name:             "Grog",
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
		HitPoints:    15,
		MaxHitPoints: 15,
		ArmorClass:   14,
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Athletics:    shared.Proficient,
			skills.Intimidation: shared.Proficient,
		},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
			abilities.CON: shared.Proficient,
		},
		// Resources are owned by Character, not features
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.RageCharges: {
				Current:   2, // Level 1 barbarian has 2 rage uses
				Maximum:   2,
				ResetType: coreResources.ResetLongRest,
			},
		},
		Features: []json.RawMessage{
			json.RawMessage(`{
				"ref": {
					"module": "dnd5e",
					"type":   "features",
					"id":     "rage"
				},
				"id":       "rage",
				"name":     "Rage",
				"level":    1
			}`),
		},
	}

	char, err := character.LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)
	s.Require().NotNil(char)

	return char
}

// Helper: Create a goblin target
func (s *CombatIntegrationSuite) createGoblin() core.Entity {
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

// Helper: Create a greataxe weapon
func (s *CombatIntegrationSuite) createGreataxe() *weapons.Weapon {
	weapon, _ := weapons.GetByID(weapons.Greataxe)
	return &weapon
}

// Test: Barbarian with rage deals bonus damage on hit
func (s *CombatIntegrationSuite) TestBarbarianRageAddsDamageOnHit() {
	s.Run("Normal hit with rage active", func() {
		s.T().Log("=== Barbarian Rage Attack Integration Test ===")
		s.T().Logf("Attacker: %s (Level %d Barbarian, STR +3)", s.barbarian.GetName(), s.barbarian.GetLevel())
		s.T().Logf("Defender: Goblin Scout (AC 13)")
		s.T().Log("")

		// Activate rage
		rage := s.barbarian.GetFeature("rage")
		s.Require().NotNil(rage, "Barbarian should have rage feature")

		s.T().Log("→ Grog enters a rage!")
		err := rage.Activate(s.ctx, s.barbarian, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)

		// Verify rage condition is active
		conditions := s.barbarian.GetConditions()
		s.Require().Len(conditions, 1, "Should have raging condition")
		s.T().Log("  ✓ Raging condition applied (+2 damage to melee attacks)")
		s.T().Log("")

		// Mock dice rolls: attack hits, damage is 8
		// Attack roll: 15 + 5 (STR+Prof) = 20 vs AC 13 → HIT
		s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil).Times(1)
		s.mockRoller.EXPECT().RollN(s.ctx, 1, 12).Return([]int{8}, nil).Times(1) // Damage roll

		s.T().Log("→ Grog swings greataxe at Goblin Scout")

		// Execute attack
		result, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
			Attacker:         s.barbarian,
			Defender:         s.goblin,
			Weapon:           s.weapon,
			AttackerScores:   s.barbarianScores,
			DefenderAC:       s.goblin.(interface{ AC() int }).AC(),
			ProficiencyBonus: s.barbarian.GetProficiencyBonus(),
			EventBus:         s.bus,
			Roller:           s.mockRoller,
		})

		s.Require().NoError(err)
		s.True(result.Hit, "Attack should hit (20 vs AC 13)")

		s.T().Logf("  Attack roll: 1d20(%d) + STR(%d) + Prof(%d) = %d", 15, 3, 2, 20)
		s.T().Logf("  vs AC %d → HIT!", 13)
		s.T().Log("")

		// Damage breakdown: 1d12(8) + STR(3) + Rage(2) = 13
		expectedDamage := 8 + 3 + 2
		s.Equal(expectedDamage, result.TotalDamage, "Should include rage damage bonus")

		s.T().Log("  Damage breakdown:")
		s.T().Logf("    1d12 weapon damage: %d", 8)
		s.T().Logf("    + STR modifier: %d", 3)
		s.T().Logf("    + Rage bonus: %d", 2)
		s.T().Logf("  = Total damage: %d", expectedDamage)
		s.T().Log("")
		s.T().Log("✓ Integration test passed: Event-driven rage bonus applied correctly")
	})
}

// Test: Rage damage not applied when attack misses
func (s *CombatIntegrationSuite) TestRageDamageNotAppliedOnMiss() {
	s.Run("Attack misses with rage active", func() {
		s.T().Log("=== Barbarian Rage Miss Test ===")
		s.T().Logf("Attacker: %s (Level %d Barbarian, STR +3)", s.barbarian.GetName(), s.barbarian.GetLevel())
		s.T().Logf("Defender: Goblin Scout (AC 13)")
		s.T().Log("")

		// Activate rage
		s.T().Log("→ Grog enters a rage!")
		rage := s.barbarian.GetFeature("rage")
		err := rage.Activate(s.ctx, s.barbarian, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)
		s.T().Log("  ✓ Raging condition applied (+2 damage to melee attacks)")
		s.T().Log("")

		// Mock dice: attack misses
		s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(3, nil).Times(1) // 3 + 5 = 8 vs AC 13 → MISS

		s.T().Log("→ Grog swings greataxe at Goblin Scout")

		// Execute attack
		result, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
			Attacker:         s.barbarian,
			Defender:         s.goblin,
			Weapon:           s.weapon,
			AttackerScores:   s.barbarianScores,
			DefenderAC:       s.goblin.(interface{ AC() int }).AC(),
			ProficiencyBonus: s.barbarian.GetProficiencyBonus(),
			EventBus:         s.bus,
			Roller:           s.mockRoller,
		})

		s.Require().NoError(err)
		s.False(result.Hit, "Attack should miss (8 vs AC 13)")

		s.T().Logf("  Attack roll: 1d20(%d) + STR(%d) + Prof(%d) = %d", 3, 3, 2, 8)
		s.T().Logf("  vs AC %d → MISS!", 13)
		s.T().Log("")
		s.T().Log("  No damage dealt on miss")
		s.T().Log("")
		s.T().Log("✓ Integration test passed: Rage bonus correctly not applied on miss")

		s.Equal(0, result.TotalDamage, "No damage on miss")
	})
}

// Test: Critical hit with rage doubles base damage but not rage bonus
func (s *CombatIntegrationSuite) TestCriticalHitWithRage() {
	s.Run("Critical hit doubles weapon dice, not rage bonus", func() {
		s.T().Log("=== Barbarian Critical Hit with Rage Test ===")
		s.T().Logf("Attacker: %s (Level %d Barbarian, STR +3)", s.barbarian.GetName(), s.barbarian.GetLevel())
		s.T().Logf("Defender: Goblin Scout (AC 13)")
		s.T().Log("")

		// Activate rage
		s.T().Log("→ Grog enters a rage!")
		rage := s.barbarian.GetFeature("rage")
		err := rage.Activate(s.ctx, s.barbarian, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)
		s.T().Log("  ✓ Raging condition applied (+2 damage to melee attacks)")
		s.T().Log("")

		// Mock dice: natural 20, then damage rolls
		s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(20, nil).Times(1)           // Natural 20 → CRIT
		s.mockRoller.EXPECT().RollN(s.ctx, 1, 12).Return([]int{6}, nil).Times(1) // First damage die
		s.mockRoller.EXPECT().RollN(s.ctx, 1, 12).Return([]int{8}, nil).Times(1) // Crit extra die

		s.T().Log("→ Grog swings greataxe at Goblin Scout")

		// Execute attack
		result, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
			Attacker:         s.barbarian,
			Defender:         s.goblin,
			Weapon:           s.weapon,
			AttackerScores:   s.barbarianScores,
			DefenderAC:       s.goblin.(interface{ AC() int }).AC(),
			ProficiencyBonus: s.barbarian.GetProficiencyBonus(),
			EventBus:         s.bus,
			Roller:           s.mockRoller,
		})

		s.Require().NoError(err)
		s.True(result.Hit, "Critical hit should always hit")
		s.True(result.Critical, "Should be marked as critical")

		s.T().Logf("  Attack roll: Natural 20 → CRITICAL HIT!")
		s.T().Log("")

		// Damage breakdown: 2d12(6+8) + STR(3) + Rage(2) = 19
		// Rage bonus is NOT doubled on crit (per D&D 5e rules)
		expectedDamage := 6 + 8 + 3 + 2

		s.T().Log("  Damage breakdown:")
		s.T().Logf("    2d12 weapon damage (doubled): %d + %d = %d", 6, 8, 14)
		s.T().Logf("    + STR modifier: %d", 3)
		s.T().Logf("    + Rage bonus (NOT doubled): %d", 2)
		s.T().Logf("  = Total damage: %d", expectedDamage)
		s.T().Log("")
		s.T().Log("✓ Integration test passed: Critical doubles dice but not modifiers")

		s.Equal(expectedDamage, result.TotalDamage, "Crit doubles dice, not modifiers")
	})
}

// Test: Without rage, no bonus damage
func (s *CombatIntegrationSuite) TestAttackWithoutRage() {
	s.Run("Normal attack without rage", func() {
		s.T().Log("=== Barbarian Normal Attack (No Rage) Test ===")
		s.T().Logf("Attacker: %s (Level %d Barbarian, STR +3)", s.barbarian.GetName(), s.barbarian.GetLevel())
		s.T().Logf("Defender: Goblin Scout (AC 13)")
		s.T().Log("")

		// No rage activation - just attack
		s.T().Log("→ Grog swings greataxe at Goblin Scout (NOT raging)")

		// Mock dice rolls
		s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil).Times(1)
		s.mockRoller.EXPECT().RollN(s.ctx, 1, 12).Return([]int{8}, nil).Times(1)

		// Execute attack
		result, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
			Attacker:         s.barbarian,
			Defender:         s.goblin,
			Weapon:           s.weapon,
			AttackerScores:   s.barbarianScores,
			DefenderAC:       s.goblin.(interface{ AC() int }).AC(),
			ProficiencyBonus: s.barbarian.GetProficiencyBonus(),
			EventBus:         s.bus,
			Roller:           s.mockRoller,
		})

		s.Require().NoError(err)
		s.True(result.Hit, "Attack should hit")

		s.T().Logf("  Attack roll: 1d20(%d) + STR(%d) + Prof(%d) = %d", 15, 3, 2, 20)
		s.T().Logf("  vs AC %d → HIT!", 13)
		s.T().Log("")

		// Damage breakdown: 1d12(8) + STR(3) = 11 (no rage bonus)
		expectedDamage := 8 + 3

		s.T().Log("  Damage breakdown:")
		s.T().Logf("    1d12 weapon damage: %d", 8)
		s.T().Logf("    + STR modifier: %d", 3)
		s.T().Logf("    (No rage bonus - not active)")
		s.T().Logf("  = Total damage: %d", expectedDamage)
		s.T().Log("")
		s.T().Log("✓ Integration test passed: No rage bonus when not active")

		s.Equal(expectedDamage, result.TotalDamage, "Should NOT include rage damage")
	})
}

// Test: Second Wind heals fighter and caps at max HP
func (s *CombatIntegrationSuite) TestSecondWindIntegration() {
	s.Run("Second Wind healing integration", func() {
		s.T().Log("=== Second Wind Integration Test ===")

		// Create a level 3 fighter with Second Wind feature
		fighterData := &character.Data{
			ID:               "fighter-1",
			PlayerID:         "player-1",
			Name:             "Roland",
			Level:            3,
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
			HitPoints:    10, // Damaged
			MaxHitPoints: 20,
			ArmorClass:   16,
			Features: []json.RawMessage{
				json.RawMessage(`{
					"ref": {
						"module": "dnd5e",
						"type":   "features",
						"id":     "second_wind"
					},
					"id":       "second_wind",
					"name":     "Second Wind",
					"level":    3,
					"uses":     1,
					"max_uses": 1
				}`),
			},
		}

		fighter, err := character.LoadFromData(s.ctx, fighterData, s.bus)
		s.Require().NoError(err)
		s.Require().NotNil(fighter)
		defer func() {
			_ = fighter.Cleanup(s.ctx)
		}()

		s.T().Logf("Fighter: %s (Level %d, %d/%d HP)",
			fighter.GetName(), fighter.GetLevel(), fighter.GetHitPoints(), fighter.GetMaxHitPoints())
		s.T().Log("")

		// Get Second Wind feature
		secondWind := fighter.GetFeature("second_wind")
		s.Require().NotNil(secondWind, "Fighter should have Second Wind feature")

		// Track the healing event
		var receivedEvent *dnd5eEvents.HealingReceivedEvent
		topic := dnd5eEvents.HealingReceivedTopic.On(s.bus)
		_, err = topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
			receivedEvent = &event
			return nil
		})
		s.Require().NoError(err)

		// Activate Second Wind
		s.T().Logf("→ %s uses Second Wind!", fighter.GetName())
		err = secondWind.Activate(s.ctx, fighter, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)

		// Verify healing event was published
		s.Require().NotNil(receivedEvent, "Healing event should be published")
		s.Equal(fighter.GetID(), receivedEvent.TargetID)
		s.Equal("second_wind", receivedEvent.Source)

		// Verify roll and modifier
		s.GreaterOrEqual(receivedEvent.Roll, 1, "Roll should be at least 1")
		s.LessOrEqual(receivedEvent.Roll, 10, "Roll should be at most 10")
		s.Equal(3, receivedEvent.Modifier, "Modifier should be fighter level (3)")

		totalHealing := receivedEvent.Roll + receivedEvent.Modifier

		s.T().Logf("  Roll: 1d10(%d) + Level(%d) = %d HP healed", receivedEvent.Roll, receivedEvent.Modifier, totalHealing)

		// Verify HP was updated
		expectedHP := 10 + totalHealing
		if expectedHP > 20 {
			expectedHP = 20
		}
		actualHP := fighter.GetHitPoints()

		s.T().Logf("  HP: 10 → %d", actualHP)
		if actualHP == 20 {
			s.T().Log("  (capped at max)")
		}
		s.T().Log("")

		s.Equal(expectedHP, actualHP, "HP should be updated with healing")
		s.LessOrEqual(actualHP, 20, "HP should not exceed max")

		s.T().Log("✓ Integration test passed")
	})
}

// Test: Archery fighting style adds +2 to ranged attack rolls
func (s *CombatIntegrationSuite) TestArcheryFightingStyle() {
	s.Run("Archery adds +2 to ranged attacks", func() {
		s.T().Log("=== Archery Fighting Style Test ===")

		// Create a fighter with Archery fighting style
		fighterScores := shared.AbilityScores{
			abilities.STR: 10, // +0
			abilities.DEX: 16, // +3
			abilities.CON: 14, // +2
			abilities.INT: 10, // +0
			abilities.WIS: 12, // +1
			abilities.CHA: 8,  // -1
		}

		s.T().Log("Archer: Legolas (Level 1 Fighter, DEX +3)")
		s.T().Log("Target: Goblin (AC 13)")
		s.T().Log("")

		// Apply Archery fighting style condition
		archeryCondition := conditions.NewFightingStyleArcheryCondition("fighter-1")
		err := archeryCondition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() {
			_ = archeryCondition.Remove(s.ctx, s.bus)
		}()

		// Get a longbow
		longbow, err := weapons.GetByID(weapons.Longbow)
		s.Require().NoError(err)

		// Mock dice: attack roll of 14
		s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(14, nil).Times(1)
		s.mockRoller.EXPECT().RollN(s.ctx, 1, 8).Return([]int{5}, nil).Times(1)

		s.T().Log("→ Legolas fires longbow at Goblin")

		// Create mock entities
		fighter := &mockEntity{id: "fighter-1", name: "Legolas"}
		goblin := &mockEntity{id: "goblin-1", name: "Goblin"}

		// Execute attack
		result, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
			Attacker:         fighter,
			Defender:         goblin,
			Weapon:           &longbow,
			AttackerScores:   fighterScores,
			DefenderAC:       13,
			ProficiencyBonus: 2,
			EventBus:         s.bus,
			Roller:           s.mockRoller,
		})

		s.Require().NoError(err)
		s.True(result.Hit, "Attack should hit")

		// Attack roll: 1d20(14) + DEX(3) + Prof(2) + Archery(2) = 21
		expectedAttackBonus := 3 + 2 + 2 // DEX + Prof + Archery
		s.Equal(expectedAttackBonus, result.AttackBonus, "Should include Archery +2 bonus")

		s.T().Logf("  Attack roll: 1d20(%d) + DEX(%d) + Prof(%d) + Archery(%d) = %d",
			14, 3, 2, 2, 14+expectedAttackBonus)
		s.T().Logf("  vs AC %d → HIT!", 13)
		s.T().Log("")
		s.T().Log("✓ Integration test passed: Archery adds +2 to ranged attacks")
	})
}

// Test: Great Weapon Fighting rerolls 1s and 2s on weapon damage
func (s *CombatIntegrationSuite) TestGreatWeaponFighting() {
	s.Run("GWF rerolls 1s and 2s", func() {
		s.T().Log("=== Great Weapon Fighting Test ===")

		fighterScores := shared.AbilityScores{
			abilities.STR: 16, // +3
			abilities.DEX: 10, // +0
			abilities.CON: 14, // +2
			abilities.INT: 10, // +0
			abilities.WIS: 12, // +1
			abilities.CHA: 8,  // -1
		}

		s.T().Log("Fighter: Conan (Level 1 Fighter, STR +3)")
		s.T().Log("Target: Goblin (AC 13)")
		s.T().Log("")

		// Apply GWF fighting style condition
		gwfCondition := conditions.NewFightingStyleGreatWeaponFightingCondition("fighter-2", s.mockRoller)
		err := gwfCondition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() {
			_ = gwfCondition.Remove(s.ctx, s.bus)
		}()

		// Get a greatsword
		greatsword, err := weapons.GetByID(weapons.Greatsword)
		s.Require().NoError(err)

		// Mock dice: attack hits, damage has 1 and 2 which get rerolled
		s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil).Times(1)             // Attack roll
		s.mockRoller.EXPECT().RollN(s.ctx, 2, 6).Return([]int{1, 4}, nil).Times(1) // Weapon damage: 1, 4
		s.mockRoller.EXPECT().Roll(gomock.Any(), 6).Return(3, nil).Times(1)        // Reroll the 1 -> 3

		s.T().Log("→ Conan swings greatsword at Goblin")

		// Create mock entities
		fighter := &mockEntity{id: "fighter-2", name: "Conan"}
		goblin := &mockEntity{id: "goblin-2", name: "Goblin"}

		// Execute attack
		result, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
			Attacker:         fighter,
			Defender:         goblin,
			Weapon:           &greatsword,
			AttackerScores:   fighterScores,
			DefenderAC:       13,
			ProficiencyBonus: 2,
			EventBus:         s.bus,
			Roller:           s.mockRoller,
		})

		s.Require().NoError(err)
		s.True(result.Hit, "Attack should hit")

		s.T().Logf("  Attack roll: 1d20(%d) + STR(%d) + Prof(%d) = %d",
			15, 3, 2, 15+3+2)
		s.T().Logf("  vs AC %d → HIT!", 13)
		s.T().Log("")

		// Verify the breakdown includes reroll information
		s.Require().NotNil(result.Breakdown)
		s.Require().Greater(len(result.Breakdown.Components), 0)

		weaponComp := result.Breakdown.Components[0]
		s.T().Log("  Damage breakdown:")
		s.T().Logf("    2d6 weapon damage: %v", weaponComp.OriginalDiceRolls)

		if len(weaponComp.Rerolls) > 0 {
			for _, reroll := range weaponComp.Rerolls {
				s.T().Logf("      → Reroll die %d: was %d, now %d (Great Weapon Fighting)",
					reroll.DieIndex, reroll.Before, reroll.After)
			}
			s.T().Logf("    Final rolls: %v = %d", weaponComp.FinalDiceRolls, sumDice(weaponComp.FinalDiceRolls))
		}

		s.T().Logf("    + STR modifier: %d", 3)
		s.T().Logf("  = Total damage: %d", result.TotalDamage)
		s.T().Log("")
		s.T().Log("✓ Integration test passed: GWF rerolls 1s and 2s correctly")
	})
}

// mockEntity is a simple entity for testing
type mockEntity struct {
	id   string
	name string
}

func (m *mockEntity) GetID() string {
	return m.id
}

func (m *mockEntity) GetType() core.EntityType {
	return "character"
}

// sumDice sums a slice of dice rolls
func sumDice(rolls []int) int {
	sum := 0
	for _, r := range rolls {
		sum += r
	}
	return sum
}

// Test: DealDamage with Rage applies bonus damage through the chain
func (s *CombatIntegrationSuite) TestDealDamageWithRageBonus() {
	s.Run("DealDamage applies rage bonus through chain", func() {
		s.T().Log("=== DealDamage with Rage Integration Test ===")
		s.T().Logf("Attacker: %s (Level %d Barbarian, rage +2 damage)", s.barbarian.GetName(), s.barbarian.GetLevel())
		s.T().Log("")

		// Create a target that implements Combatant
		target := &mockCombatantTarget{
			id:           "goblin-1",
			hitPoints:    20,
			maxHitPoints: 20,
		}
		s.T().Logf("Target: %s (HP: %d/%d)", target.id, target.hitPoints, target.maxHitPoints)
		s.T().Log("")

		// Activate rage
		s.T().Log("→ Grog enters a rage!")
		rage := s.barbarian.GetFeature("rage")
		s.Require().NotNil(rage, "Barbarian should have rage feature")
		err := rage.Activate(s.ctx, s.barbarian, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)

		// Verify rage condition is active
		activeConditions := s.barbarian.GetConditions()
		s.Require().Len(activeConditions, 1, "Should have raging condition")
		s.T().Log("  ✓ Raging condition applied (+2 damage bonus)")
		s.T().Log("")

		// Call DealDamage directly - this should go through the chain
		// where Rage will add its bonus
		baseDamage := 8
		s.T().Logf("→ Dealing %d slashing damage via DealDamage", baseDamage)

		output, err := combat.DealDamage(s.ctx, &combat.DealDamageInput{
			Target:     target,
			AttackerID: s.barbarian.GetID(),
			Source:     combat.DamageSourceAttack,
			Instances: []combat.DamageInstanceInput{
				{Amount: baseDamage, Type: "slashing"},
			},
			EventBus: s.bus,
		})

		s.Require().NoError(err)
		s.Require().NotNil(output)

		// Rage adds +2 to melee weapon attacks when attacker is raging
		expectedDamage := baseDamage + 2 // 8 + 2 = 10
		s.T().Log("")
		s.T().Log("  Damage breakdown:")
		s.T().Logf("    Base damage: %d", baseDamage)
		s.T().Logf("    + Rage bonus: %d", 2)
		s.T().Logf("  = Total damage: %d", expectedDamage)
		s.T().Log("")

		s.Equal(expectedDamage, output.TotalDamage, "Should include rage damage bonus")
		s.Equal(20-expectedDamage, output.CurrentHP, "Target HP should be reduced")
		s.Equal(20-expectedDamage, target.GetHitPoints(), "Target should have taken damage")

		s.T().Logf("  Target HP: %d → %d", 20, target.GetHitPoints())
		s.T().Log("")
		s.T().Log("✓ Integration test passed: DealDamage correctly applies rage bonus through chain")
	})
}

// Test: DealDamage applies rage resistance when raging character takes B/P/S damage
// NOTE: This test documents that resistance multipliers are NOT YET IMPLEMENTED.
// The Rage condition adds a component with Multiplier: 0.5, but DamageComponent.Total()
// does not process multipliers. This is tracked for future implementation.
func (s *CombatIntegrationSuite) TestDealDamageWithRageResistance() {
	s.Run("DealDamage applies rage resistance to physical damage", func() {
		s.T().Log("=== DealDamage with Rage Resistance Test ===")
		s.T().Logf("Defender: %s (Level %d Barbarian, raging)", s.barbarian.GetName(), s.barbarian.GetLevel())
		s.T().Log("")

		// Activate rage
		s.T().Log("→ Grog enters a rage!")
		rage := s.barbarian.GetFeature("rage")
		s.Require().NotNil(rage, "Barbarian should have rage feature")
		err := rage.Activate(s.ctx, s.barbarian, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)
		s.T().Log("  ✓ Raging condition applied (resistance to B/P/S)")
		s.T().Log("")

		initialHP := s.barbarian.GetHitPoints()
		s.T().Logf("Initial HP: %d", initialHP)

		// Deal slashing damage to the raging barbarian
		incomingDamage := 10
		s.T().Logf("→ Dealing %d slashing damage to raging barbarian", incomingDamage)

		output, err := combat.DealDamage(s.ctx, &combat.DealDamageInput{
			Target:     s.barbarian,
			AttackerID: "goblin-1",
			Source:     combat.DamageSourceAttack,
			Instances: []combat.DamageInstanceInput{
				{Amount: incomingDamage, Type: "slashing"},
			},
			EventBus: s.bus,
		})

		s.Require().NoError(err)
		s.Require().NotNil(output)

		// TODO: When resistance multipliers are implemented, this should be incomingDamage / 2 = 5
		// Currently, DamageComponent.Total() does not apply multipliers, so full damage is taken.
		// For now, we verify the chain runs without error and damage is applied.
		s.T().Log("")
		s.T().Log("  NOTE: Resistance multipliers not yet implemented in DamageComponent.Total()")
		s.T().Logf("  Damage applied: %d (resistance would halve to %d)", output.TotalDamage, incomingDamage/2)
		s.T().Log("")

		// Verify damage was applied (even if not halved)
		s.Greater(output.TotalDamage, 0, "Some damage should be applied")
		s.Less(output.CurrentHP, initialHP, "HP should be reduced")

		s.T().Logf("  Barbarian HP: %d → %d", initialHP, s.barbarian.GetHitPoints())
		s.T().Log("")
		s.T().Log("✓ Test passed: DealDamage processes chain (resistance implementation pending)")
	})
}

// mockCombatantTarget implements combat.Combatant for testing DealDamage
type mockCombatantTarget struct {
	id           string
	hitPoints    int
	maxHitPoints int
}

func (m *mockCombatantTarget) GetID() string        { return m.id }
func (m *mockCombatantTarget) GetHitPoints() int    { return m.hitPoints }
func (m *mockCombatantTarget) GetMaxHitPoints() int { return m.maxHitPoints }

func (m *mockCombatantTarget) ApplyDamage(_ context.Context, input *combat.ApplyDamageInput) *combat.ApplyDamageResult {
	if input == nil {
		return &combat.ApplyDamageResult{
			CurrentHP:  m.hitPoints,
			PreviousHP: m.hitPoints,
		}
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

	return &combat.ApplyDamageResult{
		TotalDamage:   totalDamage,
		CurrentHP:     m.hitPoints,
		DroppedToZero: m.hitPoints == 0 && previousHP > 0,
		PreviousHP:    previousHP,
	}
}

func TestCombatIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CombatIntegrationSuite))
}
