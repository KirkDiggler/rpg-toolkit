// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/initiative"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
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
//
// It is built PER VERB, beside that verb's sheets, because the modifier is
// read off the rolling member's sheet at the moment the fight forms — the
// same "sheet facts asked at use time" rule [sheetSeam] follows, and the same
// sheets it reads.
type initiativeSeam struct {
	dice   Roller
	sheets sheetSeam
}

// initiativeFor is the initiative capability for one verb, reading the given
// verb's sheets.
func (m *Manager) initiativeFor(sheets sheetSeam) initiativeSeam {
	return initiativeSeam{dice: m.dice, sheets: sheets}
}

// initiativeContribution is one named thing a member adds to their initiative
// roll, and how much.
type initiativeContribution struct {
	// Source names where the amount comes from, as a player would read it.
	Source string
	// Amount is what it adds; negative subtracts.
	Amount int
}

// initiativeContributions is THE ONE PLACE a member's initiative modifier is
// worked out: every named contribution, read off the member's sheet. The
// modifier is their sum ([initiativeModifier]); a new way to add to
// initiative is one more entry here, never a second path.
//
// Today the list has one entry, the Dexterity modifier, for a player (from its
// character record) and for a monster (from its stat block). A world NPC's
// recorded content states no ability scores, so it contributes nothing — the
// true answer from that content, as [sheetSeam.Sheets] answers zero speed.
//
// A member the verb holds no sheet for is refused by [sheetSeam.sheetOf],
// never answered with zero: zero is a real modifier.
func (s initiativeSeam) initiativeContributions(id encounter.MemberID) ([]initiativeContribution, error) {
	held, err := s.sheets.sheetOf(id)
	if err != nil {
		return nil, err
	}
	var scores shared.AbilityScores
	switch {
	case held.character != nil:
		scores = held.character.AbilityScores
	case held.monster != nil:
		scores = held.monster.AbilityScores
	default:
		return nil, nil
	}
	return []initiativeContribution{{Source: "Dexterity", Amount: scores.Modifier(abilities.DEX)}}, nil
}

// initiativeModifier sums a member's contributions.
func initiativeModifier(contributions []initiativeContribution) int {
	total := 0
	for _, c := range contributions {
		total += c.Amount
	}
	return total
}

// RollInitiative asks the rulebook to order the members a fight is starting
// between, using the host's dice.
//
// It asks ONE MEMBER AT A TIME, in sorted order, and that loop is the point of
// this function rather than an accident of it. dnd5e/initiative.RollForOrder
// takes a map and iterates it, so handing it the whole fight would draw the
// rolls in a random order and assign them to whoever came up first — two
// replays of the same session, with the same dice, ordering the fight
// differently. Its own source says so (TODO #285). Asking per member keeps the
// rule where it belongs (the d20 and the modifier are still the rulebook's) and
// puts the only non-reproducible part, the ORDER OF THE ASKING, under this
// package's control.
//
// The result is then ordered by total, and equal totals by ID. Ordering equal
// rolls is not something 5e states — it leaves ties to the table — so choosing
// one is a reproducibility decision (C8: identical inputs, identical outputs),
// made in the open rather than left to map iteration. Flat rolls make ties
// common rather than rare, which is what makes it load-bearing today.
//
// EACH MEMBER ADDS ITS MODIFIER, worked out by [initiativeContributions] from
// the sheet this verb holds (rpg-project#990): today the Dexterity modifier
// and nothing else. Neither the host's interface nor the composition's
// changed to carry it; the composition still hands over IDs only, and the
// order is still all it gets back. Ties are NOT broken by Dexterity: equal
// totals order by ID, as before.
//
// A dice failure ABORTS, and has to be caught rather than returned: RollForOrder
// discards the roller's error (`roll, _ := roller.Roll(...)`) and would hand
// back a member who rolled zero. A fight that cannot be ordered must not
// half-start, so the seam remembers what the rulebook dropped.
func (s initiativeSeam) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	asking := append([]encounter.MemberID(nil), members...)
	sort.Slice(asking, func(i, j int) bool { return asking[i] < asking[j] })

	dice := &diceSeam{roller: s.dice}
	rolls := make([]initiative.Roll, 0, len(asking))
	for _, id := range asking {
		contributions, err := s.initiativeContributions(id)
		if err != nil {
			return nil, fmt.Errorf("initiative modifier for %q: %w", id, err)
		}
		one := initiative.RollForOrder(map[core.Entity]int{memberEntity(id): initiativeModifier(contributions)}, dice)
		if dice.err != nil {
			return nil, fmt.Errorf("rolling initiative for %q: %w", id, dice.err)
		}
		rolls = append(rolls, one...)
	}

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

// diceSeam adapts the host's Roller to the one the rulebook takes, and
// remembers the error the rulebook throws away.
//
// The remembering is not defensive tidiness. RollForOrder discards its roller's
// error, so a host whose randomness failed would silently contribute a member
// who rolled zero — a wrong fight rather than a refused one. The first failure
// is kept and the caller checks it.
//
// RollN is a loop because the rule only ever rolls a single d20; implementing
// it by delegation keeps the host's side to the one method it genuinely owns.
type diceSeam struct {
	roller Roller
	err    error
}

func (d *diceSeam) Roll(ctx context.Context, size int) (int, error) {
	roll, err := d.roller.Roll(ctx, size)
	if err != nil && d.err == nil {
		d.err = err
	}
	return roll, err
}

func (d *diceSeam) RollN(ctx context.Context, count, size int) ([]int, error) {
	out := make([]int, 0, count)
	for i := 0; i < count; i++ {
		roll, err := d.Roll(ctx, size)
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
	_ dice.Roller                = &diceSeam{}
	_ core.Entity                = memberEntity("")
)
