// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// two_strikes_walk_test.go plays "two strikes at the captain" (rpg-project#1007)
// on its own small site, testdata/two-strikes-walk.yaml: a first lie is warned
// and a second is barred, and the captain reads the value of the note he gave.
// The lie's DC is 30 and every d20 is a 20, so it fails on the best roll.

// TwoStrikesWalkSuite plays the site through session.Manager.
type TwoStrikesWalkSuite struct {
	suite.Suite
	compiled   dungeonspec.Compiled
	characters *fakeCharacters
	sessions   *fakeSessions
	encounters *fakeEncounters
	live       *actCapture
}

func TestTwoStrikesWalkSuite(t *testing.T) { suite.Run(t, new(TwoStrikesWalkSuite)) }

func (s *TwoStrikesWalkSuite) SetupTest() {
	raw, err := os.ReadFile("testdata/two-strikes-walk.yaml")
	s.Require().NoError(err, "the site file is there")
	s.compiled, err = dungeonspec.Load(raw)
	s.Require().NoError(err, "the site compiles")
	alice, bob := armedFighter("alice"), armedFighter("bob")
	alice.AbilityScores[abilities.CHA], bob.AbilityScores[abilities.CHA] = 16, 16
	s.characters = newFakeCharacters(alice, bob)
	s.sessions, s.encounters, s.live = newFakeSessions(), newFakeEncounters(), &actCapture{}
}

// manager is a Manager over the site's stores; every d20 is a 20, the best
// roll there is. A second one is the host reloading what was saved.
func (s *TwoStrikesWalkSuite) manager() *session.Manager {
	faces := make([]int, 64)
	for i := range faces {
		faces[i] = 20
	}
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: s.live, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: &d20Script{faces: faces},
		TurnDriver: session.Driver(), Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
	})
	s.Require().NoError(err)
	return mgr
}

func (s *TwoStrikesWalkSuite) launch(party ...string) *session.Manager {
	mgr := s.manager()
	_, err := mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &s.compiled, Party: party})
	s.Require().NoError(err, "the site launches")
	return mgr
}

// offered is the ids of the captain's lines the member is offered right now.
func (s *TwoStrikesWalkSuite) offered(mgr *session.Manager, member string) map[string]bool {
	panel, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	out := map[string]bool{}
	for _, d := range panel.Declarations {
		for _, row := range d.SpeakOptions {
			if row.Target == "captain" && row.Available {
				out[row.OptionID] = true
			}
		}
	}
	return out
}

func (s *TwoStrikesWalkSuite) say(mgr *session.Manager, member, option string) (*session.SpeakOutput, []session.Act) {
	s.T().Helper()
	s.live.mu.Lock()
	s.live.acts = nil
	s.live.mu.Unlock()
	out, err := speakAsOffered(context.Background(), mgr, &session.SpeakInput{Session: "sess", Member: member, Target: "captain", OptionID: option})
	s.Require().NoError(err, "%s says %s", member, option)
	s.live.mu.Lock()
	defer s.live.mu.Unlock()
	return out, append([]session.Act(nil), s.live.acts...)
}

func (s *TwoStrikesWalkSuite) where(mgr *session.Manager, member string) spatial.Position {
	out, err := mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return out.Position
}

func (s *TwoStrikesWalkSuite) captainAt(mgr *session.Manager) spatial.Position {
	enc, err := mgr.LoadWorldForTest(context.Background(), "sess")
	s.Require().NoError(err)
	return memberOf(s.T(), enc, "captain").Position
}

func (s *TwoStrikesWalkSuite) walk(mgr *session.Manager, member string, path ...spatial.Position) *session.MoveOutput {
	s.T().Helper()
	out, err := walkOn(context.Background(), mgr, &session.MoveInput{Session: "sess", Member: member, Path: path})
	s.Require().NoError(err)
	return out
}

func (s *TwoStrikesWalkSuite) captainSays(acts []session.Act) string {
	captain := session.Party{Kind: session.PartyMember, ID: "captain"}
	lines := where[session.SpeechAct](acts, func(sp session.SpeechAct) bool { return sp.Speaker == captain && sp.Words != nil })
	s.Require().NotEmpty(lines, "the captain answered aloud")
	return *lines[0].body.Words
}

// waitUntilAtTheGate passes world rounds as alice, at most ten, until the
// captain stands on his gate spot.
func (s *TwoStrikesWalkSuite) waitUntilAtTheGate(mgr *session.Manager) int {
	for round := 1; round <= 10; round++ {
		_, err := mgr.Wait(context.Background(), &session.WaitInput{Session: "sess", Member: "alice"})
		s.Require().NoError(err)
		if s.captainAt(mgr) == twoStrikesGateSpot {
			return round
		}
	}
	s.Require().Equal(twoStrikesGateSpot, s.captainAt(mgr), "the captain walked out to his gate within ten rounds")
	return 0
}

// The site's cells, read from the file: the door's two crossings and the
// captain's start and gate spot.
var (
	twoStrikesGateSpot = spatial.Position{X: -4, Y: 8}
	twoStrikesStart    = spatial.Position{X: -1, Y: 4}
	strikesOutsideDoor = spatial.Position{X: -3, Y: 7}
	strikesBesideDoor  = spatial.Position{X: -4, Y: 7}
	strikesDoorway     = spatial.Position{X: -3, Y: 6}
)

const (
	firstWarning = "One more lie and you are out."
	secondLie    = "Lies. Get out before I change my mind."
)

// offeredIs asserts the member is offered exactly the named lines of the three.
func (s *TwoStrikesWalkSuite) offeredIs(mgr *session.Manager, member, when string, want ...string) {
	s.T().Helper()
	got := s.offered(mgr, member)
	wanted := map[string]bool{}
	for _, id := range want {
		wanted[id] = true
	}
	for _, id := range []string{"lie", "lie-again", "the-way-in"} {
		s.Equal(wanted[id], got[id], "%s %s: offered %q should be %v", member, when, id, wanted[id])
	}
}

// untilBobIsWarnedAtTheGate plays steps 1 to 5: alice lies twice and is sent
// out, both walk out, and bob lies once at the gate and is warned.
func (s *TwoStrikesWalkSuite) untilBobIsWarnedAtTheGate() *session.Manager {
	mgr := s.launch("alice", "bob")

	// 1. Both are offered the lie and the question, not the second lie.
	for _, who := range []string{"alice", "bob"} {
		s.offeredIs(mgr, who, "at the start", "lie", "the-way-in")
	}

	// 2. Alice's first lie fails and he warns her; he stays.
	lied, acts := s.say(mgr, "alice", "lie")
	s.Equal(30, lied.DC, "the lie's DC is 30")
	s.GreaterOrEqual(lied.Total, 20, "the roll was the best there is, a 20")
	s.False(lied.Beaten, "the first lie fails (total %d)", lied.Total)
	s.Equal(firstWarning, s.captainSays(acts), "he warns her")
	_, err := mgr.Wait(context.Background(), &session.WaitInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(twoStrikesStart, s.captainAt(mgr), "after one Wait he has not left his cell")
	s.offeredIs(mgr, "alice", "after her first lie", "lie-again", "the-way-in")
	s.offeredIs(mgr, "bob", "while alice is warned", "lie", "the-way-in")

	// 3. Her second lie fails; he sends her out and walks to his gate.
	lied, acts = s.say(mgr, "alice", "lie-again")
	s.False(lied.Beaten, "the second lie fails (total %d)", lied.Total)
	s.Equal(secondLie, s.captainSays(acts), "he calls it a lie")
	s.waitUntilAtTheGate(mgr)

	// 4. Both walk out; alice has lost every line, bob none.
	aliceWalk := s.walk(mgr, "alice", s.pathOut(mgr, "alice")...)
	s.Equal(session.MovementCompleted, aliceWalk.Status, "alice leaves through the door: %s", aliceWalk.StopReason)
	bobWalk := s.walk(mgr, "bob", s.pathOut(mgr, "bob")...)
	s.Equal(session.MovementCompleted, bobWalk.Status, "bob leaves through the door: %s", bobWalk.StopReason)
	s.offeredIs(mgr, "alice", "barred, at the gate")
	s.offeredIs(mgr, "bob", "unwarned, at the gate", "lie", "the-way-in")

	// 5. Bob lies once at the gate: warned, and he stays.
	_, acts = s.say(mgr, "bob", "lie")
	s.Equal(firstWarning, s.captainSays(acts), "he warns bob")
	s.Equal(twoStrikesGateSpot, s.captainAt(mgr), "and stays at his gate")
	s.offeredIs(mgr, "bob", "after his first lie", "lie-again", "the-way-in")
	return mgr
}

func (s *TwoStrikesWalkSuite) TestTwoStrikesAtTheCaptain() {
	mgr := s.untilBobIsWarnedAtTheGate()

	// 6. Alice, barred, is stopped at the door.
	before := s.where(mgr, "alice")
	back := s.walk(mgr, "alice", s.pathIn(before)...)
	s.Equal(session.MovementStopped, back.Status, "the barred one is stopped at the door")
	s.Equal("Guard Captain bars the way", back.StopReason, "and told who bars it")
	s.Empty(back.Steps, "she takes no step")
	s.Equal(before, s.where(mgr, "alice"), "she has not moved")

	// 7. Bob, only warned, walks in.
	in := s.walk(mgr, "bob", s.pathIn(s.where(mgr, "bob"))...)
	s.Equal(session.MovementCompleted, in.Status, "the warned one walks in: %s", in.StopReason)
	s.Equal(strikesDoorway, s.where(mgr, "bob"), "and stands inside, in the doorway")

	// 8. A reload reads the same lines.
	reloaded := s.manager()
	s.offeredIs(reloaded, "alice", "after a reload")
	s.offeredIs(reloaded, "bob", "after a reload", "lie-again", "the-way-in")
}

func (s *TwoStrikesWalkSuite) TestTheSecondPersonIsBarredAtTheGateToo() {
	mgr := s.untilBobIsWarnedAtTheGate()

	lied, acts := s.say(mgr, "bob", "lie-again")
	s.False(lied.Beaten, "bob's second lie fails (total %d)", lied.Total)
	s.Equal(secondLie, s.captainSays(acts), "he calls it a lie")
	s.offeredIs(mgr, "bob", "barred at the gate")

	before := s.where(mgr, "bob")
	back := s.walk(mgr, "bob", s.pathIn(before)...)
	s.Equal(session.MovementStopped, back.Status, "bob is stopped at the door as alice is")
	s.Equal("Guard Captain bars the way", back.StopReason, "and told who bars it")
	s.Empty(back.Steps, "he takes no step")
}

// pathOut walks a member from where they stand in the hall to a cell beside
// the captain's gate spot, out through the door.
func (s *TwoStrikesWalkSuite) pathOut(mgr *session.Manager, member string) []spatial.Position {
	here := s.where(mgr, member)
	path := s.route(here, strikesDoorway)
	if member == "alice" {
		return append(path, strikesOutsideDoor)
	}
	return append(path, strikesBesideDoor)
}

// pathIn is the step from a cell beside the door into its doorway.
func (s *TwoStrikesWalkSuite) pathIn(_ spatial.Position) []spatial.Position {
	return []spatial.Position{strikesDoorway}
}

// route is a straight walk of adjacent hex cells from a to b, down the r axis
// first: the hall is open floor, so no cell on it is in the way.
func (s *TwoStrikesWalkSuite) route(a, b spatial.Position) []spatial.Position {
	var out []spatial.Position
	cur := a
	for cur != b {
		switch {
		case cur.Y < b.Y && cur.X+cur.Y/2 >= b.X+b.Y/2 && cur.X > b.X-1:
			cur = spatial.Position{X: cur.X, Y: cur.Y + 1}
		case cur.Y < b.Y:
			cur = spatial.Position{X: cur.X - 1, Y: cur.Y + 1}
		case cur.X < b.X:
			cur = spatial.Position{X: cur.X + 1, Y: cur.Y}
		default:
			cur = spatial.Position{X: cur.X - 1, Y: cur.Y}
		}
		out = append(out, cur)
	}
	return out
}
