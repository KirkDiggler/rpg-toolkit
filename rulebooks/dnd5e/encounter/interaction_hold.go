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
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
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
// within or out of the target's sight — is ended. A fight alone ends
// nothing: Speak does not refuse a line said to a creature in a fight. The
// host still releases a window it closes ([Encounter.ReleaseReservation]); a
// holder who exits drops their holds.
//
// # A roll is kept (rpg-project#938)
//
// HOW the reservation ends depends on why its settle stopped being
// admitted. When the holder and the target lose reach or sight of each
// other — the line's two LIVE doors, [ErrOutOfRange] and [ErrUnwitnessed] —
// the roll was already made and shown, so it is not undone: the reservation
// is ABANDONED ([InteractionReservation.Abandoned]), not released. The lines
// stay pending, the revision does not move, and the settle is still the one
// line admitted: the host resumes the roll DECLINED (walking away is
// declining the held benefit, never spending it) and says the line
// ([Encounter.Speak]) at the reserved revision as for any settle. An
// abandoned settle does not ask the reach or witness doors again — the
// words were spoken when the roll was made — and is judged in the scene of
// that moment ([ReservedScene]): who witnessed it, where the speaker stood,
// and whom the creature spoken to saw. The verdict, its answer, its advance
// and its row's grants land exactly once; the reservation clears and the
// revision moves, so a second settle is refused stale.
//
// Every other reason — the target gone, down or no longer shown, the line no
// longer offered, the holder unplaced — still RELEASES with no verdict:
// there is nobody to answer. A holder who exits drops their holds. An
// abandoned reservation is swept on the site and line doors alone, so a
// target who falls before the settle still releases it. A reservation saved
// before scenes were kept has none, and is released rather than abandoned.

// BeatInteractionHeld is the "beat" value of the story beat appended when a
// reservation is placed (`held: true`), abandoned by its holder (`held:
// true, abandoned: true`; rpg-project#938) or released without a verdict
// (`held: false`): the interaction, the holder and the revision now. The
// line itself is NOT named — nothing has been said yet. A settle writes no held beat: the
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

	// Abandoned is whether the holder and the target lost reach or sight of
	// each other while the roll waited (rpg-project#938): the roll is kept,
	// and its settle is admitted without the reach and witness doors, judged
	// in [InteractionReservation.Scene].
	Abandoned bool `json:"abandoned,omitzero"`

	// Scene is the scene the line was spoken in, kept when it was reserved
	// (rpg-project#938). Nil only on a reservation saved before scenes were
	// kept, which is released rather than abandoned.
	Scene *ReservedScene `json:"scene,omitempty"`
}

// ReservedScene is the scene a waiting line was spoken in (rpg-project#938),
// kept so an abandoned settle is judged as of the words, not as of where the
// holder walked to.
type ReservedScene struct {
	// Where is the cell the speaker stood on.
	Where spatial.Position `json:"where"`

	// Witnesses is who witnessed the speaker there, as Speak asks it
	// ([Encounter.audienceOf]): the line's audience, and the carriers who
	// may hear its grants.
	Witnesses []MemberID `json:"witnesses,omitempty"`

	// Seen is every member the creature spoken to saw then, the speaker
	// among them: whom a grant's holder may recognise. Empty for a line said
	// to a prop.
	Seen []MemberID `json:"seen,omitempty"`

	// Perceived is, for each witness, the members of the speaker's party that
	// witness saw then, in id order; a witness who saw none is absent. It is
	// what each reader of a recognition's beats may be told: nothing the
	// reader did not perceive when the words were said.
	Perceived map[MemberID][]MemberID `json:"perceived,omitempty"`
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

	// Abandoned is whether the reservation the line waits on was abandoned
	// (rpg-project#938), told ONLY to its holder, on the reserved line
	// itself: the host resumes that roll declined and settles it.
	Abandoned bool
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
	// seen is who the creature spoken to saw when an ABANDONED settle's line
	// was spoken ([ReservedScene.Seen]); nil for every other reply, which is
	// judged on live sight.
	seen []MemberID
	// perceived is that scene's [ReservedScene.Perceived], for the same
	// settle; nil otherwise.
	perceived map[MemberID][]MemberID
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
// pass it. The settle of an ABANDONED reservation (rpg-project#938) skips the
// witness and reach doors and answers with the scene it was spoken in
// ([ReservedScene]): the words were said when the roll was made.
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
	var settle *InteractionReservation
	if settling {
		settle = e.settleFor(in, options)
	}
	if settle != nil && settle.Abandoned && settle.Scene != nil {
		option, err := e.lineDoor(in.actor, in.target, options, in.optionID, in.revision, settle)
		if err != nil {
			return replied{}, fmt.Errorf("%s: %w", verb, err)
		}
		scene := settle.Scene
		// The audience is the scene's witnesses as they were: one who has since
		// left the run is still named in the settle's beats.
		return replied{
			option: option, settle: settle, where: scene.Where,
			witnesses: slices.Clone(scene.Witnesses), seen: slices.Clone(scene.Seen),
			perceived: clonePerceived(scene.Perceived),
		}, nil
	}
	where, witnesses, err := e.audienceOf(in.actor)
	if err != nil {
		return replied{}, fmt.Errorf("%s: %w", verb, err)
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
// because the settle could no longer be admitted ([Encounter.sweepHolds]) —
// or, when the holder and the target lost reach or sight of each other,
// marks it abandoned for the host to settle as rolled (rpg-project#938). The
// scene the line was spoken in is kept with it ([ReservedScene]).
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
	var seen []MemberID
	var perceived map[MemberID][]MemberID
	err := e.asShown(in.Actor, func() error {
		var err error
		if admitted, err = e.replyDoor("reserve", in.reply(), false); err != nil {
			return err
		}
		// THE SCENE IS KEPT AS SHOWN (rpg-project#938): whom the creature
		// spoken to sees, and which of the speaker's party each witness sees,
		// read on the same live reach the door just asked.
		if seen, err = e.membersSeenBy(in.Target); err != nil {
			return err
		}
		perceived, err = e.partyPerceivedBy(in.Actor, admitted.witnesses)
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
		Scene: &ReservedScene{
			Where: admitted.where, Witnesses: slices.Clone(admitted.witnesses), Seen: seen, Perceived: perceived,
		},
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
// calls it when it closes the window with no verdict — an abandoned
// reservation included, when the host can no longer settle it; the
// encounter releases on its own what its door would no longer admit for any
// reason but the holder's leaving ([Encounter.sweepHolds]).
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

// sweepHolds ends every reservation whose settle [Encounter.replyDoor] would
// refuse now, in interaction id order (rpg-project#924): THE ENCOUNTER DOES
// NOT LEAVE A ROLL HOLDING A CONVERSATION ITS VERDICT COULD NEVER LAND IN.
// Refused for reach or sight ([ErrOutOfRange], [ErrUnwitnessed]), a
// reservation with a scene is ABANDONED, its `held: true, abandoned: true`
// beat told (rpg-project#938: a roll is kept); refused for anything else, or
// with no scene, it is released with its `held: false` beat. An abandoned
// reservation is asked the site and line doors only, so it is released if
// its target falls or leaves before the host settles it. Asked at every
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
		// A ROLL IS KEPT (rpg-project#938): out of reach or out of sight, the
		// roll was made and stands; anything else has nobody to answer it.
		if (errors.Is(err, ErrOutOfRange) || errors.Is(err, ErrUnwitnessed)) && hold.Scene != nil {
			if err := e.abandon(id); err != nil {
				return fmt.Errorf("interaction %q hold: %w", id, err)
			}
			continue
		}
		if _, err := e.release(id); err != nil {
			return fmt.Errorf("interaction %q hold: %w", id, err)
		}
	}
	return nil
}

// abandon marks one interaction's reservation abandoned (rpg-project#938)
// and tells it in a `held: true, abandoned: true` [BeatInteractionHeld] beat
// to the holder's witnesses. The revision does not move and the lines stay
// pending: the settle that clears it is still the one reply admitted.
func (e *Encounter) abandon(id InteractionID) error {
	hold := e.interactions[id].reservation
	hold.Abandoned = true
	_, err := e.appendHeldBeatOf(id, hold.Holder, true, true)
	return err
}

// membersSeenBy is every member target sees on the live reach now, in id order,
// with the speaker's sneaking as the caller left it; nil for no target (a
// line said to a prop).
func (e *Encounter) membersSeenBy(target MemberID) ([]MemberID, error) {
	if target == "" {
		return nil, nil
	}
	reach, err := e.placedSightReach()
	if err != nil {
		return nil, err
	}
	var out []MemberID
	for _, id := range slices.Sorted(maps.Keys(e.members)) {
		if id != target && reach.Reaches(perception.Sight, target, id) {
			out = append(out, id)
		}
	}
	return out, nil
}

// partyPerceivedBy is, for each witness, the members of the speaker's party
// that witness sees on the live reach now, in id order; a witness who sees
// none is left out, and the map is nil when no witness sees any.
func (e *Encounter) partyPerceivedBy(speaker MemberID, witnesses []MemberID) (map[MemberID][]MemberID, error) {
	reach, err := e.placedSightReach()
	if err != nil {
		return nil, err
	}
	party := e.partyOf(speaker)
	var out map[MemberID][]MemberID
	for _, w := range witnesses {
		var saw []MemberID
		for _, id := range party {
			if id == w || reach.Reaches(perception.Sight, w, id) {
				saw = append(saw, id)
			}
		}
		if len(saw) == 0 {
			continue
		}
		if out == nil {
			out = map[MemberID][]MemberID{}
		}
		out[w] = saw
	}
	return out, nil
}

// clonePerceived copies a scene's per-witness sight so no caller's value is
// aliased.
func clonePerceived(perceived map[MemberID][]MemberID) map[MemberID][]MemberID {
	if perceived == nil {
		return nil
	}
	out := make(map[MemberID][]MemberID, len(perceived))
	for w, saw := range perceived {
		out[w] = slices.Clone(saw)
	}
	return out
}

// AbandonReservationInput names whose waiting line the host judges
// abandoned.
type AbandonReservationInput struct {
	// Member is the holder. REQUIRED, a member.
	Member MemberID
}

// AbandonReservationOutput reports every reservation the member held, each
// abandoned or, saved before scenes were kept, released.
type AbandonReservationOutput struct {
	// Abandoned is each interaction now abandoned, with its revision
	// (unmoved), in id order.
	Abandoned []InteractionRevision

	// Released is each interaction released instead, with its revision now:
	// a reservation with no [ReservedScene] cannot be judged as spoken.
	Released []InteractionRevision
}

// AbandonReservation marks every reservation a member holds ABANDONED
// (rpg-project#938), as [Encounter.sweepHolds] does when the holder and the
// target lose reach or sight of each other: the host calls it when its own
// check before a resume finds the same thing and no participation pass has
// run since. It does not ask why. An abandoned reservation is left as it
// is; one saved without a scene is released.
//
// Errors: ErrNilInput, ErrNoMember (no member named), ErrNotMember,
// ErrNoReservation (the member holds none).
func (e *Encounter) AbandonReservation(in *AbandonReservationInput) (*AbandonReservationOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("abandon reservation: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("abandon reservation: %w", ErrNoMember)
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("abandon reservation: member %q: %w", in.Member, ErrNotMember)
	}
	out := &AbandonReservationOutput{}
	for _, id := range slices.Sorted(maps.Keys(e.interactions)) {
		r := e.interactions[id]
		hold := r.reservation
		if hold == nil || hold.Holder != in.Member {
			continue
		}
		switch {
		case hold.Scene == nil:
			revision, err := e.release(id)
			if err != nil {
				return nil, fmt.Errorf("abandon reservation: %w", err)
			}
			out.Released = append(out.Released, InteractionRevision{Interaction: id, Revision: revision})
			continue
		case !hold.Abandoned:
			if err := e.abandon(id); err != nil {
				return nil, fmt.Errorf("abandon reservation: %w", err)
			}
		}
		out.Abandoned = append(out.Abandoned, InteractionRevision{Interaction: id, Revision: r.revision})
	}
	if len(out.Abandoned)+len(out.Released) == 0 {
		return nil, fmt.Errorf("abandon reservation: member %q: %w", in.Member, ErrNoReservation)
	}

	return out, nil
}

// appendHeldBeat writes a [BeatInteractionHeld] beat for one interaction to
// the holder's witnesses — the holder alone when they stand nowhere.
func (e *Encounter) appendHeldBeat(id InteractionID, holder MemberID, held bool) (uint64, error) {
	return e.appendHeldBeatOf(id, holder, held, false)
}

// appendHeldBeatOf is [Encounter.appendHeldBeat], with `abandoned: true` on
// the beat when the holder walked away from a kept roll (rpg-project#938).
func (e *Encounter) appendHeldBeatOf(id InteractionID, holder MemberID, held, abandoned bool) (uint64, error) {
	audience := []MemberID{holder}
	if _, witnesses, err := e.audienceOf(holder); err == nil {
		audience = witnesses
	}
	fields := map[string]interface{}{
		"beat":        BeatInteractionHeld,
		"interaction": string(id),
		"holder":      string(holder),
		"held":        held,
		"revision":    e.interactions[id].revision,
	}
	if abandoned {
		fields["abandoned"] = true
	}
	payload, err := json.Marshal(fields)
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
	if r.Scene != nil {
		scene := *r.Scene
		scene.Witnesses, scene.Seen = slices.Clone(r.Scene.Witnesses), slices.Clone(r.Scene.Seen)
		scene.Perceived = clonePerceived(r.Scene.Perceived)
		c.Scene = &scene
	}
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
	case r.Abandoned && r.Scene == nil:
		return fmt.Errorf("interaction %q is saved abandoned with no scene to judge it in", id)
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
