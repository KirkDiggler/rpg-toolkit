// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
)

// castView answers [gamectx.Cast] over the participants this interaction
// loaded, in the run it loaded them into.
//
// This is the only implementation there is, and this is the only package that
// could hold it: an effect's questions are about the OTHER participants, and
// R3 — pass everyone in — means resolution is the one place that has them all.
// The registries this replaces lived in gamectx and in combat, one import
// level below the data they described, which is why neither was ever installed
// by anything (rpg-toolkit#1251).
//
// It is a view, not a copy. The sheets are the same objects the machine is
// about to run against, so an effect asking about a participant mid-fold sees
// that participant as it is right now rather than as it was when the
// interaction opened. That matters for the questions worth asking — "is my
// target still up", eventually "is my target still poisoned" — and it costs
// nothing, because both die with the call.
//
// # Two sources, one view
//
// WHO IS HERE comes from the cast: the sheets attachAll loaded. WHO THEY ARE
// TO EACH OTHER comes from the run: the encounter resolveOn reloaded from
// Input.World, whose graph folds the factions the dungeon declared, the
// disposition between each pair, and the facts the run has learned since
// (rpg-project#375, the hold-out design §4). This view keeps no table of its
// own. The law that slice adopted is that the run's world is the only state
// and no reader keeps a copy, and the fixed two-side table that used to live
// in this file — character and monster, mutually hostile, declared once and
// shared by every interaction (rpg-toolkit#1352) — was the copy it named and
// deleted.
type castView struct {
	cast *Participants

	// run is the encounter this interaction is happening in, or nil on the
	// entries that have no world — a join, a check, a rest, a death save, a
	// participation refresh. A side question with no run to fold over is
	// unknown, never a guess from the kind of sheet somebody carries.
	run *encounter.Encounter
}

var _ gamectx.Cast = (*castView)(nil)

// Member returns a participant's combat-facing READ surface.
//
// Characters and monsters are the same thing here, which is the point:
// [gamectx.CharacterRegistry] could name weapons and ability scores and action
// economy and could not describe a wolf at all, so every predicate that wanted
// to ask about "whoever is standing there" had nothing to ask.
//
// # combat.Member, and the sheets behind it are still live
//
// The narrowing is the whole point of the type and changes nothing about what
// is behind it. These ARE the objects the machine is about to run against — the
// paragraph above says a view, not a copy, and it still means it. What changes
// is that an effect holding one can no longer call ApplyDamage on a sheet it
// does not own: the read law and the write law now differ by a type rather
// than by discipline (rpg-toolkit#1300). MarkClean used to be named here too
// and is gone entirely — it was on combat.Combatant with no caller anywhere,
// and rpg-project#319 Phase 6 deleted it.
//
// This package keeps the writer surface, and keeps it deliberately. Resolution
// IS the keeper — it applies the damage and builds the dirty set — so
// [Participants] hands out combatants internally and hands out members here,
// at the seam where rules read.
func (v *castView) Member(id string) (combat.Member, bool) {
	if ch, ok := v.cast.Character(id); ok {
		return ch, true
	}
	if m, ok := v.cast.Monster(id); ok {
		return m, true
	}

	return nil, false
}

// Members returns every participant, in attach order.
//
// Attach order is sorted order (R4) — resolution attaches in sorted participant
// order precisely so two resolutions over identical data produce identical
// registration lists, which is what makes a suspension resumable into the world
// it left. Handing that same order out here means an effect that iterates the
// cast is deterministic for free rather than by remembering to sort.
func (v *castView) Members() []string {
	return v.cast.IDs()
}

// IsHostile answers whether b is an enemy of a: a hostile-to edge between
// their factions in the run's graph, right now.
//
// One call, and the run owns the answer. Until rpg-project#375 this body read
// a table of two sides declared in this package, so what it said depended on
// KIND — character or monster — and on nothing the run had done. Now a
// dungeon declares factions, a member belongs to one, a hostile disposition
// can end on a fact the faction's mind comes to know, and the encounter folds
// all of that in one place ([encounter.Encounter.IsHostile]). The raider
// beside you is your enemy until the chief reads the letter, and Sneak Attack
// asks the question it always asked and gets the run's answer.
//
// known is false when there is no run to ask, or when either id is not a
// member of it: an effect asking about somebody who is not here has to be
// able to tell that apart from an answer, or it invents a rule out of missing
// data — the distinction conditions/prone.go draws between "not within reach"
// and "nobody knows where these two are standing". A prop standing beside the
// target is unknown, not "not hostile".
func (v *castView) IsHostile(a, b string) (hostile, known bool) {
	if v.run == nil {
		return false, false
	}

	return v.run.IsHostile(encounter.MemberID(a), encounter.MemberID(b))
}

// IsAllied answers whether b is on a's side: an allied-with edge between
// their factions, which every faction has with itself and which a
// disposition can declare between two.
//
// Not the negation of [castView.IsHostile], and now visibly so. A pair a fact
// has turned is NEUTRAL — neither edge stands — so once the camp turns, a
// raider is no longer an enemy for Sneak Attack and still not an ally for
// Pack Tactics, while two raiders stay each other's allies before and after.
// Each rule asks for what it means and the graph answers each literally.
func (v *castView) IsAllied(a, b string) (allied, known bool) {
	if v.run == nil {
		return false, false
	}

	return v.run.IsAllied(encounter.MemberID(a), encounter.MemberID(b))
}
