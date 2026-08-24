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

type SkeletonCaptainTestSuite struct {
	suite.Suite
}

func TestSkeletonCaptainSuite(t *testing.T) {
	suite.Run(t, new(SkeletonCaptainTestSuite))
}

func (s *SkeletonCaptainTestSuite) TestNewSkeletonCaptain() {
	captain := NewSkeletonCaptain("skeleton-captain-1")

	s.Require().NotNil(captain)
	s.Assert().Equal("skeleton-captain-1", captain.GetID())
	s.Assert().Equal("Skeleton Captain", captain.Name())
	s.Assert().Equal(refs.Monsters.SkeletonCaptain(), captain.Ref())

	// Check stats (CR 2 crypt boss)
	s.Assert().Equal(45, captain.HP())
	s.Assert().Equal(45, captain.MaxHP())
	s.Assert().Equal(16, captain.AC())

	// Check ability scores
	scores := captain.AbilityScores()
	s.Assert().Equal(16, scores[abilities.STR])
	s.Assert().Equal(14, scores[abilities.DEX])
	s.Assert().Equal(16, scores[abilities.CON])
	s.Assert().Equal(6, scores[abilities.INT])
	s.Assert().Equal(8, scores[abilities.WIS])
	s.Assert().Equal(5, scores[abilities.CHA])

	// Check speed
	speed := captain.Speed()
	s.Assert().Equal(30, speed.Walk)

	// The component attack remains; multiattack waits for a sequence profile.
	actions := captain.Actions()
	s.Require().Len(actions, 1)
	s.Equal(refs.MonsterActions.SkeletonCaptainLongsword(), &actions[0].Ref)
}

func (s *SkeletonCaptainTestSuite) TestSkeletonCaptainTraitsIncludedInData() {
	captain := NewSkeletonCaptain("skeleton-captain-1")
	s.Require().NotNil(captain)

	data := captain.ToData()
	s.Require().NotNil(data)

	// Should have 2 conditions: vulnerability to bludgeoning, immunity to poison
	// (same undead traits as a rank-and-file skeleton - no Life Drain, no
	// Undead Fortitude, no paralysis: none of those are wired in the toolkit today)
	s.Require().Len(data.Conditions, 2, "skeleton captain should have 2 trait conditions")

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

	s.Assert().True(hasVulnerability, "skeleton captain should have vulnerability trait")
	s.Assert().True(hasImmunity, "skeleton captain should have immunity trait")
}

func (s *SkeletonCaptainTestSuite) TestSkeletonCaptainTraitsLoadedFromData() {
	ctx := context.Background()

	captain := NewSkeletonCaptain("skeleton-captain-1")
	data := captain.ToData()

	bus := events.NewEventBus()
	loaded, err := monster.LoadFromData(ctx, data, bus)
	s.Require().NoError(err)
	s.Require().NotNil(loaded)
	defer func() { _ = loaded.Cleanup(ctx) }()

	err = monstertraits.LoadMonsterConditions(ctx, loaded, data.Conditions, bus, nil)
	s.Require().NoError(err)

	conditions := loaded.GetConditions()
	s.Assert().Len(conditions, 2, "loaded skeleton captain should have 2 conditions applied")

	for _, cond := range conditions {
		s.Assert().True(cond.IsApplied(), "condition should be applied to bus")
	}
}
