// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import (
	"context"
)

// Lazy represents a dice roll that is evaluated fresh each time.
// Unlike Roll which caches its result, Lazy rolls fresh dice every time GetValue() is called.
// This is essential for effects like Bless that add dice to each attack.
type Lazy struct {
	pool   *Pool
	roller Roller
}

// NewLazy creates a new lazy dice roll from a Pool.
func NewLazy(pool *Pool) *Lazy {
	return &Lazy{
		pool:   pool,
		roller: NewRoller(),
	}
}

// NewLazyWithRoller creates a new lazy dice roll with a specific roller.
func NewLazyWithRoller(pool *Pool, roller Roller) *Lazy {
	if roller == nil {
		roller = NewRoller()
	}
	return &Lazy{
		pool:   pool,
		roller: roller,
	}
}

// LazyFromNotation creates a lazy roll from a dice notation string.
func LazyFromNotation(notation string) (*Lazy, error) {
	pool, err := ParseNotation(notation)
	if err != nil {
		return nil, err
	}
	return NewLazy(pool), nil
}

// GetValue rolls the dice fresh and returns the total.
// Each call produces a new random result.
func (l *Lazy) GetValue() int {
	return l.GetValueWithContext(context.Background())
}

// GetValueWithContext rolls the dice fresh with context support.
func (l *Lazy) GetValueWithContext(ctx context.Context) int {
	result := l.pool.RollContext(ctx, l.roller)
	if result.Error() != nil {
		return 0
	}
	return result.Total()
}

// GetDescription returns a description of the most recent roll.
func (l *Lazy) GetDescription() string {
	return l.GetDescriptionWithContext(context.Background())
}

// GetDescriptionWithContext returns a description with context support.
func (l *Lazy) GetDescriptionWithContext(ctx context.Context) string {
	result := l.pool.RollContext(ctx, l.roller)
	if result.Error() != nil {
		return "ERROR: " + result.Error().Error()
	}
	// Add + prefix for positive values to match modifier format
	if result.Total() >= 0 {
		return "+" + result.Description()
	}
	return result.Description()
}

// Pool returns the underlying dice pool configuration.
func (l *Lazy) Pool() *Pool {
	return l.pool
}
