package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Skills provides type-safe, discoverable references to D&D 5e skills.
// Use IDE autocomplete: refs.Skills.<tab> to discover available skills.
var Skills = skillsNS{}

type skillsNS struct{}

// Acrobatics returns a reference to the Acrobatics skill.
func (skillsNS) Acrobatics() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "acrobatics"}
}

// AnimalHandling returns a reference to the Animal Handling skill.
func (skillsNS) AnimalHandling() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "animal-handling"}
}

// Arcana returns a reference to the Arcana skill.
func (skillsNS) Arcana() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "arcana"}
}

// Athletics returns a reference to the Athletics skill.
func (skillsNS) Athletics() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "athletics"}
}

// Deception returns a reference to the Deception skill.
func (skillsNS) Deception() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "deception"}
}

// History returns a reference to the History skill.
func (skillsNS) History() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "history"}
}

// Insight returns a reference to the Insight skill.
func (skillsNS) Insight() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "insight"}
}

// Intimidation returns a reference to the Intimidation skill.
func (skillsNS) Intimidation() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "intimidation"}
}

// Investigation returns a reference to the Investigation skill.
func (skillsNS) Investigation() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "investigation"}
}

// Medicine returns a reference to the Medicine skill.
func (skillsNS) Medicine() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "medicine"}
}

// Nature returns a reference to the Nature skill.
func (skillsNS) Nature() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "nature"}
}

// Perception returns a reference to the Perception skill.
func (skillsNS) Perception() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "perception"}
}

// Performance returns a reference to the Performance skill.
func (skillsNS) Performance() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "performance"}
}

// Persuasion returns a reference to the Persuasion skill.
func (skillsNS) Persuasion() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "persuasion"}
}

// Religion returns a reference to the Religion skill.
func (skillsNS) Religion() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "religion"}
}

// SleightOfHand returns a reference to the Sleight of Hand skill.
func (skillsNS) SleightOfHand() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "sleight-of-hand"}
}

// Stealth returns a reference to the Stealth skill.
func (skillsNS) Stealth() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "stealth"}
}

// Survival returns a reference to the Survival skill.
func (skillsNS) Survival() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSkills, ID: "survival"}
}
