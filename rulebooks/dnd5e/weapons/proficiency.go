// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package weapons

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"

// This mapping lives in weapons, and must stay here.
//
// It reads like proficiencies package material — it is, after all, about
// proficiency grants — but moving it there is an import cycle: shared imports
// proficiencies (shared/types.go), and weapons imports shared, so proficiencies
// can never import weapons or shared back. weapons already depends on
// proficiencies transitively through shared, so this direction is free.
//
// Stated here because "tidy this into the proficiencies package" is the
// obvious-looking refactor, and it does not compile.

// specificWeaponGrants maps each specific weapon proficiency to the weapon it
// names. Grants are plural nouns ("longswords") where weapon IDs are singular
// ("longsword"), so the two vocabularies need a table rather than a cast.
//
// A table rather than depluralization-by-trimming-an-s: every grant in the
// ruleset happens to be the weapon ID plus "s" today, but that is a
// coincidence of which weapons have class grants, not a rule — and a silent
// mismatch here reads as "not proficient", which costs a character their
// proficiency bonus without erroring. TestSpecificGrantsNameRealWeapons pins
// every entry against the catalog.
var specificWeaponGrants = map[proficiencies.Weapon]WeaponID{
	proficiencies.WeaponClub:          Club,
	proficiencies.WeaponDagger:        Dagger,
	proficiencies.WeaponDart:          Dart,
	proficiencies.WeaponJavelin:       Javelin,
	proficiencies.WeaponLightCrossbow: LightCrossbow,
	proficiencies.WeaponMace:          Mace,
	proficiencies.WeaponQuarterstaff:  Quarterstaff,
	proficiencies.WeaponShortbow:      Shortbow,
	proficiencies.WeaponSickle:        Sickle,
	proficiencies.WeaponSling:         Sling,
	proficiencies.WeaponSpear:         Spear,

	proficiencies.WeaponHandCrossbow: HandCrossbow,
	proficiencies.WeaponLongbow:      Longbow,
	proficiencies.WeaponLongsword:    Longsword,
	proficiencies.WeaponRapier:       Rapier,
	proficiencies.WeaponScimitar:     Scimitar,
	proficiencies.WeaponShortsword:   Shortsword,
}

// CoveredBy reports whether a single weapon-proficiency grant covers this
// weapon. A category grant ("simple"/"martial") covers every weapon of that
// category; a specific grant ("longswords") covers the one weapon it names.
//
// This is the first place in the toolkit that answers the question at all:
// the old attack path adds the proficiency bonus unconditionally, to every
// attacker with any weapon (rpg-toolkit#1005). Fixing that path is its own
// change — this one only supplies the rule it will need.
func (w Weapon) CoveredBy(grant proficiencies.Weapon) bool {
	switch grant {
	case proficiencies.WeaponSimple:
		return w.IsSimple()
	case proficiencies.WeaponMartial:
		return w.IsMartial()
	default:
		named, ok := specificWeaponGrants[grant]

		return ok && named == w.ID
	}
}
