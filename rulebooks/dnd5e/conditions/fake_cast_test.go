// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// fakeCast answers gamectx.Cast from a plain member -> side map.
//
// Sides rather than a party/monster flag on purpose: the rules being tested
// here are the ones that were wrong precisely because they assumed two sides,
// and a fake that can only express two would keep them looking right. Give a
// member its own side and it is everyone's enemy; that is the three-faction
// dungeon these predicates have to survive.
type fakeCast struct {
	side map[string]string
}

func (f *fakeCast) Member(string) (combat.Combatant, bool) { return nil, false }

func (f *fakeCast) Members() []string {
	ids := make([]string, 0, len(f.side))
	for id := range f.side {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (f *fakeCast) IsHostile(a, b string) (hostile, known bool) {
	sa, oka := f.side[a]
	sb, okb := f.side[b]
	if !oka || !okb {
		return false, false
	}
	return sa != sb, true
}

func (f *fakeCast) IsAllied(a, b string) (allied, known bool) {
	sa, oka := f.side[a]
	sb, okb := f.side[b]
	if !oka || !okb {
		return false, false
	}
	return sa == sb, true
}
