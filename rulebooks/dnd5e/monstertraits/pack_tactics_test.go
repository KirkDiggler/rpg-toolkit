// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type PackTacticsTestSuite struct {
	suite.Suite
	bus         events.EventBus
	ctx         context.Context
	packTactics *packTacticsCondition
}

func TestPackTacticsTestSuite(t *testing.T) {
	suite.Run(t, new(PackTacticsTestSuite))
}

func (s *PackTacticsTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.ctx = context.Background()
	s.packTactics = PackTactics("wolf-1").(*packTacticsCondition)
	s.Require().NoError(s.packTactics.Apply(s.ctx, s.bus))
}

// beside is one member standing distance cells from pc-1, with the stance
// wolf-1 holds toward it.
type beside struct {
	id       string
	distance contributions.Fact[float64]
	stance   contributions.Fact[contributions.Stance]
}

// packFrame is the attack-roll frame resolution builds for wolf-1's attack on
// pc-1: complete, with each listed member's pair to the target and wolf-1's
// stance toward it.
func packFrame(members ...beside) contributions.Frame {
	frame := contributions.Frame{
		Actor:    "wolf-1",
		Target:   contributions.Known("pc-1"),
		Action:   contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack), Melee: contributions.Known(true)},
		Complete: true,
		Pairs: []contributions.PairFacts{{
			From: "wolf-1", To: "pc-1", DistanceCells: contributions.Known(4.0),
			Stance: contributions.Known(contributions.StanceHostile),
		}},
	}
	for _, m := range members {
		frame.Pairs = append(frame.Pairs,
			contributions.PairFacts{From: m.id, To: "pc-1", DistanceCells: m.distance},
			contributions.PairFacts{From: "wolf-1", To: m.id, Stance: m.stance},
		)
	}
	return frame
}

// swing folds one attack by attacker against pc-1 under frame.
func (s *PackTacticsTestSuite) swing(attacker string, frame contributions.Frame) (dnd5eEvents.AttackChainEvent, error) {
	s.T().Helper()
	event := dnd5eEvents.AttackChainEvent{
		AttackerID: attacker, TargetID: "pc-1", AttackBonus: 4, TargetAC: 15, Frame: frame,
	}
	c := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	modifiedChain, err := dnd5eEvents.AttackChain.On(s.bus).PublishWithChain(s.ctx, event, c)
	if err != nil {
		return event, err
	}
	return modifiedChain.Execute(s.ctx, event)
}

func allied() contributions.Fact[contributions.Stance] {
	return contributions.Known(contributions.StanceAllied)
}

func (s *PackTacticsTestSuite) TestPackTacticsGrantsAdvantage() {
	result, err := s.swing("wolf-1", packFrame(beside{"wolf-2", contributions.Known(1.0), allied()}))
	s.Require().NoError(err)

	s.Require().Len(result.AdvantageSources, 1, "a packmate beside the target grants advantage")
	s.Equal(refs.MonsterTraits.PackTactics().String(), result.AdvantageSources[0].SourceRef.String())
	s.Equal("wolf-1", result.AdvantageSources[0].SourceID)
}

// TestPackTacticsWithoutAnAllyAdjacent — the target's own ally beside it is
// not the wolf's packmate.
func (s *PackTacticsTestSuite) TestPackTacticsWithoutAnAllyAdjacent() {
	result, err := s.swing("wolf-1", packFrame(
		beside{"pc-2", contributions.Known(1.0), contributions.Known(contributions.StanceHostile)},
	))
	s.Require().NoError(err)
	s.Empty(result.AdvantageSources)
}

// TestPackTacticsDoesNotCountANeutralOrUnsidedMember is the case a two-sided
// model gets wrong: a neutral faction, or a member with no side at all, beside
// the target is "not my enemy" yet grants the wolf nothing.
func (s *PackTacticsTestSuite) TestPackTacticsDoesNotCountANeutralOrUnsidedMember() {
	for _, stance := range []contributions.Stance{contributions.StanceNeutral, contributions.StanceNone} {
		result, err := s.swing("wolf-1", packFrame(beside{"bystander", contributions.Known(1.0), contributions.Known(stance)}))
		s.Require().NoError(err, stance)
		s.Empty(result.AdvantageSources, stance)
	}
}

// TestPackTacticsReachIsFiveFeet: a packmate 1.4 cells (seven feet) from the
// target is not within five feet; one at exactly one cell is.
func (s *PackTacticsTestSuite) TestPackTacticsReachIsFiveFeet() {
	result, err := s.swing("wolf-1", packFrame(beside{"wolf-2", contributions.Known(1.4), allied()}))
	s.Require().NoError(err)
	s.Empty(result.AdvantageSources, "a packmate seven feet away is not within five feet")

	result, err = s.swing("wolf-1", packFrame(beside{"wolf-2", contributions.Known(combat.AdjacentCells), allied()}))
	s.Require().NoError(err)
	s.NotEmpty(result.AdvantageSources)
}

// TestPackTacticsFailsWhenTheFrameCannotAnswer is R13: an unknown stance or
// distance beside the target, or a frame whose pairs are not complete, fails
// the attack instead of reading as "no packmate".
func (s *PackTacticsTestSuite) TestPackTacticsFailsWhenTheFrameCannotAnswer() {
	s.Run("unknown stance", func() {
		_, err := s.swing("wolf-1", packFrame(beside{"wolf-2", contributions.Known(1.0), contributions.Unknown[contributions.Stance]()}))
		s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
	})
	s.Run("unknown distance to an ally", func() {
		_, err := s.swing("wolf-1", packFrame(beside{"wolf-2", contributions.Unknown[float64](), allied()}))
		s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
	})
	s.Run("incomplete pairs", func() {
		frame := packFrame()
		frame.Complete = false
		_, err := s.swing("wolf-1", frame)
		s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
	})
	s.Run("invalid frame", func() {
		_, err := s.swing("wolf-1", contributions.Frame{})
		s.ErrorIs(err, contributions.ErrRuleCannotAnswer)
	})
}

// TestPackTacticsIgnoresOtherAttackers: another member's attack is not this
// trait's, even with a packmate beside the target and a frame that would
// fail its own question.
func (s *PackTacticsTestSuite) TestPackTacticsIgnoresOtherAttackers() {
	result, err := s.swing("wolf-2", contributions.Frame{})
	s.Require().NoError(err)
	s.Empty(result.AdvantageSources)
}

func (s *PackTacticsTestSuite) TestPackTacticsCanBeRemoved() {
	s.True(s.packTactics.IsApplied())
	s.Require().NoError(s.packTactics.Remove(s.ctx, s.bus))
	s.False(s.packTactics.IsApplied())
}
