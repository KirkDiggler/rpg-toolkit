// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resources

import "fmt"

// Counter tracks a simple count with an optional limit.
// Examples: death saves, concentration checks failed, attacks made.
type Counter struct {
	ID    string // Unique identifier
	Count int    // Current count
	Limit int    // Maximum count (0 = no limit)
}

// NewCounter creates a new counter starting at zero.
func NewCounter(id string, limit int) *Counter {
	return &Counter{
		ID:    id,
		Count: 0,
		Limit: limit,
	}
}

// Increment adds one to the count.
// Returns an error if at the limit.
func (c *Counter) Increment() error {
	if c.Limit > 0 && c.Count >= c.Limit {
		return fmt.Errorf("counter %s at limit: %d", c.ID, c.Limit)
	}
	c.Count++
	return nil
}

// IncrementBy adds the specified amount to the count.
// Returns an error if it would exceed the limit.
func (c *Counter) IncrementBy(amount int) error {
	if amount < 0 {
		return fmt.Errorf("cannot increment by negative amount: %d", amount)
	}
	if c.Limit > 0 && c.Count+amount > c.Limit {
		return fmt.Errorf("counter %s would exceed limit: %d", c.ID, c.Limit)
	}
	c.Count += amount
	return nil
}

// Decrement subtracts one from the count (minimum 0).
func (c *Counter) Decrement() {
	if c.Count > 0 {
		c.Count--
	}
}

// DecrementBy subtracts the specified amount from the count (minimum 0).
func (c *Counter) DecrementBy(amount int) {
	if amount < 0 {
		return // Ignore negative amounts
	}
	c.Count = max(0, c.Count-amount)
}

// Reset sets the count back to zero.
func (c *Counter) Reset() {
	c.Count = 0
}

// SetCount sets the count directly, respecting the limit.
func (c *Counter) SetCount(value int) error {
	if value < 0 {
		return fmt.Errorf("count cannot be negative: %d", value)
	}
	if c.Limit > 0 && value > c.Limit {
		return fmt.Errorf("count %d exceeds limit %d", value, c.Limit)
	}
	c.Count = value
	return nil
}

// AtLimit returns true if the counter is at its limit.
func (c *Counter) AtLimit() bool {
	return c.Limit > 0 && c.Count >= c.Limit
}

// IsZero returns true if the count is zero.
func (c *Counter) IsZero() bool {
	return c.Count == 0
}

// HasLimit returns true if the counter has a limit.
func (c *Counter) HasLimit() bool {
	return c.Limit > 0
}
