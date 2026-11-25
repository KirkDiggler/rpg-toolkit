// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/initiative"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
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
// read off the rolling member's sheet at the moment the fight forms or is joined — the
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

// initiativeModifiers is what a member's own sheet answers to
// [character.Character.Initiative] or [monster.Monster.Initiative], as the
// rulebook's roll takes it: one modifier per contribution, in the sheet's
// order. The session names no source and sums nothing itself; what adds to
// initiative (Dexterity by rule, a held status's bonus) is the rulebook's,
// read off the sheet at the moment the fight forms or is joined.
//
// A monster is loaded with its carried conditions prepared, so a status it
// holds is counted. A world NPC's recorded content has no sheet to ask, so it
// adds nothing — the true answer from that content, as [sheetSeam.Sheets]
// answers zero speed. A member the verb holds no sheet for is refused by
// [sheetSeam.sheetOf], never answered with no modifiers: zero is a real
// modifier.
func (s initiativeSeam) initiativeModifiers(id encounter.MemberID) ([]initiative.Modifier, error) {
	held, err := s.sheets.sheetOf(id)
	if err != nil {
		return nil, err
	}
	switch {
	case held.character != nil:
		sheet, err := character.Load(s.sheets.ctx, held.character)
		if err != nil {
			return nil, err
		}
		answer, err := sheet.Initiative()
		if err != nil {
			return nil, err
		}
		return modifiersFor(answer.Contributions), nil
	case held.monster != nil:
		sheet, err := monstertraits.LoadMonster(s.sheets.ctx, held.monster)
		if err != nil {
			return nil, err
		}
		answer, err := sheet.Initiative()
		if err != nil {
			return nil, err
		}
		return modifiersFor(answer.Contributions), nil
	default:
		return nil, nil
	}
}

// modifiersFor turns a sheet's initiative contributions into the modifiers the
// rulebook's roll takes, one each, in order.
func modifiersFor(contributions []conditions.InitiativeContribution) []initiative.Modifier {
	modifiers := make([]initiative.Modifier, 0, len(contributions))
	for _, c := range contributions {
		modifiers = append(modifiers, initiative.Modifier{
			Source: dnd5eEvents.RollSource{Ref: c.Ref, Name: c.Source},
			Amount: c.Amount,
		})
	}
	return modifiers
}

// RollInitiative rolls each member's initiative with the host's dice and
// hands every roll back to the composition, which records it and orders the
// fight by it.
//
// It asks ONE MEMBER AT A TIME, in sorted ID order, and that loop is the point
// of this function rather than an accident of it. Handing the rulebook the
// whole fight as a map would draw the rolls in map-iteration order and assign
// them to whoever came up first: two replays of the same session, with the
// same dice, ordering the fight differently. Asking per member keeps the rule
// where it belongs (the d20 and the modifiers are the rulebook's, through
// [initiative.RollOne], its one d20 entry) and puts the only non-reproducible
// part, the ORDER OF THE ASKING, under this package's control.
//
// EACH MEMBER ADDS WHAT ITS OWN SHEET ANSWERS ([initiativeSeam.initiativeModifiers],
// rpg-project#990): every initiative effect it has, by rule (Dexterity) and
// held (a status's bonus), one modifier per contribution, in the sheet's
// order. The rulebook builds the calculation from them and the session carries
// it as is, so the roll everyone reads is the roll the rulebook made.
//
// The result is then ordered by total, and equal totals by ID. Ordering equal
// rolls is not something 5e states — it leaves ties to the table — so choosing
// one is a reproducibility decision (C8: identical inputs, identical outputs),
// made in the open. Ties are NOT broken by Dexterity.
//
// A dice failure ABORTS: the rulebook returns the roller's error itself, and a
// fight that cannot be ordered must not half-start.
func (s initiativeSeam) RollInitiative(members []encounter.MemberID) ([]encounter.InitiativeRoll, error) {
	asking := append([]encounter.MemberID(nil), members...)
	sort.Slice(asking, func(i, j int) bool { return asking[i] < asking[j] })

	rolls := make([]encounter.InitiativeRoll, 0, len(asking))
	for _, id := range asking {
		modifiers, err := s.initiativeModifiers(id)
		if err != nil {
			return nil, fmt.Errorf("initiative modifier for %q: %w", id, err)
		}
		out, err := initiative.RollOne(s.sheets.ctx, &initiative.RollOneInput{
			MemberID:  string(id),
			Modifiers: modifiers,
			Roller:    &diceSeam{roller: s.dice},
		})
		if err != nil {
			return nil, fmt.Errorf("rolling initiative for %q: %w", id, err)
		}
		rolls = append(rolls, encounter.InitiativeRoll{
			Member: id, Roll: out.Face, Total: out.Total, Calculation: rollCalculationFor(out.Calculation),
		})
	}

	sort.SliceStable(rolls, func(i, j int) bool {
		if rolls[i].Total != rolls[j].Total {
			return rolls[i].Total > rolls[j].Total
		}
		return rolls[i].Member < rolls[j].Member
	})
	return rolls, nil
}

// diceSeam adapts the host's Roller to the one the rulebook takes, and keeps
// the first error the host's dice returned.
//
// The rulebook's [initiative.RollOne] returns a failed die itself, so the
// session no longer depends on the kept error to refuse a fight; it stays as a
// record of what the host's dice said.
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
)
