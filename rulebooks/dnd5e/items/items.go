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
	Name   string
	Weight float64
	Cost   string
}

// All maps item IDs to their definitions.
var All = map[ItemID]Item{
	ComponentPouch: {Name: "Component Pouch", Weight: 2, Cost: "25 gp"},
	ArcaneFocus:    {Name: "Arcane Focus", Weight: 1, Cost: "10 gp"},
	DruidicFocus:   {Name: "Druidic Focus", Weight: 0, Cost: "1 gp"},
	HolySymbol:     {Name: "Holy Symbol", Weight: 0, Cost: "5 gp"},
	Spellbook:      {Name: "Spellbook", Weight: 3, Cost: "50 gp"},
}

// TODO: Add more items as needed
