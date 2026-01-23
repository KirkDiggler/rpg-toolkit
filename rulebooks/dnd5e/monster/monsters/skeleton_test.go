// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
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
	skeleton := NewSkeleton("skeleton-1")
	s.Require().NotNil(skeleton)

	// Verify basic trait expectations via stats
	scores := skeleton.AbilityScores()
	s.Assert().Equal(15, scores[abilities.CON], "skeletons have good CON")
	s.Assert().Equal(5, scores[abilities.CHA], "skeletons have very low CHA")
}

func (s *SkeletonTestSuite) TestSkeletonTraitsIncludedInData() {
	// Create skeleton - factory should add trait data
	skeleton := NewSkeleton("skeleton-1")
	s.Require().NotNil(skeleton)

	// Convert to Data - should include traits
	data := skeleton.ToData()
	s.Require().NotNil(data)

	// Should have 2 conditions: vulnerability to bludgeoning, immunity to poison
	s.Require().Len(data.Conditions, 2, "skeleton should have 2 trait conditions")

	// Verify the traits by peeking at the refs
	// Refs are serialized as strings like "dnd5e:monster_traits:vulnerability"
	var hasVulnerability, hasImmunity bool
	for _, condJSON := range data.Conditions {
		var peek struct {
			Ref        string      `json:"ref"`
			DamageType damage.Type `json:"damage_type"`
		}
		err := json.Unmarshal(condJSON, &peek)
		s.Require().NoError(err)

		if peek.Ref == refs.MonsterTraits.Vulnerability().String() {
			hasVulnerability = true
			s.Assert().Equal(damage.Bludgeoning, peek.DamageType, "vulnerability should be to bludgeoning")
		}
		if peek.Ref == refs.MonsterTraits.Immunity().String() {
			hasImmunity = true
			s.Assert().Equal(damage.Poison, peek.DamageType, "immunity should be to poison")
		}
	}

	s.Assert().True(hasVulnerability, "skeleton should have vulnerability trait")
	s.Assert().True(hasImmunity, "skeleton should have immunity trait")
}

func (s *SkeletonTestSuite) TestSkeletonTraitsLoadedFromData() {
	ctx := context.Background()

	// Create skeleton and get its data
	skeleton := NewSkeleton("skeleton-1")
	data := skeleton.ToData()

	// Create event bus and load from data
	bus := events.NewEventBus()
	loaded, err := monster.LoadFromData(ctx, data, bus)
	s.Require().NoError(err)
	s.Require().NotNil(loaded)
	defer func() { _ = loaded.Cleanup(ctx) }()

	// Load conditions using the helper (this applies them to the bus)
	err = monstertraits.LoadMonsterConditions(ctx, loaded, data.Conditions, bus, nil)
	s.Require().NoError(err)

	// Verify conditions were applied
	conditions := loaded.GetConditions()
	s.Assert().Len(conditions, 2, "loaded skeleton should have 2 conditions applied")

	// Verify all conditions are applied (subscribed to bus)
	for _, cond := range conditions {
		s.Assert().True(cond.IsApplied(), "condition should be applied to bus")
	}
}
