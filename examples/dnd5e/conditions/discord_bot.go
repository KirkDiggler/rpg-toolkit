// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package examples demonstrates how the Discord bot can use enhanced conditions.
package examples

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
	conditions "github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
)

// Character represents a Discord bot character
type Character struct {
	ID   string
	Name string
	HP   int
	// Other character fields...
}

func (c *Character) GetID() string   { return c.ID }
func (c *Character) GetType() string { return "character" }

// CombatSystem demonstrates condition usage in combat
type CombatSystem struct {
	bus              events.EventBus
	conditionManager *conditions.ConditionManager
	exhaustionMgr    *conditions.ExhaustionManager
}

// NewCombatSystem creates a new combat system
func NewCombatSystem() *CombatSystem {
	bus := events.NewBus()
	cm := conditions.NewConditionManager(bus)

	return &CombatSystem{
		bus:              bus,
		conditionManager: cm,
		exhaustionMgr:    conditions.NewExhaustionManager(cm),
	}
}

// CastHoldPerson applies hold person spell to a target.
func (cs *CombatSystem) CastHoldPerson(caster, target *Character, spellDC int) error {
	// Hold Person applies paralyzed condition
	paralyzed, err := conditions.Paralyzed().
		WithTarget(target).
		WithSource(fmt.Sprintf("hold_person_%s", caster.ID)).
		WithSaveDC(spellDC).
		WithMinutesDuration(1). // Concentration up to 1 minute
		WithConcentration().
		Build()

	if err != nil {
		return err
	}

	// Apply the condition
	if err := cs.conditionManager.ApplyCondition(paralyzed); err != nil {
		return fmt.Errorf("failed to paralyze target: %w", err)
	}

	// In Discord bot, you'd send a message
	fmt.Printf("%s is paralyzed by Hold Person! (DC %d Wisdom save at end of turn)\n",
		target.Name, spellDC)

	return nil
}

// CastViciousMockery applies vicious mockery with disadvantage on next attack.
func (cs *CombatSystem) CastViciousMockery(caster, target *Character) error {
	// Custom condition that gives disadvantage on next attack
	mockeryDebuff := conditions.NewSimpleCondition(conditions.SimpleConditionConfig{
		ID:     fmt.Sprintf("vicious_mockery_%s", target.ID),
		Type:   "vicious_mockery_debuff",
		Target: target,
		Source: fmt.Sprintf("vicious_mockery_%s", caster.ID),
		ApplyFunc: func(c *conditions.SimpleCondition, bus events.EventBus) error {
			// Subscribe to next attack roll
			handler := func(_ context.Context, event events.Event) error {
				if event.Source() != target {
					return nil
				}

				// Apply disadvantage
				event.Context().AddModifier(events.NewModifier(
					"vicious_mockery",
					events.ModifierDisadvantage,
					events.IntValue(1),
					100,
				))

				// Remove this condition after applying
				go func() { _ = cs.conditionManager.RemoveCondition(c) }()

				return nil
			}

			c.Subscribe(bus, events.EventOnAttackRoll, 50, handler)
			return nil
		},
	})

	// Apply the debuff
	if err := cs.conditionManager.ApplyCondition(mockeryDebuff); err != nil {
		return err
	}

	fmt.Printf("%s has disadvantage on their next attack from Vicious Mockery!\n", target.Name)
	return nil
}

// ApplyExtremeHeat applies exhaustion from extreme heat.
func (cs *CombatSystem) ApplyExtremeHeat(characters []*Character) error {
	for _, char := range characters {
		// Check if they have fire resistance or immunity
		if cs.conditionManager.IsImmune(char, conditions.ConditionExhaustion) {
			fmt.Printf("%s is immune to exhaustion from extreme heat\n", char.Name)
			continue
		}

		// Apply 1 level of exhaustion
		if err := cs.exhaustionMgr.AddExhaustion(char, 1, "extreme_heat"); err != nil {
			return err
		}

		level := cs.exhaustionMgr.GetExhaustionLevel(char)
		fmt.Printf("%s gains 1 level of exhaustion from extreme heat (now level %d)\n",
			char.Name, level)

		// Check for death
		if cs.exhaustionMgr.CheckExhaustionDeath(char) {
			fmt.Printf("%s has died from exhaustion!\n", char.Name)
		}
	}

	return nil
}

// RollAttack handles attack rolls with condition effects.
func (cs *CombatSystem) RollAttack(attacker, target *Character) (bool, error) {
	// Create attack event
	attackEvent := events.NewGameEvent(
		events.EventOnAttackRoll,
		attacker,
		target,
	)
	attackEvent.Context().Set("weapon", "longsword")
	attackEvent.Context().Set("base_bonus", 5)

	// Publish event - conditions will automatically apply their effects
	if err := cs.bus.Publish(context.Background(), attackEvent); err != nil {
		return false, err
	}

	// Check modifiers applied by conditions
	hasAdvantage := false
	hasDisadvantage := false

	for _, mod := range attackEvent.Context().Modifiers() {
		switch mod.Type() {
		case events.ModifierAdvantage:
			hasAdvantage = true
			fmt.Printf("  Advantage from: %s\n", mod.Source())
		case events.ModifierDisadvantage:
			hasDisadvantage = true
			fmt.Printf("  Disadvantage from: %s\n", mod.Source())
		}
	}

	// Roll dice based on advantage/disadvantage
	if hasAdvantage && !hasDisadvantage {
		fmt.Printf("%s attacks with advantage!\n", attacker.Name)
	} else if hasDisadvantage && !hasAdvantage {
		fmt.Printf("%s attacks with disadvantage!\n", attacker.Name)
	}

	// Actual dice rolling would happen here
	return true, nil
}

// GetConditionDisplay returns a formatted string for Discord embeds.
func (cs *CombatSystem) GetConditionDisplay(character *Character) string {
	conditions := cs.conditionManager.GetConditions(character)
	if len(conditions) == 0 {
		return "No active conditions"
	}

	display := "**Active Conditions:**\n"
	for _, cond := range conditions {
		// Get condition type and icon
		icon := "❓"
		condTypeStr := cond.GetType()
		icon = getConditionIcon(condTypeStr)

		display += fmt.Sprintf("%s %s", icon, condTypeStr)

		// Add source if relevant
		if cond.Source() != "" && cond.Source() != "unknown" {
			display += fmt.Sprintf(" (from %s)", cond.Source())
		}

		display += "\n"
	}

	// Add exhaustion level if present
	exhaustionLevel := cs.exhaustionMgr.GetExhaustionLevel(character)
	if exhaustionLevel > 0 {
		display += fmt.Sprintf("💀 Exhaustion Level %d\n", exhaustionLevel)
	}

	return display
}

// Helper function for condition icons
func getConditionIcon(condType string) string {
	icons := map[string]string{
		"blinded":       "👁️",
		"charmed":       "💕",
		"deafened":      "🔇",
		"frightened":    "😱",
		"grappled":      "🤝",
		"incapacitated": "💫",
		"invisible":     "👻",
		"paralyzed":     "🚫",
		"petrified":     "🗿",
		"poisoned":      "🤢",
		"prone":         "🔽",
		"restrained":    "⛓️",
		"stunned":       "💥",
		"unconscious":   "😵",
		"exhaustion":    "💀",
	}

	if icon, exists := icons[condType]; exists {
		return icon
	}
	return "❓"
}

// MakeSavingThrow makes a saving throw to end conditions.
func (cs *CombatSystem) MakeSavingThrow(character *Character, _ string, bonus int) error {
	// Get conditions that might end on a save
	allConditions := cs.conditionManager.GetConditions(character)

	for _, cond := range allConditions {
		enhanced, ok := cond.(interface {
			GetSaveDC() int
		})
		if !ok || enhanced.GetSaveDC() == 0 {
			continue // No save DC means it doesn't end on a save
		}

		// Roll d20 + bonus
		roll := 10 + bonus // Placeholder - actual dice rolling would go here

		if roll >= enhanced.GetSaveDC() {
			fmt.Printf("%s succeeds on save (rolled %d vs DC %d) and is no longer %s!\n",
				character.Name, roll, enhanced.GetSaveDC(), cond.GetType())

			// Remove the condition
			if err := cs.conditionManager.RemoveCondition(cond); err != nil {
				return err
			}
		} else {
			fmt.Printf("%s fails save (rolled %d vs DC %d) and remains %s\n",
				character.Name, roll, enhanced.GetSaveDC(), cond.GetType())
		}
	}

	return nil
}

// ApplyPaladinAura grants immunity to poisoned and frightened.
func (cs *CombatSystem) ApplyPaladinAura(paladin *Character, allies []*Character) {
	// Paladins grant immunity to disease (poisoned) and fear (frightened)
	for _, ally := range allies {
		cs.conditionManager.AddImmunity(ally, conditions.ConditionPoisoned)
		cs.conditionManager.AddImmunity(ally, conditions.ConditionFrightened)

		fmt.Printf("%s gains immunity to poisoned and frightened from %s's aura\n",
			ally.Name, paladin.Name)
	}
}

// BreakConcentration removes concentration effects.
func (cs *CombatSystem) BreakConcentration(caster *Character) error {
	// Find all conditions this caster is concentrating on
	// This would use the RelationshipManager to find concentration relationships

	fmt.Printf("%s loses concentration!\n", caster.Name)

	// Remove all concentration effects
	// In a real implementation, you'd track which conditions require concentration
	// and remove them when concentration breaks

	return nil
}
