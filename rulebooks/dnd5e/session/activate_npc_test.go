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

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// activate_npc_test.go plays "a creature uses what its stat block lets it"
// (rpg-project#1131, slice rpg-project#1240) on
// testdata/lieutenant-uses-second-wind-walk.yaml: the Lieutenant of
// template_lets_test.go, whose table now says `activate: second wind` first.
//
// THE DICE are secondWindDice with strike off, as a_run_and_a_retreat_test.go has
// them: every die is min(size, 10), so the heal rolls 10 and adds 1. The fighter
// is a level 1 fighter with 60 of 60 hit points who only ends her turn. The
// initiative order (fighter, then the Lieutenant) is fixed by those dice, so one
// EndTurn of hers is one whole turn of his.
//
// The Lieutenant is wounded for Test 2 by writing his stored hit points before
// his first turn, the way worn_item and cure_wounds tests wound a patient: the
// fighter is three cells away and only ends her turn, so no blow is scripted.

const usesSitePath = "testdata/lieutenant-uses-second-wind-walk.yaml"

const misspeltRow = "{ activate: secnd wind }"

func usesSite(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(usesSitePath)
	require.NoError(t, err)
	return raw
}

// usesVariant is the site with one piece of text replaced, once.
func usesVariant(t *testing.T, from, to string) []byte {
	t.Helper()
	raw := usesSite(t)
	require.Equal(t, 1, bytes.Count(raw, []byte(from)), "precondition: %q occurs once in the site", from)
	return bytes.Replace(raw, []byte(from), []byte(to), 1)
}

const activateRow = "{ activate: second wind }"

// usesScene is a launched site played one fighter's turn at a time.
type usesScene struct {
	*lieutenantScene
	acts   *actCapture
	rounds [][]witnessed
}

func newUsesScene(t *testing.T, compiled dungeonspec.Compiled) *usesScene {
	t.Helper()
	fighter := armedFighter("alice")
	setLevel(fighter, 1)
	fighter.HitPoints, fighter.MaxHitPoints = 60, 60
	acts := &actCapture{}
	l := &lieutenantScene{t: t, sessions: newFakeSessions(), enc: newFakeEncounters()}
	var err error
	l.mgr, err = session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: acts, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: &secondWindDice{},
		TurnDriver: session.Driver(), Sessions: l.sessions, Encounters: l.enc, Characters: newFakeCharacters(fighter),
	})
	require.NoError(t, err)
	_, err = l.mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice"}})
	require.NoError(t, err)
	turn, err := l.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, "alice", turn.Active, "the fight formed at launch and the fighter is first")
	return &usesScene{lieutenantScene: l, acts: acts}
}

// play ends the fighter's turn n times, keeping what each told her.
func (s *usesScene) play(n int) {
	s.t.Helper()
	for i := 0; i < n; i++ {
		mark := lastSeq(storyBeats(s.t, s.mgr, "sess", "alice"))
		_, err := s.mgr.EndTurn(context.Background(), &session.EndTurnInput{
			Session: "sess", Member: "alice", DeclarationID: currentEndTurnID(s.t, s.mgr, "sess", "alice"),
		})
		require.NoError(s.t, err, "round %d: EndTurn returns no error", i+1)
		s.rounds = append(s.rounds, storyBeatsAfter(s.t, s.mgr, "sess", "alice", mark))
	}
}

// by is the beats of one kind whose actor is who.
func by(beats []witnessed, who string, names ...string) []witnessed {
	var out []witnessed
	for _, b := range beatsNamed(beats, names...) {
		if b.Fields["actor"] == who {
			out = append(out, b)
		}
	}
	return out
}

// all is every beat of every round played.
func (s *usesScene) all() []witnessed {
	var out []witnessed
	for _, r := range s.rounds {
		out = append(out, r...)
	}
	return out
}

// Test 1. He uses it on his first turn, and his own sheet pays.
func TestTheLieutenantUsesSecondWindOnHisFirstTurnAndHisSheetPays(t *testing.T) {
	s := newUsesScene(t, compileSite(t, usesSite(t)))
	before := s.sheet("lieutenant")
	require.Equal(t, 1, before.SlotsLeft(coreCombat.ActionBonus))
	max := before.MaxHitPoints
	require.Equal(t, max, before.HitPoints, "he starts at full hit points")

	s.play(1)

	activated := by(s.rounds[0], "lieutenant", encounter.BeatActivated)
	require.Len(t, activated, 1, "exactly one activated beat, his")
	var told struct {
		Ability session.SpellRef `json:"ability"`
		Target  string           `json:"target"`
	}
	activated[0].decode(t, &told)
	require.Equal(t, "Second Wind", told.Ability.Name)
	require.Contains(t, told.Ability.Ref, "second", told.Ability.Ref)
	require.Empty(t, told.Target, "it lands on its user, so it names no target")

	results := by(s.rounds[0], "lieutenant", encounter.BeatActivationResult)
	require.Len(t, results, 1)
	result := resultOf(t, results[0])
	require.Equal(t, encounter.ResultHealingApplied, result.Kind)
	require.Equal(t, "lieutenant", result.Target)
	// At full hit points the resolver reports amount 0 with the roll as requested.
	require.Equal(t, 0, *result.Amount)
	require.Equal(t, 11, *result.Requested, "1d10 at its maximum, plus 1")
	require.Equal(t, max, *result.Before)
	require.Equal(t, max, *result.After)
	t.Logf("beats of his first turn: %v", beatNames(s.rounds[0]))
	t.Logf("heal at full hit points: amount %d requested %d before %d after %d", *result.Amount, *result.Requested, *result.Before, *result.After)

	after := s.sheet("lieutenant")
	require.Equal(t, 0, after.Left["second-wind"], "the use is spent on his own sheet")
	require.Equal(t, 0, after.SlotsLeft(coreCombat.ActionBonus), "and so is the bonus action")
	require.Equal(t, max, after.HitPoints)
}

// Test 2. Hurt first, he heals.
func TestAWoundedLieutenantHealsWhatTheRollSays(t *testing.T) {
	s := newUsesScene(t, compileSite(t, usesSite(t)))
	max := s.sheet("lieutenant").MaxHitPoints
	wounded := max - 4
	require.Positive(t, wounded)
	s.sheet("lieutenant").HitPoints = wounded

	s.play(1)

	results := by(s.rounds[0], "lieutenant", encounter.BeatActivationResult)
	require.Len(t, results, 1)
	result := resultOf(t, results[0])
	require.Equal(t, encounter.ResultHealingApplied, result.Kind)
	// 10 + 1, capped at his maximum: only 4 can be healed.
	require.Equal(t, wounded, *result.Before)
	require.Equal(t, max, *result.After)
	require.Equal(t, 4, *result.Amount)
	require.Equal(t, 11, *result.Requested)
	require.Equal(t, max, s.sheet("lieutenant").HitPoints, "his stored hit points say the same")
	t.Logf("heal when hurt: before %d after %d amount %d requested %d (max %d)", *result.Before, *result.After, *result.Amount, *result.Requested, max)

	// And under the cap: wounded deeper than one roll.
	deep := newUsesScene(t, compileSite(t, usesSite(t)))
	deepMax := deep.sheet("lieutenant").MaxHitPoints
	if deepMax > 12 {
		deep.sheet("lieutenant").HitPoints = deepMax - 12
		deep.play(1)
		got := resultOf(t, by(deep.rounds[0], "lieutenant", encounter.BeatActivationResult)[0])
		require.Equal(t, deepMax-12+11, *got.After, "wounded plus the roll plus 1")
		require.Equal(t, deepMax-1, deep.sheet("lieutenant").HitPoints)
	}
}

// Test 3. Once, and never twice in a turn.
func TestTheLieutenantUsesItOnceAndNeverTwiceInATurn(t *testing.T) {
	s := newUsesScene(t, compileSite(t, usesSite(t)))
	s.play(3)

	require.Len(t, by(s.rounds[0], "lieutenant", encounter.BeatActivated), 1, "within his first turn: one, the second ask read the spent sheet")
	require.Len(t, by(s.all(), "lieutenant", encounter.BeatActivated), 1, "over three rounds: still one")
	require.Len(t, by(s.all(), "lieutenant", encounter.BeatActivationResult), 1)
	sheet := s.sheet("lieutenant")
	require.Equal(t, 0, sheet.Left["second-wind"], "nothing gave the use back")
	require.Equal(t, 1, sheet.SlotsLeft(coreCombat.ActionBonus), "after his next turn began, his bonus action is back")
}

// Test 4. He still fights.
func TestTheLieutenantUsesItAndStillFights(t *testing.T) {
	s := newUsesScene(t, compileSite(t, usesSite(t)))
	s.play(3)

	require.NotEmpty(t, by(s.all(), "lieutenant", "struck", "missed"), "he swung at her as his other rows say")
}

// Test 5. The player is told: what the fighter's own client is sent is the same
// shape as for a player's activation (activation_events_test.go).
func TestThePlayerIsToldTheLieutenantsActivationAsAPlayersIs(t *testing.T) {
	s := newUsesScene(t, compileSite(t, usesSite(t)))
	s.play(1)

	var told []session.ActivateAct
	for _, act := range s.acts.of("alice") {
		if body, ok := act.Body.(session.ActivateAct); ok {
			told = append(told, body)
			require.Equal(t, "activate", act.Verb, "the act id is an activation's own")
		}
	}
	require.Len(t, told, 1, "her client was handed his activation live")
	require.Equal(t, session.Party{Kind: session.PartyMember, ID: "lieutenant"}, told[0].By)
	require.Equal(t, "Second Wind", told[0].Ability.Name)
	require.Equal(t, session.Party{}, told[0].Target)
	require.Len(t, told[0].Results, 1)
	require.Equal(t, string(encounter.ResultHealingApplied), told[0].Results[0].Kind)
	require.Equal(t, session.Party{Kind: session.PartyMember, ID: "lieutenant"}, told[0].Results[0].Target)
	t.Logf("sent to her: %+v", told[0])
}

// twoTables is the site with a second behaviour definition, bound beside the
// first, whose table says the same row.
func twoTables(t *testing.T, row string) []byte {
	t.Helper()
	raw := string(usesVariant(t, activateRow, row))
	second := `  lieutenant-policy-two:
    recipe:
      ref: site:behaviors:lieutenant-policy-two
      initial: fighting
      states:
        fighting:
          transitions:
            - { on: time, do: [{ pick: time }] }
            - { on: turn, do: [{ pick: time }] }
    table:
      time:
        - ` + row + `
        - { when: { enemy: walk }, toward: enemy }

`
	require.Equal(t, 1, strings.Count(raw, "\nroom:\n  version: 3"))
	raw = strings.Replace(raw, "\nroom:\n  version: 3", "\n"+second+"room:\n  version: 3", 1)
	bound := "          - id: policy\n            use: lieutenant-policy\n"
	require.Equal(t, 1, strings.Count(raw, bound))
	return []byte(strings.Replace(raw, bound, bound+"          - id: policy-two\n            use: lieutenant-policy-two\n", 1))
}

// inABundle is the site whose row sits inside a bundle's group.
func inABundle(t *testing.T, row string) []byte {
	t.Helper()
	raw := string(usesVariant(t, activateRow, "{ use: rally }"))
	require.Equal(t, 1, strings.Count(raw, "\nroom:\n  version: 3"))
	return []byte(strings.Replace(raw, "\nroom:\n  version: 3",
		"\nbundles:\n  rally:\n    rows:\n      - "+row+"\nroom:\n  version: 3", 1))
}

func wordsOf(t *testing.T) string {
	t.Helper()
	return strings.Join(features.NamedAbilities(), ", ")
}

func refusalMessage(word string, t *testing.T) string {
	t.Helper()
	return "a table row says `activate: " + word + "`, and the rules describe no ability by that word; it is one of " + wordsOf(t)
}

// Test 6. The content check.
func TestTheContentCheckRefusesARowWhoseWordIsNoAbility(t *testing.T) {
	check := func(raw []byte) *session.CheckContentOutput {
		compiled := compileSite(t, raw)
		out, err := session.CheckContent(&session.CheckContentInput{Dungeon: &compiled})
		require.NoError(t, err)
		return out
	}

	t.Run("the site as written", func(t *testing.T) {
		require.Empty(t, check(usesSite(t)).Activations)
	})
	t.Run("a misspelt word", func(t *testing.T) {
		out := check(usesVariant(t, activateRow, misspeltRow))
		require.Equal(t, []session.ActivationRefusal{{Word: "secnd wind", Message: refusalMessage("secnd wind", t)}}, out.Activations)
		require.Empty(t, out.Refusals)
		require.Empty(t, out.Items)
		require.Empty(t, out.Templates)
		t.Logf("refusal: %s", out.Activations[0].Message)
	})
	t.Run("the same word on two rows of two tables is one refusal", func(t *testing.T) {
		out := check(twoTables(t, misspeltRow))
		require.Len(t, out.Activations, 1)
		require.Equal(t, "secnd wind", out.Activations[0].Word)
	})
	t.Run("inside a bundle's group", func(t *testing.T) {
		out := check(inABundle(t, misspeltRow))
		require.Len(t, out.Activations, 1)
		require.Equal(t, "secnd wind", out.Activations[0].Word)
	})
	t.Run("a bundle that says a real word is not refused", func(t *testing.T) {
		require.Empty(t, check(inABundle(t, activateRow)).Activations)
	})
	t.Run("two words are two refusals in word order", func(t *testing.T) {
		raw := strings.Replace(string(twoTables(t, "{ activate: zeal }")), "{ activate: zeal }", misspeltRow, 1)
		out := check([]byte(raw))
		require.Len(t, out.Activations, 2)
		require.Equal(t, "secnd wind", out.Activations[0].Word)
		require.Equal(t, "zeal", out.Activations[1].Word)
	})
}

// Test 7. A launch refuses the misspelt site, and launches nothing.
func TestALaunchRefusesARowWhoseWordIsNoAbilityAndLaunchesNothing(t *testing.T) {
	compiled := compileSite(t, usesVariant(t, activateRow, misspeltRow))
	s, err := newLieutenantScene(t, compiled)
	require.ErrorIs(t, err, session.ErrUnknownContent)
	require.ErrorContains(t, err, refusalMessage("secnd wind", t))
	require.Empty(t, s.sessions.byID, "no session was stored")
	require.Zero(t, s.enc.saves, "no encounter was stored")
	t.Logf("launch: %v", err)
}

// Test 8. A real ability its creature lacks is not refused, and never offered.
func TestARowForAnAbilityItsCreatureLacksIsNotRefusedAndNeverOffered(t *testing.T) {
	raw, err := os.ReadFile(lieutenantSitePath)
	require.NoError(t, err)
	older := string(raw)
	row := "        - { when: { enemy: walk }, toward: enemy }\n"
	require.Equal(t, 1, strings.Count(older, row))
	older = strings.Replace(older, row, "        - "+activateRow+"\n"+row, 1)
	lets := "    lets:\n      - use: second wind\n        with: { level: 1 }\n        uses: { max: 1, refills: { on: [short rest, long rest] } }\n"
	require.Equal(t, 1, strings.Count(older, lets))
	older = strings.Replace(older, lets, "", 1)

	compiled := compileSite(t, []byte(older))
	checked, err := session.CheckContent(&session.CheckContentInput{Dungeon: &compiled})
	require.NoError(t, err)
	require.Empty(t, checked.Activations)
	require.Empty(t, checked.Refusals)
	require.Empty(t, checked.Items)
	require.Empty(t, checked.Templates)

	s := newUsesScene(t, compiled)
	require.Nil(t, s.sheet("lieutenant").Kit)
	s.play(2)
	require.Empty(t, beatsNamed(s.all(), encounter.BeatActivated), "he has nothing to use")
	require.NotEmpty(t, by(s.all(), "lieutenant", "struck", "missed", encounter.BeatMoved), "he walks and swings")
}

// Test 9. A creature with no kit says nothing: its sheet lists nothing it can
// use, read the way the rules read it (monster.AbilitiesOf, which readFacts
// hands the encounter), and the Runner's table has no activate row to offer.
func TestACreatureWithNoKitStatesNoAbilities(t *testing.T) {
	compiled := compileSite(t, []byte(runWalkSite(t)))
	s, err := newLieutenantScene(t, compiled)
	require.NoError(t, err)
	got, err := monster.AbilitiesOf(&monster.AbilitiesOfInput{Sheet: s.sheet("runner")})
	require.NoError(t, err)
	require.Empty(t, got.Abilities)

	lieutenant := newUsesScene(t, compileSite(t, usesSite(t)))
	listed, err := monster.AbilitiesOf(&monster.AbilitiesOfInput{Sheet: lieutenant.sheet("lieutenant")})
	require.NoError(t, err)
	require.Len(t, listed.Abilities, 1)
	require.Equal(t, "second wind", listed.Abilities[0].Word)
	require.True(t, listed.Abilities[0].Payable)
}
