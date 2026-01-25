package combat_test

import (
	"context"
	"encoding/json"
	"strings"
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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// integrationLookup provides combatant lookup for integration tests
type integrationLookup struct {
	combatants map[string]combat.Combatant
}

func newIntegrationLookup() *integrationLookup {
	return &integrationLookup{combatants: make(map[string]combat.Combatant)}
}

func (l *integrationLookup) Add(c combat.Combatant) {
	l.combatants[c.GetID()] = c
}

func (l *integrationLookup) Get(id string) (combat.Combatant, error) {
	if c, ok := l.combatants[id]; ok {
		return c, nil
	}
	return nil, nil
}

// CombatIntegrationSuite tests the full combat flow with real components
type CombatIntegrationSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	bus        events.EventBus
	mockRoller *mock_dice.MockRoller
	lookup     *integrationLookup

	// Test fixtures reset per subtest
	barbarian       *character.Character
	barbarianScores shared.AbilityScores
	goblin          combat.Combatant
	weapon          *weapons.Weapon
}

// SetupTest runs before each test function
func (s *CombatIntegrationSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.bus = events.NewEventBus()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
	s.lookup = newIntegrationLookup()
	s.ctx = combat.WithCombatantLookup(context.Background(), s.lookup)
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

	// Register combatants with lookup for ResolveAttack
	s.lookup.Add(s.barbarian)
	s.lookup.Add(s.goblin)
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
func (s *CombatIntegrationSuite) createGoblin() combat.Combatant {
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
			AttackerID: s.barbarian.GetID(),
			TargetID:   s.goblin.GetID(),
			Weapon:     s.weapon,
			EventBus:   s.bus,
			Roller:     s.mockRoller,
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
			AttackerID: s.barbarian.GetID(),
			TargetID:   s.goblin.GetID(),
			Weapon:     s.weapon,
			EventBus:   s.bus,
			Roller:     s.mockRoller,
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
			AttackerID: s.barbarian.GetID(),
			TargetID:   s.goblin.GetID(),
			Weapon:     s.weapon,
			EventBus:   s.bus,
			Roller:     s.mockRoller,
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
			AttackerID: s.barbarian.GetID(),
			TargetID:   s.goblin.GetID(),
			Weapon:     s.weapon,
			EventBus:   s.bus,
			Roller:     s.mockRoller,
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

		// Create mock combatants and register with lookup
		fighter := &mockEntity{
			id:               "fighter-1",
			name:             "Legolas",
			abilityScores:    fighterScores,
			proficiencyBonus: 2,
			ac:               16,
			hitPoints:        20,
			maxHitPoints:     20,
		}
		goblin := &mockEntity{
			id:           "goblin-1",
			name:         "Goblin",
			ac:           13,
			hitPoints:    7,
			maxHitPoints: 7,
		}
		s.lookup.Add(fighter)
		s.lookup.Add(goblin)

		// Execute attack
		result, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
			AttackerID: fighter.GetID(),
			TargetID:   goblin.GetID(),
			Weapon:     &longbow,
			EventBus:   s.bus,
			Roller:     s.mockRoller,
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

		// Create mock combatants and register with lookup
		fighter := &mockEntity{
			id:               "fighter-2",
			name:             "Conan",
			abilityScores:    fighterScores,
			proficiencyBonus: 2,
			ac:               16,
			hitPoints:        20,
			maxHitPoints:     20,
		}
		goblin := &mockEntity{
			id:           "goblin-2",
			name:         "Goblin",
			ac:           13,
			hitPoints:    7,
			maxHitPoints: 7,
		}
		s.lookup.Add(fighter)
		s.lookup.Add(goblin)

		// Execute attack
		result, err := combat.ResolveAttack(s.ctx, &combat.AttackInput{
			AttackerID: fighter.GetID(),
			TargetID:   goblin.GetID(),
			Weapon:     &greatsword,
			EventBus:   s.bus,
			Roller:     s.mockRoller,
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

// mockEntity implements combat.Combatant for simple test cases
type mockEntity struct {
	id               string
	name             string
	hitPoints        int
	maxHitPoints     int
	ac               int
	abilityScores    shared.AbilityScores
	proficiencyBonus int
}

func (m *mockEntity) GetID() string                       { return m.id }
func (m *mockEntity) GetHitPoints() int                   { return m.hitPoints }
func (m *mockEntity) GetMaxHitPoints() int                { return m.maxHitPoints }
func (m *mockEntity) AC() int                             { return m.ac }
func (m *mockEntity) IsDirty() bool                       { return false }
func (m *mockEntity) MarkClean()                          {}
func (m *mockEntity) AbilityScores() shared.AbilityScores { return m.abilityScores }
func (m *mockEntity) ProficiencyBonus() int               { return m.proficiencyBonus }

func (m *mockEntity) ApplyDamage(_ context.Context, input *combat.ApplyDamageInput) *combat.ApplyDamageResult {
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
	return &combat.ApplyDamageResult{
		TotalDamage:   totalDamage,
		CurrentHP:     m.hitPoints,
		DroppedToZero: m.hitPoints == 0 && previousHP > 0,
		PreviousHP:    previousHP,
	}
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

		// Rage provides resistance to B/P/S damage = half damage
		// 10 slashing * 0.5 = 5 damage
		expectedDamage := incomingDamage / 2
		s.T().Log("")
		s.T().Log("  Damage breakdown:")
		s.T().Logf("    Incoming damage: %d", incomingDamage)
		s.T().Logf("    × Rage resistance (0.5): %d", expectedDamage)
		s.T().Log("")

		s.Equal(expectedDamage, output.TotalDamage, "Rage should halve physical damage")
		s.Equal(initialHP-expectedDamage, output.CurrentHP, "HP should reflect halved damage")

		s.T().Logf("  Barbarian HP: %d → %d", initialHP, s.barbarian.GetHitPoints())
		s.T().Log("")
		s.T().Log("✓ Integration test passed: Rage resistance correctly halves physical damage")
	})
}

// mockCombatantTarget implements combat.Combatant for testing DealDamage
type mockCombatantTarget struct {
	id               string
	hitPoints        int
	maxHitPoints     int
	ac               int
	dirty            bool
	abilityScores    shared.AbilityScores
	proficiencyBonus int
}

func (m *mockCombatantTarget) GetID() string                       { return m.id }
func (m *mockCombatantTarget) GetHitPoints() int                   { return m.hitPoints }
func (m *mockCombatantTarget) GetMaxHitPoints() int                { return m.maxHitPoints }
func (m *mockCombatantTarget) AC() int                             { return m.ac }
func (m *mockCombatantTarget) IsDirty() bool                       { return m.dirty }
func (m *mockCombatantTarget) MarkClean()                          { m.dirty = false }
func (m *mockCombatantTarget) AbilityScores() shared.AbilityScores { return m.abilityScores }
func (m *mockCombatantTarget) ProficiencyBonus() int               { return m.proficiencyBonus }

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

// =============================================================================
// TurnManager Integration Suite
// =============================================================================

// TurnManagerIntegrationSuite tests TurnManager with real components
type TurnManagerIntegrationSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	bus        events.EventBus
	mockRoller *mock_dice.MockRoller
	lookup     *integrationLookup
	room       spatial.Room

	fighter *character.Character
	goblin  *monster.Monster
	weapon  *weapons.Weapon
}

func (s *TurnManagerIntegrationSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.bus = events.NewEventBus()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
	s.lookup = newIntegrationLookup()
	s.ctx = context.Background()

	// Create a 10x10 square grid room
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

func (s *TurnManagerIntegrationSuite) SetupSubTest() {
	s.bus = events.NewEventBus()
	s.fighter = s.createFighter()
	s.goblin = s.createGoblin()
	s.weapon = s.createLongsword()

	s.lookup = newIntegrationLookup()
	s.lookup.Add(s.fighter)
	s.lookup.Add(s.goblin)

	// Place entities in room - fighter at (2,2), goblin at (3,2) (adjacent)
	_ = s.room.PlaceEntity(s.fighter, spatial.Position{X: 2, Y: 2})
	_ = s.room.PlaceEntity(s.goblin, spatial.Position{X: 3, Y: 2})
}

func (s *TurnManagerIntegrationSuite) TearDownSubTest() {
	_ = s.room.RemoveEntity(s.fighter.GetID())
	_ = s.room.RemoveEntity(s.goblin.GetID())
	if s.fighter != nil {
		_ = s.fighter.Cleanup(s.ctx)
	}
}

func (s *TurnManagerIntegrationSuite) TearDownTest() {
	s.ctrl.Finish()
}

// createFighter creates a level 5 fighter with Extra Attack
func (s *TurnManagerIntegrationSuite) createFighter() *character.Character {
	data := &character.Data{
		ID:               "fighter-1",
		PlayerID:         "player-1",
		Name:             "Sir Reginald",
		Level:            5,
		ProficiencyBonus: 3,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 18, // +4
			abilities.DEX: 14, // +2
			abilities.CON: 16, // +3
			abilities.INT: 10, // +0
			abilities.WIS: 12, // +1
			abilities.CHA: 10, // +0
		},
		HitPoints:    44,
		MaxHitPoints: 44,
		ArmorClass:   18,
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Athletics: shared.Proficient,
		},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
			abilities.CON: shared.Proficient,
		},
	}

	char, err := character.LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)

	// Add standard combat abilities
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewAttack("attack")))
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewDash("dash")))
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewDisengage("disengage")))
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewDodge("dodge")))

	return char
}

// createGoblin creates a goblin monster
func (s *TurnManagerIntegrationSuite) createGoblin() *monster.Monster {
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

func (s *TurnManagerIntegrationSuite) createLongsword() *weapons.Weapon {
	weapon, _ := weapons.GetByID(weapons.Longsword)
	return &weapon
}

func (s *TurnManagerIntegrationSuite) createTurnManager() *combat.TurnManager {
	tm, err := combat.NewTurnManager(&combat.NewTurnManagerInput{
		Character:  s.fighter,
		Combatants: s.lookup,
		Room:       s.room,
		EventBus:   s.bus,
		Roller:     s.mockRoller,
	})
	s.Require().NoError(err)
	return tm
}

// Test: Complete fighter turn with Extra Attack (2 strikes) and movement
func (s *TurnManagerIntegrationSuite) TestFighterFullTurn() {
	s.Run("Fighter takes full turn: Attack ability -> 2 strikes -> move", func() {
		s.T().Log("=== Fighter Full Turn Integration Test ===")
		s.T().Logf("Fighter: %s (Level %d, Extra Attack)", s.fighter.GetName(), s.fighter.GetLevel())
		s.T().Logf("Target: Goblin (AC 13, HP 7)")
		s.T().Log("")

		tm := s.createTurnManager()

		// Track events
		var turnStarted, turnEnded bool

		startTopic := dnd5eEvents.TurnStartTopic.On(s.bus)
		_, _ = startTopic.Subscribe(s.ctx, func(_ context.Context, _ dnd5eEvents.TurnStartEvent) error {
			turnStarted = true
			return nil
		})

		endTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
		_, _ = endTopic.Subscribe(s.ctx, func(_ context.Context, _ dnd5eEvents.TurnEndEvent) error {
			turnEnded = true
			return nil
		})

		// 1. Start turn
		s.T().Log("→ Starting turn")
		startResult, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)
		s.True(turnStarted, "TurnStartEvent should be published")
		s.Equal(30, startResult.Economy.MovementRemaining, "Fighter has 30ft movement")
		s.Equal(1, startResult.Economy.ActionsRemaining, "Has 1 action")
		s.T().Logf("  Economy: %d action, %d movement", startResult.Economy.ActionsRemaining, startResult.Economy.MovementRemaining)
		s.T().Log("")

		// 2. Use Attack ability (grants 2 attacks for level 5 fighter)
		s.T().Log("→ Using Attack ability")
		abilityResult, err := tm.UseAbility(s.ctx, &combat.UseAbilityInput{
			AbilityRef: refs.CombatAbilities.Attack(),
		})
		s.Require().NoError(err)
		s.Equal(2, abilityResult.Economy.AttacksRemaining, "Extra Attack grants 2 attacks")
		s.Equal(0, abilityResult.Economy.ActionsRemaining, "Action consumed")
		s.T().Logf("  Granted %d attacks (1 base + 1 Extra Attack)", abilityResult.Economy.AttacksRemaining)
		s.T().Log("")

		// 3. First strike - mock dice for hit
		s.T().Log("→ First Strike at Goblin")
		s.mockRoller.EXPECT().Roll(gomock.Any(), 20).Return(15, nil).Times(1)          // Attack roll: 15 + 7 = 22 vs AC 13
		s.mockRoller.EXPECT().RollN(gomock.Any(), 1, 8).Return([]int{6}, nil).Times(1) // Damage: 1d8

		strike1, err := tm.Strike(s.ctx, &combat.StrikeInput{
			TargetID: s.goblin.GetID(),
			Weapon:   s.weapon,
		})
		s.Require().NoError(err)
		s.True(strike1.Hit, "First strike should hit (22 vs AC 13)")
		expectedDamage1 := 6 + 4 // 1d8(6) + STR(4)
		s.Equal(expectedDamage1, strike1.TotalDamage)
		s.T().Logf("  Attack: 1d20(%d) + STR(%d) + Prof(%d) = %d vs AC 13 → HIT", 15, 4, 3, 22)
		s.T().Logf("  Damage: 1d8(%d) + STR(%d) = %d", 6, 4, expectedDamage1)
		s.T().Log("")

		// Check economy after first strike
		economy := tm.GetEconomy()
		s.Equal(1, economy.AttacksRemaining, "1 attack remaining after first strike")

		// 4. Second strike - mock dice for hit
		s.T().Log("→ Second Strike at Goblin")
		s.mockRoller.EXPECT().Roll(gomock.Any(), 20).Return(12, nil).Times(1)          // Attack roll: 12 + 7 = 19 vs AC 13
		s.mockRoller.EXPECT().RollN(gomock.Any(), 1, 8).Return([]int{4}, nil).Times(1) // Damage: 1d8

		strike2, err := tm.Strike(s.ctx, &combat.StrikeInput{
			TargetID: s.goblin.GetID(),
			Weapon:   s.weapon,
		})
		s.Require().NoError(err)
		s.True(strike2.Hit, "Second strike should hit (19 vs AC 13)")
		expectedDamage2 := 4 + 4 // 1d8(4) + STR(4)
		s.Equal(expectedDamage2, strike2.TotalDamage)
		s.T().Logf("  Attack: 1d20(%d) + STR(%d) + Prof(%d) = %d vs AC 13 → HIT", 12, 4, 3, 19)
		s.T().Logf("  Damage: 1d8(%d) + STR(%d) = %d", 4, 4, expectedDamage2)
		s.T().Log("")

		// Check economy - no attacks remaining
		economy = tm.GetEconomy()
		s.Equal(0, economy.AttacksRemaining, "No attacks remaining after second strike")

		// 5. Move - fighter moves from (2,2) to (4,2) (10 feet)
		s.T().Log("→ Moving 10 feet")
		currentPos, _ := s.room.GetEntityPosition(s.fighter.GetID())
		s.Equal(float64(2), currentPos.X)
		s.Equal(float64(2), currentPos.Y)

		path := []spatial.Position{
			{X: 2, Y: 2}, // Current position
			{X: 3, Y: 2}, // Step 1 (through goblin's space - allowed for allies in this test)
			{X: 4, Y: 2}, // Step 2 - destination
		}

		moveResult, err := tm.Move(s.ctx, &combat.MoveInput{
			Path: path,
		})
		s.Require().NoError(err)
		s.Equal(2, moveResult.StepsCompleted)
		s.T().Logf("  Moved from (%d,%d) to (%d,%d)", 2, 2, 4, 2)
		s.T().Log("")

		// Verify new position (moved from 2,2 to 4,2 - same row)
		newPos, _ := s.room.GetEntityPosition(s.fighter.GetID())
		s.Equal(float64(4), newPos.X)
		s.Equal(float64(2), newPos.Y)

		// Check movement remaining
		economy = tm.GetEconomy()
		s.Equal(20, economy.MovementRemaining, "20ft remaining after 10ft move")

		// 6. End turn
		s.T().Log("→ Ending turn")
		endResult, err := tm.EndTurn(s.ctx)
		s.Require().NoError(err)
		s.True(turnEnded, "TurnEndEvent should be published")
		s.Equal(s.fighter.GetID(), endResult.CharacterID)
		s.T().Log("")

		s.T().Log("=== Summary ===")
		s.T().Logf("Total damage dealt: %d", expectedDamage1+expectedDamage2)
		s.T().Logf("Movement used: 10ft")
		s.T().Log("✓ Fighter full turn integration test passed")
	})
}

// Test: Available abilities/actions reflect economy state
func (s *TurnManagerIntegrationSuite) TestAvailabilityQueries() {
	s.Run("GetAvailableAbilities and GetAvailableActions reflect economy", func() {
		s.T().Log("=== Availability Queries Integration Test ===")

		tm := s.createTurnManager()
		_, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		// Check available abilities at start
		abilities := tm.GetAvailableAbilities(s.ctx)
		s.T().Logf("Available abilities at turn start: %d", len(abilities))

		// All abilities should be available (have action/bonus)
		attackAvail := findAbility(abilities, "attack")
		s.Require().NotNil(attackAvail)
		s.True(attackAvail.CanUse, "Attack should be available")

		dashAvail := findAbility(abilities, "dash")
		s.Require().NotNil(dashAvail)
		s.True(dashAvail.CanUse, "Dash should be available")

		// Check available actions - Strike should NOT be usable (no attacks granted yet)
		actions := tm.GetAvailableActions(s.ctx)
		s.T().Logf("Available actions before Attack ability: %d", len(actions))
		strikeBeforeAttack := findAction(actions, "strike")
		if strikeBeforeAttack != nil {
			s.False(strikeBeforeAttack.CanUse, "Strike should NOT be usable before Attack ability")
		}

		// Use Attack ability
		_, err = tm.UseAbility(s.ctx, &combat.UseAbilityInput{
			AbilityRef: refs.CombatAbilities.Attack(),
		})
		s.Require().NoError(err)

		// Now check abilities again - Attack should NOT be available (no action remaining)
		abilities = tm.GetAvailableAbilities(s.ctx)
		attackAvail = findAbility(abilities, "attack")
		s.Require().NotNil(attackAvail)
		s.False(attackAvail.CanUse, "Attack should NOT be available (action spent)")
		s.T().Logf("Attack availability after use: CanUse=%v, Reason=%s", attackAvail.CanUse, attackAvail.Reason)

		// Check actions again - Strike SHOULD be usable (attacks granted)
		actions = tm.GetAvailableActions(s.ctx)
		s.T().Logf("Available actions after Attack ability: %d", len(actions))
		strikeAfterAttack := findAction(actions, "strike")
		if strikeAfterAttack != nil {
			s.True(strikeAfterAttack.CanUse, "Strike SHOULD be usable after Attack ability")
		}

		// Consume all attacks
		s.mockRoller.EXPECT().Roll(gomock.Any(), 20).Return(5, nil).Times(2) // Two misses

		_, _ = tm.Strike(s.ctx, &combat.StrikeInput{TargetID: s.goblin.GetID(), Weapon: s.weapon})
		_, _ = tm.Strike(s.ctx, &combat.StrikeInput{TargetID: s.goblin.GetID(), Weapon: s.weapon})

		// Now Strike should NOT be available
		actions = tm.GetAvailableActions(s.ctx)
		strikeAvail := findAction(actions, "strike")
		if strikeAvail != nil {
			s.False(strikeAvail.CanUse, "Strike should NOT be available (no attacks remaining)")
			s.T().Logf("Strike availability after exhaustion: CanUse=%v, Reason=%s", strikeAvail.CanUse, strikeAvail.Reason)
		}

		_, _ = tm.EndTurn(s.ctx)
		s.T().Log("✓ Availability queries integration test passed")
	})
}

// Test: Dash ability doubles movement
func (s *TurnManagerIntegrationSuite) TestDashDoublesMovement() {
	s.Run("Dash ability grants extra movement equal to speed", func() {
		s.T().Log("=== Dash Integration Test ===")

		tm := s.createTurnManager()
		startResult, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		initialMovement := startResult.Economy.MovementRemaining
		s.T().Logf("Initial movement: %d ft", initialMovement)

		// Use Dash ability
		dashResult, err := tm.UseAbility(s.ctx, &combat.UseAbilityInput{
			AbilityRef: refs.CombatAbilities.Dash(),
		})
		s.Require().NoError(err)

		// Movement should be doubled
		expectedMovement := initialMovement * 2
		s.Equal(expectedMovement, dashResult.Economy.MovementRemaining)
		s.T().Logf("Movement after Dash: %d ft (doubled)", dashResult.Economy.MovementRemaining)

		_, _ = tm.EndTurn(s.ctx)
		s.T().Log("✓ Dash integration test passed")
	})
}

// Test: Dodge ability consumes action and activates
func (s *TurnManagerIntegrationSuite) TestDodgeConsumesAction() {
	s.Run("Dodge ability consumes action and publishes event", func() {
		s.T().Log("=== Dodge Integration Test ===")

		// Track if DodgeActivatedEvent was published
		dodgeActivated := false
		dodgeTopic := dnd5eEvents.DodgeActivatedTopic.On(s.bus)
		_, err := dodgeTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DodgeActivatedEvent) error {
			if event.CharacterID == s.fighter.GetID() {
				dodgeActivated = true
			}
			return nil
		})
		s.Require().NoError(err)

		tm := s.createTurnManager()
		result, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)
		s.Equal(1, result.Economy.ActionsRemaining)
		s.T().Logf("Actions before Dodge: %d", result.Economy.ActionsRemaining)

		// Use Dodge ability
		dodgeResult, err := tm.UseAbility(s.ctx, &combat.UseAbilityInput{
			AbilityRef: refs.CombatAbilities.Dodge(),
		})
		s.Require().NoError(err)

		// Verify action was consumed
		s.Equal(0, dodgeResult.Economy.ActionsRemaining)
		s.T().Logf("Actions after Dodge: %d", dodgeResult.Economy.ActionsRemaining)

		// Verify DodgeActivatedEvent was published
		s.True(dodgeActivated, "DodgeActivatedEvent should be published")
		s.T().Log("  ✓ DodgeActivatedEvent published")

		_, _ = tm.EndTurn(s.ctx)
		s.T().Log("✓ Dodge integration test passed")
	})
}

// Test: Cannot use actions without capacity
func (s *TurnManagerIntegrationSuite) TestStrikeRequiresCapacity() {
	s.Run("Strike fails without attack capacity", func() {
		s.T().Log("=== Strike Without Capacity Test ===")

		tm := s.createTurnManager()
		_, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		// Try to Strike without using Attack ability first
		_, err = tm.Strike(s.ctx, &combat.StrikeInput{
			TargetID: s.goblin.GetID(),
			Weapon:   s.weapon,
		})
		s.Require().Error(err, "Strike should fail without attacks")
		s.T().Logf("  Expected error: %s", err.Error())

		_, _ = tm.EndTurn(s.ctx)
		s.T().Log("✓ Strike capacity requirement test passed")
	})
}

// Test: Movement respects Path[0] validation
func (s *TurnManagerIntegrationSuite) TestMovePathValidation() {
	s.Run("Move validates Path[0] is current position", func() {
		s.T().Log("=== Move Path Validation Test ===")

		tm := s.createTurnManager()
		_, err := tm.StartTurn(s.ctx)
		s.Require().NoError(err)

		// Try to move with wrong starting position
		wrongPath := []spatial.Position{
			{X: 5, Y: 5}, // Wrong - not at (2,2)
			{X: 5, Y: 6},
		}

		_, err = tm.Move(s.ctx, &combat.MoveInput{
			Path: wrongPath,
		})
		s.Require().Error(err, "Move should fail with wrong Path[0]")
		s.Contains(err.Error(), "current position")
		s.T().Logf("  Expected error: %s", err.Error())

		_, _ = tm.EndTurn(s.ctx)
		s.T().Log("✓ Move path validation test passed")
	})
}

// Helper to find ability by ID suffix
func findAbility(abilities []combat.AvailableAbility, idSuffix string) *combat.AvailableAbility {
	for i := range abilities {
		if abilities[i].Info.Ref != nil && abilities[i].Info.Ref.ID == idSuffix {
			return &abilities[i]
		}
	}
	return nil
}

// Helper to find action by ID substring
func findAction(actions []combat.AvailableAction, idContains string) *combat.AvailableAction {
	for i := range actions {
		if strings.Contains(actions[i].Info.ID, idContains) {
			return &actions[i]
		}
	}
	return nil
}

func TestTurnManagerIntegrationSuite(t *testing.T) {
	suite.Run(t, new(TurnManagerIntegrationSuite))
}
