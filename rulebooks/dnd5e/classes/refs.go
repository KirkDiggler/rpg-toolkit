package classes

import "github.com/KirkDiggler/rpg-toolkit/core"

// Ref creates a class reference from an index
func Ref(index string) *core.Ref {
	return core.MustNewRef(core.RefInput{
		Module: "dnd5e",
		Type:   "class",
		Value:  index,
	})
}

// Common class refs as constants
var (
	// Player's Handbook classes
	Barbarian = Ref("barbarian")
	Bard      = Ref("bard")
	Cleric    = Ref("cleric")
	Druid     = Ref("druid")
	Fighter   = Ref("fighter")
	Monk      = Ref("monk")
	Paladin   = Ref("paladin")
	Ranger    = Ref("ranger")
	Rogue     = Ref("rogue")
	Sorcerer  = Ref("sorcerer")
	Warlock   = Ref("warlock")
	Wizard    = Ref("wizard")
)