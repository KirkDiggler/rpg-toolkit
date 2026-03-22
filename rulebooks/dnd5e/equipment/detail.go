package equipment

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/ammunition"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/packs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/tools"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// EquipmentDetail contains resolved stats for an equipment item.
// It provides a flat, UI-friendly representation of any equipment type.
type EquipmentDetail struct {
	Name   string               `json:"name"`
	Type   shared.EquipmentType `json:"type"`
	Weight float64              `json:"weight"`
	Cost   string               `json:"cost"`
	Weapon *WeaponDetail        `json:"weapon,omitempty"`
	Armor  *ArmorDetail         `json:"armor,omitempty"`
}

// WeaponDetail contains weapon-specific stats.
type WeaponDetail struct {
	Category   weapons.WeaponCategory   `json:"category"`
	Damage     string                   `json:"damage"`
	DamageType damage.Type              `json:"damage_type"`
	Properties []weapons.WeaponProperty `json:"properties"`
	Range      *weapons.Range           `json:"range,omitempty"`
}

// ArmorDetail contains armor-specific stats.
type ArmorDetail struct {
	Category            armor.ArmorCategory `json:"category"`
	BaseAC              int                 `json:"base_ac"`
	DexBonus            bool                `json:"dex_bonus"`
	MaxDexBonus         *int                `json:"max_dex_bonus,omitempty"`
	StrengthRequirement int                 `json:"strength_requirement,omitempty"`
	StealthDisadvantage bool                `json:"stealth_disadvantage"`
}

// ResolveEquipmentDetail looks up an equipment ID across all registries
// and returns a populated detail struct. Returns nil if not found.
func ResolveEquipmentDetail(id shared.EquipmentID) *EquipmentDetail {
	// Check weapons
	if wep, ok := weapons.All[id]; ok {
		return resolveWeaponDetail(&wep)
	}
	// Check armor
	if arm, ok := armor.All[id]; ok {
		return resolveArmorDetail(&arm)
	}
	// Check tools
	if tool, ok := tools.All[id]; ok {
		return &EquipmentDetail{
			Name:   tool.Name,
			Type:   shared.EquipmentTypeTool,
			Weight: float64(tool.Weight),
			Cost:   tool.Cost,
		}
	}
	// Check packs
	if pack, ok := packs.All[id]; ok {
		return &EquipmentDetail{
			Name:   pack.Name,
			Type:   shared.EquipmentTypePack,
			Weight: float64(pack.Weight),
			Cost:   pack.Cost,
		}
	}
	// Check ammunition
	if ammo, ok := ammunition.StandardAmmunition[id]; ok {
		return &EquipmentDetail{
			Name:   ammo.Name,
			Type:   shared.EquipmentTypeAmmunition,
			Weight: ammo.Weight,
			Cost:   ammo.Cost,
		}
	}
	// Check miscellaneous items
	if item, ok := items.All[id]; ok {
		return &EquipmentDetail{
			Name:   item.Name,
			Type:   shared.EquipmentTypeItem,
			Weight: item.Weight,
			Cost:   item.Cost,
		}
	}
	return nil
}

func resolveWeaponDetail(wep *weapons.Weapon) *EquipmentDetail {
	return &EquipmentDetail{
		Name:   wep.Name,
		Type:   shared.EquipmentTypeWeapon,
		Weight: wep.Weight,
		Cost:   wep.Cost,
		Weapon: &WeaponDetail{
			Category:   wep.Category,
			Damage:     wep.Damage,
			DamageType: wep.DamageType,
			Properties: wep.Properties,
			Range:      wep.Range,
		},
	}
}

func resolveArmorDetail(arm *armor.Armor) *EquipmentDetail {
	return &EquipmentDetail{
		Name:   arm.Name,
		Type:   shared.EquipmentTypeArmor,
		Weight: float64(arm.Weight),
		Cost:   arm.Cost,
		Armor: &ArmorDetail{
			Category:            arm.Category,
			BaseAC:              arm.AC,
			DexBonus:            arm.MaxDexBonus == nil || *arm.MaxDexBonus > 0,
			MaxDexBonus:         arm.MaxDexBonus,
			StrengthRequirement: arm.Strength,
			StealthDisadvantage: arm.StealthDisadvantage,
		},
	}
}
