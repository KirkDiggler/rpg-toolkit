package combat_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
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
		Features: []json.RawMessage{
			json.RawMessage(`{
				"ref": {
					"module": "dnd5e",
					"type":   "features",
					"value":  "rage"
				},
				"id":       "rage",
				"name":     "Rage",
				"level":    1,
				"uses":     2,
				"max_uses": 2
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

func TestCombatIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CombatIntegrationSuite))
}
