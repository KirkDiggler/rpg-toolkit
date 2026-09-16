// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Every word the rulebook ships means one Retaliator under a profile, and
// the profile is what its doc says it is.
//
// This is the only test written from INSIDE the package, and it is inside
// because the numbers are: a preset is not reachable from a driver's answer
// except through a scene built to tell one number from another, and the
// scenes that do that live next door. A doc that drifted from its table
// would be caught here and nowhere else.
func TestEachWordsPresetIsWhatItsDocSays(t *testing.T) {
	// A catalog that calls exactly one absurd thing ranged, so a preset
	// holding the driver's knob is distinguishable from one that fell back
	// to the rulebook's own weapons.
	catalog := func(item string) bool { return item == "a thrown chair" }

	driver, err := NewMinded(&NewMindedInput{Ranged: catalog})
	require.NoError(t, err)

	cases := []struct {
		word   string
		grudge Grudge
		fear   Fear
		room   int
	}{
		{word: MindRetaliator, grudge: Grudge{
			Patience: 3, Excuse: ExcuseUnarmed,
			Provokes: []string{encounter.DeedAttack},
		}},
		{word: MindBerserker, grudge: Grudge{
			Patience: 10, Excuse: ExcuseNever,
			Provokes: []string{encounter.DeedAttack, encounter.DeedIntimidate},
		}},
		{word: MindCoward, room: 2, fear: Fear{Patience: 3}},
	}

	require.Len(t, presets, len(cases),
		"a word the rulebook ships and this table does not name is a preset nothing pins: add the row")

	for _, tc := range cases {
		t.Run(tc.word, func(t *testing.T) {
			mind, err := driver.mindFor(tc.word)
			require.NoError(t, err)

			retaliator, ok := mind.(*Retaliator)
			require.True(t, ok, "three words, one mind, three profiles")

			require.Equal(t, tc.grudge, retaliator.Grudge)
			require.Equal(t, tc.fear, retaliator.Fear)
			require.Equal(t, tc.room, retaliator.Room)
			require.Same(t, driver.board, retaliator.Space,
				"geometry is the driver's board and never a preset's")
			require.NotNil(t, retaliator.Ranged)
			require.True(t, retaliator.Ranged("a thrown chair"),
				"the catalog knob is handed to every preset, and it is the driver's own")
		})
	}
}

// The zero Grudge holds nothing, and the zero Excuse is the one that lets
// nobody go.
//
// Both halves are claims the type doc makes, and both are the reason
// Patience is a SPAN. Read as a maximum age instead, a Patience of zero
// would answer a deed of age zero — so the mind with no grudge would hold
// one for exactly one tick, which is the tick anybody is looking.
func TestTheZeroGrudgeIsNoGrudge(t *testing.T) {
	var zero Grudge

	require.Equal(t, ExcuseNever, zero.Excuse,
		"an author who named no way out of a grudge wrote down none")
	require.False(t, zero.fresh(3, 3), "not even the deed that landed this tick")
	require.False(t, zero.fresh(9, 3))
}

// Patience counts the ticks a deed stays worth answering, from the tick it
// was confirmed on.
func TestPatienceIsASpanFromTheTickTheDeedLanded(t *testing.T) {
	cases := []struct {
		scene    string
		patience uint64
		at       uint64
		landed   uint64
		fresh    bool
	}{
		{scene: "the tick it landed", patience: 3, at: 3, landed: 3, fresh: true},
		{scene: "one tick later", patience: 3, at: 4, landed: 3, fresh: true},
		{scene: "two ticks later, the last one it covers", patience: 3, at: 5, landed: 3, fresh: true},
		{scene: "three ticks later, one past the span", patience: 3, at: 6, landed: 3, fresh: false},
		{scene: "a one-tick span covers only its own tick", patience: 1, at: 3, landed: 3, fresh: true},
		{scene: "and nothing after it", patience: 1, at: 4, landed: 3, fresh: false},
		{
			scene:    "testimony confirmed ahead of the situation's clock is as fresh as it gets",
			patience: 1, at: 3, landed: 4, fresh: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.scene, func(t *testing.T) {
			require.Equal(t, tc.fresh, Grudge{Patience: tc.patience}.fresh(tc.at, tc.landed))
		})
	}
}
