// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Define game event types with TypedRefs

// ConditionAppliedEvent - when a status effect is applied
type ConditionAppliedEvent struct {
	ctx       *events.EventContext
	Target    string // EntityID
	Condition string // "poisoned", "burning", etc
	Duration  int    // turns
	Source    string // Who applied it
}

func (e *ConditionAppliedEvent) EventRef() *core.Ref {
	return ConditionAppliedRef.Ref
}

func (e *ConditionAppliedEvent) Context() *events.EventContext {
	if e.ctx == nil {
		e.ctx = events.NewEventContext()
	}
	return e.ctx
}

var ConditionAppliedRef = &core.TypedRef[*ConditionAppliedEvent]{
	Ref: func() *core.Ref {
		r, err := core.ParseString("events:condition:applied")
		if err != nil {
			panic(err)
		}
		return r
	}(),
}

// DamageIntentEvent - when damage is about to be dealt
type DamageIntentEvent struct {
	ctx    *events.EventContext
	Source string // EntityID
	Target string // EntityID
	Amount int
	Type   string // "fire", "poison", etc
}

func (e *DamageIntentEvent) EventRef() *core.Ref {
	return DamageIntentRef.Ref
}

func (e *DamageIntentEvent) Context() *events.EventContext {
	if e.ctx == nil {
		e.ctx = events.NewEventContext()
	}
	return e.ctx
}

var DamageIntentRef = &core.TypedRef[*DamageIntentEvent]{
	Ref: func() *core.Ref {
		r, err := core.ParseString("events:damage:intent")
		if err != nil {
			panic(err)
		}
		return r
	}(),
}

// TestGameEventFlow shows how events enable decoupled game mechanics
func TestGameEventFlow(t *testing.T) {
	bus := events.NewBus()
	ctx := context.Background()

	// Track what happened
	var appliedConditions []string
	var damageDealt []int

	// Subscribe to condition applied events
	// This would trigger a pipeline in real usage
	subID1, err := events.Subscribe(ctx, bus, ConditionAppliedRef,
		func(ctx context.Context, e *ConditionAppliedEvent) error {
			appliedConditions = append(appliedConditions, e.Condition)

			// Burning condition causes damage (decoupled!)
			if e.Condition == "burning" {
				// Publish damage intent - this is decoupled from condition logic
				return events.Publish(ctx, bus, &DamageIntentEvent{
					Source: "burning-effect",
					Target: e.Target,
					Amount: 5,
					Type:   "fire",
				})
			}
			return nil
		},
	)
	require.NoError(t, err)
	defer func() { err := events.Unsubscribe(ctx, bus, subID1); require.NoError(t, err) }()

	// Subscribe to damage events
	// This would be a damage pipeline in real usage
	subID2, err := events.Subscribe(ctx, bus, DamageIntentRef,
		func(_ context.Context, e *DamageIntentEvent) error {
			damageDealt = append(damageDealt, e.Amount)
			// In real usage, this would run through damage pipeline:
			// - Calculate resistances
			// - Apply shields
			// - Deal final damage
			// - Check for death
			return nil
		},
	)
	require.NoError(t, err)
	defer func() { err := events.Unsubscribe(ctx, bus, subID2); require.NoError(t, err) }()

	// Game action: Apply burning condition
	err = events.Publish(ctx, bus, &ConditionAppliedEvent{
		Target:    "player-1",
		Condition: "burning",
		Duration:  3,
		Source:    "fire-trap",
	})
	require.NoError(t, err)

	// Verify the cascade of events
	assert.Equal(t, []string{"burning"}, appliedConditions)
	assert.Equal(t, []int{5}, damageDealt) // Burning triggered damage

	// Apply a non-damaging condition
	err = events.Publish(ctx, bus, &ConditionAppliedEvent{
		Target:    "player-1",
		Condition: "slowed",
		Duration:  2,
		Source:    "ice-spell",
	})
	require.NoError(t, err)

	// Verify only condition was applied, no damage
	assert.Equal(t, []string{"burning", "slowed"}, appliedConditions)
	assert.Equal(t, []int{5}, damageDealt) // No new damage
}

// TestEventToPipelineTrigger shows how events would trigger pipelines
func TestEventToPipelineTrigger(t *testing.T) {
	bus := events.NewBus()
	ctx := context.Background()

	// Mock pipeline stages
	pipelineStagesRun := []string{}

	// Simulate a damage pipeline
	damagePipeline := func(_ context.Context, _ *DamageIntentEvent) error {
		// Each stage would process the event
		stages := []string{"validate", "calculate", "resistance", "apply"}
		pipelineStagesRun = append(pipelineStagesRun, stages...)
		// In real implementation, each stage might modify the event context
		// or produce new events
		return nil
	}

	// Connect event to pipeline
	subID, err := events.Subscribe(ctx, bus, DamageIntentRef, damagePipeline)
	require.NoError(t, err)
	defer func() { err := events.Unsubscribe(ctx, bus, subID); require.NoError(t, err) }()

	// Trigger the pipeline via event
	err = events.Publish(ctx, bus, &DamageIntentEvent{
		Source: "player-1",
		Target: "goblin-1",
		Amount: 10,
		Type:   "slashing",
	})
	require.NoError(t, err)

	// Verify pipeline ran
	assert.Equal(t, []string{"validate", "calculate", "resistance", "apply"},
		pipelineStagesRun)
}

// TestTypedRefSafety verifies compile-time type safety
func TestTypedRefSafety(t *testing.T) {
	bus := events.NewBus()
	ctx := context.Background()

	// This gives compile-time type safety
	subID, err := events.Subscribe(ctx, bus, ConditionAppliedRef,
		func(_ context.Context, e *ConditionAppliedEvent) error {
			// e is guaranteed to be *ConditionAppliedEvent
			// No type assertion needed!
			_ = e.Condition // Can access fields directly
			_ = e.Duration
			_ = e.Target
			return nil
		},
	)
	require.NoError(t, err)
	defer func() { err := events.Unsubscribe(ctx, bus, subID); require.NoError(t, err) }()

	// PublishWithTypedRef adds extra safety
	event := &ConditionAppliedEvent{
		Target:    "test",
		Condition: "test",
		Duration:  1,
	}

	// This verifies the event matches the ref
	err = events.PublishWithTypedRef(ctx, bus, ConditionAppliedRef, event)
	require.NoError(t, err)
}
