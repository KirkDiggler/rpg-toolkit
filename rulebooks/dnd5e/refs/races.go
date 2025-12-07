//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Races provides type-safe, discoverable references to D&D 5e races.
// Use IDE autocomplete: refs.Races.<tab> to discover available races.
var Races = racesNS{ns{TypeRaces}}

type racesNS struct{ ns }

// Base Races
func (n racesNS) Human() *core.Ref      { return n.ref("human") }
func (n racesNS) Elf() *core.Ref        { return n.ref("elf") }
func (n racesNS) Dwarf() *core.Ref      { return n.ref("dwarf") }
func (n racesNS) Halfling() *core.Ref   { return n.ref("halfling") }
func (n racesNS) Dragonborn() *core.Ref { return n.ref("dragonborn") }
func (n racesNS) Gnome() *core.Ref      { return n.ref("gnome") }
func (n racesNS) HalfElf() *core.Ref    { return n.ref("half-elf") }
func (n racesNS) HalfOrc() *core.Ref    { return n.ref("half-orc") }
func (n racesNS) Tiefling() *core.Ref   { return n.ref("tiefling") }

// Elf Subraces
func (n racesNS) HighElf() *core.Ref { return n.ref("high-elf") }
func (n racesNS) WoodElf() *core.Ref { return n.ref("wood-elf") }
func (n racesNS) DarkElf() *core.Ref { return n.ref("dark-elf") }

// Dwarf Subraces
func (n racesNS) MountainDwarf() *core.Ref { return n.ref("mountain-dwarf") }
func (n racesNS) HillDwarf() *core.Ref     { return n.ref("hill-dwarf") }

// Halfling Subraces
func (n racesNS) LightfootHalfling() *core.Ref { return n.ref("lightfoot-halfling") }
func (n racesNS) StoutHalfling() *core.Ref     { return n.ref("stout-halfling") }

// Gnome Subraces
func (n racesNS) ForestGnome() *core.Ref { return n.ref("forest-gnome") }
func (n racesNS) RockGnome() *core.Ref   { return n.ref("rock-gnome") }
