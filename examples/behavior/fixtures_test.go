// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/examples/behavior"
	"github.com/KirkDiggler/rpg-toolkit/examples/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/examples/behavior/minds"
	"github.com/KirkDiggler/rpg-toolkit/examples/behavior/stage"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/belief"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/content"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/projection"
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// The fixtures in README.md, in order. Each pays for one primitive.

const (
	sight                     = testimony.Sight
	hearing testimony.Channel = "hearing"

	room     = "room"
	corridor = "corridor"
	hall     = "hall"

	zombie  testimony.Observer = "zombie"
	captain testimony.Observer = "captain"
	archer  testimony.Observer = "archer"
	goblin  testimony.Observer = "goblin"

	// Ledger handles. They never leave the projection; the test is the game
	// master and may mint track handles from them to assert with.
	knight = "knight"
	mage   = "mage"
	cleric = "cleric"
	banner = "banner"
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
	return standingAt(source, room)
}

// standingAt puts an actor somewhere on the truth surface where nothing
// perceives it.
func standingAt(source, where string) projection.Presence {
	return projection.Presence{Source: source, Where: where}
}

// figure is somebody in the room, seen, and heard if chanting.
func figure(source, look string, chanting bool) projection.Presence {
	return figureAt(source, room, look, chanting)
}

// figureAt is somebody somewhere, seen, and heard if chanting.
func figureAt(source, where, look string, chanting bool) projection.Presence {
	p := projection.Presence{
		Source: source,
		Where:  where,
		Says:   map[testimony.Channel]projection.Says{sight: says(content.Creature, "human", look, where)},
	}

	if chanting {
		p.Says[hearing] = says(content.Noise, "", minds.Chanting, where)
	}

	return p
}

// bothSenses gives each observer sight and hearing over the room.
func bothSenses(observers ...testimony.Observer) []projection.Sense {
	return reaching([]string{room}, observers...)
}

// reaching gives each observer sight and hearing over the given places.
func reaching(reach []string, observers ...testimony.Observer) []projection.Sense {
	out := make([]projection.Sense, 0, 2*len(observers))
	for _, o := range observers {
		out = append(out,
			projection.Sense{Observer: o, Channel: sight, Reach: reach},
			projection.Sense{Observer: o, Channel: hearing, Reach: reach},
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
		Senses: bothSenses(zombie, captain),
		At:     moment(1),
	}))

	// Tick 2: the mage walks in, robed, and starts chanting.
	in := projection.Input{
		Presences: []projection.Presence{
			standing(string(zombie)), standing(string(captain)),
			figure(knight, "armoured", false),
			figure(mage, minds.Robed, true),
		},
		Senses: bothSenses(zombie, captain),
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
		Senses: bothSenses(captain),
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

// TestFixture2_AHealInSightRetargetsTheCaptain: the cleric mends the knight
// where the captain can see it. The deed lands as testimony on the captain's
// deeds channel; the captain's Judge attaches it to the hooded figure, and its
// Rank puts the healer first. The zombie holds the very same deed and never
// attaches it to anyone.
func TestFixture2_AHealInSightRetargetsTheCaptain(t *testing.T) {
	g := behavior.New()
	g.Mind(zombie, minds.Zombie{})
	g.Mind(captain, minds.Captain{})

	in := projection.Input{
		Presences: []projection.Presence{
			standing(string(zombie)), standing(string(captain)),
			figure(knight, "armoured", false),
			figure(mage, minds.Robed, true),
			figure(cleric, "hooded", false),
		},
		Senses: bothSenses(zombie, captain),
		At:     moment(1),
	}
	require.NoError(t, g.Tick(in))

	ci, cs, err := g.Turn(captain, moment(1))
	require.NoError(t, err)
	require.Equal(t, mage, stage.Aim(in, cs, ci), "before the heal, the caster")

	// The cleric heals the knight, in the room, in front of everyone.
	heal := stage.Deed{Actor: cleric, Target: knight, Verb: minds.Heal, Where: room}
	require.NoError(t, stage.Land(g, in, heal, moment(2)))

	ci, cs, err = g.Turn(captain, moment(2))
	require.NoError(t, err)
	assert.Equal(t, cleric, stage.Aim(in, cs, ci), "after the heal, the healer")

	healed := projection.Handle(deed.Channel, cleric)
	hooded := projection.Handle(sight, cleric)

	cc, held := bundleOf(cs, healed)
	require.True(t, held)
	assert.True(t, holds(cc, hooded), "the captain attached the deed to the figure it saw do it")
	assert.False(t, cc.Tracks[0].Current && cc.Tracks[len(cc.Tracks)-1].Current,
		"a deed is never current; only the figure is")

	// The zombie saw exactly the same thing and it changed nothing.
	zi, zs, err := g.Turn(zombie, moment(2))
	require.NoError(t, err)
	assert.Equal(t, knight, stage.Aim(in, zs, zi), "the zombie still swings at whoever it saw first")

	zc, held := bundleOf(zs, healed)
	require.True(t, held, "it holds the deed")
	assert.Len(t, zc.Tracks, 1, "and it is its own thing, attached to nobody")
}

// TestFixture2_TheSameHealOutOfSightChangesNothing: the cleric heals from the
// corridor, where the captain's senses do not reach. No deed lands, and the
// captain goes on believing what it believed. Knowledge is the audience of
// facts, and the captain was not in it.
func TestFixture2_TheSameHealOutOfSightChangesNothing(t *testing.T) {
	g := behavior.New()
	g.Mind(captain, minds.Captain{})

	in := projection.Input{
		Presences: []projection.Presence{
			standing(string(captain)),
			figure(knight, "armoured", false),
			figure(mage, minds.Robed, true),
			figureAt(cleric, corridor, "hooded", false),
		},
		Senses: bothSenses(captain),
		At:     moment(1),
	}
	require.NoError(t, g.Tick(in))

	heal := stage.Deed{Actor: cleric, Target: knight, Verb: minds.Heal, Where: corridor}
	require.NoError(t, stage.Land(g, in, heal, moment(2)))

	ci, cs, err := g.Turn(captain, moment(2))
	require.NoError(t, err)
	assert.Equal(t, mage, stage.Aim(in, cs, ci), "still the caster")

	_, held := bundleOf(cs, projection.Handle(deed.Channel, cleric))
	assert.False(t, held, "it never learned a heal happened")
}

// TestFixture3_TheArcherKeepsItsRange: three regions in a line. The archer
// shoots the knight from the next region; when the knight closes, it steps
// away rather than shooting; from its new region it shoots again. The zombie
// beside it walks in and swings. The archer's whole difference from the zombie
// is one number its mind returns, and a bow on its sheet.
func TestFixture3_TheArcherKeepsItsRange(t *testing.T) {
	g := behavior.New()
	g.Connect(room, corridor)
	g.Connect(corridor, hall)
	g.Mind(archer, minds.Archer{})
	g.Sheet(archer, behavior.Sheet{Reach: 1})
	g.Mind(zombie, minds.Zombie{})

	everywhere := []string{room, corridor, hall}

	// Both stand in the corridor; the knight is in the room next door.
	in := projection.Input{
		Presences: []projection.Presence{
			standingAt(string(archer), corridor), standingAt(string(zombie), corridor),
			figureAt(knight, room, "armoured", false),
		},
		Senses: reaching(everywhere, archer, zombie),
		At:     moment(1),
	}
	require.NoError(t, g.Tick(in))

	ai, as, err := g.Turn(archer, moment(1))
	require.NoError(t, err)
	require.Equal(t, behavior.Attack, ai.Verb, "it fires from the next region")
	assert.Equal(t, knight, stage.Aim(in, as, ai))

	zi, zs, err := g.Turn(zombie, moment(1))
	require.NoError(t, err)
	require.Equal(t, behavior.Toward, zi.Verb, "the zombie has to walk")

	to, ok := stage.Step(g, zs, zi)
	require.True(t, ok)
	assert.Equal(t, room, to)

	// The knight steps into the corridor with them.
	in.Presences[2] = figureAt(knight, corridor, "armoured", false)
	in.At = moment(2)
	require.NoError(t, g.Tick(in))

	ai, as, err = g.Turn(archer, moment(2))
	require.NoError(t, err)
	require.Equal(t, behavior.Away, ai.Verb, "too close to shoot: it steps away first")

	to, ok = stage.Step(g, as, ai)
	require.True(t, ok)

	believed, _ := stage.Recall(as, ai.Target)
	assert.Equal(t, corridor, believed, "it steps away from where it believes the knight is")
	assert.NotEqual(t, corridor, to)
	assert.Contains(t, everywhere, to)

	zi, zs, err = g.Turn(zombie, moment(2))
	require.NoError(t, err)
	require.Equal(t, behavior.Attack, zi.Verb, "the zombie, same situation, swings")
	assert.Equal(t, knight, stage.Aim(in, zs, zi))

	// The archer is where it stepped; the knight is still in the corridor.
	in.Presences[0] = standingAt(string(archer), to)
	in.At = moment(3)
	require.NoError(t, g.Tick(in))

	ai, as, err = g.Turn(archer, moment(3))
	require.NoError(t, err)
	require.Equal(t, behavior.Attack, ai.Verb, "and shoots again from the next region")
	assert.Equal(t, knight, stage.Aim(in, as, ai))
}

// TestFixture4_TheIntimidatedGoblin: a goblin archer at the far end of the
// line would walk toward the knight. Frightened of him, it will not: it
// flees instead. When the knight comes within bowshot anyway, it shoots — a
// fence forbids approach and nothing else. The rule lives in the ladder; the
// mind was never asked.
func TestFixture4_TheIntimidatedGoblin(t *testing.T) {
	g := behavior.New()
	g.Connect(room, corridor)
	g.Connect(corridor, hall)
	g.Mind(goblin, minds.Archer{})
	g.Sheet(goblin, behavior.Sheet{Reach: 1})

	everywhere := []string{room, corridor, hall}

	// Goblin in the hall, knight two regions away in the room.
	in := projection.Input{
		Presences: []projection.Presence{
			standingAt(string(goblin), hall),
			figureAt(knight, room, "armoured", false),
		},
		Senses: reaching(everywhere, goblin),
		At:     moment(1),
	}
	require.NoError(t, g.Tick(in))

	gi, gs, err := g.Turn(goblin, moment(1))
	require.NoError(t, err)
	require.Equal(t, behavior.Toward, gi.Verb, "unafraid, it closes to bowshot")

	to, ok := stage.Step(g, gs, gi)
	require.True(t, ok)
	assert.Equal(t, corridor, to)

	// The knight intimidates it. The condition lands on the sheet.
	g.Frighten(goblin, knight)
	in.At = moment(2)
	require.NoError(t, g.Tick(in))

	gi, gs, err = g.Turn(goblin, moment(2))
	require.NoError(t, err)
	require.Equal(t, behavior.Away, gi.Verb, "afraid and out of reach, it will not approach: it flees")
	assert.Equal(t, gi.Target, mustName(t, gs, projection.Handle(sight, knight)), "from him, specifically")

	_, ok = stage.Step(g, gs, gi)
	assert.False(t, ok, "but the hall is a dead end, so there is nowhere to go")

	// The knight comes to the corridor: within bowshot.
	in.Presences[1] = figureAt(knight, corridor, "armoured", false)
	in.At = moment(3)
	require.NoError(t, g.Tick(in))

	gi, gs, err = g.Turn(goblin, moment(3))
	require.NoError(t, err)
	require.Equal(t, behavior.Attack, gi.Verb, "frightened is not disarmed: it shoots")
	assert.Equal(t, knight, stage.Aim(in, gs, gi))
}

// mustName is what the actor calls the contact holding a track.
func mustName(t *testing.T, s behavior.Situation, track testimony.TrackID) belief.Name {
	t.Helper()

	c, held := bundleOf(s, track)
	require.True(t, held)
	require.True(t, c.Named)

	return c.Name
}

// post is a place a guard was told to stand, perceived like anything else.
func post(where string) projection.Presence {
	return projection.Presence{
		Source: banner,
		Where:  where,
		Says:   map[testimony.Channel]projection.Says{sight: says(minds.PostKind, "banner", "", where)},
	}
}

// TestFixture5_TheGhostWorthWalkingTo: the knight is seen in the room and
// vanishes. Both monsters walk to where they last saw him. Finding nothing,
// the captain goes back to its post; the zombie stands on the spot, still
// believing, for as long as anyone cares to tick.
func TestFixture5_TheGhostWorthWalkingTo(t *testing.T) {
	g := behavior.New()
	g.Connect(room, corridor)
	g.Connect(corridor, hall)
	g.Mind(captain, minds.Captain{})
	g.Mind(zombie, minds.Zombie{})

	everywhere := []string{room, corridor, hall}

	// Tick 1: both at the post in the corridor; the knight is in the room.
	in := projection.Input{
		Presences: []projection.Presence{
			standingAt(string(captain), corridor), standingAt(string(zombie), corridor), post(corridor),
			figureAt(knight, room, "armoured", false),
		},
		Senses: reaching(everywhere, captain, zombie),
		At:     moment(1),
	}
	require.NoError(t, g.Tick(in))

	// Tick 2: the knight is gone. Looking and finding nothing is a write.
	in.Presences = in.Presences[:3]
	in.At = moment(2)
	require.NoError(t, g.Tick(in))

	ci, cs, err := g.Turn(captain, moment(2))
	require.NoError(t, err)
	require.Equal(t, behavior.Toward, ci.Verb, "the captain goes to look where it last saw him")

	to, ok := stage.Step(g, cs, ci)
	require.True(t, ok)
	require.Equal(t, room, to)

	zi, _, err := g.Turn(zombie, moment(2))
	require.NoError(t, err)
	require.Equal(t, behavior.Toward, zi.Verb, "so does the zombie")

	// Tick 3: both stand in the room. Nothing is there.
	in.Presences[0] = standingAt(string(captain), room)
	in.Presences[1] = standingAt(string(zombie), room)
	in.At = moment(3)
	require.NoError(t, g.Tick(in))

	ci, cs, err = g.Turn(captain, moment(3))
	require.NoError(t, err)
	require.Equal(t, behavior.Toward, ci.Verb)
	assert.Equal(t, belief.Name("my post"), ci.Target, "nothing here: back to the post")

	to, ok = stage.Step(g, cs, ci)
	require.True(t, ok)
	assert.Equal(t, corridor, to)

	zi, _, err = g.Turn(zombie, moment(3))
	require.NoError(t, err)
	assert.Equal(t, behavior.Pass, zi.Verb, "the zombie has arrived, and has nowhere else it wants to be")

	// Tick 30: the zombie is still standing there, still believing.
	in.At = moment(30)
	require.NoError(t, g.Tick(in))

	zi, zs, err := g.Turn(zombie, moment(30))
	require.NoError(t, err)
	assert.Equal(t, behavior.Pass, zi.Verb)

	ghost, held := bundleOf(zs, projection.Handle(sight, knight))
	require.True(t, held, "it still holds him")
	assert.False(t, ghost.Current(), "as a ghost")
	assert.Equal(t, uint64(1), ghost.LastConfirmed().Tick, "last seen at tick one, and nothing has touched that")
}

// TestFixture5_TheGhostNotWorthWalkingTo: the knight was seen an age ago. The
// captain will not leave its post for a memory that old; the zombie does not
// know what old means.
func TestFixture5_TheGhostNotWorthWalkingTo(t *testing.T) {
	g := behavior.New()
	g.Connect(room, corridor)
	g.Mind(captain, minds.Captain{})
	g.Mind(zombie, minds.Zombie{})

	in := projection.Input{
		Presences: []projection.Presence{
			standingAt(string(captain), corridor), standingAt(string(zombie), corridor), post(corridor),
			figureAt(knight, room, "armoured", false),
		},
		Senses: reaching([]string{room, corridor}, captain, zombie),
		At:     moment(1),
	}
	require.NoError(t, g.Tick(in))

	in.Presences = in.Presences[:3]

	for tick := uint64(2); tick <= minds.Patience+2; tick++ {
		in.At = moment(tick)
		require.NoError(t, g.Tick(in))
	}

	stale := moment(minds.Patience + 2)

	ci, _, err := g.Turn(captain, stale)
	require.NoError(t, err)
	assert.Equal(t, behavior.Pass, ci.Verb, "too old to be worth the walk; it is already at its post")

	zi, _, err := g.Turn(zombie, stale)
	require.NoError(t, err)
	assert.Equal(t, behavior.Toward, zi.Verb, "the zombie sets off regardless")
}
