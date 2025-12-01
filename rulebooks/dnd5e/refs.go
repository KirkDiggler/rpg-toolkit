// Package dnd5e provides ref builders for D&D 5e content.
package dnd5e

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// ClassRef builds a ref for a class.
// Example: ClassRef(classes.Barbarian) -> {Module: "dnd5e", Type: "classes", ID: "barbarian"}
func ClassRef(c classes.Class) core.Ref {
	return core.Ref{Module: Module, Type: "classes", ID: c}
}

// FeatureRef builds a ref for a feature.
// Example: FeatureRef("rage") -> {Module: "dnd5e", Type: "features", ID: "rage"}
func FeatureRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: "features", ID: id}
}

// ConditionRef builds a ref for a condition.
// Example: ConditionRef("unarmored_defense") -> {Module: "dnd5e", Type: "conditions", ID: "unarmored_defense"}
func ConditionRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: "conditions", ID: id}
}

// SpellRef builds a ref for a spell.
func SpellRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: "spells", ID: id}
}

// SkillRef builds a ref for a skill.
func SkillRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: "skills", ID: id}
}

// RaceRef builds a ref for a race.
func RaceRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: "races", ID: id}
}

// BackgroundRef builds a ref for a background.
func BackgroundRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: "backgrounds", ID: id}
}
