// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// Cast is what an effect may ask about the OTHER participants in the
// interaction it is resolving.
//
// Resolution will install it on every path, exactly like the room, and it will
// die with that call. That wiring is rpg-toolkit#1252 and is NOT in the tree
// yet: there is no WithCast call site today, so CastOf currently returns
// ok=false everywhere and every consumer takes its "cannot answer" branch.
//
// Two registries answered pieces of this question before and neither was ever
// installed either — gamectx.CharacterRegistry (character-shaped, so it could
// not serve a monster at all) and combat.CombatantLookup (a second mechanism
// in a second package, with its own context key). Both are gone; this is the
// one seam.
//
// # Questions, not fields
//
// Every method here is a QUESTION. None of them hands back a field for a rule
// to reason over, and that is deliberate rather than stylistic.
//
// The rule this exists to stop writing is conditions/sneak_attack.go's, which
// decided allyness by testing whether the other entity was a "character" —
// baking a two-sided world into a predicate. When allegiance arrives it will
// be a directed relation between factions that quest events can change
// mid-run, and every rule that compared identifiers itself would have to be
// found and rewritten. Rules that asked IsHostile will not change at all.
//
// See ideas/session-combat/effect-context/brainstorm.md in rpg-project.
//
// # known, not error
//
// Each answer carries whether it could be answered at all. "b is not an enemy
// of a" and "nobody can tell who these two are to each other" are different
// facts, and collapsing them invents a rule out of missing data — the same
// distinction conditions/prone.go:284 draws between "not within reach" and
// "nobody knows where these two are standing".
//
// An effect that cannot answer its question must leave the chain unchanged. It
// must NOT return an error: Character.EffectiveAC swallows fold errors, so an
// erroring condition silently drops every OTHER contributor to that AC along
// with its own. That is how a barbarian ended up fighting at 10+DEX with
// Unarmored Defense attached and nothing logged.
type Cast interface {
	// Member returns a participant's combat-facing sheet.
	//
	// combat.Combatant rather than a concrete type: a monster and a character
	// are the same thing at this seam, and a registry shaped around one of
	// them is what the deleted CharacterRegistry was.
	Member(id string) (combat.Combatant, bool)

	// Members returns every participant in the interaction.
	//
	// Deterministic order, because two resolutions over identical data must
	// produce identical registration lists or a suspension cannot be resumed
	// into the world it left (resolution's R4).
	Members() []string

	// IsHostile answers whether b is an enemy of a, right now.
	//
	// Directed on purpose. Charm is one-sided — a charmed creature treats its
	// charmer as a friend while the charmer is under no such restriction — and
	// provoked infighting starts one-sided too. A symmetric answer would have
	// to be broken open the first time either lands.
	//
	// known is false when the question cannot be answered at all.
	IsHostile(a, b string) (hostile, known bool)

	// IsAllied answers whether b is on a's side, right now.
	//
	// NOT the negation of IsHostile, and the difference is the whole reason
	// both exist. Once a third faction can be neutral toward you, "not my
	// enemy" and "my ally" stop being the same set, and a rule that assumed
	// they were would silently start counting bystanders. Sneak Attack asks
	// the first question ("another enemy OF THE TARGET is within 5 feet of
	// it"); Pack Tactics asks the second ("one of the attacker's ALLIES").
	//
	// Both are answered as "same MemberKind" today, so they are momentarily
	// each other's complement. They are not the same question, and the rules
	// must not be written as though they are.
	//
	// known is false when the question cannot be answered at all.
	IsAllied(a, b string) (allied, known bool)
}

// castContextKey is the key type for storing a Cast in context.Context.
type castContextKey struct{}

// WithCast wraps a context.Context with the cast of the interaction being
// resolved.
//
// Resolution is intended to call this unconditionally, beside the room, so
// that there is no "no cast" mode — an ambient dependency that is sometimes
// absent fails silently, which is the defect this seam exists to replace.
//
// That call site does not exist yet (rpg-toolkit#1252). Until it lands, read
// CastOf's second return exactly as documented rather than assuming a cast is
// present.
func WithCast(ctx context.Context, c Cast) context.Context {
	return context.WithValue(ctx, castContextKey{}, c)
}

// CastOf retrieves the Cast from the context, and whether one was there.
//
// Read it defensively. Nothing installs a cast yet (rpg-toolkit#1252), and
// even once resolution does, an effect can be attached by something other than
// a resolution — session's standing and preflight paths both do. "I could not
// ask" has to stay expressible rather than becoming a panic or a rule invented
// out of missing data.
func CastOf(ctx context.Context) (Cast, bool) {
	if c, ok := ctx.Value(castContextKey{}).(Cast); ok && c != nil {
		return c, true
	}
	return nil, false
}
