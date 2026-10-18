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

// option.go is THE ONE SOCIAL VERB (rpg-project#569, the mad king use case
// §2 and rulings R1–R3, R9): a line a character can SAY to a creature or a
// prop, the check that line rolls, what it waits on, and what is answered on
// either verdict.
//
// # It replaced two verbs and four table keys
//
// Intimidate and Persuade were one body ([Encounter.social], as it was) under
// two names, and what a creature answered was keyed by the VERB on its table
// (`intimidated`, `persuade_failed`, …). That could say one thing per verb per
// verdict and nothing else: a creature answered a threat the same way however
// it was phrased, and a line that rolled Performance or Religion had nowhere to
// live at all. An option is the line itself — flat, authored, gated by facts
// (R1) — and it carries its own answer rows (R2), so the next line is a row in
// the file and never a field here.
//
// # The deed is the skill, not the line
//
// A beaten check lands the deed of the SKILL it rolled ([deedForSkill], R3):
// Intimidation lands `intimidate` on every witness whatever the line said,
// Persuasion lands `persuade`, and every other skill lands nothing. What a
// held deed MEANS is still the mind's (rulebooks/dnd5e/behavior) — this verb
// lands testimony and forms no opinion.
//
// # The audience is the witnesses, and the target must be among them
//
// Who learns is [Encounter.witnessesOf] — sight and line of sight from the
// ACTOR's cell, the set [Encounter.landAttack] computes. A creature spoken to
// must be able to see who is speaking ([ErrUnwitnessed]); a PROP sees nothing,
// so speaking to one is gated by the actor seeing the prop (the probe law,
// [ErrNoProp]) and its audience is still whoever sees the actor — the fact an
// altar teaches lands on everyone who watched the character read it.
//
// # `requires` is read, never fired
//
// An option's [Option.Requires] is judged against the SPEAKER's own holdings
// (R9): a fact that speaker knows, a prop that speaker carries. It is asked at
// the offer ([Encounter.OptionsFor]) and again at the door ([Encounter.Speak]),
// and nowhere else — which is why [TriggerHolds] exists only here and is
// refused as an ending, an arrival or an until.

// Option is one line a character can say to a creature or a prop: what is
// said, the check it rolls, what it waits on, and what is answered on each
// verdict.
type Option struct {
	// ID names the line on its target. REQUIRED non-empty and unique among
	// one target's options — the session offers it by this id and
	// [SpeakInput.OptionID] names it back.
	ID string

	// Say is the character's line, carried VERBATIM onto the [BeatSpoke]
	// beat. REQUIRED non-empty: an option is something said.
	Say string

	// Check is the approaches through the line's check — the same list a
	// lock carries, priced per route. REQUIRED, at least one, each with a DC
	// of at least 1. CARRIED, NEVER COMPARED, as every check is here: the
	// session rolls it, and this composition is told whether it was beaten.
	Check []CheckApproach

	// Requires is what must hold for the SPEAKER before the line is offered
	// at all: nil for always, else a [TriggerFact] (the speaker knows the
	// fact) or a [TriggerHolds] (the speaker carries the prop). No other
	// trigger is a requirement, and the door refuses one ([ErrBadMember] on
	// a member, [ErrNoField] on a prop).
	Requires Trigger `json:"Requires,omitempty"`

	// Success is the rows answered when the check was beaten, rolled under
	// [AnswerSuccess]. Nil answers nothing.
	Success []Answer `json:"Success,omitempty"`

	// Failure is the rows answered when the check was missed, rolled under
	// [AnswerFailure]. Nil answers nothing.
	Failure []Answer `json:"Failure,omitempty"`
}

// TriggerHolds is a requirement that the speaker CARRIES a prop
// (`requires: { holds: <prop id> }`, rpg-project#569).
//
// READ, NEVER EDGE-FIRED. It is asked of one member at the moment an option
// is offered or spoken, and of no one at any other time: nothing waits for it,
// so it is refused as an ending, an arrival and an until. The prop must be one
// this field declares holdable — a requirement nobody can ever meet is an
// option nobody can ever be offered, refused at the door it came in through.
type TriggerHolds struct {
	// Prop is the prop the speaker must be carrying. REQUIRED non-empty.
	Prop PropID
}

// isTrigger marks TriggerHolds as a Trigger.
func (t TriggerHolds) isTrigger() {}

// BeatSpoke is the "beat" value of the story beat this composition appends
// when somebody speaks an option — landed or not.
//
// EXPORTED BECAUSE A DECODER READS IT: a session-side decoder is written
// against it in the same wave, so a rename fails to compile there instead of
// quietly producing a beat nobody renders.
//
// IT IS THE ONLY ACCOUNT OF THE ROLL. A missed line writes no deed and, when
// the author wrote no failure rows, nothing else at all — a client that cannot
// decode this beat cannot tell the table what happened.
const BeatSpoke = "spoke"

// verbSpeak is the bare verb name the answered beat carries for a spoken
// answer, and the tag the spoke beat goes down under.
const verbSpeak = "speak"

// Witnesses is every member whose senses reach a member's own cell: who would
// learn what that member does where they stand.
//
// A READ, and the one the caller needs BEFORE it does anything expensive. The
// session prices a line in the actor's action and must know the target can
// see them before it charges — a verb that takes the action and then refuses
// is one nobody would forgive — and the same list is what an action panel
// offers as the people you can talk to. [Encounter.Speak] asks again for
// itself and does not trust the answer it handed out: this is a read of a
// moment, and a door may close between the two calls.
//
// Sorted, and includes the member themselves whenever they can see their own
// cell — which is every placed member, and is why the audience of a `spoke`
// beat has the actor in it.
//
// Errors: ErrNotMember (no such member), ErrBadPlacement (not placed).
func (e *Encounter) Witnesses(of MemberID) ([]MemberID, error) {
	_, witnesses, err := e.audienceOf(of)
	if err != nil {
		return nil, fmt.Errorf("witnesses: %w", err)
	}

	return witnesses, nil
}

// OptionsFor is the options a target creature offers one speaker: every
// option on the target whose [Option.Requires] holds for the SPEAKER (R9), in
// authored order, deep-copied.
//
// A CREATURE WITH NO OPTIONS OFFERS NONE, and that is an empty list and a nil
// error — what the absence means (the shortfall a client is shown) is the
// offer side's to decide. Whether the target can SEE the speaker is not asked
// here: that is [Encounter.Witnesses]' question, asked by whoever prices the
// line, and asked again by [Encounter.Speak] at the door.
//
// Errors: ErrNotMember (no such speaker or target).
func (e *Encounter) OptionsFor(actor, target MemberID) ([]Option, error) {
	if _, ok := e.members[actor]; !ok {
		return nil, fmt.Errorf("options for: actor %q: %w", actor, ErrNotMember)
	}
	record, ok := e.members[target]
	if !ok {
		return nil, fmt.Errorf("options for: target %q: %w", target, ErrNotMember)
	}

	return e.offered(actor, record.Options), nil
}

// PropOptionsFor is the options a prop offers one speaker: every option on
// the prop whose [Option.Requires] holds for the speaker, in authored order,
// deep-copied.
//
// THE PROBE LAW FIRST. A prop the speaker cannot be shown — one that names
// nothing, is hidden from them, waits in reserve, or is in somebody else's
// hands — is refused exactly as an id that names nothing ([ErrNoProp]), so an
// offer cannot confirm a thing no map shows. A prop the speaker carries is
// always shown to them.
//
// Errors: ErrNotMember (no such speaker), ErrNoProp (no prop this speaker can
// be shown).
func (e *Encounter) PropOptionsFor(actor MemberID, prop PropID) ([]Option, error) {
	if _, ok := e.members[actor]; !ok {
		return nil, fmt.Errorf("prop options for: actor %q: %w", actor, ErrNotMember)
	}
	options, err := e.speakableProp(actor, prop)
	if err != nil {
		return nil, fmt.Errorf("prop options for: %w", err)
	}

	return e.offered(actor, options), nil
}

// offered filters a target's options to the ones whose requirement holds for
// the speaker, deep-copied so a caller's edit cannot reach the field.
func (e *Encounter) offered(actor MemberID, options []Option) []Option {
	out := make([]Option, 0, len(options))
	for _, o := range options {
		if e.requirementHolds(actor, o.Requires) {
			out = append(out, cloneOption(o))
		}
	}

	return out
}

// requirementHolds answers one option's `requires` for one speaker, against
// THAT SPEAKER's own holdings (R9): the facts they know, the props they carry.
// Nil holds always. A trigger of any other kind holds never — the door
// refuses one at construction, so this arm is the fail-closed answer for a
// value that could only arrive by hand.
func (e *Encounter) requirementHolds(actor MemberID, requires Trigger) bool {
	switch t := requires.(type) {
	case nil:
		return true
	case TriggerFact:
		return e.world.knowsFact(actor, t.Fact)
	case TriggerHolds:
		return slices.Contains(e.heldPropsOf(actor), t.Prop)
	default:
		return false
	}
}

// speakableProp answers a prop's options when the speaker may address it, and
// ErrNoProp — the one refusal an id that names nothing earns — when they may
// not: no such prop, hidden from them, in reserve, held by somebody else, or
// standing nowhere they are shown.
//
// THE SAME QUESTIONS [Encounter.Hold] ASKS, minus reach: a character may read
// an altar across a lit hall, and the DC is the line's, never the range's.
func (e *Encounter) speakableProp(actor MemberID, prop PropID) ([]Option, error) {
	refused := fmt.Errorf("prop %q: %w", prop, ErrNoProp)
	if prop == "" || e.hiddenPropTo(actor, prop) {
		return nil, refused
	}
	if index := e.field.propIndexOf(prop); index >= 0 {
		p := e.field.props[index]
		if slices.Contains(e.heldPropsOf(actor), prop) {
			return p.Options, nil
		}
		cell := e.field.cellAt(p.At)
		placement, moved := e.holdings.propPlacements()[prop]
		if p.Arrives != nil && !placement.arrived {
			return nil, refused
		}
		if moved && placement.gone {
			return nil, refused
		}
		if moved {
			cell = placement.at
		}
		shown, err := e.showsCellTo(actor, cell)
		if err != nil {
			return nil, err
		}
		if !shown {
			return nil, refused
		}

		return p.Options, nil
	}
	if index := e.field.placedIndexOf(prop); index >= 0 {
		p := &e.field.placed[index]
		if slices.Contains(e.heldPropsOf(actor), prop) {
			return p.options, nil
		}
		now := e.field.placedNow()
		placement, standing := now.stands(p)
		if !standing {
			return nil, refused
		}
		shown, err := e.showsAnyCellTo(actor, e.field.placedCells(placement))
		if err != nil {
			return nil, err
		}
		if !shown {
			return nil, refused
		}

		return p.options, nil
	}

	return nil, refused
}

// SpeakInput names who spoke, to what, which line, and the check the session
// rolled for it.
type SpeakInput struct {
	// Actor is the member speaking. Must be a member (ErrNotMember) and
	// placed (ErrBadPlacement).
	Actor MemberID

	// Target is the creature spoken to, or empty when the line is said to a
	// prop. EXACTLY ONE of Target and Prop (ErrNoMember otherwise). A target
	// must be able to SEE the actor ([ErrUnwitnessed]).
	Target MemberID

	// Prop is the prop spoken to, or empty when the line is said to a
	// creature. The actor must be shown it ([ErrNoProp]).
	Prop PropID

	// OptionID names the line, on the target. It must be one the target
	// carries AND whose requirement holds for this actor ([ErrNoOption]) —
	// the offer and the door agree.
	OptionID string

	// Skill is the approach the check was made with — one of the option's
	// [CheckApproach.Ability] words ([ErrNoOption] otherwise). It is what the
	// deed is decided by ([deedForSkill], R3) and what the beat reports.
	Skill string

	// Beaten is whether the check beat the DC. CARRIED, NEVER COMPARED: this
	// module holds no 5e, so who decides a total beats a difficulty lives on
	// the other side of this seam. FALSE LANDS NO DEED and is not an error;
	// it answers from the option's failure rows.
	Beaten bool

	// DC and Total are the numbers the table sees, carried onto the beat so
	// the roll is visible whether it landed or not. Nothing here reads them.
	DC    int
	Total int

	// Calculation is the full sourced arithmetic behind Total — the d20 pool
	// with every face it threw and the keep record naming the rule that
	// decided which one counted (rpg-project#462). Optional; when present it
	// must describe Total and open with a d20 pool, or the line is refused
	// rather than written down wrong.
	Calculation *RollCalculation

	// Roller is THE WORLD'S DIE, the one the answer rows are picked with.
	// REQUIRED — supplied, never defaulted ([ErrNoRoller]): a silent default
	// puts untestable randomness into a result that looks fine, and the roll
	// is shown to the table. Per call, because the caller that rolled the
	// check is the caller that holds the die.
	Roller dice.Roller
}

// SpeakOutput reports what the line reached.
type SpeakOutput struct {
	// Beaten echoes what the caller said, so a caller reads the result off
	// the answer rather than off the fact that it called.
	Beaten bool

	// Witnesses is every member who saw it — who holds the deed on a
	// success, who learns a fact the answer teaches, and who was told
	// regardless. Sorted, the audience's own order.
	Witnesses []MemberID

	// Deed is the deed verb that landed on the witnesses — [DeedIntimidate]
	// or [DeedPersuade] — or empty when none did: a miss, a skill that lands
	// none, or a prop.
	Deed string

	// Seq is the sequence number of the `spoke` beat. A missed line gets one
	// too: somebody tried, and the story is what happened.
	Seq uint64

	// Answered is the row the world's die picked from the verdict's rows,
	// with the roll — nil when the option has no rows for that verdict.
	Answered *Pick
}

// Speak reports a line said to a creature or a prop, lands the deed its skill
// lands on a beaten check, and rolls what the author wrote is answered.
//
// Validation order (R5 atomicity — every refusal before the first write): nil
// input → no actor, or not exactly one of target and prop → no option id → no
// roller → closed → actor is a member → target is a member and not the actor
// → the actor is placed → the option is on the target (a prop: the actor is
// shown it) and its requirement holds for the actor → the skill is one of its
// approaches → a creature target can see the actor → the beat → the deed → the
// answer → the world's price for an action.
//
// THE BEAT IS APPENDED BEFORE ITS CONSEQUENCES, the law
// [Encounter.refreshSight] states: the line is the cause, and a stance beat
// ahead of the line that explains it would be a story told backwards. The
// ANSWERED beat comes after both, because it is the result (answer.go).
//
// Errors: ErrNilInput, ErrNoMember, ErrNoOption, ErrNoRoller, ErrClosed,
// ErrNotMember, ErrBadPlacement, ErrNoProp, ErrUnwitnessed, and a refused
// Calculation.
func (e *Encounter) Speak(ctx context.Context, in *SpeakInput) (*SpeakOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("speak: %w", ErrNilInput)
	}
	if in.Actor == "" {
		return nil, fmt.Errorf("speak: no actor: %w", ErrNoMember)
	}
	if (in.Target == "") == (in.Prop == "") {
		return nil, fmt.Errorf("speak: a line is said to exactly one creature or prop: %w", ErrNoMember)
	}
	if in.OptionID == "" {
		return nil, fmt.Errorf("speak: no line named: %w", ErrNoOption)
	}
	if in.Roller == nil {
		return nil, fmt.Errorf("speak: the world rolls the answer: %w", ErrNoRoller)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("speak: %w", ErrClosed)
	}
	if _, ok := e.members[in.Actor]; !ok {
		return nil, fmt.Errorf("speak: actor %q: %w", in.Actor, ErrNotMember)
	}
	var options []Option
	if in.Target != "" {
		target, ok := e.members[in.Target]
		if !ok {
			return nil, fmt.Errorf("speak: target %q: %w", in.Target, ErrNotMember)
		}
		// Talking to yourself is not a shenanigan, it is a caller defect —
		// and it would land a deed naming its own holder, which no mind can
		// read as anything.
		if in.Actor == in.Target {
			return nil, fmt.Errorf("speak: actor %q cannot address itself: %w", in.Actor, ErrNotMember)
		}
		options = target.Options
	}

	where, witnesses, err := e.audienceOf(in.Actor)
	if err != nil {
		return nil, fmt.Errorf("speak: %w", err)
	}

	if in.Prop != "" {
		options, err = e.speakableProp(in.Actor, in.Prop)
		if err != nil {
			return nil, fmt.Errorf("speak: %w", err)
		}
	}
	option, ok := e.optionOffered(in.Actor, options, in.OptionID)
	if !ok {
		return nil, fmt.Errorf("speak: %q is not a line this speaker is offered: %w", in.OptionID, ErrNoOption)
	}
	if !slices.ContainsFunc(option.Check, func(a CheckApproach) bool { return a.Ability == in.Skill }) {
		return nil, fmt.Errorf("speak: line %q offers no %q approach: %w", in.OptionID, in.Skill, ErrNoOption)
	}
	if in.Target != "" && !slices.Contains(witnesses, in.Target) {
		return nil, fmt.Errorf("speak: target %q cannot see the actor: %w", in.Target, ErrUnwitnessed)
	}

	// SPEAKING SHOWS YOU (sneaking R7): the mode ends before the line is
	// said, so whatever this verb refreshes sees the speaker plainly.
	e.endSneaking(in.Actor)

	at := uint64(e.clock.ToData().HighWater)
	seq, err := e.appendSpokeBeat(in, option, witnesses, at)
	if err != nil {
		return nil, err
	}

	// THE DEED IS THE SKILL'S (R3), and only a creature can hold one about
	// being spoken to: a prop is persuaded of nothing.
	landed := ""
	if verb, lands := deedForSkill(in.Skill); lands && in.Beaten && in.Target != "" {
		if err := e.landDeed(verb, in.Actor, in.Target, where, witnesses); err != nil {
			return nil, fmt.Errorf("speak: %w", err)
		}
		landed = verb
	}

	key, rows := AnswerFailure, option.Failure
	if in.Beaten {
		key, rows = AnswerSuccess, option.Success
	}
	answered, err := e.answer(ctx, answerInput{
		creature: in.Target, prop: in.Prop, actor: in.Actor,
		option: option.ID, key: key, rows: rows,
		verb: verbSpeak, beaten: in.Beaten,
		witnesses: witnesses, at: at, roller: in.Roller,
	})
	if err != nil {
		return nil, fmt.Errorf("speak: %w", err)
	}

	// THE WORLD'S PRICE FOR AN ACTION, paid after the answer has landed — the
	// fact it taught, the deed it lodged, the running it set up — and before
	// anything refreshes sight (design §5, worldtime.go). It is what makes a
	// cowed goblin keep running while the party talks: the line that scared
	// it pays the round its own `time` table then spends.
	//
	// Nothing at all for a member inside a fight, where the round prices time.
	if err := e.spendWorldAction(in.Actor); err != nil {
		return nil, fmt.Errorf("speak: %w", err)
	}

	return &SpeakOutput{
		Beaten: in.Beaten, Witnesses: witnesses, Deed: landed, Seq: seq, Answered: answered.pick,
	}, nil
}

// optionOffered finds an option by id among a target's options, and answers
// it only when its requirement holds for the speaker — the door asking the
// offer's own question, so the two cannot disagree.
func (e *Encounter) optionOffered(actor MemberID, options []Option, id string) (Option, bool) {
	for _, o := range options {
		if o.ID == id {
			return o, e.requirementHolds(actor, o.Requires)
		}
	}

	return Option{}, false
}

// appendSpokeBeat writes what the table saw: who said what to whom, the skill,
// the DC, the total, and whether it landed.
//
// EVERY WITNESS IS THE AUDIENCE, not just the two parties — everyone in the
// set learns what happened. The numbers are written unconditionally, false
// beside a miss included, and `target` and `prop` are both written with the
// one not spoken to empty, for the reason a struck beat writes `critical:
// false`: absent must not become a third state for a reader downstream.
func (e *Encounter) appendSpokeBeat(in *SpeakInput, option Option, witnesses []MemberID, at uint64) (uint64, error) {
	body := map[string]interface{}{
		"beat":   BeatSpoke,
		"actor":  string(in.Actor),
		"target": string(in.Target),
		"prop":   string(in.Prop),
		"option": option.ID,
		"line":   option.Say,
		"skill":  in.Skill,
		"dc":     in.DC,
		"total":  in.Total,
		"beaten": in.Beaten,
	}
	if in.Calculation != nil {
		// Refused here rather than written: a beat is what the table saw, and
		// arithmetic that cannot have happened as told is not something
		// anybody saw. Absent stays absent — the key is omitted, never a
		// zero-valued calculation, which would be a third state downstream.
		if err := validateRecordedTotal(in.Calculation, in.Total); err != nil {
			return 0, fmt.Errorf("speak: calculation: %w", err)
		}
		body["calculation"] = in.Calculation
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("speak: marshal beat: %w", err)
	}

	out, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: witnesses,
		Tags:     map[string]string{"tag": verbSpeak},
		Payload:  payload,
	})
	if err != nil {
		return 0, fmt.Errorf("speak: %w", err)
	}

	return out.Seq, nil
}

// validateOptions refuses a list of options this composition could not run:
// a line with no id or two lines under one, a line that says nothing, a check
// with no way through or an approach with nothing to beat, a requirement that
// is not a fact or a holding, and a row its verdict could not roll.
//
// AT THE DOOR, NOT AT THE ROLL — [validateTable]'s reason. A line that can
// never be spoken is a misconfiguration, and the moment to find out is when
// its holder enters, not when a player finally picks it. The authoring
// dialect refuses the same things in the author's own words; this is the
// composition refusing them for every other caller, a persisted blob somebody
// edited included.
//
// WHETHER A HELD PROP EXISTS is not asked here: it is a question about the
// field, asked by [field.validateOptionHolds] once the field is compiled.
func validateOptions(options []Option) error {
	seen := make(map[string]bool, len(options))
	for i, o := range options {
		if o.ID == "" {
			return fmt.Errorf("options[%d] has no id", i)
		}
		if seen[o.ID] {
			return fmt.Errorf("option %q is declared twice", o.ID)
		}
		seen[o.ID] = true
		if o.Say == "" {
			return fmt.Errorf("option %q does not say what the character says", o.ID)
		}
		if len(o.Check) == 0 {
			return fmt.Errorf("option %q has no way to make its check", o.ID)
		}
		for _, a := range o.Check {
			if a.Ability == "" {
				return fmt.Errorf("option %q has an approach that rolls nothing", o.ID)
			}
			if a.DC < 1 {
				return fmt.Errorf("option %q approach %q at DC %d has nothing to beat", o.ID, a.Ability, a.DC)
			}
		}
		switch t := o.Requires.(type) {
		case nil:
		case TriggerFact:
			if t.Fact == "" {
				return fmt.Errorf("option %q requires a fact and names none", o.ID)
			}
		case TriggerHolds:
			if t.Prop == "" {
				return fmt.Errorf("option %q requires holding a prop and names none", o.ID)
			}
		default:
			return fmt.Errorf("option %q requires a %T: an option requires a fact or a holding", o.ID, o.Requires)
		}
		for _, verdict := range []struct {
			key  AnswerKey
			rows []Answer
		}{{AnswerSuccess, o.Success}, {AnswerFailure, o.Failure}} {
			for j, row := range verdict.rows {
				if err := validateEntry(verdict.key, j, row); err != nil {
					return fmt.Errorf("option %q: %w", o.ID, err)
				}
			}
		}
	}

	return nil
}

// validatePropOptions is [validateOptions] for a prop's lines, plus the two
// things only a prop can get wrong: it must be nameable, since a line is
// spoken to it by id, and none of its rows may `flee` — a prop cannot run.
// Refused with [ErrNoField], the door a prop comes in through.
func validatePropOptions(id PropID, ref string, options []Option) error {
	if len(options) == 0 {
		return nil
	}
	if id == "" {
		return fmt.Errorf("prop %q carries options and has no id: %w", ref, ErrNoField)
	}
	if err := validateOptions(options); err != nil {
		return fmt.Errorf("prop %q: %w: %w", id, err, ErrNoField)
	}
	for _, o := range options {
		for _, row := range append(append([]Answer(nil), o.Success...), o.Failure...) {
			if row.Flee {
				return fmt.Errorf("prop %q option %q: a prop cannot run: %w", id, o.ID, ErrNoField)
			}
		}
	}

	return nil
}

// validateOptionHolds refuses an option requiring a holding of a prop this
// field does not declare holdable — a requirement nobody can ever meet, which
// is an option nobody can ever be offered. Asked once the field is compiled,
// because only then is the holdable index whole.
func (f *field) validateOptionHolds(options []Option) error {
	for _, o := range options {
		t, ok := o.Requires.(TriggerHolds)
		if !ok {
			continue
		}
		if f.propIndexOf(t.Prop) < 0 && f.placedIndexOf(t.Prop) < 0 {
			return fmt.Errorf("option %q requires holding %q, and no prop has that id", o.ID, t.Prop)
		}
		if !f.holdable[t.Prop] {
			return fmt.Errorf("option %q requires holding %q, which nobody can pick up", o.ID, t.Prop)
		}
	}

	return nil
}

// cloneOptions deep-copies a list of options so a caller's slice and the
// composition's cannot be the same one — [MemberInput]'s standing rule that
// nothing crossing this door stays aliased. Nil stays nil.
func cloneOptions(options []Option) []Option {
	if options == nil {
		return nil
	}
	out := make([]Option, 0, len(options))
	for _, o := range options {
		out = append(out, cloneOption(o))
	}

	return out
}

// cloneOption deep-copies one option: its approaches and both verdicts' rows.
func cloneOption(o Option) Option {
	out := o
	out.Check = copyApproaches(o.Check)
	out.Success = cloneRows(o.Success)
	out.Failure = cloneRows(o.Failure)

	return out
}

// cloneRows deep-copies one verdict's rows. Nil stays nil.
func cloneRows(rows []Answer) []Answer {
	if rows == nil {
		return nil
	}
	out := make([]Answer, 0, len(rows))
	for _, row := range rows {
		out = append(out, cloneAnswer(row))
	}

	return out
}

// optionFacts is every fact id a list of options can teach — what joins
// [mintedFactIDs] so a world blob may name a fact only a line plants.
func optionFacts(options []Option) []FactID {
	var out []FactID
	for _, o := range options {
		for _, row := range append(append([]Answer(nil), o.Success...), o.Failure...) {
			if row.Fact != "" {
				out = append(out, row.Fact)
			}
		}
	}

	return out
}
