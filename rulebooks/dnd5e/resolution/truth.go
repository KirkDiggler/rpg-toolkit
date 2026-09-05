// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// installTruth is THE DOOR: the one function allowed to call a gamectx.With*,
// and TestOnlyTheDoorInstallsGameContext says so by reading this package's
// source rather than by convention.
//
// ONE FUNCTION, because the defect it exists to prevent is a fact that is
// SOMETIMES installed. gamectx once carried five registries and a sixth in
// combat, and between them they were installed zero times: three conditions
// read one nobody supplied and returned its error into a chain fold that
// swallows errors, so a barbarian fought at base AC in every real fight and
// nothing was logged (rpg-toolkit#1251). Spreading installs across the call
// sites that happen to need them is how that state is reached — every site is
// individually reasonable, and the set of them is what nobody checks. Collected
// here, "did this path install the truth" is one question with one answer, and
// a path that skips this function is a bug rather than a mode.
//
// EVERYTHING IT INSTALLS IS DERIVED FROM WHAT THE INTERACTION ALREADY HOLDS.
// The room is the canvas the composition compiled, the cast is the participants
// attachAll loaded, the run is the composition itself as resolveOn reloaded it
// from Input.World, and readiness is a function of the cast. Nothing here
// reaches for a repository: a fact that needs one is a record the caller loads,
// not a tenant of this function. A new tenant is a design decision, not a
// pattern to follow — bring it to the design before writing it.
//
// DEPENDENCY ORDER IS THE DOCUMENTATION. The body is plain sequential code
// because the order is the whole story: readiness is derived from the cast, so
// it follows the cast, and nothing else here has an order to respect.
//
// It is called AFTER attachAll, because that is the call that loads the sheets
// and there is no cast before it. The world does not have to wait and used to
// not: nothing on the attach path reads game context, and nothing on it
// publishes. Effects subscribe during Apply and ask their questions at fold
// time, and a CHAIN handler is handed the PUBLISHER's context — events'
// chainedTopic carries the ctx it was published with — so the earliest any of
// this is read is the first fold, which is after every line below.
//
// The word CHAIN is load-bearing there. A plain typed subscription is the other
// way round: typedTopic.Subscribe closes over the context it was SUBSCRIBED
// with and simpleEventBus.Publish drops the publisher's, so a turn-start or
// damage-received handler sees whatever was installed when its sheet attached —
// which, from where this function is called, is nothing. Every gamectx reader in
// the rulebook today is a chain handler, and every typed handler that could
// reach one takes its context as `_`, so nothing is gated on that. It is still
// the sharp edge nearest this function: a reader added to a typed handler would
// fail closed and log nothing, which is rpg-toolkit#1251 said back to us.
func installTruth(ctx context.Context, room spatial.Room, cast *Participants, run *encounter.Encounter) context.Context {
	// INSTALL THE WORLD. One world, and it is installed every time.
	//
	// EVERY TIME is the half that bit. "Which room describes this interaction"
	// used to be a question, and the answer this package gave when it could not
	// decide — install nothing — silently switched off every predicate that
	// reads positions the moment one party member wandered off, which in a
	// dungeon is most of the time (rpg-toolkit#1090). There is one map, so
	// there is nothing to choose between, and no input can produce an
	// interaction without a world. TestNoCodePathProducesARoomlessInteraction
	// holds that structurally rather than by example.
	ctx = gamectx.WithRoom(ctx, room)

	// The other half of the read channel: the room says where everyone is
	// standing, and this says who they are to each other.
	//
	// EVERY TIME, for the same reason and with the same pin. Five registries in
	// gamectx and a sixth in combat tried to answer pieces of this and between
	// them were installed zero times, so three conditions read a registry
	// nobody supplied and returned its error into a chain fold that swallows
	// errors — a barbarian fought at base AC in every real fight and nothing
	// was logged (rpg-toolkit#1251). An ambient dependency that is SOMETIMES
	// present is the defect; being always present is the fix, and
	// TestNoCodePathProducesACastlessInteraction holds that structurally rather
	// than by example.
	//
	// The cast carries the run with it, because "who they are to each other"
	// is the run's to answer: the encounter's graph folds the dungeon's
	// factions and dispositions with the facts the run has learned, and the
	// cast asks it rather than keeping a table of sides (rpg-project#375,
	// design §4). The run is nil on the entries that have no world — the same
	// entries that install no room — and there a side question is unknown,
	// the absent value that says what the author meant.
	ctx = gamectx.WithCast(ctx, &castView{cast: cast, run: run})

	// The SIXTH registry in the family rpg-toolkit#1251 was about, installed
	// zero times until it was wired here.
	//
	// gamectx.IsReactionReady fails closed by design, so every reaction
	// condition in the rulebook has been gated behind a map nobody supplied —
	// the opportunity attack could not fire in any real interaction, and the
	// only reason its own suite is green is that each test installs a map by
	// hand. That is the same shape as the barbarian who fought at base AC in
	// every real fight, and it gets the same fix: ALWAYS PRESENT, derived
	// here, held structurally by TestNoCodePathProducesAReadinesslessInteraction
	// rather than by example.
	//
	// Free reactions are readied for everyone; costed ones are not readied at
	// all. That is the Wave 2.11d ruling reused rather than re-derived — "free-
	// cost reactions like OA are default-on for melee combatants, spell-cost
	// reactions like Shield are default-off" — and it is why Shield is absent
	// from the set below rather than present-and-false. Opting INTO a costed
	// reaction is a decision a player makes, and this package is not where a
	// player is asked anything.
	ctx = gamectx.WithReactionReadiness(ctx, defaultReadiness(cast))

	return ctx
}

// freeReactions are the reactions a member has readied simply by being in the
// interaction, because they cost nothing to hold ready.
//
// ONE ENTRY, and the list is the rule rather than an optimisation of it. A
// costed reaction — Shield burns a spell slot, Uncanny Dodge burns the class
// feature — is not readied by existing, so adding one here would silently opt
// every caster into spending a slot they never agreed to spend.
var freeReactions = []*core.Ref{
	refs.Conditions.OpportunityAttack(),
}

// defaultReadiness readies every participant for every free reaction.
//
// EVERY participant, not "every melee combatant". Whether a particular member
// can actually take an opportunity attack is the condition's own predicate —
// it is only seated on someone who has it, it checks its own reach, its own
// meter and its own purse — and deciding it out here would put a rule in the
// wiring, which is the same objection boundaryCast makes one file over.
func defaultReadiness(cast *Participants) gamectx.ReactionReadinessMap {
	ready := make(gamectx.ReactionReadinessMap, len(cast.order))
	for _, id := range cast.order {
		per := make(map[string]bool, len(freeReactions))
		for _, ref := range freeReactions {
			per[ref.String()] = true
		}
		ready[id] = per
	}

	return ready
}
