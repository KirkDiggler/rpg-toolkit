// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package integration provides comprehensive encounter-level integration tests
// that demonstrate how each class's features work in combat scenarios.
// These tests serve as both verification AND documentation for toolkit integrators.
package integration

import (
	"context"
	"fmt"
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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ============================================================================
// FIGHTER ENCOUNTER TEST SUITE
// Level 1 Fighter Features:
//   - Second Wind (bonus action, heal 1d10 + level, once per short/long rest)
//   - Fighting Style (Defense: +1 AC with armor, Dueling: +2 damage one-handed, etc.)
//   - Proficiencies (all armor, shields, simple/martial weapons)
// ============================================================================

type FighterEncounterSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	bus        events.EventBus
	mockRoller *mock_dice.MockRoller
	lookup     *integrationLookup
	room       spatial.Room
	registry   *gamectx.BasicCharacterRegistry

	fighter *mockFighterCharacter
	goblin  *monster.Monster
	rapier  *weapons.Weapon
}

// mockFighterCharacter implements the interfaces needed for fighter testing
type mockFighterCharacter struct {
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

func (m *mockFighterCharacter) GetID() string                                  { return m.id }
func (m *mockFighterCharacter) GetType() core.EntityType                       { return "character" }
func (m *mockFighterCharacter) GetName() string                                { return m.name }
func (m *mockFighterCharacter) GetLevel() int                                  { return m.level }
func (m *mockFighterCharacter) AbilityScores() shared.AbilityScores            { return m.abilityScores }
func (m *mockFighterCharacter) ProficiencyBonus() int                          { return m.proficiencyBonus }
func (m *mockFighterCharacter) GetHitPoints() int                              { return m.hitPoints }
func (m *mockFighterCharacter) GetMaxHitPoints() int                           { return m.maxHitPoints }
func (m *mockFighterCharacter) AC() int                                        { return m.armorClass }
func (m *mockFighterCharacter) IsDirty() bool                                  { return false }
func (m *mockFighterCharacter) MarkClean()                                     {}
func (m *mockFighterCharacter) GetConditions() []dnd5eEvents.ConditionBehavior { return m.conditions }

func (m *mockFighterCharacter) ApplyDamage(_ context.Context, input *combat.ApplyDamageInput) *combat.ApplyDamageResult {
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

func (m *mockFighterCharacter) ApplyHealing(_ context.Context, amount int) int {
	previousHP := m.hitPoints
	m.hitPoints += amount
	if m.hitPoints > m.maxHitPoints {
		m.hitPoints = m.maxHitPoints
	}
	return m.hitPoints - previousHP
}

func TestFighterEncounterSuite(t *testing.T) {
	suite.Run(t, new(FighterEncounterSuite))
}

func (s *FighterEncounterSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.bus = events.NewEventBus()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
	s.lookup = newIntegrationLookup()
	s.ctx = context.Background()

	grid := spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 10, Height: 10})
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{ID: "combat-room", Type: "combat", Grid: grid})
}

func (s *FighterEncounterSuite) SetupSubTest() {
	s.bus = events.NewEventBus()
	s.lookup = newIntegrationLookup()

	s.fighter = s.createLevel1Fighter()
	s.goblin = s.createGoblin()
	s.rapier = s.createRapier()

	s.lookup.Add(s.fighter)
	s.lookup.Add(s.goblin)

	s.registry = gamectx.NewBasicCharacterRegistry()
	scores := &gamectx.AbilityScores{
		Strength: 16, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 12, Charisma: 10,
	}
	s.registry.AddAbilityScores(s.fighter.GetID(), scores)

	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{CharacterRegistry: s.registry})
	s.ctx = combat.WithCombatantLookup(context.Background(), s.lookup)
	s.ctx = gamectx.WithGameContext(s.ctx, gameCtx)
	s.ctx = gamectx.WithRoom(s.ctx, s.room)

	_ = s.room.PlaceEntity(s.fighter, spatial.Position{X: 2, Y: 2})
	_ = s.room.PlaceEntity(s.goblin, spatial.Position{X: 3, Y: 2})
}

func (s *FighterEncounterSuite) TearDownSubTest() {
	_ = s.room.RemoveEntity(s.fighter.GetID())
	_ = s.room.RemoveEntity(s.goblin.GetID())
}

func (s *FighterEncounterSuite) TearDownTest() {
	s.ctrl.Finish()
}

// =============================================================================
// CHARACTER CREATION HELPERS
// =============================================================================

func (s *FighterEncounterSuite) createLevel1Fighter() *mockFighterCharacter {
	return &mockFighterCharacter{
		id: "fighter-1", name: "Sir Aldric", level: 1, proficiencyBonus: 2,
		hitPoints: 12, maxHitPoints: 12, armorClass: 16, // Chain mail + shield
		abilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 10,
		},
		conditions: []dnd5eEvents.ConditionBehavior{},
	}
}

func (s *FighterEncounterSuite) createGoblin() *monster.Monster {
	return monster.New(monster.Config{
		ID: "goblin-1", Name: "Goblin", AC: 15, HP: 7,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8, abilities.DEX: 14, abilities.CON: 10,
			abilities.INT: 10, abilities.WIS: 8, abilities.CHA: 8,
		},
	})
}

func (s *FighterEncounterSuite) createRapier() *weapons.Weapon {
	weapon, err := weapons.GetByID(weapons.Rapier)
	s.Require().NoError(err)
	return &weapon
}

// createSecondWind creates a Second Wind feature for testing.
func (s *FighterEncounterSuite) createSecondWind(level int, characterID string) *features.SecondWind {
	// Create config with level
	config := []byte(`{"level": ` + string(rune('0'+level)) + `}`)
	if level >= 10 {
		// Handle multi-digit levels
		config = []byte(`{"level": ` + fmt.Sprintf("%d", level) + `}`)
	}

	// Use the factory to create Second Wind properly
	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.SecondWind().String(),
		Config:      config,
		CharacterID: characterID,
	})
	s.Require().NoError(err)
	secondWind, ok := output.Feature.(*features.SecondWind)
	s.Require().True(ok, "Expected SecondWind feature")
	return secondWind
}

// =============================================================================
// SECOND WIND TESTS
// =============================================================================

func (s *FighterEncounterSuite) TestSecondWind_HealsCharacter() {
	s.Run("Second Wind heals 1d10 + fighter level", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  FIGHTER SECOND WIND: Basic Healing                              ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		// Fighter took 8 damage
		s.fighter.hitPoints = 4

		// Create Second Wind feature
		secondWind := s.createSecondWind(1, s.fighter.GetID())
		err := secondWind.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = secondWind.Remove(s.ctx, s.bus) }()

		// Subscribe to healing event
		var healingEvent *dnd5eEvents.HealingReceivedEvent
		topic := dnd5eEvents.HealingReceivedTopic.On(s.bus)
		_, err = topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
			healingEvent = &event
			return nil
		})
		s.Require().NoError(err)

		// Activate Second Wind
		err = secondWind.Activate(s.ctx, s.fighter, features.FeatureInput{
			Bus: s.bus,
		})
		s.Require().NoError(err)

		// Verify healing event was published
		s.Require().NotNil(healingEvent, "Should publish HealingReceivedEvent")
		s.Equal(s.fighter.GetID(), healingEvent.TargetID)
		s.Equal(1, healingEvent.Modifier, "Modifier should be fighter level (1)")
		s.GreaterOrEqual(healingEvent.Roll, 1, "Roll should be at least 1")
		s.LessOrEqual(healingEvent.Roll, 10, "Roll should be at most 10")
		s.Equal(healingEvent.Roll+healingEvent.Modifier, healingEvent.Amount)

		s.T().Logf("✓ Second Wind healed for %d (rolled %d + %d level)", healingEvent.Amount, healingEvent.Roll, healingEvent.Modifier)
	})
}

func (s *FighterEncounterSuite) TestSecondWind_OncePerShortRest() {
	s.Run("Second Wind can only be used once per short rest", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  FIGHTER SECOND WIND: Once Per Short Rest                        ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		s.fighter.hitPoints = 4

		secondWind := s.createSecondWind(1, s.fighter.GetID())
		err := secondWind.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = secondWind.Remove(s.ctx, s.bus) }()

		// First use should succeed
		err = secondWind.Activate(s.ctx, s.fighter, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)

		// Second use should fail
		err = secondWind.Activate(s.ctx, s.fighter, features.FeatureInput{Bus: s.bus})
		s.Require().Error(err)
		s.Contains(err.Error(), "no second wind uses remaining")

		s.T().Log("✓ Second Wind correctly limited to once per short rest")
	})
}

func (s *FighterEncounterSuite) TestSecondWind_ResetsOnShortRest() {
	s.Run("Second Wind resets on short rest", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  FIGHTER SECOND WIND: Resets on Short Rest                       ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		s.fighter.hitPoints = 4

		secondWind := s.createSecondWind(1, s.fighter.GetID())
		err := secondWind.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = secondWind.Remove(s.ctx, s.bus) }()

		// Use Second Wind
		err = secondWind.Activate(s.ctx, s.fighter, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)

		// Verify it's exhausted
		err = secondWind.CanActivate(s.ctx, s.fighter, features.FeatureInput{})
		s.Require().Error(err)

		// Publish short rest event
		restTopic := dnd5eEvents.RestTopic.On(s.bus)
		err = restTopic.Publish(s.ctx, dnd5eEvents.RestEvent{
			RestType:    coreResources.ResetShortRest,
			CharacterID: s.fighter.GetID(),
		})
		s.Require().NoError(err)

		// Should be able to use again
		err = secondWind.CanActivate(s.ctx, s.fighter, features.FeatureInput{})
		s.NoError(err, "Second Wind should be available after short rest")

		s.T().Log("✓ Second Wind correctly resets on short rest")
	})
}

func (s *FighterEncounterSuite) TestSecondWind_ScalesWithLevel() {
	s.Run("Second Wind healing scales with fighter level", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  FIGHTER SECOND WIND: Level Scaling                              ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		s.fighter.hitPoints = 4
		s.fighter.level = 5 // Level 5 fighter

		secondWind := s.createSecondWind(5, s.fighter.GetID()) // Level 5
		err := secondWind.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = secondWind.Remove(s.ctx, s.bus) }()

		var healingEvent *dnd5eEvents.HealingReceivedEvent
		topic := dnd5eEvents.HealingReceivedTopic.On(s.bus)
		_, err = topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
			healingEvent = &event
			return nil
		})
		s.Require().NoError(err)

		err = secondWind.Activate(s.ctx, s.fighter, features.FeatureInput{Bus: s.bus})
		s.Require().NoError(err)

		s.Require().NotNil(healingEvent)
		s.Equal(5, healingEvent.Modifier, "Modifier should be fighter level (5)")
		s.Equal(healingEvent.Roll+5, healingEvent.Amount, "Total should be 1d10 + 5")

		s.T().Logf("✓ Second Wind at level 5 healed for %d (rolled %d + 5)", healingEvent.Amount, healingEvent.Roll)
	})
}

// =============================================================================
// FIGHTING STYLE: DEFENSE TESTS
// =============================================================================

func (s *FighterEncounterSuite) TestFightingStyleDefense_AddsACWithArmor() {
	s.Run("Defense fighting style adds +1 AC when wearing armor", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  FIGHTER DEFENSE: +1 AC With Armor                               ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		// Apply Defense fighting style
		defense := conditions.NewFightingStyleDefenseCondition(s.fighter.GetID())
		err := defense.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = defense.Remove(s.ctx, s.bus) }()

		// Create AC chain event (fighter is wearing armor)
		acEvent := &combat.ACChainEvent{
			CharacterID: s.fighter.GetID(),
			HasArmor:    true,
			Breakdown:   &combat.ACBreakdown{Total: 16}, // Chain mail + shield
		}

		// Execute through AC chain
		acChain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
		acTopic := combat.ACChain.On(s.bus)
		modifiedChain, err := acTopic.PublishWithChain(s.ctx, acEvent, acChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, acEvent)
		s.Require().NoError(err)

		// Verify +1 AC bonus was added
		s.Equal(17, finalEvent.Breakdown.Total, "AC should be 16 + 1 = 17")

		s.T().Log("✓ Defense fighting style correctly adds +1 AC with armor")
	})
}

func (s *FighterEncounterSuite) TestFightingStyleDefense_NoBonus_WithoutArmor() {
	s.Run("Defense fighting style does NOT add AC without armor", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  FIGHTER DEFENSE: No Bonus Without Armor                         ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		defense := conditions.NewFightingStyleDefenseCondition(s.fighter.GetID())
		err := defense.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = defense.Remove(s.ctx, s.bus) }()

		// Fighter is NOT wearing armor (unarmored)
		acEvent := &combat.ACChainEvent{
			CharacterID: s.fighter.GetID(),
			HasArmor:    false,                          // No armor!
			Breakdown:   &combat.ACBreakdown{Total: 12}, // Just DEX
		}

		acChain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)
		acTopic := combat.ACChain.On(s.bus)
		modifiedChain, err := acTopic.PublishWithChain(s.ctx, acEvent, acChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, acEvent)
		s.Require().NoError(err)

		// AC should remain unchanged
		s.Equal(12, finalEvent.Breakdown.Total, "AC should remain 12 without armor")

		s.T().Log("✓ Defense fighting style correctly denied without armor")
	})
}

// =============================================================================
// FIGHTING STYLE: DUELING TESTS
// =============================================================================

func (s *FighterEncounterSuite) TestFightingStyleDueling_AddsDamage() {
	s.Run("Dueling fighting style adds +2 damage with one-handed melee weapon", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  FIGHTER DUELING: +2 Damage One-Handed                           ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		// Set up weapons for the fighter (rapier in main hand, nothing in off hand)
		mainHand := &gamectx.EquippedWeapon{
			ID:          "rapier-1",
			WeaponID:    weapons.Rapier,
			Name:        "Rapier",
			Slot:        "main_hand",
			IsMelee:     true,
			IsTwoHanded: false,
		}
		weaponSet := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{mainHand})
		s.registry.Add(s.fighter.GetID(), weaponSet)

		// Apply Dueling fighting style
		dueling := conditions.NewFightingStyleDuelingCondition(s.fighter.GetID())
		err := dueling.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = dueling.Remove(s.ctx, s.bus) }()

		// Create damage event
		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID:  s.fighter.GetID(),
			TargetID:    s.goblin.GetID(),
			DamageType:  damage.Piercing,
			AbilityUsed: abilities.STR,
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

		// Verify +2 damage was added
		var duelingBonus int
		for _, comp := range finalEvent.Components {
			if comp.Source == dnd5eEvents.DamageSourceFeature {
				duelingBonus = comp.FlatBonus
				break
			}
		}
		s.Equal(2, duelingBonus, "Dueling should add +2 flat damage")

		s.T().Log("✓ Dueling fighting style correctly adds +2 damage")
	})
}

func (s *FighterEncounterSuite) TestFightingStyleDueling_NoBonus_TwoHanded() {
	s.Run("Dueling does NOT add damage with two-handed weapon", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  FIGHTER DUELING: No Bonus With Two-Handed                       ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		// Fighter wielding a greatsword (two-handed)
		mainHand := &gamectx.EquippedWeapon{
			ID:          "greatsword-1",
			WeaponID:    weapons.Greatsword,
			Name:        "Greatsword",
			Slot:        "main_hand",
			IsMelee:     true,
			IsTwoHanded: true,
		}
		weaponSet := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{mainHand})
		s.registry.Add(s.fighter.GetID(), weaponSet)

		dueling := conditions.NewFightingStyleDuelingCondition(s.fighter.GetID())
		err := dueling.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = dueling.Remove(s.ctx, s.bus) }()

		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID:  s.fighter.GetID(),
			TargetID:    s.goblin.GetID(),
			DamageType:  damage.Slashing,
			AbilityUsed: abilities.STR,
			Components:  []dnd5eEvents.DamageComponent{{Source: dnd5eEvents.DamageSourceWeapon}},
		}

		damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		damageTopic := dnd5eEvents.DamageChain.On(s.bus)
		modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, damageChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
		s.Require().NoError(err)

		// Should only have weapon damage, no dueling bonus
		s.Len(finalEvent.Components, 1, "Should only have weapon damage with two-handed")

		s.T().Log("✓ Dueling correctly denied with two-handed weapon")
	})
}

func (s *FighterEncounterSuite) TestFightingStyleDueling_NoBonus_DualWielding() {
	s.Run("Dueling does NOT add damage when dual wielding", func() {
		s.T().Log("╔══════════════════════════════════════════════════════════════════╗")
		s.T().Log("║  FIGHTER DUELING: No Bonus When Dual Wielding                    ║")
		s.T().Log("╚══════════════════════════════════════════════════════════════════╝")

		// Fighter dual wielding shortswords
		mainHand := &gamectx.EquippedWeapon{
			ID:          "shortsword-1",
			WeaponID:    weapons.Shortsword,
			Name:        "Shortsword",
			Slot:        "main_hand",
			IsMelee:     true,
			IsTwoHanded: false,
		}
		offHand := &gamectx.EquippedWeapon{
			ID:          "shortsword-2",
			WeaponID:    weapons.Shortsword,
			Name:        "Shortsword",
			Slot:        "off_hand",
			IsMelee:     true,
			IsTwoHanded: false,
		}
		weaponSet := gamectx.NewCharacterWeapons([]*gamectx.EquippedWeapon{mainHand, offHand})
		s.registry.Add(s.fighter.GetID(), weaponSet)

		dueling := conditions.NewFightingStyleDuelingCondition(s.fighter.GetID())
		err := dueling.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		defer func() { _ = dueling.Remove(s.ctx, s.bus) }()

		damageEvent := &dnd5eEvents.DamageChainEvent{
			AttackerID:  s.fighter.GetID(),
			TargetID:    s.goblin.GetID(),
			DamageType:  damage.Piercing,
			AbilityUsed: abilities.STR,
			Components:  []dnd5eEvents.DamageComponent{{Source: dnd5eEvents.DamageSourceWeapon}},
		}

		damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
		damageTopic := dnd5eEvents.DamageChain.On(s.bus)
		modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, damageChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, damageEvent)
		s.Require().NoError(err)

		// Should only have weapon damage, no dueling bonus
		s.Len(finalEvent.Components, 1, "Should only have weapon damage when dual wielding")

		s.T().Log("✓ Dueling correctly denied when dual wielding")
	})
}
