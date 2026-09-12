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
	"github.com/KirkDiggler/rpg-toolkit/examples/perception/projection"
)

// The use cases in README.md, in order, each measured as the story it is.
// Every proof pays for one primitive, and every assertion says why.

// Use case 1 — zombie and captain, one situation, two targets.
//
// A zombie and a captain stand in one room with byte-identical testimony and
// attack different people. The difference is entirely which mind they have.
func TestUseCase1_OneSituationTwoTargets(t *testing.T) {
	s := newScene(t)
	s.mind(zombie, minds.Zombie{}, room)
	s.mind(captain, minds.Captain{}, room)
	s.senses([]string{room}, zombie, captain)

	s.figure(knight, room, armoured, silent)
	s.look()
	s.figure(mage, room, minds.Robed, chanting)
	s.look()

	z, c := s.turn(zombie), s.turn(captain)

	assert.Len(t, z.contactHolding(hearingOf(mage)).Tracks, 1,
		"to the zombie the chant is its own thing")
	assert.True(t, holds(c.contactHolding(hearingOf(mage)), sightOf(mage)),
		"to the captain the chant is the robed figure")

	z.attacks(knight, "the zombie swings at whoever it noticed first")
	c.attacks(mage, "the captain goes for the caster")

	s.look()
	assert.Equal(t, c.contactHolding(hearingOf(mage)).Name, s.turn(captain).contactHolding(hearingOf(mage)).Name,
		"naming landed as a claim: the next situation finds the word already there")
}

// Use case 1, the other half — the captain can be wrong.
//
// The armoured one is the bard. The captain merges the chant into the robed
// figure anyway, because that is what it believes casters look like, and
// nothing it holds could tell it otherwise.
func TestUseCase1_TheCaptainCanBeWrong(t *testing.T) {
	s := newScene(t)
	s.mind(captain, minds.Captain{}, room)
	s.senses([]string{room}, captain)

	s.figure(knight, room, armoured, chanting) // the one actually chanting
	s.figure(mage, room, minds.Robed, silent)
	s.look()

	s.turn(captain).attacks(mage, "confidently wrong, and it cost only a claim")

	assert.Equal(t, knight, projection.Bind(s.truth)[hearingOf(knight)],
		"the truth surface, which the captain never sees, knows better")
}

// Use case 2 — a heal in sight retargets the captain.
//
// The cleric mends the knight where the captain can see it. The deed lands as
// testimony on the captain's deeds channel; the captain's Judge attaches it to
// the hooded figure, and its Rank puts the healer first. The zombie holds the
// very same deed and never attaches it to anyone.
func TestUseCase2_AHealInSightRetargetsTheCaptain(t *testing.T) {
	s := newScene(t)
	s.mind(zombie, minds.Zombie{}, room)
	s.mind(captain, minds.Captain{}, room)
	s.senses([]string{room}, zombie, captain)

	s.figure(knight, room, armoured, silent)
	s.figure(mage, room, minds.Robed, chanting)
	s.figure(cleric, room, hooded, silent)
	s.look()

	s.turn(captain).attacks(mage, "before the heal, the caster")

	s.happens(cleric, minds.Heal, knight, room)

	c := s.turn(captain)
	c.attacks(cleric, "after the heal, the healer")

	healed := c.contactHolding(projection.Handle(deed.Channel, cleric))
	assert.True(t, holds(healed, sightOf(cleric)), "the captain attached the deed to the figure it saw do it")

	z := s.turn(zombie)
	z.attacks(knight, "the zombie saw exactly the same thing and still swings at whoever it saw first")
	assert.Len(t, z.contactHolding(projection.Handle(deed.Channel, cleric)).Tracks, 1,
		"it holds the deed, and it is its own thing, attached to nobody")
}

// Use case 2, the other half — the same heal out of sight changes nothing.
//
// The cleric heals from the corridor, where the captain's senses do not reach.
// No deed lands. Knowledge is the audience of facts, and the captain was not
// in it.
func TestUseCase2_TheSameHealOutOfSightChangesNothing(t *testing.T) {
	s := newScene(t)
	s.mind(captain, minds.Captain{}, room)
	s.senses([]string{room}, captain)

	s.figure(knight, room, armoured, silent)
	s.figure(mage, room, minds.Robed, chanting)
	s.figure(cleric, corridor, hooded, silent)
	s.look()

	s.happens(cleric, minds.Heal, knight, corridor)

	c := s.turn(captain)
	c.attacks(mage, "still the caster")
	c.holdsNothingOf(projection.Handle(deed.Channel, cleric), "it never learned a heal happened")
}

// Use case 3 — the archer keeps its range.
//
// Three regions in a line. The archer shoots the knight from the next region;
// when the knight closes, it steps away rather than shooting; from its new
// region it shoots again. The zombie beside it walks in and swings. The
// archer's whole difference from the zombie is one number its mind returns,
// and a bow on its sheet.
func TestUseCase3_TheArcherKeepsItsRange(t *testing.T) {
	s := newScene(t)
	s.doors(door(room, corridor), door(corridor, hall))
	s.mind(archer, minds.Archer{}, corridor)
	s.bow(archer)
	s.mind(zombie, minds.Zombie{}, corridor)
	s.senses([]string{room, corridor, hall}, archer, zombie)

	s.figure(knight, room, armoured, silent)
	s.look()

	s.turn(archer).attacks(knight, "it fires from the next region")
	s.turn(zombie).walksTo(room, "the zombie has to walk")

	s.moves(knight, corridor)
	s.look()

	a := s.turn(archer)
	a.backsOff(corridor, "too close to shoot: it steps away from where it believes the knight is")
	s.turn(zombie).attacks(knight, "the zombie, same situation, swings")

	a.moved()
	s.look()

	s.turn(archer).attacks(knight, "and shoots again from the next region")
}

// Use case 4 — the intimidated goblin.
//
// A goblin archer at the far end of the line would walk toward the knight.
// Frightened of him, it will not: it flees instead. When the knight comes
// within bowshot anyway, it shoots — a fence forbids approach and nothing
// else. The rule lives in the ladder; the mind was never asked.
func TestUseCase4_TheIntimidatedGoblin(t *testing.T) {
	s := newScene(t)
	s.doors(door(room, corridor), door(corridor, hall))
	s.mind(goblin, minds.Archer{}, hall)
	s.bow(goblin)
	s.senses([]string{room, corridor, hall}, goblin)

	s.figure(knight, room, armoured, silent)
	s.look()

	s.turn(goblin).walksTo(corridor, "unafraid, it closes to bowshot")

	s.frightens(goblin, knight)
	s.look()

	g := s.turn(goblin)
	g.cornered("afraid and out of reach, it will not approach; it would flee, but the hall is a dead end")
	assert.Equal(t, g.contactHolding(sightOf(knight)).Name, g.out.Intent.Target, "and it is him it fears")

	s.moves(knight, corridor)
	s.look()

	s.turn(goblin).attacks(knight, "frightened is not disarmed: within bowshot, it shoots")
}

// Use case 5 — the ghost worth walking to.
//
// The knight is seen in the room and vanishes. Both monsters walk to where
// they last saw him. Finding nothing, the captain goes back to its post; the
// zombie stands on the spot, still believing, for as long as anyone cares to
// tick.
func TestUseCase5_TheGhostWorthWalkingTo(t *testing.T) {
	s := newScene(t)
	s.doors(door(room, corridor), door(corridor, hall))
	s.mind(captain, minds.Captain{}, corridor)
	s.mind(zombie, minds.Zombie{}, corridor)
	s.post(corridor)
	s.senses([]string{room, corridor, hall}, captain, zombie)

	s.figure(knight, room, armoured, silent)
	s.look()

	s.leaves(knight)
	s.look()

	c, z := s.turn(captain), s.turn(zombie)
	c.walksTo(room, "the captain goes to look where it last saw him")
	z.walksTo(room, "so does the zombie")
	c.moved()
	z.moved()
	s.look()

	c = s.turn(captain)
	c.walksTo(corridor, "nothing here: back to the post")
	assert.Equal(t, "my post", string(c.out.Intent.Target))

	s.turn(zombie).stays("the zombie has arrived, and has nowhere else it wants to be")

	s.looks(27)

	z = s.turn(zombie)
	z.stays("still standing there")

	ghost := z.contactHolding(sightOf(knight))
	assert.False(t, ghost.Current(), "still holding him, as a ghost")
	assert.Equal(t, uint64(1), ghost.LastConfirmed().Tick, "last seen at tick one, and nothing has touched that")
}

// Use case 5, the other half — the ghost not worth walking to.
//
// The knight was seen an age ago. The captain will not leave its post for a
// memory that old; the zombie does not know what old means.
func TestUseCase5_TheGhostNotWorthWalkingTo(t *testing.T) {
	s := newScene(t)
	s.doors(door(room, corridor))
	s.mind(captain, minds.Captain{}, corridor)
	s.mind(zombie, minds.Zombie{}, corridor)
	s.post(corridor)
	s.senses([]string{room, corridor}, captain, zombie)

	s.figure(knight, room, armoured, silent)
	s.look()

	s.leaves(knight)
	s.looks(minds.Patience + 1)

	s.turn(captain).stays("too old to be worth the walk, and it is already at its post")
	s.turn(zombie).walksTo(room, "the zombie sets off regardless")
}

// Sanity on the vocabulary itself: an actor nobody placed is a wiring fault,
// and it fails loudly rather than passing forever.
func TestAnActorNobodyPlacedIsAWiringFault(t *testing.T) {
	g := behavior.New()
	g.Mind(&behavior.MindInput{Actor: zombie, Mind: minds.Zombie{}})

	_, err := g.Turn(&behavior.TurnInput{Actor: zombie})
	require.ErrorIs(t, err, behavior.ErrNoSelf)
}
