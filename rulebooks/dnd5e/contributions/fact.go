// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package contributions

// Scalar bounds facts to detached values, including the rulebook's named enums.
// Pointer, slice, map and live-provider facts cannot enter through this container.
// Field owners still validate domain restrictions such as finite distances.
type Scalar interface {
	~bool | ~int | ~float64 | ~string
}

// Fact preserves known zero/false/empty separately from unknown. Its zero value
// is unknown and carries no meaningful value. It is evaluation data, not a new
// persistence format or a callback that can consult live hidden state.
type Fact[T Scalar] struct {
	value T
	known bool
}

// Known records a supplied fact, including a known negative or empty value.
func Known[T Scalar](value T) Fact[T] { return Fact[T]{value: value, known: true} }

// Unknown records that permitted inputs do not establish this fact.
func Unknown[T Scalar]() Fact[T] { return Fact[T]{} }

// Get returns the value and whether it is established. Consumers must check the
// flag before treating the value as evidence; unknown is never a definite false.
func (f Fact[T]) Get() (T, bool) { return f.value, f.known }
