// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// OutcomeKind names a rulebook result the story can carry.
//
// A CLOSED SET, and that is the point of the whole shape. The composition is
// the only thing that can write to an encounter's record, which is what keeps
// one transcript honest — every beat is stamped by the same hand, at the same
// clock, with the same audience rule. Letting a rulebook write whatever it
// liked would keep the transcript in one place and give up the reason that
// mattered: that a reader can trust what is in it.
//
// So a rulebook does not describe what happened. It names a kind the
// composition already knows and hands over numbers. Adding a kind is a change
// here, visible in a diff, and that is the cost being chosen on purpose.
type OutcomeKind string

const (
	// OutcomeStruck is an attack that landed.
	OutcomeStruck OutcomeKind = "struck"

	// OutcomeMissed is an attack that did not.
	OutcomeMissed OutcomeKind = "missed"
)

// OutcomeValue names one number a rulebook outcome carries.
//
// Deliberately generic — "the roll", "what it needed", "how much" — because
// the composition must not learn what an armour class is. The rulebook knows
// which of its numbers is which; the composition knows only that they are
// numbers, and refuses any name not on this list.
type OutcomeValue string

const (
	// ValueRoll is the die as rolled.
	ValueRoll OutcomeValue = "roll"

	// ValueTotal is the roll plus whatever the rules added to it.
	ValueTotal OutcomeValue = "total"

	// ValueAgainst is the number the total had to reach.
	ValueAgainst OutcomeValue = "against"

	// ValueAmount is how much was done — damage, healing, whatever the kind
	// means by it.
	ValueAmount OutcomeValue = "amount"
)

// RecordInput is one rulebook outcome, offered to the story.
//
// THERE IS NO STRING FIELD HERE, and there will not be one. Every member is an
// ID the composition validates against its own roster, the kind is a closed
// enum, and the values are integers under closed keys. Prose is not something
// this input can express — not discouraged, not filtered, INEXPRESSIBLE — so
// no caller can narrate into a transcript that other players read, and no
// future caller can start.
//
// That is the difference between this and the general "append anything" hole
// it replaces the need for. The composition stays the only author of its
// record, and what a rulebook contributes is facts it can check.
type RecordInput struct {
	// Kind is what happened. Must be a kind this composition knows.
	Kind OutcomeKind

	// Actor is who did it. Must be a current member.
	Actor MemberID

	// Targets are who it was done to, if anyone. Each must be a current
	// member. Recorded sorted, for the same C8 reason every other list here
	// is: identical inputs must produce identical stories.
	Targets []MemberID

	// Values are the numbers, under names from the closed set. Absent is
	// legal — a kind that carries no numbers is a kind, not an error.
	Values map[OutcomeValue]int
}

// RecordOutput reports where the outcome landed in the story.
type RecordOutput struct {
	// Seq is the story sequence of the recorded beat.
	Seq uint64
}

// Record puts one rulebook outcome into the encounter's story.
//
// It exists because a strike resolved outside this module was INVISIBLE:
// resolution returns an outcome value and writes no beat, appendBeat is
// unexported, and the SDK builds every client event by reading each member's
// story — so an attack produced nothing to render and nothing to re-read
// (rpg-toolkit#966). One transcript is the product; a rule that resolves
// somewhere else still has to land in it.
//
// It records and does NOTHING ELSE. No sight refresh, no trigger detection, no
// clock movement — the beat is the whole effect. That is what makes the
// ordering law hold for free: the rule at [Encounter.refreshSight] is that a
// verb's own beat precedes any beat its consequences append, and an outcome IS
// a consequence, of whatever the caller did to produce it. Its beat therefore
// lands after that verb's, because the caller records it after, and nothing
// here can reorder them.
//
// The audience is every current member, at the current clock reading — the
// same convention the clock beats use. An outcome is not secret: a fight is
// localized but visible, and a client that learned about a strike only from
// the striker's own response could not render the scene the party is in.
//
// Errors: ErrNilInput, ErrClosed, ErrNoMember (empty or unknown actor, unknown
// target), ErrInvalidData (a kind or value name this composition does not
// know).
func (e *Encounter) Record(in *RecordInput) (*RecordOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("record: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("record: %w", ErrClosed)
	}

	switch in.Kind {
	case OutcomeStruck, OutcomeMissed:
	default:
		return nil, fmt.Errorf("record: outcome kind %q: %w", in.Kind, ErrInvalidData)
	}

	if in.Actor == "" {
		return nil, fmt.Errorf("record: actor: %w", ErrNoMember)
	}
	if _, ok := e.members[in.Actor]; !ok {
		return nil, fmt.Errorf("record: actor %q: %w", in.Actor, ErrNoMember)
	}

	targets := append([]MemberID(nil), in.Targets...)
	for _, id := range targets {
		if _, ok := e.members[id]; !ok {
			return nil, fmt.Errorf("record: target %q: %w", id, ErrNoMember)
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i] < targets[j] })

	// Values are marshalled through a sorted slice rather than the map: a Go
	// map has no order, and a beat whose bytes differ between two runs of the
	// same input is a transcript that cannot be compared (C8).
	names := make([]OutcomeValue, 0, len(in.Values))
	for name := range in.Values {
		switch name {
		case ValueRoll, ValueTotal, ValueAgainst, ValueAmount:
			names = append(names, name)
		default:
			return nil, fmt.Errorf("record: value %q: %w", name, ErrInvalidData)
		}
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })

	payload := map[string]interface{}{
		"beat":  string(in.Kind),
		"actor": string(in.Actor),
	}
	if len(targets) > 0 {
		out := make([]string, 0, len(targets))
		for _, id := range targets {
			out = append(out, string(id))
		}
		payload["targets"] = out
	}
	for _, name := range names {
		payload[string(name)] = in.Values[name]
	}

	beatBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("record: outcome payload: %w", err)
	}

	memberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		memberIDs = append(memberIDs, id)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	appended, err := e.appendBeat(&record.AppendInput{
		At:       uint64(e.clock.ToData().HighWater),
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "outcome"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("record: %w", err)
	}

	return &RecordOutput{Seq: appended.Seq}, nil
}
