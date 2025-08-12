// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package features provides a system for implementing character abilities, racial traits, and feats.
package features

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

//go:generate mockgen -destination=mock/mock_feature.go -package=mock github.com/KirkDiggler/rpg-toolkit/mechanics/features Feature

// Feature represents a character ability, racial trait, or feat.
// Features are data-driven - they load from JSON and handle their own behavior.
type Feature interface {
	// Identity
	Ref() *core.Ref         // Unique identifier like "dnd5e:feature:rage"
	Name() string           // Display name
	Description() string    // Human-readable description

	// Activation
	NeedsTarget() bool                                          // Does UI need to ask for a target?
	Activate(owner core.Entity, opts ...ActivateOption) error   // Activate the feature
	IsActive() bool                                             // Is it currently active?

	// Events - features subscribe to and modify game events
	Apply(bus events.EventBus) error    // Subscribe to events
	Remove(bus events.EventBus) error   // Unsubscribe from events

	// Persistence
	ToJSON() (json.RawMessage, error)  // Save feature state
	IsDirty() bool                     // Has state changed since last save?
	MarkClean()                        // Mark as saved
}

// ActivateContext holds options for feature activation.
type ActivateContext struct {
	Target     core.Entity // Optional target
	SpellLevel int         // For spell features
	Position   interface{} // For area effects
}

// ActivateOption configures feature activation.
type ActivateOption func(*ActivateContext)

// WithTarget specifies a target for the activation.
func WithTarget(target core.Entity) ActivateOption {
	return func(ctx *ActivateContext) {
		ctx.Target = target
	}
}

// WithSpellLevel specifies the spell level for casting.
func WithSpellLevel(level int) ActivateOption {
	return func(ctx *ActivateContext) {
		ctx.SpellLevel = level
	}
}

// parseOptions applies activation options to create a context.
func parseOptions(opts ...ActivateOption) *ActivateContext {
	ctx := &ActivateContext{}
	for _, opt := range opts {
		opt(ctx)
	}
	return ctx
}