// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dice

import (
	"fmt"
	"strings"
)

// Result represents the outcome of rolling a dice pool
type Result struct {
	pool     *Pool   // The pool that was rolled
	rolls    [][]int // Actual rolls grouped by dice spec
	modifier int     // Static modifier
	total    int     // Total result
	err      error   // Any error that occurred
}

// Total returns the total value of the roll
func (r *Result) Total() int {
	return r.total
}

// Rolls returns the individual dice rolls
func (r *Result) Rolls() [][]int {
	return r.rolls
}

// Modifier returns the static modifier
func (r *Result) Modifier() int {
	return r.modifier
}

// Error returns any error that occurred during rolling
func (r *Result) Error() error {
	return r.err
}

// Description returns a formatted description of the roll
// Format: "2d6+3: [4,2]+3 = 9" or "3d8: [7,2,5] = 14"
func (r *Result) Description() string {
	if r.err != nil {
		return fmt.Sprintf("ERROR: %v", r.err)
	}

	var parts []string

	// Build the roll details
	for i, group := range r.rolls {
		if len(group) == 0 {
			continue
		}

		// Convert rolls to strings
		rollStrs := make([]string, len(group))
		for j, roll := range group {
			rollStrs[j] = fmt.Sprintf("%d", roll)
		}

		// Format based on count
		spec := r.pool.dice[i]
		if spec.Count == 1 {
			parts = append(parts, fmt.Sprintf("d%d:[%s]", spec.Size, strings.Join(rollStrs, ",")))
		} else {
			parts = append(parts, fmt.Sprintf("%dd%d:[%s]", spec.Count, spec.Size, strings.Join(rollStrs, ",")))
		}
	}

	// Add modifier if present
	result := strings.Join(parts, " + ")
	if r.modifier > 0 {
		result = fmt.Sprintf("%s + %d", result, r.modifier)
	} else if r.modifier < 0 {
		result = fmt.Sprintf("%s - %d", result, -r.modifier)
	}

	// Add total
	return fmt.Sprintf("%s = %d", result, r.total)
}

// String implements Stringer interface
func (r *Result) String() string {
	return r.Description()
}
