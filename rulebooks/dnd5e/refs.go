// Package dnd5e provides ref builders for D&D 5e content.
package dnd5e

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// ClassRef builds a ref for a class.
// Example: ClassRef(classes.Barbarian) -> {Module: "dnd5e", Type: "classes", ID: "barbarian"}
func ClassRef(c classes.Class) core.Ref {
	return core.Ref{Module: Module, Type: classes.Type, ID: c}
}

// FeatureRef builds a ref for a feature.
// Example: FeatureRef(features.RageID) -> {Module: "dnd5e", Type: "features", ID: "rage"}
func FeatureRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: features.Type, ID: id}
}

// ConditionRef builds a ref for a condition.
// Example: ConditionRef(conditions.RagingID) -> {Module: "dnd5e", Type: "conditions", ID: "raging"}
func ConditionRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: conditions.Type, ID: id}
}

// SpellRef builds a ref for a spell.
// Example: SpellRef(spells.Fireball) -> {Module: "dnd5e", Type: "spells", ID: "fireball"}
func SpellRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: spells.Type, ID: id}
}

// SkillRef builds a ref for a skill.
// Example: SkillRef(skills.Athletics) -> {Module: "dnd5e", Type: "skills", ID: "athletics"}
func SkillRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: skills.Type, ID: id}
}

// RaceRef builds a ref for a race.
// Example: RaceRef(races.Human) -> {Module: "dnd5e", Type: "races", ID: "human"}
func RaceRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: races.Type, ID: id}
}

// BackgroundRef builds a ref for a background.
// Example: BackgroundRef(backgrounds.Soldier) -> {Module: "dnd5e", Type: "backgrounds", ID: "soldier"}
func BackgroundRef(id core.ID) core.Ref {
	return core.Ref{Module: Module, Type: backgrounds.Type, ID: id}
}
