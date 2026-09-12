// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import "github.com/KirkDiggler/rpg-toolkit/core"

// CreatureType returns the sheet's creature family, distinct from its entity
// kind (monster). Unknown custom sheets return an empty type.
func (m *Monster) CreatureType() string { return creatureTypeFor(m.creatureType, m.ref) }

// creatureTypeFor supplies catalogue facts for older persisted sheets. Explicit
// types support custom monsters; unknown refs are never guessed to be humanoid.
func creatureTypeFor(explicit string, ref *core.Ref) string {
	if explicit != "" {
		return explicit
	}
	if ref == nil || ref.Module != "dnd5e" || ref.Type != "monsters" {
		return ""
	}
	return catalogueCreatureTypes[ref.ID]
}

var catalogueCreatureTypes = map[string]string{
	"skeleton": "undead", "skeleton-captain": "undead", "skeleton-archer": "undead", "zombie": "undead", "ghoul": "undead",
	"animated-armor": "construct",
	"brown-bear":     "beast", "giant-rat": "beast", "wolf": "beast", "giant-spider": "beast", "giant-wolf-spider": "beast",
	"bandit": "humanoid", "bandit-archer": "humanoid", "bandit-captain": "humanoid", "thug": "humanoid", "goblin": "humanoid",
}
