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

// GetHitDice returns the hit die size for a class.
// This is intrinsic class data, not a grant.
// Returns 0 if the classID is invalid.
func GetHitDice(classID Class) int {
	data := GetData(classID)
	if data == nil {
		return 0
	}
	return data.HitDice
}

// GetSavingThrows returns the saving throw proficiencies for a class.
// This is intrinsic class data, not a grant.
// Returns nil if the classID is invalid.
func GetSavingThrows(classID Class) []abilities.Ability {
	data := GetData(classID)
	if data == nil {
		return nil
	}
	return data.SavingThrows
}
