// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// hit_points_npc_test.go plays "a creature acts on its own hit points"
// (rpg-project#1131, slice rpg-project#1246) on
// testdata/lieutenant-heals-when-hurt-walk.yaml: the Lieutenant of
// activate_npc_test.go, whose first row now says
// `when: { hit_points: { at_most: 30 } }, activate: second wind`. The module
// adds no rule and no check: it tells the encounter two numbers off his stored
// sheet, and the encounter's table reads them.
//
// The dice, the fighter and the helpers are activate_npc_test.go's: the fighter
// only ends her turn, so one EndTurn of hers is one whole turn of his, and the
// heal rolls 10 and adds 1. He is wounded by writing his stored hit points
// before his turn, as TestAWoundedLieutenantHealsWhatTheRollSays does.

const hurtSitePath = "testdata/lieutenant-heals-when-hurt-walk.yaml"

// healShare is the percent the site's row writes.
const healShare = 30

func hurtSite(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(hurtSitePath)
	require.NoError(t, err)
	return raw
}

func newHurtScene(t *testing.T) *usesScene {
	t.Helper()
	return newUsesScene(t, compileSite(t, hurtSite(t)))
}

// shareOf is the largest hit-point count at or below healShare percent of max,
// compared exactly: hit points * 100 <= share * max.
func shareOf(max int) int {
	return healShare * max / 100
}

// his is the stored hit points and use count of the Lieutenant now.
func (s *usesScene) his() (hp, left int) {
	sheet := s.sheet("lieutenant")
	return sheet.HitPoints, sheet.Left["second-wind"]
}

// doneBy is the beats of those kinds whose actor, or whose member (a step names
// its mover so), is who.
func doneBy(beats []witnessed, who string, names ...string) []witnessed {
	var out []witnessed
	for _, b := range beatsNamed(beats, names...) {
		if b.Fields["actor"] == who || b.Fields["member"] == who {
			out = append(out, b)
		}
	}
	return out
}

// Test 1. Unhurt, he does not use it.
func TestAnUnhurtLieutenantDoesNotUseSecondWind(t *testing.T) {
	s := newHurtScene(t)
	max := s.sheet("lieutenant").MaxHitPoints
	require.Equal(t, max, s.sheet("lieutenant").HitPoints)

	s.play(3)

	require.Empty(t, by(s.all(), "lieutenant", encounter.BeatActivated), "at full hit points the row is not true")
	require.NotEmpty(t, doneBy(s.all(), "lieutenant", encounter.BeatMoved), "he walks to her")
	require.NotEmpty(t, by(s.all(), "lieutenant", "struck", "missed"), "and swings")
	_, left := s.his()
	require.Equal(t, 1, left, "the use is still his")
}

// Test 2. Hurt to the share, he uses it on his next turn; one more, he does not.
func TestALieutenantHurtToTheShareUsesSecondWindOnHisNextTurn(t *testing.T) {
	s := newHurtScene(t)
	max := s.sheet("lieutenant").MaxHitPoints
	share := shareOf(max)
	require.Equal(t, 27, max, "6d8 is 27 hit points")
	require.Equal(t, 8, share, "8 of 27 is 29.6%; 9 is 33.3%")
	s.sheet("lieutenant").HitPoints = share

	s.play(1)

	require.Len(t, by(s.rounds[0], "lieutenant", encounter.BeatActivated), 1, "one activated beat, his")
	results := by(s.rounds[0], "lieutenant", encounter.BeatActivationResult)
	require.Len(t, results, 1)
	result := resultOf(t, results[0])
	require.Equal(t, encounter.ResultHealingApplied, result.Kind)
	require.Equal(t, share, *result.Before)
	require.Equal(t, share+11, *result.After, "1d10 rolls 10, plus 1")
	hp, left := s.his()
	require.Equal(t, share+11, hp, "his stored hit points are up by the roll plus 1")
	require.Zero(t, left)
	t.Logf("beats of the turn he heals in: %v", beatNames(s.rounds[0]))

	t.Run("one more hit point and he does not", func(t *testing.T) {
		s := newHurtScene(t)
		s.sheet("lieutenant").HitPoints = share + 1
		s.play(1)
		require.Empty(t, by(s.rounds[0], "lieutenant", encounter.BeatActivated))
		_, left := s.his()
		require.Equal(t, 1, left)
	})
}

// Test 3. Hurt and healed in one verb: the ask after the heal reads the healed
// sheet. Within the turn there is one activation (a memo that kept his old hit
// points would offer it again and the Len below would be 2); and on his next turn
// he is above the share and has no use, so there is none.
func TestAHealedLieutenantIsNotAskedToActivateAgain(t *testing.T) {
	s := newHurtScene(t)
	share := shareOf(s.sheet("lieutenant").MaxHitPoints)
	s.sheet("lieutenant").HitPoints = share

	s.play(2)

	require.Len(t, by(s.rounds[0], "lieutenant", encounter.BeatActivated), 1, "within the turn he heals: once")
	require.Empty(t, by(s.rounds[1], "lieutenant", encounter.BeatActivated), "his next turn: healed and spent")
	hp, _ := s.his()
	require.Greater(t, hp, share, "he is above the share")
}

// Test 4. Once: hurt again, the use is spent and the row is absent.
func TestAHurtLieutenantWithNoUseLeftDoesNotActivate(t *testing.T) {
	s := newHurtScene(t)
	share := shareOf(s.sheet("lieutenant").MaxHitPoints)
	s.sheet("lieutenant").HitPoints = share
	s.play(1)
	require.Len(t, by(s.rounds[0], "lieutenant", encounter.BeatActivated), 1)

	s.sheet("lieutenant").HitPoints = share
	s.play(1)

	require.Empty(t, by(s.rounds[1], "lieutenant", encounter.BeatActivated), "at the share again, with nothing left to pay")
	require.Equal(t, 0, s.sheet("lieutenant").Left["second-wind"])
	require.Equal(t, 1, s.sheet("lieutenant").SlotsLeft(coreCombat.ActionBonus), "his bonus action is back, so the use is what is missing")
}

// Test 5. He still fights on the turn he heals.
func TestTheLieutenantHealsAndStillFights(t *testing.T) {
	s := newHurtScene(t)
	s.sheet("lieutenant").HitPoints = shareOf(s.sheet("lieutenant").MaxHitPoints)

	s.play(1)

	require.Len(t, by(s.rounds[0], "lieutenant", encounter.BeatActivated), 1)
	require.NotEmpty(t, doneBy(s.rounds[0], "lieutenant", "struck", "missed", encounter.BeatMoved),
		"he steps toward her or swings in the same turn")
}

// Test 6. The content check and the site check accept the site, and the
// read-back says the row's condition.
func TestTheContentCheckAcceptsTheHurtSite(t *testing.T) {
	compiled := compileSite(t, hurtSite(t))
	checked, err := session.CheckContent(&session.CheckContentInput{Dungeon: &compiled})
	require.NoError(t, err)
	require.Empty(t, checked.Refusals)
	require.Empty(t, checked.Items)
	require.Empty(t, checked.Templates)
	require.Empty(t, checked.Activations)

	rb, err := readback.Of(hurtSite(t))
	require.NoError(t, err)
	require.Empty(t, rb.NotYetSaid, "every word the site writes is one the read-back can say")
	var said []string
	for _, st := range rb.Statements() {
		if strings.Contains(st.Sentence, "hit points") && strings.Contains(st.Sentence, "30") {
			said = append(said, st.Sentence)
		}
	}
	require.NotEmpty(t, said, "the read-back says the row's hit-points condition")
	t.Logf("read-back: %s", said[0])
}
