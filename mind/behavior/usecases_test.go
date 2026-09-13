// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/stage"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// The use cases in docs/ideas/mind/behavior/design.md, in order, each
// measured as the story it is. Every proof pays for one rule, and every
// assertion says why.

// Use case 1 — zombie and captain, one situation, two targets.
//
// A zombie and a captain stand in one room with byte-identical testimony and
// attack different people. The difference is entirely which mind they have.
func TestUseCase1_OneSituationTwoTargets(t *testing.T) {
	s := newScene(t)
	s.mind(zombie, zombieMind{}, room)
	s.mind(captain, captainMind{}, room)
	s.sees([]string{room}, zombie, captain)

	s.person(knight, room, armoured, silent)
	s.look()
	s.person(mage, room, robed, chants)
	s.look()

	z, c := s.turn(zombie), s.turn(captain)

	assert.Len(t, z.contactHolding(hearingOf(mage)).Holdings, 1,
		"to the zombie the chant is its own thing")
	assert.True(t, c.contactHolding(hearingOf(mage)).Holds(mage),
		"to the captain the chant is the robed figure")

	z.attacks(knight, "the zombie swings at whoever it noticed first")
	c.attacks(mage, "the captain goes for the caster")

	s.look()
	assert.Equal(t, c.contactHolding(hearingOf(mage)).Name, s.turn(captain).contactHolding(hearingOf(mage)).Name,
		"naming persisted: the next situation finds the word already there")
}

// Use case 1, the other half — the captain can be wrong.
//
// The armoured one is the bard. The captain merges the chant into the robed
// figure anyway, because that is what it believes casters look like, and
// nothing it holds could tell it otherwise.
func TestUseCase1_TheCaptainCanBeWrong(t *testing.T) {
	s := newScene(t)
	s.mind(captain, captainMind{}, room)
	s.sees([]string{room}, captain)

	s.person(knight, room, armoured, chants) // the one actually chanting
	s.person(mage, room, robed, silent)
	s.look()

	c := s.turn(captain)
	c.attacks(mage, "confidently wrong, and it cost only a claim")
	assert.True(t, c.contactHolding(hearingOf(knight)).Holds(mage),
		"the knight's chant, bundled into the robed figure")
}

// Use case 2 — a heal in sight retargets the captain.
//
// The cleric mends the knight where the captain can see it. The deed lands
// as testimony on the captain's deeds channel; the captain's Judge attaches
// it to the hooded figure, and its Rank puts the healer first. The zombie
// holds the very same deed and never attaches it to anyone.
func TestUseCase2_AHealInSightRetargetsTheCaptain(t *testing.T) {
	s := newScene(t)
	s.mind(zombie, zombieMind{}, room)
	s.mind(captain, captainMind{}, room)
	s.sees([]string{room}, zombie, captain)

	s.person(knight, room, armoured, silent)
	s.look()
	s.person(mage, room, robed, chants)
	s.person(cleric, room, hooded, silent)
	s.look()

	s.turn(captain).attacks(mage, "before the heal, the caster")

	s.happens(cleric, heal, knight, room)

	c := s.turn(captain)
	c.attacks(cleric, "after the heal, the healer")

	healed := c.contactHolding(deed.Subject(cleric))
	assert.True(t, healed.Holds(cleric), "the captain attached the deed to the figure it saw do it")
	assert.False(t, healed.Holdings[0].CurrentOn(deed.Channel), "a deed is in the past the moment it exists")

	z := s.turn(zombie)
	z.attacks(knight, "the zombie saw exactly the same thing and still swings at whoever it saw first")
	assert.Len(t, z.contactHolding(deed.Subject(cleric)).Holdings, 1,
		"it holds the deed, and it is its own thing, attached to nobody")
}

// Use case 2, the other half — the same heal out of sight changes nothing.
//
// The cleric heals from the corridor, where the captain's senses do not
// reach. No deed lands. Knowledge is the audience of facts, and the captain
// was not in it.
func TestUseCase2_TheSameHealOutOfSightChangesNothing(t *testing.T) {
	s := newScene(t)
	s.mind(captain, captainMind{}, room)
	s.sees([]string{room}, captain)

	s.person(knight, room, armoured, silent)
	s.person(mage, room, robed, chants)
	s.person(cleric, corridor, hooded, silent)
	s.look()

	s.happens(cleric, heal, knight, corridor)

	c := s.turn(captain)
	c.attacks(mage, "still the caster")
	c.holdsNothingOf(deed.Subject(cleric), "it never learned a heal happened")
}

// Use case 3 — the archer keeps its range.
//
// Three rooms in a line. The archer shoots the knight from the next
// room; when the knight closes, it steps away rather than shooting; from
// its new room it shoots again. The zombie beside it walks in and swings.
// The archer's whole difference from the zombie is one number its mind
// returns, and a bow on its sheet.
func TestUseCase3_TheArcherKeepsItsRange(t *testing.T) {
	s := newScene(t)
	s.doors(door(room, corridor), door(corridor, hall))
	s.mind(archer, archerMind{}, corridor)
	s.bow(archer)
	s.mind(zombie, zombieMind{}, corridor)
	s.sees([]string{room, corridor, hall}, archer, zombie)

	s.person(knight, room, armoured, silent)
	s.look()

	s.turn(archer).attacks(knight, "it fires from the next room")
	s.turn(zombie).walksTo(room, "the zombie has to walk")

	s.moves(knight, corridor)
	s.look()

	a := s.turn(archer)
	a.backsOff(corridor, "too close to shoot: it steps away from where it believes the knight is")
	s.turn(zombie).attacks(knight, "the zombie, same situation, swings")

	a.moved()
	s.look()

	s.turn(archer).attacks(knight, "and shoots again from the next room")
}

// Use case 3, the other half — the archer with its back to the wall.
//
// A room with no doors, the knight in it. The archer would rather keep a
// step between them; there is nowhere to step. Keeping range is a
// preference, not a fear: it stands and shoots rather than spending the
// turn on a flight the space refuses.
func TestUseCase3_TheArcherWithItsBackToTheWall(t *testing.T) {
	s := newScene(t)
	s.mind(archer, archerMind{}, room)
	s.bow(archer)
	s.sees([]string{room}, archer)

	s.person(knight, room, armoured, silent)
	s.look()

	s.turn(archer).attacks(knight, "nowhere to step away to, so it shoots")
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
	s.mind(goblin, archerMind{}, hall)
	s.bow(goblin)
	s.sees([]string{room, corridor, hall}, goblin)

	s.person(knight, room, armoured, silent)
	s.look()

	s.turn(goblin).walksTo(corridor, "unafraid, it closes to bowshot")

	s.frightens(goblin, knight)
	s.look()

	g := s.turn(goblin)
	g.cornered("afraid and out of reach, it will not approach; it would flee, but the hall is a dead end")
	assert.Equal(t, g.contactHolding(knight).Name, g.out.Intent.Target, "and it is him it fears")

	s.moves(knight, corridor)
	s.look()

	s.turn(goblin).attacks(knight, "frightened is not disarmed: within bowshot, it shoots")
}

// Use case 5 — the ghost worth walking to.
//
// The knight is seen in the room and vanishes. Both monsters walk to where
// they last saw him. Finding nothing, the captain goes back to its post; the
// zombie stands on the spot, still believing, for as long as anyone cares
// to tick.
func TestUseCase5_TheGhostWorthWalkingTo(t *testing.T) {
	s := newScene(t)
	s.doors(door(room, corridor), door(corridor, hall))
	s.mind(captain, captainMind{}, corridor)
	s.mind(zombie, zombieMind{}, corridor)
	s.post(corridor)
	s.sees([]string{room, corridor, hall}, captain, zombie)

	s.person(knight, room, armoured, silent)
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
	assert.Equal(t, behavior.Name("my post"), c.out.Intent.Target)

	s.turn(zombie).stays("the zombie has arrived, and has nowhere else it wants to be")

	s.looks(27)

	z = s.turn(zombie)
	z.stays("still standing there")

	ghost := z.contactHolding(knight)
	assert.False(t, ghost.Current(), "still holding him, as a ghost")
	assert.Equal(t, uint64(1), ghost.LastConfirmed(), "last seen at tick one, and nothing has touched that")
}

// Use case 5, the other half — the ghost not worth walking to.
//
// The knight was seen an age ago. The captain will not leave its post for a
// memory that old; the zombie does not know what old means.
func TestUseCase5_TheGhostNotWorthWalkingTo(t *testing.T) {
	s := newScene(t)
	s.doors(door(room, corridor))
	s.mind(captain, captainMind{}, corridor)
	s.mind(zombie, zombieMind{}, corridor)
	s.post(corridor)
	s.sees([]string{room, corridor}, captain, zombie)

	s.person(knight, room, armoured, silent)
	s.look()

	s.leaves(knight)
	s.looks(patience + 1)

	s.turn(captain).stays("too old to be worth the walk, and it is already at its post")
	s.turn(zombie).walksTo(room, "the zombie sets off regardless")
}

// The sentinels, each from a call that returns it. An actor nobody placed
// is a wiring fault, and it fails loudly rather than passing forever.
func TestWiringFaultsFailLoudly(t *testing.T) {
	_, err := behavior.New(nil)
	require.ErrorIs(t, err, behavior.ErrNoReader)

	_, err = behavior.New(&behavior.NewInput{Reader: reader{}})
	require.ErrorIs(t, err, behavior.ErrNoSpace)

	g, err := behavior.New(&behavior.NewInput{Reader: reader{}, Space: &rooms{doors: make(map[string][]string)}})
	require.NoError(t, err)

	_, err = g.Turn(&behavior.TurnInput{Actor: zombie})
	require.ErrorIs(t, err, behavior.ErrNoMind)

	g.Mind(&behavior.MindInput{Actor: zombie, Mind: zombieMind{}})

	_, err = g.Turn(&behavior.TurnInput{Actor: zombie})
	require.ErrorIs(t, err, behavior.ErrNoSelf)
}

// A creature of unknown place is never fled and never attacked. Known to be
// there, not known where, is not nearer than anything: a mind that keeps
// more distance than the grain can measure must not spend its turn fleeing
// something the stage could not walk away from.
func TestACreatureOfUnknownPlaceIsNeverFled(t *testing.T) {
	skittish := keeps{zombieMind{}, 3}
	unplaced := behavior.Contact{
		Holdings: []behavior.Holding{{
			Holding: perception.Holding{Subject: knight, CurrentVia: []perception.Channel{sight}},
			Reading: behavior.Reading{Creature: true},
		}},
		Name: "thing knight", Named: true, Bearer: knight,
	}

	r := &rooms{doors: make(map[string][]string)}
	r.connect(room, corridor)

	out, err := behavior.Decide(&behavior.DecideInput{
		Situation: behavior.Situation{Contacts: []behavior.Contact{unplaced}, Self: behavior.Self{Where: room}},
		Mind:      skittish,
		Space:     r,
	})
	require.NoError(t, err)
	assert.Equal(t, behavior.Pass, out.Intent.Verb, "nothing it can act on")
}

// A situation is the caller's to keep. Rewriting what it reports must not
// reach the game's own sheets.
func TestASituationDoesNotAliasTheGame(t *testing.T) {
	s := newScene(t)
	s.mind(zombie, zombieMind{}, room)
	s.frightens(zombie, knight)

	first := s.turn(zombie).out.Situation.Self
	first.Fences[0] = mage

	second := s.turn(zombie).out.Situation.Self
	assert.Equal(t, []core.EntityID{knight}, second.Fences, "the fear is what it was")
}

// A deed nobody did is a wiring fault, not a rumour.
func TestADeedWithNoActorIsRefused(t *testing.T) {
	s := newScene(t)
	s.mind(zombie, zombieMind{}, room)

	err := stage.Land(&stage.LandInput{
		Store:     s.p,
		Deed:      deed.Deed{Verb: heal, Where: room},
		Witnesses: []core.EntityID{zombie},
	})
	require.ErrorIs(t, err, stage.ErrNoActor)
}
