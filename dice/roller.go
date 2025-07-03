// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// Roller is the interface for random number generation in the dice package.
// Implementations must be safe for concurrent use.
type Roller interface {
	// Roll returns a random number from 1 to size (inclusive).
	// Panics if size <= 0.
	Roll(size int) int

	// RollN rolls count dice of the given size.
	// Returns a slice containing each individual roll result.
	// Panics if size <= 0 or count < 0.
	RollN(count, size int) []int
}

// CryptoRoller implements Roller using crypto/rand for cryptographically secure randomness.
type CryptoRoller struct{}

// Roll returns a cryptographically secure random number from 1 to size.
func (c *CryptoRoller) Roll(size int) int {
	if size <= 0 {
		panic(fmt.Sprintf("dice: invalid die size %d", size))
	}

	// crypto/rand.Int returns [0, max), so we use size as max to get [0, size-1]
	// then add 1 to get [1, size]
	n, err := rand.Int(rand.Reader, big.NewInt(int64(size)))
	if err != nil {
		panic(fmt.Sprintf("dice: crypto/rand error: %v", err))
	}

	return int(n.Int64()) + 1
}

// RollN rolls multiple dice using crypto/rand.
func (c *CryptoRoller) RollN(count, size int) []int {
	if size <= 0 {
		panic(fmt.Sprintf("dice: invalid die size %d", size))
	}
	if count < 0 {
		panic(fmt.Sprintf("dice: invalid die count %d", count))
	}

	results := make([]int, count)
	for i := 0; i < count; i++ {
		results[i] = c.Roll(size)
	}
	return results
}

// DefaultRoller is the default roller using crypto/rand.
var DefaultRoller Roller = &CryptoRoller{}

// SetDefaultRoller allows changing the default roller (primarily for testing).
// This function is not safe for concurrent use with other dice operations.
func SetDefaultRoller(r Roller) {
	DefaultRoller = r
}