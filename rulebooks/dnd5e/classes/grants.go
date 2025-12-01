package classes

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// EquipmentItem represents an item with quantity
type EquipmentItem struct {
	ID       shared.EquipmentID `json:"id"`       // Equipment ID
	Quantity int                `json:"quantity"` // How many (default 1)
}

// GetHitDice is a convenience function to quickly get a class's hit die.
// HitDice is an intrinsic class property ("what you are"), not a grant ("what you get").
// Returns 0 if the classID is invalid.
func GetHitDice(classID Class) int {
	data := GetData(classID)
	if data == nil {
		return 0
	}
	return data.HitDice
}

// GetSavingThrows is a convenience function to get saving throw proficiencies.
// SavingThrows are intrinsic class properties ("what you are"), not grants ("what you get").
// Returns nil if the classID is invalid.
func GetSavingThrows(classID Class) []abilities.Ability {
	data := GetData(classID)
	if data == nil {
		return nil
	}
	return data.SavingThrows
}
