// Package equipment provides a unified interface for D&D 5e equipment items
package equipment

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/ammunition"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/packs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/tools"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// Category constants re-exported from shared, which is the single source
// of truth (see shared.EquipmentCategory's doc comment) — kept here too so
// existing callers referencing equipment.CategoryX don't need to change.
const (
	CategoryMusicalInstruments = shared.CategoryMusicalInstruments
	CategoryDruidicFoci        = shared.CategoryDruidicFoci
	CategoryHolySymbols        = shared.CategoryHolySymbols
	CategoryArcaneFoci         = shared.CategoryArcaneFoci
	// CategoryArtisanTools selects all artisan's tool kits.
	CategoryArtisanTools = shared.CategoryArtisanTools
)

// Equipment represents any item that can be owned, carried, or equipped
type Equipment interface {
	// EquipmentID returns the unique identifier for this equipment
	EquipmentID() string

	// EquipmentType returns the category of equipment
	EquipmentType() shared.EquipmentType

	// EquipmentCategories returns the shared.EquipmentCategory tags this
	// item can be found by in a category choice (e.g. a longsword reports
	// its weapon category, a lute reports CategoryMusicalInstruments). Most
	// equipment reports none — only items that actually participate in a
	// category choice today have any.
	EquipmentCategories() []shared.EquipmentCategory

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
func GetByID(id shared.SelectionID) (Equipment, error) {
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

	// Check ammunition
	ammo, ok := ammunition.StandardAmmunition[id]
	if ok {
		return ammo, nil
	}

	// Check miscellaneous items
	item, ok := items.All[id]
	if ok {
		return &item, nil
	}

	return nil, rpgerr.New(rpgerr.CodeNotFound, "equipment not found")
}

// GetByCategory returns all equipment matching any of the given
// categories, one requested category at a time (each in its own
// registry's deterministic order) so a caller requesting multiple
// categories together (e.g. simple-melee and simple-ranged weapons) still
// gets them grouped by category, matching how these lists are presented
// to a player.
//
// equipType is accepted for backward compatibility with existing callers
// but no longer filters results. Category values are inherently
// type-specific already — no weapon category string collides with a tool
// or item category string — so gating on equipType would only risk
// silently dropping a real result, exactly as it would have for a holy
// symbol (which reports EquipmentTypeItem, not EquipmentTypeTool) had the
// previous switch-based dispatch's implicit exemption for it not existed.
func GetByCategory(equipType shared.EquipmentType, categories []shared.EquipmentCategory) ([]Equipment, error) {
	_ = equipType
	if len(categories) == 0 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "no categories specified")
	}

	var result []Equipment
	for _, cat := range categories {
		result = append(result, matchCategory(cat)...)
	}

	// Retain the first occurrence while collapsing an ID that belongs to
	// more than one requested category, so category choices preserve
	// their grouped order without duplicate options.
	seen := make(map[string]struct{}, len(result))
	unique := make([]Equipment, 0, len(result))
	for _, item := range result {
		if _, ok := seen[item.EquipmentID()]; ok {
			continue
		}
		seen[item.EquipmentID()] = struct{}{}
		unique = append(unique, item)
	}

	return unique, nil
}

// matchCategory returns every equipped item tagged with cat, in each
// registry's own deterministic order. Weapons and armor already classify
// themselves using the shared vocabulary directly (WeaponCategory/
// ArmorCategory are aliases of shared.EquipmentCategory), so their
// existing GetByCategory is reused as-is; tools and items each own a
// translation from their internal classification to the shared
// vocabulary (EligibleForCategory), since neither's internal category
// values match the public ones directly.
func matchCategory(cat shared.EquipmentCategory) []Equipment {
	var result []Equipment

	for _, w := range weapons.GetByCategory(cat) {
		result = append(result, &w)
	}
	for _, a := range armor.GetByCategory(cat) {
		result = append(result, &a)
	}
	for _, t := range tools.EligibleForCategory(cat) {
		result = append(result, &t)
	}
	for _, i := range items.EligibleForCategory(cat) {
		result = append(result, &i)
	}

	return result
}
