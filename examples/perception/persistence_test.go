// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package perception_test

import (
	"encoding/json"
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

// TestKnowledgeSurvivesTheProcess is the claim the whole carry story rests on,
// made real: a player's knowledge goes out to bytes, everything in memory is
// dropped, and a run months later is built from nothing but those bytes.
func TestKnowledgeSurvivesTheProcess(t *testing.T) {
	keeping, kept := whatBramTookHome(t)

	heldBytes, err := json.Marshal(keeping.ToData())
	require.NoError(t, err)

	claimBytes, err := json.Marshal(kept.ToData())
	require.NoError(t, err)

	// Everything in memory is gone. Only the bytes are left.
	keeping, kept = nil, nil

	var heldData testimony.Data
	require.NoError(t, json.Unmarshal(heldBytes, &heldData))

	restored, err := testimony.Load(heldData)
	require.NoError(t, err)

	var claimData belief.Data
	require.NoError(t, json.Unmarshal(claimBytes, &claimData))

	restoredNames, err := belief.Load(claimData)
	require.NoError(t, err)

	// A new run, from the bytes alone.
	back := perception.NewGame()
	back.Mind(bram, reconcile.Woodwise{})

	inbound := carry.Out(carry.Input{
		Observer: playerBram,
		Tracks:   restored.Held(playerBram),
		Names:    restoredNames,
	})
	require.Empty(t, inbound.Unnamed, "the names came back with the testimony")
	require.NoError(t, back.Remember(bram, inbound.Carried))

	remembered := carry.Handle(easternName, sight)

	memory, ok := back.Track(bram, remembered)
	require.True(t, ok, "he still knows what he found down there")
	assert.False(t, memory.Current, "and still knows he is not looking at it")
	assert.Equal(t, learnedAt, memory.Latest().Confirmed, "as old as it really is")
	assert.Equal(t, "a dozen of them", mustRead(t, memory.Latest().Payload).Note)

	// And he still recognises the place when he walks back into it.
	_, err = back.Tick(projection.Input{
		Presences: []projection.Presence{{
			Source: goblinBand,
			Where:  tunnels,
			Says:   map[testimony.Channel]projection.Says{sight: says(content.Creature, "goblin", "a dozen of them", tunnels)},
		}},
		Senses: senses(sight, []string{tunnels}, bram),
		At:     returnedAt,
	})
	require.NoError(t, err)

	rel, _ := back.Relation(bram, remembered, handle(sight, goblinBand))
	assert.Equal(t, belief.Same, rel, "this is the band I remember — across a process boundary")
}

// TestARoundTripChangesNothing checks both directions, and then checks the
// values themselves. A conversion that is wrong in the same way both ways
// round-trips perfectly, so equality alone proves very little.
func TestARoundTripChangesNothing(t *testing.T) {
	keeping, kept := whatBramTookHome(t)

	held := keeping.ToData()
	restored, err := testimony.Load(held)
	require.NoError(t, err)
	assert.Equal(t, held, restored.ToData())

	claims := kept.ToData()
	restoredNames, err := belief.Load(claims)
	require.NoError(t, err)
	assert.Equal(t, claims, restoredNames.ToData())

	// Now say what the values actually are, so a symmetric mistake has nowhere
	// to hide.
	stored := held.Observers[playerBram].Tracks[carry.Handle(easternName, sight)]
	assert.Equal(t, sight, stored.Channel)
	assert.False(t, stored.Current, "a carried belief is stored as the ghost it is")
	require.Equal(t, 1, len(stored.Entries))
	assert.Equal(t, testimony.StampData{Tick: 10}, stored.Entries[0].Confirmed)
	assert.Equal(t, tunnels, stored.Entries[0].Where)

	named := claims.Observers[playerBram].Names
	require.Equal(t, 2, len(named), "one name per channel carried")
	assert.Equal(t, easternName, named[0].Name)
}

// TestNothingHeldIsNothingStored keeps the common case free. What has never been
// perceived has no entry in storage either, so it cannot be counted there.
func TestNothingHeldIsNothingStored(t *testing.T) {
	s := testimony.New()
	assert.Empty(t, s.ToData().Observers)

	restored, err := testimony.Load(s.ToData())
	require.NoError(t, err)
	assert.Nil(t, restored.Held("nobody-has-ever-looked"))

	b := belief.New()
	assert.Empty(t, b.ToData().Observers)

	// A claim made and then retracted leaves nothing behind to store.
	require.NoError(t, b.Assert(bram, "t-a", "t-b", belief.Same, moment(1)))
	require.NoError(t, b.Assert(bram, "t-a", "t-b", belief.Unrelated, moment(2)))
	assert.Empty(t, b.ToData().Observers, "no claim is not a claim, here either")
}

// Track handles used by the saved-state fixtures below.
const (
	trackA       = "k-a"
	trackB       = "k-b"
	savedGoblins = "k-goblins"
)

// validHeld is one saved track, as the store would really have written it.
func validHeld() testimony.Data {
	return testimony.Data{
		Observers: map[testimony.Observer]testimony.ObserverData{
			playerBram: {Tracks: map[testimony.TrackID]testimony.TrackData{
				savedGoblins: {
					Channel: sight,
					Entries: []testimony.EntryData{{
						ChangeKey: "creature|goblin|",
						Where:     tunnels,
						Observed:  testimony.StampData{Tick: 10},
						Confirmed: testimony.StampData{Tick: 12},
					}},
				},
			}},
		},
	}
}

// TestLoadRefusesWhatTheStoreCouldNotHaveWritten is the fail-closed half of the
// contract. Every shape here names a state the store cannot reach on its own, so
// meeting one means the data came from somewhere else and nothing in it can be
// trusted as testimony anybody gave.
func TestLoadRefusesWhatTheStoreCouldNotHaveWritten(t *testing.T) {
	for name, damage := range map[string]func(*testimony.Data){
		"an observer holding nothing": func(d *testimony.Data) {
			d.Observers["ghost-observer"] = testimony.ObserverData{}
		},
		"a track with no testimony": func(d *testimony.Data) {
			d.Observers[playerBram].Tracks["k-empty"] = testimony.TrackData{Channel: sight}
		},
		"a track with no channel": func(d *testimony.Data) {
			track := d.Observers[playerBram].Tracks[savedGoblins]
			track.Channel = ""
			d.Observers[playerBram].Tracks[savedGoblins] = track
		},
		"an entry with no change key": func(d *testimony.Data) {
			track := d.Observers[playerBram].Tracks[savedGoblins]
			track.Entries[0].ChangeKey = ""
			d.Observers[playerBram].Tracks[savedGoblins] = track
		},
		"confirmed before it was observed": func(d *testimony.Data) {
			track := d.Observers[playerBram].Tracks[savedGoblins]
			track.Entries[0].Confirmed = testimony.StampData{Tick: 1}
			d.Observers[playerBram].Tracks[savedGoblins] = track
		},
		"an entry that repeats the one before it": func(d *testimony.Data) {
			track := d.Observers[playerBram].Tracks[savedGoblins]
			repeat := track.Entries[0]
			repeat.Observed = testimony.StampData{Tick: 20}
			repeat.Confirmed = testimony.StampData{Tick: 20}
			track.Entries = append(track.Entries, repeat)
			d.Observers[playerBram].Tracks[savedGoblins] = track
		},
		"a log that runs backwards": func(d *testimony.Data) {
			track := d.Observers[playerBram].Tracks[savedGoblins]
			track.Entries = append(track.Entries, testimony.EntryData{
				ChangeKey: "creature|goblin|fleeing",
				Where:     tunnels,
				Observed:  testimony.StampData{Tick: 3},
				Confirmed: testimony.StampData{Tick: 3},
			})
			d.Observers[playerBram].Tracks[savedGoblins] = track
		},
	} {
		t.Run(name, func(t *testing.T) {
			data := validHeld()
			damage(&data)

			_, err := testimony.Load(data)
			require.ErrorIs(t, err, testimony.ErrInvalidData)
		})
	}

	// The valid shape it was built from really does load, so the refusals above
	// are refusing the break and not the fixture.
	_, err := testimony.Load(validHeld())
	require.NoError(t, err)
}

// TestLoadRefusesAClaimNobodyMade is the same discipline for beliefs.
func TestLoadRefusesAClaimNobodyMade(t *testing.T) {
	valid := func() belief.Data {
		return belief.Data{
			Observers: map[testimony.Observer]belief.ObserverData{
				playerBram: {
					Claims: []belief.ClaimData{{A: trackA, B: trackB, Relation: "same"}},
					Names:  []belief.NameData{{Track: trackA, Name: easternName}},
				},
			},
		}
	}

	for name, damage := range map[string]func(*belief.Data){
		"no claim stored as a claim": func(d *belief.Data) {
			d.Observers[playerBram].Claims[0].Relation = "unrelated"
		},
		"a relation nobody has a word for": func(d *belief.Data) {
			d.Observers[playerBram].Claims[0].Relation = "maybe"
		},
		"a pair stored out of order": func(d *belief.Data) {
			d.Observers[playerBram].Claims[0].A = "k-z"
		},
		"a track claimed against itself": func(d *belief.Data) {
			d.Observers[playerBram].Claims[0].B = trackA
		},
		"the same pair claimed twice": func(d *belief.Data) {
			od := d.Observers[playerBram]
			od.Claims = append(od.Claims, belief.ClaimData{A: trackA, B: trackB, Relation: "distinct"})
			d.Observers[playerBram] = od
		},
		"a track named nothing": func(d *belief.Data) {
			d.Observers[playerBram].Names[0].Name = ""
		},
		"a track named twice": func(d *belief.Data) {
			od := d.Observers[playerBram]
			od.Names = append(od.Names, belief.NameData{Track: trackA, Name: "something else"})
			d.Observers[playerBram] = od
		},
		"an observer claiming nothing": func(d *belief.Data) {
			d.Observers["ghost-observer"] = belief.ObserverData{}
		},
	} {
		t.Run(name, func(t *testing.T) {
			data := valid()
			damage(&data)

			_, err := belief.Load(data)
			require.ErrorIs(t, err, belief.ErrInvalidData)
		})
	}

	_, err := belief.Load(valid())
	require.NoError(t, err)
}
