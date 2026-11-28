// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// action_surge_npc_test.go plays "a creature uses Action Surge" (rpg-project#1131,
// slice rpg-project#1261) on testdata/lieutenant-surges-walk.yaml: the Lieutenant
// of activate_npc_test.go, whose stat block lets him use Action Surge once and
// whose table's first row says `when: { enemy: reach }, activate: action surge`.
// The module adds nothing: the pins carry the ability, its price and its door.
//
// THE DICE. secondWindDice is every die min(size, 10): the table's pick (a die
// over the sum of its candidates' shares) lands on the FIRST candidate. faceDice
// with a top face lands on the LAST (hit_points_npc_test.go's last test does the
// same). The fighter is a level 1 fighter with 60 hit points who only ends her
// turn, so one EndTurn of hers is one whole turn of his.

const surgeSitePath = "testdata/lieutenant-surges-walk.yaml"

func surgeSite(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(surgeSitePath)
	require.NoError(t, err)
	return raw
}

// surgeVariant is the site with one piece of text replaced, once.
func surgeVariant(t *testing.T, from, to string) []byte {
	t.Helper()
	raw := surgeSite(t)
	require.Equal(t, 1, bytes.Count(raw, []byte(from)), "precondition: %q occurs once in the site", from)
	return bytes.Replace(raw, []byte(from), []byte(to), 1)
}

// swings is the blows of his turns: struck and missed beats whose actor is him.
func swings(beats []witnessed) []witnessed { return by(beats, "lieutenant", "struck", "missed") }

// newSurgeSceneOn is the site launched on the dice given, the fighter first.
func newSurgeSceneOn(t *testing.T, raw []byte, dice session.Roller) *usesScene {
	t.Helper()
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
	compiled := compileSite(t, raw)
	_, err = l.mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice"}})
	require.NoError(t, err)
	turn, err := l.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, "alice", turn.Active, "the fighter is first")
	return &usesScene{lieutenantScene: l, acts: acts}
}

// Test 1. He surges and swings twice. The site puts him 3 cells from her, so
// the turn is: he walks into her reach, the row's `when` becomes true, he
// surges, and he swings twice.
func TestTheLieutenantSurgesAndSwingsTwice(t *testing.T) {
	s := newUsesScene(t, compileSite(t, surgeSite(t)))
	require.Equal(t, 1, s.sheet("lieutenant").Left["action-surge"], "he starts with his one use")

	s.play(1)
	turn := s.rounds[0]
	t.Logf("beats of the turn he surges in: %v", beatNames(turn))

	activated := by(turn, "lieutenant", encounter.BeatActivated)
	require.Len(t, activated, 1, "exactly one activated beat, his")
	var told struct {
		Ability session.SpellRef `json:"ability"`
		Target  string           `json:"target"`
	}
	activated[0].decode(t, &told)
	require.Equal(t, "Action Surge", told.Ability.Name)
	require.Empty(t, told.Target, "it lands on its user")

	results := by(turn, "lieutenant", encounter.BeatActivationResult)
	require.Len(t, results, 1)
	result := resultOf(t, results[0])
	require.Equal(t, encounter.ActivationResultKind("capacity-granted"), result.Kind)
	require.Equal(t, "1 action", result.Description)
	t.Logf("result as recorded: %+v", result)

	require.Len(t, swings(turn), 2, "two actions, two swings")
	require.Equal(t, 0, s.sheet("lieutenant").Left["action-surge"], "his stored sheet: the use is spent")
}

// Test 2. Once. His next turn in her reach has no surge and one swing.
func TestTheLieutenantSurgesOnceAndSwingsOnceAfter(t *testing.T) {
	s := newUsesScene(t, compileSite(t, surgeSite(t)))
	s.play(2)

	require.Empty(t, by(s.rounds[1], "lieutenant", encounter.BeatActivated), "no second surge")
	require.Len(t, swings(s.rounds[1]), 1, "one action, one swing")
	require.Len(t, by(s.all(), "lieutenant", encounter.BeatActivated), 1, "one in all")
	require.Equal(t, 0, s.sheet("lieutenant").Left["action-surge"], "nothing gave the use back")
	t.Logf("his next turn: %v", beatNames(s.rounds[1]))
}

// Test 3. Not before he has an enemy in reach. At 8 cells she is farther than
// a turn's walk (he needs 7 steps to stand next to her and has 6), so the walk
// row is false as well as the surge row: the observed turn is neither a surge
// nor a step nor a swing. At 7 cells he walks all six steps and surges on
// arrival in the same turn (checked by the second case).
func TestTheLieutenantDoesNotSurgeBeforeHeHasAnEnemyInReach(t *testing.T) {
	far := surgeVariant(t, "startingCell: { location: { q: -5, r: 4 }", "startingCell: { location: { q: 0, r: 4 }")
	s := newSurgeSceneOn(t, far, &secondWindDice{})
	s.play(1)
	turn := s.rounds[0]
	t.Logf("a turn with her beyond a turn's walk: %v", beatNames(turn))

	require.Greater(t, cells(s.where("lieutenant"), s.where("alice")), 1, "she is not in his reach at the end of it")
	require.Empty(t, by(turn, "lieutenant", encounter.BeatActivated), "the row's `when` is false")
	require.Empty(t, swings(turn))
	require.Equal(t, 1, s.sheet("lieutenant").Left["action-surge"], "his use is still his")

	t.Run("one cell nearer he walks the whole way and surges on arrival", func(t *testing.T) {
		near := surgeVariant(t, "startingCell: { location: { q: -5, r: 4 }", "startingCell: { location: { q: -1, r: 4 }")
		s := newSurgeSceneOn(t, near, &secondWindDice{})
		s.play(1)
		names := beatNames(s.rounds[0])
		t.Logf("seven cells: %v", names)
		require.Len(t, by(s.rounds[0], "lieutenant", encounter.BeatActivated), 1)
		require.Len(t, doneBy(s.rounds[0], "lieutenant", encounter.BeatMoved), 6, "six steps first")
		require.Len(t, swings(s.rounds[0]), 2)
	})
}

// Test 4. Whichever way the roll lands. "Enemy in reach" holds only while he
// can still attack, so the surge row is off once he has swung. The die says its
// top face, which lands on the LAST candidate: in reach the candidates are the
// surge row (9 shares) and the swing row (1), so the last is the swing. He swings
// once and does not surge that turn. On his next turn with the pick on the
// FIRST candidate he surges and swings twice.
func TestTheLieutenantWhosePickLandsOnTheSwingSwingsOnceAndSurgesALaterTurn(t *testing.T) {
	dice := &faceDice{face: 10}
	s := newSurgeSceneOn(t, surgeSite(t), dice)
	dice.face = 1000
	s.play(1)
	t.Logf("pick on the last candidate: %v", beatNames(s.rounds[0]))

	require.Empty(t, by(s.rounds[0], "lieutenant", encounter.BeatActivated), "no surge: he swung first and the row is off")
	require.Len(t, swings(s.rounds[0]), 1, "one action, one swing")
	require.Equal(t, 1, s.sheet("lieutenant").Left["action-surge"], "his use is still his")

	dice.face = 10
	s.play(1)
	t.Logf("next turn, pick on the first candidate: %v", beatNames(s.rounds[1]))
	require.Len(t, by(s.rounds[1], "lieutenant", encounter.BeatActivated), 1, "he surges")
	require.Len(t, swings(s.rounds[1]), 2, "and swings twice")
	require.Equal(t, 0, s.sheet("lieutenant").Left["action-surge"])
}

// Test 4b. The weight is in the pick's record. On the turn he has her in reach
// and his use left, the `answered` beat that chose `activate` lists two
// candidates: the surge row with weight 9 and the swing row with weight 1, as
// `loaded` 900 and 100 of `of` 1000 (the pick is a die over the loaded sum), so
// nine in ten. The die said 10, inside the first candidate's 900.
func TestThePicksRecordShowsTheSurgeRowWithNineSharesInTen(t *testing.T) {
	s := newUsesScene(t, compileSite(t, surgeSite(t)))
	s.play(1)

	var picks []witnessed
	for _, b := range beatsNamed(s.rounds[0], encounter.BeatAnswered) {
		if b.Fields["creature"] == "lieutenant" && b.Fields["word"] == "activate" {
			picks = append(picks, b)
		}
	}
	require.Len(t, picks, 1, "one pick chose the surge")
	var answered struct {
		Of         int `json:"of"`
		Roll       int `json:"roll"`
		Entry      int `json:"entry"`
		Candidates []struct {
			Entry  int `json:"entry"`
			Weight int `json:"weight"`
			Loaded int `json:"loaded"`
		} `json:"candidates"`
	}
	picks[0].decode(t, &answered)
	t.Logf("the pick's record: %+v", answered)
	require.Equal(t, 1000, answered.Of)
	require.Len(t, answered.Candidates, 2)
	require.Equal(t, 0, answered.Candidates[0].Entry, "the surge row")
	require.Equal(t, 9, answered.Candidates[0].Weight)
	require.Equal(t, 900, answered.Candidates[0].Loaded, "900 of 1000")
	require.Equal(t, 1, answered.Candidates[1].Weight, "the swing row")
	require.Equal(t, 100, answered.Candidates[1].Loaded)
	require.LessOrEqual(t, answered.Roll, answered.Candidates[0].Loaded, "the die fell inside the surge's share")
}

// Test 5. The player is told his surge as a player's own activation is told.
func TestThePlayerIsToldTheLieutenantsSurgeAsAPlayersIs(t *testing.T) {
	s := newUsesScene(t, compileSite(t, surgeSite(t)))
	s.play(1)

	var told []session.ActivateAct
	for _, act := range s.acts.of("alice") {
		if body, ok := act.Body.(session.ActivateAct); ok {
			told = append(told, body)
			require.Equal(t, "activate", act.Verb)
		}
	}
	require.Len(t, told, 1, "her client was handed his activation live")
	require.Equal(t, session.Party{Kind: session.PartyMember, ID: "lieutenant"}, told[0].By)
	require.Equal(t, "Action Surge", told[0].Ability.Name)
	require.Equal(t, session.Party{}, told[0].Target)
	require.Len(t, told[0].Results, 1)
	require.Equal(t, "capacity-granted", told[0].Results[0].Kind)
	t.Logf("sent to her: %+v", told[0])
}

// Test 7. The content check and the launch.
func TestTheContentCheckAndTheLaunchJudgeActionSurge(t *testing.T) {
	check := func(raw []byte) *session.CheckContentOutput {
		compiled := compileSite(t, raw)
		out, err := session.CheckContent(&session.CheckContentInput{Dungeon: &compiled})
		require.NoError(t, err)
		return out
	}

	t.Run("the site as written", func(t *testing.T) {
		out := check(surgeSite(t))
		require.Empty(t, out.Refusals)
		require.Empty(t, out.Items)
		require.Empty(t, out.Templates)
		require.Empty(t, out.Activations)
		rb, err := readback.Of(surgeSite(t))
		require.NoError(t, err)
		require.Empty(t, rb.NotYetSaid)
		var said []string
		for _, st := range rb.Statements() {
			if strings.Contains(st.Sentence, "action surge") {
				said = append(said, st.Sentence)
			}
		}
		require.NotEmpty(t, said)
		for _, sentence := range said {
			t.Logf("read-back: %s", sentence)
		}
	})
	t.Run("a misspelt word", func(t *testing.T) {
		out := check(surgeVariant(t, "activate: action surge, weight: 9 }", "activate: actoin surge, weight: 9 }"))
		require.Equal(t, []session.ActivationRefusal{{Word: "actoin surge", Message: refusalMessage("actoin surge", t)}}, out.Activations)
		require.Contains(t, out.Activations[0].Message, "one of action surge, second wind")
		t.Logf("refusal: %s", out.Activations[0].Message)
	})
	t.Run("a stat block with no count is refused by the rules, at the check and at the launch", func(t *testing.T) {
		uncounted := surgeVariant(t, "        uses: { max: 1, refills: { on: [short rest, long rest] } }\n", "")
		compiled := compileSite(t, uncounted) // the compiler does not judge it: the rules do
		const sentence = "stat block \"gate-lieutenant\": lets[0]: `use` is \"action surge\", which gives its user something for its count alone; write `uses` or a `cost`"

		out, err := session.CheckContent(&session.CheckContentInput{Dungeon: &compiled})
		require.NoError(t, err)
		require.Len(t, out.Templates, 1)
		require.Equal(t, "gate-lieutenant", out.Templates[0].TemplateID)
		require.Equal(t, sentence, out.Templates[0].Message)

		s, err := newLieutenantScene(t, compiled)
		require.Error(t, err)
		require.NotErrorIs(t, err, session.ErrUnknownContent, "it is the assembly that refuses, not the content check's own error")
		require.ErrorContains(t, err, sentence)
		require.Empty(t, s.sessions.byID, "no session was stored")
		t.Logf("launch: %v", err)
	})
}
