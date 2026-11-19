// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestEveryDeedSentence pins the sentence for every deed word in every scope
// (rpg-project#987), and who a row's `actor` selector names with it.
func TestEveryDeedSentence(t *testing.T) {
	b := &builder{names: map[string]string{"king": "The Mad King (king)", "x": "X (x)"}}
	when := func(deed string, scope behavior.DeedScope, who string) behavior.When {
		return behavior.When{Deed: deed, Within: 3, Scope: scope, Who: core.EntityID(who)}
	}
	for _, tc := range []struct {
		when     behavior.When
		sentence string
		actor    string
	}{
		{when("attacked", behavior.ScopeSelf, ""), "X (x) saw someone attack X (x) within the last 3 rounds", "the one who attacked it"},
		{when("attacked", behavior.ScopeAlly, ""), "X (x) saw someone attack a creature on X (x)'s side within the last 3 rounds",
			"the one who attacked a creature on its side"},
		{when("attacked", behavior.ScopeActor, ""), "X (x) attacked someone within the last 3 rounds", "itself"},
		{when("intimidated", behavior.ScopeSelf, ""), "X (x) saw someone intimidate X (x) within the last 3 rounds", "the one who intimidated it"},
		{when("intimidated", behavior.ScopeAlly, ""), "X (x) saw someone intimidate a creature on X (x)'s side within the last 3 rounds",
			"the one who intimidated a creature on its side"},
		{when("intimidated", behavior.ScopeActor, ""), "X (x) intimidated someone within the last 3 rounds", "itself"},
		{when("persuaded", behavior.ScopeSelf, ""), "X (x) saw someone persuade X (x) within the last 3 rounds", "the one who persuaded it"},
		{when("persuaded", behavior.ScopeAlly, ""), "X (x) saw someone persuade a creature on X (x)'s side within the last 3 rounds",
			"the one who persuaded a creature on its side"},
		{when("persuaded", behavior.ScopeActor, ""), "X (x) persuaded someone within the last 3 rounds", "itself"},
		{when("fled", behavior.ScopeSelf, ""), "X (x) fled from someone within the last 3 rounds", "the one it fled from"},
		{when("fled", behavior.ScopeAlly, ""), "never true: `fled` is held only by the creature it was done to, so `on: ally` can never be true",
			"the one it fled from"},
		{when("fled", behavior.ScopeActor, ""),
			"never true: `fled` is held only by the creature it was done to, never by the one who did it, so `as: actor` can never be true", "itself"},
		{when("fell", behavior.ScopeSelf, ""), "X (x) fell within the last 3 rounds", "the one who felled it"},
		{when("fell", behavior.ScopeSelf, "x"), "X (x) fell within the last 3 rounds", "the one who felled it"},
		{when("fell", behavior.ScopeSelf, "king"),
			"X (x) fell within the last 3 rounds; `who` names The Mad King (king), so this is true only if that is X (x)", "the one who felled it"},
		{when("fell", behavior.ScopeAlly, ""), "X (x) saw a creature on X (x)'s side fall within the last 3 rounds",
			"the one who felled a creature on its side"},
		{when("fell", behavior.ScopeAlly, "king"), "X (x) saw The Mad King (king), on X (x)'s side, fall within the last 3 rounds",
			"the one who felled The Mad King (king)"},
		{when("fell", behavior.ScopeActor, ""), "X (x) felled someone, or fell with no blow to explain it, within the last 3 rounds", "itself"},
		{when("fell", behavior.ScopeActor, "king"), "X (x) felled The Mad King (king) and saw it fall within the last 3 rounds", "itself"},
	} {
		name := tc.when.Deed + "/" + string(tc.when.Scope) + "/" + string(tc.when.Who)
		t.Run(name, func(t *testing.T) {
			s, _ := b.whenWords(tc.when, "X (x)")
			require.Equal(t, tc.sentence, s)
			require.NotContains(t, s, "noted")
			require.Equal(t, tc.actor, b.actorWords(behavior.Answer{When: []behavior.When{tc.when}}))
		})
	}
}

// TestEveryDeedWordIsSaid walks every `when` deed word in every scope: each
// has its words and a record of how it reaches a creature, and is said
// without "noted" and without the fallback.
func TestEveryDeedWordIsSaid(t *testing.T) {
	b := &builder{names: map[string]string{}}
	for _, word := range encounter.WhenDeeds {
		_, said := deedVerbs[word]
		require.True(t, said, "the deed word %q has its words", word)
		for _, scope := range encounter.DeedScopes {
			s, _ := b.whenWords(behavior.When{Deed: word, Within: 2, Scope: scope}, "X")
			require.NotContains(t, s, "noted", "%s %s", word, scope)
			require.False(t, strings.Contains(s, "cannot say"), "%s %s: %s", word, scope, s)
		}
	}
	require.Equal(t, "the one who acted", b.actorWords(behavior.Answer{}), "a row with no deed")
}
