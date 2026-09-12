// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/examples/behavior"
	"github.com/KirkDiggler/rpg-toolkit/examples/behavior/minds"
	"github.com/KirkDiggler/rpg-toolkit/examples/behavior/stage"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/content"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/projection"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// The fixtures in README.md, in order. Each pays for one primitive.

const (
	sight                     = testimony.Sight
	hearing testimony.Channel = "hearing"

	room = "room"

	zombie  testimony.Observer = "zombie"
	captain testimony.Observer = "captain"

	// Ledger handles. They never leave the projection; the test is the game
	// master and may mint track handles from them to assert with.
	knight = "knight"
	mage   = "mage"
)

func moment(tick uint64) testimony.Stamp { return testimony.Stamp{Tick: tick} }

func says(kind content.Kind, of, note, where string) projection.Says {
	p := content.Percept{Kind: kind, Of: of, Note: note}

	return projection.Says{Payload: content.Encode(p), ChangeKey: content.Key(p), Locus: testimony.Locus{Where: where}}
}

// standing puts an actor on the truth surface where nothing perceives it. The
// monsters in these fixtures do not perceive each other; a presence that says
// nothing to any channel is how that is spelled.
func standing(source string) projection.Presence {
	return projection.Presence{Source: source, Where: room}
}

// figure is somebody seen, and heard if chanting.
func figure(source, look string, chanting bool) projection.Presence {
	p := projection.Presence{
		Source: source,
		Where:  room,
		Says:   map[testimony.Channel]projection.Says{sight: says(content.Creature, "human", look, room)},
	}

	if chanting {
		p.Says[hearing] = says(content.Noise, "", minds.Chanting, room)
	}

	return p
}

func bothSenses(where string, observers ...testimony.Observer) []projection.Sense {
	out := make([]projection.Sense, 0, 2*len(observers))
	for _, o := range observers {
		out = append(out,
			projection.Sense{Observer: o, Channel: sight, Reach: []string{where}},
			projection.Sense{Observer: o, Channel: hearing, Reach: []string{where}},
		)
	}

	return out
}

func bundleOf(s behavior.Situation, track testimony.TrackID) (behavior.Contact, bool) {
	for _, c := range s.Contacts {
		for _, v := range c.Tracks {
			if v.ID == track {
				return c, true
			}
		}
	}

	return behavior.Contact{}, false
}

// TestFixture1_OneSituationTwoTargets: a zombie and a captain stand in one
// room with byte-identical testimony, and attack different people. The
// difference is entirely which mind they have.
func TestFixture1_OneSituationTwoTargets(t *testing.T) {
	g := behavior.New()
	g.Mind(zombie, minds.Zombie{})
	g.Mind(captain, minds.Captain{})

	// Tick 1: the knight walks in.
	require.NoError(t, g.Tick(projection.Input{
		Presences: []projection.Presence{
			standing(string(zombie)), standing(string(captain)),
			figure(knight, "armoured", false),
		},
		Senses: bothSenses(room, zombie, captain),
		At:     moment(1),
	}))

	// Tick 2: the mage walks in, robed, and starts chanting.
	in := projection.Input{
		Presences: []projection.Presence{
			standing(string(zombie)), standing(string(captain)),
			figure(knight, "armoured", false),
			figure(mage, minds.Robed, true),
		},
		Senses: bothSenses(room, zombie, captain),
		At:     moment(2),
	}
	require.NoError(t, g.Tick(in))

	chant := projection.Handle(hearing, mage)
	robed := projection.Handle(sight, mage)

	// The zombie holds the chant and the figure as two things.
	zi, zs, err := g.Turn(zombie, moment(2))
	require.NoError(t, err)

	zc, held := bundleOf(zs, chant)
	require.True(t, held)
	assert.Len(t, zc.Tracks, 1, "to the zombie the chant is its own thing")

	// The captain has folded them into one.
	ci, cs, err := g.Turn(captain, moment(2))
	require.NoError(t, err)

	cc, held := bundleOf(cs, chant)
	require.True(t, held)
	assert.True(t, holds(cc, robed), "to the captain the chant is the robed figure")

	// Same room, same senses, same bytes: two intents.
	require.Equal(t, behavior.Attack, zi.Verb)
	require.Equal(t, behavior.Attack, ci.Verb)

	assert.Equal(t, knight, stage.Aim(in, zs, zi), "the zombie swings at whoever it noticed first")
	assert.Equal(t, mage, stage.Aim(in, cs, ci), "the captain goes for the caster")

	// Naming persisted as a claim: the next situation finds the words already there.
	again, err := g.Situation(captain, moment(3))
	require.NoError(t, err)

	cc2, _ := bundleOf(again, chant)
	assert.Equal(t, cc.Name, cc2.Name)
}

// TestFixture1_TheCaptainCanBeWrong: the armoured one is the bard. The captain
// merges the chant into the robed figure anyway, because that is what it
// believes casters look like, and nothing it holds could tell it otherwise.
func TestFixture1_TheCaptainCanBeWrong(t *testing.T) {
	g := behavior.New()
	g.Mind(captain, minds.Captain{})

	in := projection.Input{
		Presences: []projection.Presence{
			standing(string(captain)),
			figure(knight, "armoured", true), // the one actually chanting
			figure(mage, minds.Robed, false), // silent
		},
		Senses: bothSenses(room, captain),
		At:     moment(1),
	}
	require.NoError(t, g.Tick(in))

	ci, cs, err := g.Turn(captain, moment(1))
	require.NoError(t, err)
	require.Equal(t, behavior.Attack, ci.Verb)

	assert.Equal(t, mage, stage.Aim(in, cs, ci), "confidently wrong, and it cost only a claim")

	// The truth surface, which the captain never sees, knows better.
	assert.Equal(t, knight, projection.Bind(in)[projection.Handle(hearing, knight)])
}

func holds(c behavior.Contact, track testimony.TrackID) bool {
	for _, v := range c.Tracks {
		if v.ID == track {
			return true
		}
	}

	return false
}
