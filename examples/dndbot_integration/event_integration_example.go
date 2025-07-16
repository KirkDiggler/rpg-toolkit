// Package dndbot demonstrates comprehensive event bus integration
package dndbot

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// CompleteEventIntegration shows a full DND bot integration with the event bus
type CompleteEventIntegration struct {
	adapter *EventBusAdapter
	bus     *events.Bus

	// Services
	proficiency *ProficiencyIntegration
}

// NewCompleteEventIntegration creates a new complete integration
func NewCompleteEventIntegration() *CompleteEventIntegration {
	adapter := NewEventBusAdapter()
	bus := adapter.GetToolkitBus()

	return &CompleteEventIntegration{
		adapter:     adapter,
		bus:         bus,
		proficiency: NewProficiencyIntegration(bus),
	}
}

// SetupCombatHandlers sets up all combat-related event handlers
func (i *CompleteEventIntegration) SetupCombatHandlers() {
	// Attack roll handler - adds various modifiers
	i.bus.SubscribeFunc(events.EventOnAttackRoll, 100, func(_ context.Context, e events.Event) error {
		// Get attack info
		weapon, _ := e.Context().GetString("weapon")
		if e.Source() == nil {
			return nil
		}

		attackerID := e.Source().GetID()

		// Add proficiency bonus
		if i.proficiency.CheckProficiency(attackerID, weapon) {
			// In real integration, get level from character service
			level := 5 // Example
			bonus := GetProficiencyBonus(level)

			e.Context().AddModifier(events.NewModifier(
				"proficiency",
				events.ModifierAttackBonus,
				events.NewRawValue(bonus, "proficiency"),
				100,
			))

			log.Printf("Added proficiency bonus +%d to attack", bonus)
		}

		// Check for advantage/disadvantage
		if hasAdvantage, _ := e.Context().GetBool("has_advantage"); hasAdvantage {
			e.Context().Set("roll_advantage", true)
			log.Println("Attack has advantage")
		}

		return nil
	})

	// Damage roll handler
	i.bus.SubscribeFunc(events.EventOnDamageRoll, 100, func(_ context.Context, e events.Event) error {
		// Check for critical hit
		if isCrit, _ := e.Context().GetBool("is_critical"); isCrit {
			// For critical hits, we'd typically double the dice, but for now add flat bonus
			e.Context().AddModifier(events.NewModifier(
				"critical",
				events.ModifierDamageBonus,
				events.NewRawValue(8, "critical hit"), // Add base damage again
				200,                                   // High priority
			))
			log.Println("Critical hit! Damage doubled")
		}

		// Check for rage (barbarian)
		if hasRage, _ := e.Context().GetBool("has_rage"); hasRage {
			damageType, _ := e.Context().GetString("damage_type")
			if damageType == "melee" {
				e.Context().AddModifier(events.NewModifier(
					"rage",
					events.ModifierDamageBonus,
					events.NewRawValue(2, "rage"), // +2 for level 1-8
					50,
				))
				log.Println("Rage damage bonus applied")
			}
		}

		return nil
	})

	// Before taking damage - apply resistances
	i.bus.SubscribeFunc(events.EventBeforeTakeDamage, 100, func(_ context.Context, e events.Event) error {
		if e.Target() == nil {
			return nil
		}

		damageType, _ := e.Context().GetString("damage_type")

		// Check for resistances (stored as CSV for simplicity)
		resistancesStr, _ := e.Context().GetString("resistances")
		if resistancesStr != "" {
			resistances := strings.Split(resistancesStr, ",")
			for _, resist := range resistances {
				if resist == damageType {
					// Resistance halves damage - we'd use a negative modifier
					e.Context().AddModifier(events.NewModifier(
						"resistance",
						events.ModifierDamageBonus, // Use negative value
						events.NewRawValue(-5, fmt.Sprintf("resistant to %s", damageType)), // Example: -5 from 10
						100,
					))
					log.Printf("Target has resistance to %s damage", damageType)
					break
				}
			}
		}

		// Check for vulnerabilities (stored as CSV for simplicity)
		vulnerabilitiesStr, _ := e.Context().GetString("vulnerabilities")
		if vulnerabilitiesStr != "" {
			vulnerabilities := strings.Split(vulnerabilitiesStr, ",")
			for _, vuln := range vulnerabilities {
				if vuln == damageType {
					// Vulnerability doubles damage
					baseDamage, _ := e.Context().GetInt("damage_amount")
					e.Context().AddModifier(events.NewModifier(
						"vulnerability",
						events.ModifierDamageBonus,
						events.NewRawValue(baseDamage, fmt.Sprintf("vulnerable to %s", damageType)),
						100,
					))
					log.Printf("Target is vulnerable to %s damage", damageType)
					break
				}
			}
		}

		return nil
	})
}

// SetupSaveHandlers sets up saving throw handlers
func (i *CompleteEventIntegration) SetupSaveHandlers() {
	// Saving throw handler
	i.bus.SubscribeFunc(events.EventOnSavingThrow, 100, func(_ context.Context, e events.Event) error {
		if e.Source() == nil {
			return nil
		}

		ability, _ := e.Context().GetString("ability")
		charID := e.Source().GetID()

		// Check for proficiency in this save
		saveProf := fmt.Sprintf("%s-save", ability)
		if i.proficiency.CheckProficiency(charID, saveProf) {
			level := 5 // Example
			bonus := GetProficiencyBonus(level)

			e.Context().AddModifier(events.NewModifier(
				"proficiency",
				events.ModifierSaveBonus,
				events.NewRawValue(bonus, "save proficiency"),
				100,
			))

			log.Printf("Added proficiency bonus to %s save", ability)
		}

		// Check for bless
		if hasBlessing, _ := e.Context().GetBool("has_bless"); hasBlessing {
			e.Context().AddModifier(events.NewModifier(
				"bless",
				events.ModifierSaveBonus,
				events.NewDiceValue(1, 4, "bless"),
				50,
			))
			log.Println("Bless bonus applied to save")
		}

		return nil
	})
}

// SetupConditionHandlers sets up condition-related handlers
func (i *CompleteEventIntegration) SetupConditionHandlers() {
	// When a condition is applied
	i.bus.SubscribeFunc(events.EventOnConditionApplied, 50, func(_ context.Context, e events.Event) error {
		condition, _ := e.Context().GetString("condition")
		target := e.Target()

		if target == nil {
			return nil
		}

		log.Printf("Condition '%s' applied to %s", condition, target.GetID())

		// Handle specific conditions
		switch condition {
		case "paralyzed":
			// Auto-fail STR and DEX saves
			e.Context().Set("auto_fail_str_saves", true)
			e.Context().Set("auto_fail_dex_saves", true)
			// Attacks against have advantage
			e.Context().Set("grant_advantage_against", true)

		case "poisoned":
			// Disadvantage on attacks and ability checks
			e.Context().Set("attack_disadvantage", true)
			e.Context().Set("ability_check_disadvantage", true)

		case "restrained":
			// Disadvantage on DEX saves
			e.Context().Set("dex_save_disadvantage", true)
			// Attacks have disadvantage
			e.Context().Set("attack_disadvantage", true)
			// Attacks against have advantage
			e.Context().Set("grant_advantage_against", true)
		}

		return nil
	})
}

// MigrateOldHandler shows how to migrate an old handler
func (i *CompleteEventIntegration) MigrateOldHandler() {
	// Old DND bot handler (still works through adapter)
	i.adapter.Subscribe("OnAttackRoll", 90, func(data interface{}) error {
		// Old style handler
		attackData, ok := data.(map[string]interface{})
		if !ok {
			return nil
		}

		// Old logic still works
		if weapon, ok := attackData["weapon"].(string); ok {
			log.Printf("Old handler: Attack with %s", weapon)
		}

		return nil
	})
}

// ExampleCombatFlow demonstrates a complete combat flow
func (i *CompleteEventIntegration) ExampleCombatFlow() {
	ctx := context.Background()

	// Create combatants
	fighter := WrapCharacter("fighter-123", "Fighter", 5)
	goblin := WrapCharacter("goblin-456", "Goblin", 1)

	// Add fighter proficiencies
	_ = i.proficiency.AddCharacterProficiencies(fighter.GetID(), 5)

	// 1. Attack Roll
	fmt.Println("\n--- Attack Roll ---")
	attackEvent := events.NewGameEvent(events.EventOnAttackRoll, fighter, goblin)
	attackEvent.Context().Set("weapon", "longsword")
	attackEvent.Context().Set("has_advantage", true)

	err := i.bus.Publish(ctx, attackEvent)
	if err != nil {
		log.Printf("Error publishing attack: %v", err)
	}

	// Show modifiers
	fmt.Println("Attack modifiers:")
	for _, mod := range attackEvent.Context().Modifiers() {
		fmt.Printf("  %s: +%d (%s)\n",
			mod.Source(),
			mod.ModifierValue().GetValue(),
			mod.ModifierValue().GetDescription())
	}

	// 2. Damage Roll (assuming hit)
	fmt.Println("\n--- Damage Roll ---")
	damageEvent := events.NewGameEvent(events.EventOnDamageRoll, fighter, goblin)
	damageEvent.Context().Set("weapon", "longsword")
	damageEvent.Context().Set("damage_type", "slashing")
	damageEvent.Context().Set("base_damage", 8) // 1d8
	damageEvent.Context().Set("is_critical", false)

	err = i.bus.Publish(ctx, damageEvent)
	if err != nil {
		log.Printf("Error publishing damage: %v", err)
	}

	// 3. Taking Damage
	fmt.Println("\n--- Taking Damage ---")
	takeDamageEvent := events.NewGameEvent(events.EventBeforeTakeDamage, fighter, goblin)
	takeDamageEvent.Context().Set("damage_type", "slashing")
	takeDamageEvent.Context().Set("damage_amount", 10)
	takeDamageEvent.Context().Set("resistances", "bludgeoning,piercing")

	err = i.bus.Publish(ctx, takeDamageEvent)
	if err != nil {
		log.Printf("Error publishing take damage: %v", err)
	}

	// Show damage modifiers
	fmt.Println("Damage modifiers:")
	for _, mod := range takeDamageEvent.Context().Modifiers() {
		fmt.Printf("  %s: %s\n",
			mod.Source(),
			mod.ModifierValue().GetDescription())
	}

	// 4. Saving Throw
	fmt.Println("\n--- Saving Throw ---")
	saveEvent := events.NewGameEvent(events.EventOnSavingThrow, fighter, nil)
	saveEvent.Context().Set("ability", "strength")
	saveEvent.Context().Set("dc", 15)
	saveEvent.Context().Set("has_bless", true)

	err = i.bus.Publish(ctx, saveEvent)
	if err != nil {
		log.Printf("Error publishing save: %v", err)
	}

	// Show save modifiers
	fmt.Println("Save modifiers:")
	for _, mod := range saveEvent.Context().Modifiers() {
		fmt.Printf("  %s: %s\n",
			mod.Source(),
			mod.ModifierValue().GetDescription())
	}
}

// ExampleCompleteIntegration demonstrates a complete DND bot integration
func ExampleCompleteIntegration() {
	fmt.Println("=== DND Bot Event Bus Integration Example ===")

	// Create integration
	integration := NewCompleteEventIntegration()

	// Setup all handlers
	integration.SetupCombatHandlers()
	integration.SetupSaveHandlers()
	integration.SetupConditionHandlers()

	// Show old handler still works
	integration.MigrateOldHandler()

	// Run example combat flow
	integration.ExampleCombatFlow()
}
