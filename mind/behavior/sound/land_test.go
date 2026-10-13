// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package sound_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/sound"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// land_test.go is [sound.Land]'s coverage: a sound lands on every hearer the
// same, names nobody, and is stamped when it was made (rpg-project#569 R4).

// heardBy is what one observer holds on the sounds channel, read back
// through the one projection this package offers.
func heardBy(t *testing.T, store *perception.Perception, who core.EntityID) []sound.Sound {
	t.Helper()

	held, err := store.Held(who)
	require.NoError(t, err)
	sounds, err := sound.Held(held)
	require.NoError(t, err)

	return sounds
}

// EVERY HEARER HOLDS THE SAME REPORT, AND IT NAMES NOBODY. A deed is told to
// each witness in its own terms because a deed has figures in it; a sound has
// none, so there is nothing for one hearer to know that another does not.
func TestASoundLandsTheSameOnEveryHearerAndNamesNobody(t *testing.T) {
	store, err := perception.New()
	require.NoError(t, err)

	clang := sound.Sound{Kind: "clang", Where: spatial.Position{X: 4, Y: 7}, At: 9}
	require.NoError(t, sound.Land(&sound.LandInput{
		Store:   store,
		Sound:   clang,
		Hearers: []core.EntityID{"guard", "knight", "cook"},
	}))

	for _, hearer := range []core.EntityID{"guard", "knight", "cook"} {
		t.Run(string(hearer), func(t *testing.T) {
			require.Equal(t, []sound.Sound{clang}, heardBy(t, store, hearer),
				"the kind, where it was, and At exactly as given")

			held, err := store.Held(hearer)
			require.NoError(t, err)
			require.Len(t, held, 1)
			require.Equal(t, sound.Channel, held[0].Channel)
			require.Empty(t, held[0].CurrentVia, "a sound is over the moment it is heard")

			var payload map[string]any
			require.NoError(t, json.Unmarshal(held[0].Payload, &payload))
			require.NotContains(t, payload, "actor", "a sound never names its maker")
			require.NotContains(t, payload, "target", "and has nobody it was done to")
		})
	}

	require.Empty(t, heardBy(t, store, "sentry"), "somebody nobody named as a hearer heard nothing")
}

// A sound with no kind is refused: the kind is what a `heard` condition
// matches and the subject it is filed under.
func TestASoundWithNoKindIsRefused(t *testing.T) {
	store, err := perception.New()
	require.NoError(t, err)

	err = sound.Land(&sound.LandInput{Store: store, Sound: sound.Sound{At: 1}, Hearers: []core.EntityID{"guard"}})
	require.ErrorIs(t, err, sound.ErrNoKind)
	require.Empty(t, heardBy(t, store, "guard"), "a refused landing writes nothing")
}

// THE FRESHEST OF A KIND IS WHAT IS HELD, the deeds channel's own narrowing:
// a `heard` condition asks how long ago that kind was last heard, and a second
// clang answers it better than the first.
func TestASecondSoundOfOneKindReplacesTheFirst(t *testing.T) {
	store, err := perception.New()
	require.NoError(t, err)

	for _, s := range []sound.Sound{
		{Kind: "clang", Where: spatial.Position{X: 1, Y: 1}, At: 2},
		{Kind: "voice", Where: spatial.Position{X: 3, Y: 3}, At: 3},
		{Kind: "clang", Where: spatial.Position{X: 5, Y: 5}, At: 4},
	} {
		require.NoError(t, sound.Land(&sound.LandInput{Store: store, Sound: s, Hearers: []core.EntityID{"guard"}}))
	}

	require.ElementsMatch(t, []sound.Sound{
		{Kind: "voice", Where: spatial.Position{X: 3, Y: 3}, At: 3},
		{Kind: "clang", Where: spatial.Position{X: 5, Y: 5}, At: 4},
	}, heardBy(t, store, "guard"))
}
