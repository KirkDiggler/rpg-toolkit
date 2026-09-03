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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
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
	ctx  context.Context
	bus  events.EventBus
	room spatial.Room

	monk       *character.Character
	goblin     *monster.Monster
	shortsword *weapons.Weapon
}

func (s *MonkEncounterSuite) SetupTest() {
	s.bus = events.NewEventBus()
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

	// Default to level 1 monk — tests that need level 2 will override
	s.monk = s.createLevel1Monk()
	s.goblin = s.createGoblin()
	s.shortsword = s.createShortsword()

	s.ctx = context.Background()

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
				"member_id": "shadow-monk",
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
				"member_id": "shadow-monk"
			}`),
			json.RawMessage(`{
				"ref": {"module": "dnd5e", "type": "features", "id": "patient_defense"},
				"id": "patient_defense",
				"name": "Patient Defense",
				"member_id": "shadow-monk"
			}`),
			json.RawMessage(`{
				"ref": {"module": "dnd5e", "type": "features", "id": "step_of_the_wind"},
				"id": "step_of_the_wind",
				"name": "Step of the Wind",
				"member_id": "shadow-monk"
			}`),
		},
		Conditions: []json.RawMessage{
			json.RawMessage(`{
				"ref": {"module": "dnd5e", "type": "conditions", "id": "martial_arts"},
				"member_id": "shadow-monk",
				"monk_level": 2
			}`),
			json.RawMessage(`{
				"ref": {"module": "dnd5e", "type": "conditions", "id": "unarmored_movement"},
				"member_id": "shadow-monk",
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
					Source:     dnd5eEvents.DamageSourceWeapon,
					Properties: []damage.Property{damage.AddsAttackAbilityModifier},
					Roll: dnd5eEvents.RollComponent{
						Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
						Dice:   testDiceTrace(6, 1),
					}},
				{
					Source: dnd5eEvents.DamageSourceAbility,
					Roll: dnd5eEvents.RollComponent{
						Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
						Modifier: intPtr(0), // STR +0 (should be replaced with DEX +3)
					},
				},
			},
			AbilityUsed: abilities.STR,
		}

		// Execute through damage chain, with the monk in the cast: Martial Arts
		// reads its own ability scores out of it. See castOf for why installing
		// one here stands in for resolution's door rather than inventing a
		// registry nothing in production builds.
		ctx := castOf(s.ctx, s.monk)
		damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		damageTopic := dnd5eEvents.DamageChain.On(s.bus)
		modifiedChain, err := damageTopic.PublishWithChain(ctx, damageEvent, damageChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
		s.Require().NoError(err)

		// Verify DEX was used (ability component should have +3)
		var abilityBonus int
		for _, comp := range finalEvent.Components {
			if comp.Source == dnd5eEvents.DamageSourceAbility {
				abilityBonus = comp.Total()
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

		// The condition rolls with its own dice.NewRoller(), so the VALUE is
		// non-deterministic and the DICE STRING is not. The seeded roll below is
		// deliberately outside 1d4's range: this test used to seed a 1, which
		// is a legal 1d4 result, so every assertion here passed unchanged when
		// the condition did not fire at all. It said so in this very comment —
		// "we verify the damage string is upgraded" — while asserting no such
		// thing.

		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID: s.monk.GetID(),
			TargetID:   s.goblin.GetID(),
			WeaponRef:  refs.Weapons.UnarmedStrike(),
			Components: []dnd5eEvents.DamageComponent{
				{
					Source:     dnd5eEvents.DamageSourceWeapon,
					Properties: []damage.Property{damage.AddsAttackAbilityModifier},
					Roll: dnd5eEvents.RollComponent{
						Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
						Dice:   testDiceTrace(7, 7),
					}},
				{
					Source: dnd5eEvents.DamageSourceAbility,
					Roll: dnd5eEvents.RollComponent{
						Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
						Modifier: intPtr(0),
					},
				},
			},
			AbilityUsed: abilities.STR,
		}

		// Execute through damage chain, with the monk in the cast: Martial Arts
		// reads its own ability scores out of it. See castOf for why installing
		// one here stands in for resolution's door rather than inventing a
		// registry nothing in production builds.
		ctx := castOf(s.ctx, s.monk)
		damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		damageTopic := dnd5eEvents.DamageChain.On(s.bus)
		modifiedChain, err := damageTopic.PublishWithChain(ctx, damageEvent, damageChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
		s.Require().NoError(err)

		// The dice STRING is the deterministic half, and the one the comment
		// above always claimed was checked.
		var weaponRolls []int
		var weaponDice string
		for _, comp := range finalEvent.Components {
			if comp.Source == dnd5eEvents.DamageSourceWeapon {
				weaponRolls = comp.Roll.Dice.FinalRolls
				weaponDice = comp.Roll.Dice.Notation
				break
			}
		}
		s.Equal("d4", weaponDice, "a level 1 monk's unarmed strike is upgraded from the weapon's own die to one d4 (canonical trace notation)")
		s.Equal("1d4", finalEvent.WeaponDamageDice, "and the event carries the same upgrade for the combat log")

		s.Require().Len(weaponRolls, 1, "Should have exactly 1 die roll (1d4)")
		s.True(weaponRolls[0] >= 1 && weaponRolls[0] <= 4,
			"the seeded 7 must have been re-rolled on 1d4, got %d", weaponRolls[0])

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
					Source:     dnd5eEvents.DamageSourceWeapon,
					Properties: []damage.Property{damage.AddsAttackAbilityModifier},
					Roll: dnd5eEvents.RollComponent{
						Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
						Dice:   testDiceTrace(6, 5),
					}},
				{
					Source: dnd5eEvents.DamageSourceAbility,
					Roll: dnd5eEvents.RollComponent{
						Source:   dnd5eEvents.RollSource{Ref: refs.Abilities.Strength(), Name: "Strength"},
						Modifier: intPtr(0), // STR +0 (will be replaced with DEX +3)
					},
				},
			},
			AbilityUsed: abilities.STR,
		}

		// Execute through damage chain, with the monk in the cast: Martial Arts
		// reads its own ability scores out of it. See castOf for why installing
		// one here stands in for resolution's door rather than inventing a
		// registry nothing in production builds.
		ctx := castOf(s.ctx, s.monk)
		damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		damageTopic := dnd5eEvents.DamageChain.On(s.bus)
		modifiedChain, err := damageTopic.PublishWithChain(ctx, damageEvent, damageChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(ctx, damageEvent)
		s.Require().NoError(err)

		// Verify DEX was used
		var abilityBonus int
		for _, comp := range finalEvent.Components {
			if comp.Source == dnd5eEvents.DamageSourceAbility {
				abilityBonus = comp.Total()
				break
			}
		}
		s.Equal(3, abilityBonus, "Should use DEX (+3) for monk weapon")

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

		// Verify the character's stored AC was set correctly during creation.
		// Note: For the AC chain to work during combat, the UnarmoredDefenseCondition
		// must be in the character's Conditions JSON AND the game context must have
		// the character's ability scores. See TestUnarmoredDefense_ACChainIncludesWIS.

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
		s.T().Log("")
		s.T().Log("✓ Character stored AC matches expected Unarmored Defense formula")
	})
}

// TestUnarmoredDefense_ACChainIncludesWIS verifies that when a Monk with
// Unarmored Defense is the defender in combat, the AC chain used in
// The canonical AC chain includes the WIS modifier (not just 10 + DEX).
// Issue #456: Combat log shows Monk AC as 13 (10 + DEX 3) but should be
// 15 (10 + DEX 3 + WIS 2) — the WIS bonus was missing from AC chain.
func (s *MonkEncounterSuite) TestUnarmoredDefense_ACChainIncludesWIS() {
	s.Run("Monk AC chain includes WIS modifier from Unarmored Defense", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK UNARMORED DEFENSE: AC Chain WIS Modifier (Issue #456)     ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")

		// Create a monk with DEX 16 (+3) and WIS 14 (+2).
		// Unarmored Defense: AC = 10 + DEX(+3) + WIS(+2) = 15.
		// The bug: without the WIS component in the ACChain, EffectiveAC
		// returns only 13 (10 + DEX 3), causing attacks to incorrectly hit.
		monkWithUD := &character.Data{
			ID:               "monk-ud-test",
			PlayerID:         "player-1",
			Name:             "Wisdom Monk",
			Level:            1,
			ProficiencyBonus: 2,
			RaceID:           races.Human,
			ClassID:          classes.Monk,
			AbilityScores: shared.AbilityScores{
				abilities.STR: 10, // +0
				abilities.DEX: 16, // +3
				abilities.CON: 12, // +1
				abilities.INT: 10, // +0
				abilities.WIS: 14, // +2
				abilities.CHA: 8,  // -1
			},
			HitPoints:    10,
			MaxHitPoints: 10,
			ArmorClass:   15, // 10 + DEX(3) + WIS(2)
			Skills: map[skills.Skill]shared.ProficiencyLevel{
				skills.Acrobatics: shared.Proficient,
				skills.Stealth:    shared.Proficient,
			},
			SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
				abilities.STR: shared.Proficient,
				abilities.DEX: shared.Proficient,
			},
			// Both conditions must be present: martial_arts and unarmored_defense
			Conditions: []json.RawMessage{
				json.RawMessage(`{
					"ref": {"module": "dnd5e", "type": "conditions", "id": "martial_arts"},
					"member_id": "monk-ud-test",
					"monk_level": 1
				}`),
				json.RawMessage(`{
					"ref": {"module": "dnd5e", "type": "conditions", "id": "unarmored_defense"},
					"type": "monk",
					"member_id": "monk-ud-test",
					"source": "dnd5e:classes:monk"
				}`),
			},
		}

		bus := events.NewEventBus()
		monk, err := character.LoadFromData(s.ctx, monkWithUD, bus)
		s.Require().NoError(err)
		defer func() { _ = monk.Cleanup(s.ctx) }()

		// The cast, installed the way resolution's door installs it — see
		// castOf for why standing in for a real installer is not the
		// hand-built registry this test used to carry. Unarmored Defense reads
		// the monk out of the cast by its own ID, exactly as it would read the
		// creature next to it.
		ctx := castOf(s.ctx, monk)

		// Verify EffectiveAC includes WIS modifier via the AC chain.
		// Expected: 10 (base) + 3 (DEX) + 2 (WIS from UnarmoredDefense) = 15
		breakdown, acErr := monk.EffectiveAC(ctx)
		s.Require().NoError(acErr)

		s.T().Logf("  Monk EffectiveAC breakdown:")
		s.T().Logf("    Total: %d", breakdown.Total)
		for _, comp := range breakdown.Components {
			s.T().Logf("    Component: type=%s value=%d", comp.Type, comp.Value)
		}

		s.Equal(15, breakdown.Total,
			"Monk AC chain should be 10 + DEX(+3) + WIS(+2) = 15, not just 10 + DEX = 13")

		// Verify the WIS component is present in the breakdown
		hasWISComponent := false
		for _, comp := range breakdown.Components {
			if comp.Type == combat.ACSourceFeature && comp.Value == 2 {
				hasWISComponent = true
				break
			}
		}
		s.True(hasWISComponent,
			"AC breakdown should contain a Feature component with value 2 (WIS modifier from Unarmored Defense)")

		s.T().Log("")
		s.T().Log("✓ Monk Unarmored Defense WIS modifier included in AC chain")
	})
}

// TestUnarmoredDefense_ACChainReadsTheCast asserts BOTH halves of the read
// law: with the cast installed the monk folds 15, and with a bare context the
// same sheet folds 13.
//
// # This test has been wrong twice, in opposite directions
//
// It began as TestUnarmoredDefense_ACChainWithoutGameContext, expecting 13 and
// explaining the missing +2 as an "API WIRING REQUIREMENT: the context MUST
// include a GameContext with the defender's ability scores". Nothing ever met
// that requirement — gamectx.WithGameContext had zero non-test call sites — so
// every monk fought at base AC in every real fight and this test certified it
// as correct.
//
// It then became TestUnarmoredDefense_ACChainNeedsNoGameContext, expecting 15
// from a bare context, because the condition had been handed its own sheet at
// attach time. True while that handle existed. The handle is gone: an effect
// reads itself out of the cast, like any other participant.
//
// # So why is asserting 13 not the original sin repeating
//
// Because of WHERE the 13 happens. The first version blessed the number
// production actually got. This one pins the number a fold gets when it runs
// OUTSIDE resolution — which R6 calls the bug rather than a mode. Production
// folds inside, where one door installs the cast unconditionally, and that is
// pinned a level up by session's TestAMonksUnarmoredDefenseReachesTheJoinedAC
// (Join → resolution.ProjectCharacter → 15 on the wire).
//
// Keeping the 13 visible here is the point. It is the observable edge of the
// migration: any caller still folding an AC chain on a bare context is one
// that has to come to resolution, and it now says so in a test instead of
// being discovered as a wrong number in somebody's character sheet.
func (s *MonkEncounterSuite) TestUnarmoredDefense_ACChainReadsTheCast() {
	s.Run("Monk AC chain reads WIS off the installed cast", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK UNARMORED DEFENSE: The Cast Is The Read Channel            ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")
		s.T().Log("  With the cast installed the monk reads its own sheet and folds")
		s.T().Log("  15. With a bare context nobody can name this character, the")
		s.T().Log("  chain is left untouched, and 13 is what a fold outside")
		s.T().Log("  resolution is worth.")
		s.T().Log("")

		monkWithUD := &character.Data{
			ID:               "monk-ud-no-ctx",
			PlayerID:         "player-1",
			Name:             "Wisdom Monk",
			Level:            1,
			ProficiencyBonus: 2,
			RaceID:           races.Human,
			ClassID:          classes.Monk,
			AbilityScores: shared.AbilityScores{
				abilities.STR: 10,
				abilities.DEX: 16, // +3
				abilities.CON: 12,
				abilities.INT: 10,
				abilities.WIS: 14, // +2
				abilities.CHA: 8,
			},
			HitPoints:    10,
			MaxHitPoints: 10,
			ArmorClass:   15,
			Skills: map[skills.Skill]shared.ProficiencyLevel{
				skills.Acrobatics: shared.Proficient,
				skills.Stealth:    shared.Proficient,
			},
			SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
				abilities.STR: shared.Proficient,
				abilities.DEX: shared.Proficient,
			},
			Conditions: []json.RawMessage{
				json.RawMessage(`{
					"ref": {"module": "dnd5e", "type": "conditions", "id": "martial_arts"},
					"member_id": "monk-ud-no-ctx",
					"monk_level": 1
				}`),
				json.RawMessage(`{
					"ref": {"module": "dnd5e", "type": "conditions", "id": "unarmored_defense"},
					"type": "monk",
					"member_id": "monk-ud-no-ctx",
					"source": "dnd5e:classes:monk"
				}`),
			},
		}

		bus := events.NewEventBus()
		monk, err := character.LoadFromData(s.ctx, monkWithUD, bus)
		s.Require().NoError(err)
		defer func() { _ = monk.Cleanup(s.ctx) }()

		// WITH the cast: the monk can find itself, and folds the full number.
		withCast, acErr := monk.EffectiveAC(castOf(context.Background(), monk))
		s.Require().NoError(acErr)

		s.T().Logf("  Monk EffectiveAC (cast installed): %d", withCast.Total)

		// 10 (base) + 3 (DEX) + 2 (WIS via Unarmored Defense) = 15
		s.Equal(15, withCast.Total,
			"read off the cast: 10 + DEX(+3) + WIS(+2) = 15")

		hasWIS := false
		for _, comp := range withCast.Components {
			if comp.Type == combat.ACSourceFeature && comp.Value == 2 {
				hasWIS = true
			}
		}
		s.True(hasWIS, "the WIS component must be attributed in the breakdown, not just folded into the total")

		// WITHOUT it: nobody can name this character, so the condition leaves
		// the chain alone. NOT an error — an erroring contributor would take
		// every other AC contributor down with it, which is the failure this
		// whole channel exists to stop.
		bare, bareErr := monk.EffectiveAC(context.Background())
		s.Require().NoError(bareErr,
			"a condition that cannot answer leaves the chain untouched; it must not poison the fold")

		s.T().Logf("  Monk EffectiveAC (bare context):  %d", bare.Total)

		s.Equal(13, bare.Total,
			"10 + DEX(+3) and nothing else: a fold outside resolution has no cast to read")
		for _, comp := range bare.Components {
			s.NotEqual(combat.ACSourceFeature, comp.Type,
				"with no cast there is no feature contribution to attribute")
		}
	})
}

// =============================================================================
// LEVEL 2: KI FEATURE TESTS — REAL FEATURE ACTIVATION
// =============================================================================

func (s *MonkEncounterSuite) TestFlurryOfBlows_BanksTwoStrikes() {
	s.Run("Flurry of Blows consumes 1 Ki and banks 2 flurry strikes", func() {
		if s.monk != nil {
			_ = s.monk.Cleanup(s.ctx)
		}
		s.monk = s.createLevel2Monk()

		_, err := s.monk.StartTurn(s.ctx, &character.StartTurnInput{TurnNumber: 1, Speed: 30})
		s.Require().NoError(err)

		ki := s.monk.GetResource(resources.Ki)
		s.Require().NotNil(ki)
		s.Require().Equal(2, ki.Current())
		s.Zero(s.monk.CapacityLeft(combat.CapacityFlurryStrike))

		flurry := s.monk.GetFeature("flurry_of_blows")
		s.Require().NotNil(flurry)

		err = flurry.Activate(s.ctx, s.monk, features.FeatureInput{})

		s.Require().NoError(err)
		s.Equal(1, ki.Current())
		s.Equal(2, s.monk.CapacityLeft(combat.CapacityFlurryStrike))
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

		// No weapons registry, deliberately. The shield question is answered by
		// the member surface every combatant carries, read out of the installed
		// cast — see castOf. The registry this used to build was never
		// installed outside a test.

		s.T().Logf("  Monk: %s (Level 2, unarmored)", s.monk.GetName())
		s.T().Log("")

		// Find the UnarmoredMovementCondition from loaded conditions
		var umCondition interface {
			SpeedBonus(context.Context) (int, bool)
		}
		for _, cond := range s.monk.GetConditions() {
			if getter, ok := cond.(interface {
				SpeedBonus(context.Context) (int, bool)
			}); ok {
				umCondition = getter
				break
			}
		}
		s.Require().NotNil(umCondition, "Monk should have UnarmoredMovementCondition loaded from Data")

		// Verify speed bonus, with the monk in the cast — the condition reads
		// its own shield state off the member surface.
		bonus, known := umCondition.SpeedBonus(castOf(s.ctx, s.monk))
		s.Require().True(known, "the monk is in the cast, so the shield question has an answer")
		s.Equal(10, bonus, "Level 2 monk should get +10 ft speed bonus")

		// And with nobody in the cast the answer is UNKNOWN rather than zero.
		// Zero would read as "this monk is carrying a shield", which is a rule
		// invented out of missing data — the distinction the second return
		// exists to keep expressible.
		bare, bareKnown := umCondition.SpeedBonus(context.Background())
		s.False(bareKnown, "no cast, no answer — not a silent zero")
		s.Zero(bare)

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

// TestMartialArts_UnarmedStrikeTypedCoverage documents the superseded
// end-to-end scenario; typed Martial Arts chain tests above own this behavior.
func (s *MonkEncounterSuite) TestMartialArts_UnarmedStrikeTypedCoverage() {
	s.Run("typed unarmed strike coverage is exercised above", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  MONK UNARMED STRIKE: Typed Damage Coverage                      ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")
		s.T().Log("")
		s.T().Logf("  Monk: %s (Level 1, STR +0, DEX +3)", s.monk.GetName())
		s.T().Logf("  Target: Goblin (AC 13, HP 7)")
		s.T().Log("")

		s.T().Skip("end-to-end unarmed Strike is covered by the typed Martial Arts chain tests above")

		s.T().Log("")
		s.T().Log("✓ Monk unarmed strike coverage is provided by typed Martial Arts tests")
	})
}

func TestMonkEncounterSuite(t *testing.T) {
	suite.Run(t, new(MonkEncounterSuite))
}
