// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// compelledDriver answers for a member under a compulsion and hands every
// other turn to the host's driver untouched.
//
// # Compulsion first, brain second
//
// A commanded creature's turn is not its own, so the thing that would normally
// decide it is never asked — which is the whole reason this wrapper exists
// rather than a flag on the view. For a PLAYER there is no brain to ask at
// all, and that is the point: [encounter.TurnParticipationDriven] keeps the
// slot and hands the turn here, so a compelled player is driven by exactly the
// machinery a compelled monster is.
//
// # It decides nothing about the word
//
// The word is read off the sheet and handed to [resolution.Obey], which says
// what the order means; the effects that come back are translated into one
// intent and the composition walks it. This seam contributes the LOOKUP and
// the translation, which is what this package is for — no distance, no policy
// choice, no decision about when the turn ends.
//
// # Bound to the write scope, and only built where one exists
//
// A struct holding the scope rather than a closure over one, for the reason
// [strikerSeam] gives: the capability has to be handed to the very encounter
// it later drives, so the scope is allocated first and read at call time. The
// obeyed word can apply a condition — Grovel leaves the creature prone — and
// that has to reach disk, which is why a driver with no scope to save through
// would be a promise this type cannot keep.
type compelledDriver struct {
	// ctx is the verb's own, captured because the composition calls Act with
	// none to give — [standingSeam]'s reason, one file over.
	ctx context.Context

	m     *Manager
	scope *writeScope

	// next is the host's driver, reached for every member holding no
	// compulsion. It is the seam, not the host's own value: a monster's brain
	// is consulted through exactly the boundary it was consulted through
	// before this wrapper existed.
	next encounter.TurnDriver
}

// compile-time proof the wrapper satisfies what it is handed to.
var _ encounter.TurnDriver = compelledDriver{}

// compelledDriverFor wraps the host's driver for one write verb.
func (m *Manager) compelledDriverFor(ctx context.Context, scope *writeScope) compelledDriver {
	return compelledDriver{ctx: ctx, m: m, scope: scope, next: m.turnDriver}
}

// Act takes the turn for a compelled member and delegates every other.
//
// THE DELEGATION IS THE COMMON CASE and it is unconditional: a member with no
// compulsion reaches the host's driver having been neither delayed nor
// rewritten. What a lookup costs on every driven turn is one read of records
// this verb is already holding.
func (d compelledDriver) Act(view encounter.MonsterView) (encounter.TurnIntent, error) {
	commanded, held, err := d.compulsionOn(view.Self)
	if err != nil {
		return nil, err
	}
	if !held {
		return d.next.Act(view)
	}

	out, err := d.obey(view.Self, commanded)
	if err != nil {
		return nil, err
	}
	return compelledIntent(view.Self, out.Effects)
}

// compulsionOn reads the member's stored conditions and decodes the compulsion
// if one is there.
func (d compelledDriver) compulsionOn(
	member encounter.MemberID,
) (*conditions.CommandedConditionData, bool, error) {
	characters, monsters, err := d.scope.standing.recordsFor([]encounter.MemberID{member})
	if err != nil {
		return nil, false, fmt.Errorf("compelled turn %q: %w", member, err)
	}
	data, held := commandedIn(storedConditionsOf(characters, monsters)[string(member)])
	return data, held, nil
}

// commandedIn finds the compulsion among a sheet's stored blobs, asking
// [conditions.DecodeCommanded] ONE BLOB AT A TIME and keeping the FIRST one
// that answers.
//
// # The first applied wins, and a sheet may really hold two
//
// The caster is part of the compulsion's identity, so two casters' Commands
// stand side by side: nothing removes another caster's spell, and each ends on
// its own clock. One turn cannot obey two words, and the one it obeys is the
// one that got there FIRST — Kirk's ruling, 2026-09-12, on the precedent the
// rulebook already sets for a die: DescribeSelectedRollContributions walks the
// persisted order and takes the OLDEST applicable provider in a stacking group,
// which is why a second Bane contributes nothing while the first stands. A turn
// is that question asked of a turn instead of a roll, and answering it the
// other way would give the engine two rules for "who wins when two spells
// overlap" depending on what was overlapping.
//
// A second Command from the SAME caster never reaches this decision: resolution
// replaces an existing instance of one address before applying the new one, so
// the sheet holds one.
//
// # The order kept here is this function's own
//
// [conditions.DecodeCommanded] answers the same way for a whole list, so the
// two agree today and neither is deriving its answer from the other. That is
// worth one sentence rather than a claim of independence: the ruling moved once
// while this branch was open, and because each call here answers about a SINGLE
// blob, moving with it was one line — which match the loop keeps — rather than
// a question about what the reader would do with the rest.
//
// # Why one at a time, stated honestly
//
// DecodeCommanded reports an unreadable blob anywhere on the sheet, deliberately
// — its own doc says a sheet whose conditions cannot all be read is a sheet
// nobody should be deciding turns from. Asked per blob, an unreadable one costs
// exactly itself, so a corrupt entry cannot hide a real compulsion. Hiding one
// here is worse than it sounds, because participation has already answered
// Driven: the driver would find nothing, delegate, and a commanded PLAYER's turn
// would be taken by the host's monster brain.
//
// That scenario is NOT reachable through the verbs today, and this doc said
// otherwise before a reviewer checked. Resolution's attach refuses a record
// carrying an unreadable condition one seam earlier — outright for a monster,
// and after the lenient loader drops it for a character — so a sheet in this
// state never reaches a compelled turn at all.
//
// So the leniency is defence in depth, and it is also a DIVERGENCE from the
// provider's stated judgement, taken on purpose: this module already answers an
// unreadable sheet by refusing that member's offers rather than the whole read,
// and [holdsCompulsion] one file over must stay lenient for two tested rulings
// that predate Command. The two readers have to agree — a member called Driven
// whose driver found nothing is a turn nobody takes — so they are lenient
// together or not at all.
func commandedIn(stored []json.RawMessage) (*conditions.CommandedConditionData, bool) {
	for _, raw := range stored {
		if data, held, err := conditions.DecodeCommanded([]json.RawMessage{raw}); err == nil && held {
			return data, true
		}
	}
	return nil, false
}

// obey asks resolution what the word means, on an interaction built exactly as
// the announcer builds one, and saves whatever it wrote.
//
// # Why the whole interaction
//
// One of the three words applies a condition, and a condition is applied by
// publishing on a bus only resolution may create (ADR-0038). So Obey is handed
// the cast, the world and the seams rather than a thinner bag of sheets, and
// supplies the machine itself. The cast is the WHOLE fight, by
// [announcerSeam.boundaryCast]'s rule: which effects apply is each effect's own
// question, and choosing who to load out here would answer it in the wiring.
//
// NO TEST STANDS BEHIND THAT LAST SENTENCE TODAY, and a reviewer proved it:
// narrowing the cast to the compelled member alone leaves the package green,
// because none of the three words reaches a listener on anybody else's sheet.
// It is the rule this layer should keep rather than a guarantee this layer can
// currently demonstrate, and the first word that touches a second creature is
// what will make it demonstrable.
//
// # The save is not optional
//
// A grovelled creature is prone on its sheet, and a sheet that never reached
// disk is a condition the next verb cannot see. saveDirty writes what came
// back onto the same scope every other verb reports through, so a failure here
// is reported the way a failed swing's is rather than swallowed into a turn
// that looks like it worked.
func (d compelledDriver) obey(
	member encounter.MemberID, commanded *conditions.CommandedConditionData,
) (*resolution.ObeyOutput, error) {
	roster, err := d.scope.enc.Members()
	if err != nil {
		return nil, fmt.Errorf("compelled turn %q: %w", member, translate(err))
	}
	announcer := announcerSeam{m: d.m, scope: d.scope}
	cast, err := announcer.boundaryCast(d.ctx, roster)
	if err != nil {
		return nil, fmt.Errorf("compelled turn %q: %w", member, err)
	}

	// A pure view for resolution's Input.World — a mid-verb read, never the
	// storage boundary (encounter v0.43.0, #1385).
	world := d.scope.enc.WorldView()
	out, err := resolution.Obey(d.ctx, &resolution.ObeyInput{
		Interaction: &resolution.Input{
			World:        world,
			Participants: cast,
			Initiative:   d.m.initiative,
			Standing:     d.scope.standing,
			Sight:        &sightSeam{members: worldMembers(world)},
			Equipment:    equipmentBeside(d.scope.standing),
			// The host's driver, NOT this wrapper. Resolution carries the
			// capability without consulting it — no verb runs inside an
			// interaction — and handing it a driver that would open a second
			// interaction of its own is a loop waiting for the first caller
			// who does.
			TurnDriver:    d.m.turnDriver,
			CheckResolver: checkSeam{m: d.m, scope: d.scope},
			Witness:       witnessSeam{scope: d.scope},
			// Machine is deliberately absent: the word is the machine, and
			// Obey refuses an interaction that already carries one.
			Roller: &diceSeam{roller: d.m.dice},
		},
		Member:   string(member),
		CasterID: commanded.CasterID,
		Word:     commanded.Word,
		Source:   *refs.Conditions.Commanded(),
	})
	if err != nil {
		return nil, fmt.Errorf("compelled turn %q: %w", member, translateResolution(err))
	}

	if err := d.m.saveDirty(d.ctx, d.scope, out.Resolved); err != nil {
		return nil, fmt.Errorf("compelled turn %q: %w", member, err)
	}
	return out, nil
}

// alreadySpent are the imposed effect kinds that ask nothing of the turn:
// resolution has already applied each one by the time the driver is answering,
// so the turn's own question — where does it go — is untouched by them.
//
// # It is an allow-list, and a new kind must be added here on purpose
//
// The set happens to be every kind that exists beside [resolution.ImposedMove]
// today, which is the point rather than a coincidence. A kind added later has
// to be read and placed: does the TURN have to carry it out, or has it already
// happened? A default that waved the unknown one through would answer that
// question by accident, and answer it wrong in the direction nobody notices —
// resolution produces the effects, applies them on the bus, and then the turn
// ends as though nothing had been asked of the creature, with nothing in the
// log saying a translation was skipped.
var alreadySpent = map[resolution.ImposedEffectKind]bool{
	// Grovel's prone, applied on the interaction's own bus and saved before
	// this is reached.
	resolution.ImposedCondition: true,

	// The same word landing on a creature that is already prone: Grovel
	// inherits the same-ref replacement rule rather than restating it, so the
	// old instance's removal rides back beside the new one.
	resolution.ImposedConditionRemoved: true,

	// No word deals damage today. It is listed because a word that did would
	// still be asking nothing of the turn — the sheet has already taken it —
	// so refusing it would be this seam failing on somebody else's arithmetic.
	resolution.ImposedDamage: true,
}

// compelledIntent turns the word's effects into the one intent the composition
// executes.
//
// # One move, or none
//
// Approach and Flee each describe exactly one move and Grovel describes none,
// so those are the only two shapes this translation accepts. A second move
// would be two answers to "where does this turn go", and an effect kind that is
// on neither list is a word this seam was never taught — both are
// ErrBadTurnOutcome, which is what the driver boundary already answers when it
// cannot translate what came back.
//
// Nothing here ENDS the turn, and that is not an omission: [encounter.Routed]
// is terminal and Pass ends the turn by being Pass. The text's "and then ends
// its turn" is carried by the intent's nature rather than restated as a step,
// so there is one place that decides it.
//
// The cause is the compulsion, on every beat the walk appends. Required by the
// composition, and required for the honest reason: a creature that spent its
// whole turn walking somewhere it did not choose, narrated with no cause, is
// an observer being told it walked off of its own accord.
func compelledIntent(
	member encounter.MemberID, effects []resolution.ImposedEffect,
) (encounter.TurnIntent, error) {
	var move *resolution.MoveDirective
	for i := range effects {
		effect := effects[i]
		if effect.Kind != resolution.ImposedMove {
			if alreadySpent[effect.Kind] {
				continue
			}
			return nil, fmt.Errorf(
				"compelled turn %q: %w: the word imposed %q, which this seam cannot translate",
				member, ErrBadTurnOutcome, effect.Kind)
		}
		if move != nil {
			return nil, fmt.Errorf(
				"compelled turn %q: %w: the word imposed more than one move",
				member, ErrBadTurnOutcome)
		}
		if effect.Move == nil {
			return nil, fmt.Errorf(
				"compelled turn %q: %w: an imposed move carried no directive",
				member, ErrBadTurnOutcome)
		}
		move = effect.Move
	}

	if move == nil {
		// Grovel, and every later word whose whole turn is what it left
		// behind. The condition is already applied and saved; the turn has
		// nothing further to spend.
		return encounter.Pass{}, nil
	}

	policy, ok := routePolicy(move.Policy)
	if !ok {
		return nil, fmt.Errorf(
			"compelled turn %q: %w: policy %q is not one this composition walks",
			member, ErrBadTurnOutcome, move.Policy)
	}
	return encounter.Routed{
		Policy: policy,
		Anchor: encounter.MemberID(move.AnchorID),
		Cause:  *refs.Conditions.Commanded(),
	}, nil
}
