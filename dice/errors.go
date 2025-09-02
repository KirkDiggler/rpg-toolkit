// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import "errors"

// Common errors returned by the dice package
var (
	// ErrInvalidNotation indicates the dice notation string is invalid
	ErrInvalidNotation = errors.New("dice: invalid notation")

	// ErrNotationNotImplemented indicates notation parsing is not yet implemented
	ErrNotationNotImplemented = errors.New("dice: notation parser not implemented")

	// ErrInvalidDieSize indicates an invalid die size (must be > 0)
	ErrInvalidDieSize = errors.New("dice: invalid die size")

	// ErrInvalidDieCount indicates an invalid die count
	ErrInvalidDieCount = errors.New("dice: invalid die count")

	// ErrNilRoller indicates a nil roller was provided
	ErrNilRoller = errors.New("dice: roller cannot be nil")
)
