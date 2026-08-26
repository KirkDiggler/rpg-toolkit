// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

type PackTacticsTestSuite struct {
	suite.Suite
	bus         events.EventBus
	ctx         context.Context
	room        spatial.Room
	packTactics *packTacticsCondition
}

func TestPackTacticsTestSuite(t *testing.T) {
	suite.Run(t, new(PackTacticsTestSuite))
}

func (s *PackTacticsTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.ctx = context.Background()
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "dungeon",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 20, Height: 20}),
	})
	s.packTactics = nil // Will be created in each test
}

// place puts a body on the grid. Which side it is on is a separate matter,
// declared to the fakeCast in each test.
func (s *PackTacticsTestSuite) place(id string, x, y float64) *fakeEntity {
	s.T().Helper()
	e := &fakeEntity{id: id}
	s.Require().NoError(s.room.PlaceEntity(e, spatial.Position{X: x, Y: y}))
	return e
}

// swing folds one attack by wolf-1 against the named target and returns it.
func (s *PackTacticsTestSuite) swing(ctx context.Context, targetID string) dnd5eEvents.AttackChainEvent {
	s.T().Helper()
	event := dnd5eEvents.AttackChainEvent{
		AttackerID:  "wolf-1",
		TargetID:    targetID,
		IsMelee:     true,
		AttackBonus: 4,
		TargetAC:    15,
	}
	c := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	modifiedChain, err := dnd5eEvents.AttackChain.On(s.bus).PublishWithChain(ctx, event, c)
	s.Require().NoError(err)
	result, err := modifiedChain.Execute(ctx, event)
	s.Require().NoError(err)
	return result
}

// TestPackTacticsGrantsAdvantage now actually tests advantage.
//
// It used to assert only that the attacker and target IDs survived the fold,
// with a comment saying the real ally check "would be handled by the game
// server before publishing the attack event". No game server ever did, and
// nothing could have: the trait's whole body was commented out because there
// was no way for it to ask who anybody was to anybody else.
func (s *PackTacticsTestSuite) TestPackTacticsGrantsAdvantage() {
	s.place("wolf-1", 1, 1)
	s.place("wolf-2", 5, 6) // packmate, adjacent to the target
	s.place("pc-1", 5, 5)

	ctx := gamectx.WithRoom(s.ctx, s.room)
	ctx = gamectx.WithCast(ctx, &fakeCast{side: map[string]string{
		"wolf-1": "wolves", "wolf-2": "wolves", "pc-1": "party",
	}})

	s.packTactics = PackTactics("wolf-1").(*packTacticsCondition)
	s.Require().NoError(s.packTactics.Apply(ctx, s.bus))

	result := s.swing(ctx, "pc-1")

	s.Require().Len(result.AdvantageSources, 1, "a packmate beside the target grants advantage")
	s.Equal(refs.MonsterTraits.PackTactics().String(), result.AdvantageSources[0].SourceRef.String())
	s.Equal("wolf-1", result.AdvantageSources[0].SourceID)
}

// TestPackTacticsWithoutAnAllyAdjacent — a lone wolf gets nothing.
func (s *PackTacticsTestSuite) TestPackTacticsWithoutAnAllyAdjacent() {
	s.place("wolf-1", 1, 1)
	s.place("pc-1", 5, 5)
	s.place("pc-2", 5, 6) // adjacent to the target, but not the wolf's ally

	ctx := gamectx.WithRoom(s.ctx, s.room)
	ctx = gamectx.WithCast(ctx, &fakeCast{side: map[string]string{
		"wolf-1": "wolves", "pc-1": "party", "pc-2": "party",
	}})

	s.packTactics = PackTactics("wolf-1").(*packTacticsCondition)
	s.Require().NoError(s.packTactics.Apply(ctx, s.bus))

	result := s.swing(ctx, "pc-1")
	s.Empty(result.AdvantageSources, "the target's own ally is not the wolf's packmate")
}

// TestPackTacticsDoesNotCountARivalFaction is the case a two-sided model gets
// wrong, and the reason this asks IsAllied instead of !IsHostile.
//
// A hobgoblin stands beside the wolf's target. It is not the party, so any
// "is it a character?" or "is it not my enemy?" test would count it as a
// packmate. It is a rival faction and grants the wolf nothing.
func (s *PackTacticsTestSuite) TestPackTacticsDoesNotCountARivalFaction() {
	s.place("wolf-1", 1, 1)
	s.place("pc-1", 5, 5)
	s.place("hobgoblin-1", 5, 6)

	ctx := gamectx.WithRoom(s.ctx, s.room)
	ctx = gamectx.WithCast(ctx, &fakeCast{side: map[string]string{
		"wolf-1": "wolves", "pc-1": "party", "hobgoblin-1": "hobgoblins",
	}})

	s.packTactics = PackTactics("wolf-1").(*packTacticsCondition)
	s.Require().NoError(s.packTactics.Apply(ctx, s.bus))

	result := s.swing(ctx, "pc-1")
	s.Empty(result.AdvantageSources, "a rival faction beside the target is not a packmate")
}

// TestPackTacticsWithoutACastLeavesTheChainAlone — a question it cannot answer
// is not an error, and must not poison the fold.
func (s *PackTacticsTestSuite) TestPackTacticsWithoutACastLeavesTheChainAlone() {
	s.place("wolf-1", 1, 1)
	s.place("wolf-2", 5, 6)
	s.place("pc-1", 5, 5)

	ctx := gamectx.WithRoom(s.ctx, s.room) // room, but no cast

	s.packTactics = PackTactics("wolf-1").(*packTacticsCondition)
	s.Require().NoError(s.packTactics.Apply(ctx, s.bus))

	result := s.swing(ctx, "pc-1")
	s.Empty(result.AdvantageSources)
	s.Equal(4, result.AttackBonus, "the rest of the attack must survive an unanswerable question")
}

// TestPackTacticsReachIsFiveFeetOnAGridlessRoom pins the bug the #1255 review
// caught: the radius used to be 1.5 cells "to include diagonals".
//
// A square grid does not need that correction — SquareGrid.Distance is
// max(|dx|,|dy|), so a diagonal neighbour is already 1 — and nothing on a
// square grid ever lands between 1 and 1.5, which is why it looked harmless.
// A gridless room measures Euclidean, where 1.5 cells is 7.5 feet, and a
// packmate standing seven feet away would have granted advantage.
func (s *PackTacticsTestSuite) TestPackTacticsReachIsFiveFeetOnAGridlessRoom() {
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "open-field",
		Type: "wilderness",
		Grid: spatial.NewGridlessRoom(spatial.GridlessConfig{Width: 20, Height: 20}),
	})
	for id, pos := range map[string]spatial.Position{
		"wolf-1": {X: 1, Y: 1},
		"pc-1":   {X: 5, Y: 5},
		"wolf-2": {X: 5, Y: 6.4}, // 1.4 cells from the target — seven feet
	} {
		s.Require().NoError(room.PlaceEntity(&fakeEntity{id: id}, pos))
	}

	ctx := gamectx.WithRoom(s.ctx, room)
	ctx = gamectx.WithCast(ctx, &fakeCast{side: map[string]string{
		"wolf-1": "wolves", "wolf-2": "wolves", "pc-1": "party",
	}})

	s.packTactics = PackTactics("wolf-1").(*packTacticsCondition)
	s.Require().NoError(s.packTactics.Apply(ctx, s.bus))

	result := s.swing(ctx, "pc-1")
	s.Empty(result.AdvantageSources, "a packmate seven feet away is not within five feet")

	// And one that genuinely is adjacent still counts, so this is a boundary
	// fix rather than the rule being switched off.
	s.Require().NoError(room.RemoveEntity("wolf-2"))
	s.Require().NoError(room.PlaceEntity(&fakeEntity{id: "wolf-2"}, spatial.Position{X: 5, Y: 6}))
	s.Require().Len(s.swing(ctx, "pc-1").AdvantageSources, 1)
}

func (s *PackTacticsTestSuite) TestPackTacticsIgnoresOtherAttackers() {
	// Create Pack Tactics for wolf-1
	s.packTactics = PackTactics("wolf-1").(*packTacticsCondition)

	// Apply to bus
	err := s.packTactics.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Create attack event from different attacker
	event := dnd5eEvents.AttackChainEvent{
		AttackerID:  "wolf-2", // Different attacker
		TargetID:    "pc-1",
		IsMelee:     true,
		AttackBonus: 4,
		TargetAC:    15,
	}

	// Publish attack chain event
	chain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attackTopic := dnd5eEvents.AttackChain.On(s.bus)

	modifiedChain, err := attackTopic.PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	// Execute chain
	result, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// Pack Tactics should not apply to wolf-2
	s.Assert().Equal("wolf-2", result.AttackerID)
}

func (s *PackTacticsTestSuite) TestPackTacticsCanBeRemoved() {
	// Create and apply pack tactics
	s.packTactics = PackTactics("wolf-1").(*packTacticsCondition)
	err := s.packTactics.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().True(s.packTactics.IsApplied())

	// Remove pack tactics
	err = s.packTactics.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Assert().False(s.packTactics.IsApplied())
}
