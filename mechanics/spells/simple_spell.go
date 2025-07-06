// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package spells

import (
	"context"
	"fmt"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// SimpleSpell provides a basic spell implementation.
type SimpleSpell struct {
	id            string
	name          string
	level         int
	school        string
	castingTime   time.Duration
	rangeValue    int
	duration      events.Duration
	description   string
	components    CastingComponents
	ritual        bool
	concentration bool
	upcastable    bool
	targetType    TargetType
	aoe           *AreaOfEffect
	maxTargets    int

	// Effect function
	castFunc func(context CastContext) error
}

// SimpleSpellConfig configures a simple spell.
type SimpleSpellConfig struct {
	ID            string
	Name          string
	Level         int
	School        string
	CastingTime   time.Duration
	Range         int
	Duration      events.Duration
	Description   string
	Components    CastingComponents
	Ritual        bool
	Concentration bool
	Upcastable    bool
	TargetType    TargetType
	AreaOfEffect  *AreaOfEffect
	MaxTargets    int
	CastFunc      func(context CastContext) error
}

// NewSimpleSpell creates a new simple spell.
func NewSimpleSpell(config SimpleSpellConfig) *SimpleSpell {
	return &SimpleSpell{
		id:            config.ID,
		name:          config.Name,
		level:         config.Level,
		school:        config.School,
		castingTime:   config.CastingTime,
		rangeValue:    config.Range,
		duration:      config.Duration,
		description:   config.Description,
		components:    config.Components,
		ritual:        config.Ritual,
		concentration: config.Concentration,
		upcastable:    config.Upcastable,
		targetType:    config.TargetType,
		aoe:           config.AreaOfEffect,
		maxTargets:    config.MaxTargets,
		castFunc:      config.CastFunc,
	}
}

// Entity interface

// GetID returns the unique identifier of the spell.
func (s *SimpleSpell) GetID() string { return s.id }

// GetName returns the name of the spell.
func (s *SimpleSpell) GetName() string { return s.name }

// GetType returns the entity type.
func (s *SimpleSpell) GetType() string { return "spell" }

// Spell interface

// Level returns the spell level (0 for cantrips, 1-9 for leveled spells).
func (s *SimpleSpell) Level() int { return s.level }

// School returns the school of magic (e.g., "evocation", "necromancy").
func (s *SimpleSpell) School() string { return s.school }

// CastingTime returns the time required to cast the spell.
func (s *SimpleSpell) CastingTime() time.Duration { return s.castingTime }

// Range returns the spell's range in feet (-1 for self, 0 for touch).
func (s *SimpleSpell) Range() int { return s.rangeValue }

// Duration returns how long the spell's effects last.
func (s *SimpleSpell) Duration() events.Duration { return s.duration }

// Description returns the spell's descriptive text.
func (s *SimpleSpell) Description() string { return s.description }

// Components returns the components required to cast the spell.
func (s *SimpleSpell) Components() CastingComponents { return s.components }

// IsRitual returns true if the spell can be cast as a ritual.
func (s *SimpleSpell) IsRitual() bool { return s.ritual }

// RequiresConcentration returns true if the spell requires concentration.
func (s *SimpleSpell) RequiresConcentration() bool { return s.concentration }

// CanBeUpcast returns true if the spell can be cast using a higher level slot.
func (s *SimpleSpell) CanBeUpcast() bool { return s.upcastable }

// TargetType returns how the spell targets entities.
func (s *SimpleSpell) TargetType() TargetType { return s.targetType }

// AreaOfEffect returns the area of effect details, or nil if not an area spell.
func (s *SimpleSpell) AreaOfEffect() *AreaOfEffect { return s.aoe }

// MaxTargets returns the maximum number of targets (-1 for unlimited).
func (s *SimpleSpell) MaxTargets() int { return s.maxTargets }

// Cast executes the spell.
func (s *SimpleSpell) Cast(castContext CastContext) error {
	// Validate slot level
	if castContext.SlotLevel < s.level {
		return fmt.Errorf("spell requires at least level %d slot", s.level)
	}

	// Publish cast attempt event
	attemptEvent := events.NewGameEvent(
		EventSpellCastAttempt,
		castContext.Caster,
		nil,
	)
	attemptEvent.Context().Set("spell", s)
	attemptEvent.Context().Set("slotLevel", castContext.SlotLevel)

	// Get the context from metadata if available
	ctx, ok := castContext.Metadata["ctx"].(context.Context)
	if !ok {
		ctx = context.TODO()
	}
	if err := castContext.Bus.Publish(ctx, attemptEvent); err != nil {
		return fmt.Errorf("failed to publish cast attempt event: %w", err)
	}

	// Publish cast start event
	startEvent := events.NewGameEvent(
		EventSpellCastStart,
		castContext.Caster,
		nil,
	)
	startEvent.Context().Set("spell", s)
	startEvent.Context().Set("targets", castContext.Targets)
	startEvent.Context().Set("slotLevel", castContext.SlotLevel)

	if err := castContext.Bus.Publish(ctx, startEvent); err != nil {
		return fmt.Errorf("failed to publish cast start event: %w", err)
	}

	// Execute the spell effect
	if s.castFunc != nil {
		if err := s.castFunc(castContext); err != nil {
			// Publish cast failed event
			failedEvent := events.NewGameEvent(
				EventSpellCastFailed,
				castContext.Caster,
				nil,
			)
			failedEvent.Context().Set("spell", s)
			failedEvent.Context().Set("targets", castContext.Targets)
			failedEvent.Context().Set("slotLevel", castContext.SlotLevel)
			failedEvent.Context().Set("error", err)

			if publishErr := castContext.Bus.Publish(ctx, failedEvent); publishErr != nil {
				// Log the publish error but return the original error
				// In practice, you might want to log this somewhere
				_ = publishErr
			}
			return err
		}
	}

	// Publish cast complete event
	completeEvent := events.NewGameEvent(
		EventSpellCastComplete,
		castContext.Caster,
		nil,
	)
	completeEvent.Context().Set("spell", s)
	completeEvent.Context().Set("targets", castContext.Targets)
	completeEvent.Context().Set("slotLevel", castContext.SlotLevel)

	if err := castContext.Bus.Publish(ctx, completeEvent); err != nil {
		return fmt.Errorf("failed to publish cast complete event: %w", err)
	}

	return nil
}
