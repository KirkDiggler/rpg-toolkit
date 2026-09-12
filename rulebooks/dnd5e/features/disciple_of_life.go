// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// DiscipleOfLife contributes only when the owner heals with a leveled spell.
// Ownership is supplied by the character's persisted class and subclass.
func DiscipleOfLife(context healing.Context, ownerID string) []healing.Modifier {
	if !context.Spell || context.SpellLevel < 1 {
		return nil
	}
	ref := *refs.Features.DiscipleOfLife()
	return []healing.Modifier{{Source: events.RollSource{Ref: &ref, Name: "Disciple of Life", SourceID: ownerID}, Amount: 2 + context.SpellLevel}}
}
