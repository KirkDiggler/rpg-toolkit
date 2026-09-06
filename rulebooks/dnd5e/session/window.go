// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// windowIDString renders an interrupt window's id as the string this package
// puts on its exported surface.
//
// A STRING BECAUSE interrupt.WindowID MAY NOT CROSS (law S2, boundary_test.go).
// The ledger's own id type is an inner type; a REACT declaration encodes this
// rendering of it instead, and [Manager.React] matches a declaration id back to
// a window by regenerating every open window's selector rather than by parsing
// anything out of it.
func windowIDString(id interrupt.WindowID) string {
	return strconv.FormatUint(uint64(id), 10)
}

// windowPayloadKind discriminates what a stored window payload froze. One
// value today; it is written and checked so a payload from a build that poses
// something else is REFUSED rather than read as a reaction.
const windowPayloadKind = "reaction"

// windowPayload is the frozen half of one posed reaction window: everything
// [Manager.React] needs to run the swing that was offered, without asking the
// world again for an answer it already gave.
//
// THE DEFINITION IS STORED rather than recompiled. The offer a player accepted
// is the offer that was made — a weapon swapped, a feature spent, or a sheet
// edited between the question and the answer must not silently change what
// they swing with. The mover's kind is NOT stored, because it is a fact about
// the roster this package reloads anyway and a stored copy could only ever
// agree with it or lie to it.
type windowPayload struct {
	// Kind is [windowPayloadKind]. See its doc.
	Kind string `json:"kind"`

	// Mover is whose step is being reacted to, and From/To the step itself —
	// the cells the encounter's own paused turn holds, repeated here because
	// the strike is resolved from this payload and a reaction checked against
	// a different pair of cells is a different reaction.
	Mover string           `json:"mover"`
	From  spatial.Position `json:"from"`
	To    spatial.Position `json:"to"`

	// Reactor is who is being asked. Always the window's own audience;
	// carried inside the payload as well so a mis-paired payload and window
	// is a refusal rather than a swing by the wrong member.
	Reactor string `json:"reactor"`

	// Reaction is the ref of the condition offering the swing —
	// "dnd5e:conditions:opportunity_attack".
	Reaction string `json:"reaction"`

	// Definition is the melee attack the reactor was offered.
	Definition combatActions.Definition `json:"definition"`
}

// marshalWindowPayload renders one posed window's frozen half.
func marshalWindowPayload(p windowPayload) ([]byte, error) {
	p.Kind = windowPayloadKind
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal window payload: %w", err)
	}
	return raw, nil
}

// thawWindowPayload reads a stored window payload back, refusing anything this
// build could not have written.
//
// REJECT, NEVER GUESS — the same trust boundary the ledger load itself keeps.
// A payload of another kind, or one naming nobody, is a session record this
// build cannot act on, and answering it by inventing the missing half would
// resolve a strike that was never offered.
func thawWindowPayload(raw []byte, audience string) (windowPayload, error) {
	var p windowPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return windowPayload{}, fmt.Errorf("%w: window payload: %v", ErrInvalidSession, err)
	}
	if p.Kind != windowPayloadKind {
		return windowPayload{}, fmt.Errorf(
			"%w: window payload kind %q is not one this build poses", ErrInvalidSession, p.Kind)
	}
	if p.Mover == "" || p.Reactor == "" || p.Reaction == "" {
		return windowPayload{}, fmt.Errorf("%w: window payload names no mover, reactor or reaction", ErrInvalidSession)
	}
	if p.Reactor != audience {
		return windowPayload{}, fmt.Errorf(
			"%w: window payload names reactor %q but is posed to %q", ErrInvalidSession, p.Reactor, audience)
	}
	return p, nil
}

// movementReactionRef names the reaction a posed movement window offers.
//
// A CONSTANT BECAUSE THE MACHINE CANNOT TELL US. resolution.ReactionAttacks is
// asked AttackFor(reactorID) and nothing else — the trigger's own condition ref
// is read off the OUTCOME, which a reactor who was asked instead of swung for
// never reaches. So the pose has to name the reaction itself, and today there
// is exactly one reaction a movement fold can offer (resolution's free
// reactions; [reactionName] holds the same single entry).
//
// FAIL CLOSED WHEN THAT STOPS BEING TRUE. The guard is [poseableReaction],
// which refuses rather than guessing the moment this package can name a second
// reaction — at which point the real fix is upstream: AttackFor has to be told
// which trigger it is answering.
func movementReactionRef() string { return refs.Conditions.OpportunityAttack().String() }

// poseableReaction is the identity a posed window offers, or an error when
// this package can no longer say which reaction that is.
func poseableReaction() (ReactionRef, error) {
	if len(reactionName) != 1 {
		return ReactionRef{}, fmt.Errorf(
			"%w: %d reactions can reach a movement fold and AttackFor names none of them",
			ErrInvalidWorld, len(reactionName))
	}
	ref := movementReactionRef()
	name, known := reactionName[ref]
	if !known {
		return ReactionRef{}, fmt.Errorf("%w: no display name for %q", ErrInvalidWorld, ref)
	}
	return ReactionRef{Ref: ref, Name: name}, nil
}
