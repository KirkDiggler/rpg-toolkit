// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// Display is the rulebook-owned, display-ready projection of one condition
// ref: a never-empty display name and optional server-authored detail. It is
// the value a character StatusView (Task 4) looks up by ref so the projection
// never has to serialize a condition back to JSON to recover a human-readable
// label, and so the set of conditions a status view can name is bounded by
// this catalog rather than by whatever a live condition happens to carry.
type Display struct {
	// Name is the condition's display name. Never empty for a known ref.
	Name string

	// Detail is optional, server/toolkit-composed display text. May be empty.
	Detail string
}

// DisplayFor returns the display descriptor for the given condition ref, or
// (Display{}, false) when the ref is not in the rulebook-owned catalog. The
// character projection turns a false result into a hard error rather than
// silently dropping the condition, so an unknown ref fails loudly.
func DisplayFor(ref core.Ref) (Display, bool) {
	d, ok := displayCatalog[ref.String()]
	return d, ok
}

// displayCatalog maps the canonical ref string of every condition reachable by
// the four builds (Fighter, Barbarian, Monk, Rogue) to its display descriptor.
// It is keyed by ref.String() because at least one condition — Sneak Attack —
// names itself by a feature ref (refs.Features.SneakAttack) rather than a
// condition ref, so the type alone is not enough to disambiguate.
//
// The catalog deliberately excludes spell-oriented status (e.g. the Shield
// spell condition): a status view is a no-magic projection of the sheet, and
// the existing Shield condition is not promoted into this catalog.
var displayCatalog = map[string]Display{
	// Fighting styles (Fighter).
	refs.Conditions.FightingStyleArchery().String():             {Name: "Archery"},
	refs.Conditions.FightingStyleDefense().String():             {Name: "Defense"},
	refs.Conditions.FightingStyleDueling().String():             {Name: "Dueling"},
	refs.Conditions.FightingStyleGreatWeaponFighting().String(): {Name: "Great Weapon Fighting"},
	refs.Conditions.FightingStyleProtection().String():          {Name: "Protection"},
	refs.Conditions.FightingStyleTwoWeaponFighting().String():   {Name: "Two-Weapon Fighting"},

	// Barbarian.
	refs.Conditions.Raging().String():         {Name: "Raging"},
	refs.Conditions.RecklessAttack().String(): {Name: "Reckless Attack"},
	refs.Conditions.BrutalCritical().String(): {Name: "Brutal Critical"},

	// Fighter (champion).
	refs.Conditions.ImprovedCritical().String(): {Name: "Improved Critical"},

	// Monk.
	refs.Conditions.MartialArts().String():       {Name: "Martial Arts"},
	refs.Conditions.UnarmoredDefense().String():  {Name: "Unarmored Defense"},
	refs.Conditions.UnarmoredMovement().String(): {Name: "Unarmored Movement"},

	// Rogue. Sneak Attack names itself by a feature ref, not a condition ref.
	refs.Features.SneakAttack().String(): {Name: "Sneak Attack"},

	// Turn-based / combat-ability conditions.
	refs.Conditions.Dodging().String():           {Name: "Dodging"},
	refs.Conditions.Disengaging().String():       {Name: "Disengaging"},
	refs.Conditions.Hidden().String():            {Name: "Hidden"},
	refs.Conditions.Helped().String():            {Name: "Helped"},
	refs.Conditions.Prone().String():             {Name: "Prone"},
	refs.Conditions.OpportunityAttack().String(): {Name: "Opportunity Attack"},

	// Standard conditions reachable by the four builds.
	refs.Conditions.Unconscious().String(): {Name: "Unconscious"},
}
