// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// notationRegex matches dice notation like "2d6+3", "d20", "3d8-2", etc.
var notationRegex = regexp.MustCompile(`^([+-]?\d*)[dD](\d+)([+-]\d+)?$`)

// ParseNotation parses a dice notation string into a Pool.
// Supports formats like:
//   - "2d6" - roll 2 six-sided dice
//   - "d20" - roll 1 twenty-sided die
//   - "3d8+5" - roll 3 eight-sided dice and add 5
//   - "2d10-3" - roll 2 ten-sided dice and subtract 3
func ParseNotation(notation string) (*Pool, error) {
	notation = strings.TrimSpace(notation)
	if notation == "" {
		return nil, fmt.Errorf("%w: empty notation", ErrInvalidNotation)
	}

	// Handle multiple dice types with + separator
	if strings.Contains(notation, "+") && strings.Contains(notation, "d") {
		return parseComplexNotation(notation)
	}

	// Simple notation with single dice type
	matches := notationRegex.FindStringSubmatch(notation)
	if matches == nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidNotation, notation)
	}

	// Parse count (default to 1 if not specified)
	count := 1
	if matches[1] != "" && matches[1] != "+" && matches[1] != "-" {
		var err error
		count, err = strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid count in %s", ErrInvalidNotation, notation)
		}
	}

	// Parse die size
	size, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid die size in %s", ErrInvalidNotation, notation)
	}
	if size <= 0 {
		return nil, fmt.Errorf("%w: die size must be positive in %s", ErrInvalidDieSize, notation)
	}

	// Parse modifier
	modifier := 0
	if matches[3] != "" {
		modifier, err = strconv.Atoi(matches[3])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid modifier in %s", ErrInvalidNotation, notation)
		}
	}

	return SimplePool(count, size, modifier), nil
}

// parseComplexNotation handles notation with multiple dice types like "2d6+1d4+3"
func parseComplexNotation(notation string) (*Pool, error) {
	parts := strings.Split(notation, "+")
	var dice []Spec
	modifier := 0

	for _, part := range parts {
		part = strings.TrimSpace(part)

		// Check if this part is a dice notation or just a number
		if strings.Contains(part, "d") {
			// Handle negative dice like "-2d6" that might appear after split
			if strings.HasPrefix(part, "-") {
				// This shouldn't happen with + split, but handle it anyway
				continue
			}

			// Parse this dice part
			matches := notationRegex.FindStringSubmatch(part)
			if matches == nil {
				return nil, fmt.Errorf("%w: invalid dice part %s", ErrInvalidNotation, part)
			}

			count := 1
			if matches[1] != "" {
				var err error
				count, err = strconv.Atoi(matches[1])
				if err != nil {
					return nil, fmt.Errorf("%w: invalid count in %s", ErrInvalidNotation, part)
				}
			}

			size, err := strconv.Atoi(matches[2])
			if err != nil {
				return nil, fmt.Errorf("%w: invalid die size in %s", ErrInvalidNotation, part)
			}
			if size <= 0 {
				return nil, fmt.Errorf("%w: die size must be positive in %s", ErrInvalidDieSize, part)
			}

			dice = append(dice, Spec{Count: count, Size: size})

			// Handle any modifier attached to this dice
			if matches[3] != "" {
				mod, err := strconv.Atoi(matches[3])
				if err != nil {
					return nil, fmt.Errorf("%w: invalid modifier in %s", ErrInvalidNotation, part)
				}
				modifier += mod
			}
		} else {
			// It's just a number modifier
			mod, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid modifier %s", ErrInvalidNotation, part)
			}
			modifier += mod
		}
	}

	if len(dice) == 0 {
		return nil, fmt.Errorf("%w: no dice found in %s", ErrInvalidNotation, notation)
	}

	return NewPool(dice, modifier), nil
}

// MustParseNotation parses notation and panics on error.
// Useful for tests and compile-time known notation.
func MustParseNotation(notation string) *Pool {
	pool, err := ParseNotation(notation)
	if err != nil {
		panic(fmt.Sprintf("dice: failed to parse notation %q: %v", notation, err))
	}
	return pool
}
