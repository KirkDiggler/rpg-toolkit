// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package examples

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
)

// SpellExamples shows how to implement various D&D spells using conditions
type SpellExamples struct {
	conditionManager *conditions.ConditionManager
	bus              events.EventBus
}

// NewSpellExamples creates spell examples
func NewSpellExamples() *SpellExamples {
	bus := events.NewBus()
	return &SpellExamples{
		conditionManager: conditions.NewConditionManager(bus),
		bus:              bus,
	}
}

// Blindness/Deafness spell
func (se *SpellExamples) CastBlindnessDeafness(
	caster core.Entity,
	target core.Entity,
	choice string, // "blind" or "deaf"
	spellDC int,
) error {
	var condition *conditions.EnhancedCondition
	var err error

	switch choice {
	case "blind":
		condition, err = conditions.Blinded().
			WithTarget(target).
			WithSource(fmt.Sprintf("blindness_deafness_%s", caster.GetID())).
			WithSaveDC(spellDC).
			WithMinutesDuration(1).
			Build()
	case "deaf":
		condition, err = conditions.Deafened().
			WithTarget(target).
			WithSource(fmt.Sprintf("blindness_deafness_%s", caster.GetID())).
			WithSaveDC(spellDC).
			WithMinutesDuration(1).
			Build()
	default:
		return fmt.Errorf("invalid choice: must be 'blind' or 'deaf'")
	}

	if err != nil {
		return err
	}

	return se.conditionManager.ApplyCondition(condition)
}

// Ray of Sickness
func (se *SpellExamples) CastRayOfSickness(
	caster core.Entity,
	target core.Entity,
	spellDC int,
) error {
	// Ray of Sickness poisons until end of next turn
	poisoned, err := conditions.Poisoned().
		WithTarget(target).
		WithSource(fmt.Sprintf("ray_of_sickness_%s", caster.GetID())).
		WithSaveDC(spellDC).
		WithRoundsDuration(2). // Until end of caster's next turn
		Build()

	if err != nil {
		return err
	}

	return se.conditionManager.ApplyCondition(poisoned)
}

// Charm Person
func (se *SpellExamples) CastCharmPerson(
	caster core.Entity,
	target core.Entity,
	spellDC int,
	upcast bool, // True if cast at higher level
) error {
	duration := 60 // 1 hour base
	if upcast {
		duration = 480 // 8 hours at 3rd level or higher
	}

	charmed, err := conditions.Charmed().
		WithTarget(target).
		WithSource(fmt.Sprintf("charm_person_%s", caster.GetID())).
		WithCharmer(caster).
		WithSaveDC(spellDC).
		WithMinutesDuration(duration).
		WithConcentration().
		Build()

	if err != nil {
		return err
	}

	return se.conditionManager.ApplyCondition(charmed)
}

// Cause Fear
func (se *SpellExamples) CastCauseFear(
	caster core.Entity,
	target core.Entity,
	spellDC int,
) error {
	frightened, err := conditions.Frightened().
		WithTarget(target).
		WithSource(fmt.Sprintf("cause_fear_%s", caster.GetID())).
		WithFearSource(caster).
		WithSaveDC(spellDC).
		WithMinutesDuration(1).
		WithConcentration().
		Build()

	if err != nil {
		return err
	}

	return se.conditionManager.ApplyCondition(frightened)
}

// Sleep spell (special case - no save)
func (se *SpellExamples) CastSleep(
	caster core.Entity,
	targets []core.Entity,
	totalHP int, // Total HP affected by sleep
) error {
	remainingHP := totalHP

	// Sort targets by current HP (lowest first)
	// In real implementation, you'd sort properly

	for _, target := range targets {
		// Check immunity to being charmed (sleep doesn't work)
		if se.conditionManager.IsImmune(target, conditions.ConditionCharmed) {
			continue
		}

		// Get target's current HP somehow
		targetHP := 20 // Placeholder

		if targetHP <= remainingHP {
			// Target falls asleep
			unconscious, err := conditions.Unconscious().
				WithTarget(target).
				WithSource(fmt.Sprintf("sleep_%s", caster.GetID())).
				WithMinutesDuration(1).
				Build()

			if err != nil {
				return err
			}

			if err := se.conditionManager.ApplyCondition(unconscious); err != nil {
				return err
			}

			remainingHP -= targetHP
		}

		if remainingHP <= 0 {
			break
		}
	}

	return nil
}

// Web spell (creates difficult terrain and restrains)
func (se *SpellExamples) CastWeb(
	caster core.Entity,
	targets []core.Entity,
	spellDC int,
) error {
	for _, target := range targets {
		restrained, err := conditions.Restrained().
			WithTarget(target).
			WithSource(fmt.Sprintf("web_%s", caster.GetID())).
			WithSaveDC(spellDC).
			WithMinutesDuration(60). // 1 hour
			WithConcentration().
			WithMetadata("escape_dc", spellDC). // Can use action to escape
			Build()

		if err != nil {
			return err
		}

		// Apply restrained to those who fail save
		// In real implementation, you'd check saves first
		if err := se.conditionManager.ApplyCondition(restrained); err != nil {
			continue // Some might be immune
		}
	}

	return nil
}

// Contagion (applies poisoned, then disease)
func (se *SpellExamples) CastContagion(
	caster core.Entity,
	target core.Entity,
	disease string,
	spellDC int,
) error {
	// First apply poisoned condition
	poisoned, err := conditions.Poisoned().
		WithTarget(target).
		WithSource(fmt.Sprintf("contagion_%s", caster.GetID())).
		WithMetadata("disease_pending", disease).
		WithMetadata("saves_made", 0).
		WithMetadata("saves_failed", 0).
		Build()

	if err != nil {
		return err
	}

	// Apply custom save handler for the complex Contagion mechanics
	// After 3 failed saves, apply the disease
	// After 3 successful saves, remove poisoned

	return se.conditionManager.ApplyCondition(poisoned)
}

// Flesh to Stone (progressive petrification)
func (se *SpellExamples) CastFleshToStone(
	caster core.Entity,
	target core.Entity,
	spellDC int,
) error {
	// Start with restrained
	restrained, err := conditions.Restrained().
		WithTarget(target).
		WithSource(fmt.Sprintf("flesh_to_stone_%s", caster.GetID())).
		WithSaveDC(spellDC).
		WithMinutesDuration(1).
		WithConcentration().
		WithMetadata("petrification_saves_failed", 0).
		WithMetadata("petrification_saves_succeeded", 0).
		Build()

	if err != nil {
		return err
	}

	// Would need custom logic to track saves and eventually apply petrified
	return se.conditionManager.ApplyCondition(restrained)
}

// Sickening Radiance (exhaustion + radiant damage)
func (se *SpellExamples) CastSickeningRadiance(
	caster core.Entity,
	targetsInArea []core.Entity,
	spellDC int,
) error {
	for _, target := range targetsInArea {
		// Check save
		// If failed, apply 1 level of exhaustion
		exhaustion, err := conditions.NewExhaustionCondition(
			target,
			1,
			fmt.Sprintf("sickening_radiance_%s", caster.GetID()),
		)

		if err != nil {
			return err
		}

		// Also deal radiant damage (not shown here)

		if err := se.conditionManager.ApplyCondition(exhaustion); err != nil {
			continue // Might be immune
		}
	}

	return nil
}

// Example of checking conditions in spell casting
func (se *SpellExamples) CanCastSpell(caster core.Entity) (bool, string) {
	activeConditions := se.conditionManager.GetConditions(caster)

	for _, cond := range activeConditions {
		if enhanced, ok := cond.(*conditions.EnhancedCondition); ok {
			switch enhanced.GetConditionType() {
			case conditions.ConditionParalyzed:
				return false, "Cannot cast spells while paralyzed"
			case conditions.ConditionStunned:
				return false, "Cannot cast spells while stunned"
			case conditions.ConditionUnconscious:
				return false, "Cannot cast spells while unconscious"
			case conditions.ConditionPetrified:
				return false, "Cannot cast spells while petrified"
			}
		}

		// Check for incapacitated (can't take actions)
		if cond.GetType() == string(conditions.ConditionIncapacitated) {
			return false, "Cannot cast spells while incapacitated"
		}
	}

	// Check if can speak for verbal components
	// This would need more complex checking

	return true, ""
}
