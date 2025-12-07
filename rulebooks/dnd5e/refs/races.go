package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Races provides type-safe, discoverable references to D&D 5e races.
// Use IDE autocomplete: refs.Races.<tab> to discover available races.
var Races = racesNS{}

type racesNS struct{}

// Human returns a reference to the Human race.
func (racesNS) Human() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "human"}
}

// Elf returns a reference to the Elf race.
func (racesNS) Elf() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "elf"}
}

// Dwarf returns a reference to the Dwarf race.
func (racesNS) Dwarf() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "dwarf"}
}

// Halfling returns a reference to the Halfling race.
func (racesNS) Halfling() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "halfling"}
}

// Dragonborn returns a reference to the Dragonborn race.
func (racesNS) Dragonborn() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "dragonborn"}
}

// Gnome returns a reference to the Gnome race.
func (racesNS) Gnome() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "gnome"}
}

// HalfElf returns a reference to the Half-Elf race.
func (racesNS) HalfElf() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "half-elf"}
}

// HalfOrc returns a reference to the Half-Orc race.
func (racesNS) HalfOrc() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "half-orc"}
}

// Tiefling returns a reference to the Tiefling race.
func (racesNS) Tiefling() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "tiefling"}
}

// HighElf returns a reference to the High Elf subrace.
func (racesNS) HighElf() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "high-elf"}
}

// WoodElf returns a reference to the Wood Elf subrace.
func (racesNS) WoodElf() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "wood-elf"}
}

// DarkElf returns a reference to the Dark Elf (Drow) subrace.
func (racesNS) DarkElf() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "dark-elf"}
}

// MountainDwarf returns a reference to the Mountain Dwarf subrace.
func (racesNS) MountainDwarf() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "mountain-dwarf"}
}

// HillDwarf returns a reference to the Hill Dwarf subrace.
func (racesNS) HillDwarf() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "hill-dwarf"}
}

// LightfootHalfling returns a reference to the Lightfoot Halfling subrace.
func (racesNS) LightfootHalfling() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "lightfoot-halfling"}
}

// StoutHalfling returns a reference to the Stout Halfling subrace.
func (racesNS) StoutHalfling() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "stout-halfling"}
}

// ForestGnome returns a reference to the Forest Gnome subrace.
func (racesNS) ForestGnome() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "forest-gnome"}
}

// RockGnome returns a reference to the Rock Gnome subrace.
func (racesNS) RockGnome() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeRaces, ID: "rock-gnome"}
}
