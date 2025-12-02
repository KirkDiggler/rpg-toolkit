// Package dnd5e provides ref builders for D&D 5e content.
package dnd5e

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// Type constants - duplicated here to avoid import cycles.
// The canonical definitions are in each domain package.
const (
	TypeClasses     core.Type = "classes"
	TypeFeatures    core.Type = "features"
	TypeConditions  core.Type = "conditions"
	TypeSpells      core.Type = "spells"
	TypeSkills      core.Type = "skills"
	TypeRaces       core.Type = "races"
	TypeBackgrounds core.Type = "backgrounds"
)

// ClassRef builds a ref for a class.
// Example: ClassRef(classes.Barbarian) -> {Module: "dnd5e", Type: "classes", ID: "barbarian"}
func ClassRef(c classes.Class) core.Ref {
	return core.Ref{Module: Module, Type: TypeClasses, ID: c}
}

// FeatureRef builds a ref for a feature.
// Example: FeatureRef(features.RageID) -> {Module: "dnd5e", Type: "features", ID: "rage"}
func FeatureRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: TypeFeatures, ID: id}
}

// ConditionRef builds a ref for a condition.
// Example: ConditionRef(conditions.RagingID) -> {Module: "dnd5e", Type: "conditions", ID: "raging"}
func ConditionRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: TypeConditions, ID: id}
}

// SpellRef builds a ref for a spell.
// Example: SpellRef(spells.Fireball) -> {Module: "dnd5e", Type: "spells", ID: "fireball"}
func SpellRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: TypeSpells, ID: id}
}

// SkillRef builds a ref for a skill.
// Example: SkillRef(skills.Athletics) -> {Module: "dnd5e", Type: "skills", ID: "athletics"}
func SkillRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: TypeSkills, ID: id}
}

// RaceRef builds a ref for a race.
// Example: RaceRef(races.Human) -> {Module: "dnd5e", Type: "races", ID: "human"}
func RaceRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: TypeRaces, ID: id}
}

// BackgroundRef builds a ref for a background.
// Example: BackgroundRef(backgrounds.Soldier) -> {Module: "dnd5e", Type: "backgrounds", ID: "soldier"}
func BackgroundRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: TypeBackgrounds, ID: id}
}
