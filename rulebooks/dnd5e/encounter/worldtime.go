// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// worldtime.go is TIME ON THE WORLD CLOCK (rpg-project#465, design §5).
//
// # The clock moves because the party acts
//
// The world clock existed and NOTHING A PLAYER DID ADVANCED IT: Move,
// Unlock and the two social verbs (now Speak) only stamped its reading, and it moved on a
// fight round wrapping and inside the unwired `Pump`. So a deed landed in
// round 1 was exactly as fresh ten verbs later, and a creature waiting out a
// span never got to.
//
// Now every verb the turn clock would price as an action pays ONE ROUND on
// the world clock, and a walk pays one round per PACE — every
// `CellsFromFeet(SpeedFeet)` cells the mover walks, at the speed its sheet
// answers through [Sheets] at that step. Standing still is free,
// said plainly: a party that talks to the goblin and waits sees nothing move.
// That is the roguelike clock play/clock chose ("advances only because
// players act") and the encounter's C5 (no goroutines, no timers).
//
// REJECTED: one unit per Move verb regardless of length. The client's path
// length would set the world's speed.
//
// # The driver is always the member, never "world"
//
// The clock accrues by driver as MAX, not sum, so four players walking six
// cells together is one round rather than four. Both shipped advance sites
// used the literal driver `"world"`, which would have made a fight the front
// runner forever: after ten rounds of fighting, a player's first walk would
// raise nothing until they had walked ten paces. Every advance here names its
// member.
//
// # When time passes, the world wakes who is due
//
// Inside the same call that raised the high-water, after the verb's own beats
// and before its sight refresh, every creature the raise made due steps its
// machine (machine.go): on the default recipe it rolls its `time` table,
// executes, and spends one. Then the verb's one sight refresh runs, and a
// creature that walked into an enemy's sight joins a fight by the shipped
// path. A fight round wrapping is such a raise, so creatures OUTSIDE a fight
// close one round at a time while it runs — the primitive Alarm needs.
// (Creature-table §5's "the world thinks" is amended by rpg-project#766 R1:
// the world wakes who is due; the roster is never walked.)

// advanceWorld advances the world clock for one member and reports whether
// that raised the high-water.
//
// THE RAISE IS THE ANSWER, not the milestone. clock.Advance always reports a
// Ticked milestone and a Ready snapshot, whether or not anything was granted:
// a driver behind the front runner catches up silently, and its catching up is
// not time passing for anybody else. Whether the advance passes the
// high-water is the only question this composition has — "did anybody gain a
// round" — and it is what decides whether the world wakes. It is asked of
// [Encounter.worldRaises] before the advance, the same predicate
// [Encounter.PaceDue] asks, so the prediction and the raise are one rule.
func (e *Encounter) advanceWorld(driver MemberID, displacement int) (bool, error) {
	raises := e.worldRaises(driver, displacement)
	if _, err := e.clock.Advance(&clock.AdvanceInput{
		Driver:       core.EntityID(driver),
		Displacement: displacement,
	}); err != nil {
		return false, fmt.Errorf("advance %q: %w", driver, err)
	}

	return raises, nil
}

// worldRaises is whether advancing driver by displacement would raise the
// world clock's high-water: the clock leaf's own rule (clock.Tick.Advance
// grants a round only when the driver's cumulative progress passes the
// high-water), asked of its state without advancing it. A driver behind the
// front runner catches up without raising anything.
//
// [Encounter.advanceWorld] answers with it and [Encounter.PaceDue] predicts
// with it, so the two cannot disagree about which advance raises.
func (e *Encounter) worldRaises(driver MemberID, displacement int) bool {
	data := e.clock.ToData()
	return data.DriverProgress[core.EntityID(driver)]+displacement > data.HighWater
}

// elapseWorld JUMPS the world clock by span rounds: time that passed
// without anybody driving it (rpg-project#542 R5, a short rest's hour).
//
// A JUMP, NOT A DRIVE. [Encounter.advanceWorld] is time somebody lived
// round by round: it grants every world-clock member the rounds as budget,
// and [Encounter.wake] then spends them a turn at a time. An elapse
// grants nothing and nobody wakes — no creature acts during it, no tick
// beat is written — and NOTHING IS OWED AFTERWARDS: every budget is exactly
// what it was, and the due heap jumps with the reading ([Encounter.jumpWake]),
// so the next step anybody takes is not handed the hour.
//
// EVERY DRIVER JUMPS WITH THE READING. The clock accrues by driver as max;
// moving the high-water alone would leave every member an hour behind it,
// and their next walks would raise nothing until they had walked an hour of
// paces. So each roster member's progress (zero for one who never drove)
// and every other recorded driver's moves by span too, every gap between
// them is what it was, and the next walk paces exactly as it would have.
//
// The clock leaf has no elapse verb, and this composition owns the clock: it
// is rebuilt from its own persisted shape through [clock.LoadTick], which
// re-checks every invariant that shape carries.
func (e *Encounter) elapseWorld(span int) error {
	if span <= 0 {
		return fmt.Errorf("elapse %d rounds: %w", span, ErrInvalidData)
	}
	data := e.clock.ToData()
	progress := make(map[core.EntityID]int, len(data.DriverProgress)+len(e.members))
	for id, p := range data.DriverProgress {
		progress[id] = p + span
	}
	for _, id := range e.rosterIDs() {
		if _, ok := progress[id]; !ok {
			progress[id] = span
		}
	}
	data.DriverProgress = progress
	data.HighWater += span

	jumped, err := clock.LoadTick(data)
	if err != nil {
		return fmt.Errorf("elapse %d rounds: %w", span, err)
	}
	e.clock = jumped
	return e.jumpWake(uint64(span))
}

// seatOnWorldClock puts a NEWCOMER on the world clock at the world's own
// time: a member who joins, or arrives from reserve, into a running world.
//
// AT THE HIGH-WATER, NOT AT ZERO. The clock accrues by driver as max, and a
// driver it has never heard of counts from zero; a member seated there after
// the world has lived six hundred rounds — an hour of rest, or a long walk —
// would have to walk all of them before a step of theirs moved time for
// anybody. A newcomer arrives NOW, so their progress is the reading, exactly
// where [Encounter.elapseWorld] leaves every member who was present. The
// catch-up raises nothing (it reaches the high-water, never past it), so it
// grants nobody budget and the world does not wake on it. A creature with a
// machine is armed at the world's now: due at the next raise.
//
// A member coming back from a fight is not a newcomer and does not come
// through here: a fight advances its members round by round
// ([Encounter.spendRound]), so their progress is their own.
func (e *Encounter) seatOnWorldClock(id MemberID) error {
	if _, err := e.clock.Join(&clock.JoinInput{ID: core.EntityID(id)}); err != nil {
		return err
	}
	data := e.clock.ToData()
	e.armAt(id, uint64(data.HighWater))
	// And what its first state listens for (rpg-project#766 S4): a
	// newcomer's ears are on from the round it arrives.
	if m, ok := e.members[id]; ok {
		e.relistBehavior(m)
	}
	behind := data.HighWater - data.DriverProgress[core.EntityID(id)]
	if behind <= 0 {
		return nil
	}
	if _, err := e.clock.Advance(&clock.AdvanceInput{Driver: core.EntityID(id), Displacement: behind}); err != nil {
		return err
	}

	return nil
}

// worldDriver is the world clock's own driver on [clock.Tick]: the name a
// round the world itself lives is advanced under, when the member whose deed
// cost it is no longer on the world clock to drive it. Empty, because no
// member can be named empty, so it can never be mistaken for one.
const worldDriver = core.EntityID("")

// deedMovesTheWorld is the price of an ACTION DONE ON THE WORLD CLOCK: a deed
// done on the world clock moves the world a round, whoever it put in a fight
// (rpg-project#785 R1, R3).
//
// A swing or a hostile cast from free roam turns the pair, and the turn forms
// the fight in place, inside the deed, with the struck creature in it. By the
// time the price is charged the actor is a fighter, and a fighter's time is
// priced by the turn clock: [Encounter.spendWorldAction] would charge it
// nothing, and the round the deed took would vanish. So the deed's round is
// the world's, not the actor's.
//
// AN ACTOR STILL ON THE WORLD CLOCK DRIVES ITS OWN ROUND, exactly as
// [Encounter.spendWorldAction] charges it: a harmless cast, a swing at a
// creature whose faction is already hostile and out of sight, a swing that
// turned nobody. Its progress is its own, and its next walk paces from it.
//
// AN ACTOR THE DEED PUT IN A FIGHT does not: the world clock raises itself by
// one round under [worldDriver], and the wake pass runs for the creatures
// still on the world clock. The fighters get nothing — not a wake, not a
// budget they could spend — because the turn clock prices their time now.
// Their own progress is left where it is, so the fight's first round is the
// round the deed took, never a second one on top of it.
//
// The caller decides that the actor was on the world clock when it acted
// ([Encounter.appendTrainAndNotice], [Encounter.RecordCast]): an actor
// already in a fight when it acted is priced by the turn clock and never
// reaches here, even when its own deed ends that fight.
func (e *Encounter) deedMovesTheWorld(actor MemberID) error {
	off, err := e.onTheWorldClock(actor)
	if err != nil {
		return fmt.Errorf("world action: %w", err)
	}
	if off {
		return e.spendWorldAction(actor)
	}

	data := e.clock.ToData()
	raised, err := e.advanceWorld(MemberID(worldDriver), data.HighWater+1-data.DriverProgress[worldDriver])
	if err != nil {
		return fmt.Errorf("world action: %w", err)
	}
	if !raised {
		return nil
	}

	return e.wake()
}

// onTheWorldClock answers whether a member is outside every fight, so an
// action it takes is priced in rounds of the world. Empty asks nobody and
// answers false.
func (e *Encounter) onTheWorldClock(id MemberID) (bool, error) {
	if id == "" {
		return false, nil
	}
	bubble, err := e.bubbleFor(id)
	if err != nil && !errors.Is(err, errNotInBubble) {
		return false, err
	}
	return bubble == nil, nil
}

// spendWorldAction is what a verb the turn clock would price as an ACTION
// costs on the world clock: one round, for the actor, once its outcome has
// landed (design §5).
//
// AFTER THE OUTCOME, BEFORE THE SIGHT REFRESH. The verb's own beats, the fact
// it taught, the answer it rolled and the arrivals it caused are all stamped
// with the reading the verb STARTED at — they are what happened in that round
// — and the world moves on once they are true. A verb that advanced first
// would stamp its own effects a round after their cause.
//
// A MEMBER IN A BUBBLE PAYS NOTHING HERE. A fight prices its own time by the
// round ([Encounter.noticeRounds]), and charging a turn-clock action a second
// round would make an action inside a fight cost twice what the same action
// costs outside one.
func (e *Encounter) spendWorldAction(actor MemberID) error {
	bubble, err := e.bubbleFor(actor)
	if err != nil && !errors.Is(err, errNotInBubble) {
		return fmt.Errorf("world action: %w", err)
	}
	if bubble != nil {
		return nil
	}

	raised, err := e.advanceWorld(actor, 1)
	if err != nil {
		return fmt.Errorf("world action: %w", err)
	}
	if !raised {
		return nil
	}

	return e.wake()
}

// spendWorldPace accrues ONE CELL of walking on a member and pays a round
// every time the accrual reaches the member's own pace (design §5).
//
// THE REMAINDER CARRIES ON THE MEMBER, PERSISTED. A 30-foot mover pays a
// round every sixth cell, and a party that walks three cells, saves, reloads
// and walks three more has walked six — the remainder is part of where
// everybody is in the world's own time, not scratch state of one verb.
//
// TURN-CLOCK STEPS DO NOT TOUCH PACE. Inside a fight the round is what prices
// time, and a member whose walk also accrued pace would be paying for the
// same movement twice.
//
// THE PACE IS THE SHEET'S SPEED OF THIS STEP (rpg-project#538), asked through
// [Sheets] each time and never stored: a member whose speed changed between
// two walks paces the second at the new speed, with no write to the encounter.
//
// A MEMBER WHOSE SHEET ANSWERS A SPEED OF ZERO PACES NOTHING rather than
// paying a round per cell. Zero is that sheet's true speed — a creature that
// cannot walk — not a missing answer (a missing sheet is refused before this
// line), and dividing by it would make the slowest thing in the world the
// fastest clock in it.
func (e *Encounter) spendWorldPace(mover MemberID) error {
	m, ok := e.members[mover]
	if !ok {
		return nil
	}

	perRound, err := e.worldPacePerRound(mover)
	if err != nil {
		return fmt.Errorf("world pace: %w", err)
	}
	if perRound <= 0 {
		return nil
	}

	m.PaceCells++
	if m.PaceCells < perRound {
		return nil
	}
	m.PaceCells -= perRound

	raised, err := e.advanceWorld(mover, 1)
	if err != nil {
		return fmt.Errorf("world pace: %w", err)
	}
	if !raised {
		return nil
	}

	return e.wake()
}

// worldPacePerRound is how many cells of walking pay one world round for a
// member, or zero when their walking pays nothing at all: the ONE statement
// of the pace rule that both [Encounter.spendWorldPace] (which pays it) and
// [Encounter.PaceDue] (which predicts it) read, so the prediction cannot
// drift from the payment.
//
// Zero for a bubble member (the turn clock prices their time, not pace) and
// for a sheet whose speed is zero (a creature that cannot walk paces
// nothing). Otherwise [CellsFromFeet] of the sheet's speed, asked each time
// and never stored. Errors are the bubble lookup's and the sheet's.
func (e *Encounter) worldPacePerRound(mover MemberID) (int, error) {
	bubble, err := e.bubbleFor(mover)
	if err != nil && !errors.Is(err, errNotInBubble) {
		return 0, err
	}
	if bubble != nil {
		return 0, nil
	}

	sheet, err := e.sheetOf(mover)
	if err != nil {
		return 0, err
	}
	perRound := CellsFromFeet(sheet.SpeedFeet)
	if perRound <= 0 {
		return 0, nil
	}
	return perRound, nil
}

// PaceDueInput asks whether a member's next step pays a world round.
type PaceDueInput struct {
	// Member is the walker asked about. MUST be non-empty.
	Member MemberID
}

// PaceDueOutput answers [Encounter.PaceDue].
type PaceDueOutput struct {
	// Due is true when the member's NEXT [Encounter.Step] would raise the
	// world clock, and so wake whoever is due.
	Due bool
}

// PaceDue says whether a member's NEXT step would RAISE THE WORLD: the step
// whose pace accrual reaches their speed in [Encounter.spendWorldPace], AND
// whose round, advanced under their name, passes the clock's high-water in
// [Encounter.advanceWorld]. A member walking behind the front runner pays
// pace without raising anything, and is not due.
//
// WHY IT EXISTS (rpg-project#879 S1). The session answers a Move with the
// player's steps up to the point a pace comes due, and runs the step that
// pays the round, the raise and the rest of the walk afterwards as a
// continuation, so the verb answers before the world runs. It cannot know
// which step raises unless the encounter says so, and the encounter owns
// that rule.
//
// THE SAME RULES, NOT COPIES. Both this and the step read
// [Encounter.worldPacePerRound] and [Encounter.worldRaises]: a bubble member
// is never due (the turn clock prices their time), a zero-speed sheet is
// never due (it paces nothing), and otherwise the step is due when
// PaceCells+1 reaches the cells-per-round of the sheet's speed and one more
// round under the member's name passes the high-water.
//
// WHAT IT DOES NOT CHECK: whether the step itself would be refused. A closed
// encounter, a paused member and the turn gate all refuse [Encounter.Step]
// before any pace is spent, and a step into a body is a bump that spends
// none; the caller learns those from the step. PaceDue answers only "if
// this member's next step lands, does it raise the world".
//
// A READ: nothing is written and no beat is appended.
//
// Errors: ErrNilInput, ErrNoMember for an empty member, ErrNotMember for one
// the encounter does not hold, and the sheet's own error.
func (e *Encounter) PaceDue(in *PaceDueInput) (*PaceDueOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("pace due: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("pace due: %w", ErrNoMember)
	}
	m, ok := e.members[in.Member]
	if !ok {
		return nil, fmt.Errorf("pace due %q: %w", in.Member, ErrNotMember)
	}

	perRound, err := e.worldPacePerRound(in.Member)
	if err != nil {
		return nil, fmt.Errorf("pace due: %w", err)
	}
	if perRound <= 0 {
		return &PaceDueOutput{}, nil
	}
	if m.PaceCells+1 < perRound {
		return &PaceDueOutput{}, nil
	}
	return &PaceDueOutput{Due: e.worldRaises(in.Member, 1)}, nil
}

// spendRound is the fight's own advance: ONE ROUND PER BUBBLE MEMBER, each
// naming itself as the driver, because each of them lived that round (design
// §5).
//
// PER MEMBER, NOT ONCE. The accrual is max-by-driver, so advancing once under
// a single name would put that one member out in front and leave everybody
// else's own first walk raising nothing until they had caught up. Each fighter
// really did spend the round, and the clock is where that is said.
func (e *Encounter) spendRound(members []MemberID) (bool, error) {
	raised := false
	for _, id := range members {
		rose, err := e.advanceWorld(id, 1)
		if err != nil {
			return false, err
		}
		raised = raised || rose
	}

	return raised, nil
}

// wake gives every creature the raise made due its turn: the due heap is
// popped up to the world's new high-water, and each member popped steps its
// machine on the `time` beat (machine.go, rpg-project#766 S3, R1).
//
// CALLED BY EVERY RAISE SITE, RIGHT AFTER THE ADVANCE THAT RAISED THE
// HIGH-WATER. An advance that granted nothing wakes nothing — a driver
// catching up to the front runner is not time passing.
//
// NOBODY IS WALKED. The census this replaces asked every world-clock member
// in turn whether it had a table, a budget and no bubble; the heap holds only
// who is due, so a raise costs the creatures it wakes, never the roster. A
// creature with no table was never armed; a machine whose state has no
// `time` transition is not due; a machine with an authored wake is due at
// it. A patrol standing out a dwell is still due every round (dwell.go).
//
// IN SEAT ORDER, AND CAUGHT UP. Members due at one round pop in member order,
// the clock's own sorted order. A member due behind the high-water (a raise
// that came inside another pass) steps at its own round, re-arms at the next
// and pops again, round after round, until it is due past the high-water: the
// census's "a member two rounds behind gets two turns' worth". The owed
// rounds survive a save: a load arms each member at its first owed round,
// read off the clock's persisted budget ([Encounter.armOwed]).
//
// NO ATTACKS, AND NO REACTIONS. The budget is `AttacksLeft: 0` and an
// [Attack] returned here is [ErrAttackOffTurn] rather than a silently skipped
// intent: an enemy in reach on the world clock is a fight sight that has
// already formed, so a table that reaches this is describing a world that
// cannot happen. Steps are world steps — nobody is in a fight, so there is no
// threatened square to leave and no reaction to spend.
//
// ONE tick BEAT PER RAISE, the frame, stamped with the new reading; the moves
// inside it narrate themselves as they always did.
//
// AND ONE SIGHT REFRESH AT THE END, over the whole roster. The verbs that pay
// a round are Loot, Interact, Cast and Activation as well as Step and Unlock,
// and the first four deliberately refresh no sight because "nothing moved
// and no geometry changed" (see [Encounter.Loot]); a round of the world can
// move somebody inside them, so a creature that walks into an opposed
// member's sight during an action forms or joins a fight here, by the
// shipped path. Once for the pass, not once per creature woken: a fight
// formed halfway through the pass would be a fight formed before the rest of
// the creatures had moved. Bounding it to the observers of the cells crossed
// is rpg-project#762's.
//
// RE-ENTRANT CALLS ARE A NO-OP. A step inside a world round cannot raise the
// clock — pace is accrued by [Encounter.Step] alone — but a beat it appends
// can reach a consult that reaches a verb, and the outer call owns this pass.
// The raise it skipped is not lost: the outer pass re-reads the high-water
// before every pop, and whoever it made due is still on the heap at its round.
func (e *Encounter) wake() error {
	return e.wakePass(true, true)
}

// wakePass is [Encounter.wake] for either kind of raise (rpg-project#766
// S4): a raise of the world clock by somebody walking or acting lands
// `clock: world` (world true); a fight's round lands `clock: fight` for each
// faction engaged before it calls here ([Encounter.noticeRounds]) and lands
// no `world`, because a fight is a clock of its own (R5) and a creature
// waiting out a fight hears its rounds, not the world's. A fight round that
// raised nothing (raised false) asks nobody the time and writes no tick; it
// still matches what landed, so the reserves hear the round.
//
// THE BEATS FIRST, THEN THE TIME. What landed since the last raise is what
// the verb did — the clang, the deed — and it happened before the round
// passed, so the listeners hear it before they are asked what they do with
// the time: a guard posted at the gate is alert by the round the clang
// raised, in that raise. A pick's own beats are matched after each pick, in
// the same pass, up to [wakeBeatBound].
func (e *Encounter) wakePass(raised, world bool) error {
	if e.worldThinking || e.outcome != nil {
		return nil
	}
	e.worldThinking = true
	e.wakeMatched = 0
	e.reactionPicked = false
	e.blockingTurned = false
	e.wakeParticipation = nil
	defer func() { e.worldThinking, e.wakeParticipation = false, nil }()

	at := uint64(e.clock.ToData().HighWater)
	if raised {
		if err := e.appendTickBeat(at); err != nil {
			return err
		}
	}
	if world {
		e.landClock(worldClockName, "", at)
	}
	if err := e.matchLanded(); err != nil {
		return err
	}
	if e.outcome != nil {
		return nil
	}

	// A body has no round to spend, so its machine is not stepped at all —
	// the retired Pump's second census defect, which had dead monsters
	// patrolling. Read once for the pass, and only when somebody is due: a
	// round in which no creature woke asks no standing consult.
	var down map[MemberID]bool
	for raised {
		// THE HIGH-WATER IS RE-READ BEFORE EVERY POP. A creature's pick can
		// raise the world inside this pass (its alarm forms a fight whose
		// round wraps, flip.go's noticeRounds, whose own wake is the no-op
		// above); the creatures that round made due take it in this pass, as
		// the census's budget, re-read at each creature's turn, gave it them.
		at = uint64(e.clock.ToData().HighWater)
		woke := e.due.Pop(at)
		if len(woke) == 0 {
			break
		}
		if down == nil {
			var derr error
			if down, derr = e.downNow(); derr != nil {
				return fmt.Errorf("world wakes standing: %w", derr)
			}
		}
		for _, member := range woke {
			round := e.dueAt[member]
			delete(e.dueAt, member)
			m, ok := e.members[member]
			if !ok || !onDefaultClock(m) {
				continue
			}
			// WHO IS ON THE WORLD CLOCK NOW, NOT WHEN THE PASS BEGAN. A
			// creature woken earlier in this pass can move the clocks under
			// the rest: an alarm turns a pair hostile and the fight that
			// forms takes its members onto a turn clock (rpg-project#569).
			// The fight took this one off the heap; the dissolve or the
			// transfer that returns it re-arms it.
			on, cerr := e.onWorldClock(member)
			if cerr != nil {
				return cerr
			}
			if !on {
				continue
			}
			// A body, or a member with no round granted to spend (the
			// census's budget filter: a spent creature is not asked again),
			// waits for the round after the high-water without being asked.
			budget, berr := e.clock.Budget(&clock.BudgetInput{ID: core.EntityID(member)})
			if berr != nil {
				return fmt.Errorf("world wakes budget %q: %w", member, berr)
			}
			if down[member] || budget <= 0 {
				e.armAt(member, at)
				continue
			}
			if serr := e.stepMachine(member, m, round); serr != nil {
				return serr
			}
			if e.outcome != nil {
				return nil
			}
			// WHAT THE PICK LANDED IS HEARD IN THIS PASS: a patrol's clang
			// wakes the barracks in the round it rang.
			if len(e.landed) > 0 {
				if merr := e.matchLanded(); merr != nil {
					return merr
				}
				if e.outcome != nil {
					return nil
				}
				// A reaction can put somebody down for the rest of the pass;
				// re-read who stands.
				var derr error
				if down, derr = e.downNow(); derr != nil {
					return fmt.Errorf("world wakes standing: %w", derr)
				}
			}
		}
	}

	// A ROUND THAT RAISED NOTHING REFRESHES SIGHT ONLY IF A REACTION MOVED
	// SOMEBODY: a fight round whose listeners only changed state, or did
	// nothing, changed no geometry and nobody's view (review of #2219).
	if !raised && !e.reactionPicked && !e.blockingTurned {
		return nil
	}
	// AND THE REFRESH COSTS ITS REACH (rpg-project#762): the members within
	// sight of a cell changed since the last look — a step's two ends, a door
	// a walk opened, a creature that turned where it stands
	// ([Encounter.pendingCells]) — and nobody else. A
	// guard across the keep saw nothing move. The refresh still RUNS when no
	// cell changed, with nobody to rebuild: it is the round's notice pass
	// too, and a member the rulebook now calls down is noticed by it.
	if _, _, err := e.refreshCostingReach(nil); err != nil {
		return fmt.Errorf("world wakes refresh sight: %w", err)
	}

	return nil
}

// appendTickBeat writes the frame one raise of the world clock happened in —
// the beat the retired Pump wrote, kept verbatim in shape so a client that
// renders a tick does not have to learn a second name for the same event.
func (e *Encounter) appendTickBeat(at uint64) error {
	payload, err := json.Marshal(map[string]interface{}{
		"beat": BeatTick,
		"tick": at,
	})
	if err != nil {
		return fmt.Errorf("tick beat: %w", err)
	}
	if _, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: e.audienceFor(tableBeat),
		Tags:     map[string]string{"tag": "clock"},
		Payload:  payload,
	}); err != nil {
		return fmt.Errorf("tick beat: %w", err)
	}

	return nil
}

// validateTemper refuses, before anything is written, a temperament
// [Encounter.dealTemperFor] could not deal: a share below 1, which can never
// come up; a word in the mix with no profile saying what it means; or a mix
// with no die in this world to deal it with. An authored word deals nothing
// and passes.
//
// A COPY OF mind/behavior's DEAL RULES (dealTemper in mind/behavior's
// table.go): that module refuses the same three things, but only while it
// deals, and it exports no way to ask without dealing. Until it exports a
// validator this copy must change whenever those rules do; the follow-up is
// for behavior to export one and for this function to call it.
func (e *Encounter) validateTemper(member MemberID, temper Temper) error {
	if len(temper.Mix) == 0 {
		return nil
	}
	for word, share := range temper.Mix {
		if share < 1 {
			return fmt.Errorf("member %q: temper %q has a share of %d, which can never be dealt: %w", member, word, share, ErrBadTemper)
		}
		if _, ok := temper.Profiles[word]; !ok {
			return fmt.Errorf("member %q: temper %q is in the mix and nothing says what it means: %w", member, word, ErrBadTemper)
		}
	}
	if e.roller == nil {
		return fmt.Errorf("member %q: a temperament mix to deal: %w", member, ErrNoRoller)
	}

	return nil
}

// dealTemperFor resolves one member's temperament at the door it came in
// through: an authored word passes straight through, a faction's MIX is dealt
// once, through the world's dice, with the faction as the die's entity, and
// the result is a beat so the streamer sees which goblin came out the coward
// (design §3).
//
// AT THE DOOR, ONCE. "Four goblins, one table, four behaviours" only works if
// the deal happens when the goblin enters the run rather than on every roll it
// makes — a temperament re-dealt per pick would be noise, not a personality.
// It is why what persists is the dealt word and not the mix.
//
// AN AUTHORED WORD DEALS NOTHING AND APPENDS NOTHING. The author has already
// answered the question the mix exists to ask, and a beat saying a die chose
// this creature's nerve would be this composition inventing a roll.
func (e *Encounter) dealTemperFor(member MemberID, temper Temper, at uint64) (Temper, error) {
	if len(temper.Mix) == 0 {
		return Temper{Word: temper.Word, Profile: temper.Profile}, nil
	}

	// THE FACTION IS THE DIE'S ENTITY (rpg-project#463, design §3): the spread
	// is the FACTION's — "the instructions given to the group" — and the deal
	// is one roll out of it, so the entity whose rule threw the die is the
	// side and not the creature that came out of it. Every other pick this
	// composition makes names the creature; this one does not, and a tray that
	// draws each die in its owner's set cannot work that out from the member.
	//
	// RESOLVED HERE, ONCE, and as the side the creature is actually on: a
	// monster that named none is in the reserved `monsters` one.
	faction := FactionID("")
	if record, ok := e.members[member]; ok {
		faction = factionOf(record)
	}

	dealt, err := behavior.Deal(context.Background(), &behavior.DealInput{Temper: temper, Die: e.roller})
	if err != nil {
		return Temper{}, fmt.Errorf("member %q: %w", member, err)
	}
	if err := e.appendTemperedBeat(member, faction, dealt.Temper.Word, dealt.Roll, dealt.Of, at); err != nil {
		return Temper{}, fmt.Errorf("member %q: %w", member, err)
	}

	return dealt.Temper, nil
}

// onWorldClock reports whether a member is on the world clock right now —
// asked of the clock itself, never of a list taken earlier, because a wake
// pass moves members between clocks as it runs.
func (e *Encounter) onWorldClock(member MemberID) (bool, error) {
	on, err := e.clock.Contains(&clock.ContainsInput{ID: core.EntityID(member)})
	if err != nil {
		return false, fmt.Errorf("world wakes %q: %w", member, err)
	}

	return on, nil
}
