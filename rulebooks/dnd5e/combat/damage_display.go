package combat

import (
	"fmt"
	"strconv"
	"strings"

	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// FormatDamageComponent formats a resolved damage component for player-facing display.
func FormatDamageComponent(component dnd5eEvents.DamageComponent) string {
	rolls := make([]string, len(component.FinalDiceRolls))
	for i, roll := range component.FinalDiceRolls {
		rolls[i] = strconv.Itoa(roll)
	}

	terms := ""
	if component.DiceNotation != "" {
		terms = fmt.Sprintf("%s (%s)", component.DiceNotation, strings.Join(rolls, " + "))
	}
	if component.FlatBonus != 0 {
		bonus := fmt.Sprintf("+ %d", component.FlatBonus)
		if component.FlatBonus < 0 {
			bonus = fmt.Sprintf("- %d", -component.FlatBonus)
		}
		if terms != "" {
			terms += " "
		}
		terms += bonus
	}

	return fmt.Sprintf("%s %s = %d", terms, component.DamageType, component.Total())
}

// Display formats the resolved damage breakdown for player-facing display.
func (breakdown *DamageBreakdown) Display() string {
	components := make([]string, len(breakdown.Components))
	for i, component := range breakdown.Components {
		components[i] = FormatDamageComponent(component)
	}

	return fmt.Sprintf("%s. Total: %d damage.", strings.Join(components, "; "), breakdown.TotalDamage)
}
