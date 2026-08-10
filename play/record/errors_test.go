// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record_test

import (
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// TestSentinelsAreDistinct verifies all error sentinels are non-nil and pairwise distinct.
func TestSentinelsAreDistinct(t *testing.T) {
	sentinels := []error{
		record.ErrNilInput,
		record.ErrBadAudience,
		record.ErrBadTag,
		record.ErrNoPayload,
		record.ErrBadSeq,
		record.ErrNoViewer,
		record.ErrInvalidData,
	}

	// Assert each sentinel is non-nil
	for i, sentinel := range sentinels {
		if sentinel == nil {
			t.Errorf("sentinel[%d] is nil", i)
		}
	}

	// Assert pairwise distinctness
	for i := 0; i < len(sentinels); i++ {
		for j := 0; j < len(sentinels); j++ {
			if i != j && errors.Is(sentinels[i], sentinels[j]) {
				t.Errorf("sentinel[%d] and sentinel[%d] are not distinct", i, j)
			}
		}
	}
}
