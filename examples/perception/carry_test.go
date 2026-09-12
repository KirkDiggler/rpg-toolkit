// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package perception_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/belief"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/carry"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/content"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/projection"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/reconcile"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// The dungeon run is ephemeral and so is the party. What persists is a player,
// and these are the tests for what reaches one.

const (
	// playerBram is who keeps what bram learned, after the party is gone.
	playerBram testimony.Observer = "player:bram"
	// playerPip likewise.
	playerPip testimony.Observer = "player:pip"

	tunnels = "eastern-tunnels"

	goblinBand  = "goblin-band-2"
	easternName = belief.Name("goblins in the eastern tunnels")
)

// aRunWhereBramFindsGoblins plays one dungeon run and hands back the game.
func aRunWhereBramFindsGoblins(t *testing.T, at testimony.Stamp) *perception.Game {
	t.Helper()

	g := perception.NewGame()
	g.Mind(bram, reconcile.Woodwise{})

	band := projection.Presence{
		Source: goblinBand,
		Where:  tunnels,
		Says: map[testimony.Channel]projection.Says{
			sight:   says(content.Creature, "goblin", "a dozen of them", tunnels),
			hearing: says(content.Noise, "", "chanting", tunnels),
		},
	}

	_, err := g.Tick(projection.Input{
		Presences: []projection.Presence{band},
		Senses: append(
			senses(sight, []string{tunnels}, bram),
			senses(hearing, []string{tunnels}, bram)...),
		At: at,
	})
	require.NoError(t, err)

	return g
}

// TestTheDistillation proves what leaves a finished run: a conclusion keyed to
// a name, held and not current, with the raw run-local handles left behind.
func TestTheDistillation(t *testing.T) {
	g := aRunWhereBramFindsGoblins(t, 10)

	seen := handle(sight, goblinBand)
	heard := handle(hearing, goblinBand)

	// Bram works out what he is looking at. Both channels, one name.
	require.NoError(t, g.Identify(bram, seen, easternName, 10))
	require.NoError(t, g.Identify(bram, heard, easternName, 10))

	out := carry.Out(carry.Input{Observer: bram, Tracks: g.Held(bram), Names: g})
	assert.Empty(t, out.Unnamed)
	assert.Empty(t, out.Superseded)

	// Both fidelities travel, separately. One name, two channels.
	require.Equal(t, 2, len(out.Carried))
	assert.Equal(t, easternName, out.Carried[0].Name)
	assert.Equal(t, hearing, out.Carried[0].Channel)
	assert.Equal(t, sight, out.Carried[1].Channel)

	// The run is over. The player keeps what they concluded.
	keeping, kept := testimony.New(), belief.New()
	require.NoError(t, carry.Land(keeping, kept, playerBram, out.Carried))

	carriedSight, ok := keeping.Track(playerBram, carry.Handle(easternName, sight))
	require.True(t, ok)
	assert.False(t, carriedSight.Current,
		"a carried memory is a ghost, because that is exactly what it is")
	assert.Equal(t, "a dozen of them", mustRead(t, carriedSight.Latest().Payload).Note)
	assert.Equal(t, testimony.Stamp(10), carriedSight.Latest().Confirmed,
		"it is as old as it really is")

	carriedHearing, ok := keeping.Track(playerBram, carry.Handle(easternName, hearing))
	require.True(t, ok)
	assert.Equal(t, "chanting", mustRead(t, carriedHearing.Latest().Payload).Note)

	// The run-local handles do not travel and mean nothing here.
	_, ok = keeping.Track(playerBram, seen)
	assert.False(t, ok, "raw testimony belongs to the run that produced it")

	_, ok = keeping.Track(playerBram, heard)
	assert.False(t, ok)
}

// TestTheUnnamedCannotTravel proves naming is the gate. A contact with no word
// for it has nothing to be filed under, and saying so is part of the result.
func TestTheUnnamedCannotTravel(t *testing.T) {
	g := aRunWhereBramFindsGoblins(t, 10)

	seen := handle(sight, goblinBand)
	heard := handle(hearing, goblinBand)

	// He knows what he saw. He never worked out the chanting.
	require.NoError(t, g.Identify(bram, seen, easternName, 10))

	out := carry.Out(carry.Input{Observer: bram, Tracks: g.Held(bram), Names: g})
	assert.Equal(t, []testimony.TrackID{heard}, out.Unnamed,
		"refused out loud, not dropped quietly")
	require.Equal(t, 1, len(out.Carried))
	assert.Equal(t, sight, out.Carried[0].Channel)

	keeping, kept := testimony.New(), belief.New()
	require.NoError(t, carry.Land(keeping, kept, playerBram, out.Carried))

	_, ok := keeping.Track(playerBram, carry.Handle(easternName, hearing))
	assert.False(t, ok, "nothing invented a name for him")

	// Naming it is the only thing that changes the answer.
	require.NoError(t, g.Identify(bram, heard, easternName, 11))

	out = carry.Out(carry.Input{Observer: bram, Tracks: g.Held(bram), Names: g})
	assert.Empty(t, out.Unnamed)
	require.NoError(t, carry.Land(keeping, kept, playerBram, out.Carried))

	_, ok = keeping.Track(playerBram, carry.Handle(easternName, hearing))
	assert.True(t, ok)
}

// TestCarryingDoesNotLaunderALie is the one that matters. Pip is charmed, names
// what he was shown, and takes it home. Nothing in the crossing compared it to
// anything.
func TestCarryingDoesNotLaunderALie(t *testing.T) {
	g := perception.NewGame()
	g.Mind(pip, reconcile.Oblivious{})

	chief := projection.Presence{
		Source: goblinBand,
		Where:  tunnels,
		Says: map[testimony.Channel]projection.Says{
			sight:   says(content.Creature, "goblin", "holding a sword", tunnels),
			hearing: says(content.Noise, "", "a blade on stone", tunnels),
		},
	}
	charm := projection.Forgery{
		Where:     tunnels,
		Source:    "charm-on-pip",
		Observers: []testimony.Observer{pip},
		Says: map[testimony.Channel]projection.Says{
			sight: says(content.Creature, "goblin", "holding a toy", tunnels),
		},
		Hides:    []testimony.Channel{sight},
		Replaces: goblinBand,
	}

	in := projection.Input{
		Presences: []projection.Presence{chief},
		Forgeries: []projection.Forgery{charm},
		Senses: append(
			senses(sight, []string{tunnels}, pip),
			senses(hearing, []string{tunnels}, pip)...),
		At: 20,
	}

	_, err := g.Tick(in)
	require.NoError(t, err)

	theChief := belief.Name("the goblin chief")
	forged := handle(sight, "charm-on-pip")
	heard := handle(hearing, goblinBand)

	require.NoError(t, g.Identify(pip, forged, theChief, 20))
	require.NoError(t, g.Identify(pip, heard, theChief, 20))

	out := carry.Out(carry.Input{Observer: pip, Tracks: g.Held(pip), Names: g})

	keeping, kept := testimony.New(), belief.New()
	require.NoError(t, carry.Land(keeping, kept, playerPip, out.Carried))

	carriedSight, ok := keeping.Track(playerPip, carry.Handle(theChief, sight))
	require.True(t, ok)
	assert.Equal(t, "holding a toy", mustRead(t, carriedSight.Latest().Payload).Note,
		"the lie crossed intact — nothing on this path could tell")

	// Meanwhile the truth surface never held the toy at all.
	for _, p := range in.Truth() {
		said, decodeErr := content.Decode(p.Says[sight].Payload)
		require.NoError(t, decodeErr)
		assert.Equal(t, "holding a sword", said.Note)
	}

	// And pip carries out a contradiction under one name, on two channels,
	// side by side and unresolved: a toy he saw and a blade he heard.
	carriedHearing, ok := keeping.Track(playerPip, carry.Handle(theChief, hearing))
	require.True(t, ok)
	assert.Equal(t, "a blade on stone", mustRead(t, carriedHearing.Latest().Payload).Note)
}

// twoBandsOneName plays a run where bram sees two goblin bands and calls them
// both by the same name. seenBoth decides whether the first band is still in
// view on the second pass, which is what makes the two either equally fresh or
// not.
func twoBandsOneName(t *testing.T, seenBoth bool) carry.Result {
	t.Helper()

	g := perception.NewGame()

	byFire := projection.Presence{
		Source: "band-a",
		Where:  tunnels,
		Says:   map[testimony.Channel]projection.Says{sight: says(content.Creature, "goblin", "by the fire", tunnels)},
	}
	byDoor := projection.Presence{
		Source: "band-b",
		Where:  tunnels,
		Says:   map[testimony.Channel]projection.Says{sight: says(content.Creature, "goblin", "by the door", tunnels)},
	}

	_, err := g.Tick(projection.Input{
		Presences: []projection.Presence{byFire},
		Senses:    senses(sight, []string{tunnels}, bram),
		At:        10,
	})
	require.NoError(t, err)

	second := []projection.Presence{byDoor}
	if seenBoth {
		second = []projection.Presence{byFire, byDoor}
	}

	_, err = g.Tick(projection.Input{
		Presences: second,
		Senses:    senses(sight, []string{tunnels}, bram),
		At:        11,
	})
	require.NoError(t, err)

	require.NoError(t, g.Identify(bram, handle(sight, "band-a"), easternName, 11))
	require.NoError(t, g.Identify(bram, handle(sight, "band-b"), easternName, 11))

	return carry.Out(carry.Input{Observer: bram, Tracks: g.Held(bram), Names: g})
}

// TestTheCollapseNamesItsLoser proves the one legitimate fold is visible. Two
// tracks under one name on one channel is the observer's own claim that they are
// one thing, so the freshest travels — and what lost is reported.
func TestTheCollapseNamesItsLoser(t *testing.T) {
	// The fire band went out of view at 10; the door band was still there at 11.
	out := twoBandsOneName(t, false)

	require.Equal(t, 1, len(out.Carried))
	assert.Equal(t, "by the door", mustRead(t, out.Carried[0].Entry.Payload).Note,
		"the more recently confirmed one wins")
	assert.Equal(t, []testimony.TrackID{handle(sight, "band-a")}, out.Superseded,
		"and the one that lost is named, not silently gone")
	assert.Empty(t, out.Ambiguous)
}

// TestAnUnresolvableCollapseCarriesNothing proves the fold fails closed. Both
// bands were in view at the same moment, so nothing separates them — and picking
// one by handle order would be this package inventing a conclusion.
func TestAnUnresolvableCollapseCarriesNothing(t *testing.T) {
	out := twoBandsOneName(t, true)

	assert.Empty(t, out.Carried, "neither travels, because neither is the answer")
	assert.Empty(t, out.Superseded, "nothing lost, because nothing won")
	assert.ElementsMatch(t,
		[]testimony.TrackID{handle(sight, "band-a"), handle(sight, "band-b")},
		out.Ambiguous,
		"both named out loud: the observer used one word for two things")
}

// TestACarriedBeliefOnlyRefreshesByCarryingAgain records a gap rather than a
// feature. A later run mints its own run-local handles, so perceiving the same
// goblins again does NOT touch what the player already holds — only another
// distillation does. Inside a run, nobody is reading the player's store, so the
// carried belief can only age.
//
// The inverse move — carrying knowledge back IN, so a party enters the tunnels
// already believing something — is the next gap and is not built.
func TestACarriedBeliefOnlyRefreshesByCarryingAgain(t *testing.T) {
	keeping, kept := testimony.New(), belief.New()

	first := aRunWhereBramFindsGoblins(t, 10)
	require.NoError(t, first.Identify(bram, handle(sight, goblinBand), easternName, 10))
	require.NoError(t, first.Identify(bram, handle(hearing, goblinBand), easternName, 10))
	require.NoError(t, carry.Land(keeping, kept, playerBram,
		carry.Out(carry.Input{Observer: bram, Tracks: first.Held(bram), Names: first}).Carried))

	held, ok := keeping.Track(playerBram, carry.Handle(easternName, sight))
	require.True(t, ok)
	require.Equal(t, testimony.Stamp(10), held.Latest().Confirmed)

	// A whole second run happens. The player's store is untouched by it.
	second := aRunWhereBramFindsGoblins(t, 50)

	held, ok = keeping.Track(playerBram, carry.Handle(easternName, sight))
	require.True(t, ok)
	assert.Equal(t, testimony.Stamp(10), held.Latest().Confirmed,
		"perceiving it again did not refresh what the player holds")

	// Only another distillation does.
	require.NoError(t, second.Identify(bram, handle(sight, goblinBand), easternName, 50))
	require.NoError(t, carry.Land(keeping, kept, playerBram,
		carry.Out(carry.Input{Observer: bram, Tracks: second.Held(bram), Names: second}).Carried))

	held, ok = keeping.Track(playerBram, carry.Handle(easternName, sight))
	require.True(t, ok)
	assert.Equal(t, testimony.Stamp(50), held.Latest().Confirmed)
	assert.Equal(t, 1, len(held.Entries),
		"the same conclusion re-confirmed moves a watermark and appends nothing")
	assert.False(t, held.Current, "still a memory, however recently confirmed")
}
