// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
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

// IsHostile answers whether b is an enemy of a.
//
// This body reads a relation table now — [castRelations] — instead of
// computing a guess. v1's TABLE still only has two sides, mutually hostile
// and each self-allied, so the ANSWER is unchanged: "one of you is a
// character and the other is a monster." What changed is the SHAPE. Allegiance
// is a directed relation between factions that quest events can change
// mid-run — the design case is a dungeon holding two monster factions hostile
// to each other and to the party, and a party that can shift one of them by
// returning something it wants (rpg-project#286,
// ideas/session-combat/effect-context/brainstorm.md) — and a relation table
// is what has room for that; a two-valued guess never did.
//
// conditions/sneak_attack.go used to make the same guess for itself, inline,
// and was wrong in both directions the moment a third faction existed. That
// this body can now change without touching sneak_attack.go at all is the
// whole reason [gamectx.Cast] exposes questions instead of fields.
func (v *castView) IsHostile(a, b string) (hostile, known bool) {
	sideA, ok := v.side(a)
	if !ok {
		return false, false
	}
	sideB, ok := v.side(b)
	if !ok {
		return false, false
	}

	return castRelations().HasEdge(sideEntity(sideA), hostileTo, sideEntity(sideB)), true
}

// IsAllied answers whether b is on a's side.
//
// Not the negation of [castView.IsHostile] — read literally from the table,
// not derived from it. Today the two sides are declared BOTH mutually hostile
// and each self-allied, so the numbers still agree; the moment a third side
// is declared merely hostile-to the other two, without an allied-with edge of
// its own, the agreement ends on its own, in data, with no rule to update.
// Pack Tactics wants allies, Sneak Attack wants the target's enemies, and each
// asks the table for what it means.
func (v *castView) IsAllied(a, b string) (allied, known bool) {
	sideA, ok := v.side(a)
	if !ok {
		return false, false
	}
	sideB, ok := v.side(b)
	if !ok {
		return false, false
	}

	return castRelations().HasEdge(sideEntity(sideA), alliedWith, sideEntity(sideB)), true
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

// The relation table's two entities, and the two relations declared over
// them. Not participant ids — those never enter the table at all, because
// what the table answers depends on KIND, not on who happens to be standing
// here today. See [castRelations] for why that is v1's honest shape rather
// than a shortcut.
const (
	characterSide journal.EntityID = "resolution:cast-side:character"
	monsterSide   journal.EntityID = "resolution:cast-side:monster"

	// membershipRelation satisfies graph.Config.Membership, which every
	// declaration must name one of (graph.ErrNoMembership). Nothing in this
	// table ever uses it — two sides with no sub-groups have nothing to
	// belong to — so it is declared and never spent, the same as a scenario
	// with no slots still names a Role type.
	membershipRelation graph.Relation = "belongs-to"

	hostileTo  graph.Relation = "hostile-to"
	alliedWith graph.Relation = "allied-with"
)

// sideEntity names the relation-table entity a [castSide] stands for.
func sideEntity(s castSide) journal.EntityID {
	if s == sideMonster {
		return monsterSide
	}

	return characterSide
}

// castRelations is v1's relation table: two sides, mutually hostile, each
// self-allied — exactly today's guess, read as data instead of computed.
//
// It is a fixed, cast-independent declaration on purpose. What [IsHostile]
// and [IsAllied] answer today depends only on KIND — character or monster —
// never on which specific participants are in this interaction's cast, so a
// table built once and shared by every castView is the honest shape for that:
// rebuilding it per interaction would suggest the cast changes what it says,
// and it does not, yet. Rung 2 (rpg-project's living-world integration study)
// is where a stance WRITER — a quest event, a betrayal — earns a table that
// does depend on the specific cast; nothing about that rung requires this
// one's shape to change first.
//
// Built once, lazily, and memoized: every castView reads the same table
// rather than each paying to build it. The declaration is fixed and
// hand-verified valid — see TestCastRelationsBuilds — so a [graph.New] error
// here is not a caller mistake to recover from; it would mean this literal
// declaration is wrong, which panics loudly rather than silently answering
// every question "unknown" for the rest of the process. See doc.go's own
// stance on ambient dependencies that fail without being loud about it.
var castRelations = sync.OnceValue(func() *graph.State {
	w, err := graph.New(graph.Config{
		Membership: membershipRelation,
		Entities: []graph.Entity{
			{ID: characterSide, Kind: "cast-side"},
			{ID: monsterSide, Kind: "cast-side"},
		},
		Edges: []graph.Edge{
			{From: characterSide, Rel: hostileTo, To: monsterSide},
			{From: monsterSide, Rel: hostileTo, To: characterSide},
			{From: characterSide, Rel: alliedWith, To: characterSide},
			{From: monsterSide, Rel: alliedWith, To: monsterSide},
		},
	})
	if err != nil {
		panic(fmt.Sprintf("resolution: the cast relation table's own declaration is invalid: %v", err))
	}

	// No facts exist yet — rung 1 is a read with nothing written. Truth over
	// an empty journal is exactly the declared table, unconditionally, for
	// every observer; that stops being true only once rung 2 gives some fact
	// a reason to be witnessed unevenly, which is a question for that rung.
	return w.Truth(journal.New())
})
