// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// intimidate.go is THE FIRST SHENANIGAN (rpg-project#454,
// ideas/shenanigans/intimidate.md): a check whose success changes what a
// monster believes, and therefore what it does next.
//
// # It writes to two records and copies nothing between them
//
// A beaten threat lands a DEED on every witness — the play record, this
// goblin's own memory of who frightened it, gone with the run — and, when the
// author asked for one, teaches a FACT to the same witnesses — the world
// journal, what the camp comes to know and carries out of the run. Both are
// records, they serve different purposes, and neither is the other's cache
// (Kirk, 2026-09-16). One seam writes to each; there is no bridge to build.
//
// # NOTHING HERE DECIDES WHAT THE THREAT IS WORTH
//
// [Encounter.Unlock]'s law, applied to a mind: this verb is TOLD whether the
// check was beaten and lands the testimony. What a held `intimidate` deed
// MEANS is the preset's — the coward reads fear, the berserker a provocation,
// the retaliator nothing at all — and it is read in
// rulebooks/dnd5e/behavior, where 5e lives. The design's first broken cut was
// "on success, set the goblin to fleeing": a flee flag is state the driver
// would have to read live (rule A2), and it would make the outcome the verb's
// instead of the mind's. A berserker told to run does not run.
//
// # The audience is the witnesses, and the target must be among them
//
// Who learns is [Encounter.witnessesOf] — sight and line of sight from the
// ACTOR's cell, the same set [Encounter.landAttack] computes. A threat only
// reaches somebody who can see who is making it, so a target outside that set
// refuses ([ErrUnwitnessed]) rather than quietly frightening a goblin through
// a wall. There is no distance cap beyond sight: shouting across a lit hall
// is a shenanigan, and the DC is the monster's, not the range's.

// Witnesses is every member whose senses reach a member's own cell: who would
// learn what that member does where they stand.
//
// A READ, and the one the caller needs BEFORE it does anything expensive. The
// session prices a threat in the actor's standard action and must know the
// target can see them before it charges — a verb that takes the action and
// then refuses is one nobody would forgive — and the same list is what an
// action panel offers as the people you can shout at. [Encounter.Intimidate]
// asks again for itself and does not trust the answer it handed out: this is
// a read of a moment, and a door may close between the two calls.
//
// Sorted, and includes the member themselves whenever they can see their own
// cell — which is every placed member, and is why the audience of an
// `intimidated` beat has the actor in it.
//
// Errors: ErrNoMember (no such member), ErrBadPlacement (not placed).
func (e *Encounter) Witnesses(of MemberID) ([]MemberID, error) {
	_, witnesses, err := e.audienceOf(of)
	if err != nil {
		return nil, fmt.Errorf("witnesses: %w", err)
	}

	return witnesses, nil
}

// IntimidateInput names who threatened whom, whether the check was beaten,
// and the numbers the table should see.
type IntimidateInput struct {
	// Actor is the member making the threat. Must be a member of this
	// encounter (ErrNotMember) and must be placed (ErrBadPlacement).
	Actor MemberID

	// Target is the member being threatened. Must be a member, and must be
	// able to SEE the actor — in [Encounter.witnessesOf] of the actor's cell
	// (ErrUnwitnessed otherwise).
	Target MemberID

	// Beaten is whether the check beat the DC. CARRIED, NEVER COMPARED: this
	// module holds no 5e, so who decides a total beats a difficulty lives on
	// the other side of this seam ([Encounter.Unlock]'s reasoning, and the
	// reason the DC below is reported rather than measured).
	//
	// FALSE LANDS NOTHING — no deed, no fact — and is not an error. A threat
	// that missed is an outcome, and the beat still goes down the log.
	Beaten bool

	// DC and Total are the numbers the table sees, carried onto the beat so
	// the roll is visible whether it landed or not ("the roll is seen",
	// ideas/shenanigans/README.md). Filled by the session, which ran the
	// check; nothing here reads them.
	DC    int
	Total int
}

// IntimidateOutput reports what the threat reached.
type IntimidateOutput struct {
	// Beaten echoes what the caller said, so a caller reads the result off
	// the answer rather than off the fact that it called
	// ([UnlockOutput.Beaten]'s reasoning).
	Beaten bool

	// Witnesses is every member who saw it — who holds the deed on a
	// success, and who was told regardless. Sorted, the audience's own
	// order.
	Witnesses []MemberID

	// Fact is the world fact the witnesses learned, or empty when the
	// author planted none or the check was missed. Echoed so a caller can
	// narrate "the camp knows" without re-reading the placement.
	Fact FactID

	// Seq is the sequence number of the `intimidated` beat. A failed
	// attempt gets one too: somebody tried, and the story is what happened
	// rather than what worked ([UnlockOutput.Seq]).
	Seq uint64
}

// Intimidate reports a threat against a member, and on a beaten check lands
// the deed that frightened it.
//
// Validation order (R5 atomicity): nil input → empty actor/target → closed →
// actor is a member → target is a member → actor and target are not the same
// → the actor is placed → the target can see the actor → the beat → the deed
// → the fact.
//
// THE BEAT IS APPENDED BEFORE ITS CONSEQUENCES, the law
// [Encounter.refreshSight] states: the threat is the cause, and a stance beat
// ahead of the threat that explains it would be a story told backwards.
//
// Errors: ErrNilInput, ErrNoMember, ErrClosed, ErrNotMember, ErrBadPlacement,
// ErrUnwitnessed.
func (e *Encounter) Intimidate(in *IntimidateInput) (*IntimidateOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("intimidate: %w", ErrNilInput)
	}
	if in.Actor == "" || in.Target == "" {
		return nil, fmt.Errorf("intimidate: %w", ErrNoMember)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("intimidate: %w", ErrClosed)
	}

	if _, ok := e.members[in.Actor]; !ok {
		return nil, fmt.Errorf("intimidate: actor %q: %w", in.Actor, ErrNotMember)
	}
	target, ok := e.members[in.Target]
	if !ok {
		return nil, fmt.Errorf("intimidate: target %q: %w", in.Target, ErrNotMember)
	}
	// Frightening yourself is not a shenanigan, it is a caller defect — and
	// it would land a deed naming its own holder, which no mind can read as
	// anything.
	if in.Actor == in.Target {
		return nil, fmt.Errorf("intimidate: actor %q cannot threaten itself: %w", in.Actor, ErrNotMember)
	}

	where, witnesses, err := e.audienceOf(in.Actor)
	if err != nil {
		return nil, fmt.Errorf("intimidate: %w", err)
	}
	if !slices.Contains(witnesses, in.Target) {
		return nil, fmt.Errorf("intimidate: target %q cannot see the actor: %w", in.Target, ErrUnwitnessed)
	}

	at := uint64(e.clock.ToData().HighWater)
	seq, err := e.appendIntimidatedBeat(in, witnesses, at)
	if err != nil {
		return nil, err
	}

	out := &IntimidateOutput{Beaten: in.Beaten, Witnesses: witnesses, Seq: seq}
	if !in.Beaten {
		return out, nil
	}

	if err := e.landDeed(DeedIntimidate, in.Actor, in.Target, where, witnesses); err != nil {
		return nil, fmt.Errorf("intimidate: %w", err)
	}

	// THE WORLD HALF IS OPT-IN and it is the TARGET's placement that opts
	// in: `on: { intimidated: { fact: sergeant-cowed } }` is authored on the
	// monster being threatened, and the witnesses to that threat are who
	// learn it. Absent means no fact and the camp does not care.
	if target.OnIntimidated == "" {
		return out, nil
	}
	for _, id := range witnesses {
		if err := e.learnFact(id, target.OnIntimidated, "intimidated", at); err != nil {
			return nil, fmt.Errorf("intimidate: %w", err)
		}
	}
	out.Fact = target.OnIntimidated

	return out, nil
}

// appendIntimidatedBeat writes what the table saw: who threatened whom, the
// DC, the total, and whether it landed.
//
// EVERY WITNESS IS THE AUDIENCE, not just the two parties — "everyone in the
// set learns what happened, not only the target" (the design, decision 2).
// The numbers are written unconditionally, false beside a miss included, for
// the reason a struck beat writes `critical: false`: absent must not become a
// third state for a reader downstream.
func (e *Encounter) appendIntimidatedBeat(in *IntimidateInput, witnesses []MemberID, at uint64) (uint64, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"beat":   "intimidated",
		"actor":  string(in.Actor),
		"target": string(in.Target),
		"dc":     in.DC,
		"total":  in.Total,
		"beaten": in.Beaten,
	})
	if err != nil {
		return 0, fmt.Errorf("intimidate: marshal beat: %w", err)
	}

	out, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: witnesses,
		Tags:     map[string]string{"tag": "intimidate"},
		Payload:  payload,
	})
	if err != nil {
		return 0, fmt.Errorf("intimidate: %w", err)
	}

	return out.Seq, nil
}
