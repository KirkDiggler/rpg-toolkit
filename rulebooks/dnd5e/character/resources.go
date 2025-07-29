// Package character provides D&D 5e character creation and management functionality
package character

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// initializeClassResources processes class resources based on level and ability scores
func initializeClassResources(classData *class.Data, level int,
	abilityScores shared.AbilityScores) map[string]Resource {
	resources := make(map[string]Resource)

	for _, resourceData := range classData.Resources {
		maxValue := calculateResourceMax(resourceData, level, abilityScores)
		if maxValue > 0 {
			resources[resourceData.ID] = Resource{
				Name:    resourceData.Name,
				Max:     maxValue,
				Current: maxValue, // Start at full
				Resets:  shared.ResetType(resourceData.ResetOn),
			}
		}
	}

	return resources
}

// calculateResourceMax evaluates the resource formula or uses the uses-per-level table
func calculateResourceMax(resourceData class.ResourceData, level int, abilityScores shared.AbilityScores) int {
	// First check if we have a uses-per-level table
	if resourceData.UsesPerLevel != nil {
		if uses, ok := resourceData.UsesPerLevel[level]; ok {
			return uses
		}
	}

	// Otherwise, evaluate the formula
	if resourceData.MaxFormula != "" {
		return evaluateResourceFormula(resourceData.MaxFormula, level, abilityScores)
	}

	return 0
}

// evaluateResourceFormula parses and evaluates simple formulas like "level", "1 + charisma_modifier"
// Supported patterns:
//   - "level" - character level
//   - "charisma_modifier" or "cha_modifier" - ability modifiers
//   - "5" - constants
//   - "1 + charisma_modifier" - basic arithmetic
//   - "min(5, level)" or "max(1, wisdom_modifier)" - min/max functions
//
// NOT supported:
//   - "10 + -3" - consecutive operators
//   - Complex parentheses beyond min/max
//   - Variables other than level and ability modifiers
func evaluateResourceFormula(formula string, level int, abilityScores shared.AbilityScores) int {
	// Simple formula parser - handles basic cases
	formula = strings.TrimSpace(strings.ToLower(formula))

	// Direct level reference
	if formula == "level" {
		return level
	}

	// Replace ability modifiers
	formula = replaceAbilityModifiers(formula, abilityScores)

	// Replace level references
	formula = strings.ReplaceAll(formula, "level", strconv.Itoa(level))

	// Evaluate the formula
	result, err := evaluateSimpleExpression(formula)
	if err != nil {
		// If we can't evaluate, return 0
		return 0
	}

	return result
}

// replaceAbilityModifiers replaces ability modifier references in a formula
func replaceAbilityModifiers(formula string, abilityScores shared.AbilityScores) string {
	// Replace each ability modifier reference
	modifiers := map[string]int{
		"strength_modifier":     abilityScores.Modifier(constants.STR),
		"dexterity_modifier":    abilityScores.Modifier(constants.DEX),
		"constitution_modifier": abilityScores.Modifier(constants.CON),
		"intelligence_modifier": abilityScores.Modifier(constants.INT),
		"wisdom_modifier":       abilityScores.Modifier(constants.WIS),
		"charisma_modifier":     abilityScores.Modifier(constants.CHA),
		// Short forms
		"str_modifier": abilityScores.Modifier(constants.STR),
		"dex_modifier": abilityScores.Modifier(constants.DEX),
		"con_modifier": abilityScores.Modifier(constants.CON),
		"int_modifier": abilityScores.Modifier(constants.INT),
		"wis_modifier": abilityScores.Modifier(constants.WIS),
		"cha_modifier": abilityScores.Modifier(constants.CHA),
	}

	for name, value := range modifiers {
		formula = strings.ReplaceAll(formula, name, strconv.Itoa(value))
	}

	return formula
}

// evaluateSimpleExpression evaluates a simple mathematical expression
// Supports: +, -, *, /, min, max, and parentheses
func evaluateSimpleExpression(expr string) (int, error) {
	// Remove spaces
	expr = strings.ReplaceAll(expr, " ", "")

	// Validate expression doesn't have patterns we can't handle
	// Check for operators followed by operators (like "+-" or "*-")
	for i := 0; i < len(expr)-1; i++ {
		curr := expr[i]
		next := expr[i+1]
		if (curr == '+' || curr == '-' || curr == '*' || curr == '/') &&
			(next == '+' || next == '-' || next == '*' || next == '/') {
			return 0, fmt.Errorf("unsupported expression: consecutive operators at position %d", i)
		}
	}

	// Handle min/max functions
	if strings.HasPrefix(expr, "min(") || strings.HasPrefix(expr, "max(") {
		return evaluateMinMax(expr)
	}

	// For now, we'll implement a simple parser for basic arithmetic
	// This handles expressions like "1+2", "3*4", "5-1", etc.

	// Split on + and - (lowest precedence)
	if idx := strings.LastIndexAny(expr, "+-"); idx > 0 {
		left, err := evaluateSimpleExpression(expr[:idx])
		if err != nil {
			return 0, err
		}
		right, err := evaluateSimpleExpression(expr[idx+1:])
		if err != nil {
			return 0, err
		}
		if expr[idx] == '+' {
			return left + right, nil
		}
		return left - right, nil
	}

	// Split on * and / (higher precedence)
	if idx := strings.LastIndexAny(expr, "*/"); idx > 0 {
		left, err := evaluateSimpleExpression(expr[:idx])
		if err != nil {
			return 0, err
		}
		right, err := evaluateSimpleExpression(expr[idx+1:])
		if err != nil {
			return 0, err
		}
		if expr[idx] == '*' {
			return left * right, nil
		}
		if right == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return left / right, nil
	}

	// Try to parse as number
	if val, err := strconv.Atoi(expr); err == nil {
		return val, nil
	}

	// Handle negative numbers
	if strings.HasPrefix(expr, "-") && len(expr) > 1 {
		val, err := strconv.Atoi(expr[1:])
		if err == nil {
			return -val, nil
		}
	}

	return 0, fmt.Errorf("unable to parse expression: %s", expr)
}

// evaluateMinMax evaluates min() or max() functions
func evaluateMinMax(expr string) (int, error) {
	isMax := strings.HasPrefix(expr, "max(")

	// Extract the arguments
	start := 4 // len("min(") or len("max(")
	end := strings.LastIndex(expr, ")")
	if end == -1 {
		return 0, fmt.Errorf("unclosed function call")
	}

	args := strings.Split(expr[start:end], ",")
	if len(args) < 2 {
		return 0, fmt.Errorf("min/max requires at least 2 arguments")
	}

	// Evaluate first argument
	result, err := evaluateSimpleExpression(args[0])
	if err != nil {
		return 0, err
	}

	// Compare with remaining arguments
	for i := 1; i < len(args); i++ {
		val, err := evaluateSimpleExpression(args[i])
		if err != nil {
			return 0, err
		}
		if isMax && val > result {
			result = val
		} else if !isMax && val < result {
			result = val
		}
	}

	return result, nil
}

// initializeSpellSlots sets up spell slots based on class and level
func initializeSpellSlots(classData *class.Data, level int) SpellSlots {
	slots := make(SpellSlots)

	if classData.Spellcasting == nil || classData.Spellcasting.SpellSlots == nil {
		return slots
	}

	// Get spell slots for the character's level
	if slotsForLevel, ok := classData.Spellcasting.SpellSlots[level]; ok {
		// The array is 0-indexed but represents spell levels 1-9
		for i, numSlots := range slotsForLevel {
			spellLevel := i + 1 // Convert 0-indexed to 1-indexed spell level
			if numSlots > 0 {
				slots[spellLevel] = SlotInfo{
					Max:  numSlots,
					Used: 0,
				}
			}
		}
	}

	return slots
}
