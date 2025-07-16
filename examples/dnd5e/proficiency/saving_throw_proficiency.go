package proficiency

import (
	"context"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency"
)

// SavingThrowProficiency implements saving throw proficiency for D&D 5e
type SavingThrowProficiency struct {
	*proficiency.SimpleProficiency
	savingThrow SavingThrow
	level       int
}

// NewSavingThrowProficiency creates a saving throw proficiency
func NewSavingThrowProficiency(owner core.Entity, savingThrow SavingThrow, source string, level int) *SavingThrowProficiency {
	stp := &SavingThrowProficiency{
		savingThrow: savingThrow,
		level:       level,
	}

	// Create the underlying simple proficiency with custom handlers
	stp.SimpleProficiency = proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
		ID:      fmt.Sprintf("%s-save-prof-%s", owner.GetID(), savingThrow),
		Type:    "proficiency.saving-throw",
		Owner:   owner,
		Subject: string(savingThrow),
		Source:  source,
		ApplyFunc: func(p *proficiency.SimpleProficiency, bus events.EventBus) error {
			// Subscribe to saving throw events
			p.Subscribe(bus, events.EventOnSavingThrow, 100, stp.handleSavingThrow)
			return nil
		},
	})

	return stp
}

// handleSavingThrow adds proficiency bonus to saving throws
func (stp *SavingThrowProficiency) handleSavingThrow(ctx context.Context, e events.Event) error {
	// Only apply to our owner
	if e.Source() == nil || e.Source().GetID() != stp.Owner().GetID() {
		return nil
	}

	// Get the ability being used for the save
	ability, ok := e.Context().GetString("ability")
	if !ok {
		return nil
	}

	// Check if this matches our saving throw type
	if !strings.EqualFold(ability, string(stp.savingThrow)) {
		return nil
	}

	// Add proficiency bonus
	profBonus := GetProficiencyBonus(stp.level)
	e.Context().AddModifier(events.NewModifier(
		"save-proficiency",
		events.ModifierSaveBonus,
		events.NewRawValue(profBonus, fmt.Sprintf("%s save proficiency", stp.savingThrow)),
		50, // Apply after ability modifier
	))

	return nil
}
