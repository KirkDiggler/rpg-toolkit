// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestAnUnrecognisedBeatStaysUnknown pins the mapper's default arm.
//
// Pinned HERE, against kindOf directly, rather than through a verb — because no
// verb can produce it. Every beat the composition emits today has a case, which
// is the point of rpg-toolkit#1038; the arm exists for the beat a LATER
// composition adds, and there is no honest way to drive that from this side of
// the seam without faking the composition itself.
//
// It is load-bearing rather than defensive. Delivering an uninterpretable beat
// keeps a client's sequence gapless, so it can tell "I do not understand this"
// from "I missed something" — dropping it would manufacture a hole and send the
// client into a resync it never needed. That is also what lets a newer
// composition ship a beat this version has never heard of without older clients
// losing their place.
func TestAnUnrecognisedBeatStaysUnknown(t *testing.T) {
	require.Equal(t, EventUnknown, kindOf([]byte(`{"beat":"transmogrified"}`)),
		"a beat this version does not know is delivered, not dropped")
	require.Equal(t, EventUnknown, kindOf([]byte(`{"actor":"alice"}`)),
		"and so is a beat with no name at all")
	require.Equal(t, EventUnknown, kindOf([]byte(`not json`)),
		"including a payload this package cannot even parse")
}

// TestTheOutcomeBeatsAreTheCompositionsOwnStrings guards the coupling from the
// other side.
//
// The seam-level pins drive real scenes and are the evidence that matters —
// attackevents_test.go for the two swings, death_test.go for the third. This
// asks the narrower question those cannot: does kindOf's literal still equal
// the composition's own constant?
//
// It is BUILT FROM the composition's constants rather than from strings typed
// twice, and that is the whole strength of it. kindOf matches on literals, so a
// rename upstream degrades every event of that kind to EventUnknown with
// nothing failing — the silent failure its own doc warns about. A test that
// also spelled the literal out would keep passing through exactly that change.
// Reading the constant means the rename lands here as a red test instead of in
// a game as a table that has stopped narrating.
func TestTheOutcomeBeatsAreTheCompositionsOwnStrings(t *testing.T) {
	cases := []struct {
		kind encounter.OutcomeKind
		want EventKind
	}{
		{encounter.OutcomeStruck, EventStruck},
		{encounter.OutcomeMissed, EventMissed},
		// The third outcome beat. Unlike the two above, no caller can push it
		// in — the composition writes it itself when it notices a body
		// (rpg-toolkit#1077), which is why the beat carries a member rather
		// than an actor and targets.
		{encounter.OutcomeDown, EventDowned},
	}

	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			require.Equal(t, tc.want,
				kindOf([]byte(`{"beat":"`+string(tc.kind)+`","member":"goblin"}`)))
		})
	}
}

// TestTheSeamSaysDownedWhereTheCompositionSaysDown makes Kirk's ruling
// mechanical (rpg-toolkit#1084).
//
// The two strings are DIFFERENT ON PURPOSE and neither may drift toward the
// other, which is why both halves and the gap between them are asserted rather
// than just the one this package owns.
//
//   - The seam publishes "downed", because a bare "down" also reads as PRONE —
//     a posture the rulebook tracks and this package never gates on. This is
//     the wire value rpg-api adopts, so it is contract, not cosmetics, and
//     asserting the CONSTANT alone would not have noticed it changing.
//   - The composition keeps "down", because that kind is persisted in every
//     stored world. Renaming it there is a migration; translating here is a
//     line in kindOf.
//
// The third assertion is the one that earns its keep. Both single-value rows
// pass happily if somebody aligns the two — that is the tidy-looking change
// this asymmetry invites, and it silently either breaks every stored world or
// hands clients back the ambiguous word.
func TestTheSeamSaysDownedWhereTheCompositionSaysDown(t *testing.T) {
	require.Equal(t, EventKind("downed"), EventDowned,
		"the wire value a client branches on, and rpg-api's vocabulary")
	require.Equal(t, encounter.OutcomeKind("down"), encounter.OutcomeDown,
		"the composition's persisted kind, deliberately left alone")
	require.NotEqual(t, string(EventDowned), string(encounter.OutcomeDown),
		"these two must not be aligned: see kindOf, and rpg-toolkit#1084")
}
