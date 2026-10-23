// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// memory.go is PURSUIT'S MEMORY (rpg-project#668, ideas/smarter-monsters):
// what a creature's own sight testimony of an enemy becomes as it chases a
// memory, searches, and gives up.
//
//	remembered ──its `toward` finds nothing to walk to──▶ lost ──search ends──▶ unknown
//
// EVERY STATE IS THE CREATURE'S OWN TESTIMONY, rewritten in its own holding
// and nowhere else. Nothing here reads where the subject actually is: a
// remembered cell the subject left long ago is still walked to, and a search
// runs along the heading the creature last SAW, right or wrong. Fresh sight
// replaces any of the three on the next sweep, which is what lets a glimpse
// restart the chase.
//
// NO STATE OF ITS OWN. The search cell is derived each turn from the lost
// testimony and the row's distance ([LostMember.Trail]), as a patrol's next
// leg is derived from where the walker stands; giving up is the testimony
// changing, so a returning creature is simply one whose `enemy: none` rows
// hold again.

// loseMemory turns the observer's own REMEMBERED testimony of subject lost
// (rpg-project#668 R3): its `toward` found nothing left to walk to — it stands
// on the remembered cell, or no route reaches it. The cell and the heading
// stay; only the state says the place has been looked at.
//
// ONLY A HELD, PLACED MEMORY. A subject in sight now has nothing to lose, an
// unknown or lost one has nothing left to mark, and a player's memory is
// never asked: only a driven creature's own `toward` sends this.
func (e *Encounter) loseMemory(observer, subject MemberID) error {
	holdings, err := e.memberIntel(observer)
	if err != nil {
		return fmt.Errorf("lose %q: %w", subject, err)
	}

	return e.rewriteSight(observer, subject, holdings, func(t *SightTestimony) bool {
		if t.State != LocationKnown {
			return false
		}
		t.State = LocationLost

		return true
	})
}

// giveUpSearch ends a search (rpg-project#668 R5): the member's testimony of
// the subject becomes UNKNOWN — known to exist, placed nowhere — so no band
// above `none` holds for it, and whatever the author wrote under `enemy:
// none` carries the member home.
func (e *Encounter) giveUpSearch(observer, subject MemberID) error {
	holdings, err := e.memberIntel(observer)
	if err != nil {
		return fmt.Errorf("give up on %q: %w", subject, err)
	}

	return e.rewriteSight(observer, subject, holdings, func(t *SightTestimony) bool {
		*t = SightTestimony{State: LocationUnknown}

		return true
	})
}

// rewriteSight lands the observer's own sight testimony of one subject again,
// changed by change, as HELD testimony ([perception.Perception.Report]): a
// discrete fact the observer now holds, never a sighting it is making. change
// reports whether it changed anything worth landing.
//
// A HOLDING THAT IS NOT THERE, NOT SIGHT TESTIMONY, OR CURRENT IS LEFT ALONE:
// there is nothing of the observer's to rewrite, inventing a holding would be
// the composition teaching a creature something it never saw, and a subject
// in sight right now is not a memory to revise.
func (e *Encounter) rewriteSight(
	observer, subject MemberID, holdings []perception.Holding, change func(*SightTestimony) bool,
) error {
	for _, h := range holdings {
		if h.Channel != perception.Sight || MemberID(h.Subject) != subject || h.CurrentOn(perception.Sight) {
			continue
		}
		testimony, ok := DecodeSightTestimony(h.Payload)
		if !ok {
			return fmt.Errorf("sighting of %q: %w", subject, ErrInvalidData)
		}
		if !change(&testimony) {
			return nil
		}
		payload, err := EncodeSightTestimony(testimony)
		if err != nil {
			return fmt.Errorf("sighting of %q: %w", subject, err)
		}
		if _, err := e.intelLog.Report(perception.ReportInput{
			Observer: sightMember(observer),
			Channel:  perception.Sight,
			At:       uint64(e.clock.ToData().HighWater),
			Reports:  []perception.Presence{{ID: sightMember(subject), Payload: payload}},
		}); err != nil {
			return fmt.Errorf("sighting of %q: %w", subject, err)
		}

		return nil
	}

	return nil
}

// searchTrail is the straight line a search walks from a lost cell along a
// heading (rpg-project#668 R4): the neighbour of `from` whose bearing is
// nearest the heading names the step, and the line repeats that step for as
// long as each next cell is a step from the last — reached by mover's own
// flood from `from` in exactly as many steps as it is along the line. The
// first cell that is not ends it: a search does not walk through a wall to
// keep its line, and does not detour round one either.
//
// NEAREST FIRST, and empty when the first step is already blocked. Ties on
// the bearing go to the grid's own neighbour order, so the same heading
// always walks the same line (C8).
func (q *cellQuery) searchTrail(mover MemberID, from spatial.Position, heading float64) ([]spatial.Position, error) {
	var step spatial.Position
	best := math.Inf(1)
	for _, n := range adjacencyGrid.GetNeighbors(from) {
		b, ok := bearing(q.field.orientation, from, n)
		if !ok {
			continue
		}
		diff := math.Abs(math.Mod(b-heading+540, 360) - 180)
		if diff < best {
			best, step = diff, spatial.Position{X: n.X - from.X, Y: n.Y - from.Y}
		}
	}
	if math.IsInf(best, 1) {
		return nil, nil
	}

	field, err := q.floodFrom(mover, mover, from, nil, 0)
	if err != nil {
		return nil, err
	}
	var trail []spatial.Position
	cell := from
	for k := 1; ; k++ {
		cell = spatial.Position{X: cell.X + step.X, Y: cell.Y + step.Y}
		if d, reached := field.Dist[cell]; !reached || d != k {
			return trail, nil
		}
		trail = append(trail, cell)
	}
}
