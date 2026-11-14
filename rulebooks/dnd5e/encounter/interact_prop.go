// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// interact_prop.go is SOMETHING DONE TO A THING (rpg-project#959, Passages S2
// of #848; the operator's ruling: "Any prop should be able to be the
// interact destination"): [Encounter.Interact] with a prop and an option
// whose kind is to do ([Option.Do]).
//
// THE LINE'S DOOR, THE LINE'S CONSEQUENCES, NONE OF ITS SPEECH. The option is
// admitted through the door a line said to a prop goes through
// ([Encounter.replyDoor]: shown to the actor — a hidden thing refused until
// known — offered, its revision current, not pending, within reach), its
// roll checked as a line's is, its jail and its relocation asked ahead, and
// then it is answered, granted, steps its conversation, settles its
// reservation and pays the world's price exactly as a line does. What a line
// has and this does not: hearers, a speech beat, a loudness, a creature
// addressed. Nobody hears it; whoever sees the actor sees it.
//
// THE BEAT IS TOLD PER VIEWER ([BeatInteracted]):
//   - the actor: the prop, the option, the `do` words and the check;
//   - a witness shown the prop: the actor, the prop and the `seen` words
//     (absent when the author wrote none: the client words a generic);
//   - a witness who sees the actor and is not shown the prop: the actor and
//     where they stand — a concealed thing is never named to one who does
//     not know it. A witness who SEES the concealed thing used knows it
//     ([Encounter.revealBySight]) and is shown it; this version is left for
//     one who sees the actor and not the thing;
//   - anyone else: nothing.
//
// It lands [DeedUse] on every witness, the deeds channel's "saw them do
// something there" — naming no thing.
//
// The answer's words are narration (a prop has no voice, answer.go): told,
// with any fact they teach and any record a row tells, to the actor and to
// the witnesses shown the prop.

// verbInteract is the bare verb name the answered beat carries for a thing
// done, as [verbSpeak] is for a line.
const verbInteract = "interact"

// The [BeatInteracted] versions a thing done is told in, under its `view`
// key (rpg-project#959).
const (
	// InteractedViewActor is the actor's own version: the prop, the option,
	// the `do` words and the check.
	InteractedViewActor = "actor"
	// InteractedViewThing is a witness shown the prop: the actor, the prop
	// and the `seen` words when the author wrote them.
	InteractedViewThing = "thing"
	// InteractedViewPlace is a witness who sees the actor and is not shown
	// the prop: the actor and the cell they stand on, nothing else.
	InteractedViewPlace = "place"
)

// interactWithProp is [Encounter.Interact] for a thing done (this file).
//
// Validation order: no actor, or not exactly one of target and prop, or a
// prop with no option → no roller → closed → [Encounter.Speak]'s door
// ([Encounter.replyDoor]) → the option is done, not said → the roll fits the
// option → a jail or a relocation its verdict would fire would refuse.
//
// Errors: ErrNoMember, ErrNoOption, ErrNoRoller, ErrClosed, every error the
// line door returns (ErrNotMember, ErrNoProp, ErrBadPlacement,
// ErrStaleOption, ErrReserved, ErrOutOfRange), and a relocation's or a
// jail's refusal (ErrInBubble, ErrInCustody, ErrBadPlacement).
func (e *Encounter) interactWithProp(ctx context.Context, in *InteractInput) (*InteractOutput, error) {
	switch {
	case in.Actor == "":
		return nil, fmt.Errorf("interact: no actor: %w", ErrNoMember)
	case in.Target != "" || in.Prop == "":
		return nil, fmt.Errorf("interact: a thing is done to exactly one prop, and to no creature: %w", ErrNoMember)
	case in.OptionID == "":
		return nil, fmt.Errorf("interact: nothing named to do to %q: %w", in.Prop, ErrNoOption)
	case in.Range < 0:
		return nil, fmt.Errorf("interact: range %d is negative: %w", in.Range, ErrBadReach)
	case in.Roller == nil:
		return nil, fmt.Errorf("interact: the world rolls the answer: %w", ErrNoRoller)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("interact: %w", ErrClosed)
	}

	admitted, err := e.replyDoor(verbInteract, lineReply{
		actor: in.Actor, prop: in.Prop, optionID: in.OptionID, revision: in.Revision, reach: in.Range,
	}, true)
	if err != nil {
		return nil, err
	}
	option, settle, where, witnesses := admitted.option, admitted.settle, admitted.where, admitted.witnesses
	if !option.Does() {
		return nil, fmt.Errorf("interact: option %q is said, not done: %w", in.OptionID, ErrNoOption)
	}
	r := roll{skill: in.Skill, dc: in.DC, total: in.Total, calculation: in.Calculation}
	if err := refuseRollMismatch(verbInteract, in.OptionID, r, option); err != nil {
		return nil, err
	}
	if in.Calculation != nil {
		if err := validateRecordedTotal(in.Calculation, in.Total); err != nil {
			return nil, fmt.Errorf("interact: calculation: %w", err)
		}
	}
	beaten := in.Beaten || !option.Checked()
	if err := e.refuseJailAhead(option, beaten); err != nil {
		return nil, fmt.Errorf("interact: %w", err)
	}
	if err := e.refuseRelocateAhead(option, beaten, in.Actor); err != nil {
		return nil, fmt.Errorf("interact: %w", err)
	}

	at := uint64(e.clock.ToData().HighWater)
	// IF YOU SAW IT, YOU KNOW IT (rpg-project#959, ruled on #848): a watcher
	// who sees the hidden thing used learns its secret first, and so is
	// shown the thing below.
	if err := e.revealBySight(in.Actor, in.Prop, witnesses, at); err != nil {
		return nil, fmt.Errorf("interact: %w", err)
	}
	shown := e.shownThePropOf(in.Actor, in.Prop, witnesses)
	seq, err := e.tellInteracted(in, option, r, beaten, where, witnesses, shown, at)
	if err != nil {
		return nil, err
	}

	// SEEN IS A DEED (deed.go): whoever saw the doer holds that they used
	// something, there — never which thing.
	if err := e.landDeedAt(DeedUse, in.Actor, "", where, witnesses, at); err != nil {
		return nil, fmt.Errorf("interact: %w", err)
	}

	key, rows := AnswerFailure, option.Failure
	if beaten {
		key, rows = AnswerSuccess, option.Success
	}
	answered, err := e.answer(ctx, answerInput{
		prop: in.Prop, actor: in.Actor, option: option.ID, key: key, rows: rows,
		verb: verbInteract, beaten: beaten, witnesses: shown, lineCatchers: shown, at: at, roller: in.Roller,
	})
	if err != nil {
		return nil, fmt.Errorf("interact: %w", err)
	}

	var formed *FormedBubble
	if answered.pick != nil && len(option.Grants) > 0 {
		iid, _ := optionInteraction(option)
		if formed, err = e.applyGrants(option, beaten, answered.pick.Entry, recognitionLine{
			interaction: iid, option: option.ID, speaker: in.Actor, witnesses: witnesses, at: at,
			learners: answered.learners, reach: in.Range,
			seen: admitted.seen, perceived: admitted.perceived, partyAt: admitted.partyAt,
		}); err != nil {
			return nil, fmt.Errorf("interact: %w", err)
		}
	}
	if err := e.advanceInteraction(option, beaten, recognitionLine{
		speaker: in.Actor, witnesses: witnesses, at: at, reach: in.Range,
		seen: admitted.seen, perceived: admitted.perceived, partyAt: admitted.partyAt,
	}); err != nil {
		return nil, fmt.Errorf("interact: %w", err)
	}
	e.settle(settle, option)

	// THE PROP'S DEED, as a beaten line said to it is (option.go): the bell
	// rung for whoever listens for it.
	if beaten {
		e.landClock(bellPrefix+string(in.Prop), "", at)
	}
	// A CHECKED ONE IS AN ACTION; a plain one is free, as a plain line is.
	if option.Checked() {
		if err := e.spendWorldAction(in.Actor); err != nil {
			return nil, fmt.Errorf("interact: %w", err)
		}
	}

	return &InteractOutput{
		Prop: in.Prop, Seq: seq, Beaten: beaten, Witnesses: witnesses, Answered: answered.pick, Formed: formed,
	}, nil
}

// shownThePropOf is the actor and every witness shown the prop: one who may
// be shown it where it stands ([Encounter.speakableProp]'s probe), or, for a
// prop the actor carries, one it is not hidden from. In witness order, the
// actor always among them.
func (e *Encounter) shownThePropOf(actor MemberID, prop PropID, witnesses []MemberID) []MemberID {
	out := []MemberID{}
	for _, w := range witnesses {
		if e.propShownTo(actor, w, prop) {
			out = append(out, w)
		}
	}
	if !slices.Contains(out, actor) {
		out = append([]MemberID{actor}, out...)
	}
	return out
}

// propShownTo is whether reader is shown the prop actor said a line to or
// did a thing to: the actor always; for a prop the actor carries, one it is
// not hidden from; else one who may be shown it where it stands
// ([Encounter.speakableProp]'s probe).
func (e *Encounter) propShownTo(actor, reader MemberID, prop PropID) bool {
	switch {
	case reader == actor:
		return true
	case slices.Contains(e.heldPropsOf(actor), prop):
		return !e.hiddenPropTo(reader, prop)
	}
	_, _, err := e.speakableProp(reader, prop)
	return err == nil
}

// tellInteracted writes the [BeatInteracted] versions of a thing done (this
// file's doc) and returns the actor's version's sequence.
func (e *Encounter) tellInteracted(
	in *InteractInput, option Option, r roll, beaten bool, where spatial.Position,
	witnesses, shown []MemberID, at uint64,
) (uint64, error) {
	var thing, place []MemberID
	for _, w := range witnesses {
		switch {
		case w == in.Actor:
		case slices.Contains(shown, w):
			thing = append(thing, w)
		default:
			place = append(place, w)
		}
	}
	actor := map[string]interface{}{
		"beat": BeatInteracted, "view": InteractedViewActor, "actor": string(in.Actor),
		"prop": string(in.Prop), "option": option.ID, "do": option.Do, "beaten": beaten,
	}
	if option.Checked() {
		actor["skill"], actor["dc"], actor["total"] = r.skill, r.dc, r.total
		if r.calculation != nil {
			actor["calculation"] = r.calculation
		}
	}
	seen := map[string]interface{}{
		"beat": BeatInteracted, "view": InteractedViewThing, "actor": string(in.Actor), "prop": string(in.Prop),
	}
	if option.Seen != "" {
		seen["seen"] = option.Seen
	}
	versions := []struct {
		audience []MemberID
		payload  map[string]interface{}
	}{
		{[]MemberID{in.Actor}, actor},
		{thing, seen},
		{place, map[string]interface{}{"beat": BeatInteracted, "view": InteractedViewPlace, "actor": string(in.Actor), "at": where}},
	}
	var seq uint64
	for i, v := range versions {
		if len(v.audience) == 0 {
			continue
		}
		payload, err := json.Marshal(v.payload)
		if err != nil {
			return 0, fmt.Errorf("interact: marshal beat: %w", err)
		}
		out, err := e.appendBeat(&record.AppendInput{
			At: at, Audience: v.audience, Tags: map[string]string{"tag": "interact"}, Payload: payload,
		})
		if err != nil {
			return 0, fmt.Errorf("interact: append beat: %w", err)
		}
		if i == 0 {
			seq = out.Seq
		}
	}
	return seq, nil
}

// revealBySight is IF YOU SAW IT, YOU KNOW IT (rpg-project#959, ruled on
// #848): every witness of a line said to, or a thing done to, a concealed
// prop who sees the prop — a cell it stands on in their sight now, or the
// doer's hands when the doer carries it — and does not know its secret comes
// to know it, by sight, through the one reveal writer: the sibling of a
// secret door seen standing open ([Encounter.sweepConcealment]). Player or
// creature alike. A witness who sees the doer and not where the thing
// stands learns nothing. A prop no concealment hides is not this rule's.
func (e *Encounter) revealBySight(actor MemberID, prop PropID, witnesses []MemberID, at uint64) error {
	id, hidden := e.field.concealmentOfProp[prop]
	if !hidden {
		return nil
	}
	c := e.field.concealmentOf(id)
	if c == nil {
		return nil
	}
	_, cells, err := e.speakableProp(actor, prop)
	if err != nil {
		return err
	}
	reach, err := e.placedSightReach()
	if err != nil {
		return err
	}
	for _, w := range witnesses {
		if w == actor || e.members[w] == nil || e.world.knowsConcealment(w, id) {
			continue
		}
		seen := cells == nil // carried: it is in the hands they see
		for _, cell := range cells {
			seen = seen || reach.reachesCell(w, cell)
		}
		if !seen {
			continue
		}
		if err := e.revealConcealmentTo(w, c, "saw it used", at); err != nil {
			return err
		}
	}
	return nil
}
