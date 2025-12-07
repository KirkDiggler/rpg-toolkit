package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Conditions provides type-safe, discoverable references to D&D 5e conditions.
// Use IDE autocomplete: refs.Conditions.<tab> to discover available conditions.
var Conditions = conditionsNS{ns{TypeConditions}}

type conditionsNS struct{ ns }

// Class-based conditions
func (n conditionsNS) Raging() *core.Ref           { return n.ref("raging") }
func (n conditionsNS) BrutalCritical() *core.Ref   { return n.ref("brutal_critical") }
func (n conditionsNS) UnarmoredDefense() *core.Ref { return n.ref("unarmored_defense") }
func (n conditionsNS) FightingStyle() *core.Ref    { return n.ref("fighting_style") }
func (n conditionsNS) ImprovedCritical() *core.Ref { return n.ref("improved_critical") }

// Standard D&D 5e Conditions
func (n conditionsNS) Blinded() *core.Ref       { return n.ref("blinded") }
func (n conditionsNS) Charmed() *core.Ref       { return n.ref("charmed") }
func (n conditionsNS) Deafened() *core.Ref      { return n.ref("deafened") }
func (n conditionsNS) Frightened() *core.Ref    { return n.ref("frightened") }
func (n conditionsNS) Grappled() *core.Ref      { return n.ref("grappled") }
func (n conditionsNS) Incapacitated() *core.Ref { return n.ref("incapacitated") }
func (n conditionsNS) Invisible() *core.Ref     { return n.ref("invisible") }
func (n conditionsNS) Paralyzed() *core.Ref     { return n.ref("paralyzed") }
func (n conditionsNS) Petrified() *core.Ref     { return n.ref("petrified") }
func (n conditionsNS) Poisoned() *core.Ref      { return n.ref("poisoned") }
func (n conditionsNS) Prone() *core.Ref         { return n.ref("prone") }
func (n conditionsNS) Restrained() *core.Ref    { return n.ref("restrained") }
func (n conditionsNS) Stunned() *core.Ref       { return n.ref("stunned") }
func (n conditionsNS) Unconscious() *core.Ref   { return n.ref("unconscious") }
func (n conditionsNS) Exhaustion() *core.Ref    { return n.ref("exhaustion") }
