package combat

import (
	"fmt"
	"strconv"
	"strings"

	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// FormatDamageComponent formats a resolved damage component for player-facing display.
func FormatDamageComponent(component dnd5eEvents.DamageComponent) string {
	if len(component.Terms) > 0 {
		return formatSignedDamageComponent(component)
	}

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

func formatSignedDamageComponent(component dnd5eEvents.DamageComponent) string {
	terms := ""
	for i, term := range component.Terms {
		rolls := make([]string, len(term.Final))
		for j, roll := range term.Final {
			rolls[j] = strconv.Itoa(roll)
		}
		formatted := fmt.Sprintf("%s (%s)", term.Dice, strings.Join(rolls, " + "))
		if i == 0 && term.Sign > 0 {
			terms = formatted
			continue
		}
		if terms != "" {
			terms += " "
		}
		if term.Sign < 0 {
			terms += "- "
		} else {
			terms += "+ "
		}
		terms += formatted
	}
	if component.FlatBonus != 0 {
		if terms != "" {
			terms += " "
		}
		if component.FlatBonus < 0 {
			terms += fmt.Sprintf("- %d", -component.FlatBonus)
		} else {
			terms += fmt.Sprintf("+ %d", component.FlatBonus)
		}
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
