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
)

// What a name holds on to (R4). A bearer is the one handle a word survives
// on, so a bearer nobody can reach is a word nobody can use.

// The shooter steps out of sight after the shot.
//
// The captain sees zara, is shot at, and loses sight of her before it has
// ever taken a situation — so the first contact it folds of her is a ghost
// and a deed, with no current creature to bear the name. Holdings sort by
// subject and "deeds|zara" sorts before "zara", so the first rung of the
// old bearer handed the name to the deed. A deed is never present on any
// truth: the word would have been recorded on a handle no swing can land
// on and no driver can match, for as long as the captain lived.
func TestANameIsNeverBorneByADeed(t *testing.T) {
	s := newScene(t)
	mind := &asked{Mind: captainMind{}}
	s.mind(captain, mind, room)
	s.sees([]string{room}, captain)

	s.person(zara, room, hooded, silent)
	s.look()
	s.happens(zara, "attack", captain, room)
	s.moves(zara, hall)
	s.look()

	first := contactOf(t, s.situation(captain), zara)
	require.True(t, first.Holds(deed.Subject(zara)), "a ghost and a deed, folded as one person")
	require.False(t, first.Current(), "and nothing is delivering her now")

	assert.Equal(t, behavior.Name("the hooded human"), first.Name, "it has a word for her")
	assert.Equal(t, zara, first.Bearer, "and the word is on zara, not on what zara did")

	second := contactOf(t, s.situation(captain), zara)
	assert.Equal(t, zara, second.Bearer, "the next situation finds it recorded under zara")
	assert.Equal(t, first.Name, second.Name, "and reads back the same word")
	assert.Equal(t, 1, mind.names, "so the captain was never asked a second time")
}

// A contact of deeds alone bears its first deed.
//
// The zombie hears the heal happen in its room and never saw who did it, so
// the only handle it holds for that contact is the deeds subject. There is
// nothing better to put the word on, and the word is still worth keeping:
// the last rung is not a fallback that fires by accident, it is the answer
// when a contact really is nothing but deeds.
func TestAContactOfDeedsAloneBearsItsDeed(t *testing.T) {
	s := newScene(t)
	s.mind(zombie, zombieMind{}, room)
	s.sees([]string{room}, zombie)

	s.person(zara, hall, hooded, silent) // out of the zombie's reach, always
	s.look()
	s.happens(zara, heal, knight, room)

	only := contactOf(t, s.situation(zombie), deed.Subject(zara))
	require.Len(t, only.Holdings, 1, "a deed and nothing else")

	assert.Equal(t, deed.Subject(zara), only.Bearer, "there is no other handle to hold the word")
}

// contactOf is the situation's contact that holds a subject.
func contactOf(t *testing.T, s *behavior.Situation, subject core.EntityID) behavior.Contact {
	t.Helper()

	for _, c := range s.Contacts {
		if c.Holds(subject) {
			return c
		}
	}

	require.Failf(t, "not held", "%s holds nothing of %s", s.Actor, subject)

	return behavior.Contact{}
}
