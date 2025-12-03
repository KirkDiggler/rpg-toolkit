package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Classes provides type-safe, discoverable references to D&D 5e classes.
// Use IDE autocomplete: refs.Classes.<tab> to discover available classes.
var Classes = classesNS{}

type classesNS struct{}

// Barbarian returns a reference to the Barbarian class.
func (classesNS) Barbarian() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeClasses, ID: "barbarian"}
}

// Bard returns a reference to the Bard class.
func (classesNS) Bard() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeClasses, ID: "bard"}
}

// Cleric returns a reference to the Cleric class.
func (classesNS) Cleric() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeClasses, ID: "cleric"}
}

// Druid returns a reference to the Druid class.
func (classesNS) Druid() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeClasses, ID: "druid"}
}

// Fighter returns a reference to the Fighter class.
func (classesNS) Fighter() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeClasses, ID: "fighter"}
}

// Monk returns a reference to the Monk class.
func (classesNS) Monk() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeClasses, ID: "monk"}
}

// Paladin returns a reference to the Paladin class.
func (classesNS) Paladin() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeClasses, ID: "paladin"}
}

// Ranger returns a reference to the Ranger class.
func (classesNS) Ranger() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeClasses, ID: "ranger"}
}

// Rogue returns a reference to the Rogue class.
func (classesNS) Rogue() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeClasses, ID: "rogue"}
}

// Sorcerer returns a reference to the Sorcerer class.
func (classesNS) Sorcerer() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeClasses, ID: "sorcerer"}
}

// Warlock returns a reference to the Warlock class.
func (classesNS) Warlock() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeClasses, ID: "warlock"}
}

// Wizard returns a reference to the Wizard class.
func (classesNS) Wizard() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeClasses, ID: "wizard"}
}
