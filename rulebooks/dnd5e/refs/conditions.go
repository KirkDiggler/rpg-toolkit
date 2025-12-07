package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Conditions provides type-safe, discoverable references to D&D 5e conditions.
// Use IDE autocomplete: refs.Conditions.<tab> to discover available conditions.
var Conditions = conditionsNS{}

type conditionsNS struct{}

// Raging returns a reference to the Raging condition (active during Barbarian's rage).
func (conditionsNS) Raging() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "raging"}
}

// BrutalCritical returns a reference to the Brutal Critical condition.
func (conditionsNS) BrutalCritical() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "brutal_critical"}
}

// UnarmoredDefense returns a reference to the Unarmored Defense condition.
func (conditionsNS) UnarmoredDefense() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "unarmored_defense"}
}

// FightingStyle returns a reference to the Fighting Style condition.
func (conditionsNS) FightingStyle() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "fighting_style"}
}

// ImprovedCritical returns a reference to the Improved Critical condition (Champion subclass feature).
func (conditionsNS) ImprovedCritical() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "improved_critical"}
}

// Standard D&D 5e Conditions
// These are core conditions that affect combat, movement, and character capabilities

// Blinded returns a reference to the Blinded condition.
func (conditionsNS) Blinded() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "blinded"}
}

// Charmed returns a reference to the Charmed condition.
func (conditionsNS) Charmed() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "charmed"}
}

// Deafened returns a reference to the Deafened condition.
func (conditionsNS) Deafened() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "deafened"}
}

// Frightened returns a reference to the Frightened condition.
func (conditionsNS) Frightened() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "frightened"}
}

// Grappled returns a reference to the Grappled condition.
func (conditionsNS) Grappled() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "grappled"}
}

// Incapacitated returns a reference to the Incapacitated condition.
func (conditionsNS) Incapacitated() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "incapacitated"}
}

// Invisible returns a reference to the Invisible condition.
func (conditionsNS) Invisible() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "invisible"}
}

// Paralyzed returns a reference to the Paralyzed condition.
func (conditionsNS) Paralyzed() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "paralyzed"}
}

// Petrified returns a reference to the Petrified condition.
func (conditionsNS) Petrified() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "petrified"}
}

// Poisoned returns a reference to the Poisoned condition.
func (conditionsNS) Poisoned() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "poisoned"}
}

// Prone returns a reference to the Prone condition.
func (conditionsNS) Prone() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "prone"}
}

// Restrained returns a reference to the Restrained condition.
func (conditionsNS) Restrained() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "restrained"}
}

// Stunned returns a reference to the Stunned condition.
func (conditionsNS) Stunned() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "stunned"}
}

// Unconscious returns a reference to the Unconscious condition.
func (conditionsNS) Unconscious() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "unconscious"}
}

// Exhaustion returns a reference to the Exhaustion condition.
func (conditionsNS) Exhaustion() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeConditions, ID: "exhaustion"}
}
