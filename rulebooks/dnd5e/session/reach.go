// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// rosterPositions indexes a roster read by member id, for the range checks
// below and for Afford's per-target declarations — both need "where does
// this member stand" and both already hold the roster read that answers it,
// so this is a lookup rather than a second fetch.
func rosterPositions(roster []encounter.Member) map[string]spatial.Position {
	out := make(map[string]spatial.Position, len(roster))
	for _, m := range roster {
		out[string(m.ID)] = m.Position
	}
	return out
}

// sheetSpeeds asks the members' own sheets for the one fact a directed move
// needs and the content cannot supply: how far this creature's own legs carry
// it, in feet.
//
// THE SAME ANSWER THE COMPOSITION GETS. A push budgeted by the mover's speed
// asks the [sheetSeam] the encounter's own pace and turn budget ask, at the
// moment the push runs — never a copy taken at Join, which is the record
// rpg-project#538 retired. A member the verb holds no sheet for is refused by
// the seam, not answered with zero; a sheet that answers zero is a true speed
// of zero, and the caller turns that into a refusal with a sentence on it
// (routeCastPushes, noSpeedToRunWith).
//
// FULL SPEED, EVEN FOR A SNEAKER. A directed move is imposed on the mover,
// not walked by it, and sneaking halves only the sneaker's own walking (lead
// ruling on rpg-toolkit#2079), so the seam is asked [sheetSeam.unhalved].
func sheetSpeeds(sheets sheetSeam, members []encounter.MemberID) (map[string]int, error) {
	if len(members) == 0 {
		return map[string]int{}, nil
	}
	facts, err := sheets.unhalved().Sheets(members)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int, len(facts))
	for id, f := range facts {
		out[string(id)] = f.SpeedFeet
	}
	return out, nil
}

// inRange reports whether to is within the declared maximum range in feet of
// from, on enc's own grid (Encounter.Distance — the same primitive
// refreshSight's sight check uses internally, exposed minimally:
// rpg-toolkit#1010). Range is converted to cells once, here, via
// encounter.CellsFromFeet; a cell is five feet.
func inRange(enc *encounter.Encounter, from, to spatial.Position, rangeFeet int) bool {
	return enc.Distance(from, to) <= float64(encounter.CellsFromFeet(rangeFeet))
}

// approachCandidates names, on each out-of-reach Attack candidate, the cell
// the actor's own walk this turn ends on within rangeFeet and in sight of it
// (rpg-project#872 D4) — [TargetCandidate.Approach]. The encounter answers
// it ([encounter.Encounter.Approaches]): where a walk ends is a placement
// question, and this seam only says how far the walk may go.
//
// THE BUDGET IS THE MOVE'S OWN. On the turn clock it is the movement the
// readied sheet has left — the capacity a Move is priced against, five feet
// a cell — and only for the active member, whose Move alone is accepted. On
// the world clock a walk spends nothing, so it is unbounded.
//
// ONE READ PER AFFORD, and none when every candidate is in reach or the
// candidates are refused for anything but reach.
func approachCandidates(
	enc *encounter.Encounter, clock *encounter.ClockOfOutput, member string, sheet *character.Character,
	candidates []targetPreflight, rangeFeet int,
) ([]targetPreflight, error) {
	var targets []encounter.MemberID
	rows := map[encounter.MemberID]int{}
	for i, c := range candidates {
		if c.available || c.why == nil || c.why.Reason != ShortfallTargetOutOfReach {
			continue
		}
		rows[encounter.MemberID(c.member)] = i
		targets = append(targets, encounter.MemberID(c.member))
	}
	if len(targets) == 0 {
		return candidates, nil
	}
	in := encounter.ApproachesInput{Mover: encounter.MemberID(member), Targets: targets, RangeFeet: rangeFeet}
	if ClockKind(clock.Kind) == ClockTurn {
		if string(clock.Active) != member {
			return candidates, nil
		}
		in.Budget = max(0, encounter.CellsFromFeet(sheet.CapacityLeft(combat.CapacityMovement)))
	} else {
		in.Unbounded = true
	}
	out, err := enc.Approaches(in)
	if err != nil {
		return nil, fmt.Errorf("attack approaches: %w", translate(err))
	}
	for _, a := range out.Approaches {
		if !a.Reached || len(a.Path) == 0 {
			continue
		}
		cell := a.Cell
		candidates[rows[a.Target]].approach = &cell
	}
	return candidates, nil
}
