// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
)

// castView answers [gamectx.Cast] over the participants this interaction
// loaded.
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
type castView struct {
	cast *Participants
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
// is that an effect holding one can no longer call ApplyDamage or MarkClean on
// a sheet it does not own: the read law and the write law now differ by a type
// rather than by discipline (rpg-toolkit#1300).
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

// IsHostile answers whether b is an enemy of a.
//
// v1 answers "one of you is a character and the other is a monster", and that
// is a LIE with a known expiry. Allegiance is a directed relation between
// factions that quest events can change mid-run — the design case is a dungeon
// holding two monster factions hostile to each other and to the party, and a
// party that can shift one of them by returning something it wants
// (rpg-project#286, ideas/session-combat/effect-context/brainstorm.md).
//
// The lie is confined HERE, to one function, on purpose. That is the whole
// reason [gamectx.Cast] exposes questions instead of fields: when stance
// arrives, this body reads a relation table and not one rule changes.
// conditions/sneak_attack.go used to make the same guess for itself, inline,
// and was wrong in both directions the moment a third faction existed.
func (v *castView) IsHostile(a, b string) (hostile, known bool) {
	sideA, ok := v.side(a)
	if !ok {
		return false, false
	}
	sideB, ok := v.side(b)
	if !ok {
		return false, false
	}

	return sideA != sideB, true
}

// IsAllied answers whether b is on a's side.
//
// Not the negation of [castView.IsHostile], even though today it computes as
// one. Both derive from the same two-valued guess, so they are momentarily
// complements; the moment a faction can be NEUTRAL toward another they stop
// being, and a rule written against the wrong one starts counting bystanders.
// Pack Tactics wants allies, Sneak Attack wants the target's enemies, and each
// asks for what it means.
func (v *castView) IsAllied(a, b string) (allied, known bool) {
	sideA, ok := v.side(a)
	if !ok {
		return false, false
	}
	sideB, ok := v.side(b)
	if !ok {
		return false, false
	}

	return sideA == sideB, true
}

// castSide is v1's stand-in for allegiance: which map a participant loaded
// into.
type castSide int

const (
	sideCharacter castSide = iota
	sideMonster
)

// side reports which side a participant is on, and whether it is in this
// interaction at all.
//
// Not-in-the-cast is the "cannot answer" case rather than a default side. An
// effect asking about somebody who is not here has to be able to tell that
// apart from an answer, or it invents a rule out of missing data — the
// distinction conditions/prone.go draws between "not within reach" and "nobody
// knows where these two are standing".
func (v *castView) side(id string) (castSide, bool) {
	if _, ok := v.cast.Character(id); ok {
		return sideCharacter, true
	}
	if _, ok := v.cast.Monster(id); ok {
		return sideMonster, true
	}

	return 0, false
}
