// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// worldtime.go is TIME ON THE WORLD CLOCK (rpg-project#465, design §5).
//
// # The clock moves because the party acts
//
// The world clock existed and NOTHING A PLAYER DID ADVANCED IT: Move, Search,
// Unlock, Intimidate and Persuade only stamped its reading, and it moved on a
// fight round wrapping and inside the unwired `Pump`. So a deed landed in
// round 1 was exactly as fresh ten verbs later, and a creature waiting out a
// span never got to.
//
// Now every verb the turn clock would price as an action pays ONE ROUND on
// the world clock, and a walk pays one round per PACE — every
// `CellsFromFeet(SpeedFeet)` cells the mover walks. Standing still is free,
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
// # When time passes, the world thinks
//
// Inside the same call that raised the high-water, after the verb's own beats
// and before its sight refresh, every standing monster on the world clock
// with budget is given one turn's worth of doing, rolls its `time` table, and
// spends one. Then the verb's one sight refresh runs, and a creature that
// walked into an enemy's sight joins a fight by the shipped path. A fight
// round wrapping is such a raise, so creatures OUTSIDE a fight close one
// round at a time while it runs — the primitive Alarm needs.

// advanceWorld advances the world clock for one member and reports whether
// that raised the high-water.
//
// THE RAISE IS THE ANSWER, not the milestone. clock.Advance always reports a
// Ticked milestone and a Ready snapshot, whether or not anything was granted:
// a driver behind the front runner catches up silently, and its catching up is
// not time passing for anybody else. Comparing the high-water before and after
// is the only question this composition has — "did anybody gain a round" — and
// it is what decides whether the world thinks.
func (e *Encounter) advanceWorld(driver MemberID, displacement int) (bool, error) {
	before := e.clock.ToData().HighWater
	if _, err := e.clock.Advance(&clock.AdvanceInput{
		Driver:       core.EntityID(driver),
		Displacement: displacement,
	}); err != nil {
		return false, fmt.Errorf("advance %q: %w", driver, err)
	}

	return e.clock.ToData().HighWater > before, nil
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
// round ([Encounter.noticeRounds]), and charging a turn-clock Search a second
// round would make an action inside a fight cost twice what the same action
// costs outside one.
func (e *Encounter) spendWorldAction(actor MemberID) error {
	bubble, err := e.bubbleFor(actor)
	if err != nil {
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

	return e.worldThinks()
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
// A MEMBER WITH NO SPEED PACES NOTHING rather than paying a round per cell.
// Zero speed is a roster row that carried no number, and dividing by it would
// make the slowest thing in the world the fastest clock in it.
func (e *Encounter) spendWorldPace(mover MemberID) error {
	m, ok := e.members[mover]
	if !ok {
		return nil
	}
	perRound := CellsFromFeet(m.SpeedFeet)
	if perRound <= 0 {
		return nil
	}

	bubble, err := e.bubbleFor(mover)
	if err != nil {
		return fmt.Errorf("world pace: %w", err)
	}
	if bubble != nil {
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

	return e.worldThinks()
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

// worldThinks gives every standing monster on the world clock its budget:
// one turn's worth of doing per unit, rolled off its own `time` table.
//
// CALLED BY EVERY RAISE SITE, RIGHT AFTER THE ADVANCE THAT RAISED THE
// HIGH-WATER. An advance that granted nothing thinks nothing — a driver
// catching up to the front runner is not time passing.
//
// NO ATTACKS, AND NO REACTIONS. The budget is `AttacksLeft: 0` and an
// [Attack] returned here is [ErrAttackOffTurn] rather than a silently skipped
// intent: an enemy in reach on the world clock is a fight sight that has
// already formed, so a table that reaches this is describing a world that
// cannot happen. Steps are world steps — nobody is in a fight, so there is no
// threatened square to leave and no reaction to spend, which is the rule the
// retired Pump kept and the comment it kept it with.
//
// ONE tick BEAT PER RAISE, the frame, stamped with the new reading; the moves
// inside it narrate themselves as they always did.
//
// AND ONE SIGHT REFRESH AT THE END, over the whole roster — the retired Pump's
// own shape, and a correction to this slice's brief, which said the calling
// verb's refresh would do. It cannot: the verbs that pay a round are Search,
// Loot, Interact, Cast and Activation as well as Step and Unlock, and the
// first five deliberately refresh no sight because "nothing moved and no
// geometry changed" (see [Encounter.Search]). That stopped being true the
// moment a round of the world could move somebody inside them. A creature
// that walks into an opposed member's sight during a Search therefore forms
// or joins a fight here, by the shipped path, and leaves the world clock with
// its budget — rather than standing invisibly in the doorway until somebody
// takes a step.
//
// ONCE FOR THE PASS, not once per creature the world thought for: a fight
// formed halfway through the pass would be a fight formed before the rest of
// the creatures had moved.
//
// RE-ENTRANT CALLS ARE A NO-OP. A step inside a world round cannot raise the
// clock — pace is accrued by [Encounter.Step] alone — but a beat it appends
// can reach a consult that reaches a verb, and the outer call owns this pass.
func (e *Encounter) worldThinks() error {
	if e.worldThinking || e.outcome != nil {
		return nil
	}
	e.worldThinking = true
	defer func() { e.worldThinking = false }()

	at := uint64(e.clock.ToData().HighWater)
	if err := e.appendTickBeat(at); err != nil {
		return err
	}

	thinkers, err := e.worldThinkers()
	if err != nil {
		return err
	}
	// A WORLD WITH NOTHING IN IT THAT CAN ACT ASKS NOBODY, which is what
	// [Encounter.worldThinkers] answers: no standing consult and no driver
	// consult for a round in which no creature carries orders. The refresh
	// below still runs — time passing is a moment the world notices things,
	// which is what the retired Pump's own end-of-tick refresh was.
	if len(thinkers) > 0 {
		// A body has no round to spend, so its driver is not consulted at all
		// rather than consulted and discarded — the retired Pump's second
		// census defect, which had dead monsters patrolling.
		down, derr := e.downNow()
		if derr != nil {
			return fmt.Errorf("world thinks standing: %w", derr)
		}

		for _, member := range thinkers {
			m, ok := e.members[member]
			if !ok || down[member] {
				continue
			}
			if terr := e.thinkFor(member, m); terr != nil {
				return terr
			}
			if e.outcome != nil {
				return nil
			}
		}
	}

	if _, _, err := e.refreshSight(e.rosterIDs()); err != nil {
		return fmt.Errorf("world thinks refresh sight: %w", err)
	}

	return nil
}

// worldThinkers is every creature the world has something to ask this round,
// in the clock's own stable order: a monster, on the world clock, with budget
// to spend and a table to spend it on.
//
// A TABLE IS PART OF THE TEST, not an afterthought inside the loop. A creature
// nobody wrote orders for holds, and holding is indistinguishable from not
// being asked — so asking would cost the whole round a standing consult and a
// sight sweep to produce a world nobody changed.
func (e *Encounter) worldThinkers() ([]MemberID, error) {
	ids, err := e.clock.Members()
	if err != nil {
		return nil, fmt.Errorf("world thinks: %w", err)
	}

	var out []MemberID
	for _, id := range ids {
		member := MemberID(id)
		m, ok := e.members[member]
		if !ok || m.Kind != KindMonster || len(m.Table) == 0 {
			continue
		}
		// A monster caught in a bubble is not the world's to think for: the
		// world thinks on the tick, and a fight thinks in turns. Form already
		// took it off this clock, so this is the belt on the braces — and the
		// claim a test pins.
		bubble, berr := e.bubbleFor(member)
		if berr != nil {
			return nil, fmt.Errorf("world thinks: %w", berr)
		}
		if bubble != nil {
			continue
		}
		budget, berr := e.clock.Budget(&clock.BudgetInput{ID: id})
		if berr != nil {
			return nil, fmt.Errorf("world thinks budget %q: %w", member, berr)
		}
		if budget <= 0 {
			continue
		}
		out = append(out, member)
	}

	return out, nil
}

// thinkFor spends one member's whole world-clock budget, a turn's worth of
// doing at a time.
//
// THE BUDGET IS READ ONCE AND SPENT DOWN. A member two rounds behind gets two
// turns' worth in this call, which is what makes a creature that was in a
// fight while the party walked catch up the moment the fight lets it go.
func (e *Encounter) thinkFor(member MemberID, m *memberRecord) error {
	budget, err := e.clock.Budget(&clock.BudgetInput{ID: core.EntityID(member)})
	if err != nil {
		return fmt.Errorf("world thinks budget %q: %w", member, err)
	}

	for spent := 0; spent < budget; spent++ {
		turnBudget := TurnBudget{AttacksLeft: 0, MovementFeet: m.SpeedFeet}
		// The same anti-spin bound a fight's own turn uses: one terminating
		// Pass plus however many cells this member could ever ask for one at
		// a time. There is no attack to allow for here.
		bound := 2 + CellsFromFeet(turnBudget.MovementFeet)

		if _, err := e.runIntents(nil, core.EntityID(member), m, 0, &turnBudget, 0, bound, ClockWorld); err != nil {
			return fmt.Errorf("world thinks %q: %w", member, err)
		}

		if _, err := e.clock.Spend(&clock.SpendInput{ID: core.EntityID(member), Amount: 1}); err != nil {
			return fmt.Errorf("world thinks spend %q: %w", member, err)
		}
		if e.outcome != nil {
			return nil
		}
	}

	return nil
}

// appendTickBeat writes the frame one raise of the world clock happened in —
// the beat the retired Pump wrote, kept verbatim in shape so a client that
// renders a tick does not have to learn a second name for the same event.
func (e *Encounter) appendTickBeat(at uint64) error {
	payload, err := json.Marshal(map[string]interface{}{
		"beat": "tick",
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

	dealt, roll, of, err := dealTemper(context.Background(), temper, e.roller)
	if err != nil {
		return Temper{}, fmt.Errorf("member %q: %w", member, err)
	}
	if err := e.appendTemperedBeat(member, dealt.Word, roll, of, at); err != nil {
		return Temper{}, fmt.Errorf("member %q: %w", member, err)
	}

	return dealt, nil
}
