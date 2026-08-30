// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

// markSaved clears the persistence flag so the next assertion is about the
// action under test alone, and nothing that happened while arranging it.
//
// Character.MarkClean did this until rpg-project#319 Phase 6 deleted it: it
// was on combat.Combatant and on both sheets, and no production code ever
// called it — resolution READS IsDirty to decide what to hand back and never
// clears it. The tests using it were pinning dirty TRANSITIONS, not the
// method, so they say what they mean now instead of reaching for a production
// verb that existed for their benefit.
func markSaved(c *Character) {
	c.dirty = false
}
