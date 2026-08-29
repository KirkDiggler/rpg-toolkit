// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"context"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
)

// fakeCast answers gamectx.Cast from a plain member -> side map, standing in
// for the implementation resolution installs (rpg-toolkit#1252).
//
// Sides rather than a party/monster flag, because the rules under test here
// are exactly the ones that used to decide allegiance from entity type.
//
// Member serves real sheets now. It answered (nil, false) while nothing read a
// participant's sheet off the cast; a condition reading ITSELF is a member
// lookup, so refusing every lookup would deny every rule its own sheet.
type fakeCast struct {
	side    map[string]string
	members map[string]combat.Combatant
}

func (f *fakeCast) Member(id string) (combat.Combatant, bool) {
	m, ok := f.members[id]
	if !ok || m == nil {
		return nil, false
	}

	return m, true
}

// castOf installs a cast holding these members, standing in for what
// resolution's one door does on every path that folds anything.
//
// A test in THIS module cannot reach that door — resolution imports this
// rulebook, not the other way round — so it hand-installs the same tenant the
// door installs. That is standing in for a real installer, not inventing one:
// the proof that production actually installs it is a level up, in session's
// TestAMonksUnarmoredDefenseReachesTheJoinedAC, which drives Join through
// resolution.ProjectCharacter and reads 15 off the wire.
//
// The distinction matters here more than most places. These very tests once
// built a gamectx registry by hand that PRODUCTION NEVER BUILT ANYWHERE, and
// so certified a monk's AC as correct while every real fight ran at base
// armour (rpg-toolkit#1251). A stand-in is legitimate exactly when the thing
// it stands in for exists and is pinned somewhere; that is the difference.
func castOf(ctx context.Context, members ...combat.Combatant) context.Context {
	cast := &fakeCast{
		side:    make(map[string]string, len(members)),
		members: make(map[string]combat.Combatant, len(members)),
	}
	for _, m := range members {
		cast.side[m.GetID()] = m.GetID()
		cast.members[m.GetID()] = m
	}

	return gamectx.WithCast(ctx, cast)
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
