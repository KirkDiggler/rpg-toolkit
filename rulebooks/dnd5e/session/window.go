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

// The kinds of thing a stored window payload can have frozen.
//
// WRITTEN AND CHECKED, so a payload from a build that poses something else is
// REFUSED rather than read as the wrong question. There were two readings of
// the single value this replaced — "the kind" and "the only kind" — and the
// second stopped being true the moment a roll could pause.
const (
	// windowKindReaction is a step that stopped to offer a swing at the mover
	// walking away (rpg-project#316 rung 3).
	windowKindReaction = "reaction"

	// windowKindPostRoll is a d20 that stopped to ask its roller whether they
	// spend something they hold on it (rpg-project#398).
	windowKindPostRoll = "post_roll"
)

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

// postRollWindowPayload is the frozen half of one posed post-roll window.
//
// # The machine's own bytes ride along untouched
//
// [postRollWindowPayload.Frozen] is resolution's, opaque here, and handed back
// whole. This package stores the numbers BESIDE it — audience, offer, roll,
// total, and what the resulting beat needs — rather than reaching into the
// blob, because reading it would put this seam inside a rules machine's state.
// The duplication is the seam: two modules each keep what they own.
type postRollWindowPayload struct {
	// Kind is [windowKindPostRoll]. See its doc.
	Kind string `json:"kind"`

	// Audience is who is being asked. Always the window's own audience, and
	// always the member whose d20 was rolled — this slice poses to the roller
	// and to nobody else. Carried inside the payload as well so a mis-paired
	// payload and window is a refusal rather than an answer by the wrong
	// member.
	Audience string `json:"audience"`

	// Target is who was being swung at, and Attack what was swung. Both are
	// here for the struck/missed beat the ANSWER writes: the first call
	// recorded no outcome beat at all, and the second has no compiled offer to
	// read them off.
	Target string    `json:"target"`
	Attack AttackRef `json:"attack"`

	// PresentationID is the token the declaring client is already correlating
	// its own simulated throw against. It was minted before the dice and must
	// survive the pause, or the throw the player watched belongs to nothing.
	PresentationID string `json:"presentation_id"`

	// Offer is what the audience holds, as the effect that offered it named
	// itself — the ref the button is keyed to and the name it is labelled
	// with.
	Offer ReactionRef `json:"offer"`

	// Roll and Total are the d20 and the number the offer would join, carried
	// for the beat that asks. THE TARGET'S AC IS NOT HERE, and its absence is
	// the design: a player who could see it would be deciding whether the die
	// closes the gap rather than whether it is worth spending.
	Roll  int `json:"roll"`
	Total int `json:"total"`

	// Frozen is resolution's own machine state, opaque to this package.
	Frozen []byte `json:"frozen"`
}

// marshalWindowPayload renders one posed reaction window's frozen half.
func marshalWindowPayload(p windowPayload) ([]byte, error) {
	p.Kind = windowKindReaction
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal window payload: %w", err)
	}
	return raw, nil
}

// marshalPostRollPayload renders one posed post-roll window's frozen half.
func marshalPostRollPayload(p postRollWindowPayload) ([]byte, error) {
	p.Kind = windowKindPostRoll
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal post-roll window payload: %w", err)
	}
	return raw, nil
}

// windowKindOf reads which question a stored payload froze, without decoding
// the rest of it.
//
// A PEEK RATHER THAN A UNION. Every reader here already knows which kind it
// can act on, so each asks for that kind by name and is refused if the payload
// is the other one. A single struct carrying both halves would have a field
// lying on every payload of the wrong kind.
func windowKindOf(raw []byte) (string, error) {
	var peek struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &peek); err != nil {
		return "", fmt.Errorf("%w: window payload: %v", ErrInvalidSession, err)
	}
	switch peek.Kind {
	case windowKindReaction, windowKindPostRoll:
		return peek.Kind, nil
	default:
		return "", fmt.Errorf(
			"%w: window payload kind %q is not one this build poses", ErrInvalidSession, peek.Kind)
	}
}

// thawWindowPayload reads a stored REACTION window payload back, refusing
// anything this build could not have written.
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
	if p.Kind != windowKindReaction {
		return windowPayload{}, fmt.Errorf(
			"%w: window payload kind %q is not a reaction window", ErrInvalidSession, p.Kind)
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

// thawPostRollPayload reads a stored POST-ROLL window payload back, under
// thawWindowPayload's rule and for the same reason.
func thawPostRollPayload(raw []byte, audience string) (postRollWindowPayload, error) {
	var p postRollWindowPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return postRollWindowPayload{}, fmt.Errorf("%w: post-roll window payload: %v", ErrInvalidSession, err)
	}
	if p.Kind != windowKindPostRoll {
		return postRollWindowPayload{}, fmt.Errorf(
			"%w: window payload kind %q is not a post-roll window", ErrInvalidSession, p.Kind)
	}
	if p.Audience == "" || p.Target == "" || p.Offer.Ref == "" || p.Offer.Name == "" {
		return postRollWindowPayload{}, fmt.Errorf(
			"%w: post-roll window payload names no audience, target or offer", ErrInvalidSession)
	}
	if len(p.Frozen) == 0 {
		return postRollWindowPayload{}, fmt.Errorf(
			"%w: post-roll window payload froze no machine to resume", ErrInvalidSession)
	}
	if p.Audience != audience {
		return postRollWindowPayload{}, fmt.Errorf(
			"%w: post-roll window payload names %q but is posed to %q", ErrInvalidSession, p.Audience, audience)
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
