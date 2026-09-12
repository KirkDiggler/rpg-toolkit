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

const (
	// learnedAt is the run where bram first found the band.
	learnedAt testimony.Stamp = 10
	// returnedAt is the much later run he walks back in on.
	returnedAt testimony.Stamp = 50
)

// whatBramTookHome runs one dungeon, has bram name the band on both channels,
// and hands back the player's store and names.
func whatBramTookHome(t *testing.T) (*testimony.Store, *belief.Beliefs) {
	t.Helper()

	run := aRunWhereBramFindsGoblins(t, learnedAt)
	require.NoError(t, run.Identify(bram, handle(sight, goblinBand), easternName, learnedAt))
	require.NoError(t, run.Identify(bram, handle(hearing, goblinBand), easternName, learnedAt))

	keeping, kept := testimony.New(), belief.New()
	require.NoError(t, carry.Land(keeping, kept, playerBram,
		carry.Out(carry.Input{Observer: bram, Tracks: run.Held(bram), Names: run}).Carried))

	return keeping, kept
}

// carriedIn starts a new run with bram already believing what he took home.
func carriedIn(t *testing.T, mind reconcile.Reconciler) *perception.Game {
	t.Helper()

	keeping, kept := whatBramTookHome(t)

	g := perception.NewGame()
	g.Mind(bram, mind)

	// Carrying IN is Out and Land again, with the two stores swapped. The
	// player's store can be read by Out at all only because Land recorded what
	// they call what they were handed.
	inbound := carry.Out(carry.Input{Observer: playerBram, Tracks: keeping.Held(playerBram), Names: kept})
	require.Empty(t, inbound.Unnamed)
	require.NoError(t, g.Remember(bram, inbound.Carried))

	return g
}

// TestCarryingIn proves the direction is the only thing that changes: a belief
// arrives in a fresh run held, never current, and as old as it really is.
func TestCarryingIn(t *testing.T) {
	g := carriedIn(t, reconcile.Oblivious{})

	remembered := carry.Handle(easternName, sight)

	memory, ok := g.Track(bram, remembered)
	require.True(t, ok)
	assert.False(t, memory.Current, "walking in believing something is not perceiving it")
	assert.Equal(t, learnedAt, memory.Latest().Confirmed,
		"the memory is as stale as it actually is, not as fresh as the new run")
	assert.Equal(t, "a dozen of them", mustRead(t, memory.Latest().Payload).Note)

	// Now he actually looks. The projection mints its own run-local handle, so
	// the thing he sees is a DIFFERENT track from the thing he remembers.
	_, err := g.Tick(projection.Input{
		Presences: []projection.Presence{{
			Source: goblinBand,
			Where:  tunnels,
			Says:   map[testimony.Channel]projection.Says{sight: says(content.Creature, "goblin", "a dozen of them", tunnels)},
		}},
		Senses: senses(sight, []string{tunnels}, bram),
		At:     returnedAt,
	})
	require.NoError(t, err)

	seen, ok := g.Track(bram, handle(sight, goblinBand))
	require.True(t, ok)
	assert.True(t, seen.Current)
	assert.NotEqual(t, remembered, seen.ID, "remembering and seeing are two tracks")

	memory, ok = g.Track(bram, remembered)
	require.True(t, ok)
	assert.Equal(t, learnedAt, memory.Latest().Confirmed,
		"and seeing it did not quietly refresh the memory")
}

// TestRecognisingWhatYouRemember proves the knob reaches memory too. Same
// testimony, two minds: one walks in and knows the place, one does not.
func TestRecognisingWhatYouRemember(t *testing.T) {
	band := projection.Presence{
		Source: goblinBand,
		Where:  tunnels,
		Says:   map[testimony.Channel]projection.Says{sight: says(content.Creature, "goblin", "a dozen of them", tunnels)},
	}

	remembered := carry.Handle(easternName, sight)
	rememberedHeard := carry.Handle(easternName, hearing)
	seen := handle(sight, goblinBand)

	woodwise := carriedIn(t, reconcile.Woodwise{})
	_, err := woodwise.Tick(projection.Input{
		Presences: []projection.Presence{band},
		Senses:    senses(sight, []string{tunnels}, bram),
		At:        returnedAt,
	})
	require.NoError(t, err)

	rel, at := woodwise.Relation(bram, remembered, seen)
	assert.Equal(t, belief.Same, rel, "this is the band I remember")
	assert.Equal(t, returnedAt, at)

	oblivious := carriedIn(t, reconcile.Oblivious{})
	_, err = oblivious.Tick(projection.Input{
		Presences: []projection.Presence{band},
		Senses:    senses(sight, []string{tunnels}, bram),
		At:        returnedAt,
	})
	require.NoError(t, err)

	rel, _ = oblivious.Relation(bram, remembered, seen)
	assert.Equal(t, belief.Unrelated, rel,
		"the same evidence, and no idea it is the same goblins")

	// The carried hearing memory is still a memory: nothing is delivering it,
	// so it stays a ghost in both heads.
	for _, g := range []*perception.Game{woodwise, oblivious} {
		heard, ok := g.Track(bram, rememberedHeard)
		require.True(t, ok)
		assert.False(t, heard.Current)
	}
}

// TestTwoCurrentSightingsStillDoNotMerge guards the loosening. Letting a memory
// join a live percept on one channel must not let two live percepts join.
func TestTwoCurrentSightingsStillDoNotMerge(t *testing.T) {
	g := perception.NewGame()
	g.Mind(bram, reconcile.Woodwise{})

	_, err := g.Tick(projection.Input{
		Presences: []projection.Presence{
			{
				Source: "band-a",
				Where:  tunnels,
				Says:   map[testimony.Channel]projection.Says{sight: says(content.Creature, "goblin", "by the fire", tunnels)},
			},
			{
				Source: "band-b",
				Where:  tunnels,
				Says:   map[testimony.Channel]projection.Says{sight: says(content.Creature, "goblin", "by the door", tunnels)},
			},
		},
		Senses: senses(sight, []string{tunnels}, bram),
		At:     returnedAt,
	})
	require.NoError(t, err)

	rel, _ := g.Relation(bram, handle(sight, "band-a"), handle(sight, "band-b"))
	assert.Equal(t, belief.Unrelated, rel,
		"you would be looking straight at both of them")
}

// TestAnEmptyRoomDoesNotDeleteYourMemory is the case the whole design is for.
//
// Bram walks into the tunnels remembering a dozen goblins, and the tunnels are
// empty. Nothing corrects him. Absence of evidence is not a negative claim, so
// looking at an empty room writes nothing about what is not in it — he still
// believes, he can still act on it, and the only thing that has changed is how
// old the belief is.
func TestAnEmptyRoomDoesNotDeleteYourMemory(t *testing.T) {
	g := carriedIn(t, reconcile.Woodwise{})

	remembered := carry.Handle(easternName, sight)

	before, ok := g.Track(bram, remembered)
	require.True(t, ok)

	// He looks. There is nothing there. The percept is complete and empty.
	landings, err := g.Tick(projection.Input{
		Senses: senses(sight, []string{tunnels}, bram),
		At:     returnedAt,
	})
	require.NoError(t, err)
	assert.Empty(t, landings[0].Delta.Faded,
		"a memory was never current, so an empty room does not even fade it")

	after, ok := g.Track(bram, remembered)
	require.True(t, ok, "he still believes there are goblins in the eastern tunnels")
	assert.Equal(t, before.Entries, after.Entries, "nothing revised it")
	assert.False(t, after.Current)
	assert.Equal(t, learnedAt, after.Latest().Confirmed,
		"forty stamps stale, and nothing in the system has an opinion about that")

	// Nothing invented a doubt on his behalf either, and nothing new arrived:
	// what he holds is exactly the two memories he walked in with, which his
	// mind rightly reads as one remembered thing, seen and heard.
	assertBundles(t, [][]testimony.TrackID{{remembered, carry.Handle(easternName, hearing)}}, g.Contacts(bram))
}

// TestAFreshPerceptDoesNotCorrectAMemory proves a contradiction is held rather
// than resolved. He remembers a dozen goblins and is looking at three wolves;
// both are his, side by side, and nothing chooses.
func TestAFreshPerceptDoesNotCorrectAMemory(t *testing.T) {
	g := carriedIn(t, reconcile.Oblivious{})

	_, err := g.Tick(projection.Input{
		Presences: []projection.Presence{{
			Source: "wolves-1",
			Where:  tunnels,
			Says:   map[testimony.Channel]projection.Says{sight: says(content.Creature, "wolf", "three of them", tunnels)},
		}},
		Senses: senses(sight, []string{tunnels}, bram),
		At:     returnedAt,
	})
	require.NoError(t, err)

	memory, ok := g.Track(bram, carry.Handle(easternName, sight))
	require.True(t, ok)
	assert.Equal(t, "goblin", mustRead(t, memory.Latest().Payload).Of)

	live, ok := g.Track(bram, handle(sight, "wolves-1"))
	require.True(t, ok)
	assert.Equal(t, "wolf", mustRead(t, live.Latest().Payload).Of)

	assert.False(t, memory.Current)
	assert.True(t, live.Current)
}
