// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// mockCombatant implements combat.Combatant for testing
type mockCombatant struct {
	id           string
	hitPoints    int
	maxHitPoints int
	ac           int
	dirty        bool
}

func (m *mockCombatant) GetID() string        { return m.id }
func (m *mockCombatant) GetHitPoints() int    { return m.hitPoints }
func (m *mockCombatant) GetMaxHitPoints() int { return m.maxHitPoints }
func (m *mockCombatant) AC() int              { return m.ac }
func (m *mockCombatant) IsDirty() bool        { return m.dirty }
func (m *mockCombatant) MarkClean()           { m.dirty = false }

func (m *mockCombatant) ApplyDamage(_ context.Context, input *combat.ApplyDamageInput) *combat.ApplyDamageResult {
	if input == nil {
		return &combat.ApplyDamageResult{
			CurrentHP:  m.hitPoints,
			PreviousHP: m.hitPoints,
		}
	}

	previousHP := m.hitPoints
	totalDamage := 0

	for _, instance := range input.Instances {
		totalDamage += instance.Amount
	}

	m.hitPoints -= totalDamage
	if m.hitPoints < 0 {
		m.hitPoints = 0
	}

	return &combat.ApplyDamageResult{
		TotalDamage:   totalDamage,
		CurrentHP:     m.hitPoints,
		DroppedToZero: m.hitPoints == 0 && previousHP > 0,
		PreviousHP:    previousHP,
	}
}

// DealDamageTestSuite tests the DealDamage function
type DealDamageTestSuite struct {
	suite.Suite
	ctx      context.Context
	eventBus events.EventBus
}

func (s *DealDamageTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.eventBus = events.NewEventBus()
}

func (s *DealDamageTestSuite) TestValidateNilInput() {
	err := (*combat.DealDamageInput)(nil).Validate()
	s.Require().Error(err)
	s.Contains(err.Error(), "nil")
}

func (s *DealDamageTestSuite) TestValidateNilTarget() {
	input := &combat.DealDamageInput{
		Target:   nil,
		EventBus: s.eventBus,
		Instances: []combat.DamageInstanceInput{
			{Amount: 5, Type: damage.Slashing},
		},
	}
	err := input.Validate()
	s.Require().Error(err)
	s.Contains(err.Error(), "Target")
}

func (s *DealDamageTestSuite) TestValidateNilEventBus() {
	target := &mockCombatant{id: "hero-1", hitPoints: 20, maxHitPoints: 20}
	input := &combat.DealDamageInput{
		Target:   target,
		EventBus: nil,
		Instances: []combat.DamageInstanceInput{
			{Amount: 5, Type: damage.Slashing},
		},
	}
	err := input.Validate()
	s.Require().Error(err)
	s.Contains(err.Error(), "EventBus")
}

func (s *DealDamageTestSuite) TestValidateNoInstancesOrComponents() {
	target := &mockCombatant{id: "hero-1", hitPoints: 20, maxHitPoints: 20}
	input := &combat.DealDamageInput{
		Target:    target,
		EventBus:  s.eventBus,
		Instances: []combat.DamageInstanceInput{},
	}
	err := input.Validate()
	s.Require().Error(err)
	s.Contains(err.Error(), "Instances or Components")
}

func (s *DealDamageTestSuite) TestDealDamageBasic() {
	target := &mockCombatant{
		id:           "hero-1",
		hitPoints:    20,
		maxHitPoints: 20,
	}

	// Track DamageReceivedEvent
	var receivedEvent dnd5eEvents.DamageReceivedEvent
	damageTopic := dnd5eEvents.DamageReceivedTopic.On(s.eventBus)
	_, err := damageTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DamageReceivedEvent) error {
		receivedEvent = event
		return nil
	})
	s.Require().NoError(err)

	output, err := combat.DealDamage(s.ctx, &combat.DealDamageInput{
		Target:     target,
		AttackerID: "goblin-1",
		Source:     combat.DamageSourceAttack,
		Instances: []combat.DamageInstanceInput{
			{Amount: 8, Type: damage.Slashing},
		},
		EventBus: s.eventBus,
	})

	s.Require().NoError(err)
	s.Require().NotNil(output)

	// Verify output
	s.Equal(8, output.TotalDamage)
	s.Equal(12, output.CurrentHP)
	s.False(output.DroppedToZero)
	s.Len(output.FinalInstances, 1)
	s.Equal(8, output.FinalInstances[0].Amount)
	s.Equal(damage.Slashing, output.FinalInstances[0].Type)

	// Verify target was updated
	s.Equal(12, target.GetHitPoints())

	// Verify notification event was published
	s.Equal("hero-1", receivedEvent.TargetID)
	s.Equal("goblin-1", receivedEvent.SourceID)
	s.Equal(8, receivedEvent.Amount)
	s.Equal(damage.Slashing, receivedEvent.DamageType)
}

func (s *DealDamageTestSuite) TestDealDamageMultipleInstances() {
	target := &mockCombatant{
		id:           "hero-1",
		hitPoints:    30,
		maxHitPoints: 30,
	}

	output, err := combat.DealDamage(s.ctx, &combat.DealDamageInput{
		Target:     target,
		AttackerID: "flaming-skeleton",
		Source:     combat.DamageSourceAttack,
		Instances: []combat.DamageInstanceInput{
			{Amount: 7, Type: damage.Slashing},
			{Amount: 5, Type: damage.Fire},
		},
		EventBus: s.eventBus,
	})

	s.Require().NoError(err)
	s.Require().NotNil(output)

	// Total should be sum of all instances
	s.Equal(12, output.TotalDamage)
	s.Equal(18, output.CurrentHP)
	s.Len(output.FinalInstances, 2)
}

func (s *DealDamageTestSuite) TestDealDamageDropsToZero() {
	target := &mockCombatant{
		id:           "hero-1",
		hitPoints:    10,
		maxHitPoints: 20,
	}

	output, err := combat.DealDamage(s.ctx, &combat.DealDamageInput{
		Target:     target,
		AttackerID: "dragon",
		Source:     combat.DamageSourceSpell,
		Instances: []combat.DamageInstanceInput{
			{Amount: 15, Type: damage.Fire},
		},
		EventBus: s.eventBus,
	})

	s.Require().NoError(err)
	s.Require().NotNil(output)

	s.Equal(15, output.TotalDamage)
	s.Equal(0, output.CurrentHP)
	s.True(output.DroppedToZero)

	// Verify target HP is capped at 0
	s.Equal(0, target.GetHitPoints())
}

func (s *DealDamageTestSuite) TestDealDamageCritical() {
	target := &mockCombatant{
		id:           "hero-1",
		hitPoints:    20,
		maxHitPoints: 20,
	}

	// Track DamageReceivedEvent
	var receivedEvent dnd5eEvents.DamageReceivedEvent
	damageTopic := dnd5eEvents.DamageReceivedTopic.On(s.eventBus)
	_, err := damageTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DamageReceivedEvent) error {
		receivedEvent = event
		return nil
	})
	s.Require().NoError(err)

	output, err := combat.DealDamage(s.ctx, &combat.DealDamageInput{
		Target:     target,
		AttackerID: "assassin",
		Source:     combat.DamageSourceAttack,
		Instances: []combat.DamageInstanceInput{
			{Amount: 14, Type: damage.Piercing}, // Double dice on crit
		},
		IsCritical: true,
		EventBus:   s.eventBus,
	})

	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Equal(14, output.TotalDamage)

	// The event should reflect the damage
	s.Equal(14, receivedEvent.Amount)
}

func (s *DealDamageTestSuite) TestDealDamageConditionSource() {
	target := &mockCombatant{
		id:           "poisoned-hero",
		hitPoints:    15,
		maxHitPoints: 20,
	}

	output, err := combat.DealDamage(s.ctx, &combat.DealDamageInput{
		Target: target,
		Source: combat.DamageSourceCondition,
		Instances: []combat.DamageInstanceInput{
			{Amount: 3, Type: damage.Poison},
		},
		EventBus: s.eventBus,
	})

	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Equal(3, output.TotalDamage)
	s.Equal(12, output.CurrentHP)
}

func (s *DealDamageTestSuite) TestDealDamageEnvironmentSource() {
	target := &mockCombatant{
		id:           "falling-hero",
		hitPoints:    20,
		maxHitPoints: 20,
	}

	output, err := combat.DealDamage(s.ctx, &combat.DealDamageInput{
		Target: target,
		Source: combat.DamageSourceEnvironment,
		Instances: []combat.DamageInstanceInput{
			{Amount: 10, Type: damage.Bludgeoning},
		},
		EventBus: s.eventBus,
	})

	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Equal(10, output.TotalDamage)
	s.Equal(10, output.CurrentHP)
}

func TestDealDamageSuite(t *testing.T) {
	suite.Run(t, new(DealDamageTestSuite))
}
