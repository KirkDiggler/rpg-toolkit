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
