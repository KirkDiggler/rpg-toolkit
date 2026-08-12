// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/stretchr/testify/assert"
)

func TestSentinelsAreDistinct(t *testing.T) {
	sentinels := []error{
		encounter.ErrNilInput,
		encounter.ErrNoMember,
		encounter.ErrNotMember,
		encounter.ErrNoEnding,
		encounter.ErrClosed,
		encounter.ErrNoField,
		encounter.ErrBadPlacement,
		encounter.ErrBadConnection,
		encounter.ErrNoConnection,
		encounter.ErrInvalidData,
	}

	// Each sentinel must be non-nil
	for i, sentinel := range sentinels {
		assert.NotNil(t, sentinel, "sentinel at index %d is nil", i)
	}

	// Pairwise distinct via errors.Is
	for i, sentinelI := range sentinels {
		for j, sentinelJ := range sentinels {
			if i == j {
				// Same sentinel should match itself
				assert.True(t, errors.Is(sentinelI, sentinelJ),
					"sentinel at index %d should match itself", i)
			} else {
				// Different sentinels should not match
				assert.False(t, errors.Is(sentinelI, sentinelJ),
					"sentinel at index %d should not match sentinel at index %d", i, j)
			}
		}
	}
}
