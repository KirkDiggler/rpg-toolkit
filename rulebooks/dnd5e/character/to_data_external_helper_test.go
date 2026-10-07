// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
)

// mustToData is the external test package's twin of the internal helper of
// the same name: serialize a sheet, failing the test when ToData refuses.
func mustToData(tb testing.TB, c *character.Character) *character.Data {
	tb.Helper()
	data, err := c.ToData()
	require.NoError(tb, err)

	return data
}
