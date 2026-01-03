// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/stretchr/testify/suite"
)

type CombatantDirtyTestSuite struct {
	suite.Suite
}

func TestCombatantDirtySuite(t *testing.T) {
	suite.Run(t, new(CombatantDirtyTestSuite))
}

// mockDirtyCombatant implements the extended Combatant interface with dirty tracking
type mockDirtyCombatant struct {
	id    string
	hp    int
	maxHP int
	ac    int
	dirty bool
}

func (m *mockDirtyCombatant) GetID() string        { return m.id }
func (m *mockDirtyCombatant) GetHitPoints() int    { return m.hp }
func (m *mockDirtyCombatant) GetMaxHitPoints() int { return m.maxHP }
func (m *mockDirtyCombatant) AC() int              { return m.ac }
func (m *mockDirtyCombatant) IsDirty() bool        { return m.dirty }
func (m *mockDirtyCombatant) MarkClean()           { m.dirty = false }

func (m *mockDirtyCombatant) ApplyDamage(ctx context.Context, input *combat.ApplyDamageInput) *combat.ApplyDamageResult {
	prev := m.hp
	total := 0
	for _, inst := range input.Instances {
		total += inst.Amount
	}
	m.hp -= total
	if m.hp < 0 {
		m.hp = 0
	}
	m.dirty = true // Mark dirty when HP changes
	return &combat.ApplyDamageResult{
		TotalDamage:   total,
		CurrentHP:     m.hp,
		DroppedToZero: m.hp == 0,
		PreviousHP:    prev,
	}
}

// Test that Combatant interface includes AC method
func (s *CombatantDirtyTestSuite) TestCombatant_HasAC() {
	combatant := &mockDirtyCombatant{
		id:    "test-1",
		hp:    20,
		maxHP: 20,
		ac:    15,
	}

	// This should compile - Combatant interface should have AC()
	var c combat.Combatant = combatant
	s.Equal(15, c.AC())
}

// Test that Combatant interface includes IsDirty method
func (s *CombatantDirtyTestSuite) TestCombatant_IsDirty() {
	combatant := &mockDirtyCombatant{
		id:    "test-1",
		hp:    20,
		maxHP: 20,
		dirty: false,
	}

	var c combat.Combatant = combatant
	s.False(c.IsDirty())
}

// Test that Combatant interface includes MarkClean method
func (s *CombatantDirtyTestSuite) TestCombatant_MarkClean() {
	combatant := &mockDirtyCombatant{
		id:    "test-1",
		hp:    20,
		maxHP: 20,
		dirty: true,
	}

	var c combat.Combatant = combatant
	s.True(c.IsDirty())
	c.MarkClean()
	s.False(c.IsDirty())
}

// Test that ApplyDamage marks combatant as dirty
func (s *CombatantDirtyTestSuite) TestApplyDamage_MarksDirty() {
	combatant := &mockDirtyCombatant{
		id:    "test-1",
		hp:    20,
		maxHP: 20,
		dirty: false,
	}

	var c combat.Combatant = combatant
	s.False(c.IsDirty())

	c.ApplyDamage(context.Background(), &combat.ApplyDamageInput{
		Instances: []combat.DamageInstance{{Amount: 5, Type: "slashing"}},
	})

	s.True(c.IsDirty())
	s.Equal(15, c.GetHitPoints())
}
