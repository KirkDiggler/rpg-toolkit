// Package skills provides ref builders and constants for D&D 5e skills.
package skills

import "github.com/KirkDiggler/rpg-toolkit/core"

// Ref creates a skill reference from an index
func Ref(index string) *core.Ref {
	return core.MustNewRef(core.RefInput{
		Module: "dnd5e",
		Type:   "skill",
		Value:  index,
	})
}

// All D&D 5e skills as constants
var (
	// Strength
	Athletics = Ref("athletics")

	// Dexterity
	Acrobatics    = Ref("acrobatics")
	SleightOfHand = Ref("sleight-of-hand")
	Stealth       = Ref("stealth")

	// Intelligence
	Arcana        = Ref("arcana")
	History       = Ref("history")
	Investigation = Ref("investigation")
	Nature        = Ref("nature")
	Religion      = Ref("religion")

	// Wisdom
	AnimalHandling = Ref("animal-handling")
	Insight        = Ref("insight")
	Medicine       = Ref("medicine")
	Perception     = Ref("perception")
	Survival       = Ref("survival")

	// Charisma
	Deception    = Ref("deception")
	Intimidation = Ref("intimidation")
	Performance  = Ref("performance")
	Persuasion   = Ref("persuasion")
)
