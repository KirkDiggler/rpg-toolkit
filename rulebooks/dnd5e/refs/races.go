//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Race singletons - unexported for controlled access via methods
var (
	// Base Races
	raceHuman      = &core.Ref{Module: Module, Type: TypeRaces, ID: "human"}
	raceElf        = &core.Ref{Module: Module, Type: TypeRaces, ID: "elf"}
	raceDwarf      = &core.Ref{Module: Module, Type: TypeRaces, ID: "dwarf"}
	raceHalfling   = &core.Ref{Module: Module, Type: TypeRaces, ID: "halfling"}
	raceDragonborn = &core.Ref{Module: Module, Type: TypeRaces, ID: "dragonborn"}
	raceGnome      = &core.Ref{Module: Module, Type: TypeRaces, ID: "gnome"}
	raceHalfElf    = &core.Ref{Module: Module, Type: TypeRaces, ID: "half-elf"}
	raceHalfOrc    = &core.Ref{Module: Module, Type: TypeRaces, ID: "half-orc"}
	raceTiefling   = &core.Ref{Module: Module, Type: TypeRaces, ID: "tiefling"}

	// Elf Subraces
	raceHighElf = &core.Ref{Module: Module, Type: TypeRaces, ID: "high-elf"}
	raceWoodElf = &core.Ref{Module: Module, Type: TypeRaces, ID: "wood-elf"}
	raceDarkElf = &core.Ref{Module: Module, Type: TypeRaces, ID: "dark-elf"}

	// Dwarf Subraces
	raceMountainDwarf = &core.Ref{Module: Module, Type: TypeRaces, ID: "mountain-dwarf"}
	raceHillDwarf     = &core.Ref{Module: Module, Type: TypeRaces, ID: "hill-dwarf"}

	// Halfling Subraces
	raceLightfootHalfling = &core.Ref{Module: Module, Type: TypeRaces, ID: "lightfoot-halfling"}
	raceStoutHalfling     = &core.Ref{Module: Module, Type: TypeRaces, ID: "stout-halfling"}

	// Gnome Subraces
	raceForestGnome = &core.Ref{Module: Module, Type: TypeRaces, ID: "forest-gnome"}
	raceRockGnome   = &core.Ref{Module: Module, Type: TypeRaces, ID: "rock-gnome"}
)

// Races provides type-safe, discoverable references to D&D 5e races.
// Use IDE autocomplete: refs.Races.<tab> to discover available races.
// Methods return singleton pointers enabling identity comparison (ref == refs.Races.Human()).
var Races = racesNS{}

type racesNS struct{}

// Base Races
func (n racesNS) Human() *core.Ref      { return raceHuman }
func (n racesNS) Elf() *core.Ref        { return raceElf }
func (n racesNS) Dwarf() *core.Ref      { return raceDwarf }
func (n racesNS) Halfling() *core.Ref   { return raceHalfling }
func (n racesNS) Dragonborn() *core.Ref { return raceDragonborn }
func (n racesNS) Gnome() *core.Ref      { return raceGnome }
func (n racesNS) HalfElf() *core.Ref    { return raceHalfElf }
func (n racesNS) HalfOrc() *core.Ref    { return raceHalfOrc }
func (n racesNS) Tiefling() *core.Ref   { return raceTiefling }

// Elf Subraces
func (n racesNS) HighElf() *core.Ref { return raceHighElf }
func (n racesNS) WoodElf() *core.Ref { return raceWoodElf }
func (n racesNS) DarkElf() *core.Ref { return raceDarkElf }

// Dwarf Subraces
func (n racesNS) MountainDwarf() *core.Ref { return raceMountainDwarf }
func (n racesNS) HillDwarf() *core.Ref     { return raceHillDwarf }

// Halfling Subraces
func (n racesNS) LightfootHalfling() *core.Ref { return raceLightfootHalfling }
func (n racesNS) StoutHalfling() *core.Ref     { return raceStoutHalfling }

// Gnome Subraces
func (n racesNS) ForestGnome() *core.Ref { return raceForestGnome }
func (n racesNS) RockGnome() *core.Ref   { return raceRockGnome }
