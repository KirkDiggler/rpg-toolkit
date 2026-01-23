// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

//nolint:dupl // Monster factory tests intentionally follow same structure with different values
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

type ZombieTestSuite struct {
	suite.Suite
}

func TestZombieSuite(t *testing.T) {
	suite.Run(t, new(ZombieTestSuite))
}

func (s *ZombieTestSuite) TestNewZombie() {
	zombie := NewZombie("zombie-1")

	s.Require().NotNil(zombie)
	s.Assert().Equal("zombie-1", zombie.GetID())
	s.Assert().Equal("Zombie", zombie.Name())

	// Check stats
	s.Assert().Equal(22, zombie.HP())
	s.Assert().Equal(22, zombie.MaxHP())
	s.Assert().Equal(8, zombie.AC())

	// Check ability scores
	scores := zombie.AbilityScores()
	s.Assert().Equal(13, scores[abilities.STR])
	s.Assert().Equal(6, scores[abilities.DEX])
	s.Assert().Equal(16, scores[abilities.CON])
	s.Assert().Equal(3, scores[abilities.INT])
	s.Assert().Equal(6, scores[abilities.WIS])
	s.Assert().Equal(5, scores[abilities.CHA])

	// Check speed
	speed := zombie.Speed()
	s.Assert().Equal(20, speed.Walk)

	// Check actions - should have slam attack
	actions := zombie.Actions()
	s.Require().Len(actions, 1)
	s.Assert().Equal("slam", actions[0].GetID())
}

func (s *ZombieTestSuite) TestZombieTraits() {
	zombie := NewZombie("zombie-1")
	s.Require().NotNil(zombie)

	// Zombies have high CON (+3) for Undead Fortitude
	scores := zombie.AbilityScores()
	s.Assert().Equal(16, scores[abilities.CON], "zombies have high CON for undead fortitude")
	s.Assert().Equal(6, scores[abilities.DEX], "zombies are very slow")
}

func (s *ZombieTestSuite) TestZombieTraitsIncludedInData() {
	// Create zombie - factory should add trait data
	zombie := NewZombie("zombie-1")
	s.Require().NotNil(zombie)

	// Convert to Data - should include traits
	data := zombie.ToData()
	s.Require().NotNil(data)

	// Should have 1 condition: immunity to poison
	s.Require().Len(data.Conditions, 1, "zombie should have 1 trait condition")

	// Verify the trait
	var peek struct {
		Ref        string      `json:"ref"`
		DamageType damage.Type `json:"damage_type"`
	}
	err := json.Unmarshal(data.Conditions[0], &peek)
	s.Require().NoError(err)

	s.Assert().Equal(refs.MonsterTraits.Immunity().String(), peek.Ref, "should be immunity trait")
	s.Assert().Equal(damage.Poison, peek.DamageType, "immunity should be to poison")
}

func (s *ZombieTestSuite) TestZombieTraitsLoadedFromData() {
	ctx := context.Background()

	// Create zombie and get its data
	zombie := NewZombie("zombie-1")
	data := zombie.ToData()

	// Create event bus and load from data
	bus := events.NewEventBus()
	loaded, err := monster.LoadFromData(ctx, data, bus)
	s.Require().NoError(err)
	s.Require().NotNil(loaded)
	defer func() { _ = loaded.Cleanup(ctx) }()

	// Load conditions using the helper (this applies them to the bus)
	err = monstertraits.LoadMonsterConditions(ctx, loaded, data.Conditions, bus, nil)
	s.Require().NoError(err)

	// Verify condition was applied
	conditions := loaded.GetConditions()
	s.Assert().Len(conditions, 1, "loaded zombie should have 1 condition applied")

	// Verify condition is applied (subscribed to bus)
	s.Assert().True(conditions[0].IsApplied(), "condition should be applied to bus")
}
