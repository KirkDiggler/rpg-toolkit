package validation

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// ValidateClassChoicesWrapper wraps the old API but uses the new V2 implementation
// This allows gradual migration without breaking existing code
func ValidateClassChoicesWrapper(classID classes.Class, choicesData []character.ChoiceData) ([]Error, error) {
	// Convert old ChoiceData to new Choice types
	var choices []character.Choice
	for _, old := range choicesData {
		choice, err := character.ConvertFromChoiceData(old)
		if err != nil {
			// If conversion fails, fall back to old validator
			return ValidateClassChoices(classID, choicesData)
		}
		choices = append(choices, choice)
	}
	
	// Use new V2 validation
	return ValidateClassChoicesV2(classID, choices)
}