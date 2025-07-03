// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

// ModifierValue represents any value that can modify a game mechanic.
// Implementations should be immutable after creation.
type ModifierValue interface {
	// GetValue returns the numeric value to apply.
	GetValue() int

	// GetDescription returns how this value was determined,
	// e.g., "+2 (proficiency)" or "+d4[3]=3 (bless)".
	GetDescription() string
}