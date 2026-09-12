// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// CastDefinition compiles content with this character's current casting facts.
// Knowing or preparing the spell is the caller's access check.
func (c *Character) CastDefinition(id spells.Spell) *actions.Definition {
	input := spells.CastDefinitionInput{Spell: id, SpellSaveDC: c.SpellSaveDC()}
	class := classes.ClassData[c.classID]
	if class != nil && class.SpellcastingAbility != "" {
		ability := class.SpellcastingAbility
		input.HealingModifiers = []healing.Modifier{{Source: events.RollSource{
			Ref:  &core.Ref{Module: refs.Module, Type: refs.TypeAbilities, ID: string(ability)},
			Name: ability.Display(), Label: "Spellcasting modifier", SourceID: c.id,
		}, Amount: c.GetAbilityModifier(ability)}}
		if data := spells.GetData(id); data != nil && c.classID == classes.Cleric && c.subclassID == classes.LifeDomain && c.level >= 1 {
			input.HealingModifiers = append(input.HealingModifiers, features.DiscipleOfLife(healing.Context{Spell: true, SpellLevel: data.Level}, c.id)...)
		}
	}
	return spells.CastDefinition(input)
}
