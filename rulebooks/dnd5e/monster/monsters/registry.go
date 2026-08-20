// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// Constructor builds a monster's full stat bundle. It matches the signature
// shared by the existing NewSkeleton/NewGhoul/... constructors.
type Constructor = func(id string) *monster.Monster

// byRef maps each canonical monster ref string (refs.Monsters.*.String(),
// full "dnd5e:monsters:<id>" form) to its existing constructor. This is a
// lookup table over constructors that already exist — no new stat
// interpretation lives here (rpg-toolkit#842).
//
// Four canonical refs have no constructor today and are deliberately absent:
// skeleton-archer, giant-spider, giant-wolf-spider, bandit-captain.
var byRef = map[string]Constructor{
	refs.Monsters.Skeleton().String():        NewSkeleton,
	refs.Monsters.Zombie().String():          NewZombie,
	refs.Monsters.SkeletonCaptain().String(): NewSkeletonCaptain,
	refs.Monsters.Ghoul().String():           NewGhoul,
	refs.Monsters.GiantRat().String():        NewGiantRat,
	refs.Monsters.Wolf().String():            NewWolf,
	refs.Monsters.BrownBear().String():       NewBrownBear,
	refs.Monsters.Bandit().String():          NewBanditMelee,
	refs.Monsters.BanditArcher().String():    NewBanditRanged,
	refs.Monsters.Thug().String():            NewThug,
	refs.Monsters.Goblin().String():          NewGoblin,
}

// ByRef resolves a canonical monster ref to its constructor. The bool
// reports whether the ref is known — callers validate at author/load
// time so a bad ref fails a file, never a spawn.
func ByRef(ref string) (Constructor, bool) {
	c, ok := byRef[ref]
	return c, ok
}

// Refs returns every ref known to the registry, sorted. Callers (e.g.
// dungeonspec's validation) can use this to name the known set in an
// "unknown monster ref" error message.
func Refs() []string {
	known := make([]string, 0, len(byRef))
	for ref := range byRef {
		known = append(known, ref)
	}
	sort.Strings(known)
	return known
}
