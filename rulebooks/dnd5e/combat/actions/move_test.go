// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// MoveSuite covers the declaration a cast makes when it moves a creature that
// did not choose to move.
type MoveSuite struct {
	suite.Suite
}

func TestMoveSuite(t *testing.T) {
	suite.Run(t, new(MoveSuite))
}

// thunderwavePush is the least permissive directive there is, and every field
// that makes it so is a zero value.
func thunderwavePush() *actions.CastMove {
	return &actions.CastMove{Policy: actions.MoveLine, Cells: 2}
}

// TestThePushIsTheZeroValue — Thunderwave's shove pays nothing and provokes
// nothing, and those are the defaults rather than choices content had to make.
//
// Worth pinning because it is the direction the defaults must lean. A move
// that provoked by default would give every future spell an opportunity attack
// its author never wrote, and a move that spent a reaction by default would
// silently debit an economy nobody asked it to touch.
func (s *MoveSuite) TestThePushIsTheZeroValue() {
	move := thunderwavePush()
	s.Require().NoError(move.Validate())
	s.Equal(actions.PaysNothing, move.Pays, "a push costs the creature nothing")
	s.False(move.Provokes, "and nobody's reaction fires as it slides")
}

// TestAMoveDeclaresExactlyOneBudget catches both halves of the zero value that
// would lie here: a directive with no budget at all, which would move a
// creature zero cells and record that it moved, and one that declares both a
// fixed count and the mover's own speed, which is two answers to one question.
func (s *MoveSuite) TestAMoveDeclaresExactlyOneBudget() {
	s.Run("a fixed count is a budget", func() {
		s.NoError((&actions.CastMove{Policy: actions.MoveLine, Cells: 2}).Validate())
	})
	s.Run("the mover's own speed is a budget", func() {
		s.NoError((&actions.CastMove{Policy: actions.MoveLine, Speed: true}).Validate())
	})
	s.Run("neither is refused", func() {
		err := (&actions.CastMove{Policy: actions.MoveLine}).Validate()
		s.Require().Error(err)
		s.Contains(err.Error(), "budget")
	})
	s.Run("both is refused", func() {
		err := (&actions.CastMove{Policy: actions.MoveLine, Cells: 2, Speed: true}).Validate()
		s.Require().Error(err)
		s.Contains(err.Error(), "budget")
	})
	s.Run("a negative count is refused", func() {
		err := (&actions.CastMove{Policy: actions.MoveLine, Cells: -1}).Validate()
		s.Require().Error(err)
		s.Contains(err.Error(), "negative",
			"a typo is reported as a typo, not as a budget nobody declared")
	})
}

// TestTheVocabularyIsClosed — an unknown policy or an unknown price is refused
// here rather than reaching the engine that would have to guess at it.
func (s *MoveSuite) TestTheVocabularyIsClosed() {
	s.Run("an unknown policy is refused", func() {
		err := (&actions.CastMove{Policy: "sideways", Cells: 2}).Validate()
		s.Require().Error(err)
		s.Contains(err.Error(), "unknown move policy")
	})
	s.Run("an unknown price is refused", func() {
		err := (&actions.CastMove{Policy: actions.MoveLine, Cells: 2, Pays: "a favour"}).Validate()
		s.Require().Error(err)
		s.Contains(err.Error(), "unknown move price")
	})
	s.Run("every declared policy is known", func() {
		for _, policy := range []actions.MovePolicy{actions.MoveLine, actions.MoveAway, actions.MoveToward} {
			s.NoError((&actions.CastMove{Policy: policy, Cells: 2}).Validate(), "%s", policy)
		}
	})
}

// moveProfile is a gated cast that also shoves, the shape Thunderwave takes.
func moveProfile(mutate func(p *actions.CastProfile)) actions.CastProfile {
	p := areaProfile(func(p *actions.CastProfile) {
		p.RangeFeet = 15
		p.Area = &actions.CastArea{Footprint: thunderwaveShape(), Catches: actions.AreaCatchesOthers}
		p.Save = &saves.SaveGate{
			Abilities:  []abilities.Ability{abilities.CON},
			DC:         saves.DCStatic(13),
			OnSuccess:  saves.Negated,
			Recurrence: saves.RecurrenceNone,
		}
		p.Move = thunderwavePush()
	})
	if mutate != nil {
		mutate(&p)
	}
	return p
}

// TestAMoveImposedOnAFailedSaveNeedsASave is the binding between the two
// halves, in the direction that can actually go wrong.
//
// The move is what a FAILURE costs. A profile declaring one with no gate has
// no failure to hang it on, so whoever resolved it would have to invent the
// moment — either shoving on every cast or never shoving at all, both of them
// rules nobody wrote.
func (s *MoveSuite) TestAMoveImposedOnAFailedSaveNeedsASave() {
	s.Run("a gated cast may shove", func() {
		s.NoError(moveProfile(nil).Validate())
	})
	s.Run("an ungated cast may not", func() {
		err := moveProfile(func(p *actions.CastProfile) { p.Save = nil }).Validate()
		s.Require().Error(err)
		s.Contains(err.Error(), "save")
	})
	s.Run("an invalid move is refused through the profile", func() {
		err := moveProfile(func(p *actions.CastProfile) { p.Move = &actions.CastMove{Policy: actions.MoveLine} }).Validate()
		s.Require().Error(err)
		s.Contains(err.Error(), "budget")
	})
	s.Run("a cast that shoves nobody is still valid", func() {
		s.NoError(moveProfile(func(p *actions.CastProfile) { p.Move = nil }).Validate())
	})
}

// TestCloningCarriesTheMoveSomewhereElse — Clone exists so a running
// interaction cannot be rewritten by a caller reusing its definition, and a
// move left sharing a pointer would be exactly that hole.
func (s *MoveSuite) TestCloningCarriesTheMoveSomewhereElse() {
	original := moveProfile(nil)
	clone := original.Clone()

	s.Require().NotNil(clone.Move)
	s.Equal(*original.Move, *clone.Move)
	s.NotSame(original.Move, clone.Move, "the clone must not share the original's move")

	clone.Move.Cells = 30
	s.Equal(2, original.Move.Cells, "editing a clone must not reach the original")
}
