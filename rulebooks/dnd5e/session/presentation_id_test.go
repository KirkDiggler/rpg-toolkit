// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPresentationIDValidation(t *testing.T) {
	t.Parallel()

	for _, id := range []string{
		"opaque-123", "with_underscore", "with.period", "with~tilde",
		strings.Repeat("a", maxPresentationIDBytes),
	} {
		require.NoError(t, validatePresentationID(id), id)
	}

	for _, id := range []string{
		"", "has space", "has/slash", "has+plus", "line\nbreak", "snowman-☃",
		strings.Repeat("a", maxPresentationIDBytes+1),
	} {
		require.Error(t, validatePresentationID(id), id)
	}
}
