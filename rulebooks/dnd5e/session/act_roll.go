// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// act_roll.go is R17's ROLL: every die an act carries, said one way.
//
// A roll is built from the beat's own `calculation` — the sourced dice and
// modifiers the rulebook produced — through [decodeRollCalculation], the one
// decoder this package keeps for it. Nothing is re-added: Total is the
// calculation's authoritative total, and the faces are the ones it kept.

// Roll is one roll as an act states it.
type Roll struct {
	// Expression is the roll as dice and a flat modifier, "1d20+5", built
	// from the calculation's kept dice and its modifiers. "" when the beat
	// carried no calculation to build it from.
	Expression string

	// Dice are the faces that counted, in calculation order. A die a keep
	// rule discarded (the low face of advantage) is not here.
	Dice []int

	// Modifiers are the flat contributions, each named by its source.
	Modifiers []RollModifier

	// Total is the roll's result.
	Total int

	// Against is the number the roll was made against — a DC or an AC. Nil
	// for an unopposed roll: damage, healing, a table die.
	Against *int

	// Beaten is whether the roll beat Against, as the beat says it. Nil when
	// Against is nil.
	Beaten *bool

	// Of is the die size of a table roll, a roll of N on a table of N
	// entries. 0 otherwise.
	Of int

	// Influences are the advantage and disadvantage that shaped the roll,
	// each with its source, in the order the beat recorded them (R17: a roll
	// carries its sources). Read from the calculation's dice keep (Granted is
	// advantage, Imposed is disadvantage); a swing's own
	// `advantage_sources`/`disadvantage_sources` keys take precedence
	// (attackPiece). Empty when nothing granted or imposed either.
	Influences []RollInfluence
}

// InfluenceKind is whether a source granted advantage or imposed
// disadvantage.
type InfluenceKind string

const (
	// InfluenceAdvantage is a source that granted advantage.
	InfluenceAdvantage InfluenceKind = "advantage"
	// InfluenceDisadvantage is a source that imposed disadvantage.
	InfluenceDisadvantage InfluenceKind = "disadvantage"
)

// RollInfluence is one source of advantage or disadvantage on a roll.
type RollInfluence struct {
	Kind InfluenceKind
	// SourceRef is the source's full ref, verbatim.
	SourceRef string
	// SourceID is the entity the source belongs to, when one was named.
	SourceID string
}

// RollModifier is one flat contribution to a roll.
type RollModifier struct {
	// Source is the contribution's source name ("Strength", "Proficiency").
	Source string
	// Value is what it added.
	Value int
}

// errCalculation is a calculation present on a beat that does not replay.
var errCalculation = errors.New("calculation does not replay")

// rollOf builds a Roll from a beat's calculation, or from its bare faces and
// total when it carried none (a beat written before roll traces). against
// and beaten come from the beat; either may be nil.
func rollOf(calc json.RawMessage, faces []int, total int, against *int, beaten *bool) (*Roll, error) {
	roll := &Roll{Total: total, Against: against, Beaten: beaten}
	if against == nil {
		roll.Beaten = nil
	}
	if len(calc) == 0 || string(calc) == "null" {
		roll.Dice = faces
		return roll, nil
	}
	decoded, ok := decodeRollCalculation(calc)
	if !ok {
		return nil, errCalculation
	}
	roll.Total = decoded.Total
	var dice []string
	sum, mods := 0, false
	for _, c := range decoded.Components {
		if c.Dice != nil {
			kept := c.Dice.FinalRolls
			if len(c.Dice.KeptIndices) > 0 {
				kept = make([]int, 0, len(c.Dice.KeptIndices))
				for _, i := range c.Dice.KeptIndices {
					kept = append(kept, c.Dice.FinalRolls[i])
				}
			}
			roll.Dice = append(roll.Dice, kept...)
			term := fmt.Sprintf("%dd%d", len(kept), c.Dice.DieSize)
			switch {
			case c.SubtractDice:
				term = "-" + term
			case len(dice) > 0:
				term = "+" + term
			}
			dice = append(dice, term)
		}
		if c.Dice != nil && c.Dice.Keep != nil {
			for _, g := range c.Dice.Keep.Granted {
				roll.Influences = append(roll.Influences, RollInfluence{Kind: InfluenceAdvantage, SourceRef: g.Ref, SourceID: g.SourceID})
			}
			for _, g := range c.Dice.Keep.Imposed {
				roll.Influences = append(roll.Influences, RollInfluence{Kind: InfluenceDisadvantage, SourceRef: g.Ref, SourceID: g.SourceID})
			}
		}
		if c.Modifier != nil {
			mods = true
			sum += *c.Modifier
			roll.Modifiers = append(roll.Modifiers, RollModifier{Source: c.Source.Name, Value: *c.Modifier})
		}
	}
	expression := strings.Join(dice, "")
	if mods {
		if sum >= 0 && expression != "" {
			expression += "+"
		}
		expression += strconv.Itoa(sum)
	}
	roll.Expression = expression
	return roll, nil
}

// intp is a pointer to v, for a roll's Against.
func intp(v int) *int { return &v }

// boolp is a pointer to v, for a roll's Beaten.
func boolp(v bool) *bool { return &v }

// facesOf is a lone face as a dice list, nil for no face.
func facesOf(face int) []int {
	if face == 0 {
		return nil
	}
	return []int{face}
}
