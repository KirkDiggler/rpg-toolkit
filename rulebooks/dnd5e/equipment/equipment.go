// Package equipment provides a unified interface for D&D 5e equipment items
package equipment

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/packs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/tools"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// Equipment represents any item that can be owned, carried, or equipped
type Equipment interface {
	// EquipmentID returns the unique identifier for this equipment
	EquipmentID() string

	// EquipmentType returns the category of equipment
	EquipmentType() shared.EquipmentType

	// EquipmentName returns the display name
	EquipmentName() string

	// EquipmentWeight returns the weight in pounds
	EquipmentWeight() float32

	// EquipmentValue returns the value in copper pieces
	EquipmentValue() int

	// EquipmentDescription returns a description of the item
	EquipmentDescription() string
}

// GetByID returns equipment by its ID
func GetByID(id string) (Equipment, error) {
	if id == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "invalid equipment ID")
	}

	wep, ok := weapons.All[id]
	if ok {
		return &wep, nil
	}

	arm, ok := armor.All[id]
	if ok {
		return &arm, nil
	}

	tool, ok := tools.All[id]
	if ok {
		return &tool, nil
	}

	pack, ok := packs.All[id]
	if ok {
		return &pack, nil
	}

	return nil, rpgerr.New(rpgerr.CodeNotFound, "equipment not found")
}

// GetByCategory returns all equipment matching the specified type and categories
func GetByCategory(equipType shared.EquipmentType, categories []shared.EquipmentCategory) ([]Equipment, error) {
	if len(categories) == 0 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "no categories specified")
	}

	var result []Equipment

	switch equipType {
	case shared.EquipmentTypeWeapon:
		// Get weapons for each category
		for _, cat := range categories {
			weaponList := weapons.GetByCategory(cat)
			for _, w := range weaponList {
				wCopy := w // Create a copy to avoid pointer issues
				result = append(result, &wCopy)
			}
		}

	case shared.EquipmentTypeArmor:
		// Get armor for each category
		for _, cat := range categories {
			armorList := armor.GetByCategory(cat)
			for _, a := range armorList {
				aCopy := a // Create a copy to avoid pointer issues
				result = append(result, &aCopy)
			}
		}

	default:
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "category queries not supported for this equipment type")
	}

	return result, nil
}
