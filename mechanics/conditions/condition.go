// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package conditions provides the infrastructure for status effects and conditions
// that can be applied to entities in the RPG toolkit.
package conditions

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Condition represents a status effect that affects an entity.
// Conditions modify game behavior through event handlers and can be
// persisted/loaded as data.
//
//go:generate mockgen -destination=mock/mock_condition.go -package=mock github.com/KirkDiggler/rpg-toolkit/mechanics/conditions Condition
type Condition interface {
	// Ref returns the condition's reference for identification
	Ref() *core.Ref

	// Name returns the display name of the condition
	Name() string

	// Description returns what this condition does
	Description() string

	// Target returns the entity this condition affects
	Target() core.Entity

	// Source returns what created this condition (spell, ability, etc)
	Source() string

	// Apply activates the condition's effects
	Apply(target core.Entity, bus events.EventBus, opts ...ApplyOption) error

	// IsActive returns whether the condition is currently applied
	IsActive() bool

	// Remove deactivates the condition's effects
	Remove(bus events.EventBus) error

	// ToJSON serializes the condition state
	ToJSON() (json.RawMessage, error)

	// IsDirty returns true if the condition has unsaved changes
	IsDirty() bool

	// MarkClean marks the condition as having no unsaved changes
	MarkClean()
}
