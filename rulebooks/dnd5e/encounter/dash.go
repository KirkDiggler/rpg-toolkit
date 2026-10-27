// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"slices"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// dash.go is WHAT A MOVE BUYS (rpg-project#671, R17, R18): the facts the
// table's `dash` word is judged by, computed by the host because only the host
// holds a speed, a reach and a wall.
//
// TWO QUESTIONS, BOTH IN THE NPC'S TERMS, NEITHER A NUMBER (Kirk, 2026-10-26:
// "the actual distance doesn't matter; what it means to the NPC does").
//
//   - `enemy: walk | dash | far`: against the foe `enemy` names, does a walk at
//     this turn's movement left end in reach, does only a dash (that movement
//     twice), or not even a dash ([moveBuys]).
//   - `threat: none | walk | dash`: at the cell a row's move would LAND on, who
//     ELSE could walk into reach of it and swing on their next turn, or only
//     dash into it, or nobody ([threatJudge]).
//
// The run itself — the doubled movement and the swing it costs — is charged
// where every intent is judged ([Encounter.chargeDash]).

// moveBuys fills [Facts.EnemyWalk], [Facts.EnemyDash] and [Facts.EnemyFar]:
// what this turn's movement buys against the foe `enemy` names — the nearest
// opposed member in sight and standing, else the nearest remembered
// ([nearestOpposed], the selector's own reading, so the condition and the
// row's target are one member).
//
// THE COMPUTATION. Let `need` be the cells this monster must walk before that
// foe's believed position is inside the reach of one of its own actions
// ([cellsToReach]), and `cells` the turn's movement left
// ([TurnBudget.MovementFeet] in cells). Then:
//
//   - walk: need <= cells — it can walk there and still swing;
//   - dash: cells < need <= 2*cells — only a run gets there this turn;
//   - far:  need > 2*cells, or no route ends in reach at all.
//
// With speed 30 feet (6 cells) and a 5-foot reach (1 cell), a foe at a gap of
// g cells needs g-1 cells walked: g of 2..7 is walk, 8..13 dash, 14 and on
// far. A gap of 1 is reach, where none of the three holds.
//
// EXCLUSIVE AND CLEARED BY REACH, behavior's contract: while
// [Facts.EnemyInReach] holds none is set, and with no opposed member held
// placed none is set; otherwise exactly one is.
func moveBuys(facts *Facts, view MonsterView) {
	if facts.EnemyInReach {
		return
	}
	target, ok := nearestOpposed(view)
	if !ok {
		return
	}
	need, reachable := cellsToReach(view, target)
	cells := CellsFromFeet(view.Budget.MovementFeet)
	switch {
	case reachable && need <= cells:
		facts.EnemyWalk = true
	case reachable && need <= 2*cells:
		facts.EnemyDash = true
	default:
		facts.EnemyFar = true
	}
}

// cellsToReach is how many cells this monster must walk before target's
// believed position is inside the reach of one of its own actions, and
// whether any route gets it there.
//
//   - In sight: zero when [SeenMember.InReach] already holds for an action;
//     otherwise the seen route's length, which ends on the nearest cell from
//     which the target is in reach of the longest-reaching action — the same
//     cell `toward` walks to. No route, no reach.
//   - Remembered: the route walks onto the remembered cell, so the walk that
//     brings it into reach is the route less its [RememberedMember.ReachTail].
//     No route is no reach, unless the monster stands on the memory itself.
func cellsToReach(view MonsterView, target MemberID) (int, bool) {
	for _, s := range view.Seen {
		if s.ID != target {
			continue
		}
		if alreadyInReach(view, target) {
			return 0, true
		}

		return len(s.Path), len(s.Path) > 0
	}
	for _, r := range view.Remembered {
		if r.ID != target {
			continue
		}
		if len(r.Path) == 0 {
			return 0, r.DistanceCells == 0
		}

		return max(len(r.Path)-r.ReachTail, 0), true
	}

	return 0, false
}

// threatJudge is [Facts.Threat] for this view: who ELSE could reach the cell a
// row's move lands on, and swing, on their next turn.
//
// THE COMPUTATION, per row:
//
//  1. The row's LANDING cell and its own target ([landingOf]): where the
//     row's walk, cut to its pace, ends this turn, or where the monster stands
//     for a row that does not move.
//  2. Over every [ThreatZone] on the view — each opposed member held placed,
//     at its believed position — BUT the row's own target (running into reach
//     of somebody means they can hit back, and that is the point; `threat`
//     asks who else can):
//     - walk, when any of them could walk into reach of the landing cell;
//     - dash, when none could by walking and one could by dashing;
//     - none, otherwise — a view with no zones included.
//
// FAIL CLOSED WHERE THE LANDING IS NOT THIS VIEW'S TO KNOW. A walk the
// encounter routes itself at execution — to an authored cell, to a sound,
// away from somebody, a search, a patrol walking back to its route — has no
// path on the view, and the empty answer is behavior's "I did not judge it":
// a `threat` row of those words is never a candidate, `none` included.
func threatJudge(view MonsterView) func(Answer) behavior.ThreatWord {
	return func(row Answer) behavior.ThreatWord {
		landing, target, known := landingOf(view, row)
		if !known {
			return ""
		}
		word := behavior.ThreatNone
		for _, zone := range view.Threats {
			if zone.Member == target {
				continue
			}
			if slices.Contains(zone.Walk, landing) {
				return behavior.ThreatWalk
			}
			if slices.Contains(zone.Dash, landing) {
				word = behavior.ThreatDash
			}
		}

		return word
	}
}

// landingOf is where a row would leave this monster at the end of its move
// this turn, whom that row acts on, and whether this view can say.
//
// A GROUP, A HOLD, AN ATTACK AND A LINE STAND WHERE THE MONSTER IS. An attack's
// target is the member it would strike. A `toward` and a `dash` at a member or
// along a route land where the [TableDriver.walkIntent] would cut the walk —
// the same selector, the same path, the same pace — or where the monster stands
// when that walk would not move it. Everything else is unknown ([threatJudge]).
// The walk's tail can still be trimmed at execution off an ally's cell
// ([Encounter.standableEnd]); the landing here is the walk as asked.
func landingOf(view MonsterView, row Answer) (spatial.Position, MemberID, bool) {
	var d TableDriver
	switch {
	case len(row.Rows) > 0, row.Hold:
		return view.Position, "", true
	case row.Attack != nil:
		target, _ := d.selectMember(view, row, *row.Attack)

		return view.Position, target, true
	case row.Toward != nil:
		return walkLanding(view, row, *row.Toward, walkPace(view))
	case row.Dash != nil:
		return walkLanding(view, row, *row.Dash, dashPace(view))
	case row.Away != nil, row.Search != nil, row.Flee:
		return spatial.Position{}, "", false
	default:
		return view.Position, "", true
	}
}

// walkLanding is [landingOf] for a walking row: [TableDriver.walkIntent]'s
// own branches, asked for the last cell instead of the intent.
func walkLanding(view MonsterView, row Answer, sel Selector, p pace) (spatial.Position, MemberID, bool) {
	var d TableDriver
	switch {
	case p.cells <= 0:
		return view.Position, "", true
	case sel.At != nil, sel.Word == SelectorSound:
		return spatial.Position{}, "", false
	case len(sel.Path) > 0:
		if view.Dwelling {
			return view.Position, "", true
		}
		ahead, _ := patrolStep(view.Position, sel.Path)
		if len(ahead) == 0 {
			return spatial.Position{}, "", false
		}

		return ahead[min(p.cells, len(ahead))-1], "", true
	}

	target, ok := d.selectMember(view, row, sel)
	if !ok {
		return view.Position, "", true
	}
	if alreadyInReach(view, target) {
		return view.Position, target, true
	}
	path := d.pathTo(view, target)
	if len(path) == 0 {
		return view.Position, target, true
	}

	return path[min(p.cells, len(path))-1], target, true
}

// tableNamesThreat reports whether any `time` row — inside a group, inside a
// `not` — names a `threat` condition: the one reader of [MonsterView.Threats],
// so a monster whose table never asks pays no flood for it.
func tableNamesThreat(rows []Answer) bool {
	for i := range rows {
		for j := range rows[i].When {
			for w := &rows[i].When[j]; w != nil; w = w.Not {
				if w.Threat != "" {
					return true
				}
			}
		}
		if tableNamesThreat(rows[i].Rows) {
			return true
		}
	}

	return false
}

// threatZone is one opposed member's [ThreatZone], from where the observer
// believes it stands.
//
// THE COMPUTATION. A flood from that cell, as that member walks (its own
// occupancy rules, the observer's own picture of the walls), out to twice its
// sheet speed in cells; then every cell within its reach — its longest
// action's range in cells, at least one, measured as [SeenMember.InReach]
// measures it, through nothing — of a cell it reaches in at most its speed is
// Walk, and of one it reaches in at most twice its speed and not already in
// Walk is Dash. A member with no speed threatens only around where it stands.
func (q *cellQuery) threatZone(observer, foe MemberID, at spatial.Position) (ThreatZone, error) {
	zone := ThreatZone{Member: foe}
	sheet, err := q.sheetOf(foe)
	if err != nil {
		return zone, err
	}
	speed := CellsFromFeet(sheet.SpeedFeet)
	reach := 1
	for _, a := range sheet.Actions {
		reach = max(reach, CellsFromFeet(a.RangeFeet))
	}

	walkFrom, dashFrom := []spatial.Position{at}, []spatial.Position{at}
	if speed > 0 {
		field, ferr := q.floodFrom(foe, observer, at, nil, 2*speed)
		if ferr != nil {
			return zone, ferr
		}
		walkFrom, dashFrom = nil, nil
		for cell, steps := range field.Dist {
			if steps <= speed {
				walkFrom = append(walkFrom, cell)
			}
			dashFrom = append(dashFrom, cell)
		}
	}

	walk, err := q.withinReach(walkFrom, reach)
	if err != nil {
		return zone, err
	}
	dash, err := q.withinReach(dashFrom, reach)
	if err != nil {
		return zone, err
	}
	walkSet := make(map[spatial.Position]bool, len(walk))
	for _, c := range walk {
		walkSet[c] = true
	}
	for _, c := range dash {
		if !walkSet[c] {
			zone.Dash = append(zone.Dash, c)
		}
	}
	zone.Walk = walk

	return zone, nil
}

// withinReach is every cell within reach steps of any of from, through
// nothing — reach is measured, not walked — in scan order.
func (q *cellQuery) withinReach(from []spatial.Position, reach int) ([]spatial.Position, error) {
	field, err := spatial.Field(q.canvas.GetGrid(), spatial.FieldInput{
		Sources:  from,
		Limit:    reach,
		Passable: func(_, _ spatial.Position) bool { return true },
	})
	if err != nil {
		return nil, err
	}
	out := make([]spatial.Position, 0, len(field.Dist))
	for cell := range field.Dist {
		out = append(out, cell)
	}
	sort.Slice(out, func(i, j int) bool { return beforeInScanOrder(out[i], out[j]) })

	return out, nil
}

// reachTail is [RememberedMember.ReachTail]: how many of path's last cells a
// walk may leave untaken and still end with the remembered cell inside
// rangeCells of where it stops.
func (e *Encounter) reachTail(own, remembered spatial.Position, path []spatial.Position, rangeCells int) int {
	if e.Distance(own, remembered) <= float64(rangeCells) {
		return len(path)
	}
	for i, cell := range path {
		if e.Distance(cell, remembered) <= float64(rangeCells) {
			return len(path) - (i + 1)
		}
	}

	return 0
}
