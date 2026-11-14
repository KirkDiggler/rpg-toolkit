// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// wanted.go is THE WANTED, SEIZED ON SIGHT (rpg-project#925, ruled
// 2026-11-14: "Def a… in this case though we are doing the same as the
// others"): a member who was at the gate and left is wanted, and an arresting
// guard who sees them again takes them where they stand.
//
// # The rule
//
// On every sight refresh, for each arrest record with wanted members: an
// arresting guard who is up and not in a fight, who holds [TopicWanted]
// about the member, sees them on the live reach — so a sneak that beat him
// keeps them unseen — and is not opposed to them, SEIZES them. A hostile
// pair is fought, not arrested: the stance path forms that fight.
//
// WITHIN THE HAIL. When the conversation declares an approach, the guard
// seizes only a member within its hail distance ([Approach.Within]): the one
// it would hail, so the step that brings them up is answered with the lines
// the held are offered. A member seized out of speaking reach could neither
// walk away nor answer, with no way out at all.
//
// # What a seizure does
//
// The member leaves the record's Wanted and joins its Taken, held in their
// own right ([ArrestedMember.Custody] is [CustodyHeld]) wherever they stand
// — whatever phase the group taken at the arrest is in (held, jailed, or
// freed by a fight). Every arresting guard who is up comes to hold
// [TopicArrested] about them, told to that guard alone. The member is told
// an `arrested` beat naming themselves and the cell, as the first group was;
// a witness who sees them is told who was taken and no cell. Their sneak
// ends. From there they have the choices the others had: the lines the
// arrested gate offers a held member, or a fight.
//
// # Cost
//
// Nothing is computed when no record has wanted members: one scan of the
// run's interactions, and the sight reach only when one qualifies.

// sweepWanted is the seizure rule (this file's doc), asked on every sight
// refresh beside [Encounter.sweepEscapes].
func (e *Encounter) sweepWanted() error {
	var reach sightReach
	asked := false
	for _, id := range slices.Sorted(maps.Keys(e.interactions)) {
		made := e.interactions[id].arrest
		if made == nil || len(made.Wanted) == 0 {
			continue
		}
		if !asked {
			var err error
			if reach, err = e.placedSightReach(); err != nil {
				return err
			}
			asked = true
		}
		guards, err := e.arrestingGuards(id)
		if err != nil {
			return err
		}
		var seized []MemberID
		seers := map[MemberID][]MemberID{}
		within := -1
		if a := e.interactions[id].input.Approach; a != nil {
			within = a.Within
		}
		for _, w := range made.Wanted {
			at, placed := e.canvas.GetEntityPosition(string(w.ID))
			if _, ok := e.members[w.ID]; !ok || !placed {
				continue
			}
			for _, g := range guards {
				if !reach.Reaches(perception.Sight, g, w.ID) || e.opposed(g, w.ID) {
					continue
				}
				// WITHIN THE GATE'S HAIL: a conversation with an approach seizes
				// only whom it would hail, so the seized can answer — a member held
				// out of speaking reach could neither walk nor speak.
				if within >= 0 {
					from, ok := e.canvas.GetEntityPosition(string(g))
					if !ok || e.Distance(from, at) > float64(within) {
						continue
					}
				}
				if fighting, err := e.inFight(g); err != nil {
					return err
				} else if fighting {
					continue
				}
				if _, held := e.intelLog.AboutOn(sightMember(g), sightMember(w.ID), perception.Topic(TopicWanted)); held != nil {
					continue
				}
				seers[w.ID] = append(seers[w.ID], g)
			}
			if len(seers[w.ID]) > 0 {
				seized = append(seized, w.ID)
			}
		}
		if len(seized) == 0 {
			continue
		}
		if err := e.seize(id, made, guards, seized, seers, reach); err != nil {
			return err
		}
	}
	return nil
}

// seize takes the wanted members given into the record's custody (this
// file's doc).
func (e *Encounter) seize(id InteractionID, made *ArrestRecord, guards, seized []MemberID,
	seers map[MemberID][]MemberID, reach sightReach) error {
	at := uint64(e.clock.ToData().HighWater)
	made.Wanted = slices.DeleteFunc(made.Wanted, func(w WantedMember) bool { return slices.Contains(seized, w.ID) })
	for _, m := range seized {
		e.endSneaking(m)
		// A TAKEN MEMBER WHO LEFT BEFORE THE JAIL is already on the record
		// (jail.go's "the gone stay wanted"): they are held again in their
		// own right. Anyone else joins the taken.
		if i := slices.IndexFunc(made.Taken, func(t ArrestedMember) bool { return t.ID == m }); i >= 0 {
			made.Taken[i].Custody, made.Taken[i].SeenBy = CustodyHeld, seers[m]
			continue
		}
		made.Taken = append(made.Taken, ArrestedMember{ID: m, SeenBy: seers[m], Custody: CustodyHeld})
	}

	line := recognitionLine{interaction: id, at: at, holdersOnly: true}
	if err := e.grantArrestedTo(reach, guards, seized, line); err != nil {
		return err
	}
	// THE SEIZED ARE TOLD, as the first group was; a witness who sees one is
	// told who, never the cell ([Encounter.tellArrest]).
	down, err := e.downNow()
	if err != nil {
		return err
	}
	told := ArrestRecord{Cell: made.Cell}
	for _, m := range seized {
		told.Taken = append(told.Taken, ArrestedMember{ID: m})
	}
	var witnesses []MemberID
	for _, w := range e.rosterIDs() {
		if down[w] || slices.Contains(guards, w) || slices.Contains(seized, w) {
			continue
		}
		for _, m := range seized {
			if reach.Reaches(perception.Sight, w, m) {
				witnesses = append(witnesses, w)
				break
			}
		}
	}
	line.witnesses, line.holdersOnly = witnesses, false
	return e.tellArrest(reach, told, guards, line)
}

// grantArrestedTo has every arresting guard who is up come to hold
// [TopicArrested] about the members given, each told his own holding alone.
func (e *Encounter) grantArrestedTo(reach sightReach, guards, members []MemberID, line recognitionLine) error {
	for _, g := range guards {
		if err := e.holdTopic(reach, g, members, TopicArrested, ChannelJudgement, line); err != nil {
			return err
		}
	}
	return nil
}
