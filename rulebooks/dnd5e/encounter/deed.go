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
// witness would say they saw somebody DO when a swing, a shot or a harmful
// spell resolved. Struck, missed, saved against or simply landed, it is the
// same deed — a miss is still a shot at you, and so is a spell you shrugged
// off (rpg-project#493, R5; [hostileIntent]).
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

// DeedFled is the verb a creature's own memory of having been made to run
// lands under: what the creature would say happened TO IT when a `flee` entry
// fired (rpg-project#465, ideas/creature-table/design.md §2).
//
// ACTOR IS WHO CAUSED IT, not who ran. That is the rule every deed a `when`
// condition reads keeps — [DeedAttack], [DeedIntimidate] and [DeedPersuade]
// all name the doer in Actor and the creature in Target — and it is what
// makes `away: actor` on the creature's own `time` table run away from the
// one who scared it rather than from itself.
//
// IT LANDS ON THE CREATURE AND NOBODY ELSE ([Encounter.landFled]), which is
// the difference between this verb and the other three. A deed is normally
// what a witness SAW; this one is what the creature KNOWS about itself, and
// telling the room would publish a goblin's private state.
//
// THERE IS NO RUNNING IN IT. The deed is the whole of what `flee` does: the
// verb that scared the creature pays a round on the world clock, and the
// creature's own `time` table reading `fled: { within: N }` is what walks it
// away, for as many rounds as the author wrote.
const DeedFled = "fled"

// landAttack tells every member whose senses reach the actor's cell that
// the actor attacked, through perception's own Report door, in each
// witness's terms (mind/behavior rule A4: a deed is landed where the fact
// is known and nowhere else, and a recorded outcome is where an attack is
// known).
//
// A deed names one target. When an outcome has several, the first in
// sorted order is the one named; the others are the outcome beat's to
// tell. No use case has paid for more than one yet.
//
// A SPELL COMES THROUGH HERE TOO (rpg-project#493, R5; [hostileIntent]).
// [Encounter.Record]'s struck and missed kinds are one door and
// [Encounter.RecordCast] is the other, and the cast's door is not any one of
// its arms: a spell attack, a save asked against a harmful effect, and a
// delivery that lands its harm with no gate at all each land the same
// [DeedAttack] on the same recipient, so `attacked: { within: N }` and the
// `attacker` selector see a spell however it arrived. The verb is the same
// because the testimony is: a witness saw somebody try to hurt somebody.
//
// THE AGGRESSION LAW RUNS FIRST (rpg-project#493, R3; turning.go): hostile
// intent across a neutral pair makes it hostile, and it has to make it
// hostile BEFORE this testimony lands, or the very pick the deed provokes
// reads `enemy: none` on a camp that is already at war. The audience is asked
// afterwards for the same reason it is asked at all — a fight forming is a
// sight refresh, and the witnesses of the deed are the ones there are now.
func (e *Encounter) landAttack(actor MemberID, targets []MemberID) error {
	var target MemberID
	if len(targets) > 0 {
		target = targets[0]
	}

	if err := e.aggression(actor, target, uint64(e.clock.ToData().HighWater)); err != nil {
		return err
	}

	where, witnesses, err := e.audienceOf(actor)
	if err != nil {
		return err
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
	return e.landDeedAt(verb, actor, target, where, witnesses, uint64(e.clock.ToData().HighWater))
}

// landDeedAt is [Encounter.landDeed] with the stamp named rather than read.
//
// THE STAMP IS THE CAUSE'S, NOT THE LANDING'S. A verb reads the clock once at
// its start and every beat and deed it produces carries that one reading, so
// a verb that advances the world clock on its way out cannot leave its own
// deed stamped a round after the thing that caused it. Every caller that has
// a reading in hand passes it; landDeed is the name for "now".
func (e *Encounter) landDeedAt(
	verb string, actor, target MemberID, where spatial.Position, witnesses []core.EntityID, at uint64,
) error {
	if err := stage.Land(&stage.LandInput{
		Store:     e.intelLog,
		Deed:      deed.Deed{Verb: verb, Actor: actor, Target: target, Where: where.String()},
		Witnesses: witnesses,
		At:        at,
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
