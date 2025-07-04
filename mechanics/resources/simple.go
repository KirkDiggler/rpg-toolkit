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

	// Restoration configuration (deprecated - use RestoreTriggers instead)
	ShortRestRestore int             // Amount restored on short rest
	LongRestRestore  int             // Amount restored on long rest (-1 for full)
	RestoreType      RestorationType // Primary restoration method

	// RestoreTriggers maps trigger strings to restoration amounts.
	// Special values:
	//   -1: Restore to full
	//    0: No restoration (or omit from map)
	//   >0: Restore specific amount
	RestoreTriggers map[string]int
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
	restoreTriggers  map[string]int
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

	// Build trigger map from legacy config if not provided
	triggers := cfg.RestoreTriggers
	if triggers == nil {
		triggers = make(map[string]int)
	}

	// Map legacy rest configuration to triggers for backward compatibility
	if cfg.ShortRestRestore != 0 || cfg.RestoreType == RestoreShortRest {
		if _, exists := triggers["short_rest"]; !exists {
			triggers["short_rest"] = cfg.ShortRestRestore
			if triggers["short_rest"] == 0 && cfg.RestoreType == RestoreShortRest {
				triggers["short_rest"] = -1 // Full restore
			}
		}
	}

	if cfg.LongRestRestore != 0 || cfg.RestoreType == RestoreLongRest || cfg.RestoreType == RestoreShortRest {
		if _, exists := triggers["long_rest"]; !exists {
			triggers["long_rest"] = cfg.LongRestRestore
			// Short rest resources also restore on long rest
			if triggers["long_rest"] == 0 && cfg.RestoreType == RestoreShortRest {
				triggers["long_rest"] = cfg.ShortRestRestore
				if triggers["long_rest"] == 0 {
					triggers["long_rest"] = -1 // Full restore
				}
			}
		}
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
		restoreTriggers:  triggers,
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
// Deprecated: Use RestoreOnTrigger("short_rest") instead
func (r *SimpleResource) RestoreOnShortRest() int {
	return r.RestoreOnTrigger("short_rest")
}

// RestoreOnLongRest returns the amount restored on a long rest
// Deprecated: Use RestoreOnTrigger("long_rest") instead
func (r *SimpleResource) RestoreOnLongRest() int {
	return r.RestoreOnTrigger("long_rest")
}

// RestoreOnTrigger returns the amount to restore for a given trigger
func (r *SimpleResource) RestoreOnTrigger(trigger string) int {
	amount, exists := r.restoreTriggers[trigger]
	if !exists {
		return 0
	}

	// Special value: -1 means restore to full
	if amount == -1 {
		return r.maximum - r.current
	}

	return amount
}

// IsAvailable returns true if any resource is available
func (r *SimpleResource) IsAvailable() bool {
	return r.current > 0
}
