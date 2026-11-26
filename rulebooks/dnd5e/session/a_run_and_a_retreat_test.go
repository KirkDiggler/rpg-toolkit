// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// a_run_and_a_retreat_test.go plays "an NPC's turn is paid at the one gate"
// (rpg-project#1131, slice rpg-project#1211) on testdata/a-run-and-a-retreat-walk.yaml:
// a Runner who runs at the fighter and then swings once a turn, and a goblin
// Skirmisher who swings and then leaves her reach, disengaging first.
//
// THE DICE are secondWindDice with strike off: every die is min(size, 10), so
// every d20 is a 10 and every d6 a 6. The initiative order (fighter, Skirmisher,
// Runner) and every blow landing are fixed by that. None of the claims below
// depends on a roll: they count beats and read sheets, and a blow that missed
// would be a `missed` beat, which these counts also take.
//
// THE FIGHTER is a level 1 fighter with 60 of 60 hit points, set on the sheet
// before launch: three rounds of two attackers at most 6 a blow cannot reach
// that. She only ends her turn.

const runWalkSitePath = "testdata/a-run-and-a-retreat-walk.yaml"

// runWalkRound is what one round (the fighter's EndTurn, and the NPC turns it
// plays) told the fighter, split by who did it.
type runWalkRound struct {
	beats []witnessed
	// left is what each NPC's stored sheet held when the round ended.
	left map[string]runWalkLeft
}

// runWalkLeft is what a stored sheet says is left of a turn.
type runWalkLeft struct{ action, bonus int }

func (r runWalkRound) of(who string, beat ...string) []witnessed {
	var out []witnessed
	for _, b := range beatsNamed(r.beats, beat...) {
		for _, key := range []string{"actor", "member"} {
			if b.Fields[key] == who {
				out = append(out, b)
				break
			}
		}
	}
	return out
}

func (r runWalkRound) strikes(who string) []witnessed { return r.of(who, "struck", "missed") }

// runWalkScene is the site launched with a level 1 fighter, played round by round.
type runWalkScene struct {
	t        *testing.T
	mgr      *session.Manager
	sessions *fakeSessions
	rounds   []runWalkRound
}

func newRunWalkScene(t *testing.T, text string) *runWalkScene {
	t.Helper()
	compiled, err := dungeonspec.Load([]byte(text))
	require.NoError(t, err, "the site compiles")
	fighter := armedFighter("alice")
	setLevel(fighter, 1)
	fighter.HitPoints, fighter.MaxHitPoints = 60, 60
	s := &runWalkScene{t: t, sessions: newFakeSessions()}
	s.mgr, err = session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: &actCapture{}, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: &secondWindDice{},
		TurnDriver: session.Driver(), Sessions: s.sessions, Encounters: newFakeEncounters(), Characters: newFakeCharacters(fighter),
	})
	require.NoError(t, err)
	_, err = s.mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice"}})
	require.NoError(t, err)
	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, "alice", turn.Active, "the fight formed at launch and the fighter is first")
	return s
}

// play ends the fighter's turn n times and keeps what each round told her.
func (s *runWalkScene) play(n int) {
	s.t.Helper()
	for i := 0; i < n; i++ {
		mark := lastSeq(storyBeats(s.t, s.mgr, "sess", "alice"))
		_, err := s.mgr.EndTurn(context.Background(), &session.EndTurnInput{
			Session: "sess", Member: "alice", DeclarationID: currentEndTurnID(s.t, s.mgr, "sess", "alice"),
		})
		require.NoError(s.t, err, "claim 6: EndTurn in round %d returns no error", i+1)
		round := runWalkRound{beats: storyBeatsAfter(s.t, s.mgr, "sess", "alice", mark), left: map[string]runWalkLeft{}}
		for _, id := range []string{"runner", "skirmisher"} {
			sheet := s.sheet(id)
			round.left[id] = runWalkLeft{action: sheet.SlotsLeft(coreCombat.ActionStandard), bonus: sheet.SlotsLeft(coreCombat.ActionBonus)}
		}
		s.rounds = append(s.rounds, round)
	}
}

// sheet is the NPC's stored sheet now.
func (s *runWalkScene) sheet(id string) *monster.Data {
	s.t.Helper()
	for i := range s.sessions.byID["sess"].NPCs {
		if s.sessions.byID["sess"].NPCs[i].ID == id {
			return &s.sessions.byID["sess"].NPCs[i]
		}
	}
	require.FailNow(s.t, "no stored sheet", id)
	return nil
}

func runWalkSite(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(runWalkSitePath)
	require.NoError(t, err)
	return string(raw)
}

// Claim 7. The site compiles, and the read-back says nothing is unsaid about
// what it writes: the check the custom-item sites use (readback.Of's NotYetSaid).
func TestTheRunAndRetreatSiteCompilesWithNothingNotYetSaid(t *testing.T) {
	rb, err := readback.Of([]byte(runWalkSite(t)))
	require.NoError(t, err)
	require.Empty(t, rb.NotYetSaid, "every word the site writes is one the read-back can say")
}

// Claim 1. The Runner runs on his first turn and does not swing.
func TestTheRunnerRunsOnHisFirstTurnAndDoesNotSwing(t *testing.T) {
	s := newRunWalkScene(t, runWalkSite(t))
	s.play(1)
	round := s.rounds[0]

	steps := round.of("runner", "moved")
	require.GreaterOrEqual(t, len(steps), 10, "a run covers the 11 cells in one turn but for the last")
	for _, step := range steps {
		require.Equal(t, "encounter:table:dash", step.Fields["cause"], "every step names the run as its cause")
	}
	require.Empty(t, round.strikes("runner"), "he does not swing the turn he runs")
	require.Zero(t, round.left["runner"].action, "the run spent his action on the stored sheet")
}

// Claims 2, 4, 5 and 6, and the first three rounds of 3 where they hold.
func TestARunnerSwingsOnceATurnAndASkirmisherDisengagesOnceAndTheTurnComesBack(t *testing.T) {
	s := newRunWalkScene(t, runWalkSite(t))
	s.play(3)

	// Claim 2. After the run he strikes exactly once in each later round.
	for i, round := range s.rounds {
		require.LessOrEqual(t, len(round.strikes("runner")), 1, "round %d: he never strikes twice", i+1)
		if i > 0 {
			require.Len(t, round.strikes("runner"), 1, "round %d: in reach, he strikes once", i+1)
			require.Empty(t, round.of("runner", "moved"), "round %d: he is already beside her and does not walk", i+1)
		}
	}

	// Claim 4. One Nimble Escape, round one, before the walk; never two in a turn.
	first := s.rounds[0].of("skirmisher", "activated", "moved")
	require.NotEmpty(t, first)
	require.Equal(t, "activated", first[0].Beat, "the disengage is told before the walk")
	var named struct {
		Ability struct {
			Name string `json:"name"`
		} `json:"ability"`
	}
	first[0].decode(t, &named)
	require.Equal(t, "Nimble Escape", named.Ability.Name)
	for i, round := range s.rounds {
		require.LessOrEqual(t, len(round.of("skirmisher", "activated")), 1, "round %d: never two disengages in a turn", i+1)
	}
	require.Len(t, s.rounds[0].of("skirmisher", "activated"), 1)
	// After its turn, its stored sheet shows the action and the bonus action spent.
	require.Equal(t, runWalkLeft{action: 0, bonus: 0}, s.rounds[0].left["skirmisher"])
	// Round two's walk left nobody's melee reach: nothing asked it to disengage,
	// so no beat names it and its bonus action is not spent.
	require.Empty(t, s.rounds[1].of("skirmisher", "activated"))
	require.Equal(t, runWalkLeft{action: 0, bonus: 1}, s.rounds[1].left["skirmisher"])

	// Claim 3. One strike in each turn with a target in reach or range (rounds
	// one and two), never two; round three has none and says why.
	require.Len(t, s.rounds[0].strikes("skirmisher"), 1, "round one: it stabs her")
	require.Len(t, s.rounds[1].strikes("skirmisher"), 1, "round two: it shoots her from range")
	require.Empty(t, s.rounds[2].strikes("skirmisher"), "round three: nobody to strike")
	stayed := s.rounds[2].of("skirmisher", "stayed")
	require.Len(t, stayed, 1)
	require.Contains(t, stayed[0].Fields["why"], "nowhere farther", "and a stayed beat says why")

	// Claim 5. The turn comes back: the sheets are spent at the end of round one,
	// and in round two both act again, which they can only do if each one's turn
	// start gave the action back. (The boundary itself is inside one EndTurn, so
	// the fixture cannot stop between the boundary and the first act.)
	require.Len(t, s.rounds[1].strikes("skirmisher"), 1, "round two: the Skirmisher acts again")
	require.Len(t, s.rounds[1].strikes("runner"), 1, "round two: the Runner acts again")
	require.Equal(t, 0, s.rounds[0].left["runner"].action, "the Runner's sheet was spent at the end of round one")
	require.Equal(t, 0, s.rounds[1].left["runner"].action, "and again at the end of round two")
}
