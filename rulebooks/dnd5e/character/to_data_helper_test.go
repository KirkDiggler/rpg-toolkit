// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// mustToData serializes a sheet a test built, failing the test when ToData
// refuses. ToData is fallible — a feature or condition that cannot serialize
// fails the whole write — and a test that only wants the record is not the
// test of that refusal.
func mustToData(tb testing.TB, c *Character) *Data {
	tb.Helper()
	data, err := c.ToData()
	require.NoError(tb, err)

	return data
}
