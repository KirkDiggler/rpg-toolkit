// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

// DefaultSightFeet is the rulebook's stated sight range for a sheet that
// states none: 120 feet, 24 cells — normal vision on a lit dungeon floor
// (rpg-project#254 design §5, the owner's ruling), this build's answer until
// a light model exists to do better (R10).
//
// ONE NUMBER, BOTH KINDS. Silence means the same thing whoever is silent: a
// character whose race table states no range, and a monster whose stat block
// authors no darkvision, both see this far. A narrower monster-only floor
// was tried and retired (rpg-project#254 review) — it read as a fact about
// darkness this build does not model, when it was really standing in for a
// stat block nobody had written senses for. A genuinely narrower number
// belongs to the light model, read off content, never guessed at by kind.
const DefaultSightFeet = 120

// SightHolder answers the named sight question: how far a member can see, in
// feet, read off its own sheet at the moment it is asked. Both sheet kinds
// answer it — a character from its race table, a monster from its stat
// block — so the session can hold either behind this one interface when it
// answers the encounter's Sight capability.
//
// The answer is always the rulebook's number, stated: a sheet that states no
// range answers [DefaultSightFeet] itself, so zero is never a hidden default
// for a caller to fill in, and no session constant stands in for a missing
// row. It is range alone; line of sight and light are not part of it.
type SightHolder interface {
	// SightFeet returns how far the member can see, in feet; never zero.
	SightFeet() int
}
