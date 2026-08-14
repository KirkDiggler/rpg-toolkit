// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// ClockKind names which kind of clock a member is on.
//
// It is an enumerated kind rather than a bool because "is this member in
// combat" is the question a caller will be tempted to ask, and it is the wrong
// one: a third clock kind is expressible later, and a bool would have to be
// re-litigated the moment one arrives.
type ClockKind string

const (
	// ClockWorld is the player-driven world clock — every member's home, and
	// where a member is when no fight has pulled them elsewhere. It is not a
	// "free roam mode"; there is no mode.
	ClockWorld ClockKind = "world"

	// ClockTurn is a localized initiative bubble: an ordered clock that
	// advances a turn at a time and holds only the members caught up in one
	// fight. Members elsewhere in the encounter keep running on ClockWorld
	// while it does.
	ClockTurn ClockKind = "turn"
)

// ClockOfInput names the member whose clock is being read.
type ClockOfInput struct {
	// Member is who to look up.
	Member MemberID
}

// ClockOfOutput reports which clock a member is on, and — when that clock is
// ordered — where in it they stand.
//
// There is deliberately no bubble identifier here. A bubble is never addressed
// by name; it is reached through a member, which R6 ("an entity belongs to at
// most one clock") makes a total function.
type ClockOfOutput struct {
	// Kind is which clock the member is on.
	Kind ClockKind

	// Active is whose turn it currently is on that clock. Empty for
	// ClockWorld, which has no turn order — on the world clock everyone acts,
	// and their own movement is what advances it.
	Active MemberID

	// Round is the current round of that clock, or 0 for ClockWorld.
	Round int

	// Order is the full initiative order of that clock, or nil for ClockWorld.
	// Safe to mutate — clock.Turn.Order copies before returning it, so this is
	// already the caller's own slice.
	Order []MemberID
}

// ClockOf reports which clock a member is on.
//
// This is the query the composition is expected to be asked constantly, and it
// is deliberately member-first rather than clock-first: "whose turn is it" is
// not a question an encounter can answer, because several clocks may be running
// and each has its own answer. Asking it of the encounter would force a single
// privileged clock to exist, which is exactly the mode model this stack does
// not have.
//
// Errors: ErrNilInput, ErrNotMember (the member is not in this encounter).
func (e *Encounter) ClockOf(in *ClockOfInput) (*ClockOfOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("clock_of: %w", ErrNilInput)
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("clock_of %q: %w", in.Member, ErrNotMember)
	}

	bubble, err := e.bubbleFor(in.Member)
	if err != nil {
		return nil, fmt.Errorf("clock_of %q: %w", in.Member, err)
	}
	if bubble == nil {
		// Not in a bubble is not the same as being on the world clock, and
		// answering ClockWorld without checking would make the two
		// indistinguishable.
		//
		// This check is the net under the clock verbs. Form, Transfer, and
		// Dissolve move members BETWEEN clocks, and the way that goes wrong is
		// leaving somebody on neither — a failure mid-Dissolve, for example,
		// returns with only some members re-homed to the world clock (doc.go:
		// the mutate phase is not atomic, and the caller's obligation on error
		// is to drop the encounter unsaved). Without this check that bug would
		// report "free roaming", which is a plausible answer and therefore the
		// expensive kind of wrong.
		// TestClockOfReportsAMemberOnNoClockInsteadOfGuessing fabricates the
		// exact defect this nets and pins the rejection.
		onWorld, cerr := e.clock.Contains(&clock.ContainsInput{ID: core.EntityID(in.Member)})
		if cerr != nil {
			return nil, fmt.Errorf("clock_of %q world: %w", in.Member, cerr)
		}
		if !onWorld {
			return nil, fmt.Errorf("clock_of %q: member is on no clock: %w", in.Member, ErrInvalidData)
		}
		return &ClockOfOutput{Kind: ClockWorld}, nil
	}

	active, err := bubble.Active()
	if err != nil {
		return nil, fmt.Errorf("clock_of %q active: %w", in.Member, err)
	}
	round, err := bubble.Round()
	if err != nil {
		return nil, fmt.Errorf("clock_of %q round: %w", in.Member, err)
	}
	order, err := bubble.Order()
	if err != nil {
		return nil, fmt.Errorf("clock_of %q order: %w", in.Member, err)
	}

	// order is already the caller's own slice to keep: clock.Turn.Order copies
	// before returning. Copying it again here would be a second guarantee with
	// no second effect — and mutation testing showed a redundant copy reads, to
	// the next person, as the thing that makes this safe when it is not.
	return &ClockOfOutput{
		Kind:   ClockTurn,
		Active: MemberID(active),
		Round:  round,
		Order:  order,
	}, nil
}

// bubbleFor returns the bubble holding this member, or nil if they are on the
// world clock. Never returns two: R6 is validated on load and upheld by every
// verb that moves a member between clocks.
func (e *Encounter) bubbleFor(id MemberID) (*clock.Turn, error) {
	for _, b := range e.bubbles {
		in, err := b.Contains(&clock.ContainsInput{ID: core.EntityID(id)})
		if err != nil {
			return nil, err
		}
		if in {
			return b, nil
		}
	}
	return nil, nil
}

// leaveAnyClock removes a member from whichever clock holds them.
//
// Callers that are removing a member from the encounter entirely (Exit) must
// use this rather than leaving the world clock directly: a member caught in a
// bubble is NOT on the world clock, so a bare tick Leave would fail for exactly
// the members most likely to be leaving — the ones in a fight.
func (e *Encounter) leaveAnyClock(id MemberID) error {
	bubble, err := e.bubbleFor(id)
	if err != nil {
		return err
	}
	if bubble != nil {
		if _, rerr := bubble.Remove(&clock.RemoveInput{ID: core.EntityID(id)}); rerr != nil {
			return rerr
		}
		return e.dropBubbleIfIdle(bubble)
	}
	_, lerr := e.clock.Leave(&clock.LeaveInput{ID: core.EntityID(id)})
	return lerr
}

// dropBubbleIfIdle removes b from the bubble list once its order has emptied.
//
// A bubble exists only while a fight does (see Encounter.bubbles). Exit and
// Transfer can each drain a fight one member at a time, and the moment the
// last member is gone the husk must go too: an idle bubble would persist as a
// fight that is not happening, ToData would write it into every blob, and
// LoadEncounter rejects exactly that shape at the trust boundary — the two
// ends of the module must agree on what a bubble in a blob means.
func (e *Encounter) dropBubbleIfIdle(b *clock.Turn) error {
	order, err := b.Order()
	if err != nil {
		return err
	}
	if len(order) > 0 {
		return nil
	}
	for i, x := range e.bubbles {
		if x == b {
			e.bubbles = append(e.bubbles[:i], e.bubbles[i+1:]...)
			break
		}
	}
	return nil
}

// appendClockBeat records one clock-lifecycle story beat, heard by every
// current member and stamped at the world clock's current reading — the same
// audience-and-timestamp convention the membership beats use (Join, Exit).
// Tag "clock" matches Pump's tick beat: one tag family for everything the
// clocks do.
func (e *Encounter) appendClockBeat(payload map[string]interface{}) (uint64, error) {
	memberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		memberIDs = append(memberIDs, id)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	// Unreachable for today's payloads (strings and ID slices marshal
	// unconditionally), but this helper is the one seam every clock beat
	// flows through — a future payload that cannot marshal must fail its
	// verb loudly here, not append a truncated beat and lose the failure.
	// The per-verb beat sites elsewhere in this module predate the seam and
	// keep their own convention.
	beatBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("clock beat payload: %w", err)
	}
	out, err := e.appendBeat(&record.AppendInput{
		At:       uint64(e.clock.ToData().HighWater),
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "clock"},
		Payload:  beatBytes,
	})
	if err != nil {
		return 0, err
	}
	return out.Seq, nil
}

// FormInput carries the rulebook-rolled initiative order a new bubble starts
// with.
type FormInput struct {
	// Order is the complete initiative order for the fight, first-to-act
	// first. It comes from OUTSIDE: play/clock holds no randomness (R7) and
	// this composition holds none either — the rulebook rolls initiative and
	// hands the result in.
	Order []MemberID
}

// FormOutput reports what forming the bubble appended to the story.
type FormOutput struct {
	// Seq is the story sequence of the formation beat.
	Seq uint64
}

// Form starts a fight: the named members leave the world clock together and
// become a turn bubble in the given order. Everyone NOT named stays on the
// world clock and keeps free-roaming while the fight runs — a fight is
// localized, and the rest of the encounter is not paused by it.
//
// Form does not decide WHEN a fight starts. Trigger detection is a later
// step and is deliberately absent here: this verb is explicit and
// caller-driven, and something else decides to call it.
//
// Policy today is ONE bubble per encounter — fights stay linear and the
// party stays together. A second Form while a bubble runs is rejected with
// ErrInBubble whether or not the two fights share a member; when that policy
// lifts, the per-member overlap check below becomes the load-bearing one
// (overlapping bubbles merge via a Merge verb then, but they never form).
//
// Errors: ErrNilInput, ErrClosed, ErrNoMember (empty order or a duplicated
// entry), ErrNotMember (the order names somebody not in this encounter),
// ErrInBubble.
func (e *Encounter) Form(in *FormInput) (*FormOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("form: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("form: %w", ErrClosed)
	}
	if len(in.Order) == 0 {
		return nil, fmt.Errorf("form: order is empty: %w", ErrNoMember)
	}
	seen := make(map[MemberID]bool, len(in.Order))
	for _, id := range in.Order {
		if seen[id] {
			return nil, fmt.Errorf("form: %q appears twice in the order: %w", id, ErrNoMember)
		}
		seen[id] = true
		if _, ok := e.members[id]; !ok {
			return nil, fmt.Errorf("form: %q: %w", id, ErrNotMember)
		}
		bubble, err := e.bubbleFor(id)
		if err != nil {
			return nil, fmt.Errorf("form %q: %w", id, err)
		}
		if bubble != nil {
			return nil, fmt.Errorf("form: %q: %w", id, ErrInBubble)
		}
	}
	if len(e.bubbles) > 0 {
		return nil, fmt.Errorf("form: a bubble is already running and policy is one per encounter: %w", ErrInBubble)
	}

	// Mutate. Validation above guarantees every named member is on the world
	// clock (member + not-in-a-bubble + R6), so these cannot fail against a
	// coherent encounter — but a failure here still returns, and the caller's
	// obligation is doc.go's: drop the encounter unsaved.
	for _, id := range in.Order {
		if _, lerr := e.clock.Leave(&clock.LeaveInput{ID: core.EntityID(id)}); lerr != nil {
			return nil, fmt.Errorf("form member %q world clock: %w", id, lerr)
		}
	}
	bubble := &clock.Turn{}
	if _, serr := bubble.SetOrder(&clock.SetOrderInput{Order: in.Order}); serr != nil {
		return nil, fmt.Errorf("form set order: %w", serr)
	}
	e.bubbles = append(e.bubbles, bubble)

	seq, err := e.appendClockBeat(map[string]interface{}{
		"beat":  "bubble-formed",
		"order": in.Order,
	})
	if err != nil {
		return nil, fmt.Errorf("form append beat: %w", err)
	}
	return &FormOutput{Seq: seq}, nil
}

// TransferInput moves one member between the world clock and the running
// bubble, in either direction.
type TransferInput struct {
	// Member is who is being moved.
	Member MemberID

	// To names the DESTINATION clock kind explicitly. It is not inferred
	// from where the member currently is: that would make the verb a
	// toggle, and under load-act-save a toggle applied to stale state
	// silently moves somebody the WRONG way instead of failing.
	To ClockKind

	// Pos is the slot the member lands at in the bubble's order, 0-based,
	// and is only read when To is ClockTurn — the world clock is unordered,
	// the same convention as the Membership seam's own JoinMember on a Tick.
	Pos int
}

// TransferOutput reports what the transfer appended to the story.
type TransferOutput struct {
	// Seq is the story sequence of the transfer beat.
	Seq uint64
}

// Transfer moves a member between the world clock and the running bubble —
// the straggler who wandered too close falls in mid-round at a caller-chosen
// slot, and someone the fight is done with steps back out without ending it.
// R6 holds through the move: the underlying play/clock Transfer joins first
// and compensates on failure, so a rejected transfer leaves both clocks
// exactly as they were.
//
// The destination bubble is unambiguous while policy holds one bubble per
// encounter; addressing a specific fight (through one of its members — a
// bubble is never named) arrives with multiple bubbles.
//
// Errors: ErrNilInput, ErrClosed, ErrNoMember, ErrNotMember, ErrBadClock
// (To names neither kind), ErrInBubble (To is ClockTurn but the member is
// already fighting), ErrNoBubble (To is ClockTurn with no fight running, or
// To is ClockWorld for a member not in one), and play/clock's own rejections
// — an out-of-range Pos propagates clock.ErrBadPosition.
func (e *Encounter) Transfer(in *TransferInput) (*TransferOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("transfer: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("transfer: %w", ErrClosed)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("transfer: %w", ErrNoMember)
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("transfer %q: %w", in.Member, ErrNotMember)
	}
	bubble, err := e.bubbleFor(in.Member)
	if err != nil {
		return nil, fmt.Errorf("transfer %q: %w", in.Member, err)
	}

	switch in.To {
	case ClockTurn:
		if bubble != nil {
			return nil, fmt.Errorf("transfer %q: %w", in.Member, ErrInBubble)
		}
		if len(e.bubbles) == 0 {
			return nil, fmt.Errorf("transfer %q: no fight is running: %w", in.Member, ErrNoBubble)
		}
		if _, terr := clock.Transfer(&clock.TransferInput{
			From: e.clock,
			To:   e.bubbles[0],
			ID:   core.EntityID(in.Member),
			Pos:  in.Pos,
		}); terr != nil {
			return nil, fmt.Errorf("transfer %q into bubble: %w", in.Member, terr)
		}
	case ClockWorld:
		if bubble == nil {
			return nil, fmt.Errorf("transfer %q: %w", in.Member, ErrNoBubble)
		}
		if _, terr := clock.Transfer(&clock.TransferInput{
			From: bubble,
			To:   e.clock,
			ID:   core.EntityID(in.Member),
		}); terr != nil {
			return nil, fmt.Errorf("transfer %q out of bubble: %w", in.Member, terr)
		}
		if derr := e.dropBubbleIfIdle(bubble); derr != nil {
			return nil, fmt.Errorf("transfer %q prune bubble: %w", in.Member, derr)
		}
	default:
		return nil, fmt.Errorf("transfer %q to %q: %w", in.Member, in.To, ErrBadClock)
	}

	seq, err := e.appendClockBeat(map[string]interface{}{
		"beat":   "transferred",
		"member": string(in.Member),
		"to":     string(in.To),
	})
	if err != nil {
		return nil, fmt.Errorf("transfer append beat: %w", err)
	}
	return &TransferOutput{Seq: seq}, nil
}

// EndTurnInput names the member ending their own turn in the fight they are
// in.
type EndTurnInput struct {
	// Member is whose turn is ending. It must actually BE their turn —
	// ending somebody else's is rejected by the bubble itself.
	Member MemberID
}

// EndTurnOutput reports who acts next and whether the round wrapped.
type EndTurnOutput struct {
	// Next is whose turn it now is in the same bubble.
	Next MemberID

	// RoundWrapped is true when this end closed the round — the order
	// cycled back to its first member and the bubble's round advanced.
	RoundWrapped bool

	// Seq is the story sequence of the turn-ended beat.
	Seq uint64
}

// EndTurn advances the fight past the member's turn. This is the bubble's
// own advancement — the world clock is untouched, and members outside the
// fight never appear in it.
//
// Errors: ErrNilInput, ErrClosed, ErrNoMember, ErrNotMember, ErrNoBubble
// (the member is not in a fight), and the bubble's own rejection when it is
// not this member's turn (clock.ErrNotActive, with no state change).
func (e *Encounter) EndTurn(in *EndTurnInput) (*EndTurnOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("end turn: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("end turn: %w", ErrClosed)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("end turn: %w", ErrNoMember)
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("end turn %q: %w", in.Member, ErrNotMember)
	}
	bubble, err := e.bubbleFor(in.Member)
	if err != nil {
		return nil, fmt.Errorf("end turn %q: %w", in.Member, err)
	}
	if bubble == nil {
		return nil, fmt.Errorf("end turn %q: %w", in.Member, ErrNoBubble)
	}

	out, err := bubble.End(&clock.EndInput{Actor: core.EntityID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("end turn %q: %w", in.Member, err)
	}

	seq, err := e.appendClockBeat(map[string]interface{}{
		"beat":   "turn-ended",
		"member": string(in.Member),
	})
	if err != nil {
		return nil, fmt.Errorf("end turn append beat: %w", err)
	}
	return &EndTurnOutput{
		Next:         MemberID(out.Next),
		RoundWrapped: out.RoundWrapped,
		Seq:          seq,
	}, nil
}

// DissolveInput names a member of the fight being ended. The bubble is
// reached through a member — never addressed by name — which R6 makes a
// total function.
type DissolveInput struct {
	// Member is anyone in the fight to dissolve.
	Member MemberID
}

// DissolveOutput reports who the fight held and where the story recorded
// its end.
type DissolveOutput struct {
	// Members is everyone the fight held, in bubble order, all re-homed to
	// the world clock at budget zero.
	Members []MemberID

	// Seq is the story sequence of the dissolution beat.
	Seq uint64
}

// Dissolve ends the fight holding the named member: the bubble empties and
// every member re-homes to the world clock, at budget zero — time spent
// fighting was spent, not banked.
//
// Errors: ErrNilInput, ErrClosed, ErrNoMember, ErrNotMember, ErrNoBubble
// (the member is not in a fight). A failure while re-homing leaves members
// between clocks — the window ClockOf's on-no-clock check nets — and the
// caller's obligation is doc.go's: drop the encounter unsaved.
func (e *Encounter) Dissolve(in *DissolveInput) (*DissolveOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("dissolve: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("dissolve: %w", ErrClosed)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("dissolve: %w", ErrNoMember)
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("dissolve %q: %w", in.Member, ErrNotMember)
	}
	bubble, err := e.bubbleFor(in.Member)
	if err != nil {
		return nil, fmt.Errorf("dissolve %q: %w", in.Member, err)
	}
	if bubble == nil {
		return nil, fmt.Errorf("dissolve %q: %w", in.Member, ErrNoBubble)
	}

	out, err := bubble.Dissolve(&clock.DissolveInput{})
	if err != nil {
		return nil, fmt.Errorf("dissolve: %w", err)
	}
	for _, id := range out.Members {
		if _, jerr := e.clock.Join(&clock.JoinInput{ID: id}); jerr != nil {
			return nil, fmt.Errorf("dissolve member %q world clock: %w", id, jerr)
		}
	}
	if derr := e.dropBubbleIfIdle(bubble); derr != nil {
		return nil, fmt.Errorf("dissolve prune bubble: %w", derr)
	}

	seq, err := e.appendClockBeat(map[string]interface{}{
		"beat":    "bubble-dissolved",
		"members": out.Members,
	})
	if err != nil {
		return nil, fmt.Errorf("dissolve append beat: %w", err)
	}
	return &DissolveOutput{Members: out.Members, Seq: seq}, nil
}
