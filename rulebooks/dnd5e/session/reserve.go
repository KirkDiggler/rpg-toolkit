// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// reserve.go is ARRIVALS at the seam (rpg-project#375, the hold-out design
// §3.7, §5, R6): a monster spawned with a predicate waits in reserve — no
// cell, no turn, no roster row, absent from every projection for every
// member — and is placed on the first verb after its predicate holds, with an
// `arrived` beat to everyone.
//
// # The predicate crosses in this package's own words
//
// The composition's predicate is its sealed [encounter.Trigger]; S2 says no
// encounter type crosses this boundary, so the host hands in an [Arrival] —
// the designer's grammar (design §2: `round | down | fact | stance`), one type
// per form, sealed the way [DissolveCause] is — and triggerOf converts it at
// the door. A host compiling a dungeon file already holds each placement's
// predicate in the composition's form; the switch it writes to get here is the
// same four arms triggerOf writes to get back, and that round trip is the cost
// of a boundary the rest of this package pays everywhere (convert.go).
//
// # What this seam contributes beyond forwarding
//
// A reserved member ARRIVES INSIDE SOME LATER VERB — the participation pass
// that notices the chief down, the round that starts, the fact that lands —
// and the arrival's own sight refresh asks the Sight and Standing seams about
// the newcomer at once. Both seams are seeded at load from the stored world,
// so they must be seeded from what WAITS as well as from what stands
// ([worldMembers]); a seam that knew only the placed roster would refuse the
// zombie the moment it stepped onto the map, and the verb that brought it
// would fail.

// Arrival is the predicate a spawned monster waits in reserve on: exactly one
// of the forms below, nil when the monster is placed at once. See the file's
// doc for why it is this package's own type.
type Arrival interface {
	// isArrival seals the set: a form is declared here or it is not one.
	isArrival()
}

// ArrivesAtRound is `{ round: N }`: the monster arrives when any fight in the
// run starts round N (N counted from 1). Outside a fight nothing counts (R9).
type ArrivesAtRound struct {
	Round int
}

func (ArrivesAtRound) isArrival() {}

// ArrivesOnFall is `{ down: <member id> }`: the monster arrives when that
// member is Down — the chief falls and the reinforcements come.
type ArrivesOnFall struct {
	Member string
}

func (ArrivesOnFall) isArrival() {}

// ArrivesOnFact is `{ fact: <id> }`: the monster arrives when the fact exists
// in the run's journal, learned by anyone — the truth grain (R5).
type ArrivesOnFact struct {
	Fact string
}

func (ArrivesOnFact) isArrival() {}

// ArrivesOnStance is `{ stance: { between: [a, b], is: <stance> } }`: the
// monster arrives when the pair's stance folds to that word — hostile,
// neutral or allied, the dungeon file's own vocabulary.
type ArrivesOnStance struct {
	Between [2]string
	Stance  string
}

func (ArrivesOnStance) isArrival() {}

// triggerOf converts an arrival into the composition's own predicate, at the
// boundary and nowhere else. nil crosses as nil: no predicate, placed at once.
// The set is sealed, so every form has an arm; what each form MEANS — and
// whether it can ever hold — is the composition's to judge, and it refuses by
// name (ErrNoMember) a predicate nothing could fire.
func triggerOf(a Arrival) encounter.Trigger {
	switch a := a.(type) {
	case ArrivesAtRound:
		return encounter.TriggerRound{Round: a.Round}
	case ArrivesOnFall:
		return encounter.TriggerMemberDown{Member: encounter.MemberID(a.Member)}
	case ArrivesOnFact:
		return encounter.TriggerFact{Fact: a.Fact}
	case ArrivesOnStance:
		return encounter.TriggerStance{
			Between: [2]encounter.FactionID{a.Between[0], a.Between[1]},
			Stance:  encounter.Stance(a.Stance),
		}
	default:
		return nil
	}
}

// PlacementKind names what arrived: a member or a thing. A closed set, because
// a client branches on it — a monster gets a roster row and a prop does not —
// and it maps onto a proto enum (PlacementKind MONSTER | PROP).
type PlacementKind string

const (
	// PlacementMonster is a member arriving out of reserve.
	PlacementMonster PlacementKind = "monster"

	// PlacementProp is a prop appearing where the author drew it.
	PlacementProp PlacementKind = "prop"
)

// worldMembers is every member the stored world has, PLACED OR WAITING: the
// roster as persisted, followed by the reserve as the members they will be.
// What the Sight and Standing seams are seeded from at every load, because an
// arrival happens mid-verb and its own sight refresh asks both seams about the
// newcomer at once (this file's doc). Members first, so a plain world's seams
// are seeded exactly as they always were.
func worldMembers(world encounter.EncounterData) []encounter.MemberData {
	out := make([]encounter.MemberData, 0, len(world.Members)+len(world.Reserve))
	out = append(out, world.Members...)
	for _, r := range world.Reserve {
		out = append(out, encounter.MemberData{
			ID:             r.ID,
			Kind:           r.Kind,
			Name:           r.Name,
			SpeedFeet:      r.SpeedFeet,
			SightFeet:      r.SightFeet,
			Actions:        r.Actions,
			Targeting:      r.Targeting,
			BlocksMovement: r.BlocksMovement,
			Faction:        r.Faction,
		})
	}
	return out
}
