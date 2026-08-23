// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// tombsource_test.go holds the authored dungeon every test in this package is
// about, in one place, because very different suites need the same bytes:
// dialect_test.go edits it to check what gets refused and where the refusal
// points, golden_test.go compiles it against the atlas version 1 produced, and
// tomb_test.go walks it.
//
// The bytes live in testdata/reference-tomb.yaml rather than in a Go string
// since rpg-project#256: the file IS the fixture rpg-api ships as
// content/reference-tomb.yaml and the builder round-trips, so it has to be a
// file somebody can open. testdata/tomb-second-skeleton.yaml is the
// rpg-project#254 variant — the same tomb with a wall across the hall and the
// second skeleton behind it — so slice 2's fixture lives here once.

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	tombPath           = "testdata/reference-tomb.yaml"
	secondSkeletonPath = "testdata/tomb-second-skeleton.yaml"
)

// tombYAML reads the reference tomb's authored bytes.
func tombYAML(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(tombPath)
	require.NoError(t, err)
	return string(raw)
}
