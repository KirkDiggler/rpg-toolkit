// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// guard_hails_walk_test.go plays "the guard hails" (rpg-project#1139, slice
// rpg-project#1141) on testdata/guard-hails-walk.yaml: a Guard Captain with NO
// conversation hails whoever steps within two of him in his sight, from his own
// machine, and the walker's walk stops because they were spoken to. Nothing
// stands afterwards. Nothing is rolled.
//
// The cells, chosen by asking the engine (Encounter.Distance, read through the
// session's world; the test asserts each distance before it relies on it): the
// captain stands on (1,4). Alice starts on (-3,4), four from him, and bob on
// (-3,3), five. Alice's walk is (-2,4) three, (-1,4) two, (0,4) one, on
// along row 4: it must stop on (-1,4), the first cell within two. She then
// steps to (-1,5), two from him; goes out to (-2,5), three; and back to (-1,5).
// Bob's walk is (-2,3) four, (-1,3) three, (0,3) two, (1,3) one: it stops on
// (0,3). He steps away to (-1,3) and back to (0,3).

// GuardHailsWalkSuite plays the site through session.Manager.
type GuardHailsWalkSuite struct {
	suite.Suite
	compiled   dungeonspec.Compiled
	characters *fakeCharacters
	sessions   *fakeSessions
	encounters *fakeEncounters
	live       *actCapture
}

func TestGuardHailsWalkSuite(t *testing.T) { suite.Run(t, new(GuardHailsWalkSuite)) }

func (s *GuardHailsWalkSuite) SetupTest() {
	raw, err := os.ReadFile("testdata/guard-hails-walk.yaml")
	s.Require().NoError(err, "the site file is there")
	s.compiled, err = dungeonspec.Load(raw)
	s.Require().NoError(err, "the site compiles")
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	s.sessions, s.encounters, s.live = newFakeSessions(), newFakeEncounters(), &actCapture{}
}

// manager is a Manager over the site's stores; a second one is the host
// reloading what was saved.
func (s *GuardHailsWalkSuite) manager() *session.Manager {
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

func (s *GuardHailsWalkSuite) where(mgr *session.Manager, member string) spatial.Position {
	out, err := mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return out.Position
}

// distance asks the engine how far the captain is from a cell.
func (s *GuardHailsWalkSuite) distance(mgr *session.Manager, cell spatial.Position) float64 {
	enc, err := mgr.LoadWorldForTest(context.Background(), "sess")
	s.Require().NoError(err)
	return enc.Distance(memberOf(s.T(), enc, "captain").Position, cell)
}

func (s *GuardHailsWalkSuite) walk(mgr *session.Manager, member string, path ...spatial.Position) (*session.MoveOutput, []string) {
	s.T().Helper()
	s.live.mu.Lock()
	s.live.acts = nil
	s.live.mu.Unlock()
	out, err := walkOn(context.Background(), mgr, &session.MoveInput{Session: "sess", Member: member, Path: path})
	s.Require().NoError(err)
	return out, s.captainWords(member)
}

// captainWords is every line the captain spoke aloud among the acts the member
// was sent, in order. Every recipient is sent its own copy, so one member's is
// the whole story once.
func (s *GuardHailsWalkSuite) captainWords(member string) []string {
	captain := session.Party{Kind: session.PartyMember, ID: "captain"}
	var out []string
	for _, sp := range where[session.SpeechAct](s.live.of(member), func(sp session.SpeechAct) bool { return sp.Speaker == captain && sp.Words != nil }) {
		out = append(out, *sp.body.Words)
	}
	return out
}

func (s *GuardHailsWalkSuite) say(mgr *session.Manager, member, option string) []string {
	s.T().Helper()
	s.live.mu.Lock()
	s.live.acts = nil
	s.live.mu.Unlock()
	_, err := speakAsOffered(context.Background(), mgr, &session.SpeakInput{Session: "sess", Member: member, Target: "captain", OptionID: option})
	s.Require().NoError(err, "%s says %s", member, option)
	return s.captainWords(member)
}

func (s *GuardHailsWalkSuite) offered(mgr *session.Manager, member string) bool {
	panel, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	for _, d := range panel.Declarations {
		for _, row := range d.SpeakOptions {
			if row.Target == "captain" && row.OptionID == "passing" && row.Available {
				return true
			}
		}
	}
	return false
}

var (
	hailAliceStart = spatial.Position{X: -3, Y: 4}
	hailBobStart   = spatial.Position{X: -3, Y: 3}
	hailCaptain    = spatial.Position{X: 1, Y: 4}

	hailAliceFirst  = spatial.Position{X: -1, Y: 4} // first cell within two on row 4
	hailAliceNear   = spatial.Position{X: -1, Y: 5} // another cell within two
	hailAliceFar    = spatial.Position{X: -2, Y: 5} // beyond two
	hailBobFirst    = spatial.Position{X: 0, Y: 3}  // first cell within two on row 3
	hailBobAway     = spatial.Position{X: -1, Y: 3} // beyond two
	hailAliceWalked = []spatial.Position{{X: -2, Y: 4}, {X: -1, Y: 4}, {X: 0, Y: 4}, {X: 0, Y: 5}}
)

const (
	yourBusiness = "You there. State your business."
	thenPass     = "Then pass."
)

func (s *GuardHailsWalkSuite) TestTheGuardHails() {
	mgr := s.manager()
	_, err := mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &s.compiled, Party: []string{"alice", "bob"}})
	s.Require().NoError(err, "the site launches")

	// The cells, as the engine measures them from the captain.
	s.Equal(hailCaptain, memberOfWorld(s, mgr, "captain"), "he starts in the hall")
	for cell, want := range map[spatial.Position]float64{
		hailAliceStart: 4, hailBobStart: 5, {X: -2, Y: 4}: 3, hailAliceFirst: 2, {X: 0, Y: 4}: 1,
		hailAliceNear: 2, hailAliceFar: 3, {X: -2, Y: 3}: 4, {X: -1, Y: 3}: 3, hailBobFirst: 2,
	} {
		s.InDelta(want, s.distance(mgr, cell), 0.001, "cell %v is %v from the captain", cell, want)
	}

	// 1. Alice walks toward and past him: the walk stops on the first cell
	// within two, and the cells after it are not taken.
	out, words := s.walk(mgr, "alice", hailAliceWalked[0], hailAliceWalked[1], hailAliceWalked[2], hailAliceWalked[3])
	s.Equal(session.MovementStopped, out.Status, "the hail stops her walk: %s", out.StopReason)
	s.Equal([]string{"captain"}, out.Hailed, "the captain hailed her")
	s.Nil(out.Addressed, "nothing stands afterwards: it is no conversation's address")
	s.Equal([]string{yourBusiness}, words, "his line is said inside the step")
	s.Equal(hailAliceFirst, s.where(mgr, "alice"), "she stands on the first cell within two")
	s.Len(out.Steps, 2, "two steps were taken, the other two never attempted")
	s.NotEqual(hailAliceWalked[2], s.where(mgr, "alice"), "she did not go on to (0,4)")

	// 2. Alice steps to another cell within two of him: he says nothing more.
	out, words = s.walk(mgr, "alice", hailAliceNear)
	s.Equal(session.MovementCompleted, out.Status)
	s.Empty(out.Hailed, "standing near is not crossing in")
	s.Empty(words, "he says nothing more")
	s.Equal(hailAliceNear, s.where(mgr, "alice"))

	// 3. Alice walks out beyond two and back within: he hails her again.
	out, words = s.walk(mgr, "alice", hailAliceFar)
	s.Equal(session.MovementCompleted, out.Status)
	s.Empty(out.Hailed)
	s.Empty(words, "going out is not a hail")
	out, words = s.walk(mgr, "alice", hailAliceNear)
	s.Equal(session.MovementCompleted, out.Status, "the hail landed on the walk's last cell: it was not cut short")
	s.Equal([]string{"captain"}, out.Hailed, "back within two: hailed again")
	s.Equal([]string{yourBusiness}, words)
	s.Equal(hailAliceNear, s.where(mgr, "alice"))

	// 4. Bob walks up: he hails bob too. A second manager over the same stores
	// is the reload; everything after runs on it.
	mgr = s.manager()
	out, words = s.walk(mgr, "bob", spatial.Position{X: -2, Y: 3}, spatial.Position{X: -1, Y: 3}, hailBobFirst, spatial.Position{X: 1, Y: 3})
	s.Equal(session.MovementStopped, out.Status, "bob's walk stops too: %s", out.StopReason)
	s.Equal([]string{"captain"}, out.Hailed)
	s.Equal([]string{yourBusiness}, words)
	s.Equal(hailBobFirst, s.where(mgr, "bob"))
	s.Len(out.Steps, 3)

	// 5. Alice answers: he says "Then pass." and notes her greeted.
	s.True(s.offered(mgr, "alice"), "alice is offered her line")
	s.Equal([]string{thenPass}, s.say(mgr, "alice", "passing"))
	s.False(s.offered(mgr, "alice"), "he noted her greeted: the line is gone")
	s.True(s.offered(mgr, "bob"), "bob has not answered, so his line stands")

	// 6. Alice walks away and back: he says nothing and her walk is not
	// stopped. Bob walks away and back: he is hailed.
	out, words = s.walk(mgr, "alice", hailAliceFar, hailAliceNear, hailAliceFar, hailAliceNear)
	s.Equal(session.MovementCompleted, out.Status, "alice is greeted: nothing stops her: %s", out.StopReason)
	s.Empty(out.Hailed, "he does not hail alice again")
	s.Empty(words, "he says nothing")
	s.Equal(hailAliceNear, s.where(mgr, "alice"))
	out, _ = s.walk(mgr, "bob", hailBobAway)
	s.Equal(session.MovementCompleted, out.Status)
	s.Empty(out.Hailed)
	out, words = s.walk(mgr, "bob", hailBobFirst)
	s.Equal(session.MovementCompleted, out.Status, "the hail landed on the last cell")
	s.Equal([]string{"captain"}, out.Hailed, "bob is hailed again")
	s.Equal([]string{yourBusiness}, words)
}

func memberOfWorld(s *GuardHailsWalkSuite, mgr *session.Manager, id string) spatial.Position {
	enc, err := mgr.LoadWorldForTest(context.Background(), "sess")
	s.Require().NoError(err)
	return memberOf(s.T(), enc, encounter.MemberID(id)).Position
}
