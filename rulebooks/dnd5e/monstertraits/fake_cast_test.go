// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// fakeCast answers gamectx.Cast from a plain member -> side map.
//
// Sides rather than a monster/party flag: Pack Tactics belongs to a monster,
// and the case it has to get right is a dungeon holding two monster factions
// that hate each other as much as they hate the party. A fake that could only
// express "monsters" and "characters" would hide exactly that.
type fakeCast struct {
	side map[string]string
}

func (f *fakeCast) Member(string) (combat.Member, bool) { return nil, false }

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

// fakeEntity is a placeable body with an ID.
type fakeEntity struct {
	id string
}

func (f *fakeEntity) GetID() string            { return f.id }
func (f *fakeEntity) GetType() core.EntityType { return "monster" }
