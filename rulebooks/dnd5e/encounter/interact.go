// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// interact.go is the encounter half of interacting: with a placed world NPC
// (rpg-toolkit#1404, design.md's "Interaction Verb"), or — one verb, widened
// (rpg-project#959) — doing a prop's option, something done to a thing.
//
// A WORLD NPC. It answers only what this package already knows how to
// answer for any member — identity, adjacency, and visibility — and nothing
// about what the NPC IS: no ref, no capabilities, no policy, no descriptor.
// That assembly is session's job, one layer up, from its own member-ID-keyed
// NPC store (design.md's Session Seam). This file has no NPC content to leak
// because it never holds any.
//
// A THING DONE (interact_prop.go). A prop and one of its options whose kind
// is to do ([Option.Do]): the door a line said to a prop goes through, and
// everything a line has after it — answer, grants, the conversation it
// steps, the world's price — but SEEN, NOT HEARD: no hearers, no speech
// beat, no loudness, no creature addressed. Its [BeatInteracted] is told per
// viewer.

// InteractInput names who is reaching for whom, and how far that reach
// extends: a world NPC ([InteractInput.Target]), or a prop and the option
// done to it ([InteractInput.Prop], [InteractInput.OptionID]).
type InteractInput struct {
	// Actor is the player member initiating the interaction.
	Actor MemberID

	// Target is the KindWorld member being interacted with. Exactly one of
	// Target and Prop.
	Target MemberID

	// Range is the maximum distance, in cells, Target — or the nearest cell
	// of Prop — may stand from Actor. Zero (the default) means adjacent —
	// one cell.
	Range int

	// Prop is the prop an option is done to (rpg-project#959): placed, on a
	// cell, carried by the actor, or hidden and known to them — any the
	// actor may be shown. Exactly one of Target and Prop; with Prop,
	// OptionID is REQUIRED.
	Prop PropID

	// OptionID names the thing done, on Prop: an option whose kind is to do
	// ([ErrNoOption] for one that says). Empty for a world NPC.
	OptionID string

	// Revision, Skill, Beaten, DC, Total, Calculation and Roller are
	// [SpeakInput]'s, for the thing done exactly as for a line: the revision
	// it was offered at, the check the session rolled, and the world's die
	// for the answer (REQUIRED with Prop). Unread for a world NPC.
	Revision    uint64
	Skill       string
	Beaten      bool
	DC          int
	Total       int
	Calculation *RollCalculation
	Roller      dice.Roller
}

// InteractOutput confirms who was reached, or what came of the thing done.
// For a world NPC, session resolves what that member IS by looking Target up
// in its own NPC store — this type carries no NPC content because encounter
// holds none.
type InteractOutput struct {
	// Target echoes the confirmed member ID, so a caller need not trust its
	// own input back. Empty for a thing done.
	Target MemberID

	// Seq is the sequence number of the recorded interaction beat — for a
	// thing done, the actor's own [BeatInteracted] version.
	Seq uint64

	// Prop echoes the prop a thing was done to; empty for a world NPC.
	Prop PropID

	// Beaten, Witnesses, Answered and Formed are [SpeakOutput]'s, for a thing
	// done: the verdict (true for a plain one), every member who saw the
	// actor (the actor included), the row the world's die picked (nil when
	// none), and the fight this option's own row relocation formed on
	// arrival (nil otherwise). Zero for a world NPC.
	Beaten    bool
	Witnesses []MemberID
	Answered  *Pick
	Formed    *FormedBubble
}

// Interact confirms a player can reach a placed world NPC: both exist, the
// actor is a player, the target is a KindWorld member, they stand within
// range (adjacent by default), and the target is currently visible to the
// actor. It appends a story beat and returns confirmation of who was
// reached — nothing about the NPC's content, nothing about its state, and
// no feature-specific behavior. That is deliberate: this verb has nothing
// else to say.
//
// WITH A PROP OR AN OPTION it is the thing done instead
// ([Encounter.interactWithProp], rpg-project#959), with Speak's door and
// errors and its own beat; nothing below applies to it.
//
// Validation order (R5): nil input → empty actor/target → negative Range →
// closed → actor exists and is a player → target exists and is a world NPC
// → both placed → target in range → target visible.
//
// Errors: ErrNilInput, ErrNoMember (empty actor/target), ErrBadReach (a
// negative Range — the same "negative numeric input is a caller defect"
// convention the Sheets consult applies to a speed or an action's RangeFeet),
// ErrClosed, ErrNotMember (actor/target missing, or present but the wrong
// kind), ErrBadPlacement (either member has no cell), ErrOutOfRange,
// ErrNotVisible.
func (e *Encounter) Interact(in *InteractInput) (*InteractOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("interact: %w", ErrNilInput)
	}
	if in.Prop != "" || in.OptionID != "" {
		return e.interactWithProp(context.Background(), in)
	}
	if in.Actor == "" || in.Target == "" {
		return nil, fmt.Errorf("interact: %w", ErrNoMember)
	}
	// A negative Range is not a smaller reach; it is a caller defect, the
	// same call the Sheets consult makes for a speed and each action's
	// RangeFeet (validateSheetFacts) — silently normalizing it to "adjacent"
	// would hide the mistake rather than report it (Copilot, PR #1412 review).
	if in.Range < 0 {
		return nil, fmt.Errorf("interact: range %d is negative: %w", in.Range, ErrBadReach)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("interact: %w", ErrClosed)
	}

	actor, ok := e.members[in.Actor]
	if !ok {
		return nil, fmt.Errorf("interact: actor %q: %w", in.Actor, ErrNotMember)
	}
	if actor.Kind != KindPlayer {
		return nil, fmt.Errorf("interact: actor %q is not a player: %w", in.Actor, ErrNotMember)
	}

	target, ok := e.members[in.Target]
	if !ok {
		return nil, fmt.Errorf("interact: target %q: %w", in.Target, ErrNotMember)
	}
	if target.Kind != KindWorld {
		return nil, fmt.Errorf("interact: target %q is not a world npc: %w", in.Target, ErrNotMember)
	}

	actorCell, placed := e.canvas.GetEntityPosition(string(in.Actor))
	if !placed {
		return nil, fmt.Errorf("interact: actor %q: %w", in.Actor, ErrBadPlacement)
	}
	targetCell, placed := e.canvas.GetEntityPosition(string(in.Target))
	if !placed {
		return nil, fmt.Errorf("interact: target %q: %w", in.Target, ErrBadPlacement)
	}

	// Range is non-negative here — checked above. Zero means "adjacent".
	maxRange := in.Range
	if maxRange == 0 {
		maxRange = 1
	}
	if e.Distance(actorCell, targetCell) > float64(maxRange) {
		return nil, fmt.Errorf("interact: target %q: %w", in.Target, ErrOutOfRange)
	}

	visible, err := e.currentlyPerceives(in.Actor, in.Target)
	if err != nil {
		return nil, fmt.Errorf("interact: %w", err)
	}
	if !visible {
		return nil, fmt.Errorf("interact: target %q: %w", in.Target, ErrNotVisible)
	}

	at := uint64(e.clock.ToData().HighWater)
	payload, err := json.Marshal(map[string]interface{}{
		"beat":   BeatInteracted,
		"actor":  string(in.Actor),
		"target": string(in.Target),
	})
	if err != nil {
		return nil, fmt.Errorf("interact: marshal beat: %w", err)
	}

	out, err := e.appendBeat(&record.AppendInput{
		Audience: []MemberID{in.Actor, in.Target},
		Tags:     map[string]string{"tag": "interact"},
		Payload:  payload,
		At:       at,
	})
	if err != nil {
		return nil, fmt.Errorf("interact: append beat: %w", err)
	}

	// THE WORLD'S PRICE FOR AN ACTION, paid after the outcome has landed and
	// before anything refreshes sight (design §5, worldtime.go): one round on
	// the world clock for the actor, and the world thinks on it. Nothing at
	// all for a member inside a fight, where the round is what prices time.
	if err := e.spendWorldAction(in.Actor); err != nil {
		return nil, err
	}

	return &InteractOutput{Target: in.Target, Seq: out.Seq}, nil
}

// currentlyPerceives reports whether observer's CURRENT intel holdings name
// subject — the same "current, not held" rule unawareOfOpposition and
// contactBetween already apply: a subject once seen but not seen now (a
// Held ghost) does not count, the same as one never seen at all.
//
// SIGHT, NAMED. This used to be channel-blind on purpose — a holding was
// current whenever any channel confirmed it, and with sight the only channel
// this package writes, discriminating would have guarded against an input
// nothing could produce. The comment said "revisit if a second channel
// (sound, say) ever arrives" (Copilot, PR #1412 review), and mind/perception
// v0.2.0 is that arrival: currency is per-channel, and interacting is a
// thing you do with something you can SEE. Hearing a goblin behind a door is
// not reaching it.
func (e *Encounter) currentlyPerceives(observer, subject MemberID) (bool, error) {
	holdings, err := e.memberIntel(observer)
	if err != nil {
		return false, fmt.Errorf("held by %q: %w", observer, err)
	}
	for _, h := range holdings {
		if h.CurrentOn(perception.Sight) && h.Subject == subject {
			return true, nil
		}
	}
	return false, nil
}
