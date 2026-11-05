// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_companion_test.go is THE GARRISON'S TWO TURNS (rpg-project#717,
// ruled 2026-10-29): the garrison and the party are civil, turn hostile on
// recognition, and turn allied on the captain's word, and the captain's word
// outranks recognition whichever came first — the castle's `until` list,
// `guards-turned` before `party-recognized`.
//
// Recognition alone is [TestMadKingTroupeBlown] and
// [TestMadKingTheOrdersTurnTheGarrison]; the turn alone, the follow and the
// fight are [TestMadKingTurnTheGarrison]. This file is the two orders the
// list exists for, and the castle authored without the captain's follow row.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// toTheCaptainWithTheTruth is the hero out of the kitchen with Ashford in
// hand, through the gate and in front of the captain, on the castle at path
// ("" for the castle file itself).
func toTheCaptainWithTheTruth(t *testing.T, path string) *kingScene {
	t.Helper()
	s := openTheCastleAs(t, castleHost{
		at:      besideTheCook,
		doors:   map[encounter.DoorID]encounter.DoorState{kingThroneDoor: encounter.DoorIsOpen()},
		fixture: path,
	})
	s.said(kingCook, "gossip", "persuasion", true)
	s.said(kingCook, "ashford", "persuasion", true)
	s.outOfTheKitchen()
	s.throughTheGate(kingCaptain)

	return s
}

// TestMadKingTheCaptainsWordOutranksRecognition: recognized and then turned,
// the garrison is the party's side; turned and then recognized, it stays the
// party's side.
func TestMadKingTheCaptainsWordOutranksRecognition(t *testing.T) {
	t.Parallel()

	t.Run("recognized, then turned: allied", func(t *testing.T) {
		t.Parallel()
		s := toTheCaptainWithTheTruth(t, "")
		s.said(kingCaptain, "stand-aside", "intimidation", false)
		require.True(t, s.learners("party-recognized")[kingCaptain], "the captain saw through the hero")
		require.Equal(t, encounter.StanceHostile, s.stance(garrison, encounter.FactionParty), "recognition turned him")

		s.said(kingCaptain, "turn", "persuasion", true)

		require.True(t, s.learners("guards-turned")[kingCaptain])
		require.Equal(t, encounter.StanceAllied, s.stance(garrison, encounter.FactionParty),
			"the captain's word outranks the recognition that came first")
		require.False(t, s.hostile(kingCaptain, hero))
	})

	t.Run("turned, then recognized: still allied", func(t *testing.T) {
		t.Parallel()
		s := toTheCaptainWithTheTruth(t, "")
		s.said(kingCaptain, "turn", "persuasion", true)
		require.Equal(t, encounter.StanceAllied, s.stance(garrison, encounter.FactionParty))

		s.said(kingCaptain, "stand-aside", "intimidation", false)

		require.True(t, s.learners("party-recognized")[kingCaptain], "the captain learned it all the same")
		require.Equal(t, encounter.StanceAllied, s.stance(garrison, encounter.FactionParty),
			"recognition after the turn changes nothing")
	})
}

// marchingCaptain is the castle with the captain's follow row swapped back
// for the march on the throne room every other turned guard keeps: the
// castle authored without a follow row.
func marchingCaptain(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(kingFixture)
	require.NoError(t, err)
	follow := "      - when: [{ fact: guards-turned }, { enemy: none }]\n        toward: ally\n"
	march := "      - when: [{ fact: guards-turned }, { enemy: none }]\n        toward: { at: { q: 10, r: -19 } }\n"
	source := string(raw)
	require.Equal(t, 1, strings.Count(source, follow), "the castle carries the captain's follow row once")
	path := filepath.Join(t.TempDir(), "marching-captain.yaml")
	require.NoError(t, os.WriteFile(path, []byte(strings.Replace(source, follow, march, 1)), 0o600))

	return path
}

// TestMadKingACaptainWithNoFollowRowStaysBehind: turned on a castle authored
// with no follow row, the captain is still the party's side, but he never
// follows: he marches on the keep while the hero walks the other way.
func TestMadKingACaptainWithNoFollowRowStaysBehind(t *testing.T) {
	t.Parallel()
	s := toTheCaptainWithTheTruth(t, marchingCaptain(t))
	s.said(kingCaptain, "turn", "persuasion", true)
	require.Equal(t, encounter.StanceAllied, s.stance(garrison, encounter.FactionParty))
	post := s.at(kingCaptain)

	s.walkTo(partyStartCell(t))
	s.markTime(2, kingCaptain)

	require.Zero(t, s.pickedTowardAlly(kingCaptain), "no follow row, no following")
	require.Less(t, s.enc.Distance(s.at(kingCaptain), throneCell), s.enc.Distance(post, throneCell),
		"he marched toward the throne room")
	require.Greater(t, s.enc.Distance(s.at(kingCaptain), s.at(hero)), 2.0, "and left the hero behind")
}

// partyStartCell is the file's party start as an absolute cell, the frame
// [encounter.Encounter.Route] reads.
func partyStartCell(t *testing.T) spatial.Position {
	t.Helper()
	at := partyStart(t)

	return castleCell(int(at.X), int(at.Y))
}
