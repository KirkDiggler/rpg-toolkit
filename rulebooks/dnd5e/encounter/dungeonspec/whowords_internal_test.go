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
	// ONE ENTRY PER (WORD, PLACE) (rpg-project#990): a word may be written in
	// more than one place, so the claim is made per place — every word the
	// list holds anywhere, and one it holds nowhere, is accepted at a place
	// if and only if the list has an entry for it there.
	set := map[string]func(g *GiveSpec, word string){
		encounter.WhoInGiveTo:      func(g *GiveSpec, word string) { g.To = word },
		encounter.WhoInGiveAbout:   func(g *GiveSpec, word string) { g.About = word },
		encounter.WhoInGiveHeardBy: func(g *GiveSpec, word string) { g.HeardBy = word },
	}
	slot := map[string]string{encounter.WhoInGiveTo: "to", encounter.WhoInGiveAbout: "about", encounter.WhoInGiveHeardBy: "heard_by"}
	words := []string{"nobody-at-all"}
	for _, w := range encounter.WhoWords() {
		words = append(words, w.Word)
	}
	for place, write := range set {
		for _, word := range words {
			g := valid
			write(&g, word)
			_, listed := encounter.WhoWordAt(place, word)
			require.Equal(t, listed, !refused(g, slot[place]), "%s: %s", place, word)
		}
	}
	for _, w := range encounter.WhoWords() {
		if w.Role == encounter.WhoTaker {
			require.NotEmpty(t, w.Way, "%s: a taker says how it comes by the note", w.Word)
		}
	}
}
