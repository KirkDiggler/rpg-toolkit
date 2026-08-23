// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
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
		if derr := e.dropBubbleIfIdle(bubble); derr != nil {
			return derr
		}
		// The exiting member may have been active; see driveIfStillRunning
		// (rpg-toolkit#1162) — Exit reaches this the same as a mid-fight
		// Transfer does.
		return e.driveIfStillRunning(bubble)
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

// lastRecordedSeq returns the sequence of the most recently appended story
// entry, or 0 if nothing has ever been appended (unreachable in practice —
// Setup's own scene-opened beat is always first).
//
// Its one caller is driveOneMonsterTurn's own bubble-already-dissolved case:
// a driven member whose killing blow just ended the fight has no
// "turn-ended" beat of its own left to append (there is no bubble left to
// end a turn IN), but the caller still needs an honest answer to "how far
// did the story move," and the dissolution's own beat — already appended,
// several calls down, inside the Record that noticed the kill — is the
// truthful one.
func (e *Encounter) lastRecordedSeq() uint64 {
	next, err := e.story.NextSeq()
	if err != nil || next == 0 {
		return 0
	}
	return next - 1
}

// driveMonsterTurns advances bubble past every consecutive member with no
// player, starting from its current Active member, stopping at the first
// KindPlayer member.
//
// THE SINGLE CHOKE POINT both EndTurn and form route through, so "who acts
// for a member nobody plays" has exactly one answer regardless of which
// moment discovers it — a turn ending, or a fight forming with an unplayed
// member first in initiative (rpg-toolkit#1162, ADR-0043).
//
// v1's TurnDriver has exactly one outcome (Pass), so every step here is
// bubble.End plus the same "turn-ended" beat EndTurn's own acting-member step
// already produces — the story and the EventTurnEnded stream need no new
// vocabulary to show a monster's pass. wrapped is true if ANY step in the
// chain wrapped the round, and lastSeq is the final beat's sequence; a caller
// that wants every intermediate beat already has it through Story's ordinary
// baseline-and-fan-out mechanism.
//
// A DRIVER ERROR ABORTS THE WHOLE CALL — including whatever the caller
// already did before invoking this (EndTurn's own bubble.End for the acting
// member, form's bubble construction). Nothing here needs to roll that back
// by hand: this SDK's load-mutate-save shape means nothing is persisted until
// the verb's own commit, so an error return simply discards the in-memory
// encounter and leaves the stored world exactly as it was. This mirrors
// Pump's rule for Decider errors ("aborts atomically... no clock advance, no
// moves, no beats") using the mechanism this seam already has.
// bubbleHasPlayer reports whether any member of order has a player.
func (e *Encounter) bubbleHasPlayer(order []core.EntityID) bool {
	for _, id := range order {
		if m, ok := e.members[MemberID(id)]; ok && m.Kind == KindPlayer {
			return true
		}
	}
	return false
}

func (e *Encounter) driveMonsterTurns(bubble *clock.Turn) (wrapped bool, lastSeq uint64, err error) {
	// RE-ENTRANCY GUARD (rpg-toolkit#1207). driveMonsterTurns is the single
	// owner of driving a bubble forward, and this call may already be
	// running deeper on THIS SAME Go call stack: a driven member's own
	// Strike can down a teammate without deciding the fight, whose
	// noticeDown -> Transfer(id, ClockWorld) -> driveIfStillRunning reaches
	// back here while the OUTER driveOneMonsterTurn call is still mid-turn,
	// budget not yet spent. Without this check that reentry would hand the
	// SAME still-acting monster a second, undocumented turn under a fresh
	// budget — exactly the defect this guard exists to make impossible
	// rather than merely unlikely. A nested call is a no-op the caller
	// ignores: the outer call already owns driving this bubble and will
	// finish it once its own Strike call returns.
	if e.driving {
		return false, 0, nil
	}
	e.driving = true
	defer func() { e.driving = false }()

	order, err := bubble.Order()
	if err != nil {
		return false, 0, fmt.Errorf("drive monster turns: %w", err)
	}

	if !e.bubbleHasPlayer(order) {
		return false, 0, fmt.Errorf("drive monster turns: %w", ErrNoPlayerInBubble)
	}

	// Bounded by the order's own length: driving past every member once is
	// the most this loop could ever legitimately need, and hasPlayer above
	// already guarantees it terminates well before then.
	for i := 0; i < len(order); i++ {
		// A member driven earlier in THIS SAME loop can have ended the fight
		// with its own killing blow — see driveOneMonsterTurn's own doc.
		// bubble is then idle, and idle is a state bubble.Active() refuses
		// with ErrIdle. Order() never does (an idle clock answers with an
		// empty slice, by its own contract), so it is the check that lets
		// this loop notice "nothing is left to drive" and stop cleanly,
		// rather than asking a dissolved clock whose turn it still thinks it
		// is.
		remaining, oerr := bubble.Order()
		if oerr != nil {
			return wrapped, lastSeq, fmt.Errorf("drive monster turns: %w", oerr)
		}
		if len(remaining) == 0 {
			break
		}

		active, aerr := bubble.Active()
		if aerr != nil {
			return wrapped, lastSeq, fmt.Errorf("drive monster turns: %w", aerr)
		}
		activeID := MemberID(active)
		m, ok := e.members[activeID]
		if !ok || m.Kind == KindPlayer {
			break
		}

		seq, roundWrapped, derr := e.driveOneMonsterTurn(bubble, active, m)
		if derr != nil {
			return wrapped, lastSeq, fmt.Errorf("drive monster turns %q: %w", activeID, derr)
		}
		wrapped = wrapped || roundWrapped
		lastSeq = seq
	}

	return wrapped, lastSeq, nil
}

// driveOneMonsterTurn runs ONE unplayed member's whole turn: build view →
// Act → execute → repeat until [Pass] or the budget is spent, then end the
// turn. Returns the sequence of the terminating "turn-ended" beat.
//
// THE BUDGET IS ONE ATTACK PLUS THE MEMBER'S OWN SPEED, IN FEET (Kirk,
// rpg-project#254 review). A driver is asked again after every executed
// intent — an Attack that lands still leaves movement to spend, a Move that
// only closes half the distance still leaves the attack unspent — and the
// inner loop is bounded (attacksLeft + movement-in-cells + one terminating
// call) so a driver that never returns Pass cannot spin this forever.
//
// A DRIVER'S OWN Go ERROR ABORTS THE WHOLE CALL, exactly as it always has —
// see [TurnDriver.Act]'s doc. A syntactically valid but unexecutable intent
// does NOT: it is [ErrBadIntent], and this member's turn simply ends as
// [Pass] would, so the caller's own verb (EndTurn, form) still succeeds.
// This is the asymmetry the brief calls for: a driver malfunction is this
// module's problem, a bad decision is the driver's own business, and only
// one of those should cost the whole call.
func (e *Encounter) driveOneMonsterTurn(
	bubble *clock.Turn, active core.EntityID, m *memberRecord,
) (seq uint64, roundWrapped bool, err error) {
	activeID := MemberID(active)

	round, rerr := bubble.Round()
	if rerr != nil {
		return 0, false, fmt.Errorf("round: %w", rerr)
	}

	budget := TurnBudget{AttacksLeft: 1, MovementFeet: m.SpeedFeet}

	// See the function doc: bounded so a misbehaving driver cannot spin the
	// caller. +2 covers one attack and the terminating Pass a well-behaved
	// driver returns once it has nothing left; the rest is however many
	// cells this member could ever afford to ask for one at a time.
	innerBound := 2 + CellsFromFeet(budget.MovementFeet)

	for j := 0; j < innerBound; j++ {
		view, verr := e.buildMonsterView(m, budget, round)
		if verr != nil {
			return 0, false, fmt.Errorf("view: %w", verr)
		}

		intent, derr := e.turnDriver.Act(view)
		if derr != nil {
			return 0, false, fmt.Errorf("act: %w", derr)
		}

		done, dexecerr := e.executeTurnIntent(activeID, m, view, intent, &budget)
		if dexecerr != nil {
			return 0, false, fmt.Errorf("execute: %w", dexecerr)
		}
		if done {
			break
		}
	}

	// A step just executed (almost always a Strike) can have ended the fight
	// out from under this very turn: [Encounter.noticeDown], reached through
	// Record, dissolves a bubble that has run out of a side the instant it
	// notices (rpg-toolkit#1078, ByDefeat) — and a driven member's own
	// killing blow notices EXACTLY that. When it does, bubble is the same
	// *clock.Turn this call has held throughout, now idle: there is nothing
	// left to end, and nothing wrong — the "bubble-dissolved" beat that
	// noticing already appended tells the whole story of why this member's
	// own turn never explicitly closed. Calling bubble.End regardless would
	// ask an idle clock to end a turn it no longer has, and play/clock
	// refuses that with ErrIdle exactly as it should — which is what turned
	// a driven monster's own killing blow into an error on the CALLER's
	// verb (rpg-project#254's live walk, round 4) until this check existed.
	stillRunning, cerr := bubble.Contains(&clock.ContainsInput{ID: active})
	if cerr != nil {
		return 0, false, fmt.Errorf("contains: %w", cerr)
	}
	if !stillRunning {
		return e.lastRecordedSeq(), false, nil
	}

	out, eerr := bubble.End(&clock.EndInput{Actor: active})
	if eerr != nil {
		return 0, false, fmt.Errorf("end: %w", eerr)
	}

	seq, berr := e.appendClockBeat(map[string]interface{}{
		"beat":   "turn-ended",
		"member": string(activeID),
		"next":   out.Next,
	})
	if berr != nil {
		return 0, false, fmt.Errorf("append beat: %w", berr)
	}
	return seq, out.RoundWrapped, nil
}

// executeTurnIntent runs one Act call's answer and reports whether this
// member's turn is over — [Pass], an [ErrBadIntent]-refused [Attack] or
// [Move], or a [Move] that made no progress at all all end the turn the same
// way; a successful [Attack] or [Move] leaves it running so the driver is
// asked again against the updated budget and view.
//
// budget is mutated in place: the one thing every branch here shares is that
// what it spends must be visible to the NEXT buildMonsterView call, and a
// return value the caller has to remember to feed back in is exactly the
// kind of two-truths bug this composition's own field.go doc warns against
// elsewhere.
func (e *Encounter) executeTurnIntent(
	activeID MemberID, m *memberRecord, view MonsterView, intent TurnIntent, budget *TurnBudget,
) (done bool, err error) {
	switch it := intent.(type) {
	case Pass:
		return true, nil

	case Attack:
		if !attackIsInReach(view, it) || budget.AttacksLeft <= 0 {
			// ErrBadIntent: not a driver malfunction (see the type's own
			// doc) — this member's turn simply ends, exactly like Pass.
			return true, nil
		}

		// context.Background() rather than a threaded caller context: no
		// verb on this composition accepts one today (EndTurn, form, Step —
		// see this module's CLAUDE.md, "play/* leaves take no
		// context.Context"), and Striker is the first capability that does,
		// by the brief's own design. Plumbing a context through every
		// verb between a host's request and here is a bigger change than
		// this slice asks for; the day Striker's own implementation needs
		// cancellation propagated from the host, that is the day to revisit
		// this rather than the day to speculatively add it everywhere now.
		if serr := e.striker.Strike(context.Background(), e, activeID, it.Target, it.Action); serr != nil {
			return false, fmt.Errorf("strike: %w", serr)
		}
		budget.AttacksLeft = 0
		return false, nil

	case Move:
		cellsAffordable := CellsFromFeet(budget.MovementFeet)
		if len(it.Path) == 0 || len(it.Path) > cellsAffordable {
			// ErrBadIntent: a driver asking for more than its own budget, or
			// for nothing at all (that is what Pass is for) — this member's
			// turn ends rather than executing a path it did not fully ask
			// for.
			return true, nil
		}

		at := uint64(e.clock.ToData().HighWater)
		audience := e.rosterIDs()
		moved := 0
		for _, cell := range it.Path {
			action, stepped := e.stepTo(m, cell)
			if !stepped {
				// The same silent-refusal contract stepTo already has for
				// the pump: a wall stops the WALK, not the turn — whatever
				// cells succeeded before it are kept (rpg-project#254,
				// "the same refusals a player's walk meets").
				break
			}
			if _, berr := e.appendMovementBeat(action, audience, at); berr != nil {
				return false, fmt.Errorf("move beat: %w", berr)
			}
			moved++
		}
		budget.MovementFeet -= moved * FeetPerCell

		if moved > 0 {
			// Refreshed once for the whole intent, not per cell — the same
			// batching Pump uses for its own multi-member tick, and what
			// makes the NEXT buildMonsterView call in this same turn see
			// where this member actually ended up. A straggler this move
			// brings into contact JOINS the running bubble rather than
			// starting a second one (trigger.go's classify: "a fight
			// already running is joined, not started again") — the one-
			// bubble policy is not at risk here.
			if _, _, serr := e.refreshSight(audience); serr != nil {
				return false, fmt.Errorf("refresh sight: %w", serr)
			}
		}

		// Zero cells succeeded: the driver asked for a path that could not
		// even start from here. Ending the turn rather than re-asking
		// avoids spinning on a request that will not succeed differently
		// next time this same view is built.
		return moved == 0, nil

	default:
		return false, fmt.Errorf("driver returned %T: %w", intent, ErrBadTurnOutcome)
	}
}

// attackIsInReach reports whether intent's target is a currently Seen,
// Standing member within intent's own Action's reach.
//
// A SINGLE MAP LOOKUP DOES BOTH CHECKS AT ONCE, deliberately: SeenMember's
// InReach is only ever populated for THIS member's own [MonsterView.Actions]
// (see [Encounter.buildMonsterView]), so a Ref that does not name one of
// this member's own actions is simply absent from the map — Go's zero value
// for a missing key is false, which is exactly "not in reach" for an action
// this member does not have. No separate "is this even my action" check is
// needed because there is no way to reach true without it also being true.
func attackIsInReach(view MonsterView, intent Attack) bool {
	for _, sm := range view.Seen {
		if sm.ID != intent.Target {
			continue
		}
		return sm.Standing && sm.InReach[intent.Action]
	}
	return false
}

// buildMonsterView projects member's own static facts, its currently active
// sight intel, and the turn's remaining budget into the shape a [TurnDriver]
// is allowed to see — the same anti-wall-hack contract [Decider]'s Snapshot
// keeps (C2), extended to a turn's own questions.
func (e *Encounter) buildMonsterView(m *memberRecord, budget TurnBudget, round int) (MonsterView, error) {
	ownCell, err := e.cellOf(m)
	if err != nil {
		return MonsterView{}, fmt.Errorf("position: %w", err)
	}

	down, err := e.standingNow()
	if err != nil {
		return MonsterView{}, fmt.Errorf("standing: %w", err)
	}

	// The member's own holdings and nothing else (C2) — the same call
	// Pump's own Decider consult makes for a Snapshot, one seam over.
	holdings, err := e.intelLog.HeldBy(&intel.HeldByInput{Observer: m.ID})
	if err != nil {
		return MonsterView{}, fmt.Errorf("held by: %w", err)
	}

	// bestReachCells is the farthest this member's own actions can reach,
	// in cells — the arithmetic max of authored ReachFeet values, not a
	// rules opinion about which action is "best" in play (this module
	// still cannot import the rulebook, C1: it takes the number, not the
	// meaning). Floored at 1 (5 feet, a bare cell) even for a member with
	// no actions at all: the floor is a SPATIAL fact independent of combat
	// capability — nobody's own Path should ever walk onto another
	// member's occupied cell, whether or not this member has anything to
	// do once it arrives.
	bestReachCells := 1
	for _, a := range m.Actions {
		if c := CellsFromFeet(a.ReachFeet); c > bestReachCells {
			bestReachCells = c
		}
	}

	var seen []SeenMember
	for _, h := range holdings {
		// Sight channel, ACTIVELY sustained only (intel.Current) — a stale
		// "held" memory (intel.Held, a ghost: known but not currently
		// sustained) is not something this member can act on this turn, so
		// it is excluded here rather than left for a driver to filter.
		if h.Channel != intel.Sight || h.Status != intel.Current {
			continue
		}
		subjectID := MemberID(h.Subject)
		other, ok := e.members[subjectID]
		if !ok {
			continue
		}
		pos, ok := DecodeSightPayload(h.Payload)
		if !ok {
			continue
		}

		dist := e.Distance(ownCell, pos)
		inReach := make(map[core.Ref]bool, len(m.Actions))
		for _, a := range m.Actions {
			inReach[a.Ref] = dist <= float64(CellsFromFeet(a.ReachFeet))
		}

		// Path ends on the NEAREST WALKABLE cell (to this member's own
		// position — the shortest route BY STEPS, not "however far along
		// the route to pos happens to land") from which the sighting is
		// within bestReachCells (Kirk, rpg-project#254 review — this also
		// removes the "already in reach, don't bother moving" workaround
		// behavior.Basic carried before this truncation existed).
		//
		// SEARCHED AS ITS OWN GOAL, NOT TRUNCATED FROM A ROUTE TO pos
		// (Copilot, PR #1191 review): a route to pos and a route to "any
		// cell within reach of pos" are different questions the moment a
		// movement-blocking boundary makes them different — a
		// geometrically closer in-range cell can be a walkable dead end
		// relative to the far side of a wall, reachable in fewer steps
		// than continuing on toward pos itself, and InReach never checked
		// walkability to begin with (it is distance-only, matching 5e's
		// own reach rule). bfsShortestPath's goal predicate stops the
		// search the moment ANY cell — including this member's own
		// starting position, handling "already in reach" for free — is
		// within bestReachCells of pos, which is the actual nearest
		// walkable answer rather than a proxy for it. One BFS per
		// sighting, every Act call — see the field's own doc for why that
		// cost is accepted rather than deferred behind a lazy capability.
		path, _ := e.bfsShortestPath(ownCell, func(cell spatial.Position) bool {
			return e.Distance(cell, pos) <= float64(bestReachCells)
		})

		seen = append(seen, SeenMember{
			ID:            subjectID,
			Kind:          other.Kind,
			Standing:      !down[subjectID],
			Position:      pos,
			DistanceCells: dist,
			InReach:       inReach,
			Path:          path,
		})
	}
	// C8: a driver asked twice against unchanged data must see the same
	// order, not whatever order the intel map happened to range in.
	sort.Slice(seen, func(i, j int) bool { return seen[i].ID < seen[j].ID })

	return MonsterView{
		Self:      m.ID,
		Position:  ownCell,
		Actions:   m.Actions,
		Targeting: m.Targeting,
		Seen:      seen,
		Budget:    budget,
		Round:     round,
	}, nil
}

// pathTo computes the shortest path from `from` to `to` over this
// composition's own floor and walls — a plain breadth-first search, which
// is exact (not merely a heuristic) here because every edge this module's
// grids offer costs exactly one cell (Chebyshev on a square grid, cube
// distance on a hex one); there is no weighted-edge case for A* to earn its
// keep over. Returns the path EXCLUDING `from` and INCLUDING `to`, or
// ok=false when `to` is not floor or no floor-connected route reaches it.
//
// DOES NOT CONSULT OCCUPANCY, deliberately: this answers "how would the
// geometry let me get there", the same question a player planning a route
// asks before checking whether someone is standing in the doorway. A cell
// another member currently occupies still refuses at actual step time
// (stepMember's own CanPlaceEntity gate), and driveMonsterTurns already
// treats a mid-move refusal as "stop the walk, keep the turn running" —
// exactly the shape a transient occupant should have, not a permanently
// unreachable cell a stale path would have to be recomputed around.
//
// NEIGHBOURS ARE VISITED IN A FIXED ORDER (C8): map iteration has none, and
// a driver asked twice against unchanged geometry must get the same answer
// both times, not merely an equally-short one.
// bfsShortestPath is the search engine [Encounter.buildMonsterView]'s
// reach-aware Path runs on: a breadth-first search from `from` over this
// composition's own floor and
// walls, stopping at the first cell — in BFS DISCOVERY order, which is
// shortest-distance-from-`from` order — satisfying `goal`. goal(from)
// itself is checked first: a caller whose own starting cell already
// satisfies the goal gets an EMPTY path with ok=true, not a one-element
// path naming its own position.
//
// Exact (not a heuristic): every edge this module's grids offer costs
// exactly one cell (Chebyshev on a square grid, cube distance on a hex
// one), so BFS IS shortest-path here — there is no weighted-edge case for
// A* to earn its keep over. Returns the path EXCLUDING `from` and
// INCLUDING the first cell satisfying goal, or ok=false when no reachable
// cell ever does.
//
// NEIGHBOURS ARE VISITED IN A FIXED ORDER (C8): map iteration has none,
// and a driver asked twice against unchanged geometry must get the same
// answer both times, not merely an equally-short one.
func (e *Encounter) bfsShortestPath(from spatial.Position, goal func(spatial.Position) bool) ([]spatial.Position, bool) {
	if goal(from) {
		return nil, true
	}

	grid := e.canvas.GetGrid()
	visited := map[spatial.Position]bool{from: true}
	prev := make(map[spatial.Position]spatial.Position)
	queue := []spatial.Position{from}

	var found spatial.Position
	ok := false

	for len(queue) > 0 && !ok {
		cur := queue[0]
		queue = queue[1:]

		neighbors := grid.GetNeighbors(cur)
		sort.Slice(neighbors, func(i, j int) bool {
			if neighbors[i].X != neighbors[j].X {
				return neighbors[i].X < neighbors[j].X
			}
			return neighbors[i].Y < neighbors[j].Y
		})

		for _, n := range neighbors {
			if visited[n] {
				continue
			}
			if _, owned := e.RegionAt(n); !owned {
				continue
			}
			if e.canvas.IsBoundaryMovementBlocked(cur, n) {
				continue
			}
			visited[n] = true
			prev[n] = cur
			if goal(n) {
				found = n
				ok = true
				break
			}
			queue = append(queue, n)
		}
	}

	if !ok {
		return nil, false
	}

	var path []spatial.Position
	for at := found; at != from; at = prev[at] {
		path = append(path, at)
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path, true
}

// driveIfStillRunning calls driveMonsterTurns on bubble if it still holds any
// members after whatever removal the caller just made — a no-op if that
// removal emptied it (the fight is over, not stuck) and a no-op if the
// member left active already has a player.
//
// THE SECOND CHOKE POINT, alongside EndTurn and form: a member leaving a
// bubble mid-fight (Transfer to the world clock, or Exit) can hand the active
// slot to whoever was next in the order, exactly as ending a turn does — and
// if that member has no player, the fight would stall on them just as surely
// (rpg-toolkit#1162). Both callers reach this through the SAME
// driveMonsterTurns, so there remains exactly one place that decides what an
// unplayed member does.
func (e *Encounter) driveIfStillRunning(bubble *clock.Turn) error {
	order, err := bubble.Order()
	if err != nil {
		return fmt.Errorf("drive if still running: %w", err)
	}
	if len(order) == 0 {
		return nil
	}

	// UNLIKE EndTurn and form, a player-free bubble IS reachable here: Exit
	// (and Transfer to the world clock) can drain a fight's last player one
	// member at a time, leaving a fight of monsters alone as a legitimate,
	// tolerated intermediate state — TestADrainedBubbleIsPruned pins exactly
	// this ("a fight of one is still a fight"). Nobody is left to hand a
	// driven-through turn back to, so this is a no-op rather than the defect
	// ErrNoPlayerInBubble names for EndTurn/form's unreachable case.
	if !e.bubbleHasPlayer(order) {
		return nil
	}

	_, _, err = e.driveMonsterTurns(bubble)
	return err
}

// FormInput carries the rulebook-rolled initiative order a new bubble starts
// with. Built by trigger detection; see form.
type FormInput struct {
	// Order is the complete initiative order for the fight, first-to-act
	// first. It comes from OUTSIDE: play/clock holds no randomness (R7) and
	// this composition holds none either — the rulebook rolls initiative and
	// hands the result in.
	Order []MemberID

	// Surprised names the members entering this fight unaware, and is carried
	// rather than derived because awareness is a fact about the moment the
	// bubble formed: a member surprised at formation stays surprised through
	// their first turn however the fight then develops.
	//
	// A subset of Order. Nobody outside the order can be surprised by a fight
	// they are not in.
	Surprised []MemberID
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
// UNEXPORTED as of rpg-toolkit#964, and the doc this replaces predicted it:
// "Form does not decide WHEN a fight starts. Trigger detection is a later step
// and is deliberately absent here... something else decides to call it." That
// something else now exists. The verb was scaffolding until its decider
// arrived, and a caller-driven Form beside an automatic one would be two
// systems deciding the same thing (ADR-0032) — with the caller always losing,
// because trigger detection reaches the contact first.
//
// The public story of "a fight exists" is unchanged and lives elsewhere:
// [Encounter.Transfer] moves members in and out, [Encounter.ClockOf] and
// [Encounter.Status] report who is on which clock, [Encounter.Dissolve] ends
// it, and [FormedBubble] on the move-path outputs announces the start with its
// order and who was surprised. What went away is the ability to start a fight
// that nothing noticed.
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
func (e *Encounter) form(in *FormInput) (*FormOutput, error) {
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
	// Surprise is a fact about members of THIS fight. Somebody outside the
	// order cannot be surprised by it, and accepting that quietly would put a
	// name in the story that the order does not explain.
	for _, id := range in.Surprised {
		if !seen[id] {
			return nil, fmt.Errorf("form: %q is surprised but not in the order: %w", id, ErrNotMember)
		}
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

	beat := map[string]interface{}{
		"beat":  "bubble-formed",
		"order": in.Order,
	}
	// Recorded rather than merely returned: surprise is consumed a turn later
	// (a surprised creature loses its first turn), so the story has to carry
	// it for a reader reconstructing the fight from beats alone.
	if len(in.Surprised) > 0 {
		beat["surprised"] = in.Surprised
	}
	seq, err := e.appendClockBeat(beat)
	if err != nil {
		return nil, fmt.Errorf("form append beat: %w", err)
	}

	// If initiative rolled an unplayed member first, nobody has reached this
	// fight's clock yet to end their turn for them — the fight-start half of
	// rpg-toolkit#1162. Driven AFTER the formation beat, deliberately: a
	// reader replaying the story must see the fight announced before anyone's
	// turn inside it can end, and a client reading FIGHT_STARTED sees a
	// PLAYED member active by the time this verb returns either way.
	if _, _, derr := e.driveMonsterTurns(bubble); derr != nil {
		return nil, fmt.Errorf("form: %w", derr)
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
		// Read BEFORE the clock-level move: whether the departing member
		// was the one whose slot is about to need rescuing is a fact about
		// the bubble as it stood a moment ago, not after (rpg-toolkit#1207).
		wasActive, aerr := bubble.Active()
		if aerr != nil {
			return nil, fmt.Errorf("transfer %q: %w", in.Member, aerr)
		}
		departedWasActive := wasActive == core.EntityID(in.Member)

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
		// The departing member may have been active; whoever inherited the
		// slot is driven forward if they have no player (rpg-toolkit#1162) —
		// this is how noticeDown's splice of a fallen body stays safe, since
		// it reaches this same branch through Transfer.
		//
		// ONLY WHEN THE DEPARTING MEMBER GENUINELY WAS ACTIVE, though
		// (rpg-toolkit#1207). A member spliced out of a bubble whose active
		// slot belongs to somebody else — most commonly a driven monster
		// still mid-turn, reached here through its own Strike's noticeDown
		// — leaves nothing stalled to rescue: the active member's own turn
		// is already running and will end itself in the ordinary way.
		// Driving again here would hand that still-running turn a second,
		// undocumented one under a second, fresh budget — the exact defect
		// TestADownedTeammateDoesNotHandTheDrivenMonsterASecondTurn found,
		// reproduced directly against the composition in
		// rulebooks/dnd5e/encounter/monsterturn_test.go (encounter_test
		// package) after first surfacing while writing the session
		// package's own rpg-toolkit#1205. driveMonsterTurns' own
		// re-entrancy guard (its doc) makes the same mistake impossible a
		// second, structural way; this is the semantic one — the call
		// this branch would otherwise be making is simply the wrong one to
		// make at all when nothing is actually stalled.
		if departedWasActive {
			if derr := e.driveIfStillRunning(bubble); derr != nil {
				return nil, fmt.Errorf("transfer %q: %w", in.Member, derr)
			}
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
	// Next is whose turn it now is in the same bubble — ALWAYS a member with
	// a player. If the clock would otherwise have landed on one or more
	// unplayed members, this call already drove them forward via TurnDriver
	// before returning (rpg-toolkit#1162): the caller never receives a Next
	// nobody can act for.
	//
	// EXCEPT when a driven member's own turn ends the fight (its killing
	// blow — rpg-toolkit#1078, ByDefeat — noticed while THIS call was still
	// driving it forward): there is then no bubble left to name a Next
	// turn IN, and Next reports Member instead — the caller who ended their
	// own turn, now free again. A caller learns the fight actually ended,
	// as it always has, from the "bubble-dissolved" beat this same call's
	// own fan-out carries, not from a distinguishable value here.
	Next MemberID

	// RoundWrapped is true when this end, OR any unplayed member's pass this
	// call drove through on the way to Next, closed the round — the order
	// cycled back to its first member and the bubble's round advanced.
	RoundWrapped bool

	// Seq is the story sequence of the LAST beat this call recorded — its own
	// turn-ended beat, or the final unplayed member's pass beat if any were
	// driven. Every beat in between is in the story too; a caller that wants
	// each one reads Story from its own baseline, the same way every other
	// verb's fan-out works.
	Seq uint64
}

// EndTurn advances the fight past the member's turn — and past every
// consecutive member after them with no player, so the turn this call hands
// back to its caller is always somebody who can actually receive it
// (rpg-toolkit#1162, ADR-0043). This is the bubble's own advancement — the
// world clock is untouched, and members outside the fight never appear in
// it.
//
// Errors: ErrNilInput, ErrClosed, ErrNoMember, ErrNotMember, ErrNoBubble
// (the member is not in a fight), ErrNotActive when it is not this member's
// turn (with no state change — translated from play/clock's own sentinel,
// rpg-toolkit#1169, the same refusal Step now shares), and whatever
// driveMonsterTurns can return (ErrNoPlayerInBubble, ErrBadTurnOutcome, or a
// TurnDriver's own error) if an unplayed member follows. A driver error here
// means NOTHING this call did is persisted — not even the acting member's
// own end — since nothing is saved until the caller's commit; see
// driveMonsterTurns's own doc.
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
		// play/clock's own sentinel is translated here, not passed through —
		// the same discipline every other leaf-module error crossing this
		// boundary already keeps, and until now the one refusal that had not
		// (rpg-toolkit#1169): a host could only ever have matched on
		// clock.ErrNotActive, two layers down.
		if errors.Is(err, clock.ErrNotActive) {
			return nil, fmt.Errorf("end turn %q: %w", in.Member, ErrNotActive)
		}
		return nil, fmt.Errorf("end turn %q: %w", in.Member, err)
	}

	seq, err := e.appendClockBeat(map[string]interface{}{
		"beat":   "turn-ended",
		"member": string(in.Member),
		"next":   out.Next,
	})
	if err != nil {
		return nil, fmt.Errorf("end turn append beat: %w", err)
	}

	next := MemberID(out.Next)
	wrapped := out.RoundWrapped
	lastSeq := seq

	// If the clock landed on a member with no player, drive them (and any
	// consecutive unplayed members after them) forward before this verb
	// returns — rpg-toolkit#1162. The caller learns the truth about who is
	// actually waiting on THEM in one round trip; the intervening passes ride
	// the story and the event stream as their own turn-ended beats.
	if m, ok := e.members[next]; ok && m.Kind != KindPlayer {
		moreWrapped, moreSeq, derr := e.driveMonsterTurns(bubble)
		if derr != nil {
			return nil, fmt.Errorf("end turn %q: %w", in.Member, derr)
		}
		wrapped = wrapped || moreWrapped
		if moreSeq != 0 {
			lastSeq = moreSeq
		}

		// The drive just run can have ended the fight this bubble was
		// holding — a driven member's own killing blow, exactly as
		// driveOneMonsterTurn's own doc describes. bubble.Active() refuses
		// an idle clock with ErrIdle, so Order() is asked first: it never
		// does (an idle clock's Order is an empty slice, not an error), and
		// an empty one means there is no bubble left to ask whose turn it
		// is.
		remaining, oerr := bubble.Order()
		if oerr != nil {
			return nil, fmt.Errorf("end turn %q: %w", in.Member, oerr)
		}
		if len(remaining) == 0 {
			// There is no "next" turn in a bubble that no longer exists.
			// in.Member — who called this verb — is the honest answer: they
			// are the one free again, exactly as a non-driven defeat already
			// leaves the survivor (rpg-toolkit#1078). A caller learns the
			// fight actually ended the way it always has: the
			// "bubble-dissolved" beat this same call's own fan-out already
			// carries, not a new field on this output.
			next = in.Member
		} else {
			active, aerr := bubble.Active()
			if aerr != nil {
				return nil, fmt.Errorf("end turn %q: %w", in.Member, aerr)
			}
			next = MemberID(active)
		}
	}

	return &EndTurnOutput{
		Next:         next,
		RoundWrapped: wrapped,
		Seq:          lastSeq,
	}, nil
}
