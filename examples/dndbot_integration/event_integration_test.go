package dndbot

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

func TestCompleteEventIntegration(t *testing.T) {
	t.Run("combat flow with modifiers", func(t *testing.T) {
		// Create integration
		integration := NewCompleteEventIntegration()
		integration.SetupCombatHandlers()

		// Add fighter proficiencies
		err := integration.proficiency.AddCharacterProficiencies("fighter-123", 5)
		require.NoError(t, err)

		// Create combatants
		fighter := WrapCharacter("fighter-123", "Fighter", 5)
		goblin := WrapCharacter("goblin-456", "Goblin", 1)

		ctx := context.Background()

		// Test attack roll with proficiency
		attackEvent := events.NewGameEvent(events.EventOnAttackRoll, fighter, goblin)
		attackEvent.Context().Set("weapon", "longsword")
		attackEvent.Context().Set("has_advantage", true)

		err = integration.bus.Publish(ctx, attackEvent)
		require.NoError(t, err)

		// Should have proficiency modifier
		modifiers := attackEvent.Context().Modifiers()
		require.Len(t, modifiers, 1)

		mod := modifiers[0]
		assert.Equal(t, "proficiency", mod.Source())
		assert.Equal(t, events.ModifierAttackBonus, mod.Type())
		assert.Equal(t, 3, mod.ModifierValue().GetValue()) // Level 5 = +3

		// Check advantage was set
		hasAdvantage, ok := attackEvent.Context().GetBool("roll_advantage")
		assert.True(t, ok)
		assert.True(t, hasAdvantage)
	})

	t.Run("damage with rage bonus", func(t *testing.T) {
		integration := NewCompleteEventIntegration()
		integration.SetupCombatHandlers()

		barbarian := WrapCharacter("barb-123", "Barbarian", 5)
		goblin := WrapCharacter("goblin-456", "Goblin", 1)

		ctx := context.Background()

		// Test damage roll with rage
		damageEvent := events.NewGameEvent(events.EventOnDamageRoll, barbarian, goblin)
		damageEvent.Context().Set("damage_type", "melee")
		damageEvent.Context().Set("has_rage", true)

		err := integration.bus.Publish(ctx, damageEvent)
		require.NoError(t, err)

		// Should have rage modifier
		modifiers := damageEvent.Context().Modifiers()
		require.Len(t, modifiers, 1)

		mod := modifiers[0]
		assert.Equal(t, "rage", mod.Source())
		assert.Equal(t, 2, mod.ModifierValue().GetValue())
	})

	t.Run("damage resistance", func(t *testing.T) {
		integration := NewCompleteEventIntegration()
		integration.SetupCombatHandlers()

		attacker := WrapCharacter("fighter-123", "Fighter", 5)
		defender := WrapCharacter("barb-456", "Barbarian", 5)

		ctx := context.Background()

		// Test taking damage with resistance
		takeDamageEvent := events.NewGameEvent(events.EventBeforeTakeDamage, attacker, defender)
		takeDamageEvent.Context().Set("damage_type", "slashing")
		takeDamageEvent.Context().Set("damage_amount", 10)
		takeDamageEvent.Context().Set("resistances", "slashing,piercing,bludgeoning")

		err := integration.bus.Publish(ctx, takeDamageEvent)
		require.NoError(t, err)

		// Should have resistance modifier
		modifiers := takeDamageEvent.Context().Modifiers()
		require.Len(t, modifiers, 1)

		mod := modifiers[0]
		assert.Equal(t, "resistance", mod.Source())
		assert.Equal(t, -5, mod.ModifierValue().GetValue()) // Negative to reduce damage
	})

	t.Run("saving throw with proficiency and bless", func(t *testing.T) {
		integration := NewCompleteEventIntegration()
		integration.SetupCombatHandlers()
		integration.SetupSaveHandlers()

		// Add fighter with STR save proficiency
		err := integration.proficiency.AddCharacterProficiencies("fighter-123", 5)
		require.NoError(t, err)

		fighter := WrapCharacter("fighter-123", "Fighter", 5)

		ctx := context.Background()

		// Test strength save with bless
		saveEvent := events.NewGameEvent(events.EventOnSavingThrow, fighter, nil)
		saveEvent.Context().Set("ability", "strength")
		saveEvent.Context().Set("dc", 15)
		saveEvent.Context().Set("has_bless", true)

		err = integration.bus.Publish(ctx, saveEvent)
		require.NoError(t, err)

		// Should have both proficiency and bless modifiers
		modifiers := saveEvent.Context().Modifiers()
		require.Len(t, modifiers, 2)

		// Check modifiers (order may vary)
		var hasProficiency, hasBless bool
		for _, mod := range modifiers {
			switch mod.Source() {
			case "proficiency":
				hasProficiency = true
				assert.Equal(t, 3, mod.ModifierValue().GetValue())
			case "bless":
				hasBless = true
				// Bless is 1d4, average 2.5, placeholder returns 3
				assert.GreaterOrEqual(t, mod.ModifierValue().GetValue(), 1)
				assert.LessOrEqual(t, mod.ModifierValue().GetValue(), 4)
			}
		}

		assert.True(t, hasProficiency, "Should have proficiency modifier")
		assert.True(t, hasBless, "Should have bless modifier")
	})

	t.Run("condition effects", func(t *testing.T) {
		integration := NewCompleteEventIntegration()
		integration.SetupConditionHandlers()

		character := WrapCharacter("char-123", "Character", 5)

		ctx := context.Background()

		// Apply paralyzed condition
		conditionEvent := events.NewGameEvent(events.EventOnConditionApplied, nil, character)
		conditionEvent.Context().Set("condition", "paralyzed")

		err := integration.bus.Publish(ctx, conditionEvent)
		require.NoError(t, err)

		// Check condition effects were set
		autoFailStr, _ := conditionEvent.Context().GetBool("auto_fail_str_saves")
		autoFailDex, _ := conditionEvent.Context().GetBool("auto_fail_dex_saves")
		grantAdvantage, _ := conditionEvent.Context().GetBool("grant_advantage_against")

		assert.True(t, autoFailStr, "Paralyzed should auto-fail STR saves")
		assert.True(t, autoFailDex, "Paralyzed should auto-fail DEX saves")
		assert.True(t, grantAdvantage, "Paralyzed grants advantage to attackers")
	})

	t.Run("old and new handlers coexist", func(t *testing.T) {
		integration := NewCompleteEventIntegration()

		oldHandlerCalled := false
		newHandlerCalled := false

		// Old style handler
		integration.adapter.Subscribe("OnAttackRoll", 100, func(_ interface{}) error {
			oldHandlerCalled = true
			return nil
		})

		// New style handler
		integration.bus.SubscribeFunc(events.EventOnAttackRoll, 100, func(_ context.Context, _ events.Event) error {
			newHandlerCalled = true
			return nil
		})

		// Publish should trigger both
		err := integration.adapter.Publish("OnAttackRoll", map[string]interface{}{
			"test": true,
		})
		require.NoError(t, err)

		assert.True(t, oldHandlerCalled, "Old handler should be called")
		assert.True(t, newHandlerCalled, "New handler should be called")
	})
}

// Test the example flow runs without errors
func TestExampleCompleteIntegration(_ *testing.T) {
	// This just ensures the example runs without panicking
	ExampleCompleteIntegration()
}
