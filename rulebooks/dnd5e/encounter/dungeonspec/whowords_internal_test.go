// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

// whowords_internal_test.go holds the who-words list (rpg-project#1001) to
// the grammar both ways: every word the list allows in a give's slot is
// accepted there, and nothing else is.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func TestTheWhoWordsAreTheGrammars(t *testing.T) {
	refused := func(g GiveSpec, slot string) bool {
		bad := false
		givingOf("give", &g, true, func(at, _ string) { bad = bad || at == "give."+slot })
		return bad
	}
	valid := GiveSpec{Topic: "t", To: string(encounter.RecognitionSpokenTo), About: string(encounter.RecognitionSpeaker)}
	words := append(encounter.WhoWords(), encounter.WhoWord{Word: "nobody-at-all"})
	for _, w := range words {
		g := valid
		g.To = w.Word
		require.Equal(t, w.Allowed == encounter.WhoInGiveTo, !refused(g, "to"), "to: %s", w.Word)
		g = valid
		g.About = w.Word
		require.Equal(t, w.Allowed == encounter.WhoInGiveAbout, !refused(g, "about"), "about: %s", w.Word)
		g = valid
		g.HeardBy = w.Word
		require.Equal(t, w.Allowed == encounter.WhoInGiveHeardBy, !refused(g, "heard_by"), "heard_by: %s", w.Word)
	}
	for _, w := range encounter.WhoWords() {
		if w.Role == encounter.WhoTaker {
			require.NotEmpty(t, w.Way, "%s: a taker says how it comes by the note", w.Word)
		}
	}
}
