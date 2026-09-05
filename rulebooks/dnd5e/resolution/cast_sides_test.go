// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The cast these pins read castView's answers against: two characters, two
// monsters, and one id ("stranger") attached to the interaction but placed
// in no run.
const (
	heroA     = "hero-a"
	heroB     = "hero-b"
	monsterA  = "monster-a"
	monsterB  = "monster-b"
	strangeID = "stranger"
)

// sidesCast is the participants every pin here holds. Nil sheets are fine —
// nothing a side question answers looks behind an id.
func sidesCast() *Participants {
	return &Participants{
		characters: map[string]*character.Character{
			heroA:     nil,
			heroB:     nil,
			strangeID: nil,
		},
		monsters: map[string]*monster.Monster{
			monsterA: nil,
			monsterB: nil,
		},
		order: []string{heroA, heroB, monsterA, monsterB, strangeID},
	}
}

// sidesRun is a run that declares nothing about sides: two players and two
// monsters on one floor, so every answer is the default the hold-out design
// keeps for every pre-faction dungeon (A7) — party and monsters mutually
// hostile, each faction allied with itself.
func sidesRun(t *testing.T) *encounter.Encounter {
	t.Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{},
		Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("floor", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: heroA, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: heroB, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
			{ID: monsterA, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 1}},
			{ID: monsterB, Kind: encounter.KindMonster, Position: spatial.Position{X: 7, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)

	return enc
}

// TestTheCastAnswersSidesFromTheRun pins that a side question is the run's
// to answer, and pins the run's default answer for a dungeon that declares
// no faction: the table this package used to keep, now read from the one
// place it lives (rpg-project#375, design §4 and A7).
//
// The stranger is the pin's edge: a sheet attached to this interaction but
// standing in no run is not a side. The cast has it, the run does not, and
// the run decides — known is false and the boolean beside it is unreadable,
// so both are asserted false rather than left unchecked.
func TestTheCastAnswersSidesFromTheRun(t *testing.T) {
	v := &castView{cast: sidesCast(), run: sidesRun(t)}

	cases := []struct {
		name        string
		a, b        string
		wantHostile bool
		wantAllied  bool
		wantKnown   bool
	}{
		{"two characters are allied, not hostile", heroA, heroB, false, true, true},
		{"a character is hostile to a monster, not allied", heroA, monsterA, true, false, true},
		{"a monster is hostile to a character, not allied", monsterA, heroA, true, false, true},
		{"two monsters are allied, not hostile", monsterA, monsterB, false, true, true},
		{"a participant is their own ally, not their own enemy", heroA, heroA, false, true, true},
		{"a stranger as a: cannot answer", strangeID, heroA, false, false, false},
		{"a stranger as b: cannot answer", heroA, strangeID, false, false, false},
		{"two strangers: cannot answer", strangeID, strangeID, false, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hostile, hostileKnown := v.IsHostile(tc.a, tc.b)
			allied, alliedKnown := v.IsAllied(tc.a, tc.b)

			require.Equal(t, tc.wantKnown, hostileKnown, "IsHostile known")
			require.Equal(t, tc.wantKnown, alliedKnown, "IsAllied known")
			require.Equal(t, tc.wantHostile, hostile, "IsHostile answer")
			require.Equal(t, tc.wantAllied, allied, "IsAllied answer")
		})
	}
}

// TestACastWithNoRunAnswersNoSides pins the entries that install no world —
// a join, a check, a rest, a death save, a participation refresh — where the
// door hands the cast a nil run. There is nothing to fold a side over, so
// the honest answer is unknown for everyone, the cast's own members included:
// no default table steps in for a run that is not there.
func TestACastWithNoRunAnswersNoSides(t *testing.T) {
	v := &castView{cast: sidesCast(), run: nil}

	for _, pair := range [][2]string{{heroA, heroB}, {heroA, monsterA}, {monsterA, monsterB}, {heroA, heroA}} {
		hostile, known := v.IsHostile(pair[0], pair[1])
		require.False(t, known, "IsHostile(%s, %s) known", pair[0], pair[1])
		require.False(t, hostile)

		allied, known := v.IsAllied(pair[0], pair[1])
		require.False(t, known, "IsAllied(%s, %s) known", pair[0], pair[1])
		require.False(t, allied)
	}
}
