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

// slipped_away_walk_test.go plays "the one who slipped away" (rpg-project#1134,
// slice rpg-project#1136) on its own small site, testdata/slipped-away-walk.yaml:
// a state of his notes `standing` as barred about the party he sees and as
// wanted about those he saw who are gone, and a sighting of the wanted one wakes
// him: "There is the other one." Nothing is rolled. The room is that of the
// known-on-sight site (known_on_sight_walk_test.go), but its aside cell (-4,7)
// was only checked from his door spot: here bob must be out of sight while the
// captain still stands at his start (-1,4), so the aside cell was found again.
//
// THE CELLS, asked of the engine (Encounter.SeesWithin, captain to bob, through
// the session's world), bob walking a snake through the yard, once with the
// captain at his start (-1,4) and once at his door spot (-4,6):
//
//	cell     from (-1,4)  from (-4,6)
//	(-3,7)   sees         sees        the seen cell, outside the door
//	(-4,7)   sees         not         (the old aside cell: seen from his start)
//	(-5,7)   sees         not
//	(-6,7)   not          not         THE ASIDE CELL, four steps out from the doorway
//	(-7,7)   not          not
//	(-8,8)   not          not
//	(-7,8)   not          not
//	(-4,8)   sees         not
//	(-3,8)   not          sees
//	(-8,9)   not          not
//
// Bob reaches (-6,7) in one walk of the hall column to the doorway (-3,6) and
// then (-3,7), (-4,7), (-5,7), (-6,7); no Waits between. He comes back along
// the same row to (-3,7). The test asserts what he sees on the cells it uses.

// SlippedAwayWalkSuite plays the site through session.Manager.
type SlippedAwayWalkSuite struct {
	suite.Suite
	compiled   dungeonspec.Compiled
	characters *fakeCharacters
	sessions   *fakeSessions
	encounters *fakeEncounters
	live       *actCapture
}

func TestSlippedAwayWalkSuite(t *testing.T) { suite.Run(t, new(SlippedAwayWalkSuite)) }

func (s *SlippedAwayWalkSuite) SetupTest() {
	raw, err := os.ReadFile("testdata/slipped-away-walk.yaml")
	s.Require().NoError(err, "the site file is there")
	s.compiled, err = dungeonspec.Load(raw)
	s.Require().NoError(err, "the site compiles")
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	s.sessions, s.encounters, s.live = newFakeSessions(), newFakeEncounters(), &actCapture{}
}

// manager is a Manager over the site's stores; a second one is the host
// reloading what was saved.
func (s *SlippedAwayWalkSuite) manager() *session.Manager {
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

func (s *SlippedAwayWalkSuite) launch(party ...string) *session.Manager {
	mgr := s.manager()
	_, err := mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &s.compiled, Party: party})
	s.Require().NoError(err, "the site launches")
	return mgr
}

// offeredIs checks the captain's lines the member is offered right now are
// exactly the ones wanted, among the site's two.
func (s *SlippedAwayWalkSuite) offeredIs(mgr *session.Manager, member, when string, want ...string) {
	s.T().Helper()
	panel, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	got := map[string]bool{}
	for _, d := range panel.Declarations {
		for _, row := range d.SpeakOptions {
			if row.Target == "captain" && row.Available {
				got[row.OptionID] = true
			}
		}
	}
	wanted := map[string]bool{}
	for _, id := range want {
		wanted[id] = true
	}
	for _, id := range []string{"insult", "the-way-in"} {
		s.Equal(wanted[id], got[id], "%s %s: offered %q should be %v", member, when, id, wanted[id])
	}
}

func (s *SlippedAwayWalkSuite) say(mgr *session.Manager, member, option string) string {
	s.T().Helper()
	s.forgetActs()
	_, err := speakAsOffered(context.Background(), mgr, &session.SpeakInput{Session: "sess", Member: member, Target: "captain", OptionID: option})
	s.Require().NoError(err, "%s says %s", member, option)
	words := s.captainWords()
	s.Require().NotEmpty(words, "the captain answered aloud")
	return words[0]
}

func (s *SlippedAwayWalkSuite) where(mgr *session.Manager, member string) spatial.Position {
	out, err := mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return out.Position
}

func (s *SlippedAwayWalkSuite) captainAt(mgr *session.Manager) spatial.Position {
	enc, err := mgr.LoadWorldForTest(context.Background(), "sess")
	s.Require().NoError(err)
	return memberOf(s.T(), enc, "captain").Position
}

func (s *SlippedAwayWalkSuite) walk(mgr *session.Manager, member string, path ...spatial.Position) *session.MoveOutput {
	s.T().Helper()
	out, err := walkOn(context.Background(), mgr, &session.MoveInput{Session: "sess", Member: member, Path: path})
	s.Require().NoError(err)
	return out
}

func (s *SlippedAwayWalkSuite) wait(mgr *session.Manager) {
	s.T().Helper()
	_, err := mgr.Wait(context.Background(), &session.WaitInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
}

// waitUntilAtTheDoor passes world rounds, at most ten, until he stands on his door spot.
func (s *SlippedAwayWalkSuite) waitUntilAtTheDoor(mgr *session.Manager) {
	for round := 1; round <= 10; round++ {
		s.wait(mgr)
		if s.captainAt(mgr) == knownDoorSpot {
			return
		}
	}
	s.Require().Equal(knownDoorSpot, s.captainAt(mgr), "he walked to his door spot within ten rounds")
}

// sees asks the engine, through the session's world, whether the captain sees the member.
func (s *SlippedAwayWalkSuite) sees(mgr *session.Manager, member string) bool {
	enc, err := mgr.LoadWorldForTest(context.Background(), "sess")
	s.Require().NoError(err)
	sees, known := enc.SeesWithin("captain", encounter.MemberID(member), 1_000_000)
	s.Require().True(known, "the engine knows both of them")
	return sees
}

// captainWords is every line the captain spoke aloud since the acts were last forgotten, in order.
func (s *SlippedAwayWalkSuite) captainWords() []string {
	captain := session.Party{Kind: session.PartyMember, ID: "captain"}
	s.live.mu.Lock()
	defer s.live.mu.Unlock()
	var out []string
	for _, sp := range where[session.SpeechAct](s.live.acts, func(sp session.SpeechAct) bool { return sp.Speaker == captain && sp.Words != nil }) {
		out = append(out, *sp.body.Words)
	}
	return out
}

func (s *SlippedAwayWalkSuite) forgetActs() {
	s.live.mu.Lock()
	s.live.acts = nil
	s.live.mu.Unlock()
}

// outAndAside walks the member out through the door and aside behind the wall.
func (s *SlippedAwayWalkSuite) outAndAside(mgr *session.Manager, member string) {
	s.T().Helper()
	var path []spatial.Position
	for y := s.where(mgr, member).Y + 1; y <= knownDoorway.Y; y++ {
		path = append(path, spatial.Position{X: -3, Y: y})
	}
	for x := -3.0; x >= slipAside.X; x-- {
		path = append(path, spatial.Position{X: x, Y: 7})
	}
	out := s.walk(mgr, member, path...)
	s.Equal(session.MovementCompleted, out.Status, "%s walks out and aside: %s", member, out.StopReason)
	s.Equal(slipAside, s.where(mgr, member), "%s stands aside in the yard", member)
	s.False(s.sees(mgr, member), "the captain does not see %s aside in the yard", member)
}

// backInSight steps the member back to a seen cell and passes two Waits,
// returning what the captain said from the start of that step to the end.
func (s *SlippedAwayWalkSuite) backInSight(mgr *session.Manager, member string, seen spatial.Position) []string {
	s.T().Helper()
	s.forgetActs()
	var path []spatial.Position
	for x := slipAside.X + 1; x <= seen.X; x++ {
		path = append(path, spatial.Position{X: x, Y: 7})
	}
	if seen.Y != 7 {
		path = append(path, seen)
	}
	back := s.walk(mgr, member, path...)
	s.Equal(session.MovementCompleted, back.Status, "%s steps back: %s", member, back.StopReason)
	s.True(s.sees(mgr, member), "the captain sees %s on a seen cell", member)
	s.wait(mgr)
	s.wait(mgr)
	return s.captainWords()
}

// slipAside is the aside cell: unseen from his start and from his door spot.
var slipAside = spatial.Position{X: -6, Y: 7}

const thereIsTheOtherOne = "There is the other one."

func (s *SlippedAwayWalkSuite) TestTheOneWhoSlippedAway() {
	mgr := s.launch("alice", "bob")
	s.Equal(knownStart, s.captainAt(mgr), "he starts in the hall")
	for _, who := range []string{"alice", "bob"} {
		s.offeredIs(mgr, who, "at the start", "insult", "the-way-in")
	}

	// 1. Bob walks out and steps aside, where the captain does not see him.
	s.outAndAside(mgr, "bob")
	s.Equal(knownStart, s.captainAt(mgr), "the captain has not moved")

	// 2. Alice insults him; the note wakes him; he notes alice barred and bob wanted, and goes to his door.
	s.Equal(outOfMyHall, s.say(mgr, "alice", "insult"), "he bars alice")
	s.waitUntilAtTheDoor(mgr)

	// 3. Alice has lost every line.
	s.offeredIs(mgr, "alice", "barred")

	// A second manager over the same stores: everything after runs on it.
	mgr = s.manager()
	s.Equal(knownDoorSpot, s.captainAt(mgr), "he is still on his door spot after the reload")
	s.offeredIs(mgr, "alice", "barred, after the reload")

	// 4. Bob steps back into view: known on sight as the one who slipped away.
	s.Contains(s.backInSight(mgr, "bob", knownSeenCell), thereIsTheOtherOne, "bob steps back into sight: the captain says 'There is the other one.'")
	s.offeredIs(mgr, "bob", "wanted: he has the insult and not the way in", "insult")

	// 5. Alice walks out, aside and back: she is barred, not wanted; he says nothing.
	s.outAndAside(mgr, "alice")
	s.Empty(s.backInSight(mgr, "alice", knownDoorway), "alice steps back into sight, on the doorway bob left free: the captain says nothing")
}

func (s *SlippedAwayWalkSuite) TestNobodySlippedAway() {
	mgr := s.launch("alice", "bob")

	// Bob stays in the hall. Alice insults the captain: both are barred, nobody is wanted.
	s.Equal(outOfMyHall, s.say(mgr, "alice", "insult"), "he bars alice")
	s.waitUntilAtTheDoor(mgr)
	s.offeredIs(mgr, "alice", "barred")
	s.offeredIs(mgr, "bob", "barred with her: the insult and not the way in", "insult")

	// Bob walks out, aside and back: the captain says nothing.
	s.outAndAside(mgr, "bob")
	s.Empty(s.backInSight(mgr, "bob", knownSeenCell), "bob steps back into sight: the captain says nothing")
}
