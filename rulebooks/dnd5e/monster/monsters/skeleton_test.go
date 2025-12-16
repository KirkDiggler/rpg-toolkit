// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/stretchr/testify/suite"
)

type SkeletonTestSuite struct {
	suite.Suite
}

func TestSkeletonSuite(t *testing.T) {
	suite.Run(t, new(SkeletonTestSuite))
}

func (s *SkeletonTestSuite) TestNewSkeleton() {
	skeleton := NewSkeleton("skeleton-1")

	s.Require().NotNil(skeleton)
	s.Assert().Equal("skeleton-1", skeleton.GetID())
	s.Assert().Equal("Skeleton", skeleton.Name())

	// Check stats
	s.Assert().Equal(13, skeleton.HP())
	s.Assert().Equal(13, skeleton.MaxHP())
	s.Assert().Equal(13, skeleton.AC())

	// Check ability scores
	scores := skeleton.AbilityScores()
	s.Assert().Equal(10, scores[abilities.STR])
	s.Assert().Equal(14, scores[abilities.DEX])
	s.Assert().Equal(15, scores[abilities.CON])
	s.Assert().Equal(6, scores[abilities.INT])
	s.Assert().Equal(8, scores[abilities.WIS])
	s.Assert().Equal(5, scores[abilities.CHA])

	// Check speed
	speed := skeleton.Speed()
	s.Assert().Equal(30, speed.Walk)

	// Check actions - should have shortsword and shortbow
	actions := skeleton.Actions()
	s.Require().Len(actions, 2)

	// Find shortsword and shortbow
	var hasShortsword, hasShortbow bool
	for _, action := range actions {
		switch action.GetID() {
		case "shortsword":
			hasShortsword = true
		case "shortbow":
			hasShortbow = true
		}
	}
	s.Assert().True(hasShortsword, "should have shortsword action")
	s.Assert().True(hasShortbow, "should have shortbow action")
}

func (s *SkeletonTestSuite) TestSkeletonTraits() {
	// Test that skeleton can be created
	// Note: Traits (vulnerability to bludgeoning, immunity to poison) are applied
	// when the monster is loaded into combat with an event bus.
	// This test just verifies the factory creates a valid monster.
	skeleton := NewSkeleton("skeleton-1")
	s.Require().NotNil(skeleton)

	// Verify basic trait expectations via stats
	// Skeletons should have CON 15 (+2) but low CHA (undead)
	scores := skeleton.AbilityScores()
	s.Assert().Equal(15, scores[abilities.CON], "skeletons have good CON")
	s.Assert().Equal(5, scores[abilities.CHA], "skeletons have very low CHA")
}
