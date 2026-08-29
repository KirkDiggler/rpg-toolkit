// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
)

// fakeCast answers gamectx.Cast over the sheets a test loaded, standing in for
// the implementation resolution installs.
//
// A test in THIS package cannot reach that installer — resolution imports this
// package, not the other way round — so it installs the same tenant by hand.
// That is standing in for a real installer rather than inventing one, and the
// difference is the whole reason this comment exists: these packages once built
// a gamectx registry by hand that PRODUCTION NEVER BUILT ANYWHERE, and so
// certified a monk's armour class as correct while every real fight ran at base
// armour (rpg-toolkit#1251). The stand-in is legitimate because the thing it
// stands in for exists and is pinned elsewhere — session's
// TestAMonksUnarmoredDefenseReachesTheJoinedAC drives Join through
// resolution.ProjectCharacter and reads 15 off the wire.
type fakeCast struct {
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
	ids := make([]string, 0, len(f.members))
	for id := range f.members {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	return ids
}

// IsHostile and IsAllied answer "cannot tell" for every pair. Nothing in this
// package asks — allegiance predicates live on rules in conditions — and
// answering a question this fake has no basis for would be inventing one.
func (f *fakeCast) IsHostile(_, _ string) (hostile, known bool) { return false, false }
func (f *fakeCast) IsAllied(_, _ string) (allied, known bool)   { return false, false }

// castOf installs a cast holding these sheets, the way resolution's one door
// installs the real one.
func castOf(ctx context.Context, members ...combat.Member) context.Context {
	cast := &fakeCast{members: make(map[string]combat.Member, len(members))}
	for _, m := range members {
		cast.members[m.GetID()] = m
	}

	return gamectx.WithCast(ctx, cast)
}

// Ensure the fake really answers the seam resolution installs.
var _ gamectx.Cast = (*fakeCast)(nil)
