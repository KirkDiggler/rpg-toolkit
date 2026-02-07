// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
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
//   - Flurry of Blows (1 Ki → 2 unarmed strikes as bonus action)
//   - Patient Defense (1 Ki → Dodge as bonus action)
//   - Step of the Wind (1 Ki → Dash/Disengage as bonus action + double jump)
//   - Unarmored Movement (+10 ft speed)
// ============================================================================

type MonkEncounterSuite struct {
	suite.Suite
	ctx      context.Context
	bus      events.EventBus
	lookup   *integrationLookup
	room     spatial.Room
	registry *gamectx.BasicCharacterRegistry

	monk       *character.Character
	goblin     *monster.Monster
	shortsword *weapons.Weapon
}

func (s *MonkEncounterSuite) SetupTest() {
	s.bus = events.NewEventBus()
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

	// Default to level 1 monk — tests that need level 2 will override
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
	if s.monk != nil {
		_ = s.monk.Cleanup(s.ctx)
	}
}

// =============================================================================
// CHARACTER CREATION HELPERS
// =============================================================================

func (s *MonkEncounterSuite) createLevel1Monk() *character.Character {
	// Level 1 Monk with standard array:
	// STR 10 (+0), DEX 16 (+3), CON 14 (+2), INT 10 (+0), WIS 16 (+3), CHA 8 (-1)
	// Unarmored Defense: 10 + 3 + 3 = 16 AC
	// HP: 8 + 2 = 10
	// Martial Arts: 1d4 unarmed, use DEX
	data := &character.Data{
		ID:               "shadow-monk",
		PlayerID:         "player-1",
		Name:             "Shadow the Swift",
		Level:            1,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Monk,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10, // +0
			abilities.DEX: 16, // +3
			abilities.CON: 14, // +2
			abilities.INT: 10, // +0
			abilities.WIS: 16, // +3
			abilities.CHA: 8,  // -1
		},
		HitPoints:    10, // 8 base + 2 CON
		MaxHitPoints: 10,
		ArmorClass:   16, // Unarmored: 10 + DEX(3) + WIS(3)
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Acrobatics: shared.Proficient,
			skills.Stealth:    shared.Proficient,
		},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
			abilities.DEX: shared.Proficient,
		},
		// Martial Arts is a passive condition applied at character creation
		Conditions: []json.RawMessage{
			json.RawMessage(`{
				"ref": {"module": "dnd5e", "type": "conditions", "id": "martial_arts"},
				"character_id": "shadow-monk",
				"monk_level": 1
			}`),
		},
	}

	char, err := character.LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)

	// Add combat abilities
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewAttack("attack")))

	return char
}

func (s *MonkEncounterSuite) createLevel2Monk() *character.Character {
	// Level 2 Monk: gains Ki (2 points), Flurry of Blows, Patient Defense,
	// Step of the Wind, and Unarmored Movement (+10 ft)
	data := &character.Data{
		ID:               "shadow-monk",
		PlayerID:         "player-1",
		Name:             "Shadow the Swift",
		Level:            2,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Monk,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10, // +0
			abilities.DEX: 16, // +3
			abilities.CON: 14, // +2
			abilities.INT: 10, // +0
			abilities.WIS: 16, // +3
			abilities.CHA: 8,  // -1
		},
		HitPoints:    16, // 16 HP at level 2
		MaxHitPoints: 16,
		ArmorClass:   16, // Unarmored: 10 + DEX(3) + WIS(3)
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Acrobatics: shared.Proficient,
			skills.Stealth:    shared.Proficient,
		},
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
			abilities.DEX: shared.Proficient,
		},
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.Ki: {
				Current:   2,
				Maximum:   2,
				ResetType: coreResources.ResetShortRest,
			},
		},
		Features: []json.RawMessage{
			json.RawMessage(`{
				"ref": {"module": "dnd5e", "type": "features", "id": "flurry_of_blows"},
				"id": "flurry_of_blows",
				"name": "Flurry of Blows",
				"character_id": "shadow-monk"
			}`),
			json.RawMessage(`{
				"ref": {"module": "dnd5e", "type": "features", "id": "patient_defense"},
				"id": "patient_defense",
				"name": "Patient Defense",
				"character_id": "shadow-monk"
			}`),
			json.RawMessage(`{
				"ref": {"module": "dnd5e", "type": "features", "id": "step_of_the_wind"},
				"id": "step_of_the_wind",
				"name": "Step of the Wind",
				"character_id": "shadow-monk"
			}`),
		},
		Conditions: []json.RawMessage{
			json.RawMessage(`{
				"ref": {"module": "dnd5e", "type": "conditions", "id": "martial_arts"},
				"character_id": "shadow-monk",
				"monk_level": 2
			}`),
			json.RawMessage(`{
				"ref": {"module": "dnd5e", "type": "conditions", "id": "unarmored_movement"},
				"character_id": "shadow-monk",
				"monk_level": 2
			}`),
		},
	}

	char, err := character.LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)

	// Add combat abilities
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewAttack("attack")))

	return char
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
		s.T().Log("║  MONK MARTIAL ARTS: DEX for Unarmed Strikes                     ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")
		s.T().Logf("  Monk: %s (Level 1, STR +0, DEX +3)", s.monk.GetName())
		s.T().Logf("  Target: Goblin Scout (AC 13, HP 7)")
		s.T().Log("")

		// Verify Martial Arts condition is loaded
		monkConditions := s.monk.GetConditions()
		s.Require().NotEmpty(monkConditions, "Monk should have Martial Arts condition loaded from Data")
		s.T().Log("→ Martial Arts active: Can use DEX for unarmed strikes")
		s.T().Log("")

		// Note: The MartialArtsCondition loaded via JSON uses its own dice.NewRoller()
		// (not the mock), so we verify behavior (DEX swap, damage string) rather than
		// exact roll values.

		// Create damage chain event for unarmed strike
		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID: s.monk.GetID(),
			TargetID:   s.goblin.GetID(),
			WeaponRef:  refs.Weapons.UnarmedStrike(),
			Components: []dnd5eEvents.DamageComponent{
				{
					Source:            dnd5eEvents.DamageSourceWeapon,
					OriginalDiceRolls: []int{1},
					FinalDiceRolls:    []int{1},
				},
				{
					Source:    dnd5eEvents.DamageSourceAbility,
					FlatBonus: 0, // STR +0 (should be replaced with DEX +3)
				},
			},
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

		s.T().Log("  Verified:")
		s.T().Log("    Ability modifier: DEX +3 (replaced STR +0)")
		s.T().Log("    AbilityUsed field: DEX")
		s.T().Log("")
		s.T().Log("✓ Martial Arts correctly uses DEX for unarmed strikes")
	})
}

func (s *MonkEncounterSuite) TestMartialArts_UnarmedDamageScaling() {
	s.Run("Martial Arts scales unarmed damage: 1d4 at level 1", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK MARTIAL ARTS: Unarmed Damage Scaling (1d4)                ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")
		s.T().Logf("  Monk: %s (Level 1)", s.monk.GetName())
		s.T().Log("  Martial Arts Die: 1d4 (levels 1-4)")
		s.T().Log("")

		s.T().Log("→ Shadow throws a punch!")

		// Note: The MartialArtsCondition loaded via JSON uses its own dice.NewRoller()
		// so the actual roll value is non-deterministic. We verify the damage string
		// is upgraded to "1d4" and the roll count is correct.

		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID: s.monk.GetID(),
			TargetID:   s.goblin.GetID(),
			WeaponRef:  refs.Weapons.UnarmedStrike(),
			Components: []dnd5eEvents.DamageComponent{
				{
					Source:            dnd5eEvents.DamageSourceWeapon,
					OriginalDiceRolls: []int{1},
					FinalDiceRolls:    []int{1},
				},
				{
					Source:    dnd5eEvents.DamageSourceAbility,
					FlatBonus: 0,
				},
			},
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

		// Verify weapon component has exactly 1 die roll (1d4 = one die)
		var weaponRolls []int
		for _, comp := range finalEvent.Components {
			if comp.Source == dnd5eEvents.DamageSourceWeapon {
				weaponRolls = comp.FinalDiceRolls
				break
			}
		}
		s.Require().Len(weaponRolls, 1, "Should have exactly 1 die roll (1d4)")
		s.True(weaponRolls[0] >= 1 && weaponRolls[0] <= 4, "Roll should be in [1,4] range, got %d", weaponRolls[0])

		s.T().Log("  Damage die progression:")
		s.T().Log("    Levels 1-4:   1d4")
		s.T().Log("    Levels 5-10:  1d6")
		s.T().Log("    Levels 11-16: 1d8")
		s.T().Log("    Levels 17+:   1d10")
		s.T().Log("")
		s.T().Logf("  This attack: 1d4 → %d (verified in [1,4] range)", weaponRolls[0])
		s.T().Log("")
		s.T().Log("✓ Martial Arts correctly upgrades unarmed damage to 1d4 at level 1")
	})
}

func (s *MonkEncounterSuite) TestMartialArts_MonkWeaponWithDEX() {
	s.Run("Martial Arts allows DEX for monk weapons (shortsword)", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK MARTIAL ARTS: DEX for Monk Weapons                        ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")
		s.T().Logf("  Monk: %s (Level 1, STR +0, DEX +3)", s.monk.GetName())
		s.T().Logf("  Weapon: Shortsword (1d6 piercing, monk weapon)")
		s.T().Log("")

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
				},
				{
					Source:    dnd5eEvents.DamageSourceAbility,
					FlatBonus: 0, // STR +0 (will be replaced with DEX +3)
				},
			},
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
		s.T().Log("✓ Martial Arts correctly uses DEX for monk weapons")
	})
}

// =============================================================================
// LEVEL 1: UNARMORED DEFENSE TESTS
// =============================================================================

func (s *MonkEncounterSuite) TestUnarmoredDefense_ExpectedAC() {
	s.Run("Unarmored Defense expected AC: 10 + DEX + WIS", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK UNARMORED DEFENSE: Expected AC                            ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// TODO: When UnarmoredDefenseCondition is wired to ACChain,
		// this test should apply the real condition and verify AC through
		// the EffectiveAC() calculation chain. For now, we verify the
		// character was loaded with the correct AC from Data.

		// Monk stats: DEX 16 (+3), WIS 16 (+3)
		// Unarmored Defense: 10 + 3 + 3 = 16
		expectedAC := 16

		actualAC := s.monk.AC()
		s.Equal(expectedAC, actualAC, "Character AC should match expected Unarmored Defense formula")

		s.T().Log("  Ability Scores:")
		s.T().Log("    DEX: 16 (+3)")
		s.T().Log("    WIS: 16 (+3)")
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
		s.T().Log("✓ Character AC matches expected Unarmored Defense formula")
	})
}

// =============================================================================
// LEVEL 2: KI FEATURE TESTS — REAL FEATURE ACTIVATION
// =============================================================================

func (s *MonkEncounterSuite) TestFlurryOfBlows_GrantsTwoStrikes() {
	s.Run("Flurry of Blows consumes 1 Ki and grants 2 FlurryStrike actions", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK FLURRY OF BLOWS: Ki + Strike Grants                       ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// Override with level 2 monk that has Ki and features
		if s.monk != nil {
			_ = s.monk.Cleanup(s.ctx)
		}
		s.monk = s.createLevel2Monk()
		s.lookup.Add(s.monk)

		// Verify Ki is available
		ki := s.monk.GetResource(resources.Ki)
		s.Require().NotNil(ki, "Level 2 monk should have Ki resource")
		s.Require().Equal(2, ki.Current(), "Should start with 2 Ki")
		s.T().Logf("  Monk: %s (Level 2)", s.monk.GetName())
		s.T().Logf("  Initial Ki: %d/2", ki.Current())
		s.T().Log("")

		// Get the Flurry of Blows feature
		flurry := s.monk.GetFeature("flurry_of_blows")
		s.Require().NotNil(flurry, "Monk should have Flurry of Blows feature loaded from Data")

		// Verify no extra actions before activation
		initialActions := s.monk.GetActions()
		s.T().Logf("  Actions before Flurry: %d", len(initialActions))

		s.T().Log("")
		s.T().Log("→ Shadow uses Flurry of Blows!")
		s.T().Log("  [Bonus Action] Spend 1 Ki")

		// Activate Flurry of Blows
		err := flurry.Activate(s.ctx, s.monk, features.FeatureInput{
			Bus: s.bus,
		})
		s.Require().NoError(err)

		// Verify Ki was consumed
		s.Equal(1, ki.Current(), "Should consume 1 Ki point")
		s.T().Logf("  Ki remaining: %d/2", ki.Current())

		// Verify 2 FlurryStrike actions were granted to the character by ID
		allActions := s.monk.GetActions()
		monkID := s.monk.GetID()
		expectedFlurry1 := monkID + "-flurry-strike-1"
		expectedFlurry2 := monkID + "-flurry-strike-2"

		flurryActions := 0
		foundFlurry1 := false
		foundFlurry2 := false

		for _, action := range allActions {
			switch action.GetID() {
			case expectedFlurry1:
				foundFlurry1 = true
				s.True(action.IsTemporary(), "FlurryStrike action 1 should be temporary")
				flurryActions++
			case expectedFlurry2:
				foundFlurry2 = true
				s.True(action.IsTemporary(), "FlurryStrike action 2 should be temporary")
				flurryActions++
			}
		}

		s.True(foundFlurry1, "Expected FlurryStrike action ID %q to exist on monk", expectedFlurry1)
		s.True(foundFlurry2, "Expected FlurryStrike action ID %q to exist on monk", expectedFlurry2)
		s.Equal(2, flurryActions, "Should grant 2 FlurryStrike actions")

		s.T().Log("")
		s.T().Logf("  Actions after Flurry: %d total (%d flurry strikes)", len(allActions), flurryActions)
		s.T().Log("")
		s.T().Log("  Flurry of Blows:")
		s.T().Log("    Cost: 1 Ki point")
		s.T().Log("    Effect: 2 unarmed strikes as bonus action")
		s.T().Log("    Timing: Immediately after Attack action")
		s.T().Log("")
		s.T().Log("✓ Flurry of Blows consumes Ki AND grants 2 strike actions")
	})
}

func (s *MonkEncounterSuite) TestPatientDefense_PublishesDodgeEvent() {
	s.Run("Patient Defense consumes 1 Ki and publishes dodge event", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK PATIENT DEFENSE: Ki + Dodge Event                         ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// Override with level 2 monk
		if s.monk != nil {
			_ = s.monk.Cleanup(s.ctx)
		}
		s.monk = s.createLevel2Monk()
		s.lookup.Add(s.monk)

		ki := s.monk.GetResource(resources.Ki)
		s.Require().Equal(2, ki.Current())
		s.T().Logf("  Monk: %s (Level 2)", s.monk.GetName())
		s.T().Logf("  Initial Ki: %d/2", ki.Current())
		s.T().Log("")

		// Get the Patient Defense feature
		patientDefense := s.monk.GetFeature("patient_defense")
		s.Require().NotNil(patientDefense, "Monk should have Patient Defense feature loaded from Data")

		// Subscribe to the event to verify it fires
		var receivedEvent *dnd5eEvents.PatientDefenseActivatedEvent
		topic := dnd5eEvents.PatientDefenseActivatedTopic.On(s.bus)
		_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.PatientDefenseActivatedEvent) error {
			receivedEvent = &event
			return nil
		})
		s.Require().NoError(err)

		s.T().Log("→ Shadow uses Patient Defense!")
		s.T().Log("  [Bonus Action] Spend 1 Ki")

		// Activate Patient Defense
		err = patientDefense.Activate(s.ctx, s.monk, features.FeatureInput{
			Bus: s.bus,
		})
		s.Require().NoError(err)

		// Verify Ki was consumed
		s.Equal(1, ki.Current(), "Should consume 1 Ki point")
		s.T().Logf("  Ki remaining: %d/2", ki.Current())

		// Verify the event was published with correct data
		s.Require().NotNil(receivedEvent, "PatientDefenseActivatedEvent should have been published")
		s.Equal(s.monk.GetID(), receivedEvent.CharacterID)
		s.Equal(refs.Features.PatientDefense().ID, receivedEvent.Source)

		s.T().Log("")
		s.T().Logf("  Event received: CharacterID=%s, Source=%s", receivedEvent.CharacterID, receivedEvent.Source)
		s.T().Log("")
		s.T().Log("  Patient Defense:")
		s.T().Log("    Cost: 1 Ki point")
		s.T().Log("    Effect: Dodge action as bonus action")
		s.T().Log("    Dodge: Attack rolls against you have disadvantage")
		s.T().Log("")
		s.T().Log("✓ Patient Defense consumes Ki AND publishes dodge event")
	})
}

func (s *MonkEncounterSuite) TestStepOfTheWind_PublishesDashEvent() {
	s.Run("Step of the Wind with dash action publishes correct event", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK STEP OF THE WIND: Ki + Dash Event                         ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// Override with level 2 monk
		if s.monk != nil {
			_ = s.monk.Cleanup(s.ctx)
		}
		s.monk = s.createLevel2Monk()
		s.lookup.Add(s.monk)

		ki := s.monk.GetResource(resources.Ki)
		s.Require().Equal(2, ki.Current())
		s.T().Logf("  Monk: %s (Level 2)", s.monk.GetName())
		s.T().Logf("  Initial Ki: %d/2", ki.Current())
		s.T().Log("")

		// Get the Step of the Wind feature
		stepOfWind := s.monk.GetFeature("step_of_the_wind")
		s.Require().NotNil(stepOfWind, "Monk should have Step of the Wind feature loaded from Data")

		// Subscribe to the event
		var receivedEvent *dnd5eEvents.StepOfTheWindActivatedEvent
		topic := dnd5eEvents.StepOfTheWindActivatedTopic.On(s.bus)
		_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.StepOfTheWindActivatedEvent) error {
			receivedEvent = &event
			return nil
		})
		s.Require().NoError(err)

		s.T().Log("→ Shadow uses Step of the Wind (Dash)!")
		s.T().Log("  [Bonus Action] Spend 1 Ki")

		// Activate Step of the Wind with "dash"
		err = stepOfWind.Activate(s.ctx, s.monk, features.FeatureInput{
			Bus:    s.bus,
			Action: "dash",
		})
		s.Require().NoError(err)

		// Verify Ki was consumed
		s.Equal(1, ki.Current(), "Should consume 1 Ki point")

		// Verify the event was published with correct action
		s.Require().NotNil(receivedEvent, "StepOfTheWindActivatedEvent should have been published")
		s.Equal(s.monk.GetID(), receivedEvent.CharacterID)
		s.Equal("dash", receivedEvent.Action)
		s.Equal(refs.Features.StepOfTheWind().ID, receivedEvent.Source)

		s.T().Log("")
		s.T().Logf("  Event received: CharacterID=%s, Action=%s", receivedEvent.CharacterID, receivedEvent.Action)
		s.T().Log("")
		s.T().Log("  Step of the Wind (Dash):")
		s.T().Log("    Cost: 1 Ki point")
		s.T().Log("    Effect: Dash as bonus action + double jump distance")
		s.T().Log("")
		s.T().Log("✓ Step of the Wind (Dash) consumes Ki AND publishes correct event")
	})
}

func (s *MonkEncounterSuite) TestStepOfTheWind_PublishesDisengageEvent() {
	s.Run("Step of the Wind with disengage action publishes correct event", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK STEP OF THE WIND: Ki + Disengage Event                    ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// Override with level 2 monk
		if s.monk != nil {
			_ = s.monk.Cleanup(s.ctx)
		}
		s.monk = s.createLevel2Monk()
		s.lookup.Add(s.monk)

		ki := s.monk.GetResource(resources.Ki)
		s.Require().Equal(2, ki.Current())

		// Get the Step of the Wind feature
		stepOfWind := s.monk.GetFeature("step_of_the_wind")
		s.Require().NotNil(stepOfWind)

		// Subscribe to the event
		var receivedEvent *dnd5eEvents.StepOfTheWindActivatedEvent
		topic := dnd5eEvents.StepOfTheWindActivatedTopic.On(s.bus)
		_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.StepOfTheWindActivatedEvent) error {
			receivedEvent = &event
			return nil
		})
		s.Require().NoError(err)

		s.T().Log("→ Shadow uses Step of the Wind (Disengage)!")

		// Activate Step of the Wind with "disengage"
		err = stepOfWind.Activate(s.ctx, s.monk, features.FeatureInput{
			Bus:    s.bus,
			Action: "disengage",
		})
		s.Require().NoError(err)

		// Verify Ki was consumed
		s.Equal(1, ki.Current(), "Should consume 1 Ki point")

		// Verify event
		s.Require().NotNil(receivedEvent)
		s.Equal(s.monk.GetID(), receivedEvent.CharacterID)
		s.Equal("disengage", receivedEvent.Action)
		s.Equal(refs.Features.StepOfTheWind().ID, receivedEvent.Source)

		s.T().Log("")
		s.T().Logf("  Event received: CharacterID=%s, Action=%s, Source=%s", receivedEvent.CharacterID, receivedEvent.Action, receivedEvent.Source)
		s.T().Log("")
		s.T().Log("✓ Step of the Wind (Disengage) consumes Ki AND publishes correct event")
	})
}

// =============================================================================
// LEVEL 2: UNARMORED MOVEMENT TESTS
// =============================================================================

func (s *MonkEncounterSuite) TestUnarmoredMovement_SpeedBonus() {
	s.Run("Unarmored Movement grants +10ft speed at level 2", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK UNARMORED MOVEMENT: Speed Bonus                           ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// Override with level 2 monk that has Unarmored Movement condition
		if s.monk != nil {
			_ = s.monk.Cleanup(s.ctx)
		}
		s.monk = s.createLevel2Monk()
		s.lookup.Add(s.monk)

		// Set up weapons registry (no weapons = unarmored)
		charWeapons := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{})
		s.registry.Add(s.monk.GetID(), charWeapons)

		s.T().Logf("  Monk: %s (Level 2, unarmored)", s.monk.GetName())
		s.T().Log("")

		// Find the UnarmoredMovementCondition from loaded conditions
		var umCondition interface {
			GetSpeedBonus(context.Context) (int, error)
		}
		for _, cond := range s.monk.GetConditions() {
			if getter, ok := cond.(interface {
				GetSpeedBonus(context.Context) (int, error)
			}); ok {
				umCondition = getter
				break
			}
		}
		s.Require().NotNil(umCondition, "Monk should have UnarmoredMovementCondition loaded from Data")

		// Verify speed bonus
		bonus, err := umCondition.GetSpeedBonus(s.ctx)
		s.Require().NoError(err)
		s.Equal(10, bonus, "Level 2 monk should get +10 ft speed bonus")

		s.T().Log("  Speed bonus by level:")
		s.T().Log("    Level 2-5:   +10 ft")
		s.T().Log("    Level 6-9:   +15 ft")
		s.T().Log("    Level 10-13: +20 ft")
		s.T().Log("    Level 14-17: +25 ft")
		s.T().Log("    Level 18+:   +30 ft")
		s.T().Log("")
		s.T().Logf("  Current bonus: +%d ft", bonus)
		s.T().Log("")
		s.T().Log("✓ Unarmored Movement correctly grants speed bonus")
	})
}

// =============================================================================
// LEVEL 2: KI EXHAUSTION
// =============================================================================

func (s *MonkEncounterSuite) TestKi_ExhaustionPreventsActivation() {
	s.Run("Ki features fail when Ki is exhausted", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK KI: Exhaustion Prevents Activation                        ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// Override with level 2 monk
		if s.monk != nil {
			_ = s.monk.Cleanup(s.ctx)
		}
		s.monk = s.createLevel2Monk()
		s.lookup.Add(s.monk)

		ki := s.monk.GetResource(resources.Ki)
		s.T().Logf("  Monk: %s (Level 2, Ki: %d/2)", s.monk.GetName(), ki.Current())
		s.T().Log("")

		// Use Flurry of Blows to consume 1 Ki
		flurry := s.monk.GetFeature("flurry_of_blows")
		s.Require().NotNil(flurry)

		err := flurry.Activate(s.ctx, s.monk, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)
		s.T().Log("→ Flurry of Blows activated (1 Ki spent)")
		s.T().Logf("  Ki remaining: %d/2", ki.Current())

		// Use Patient Defense to consume the last Ki
		patientDefense := s.monk.GetFeature("patient_defense")
		s.Require().NotNil(patientDefense)

		err = patientDefense.Activate(s.ctx, s.monk, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)
		s.T().Log("→ Patient Defense activated (1 Ki spent)")
		s.T().Logf("  Ki remaining: %d/2", ki.Current())
		s.Equal(0, ki.Current(), "All Ki should be consumed")

		// Now Step of the Wind should fail
		stepOfWind := s.monk.GetFeature("step_of_the_wind")
		s.Require().NotNil(stepOfWind)

		s.T().Log("")
		s.T().Log("→ Shadow tries Step of the Wind with 0 Ki...")

		err = stepOfWind.Activate(s.ctx, s.monk, features.FeatureInput{
			Bus:    s.bus,
			Action: "dash",
		})
		s.Require().Error(err, "Step of the Wind should fail with no Ki")
		s.T().Logf("  Error: %v", err)

		s.T().Log("")
		s.T().Log("✓ Ki exhaustion correctly prevents feature activation")
	})
}

func TestMonkEncounterSuite(t *testing.T) {
	suite.Run(t, new(MonkEncounterSuite))
}
