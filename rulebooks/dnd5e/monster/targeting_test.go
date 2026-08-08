// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

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
		MyPosition: hexAt(0),
		Enemies: []PerceivedEntity{
			{
				Entity:   &mockEntity{id: "enemy-1", hp: 25, ac: 15},
				Position: hexAt(1),
				Distance: 1, // 1 hex = adjacent
				Adjacent: true,
				HP:       25,
				AC:       15,
			},
			{
				Entity:   &mockEntity{id: "enemy-2", hp: 10, ac: 18},
				Position: hexAt(2),
				Distance: 2, // 2 hexes away
				Adjacent: false,
				HP:       10,
				AC:       18,
			},
			{
				Entity:   &mockEntity{id: "enemy-3", hp: 30, ac: 12},
				Position: hexAt(3),
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

// TestTargetingDefault verifies a freshly-constructed monster's stored
// targeting strategy is TargetingUnspecified (the zero value) — NOT
// TargetClosest. rpg-toolkit#895 gate-review hardening deliberately moved
// TargetClosest off the zero value so "unset" and "explicit closest" are
// distinguishable in storage; this proves the zero value really is
// Unspecified now, and that it still SELECTS exactly like TargetClosest
// would (the decision-time equivalence, not a fourth strategy).
func (s *TargetingTestSuite) TestTargetingDefault() {
	s.Equal(TargetingUnspecified, s.monster.Targeting(),
		"a fresh monster's targeting must be the zero value, TargetingUnspecified, not TargetClosest")

	action := NewScimitarAction(ScimitarConfig{AttackBonus: 4, DamageDice: "1d6+2"})
	target := s.monster.selectTarget(action, s.perception)
	s.Require().NotNil(target)
	s.Equal("enemy-1", target.GetID(), "TargetingUnspecified must select exactly like TargetClosest")
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

// TestParseTargetingStrategy_Table covers the parse/String round trip for
// every accepted label, plus the rejection paths (rpg-toolkit#895).
func TestParseTargetingStrategy_Table(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    TargetingStrategy
		wantErr bool
	}{
		{name: "closest", input: "closest", want: TargetClosest},
		{name: "lowest-health", input: "lowest-health", want: TargetLowestHP},
		{name: "lowest-ac", input: "lowest-ac", want: TargetLowestAC},
		{name: "empty string rejected", input: "", wantErr: true},
		{name: "unknown value rejected", input: "random", wantErr: true},
		{name: "lowest-hp is not the author-facing label", input: "lowest-hp", wantErr: true},
		{name: "unspecified is not an authorable choice", input: "unspecified", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTargetingStrategy(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.input)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			// String() is the inverse of ParseTargetingStrategy for every
			// accepted label.
			require.Equal(t, tc.input, got.String())
		})
	}
}

// TestTargetingStrategy_Ref verifies the decision-rationale ref mapping used
// by ActionResolvedEvent.TargetRationale (rpg-toolkit#895). The "lowest-hp"
// ref segment deliberately differs from String()'s "lowest-health" label.
func TestTargetingStrategy_Ref(t *testing.T) {
	require.Equal(t, "dnd5e:targeting:closest", TargetClosest.Ref())
	require.Equal(t, "dnd5e:targeting:lowest-hp", TargetLowestHP.Ref())
	require.Equal(t, "dnd5e:targeting:lowest-ac", TargetLowestAC.Ref())
	require.Equal(t, "dnd5e:targeting:closest", TargetingUnspecified.Ref(),
		"unspecified must report the same rationale as closest — that IS the decision being made")
}

// TestTargetingStrategy_ZeroValueIsUnspecified locks in the gate-review
// hardening (rpg-toolkit#895): TargetingUnspecified, not TargetClosest, is
// the zero value. This is what lets a SpawnInstruction/persisted Data tell
// "author explicitly wrote closest" apart from "author wrote nothing" by
// construction — the entire point of the enum renumbering.
func TestTargetingStrategy_ZeroValueIsUnspecified(t *testing.T) {
	var zero TargetingStrategy
	require.Equal(t, TargetingUnspecified, zero)
	require.NotEqual(t, TargetClosest, zero, "TargetClosest must not be the zero value")
	require.Equal(t, "unspecified", zero.String())
	require.Equal(t, "closest", TargetClosest.String(),
		"TargetClosest keeps its own distinct label even though it now behaves like the zero value would")
}
