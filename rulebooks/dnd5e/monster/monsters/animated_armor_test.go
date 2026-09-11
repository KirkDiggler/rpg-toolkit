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

type AnimatedArmorTestSuite struct {
	suite.Suite
}

func TestAnimatedArmorSuite(t *testing.T) {
	suite.Run(t, new(AnimatedArmorTestSuite))
}

func (s *AnimatedArmorTestSuite) TestNewAnimatedArmor() {
	armor := NewAnimatedArmor("animated-armor-1")

	s.Require().NotNil(armor)
	s.Assert().Equal("animated-armor-1", armor.GetID())
	s.Assert().Equal("Animated Armor", armor.Name())

	// Check stats
	s.Assert().Equal(33, armor.HP())
	s.Assert().Equal(33, armor.MaxHP())
	s.Assert().Equal(18, armor.AC())

	// Check ability scores
	scores := armor.AbilityScores()
	s.Assert().Equal(14, scores[abilities.STR])
	s.Assert().Equal(11, scores[abilities.DEX])
	s.Assert().Equal(13, scores[abilities.CON])
	s.Assert().Equal(1, scores[abilities.INT])
	s.Assert().Equal(3, scores[abilities.WIS])
	s.Assert().Equal(1, scores[abilities.CHA])

	// Check speed
	speed := armor.Speed()
	s.Assert().Equal(25, speed.Walk)

	// Check actions - one slam. The SRD's Multiattack (two slams) is deferred
	// with the rest of the roster, so a second registered action here would be
	// a mechanism nothing drives.
	actions := armor.Actions()
	s.Require().Len(actions, 1)
	s.Assert().Equal(refs.MonsterActions.AnimatedArmorSlam(), &actions[0].Ref)
}

// TestAnimatedArmorCarriesBothImmunities is the reason this monster needed a
// test of its own rather than another copy of the roster's shape: it is the
// first to carry two damage immunities, and the two trait blobs share a ref.
// If AddTraitData kept traits in a map keyed by ref, or the loader coalesced
// them, the second immunity would silently replace the first and the armor
// would take full psychic damage. Asserting the SET of damage types — rather
// than a count, or the first element — is what makes that failure visible.
func (s *AnimatedArmorTestSuite) TestAnimatedArmorCarriesBothImmunities() {
	armor := NewAnimatedArmor("animated-armor-1")
	s.Require().NotNil(armor)

	data := armor.ToData()
	s.Require().NotNil(data)

	immune := make([]damage.Type, 0, len(data.Conditions))
	for _, blob := range data.Conditions {
		var peek struct {
			Ref        string      `json:"ref"`
			DamageType damage.Type `json:"damage_type"`
		}
		s.Require().NoError(json.Unmarshal(blob, &peek))
		s.Assert().Equal(refs.MonsterTraits.Immunity().String(), peek.Ref,
			"every trait the armor carries today is an immunity")
		immune = append(immune, peek.DamageType)
	}

	s.Assert().ElementsMatch([]damage.Type{damage.Poison, damage.Psychic}, immune,
		"an empty suit has nothing to poison and no mind to assail")
}

// TestAnimatedArmorImmunitiesLoadedFromData proves both immunities survive the
// round trip and each subscribes to the bus in its own right. ToData holding
// two blobs would still be worthless if attaching them produced one condition.
func (s *AnimatedArmorTestSuite) TestAnimatedArmorImmunitiesLoadedFromData() {
	ctx := context.Background()

	armor := NewAnimatedArmor("animated-armor-1")
	data := armor.ToData()

	bus := events.NewEventBus()
	loaded, err := monster.LoadFromData(ctx, data, bus)
	s.Require().NoError(err)
	s.Require().NotNil(loaded)
	defer func() { _ = loaded.Cleanup(ctx) }()

	err = monstertraits.LoadMonsterConditions(ctx, loaded, data.Conditions, bus, nil)
	s.Require().NoError(err)

	conditions := loaded.GetConditions()
	s.Require().Len(conditions, 2, "both immunities must attach, not collapse into one")
	for i, condition := range conditions {
		s.Assert().True(condition.IsApplied(), "condition %d should be applied to bus", i)
	}
}
