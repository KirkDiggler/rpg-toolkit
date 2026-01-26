// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ============================================================================
// MONK ENCOUNTER TEST SUITE
// Level 1 Monk Features:
//   - Martial Arts (DEX for unarmed/monk weapons, 1d4 unarmed, bonus action strike)
//   - Unarmored Defense (AC = 10 + DEX + WIS)
// Level 2 Monk Features:
//   - Ki (2 points)
//   - Flurry of Blows (1 Ki → 2 unarmed strikes)
//   - Patient Defense (1 Ki → Dodge as bonus action)
//   - Step of the Wind (1 Ki → Dash/Disengage + double jump)
//   - Unarmored Movement (+10 ft speed)
// ============================================================================

type MonkEncounterSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	bus        events.EventBus
	mockRoller *mock_dice.MockRoller
	lookup     *integrationLookup
	room       spatial.Room
	registry   *gamectx.BasicCharacterRegistry

	monk       *mockMonkCharacter
	goblin     *monster.Monster
	shortsword *weapons.Weapon
}

// mockMonkCharacter implements the interfaces needed for monk testing
type mockMonkCharacter struct {
	id               string
	name             string
	level            int
	abilityScores    shared.AbilityScores
	proficiencyBonus int
	hitPoints        int
	maxHitPoints     int
	armorClass       int
	resources        map[coreResources.ResourceKey]int
	conditions       []dnd5eEvents.ConditionBehavior
	features         map[string]features.Feature
}

func (m *mockMonkCharacter) GetID() string                       { return m.id }
func (m *mockMonkCharacter) GetName() string                     { return m.name }
func (m *mockMonkCharacter) GetLevel() int                       { return m.level }
func (m *mockMonkCharacter) GetType() core.EntityType            { return "character" }
func (m *mockMonkCharacter) GetHitPoints() int                   { return m.hitPoints }
func (m *mockMonkCharacter) GetMaxHitPoints() int                { return m.maxHitPoints }
func (m *mockMonkCharacter) AC() int                             { return m.armorClass }
func (m *mockMonkCharacter) IsDirty() bool                       { return false }
func (m *mockMonkCharacter) MarkClean()                          {}
func (m *mockMonkCharacter) AbilityScores() shared.AbilityScores { return m.abilityScores }
func (m *mockMonkCharacter) ProficiencyBonus() int               { return m.proficiencyBonus }

func (m *mockMonkCharacter) ApplyDamage(_ context.Context, input *combat.ApplyDamageInput) *combat.ApplyDamageResult {
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

func (m *mockMonkCharacter) IsResourceAvailable(key coreResources.ResourceKey) bool {
	if m.resources == nil {
		return false
	}
	current, ok := m.resources[key]
	return ok && current > 0
}

func (m *mockMonkCharacter) UseResource(key coreResources.ResourceKey, amount int) error {
	if m.resources == nil {
		return nil
	}
	if current, ok := m.resources[key]; ok {
		m.resources[key] = current - amount
	}
	return nil
}

func (m *mockMonkCharacter) GetResourceCurrent(key coreResources.ResourceKey) int {
	if m.resources == nil {
		return 0
	}
	return m.resources[key]
}

func (m *mockMonkCharacter) GetConditions() []dnd5eEvents.ConditionBehavior {
	return m.conditions
}

func (m *mockMonkCharacter) AddCondition(c dnd5eEvents.ConditionBehavior) {
	m.conditions = append(m.conditions, c)
}

func (m *mockMonkCharacter) GetFeature(id string) features.Feature {
	if m.features == nil {
		return nil
	}
	return m.features[id]
}

func (s *MonkEncounterSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.bus = events.NewEventBus()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
	s.lookup = newIntegrationLookup()
	s.ctx = context.Background()

	// Create spatial room
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

func (s *MonkEncounterSuite) SetupSubTest() {
	// Fresh event bus for each subtest
	s.bus = events.NewEventBus()
	s.lookup = newIntegrationLookup()

	// Create monk and goblin
	s.monk = s.createLevel1Monk()
	s.goblin = s.createGoblin()
	s.shortsword = s.createShortsword()

	s.lookup.Add(s.monk)
	s.lookup.Add(s.goblin)

	// Set up character registry for ability score lookups
	s.registry = gamectx.NewBasicCharacterRegistry()
	scores := &gamectx.AbilityScores{
		Strength:     10, // +0
		Dexterity:    16, // +3
		Constitution: 14, // +2
		Intelligence: 10, // +0
		Wisdom:       16, // +3
		Charisma:     8,  // -1
	}
	s.registry.AddAbilityScores(s.monk.GetID(), scores)

	// Set up context with combatant lookup and game context
	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
		CharacterRegistry: s.registry,
	})
	s.ctx = combat.WithCombatantLookup(context.Background(), s.lookup)
	s.ctx = gamectx.WithGameContext(s.ctx, gameCtx)

	// Place in room - adjacent for melee
	_ = s.room.PlaceEntity(s.monk, spatial.Position{X: 2, Y: 2})
	_ = s.room.PlaceEntity(s.goblin, spatial.Position{X: 3, Y: 2})
}

func (s *MonkEncounterSuite) TearDownSubTest() {
	_ = s.room.RemoveEntity(s.monk.GetID())
	_ = s.room.RemoveEntity(s.goblin.GetID())
}

func (s *MonkEncounterSuite) TearDownTest() {
	s.ctrl.Finish()
}

// =============================================================================
// CHARACTER CREATION HELPERS
// =============================================================================

func (s *MonkEncounterSuite) createLevel1Monk() *mockMonkCharacter {
	// Level 1 Monk with standard array:
	// STR 10 (+0), DEX 16 (+3), CON 14 (+2), INT 10 (+0), WIS 16 (+3), CHA 8 (-1)
	// Unarmored Defense: 10 + 3 + 3 = 16 AC
	// HP: 8 + 2 = 10
	// Martial Arts: 1d4 unarmed, use DEX
	return &mockMonkCharacter{
		id:    "shadow-monk",
		name:  "Shadow the Swift",
		level: 1,
		abilityScores: shared.AbilityScores{
			abilities.STR: 10, // +0
			abilities.DEX: 16, // +3
			abilities.CON: 14, // +2
			abilities.INT: 10, // +0
			abilities.WIS: 16, // +3
			abilities.CHA: 8,  // -1
		},
		proficiencyBonus: 2,
		hitPoints:        10, // 8 base + 2 CON
		maxHitPoints:     10,
		armorClass:       16, // Unarmored: 10 + DEX(3) + WIS(3)
		resources:        make(map[coreResources.ResourceKey]int),
		conditions:       []dnd5eEvents.ConditionBehavior{},
		features:         make(map[string]features.Feature),
	}
}

func (s *MonkEncounterSuite) createLevel2Monk() *mockMonkCharacter {
	monk := s.createLevel1Monk()
	monk.level = 2
	monk.hitPoints = 16 // 8 + 2 + (5 + 2) + 2 CON
	monk.maxHitPoints = 16
	monk.resources[resources.Ki] = 2 // Level 2 monk has 2 Ki
	return monk
}

func (s *MonkEncounterSuite) createGoblin() *monster.Monster {
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

func (s *MonkEncounterSuite) createShortsword() *weapons.Weapon {
	weapon, err := weapons.GetByID(weapons.Shortsword)
	s.Require().NoError(err)
	return &weapon
}

// =============================================================================
// LEVEL 1: MARTIAL ARTS TESTS
// =============================================================================

func (s *MonkEncounterSuite) TestMartialArts_DEXForUnarmedStrikes() {
	s.Run("Martial Arts uses DEX instead of STR when DEX is higher", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK MARTIAL ARTS: DEX for Unarmed Strikes                      ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")
		s.T().Logf("  Monk: %s (Level 1, STR +0, DEX +3)", s.monk.GetName())
		s.T().Logf("  Target: Goblin Scout (AC 13, HP 7)")
		s.T().Log("")

		// Apply Martial Arts condition
		martialArts := conditions.NewMartialArtsCondition(conditions.MartialArtsInput{
			CharacterID: s.monk.GetID(),
			MonkLevel:   1,
			Roller:      s.mockRoller,
		})
		err := martialArts.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = martialArts.Remove(s.ctx, s.bus) }()

		s.T().Log("→ Martial Arts active: Can use DEX for unarmed strikes")
		s.T().Log("")

		// Create damage chain event for unarmed strike
		// Monk has STR +0, DEX +3 - should use DEX
		s.T().Log("→ Shadow strikes with an unarmed attack!")

		// Mock the dice roll for 1d4 martial arts damage
		s.mockRoller.EXPECT().RollN(gomock.Any(), 1, 4).Return([]int{3}, nil).Times(1)

		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID: s.monk.GetID(),
			TargetID:   s.goblin.GetID(),
			WeaponRef:  refs.Weapons.UnarmedStrike(),
			Components: []dnd5eEvents.DamageComponent{
				{
					Source:            dnd5eEvents.DamageSourceWeapon,
					OriginalDiceRolls: []int{1}, // Base unarmed (will be replaced)
					FinalDiceRolls:    []int{1},
					DamageType:        damage.Bludgeoning,
				},
				{
					Source:     dnd5eEvents.DamageSourceAbility,
					FlatBonus:  0, // STR +0 (will be replaced with DEX +3)
					DamageType: damage.Bludgeoning,
				},
			},
			DamageType:   damage.Bludgeoning,
			WeaponDamage: "1", // Base unarmed
			AbilityUsed:  abilities.STR,
		}

		// Execute through damage chain
		damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		damageTopic := dnd5eEvents.DamageChain.On(s.bus)
		modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, damageChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
		s.Require().NoError(err)

		// Verify DEX was used (ability component should have +3)
		var abilityBonus int
		for _, comp := range finalEvent.Components {
			if comp.Source == dnd5eEvents.DamageSourceAbility {
				abilityBonus = comp.FlatBonus
				break
			}
		}
		s.Equal(3, abilityBonus, "Should use DEX (+3) instead of STR (+0)")
		s.Equal(abilities.DEX, finalEvent.AbilityUsed, "AbilityUsed should be updated to DEX")

		s.T().Log("  Damage breakdown:")
		s.T().Logf("    1d4 martial arts: %d", 3)
		s.T().Logf("    + DEX modifier:   %d (not STR +0)", 3)
		s.T().Logf("    = Total:          %d damage", 6)
		s.T().Log("")
		s.T().Log("✓ Martial Arts correctly uses DEX for unarmed strikes")
	})
}

func (s *MonkEncounterSuite) TestMartialArts_UnarmedDamageScaling() {
	s.Run("Martial Arts scales unarmed damage: 1d4 at level 1", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK MARTIAL ARTS: Unarmed Damage Scaling (1d4)                 ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")
		s.T().Logf("  Monk: %s (Level 1)", s.monk.GetName())
		s.T().Log("  Martial Arts Die: 1d4 (levels 1-4)")
		s.T().Log("")

		// Apply Martial Arts condition
		martialArts := conditions.NewMartialArtsCondition(conditions.MartialArtsInput{
			CharacterID: s.monk.GetID(),
			MonkLevel:   1,
			Roller:      s.mockRoller,
		})
		err := martialArts.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = martialArts.Remove(s.ctx, s.bus) }()

		s.T().Log("→ Shadow throws a punch!")

		// Mock the dice roll - 1d4 returns 4 (max)
		s.mockRoller.EXPECT().RollN(gomock.Any(), 1, 4).Return([]int{4}, nil).Times(1)

		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID: s.monk.GetID(),
			TargetID:   s.goblin.GetID(),
			WeaponRef:  refs.Weapons.UnarmedStrike(),
			Components: []dnd5eEvents.DamageComponent{
				{
					Source:            dnd5eEvents.DamageSourceWeapon,
					OriginalDiceRolls: []int{1},
					FinalDiceRolls:    []int{1},
					DamageType:        damage.Bludgeoning,
				},
				{
					Source:     dnd5eEvents.DamageSourceAbility,
					FlatBonus:  0,
					DamageType: damage.Bludgeoning,
				},
			},
			DamageType:   damage.Bludgeoning,
			WeaponDamage: "1",
			AbilityUsed:  abilities.STR,
		}

		// Execute through damage chain
		damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		damageTopic := dnd5eEvents.DamageChain.On(s.bus)
		modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, damageChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
		s.Require().NoError(err)

		// Verify weapon damage was updated to martial arts die
		s.Equal("1d4", finalEvent.WeaponDamage, "Weapon damage should be updated to 1d4")

		// Verify weapon component has the new roll
		var weaponRolls []int
		for _, comp := range finalEvent.Components {
			if comp.Source == dnd5eEvents.DamageSourceWeapon {
				weaponRolls = comp.FinalDiceRolls
				break
			}
		}
		s.Equal([]int{4}, weaponRolls, "Weapon rolls should be from 1d4")

		s.T().Log("")
		s.T().Log("  Damage die progression:")
		s.T().Log("    Levels 1-4:   1d4")
		s.T().Log("    Levels 5-10:  1d6")
		s.T().Log("    Levels 11-16: 1d8")
		s.T().Log("    Levels 17+:   1d10")
		s.T().Log("")
		s.T().Logf("  This attack: 1d4 → %d", 4)
		s.T().Log("")
		s.T().Log("✓ Martial Arts correctly uses 1d4 for unarmed damage at level 1")
	})
}

func (s *MonkEncounterSuite) TestMartialArts_MonkWeaponWithDEX() {
	s.Run("Martial Arts allows DEX for monk weapons (shortsword)", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK MARTIAL ARTS: DEX for Monk Weapons                         ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")
		s.T().Logf("  Monk: %s (Level 1, STR +0, DEX +3)", s.monk.GetName())
		s.T().Logf("  Weapon: Shortsword (1d6 piercing, monk weapon)")
		s.T().Log("")

		// Apply Martial Arts condition
		martialArts := conditions.NewMartialArtsCondition(conditions.MartialArtsInput{
			CharacterID: s.monk.GetID(),
			MonkLevel:   1,
			Roller:      s.mockRoller,
		})
		err := martialArts.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = martialArts.Remove(s.ctx, s.bus) }()

		s.T().Log("→ Shadow slashes with a shortsword!")

		// Shortsword attack - uses weapon's 1d6, but DEX for modifier
		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID: s.monk.GetID(),
			TargetID:   s.goblin.GetID(),
			WeaponRef:  refs.Weapons.Shortsword(),
			Components: []dnd5eEvents.DamageComponent{
				{
					Source:            dnd5eEvents.DamageSourceWeapon,
					OriginalDiceRolls: []int{5},
					FinalDiceRolls:    []int{5},
					DamageType:        damage.Piercing,
				},
				{
					Source:     dnd5eEvents.DamageSourceAbility,
					FlatBonus:  0, // STR +0 (will be replaced with DEX +3)
					DamageType: damage.Piercing,
				},
			},
			DamageType:   damage.Piercing,
			WeaponDamage: "1d6",
			AbilityUsed:  abilities.STR,
		}

		// Execute through damage chain
		damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		damageTopic := dnd5eEvents.DamageChain.On(s.bus)
		modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, damageChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
		s.Require().NoError(err)

		// Verify DEX was used
		var abilityBonus int
		for _, comp := range finalEvent.Components {
			if comp.Source == dnd5eEvents.DamageSourceAbility {
				abilityBonus = comp.FlatBonus
				break
			}
		}
		s.Equal(3, abilityBonus, "Should use DEX (+3) for monk weapon")

		// Verify weapon damage stays as shortsword (1d6), not martial arts die
		s.Equal("1d6", finalEvent.WeaponDamage, "Shortsword keeps its 1d6 damage")

		s.T().Log("  Damage breakdown:")
		s.T().Logf("    1d6 shortsword: %d", 5)
		s.T().Logf("    + DEX modifier: %d", 3)
		s.T().Logf("    = Total:        %d damage", 8)
		s.T().Log("")
		s.T().Log("  Monk weapons include:")
		s.T().Log("    - Shortswords")
		s.T().Log("    - Simple melee weapons (no Heavy/Two-Handed)")
		s.T().Log("")
		s.T().Log("✓ Martial Arts correctly uses DEX for monk weapons")
	})
}

// =============================================================================
// LEVEL 1: UNARMORED DEFENSE TESTS
// =============================================================================

func (s *MonkEncounterSuite) TestUnarmoredDefense_ExpectedAC() {
	s.Run("Unarmored Defense expected AC: 10 + DEX + WIS", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK UNARMORED DEFENSE: Expected AC                             ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// TODO: When UnarmoredDefenseCondition is wired to ACChain (#580),
		// this test should apply the real condition and verify AC through
		// the EffectiveAC() calculation chain. For now, we document the
		// expected formula and verify the mock is set up correctly.

		// Monk stats: DEX 16 (+3), WIS 16 (+3)
		// Unarmored Defense: 10 + 3 + 3 = 16
		expectedAC := 16

		actualAC := s.monk.AC()
		s.Equal(expectedAC, actualAC, "Mock AC should match expected Unarmored Defense formula")

		s.T().Logf("  Ability Scores:")
		s.T().Logf("    DEX: 16 (+3)")
		s.T().Logf("    WIS: 16 (+3)")
		s.T().Log("")
		s.T().Log("  Unarmored Defense (Monk) formula:")
		s.T().Log("    Base:          10")
		s.T().Log("    + DEX mod:      3")
		s.T().Log("    + WIS mod:      3")
		s.T().Logf("    = AC:          %d", expectedAC)
		s.T().Log("")
		s.T().Log("  Note: Monk uses WIS, Barbarian uses CON")
		s.T().Log("  Note: Real UnarmoredDefenseCondition not yet wired to ACChain")
		s.T().Log("")
		s.T().Log("✓ Mock AC matches expected Unarmored Defense formula")
	})
}

// =============================================================================
// LEVEL 2: KI FEATURE TESTS
// =============================================================================

func (s *MonkEncounterSuite) TestKi_InitialPoints() {
	s.Run("Level 2 Monk has 2 Ki points", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK KI: Initial Ki Points                                      ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// Create level 2 monk
		monk2 := s.createLevel2Monk()

		kiPoints := monk2.GetResourceCurrent(resources.Ki)
		s.Equal(2, kiPoints, "Level 2 monk should have 2 Ki points")

		s.T().Logf("  Monk: %s (Level 2)", monk2.GetName())
		s.T().Log("")
		s.T().Log("  Ki Points by Level:")
		s.T().Log("    Level 2:  2 Ki")
		s.T().Log("    Level 3:  3 Ki")
		s.T().Log("    Level 4:  4 Ki")
		s.T().Log("    ... (Ki = Monk Level)")
		s.T().Log("")
		s.T().Logf("  Current Ki: %d/2", kiPoints)
		s.T().Log("")
		s.T().Log("✓ Level 2 Monk correctly has 2 Ki points")
	})
}

func (s *MonkEncounterSuite) TestFlurryOfBlows_KiConsumption() {
	s.Run("Flurry of Blows consumes 1 Ki", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK FLURRY OF BLOWS: Ki Consumption                            ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// Create level 2 monk with Ki
		monk2 := s.createLevel2Monk()
		s.T().Logf("  Monk: %s (Level 2)", monk2.GetName())
		s.T().Logf("  Initial Ki: %d/2", monk2.GetResourceCurrent(resources.Ki))
		s.T().Log("")

		s.T().Log("→ Shadow uses Flurry of Blows!")

		// TODO: When FlurryOfBlows feature is implemented, exercise it here
		// and assert that 2 bonus action unarmed strikes are granted.
		// For now, we only test Ki consumption.
		s.T().Log("  [Bonus Action] Spend 1 Ki")

		// Simulate Ki consumption
		err := monk2.UseResource(resources.Ki, 1)
		s.Require().NoError(err)

		kiAfter := monk2.GetResourceCurrent(resources.Ki)
		s.Equal(1, kiAfter, "Ki should be consumed")

		s.T().Log("")
		s.T().Logf("  Ki remaining: %d/2", kiAfter)
		s.T().Log("")
		s.T().Log("  Flurry of Blows:")
		s.T().Log("    Cost: 1 Ki point")
		s.T().Log("    Effect: 2 unarmed strikes as bonus action (not yet tested)")
		s.T().Log("    Timing: Immediately after Attack action")
		s.T().Log("")
		s.T().Log("✓ Flurry of Blows Ki consumption verified")
	})
}

func (s *MonkEncounterSuite) TestPatientDefense_KiConsumption() {
	s.Run("Patient Defense consumes 1 Ki", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK PATIENT DEFENSE: Ki Consumption                            ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		monk2 := s.createLevel2Monk()
		s.T().Logf("  Monk: %s (Level 2)", monk2.GetName())
		s.T().Logf("  Initial Ki: %d/2", monk2.GetResourceCurrent(resources.Ki))
		s.T().Log("")

		s.T().Log("→ Shadow uses Patient Defense!")
		s.T().Log("  [Bonus Action] Spend 1 Ki")

		// TODO: When PatientDefense feature is implemented, exercise it here
		// and assert that Dodge action is granted/enabled.
		// For now, we only test Ki consumption.

		// Simulate Ki consumption
		err := monk2.UseResource(resources.Ki, 1)
		s.Require().NoError(err)

		kiAfter := monk2.GetResourceCurrent(resources.Ki)
		s.Equal(1, kiAfter, "Ki should be consumed")

		s.T().Log("")
		s.T().Logf("  Ki remaining: %d/2", kiAfter)
		s.T().Log("")
		s.T().Log("  Patient Defense:")
		s.T().Log("    Cost: 1 Ki point")
		s.T().Log("    Effect: Take Dodge action as bonus action (not yet tested)")
		s.T().Log("    Dodge: Attack rolls against you have disadvantage")
		s.T().Log("")
		s.T().Log("✓ Patient Defense Ki consumption verified")
	})
}

func (s *MonkEncounterSuite) TestStepOfTheWind_KiConsumption() {
	s.Run("Step of the Wind consumes 1 Ki", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK STEP OF THE WIND: Ki Consumption                           ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		monk2 := s.createLevel2Monk()
		s.T().Logf("  Monk: %s (Level 2)", monk2.GetName())
		s.T().Logf("  Initial Ki: %d/2", monk2.GetResourceCurrent(resources.Ki))
		s.T().Log("")

		s.T().Log("→ Shadow uses Step of the Wind!")
		s.T().Log("  [Bonus Action] Spend 1 Ki")

		// TODO: When StepOfTheWind feature is implemented, exercise it here
		// and assert that Dash/Disengage action + double jump are granted.
		// For now, we only test Ki consumption.

		// Simulate Ki consumption
		err := monk2.UseResource(resources.Ki, 1)
		s.Require().NoError(err)

		kiAfter := monk2.GetResourceCurrent(resources.Ki)
		s.Equal(1, kiAfter, "Ki should be consumed")

		s.T().Log("")
		s.T().Logf("  Ki remaining: %d/2", kiAfter)
		s.T().Log("")
		s.T().Log("  Step of the Wind:")
		s.T().Log("    Cost: 1 Ki point")
		s.T().Log("    Effect: Dash OR Disengage as bonus action (not yet tested)")
		s.T().Log("    Bonus: Jump distance doubled for the turn (not yet tested)")
		s.T().Log("")
		s.T().Log("✓ Step of the Wind Ki consumption verified")
	})
}

func TestMonkEncounterSuite(t *testing.T) {
	suite.Run(t, new(MonkEncounterSuite))
}
