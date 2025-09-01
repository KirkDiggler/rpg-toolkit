package equipment

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/packs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/tools"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// GetByID retrieves equipment by its ID from any equipment category
func GetByID(id string) (Equipment, error) {
	// Check weapons
	if weapon, ok := weapons.All[weapons.WeaponID(id)]; ok {
		return &weapon, nil
	}
	
	// Check armor
	if armorItem, ok := armor.All[armor.ArmorID(id)]; ok {
		return &armorItem, nil
	}
	
	// Check tools
	if tool, ok := tools.All[tools.ToolID(id)]; ok {
		return &tool, nil
	}
	
	// Check packs
	if pack, ok := packs.All[packs.PackID(id)]; ok {
		return &pack, nil
	}
	
	// TODO: Check general gear when available (torches, rope, etc.)
	
	return nil, rpgerr.New(rpgerr.CodeNotFound, "equipment not found",
		rpgerr.WithMeta("id", id))
}