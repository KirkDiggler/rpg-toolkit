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
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// These run against the real world/journal rather than a stand-in, because the
// point is the FIT and a mock cannot prove a fit.
//
// One piece of friction to note before anything else: journal speaks
// journal.EntityID and perception speaks testimony.Observer. They are both
// strings and they are not the same type, so the composition converts at the
// seam. That is correct — neither module should learn the other's vocabulary —
// but it is a conversion somebody has to own, and these tests are where it shows.

func asEntity(o testimony.Observer) journal.EntityID { return journal.EntityID(o) }

// worldOrder is the coordinate the world is currently at: the append position
// of the last fact recorded. It advances when something HAPPENS.
func worldOrder(j *journal.Journal) int { return j.Len() }

// TestTheWorldsOrderOutlivesTheRun proves why a stamp needs both halves.
//
// Run-local ticks order testimony inside a run and are meaningless outside it —
// so meaningless that comparing them alone says a memory from last season is
// NEWER than what is in front of you right now. The world's order is what keeps
// a carried belief honest about its age.
func TestTheWorldsOrderOutlivesTheRun(t *testing.T) {
	world := journal.New()

	_, err := world.Append(journal.Fact{
		Kind:     "took-the-contract",
		Actor:    asEntity(bram),
		Audience: journal.Audience{asEntity(bram)},
	})
	require.NoError(t, err)

	// A dungeon run happens at whatever order the world has reached. Nothing
	// inside it touches the journal, so the world's order stands still while a
	// great deal goes on.
	firstRun := worldOrder(world)
	require.Equal(t, 1, firstRun)

	g := perception.NewGame()

	band := projection.Presence{
		Source: goblinBand,
		Where:  tunnels,
		Says:   map[testimony.Channel]projection.Says{sight: says(content.Creature, "goblin", "a dozen of them", tunnels)},
	}

	for tick := uint64(1); tick <= 2; tick++ {
		_, tickErr := g.Tick(projection.Input{
			Presences: []projection.Presence{band},
			Senses:    senses(sight, []string{tunnels}, bram),
			At:        testimony.Stamp{Seq: firstRun, Tick: tick},
		})
		require.NoError(t, tickErr)
	}

	seen, ok := g.Track(bram, handle(sight, goblinBand))
	require.True(t, ok)
	assert.Equal(t, firstRun, seen.Latest().Confirmed.Seq, "a whole run at one Seq")
	assert.Equal(t, uint64(2), seen.Latest().Confirmed.Tick, "ordered inside it by tick")

	require.NoError(t, g.Identify(bram, seen.ID, easternName, seen.Latest().Confirmed))

	keeping, names := testimony.New(), belief.New()
	require.NoError(t, carry.Land(keeping, names, playerBram,
		carry.Out(carry.Input{Observer: bram, Tracks: g.Held(bram), Names: g}).Carried))

	// A season of world happens. Seq advances because things happen.
	for _, kind := range []journal.Kind{"sold-the-loot", "wintered-in-town"} {
		_, appendErr := world.Append(journal.Fact{
			Kind:     kind,
			Actor:    asEntity(bram),
			Audience: journal.Audience{asEntity(bram)},
		})
		require.NoError(t, appendErr)
	}

	laterRun := worldOrder(world)
	require.Equal(t, 3, laterRun)

	back := perception.NewGame()
	back.Mind(bram, reconcile.Woodwise{})
	require.NoError(t, back.Remember(bram,
		carry.Out(carry.Input{Observer: playerBram, Tracks: keeping.Held(playerBram), Names: names}).Carried))

	memory, ok := back.Track(bram, carry.Handle(easternName, sight))
	require.True(t, ok)

	// THE POINT. The first tick of the new run is 1; the memory's tick is 2.
	// On ticks alone the memory looks newer than what he is looking at.
	now := testimony.Stamp{Seq: laterRun, Tick: 1}
	assert.Greater(t, memory.Latest().Confirmed.Tick, now.Tick,
		"ticks alone would call a season-old memory the fresher of the two")
	assert.True(t, memory.Latest().Confirmed.Before(now),
		"and the world's order gets it right")
	assert.Equal(t, 2, now.Seq-memory.Latest().Confirmed.Seq,
		"two things happened in the world since he learned it")
}

// TestABeliefCannotBecomeAFact is the refusal, and the real journal enforces it
// without knowing perception exists.
//
// Every fact names an actor, because a fact is something somebody DID. A track's
// latest entry is something somebody NOTICED, and noticing has no actor to name.
// So there is no shape of belief that the journal will accept.
func TestABeliefCannotBecomeAFact(t *testing.T) {
	world := journal.New()

	_, err := world.Append(journal.Fact{
		Kind:     "goblins-in-the-eastern-tunnels",
		Audience: journal.Audience{asEntity(bram)},
	})
	require.ErrorIs(t, err, journal.ErrEmptyActor)
	assert.Equal(t, 0, world.Len(), "and nothing landed")

	// Worth being exact about what this does and does not buy. The journal is
	// incurious: it will happily record a fact that is false. What it cannot
	// record is a fact with nobody behind it — so the discipline that a lie
	// never reaches the ledger is not enforced here. It holds because the
	// projection, which is the only thing that authors a lie, has no path to
	// Append at all.
	planted, err := world.Append(journal.Fact{
		Kind:     "leads",
		Actor:    asEntity(pip),
		Audience: journal.Audience{asEntity(pip)},
	})
	require.NoError(t, err, "the journal does not check that a fact is true")
	assert.Equal(t, 1, planted.Seq)
}

// TestTheLedgerHoldsTheActNotTheBelief proves the crossing rule against the real
// type. Pip is charmed, believes he is looking at a toy, and acts. What reaches
// the world is what he DID and who saw him do it — never what he thought.
func TestTheLedgerHoldsTheActNotTheBelief(t *testing.T) {
	world := journal.New()

	g := perception.NewGame()
	g.Mind(pip, reconcile.Oblivious{})

	chief := projection.Presence{
		Source: goblinBand,
		Where:  tunnels,
		Says:   map[testimony.Channel]projection.Says{sight: says(content.Creature, "goblin", "holding a sword", tunnels)},
	}
	charm := projection.Forgery{
		Where:     tunnels,
		Source:    charmOnPip,
		Observers: []testimony.Observer{pip},
		Says: map[testimony.Channel]projection.Says{
			sight: says(content.Creature, "goblin", "holding a toy", tunnels),
		},
		Hides:    []testimony.Channel{sight},
		Replaces: goblinBand,
	}

	_, err := g.Tick(projection.Input{
		Presences: []projection.Presence{chief},
		Forgeries: []projection.Forgery{charm},
		Senses:    senses(sight, []string{tunnels}, pip, bram),
		At:        testimony.Stamp{Tick: 1},
	})
	require.NoError(t, err)

	// He walks up to a thing he believes is harmless.
	believed, ok := g.Track(pip, handle(sight, charmOnPip))
	require.True(t, ok)
	require.Equal(t, "holding a toy", mustRead(t, believed.Latest().Payload).Note)

	// The audience comes from what was really there to be seen, not from what
	// pip believed — bram watched him do it.
	_, err = world.Append(journal.Fact{
		Kind:     "walked-up-to",
		Actor:    asEntity(pip),
		Subject:  journal.EntityID(goblinBand),
		Audience: journal.Audience{asEntity(pip), asEntity(bram)},
	})
	require.NoError(t, err)

	for _, fact := range world.All() {
		assert.NotContains(t, string(fact.Kind), "toy",
			"the ledger records the act, and the belief behind it never crossed")
	}

	assert.Equal(t, 1, len(world.WitnessedBy(asEntity(bram))),
		"bram saw what happened regardless of what pip thought he was doing")

	// And pip still believes it, afterwards, unchanged by having acted.
	after, ok := g.Track(pip, handle(sight, charmOnPip))
	require.True(t, ok)
	assert.Equal(t, believed.Entries, after.Entries)
}
