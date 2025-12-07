//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Classes provides type-safe, discoverable references to D&D 5e classes.
// Use IDE autocomplete: refs.Classes.<tab> to discover available classes.
var Classes = classesNS{ns{TypeClasses}}

type classesNS struct{ ns }

func (n classesNS) Barbarian() *core.Ref { return n.ref("barbarian") }
func (n classesNS) Bard() *core.Ref      { return n.ref("bard") }
func (n classesNS) Cleric() *core.Ref    { return n.ref("cleric") }
func (n classesNS) Druid() *core.Ref     { return n.ref("druid") }
func (n classesNS) Fighter() *core.Ref   { return n.ref("fighter") }
func (n classesNS) Monk() *core.Ref      { return n.ref("monk") }
func (n classesNS) Paladin() *core.Ref   { return n.ref("paladin") }
func (n classesNS) Ranger() *core.Ref    { return n.ref("ranger") }
func (n classesNS) Rogue() *core.Ref     { return n.ref("rogue") }
func (n classesNS) Sorcerer() *core.Ref  { return n.ref("sorcerer") }
func (n classesNS) Warlock() *core.Ref   { return n.ref("warlock") }
func (n classesNS) Wizard() *core.Ref    { return n.ref("wizard") }
