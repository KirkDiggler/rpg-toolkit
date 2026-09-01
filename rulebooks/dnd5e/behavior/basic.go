// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package behavior holds encounter.TurnDriver implementations for ANY
// member nobody is playing right now (Kirk, rpg-project#254 review) —
// monsters today, but the seam is not monster-shaped: a disconnected
// player auto-passing until they reconnect, an allied NPC, a summoned
// creature tomorrow all land on the identical question a fight's clock
// asks when it reaches an unplayed member's turn, answered from the same
// shared member record (encounter.MemberInput's SpeedFeet, SightFeet,
// Actions, Targeting — filled for every kind, not just monsters) through
// the same encounter.MonsterView. A driver for one of those cases is
// another type in this package, not a different seam.
//
// Basic is the first of them, and the Monster AI owner's starting point: a
// real, working driver, small enough to read in one sitting and heavily
// documented as exactly that — a foundation to extend, not a finished
// decision system. The richer design (mode machines, disposition,
// stimuli-and-memory perception) lives in rpg-project's own
// ideas/monster-ai/design.md and is deliberately NOT built here; this
// package only has to prove the seam works end to end.
package behavior

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Basic is the simplest TurnDriver that does something: attack the closest
// standing player if one is in reach, otherwise close the distance; when no
// standing player is visible, close on the closest reachable remembered
// player, otherwise pass. It never learns what a monster IS, and never touches a live
// *encounter.Encounter — everything it reads comes off
// [encounter.MonsterView], which stays plain data end to end (perception is
// data; the decision layer never reaches live state).
type Basic struct{}

// Act implements [encounter.TurnDriver].
func (Basic) Act(view encounter.MonsterView) (encounter.TurnIntent, error) {
	target, ok := closest(view.Seen)
	if ok {
		if view.Budget.AttacksLeft > 0 {
			for _, action := range view.Actions {
				if target.InReach[action.Ref] {
					return encounter.Attack{Target: target.ID, Action: action.Ref}, nil
				}
			}
		}

		// No separate "already in reach, don't bother moving" check needed
		// here: [SeenMember.Path] already ends wherever InReach would turn
		// true (rpg-project#254 review), so a target within reach — whether
		// or not this member still has an attack to spend on it — simply has
		// an empty Path, and this falls through to Pass on its own.
		if view.Budget.MovementFeet > 0 && len(target.Path) > 0 {
			return encounter.Move{Path: target.Path[:1]}, nil
		}

		return encounter.Pass{}, nil
	}

	remembered, ok := closestRemembered(view.Remembered)
	if !ok || view.Budget.MovementFeet <= 0 {
		return encounter.Pass{}, nil
	}
	return encounter.Move{Path: remembered.Path[:1]}, nil
}

// closest picks the nearest standing player among seen — v1's whole answer
// to Targeting (view.Targeting is read by nobody here). "lowest-health" and
// "lowest-ac" are real strategies the rulebook can author, but MonsterView
// carries neither HP nor AC for a sighted member (a stimuli-and-memory
// percept, not a stat block leak) — so every strategy this view's data can
// actually support collapses to "closest", and honestly falling back beats
// silently mis-targeting. A future view that carries more can make this
// switch on view.Targeting for real.
//
// Ties break on ID (C8-style determinism): the same Seen, asked twice,
// must choose the same target both times regardless of slice order.
func closest(seen []encounter.SeenMember) (encounter.SeenMember, bool) {
	var best encounter.SeenMember
	found := false
	for _, sm := range seen {
		if sm.Kind != encounter.KindPlayer || !sm.Standing {
			continue
		}
		if !found || sm.DistanceCells < best.DistanceCells ||
			(sm.DistanceCells == best.DistanceCells && sm.ID < best.ID) {
			best = sm
			found = true
		}
	}
	return best, found
}

// closestRemembered picks the nearest reachable remembered player. Remembered
// knowledge has no standing or reach facts, so it can only produce movement,
// never an attack.
func closestRemembered(remembered []encounter.RememberedMember) (encounter.RememberedMember, bool) {
	var best encounter.RememberedMember
	found := false
	for _, rm := range remembered {
		if rm.Kind != encounter.KindPlayer || len(rm.Path) == 0 {
			continue
		}
		if !found || rm.DistanceCells < best.DistanceCells ||
			(rm.DistanceCells == best.DistanceCells && rm.ID < best.ID) {
			best = rm
			found = true
		}
	}
	return best, found
}
