// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// cubePos creates a CubeCoordinate from X (z defaults to 0), deriving Y = -X
func cubePos(x int) spatial.CubeCoordinate {
	return spatial.CubeCoordinate{X: x, Y: -x, Z: 0}
}

// mockEntity implements core.Entity for testing
type mockEntity struct {
	id string
	hp int
	ac int
}

func (m *mockEntity) GetID() string {
	return m.id
}

func (m *mockEntity) GetType() core.EntityType {
	return dnd5e.EntityTypeCharacter
}

func (m *mockEntity) HP() int {
	return m.hp
}

func (m *mockEntity) AC() int {
	return m.ac
}

// TargetingTestSuite tests targeting strategy functionality
type TargetingTestSuite struct {
	suite.Suite
	monster    *Monster
	perception *PerceptionData
}

// SetupTest runs before each test function
func (s *TargetingTestSuite) SetupTest() {
	// Create a test monster
	s.monster = New(Config{
		ID:   "test-monster",
		Name: "Test Monster",
		HP:   20,
		AC:   14,
	})

	// Create test perception data with multiple enemies
	// Using hex distances: 1 = adjacent, 2 = 2 hexes away, etc.
	s.perception = &PerceptionData{
		MyPosition: cubePos(0),
		Enemies: []PerceivedEntity{
			{
				Entity:   &mockEntity{id: "enemy-1", hp: 25, ac: 15},
				Position: cubePos(1),
				Distance: 1, // 1 hex = adjacent
				Adjacent: true,
				HP:       25,
				AC:       15,
			},
			{
				Entity:   &mockEntity{id: "enemy-2", hp: 10, ac: 18},
				Position: cubePos(2),
				Distance: 2, // 2 hexes away
				Adjacent: false,
				HP:       10,
				AC:       18,
			},
			{
				Entity:   &mockEntity{id: "enemy-3", hp: 30, ac: 12},
				Position: cubePos(3),
				Distance: 3, // 3 hexes away
				Adjacent: false,
				HP:       30,
				AC:       12,
			},
		},
	}
}

// TestTargetClosest verifies TargetClosest strategy selects the nearest enemy
func (s *TargetingTestSuite) TestTargetClosest() {
	s.monster.SetTargeting(TargetClosest)

	// Create a simple melee action to test selectTarget
	action := NewScimitarAction(ScimitarConfig{
		AttackBonus: 4,
		DamageDice:  "1d6+2",
	})

	target := s.monster.selectTarget(action, s.perception)
	s.Require().NotNil(target)
	s.Equal("enemy-1", target.GetID(), "should select closest enemy")
}

// TestTargetLowestHP verifies TargetLowestHP strategy selects the enemy with lowest HP
func (s *TargetingTestSuite) TestTargetLowestHP() {
	s.monster.SetTargeting(TargetLowestHP)

	// Create a simple melee action to test selectTarget
	action := NewScimitarAction(ScimitarConfig{
		AttackBonus: 4,
		DamageDice:  "1d6+2",
	})

	target := s.monster.selectTarget(action, s.perception)
	s.Require().NotNil(target)
	s.Equal("enemy-2", target.GetID(), "should select enemy with lowest HP (10)")
}

// TestTargetLowestAC verifies TargetLowestAC strategy selects the enemy with lowest AC
func (s *TargetingTestSuite) TestTargetLowestAC() {
	s.monster.SetTargeting(TargetLowestAC)

	// Create a simple melee action to test selectTarget
	action := NewScimitarAction(ScimitarConfig{
		AttackBonus: 4,
		DamageDice:  "1d6+2",
	})

	target := s.monster.selectTarget(action, s.perception)
	s.Require().NotNil(target)
	s.Equal("enemy-3", target.GetID(), "should select enemy with lowest AC (12)")
}

// TestTargetingDefault verifies default targeting strategy is TargetClosest
func (s *TargetingTestSuite) TestTargetingDefault() {
	s.Equal(TargetClosest, s.monster.Targeting(), "default targeting should be TargetClosest")
}

// TestTargetingNoEnemies verifies selectTarget returns nil when no enemies available
func (s *TargetingTestSuite) TestTargetingNoEnemies() {
	s.perception.Enemies = []PerceivedEntity{}

	action := NewScimitarAction(ScimitarConfig{
		AttackBonus: 4,
		DamageDice:  "1d6+2",
	})

	target := s.monster.selectTarget(action, s.perception)
	s.Nil(target, "should return nil when no enemies")
}

// TestTargetingWithTies verifies behavior when multiple enemies have same HP/AC
func (s *TargetingTestSuite) TestTargetingWithTies() {
	s.Run("lowest hp tie - picks first", func() {
		s.monster.SetTargeting(TargetLowestHP)

		// Create enemies with same HP
		s.perception.Enemies = []PerceivedEntity{
			{
				Entity:   &mockEntity{id: "enemy-1", hp: 10, ac: 15},
				Distance: 1,
				HP:       10,
				AC:       15,
			},
			{
				Entity:   &mockEntity{id: "enemy-2", hp: 10, ac: 18},
				Distance: 2,
				HP:       10,
				AC:       18,
			},
		}

		action := NewScimitarAction(ScimitarConfig{AttackBonus: 4, DamageDice: "1d6+2"})
		target := s.monster.selectTarget(action, s.perception)
		s.Require().NotNil(target)
		s.Equal("enemy-1", target.GetID(), "when HP tied, should pick first")
	})

	s.Run("lowest ac tie - picks first", func() {
		s.monster.SetTargeting(TargetLowestAC)

		// Create enemies with same AC
		s.perception.Enemies = []PerceivedEntity{
			{
				Entity:   &mockEntity{id: "enemy-1", hp: 25, ac: 12},
				Distance: 1,
				HP:       25,
				AC:       12,
			},
			{
				Entity:   &mockEntity{id: "enemy-2", hp: 10, ac: 12},
				Distance: 2,
				HP:       10,
				AC:       12,
			},
		}

		action := NewScimitarAction(ScimitarConfig{AttackBonus: 4, DamageDice: "1d6+2"})
		target := s.monster.selectTarget(action, s.perception)
		s.Require().NotNil(target)
		s.Equal("enemy-1", target.GetID(), "when AC tied, should pick first")
	})
}

// TestTargetingSerialization verifies targeting strategy persists through serialization
func (s *TargetingTestSuite) TestTargetingSerialization() {
	s.monster.SetTargeting(TargetLowestHP)

	// Serialize to Data
	data := s.monster.ToData()
	s.Equal(TargetLowestHP, data.Targeting, "targeting strategy should be serialized")

	// Deserialize and verify
	// (Note: LoadFromData test would require event bus setup,
	// just verify the data round-trip for now)
	s.Equal(TargetLowestHP, data.Targeting)
}

// Run the suite
func TestTargetingSuite(t *testing.T) {
	suite.Run(t, new(TargetingTestSuite))
}
