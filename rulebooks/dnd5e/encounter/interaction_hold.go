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
// A reservation ends without a verdict through [Encounter.ReleaseReservation]
// (the host decides when: the speaker left Speak reach, the window was
// abandoned), and when its holder exits the run.

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

	// PendingOn is the member whose waiting check holds this line: a pending
	// line is shown and cannot be spoken ([ErrReserved]) until the check
	// settles or is released. Empty for a line that can be spoken now. It
	// may name the speaker themself, whose own line waits on their roll.
	PendingOn MemberID
}

// Pending reports whether the line waits on somebody's roll and cannot be
// spoken now.
func (o OfferedOption) Pending() bool { return o.PendingOn != "" }

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
func (e *Encounter) settleFor(in *SpeakInput, options []Option) *InteractionReservation {
	i := slices.IndexFunc(options, func(o Option) bool { return o.ID == in.OptionID })
	if i < 0 {
		return nil
	}
	hold := e.holdOf(options[i])
	if hold == nil || hold.Holder != in.Actor || hold.Target != in.Target || hold.Prop != in.Prop || hold.Option != in.OptionID {
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
	actor MemberID, options []Option, id string, revision uint64, settle *InteractionReservation,
) (Option, error) {
	evaluation, err := e.evaluateOptions(actor, options, settle)
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
// probe law).
func (e *Encounter) lineSite(verb string, actor, target MemberID, prop PropID) ([]Option, error) {
	if actor == "" {
		return nil, fmt.Errorf("%s: no actor: %w", verb, ErrNoMember)
	}
	if (target == "") == (prop == "") {
		return nil, fmt.Errorf("%s: a line is said to exactly one creature or prop: %w", verb, ErrNoMember)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("%s: %w", verb, ErrClosed)
	}
	if _, ok := e.members[actor]; !ok {
		return nil, fmt.Errorf("%s: actor %q: %w", verb, actor, ErrNotMember)
	}
	if prop != "" {
		options, _, err := e.speakableProp(actor, prop)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", verb, err)
		}
		return options, nil
	}
	t, ok := e.members[target]
	if !ok {
		return nil, fmt.Errorf("%s: target %q: %w", verb, target, ErrNotMember)
	}
	if actor == target {
		return nil, fmt.Errorf("%s: actor %q cannot address itself: %w", verb, actor, ErrNotMember)
	}
	down, err := e.isDown(target)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", verb, err)
	}
	if down {
		return nil, fmt.Errorf("%s: %q is down and cannot answer: %w", verb, target, ErrMemberDown)
	}
	return t.Options, nil
}

// ChooseOptionInput names a reply to an offered line: who, to which creature
// or prop, which line, and the revision it was offered at.
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
}

// ChooseOptionOutput is the line the reply chose, as offered now.
type ChooseOptionOutput struct {
	// Option is the chosen line, deep-copied, with its interaction and
	// revision now.
	Option OfferedOption
}

// ChooseOption is the line door asked BEFORE anything is charged or rolled
// (rpg-project#924): whether this reply names a line offered to this
// speaker now, from the conversation as it is now. A read: it writes
// nothing. [Encounter.Speak] asks the same door again for itself.
//
// Validation order: nil input → no actor, or not exactly one of target and
// prop → closed → actor is a member → a creature target is a member, not the
// actor and not down; a prop is shown to the actor → no option id → the
// line is offered ([ErrNoOption]) → its revision is the interaction's now
// ([ErrStaleOption]) → it is not pending ([ErrReserved]).
//
// Errors: ErrNilInput, ErrNoMember, ErrNoOption, ErrClosed, ErrNotMember,
// ErrMemberDown, ErrNoProp, ErrStaleOption, ErrReserved.
func (e *Encounter) ChooseOption(in *ChooseOptionInput) (*ChooseOptionOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("choose option: %w", ErrNilInput)
	}
	option, err := e.chooseLine("choose option", in)
	if err != nil {
		return nil, err
	}

	return &ChooseOptionOutput{Option: e.offerOf(option, "")}, nil
}

// chooseLine is [Encounter.ChooseOption]'s door, shared with
// [Encounter.Reserve]: the site, then the line door with no settle.
func (e *Encounter) chooseLine(verb string, in *ChooseOptionInput) (Option, error) {
	options, err := e.lineSite(verb, in.Actor, in.Target, in.Prop)
	if err != nil {
		return Option{}, err
	}
	if in.OptionID == "" {
		return Option{}, fmt.Errorf("%s: no line named: %w", verb, ErrNoOption)
	}
	option, err := e.lineDoor(in.Actor, options, in.OptionID, in.Revision, nil)
	if err != nil {
		return Option{}, fmt.Errorf("%s: %w", verb, err)
	}

	return option, nil
}

// offerOf is one line as offered: deep-copied, with the interaction it
// touches and that interaction's revision now, and who it is pending on.
func (e *Encounter) offerOf(o Option, pendingOn MemberID) OfferedOption {
	out := OfferedOption{Option: cloneOption(o), PendingOn: pendingOn}
	if id, ok := optionInteraction(o); ok {
		out.Interaction = id
		if r, ok := e.interactions[id]; ok {
			out.Revision = r.revision
		}
	}
	return out
}

// ReserveInput names the checked line that paused on its speaker's roll:
// the same reply [ChooseOptionInput] names, at the revision it was offered.
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
// prop it is said to, and the revision it was offered at; the revision
// moves; and a [BeatInteractionHeld] beat goes to the holder's witnesses.
// From then on every line touching that interaction is pending, for every
// speaker, until the holder's [Encounter.Speak] settles it or
// [Encounter.ReleaseReservation] ends it.
//
// Validation order: [Encounter.ChooseOption]'s, whole — so a stale reply is
// refused [ErrStaleOption] and a line on an already-held interaction
// [ErrReserved] — then the line must roll a check ([ErrNoOption]: a plain
// line never waits on a roll).
//
// Errors: ErrNilInput and every error [Encounter.ChooseOption] returns.
func (e *Encounter) Reserve(in *ReserveInput) (*ReserveOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("reserve: %w", ErrNilInput)
	}
	option, err := e.chooseLine("reserve", in)
	if err != nil {
		return nil, err
	}
	if !option.Checked() {
		return nil, fmt.Errorf("reserve: line %q rolls no check and never waits on one: %w", in.OptionID, ErrNoOption)
	}
	id, ok := optionInteraction(option)
	if !ok {
		return &ReserveOutput{}, nil
	}
	r := e.interactions[id]
	r.reservation = &InteractionReservation{
		Holder: in.Actor, Target: in.Target, Prop: in.Prop, Option: in.OptionID, Revision: in.Revision,
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
// decides when: a waiting check whose speaker left Speak reach, or a window
// abandoned, so an unanswered roll cannot hold the conversation forever.
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
	released := e.dropHoldsOf(in.Member)
	if len(released) == 0 {
		return nil, fmt.Errorf("release reservation: member %q: %w", in.Member, ErrNoReservation)
	}
	for _, r := range released {
		if _, err := e.appendHeldBeat(r.Interaction, in.Member, false); err != nil {
			return nil, fmt.Errorf("release reservation: %w", err)
		}
	}

	return &ReleaseReservationOutput{Released: released}, nil
}

// dropHoldsOf clears every reservation a member holds and moves each
// revision, writing nothing: [Encounter.ReleaseReservation] tells the table,
// and an exit's own beat already does.
func (e *Encounter) dropHoldsOf(member MemberID) []InteractionRevision {
	var out []InteractionRevision
	for _, id := range slices.Sorted(maps.Keys(e.interactions)) {
		r := e.interactions[id]
		if r.reservation == nil || r.reservation.Holder != member {
			continue
		}
		r.reservation = nil
		r.revision++
		out = append(out, InteractionRevision{Interaction: id, Revision: r.revision})
	}
	return out
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
	}
	return nil
}

// validateInteractionReservations refuses a saved reservation naming a
// holder or a creature the run does not have, or a line that creature does
// not carry. options is every saved member's lines, by id. A line said to a
// prop is held to its shape alone ([validateReservationShape]): the prop's
// own door is the probe law, asked again by the settle.
func validateInteractionReservations(interactions map[InteractionID]*interactionRecord, options map[MemberID][]Option) error {
	for _, id := range slices.Sorted(maps.Keys(interactions)) {
		r := interactions[id].reservation
		if r == nil {
			continue
		}
		if _, ok := options[r.Holder]; !ok {
			return fmt.Errorf("interaction %q is saved reserved by %q, who is not in the run", id, r.Holder)
		}
		if r.Target == "" {
			continue
		}
		lines, ok := options[r.Target]
		if !ok {
			return fmt.Errorf("interaction %q is saved reserved for a line said to %q, who is not in the run", id, r.Target)
		}
		if !slices.ContainsFunc(lines, func(o Option) bool { return o.ID == r.Option }) {
			return fmt.Errorf("interaction %q is saved reserved for line %q, which %q does not carry", id, r.Option, r.Target)
		}
	}
	return nil
}
