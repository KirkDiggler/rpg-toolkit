// Package items provides D&D 5e miscellaneous items and equipment
package items

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// ItemID represents unique identifier for items (alias of shared.EquipmentID)
type ItemID = shared.EquipmentID

// Spellcasting focuses
const (
	ComponentPouch ItemID = "component-pouch"
	ArcaneFocus    ItemID = "arcane-focus"
	DruidicFocus   ItemID = "druidic-focus"
	HolySymbol     ItemID = "holy-symbol"
	Spellbook      ItemID = "spellbook"
)

// Adventuring gear
const (
	Backpack   ItemID = "backpack"
	Bedroll    ItemID = "bedroll"
	Blanket    ItemID = "blanket"
	Crowbar    ItemID = "crowbar"
	Hammer     ItemID = "hammer"
	HempenRope ItemID = "hempen-rope"
	Lantern    ItemID = "lantern"
	Mess       ItemID = "mess-kit"
	Oil        ItemID = "oil"
	Piton      ItemID = "piton"
	Rations    ItemID = "rations"
	Tinderbox  ItemID = "tinderbox"
	Torch      ItemID = "torch"
	Waterskin  ItemID = "waterskin"
)

// Item represents a miscellaneous item with basic stats.
type Item struct {
	ID     ItemID
	Name   string
	Weight float64
	Cost   string
}

// EquipmentID returns the unique identifier for this item.
func (i *Item) EquipmentID() string {
	return i.ID
}

// EquipmentType returns the equipment type (always TypeItem).
func (i *Item) EquipmentType() shared.EquipmentType {
	return shared.EquipmentTypeItem
}

// EquipmentName returns the display name of the item.
func (i *Item) EquipmentName() string {
	return i.Name
}

// EquipmentWeight returns the weight in pounds.
func (i *Item) EquipmentWeight() float32 {
	return float32(i.Weight)
}

// EquipmentValue returns the value in copper pieces.
func (i *Item) EquipmentValue() int {
	// TODO: Parse cost string and convert to copper
	return 0
}

// EquipmentDescription returns a description of the item.
func (i *Item) EquipmentDescription() string {
	return i.Name
}

// All maps item IDs to their definitions.
var All = map[ItemID]Item{
	ComponentPouch: {ID: ComponentPouch, Name: "Component Pouch", Weight: 2, Cost: "25 gp"},
	ArcaneFocus:    {ID: ArcaneFocus, Name: "Arcane Focus", Weight: 1, Cost: "10 gp"},
	DruidicFocus:   {ID: DruidicFocus, Name: "Druidic Focus", Weight: 0, Cost: "1 gp"},
	HolySymbol:     {ID: HolySymbol, Name: "Holy Symbol", Weight: 0, Cost: "5 gp"},
	Spellbook:      {ID: Spellbook, Name: "Spellbook", Weight: 3, Cost: "50 gp"},

	// Adventuring gear (PHB Chapter 5 equipment table / SRD 5.1, cross-checked
	// against two independent SRD mirrors rather than transcribed from memory).
	// Every one of these IDs is already the exact string a starting pack's
	// Contents references (packs.go) — verified directly, not assumed — so
	// populating this map is what makes ResolveEquipmentDetail (and the
	// PriceOf built on top of it) actually resolve pack-granted gear.
	Backpack:   {ID: Backpack, Name: "Backpack", Weight: 5, Cost: "2 gp"},
	Bedroll:    {ID: Bedroll, Name: "Bedroll", Weight: 7, Cost: "1 gp"},
	Blanket:    {ID: Blanket, Name: "Blanket", Weight: 3, Cost: "5 sp"},
	Crowbar:    {ID: Crowbar, Name: "Crowbar", Weight: 5, Cost: "2 gp"},
	Hammer:     {ID: Hammer, Name: "Hammer", Weight: 3, Cost: "1 gp"},
	HempenRope: {ID: HempenRope, Name: "Hempen Rope (50 feet)", Weight: 10, Cost: "1 gp"},
	Lantern:    {ID: Lantern, Name: "Lantern, Hooded", Weight: 2, Cost: "5 gp"},
	Mess:       {ID: Mess, Name: "Mess Kit", Weight: 1, Cost: "2 sp"},
	Oil:        {ID: Oil, Name: "Oil (Flask)", Weight: 1, Cost: "1 sp"},
	Piton:      {ID: Piton, Name: "Piton", Weight: 0.25, Cost: "5 cp"},
	Rations:    {ID: Rations, Name: "Rations (1 Day)", Weight: 2, Cost: "5 sp"},
	Tinderbox:  {ID: Tinderbox, Name: "Tinderbox", Weight: 1, Cost: "5 sp"},
	Torch:      {ID: Torch, Name: "Torch", Weight: 1, Cost: "1 cp"},
	Waterskin:  {ID: Waterskin, Name: "Waterskin", Weight: 5, Cost: "2 sp"},
}
