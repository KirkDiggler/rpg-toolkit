// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package chain provides interfaces for ordered processing of data through stages.
// Chains allow modifications to be applied in a predictable order, making complex
// game mechanics composable and testable.
package chain

import "context"

// Stage represents a processing stage in a chain.
// Stages determine the order of execution for modifications.
type Stage string

// Chain processes data through ordered stages of modifications.
// Each modification transforms the data and passes it to the next.
type Chain[T any] interface {
	// Add registers a modifier at the specified stage with a unique ID.
	// Returns an error if the ID already exists.
	Add(stage Stage, id string, modifier func(context.Context, T) (T, error)) error

	// Remove unregisters a modifier by its ID.
	// Returns an error if the ID does not exist.
	Remove(id string) error

	// Execute runs all modifiers in stage order, transforming the data.
	Execute(ctx context.Context, data T) (T, error)
}
