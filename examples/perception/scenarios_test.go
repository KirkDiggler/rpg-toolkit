// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package perception_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/belief"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/content"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/projection"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/reconcile"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

const (
	sight   testimony.Channel = "sight"
	hearing testimony.Channel = "hearing"

	// pip holds everything separately and decides nothing.
	pip testimony.Observer = "pip"
	// bram is woodwise: merges across channels, rules out by sign.
	bram testimony.Observer = "bram"
	// nan merges whatever shares a place.
	nan testimony.Observer = "nan"

	hall = "hall"
	room = "room"
	mud  = "mud"

	// Ledger handles for the things in these stories. They never leave the
	// projection: everything downstream sees only the opaque track handle
	// minted from them.
	goblinOne   = "goblin-1"
	silentImage = "silent-image-1"
)

// says builds what one channel reports. Where is what THIS channel can place —
// empty means it perceived something it could not localise.
func says(kind content.Kind, of, note, where string) projection.Says {
	p := content.Percept{Kind: kind, Of: of, Note: note}

	return projection.Says{
		Payload:   content.Encode(p),
		ChangeKey: content.Key(p),
		Locus:     testimony.Locus{Where: where},
	}
}

func senses(channel testimony.Channel, reach []string, observers ...testimony.Observer) []projection.Sense {
	out := make([]projection.Sense, 0, len(observers))
	for _, o := range observers {
		out = append(out, projection.Sense{Observer: o, Channel: channel, Reach: reach})
	}

	return out
}

func read(t *testing.T, track testimony.Track) content.Percept {
	t.Helper()

	p, err := content.Decode(track.Latest().Payload)
	require.NoError(t, err)

	return p
}

// assertBundles compares how an observer groups their tracks, order-insensitively.
func assertBundles(t *testing.T, want [][]testimony.TrackID, got []belief.Contact) {
	t.Helper()

	normalize := func(in [][]testimony.TrackID) [][]testimony.TrackID {
		out := make([][]testimony.TrackID, 0, len(in))

		for _, bundle := range in {
			b := slices.Clone(bundle)
			slices.Sort(b)
			out = append(out, b)
		}

		slices.SortFunc(out, func(a, b []testimony.TrackID) int {
			return strings.Compare(string(a[0]), string(b[0]))
		})

		return out
	}

	actual := make([][]testimony.TrackID, 0, len(got))
	for _, c := range got {
		actual = append(actual, c.Tracks)
	}

	assert.Equal(t, normalize(want), normalize(actual))
}

// The test is the game master, so it may mint handles to assert with. Nothing
// inside the system is ever given the source these are minted from.
func handle(channel testimony.Channel, source string) testimony.TrackID {
	return projection.Handle(channel, source)
}

// TestTheDoor proves reconciliation is the observer's job: two observers given
// byte-identical testimony reach different conclusions, and the difference is
// which mind they have.
func TestTheDoor(t *testing.T) {
	g := perception.NewGame()
	g.Mind(pip, reconcile.Oblivious{})
	g.Mind(bram, reconcile.Woodwise{})

	heard := handle(hearing, goblinOne)
	seen := handle(sight, goblinOne)

	// Door shut. Sight reaches the hall only. Sound carries through the door
	// but cannot be placed.
	shut := projection.Presence{
		Source: goblinOne,
		Where:  room,
		Says: map[testimony.Channel]projection.Says{
			sight:   says(content.Creature, "goblin", "", room),
			hearing: says(content.Noise, "", "metal scraping", ""),
		},
	}

	_, err := g.Tick(projection.Input{
		Presences: []projection.Presence{shut},
		Senses: append(
			senses(sight, []string{hall}, pip, bram),
			senses(hearing, []string{hall, room}, pip, bram)...),
		At: 1,
	})
	require.NoError(t, err)

	for _, who := range []testimony.Observer{pip, bram} {
		noise, held := g.Track(who, heard)
		require.True(t, held, "%s should hold the noise", who)
		assert.Equal(t, content.Noise, read(t, noise).Kind)
		assert.Empty(t, noise.Latest().Locus.Where,
			"a noise through a shut door is known to be there and not known where")

		_, sighted := g.Track(who, seen)
		assert.False(t, sighted, "%s cannot see through a shut door", who)
	}

	// The door opens. Sight reaches the room, and now the sound can be placed.
	open := shut
	open.Says = map[testimony.Channel]projection.Says{
		sight:   says(content.Creature, "goblin", "", room),
		hearing: says(content.Noise, "", "metal scraping", room),
	}

	_, err = g.Tick(projection.Input{
		Presences: []projection.Presence{open},
		Senses: append(
			senses(sight, []string{hall, room}, pip, bram),
			senses(hearing, []string{hall, room}, pip, bram)...),
		At: 2,
	})
	require.NoError(t, err)

	// Identical testimony for both.
	for _, who := range []testimony.Observer{pip, bram} {
		_, sighted := g.Track(who, seen)
		require.True(t, sighted, "%s should now see it", who)
	}

	// Different conclusions. Nothing merged for pip, because nothing in the
	// system merges on its own.
	assertBundles(t, [][]testimony.TrackID{{heard}, {seen}}, g.Contacts(pip))
	rel, _ := g.Relation(pip, heard, seen)
	assert.Equal(t, belief.Unrelated, rel, "no claim is not a claim of difference")

	assertBundles(t, [][]testimony.TrackID{{heard, seen}}, g.Contacts(bram))
	rel, at := g.Relation(bram, heard, seen)
	assert.Equal(t, belief.Same, rel)
	assert.Equal(t, testimony.Stamp(2), at, "the claim is stamped with when it was made")
}

// TestSilentImage proves the lie lives in the projection and never in the truth
// surface, and that a disagreement between two channels is HELD rather than
// resolved.
func TestSilentImage(t *testing.T) {
	g := perception.NewGame()
	g.Mind(bram, reconcile.Woodwise{})

	goblin := projection.Presence{
		Source: goblinOne,
		Where:  room,
		Says: map[testimony.Channel]projection.Says{
			sight:   says(content.Creature, "goblin", "", room),
			hearing: says(content.Noise, "", "metal scraping", room),
		},
	}

	// Silent Image writes to sight and says nothing to hearing. It hides
	// nothing: the goblin is still plainly there behind the dragon.
	illusion := projection.Forgery{
		Where:  room,
		Source: silentImage,
		Says: map[testimony.Channel]projection.Says{
			sight: says(content.Creature, "dragon", "", room),
		},
	}

	in := projection.Input{
		Presences: []projection.Presence{goblin},
		Forgeries: []projection.Forgery{illusion},
		Senses: append(
			senses(sight, []string{room}, bram),
			senses(hearing, []string{room}, bram)...),
		At: 1,
	}

	_, err := g.Tick(in)
	require.NoError(t, err)

	// THE POINT: the truth surface has no dragon in it. The wizard casting the
	// spell is a true fact about the world; the dragon is not a fact at all.
	for _, p := range in.Truth() {
		for channel, s := range p.Says {
			said, decodeErr := content.Decode(s.Payload)
			require.NoError(t, decodeErr)
			assert.NotEqual(t, "dragon", said.Of,
				"truth surface carries a dragon on %s", channel)
		}
	}

	dragon := handle(sight, silentImage)
	seen := handle(sight, goblinOne)
	heard := handle(hearing, goblinOne)

	sightedDragon, held := g.Track(bram, dragon)
	require.True(t, held)
	assert.Equal(t, "dragon", read(t, sightedDragon).Of)

	sightedGoblin, held := g.Track(bram, seen)
	require.True(t, held)
	assert.Equal(t, "goblin", read(t, sightedGoblin).Of)

	// Two sight tracks in one place saying different things, both retrievable,
	// neither preferred. There is no call that returns one answer.
	assert.NotEqual(t, sightedDragon.Latest().Payload, sightedGoblin.Latest().Payload)

	// Woodwise puts the sound together with both shapes, because both are
	// present things in the same place on another channel. It has no way to
	// know one of them is a lie — and neither has the store.
	assertBundles(t, [][]testimony.TrackID{{dragon, seen, heard}}, g.Contacts(bram))

	sameChannel, _ := g.Relation(bram, dragon, seen)
	assert.Equal(t, belief.Unrelated, sameChannel,
		"two things you can both see at once are not merged by co-location")
}

// TestTheWrongMerge proves a bad inference costs the observer nothing but the
// claim: the testimony underneath is never revised, and the split is a write.
func TestTheWrongMerge(t *testing.T) {
	g := perception.NewGame()
	g.Mind(bram, reconcile.Woodwise{})

	goblin := projection.Presence{
		Source: goblinOne,
		Where:  room,
		Says: map[testimony.Channel]projection.Says{
			sight:   says(content.Creature, "goblin", "", room),
			hearing: says(content.Noise, "", "metal scraping", room),
		},
	}
	illusion := projection.Forgery{
		Where:  room,
		Source: silentImage,
		Says: map[testimony.Channel]projection.Says{
			sight: says(content.Creature, "dragon", "", room),
		},
	}

	both := append(senses(sight, []string{room}, bram), senses(hearing, []string{room}, bram)...)

	_, err := g.Tick(projection.Input{
		Presences: []projection.Presence{goblin},
		Forgeries: []projection.Forgery{illusion},
		Senses:    both,
		At:        1,
	})
	require.NoError(t, err)

	dragon := handle(sight, silentImage)
	seen := handle(sight, goblinOne)
	heard := handle(hearing, goblinOne)

	before, held := g.Track(bram, dragon)
	require.True(t, held)

	// Dispelled. The dragon is simply not delivered any more.
	_, err = g.Tick(projection.Input{
		Presences: []projection.Presence{goblin},
		Senses:    both,
		At:        2,
	})
	require.NoError(t, err)

	after, held := g.Track(bram, dragon)
	require.True(t, held, "a dispelled illusion is still remembered")
	assert.False(t, after.Current, "it is a ghost: held, no longer delivered")
	assert.Equal(t, before.Entries, after.Entries,
		"nothing revised the memory of the dragon — not the dispel, not the system")

	// Still merged, because nobody has decided otherwise yet.
	assertBundles(t, [][]testimony.TrackID{{dragon, seen, heard}}, g.Contacts(bram))

	// Bram works it out and takes it apart himself. Ruling it out is a write.
	require.NoError(t, g.Claim(bram, dragon, heard, belief.Distinct, 3))
	require.NoError(t, g.Claim(bram, dragon, seen, belief.Distinct, 3))

	assertBundles(t, [][]testimony.TrackID{{dragon}, {seen, heard}}, g.Contacts(bram))

	stillThere, held := g.Track(bram, dragon)
	require.True(t, held)
	assert.Equal(t, before.Entries, stillThere.Entries,
		"splitting regrouped a claim and restored nothing, because nothing was lost")

	// A third pass cannot undo the ruling: a mind proposes, it does not overrule.
	_, err = g.Tick(projection.Input{
		Presences: []projection.Presence{goblin},
		Senses:    both,
		At:        4,
	})
	require.NoError(t, err)
	assertBundles(t, [][]testimony.TrackID{{dragon}, {seen, heard}}, g.Contacts(bram))
}

// TestTwoObserversContradict proves there is no reconciliation between people:
// one of them is deceived, both are correct about what they perceived, and
// nothing merges their knowledge.
func TestTwoObserversContradict(t *testing.T) {
	g := perception.NewGame()
	g.Mind(pip, reconcile.Oblivious{})
	g.Mind(bram, reconcile.Oblivious{})

	goblin := projection.Presence{
		Source: goblinOne,
		Where:  room,
		Says: map[testimony.Channel]projection.Says{
			sight:   says(content.Creature, "goblin", "holding a sword", room),
			hearing: says(content.Noise, "", "metal scraping", room),
		},
	}

	// Pip alone is charmed: sight is forged and the real goblin hidden from it.
	// Hearing is untouched, so pip still hears the blade.
	charm := projection.Forgery{
		Where:     room,
		Source:    "charm-on-pip",
		Observers: []testimony.Observer{pip},
		Says: map[testimony.Channel]projection.Says{
			sight: says(content.Creature, "goblin", "holding a toy", room),
		},
		Hides:    []testimony.Channel{sight},
		Replaces: goblinOne,
	}

	_, err := g.Tick(projection.Input{
		Presences: []projection.Presence{goblin},
		Forgeries: []projection.Forgery{charm},
		Senses: append(
			senses(sight, []string{room}, pip, bram),
			senses(hearing, []string{room}, pip, bram)...),
		At: 1,
	})
	require.NoError(t, err)

	toy := handle(sight, "charm-on-pip")
	sword := handle(sight, goblinOne)
	heard := handle(hearing, goblinOne)

	pipsSight, held := g.Track(pip, toy)
	require.True(t, held)
	assert.Equal(t, "holding a toy", read(t, pipsSight).Note)

	_, held = g.Track(pip, sword)
	assert.False(t, held, "the charm stood in front of the real thing")

	bramsSight, held := g.Track(bram, sword)
	require.True(t, held)
	assert.Equal(t, "holding a sword", read(t, bramsSight).Note)

	_, held = g.Track(bram, toy)
	assert.False(t, held, "bram was never deceived")

	// Hearing was not forged, so both hold the same handle saying the same
	// thing. The contradiction is confined to the channel that was lied to.
	for _, who := range []testimony.Observer{pip, bram} {
		noise, ok := g.Track(who, heard)
		require.True(t, ok)
		assert.Equal(t, "metal scraping", read(t, noise).Note)
	}
}

// TestStaleness proves the two stamps answer "how old is this" without ever
// answering "is it still true", and that changed content APPENDS.
func TestStaleness(t *testing.T) {
	g := perception.NewGame()

	standing := projection.Presence{
		Source: goblinOne,
		Where:  room,
		Says: map[testimony.Channel]projection.Says{
			sight: says(content.Creature, "goblin", "standing", room),
		},
	}

	watching := senses(sight, []string{room}, bram)

	// Three passes of the same thing.
	for at := testimony.Stamp(1); at <= 3; at++ {
		_, err := g.Tick(projection.Input{
			Presences: []projection.Presence{standing},
			Senses:    watching,
			At:        at,
		})
		require.NoError(t, err)
	}

	seen := handle(sight, goblinOne)

	held, ok := g.Track(bram, seen)
	require.True(t, ok)
	require.Equal(t, 1, len(held.Entries), "identical content does not append")
	assert.Equal(t, testimony.Stamp(1), held.Entries[0].Observed)
	assert.Equal(t, testimony.Stamp(3), held.Entries[0].Confirmed,
		"first seen at 1, still saying the same thing at 3")

	// It draws a blade. New content, so a new entry — the old one stands.
	drawn := standing
	drawn.Says = map[testimony.Channel]projection.Says{
		sight: says(content.Creature, "goblin", "blade drawn", room),
	}

	landings, err := g.Tick(projection.Input{
		Presences: []projection.Presence{drawn},
		Senses:    watching,
		At:        4,
	})
	require.NoError(t, err)
	assert.Equal(t, []testimony.TrackID{seen}, landings[0].Delta.Changed,
		"changed is a real transition, not a redelivery")

	held, ok = g.Track(bram, seen)
	require.True(t, ok)
	require.Equal(t, 2, len(held.Entries))
	assert.Equal(t, testimony.Stamp(1), held.Entries[0].Observed)
	assert.Equal(t, testimony.Stamp(3), held.Entries[0].Confirmed)
	assert.Equal(t, "standing", mustRead(t, held.Entries[0].Payload).Note,
		"what it was doing at 1 through 3 is still held")
	assert.Equal(t, testimony.Stamp(4), held.Entries[1].Observed)

	// Bram looks away. Complete percept with nothing in it is a write.
	landings, err = g.Tick(projection.Input{
		Presences: []projection.Presence{drawn},
		Senses:    senses(sight, nil, bram),
		At:        5,
	})
	require.NoError(t, err)
	assert.Equal(t, []testimony.TrackID{seen}, landings[0].Delta.Faded)

	// Two more passes with the goblin still there and bram still looking away.
	for at := testimony.Stamp(6); at <= 7; at++ {
		_, err = g.Tick(projection.Input{
			Presences: []projection.Presence{drawn},
			Senses:    senses(sight, nil, bram),
			At:        at,
		})
		require.NoError(t, err)
	}

	held, ok = g.Track(bram, seen)
	require.True(t, ok, "the ghost keeps what was last seen")
	assert.False(t, held.Current)
	assert.Equal(t, testimony.Stamp(4), held.Latest().Confirmed,
		"last confirmed at 4 — and nothing here says whether it still holds")
}

// TestFootprints proves the third state. Three minds, identical testimony:
// one holds no opinion, one is confidently wrong, and one RULES OUT — and
// ruling out is not the same as never having decided.
func TestFootprints(t *testing.T) {
	g := perception.NewGame()
	g.Mind(pip, reconcile.Oblivious{})
	g.Mind(nan, reconcile.Credulous{})
	g.Mind(bram, reconcile.Woodwise{})

	wolfSign := projection.Presence{
		Source: "tracks-1",
		Where:  mud,
		Says: map[testimony.Channel]projection.Says{
			sight: says(content.Trace, "wolf", "pad marks in the mud", mud),
		},
	}
	goblins := projection.Presence{
		Source: "goblin-2",
		Where:  mud,
		Says: map[testimony.Channel]projection.Says{
			sight: says(content.Creature, "goblin", "", mud),
		},
	}

	_, err := g.Tick(projection.Input{
		Presences: []projection.Presence{wolfSign, goblins},
		Senses:    senses(sight, []string{mud}, pip, nan, bram),
		At:        1,
	})
	require.NoError(t, err)

	tracks := handle(sight, "tracks-1")
	goblin := handle(sight, "goblin-2")

	// The zombie: no claim, ever. Not confused because it merged wrongly —
	// confused because it can never rule anything out.
	assertBundles(t, [][]testimony.TrackID{{tracks}, {goblin}}, g.Contacts(pip))
	rel, _ := g.Relation(pip, tracks, goblin)
	assert.Equal(t, belief.Unrelated, rel)

	// The panicked commoner: same place, must be the same thing. Wrong.
	assertBundles(t, [][]testimony.TrackID{{tracks, goblin}}, g.Contacts(nan))
	rel, _ = g.Relation(nan, tracks, goblin)
	assert.Equal(t, belief.Same, rel)

	// The ranger: those are not goblin tracks. Same bundles as the zombie,
	// entirely different knowledge.
	assertBundles(t, [][]testimony.TrackID{{tracks}, {goblin}}, g.Contacts(bram))
	rel, at := g.Relation(bram, tracks, goblin)
	assert.Equal(t, belief.Distinct, rel, "ruled out, not merely undecided")
	assert.Equal(t, testimony.Stamp(1), at)
}

// TestTheRangerCannotRuleIn proves the asymmetry: matching sign is consistent,
// and consistent is not identical. Woodwise makes no claim either way.
func TestTheRangerCannotRuleIn(t *testing.T) {
	g := perception.NewGame()
	g.Mind(bram, reconcile.Woodwise{})

	goblinSign := projection.Presence{
		Source: "tracks-2",
		Where:  mud,
		Says: map[testimony.Channel]projection.Says{
			sight: says(content.Trace, "goblin", "three-toed, fresh", mud),
		},
	}
	goblins := projection.Presence{
		Source: "goblin-3",
		Where:  mud,
		Says: map[testimony.Channel]projection.Says{
			sight: says(content.Creature, "goblin", "", mud),
		},
	}

	_, err := g.Tick(projection.Input{
		Presences: []projection.Presence{goblinSign, goblins},
		Senses:    senses(sight, []string{mud}, bram),
		At:        1,
	})
	require.NoError(t, err)

	tracks := handle(sight, "tracks-2")
	goblin := handle(sight, "goblin-3")

	rel, _ := g.Relation(bram, tracks, goblin)
	assert.Equal(t, belief.Unrelated, rel,
		"the sign fits, which is not the same as it being these goblins")
	assertBundles(t, [][]testimony.TrackID{{tracks}, {goblin}}, g.Contacts(bram))
}

func mustRead(t *testing.T, payload []byte) content.Percept {
	t.Helper()

	p, err := content.Decode(payload)
	require.NoError(t, err)

	return p
}
