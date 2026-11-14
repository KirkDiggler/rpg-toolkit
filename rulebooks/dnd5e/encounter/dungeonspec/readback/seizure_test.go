// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// seizure_test.go holds the arrest's seizure sentence to the engine's rule
// (wanted.go, rpg-project#925): a wanted member is seized within the
// conversation's hail distance, or anywhere in sight when it declares no
// approach. The hail site with an arrest on its gate, compiled at each
// distance: the sentence follows the content.

// arrestingHailSite is the hail site with an arrest on its gate's `admitted`
// state and the approach line given ("" for none).
func arrestingHailSite(t *testing.T, approach string) []byte {
	t.Helper()
	raw, err := os.ReadFile(hailPath)
	require.NoError(t, err)
	site := strings.Replace(string(raw), "    approach: { within: 2 }\n", approach+`    outcomes:
      admitted:
        - arrest:
            cell: region-hall
            area: { regions: [region-hall] }
`, 1)
	require.NotEqual(t, string(raw), site, "precondition: the hail site's approach is authored")
	return []byte(site)
}

// seizure is the arrest's full sentence and the folded text.
func seizure(t *testing.T, approach string) (string, string) {
	t.Helper()
	rb, err := readback.Of(arrestingHailSite(t, approach))
	require.NoError(t, err)
	for _, st := range rb.Statements() {
		if st.Path == "interactions.gate.outcomes.admitted[0].arrest" {
			return st.Sentence, rb.FoldedText()
		}
	}
	t.Fatal("no arrest statement")
	return "", ""
}

func TestTheSeizureSaysTheHailsDistance(t *testing.T) {
	for _, tc := range []struct {
		approach, full, folded string
	}{
		{"    approach: { within: 2 }\n", "sees again within 2 cells (the conversation's hail distance)", "within 2 cells (the conversation's hail"},
		{"    approach: { within: 3 }\n", "sees again within 3 cells (the conversation's hail distance)", "within 3 cells (the conversation's hail"},
		{"", "sees again anywhere in their sight", "sees them again anywhere in their sight"},
	} {
		full, folded := seizure(t, tc.approach)
		require.Contains(t, full, tc.full)
		require.Contains(t, strings.Join(strings.Fields(folded), " "), tc.folded)
	}
}
