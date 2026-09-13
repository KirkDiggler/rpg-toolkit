// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/stage"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// DeedAttack is the verb an attack lands under on the deeds channel: what a
// witness would say they saw somebody DO when a swing or a shot resolved.
// Struck or missed, it is the same deed — a miss is still a shot at you.
const DeedAttack = "attack"

// landAttack tells every member whose senses reach the actor's cell that
// the actor attacked, through perception's own Report door, in each
// witness's terms (mind/behavior rule A4: a deed is landed where the fact
// is known and nowhere else, and Record is where an attack is known).
//
// A deed names one target. When an outcome has several, the first in
// sorted order is the one named; the others are the outcome beat's to
// tell. No use case has paid for more than one yet.
func (e *Encounter) landAttack(actor MemberID, targets []MemberID) error {
	record, ok := e.members[actor]
	if !ok {
		return fmt.Errorf("deed: actor %q: %w", actor, ErrNoMember)
	}

	where, err := e.cellOf(record)
	if err != nil {
		return fmt.Errorf("deed: actor %q: %w", actor, err)
	}

	witnesses, err := e.witnessesOf(where)
	if err != nil {
		return fmt.Errorf("deed: %w", err)
	}

	d := deed.Deed{Verb: DeedAttack, Actor: actor, Where: where.String()}
	if len(targets) > 0 {
		d.Target = targets[0]
	}

	if err := stage.Land(&stage.LandInput{
		Store:     e.intelLog,
		Deed:      d,
		Witnesses: witnesses,
		At:        uint64(e.clock.ToData().HighWater),
	}); err != nil {
		return fmt.Errorf("deed: %w", err)
	}

	return nil
}

// witnessesOf is every placed member whose sight reaches a cell: within its
// sight range and not blocked — the same two tests [sightReach] makes for a
// pass, asked of a place instead of a subject. Sorted by member so two
// identical outcomes land identically.
func (e *Encounter) witnessesOf(cell spatial.Position) ([]core.EntityID, error) {
	cells, err := e.sightNow()
	if err != nil {
		return nil, fmt.Errorf("witnesses: %w", err)
	}

	ids := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	out := make([]core.EntityID, 0, len(ids))
	for _, id := range ids {
		pos, err := e.cellOf(e.members[id])
		if err != nil {
			continue
		}
		if e.canvas.GetGrid().Distance(pos, cell) > float64(cells[id]) {
			continue
		}
		if e.canvas.IsLineOfSightBlocked(pos, cell) {
			continue
		}
		out = append(out, id)
	}

	return out, nil
}
