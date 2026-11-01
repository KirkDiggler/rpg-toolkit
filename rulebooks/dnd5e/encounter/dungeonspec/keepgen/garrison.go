// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"errors"
	"fmt"
)

// garrison.go gives the placed monsters orders (rpg-project#766 S1): the
// first [Options.Garrison] in placement order join one faction, `garrison`,
// and of those the first [Options.Patrols] each walk a route of their own;
// the rest hold their post. It is the population the world clock is measured
// on: who is due, and who is not. The garrison is NEUTRAL to the party, so a
// party at the start is watched rather than fought and the world clock,
// never a fight's turn clock, carries the Moves the story measures.
//
// A ROUTE IS A TRIANGLE OF STRAIGHT LEGS. A patrol starts on its own cell
// and walks two more, each leg one hex line along an axis, so the leg the
// engine walks ([encounter.LegLine]) is exactly the cells checked here. The
// loop closes itself — the last leg leads back to the first waypoint — so
// the first waypoint is not written twice: a repeated waypoint would make
// the first and the third legs share it, which the dialect refuses as a
// route that retraces itself. Every cell of every leg is floor of the
// monster's own region, under no prop and on no other monster's start.

// ErrBadOptions is the refusal of an option that cannot be met: a negative
// garrison or patrol count, more patrols than garrison, or more garrison
// than the monsters placed.
var ErrBadOptions = errors.New("keepgen: bad options")

const (
	// garrisonFaction is the one side every ordered monster joins.
	garrisonFaction = "garrison"
	// postTable is the table of a monster that holds where it stands.
	postTable = "post"
	// maxLeg is the longest leg a route is drawn with, in steps.
	maxLeg = 3
)

// ringDirections are the six axial steps in ring order: any two in a row
// are sixty degrees apart, so with the start they make an equilateral
// triangle whose third side is a step direction too.
var ringDirections = [6][2]int{{1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}, {0, 1}}

// order is one monster's orders: its index in [site.monsters], and its
// route's waypoints, or none for a post.
type order struct {
	monster int
	route   []cell
}

// patrolTable is the table id of the i-th patrol.
func patrolTable(i int) string { return fmt.Sprintf("patrol-%d", i) }

// placeGarrison gives the first o.Garrison monsters their orders.
func (s *site) placeGarrison(owner map[cell]int, occupied, taken map[cell]bool, o Options) error {
	if o.Garrison > len(s.monsters) {
		return fmt.Errorf("%w: garrison %d is more than the %d monsters placed", ErrBadOptions, o.Garrison, len(s.monsters))
	}
	for i := 0; i < o.Garrison; i++ {
		ord := order{monster: i}
		if i < o.Patrols {
			m := s.monsters[i]
			route, ok := routeFor(m, owner, occupied, taken, o.Seed)
			if !ok {
				return fmt.Errorf("keepgen: patrol %s: no route of floor cells fits around %v", m.id, m.at.axial())
			}
			ord.route = route
		}
		s.orders = append(s.orders, ord)
	}
	return nil
}

// routeFor draws m's route: the start and two cells a leg away along two
// step directions sixty degrees apart, the longest legs that fit first, the
// first direction drawn from m's own seeded source.
func routeFor(m placedMonster, owner map[cell]int, occupied, taken map[cell]bool, seed int64) ([]cell, bool) {
	walkable := func(c cell) bool {
		if r, ok := owner[c]; !ok || r != m.region || occupied[c] {
			return false
		}
		return c == m.at || !taken[c]
	}
	a := m.at.axial()
	turn := stream(seed, m.id, "patrol").IntN(len(ringDirections))
	for k := maxLeg; k >= 1; k-- {
		for t := range ringDirections {
			d1 := ringDirections[(turn+t)%len(ringDirections)]
			d2 := ringDirections[(turn+t+1)%len(ringDirections)]
			step := func(j int, d [2]int) cell { return fromAxial(a.Q+j*d[0], a.R+j*d[1]) }
			ok := true
			for j := 1; j <= k && ok; j++ {
				// Out along d1, across from d1·k to d2·k, and back along d2.
				across := fromAxial(a.Q+k*d1[0]+j*(d2[0]-d1[0]), a.R+k*d1[1]+j*(d2[1]-d1[1]))
				ok = walkable(step(j, d1)) && walkable(across) && walkable(step(j, d2))
			}
			if ok {
				return []cell{m.at, step(k, d1), step(k, d2)}, true
			}
		}
	}
	return nil, false
}
