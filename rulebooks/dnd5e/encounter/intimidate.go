// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/dice"
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

// BeatIntimidated is the "beat" value of the story beat this composition
// appends when somebody threatens a member — landed or not.
//
// EXPORTED BECAUSE A DECODER READS IT, for the same reason [BeatSighted]
// and [BeatWindowOpened] are: a session-side decoder is written against it
// in the same wave, so a rename fails to compile there instead of quietly
// producing a beat nobody renders. That failure is the one this constant
// exists to prevent, and it is not hypothetical — this beat shipped
// untyped at the seam once, and the roll reached no client at all.
//
// IT MATTERS MORE HERE THAN FOR MOST BEATS. This beat is the ONLY account
// of the roll: unlike a swing, a threat writes no outcome and a missed one
// writes nothing else whatsoever. A client that cannot decode it cannot
// tell the table what happened.
const BeatIntimidated = "intimidated"

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

	// Calculation is the full sourced arithmetic behind Total — the d20 pool
	// with every face it threw and the keep record naming the rule that
	// decided which one counted. CARRIED, NEVER COMPARED, exactly like DC and
	// Total; it rides the beat so the story can say "2d20 [7, 18] kept 7 ·
	// disadvantage: Untrained" instead of one number (rpg-project#462).
	//
	// Optional. When present it must describe Total and open with a d20 pool,
	// or the attempt is refused rather than written down wrong.
	Calculation *RollCalculation

	// Roller is THE WORLD'S DIE, the one the answer table is picked with.
	// REQUIRED — supplied, never defaulted ([ErrNoRoller]), for
	// resolution.CheckInput.Roller's standing reason: a silent default puts
	// untestable randomness into a result that looks fine, and R1 says this
	// roll is shown to the table.
	//
	// PER CALL RATHER THAN A CONSTRUCTOR CAPABILITY, unlike
	// [InitiativeRoller]. Every capability refused at [NewEncounter] is one
	// the composition consults from first light and cannot avoid; this one
	// is consulted only when somebody speaks to a creature whose author
	// wrote it a table. That is [Decider]'s shape, not Initiative's — and
	// the caller that rolled the check is the caller that holds the die.
	Roller dice.Roller
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

	// Seq is the sequence number of the `intimidated` beat. A failed
	// attempt gets one too: somebody tried, and the story is what happened
	// rather than what worked ([UnlockOutput.Seq]).
	Seq uint64
}

// Intimidate reports a threat against a member, and on a beaten check lands
// the deed that frightened it — then rolls whatever the author wrote the
// creature does about it.
//
// Validation order (R5 atomicity): nil input → empty actor/target → no roller
// → closed → actor is a member → target is a member → actor and target are not
// the same → the actor is placed → the target can see the actor → the beat →
// the deed → the answer.
//
// THE BEAT IS APPENDED BEFORE ITS CONSEQUENCES, the law
// [Encounter.refreshSight] states: the threat is the cause, and a stance beat
// ahead of the threat that explains it would be a story told backwards. The
// ANSWERED beat comes after both, because it is the result rather than the
// cause (answer.go).
//
// Errors: ErrNilInput, ErrNoMember, ErrNoRoller, ErrClosed, ErrNotMember,
// ErrBadPlacement, ErrUnwitnessed.
func (e *Encounter) Intimidate(ctx context.Context, in *IntimidateInput) (*IntimidateOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("intimidate: %w", ErrNilInput)
	}
	out, err := e.social(ctx, socialInput{
		verb: DeedIntimidate, beat: BeatIntimidated, tag: "intimidate",
		actor: in.Actor, target: in.Target, beaten: in.Beaten,
		dc: in.DC, total: in.Total, calculation: in.Calculation, roller: in.Roller,
	})
	if err != nil {
		return nil, err
	}

	return &IntimidateOutput{Beaten: out.beaten, Witnesses: out.witnesses, Seq: out.seq}, nil
}

// socialInput is one social verb's landing, in the vocabulary the shared body
// takes it in. Unexported: the seam's own words are [IntimidateInput] and
// [PersuadeInput], and this is the machine underneath them.
type socialInput struct {
	verb        string
	beat        string
	tag         string
	actor       MemberID
	target      MemberID
	beaten      bool
	dc          int
	total       int
	calculation *RollCalculation
	roller      dice.Roller
}

// socialOutput is what the shared body reached.
type socialOutput struct {
	beaten    bool
	witnesses []MemberID
	seq       uint64
}

// social is THE ONE BODY BOTH SOCIAL VERBS RUN, and sharing it is the design's
// own claim: "Persuade is Intimidate's twin on the same machine". The audience,
// the refusals, the beat, the deed and the answer are identical; what differs
// is the verb the deed lands under, the beat's name and which half of the
// table the verdict reads.
//
// TWO BODIES WOULD BE TWO ANSWERS TO "WHO IS THE AUDIENCE". The moment one of
// them learned something about witnesses the other did not, a threat and an
// appeal in the same room would reach different people for reasons nobody
// wrote down. The price a verb costs and the DC it faces are exactly where the
// two verbs DO differ, and both of those live on the other side of this seam.
func (e *Encounter) social(ctx context.Context, in socialInput) (socialOutput, error) {
	if in.actor == "" || in.target == "" {
		return socialOutput{}, fmt.Errorf("%s: %w", in.verb, ErrNoMember)
	}
	if in.roller == nil {
		return socialOutput{}, fmt.Errorf("%s: the world rolls the answer: %w", in.verb, ErrNoRoller)
	}
	if e.outcome != nil {
		return socialOutput{}, fmt.Errorf("%s: %w", in.verb, ErrClosed)
	}

	if _, ok := e.members[in.actor]; !ok {
		return socialOutput{}, fmt.Errorf("%s: actor %q: %w", in.verb, in.actor, ErrNotMember)
	}
	if _, ok := e.members[in.target]; !ok {
		return socialOutput{}, fmt.Errorf("%s: target %q: %w", in.verb, in.target, ErrNotMember)
	}
	// Frightening yourself is not a shenanigan, it is a caller defect — and
	// it would land a deed naming its own holder, which no mind can read as
	// anything.
	if in.actor == in.target {
		return socialOutput{}, fmt.Errorf("%s: actor %q cannot address itself: %w",
			in.verb, in.actor, ErrNotMember)
	}

	where, witnesses, err := e.audienceOf(in.actor)
	if err != nil {
		return socialOutput{}, fmt.Errorf("%s: %w", in.verb, err)
	}
	if !slices.Contains(witnesses, in.target) {
		return socialOutput{}, fmt.Errorf("%s: target %q cannot see the actor: %w",
			in.verb, in.target, ErrUnwitnessed)
	}

	at := uint64(e.clock.ToData().HighWater)
	seq, err := e.appendSocialBeat(in, witnesses, at)
	if err != nil {
		return socialOutput{}, err
	}

	if in.beaten {
		if err := e.landDeed(in.verb, in.actor, in.target, where, witnesses); err != nil {
			return socialOutput{}, fmt.Errorf("%s: %w", in.verb, err)
		}
	}

	// THE WORLD HALF IS OPT-IN AND IT IS THE TARGET'S PLACEMENT THAT OPTS IN,
	// on either verdict: `on: { intimidated: … }` and `on: { intimidate_failed:
	// … }` are two keys of one table, and a placement that authored neither
	// answers nothing at all.
	if err := e.answer(ctx, answerInput{
		creature: in.target, actor: in.actor,
		key:  answerKeyFor(in.verb, in.beaten),
		verb: in.verb, beaten: in.beaten,
		witnesses: witnesses, at: at, roller: in.roller,
	}); err != nil {
		return socialOutput{}, fmt.Errorf("%s: %w", in.verb, err)
	}

	return socialOutput{beaten: in.beaten, witnesses: witnesses, seq: seq}, nil
}

// appendSocialBeat writes what the table saw: who addressed whom, the DC, the
// total, and whether it landed.
//
// EVERY WITNESS IS THE AUDIENCE, not just the two parties — "everyone in the
// set learns what happened, not only the target" (the design, decision 2).
// The numbers are written unconditionally, false beside a miss included, for
// the reason a struck beat writes `critical: false`: absent must not become a
// third state for a reader downstream.
func (e *Encounter) appendSocialBeat(in socialInput, witnesses []MemberID, at uint64) (uint64, error) {
	body := map[string]interface{}{
		"beat":   in.beat,
		"actor":  string(in.actor),
		"target": string(in.target),
		"dc":     in.dc,
		"total":  in.total,
		"beaten": in.beaten,
	}
	if in.calculation != nil {
		// Refused here rather than written: a beat is what the table saw, and
		// arithmetic that cannot have happened as told is not something
		// anybody saw. Absent stays absent — the key is omitted, never a
		// zero-valued calculation, which would be a third state downstream.
		if err := validateRecordedTotal(in.calculation, in.total); err != nil {
			return 0, fmt.Errorf("%s: calculation: %w", in.verb, err)
		}
		body["calculation"] = in.calculation
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("%s: marshal beat: %w", in.verb, err)
	}

	out, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: witnesses,
		Tags:     map[string]string{"tag": in.tag},
		Payload:  payload,
	})
	if err != nil {
		return 0, fmt.Errorf("%s: %w", in.verb, err)
	}

	return out.Seq, nil
}
