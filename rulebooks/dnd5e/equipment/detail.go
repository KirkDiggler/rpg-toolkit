package equipment

import (
	"fmt"

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
	Damage     []damage.Damage          `json:"damage"`
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

// ResolvedPackItem is one content line of a pack, with its real
// shared.EquipmentType resolved against the catalog — packs.PackItem
// itself only carries a bare ID, not the type an inventory needs.
type ResolvedPackItem struct {
	Type     shared.EquipmentType
	ID       string
	Quantity int
}

// ResolvePackContents resolves a pack's Contents into typed lines a caller
// can add straight to an inventory (character.AddInventoryItem) — the one
// shared primitive both character/draft.go's compileInventory and
// session.Unpack use, so pack decomposition has exactly one implementation
// rather than two that could drift (rpg-toolkit#1544).
//
// Returns (nil, false, nil) when id does not name a pack — not an error,
// the same convention npcs.VendorInventoryFromNPCData already uses to
// distinguish "doesn't apply" from "is malformed." Returns (nil, true, err)
// when id IS a pack but a content line's ID doesn't resolve against the
// catalog — a content-authoring defect. Every pack in the current catalog
// is verified clean (equipment's own pack_contents_test.go), so this arm
// exists to fail a future authoring mistake in a test rather than let it
// through silently.
func ResolvePackContents(id shared.EquipmentID) ([]ResolvedPackItem, bool, error) {
	pack, ok := packs.All[id]
	if !ok {
		return nil, false, nil
	}

	resolved := make([]ResolvedPackItem, 0, len(pack.Contents))
	for _, content := range pack.Contents {
		detail := ResolveEquipmentDetail(shared.EquipmentID(content.ItemID))
		if detail == nil {
			return nil, true, fmt.Errorf("pack %q content %q does not resolve against the catalog", id, content.ItemID)
		}
		resolved = append(resolved, ResolvedPackItem{
			Type: detail.Type, ID: content.ItemID, Quantity: content.Quantity,
		})
	}
	return resolved, true, nil
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
