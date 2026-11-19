// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

// deedrows_test.go pins the sentence of every row that reads a deed in the
// three content files that have one (rpg-project#987): none was pinned
// before, which is how a false deed sentence passed.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// deedRowFiles are the content files whose rows read a deed.
var deedRowFiles = []string{"world-builder-v4-castle.yaml", "world-builder-v4-front-room.yaml", "world-builder-v4-truce.yaml"}

// deedSaid is how a row's deed condition is said: what it saw, did or
// suffered, and within how long.
var deedSaid = regexp.MustCompile(`(saw someone|saw .* fall|fled from someone|felled|attacked someone|intimidated someone|persuaded someone| fell within|never true:)`)

func TestEveryDeedRowIsPinned(t *testing.T) {
	var out strings.Builder
	for _, name := range deedRowFiles {
		var rb *readback.ReadBack
		if name == filepath.Base(castlePath) {
			_, rb = theCastle(t)
		} else {
			raw, err := os.ReadFile(filepath.Join("..", "testdata", name))
			require.NoError(t, err)
			rb, err = readback.Of(raw)
			require.NoError(t, err)
		}
		rows := 0
		for _, st := range rb.Statements() {
			if st.Kind != readback.KindRoutineRow || !deedSaid.MatchString(st.Sentence) {
				continue
			}
			rows++
			out.WriteString(name + " " + st.Path + "\n\t" + st.Sentence + "\n")
		}
		require.Positive(t, rows, "%s has a row that reads a deed", name)
	}
	golden := filepath.Join("testdata", "deed-rows.golden.txt")
	if *update {
		require.NoError(t, os.WriteFile(golden, []byte(out.String()), 0o600))
	}
	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	require.Equal(t, string(want), out.String())
}
