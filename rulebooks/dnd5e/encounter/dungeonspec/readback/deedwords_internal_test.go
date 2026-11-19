// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
)

// DeedWordsSuite pins the sentence a row's deed condition reads as. A deed
// lands only on whoever saw its doer, so the sentence says "saw", never
// "noted" ("noted" is a topic's word; rpg-project#997).
type DeedWordsSuite struct{ suite.Suite }

func TestDeedWordsSuite(t *testing.T) { suite.Run(t, new(DeedWordsSuite)) }

func (s *DeedWordsSuite) TestADeedIsSaw() {
	b := &builder{names: map[string]string{"king": "King (king)"}}
	cases := []struct {
		name string
		when behavior.When
		want string
	}{
		{"done to it", behavior.When{Deed: "attacked", Within: 2},
			"the guard saw the deed `attacked` done to the guard within the last 2 rounds"},
		{"done to its side", behavior.When{Deed: "fell", Within: 3, Scope: behavior.ScopeAlly},
			"the guard saw the deed `fell` done to a creature on the guard's side within the last 3 rounds"},
		{"done to a named ally", behavior.When{Deed: "fell", Within: 3, Who: "king", Scope: behavior.ScopeAlly},
			"the guard saw the deed `fell` done to King (king), on the guard's side, within the last 3 rounds"},
		{"did it itself", behavior.When{Deed: "fell", Within: 3, Scope: behavior.ScopeActor},
			"the guard did the deed `fell` itself within the last 3 rounds"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, _ := b.whenWords(tc.when, "the guard")
			s.Equal(tc.want, got)
		})
	}
}
