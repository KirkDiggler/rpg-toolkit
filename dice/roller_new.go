// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

// NewRoller creates a new instance of a dice roller.
// This replaces the global DefaultRoller pattern with dependency injection.
func NewRoller() Roller {
	return &CryptoRoller{}
}

// NewMockableRoller creates a roller that can be easily mocked for testing.
// In tests, you can pass a mock that implements the Roller interface.
func NewMockableRoller(r Roller) Roller {
	if r == nil {
		return NewRoller()
	}
	return r
}
