// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package healing_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

type HealingSuite struct{ suite.Suite }

func TestHealingSuite(t *testing.T) { suite.Run(t, new(HealingSuite)) }

type fixedRoller struct {
	face  int
	calls int
	err   error
}

func (r *fixedRoller) Roll(context.Context, int) (int, error) { r.calls++; return r.face, r.err }
func (r *fixedRoller) RollN(_ context.Context, n, _ int) ([]int, error) {
	r.calls++
	out := make([]int, n)
	for i := range out {
		out[i] = r.face
	}
	return out, r.err
}

func source() events.RollSource {
	ref := *refs.Spells.CureWounds()
	return events.RollSource{Ref: &ref, Name: "Cure Wounds", SourceID: "cleric"}
}

func (s *HealingSuite) TestSourcesAndNonnegativeTotal() {
	for _, modifier := range []int{3, 0, -5} {
		d := healing.Declaration{Dice: "1d8", Modifiers: []healing.Modifier{{Source: events.RollSource{Ref: refs.Abilities.Wisdom(), Name: "Wisdom"}, Amount: modifier}}}
		roll := &fixedRoller{face: 1}
		result, err := healing.Resolve(context.Background(), d, source(), roll)
		s.Require().NoError(err)
		s.Equal(max(0, 1+modifier), result.Total)
		s.Equal(1, roll.calls)
		s.NoError(events.ValidateRollCalculation(result))
		s.Equal(modifier, *result.Components[1].Modifier)
		result.Components[1].Source.Ref.ID = "changed"
		s.Equal("wis", d.Modifiers[0].Source.Ref.ID)
	}
}

func (s *HealingSuite) TestInvalidDeclarationDoesNotRollAndBadDiceCannotBecomeHealing() {
	for _, notation := range []string{"", "-1d8", "1d8+3", "0d8", "1d0"} {
		roll := &fixedRoller{face: 5}
		_, err := healing.Resolve(context.Background(), healing.Declaration{Dice: notation}, source(), roll)
		s.Error(err)
		s.Zero(roll.calls)
	}
	for _, roll := range []*fixedRoller{{face: 0}, {face: 9}, {err: errors.New("dice failed")}} {
		result, err := healing.Resolve(context.Background(), healing.Declaration{Dice: "1d8"}, source(), roll)
		s.Error(err)
		s.Nil(result)
	}
}

func (s *HealingSuite) TestDiscipleOfLifeIsNotUniversalHealing() {
	s.Empty(features.DiscipleOfLife(healing.Context{}, "cleric"))
	s.Empty(features.DiscipleOfLife(healing.Context{Spell: true}, "cleric"))
	s.Empty(features.DiscipleOfLife(healing.Context{SpellLevel: 1}, "cleric"))
	for _, level := range []int{1, 3} {
		bonus := features.DiscipleOfLife(healing.Context{Spell: true, SpellLevel: level}, "cleric")
		s.Require().Len(bonus, 1)
		s.Equal(2+level, bonus[0].Amount)
		s.Equal("cleric", bonus[0].Source.SourceID)
		s.Equal(refs.Features.DiscipleOfLife(), bonus[0].Source.Ref)
	}
}
