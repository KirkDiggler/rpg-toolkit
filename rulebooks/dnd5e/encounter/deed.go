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

// DeedIntimidate is the verb a beaten Intimidate check lands under: what a
// witness would say they saw somebody DO when a character threatened a
// monster and the threat landed (rpg-project#454,
// ideas/shenanigans/intimidate.md).
//
// A MISSED THREAT LANDS NOTHING, which is the difference between this verb
// and [DeedAttack]. A miss is still a shot at you; a threat nobody was
// frightened by is a sentence in the air. Whether a failed attempt provokes
// anyone is an open item on rpg-project#454, deliberately not a hidden
// default here.
//
// What the deed is WORTH is the mind's business and not this module's: the
// coward reads it as fear, the berserker as a provocation, the retaliator
// as nothing at all (rulebooks/dnd5e/behavior). This composition lands the
// testimony and forms no opinion — there is no flee flag, and rule A2 is
// why (the design's first broken cut).
const DeedIntimidate = "intimidate"

// DeedPersuade is the verb a beaten Persuade check lands under: what a witness
// would say they saw somebody DO when a character talked a creature round and
// the appeal landed (rpg-project#458,
// ideas/shenanigans/front-room-goblin.md).
//
// [DeedIntimidate]'S TWIN, deliberately its own verb and not a flag on that
// one. A mind reads the verb: the coward's fear keys on being threatened and
// nothing about being reasoned with, and a single "social" deed with a
// polarity field would make every preset ask a second question before it knew
// what happened to it. What a held `persuade` deed is WORTH is the preset's
// business, and today every shipped preset holds it and does nothing — which
// is the zero value telling the truth, not a gap.
//
// A MISSED APPEAL LANDS NOTHING, [DeedIntimidate]'s rule for
// [DeedIntimidate]'s reason.
const DeedPersuade = "persuade"

// landAttack tells every member whose senses reach the actor's cell that
// the actor attacked, through perception's own Report door, in each
// witness's terms (mind/behavior rule A4: a deed is landed where the fact
// is known and nowhere else, and Record is where an attack is known).
//
// A deed names one target. When an outcome has several, the first in
// sorted order is the one named; the others are the outcome beat's to
// tell. No use case has paid for more than one yet.
func (e *Encounter) landAttack(actor MemberID, targets []MemberID) error {
	where, witnesses, err := e.audienceOf(actor)
	if err != nil {
		return err
	}

	var target MemberID
	if len(targets) > 0 {
		target = targets[0]
	}

	return e.landDeed(DeedAttack, actor, target, where, witnesses)
}

// audienceOf answers where a member is standing and who can see that cell —
// the two facts every deed needs before it can be landed, asked once so the
// Intimidate verb can refuse on the audience it is about to land on rather
// than computing it a second time.
func (e *Encounter) audienceOf(actor MemberID) (spatial.Position, []core.EntityID, error) {
	record, ok := e.members[actor]
	if !ok {
		return spatial.Position{}, nil, fmt.Errorf("deed: actor %q: %w", actor, ErrNoMember)
	}

	where, err := e.cellOf(record)
	if err != nil {
		return spatial.Position{}, nil, fmt.Errorf("deed: actor %q: %w", actor, err)
	}

	witnesses, err := e.witnessesOf(where)
	if err != nil {
		return spatial.Position{}, nil, fmt.Errorf("deed: %w", err)
	}

	return where, witnesses, nil
}

// landDeed puts one deed on every witness, at the clock's high-water mark.
//
// THE VERB IS THE CALLER'S, and so is the audience. Every deed this
// composition publishes goes through here, so "a witness keeps an actor's
// latest deed only" and the clock reading a deed is stamped with are one
// answer rather than one per verb.
func (e *Encounter) landDeed(
	verb string, actor, target MemberID, where spatial.Position, witnesses []core.EntityID,
) error {
	if err := stage.Land(&stage.LandInput{
		Store:     e.intelLog,
		Deed:      deed.Deed{Verb: verb, Actor: actor, Target: target, Where: where.String()},
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
