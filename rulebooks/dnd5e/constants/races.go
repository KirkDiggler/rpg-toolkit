package constants

// Race represents a D&D 5e race
type Race string

// Race constants
const (
	RaceHuman      Race = "human"
	RaceDwarf      Race = "dwarf"
	RaceElf        Race = "elf"
	RaceHalfling   Race = "halfling"
	RaceDragonborn Race = "dragonborn"
	RaceGnome      Race = "gnome"
	RaceHalfElf    Race = "half-elf"
	RaceHalfOrc    Race = "half-orc"
	RaceTiefling   Race = "tiefling"
)

// Display returns the human-readable name of the race
func (r Race) Display() string {
	switch r {
	case RaceHuman:
		return "Human"
	case RaceDwarf:
		return "Dwarf"
	case RaceElf:
		return "Elf"
	case RaceHalfling:
		return "Halfling"
	case RaceDragonborn:
		return "Dragonborn"
	case RaceGnome:
		return "Gnome"
	case RaceHalfElf:
		return "Half-Elf"
	case RaceHalfOrc:
		return "Half-Orc"
	case RaceTiefling:
		return "Tiefling"
	default:
		return string(r)
	}
}

// Subrace represents a D&D 5e subrace
type Subrace string

// Subrace constants
const (
	SubraceHighElf           Subrace = "high-elf"
	SubraceWoodElf           Subrace = "wood-elf"
	SubraceDarkElf           Subrace = "dark-elf"
	SubraceHillDwarf         Subrace = "hill-dwarf"
	SubraceMountainDwarf     Subrace = "mountain-dwarf"
	SubraceLightfootHalfling Subrace = "lightfoot-halfling"
	SubraceStoutHalfling     Subrace = "stout-halfling"
	SubraceForestGnome       Subrace = "forest-gnome"
	SubraceRockGnome         Subrace = "rock-gnome"
)

// Display returns the human-readable name of the subrace
func (s Subrace) Display() string {
	switch s {
	case SubraceHighElf:
		return "High Elf"
	case SubraceWoodElf:
		return "Wood Elf"
	case SubraceDarkElf:
		return "Dark Elf (Drow)"
	case SubraceHillDwarf:
		return "Hill Dwarf"
	case SubraceMountainDwarf:
		return "Mountain Dwarf"
	case SubraceLightfootHalfling:
		return "Lightfoot Halfling"
	case SubraceStoutHalfling:
		return "Stout Halfling"
	case SubraceForestGnome:
		return "Forest Gnome"
	case SubraceRockGnome:
		return "Rock Gnome"
	default:
		return string(s)
	}
}

// Size represents creature size categories
type Size string

// Size constants
const (
	SizeTiny       Size = "tiny"
	SizeSmall      Size = "small"
	SizeMedium     Size = "medium"
	SizeLarge      Size = "large"
	SizeHuge       Size = "huge"
	SizeGargantuan Size = "gargantuan"
)

// Display returns the human-readable name of the size
func (s Size) Display() string {
	switch s {
	case SizeTiny:
		return "Tiny"
	case SizeSmall:
		return "Small"
	case SizeMedium:
		return "Medium"
	case SizeLarge:
		return "Large"
	case SizeHuge:
		return "Huge"
	case SizeGargantuan:
		return "Gargantuan"
	default:
		return string(s)
	}
}
