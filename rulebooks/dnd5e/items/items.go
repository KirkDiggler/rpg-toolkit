// Package items provides a unified interface for accessing D&D 5e items
package items

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// Item represents any item in D&D 5e (weapons, armor, etc.)
type Item interface {
	GetName() string
}

// GetByID retrieves an item by its ID from any item category
func GetByID(id string) (Item, error) {
	// Check if the item is a weapon (convert string to WeaponID)
	if weapon, ok := weapons.All[weapons.WeaponID(id)]; ok {
		return weapon, nil
	}

	// Check if it is armor (convert string to ArmorID)
	if armorItem, ok := armor.All[armor.ArmorID(id)]; ok {
		return armorItem, nil
	}

	// Item not found
	return nil, rpgerr.New(rpgerr.CodeNotFound, "item not found",
		rpgerr.WithMeta("item_id", id))
}
