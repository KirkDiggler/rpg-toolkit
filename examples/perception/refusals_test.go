// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package perception_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception/belief"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/content"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// The tests in this file assert what the design must be INCAPABLE of. A demo
// can fake every happy path in scenarios_test.go; only these prove the
// boundaries hold, which is the reason the spike is worth building at all.

// TestTheStoreCannotReadAPayload is the load-bearing boundary proof.
//
// Two percepts carry different bytes under the same declared change key. If the
// store compared payloads it would see a change and append. It does not,
// because it cannot: the emitter declares what counts as a change, and the
// store takes its word. That is the same incapacity that makes it unable to
// tell an illusion from a creature.
func TestTheStoreCannotReadAPayload(t *testing.T) {
	s := testimony.New()
	track := testimony.TrackID("t-noise")

	first, err := s.Surveil(testimony.Percept{
		Observer: bram, Channel: sight, At: 1,
		Reports: []testimony.Report{{
			Track: track, Payload: []byte(`{"kind":"creature","of":"goblin"}`), ChangeKey: "k1",
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, []testimony.TrackID{track}, first.FirstContact)

	// Different bytes. Same declared key.
	second, err := s.Surveil(testimony.Percept{
		Observer: bram, Channel: sight, At: 2,
		Reports: []testimony.Report{{
			Track: track, Payload: []byte(`{"of":"goblin","kind":"creature"}`), ChangeKey: "k1",
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, []testimony.TrackID{track}, second.Confirmed)
	assert.Empty(t, second.Changed, "the store did not look at the bytes")

	held, ok := s.Track(bram, track)
	require.True(t, ok)
	assert.Equal(t, 1, len(held.Entries), "same key, same content, no new entry")
	assert.Equal(t, testimony.Stamp(2), held.Entries[0].Confirmed)

	// And the inverse: identical bytes under a new key DO append, proving the
	// key is the whole of the test.
	third, err := s.Surveil(testimony.Percept{
		Observer: bram, Channel: sight, At: 3,
		Reports: []testimony.Report{{
			Track: track, Payload: []byte(`{"of":"goblin","kind":"creature"}`), ChangeKey: "k2",
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, []testimony.TrackID{track}, third.Changed)

	held, ok = s.Track(bram, track)
	require.True(t, ok)
	assert.Equal(t, 2, len(held.Entries))
}

// TestNothingIsNotAnEntry proves the common case costs nothing. What has never
// been perceived has no entry — not a blank one, not a zero one — so it cannot
// be counted, listed, or mistaken for a negative claim.
func TestNothingIsNotAnEntry(t *testing.T) {
	s := testimony.New()

	assert.Nil(t, s.Held("nobody-has-ever-looked"))

	_, ok := s.Track("nobody-has-ever-looked", "t-anything")
	assert.False(t, ok, "false here means I do not know, never that nobody is there")

	// An empty percept from an observer with no testimony must not bring an
	// observer into existence.
	delta, err := s.Surveil(testimony.Percept{Observer: pip, Channel: sight, At: 1})
	require.NoError(t, err)
	assert.Empty(t, delta.Faded)
	assert.Nil(t, s.Held(pip), "looking at nothing, having never seen anything, is nothing")
}

// TestAbsenceIsNotANegativeClaim proves the two absences are indistinguishable
// by design: a place with a creature in it and a place with none produce the
// same "not held" answer for an observer who cannot see either.
func TestAbsenceIsNotANegativeClaim(t *testing.T) {
	s := testimony.New()

	_, err := s.Surveil(testimony.Percept{
		Observer: bram, Channel: sight, At: 1,
		Reports: []testimony.Report{{Track: "t-lit-room", Payload: []byte(`{}`), ChangeKey: "k"}},
	})
	require.NoError(t, err)

	_, emptyRoom := s.Track(bram, "t-empty-room")
	_, occupiedButUnseen := s.Track(bram, "t-dark-room")

	assert.False(t, emptyRoom)
	assert.False(t, occupiedButUnseen)
	assert.Equal(t, emptyRoom, occupiedButUnseen,
		"the store must not be able to distinguish nothing-there from nothing-known")
}

// TestValidationHappensBeforeAnyMutation proves a rejected percept leaves the
// store exactly as it was, so a partial write can never be observed.
func TestValidationHappensBeforeAnyMutation(t *testing.T) {
	s := testimony.New()

	_, err := s.Surveil(testimony.Percept{
		Observer: bram, Channel: sight, At: 1,
		Reports: []testimony.Report{{Track: "t-good", Payload: []byte(`{}`), ChangeKey: "k1"}},
	})
	require.NoError(t, err)

	before, ok := s.Track(bram, "t-good")
	require.True(t, ok)

	// A valid report ahead of an invalid one must not land.
	_, err = s.Surveil(testimony.Percept{
		Observer: bram, Channel: sight, At: 2,
		Reports: []testimony.Report{
			{Track: "t-good", Payload: []byte(`{}`), ChangeKey: "k2"},
			{Track: "t-bad", Payload: []byte(`{}`)},
		},
	})
	require.ErrorIs(t, err, testimony.ErrNoChangeKey)

	after, ok := s.Track(bram, "t-good")
	require.True(t, ok)
	assert.Equal(t, before.Entries, after.Entries, "the good report did not land either")

	_, ok = s.Track(bram, "t-bad")
	assert.False(t, ok)
}

// TestTracksBelongToOneChannel proves a channel cannot write over another
// channel's memory. Without this, hearing reporting a creature it heard would
// destroy what sight last said, and "the same thing known two ways at once"
// would be unholdable.
func TestTracksBelongToOneChannel(t *testing.T) {
	s := testimony.New()

	_, err := s.Surveil(testimony.Percept{
		Observer: bram, Channel: sight, At: 1,
		Reports: []testimony.Report{{Track: "t-shared", Payload: []byte(`{}`), ChangeKey: "k1"}},
	})
	require.NoError(t, err)

	_, err = s.Surveil(testimony.Percept{
		Observer: bram, Channel: hearing, At: 2,
		Reports: []testimony.Report{{Track: "t-shared", Payload: []byte(`{}`), ChangeKey: "k2"}},
	})
	require.ErrorIs(t, err, testimony.ErrChannelMismatch)
}

// TestTimeOnlyMovesOneWay proves a stale percept cannot overwrite a fresh one.
// Accepting a regress silently is how a replay or a late delivery would rewrite
// a memory that was already more current.
func TestTimeOnlyMovesOneWay(t *testing.T) {
	s := testimony.New()

	_, err := s.Surveil(testimony.Percept{
		Observer: bram, Channel: sight, At: 10,
		Reports: []testimony.Report{{Track: "t-x", Payload: []byte(`{}`), ChangeKey: "k1"}},
	})
	require.NoError(t, err)

	_, err = s.Surveil(testimony.Percept{
		Observer: bram, Channel: sight, At: 9,
		Reports: []testimony.Report{{Track: "t-x", Payload: []byte(`{}`), ChangeKey: "k2"}},
	})
	require.ErrorIs(t, err, testimony.ErrStampRegress)
}

// TestNoObserverCanReadAnother proves knowledge is per-observer at the storage
// level, not by convention. Every read in the store takes exactly one observer,
// and there is no call that takes two or reconciles across them.
func TestNoObserverCanReadAnother(t *testing.T) {
	s := testimony.New()

	_, err := s.Surveil(testimony.Percept{
		Observer: bram, Channel: sight, At: 1,
		Reports: []testimony.Report{{Track: "t-y", Payload: []byte(`{}`), ChangeKey: "k1"}},
	})
	require.NoError(t, err)

	_, ok := s.Track(pip, "t-y")
	assert.False(t, ok, "bram seeing it tells pip nothing")
	assert.Nil(t, s.Held(pip))
}

// TestADisagreementIsReturnedWhole proves there is no fold: a contact holding
// two tracks that disagree hands back both, and the caller must choose. If this
// ever returns one answer, illusion is foreclosed.
func TestADisagreementIsReturnedWhole(t *testing.T) {
	s := testimony.New()
	b := belief.New()

	dragon := testimony.TrackID("t-dragon")
	goblin := testimony.TrackID("t-goblin")

	for _, r := range []struct {
		track testimony.TrackID
		of    string
	}{{dragon, "dragon"}, {goblin, "goblin"}} {
		p := content.Percept{Kind: content.Creature, Of: r.of}

		_, err := s.Surveil(testimony.Percept{
			Observer: bram, Channel: sight, At: 1,
			Reports: []testimony.Report{{Track: r.track, Payload: content.Encode(p), ChangeKey: content.Key(p)}},
		})
		require.NoError(t, err)
	}

	require.NoError(t, b.Assert(bram, dragon, goblin, belief.Same, 1))

	contacts := b.Contacts(bram, []testimony.TrackID{dragon, goblin})
	require.Equal(t, 1, len(contacts), "the observer holds them as one thing")

	said := make([]string, 0, len(contacts[0].Tracks))

	for _, id := range contacts[0].Tracks {
		held, ok := s.Track(bram, id)
		require.True(t, ok)
		said = append(said, mustRead(t, held.Latest().Payload).Of)
	}

	assert.ElementsMatch(t, []string{"dragon", "goblin"}, said,
		"one contact, two irreconcilable stories, both handed back")
}

// TestATrackIsNotAPair rejects a claim about a track and itself.
func TestATrackIsNotAPair(t *testing.T) {
	b := belief.New()
	require.ErrorIs(t, b.Assert(bram, "t-a", "t-a", belief.Same, 1), belief.ErrSameTrack)
}

// TestRetractingIsNotRulingOut proves the three states are three, not two.
func TestRetractingIsNotRulingOut(t *testing.T) {
	b := belief.New()

	require.NoError(t, b.Assert(bram, "t-a", "t-b", belief.Same, 1))
	rel, _ := b.Relation(bram, "t-a", "t-b")
	require.Equal(t, belief.Same, rel)

	require.NoError(t, b.Assert(bram, "t-a", "t-b", belief.Unrelated, 2))
	rel, at := b.Relation(bram, "t-a", "t-b")
	assert.Equal(t, belief.Unrelated, rel, "retracted to no claim")
	assert.Zero(t, at, "no claim has no date, because nobody is claiming anything")

	require.NoError(t, b.Assert(bram, "t-a", "t-b", belief.Distinct, 3))
	rel, at = b.Relation(bram, "t-a", "t-b")
	assert.Equal(t, belief.Distinct, rel)
	assert.Equal(t, testimony.Stamp(3), at, "ruling out is an act, and it is dated")
}
