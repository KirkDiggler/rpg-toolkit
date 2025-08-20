// Package races provides ref builders and constants for D&D 5e races.
package races

import "github.com/KirkDiggler/rpg-toolkit/core"

// Ref creates a race reference from an index
func Ref(index string) *core.Ref {
	return core.MustNewRef(core.RefInput{
		Module: "dnd5e",
		Type:   "race",
		Value:  index,
	})
}

// Common race refs as constants
var (
	// Player's Handbook races
	Dragonborn = Ref("dragonborn")
	Dwarf      = Ref("dwarf")
	Elf        = Ref("elf")
	Gnome      = Ref("gnome")
	HalfElf    = Ref("half-elf")
	Halfling   = Ref("halfling")
	HalfOrc    = Ref("half-orc")
	Human      = Ref("human")
	Tiefling   = Ref("tiefling")
)
