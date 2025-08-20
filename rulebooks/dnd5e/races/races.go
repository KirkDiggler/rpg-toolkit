// Package races provides D&D 5e race constants and definitions
package races

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// Race represents a D&D 5e player race
type Race string

// Core races from Player's Handbook
const (
	Human      Race = "human"
	Elf        Race = "elf"
	Dwarf      Race = "dwarf"
	Halfling   Race = "halfling"
	Dragonborn Race = "dragonborn"
	Gnome      Race = "gnome"
	HalfElf    Race = "half-elf"
	HalfOrc    Race = "half-orc"
	Tiefling   Race = "tiefling"
)

// Elf subraces
const (
	HighElf Race = "high-elf"
	WoodElf Race = "wood-elf"
	DarkElf Race = "dark-elf" // Drow
)

// Dwarf subraces
const (
	MountainDwarf Race = "mountain-dwarf"
	HillDwarf     Race = "hill-dwarf"
)

// Halfling subraces
const (
	LightfootHalfling Race = "lightfoot-halfling"
	StoutHalfling     Race = "stout-halfling"
)

// Gnome subraces
const (
	ForestGnome Race = "forest-gnome"
	RockGnome   Race = "rock-gnome"
)

// All provides map lookup for races
var All = map[string]Race{
	"human":      Human,
	"elf":        Elf,
	"dwarf":      Dwarf,
	"halfling":   Halfling,
	"dragonborn": Dragonborn,
	"gnome":      Gnome,
	"half-elf":   HalfElf,
	"half-orc":   HalfOrc,
	"tiefling":   Tiefling,
	// Subraces
	"high-elf":           HighElf,
	"wood-elf":           WoodElf,
	"dark-elf":           DarkElf,
	"mountain-dwarf":     MountainDwarf,
	"hill-dwarf":         HillDwarf,
	"lightfoot-halfling": LightfootHalfling,
	"stout-halfling":     StoutHalfling,
	"forest-gnome":       ForestGnome,
	"rock-gnome":         RockGnome,
}

// GetByID returns a race by its ID
func GetByID(id string) (Race, error) {
	race, ok := All[id]
	if !ok {
		validRaces := make([]string, 0, len(All))
		for k := range All {
			validRaces = append(validRaces, k)
		}
		return "", rpgerr.New(rpgerr.CodeInvalidArgument, "invalid race",
			rpgerr.WithMeta("provided", id),
			rpgerr.WithMeta("valid_options", validRaces))
	}
	return race, nil
}

// IsSubrace returns true if this is a subrace
func (r Race) IsSubrace() bool {
	switch r {
	case HighElf, WoodElf, DarkElf,
		MountainDwarf, HillDwarf,
		LightfootHalfling, StoutHalfling,
		ForestGnome, RockGnome:
		return true
	default:
		return false
	}
}

// ParentRace returns the parent race for subraces
func (r Race) ParentRace() Race {
	switch r {
	case HighElf, WoodElf, DarkElf:
		return Elf
	case MountainDwarf, HillDwarf:
		return Dwarf
	case LightfootHalfling, StoutHalfling:
		return Halfling
	case ForestGnome, RockGnome:
		return Gnome
	default:
		return r
	}
}
