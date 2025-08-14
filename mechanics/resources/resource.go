// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package resources provides simple resource tracking for games.
// Resources track current/maximum values, counters track simple counts.
package resources

import "fmt"

// Resource tracks a depletable resource with current and maximum values.
// Examples: spell slots, hit points, rage uses, ki points.
type Resource struct {
	ID      string // Unique identifier
	Current int    // Current amount available
	Maximum int    // Maximum amount possible
}

// NewResource creates a new resource at full capacity.
func NewResource(id string, maximum int) *Resource {
	return &Resource{
		ID:      id,
		Current: maximum,
		Maximum: maximum,
	}
}

// Use attempts to consume the specified amount of resource.
// Returns an error if insufficient resources are available.
func (r *Resource) Use(amount int) error {
	if amount < 0 {
		return fmt.Errorf("cannot use negative amount: %d", amount)
	}
	if amount > r.Current {
		return fmt.Errorf("insufficient %s: have %d, need %d", r.ID, r.Current, amount)
	}
	r.Current -= amount
	return nil
}

// Restore adds the specified amount to the resource, up to maximum.
func (r *Resource) Restore(amount int) {
	if amount < 0 {
		return // Ignore negative amounts
	}
	r.Current = min(r.Current+amount, r.Maximum)
}

// RestoreToFull sets the resource back to maximum.
func (r *Resource) RestoreToFull() {
	r.Current = r.Maximum
}

// SetCurrent sets the current value directly, clamped to [0, Maximum].
func (r *Resource) SetCurrent(value int) {
	r.Current = max(0, min(value, r.Maximum))
}

// SetMaximum sets the maximum value and adjusts current if needed.
func (r *Resource) SetMaximum(value int) {
	r.Maximum = max(0, value)
	if r.Current > r.Maximum {
		r.Current = r.Maximum
	}
}

// IsAvailable returns true if any resource is available.
func (r *Resource) IsAvailable() bool {
	return r.Current > 0
}

// IsEmpty returns true if the resource is depleted.
func (r *Resource) IsEmpty() bool {
	return r.Current == 0
}

// IsFull returns true if the resource is at maximum.
func (r *Resource) IsFull() bool {
	return r.Current == r.Maximum
}
