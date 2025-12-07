package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Abilities provides type-safe, discoverable references to D&D 5e abilities.
// Use IDE autocomplete: refs.Abilities.<tab> to discover available abilities.
var Abilities = abilitiesNS{ns{TypeAbilities}}

type abilitiesNS struct{ ns }

func (n abilitiesNS) Strength() *core.Ref     { return n.ref("str") }
func (n abilitiesNS) Dexterity() *core.Ref    { return n.ref("dex") }
func (n abilitiesNS) Constitution() *core.Ref { return n.ref("con") }
func (n abilitiesNS) Intelligence() *core.Ref { return n.ref("int") }
func (n abilitiesNS) Wisdom() *core.Ref       { return n.ref("wis") }
func (n abilitiesNS) Charisma() *core.Ref     { return n.ref("cha") }
