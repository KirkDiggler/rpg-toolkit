// Package classes provides D&D 5e class constants
package classes

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// Class represents a D&D 5e character class
type Class string

// Core classes from Player's Handbook
const (
	Barbarian Class = "barbarian"
	Bard      Class = "bard"
	Cleric    Class = "cleric"
	Druid     Class = "druid"
	Fighter   Class = "fighter"
	Monk      Class = "monk"
	Paladin   Class = "paladin"
	Ranger    Class = "ranger"
	Rogue     Class = "rogue"
	Sorcerer  Class = "sorcerer"
	Warlock   Class = "warlock"
	Wizard    Class = "wizard"
)

// All provides map lookup for classes
var All = map[string]Class{
	"barbarian": Barbarian,
	"bard":      Bard,
	"cleric":    Cleric,
	"druid":     Druid,
	"fighter":   Fighter,
	"monk":      Monk,
	"paladin":   Paladin,
	"ranger":    Ranger,
	"rogue":     Rogue,
	"sorcerer":  Sorcerer,
	"warlock":   Warlock,
	"wizard":    Wizard,
}

// GetByID returns a class by its ID
func GetByID(id string) (Class, error) {
	class, ok := All[id]
	if !ok {
		validClasses := make([]string, 0, len(All))
		for k := range All {
			validClasses = append(validClasses, k)
		}
		return "", rpgerr.New(rpgerr.CodeInvalidArgument, "invalid class",
			rpgerr.WithMeta("provided", id),
			rpgerr.WithMeta("valid_options", validClasses))
	}
	return class, nil
}
