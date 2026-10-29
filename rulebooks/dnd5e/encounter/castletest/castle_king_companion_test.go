// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_companion_test.go is THE CAPTAIN COMES WITH YOU
// (rpg-project#717, Frank's list S3), on a castle built in the test.
//
// THE CASTLE FILE CANNOT SAY IT YET, and this file is why. Its garrison and
// party pair already has its one `until` — `party-recognized`, which turns it
// hostile when the party is seen through — and one pair holds one `until`
// (lane rpg-project#722, D4: no list yet). So the companion castle is the
// castle with that one line changed to `until: { fact: guards-turned }, then:
// allied`, and the captain's march on the throne room swapped for the follow
// row `{ when: [{ fact: guards-turned }, { enemy: none }], toward: ally }`.
// Whether the castle itself takes that trade is the lane's call, and
// [TestMadKingTurnTheGarrison] still holds the castle as it is: with no
// follow row, a turned captain marches on the keep and never comes with you.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// companionEdits are the two changes the companion castle makes, each
// present exactly once in the castle file.
var companionEdits = []struct{ old, new string }{
	{
		old: "      - garrison\n      - party\n    stance: neutral\n    until:\n      fact: party-recognized\n",
		new: "      - garrison\n      - party\n    stance: neutral\n    until:\n      fact: guards-turned\n    then: allied\n",
	},
	{
		old: "      # swinging at him.\n      - when: [{ fact: guards-turned }, { enemy: none }]\n        toward: { at: { q: 10, r: -19 } }\n",
		new: "      # swinging at him.\n      - when: [{ fact: guards-turned }, { enemy: none }]\n        toward: ally\n",
	},
}

// companionCastle writes the companion castle beside the test and answers
// its path.
func companionCastle(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(kingFixture)
	require.NoError(t, err)
	source := string(raw)
	for _, e := range companionEdits {
		require.Equal(t, 1, strings.Count(source, e.old), "the castle carries the line once: %q", e.old)
		source = strings.Replace(source, e.old, e.new, 1)
	}
	path := filepath.Join(t.TempDir(), "companion-castle.yaml")
	require.NoError(t, os.WriteFile(path, []byte(source), 0o600))

	return path
}

// turnTheCompanionGarrison is [turnTheGarrison] on the companion castle:
// the cook's gossip and Ashford, out through the hall and the gate, and the
// captain told what his king did.
func turnTheCompanionGarrison(t *testing.T) *kingScene {
	t.Helper()
	s := openTheCastleAs(t, castleHost{
		at:      besideTheCook,
		doors:   map[encounter.DoorID]encounter.DoorState{kingThroneDoor: encounter.DoorIsOpen()},
		fixture: companionCastle(t),
	})
	s.said(kingCook, "gossip", "persuasion", true)
	s.said(kingCook, "ashford", "persuasion", true)
	s.outOfTheKitchen()
	s.throughTheGate(kingCaptain)
	s.said(kingCaptain, "turn", "persuasion", true)

	return s
}

// TestMadKingTheTurnedCaptainComesWithYou: after `turn`, the garrison is the
// party's side; the hero walks back into the courtyard and the captain walks
// after him on his follow row, on the party's side against the court. That
// he strikes loyalists and never the party is the three-sided fight
// [TestMadKingTurnTheGarrison] already holds on the castle itself, and
// [TestAllyWalkTheVeteranJoinsFollowsAndFights] holds for a follower.
func TestMadKingTheTurnedCaptainComesWithYou(t *testing.T) {
	t.Parallel()
	s := turnTheCompanionGarrison(t)

	t.Run("the garrison is the party's side", func(t *testing.T) {
		require.Equal(t, encounter.StanceAllied, s.stance(garrison, encounter.FactionParty))
		require.Equal(t, encounter.StanceHostile, s.stance(garrison, loyalists))
	})

	post := s.at(kingCaptain)
	s.walkTo(castleCell(0, -6))
	s.markTime(2, kingCaptain)

	t.Run("the hero walks into the courtyard and the captain follows", func(t *testing.T) {
		require.Positive(t, s.pickedTowardAlly(kingCaptain), "the record shows `toward ally` firing")
		require.Less(t, s.enc.Distance(s.at(kingCaptain), s.at(hero)), s.enc.Distance(post, s.at(hero)),
			"he walked from the gate toward the hero")
		require.LessOrEqual(t, s.enc.Distance(s.at(kingCaptain), s.at(hero)), 2.0, "and stands beside him")
	})

	t.Run("and he stands on the party's side against the court", func(t *testing.T) {
		require.False(t, s.hostile(kingCaptain, hero))
		require.True(t, s.hostile(kingCaptain, theKing))
	})
}
