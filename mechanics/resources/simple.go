// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resources

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// SimpleResourceConfig holds the configuration for creating a SimpleResource.
type SimpleResourceConfig struct {
	ID      string
	Type    ResourceType
	Owner   core.Entity
	Key     string
	Current int
	Maximum int

	// Restoration configuration
	ShortRestRestore int             // Amount restored on short rest
	LongRestRestore  int             // Amount restored on long rest (-1 for full)
	RestoreType      RestorationType // Primary restoration method
}

// SimpleResource provides a basic implementation of the Resource interface.
type SimpleResource struct {
	id      string
	typ     ResourceType
	owner   core.Entity
	key     string
	current int
	maximum int

	// Restoration configuration
	shortRestRestore int
	longRestRestore  int
	restoreType      RestorationType
}

// NewSimpleResource creates a new simple resource from a config.
func NewSimpleResource(cfg SimpleResourceConfig) *SimpleResource {
	// Ensure current doesn't exceed maximum
	if cfg.Current > cfg.Maximum {
		cfg.Current = cfg.Maximum
	}

	// Default long rest restore to full if not specified
	if cfg.LongRestRestore == 0 && cfg.RestoreType == RestoreLongRest {
		cfg.LongRestRestore = -1 // Full restore
	}

	return &SimpleResource{
		id:               cfg.ID,
		typ:              cfg.Type,
		owner:            cfg.Owner,
		key:              cfg.Key,
		current:          cfg.Current,
		maximum:          cfg.Maximum,
		shortRestRestore: cfg.ShortRestRestore,
		longRestRestore:  cfg.LongRestRestore,
		restoreType:      cfg.RestoreType,
	}
}

// GetID implements core.Entity
func (r *SimpleResource) GetID() string { return r.id }

// GetType implements core.Entity
func (r *SimpleResource) GetType() string { return string(r.typ) }

// Owner returns the entity that owns this resource
func (r *SimpleResource) Owner() core.Entity { return r.owner }

// Key returns the resource identifier
func (r *SimpleResource) Key() string { return r.key }

// Current returns the current amount
func (r *SimpleResource) Current() int { return r.current }

// Maximum returns the maximum amount
func (r *SimpleResource) Maximum() int { return r.maximum }

// Consume attempts to use the specified amount of resource
func (r *SimpleResource) Consume(amount int) error {
	if amount < 0 {
		return fmt.Errorf("cannot consume negative amount: %d", amount)
	}

	if amount > r.current {
		return fmt.Errorf("insufficient %s: have %d, need %d", r.key, r.current, amount)
	}

	r.current -= amount
	return nil
}

// Restore adds the specified amount to the resource
func (r *SimpleResource) Restore(amount int) {
	if amount < 0 {
		return // Ignore negative amounts
	}

	r.current += amount
	if r.current > r.maximum {
		r.current = r.maximum
	}
}

// SetCurrent sets the current value directly
func (r *SimpleResource) SetCurrent(value int) {
	if value < 0 {
		value = 0
	}
	if value > r.maximum {
		value = r.maximum
	}
	r.current = value
}

// SetMaximum sets the maximum value
func (r *SimpleResource) SetMaximum(value int) {
	if value < 0 {
		value = 0
	}
	r.maximum = value

	// Adjust current if it exceeds new maximum
	if r.current > r.maximum {
		r.current = r.maximum
	}
}

// RestoreOnShortRest returns the amount restored on a short rest
func (r *SimpleResource) RestoreOnShortRest() int {
	if r.restoreType == RestoreShortRest || r.shortRestRestore > 0 {
		if r.shortRestRestore == -1 {
			return r.maximum - r.current // Full restore
		}
		return r.shortRestRestore
	}
	return 0
}

// RestoreOnLongRest returns the amount restored on a long rest
func (r *SimpleResource) RestoreOnLongRest() int {
	if r.restoreType == RestoreLongRest || r.restoreType == RestoreShortRest || r.longRestRestore != 0 {
		if r.longRestRestore == -1 {
			return r.maximum - r.current // Full restore
		}
		return r.longRestRestore
	}
	return 0
}

// IsAvailable returns true if any resource is available
func (r *SimpleResource) IsAvailable() bool {
	return r.current > 0
}
