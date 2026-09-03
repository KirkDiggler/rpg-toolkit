// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"math"
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

	// OutcomeDeathSave is one authoritative tabletop death-save result.
	OutcomeDeathSave OutcomeKind = "death_save"

	// OutcomeDown is a member the rulebook reports out of the fight — the
	// minimal death beat ruled on rpg-toolkit#959: this kind, and who.
	//
	// NOT ACCEPTABLE TO [Encounter.Record], deliberately, and it is the only
	// kind that is not. Down is something the composition NOTICES, by asking
	// [Standing] at the choke point where it already asks about sight; a caller
	// that could push the beat in would be a second system deciding the same
	// thing, and the first one would always win because it reaches the fact
	// first. Record's switch is therefore the CALLER-WRITABLE subset of this
	// enum rather than all of it — pushing this kind gets ErrInvalidData.
	//
	// [Encounter.Record] performing that ask itself (rpg-toolkit#1083) SHARPENS
	// this refusal rather than softening it. The verb a caller uses to report a
	// blow is now the verb that finds out what the blow did — so a caller gets
	// the down beat it wanted, in the same call, and still cannot write one. The
	// beat says what the rulebook answered; it has never said what a caller
	// claimed, and a kind here would be the only way to change that.
	OutcomeDown OutcomeKind = "down"
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
// THERE IS NO PROSE FIELD HERE, and there will not be one. Every member is an
// ID the composition validates against its own roster, the kind is a closed
// enum, and the remaining primitives are catalog/rulebook facts under a closed
// shape. Prose is not something this input can express — not discouraged, not
// filtered, INEXPRESSIBLE — so no caller can narrate into a transcript that
// other players read, and no future caller can start.
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

	// Critical says whether this outcome was a critical hit. Meaningful only
	// for OutcomeStruck — a miss cannot crit — and false (the zero value) is
	// the ordinary case rather than an omission.
	Critical bool

	// Attack names what was swung, when this outcome is an attack. Present
	// for OutcomeStruck/OutcomeMissed, nil otherwise (rpg-toolkit#866,
	// rpg-toolkit#941). Every field is meant to be a catalog-owned
	// identifier the caller read off an already-compiled attack profile —
	// see AttackIdentity's own doc for exactly what this input does and
	// does not check about that.
	Attack *AttackIdentity

	// DamageComponents are the ordered primitive facts that produced a struck
	// outcome. Meaning belongs to the rulebook; this composition preserves the
	// supplied order and values without interpreting them.
	DamageComponents []DamageComponent

	// AdvantageSources and DisadvantageSources identify the ordered sources the
	// attack fold reported. They carry identifiers only, never the rules engine's
	// human-readable Reason string.
	AdvantageSources    []AttackModifierSource
	DisadvantageSources []AttackModifierSource

	// DeathSave carries the authoritative primitive facts for
	// OutcomeDeathSave. It is required for that kind and invalid for every
	// other kind; the encounter preserves it without interpreting thresholds.
	DeathSave *DeathSaveDetail
}

// DeathSaveDetail is the closed, rulebook-neutral story shape for one death
// save. Its fields are primitive facts supplied by the authoritative rulebook;
// this composition validates presence and preserves them verbatim.
type DeathSaveDetail struct {
	Roll              int    `json:"roll"`
	Outcome           string `json:"outcome"`
	SuccessesAdded    int    `json:"successes_added"`
	FailuresAdded     int    `json:"failures_added"`
	Successes         int    `json:"successes"`
	Failures          int    `json:"failures"`
	SuccessesNeeded   int    `json:"successes_needed"`
	FailuresRemaining int    `json:"failures_remaining"`
	Stabilized        bool   `json:"stabilized"`
	Dead              bool   `json:"dead"`
	Recovered         bool   `json:"recovered"`
	HPRestored        int    `json:"hp_restored"`
	Continuation      string `json:"continuation"`
	PresentationID    string `json:"presentation_id"`
}

// DamageComponent carries one ordered, rulebook-neutral damage contribution.
// It uses only primitives this module can persist without importing the
// rulebook packages that own their meaning.
type DamageComponent struct {
	Source     string   `json:"source"`
	SourceRef  string   `json:"source_ref,omitempty"`
	Dice       string   `json:"dice,omitempty"`
	FinalRolls []int    `json:"final_rolls,omitempty"`
	FlatBonus  int      `json:"flat_bonus"`
	DamageType string   `json:"damage_type"`
	Multiplier *float64 `json:"multiplier,omitempty"`
}

// AttackModifierSource identifies an entity/content source without carrying
// prose. Empty strings mean that identifier was absent.
type AttackModifierSource struct {
	SourceRef string `json:"source_ref,omitempty"`
	SourceID  string `json:"source_id,omitempty"`
}

// AttackIdentity names what was swung — ref, display name, and damage type —
// carried on a [RecordInput] whose Kind is OutcomeStruck or OutcomeMissed so
// the beat can answer "with a longsword" and "6 slashing" for every witness,
// not only the one whose own verb response held the compiled profile.
//
// PLAIN STRINGS, MEANT TO BE USED LIKE MemberID IS ON THIS INPUT — CHECKED
// FOR PRESENCE THE WAY MemberID IS, BUT NOT FOR MEANING (Copilot, PR #1172).
// [Encounter.Record] refuses ErrInvalidData when Ref or Name is empty, the
// same as it refuses an empty Actor — that is the minimum this composition
// CAN check without a catalog. It does not, and cannot, validate that Ref or
// Name names anything real, or that DamageType is one of the rulebook's own
// words: this module's go.mod cannot import the catalog that would answer
// whether "longsword" is real (C1). The intended values are the catalog's
// own identifier and display label — "longsword", "Longsword" — and the
// rulebook's own word for the damage dealt, fixed by a sealed weapon/action
// catalog rather than composed at the call site, and it is session — which
// already depends on that package — that maps DamageType onto a closed Go
// type. The composition guarantees presence; the rulebook guarantees
// meaning.
type AttackIdentity struct {
	// Ref is the catalog ref the attack compiled from — "longsword",
	// "unarmed-strike".
	Ref string

	// Name is the catalog's display name for Ref — "Longsword", "Unarmed
	// Strike".
	Name string

	// DamageType is the kind of damage the attack deals, in the rulebook's
	// own words — "slashing", "bludgeoning". A string here, not a closed
	// encounter-level enum: this module's go.mod cannot import the damage
	// package that owns that vocabulary (C1), so the value is carried
	// through exactly as Actor and Targets already are, and it is
	// session — which already depends on that package — that maps it onto
	// a closed Go type.
	DamageType string
}

// RecordOutput reports where the outcome landed in the story.
type RecordOutput struct {
	// IntelDeltas maps member IDs to their updated percepts after any driven
	// monster turns caused by noticing the recorded outcome's consequences.
	IntelDeltas map[MemberID]*IntelDelta

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
// The audience is every current member, at the current clock reading — the
// same convention the clock beats use. An outcome is not secret: a fight is
// localized but visible, and a client that learned about a strike only from
// the striker's own response could not render the scene the party is in.
//
// # It records, and then the world notices what it recorded
//
// This verb used to record and do NOTHING ELSE, and that sentence was worth the
// emphasis it carried. Two thirds of it still hold exactly: no sight refresh, no
// trigger detection, no clock movement. What it now also does is run the
// standing consult — [Encounter.noticeDown], the one place noticing happens —
// after its own beat.
//
// THE REASON IS THE ONE THING THAT MAKES THIS VERB DIFFERENT from the others
// that reach that consult. A walk cannot change who is standing; a recorded
// outcome can, because the blow this beat describes is the blow that took
// somebody to zero. Every other consult site is a verb LOOKING at a world
// somebody else changed. This one is the change. Leaving it out meant a killing
// blow landed, persisted, and left the world not knowing — no down beat, no
// [ByDefeat] ending, the turn order still holding a body — until whatever verb
// next happened to refresh sight. A party that cleared the room and stood still
// was in a fight with a corpse (rpg-toolkit#1083).
//
// NOTICING IS NOT "SOMETHING ELSE", and that is an argument rather than an
// exception carved for convenience. It is the same consult, at the same choke
// point, under the same discipline: the composition ASKS and the rulebook
// answers (C1), the answer is pulled and never remembered, and the story is the
// ledger that keeps the news from being told twice. Record is not growing a
// second mechanism for death — it is joining the list of verbs that reach the
// first one, which is exactly what "one place" is for. The alternative on offer
// was a caller pushing the beat in, and that is a different thing entirely: see
// [OutcomeDown], which is still refused, for why.
//
// # The order in one pass
//
// The recorded outcome lands FIRST, then whatever the consult makes of it: the
// strike, then the body, then the ending the body explains. That is
// [Encounter.refreshSight]'s law — a verb's own beat precedes any beat its
// consequences append — held inside one verb, the same way
// [Encounter.noticeDown] holds it inside one pass. The caller's beat is still
// the cause. What is new is that its effects can arrive in the same breath
// instead of at whatever ran next.
//
// [RecordOutput.Seq] is therefore the OUTCOME beat and never the last one
// written. A caller asked for one thing to be recorded and is told where that
// thing landed.
//
// [RecordOutput.IntelDeltas] carries any percept changes produced when that
// consult transfers a fallen active member and drives the next monster. The
// mutation belongs to this Record even though it happened in a nested turn, so
// dropping it would leave the caller unable to publish the correction.
//
// The consult runs for EVERY kind rather than only for [OutcomeStruck]. Which
// outcomes can drop somebody is a rulebook fact and this module cannot import
// the rulebook (C1) — a Record that decided for itself which of its beats were
// worth looking after would be encoding that rule, and would miss the first
// route to zero nobody has written yet.
//
// # On error
//
// Errors: ErrNilInput, ErrClosed, ErrNoMember (empty or unknown actor, unknown
// target), ErrInvalidData (a kind or value name this composition does not
// know, missing or mismatched DeathSave detail, an Attack whose Ref or Name is
// empty, or a non-finite damage multiplier JSON cannot represent), and
// anything the [Participation] capability answers with — including
// ErrNotMember for an answer naming a stranger.
//
// The input refusals all run before anything is appended, so a rejected input
// costs the rulebook nothing. The consult does not: it runs after the beat, so a
// Record that fails there leaves the in-memory encounter holding an outcome
// whose consequences were never worked out. That is R5's documented limit rather
// than a hole in it, and the caller's obligation is doc.go's whole answer to it
// — drop the encounter unsaved.
func (e *Encounter) Record(in *RecordInput) (*RecordOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("record: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("record: %w", ErrClosed)
	}

	switch in.Kind {
	case OutcomeStruck, OutcomeMissed, OutcomeDeathSave:
	default:
		return nil, fmt.Errorf("record: outcome kind %q: %w", in.Kind, ErrInvalidData)
	}
	if in.Kind == OutcomeDeathSave {
		if in.DeathSave == nil {
			return nil, fmt.Errorf("record: death save detail is required: %w", ErrInvalidData)
		}
		if in.DeathSave.Outcome == "" {
			return nil, fmt.Errorf("record: death save outcome is required: %w", ErrInvalidData)
		}
		if in.DeathSave.Continuation == "" {
			return nil, fmt.Errorf("record: death save continuation is required: %w", ErrInvalidData)
		}
		if in.DeathSave.PresentationID == "" {
			return nil, fmt.Errorf("record: death save presentation id is required: %w", ErrInvalidData)
		}
	} else if in.DeathSave != nil {
		return nil, fmt.Errorf("record: death save detail does not match outcome kind %q: %w", in.Kind, ErrInvalidData)
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

	if in.Kind == OutcomeStruck {
		for i, component := range in.DamageComponents {
			if component.Multiplier != nil &&
				(math.IsNaN(*component.Multiplier) || math.IsInf(*component.Multiplier, 0)) {
				return nil, fmt.Errorf("record: damage component %d multiplier: %w", i, ErrInvalidData)
			}
		}
	}

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

	// Critical is only ever true for a struck outcome — a miss cannot crit —
	// but it is written unconditionally rather than only when true: false
	// beside a hit is itself the answer ("not a critical"), the same
	// false-vs-absent law the wire seam keeps for its own bools, and a typed
	// reader downstream must not have to treat a missing key as a third
	// state.
	if in.Kind == OutcomeStruck {
		payload["critical"] = in.Critical
		if len(in.DamageComponents) > 0 {
			payload["damage_components"] = in.DamageComponents
		}
		if len(in.AdvantageSources) > 0 {
			payload["advantage_sources"] = in.AdvantageSources
		}
		if len(in.DisadvantageSources) > 0 {
			payload["disadvantage_sources"] = in.DisadvantageSources
		}
	}
	if in.DeathSave != nil {
		payload["death_save"] = in.DeathSave
	}
	if in.Attack != nil {
		if in.Attack.Ref == "" {
			return nil, fmt.Errorf("record: attack ref: %w", ErrInvalidData)
		}
		if in.Attack.Name == "" {
			return nil, fmt.Errorf("record: attack name: %w", ErrInvalidData)
		}
		payload["attack"] = map[string]string{
			"ref":         in.Attack.Ref,
			"name":        in.Attack.Name,
			"damage_type": in.Attack.DamageType,
		}
	}

	beatBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("record: outcome payload: %w", err)
	}

	// subjectBeat, subjects are the actor and targets — v1 still sends
	// everyone (audienceFor's doc).
	subjects := append([]MemberID{in.Actor}, targets...)
	appended, err := e.appendBeat(&record.AppendInput{
		At:       uint64(e.clock.ToData().HighWater),
		Audience: e.audienceFor(subjectBeat, subjects...),
		Tags:     map[string]string{"tag": "outcome"},
		Payload:  beatBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("record: %w", err)
	}

	// And now the world finds out what that beat just changed. AFTER the append,
	// never before: the outcome is the cause, and a down beat ahead of the strike
	// that explains it would be a story told backwards. See the godoc.
	_, intelDeltas, nerr := e.noticeDown()
	if nerr != nil {
		return nil, fmt.Errorf("record: %w", nerr)
	}

	return &RecordOutput{IntelDeltas: intelDeltas, Seq: appended.Seq}, nil
}
