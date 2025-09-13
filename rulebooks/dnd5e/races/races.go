// Package races provides D&D 5e race constants and definitions
package races

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// Race represents a D&D 5e player race
type Race string

// Subrace is an alias for Race (subraces are just specific race variants)
type Subrace = Race

// Core races from Player's Handbook
const (
	Invalid    Race = "invalid" // Invalid or unknown race
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

// Subrace constants
const (
	SubraceNone Race = "none" // No subrace (for races without subraces)
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

// All provides map lookup for base races only (no subraces)
// Deprecated: Use RaceData directly - it now contains ID field and Name()/Description() methods
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
}

// Subraces provides map lookup for subraces only
var Subraces = map[string]Race{
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

// AllIncludingSubraces provides map lookup for all races and subraces
var AllIncludingSubraces = map[string]Race{
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

// GetByID returns a race or subrace by its ID
func GetByID(id string) (Race, error) {
	race, ok := AllIncludingSubraces[id]
	if !ok {
		validRaces := make([]string, 0, len(AllIncludingSubraces))
		for k := range AllIncludingSubraces {
			validRaces = append(validRaces, k)
		}
		return "", rpgerr.New(rpgerr.CodeInvalidArgument, "invalid race",
			rpgerr.WithMeta("provided", id),
			rpgerr.WithMeta("valid_options", validRaces))
	}
	return race, nil
}

// Name is a human readable name for the race
func (r Race) Name() string {
	switch r {
	case Human:
		return "Human"
	case Elf:
		return "Elf"
	case Dwarf:
		return "Dwarf"
	case Halfling:
		return "Halfling"
	case Dragonborn:
		return "Dragonborn"
	case Gnome:
		return "Gnome"
	case HalfElf:
		return "Half-Elf"
	case HalfOrc:
		return "Half-Orc"
	case Tiefling:
		return "Tiefling"
	default:
		return "Unknown"
	}
}

// Description is a brief description for a race
func (r Race) Description() string {
	switch r {
	case Human:
		return "Humans are the most common race in the world, with a diverse array of backgrounds and cultures."
	case Elf:
		return "Elves are a long-lived race with a deep connection to nature and a love of magic."
	case Dwarf:
		return "Dwarves are a hardy race with a love of mining and a deep connection to the earth."
	case Halfling:
		return "Halflings are a small race with a love of food and a deep connection to the land."
	case Dragonborn:
		return "Dragonborn are a powerful race with a love of magic and a deep connection to the earth."
	case Gnome:
		return "Gnomes are a small race with a love of magic and a deep connection to the earth."
	case HalfElf:
		return "Half-Elves are a powerful race with a love of magic and a deep connection to the earth."
	case HalfOrc:
		return "Half-Orcs are a powerful race with a love of magic and a deep connection to the earth."
	case Tiefling:
		return "Tieflings are a powerful race with a love of magic and a deep connection to the earth."
	default:
		return "Unknown race"
	}
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
