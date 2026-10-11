// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
)

// fakeCast answers gamectx.Cast from a plain member -> side map, and hands out
// the member sheets a rule reads a participant through.
//
// Sides rather than a party/monster flag on purpose: the rules being tested
// here are the ones that were wrong precisely because they assumed two sides,
// and a fake that can only express two would keep them looking right. Give a
// member its own side and it is everyone's enemy; that is the three-faction
// dungeon these predicates have to survive.
//
// Member used to answer (nil, false) unconditionally, which was honest while
// nothing read a sheet off the cast. A condition reading ITSELF is a member
// lookup, so it has to serve now — and it serves out of the same map Members
// enumerates, so a test cannot accidentally describe a cast whose roster and
// whose sheets disagree.
type fakeCast struct {
	side    map[string]string
	members map[string]combat.Member
}

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

// castOf installs a cast holding these members, each on its own side, the way
// resolution's one door installs the real one.
//
// Its own side per member because a test about reading YOURSELF should not have
// to state an allegiance, and a shared default side would quietly make every
// member everyone's ally — which is the two-sided assumption fakeCast exists to
// avoid expressing by accident.
func castOf(ctx context.Context, members ...*fakeConditionOwner) context.Context {
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
