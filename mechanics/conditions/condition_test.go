// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
)

// MockEntity implements core.Entity for testing
type MockEntity struct {
	id  string
	typ string
}

func (m *MockEntity) GetID() string   { return m.id }
func (m *MockEntity) GetType() string { return m.typ }

// ConditionTestSuite tests the simplified condition system
type ConditionTestSuite struct {
	suite.Suite
	bus    events.EventBus
	target *MockEntity
}

func (s *ConditionTestSuite) SetupTest() {
	s.bus = events.NewBus()
	s.target = &MockEntity{id: "hero", typ: "character"}
}

func (s *ConditionTestSuite) TestSimpleConditionCreation() {
	ref := &core.Ref{
		Module: "test",
		Type:   "condition",
		Value:  "poisoned",
	}

	cond, err := conditions.NewSimpleCondition(ref)
	s.Require().NoError(err)
	s.Assert().NotNil(cond)
	s.Assert().Equal(ref, cond.Ref())
	s.Assert().False(cond.IsActive())
}

func (s *ConditionTestSuite) TestSimpleConditionInvalidRef() {
	_, err := conditions.NewSimpleCondition(nil)
	s.Assert().ErrorIs(err, conditions.ErrInvalidRef)
}

func (s *ConditionTestSuite) TestApplyCondition() {
	ref := &core.Ref{
		Module: "test",
		Type:   "condition",
		Value:  "stunned",
	}

	cond, err := conditions.NewSimpleCondition(ref)
	s.Require().NoError(err)

	cond.SetName("Stunned")
	cond.SetDescription("Cannot take actions")

	// Apply the condition
	err = cond.Apply(s.target, s.bus,
		conditions.WithSource("spell"),
		conditions.WithSaveDC(15),
		conditions.WithDuration(1*time.Minute),
	)
	s.Require().NoError(err)

	s.Assert().True(cond.IsActive())
	s.Assert().Equal(s.target, cond.Target())
	s.Assert().Equal("spell", cond.Source())
	s.Assert().Equal(15, cond.GetSaveDC())
}

func (s *ConditionTestSuite) TestCannotApplyTwice() {
	ref := &core.Ref{
		Module: "test",
		Type:   "condition",
		Value:  "paralyzed",
	}

	cond, err := conditions.NewSimpleCondition(ref)
	s.Require().NoError(err)

	// First apply succeeds
	err = cond.Apply(s.target, s.bus, conditions.WithSource("trap"))
	s.Require().NoError(err)

	// Second apply fails
	err = cond.Apply(s.target, s.bus, conditions.WithSource("spell"))
	s.Assert().ErrorIs(err, conditions.ErrAlreadyActive)
}

func (s *ConditionTestSuite) TestRemoveCondition() {
	ref := &core.Ref{
		Module: "test",
		Type:   "condition",
		Value:  "blinded",
	}

	cond, err := conditions.NewSimpleCondition(ref)
	s.Require().NoError(err)

	// Apply first
	err = cond.Apply(s.target, s.bus, conditions.WithSource("darkness"))
	s.Require().NoError(err)
	s.Assert().True(cond.IsActive())

	// Then remove
	err = cond.Remove(s.bus)
	s.Require().NoError(err)
	s.Assert().False(cond.IsActive())
}

func (s *ConditionTestSuite) TestCannotRemoveInactive() {
	ref := &core.Ref{
		Module: "test",
		Type:   "condition",
		Value:  "frightened",
	}

	cond, err := conditions.NewSimpleCondition(ref)
	s.Require().NoError(err)

	// Try to remove without applying
	err = cond.Remove(s.bus)
	s.Assert().ErrorIs(err, conditions.ErrNotActive)
}

func (s *ConditionTestSuite) TestMetadata() {
	ref := &core.Ref{
		Module: "test",
		Type:   "condition",
		Value:  "exhausted",
	}

	cond, err := conditions.NewSimpleCondition(ref)
	s.Require().NoError(err)

	// Apply with metadata
	err = cond.Apply(s.target, s.bus,
		conditions.WithSource("overexertion"),
		conditions.WithLevel(3),
		conditions.WithMetadata("cause", "forced march"),
		conditions.WithConcentration(),
	)
	s.Require().NoError(err)

	// Check metadata
	s.Assert().Equal(3, cond.GetLevel())

	cause, exists := cond.GetMetadata("cause")
	s.Assert().True(exists)
	s.Assert().Equal("forced march", cause)

	concentration, exists := cond.GetMetadata("concentration")
	s.Assert().True(exists)
	s.Assert().Equal(true, concentration)
}

func (s *ConditionTestSuite) TestToJSON() {
	ref := &core.Ref{
		Module: "test",
		Type:   "condition",
		Value:  "charmed",
	}

	cond, err := conditions.NewSimpleCondition(ref)
	s.Require().NoError(err)

	cond.SetName("Charmed")
	cond.SetDescription("Friendly to the charmer")

	err = cond.Apply(s.target, s.bus,
		conditions.WithSource("vampire"),
		conditions.WithSaveDC(17),
	)
	s.Require().NoError(err)

	// Serialize to JSON
	data, err := cond.ToJSON()
	s.Require().NoError(err)

	// Parse and verify
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	s.Require().NoError(err)

	s.Assert().Equal("Charmed", result["name"])
	s.Assert().Equal("Friendly to the charmer", result["description"])
	s.Assert().Equal("vampire", result["source"])
	s.Assert().Equal(true, result["is_active"])
	s.Assert().Equal("hero", result["target_id"])
}

func (s *ConditionTestSuite) TestDirtyTracking() {
	ref := &core.Ref{
		Module: "test",
		Type:   "condition",
		Value:  "prone",
	}

	cond, err := conditions.NewSimpleCondition(ref)
	s.Require().NoError(err)

	s.Assert().False(cond.IsDirty())

	// Setting properties marks as dirty
	cond.SetName("Prone")
	s.Assert().True(cond.IsDirty())

	cond.MarkClean()
	s.Assert().False(cond.IsDirty())

	// Applying marks as dirty
	err = cond.Apply(s.target, s.bus, conditions.WithSource("shove"))
	s.Require().NoError(err)
	s.Assert().True(cond.IsDirty())
}

func (s *ConditionTestSuite) TestEventSubscription() {
	ref := &core.Ref{
		Module: "test",
		Type:   "condition",
		Value:  "blessed",
	}

	cond, err := conditions.NewSimpleCondition(ref)
	s.Require().NoError(err)

	// Track if our handler was called
	handlerCalled := false

	// Create a custom apply function that subscribes to events
	handler := func(ctx context.Context, e events.Event) error {
		handlerCalled = true
		return nil
	}

	// Apply and subscribe
	err = cond.Apply(s.target, s.bus, conditions.WithSource("cleric"))
	s.Require().NoError(err)

	cond.Subscribe(s.bus, events.EventOnAttackRoll, 100, handler)

	// Publish event
	ctx := context.Background()
	event := events.NewGameEvent(events.EventOnAttackRoll, s.target, nil)
	err = s.bus.Publish(ctx, event)
	s.Require().NoError(err)

	s.Assert().True(handlerCalled)

	// Remove should clean up subscriptions
	err = cond.Remove(s.bus)
	s.Require().NoError(err)

	// Reset and publish again - handler should not be called
	handlerCalled = false
	err = s.bus.Publish(ctx, event)
	s.Require().NoError(err)
	s.Assert().False(handlerCalled)
}

func TestConditionSuite(t *testing.T) {
	suite.Run(t, new(ConditionTestSuite))
}
