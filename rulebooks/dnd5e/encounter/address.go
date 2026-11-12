// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// address.go is THE GUARD HAILS (rpg-project#923, Gate S3 of #769; design
// R4): an interaction carries one authored approach distance, and when a
// carrier of it sees a character come inside it, he addresses that
// character. The address is written on the interaction instance, so there
// is one addressee per interaction at a time (ruled 2026-11-11: "one person
// being spoken to per interaction").
//
// # Carriers are derived
//
// A carrier of an interaction is a member whose own lines name it, in a
// `requires` or an `advance` ([carries]). There is no second list: the
// lines are the only place the interaction meets a speaker.
//
// # The trigger, at the end of a member's step
//
// [Encounter.Step], once the step has landed, its sight is refreshed and no
// ending fired: for each interaction with an approach and no standing
// address, a carrier who is up, out of any fight, sees the mover (the
// witness answer [Encounter.Witnesses] gives), stands within the approach
// distance, and whose lines the interaction offers the mover at least one
// of, addresses the mover. Nearest first, then member id. The address is
// written, [BeatAddressed] is appended, the step reports it
// ([StepOutput.Addressed]) so the host drops the walk's remaining legs, and
// the carrier is woken with the `approached` beat ([beatApproached]) — his
// machine's `on: approached` picks his hail line through the same answer
// path a bump's line takes. A carrier whose machine has no such row
// addresses silently.
//
// # One predicate decides whether an address stands
//
// [Encounter.addressStands]: the member within the reach the host measures
// Speak with, seen by the guard and sighting him in turn (the answer
// [Encounter.WithinReach] gives, so he is always on her Speak menu), the guard up and on the map, neither in
// a fight, and the interaction still offering the member one of the guard's
// lines. [Encounter.AddressFor] and the end-of-step sweep both ask it. When
// it fails at a sweep, the address is cleared and [BeatAddressEnded] records
// why; the interaction's progress (its machine blob) is never touched.
//
// # The reach is the host's
//
// How near a line can be said is the session's ([SpeakInput.Range]), not
// this composition's: every verb here that needs it is told it. The step
// and the read take it as an input; zero, or anything below the approach
// distance, measures the approach distance instead, so an address never
// ends on a cell its own trigger would admit. [ValidateApproachReach] is the
// door a host refuses an approach farther than its reach at.

// beatApproached is the wake a hailing carrier hears (rpg-project#923): a
// character came inside an interaction's approach distance in his sight and
// he now addresses them. Matched on him alone, at once, in the step that
// triggered it, never landed — a bump's law ([beatBarred]).
const beatApproached machine.Beat = "approached"

// BeatAddressed is the "beat" value of the story beat appended when a
// carrier addresses a member (rpg-project#923): its payload names the
// interaction, the member addressed and the guard addressing them.
// Audienced as a step is. EXPORTED BECAUSE A DECODER READS IT.
const BeatAddressed = "addressed"

// BeatAddressEnded is the "beat" value of the story beat appended when a
// standing address no longer stands (rpg-project#923): the interaction, the
// member, the guard, and the reason ([AddressEndReason]). The interaction's
// progress is untouched. EXPORTED BECAUSE A DECODER READS IT.
const BeatAddressEnded = "address-ended"

// AddressEndReason names why an address ended, the first the predicate met.
type AddressEndReason string

// The reasons an address ends, in the order the predicate asks them.
const (
	// AddressGone: the member or the guard is no longer on the map.
	AddressGone AddressEndReason = "gone"
	// AddressDown: the member or the guard is down.
	AddressDown AddressEndReason = "down"
	// AddressFight: a fight holds the member or the guard.
	AddressFight AddressEndReason = "fight"
	// AddressOutOfReach: the member stands beyond the reach.
	AddressOutOfReach AddressEndReason = "out-of-reach"
	// AddressUnseen: the guard no longer sees the member.
	AddressUnseen AddressEndReason = "unseen"
	// AddressOutOfSight: the member no longer sights the guard — the answer
	// [Encounter.WithinReach] gives, so the guard is off her Speak menu.
	AddressOutOfSight AddressEndReason = "out-of-sight"
	// AddressNoReply: the interaction offers the member none of the guard's
	// lines any more.
	AddressNoReply AddressEndReason = "no-reply"
)

// InteractionApproach is an interaction's authored approach
// (`interactions.<id>.approach: { within: N }`): how near, in cells, a
// character must come in a carrier's sight to be addressed.
type InteractionApproach struct {
	// Within is the distance in cells, measured as every reach here is
	// ([Encounter.Distance]). REQUIRED at least 1. The upper bound is the
	// host's Speak reach, refused at [ValidateApproachReach].
	Within int `json:"within"`
}

// InteractionAddress is who an interaction is addressing now: the member
// spoken to and the guard speaking (rpg-project#923). Persisted on
// [InteractionData.Address]; one per interaction.
type InteractionAddress struct {
	// Member is the member addressed.
	Member MemberID `json:"member"`

	// Guard is the carrier addressing them.
	Guard MemberID `json:"guard"`
}

// Addressed is one address a step wrote ([StepOutput.Addressed]) or a read
// found ([Encounter.AddressFor]).
type Addressed struct {
	// Interaction is the interaction addressing the member.
	Interaction InteractionID

	// Guard is the carrier speaking.
	Guard MemberID

	// Seq and ID name the [BeatAddressed] beat on a step's output; zero on a
	// read, which writes nothing.
	Seq uint64
	ID  string
}

// AddressForInput asks [Encounter.AddressFor] for one member's standing
// address.
type AddressForInput struct {
	// Member is the member asked about. REQUIRED.
	Member MemberID

	// Reach is the cells the host measures Speak with
	// ([SpeakInput.Range]); below an interaction's approach distance (zero
	// included), that distance is measured instead.
	Reach int
}

// AddressForOutput is [Encounter.AddressFor]'s answer.
type AddressForOutput struct {
	// Address is the standing address, nil when none stands.
	Address *Addressed
}

// AddressFor is the standing address of one member: the interaction
// addressing them and the guard speaking, nil when none stands. A READ:
// an address the predicate refuses is not reported, and is not cleared —
// the next step's sweep clears it and writes why. When several
// interactions address the member, the first by id.
//
// Errors: ErrNilInput, ErrNotMember, and the sight or participation
// capability's own refusal.
func (e *Encounter) AddressFor(in *AddressForInput) (AddressForOutput, error) {
	if in == nil {
		return AddressForOutput{}, fmt.Errorf("address for: %w", ErrNilInput)
	}
	if _, ok := e.members[in.Member]; !ok {
		return AddressForOutput{}, fmt.Errorf("address for %q: %w", in.Member, ErrNotMember)
	}
	for _, id := range slices.Sorted(maps.Keys(e.interactions)) {
		r := e.interactions[id]
		if r.address == nil || r.address.Member != in.Member {
			continue
		}
		reason, err := e.addressStands(r, *r.address, in.Reach)
		if err != nil {
			return AddressForOutput{}, fmt.Errorf("address for %q: %w", in.Member, err)
		}
		if reason == "" {
			return AddressForOutput{Address: &Addressed{Interaction: id, Guard: r.address.Guard}}, nil
		}
	}

	return AddressForOutput{}, nil
}

// ValidateApproachReach refuses an interaction whose approach distance is
// beyond reach — the host's Speak reach — so an addressed character can
// always answer from where the hail stopped them (ruled 2026-11-11: "the
// reach inside the current speak distance"). The refusal wraps ErrNoField
// and names the interaction.
func ValidateApproachReach(interactions []InteractionInput, reach int) error {
	for _, in := range interactions {
		if in.Approach != nil && in.Approach.Within > reach {
			return fmt.Errorf("interaction %q approach within %d is beyond the reach %d: %w",
				in.ID, in.Approach.Within, reach, ErrNoField)
		}
	}
	return nil
}

// validateApproach refuses an approach nobody can ever come inside.
func validateApproach(id InteractionID, a *InteractionApproach) error {
	if a != nil && a.Within < 1 {
		return fmt.Errorf("interaction %q approach within %d: the distance is at least 1", id, a.Within)
	}
	return nil
}

// carries reports whether a member's lines name an interaction, in a
// `requires` (a `not` of one included) or an `advance`: what makes the
// member one of its carriers.
func carries(options []Option, id InteractionID) bool {
	for _, o := range options {
		if o.Advance != nil && o.Advance.Interaction == id {
			return true
		}
		for _, requires := range o.Requires {
			inner, _ := unwrapNot(requires)
			if t, ok := inner.(TriggerInteraction); ok && t.Interaction == id {
				return true
			}
		}
	}
	return false
}

// offersReply reports whether the interaction offers the member at least
// one of the guard's lines that ADVANCES it: the offer [Encounter.OptionsFor]
// makes, narrowed to the lines that move the conversation.
//
// A LINE THAT ONLY READS THE INTERACTION IS NOT THE CONVERSATION
// (rpg-project#929). After admission the gate guards carry lines that read
// the gate's state without moving it (a vouch, a straggler's own case); a
// hail opened for those would stop every admitted member who walks past the
// gate, and an address kept for them would never end. Only a line that can
// step the interaction opens one or keeps one standing.
func (e *Encounter) offersReply(id InteractionID, member, guard MemberID) (bool, error) {
	g, ok := e.members[guard]
	if !ok {
		return false, nil
	}
	// A PENDING LINE STILL REPLIES (rpg-project#924): the conversation is
	// waiting on a roll, not over, so the address stands.
	offered, err := e.offered(member, guard, g.Options)
	if err != nil {
		return false, err
	}
	for _, o := range offered {
		if o.Advance != nil && o.Advance.Interaction == id {
			return true, nil
		}
	}
	return false, nil
}

// approachReach is the distance an address is measured at: the host's
// reach, never below the interaction's own approach distance.
func approachReach(r *interactionRecord, reach int) int {
	if r.input.Approach == nil {
		return reach
	}
	return max(reach, r.input.Approach.Within)
}

// inFight reports whether a fight holds the member.
func (e *Encounter) inFight(id MemberID) (bool, error) {
	_, err := e.bubbleFor(id)
	if errors.Is(err, errNotInBubble) {
		return false, nil
	}
	return err == nil, err
}

// sees reports whether observer sees subject: the witness answer
// [Encounter.audienceOf] gives, asked of one pair.
func (e *Encounter) sees(observer, subject MemberID) (bool, error) {
	reach, err := e.placedSightReach()
	if err != nil {
		return false, err
	}
	return reach.Reaches(perception.Sight, observer, subject), nil
}

// addressStands is THE ONE PREDICATE (this file's doc): empty when the
// address stands, else the first reason it does not.
func (e *Encounter) addressStands(r *interactionRecord, a InteractionAddress, reach int) (AddressEndReason, error) {
	if _, ok := e.members[a.Member]; !ok {
		return AddressGone, nil
	}
	if _, ok := e.members[a.Guard]; !ok {
		return AddressGone, nil
	}
	from, placed := e.canvas.GetEntityPosition(string(a.Member))
	at, guardPlaced := e.canvas.GetEntityPosition(string(a.Guard))
	if !placed || !guardPlaced {
		return AddressGone, nil
	}
	for _, id := range []MemberID{a.Member, a.Guard} {
		down, err := e.isDown(id)
		if err != nil {
			return "", err
		}
		if down {
			return AddressDown, nil
		}
	}
	for _, id := range []MemberID{a.Member, a.Guard} {
		fighting, err := e.inFight(id)
		if err != nil {
			return "", err
		}
		if fighting {
			return AddressFight, nil
		}
	}
	if e.Distance(from, at) > float64(approachReach(r, reach)) {
		return AddressOutOfReach, nil
	}
	seen, err := e.sees(a.Guard, a.Member)
	if err != nil {
		return "", err
	}
	if !seen {
		return AddressUnseen, nil
	}
	// SIGHT BOTH WAYS (rpg-toolkit#2370 review): the Speak menu the address
	// rides on lists only creatures the member sights now
	// ([Encounter.WithinReach]), so an address naming a guard she cannot see
	// would open a panel with nothing in it and block every other carrier.
	sighted, err := e.creaturesSightedBy(a.Member)
	if err != nil {
		return "", err
	}
	if !sighted[a.Guard] {
		return AddressOutOfSight, nil
	}
	replies, err := e.offersReply(r.input.ID, a.Member, a.Guard)
	if err != nil {
		return "", err
	}
	if !replies {
		return AddressNoReply, nil
	}
	return "", nil
}

// hasApproaches reports whether any interaction of the run has an approach:
// a run with none pays nothing per step.
func (e *Encounter) hasApproaches() bool {
	for _, r := range e.interactions {
		if r.input.Approach != nil {
			return true
		}
	}
	return false
}

// addressOutput is what a step's address pass wrote and what the hailing
// carriers' answers made.
type addressOutput struct {
	addressed []Addressed
	deltas    map[MemberID]*IntelDelta
	formed    *FormedBubble
}

// addressAfterStep is the step's address pass: the sweep, then the trigger
// for the mover (this file's doc). Nothing at all for a run without an
// approach.
//
// Errors: the predicate's capabilities, a beat's append, the carrier's
// machine or pick, the refresh after his answer.
func (e *Encounter) addressAfterStep(mover MemberID, reach int) (addressOutput, error) {
	var out addressOutput
	if !e.hasApproaches() {
		return out, nil
	}
	if err := e.sweepAddresses(reach); err != nil {
		return out, err
	}
	for _, id := range slices.Sorted(maps.Keys(e.interactions)) {
		r := e.interactions[id]
		if r.input.Approach == nil || r.address != nil {
			continue
		}
		guard, ok, err := e.hailer(r, mover)
		if err != nil {
			return out, err
		}
		if !ok {
			continue
		}
		hailed, err := e.hail(r, mover, guard)
		if err != nil {
			return out, err
		}
		out.addressed = append(out.addressed, hailed.addressed)
		out.deltas = mergeIntelDeltas(out.deltas, hailed.deltas)
		if hailed.formed != nil {
			out.formed = hailed.formed
		}
		if e.outcome != nil {
			return out, nil
		}
	}
	return out, nil
}

// sweepAddresses clears every address the predicate refuses, in interaction
// id order, each with its [BeatAddressEnded].
func (e *Encounter) sweepAddresses(reach int) error {
	for _, id := range slices.Sorted(maps.Keys(e.interactions)) {
		r := e.interactions[id]
		if r.address == nil {
			continue
		}
		reason, err := e.addressStands(r, *r.address, reach)
		if err != nil {
			return err
		}
		if reason == "" {
			continue
		}
		ended := *r.address
		r.address = nil // never the blob: the interaction's progress stands
		if err := e.appendAddressBeat(BeatAddressEnded, id, ended, reason); err != nil {
			return err
		}
	}
	return nil
}

// hailer is the carrier who addresses the mover on one interaction, if one
// does: every carrier the trigger admits, the nearest, then by id.
func (e *Encounter) hailer(r *interactionRecord, mover MemberID) (MemberID, bool, error) {
	from, placed := e.canvas.GetEntityPosition(string(mover))
	if !placed {
		return "", false, nil
	}
	type candidate struct {
		id       MemberID
		distance float64
	}
	var near []candidate
	for id, m := range e.members {
		if id == mover || !carries(m.Options, r.input.ID) {
			continue
		}
		at, ok := e.canvas.GetEntityPosition(string(id))
		if !ok {
			continue
		}
		if d := e.Distance(from, at); d <= float64(r.input.Approach.Within) {
			near = append(near, candidate{id: id, distance: d})
		}
	}
	slices.SortFunc(near, func(a, b candidate) int {
		if a.distance != b.distance {
			if a.distance < b.distance {
				return -1
			}
			return 1
		}
		if a.id < b.id {
			return -1
		}
		if a.id > b.id {
			return 1
		}
		return 0
	})
	for _, c := range near {
		// THE SAME PREDICATE, asked of the address this would write at the
		// approach distance: a hail is an address that stands.
		reason, err := e.addressStands(r, InteractionAddress{Member: mover, Guard: c.id}, r.input.Approach.Within)
		if err != nil {
			return "", false, err
		}
		if reason == "" {
			return c.id, true, nil
		}
	}
	return "", false, nil
}

// hailed is one address written and what the carrier's answer made.
type hailed struct {
	addressed Addressed
	deltas    map[MemberID]*IntelDelta
	formed    *FormedBubble
}

// hail writes one address, its beat, and wakes the guard with `approached`
// at once — [Encounter.bump]'s shape: his answer is in the step's act, and
// the sight is refreshed only when his answer did something a look can see.
func (e *Encounter) hail(r *interactionRecord, mover, guard MemberID) (hailed, error) {
	address := InteractionAddress{Member: mover, Guard: guard}
	r.address = &address
	at := uint64(e.clock.ToData().HighWater)
	appended, err := e.appendAddressBeatEntry(BeatAddressed, r.input.ID, address, "")
	if err != nil {
		return hailed{}, err
	}
	out := hailed{addressed: Addressed{Interaction: r.input.ID, Guard: guard, Seq: appended.Seq, ID: appended.ID}}

	e.reactionPicked, e.blockingTurned, e.stanceFormed = false, false, nil
	prevAct := e.reactAct
	e.reactAct = e.CurrentAct()
	defer func() { e.reactAct = prevAct }()
	cell, _ := e.canvas.GetEntityPosition(string(mover))
	if err := e.matchOne(wakeBeat{Beat: beatApproached, Cell: &cell, Subject: mover, Member: guard, At: at}); err != nil {
		return hailed{}, fmt.Errorf("approached: %w", err)
	}
	out.formed, e.stanceFormed = e.stanceFormed, nil
	if e.outcome != nil || (!e.reactionPicked && !e.blockingTurned) {
		return out, nil
	}
	deltas, formed, err := e.refreshCostingReach(e.cellsOf(mover, guard), mover)
	if err != nil {
		return hailed{}, fmt.Errorf("approached refresh sight: %w", err)
	}
	out.deltas = deltas
	if formed != nil {
		out.formed = formed
	}
	return out, nil
}

// appendAddressBeat appends an address beat, discarding the entry.
func (e *Encounter) appendAddressBeat(beat string, id InteractionID, a InteractionAddress, reason AddressEndReason) error {
	_, err := e.appendAddressBeatEntry(beat, id, a, reason)
	return err
}

// appendAddressBeatEntry appends [BeatAddressed] or [BeatAddressEnded] (with
// its reason), audienced as a step is: to the member and the guard.
func (e *Encounter) appendAddressBeatEntry(
	beat string, id InteractionID, a InteractionAddress, reason AddressEndReason,
) (*record.AppendOutput, error) {
	payload := map[string]interface{}{
		"beat":        beat,
		"interaction": string(id),
		"member":      string(a.Member),
		"guard":       string(a.Guard),
	}
	if reason != "" {
		payload["reason"] = string(reason)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%s beat: %w", beat, err)
	}
	appended, err := e.appendBeat(&record.AppendInput{
		At:       uint64(e.clock.ToData().HighWater),
		Audience: e.audienceFor(subjectBeat, a.Member, a.Guard),
		Tags:     map[string]string{"tag": beat},
		Payload:  raw,
	})
	if err != nil {
		return nil, fmt.Errorf("%s beat: %w", beat, err)
	}
	return appended, nil
}

// validateInteractionAddresses refuses a saved address nobody could have
// written: on an interaction with no approach, naming a member the run does
// not have, a guard the run does not have or who carries none of its
// lines, or a member addressing themselves. options is every saved
// member's lines, by id.
func validateInteractionAddresses(interactions map[InteractionID]*interactionRecord, options map[MemberID][]Option) error {
	for _, id := range slices.Sorted(maps.Keys(interactions)) {
		r := interactions[id]
		if r.address == nil {
			continue
		}
		a := *r.address
		if r.input.Approach == nil {
			return fmt.Errorf("interaction %q is saved addressing %q, and it has no approach", id, a.Member)
		}
		if _, ok := options[a.Member]; !ok {
			return fmt.Errorf("interaction %q is saved addressing %q, who is not in the run", id, a.Member)
		}
		lines, ok := options[a.Guard]
		if !ok {
			return fmt.Errorf("interaction %q is saved addressed by %q, who is not in the run", id, a.Guard)
		}
		if !carries(lines, id) {
			return fmt.Errorf("interaction %q is saved addressed by %q, who carries none of its lines", id, a.Guard)
		}
		if a.Member == a.Guard {
			return fmt.Errorf("interaction %q is saved with %q addressing themselves", id, a.Member)
		}
	}
	return nil
}

// cloneApproach copies an approach so no caller's value is aliased.
func cloneApproach(a *InteractionApproach) *InteractionApproach {
	if a == nil {
		return nil
	}
	c := *a
	return &c
}

// endAddressesOf ends every address that names a member who has left the
// run, as the addressee or as the guard, each with its [BeatAddressEnded]
// and the reason [AddressGone], in interaction id order (rpg-project#942).
// Exit calls it once the member is off the roster: the end-of-step sweep
// would notice the same at the next step, but the run is saved before any
// next step, and a saved address naming a member the run lacks is refused
// on load. The interaction's progress is untouched, as at every sweep.
//
// Errors: a beat's append.
func (e *Encounter) endAddressesOf(member MemberID) error {
	for _, id := range slices.Sorted(maps.Keys(e.interactions)) {
		r := e.interactions[id]
		if r.address == nil || (r.address.Member != member && r.address.Guard != member) {
			continue
		}
		ended := *r.address
		r.address = nil
		if err := e.appendAddressBeat(BeatAddressEnded, id, ended, AddressGone); err != nil {
			return err
		}
	}
	return nil
}
