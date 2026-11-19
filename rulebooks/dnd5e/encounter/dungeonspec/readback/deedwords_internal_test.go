// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// DeedWordsSuite pins the sentence a row's deed condition reads as. Its verb
// is chosen from every way the deed can reach a creature
// ([encounter.DeedReachOf]); "noted" is a topic's word, never a deed's
// (rpg-project#997).
type DeedWordsSuite struct{ suite.Suite }

func TestDeedWordsSuite(t *testing.T) { suite.Run(t, new(DeedWordsSuite)) }

// TestOneSentencePerVerbForm pins one sentence for each verb form.
func (s *DeedWordsSuite) TestOneSentencePerVerbForm() {
	b := &builder{names: map[string]string{"king": "King (king)"}}
	cases := []struct {
		name string
		when behavior.When
		want string
	}{
		{"seen only: saw", behavior.When{Deed: "attacked", Within: 2},
			"the guard saw the deed `attacked` done to the guard within the last 2 rounds"},
		{"seen only, its side", behavior.When{Deed: "intimidated", Within: 3, Scope: behavior.ScopeAlly},
			"the guard saw the deed `intimidated` done to a creature on the guard's side within the last 3 rounds"},
		{"seen or caused: saw or caused", behavior.When{Deed: "fell", Within: 3, Who: "king", Scope: behavior.ScopeAlly},
			"the guard saw or caused the deed `fell` done to King (king), on the guard's side, within the last 3 rounds"},
		{"its own testimony: fled", behavior.When{Deed: "fled", Within: 3},
			"the guard did the deed `fled` itself within the last 3 rounds"},
		{"the actor scope", behavior.When{Deed: "fell", Within: 3, Scope: behavior.ScopeActor},
			"the guard did the deed `fell` itself within the last 3 rounds"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, _ := b.whenWords(tc.when, "the guard")
			s.Equal(tc.want, got)
		})
	}
}

// TestEveryDeedWordHasAVerb walks every `when` deed word: each reads a verb
// its reach makes true, never the told fallback while no deed is told.
func (s *DeedWordsSuite) TestEveryDeedWordHasAVerb() {
	for _, w := range encounter.WhenDeeds {
		verb, own := deedVerb(w)
		s.True(own || verb == "saw" || verb == "saw or caused", "deed %q reads %q", w, verb)
	}
	verb, own := deedVerb("shoved")
	s.False(own)
	s.Equal("saw or learned of", verb)
}
