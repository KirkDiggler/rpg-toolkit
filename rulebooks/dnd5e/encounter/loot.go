// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// loot.go is THE LOOT VERB'S RULE HALF (rpg-project#368, design §4.2), beside
// search.go and under the same law: everything that keeps a secret secret is
// decided here rather than at the seam.
//
// # Loot is offered on EVERY body (design P3)
//
// The affordance must not say which monster carries intel. Every downed
// member is lootable; a body with nothing to give transfers nothing, appends
// the same `looted` beat, and produces the same bytes as one that had
// everything — the `looted` beat names the looter and the body and NOTHING of
// what moved. Same law as search: the answer never leaks the question. The
// one observable difference between a rich body and an empty one is what the
// transfer CAUSES, and for intel that is the ordinary recipient-scoped
// DOOR_REVEALED beat a search would have produced.
//
// # Free, no check (ruled R1)
//
// "The looter gets it for free — no check." There is no [CheckResolver] here
// and there is no roll: this verb has nothing to resolve. Reading a looted
// parchment is a check (design §9, shelved); taking it off a body is not.
//
// # A second writer of the fact search writes (design P4)
//
// Intel becomes `learnDoor(looter, door, "loot")` plus the looter's own
// DOOR_REVEALED beat — the beat slice 1 already sends. ZERO new reveal shape.
// conceal.go states the rule this obeys: "a new cause is a new writer of an
// existing fact, never a new mechanism."

// LootInput declares looting one downed member.
type LootInput struct {
	// Member is who loots.
	Member MemberID

	// Target is the body.
	Target MemberID

	// Range is the maximum distance, in cells, Target may stand from Member.
	// Zero (the default) means adjacent — one cell, as [InteractInput.Range]
	// does. A negative Range is a caller defect and is refused rather than
	// normalized.
	Range int
}

// LootOutput acknowledges that the loot happened, and deliberately nothing
// more.
//
// ACK-ONLY, and the beat is the answer — [SearchOutput]'s shape, chosen for
// symmetry (design Q1's lean). A `Transferred` count was weighed: it says
// nothing about WHAT moved, and it would let a client tell "this body carried
// nothing" from "the beat has not arrived yet". Neither is a secret worth
// keeping on its own, but the two verbs answering differently about the same
// question is how a law with one exception becomes a law with two. Revisit
// when the parchment shelf (design §9) makes loot yield an item.
type LootOutput struct{}

// Loot moves everything a downed member holds to the looter — for free, no
// check (R1).
//
// Today a body holds intel, a prop, or both, and [Encounter.transferHoldings]
// moves each by its own kind: intel through the reveal path search already
// owns, a prop by becoming the looter's holding. That routine is the ONE
// place a holding changes hands, so Loot and every later caller move things
// the same way.
//
// A `looted` beat goes to everyone present, naming looter and body and
// nothing of what moved (P3). Any DOOR_REVEALED the transfer causes follows
// it, recipient-scoped — the verb's own beat precedes its consequences, the
// law stated at [Encounter.refreshSight].
//
// Validation order (R5): nil input → empty member or target → negative Range
// → closed → not a member → target not a member → target not down → not their
// turn in a fight → not in range.
//
// Errors: ErrNilInput, ErrNoMember, ErrClosed, ErrNotMember, ErrNotDown,
// ErrNotActive, ErrOutOfRange, ErrBadPlacement.
func (e *Encounter) Loot(in *LootInput) (*LootOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("loot: %w", ErrNilInput)
	}
	if in.Member == "" || in.Target == "" {
		return nil, fmt.Errorf("loot: %w", ErrNoMember)
	}
	if in.Range < 0 {
		return nil, fmt.Errorf("loot: range %d is negative: %w", in.Range, ErrNoMember)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("loot: %w", ErrClosed)
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("loot: %w", ErrNotMember)
	}
	if _, ok := e.members[in.Target]; !ok {
		return nil, fmt.Errorf("loot: target %q: %w", in.Target, ErrNotMember)
	}

	// ORDINARY REFUSALS, both of them (design §4.2): the body is visible and
	// there is no secret in whether it is down or how far away it is, so
	// these say what they mean. That is the opposite of Hold's prop
	// refusals, and the difference is exactly whether the asker could have
	// learned the answer by looking.
	down, err := e.isDown(in.Target)
	if err != nil {
		return nil, fmt.Errorf("loot: %w", err)
	}
	if !down {
		return nil, fmt.Errorf("loot: target %q: %w", in.Target, ErrNotDown)
	}

	if err := e.refuseOffTurn("loot", in.Member); err != nil {
		return nil, err
	}
	if err := e.refuseOutOfReach("loot", in.Member, in.Target, in.Range); err != nil {
		return nil, err
	}

	at := uint64(e.clock.ToData().HighWater)

	// The `looted` beat FIRST, and it says nothing about what moved: it is
	// byte-identical whether the body carried the run's only secret or
	// nothing at all (P3, pinned by test).
	payload, err := json.Marshal(map[string]interface{}{
		"beat":   "looted",
		"member": string(in.Member),
		"target": string(in.Target),
	})
	if err != nil {
		return nil, fmt.Errorf("loot: marshal beat: %w", err)
	}
	if _, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: e.audienceFor(subjectBeat, in.Member, in.Target),
		Tags:     map[string]string{"tag": "loot"},
		Payload:  payload,
	}); err != nil {
		return nil, fmt.Errorf("loot: append beat: %w", err)
	}

	if err := e.transferHoldings(in.Target, in.Member, "loot", at); err != nil {
		return nil, fmt.Errorf("loot: %w", err)
	}

	return &LootOutput{}, nil
}

// transferHoldings moves EVERY holding of one member to another — the one
// routine a holding changes hands through (design's "one transfer routine"),
// so Loot and every later caller move things identically.
//
// Each kind moves by what that kind means:
//
//   - INTEL is COPIED to the receiver: learnDoor for them, cause "loot", plus
//     their own DOOR_REVEALED beat — the reveal path search already owns
//     (P4), reused rather than duplicated. The body keeps knowing too, so
//     the second player to loot the captain learns the way in exactly as the
//     first did. Knowledge is not an object; see holdings.go on why, and on
//     which shelf turns it into one.
//   - A PROP is MOVED to the receiver. Where it physically is does not
//     change: it is already off the floor (`held`), and passing it from one
//     pair of hands to another moves nothing anybody can see. One of it
//     exists, so the body no longer has it.
//
// A body with nothing transfers nothing and appends nothing — the empty case
// is not a special case, it is the loop running zero times.
//
// INERT INTEL IS INERT, NOT AN ERROR ([MemberInput.Knows]): a door that is
// not concealed, or one the receiver already knows, writes the holding and
// causes no reveal, because there is nothing to reveal. A field with no
// concealment at all has no world, and the same holds for the same reason.
func (e *Encounter) transferHoldings(from, to MemberID, cause string, at uint64) error {
	for _, item := range e.holdings.holdingsOf(from) {
		switch {
		case item.door != "":
			if err := e.holdings.holdIntelDoor(to, item.door, cause); err != nil {
				return fmt.Errorf("transfer intel door %q: %w", item.door, err)
			}
			// Nothing to reveal when: the door is not this field's, the
			// field carries no concealment at all, the door is not
			// concealed, or the receiver already knows it.
			//
			// THE THIRD CLAUSE IS REDUNDANT AND KEPT ON PURPOSE. knowsDoor
			// folds a graph that declares only CONCEALED entities, and
			// graph.State.Visible answers true for anything it was never
			// told about — so an unconcealed door is "already known" to
			// everybody and the fourth clause alone would decide this case.
			// A mutation pass proves it: dropping `d.concealed == nil`
			// kills no test. It stays because removing it would make this
			// rule — "an unconcealed door has nothing to reveal" — true
			// only by an undocumented default of a package one layer down,
			// and the next person to read the graph's contract differently
			// would silently start narrating reveals for open doorways.
			d, ok := e.doorsByID[item.door]
			if !ok || e.world == nil || d.concealed == nil || e.world.knowsDoor(to, d.id) {
				continue
			}
			if err := e.revealDoorTo(to, d, "looted the way to it", at); err != nil {
				return err
			}
		case item.prop != "":
			if err := e.holdings.holdProp(to, item.prop, cause); err != nil {
				return fmt.Errorf("transfer prop %q: %w", item.prop, err)
			}
		}
	}
	return nil
}

// isDown asks the participation capability whether one member is down. The
// SAME question [Encounter.noticeDown] asks, through the same seam — what
// "down" means is the rulebook's and never this composition's guess.
func (e *Encounter) isDown(member MemberID) (bool, error) {
	participation, err := e.participationNow()
	if err != nil {
		return false, err
	}
	return participation.down[member], nil
}

// refuseOffTurn is the in-combat rule (design §4.4), shared by Loot and Hold.
//
// Out of combat both verbs are free. 5e gives one free object interaction a
// turn and charges an action past that, and neither verb joins the [Afford]
// enum with a slot until an acceptance scene takes something mid-fight — so
// until then a member whose fight is on the turn clock may do this ON THEIR
// OWN TURN and not otherwise. Named in the design so the first mid-fight take
// is a ruling rather than a surprise.
//
// This is [Encounter.Step]'s turn gate (rpg-toolkit#1169, ADR-0044) verbatim:
// a member on the world clock has no bubble and passes unconditionally; a
// member in one acts through their own turn or waits.
func (e *Encounter) refuseOffTurn(verb string, member MemberID) error {
	bubble, err := e.bubbleFor(member)
	if err != nil {
		return fmt.Errorf("%s: %w", verb, err)
	}
	if bubble == nil {
		return nil
	}
	active, err := bubble.Active()
	if err != nil {
		return fmt.Errorf("%s: %w", verb, err)
	}
	if MemberID(active) != member {
		return fmt.Errorf("%s: member %q: %w", verb, member, ErrNotActive)
	}
	return nil
}

// refuseOutOfReach refuses a member reaching further than Range cells for
// another member, zero meaning adjacent — [Encounter.Interact]'s reach rule,
// shared so the three verbs that reach for something measure it identically.
func (e *Encounter) refuseOutOfReach(verb string, member, target MemberID, reach int) error {
	from, placed := e.canvas.GetEntityPosition(string(member))
	if !placed {
		return fmt.Errorf("%s: member %q: %w", verb, member, ErrBadPlacement)
	}
	to, placed := e.canvas.GetEntityPosition(string(target))
	if !placed {
		return fmt.Errorf("%s: target %q: %w", verb, target, ErrBadPlacement)
	}
	return e.refuseOutOfReachCell(verb, from, to, reach, string(target))
}

// refuseOutOfReachCell is the measurement itself, against a bare cell — the
// half [Encounter.Hold] needs, since a prop is at a cell rather than being an
// entity with a position.
func (e *Encounter) refuseOutOfReachCell(verb string, from, to spatial.Position, reach int, what string) error {
	if reach == 0 {
		reach = 1
	}
	if e.Distance(from, to) > float64(reach) {
		return fmt.Errorf("%s: %s: %w", verb, what, ErrOutOfRange)
	}
	return nil
}
