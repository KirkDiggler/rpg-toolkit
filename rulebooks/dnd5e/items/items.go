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

// TODO: Add more items as needed
