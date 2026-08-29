// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
)

// fakeCast answers gamectx.Cast from a plain member -> side map.
//
// Sides rather than a monster/party flag: Pack Tactics belongs to a monster,
// and the case it has to get right is a dungeon holding two monster factions
// that hate each other as much as they hate the party. A fake that could only
// express "monsters" and "characters" would hide exactly that.
type fakeCast struct {
	side    map[string]string
	members map[string]combat.Member
}

// Member serves the sheets a test put in the cast, and answers "cannot tell"
// for anyone it was not given. It returned (nil, false) unconditionally while
// nothing here read a participant's sheet; a monster's own opportunity attack
// reads ITSELF that way, so refusing every lookup would deny a wolf the
// reaction it is entitled to.
func (f *fakeCast) Member(id string) (combat.Member, bool) {
	m, ok := f.members[id]
	if !ok || m == nil {
		return nil, false
	}

	return m, true
}

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

// castOf installs a cast holding these sheets, the way resolution's one door
// installs the real one on every path that folds anything.
func castOf(ctx context.Context, members ...combat.Member) context.Context {
	cast := &fakeCast{
		side:    make(map[string]string, len(members)),
		members: make(map[string]combat.Member, len(members)),
	}
	for _, m := range members {
		cast.side[m.GetID()] = m.GetID()
		cast.members[m.GetID()] = m
	}

	return gamectx.WithCast(ctx, cast)
}
