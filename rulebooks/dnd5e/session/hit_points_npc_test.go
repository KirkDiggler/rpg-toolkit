// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"math"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// hit_points_npc_test.go plays "a creature acts on its own hit points"
// (rpg-project#1131, slice rpg-project#1246) on
// testdata/lieutenant-heals-when-hurt-walk.yaml: the Lieutenant of
// activate_npc_test.go, whose first row now says
// `when: { hit_points: { at_most: 30% } }, activate: second wind` and whose
// second row, `{ hit_points: { at_most: 5 } }, away: enemy`, backs him away at five
// hit points (rpg-project#1250). The module
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

// ---- the number (rpg-project#1250): at 5 hit points he stops swinging and backs away.

// backAwayNumber is the hit-point count the site's second row writes.
const backAwayNumber = 5

// where is the cell the creature stands on now, read off the run.
func (s *usesScene) where(id string) spatial.Position {
	s.t.Helper()
	enc, err := s.mgr.LoadWorldForTest(context.Background(), "sess")
	require.NoError(s.t, err)
	return memberOf(s.t, enc, encounter.MemberID(id)).Position
}

// cells is the hex distance between two axial cells (X is q, Y is r).
func cells(a, b spatial.Position) int {
	dq, dr := a.X-b.X, a.Y-b.Y
	return int((math.Abs(dq) + math.Abs(dr) + math.Abs(dq+dr)) / 2)
}

// inReach plays one whole round with him unhurt, so he walks to her and swings
// and ends the round next to her: the state the next round starts from.
func inReach(t *testing.T) *usesScene {
	t.Helper()
	s := newHurtScene(t)
	s.play(1)
	require.Equal(t, 1, cells(s.where("lieutenant"), s.where("alice")), "after one round he stands in her reach")
	return s
}

// spend writes the use he has used: nothing is left to pay for it.
func (s *usesScene) spend() {
	s.t.Helper()
	s.sheet("lieutenant").Left["second-wind"] = 0
}

// opportunityAsked is whether the fighter has an open reaction window.
func (s *usesScene) opportunityAsked() bool {
	s.t.Helper()
	return s.reactRow().ID != ""
}

func (s *usesScene) reactRow() session.Declaration {
	s.t.Helper()
	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(s.t, err)
	for _, row := range out.Declarations {
		if row.Verb == session.VerbReact {
			return row
		}
	}
	return session.Declaration{}
}

// declineTheOpportunity answers her window with no attack, and keeps what the
// answer told her as the rest of the round she just played.
func (s *usesScene) declineTheOpportunity() {
	s.t.Helper()
	row := s.reactRow()
	require.NotEmpty(s.t, row.ID, "she has a window to answer")
	mark := lastSeq(storyBeats(s.t, s.mgr, "sess", "alice"))
	_, err := s.mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: row.ID, Answer: session.Decline(),
	})
	require.NoError(s.t, err)
	last := len(s.rounds) - 1
	s.rounds[last] = append(s.rounds[last], storyBeatsAfter(s.t, s.mgr, "sess", "alice", mark)...)
}

// Test 2. At the number, with his use spent and her in his reach, he does not
// swing and he ends the turn farther from her than he began it.
func TestALieutenantAtTheNumberBacksAwayAndDoesNotSwing(t *testing.T) {
	s := inReach(t)
	s.spend()
	s.sheet("lieutenant").HitPoints = backAwayNumber
	before := cells(s.where("lieutenant"), s.where("alice"))

	s.play(1)
	require.True(t, s.opportunityAsked(), "he steps out of her reach: she is asked whether to take an opportunity attack")
	s.declineTheOpportunity()

	turn := s.rounds[1]
	require.Empty(t, by(turn, "lieutenant", "struck", "missed"), "he does not swing")
	require.Empty(t, by(turn, "lieutenant", encounter.BeatActivated), "his use is spent")
	require.NotEmpty(t, doneBy(turn, "lieutenant", encounter.BeatMoved), "he moves")
	after := cells(s.where("lieutenant"), s.where("alice"))
	require.Greater(t, after, before, "he ends the turn farther from her")
	t.Logf("backs away: %d cells from her before, %d after; beats of the turn: %v", before, after, beatNames(turn))
}

// Test 3. One above the number he fights, and does not move away.
func TestALieutenantOneAboveTheNumberFights(t *testing.T) {
	s := inReach(t)
	s.spend()
	s.sheet("lieutenant").HitPoints = backAwayNumber + 1
	before := cells(s.where("lieutenant"), s.where("alice"))

	s.play(1)

	turn := s.rounds[1]
	require.NotEmpty(t, by(turn, "lieutenant", "struck", "missed"), "he swings")
	require.Equal(t, before, cells(s.where("lieutenant"), s.where("alice")), "he does not move away")
	require.Empty(t, doneBy(turn, "lieutenant", encounter.BeatMoved))
	t.Logf("fights at %d: beats of the turn: %v", backAwayNumber+1, beatNames(turn))
}

// Test 4. Both at once: his use is not spent and he is at the number, which is
// also under the share. He heals first (the first row), and the heal puts him
// above the number whatever it rolls (1d10 plus 1 is at least 2, and 5 plus 2 is
// 7), so the second row is out and he fights: one activated beat, then a swing.
func TestALieutenantAtTheNumberWithHisUseLeftHealsAndFights(t *testing.T) {
	s := inReach(t)
	s.sheet("lieutenant").HitPoints = backAwayNumber
	before := cells(s.where("lieutenant"), s.where("alice"))

	s.play(1)

	turn := s.rounds[1]
	require.Len(t, by(turn, "lieutenant", encounter.BeatActivated), 1, "exactly one activated beat, his")
	result := resultOf(t, by(turn, "lieutenant", encounter.BeatActivationResult)[0])
	require.Equal(t, backAwayNumber, *result.Before)
	require.Equal(t, backAwayNumber+11, *result.After, "the scripted die rolls 10, plus 1")
	require.NotEmpty(t, by(turn, "lieutenant", "struck", "missed"), "then he swings")
	require.Empty(t, doneBy(turn, "lieutenant", encounter.BeatMoved), "he does not back away")
	require.Equal(t, before, cells(s.where("lieutenant"), s.where("alice")))
	names := beatNames(turn)
	activated := slices.Index(names, encounter.BeatActivated)
	struck := slices.IndexFunc(names, func(n string) bool { return n == "struck" || n == "missed" })
	require.Less(t, activated, struck, "he heals before he swings")
	t.Logf("heals then fights: beats of the turn: %v", names)
}

// Test 5. The site check says each row, and the `not` rows as the read-back says a `not`.
func TestTheReadBackSaysTheShareAndTheNumber(t *testing.T) {
	compiled := compileSite(t, hurtSite(t))
	checked, err := session.CheckContent(&session.CheckContentInput{Dungeon: &compiled})
	require.NoError(t, err)
	require.Empty(t, checked.Refusals)
	require.Empty(t, checked.Items)
	require.Empty(t, checked.Templates)
	require.Empty(t, checked.Activations)

	rb, err := readback.Of(hurtSite(t))
	require.NoError(t, err)
	require.Empty(t, rb.NotYetSaid)
	var said []string
	for _, st := range rb.Statements() {
		if strings.Contains(st.Sentence, "hit points") {
			said = append(said, st.Sentence)
		}
	}
	require.Len(t, said, 4, "one sentence for each of his four rows: %v", said)
	for _, sentence := range said {
		t.Logf("read-back: %s", sentence)
	}
	require.Contains(t, said[0], "hit points are at most 30% of its maximum")
	require.Contains(t, said[1], "hit points are at most 5")
	require.NotContains(t, said[1], "of its maximum")
	require.Contains(t, said[2], "NOT (")
	require.Contains(t, said[3], "NOT (")
	require.Contains(t, said[2], "hit points are at most 5")
	require.NotContains(t, said[2], "of its maximum")
}

// newHurtSceneOn is newHurtScene played on the dice given: newUsesScene's own
// launch, which hard-wires secondWindDice.
func newHurtSceneOn(t *testing.T, dice *faceDice) *usesScene {
	t.Helper()
	compiled := compileSite(t, hurtSite(t))
	fighter := armedFighter("alice")
	setLevel(fighter, 1)
	fighter.HitPoints, fighter.MaxHitPoints = 60, 60
	acts := &actCapture{}
	l := &lieutenantScene{t: t, sessions: newFakeSessions(), enc: newFakeEncounters()}
	var err error
	l.mgr, err = session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: acts, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: dice,
		TurnDriver: session.Driver(), Sessions: l.sessions, Encounters: l.enc, Characters: newFakeCharacters(fighter),
	})
	require.NoError(t, err)
	_, err = l.mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice"}})
	require.NoError(t, err)
	return &usesScene{lieutenantScene: l, acts: acts}
}

// Test 2b. The `not` on the swing row is what keeps him from swinging. The
// table's pick is a die over the sum of its candidates' shares, and the dice
// of these tests say 10, which lands on the FIRST candidate: with the `not`
// dropped the away row is first and wins, and nothing fails. Here the die says
// its top face (faceDice, clamped to the die by onTheDie), the LAST candidate's:
// as authored there is one candidate, the away row, so he still backs away;
// without the `not` the last is the swing and he swings.
func TestALieutenantAtTheNumberBacksAwayEvenWhenThePickLandsOnTheLastCandidate(t *testing.T) {
	dice := &faceDice{face: 10}
	s := newHurtSceneOn(t, dice)
	s.play(1)
	require.Equal(t, 1, cells(s.where("lieutenant"), s.where("alice")), "after one round he stands in her reach")
	s.spend()
	s.sheet("lieutenant").HitPoints = backAwayNumber
	before := cells(s.where("lieutenant"), s.where("alice"))
	dice.face = 1000 // every die says its top face: the last candidate

	s.play(1)
	require.True(t, s.opportunityAsked(), "he steps out of her reach")
	s.declineTheOpportunity()

	turn := s.rounds[1]
	require.Empty(t, by(turn, "lieutenant", "struck", "missed"), "he does not swing")
	require.Greater(t, cells(s.where("lieutenant"), s.where("alice")), before, "he ends the turn farther from her")
	t.Logf("last candidate: beats of the turn: %v", beatNames(turn))
}
