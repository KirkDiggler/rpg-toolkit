// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
)

// A creature that joins a fight under way rolls initiative, the same roll
// everyone makes, and takes its place by it (rpg-project#1068, ruled by the
// operator 2026-11-21):
//
//   - In the fight's FIRST round it gets a turn that round, placed by its roll
//     among those who have not acted yet. A roll that would put it earlier than
//     where the round has got to lands straight after the one acting now.
//   - In any LATER round it takes its rolled place and has no turn in the round
//     it joins (the turn clock seats it "from the next round").
//   - It is given one place and keeps it; equal totals are ordered by member id,
//     as everywhere else.
//
// The fight keeps each member's total (Encounter.initiativeTotals), because a
// total is a rule's number and the clock holds no rules.

// joinPlace is the rule: where a joiner goes in the bubble's order, and
// whether it waits for the next round.
//
// order, activeIdx and round are the bubble's; totals are the totals the fight
// kept. The joiner goes in front of the first member, from the slot after the
// one acting on in round one and from the start in a later round, that its
// roll outranks (a higher total, or an equal one with the smaller id). A member
// with no total was placed without a roll and is never outranked. With no roll
// the joiner goes last. Pure.
func joinPlace(
	order []core.EntityID, totals map[MemberID]int, activeIdx, round int, roll InitiativeRoll,
) (pos int, nextRound bool) {
	lo := 0
	if round == 1 {
		lo = activeIdx + 1
	}
	pos = len(order)
	if roll.Calculation != nil {
		for i := lo; i < len(order); i++ {
			other := MemberID(order[i])
			t, ok := totals[other]
			if !ok {
				continue
			}
			if roll.Total > t || (roll.Total == t && roll.Member < other) {
				pos = i
				break
			}
		}
	}

	return pos, round > 1
}

// keepInitiativeTotal remembers one member's initiative total in a bubble.
func (e *Encounter) keepInitiativeTotal(bubble *clock.Turn, member MemberID, total int) {
	if e.initiativeTotals == nil {
		e.initiativeTotals = make(map[*clock.Turn]map[MemberID]int)
	}
	if e.initiativeTotals[bubble] == nil {
		e.initiativeTotals[bubble] = make(map[MemberID]int)
	}
	e.initiativeTotals[bubble][member] = total
}

// joinFight brings members into the running fight, each by its own initiative
// roll. It asks the initiative seam once for all of them, refuses an answer
// that is not exactly the members asked about, each once (ErrNotMember for
// somebody not asked, ErrInvalidData for one missing or doubled), then places
// them one at a time in the order the roller answered, each against the order
// as it stands after the previous one. It returns the last entry's seq.
func (e *Encounter) joinFight(members []MemberID) (uint64, map[MemberID]*IntelDelta, error) {
	if e.initiative == nil {
		return 0, nil, fmt.Errorf("join fight: no initiative roller: %w", ErrInvalidData)
	}
	if len(e.bubbles) == 0 {
		return 0, nil, fmt.Errorf("join fight: no fight is running: %w", ErrNoBubble)
	}
	rolls, err := e.initiative.RollInitiative(append([]MemberID(nil), members...))
	if err != nil {
		return 0, nil, fmt.Errorf("join fight roll initiative: %w", err)
	}
	asked := make(map[MemberID]bool, len(members))
	for _, id := range members {
		asked[id] = true
	}
	answered := make(map[MemberID]bool, len(rolls))
	for _, roll := range rolls {
		if !asked[roll.Member] {
			return 0, nil, fmt.Errorf("join fight: initiative answered %q, who was not asked: %w", roll.Member, ErrNotMember)
		}
		if answered[roll.Member] {
			return 0, nil, fmt.Errorf("join fight: initiative answered %q twice: %w", roll.Member, ErrInvalidData)
		}
		answered[roll.Member] = true
		// The whole answer is checked before anybody moves, so a bad roll
		// never leaves half of a verb's joiners in the fight.
		if roll.Calculation != nil {
			if verr := validateRecordedD20(roll.Calculation, roll.Roll, roll.Total); verr != nil {
				return 0, nil, fmt.Errorf("join fight: %q initiative roll: %w", roll.Member, verr)
			}
		}
	}
	for _, id := range members {
		if !answered[id] {
			return 0, nil, fmt.Errorf("join fight: initiative did not answer %q: %w", id, ErrInvalidData)
		}
	}

	var (
		seq    uint64
		deltas map[MemberID]*IntelDelta
	)
	for _, roll := range rolls {
		bubble := e.bubbles[0]
		data := bubble.ToData()
		pos, nextRound := joinPlace(data.Order, e.initiativeTotals[bubble], data.ActiveIdx, data.Round, roll)
		roll := roll
		out, terr := e.transfer(&TransferInput{
			Member: roll.Member, To: ClockTurn, Pos: pos, Roll: &roll, NextRound: nextRound,
		}, true)
		if terr != nil {
			return 0, nil, fmt.Errorf("join fight %q: %w", roll.Member, terr)
		}
		seq = out.Seq
		deltas = mergeIntelDeltas(deltas, out.IntelDeltas)
	}

	return seq, deltas, nil
}
