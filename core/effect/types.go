// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package effect provides interfaces for modifying chains with typed effects.
// Effects encapsulate modifications that can be applied to and removed from chains,
// enabling composition of complex game mechanics.
package effect

import "github.com/KirkDiggler/rpg-toolkit/core/chain"

// Effect modifies a chain of a specific type.
// Effects can be composed to create complex modifications.
type Effect[T any] interface {
	// Apply adds this effect's modifications to the chain.
	Apply(chain chain.Chain[T]) error

	// Remove removes this effect's modifications from the chain.
	Remove(chain chain.Chain[T]) error
}
