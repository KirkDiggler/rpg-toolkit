// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/selection"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// interaction_hold.go is RESERVE BEFORE DICE (rpg-project#924, Gate S4 of #769; design
// #916 R5, "a choice is validated and reserved before dice"): the
// conversation's revision, the one checked line an interaction holds while
// its speaker decides on their roll, and the stale reply refused.
//
// # The revision
//
// Every interaction instance carries a revision, one or more, persisted on
// [InteractionData.Revision]. It moves whenever the instance's progress moves
// (an [Advance] whose transition fired) and whenever a reservation is placed,
// settled or released. An address does not move it: a hail changes nobody's
// lines. Every line that reads or advances an interaction is offered with that
// interaction's id and revision ([OfferedOption]), and is sent back with the
// revision it was offered at. A reply from an older revision is REFUSED
// ([ErrStaleOption]) before anything is charged, rolled or reserved, and never
// re-read against the state as it is now: the castle's `troupe` claim is a
// warning from `fresh` and an arrest from `warned-guest`, so a re-read reply
// would change the stakes under the player's finger (operator, 2026-11-11).
//
// A line that touches no interaction carries no revision and is never
// revision-checked: whatever it is sent with is not read.
//
// # One interaction per line
//
// A line may read or advance at most ONE interaction, across its `requires`
// and its `advance` — refused at the door otherwise ([validateOptions]). One
// revision per line is the honest shape for one conversation per line; a line
// that straddled two conversations would need a revision per conversation on
// the wire, and no content does it.
//
// # The reservation
//
// When a checked line pauses on its speaker's roll decision (a held benefit
// such as Bardic Inspiration), the host reserves the line
// ([Encounter.Reserve]): the instance records who holds it, which line, said
// to whom, and the revision it was offered at. While it stands, EVERY line
// that reads or advances that interaction is PENDING — evaluated
// [selection.Unknown], so [selection.Choose] refuses it ([ErrReserved]) — for
// every speaker, the holder included: the holder's own other lines wait on
// their own roll. Lines that touch no interaction stay open (operator,
// 2026-11-11: "yeah A makes sense to me"). The ONE line admitted is the
// settle: [Encounter.Speak] of the reserved line, by its holder, to the same
// creature or prop, at the reserved revision. Its verdict advances progress
// once, then the reservation clears and the revision moves; a second settle
// meets a moved revision and is refused stale.
//
// A PENDING LINE IS ONE THAT WAS OFFERED. Every other requirement of the line
// is still judged as it is now, and a line with any requirement unmet is
// absent, never pending — so a reservation cannot reveal a line the speaker
// was not offered (the probe law).
//
// A pending line names who it waits on ONLY to a speaker who perceives them
// (the holder's witnesses, the held beat's audience) or to the holder.
//
// # The doors agree, and the encounter releases what they would refuse
//
// Speak, [Encounter.ChooseOption] and [Encounter.Reserve] ask ONE door
// ([Encounter.replyDoor]): placement, the line, the revision, pending,
// witnessing and reach — so a reservation is never placed whose settle the
// door would refuse. And the encounter does not wait for the host to
// remember: at every participation pass and every exit
// ([Encounter.sweepHolds]) a reservation whose settle that door would refuse
// now — its target gone or down, the holder out of the reach it reserved
// within or out of the target's sight — is released without a verdict. A
// fight alone releases nothing: Speak does not refuse a line said to a
// creature in a fight. The host still releases an abandoned window
// ([Encounter.ReleaseReservation]); a holder who exits drops their holds.

// BeatInteractionHeld is the "beat" value of the story beat appended when a
// reservation is placed (`held: true`) or released without a verdict (`held:
// false`): the interaction, the holder and the revision now. The line itself
// is NOT named — nothing has been said yet. A settle writes no held beat: the
// `spoke` beat and, when the progress moved, the [BeatInteraction] beat
// already tell that story. EXPORTED BECAUSE A DECODER READS IT.
const BeatInteractionHeld = "interaction-held"

// InteractionReservation is the one checked line an interaction holds while
// its speaker decides on their roll ([InteractionData.Reservation]).
type InteractionReservation struct {
	// Holder is the member whose roll the line waits on.
	Holder MemberID `json:"holder"`

	// Target is the creature the line is said to; empty for a prop.
	Target MemberID `json:"target,omitzero"`

	// Prop is the prop the line is said to; empty for a creature.
	Prop PropID `json:"prop,omitzero"`

	// Option is the line's id on its target or prop.
	Option string `json:"option"`

	// Revision is the interaction's revision the line was offered and
	// reserved at — the revision its settle must carry. Always below the
	// instance's revision, since placing the reservation moved it.
	Revision uint64 `json:"revision"`

	// Reach is the host's reach the line was reserved within, measured as
	// [SpeakInput.Range] is (zero means adjacent): the reach the settle is
	// asked at, and the reach [Encounter.sweepHolds] keeps it to.
	Reach int `json:"reach,omitzero"`
}

// OfferedOption is one line offered to one speaker: the option, the
// interaction it reads or advances with that interaction's revision now, and
// whether it is pending on somebody's waiting roll.
type OfferedOption struct {
	Option

	// Interaction is the one interaction the line reads or advances; empty
	// for a line that touches none.
	Interaction InteractionID

	// Revision is [OfferedOption.Interaction]'s revision now — what a reply
	// to this line sends back. Zero for a line that touches no interaction.
	Revision uint64

	// Pending is whether the line waits on somebody's roll: shown, and
	// refused if sent ([ErrReserved]) until the check settles or is
	// released.
	Pending bool

	// PendingOn is who the pending line waits on, ONLY when the asking
	// speaker perceives them — the rule a [BeatInteractionHeld] beat's
	// audience is chosen by (the holder's witnesses) — or is the holder
	// themself. Empty otherwise, and for a line that is not pending: a
	// speaker is never told the name of somebody they cannot perceive.
	PendingOn MemberID
}

// optionInteractions is every distinct interaction a line reads (a
// `requires`, a `not` of one included) or advances, in first-named order.
func optionInteractions(o Option) []InteractionID {
	var out []InteractionID
	add := func(id InteractionID) {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	for _, requires := range o.Requires {
		inner, _ := unwrapNot(requires)
		if t, ok := inner.(TriggerInteraction); ok {
			add(t.Interaction)
		}
	}
	if o.Advance != nil {
		add(o.Advance.Interaction)
	}
	return out
}

// optionInteraction is the one interaction a line reads or advances, and
// whether it touches one. The door refuses a line touching two.
func optionInteraction(o Option) (InteractionID, bool) {
	touched := optionInteractions(o)
	if len(touched) == 0 {
		return "", false
	}
	return touched[0], true
}

// heldRequirement is the requirement a line touching a reserved interaction
// carries: Unknown, the pending verdict, and never Unmet.
func heldRequirement(id InteractionID) selection.Requirement {
	return selection.Requirement{ID: "held:" + string(id), Verdict: selection.Unknown, Reason: "held"}
}

// holdOf is the reservation standing on the interaction a line touches, or
// nil when the line touches none or nothing is reserved there.
func (e *Encounter) holdOf(o Option) *InteractionReservation {
	id, ok := optionInteraction(o)
	if !ok {
		return nil
	}
	r, ok := e.interactions[id]
	if !ok {
		return nil
	}
	return r.reservation
}

// settleFor is the reservation a Speak settles: the one standing on the
// interaction the named line touches, held by this speaker for this very
// line said to this very creature or prop. Nil when the Speak is not a
// settle.
func (e *Encounter) settleFor(in lineReply, options []Option) *InteractionReservation {
	i := slices.IndexFunc(options, func(o Option) bool { return o.ID == in.optionID })
	if i < 0 {
		return nil
	}
	hold := e.holdOf(options[i])
	if hold == nil || hold.Holder != in.actor || hold.Target != in.target || hold.Prop != in.prop || hold.Option != in.optionID {
		return nil
	}
	return hold
}

// hasUnmet reports whether a judged row carries any Unmet requirement: a
// line not offered, as against one pending.
func hasUnmet(row selection.Row) bool {
	return slices.ContainsFunc(row.Requirements, func(r selection.Requirement) bool { return r.Verdict == selection.Unmet })
}

// lineDoor is THE ONE DOOR a reply to an offered line comes through
// ([Encounter.Speak], [Encounter.ChooseOption], [Encounter.Reserve]), in
// this order: the line is on the target and every requirement is Met or
// pending ([ErrNoOption] otherwise, so the door cannot confirm a gated line
// exists); a line touching an interaction carries that interaction's
// revision now — or, for the settle, the reserved revision
// ([ErrStaleOption]); and the line is not pending ([ErrReserved]).
//
// THE STALE REFUSAL COMES BEFORE THE PENDING ONE: a reply from before the
// reservation was placed is a reply from an older conversation, and the
// client refreshes; a reply at the current revision to a line it was shown
// pending is the one refused [ErrReserved].
func (e *Encounter) lineDoor(
	actor, other MemberID, options []Option, id string, revision uint64, settle *InteractionReservation,
) (Option, error) {
	evaluation, err := e.evaluateOptions(actor, other, options, settle)
	if err != nil {
		return Option{}, err
	}
	inspection, err := evaluation.Inspect()
	if err != nil {
		return Option{}, fmt.Errorf("inspect options: %w", err)
	}
	i := slices.IndexFunc(inspection.Rows, func(r selection.Row) bool { return r.ID == id })
	if i < 0 || hasUnmet(inspection.Rows[i]) {
		return Option{}, fmt.Errorf("%q is not a line this speaker is offered: %w", id, ErrNoOption)
	}
	option := options[i]
	if iid, ok := optionInteraction(option); ok {
		r, ok := e.interactions[iid]
		if !ok {
			return Option{}, fmt.Errorf("line %q reads interaction %q, and the run has no such interaction", id, iid)
		}
		want := r.revision
		if settle != nil && settle == r.reservation && settle.Option == id {
			want = settle.Revision
		}
		if revision != want {
			return Option{}, fmt.Errorf("line %q was offered at revision %d of interaction %q, which is at %d: %w",
				id, revision, iid, want, ErrStaleOption)
		}
	}
	choice, err := selection.Choose(&selection.ChooseInput{Evaluation: evaluation, ID: id})
	if errors.Is(err, selection.ErrIneligible) {
		return Option{}, fmt.Errorf("%q waits on a roll: %w", id, ErrReserved)
	}
	if errors.Is(err, selection.ErrUnknownID) {
		return Option{}, fmt.Errorf("%q is not a line this speaker is offered: %w", id, ErrNoOption)
	}
	if err != nil {
		return Option{}, fmt.Errorf("choose option %q: %w", id, err)
	}

	return options[choice.Index], nil
}

// settle clears the reservation a Speak settled and moves the revision:
// after the verdict has advanced the progress, so it advanced it exactly
// once.
func (e *Encounter) settle(hold *InteractionReservation, option Option) {
	if hold == nil {
		return
	}
	id, _ := optionInteraction(option)
	r := e.interactions[id]
	r.reservation = nil
	r.revision++
}

// lineSite answers the lines said at one creature or prop for one speaker,
// with the doors every verb asks before a line: exactly one of target and
// prop, the encounter open, the actor a member, a creature target a member
// who is not the actor and not down, and a prop the actor is shown (the
// probe law), with the cells it stands on (nil for one the actor carries).
func (e *Encounter) lineSite(verb string, actor, target MemberID, prop PropID) ([]Option, []spatial.Position, error) {
	if actor == "" {
		return nil, nil, fmt.Errorf("%s: no actor: %w", verb, ErrNoMember)
	}
	if (target == "") == (prop == "") {
		return nil, nil, fmt.Errorf("%s: a line is said to exactly one creature or prop: %w", verb, ErrNoMember)
	}
	if e.outcome != nil {
		return nil, nil, fmt.Errorf("%s: %w", verb, ErrClosed)
	}
	if _, ok := e.members[actor]; !ok {
		return nil, nil, fmt.Errorf("%s: actor %q: %w", verb, actor, ErrNotMember)
	}
	if prop != "" {
		options, cells, err := e.speakableProp(actor, prop)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", verb, err)
		}
		return options, cells, nil
	}
	t, ok := e.members[target]
	if !ok {
		return nil, nil, fmt.Errorf("%s: target %q: %w", verb, target, ErrNotMember)
	}
	// Talking to yourself is not a shenanigan, it is a caller defect — and it
	// would land a deed naming its own holder, which no mind can read as
	// anything.
	if actor == target {
		return nil, nil, fmt.Errorf("%s: actor %q cannot address itself: %w", verb, actor, ErrNotMember)
	}
	// A DOWN CREATURE ANSWERS NOTHING (rpg-project#718). It still carries its
	// lines — a fallen leader's faction lines are on it like every member's —
	// so without this door the body would "yield", learn the fact, and hold
	// the deed. Refused before anything is written.
	down, err := e.isDown(target)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", verb, err)
	}
	if down {
		return nil, nil, fmt.Errorf("%s: %q is down and cannot answer: %w", verb, target, ErrMemberDown)
	}
	return t.Options, nil, nil
}

// lineReply is one reply to an offered line, as every door reads it.
type lineReply struct {
	actor    MemberID
	target   MemberID
	prop     PropID
	optionID string
	revision uint64
	reach    int
}

// replied is what [Encounter.replyDoor] admitted: the line, the reservation
// it settles (nil unless it is the settle), and where the actor stands with
// who witnesses them there.
type replied struct {
	option    Option
	settle    *InteractionReservation
	where     spatial.Position
	witnesses []MemberID
}

// replyDoor is THE ONE DOOR a reply to a line comes through — [Encounter.Speak],
// [Encounter.ChooseOption], [Encounter.Reserve], and [Encounter.sweepHolds]
// asking whether a settle would still be admitted — so none of them can
// disagree. In order: the site ([Encounter.lineSite]) → a line named
// ([ErrNoOption]) → the actor placed ([ErrBadPlacement]) → the line door
// ([Encounter.lineDoor]: offered, current revision, not pending) → a creature
// target witnesses the actor ([ErrUnwitnessed]) → reach, LAST of the doors
// (rpg-project#753, R4), in [Encounter.Hold]'s order: only a target this
// speaker is shown and that witnesses them is measured, so a hidden thing far
// off still answers "no such thing" and never "out of range"
// ([ErrOutOfRange]).
//
// settling admits the settle of a reservation the actor holds for this very
// line at this very site ([Encounter.settleFor]); only Speak and the sweep
// pass it.
//
// THE CALLER HAS ALREADY SHOWN THE ACTOR (sneaking R7): the witnesses are
// asked as Speak asks them, with the speaker's sneaking ended.
func (e *Encounter) replyDoor(verb string, in lineReply, settling bool) (replied, error) {
	options, propCells, err := e.lineSite(verb, in.actor, in.target, in.prop)
	if err != nil {
		return replied{}, err
	}
	if in.optionID == "" {
		return replied{}, fmt.Errorf("%s: no line named: %w", verb, ErrNoOption)
	}
	where, witnesses, err := e.audienceOf(in.actor)
	if err != nil {
		return replied{}, fmt.Errorf("%s: %w", verb, err)
	}
	var settle *InteractionReservation
	if settling {
		settle = e.settleFor(in, options)
	}
	option, err := e.lineDoor(in.actor, in.target, options, in.optionID, in.revision, settle)
	if err != nil {
		return replied{}, fmt.Errorf("%s: %w", verb, err)
	}
	if in.target != "" && !slices.Contains(witnesses, in.target) {
		return replied{}, fmt.Errorf("%s: target %q cannot see the actor: %w", verb, in.target, ErrUnwitnessed)
	}
	if in.target != "" {
		err = e.refuseOutOfReach(verb, in.actor, in.target, in.reach)
	} else {
		err = e.refusePropOutOfReach(verb, where, in.prop, propCells, in.reach)
	}
	if err != nil {
		return replied{}, err
	}

	return replied{option: option, settle: settle, where: where, witnesses: witnesses}, nil
}

// asShown runs a read as though the actor had spoken — sneaking ended, as
// [Encounter.Speak] ends it before it asks the audience (sneaking R7) — and
// puts the mode back afterwards whatever happened: a door asked ahead of the
// line, or about a line waiting on its roll, has not shown anybody.
func (e *Encounter) asShown(actor MemberID, read func() error) error {
	m, ok := e.members[actor]
	if !ok || !m.Sneaking {
		return read()
	}
	verdicts := copySneakVerdicts(e.sneakVerdicts)
	e.endSneaking(actor)
	err := read()
	m.Sneaking = true
	e.sneakVerdicts = verdicts
	return err
}

// ChooseOptionInput names a reply to an offered line: who, to which creature
// or prop, which line, the revision it was offered at, and the reach the
// host measures speech within.
type ChooseOptionInput struct {
	// Actor is the speaker. REQUIRED, a member.
	Actor MemberID

	// Target is the creature the line is said to; EXACTLY ONE of Target and
	// Prop.
	Target MemberID

	// Prop is the prop the line is said to.
	Prop PropID

	// OptionID names the line. REQUIRED.
	OptionID string

	// Revision is the [OfferedOption.Revision] the line was offered at; not
	// read for a line that touches no interaction.
	Revision uint64

	// Range is the host's reach, exactly [SpeakInput.Range]: zero means
	// adjacent. The door measures it as Speak does, so the two agree.
	Range int
}

// ChooseOptionOutput is the line the reply chose, as offered now.
type ChooseOptionOutput struct {
	// Option is the chosen line, deep-copied, with its interaction and
	// revision now.
	Option OfferedOption
}

// ChooseOption is the line door asked BEFORE anything is charged or rolled
// (rpg-project#924): whether [Encounter.Speak] would admit this reply now,
// asked through the door Speak itself asks ([Encounter.replyDoor]), with the
// speaker shown as Speak shows them. A read: it writes nothing.
//
// Validation order: nil input → no actor, or not exactly one of target and
// prop → closed → actor is a member → a creature target is a member, not the
// actor and not down; a prop is shown to the actor → no option id → the
// actor is placed → the line is offered ([ErrNoOption]) → its revision is the
// interaction's now ([ErrStaleOption]) → it is not pending ([ErrReserved])
// → a creature target witnesses the actor → reach.
//
// Errors: ErrNilInput, ErrNoMember, ErrNoOption, ErrClosed, ErrNotMember,
// ErrMemberDown, ErrNoProp, ErrBadPlacement, ErrStaleOption, ErrReserved,
// ErrUnwitnessed, ErrOutOfRange.
func (e *Encounter) ChooseOption(in *ChooseOptionInput) (*ChooseOptionOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("choose option: %w", ErrNilInput)
	}
	var admitted replied
	err := e.asShown(in.Actor, func() error {
		var err error
		admitted, err = e.replyDoor("choose option", in.reply(), false)
		return err
	})
	if err != nil {
		return nil, err
	}

	return &ChooseOptionOutput{Option: e.offerOf(admitted.option, false, "")}, nil
}

// reply is the input as a [lineReply].
func (in *ChooseOptionInput) reply() lineReply {
	return lineReply{
		actor: in.Actor, target: in.Target, prop: in.Prop,
		optionID: in.OptionID, revision: in.Revision, reach: in.Range,
	}
}

// offerOf is one line as offered: deep-copied, with the interaction it
// touches and that interaction's revision now, whether it is pending, and
// who it waits on as the asker may be told it.
func (e *Encounter) offerOf(o Option, pending bool, pendingOn MemberID) OfferedOption {
	out := OfferedOption{Option: cloneOption(o), Pending: pending, PendingOn: pendingOn}
	if id, ok := optionInteraction(o); ok {
		out.Interaction = id
		if r, ok := e.interactions[id]; ok {
			out.Revision = r.revision
		}
	}
	return out
}

// pendingOnFor is who a held line waits on, as one asker may be told it: the
// holder when the asker is the holder or one of the holder's witnesses —
// the audience a [BeatInteractionHeld] beat goes to — else nobody.
func (e *Encounter) pendingOnFor(asker, holder MemberID) MemberID {
	if asker == holder {
		return holder
	}
	if _, witnesses, err := e.audienceOf(holder); err == nil && slices.Contains(witnesses, asker) {
		return holder
	}
	return ""
}

// ReserveInput names the checked line that paused on its speaker's roll:
// the same reply [ChooseOptionInput] names, at the revision it was offered,
// within the host's reach.
type ReserveInput = ChooseOptionInput

// ReserveOutput reports the reservation placed.
type ReserveOutput struct {
	// Interaction is the interaction now held; EMPTY when the line touches
	// none, in which case nothing was reserved and nothing written: a line
	// that reads no conversation holds none.
	Interaction InteractionID

	// Revision is the interaction's revision now, moved by the reservation;
	// zero when nothing was reserved.
	Revision uint64

	// Seq is the [BeatInteractionHeld] beat's sequence; zero when nothing
	// was reserved.
	Seq uint64
}

// Reserve holds the interaction a checked line touches while its speaker
// decides on their roll (rpg-project#924): the host calls it when the line's
// check pauses on a held benefit, after it has asked every door and charged
// the action. The reservation records the holder, the line, the creature or
// prop it is said to, the revision it was offered at and the reach; the
// revision moves; and a [BeatInteractionHeld] beat goes to the holder's
// witnesses. From then on every line touching that interaction is pending,
// for every speaker, until the holder's [Encounter.Speak] settles it,
// [Encounter.ReleaseReservation] ends it, or the encounter ends it itself
// because the settle could no longer be admitted ([Encounter.sweepHolds]).
//
// Validation order: [Encounter.ChooseOption]'s, whole — the door the settle
// will ask, so a reservation is never placed that its settle would refuse
// as it stands — then the line must roll a check ([ErrNoOption]: a plain line
// never waits on a roll).
//
// Errors: ErrNilInput and every error [Encounter.ChooseOption] returns.
func (e *Encounter) Reserve(in *ReserveInput) (*ReserveOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("reserve: %w", ErrNilInput)
	}
	var admitted replied
	err := e.asShown(in.Actor, func() error {
		var err error
		admitted, err = e.replyDoor("reserve", in.reply(), false)
		return err
	})
	if err != nil {
		return nil, err
	}
	option := admitted.option
	if !option.Checked() {
		return nil, fmt.Errorf("reserve: line %q rolls no check and never waits on one: %w", in.OptionID, ErrNoOption)
	}
	id, ok := optionInteraction(option)
	if !ok {
		return &ReserveOutput{}, nil
	}
	r := e.interactions[id]
	r.reservation = &InteractionReservation{
		Holder: in.Actor, Target: in.Target, Prop: in.Prop, Option: in.OptionID, Revision: in.Revision, Reach: in.Range,
	}
	r.revision++
	seq, err := e.appendHeldBeat(id, in.Actor, true)
	if err != nil {
		return nil, fmt.Errorf("reserve: %w", err)
	}

	return &ReserveOutput{Interaction: id, Revision: r.revision, Seq: seq}, nil
}

// ReleaseReservationInput names whose waiting line is released.
type ReleaseReservationInput struct {
	// Member is the holder. REQUIRED, a member.
	Member MemberID
}

// InteractionRevision is one interaction and its revision now.
type InteractionRevision struct {
	Interaction InteractionID
	Revision    uint64
}

// ReleaseReservationOutput reports every reservation released.
type ReleaseReservationOutput struct {
	// Released is each interaction the member held, with its revision now,
	// in id order. Never empty: a member holding none is refused.
	Released []InteractionRevision
}

// ReleaseReservation ends, WITHOUT A VERDICT, every reservation a member
// holds (rpg-project#924): nothing is advanced, each revision moves, the
// lines return to every speaker, and a [BeatInteractionHeld] beat with
// `held: false` goes to the holder's witnesses per interaction. The host
// calls it when the window is abandoned; the encounter releases on its own
// what its door would no longer admit ([Encounter.sweepHolds]).
//
// Errors: ErrNilInput, ErrNoMember (no member named), ErrNotMember,
// ErrNoReservation (the member holds none).
func (e *Encounter) ReleaseReservation(in *ReleaseReservationInput) (*ReleaseReservationOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("release reservation: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("release reservation: %w", ErrNoMember)
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("release reservation: member %q: %w", in.Member, ErrNotMember)
	}
	var released []InteractionRevision
	for _, id := range slices.Sorted(maps.Keys(e.interactions)) {
		hold := e.interactions[id].reservation
		if hold == nil || hold.Holder != in.Member {
			continue
		}
		revision, err := e.release(id)
		if err != nil {
			return nil, fmt.Errorf("release reservation: %w", err)
		}
		released = append(released, InteractionRevision{Interaction: id, Revision: revision})
	}
	if len(released) == 0 {
		return nil, fmt.Errorf("release reservation: member %q: %w", in.Member, ErrNoReservation)
	}

	return &ReleaseReservationOutput{Released: released}, nil
}

// release ends one interaction's reservation without a verdict: cleared,
// the revision moved, and a `held: false` [BeatInteractionHeld] beat to the
// holder's witnesses. It returns the revision now.
func (e *Encounter) release(id InteractionID) (uint64, error) {
	r := e.interactions[id]
	holder := r.reservation.Holder
	r.reservation = nil
	r.revision++
	if _, err := e.appendHeldBeat(id, holder, false); err != nil {
		return 0, err
	}
	return r.revision, nil
}

// dropHoldsOf clears every reservation a leaving member holds and moves each
// revision, writing nothing: the exit's own beat tells it, and a beat cannot
// be audienced to somebody no longer in the run.
func (e *Encounter) dropHoldsOf(member MemberID) {
	for _, id := range slices.Sorted(maps.Keys(e.interactions)) {
		r := e.interactions[id]
		if r.reservation == nil || r.reservation.Holder != member {
			continue
		}
		r.reservation = nil
		r.revision++
	}
}

// holdRefusals are the refusals of [Encounter.replyDoor] that mean the
// settle could no longer be admitted as the world stands: the target or the
// holder gone, down or unplaced, the prop no longer shown, the target no
// longer seeing the holder, out of reach. Any other error is a capability's
// or a defect's, surfaced rather than read as a release.
var holdRefusals = []error{
	ErrNotMember, ErrMemberDown, ErrBadPlacement, ErrNoProp, ErrUnwitnessed, ErrOutOfRange, ErrNoOption,
}

// sweepHolds releases every reservation whose settle [Encounter.replyDoor]
// would refuse now, each with its `held: false` beat, in interaction id
// order (rpg-project#924): THE ENCOUNTER DOES NOT LEAVE A ROLL HOLDING A
// CONVERSATION ITS VERDICT COULD NEVER LAND IN. Asked at every
// participation pass ([Encounter.noticeDown] — every step, strike, cast and
// clock pass runs one, so a fall, a walk out of reach and a lost line of
// sight are each noticed there) and after an exit. The holder is asked
// as Speak would ask them — shown — at the reach the line was reserved
// within. A closed encounter is not swept: nothing will be said in it again.
//
// Errors: a capability's or a beat's.
func (e *Encounter) sweepHolds() error {
	if e.outcome != nil {
		return nil
	}
	for _, id := range slices.Sorted(maps.Keys(e.interactions)) {
		hold := e.interactions[id].reservation
		if hold == nil {
			continue
		}
		in := lineReply{
			actor: hold.Holder, target: hold.Target, prop: hold.Prop,
			optionID: hold.Option, revision: hold.Revision, reach: hold.Reach,
		}
		err := e.asShown(hold.Holder, func() error {
			_, err := e.replyDoor("hold", in, true)
			return err
		})
		if err == nil {
			continue
		}
		if !slices.ContainsFunc(holdRefusals, func(refusal error) bool { return errors.Is(err, refusal) }) {
			return fmt.Errorf("interaction %q hold: %w", id, err)
		}
		if _, err := e.release(id); err != nil {
			return fmt.Errorf("interaction %q hold: %w", id, err)
		}
	}
	return nil
}

// appendHeldBeat writes a [BeatInteractionHeld] beat for one interaction to
// the holder's witnesses — the holder alone when they stand nowhere.
func (e *Encounter) appendHeldBeat(id InteractionID, holder MemberID, held bool) (uint64, error) {
	audience := []MemberID{holder}
	if _, witnesses, err := e.audienceOf(holder); err == nil {
		audience = witnesses
	}
	payload, err := json.Marshal(map[string]interface{}{
		"beat":        BeatInteractionHeld,
		"interaction": string(id),
		"holder":      string(holder),
		"held":        held,
		"revision":    e.interactions[id].revision,
	})
	if err != nil {
		return 0, fmt.Errorf("interaction %q: marshal held beat: %w", id, err)
	}
	out, err := e.appendBeat(&record.AppendInput{
		At: uint64(e.clock.ToData().HighWater), Audience: audience,
		Tags: map[string]string{"tag": BeatInteractionHeld}, Payload: payload,
	})
	if err != nil {
		return 0, fmt.Errorf("interaction %q: %w", id, err)
	}
	return out.Seq, nil
}

// cloneReservation copies a reservation so no caller's value is aliased.
func cloneReservation(r *InteractionReservation) *InteractionReservation {
	if r == nil {
		return nil
	}
	c := *r
	return &c
}

// validateReservationShape refuses a saved reservation nobody could have
// placed: no holder, no line, not exactly one of target and prop, or a
// revision not below the instance's (placing it moved the revision).
func validateReservationShape(id InteractionID, r InteractionReservation, revision uint64) error {
	switch {
	case r.Holder == "":
		return fmt.Errorf("interaction %q is saved reserved by nobody", id)
	case r.Option == "":
		return fmt.Errorf("interaction %q is saved reserved for no line", id)
	case (r.Target == "") == (r.Prop == ""):
		return fmt.Errorf("interaction %q is saved reserved for a line said to %q and %q; exactly one", id, r.Target, r.Prop)
	case r.Revision == 0 || r.Revision >= revision:
		return fmt.Errorf("interaction %q is saved at revision %d reserved at %d", id, revision, r.Revision)
	case r.Reach < 0:
		return fmt.Errorf("interaction %q is saved reserved within a negative reach %d", id, r.Reach)
	}
	return nil
}

// validateInteractionReservations refuses a saved reservation naming a
// holder or a creature the run does not have, a prop the field does not
// have, or a line its creature or prop does not carry or that does not touch
// the interaction it is saved on. options is every saved member's lines, by
// id.
func validateInteractionReservations(
	interactions map[InteractionID]*interactionRecord, options map[MemberID][]Option, f *field,
) error {
	for _, id := range slices.Sorted(maps.Keys(interactions)) {
		r := interactions[id].reservation
		if r == nil {
			continue
		}
		if _, ok := options[r.Holder]; !ok {
			return fmt.Errorf("interaction %q is saved reserved by %q, who is not in the run", id, r.Holder)
		}
		lines, where := options[r.Target], string(r.Target)
		if r.Target != "" {
			if _, ok := options[r.Target]; !ok {
				return fmt.Errorf("interaction %q is saved reserved for a line said to %q, who is not in the run", id, r.Target)
			}
		} else {
			var ok bool
			if lines, ok = f.propOptions(r.Prop); !ok {
				return fmt.Errorf("interaction %q is saved reserved for a line said to prop %q, which the field lacks", id, r.Prop)
			}
			where = string(r.Prop)
		}
		i := slices.IndexFunc(lines, func(o Option) bool { return o.ID == r.Option })
		if i < 0 {
			return fmt.Errorf("interaction %q is saved reserved for line %q, which %q does not carry", id, r.Option, where)
		}
		if touched, ok := optionInteraction(lines[i]); !ok || touched != id {
			return fmt.Errorf("interaction %q is saved reserved for line %q, which does not touch it", id, r.Option)
		}
	}
	return nil
}

// propOptions is the lines a prop of the field carries, and whether the
// field has the prop at all.
func (f *field) propOptions(prop PropID) ([]Option, bool) {
	if i := f.propIndexOf(prop); i >= 0 {
		return f.props[i].Options, true
	}
	if i := f.placedIndexOf(prop); i >= 0 {
		return f.placed[i].options, true
	}
	return nil, false
}
