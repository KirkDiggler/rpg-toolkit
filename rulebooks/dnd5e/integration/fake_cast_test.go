// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// fakeCast answers gamectx.Cast from a plain member -> side map, standing in
// for the implementation resolution installs (rpg-toolkit#1252).
//
// Sides rather than a party/monster flag, because the rules under test here
// are exactly the ones that used to decide allegiance from entity type.
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
