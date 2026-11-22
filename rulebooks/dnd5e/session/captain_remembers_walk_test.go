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

// captain_remembers_walk_test.go plays "the captain remembers who lied to him"
// (rpg-project#989) on its own small site, testdata/captain-remembers-walk.yaml:
// everything the scene needs is already in hand, the lie's DC is 30 so it fails
// whatever is rolled, and no step has to go right to reach the thing under test.

// The site's cells, read from the file: the door's two crossings, the captain's
// gate spot, and the cells the party stands on outside it.
var (
	theCaptainsGateSpot = spatial.Position{X: -4, Y: 8}
	outsideTheDoor      = spatial.Position{X: -3, Y: 7}
	besideTheDoor       = spatial.Position{X: -4, Y: 7}
	inTheDoorway        = spatial.Position{X: -3, Y: 6}
)

// CaptainRemembersWalkSuite plays the site through session.Manager.
type CaptainRemembersWalkSuite struct {
	suite.Suite
	compiled   dungeonspec.Compiled
	characters *fakeCharacters
	sessions   *fakeSessions
	encounters *fakeEncounters
	live       *actCapture
}

func TestCaptainRemembersWalkSuite(t *testing.T) { suite.Run(t, new(CaptainRemembersWalkSuite)) }

func (s *CaptainRemembersWalkSuite) SetupTest() {
	raw, err := os.ReadFile("testdata/captain-remembers-walk.yaml")
	s.Require().NoError(err, "the site file is there")
	s.compiled, err = dungeonspec.Load(raw)
	s.Require().NoError(err, "the site compiles")
	// Persuasion is the fighters' to try with disadvantage: CHA 16 so the best
	// roll is a real one, and the lie still falls short of 30.
	alice, bob := armedFighter("alice"), armedFighter("bob")
	alice.AbilityScores[abilities.CHA], bob.AbilityScores[abilities.CHA] = 16, 16
	s.characters = newFakeCharacters(alice, bob)
	s.sessions, s.encounters, s.live = newFakeSessions(), newFakeEncounters(), &actCapture{}
}

// manager is a Manager over the site's stores; every d20 is a 20, the best
// roll there is. A second one is the host reloading what was saved.
func (s *CaptainRemembersWalkSuite) manager() *session.Manager {
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

func (s *CaptainRemembersWalkSuite) launch(party ...string) *session.Manager {
	mgr := s.manager()
	_, err := mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &s.compiled, Party: party})
	s.Require().NoError(err, "the site launches")
	return mgr
}

// offered is the ids of the captain's lines the member is offered right now.
func (s *CaptainRemembersWalkSuite) offered(mgr *session.Manager, member string) map[string]bool {
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

func (s *CaptainRemembersWalkSuite) say(mgr *session.Manager, member, option string) (*session.SpeakOutput, []session.Act) {
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

func (s *CaptainRemembersWalkSuite) where(mgr *session.Manager, member string) spatial.Position {
	out, err := mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return out.Position
}

func (s *CaptainRemembersWalkSuite) captainAt(mgr *session.Manager) spatial.Position {
	enc, err := mgr.LoadWorldForTest(context.Background(), "sess")
	s.Require().NoError(err)
	return memberOf(s.T(), enc, "captain").Position
}

func (s *CaptainRemembersWalkSuite) walk(mgr *session.Manager, member string, path ...spatial.Position) *session.MoveOutput {
	s.T().Helper()
	out, err := walkOn(context.Background(), mgr, &session.MoveInput{Session: "sess", Member: member, Path: path})
	s.Require().NoError(err)
	return out
}

func (s *CaptainRemembersWalkSuite) captainSays(acts []session.Act) string {
	captain := session.Party{Kind: session.PartyMember, ID: "captain"}
	lines := where[session.SpeechAct](acts, func(sp session.SpeechAct) bool { return sp.Speaker == captain && sp.Words != nil })
	s.Require().NotEmpty(lines, "the captain answered aloud")
	return *lines[0].body.Words
}

// waitUntilAtTheGate passes world rounds as alice, at most ten, until the
// captain stands on his gate spot.
func (s *CaptainRemembersWalkSuite) waitUntilAtTheGate(mgr *session.Manager) int {
	for round := 1; round <= 10; round++ {
		_, err := mgr.Wait(context.Background(), &session.WaitInput{Session: "sess", Member: "alice"})
		s.Require().NoError(err)
		if s.captainAt(mgr) == theCaptainsGateSpot {
			return round
		}
	}
	s.Require().Equal(theCaptainsGateSpot, s.captainAt(mgr), "the captain walked out to his gate within ten rounds")
	return 0
}

// theLiarIsAtTheGate plays the scene up to both characters standing outside
// with the captain at his gate: alice has told the lie, he has walked out, and
// both have followed. It returns the manager and how many waits he needed.
func (s *CaptainRemembersWalkSuite) theLiarIsAtTheGate() (*session.Manager, int) {
	mgr := s.launch("alice", "bob")
	s.T().Logf("alice at %v, bob at %v, captain at %v", s.where(mgr, "alice"), s.where(mgr, "bob"), s.captainAt(mgr))

	// 1. Nothing to do first: both are offered the lie and the question.
	for _, who := range []string{"alice", "bob"} {
		lines := s.offered(mgr, who)
		s.True(lines["lie"] && lines["the-way-in"], "%s is offered the lie and the question from the start", who)
	}

	// 2. Alice lies; the best roll still fails.
	lied, acts := s.say(mgr, "alice", "lie")
	s.Equal(30, lied.DC, "the lie's DC is 30")
	s.GreaterOrEqual(lied.Total, 20, "the roll was the best there is, a 20")
	s.False(lied.Beaten, "the lie fails (total %d)", lied.Total)
	s.Equal("Lies. Get out before I change my mind.", s.captainSays(acts), "he calls it a lie")

	// 3. Rounds pass until he stands at his gate spot: one is all it takes.
	waits := s.waitUntilAtTheGate(mgr)
	s.T().Logf("the captain reached his gate after %d wait(s)", waits)

	// 4. Both walk out through the door, within his reach.
	aliceWalk := s.walk(mgr, "alice", s.pathOut(mgr, "alice")...)
	s.Equal(session.MovementCompleted, aliceWalk.Status, "alice leaves through the door: %s", aliceWalk.StopReason)
	bobWalk := s.walk(mgr, "bob", s.pathOut(mgr, "bob")...)
	s.Equal(session.MovementCompleted, bobWalk.Status, "bob leaves through the door: %s", bobWalk.StopReason)
	s.assertTheLiarLostTheLines(mgr, "after the walk out")
	return mgr, waits
}

func (s *CaptainRemembersWalkSuite) TestTheCaptainRemembersWhoLiedToHim() {
	mgr, waits := s.theLiarIsAtTheGate()
	s.Equal(1, waits, "one Wait is all he needs to reach his gate")

	// 5. The liar steps back in: stopped, by him, no step, not moved.
	before := s.where(mgr, "alice")
	back := s.walk(mgr, "alice", s.pathIn(before)...)
	s.Equal(session.MovementStopped, back.Status, "the liar is stopped at the door")
	s.Equal("Guard Captain bars the way", back.StopReason, "and told who bars it")
	s.Empty(back.Steps, "she takes no step")
	s.Equal(before, s.where(mgr, "alice"), "she has not moved")

	// 6. The friend steps in: he does.
	bobBefore := s.where(mgr, "bob")
	in := s.walk(mgr, "bob", s.pathIn(bobBefore)...)
	s.Equal(session.MovementCompleted, in.Status, "the friend walks in: %s", in.StopReason)
	s.Equal(inTheDoorway, s.where(mgr, "bob"), "and stands inside, in the doorway")

	// 7. A reload reads the same lines.
	reloaded := s.manager()
	s.assertTheLiarLostTheLines(reloaded, "after a reload")
}

// TestTheOtherCharactersLieIsCaughtToo asks what the site does not decide for
// itself: the captain is already in back-to-gate, and no transition enters that
// state again. It asserts what truly happens: the second liar is NOT caught.
func (s *CaptainRemembersWalkSuite) TestTheOtherCharactersLieIsCaughtToo() {
	mgr, _ := s.theLiarIsAtTheGate()

	lied, acts := s.say(mgr, "bob", "lie")
	s.False(lied.Beaten, "bob's lie fails too (total %d)", lied.Total)
	s.Equal("Lies. Get out before I change my mind.", s.captainSays(acts), "he calls it a lie")

	// What truly happens: he does not note the second liar. The captain is
	// already in back-to-gate and no transition enters it again, so bob's failed
	// lie writes no second note: bob is still offered both lines, and only
	// alice, the first liar, has lost them.
	s.assertTheLiarLostTheLines(mgr, "after bob's own lie")
}

// assertTheLiarLostTheLines: alice, caught lying, is offered neither the lie
// nor the way in; bob, who is not, still has both.
func (s *CaptainRemembersWalkSuite) assertTheLiarLostTheLines(mgr *session.Manager, when string) {
	s.T().Helper()
	alice, bob := s.offered(mgr, "alice"), s.offered(mgr, "bob")
	s.False(alice["lie"], "the liar is not offered \"He burned Ashford\" again %s", when)
	s.False(alice["the-way-in"], "the liar is not offered \"Where will the feast be\" %s", when)
	s.True(bob["lie"], "the friend still has \"He burned Ashford\" %s", when)
	s.True(bob["the-way-in"], "the friend still has \"Where will the feast be\" %s", when)
}

// pathOut walks a member from where they stand in the hall to a cell beside
// the captain's gate spot, out through the door.
func (s *CaptainRemembersWalkSuite) pathOut(mgr *session.Manager, member string) []spatial.Position {
	here := s.where(mgr, member)
	path := s.route(here, inTheDoorway)
	if member == "alice" {
		return append(path, outsideTheDoor)
	}
	return append(path, besideTheDoor)
}

// pathIn is the step from a cell beside the door into its doorway.
func (s *CaptainRemembersWalkSuite) pathIn(_ spatial.Position) []spatial.Position {
	return []spatial.Position{inTheDoorway}
}

// route is a straight walk of adjacent hex cells from a to b, down the r axis
// first: the hall is open floor, so no cell on it is in the way.
func (s *CaptainRemembersWalkSuite) route(a, b spatial.Position) []spatial.Position {
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
