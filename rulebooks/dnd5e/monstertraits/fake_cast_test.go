// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
)

// fakeCast answers gamectx.Cast from a plain member -> side map, for a
// trait that reads a participant's sheet out of the cast (a monster's own
// opportunity attack reads itself that way).
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

// StanceBetween folds the side map the way IsHostile and IsAllied do; a
// member missing from the map has no stance.
func (f *fakeCast) StanceBetween(a, b string) (contributions.Stance, bool) {
	sa, oka := f.side[a]
	sb, okb := f.side[b]
	if !oka || !okb {
		return "", false
	}
	if sa == sb {
		return contributions.StanceAllied, true
	}
	return contributions.StanceHostile, true
}

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
