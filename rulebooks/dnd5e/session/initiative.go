// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/initiative"
)

// Roller is the host's source of randomness. Required.
//
// THE HOST SUPPLIES ENTROPY, NEVER ORDERING. That division is the whole point
// of this seam, and getting it backwards is subtle: a fight now starts on its
// own (rpg-toolkit#964), so somebody must answer "in what order?" the moment
// contact happens — and if what the host implemented were the ORDER, every
// host would end up writing d20-against-Dexterity with a tie rule. That is a
// game rule, sitting one seam outside the package whose charter is to hold
// none, which is no better than holding it here.
//
// So the host is asked for the one thing it legitimately owns: where random
// numbers come from. A server wires its seeded or crypto source; a test wires
// a fixed one and gets a reproducible fight. The rule that turns those numbers
// into an order is the RULEBOOK's, and this package only routes between them.
//
// It is an interface over primitives rather than the toolkit's own roller so
// that a host implementing it never names a toolkit type (S2) — the same
// reason every other capability here is declared in this package's vocabulary.
// It follows the shape sketched for SessionLocker in repositories.go: a
// capability the host provides, not a mechanism the host reimplements.
type Roller interface {
	// Roll returns a random number from 1 to size inclusive, or an error.
	Roll(ctx context.Context, size int) (int, error)
}

// initiativeSeam turns the host's dice into the composition's initiative
// order, by way of the rulebook's rule.
//
// Unexported for the reason the boundary test exists: if the host had to
// satisfy encounter.InitiativeRoller directly, replacing the composition would
// break every host that implemented it.
type initiativeSeam struct {
	dice Roller
}

// RollInitiative asks the rulebook to order the members a fight is starting
// between, using the host's dice.
//
// TWO KNOWN GAPS, both deliberate and both about production rather than shape.
//
// FLAT ROLLS, NO MODIFIERS. Every member is passed a Dexterity modifier of
// zero, because a member ID is all the composition hands over at formation and
// resolving each one to a sheet mid-verb is its own piece of work. The shape is
// what matters now: when the modifier arrives it is looked up HERE, and neither
// the host's interface nor the composition's changes.
//
// DETERMINISTIC TIES. RollForOrder iterates a map and sorts unstably, so two
// members on the same total come back in whatever order Go felt like — its own
// source says so (TODO #285). Flat rolls make ties common rather than rare, and
// this module promises identical inputs yield identical outputs (C8): a session
// reloaded and re-run must produce the same fight. So the rulebook's answer is
// re-sorted here by total, then by ID. Ordering equal rolls is not a rule 5e
// states — it leaves ties to the table — so choosing one is a reproducibility
// decision, and it is made in the open rather than left to map iteration.
func (s initiativeSeam) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	entities := make(map[core.Entity]int, len(members))
	for _, id := range members {
		entities[memberEntity(id)] = 0
	}

	rolls := initiative.RollForOrder(entities, diceSeam{roller: s.dice})

	sort.SliceStable(rolls, func(i, j int) bool {
		if rolls[i].Total != rolls[j].Total {
			return rolls[i].Total > rolls[j].Total
		}
		return rolls[i].Entity.GetID() < rolls[j].Entity.GetID()
	})

	order := make([]encounter.MemberID, 0, len(rolls))
	for _, r := range rolls {
		order = append(order, core.EntityID(r.Entity.GetID()))
	}
	return order, nil
}

// memberEntity is a member ID wearing the entity interface the rulebook's
// initiative rule asks for.
//
// The rule wants entities because it was written for a stack that had them in
// hand; all this seam has at formation time is an ID, and an ID is all the
// ordering actually uses. Widening the rulebook's signature is a change in
// another module and is not this wave's.
type memberEntity string

func (m memberEntity) GetID() string          { return string(m) }
func (memberEntity) GetType() core.EntityType { return "member" }

// diceSeam adapts the host's Roller to the one the rulebook takes.
//
// RollN is a loop because the rule only ever rolls a single d20; implementing
// it by delegation keeps the host's side to the one method it genuinely owns.
type diceSeam struct {
	roller Roller
}

func (d diceSeam) Roll(ctx context.Context, size int) (int, error) {
	return d.roller.Roll(ctx, size)
}

func (d diceSeam) RollN(ctx context.Context, count, size int) ([]int, error) {
	out := make([]int, 0, count)
	for i := 0; i < count; i++ {
		roll, err := d.roller.Roll(ctx, size)
		if err != nil {
			return nil, err
		}
		out = append(out, roll)
	}
	return out, nil
}

// compile-time proof the adapters satisfy what they are handed to.
var (
	_ encounter.InitiativeRoller = initiativeSeam{}
	_ dice.Roller                = diceSeam{}
	_ core.Entity                = memberEntity("")
)
