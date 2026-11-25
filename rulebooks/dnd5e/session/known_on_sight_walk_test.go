// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// known_on_sight_walk_test.go plays "known on sight" (rpg-project#1123) on its
// own small site, testdata/known-on-sight-walk.yaml: a note he gives wakes him
// (the topic beat), and a sighting of the one he barred wakes him again (the
// seen beat), the warned one never does. Nothing is rolled.
//
// The cells, chosen by running the scene and asking the engine what he sees
// (Encounter.SeesWithin, captain to alice, read through the session's world as
// captainAt reads his position): his door spot is (-4,6), inside the hall
// beside the doorway (-3,6). From it he sees (-3,7), outside the door, and does
// not see (-4,7), the yard cell beside it behind the gate wall. Bob, who cannot
// share alice's cell, comes back to the doorway (-3,6), which he sees too. The
// test asserts what he sees on each cell before it relies on it.

// KnownOnSightWalkSuite plays the site through session.Manager.
type KnownOnSightWalkSuite struct {
	suite.Suite
	compiled   dungeonspec.Compiled
	characters *fakeCharacters
	sessions   *fakeSessions
	encounters *fakeEncounters
	live       *actCapture
}

func TestKnownOnSightWalkSuite(t *testing.T) { suite.Run(t, new(KnownOnSightWalkSuite)) }

func (s *KnownOnSightWalkSuite) SetupTest() {
	raw, err := os.ReadFile("testdata/known-on-sight-walk.yaml")
	s.Require().NoError(err, "the site file is there")
	s.compiled, err = dungeonspec.Load(raw)
	s.Require().NoError(err, "the site compiles")
	alice, bob := armedFighter("alice"), armedFighter("bob")
	s.characters = newFakeCharacters(alice, bob)
	s.sessions, s.encounters, s.live = newFakeSessions(), newFakeEncounters(), &actCapture{}
}

// manager is a Manager over the site's stores; every d20 is a 20, the best
// roll there is. A second one is the host reloading what was saved.
func (s *KnownOnSightWalkSuite) manager() *session.Manager {
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

func (s *KnownOnSightWalkSuite) launch(party ...string) *session.Manager {
	mgr := s.manager()
	_, err := mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &s.compiled, Party: party})
	s.Require().NoError(err, "the site launches")
	return mgr
}

// offered is the ids of the captain's lines the member is offered right now.
func (s *KnownOnSightWalkSuite) offered(mgr *session.Manager, member string) map[string]bool {
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

func (s *KnownOnSightWalkSuite) say(mgr *session.Manager, member, option string) (*session.SpeakOutput, []session.Act) {
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

func (s *KnownOnSightWalkSuite) where(mgr *session.Manager, member string) spatial.Position {
	out, err := mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return out.Position
}

func (s *KnownOnSightWalkSuite) captainAt(mgr *session.Manager) spatial.Position {
	enc, err := mgr.LoadWorldForTest(context.Background(), "sess")
	s.Require().NoError(err)
	return memberOf(s.T(), enc, "captain").Position
}

func (s *KnownOnSightWalkSuite) walk(mgr *session.Manager, member string, path ...spatial.Position) *session.MoveOutput {
	s.T().Helper()
	out, err := walkOn(context.Background(), mgr, &session.MoveInput{Session: "sess", Member: member, Path: path})
	s.Require().NoError(err)
	return out
}

func (s *KnownOnSightWalkSuite) captainSays(acts []session.Act) string {
	captain := session.Party{Kind: session.PartyMember, ID: "captain"}
	lines := where[session.SpeechAct](acts, func(sp session.SpeechAct) bool { return sp.Speaker == captain && sp.Words != nil })
	s.Require().NotEmpty(lines, "the captain answered aloud")
	return *lines[0].body.Words
}

// The site's cells, read from the file and checked against the engine.
var (
	knownDoorSpot   = spatial.Position{X: -4, Y: 6}
	knownStart      = spatial.Position{X: -1, Y: 4}
	knownAliceStart = spatial.Position{X: -3, Y: 4}
	knownBobStart   = spatial.Position{X: -3, Y: 3}
	knownDoorway    = spatial.Position{X: -3, Y: 6}
	knownSeenCell   = spatial.Position{X: -3, Y: 7}
	knownUnseenCell = spatial.Position{X: -4, Y: 7}
)

const (
	mindYourTongue = "Mind your tongue."
	outOfMyHall    = "Out. Out of my hall."
	iSaidOut       = "I said out."
)

func (s *KnownOnSightWalkSuite) offeredIs(mgr *session.Manager, member, when string, want ...string) {
	s.T().Helper()
	got := s.offered(mgr, member)
	wanted := map[string]bool{}
	for _, id := range want {
		wanted[id] = true
	}
	for _, id := range []string{"rude", "insult", "the-way-in"} {
		s.Equal(wanted[id], got[id], "%s %s: offered %q should be %v", member, when, id, wanted[id])
	}
}

func (s *KnownOnSightWalkSuite) wait(mgr *session.Manager) {
	s.T().Helper()
	_, err := mgr.Wait(context.Background(), &session.WaitInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
}

// waitUntilAtTheDoor passes world rounds, at most ten, until he stands on his door spot.
func (s *KnownOnSightWalkSuite) waitUntilAtTheDoor(mgr *session.Manager) {
	for round := 1; round <= 10; round++ {
		s.wait(mgr)
		if s.captainAt(mgr) == knownDoorSpot {
			return
		}
	}
	s.Require().Equal(knownDoorSpot, s.captainAt(mgr), "he walked to his door spot within ten rounds")
}

// sees asks the engine, through the session's world, whether the captain sees the member.
func (s *KnownOnSightWalkSuite) sees(mgr *session.Manager, member string) bool {
	enc, err := mgr.LoadWorldForTest(context.Background(), "sess")
	s.Require().NoError(err)
	sees, known := enc.SeesWithin("captain", encounter.MemberID(member), 1_000_000)
	s.Require().True(known, "the engine knows both of them")
	return sees
}

// captainWords is every line the captain spoke aloud among the acts, in order.
func (s *KnownOnSightWalkSuite) captainWords() []string {
	captain := session.Party{Kind: session.PartyMember, ID: "captain"}
	s.live.mu.Lock()
	defer s.live.mu.Unlock()
	var out []string
	for _, sp := range where[session.SpeechAct](s.live.acts, func(sp session.SpeechAct) bool { return sp.Speaker == captain && sp.Words != nil }) {
		out = append(out, *sp.body.Words)
	}
	return out
}

func (s *KnownOnSightWalkSuite) forgetActs() {
	s.live.mu.Lock()
	s.live.acts = nil
	s.live.mu.Unlock()
}

// outAndAside walks the member out through the door and aside behind the wall.
func (s *KnownOnSightWalkSuite) outAndAside(mgr *session.Manager, member string) {
	s.T().Helper()
	// From the hall: down the q=-3 line to the doorway, then aside. From a
	// seen cell already past the door: aside, one step.
	var path []spatial.Position
	for y := s.where(mgr, member).Y + 1; y <= knownDoorway.Y; y++ {
		path = append(path, spatial.Position{X: -3, Y: y})
	}
	out := s.walk(mgr, member, append(path, knownUnseenCell)...)
	s.Equal(session.MovementCompleted, out.Status, "%s walks out and aside: %s", member, out.StopReason)
	s.Equal(knownUnseenCell, s.where(mgr, member), "%s stands aside in the yard", member)
	s.False(s.sees(mgr, member), "the captain does not see %s aside in the yard", member)
}

// backInSight steps the member back to a seen cell and passes two Waits,
// returning what the captain said from the start of that step to the end.
func (s *KnownOnSightWalkSuite) backInSight(mgr *session.Manager, member string, seen spatial.Position) []string {
	s.T().Helper()
	s.forgetActs()
	back := s.walk(mgr, member, seen)
	s.Equal(session.MovementCompleted, back.Status, "%s steps back: %s", member, back.StopReason)
	s.True(s.sees(mgr, member), "the captain sees %s on a seen cell", member)
	s.wait(mgr)
	s.wait(mgr)
	return s.captainWords()
}

// untilAliceIsBarred plays steps 1 to 3 (bob first, then alice), then reloads.
func (s *KnownOnSightWalkSuite) untilAliceIsBarred() *session.Manager {
	mgr := s.launch("alice", "bob")
	s.Equal(knownStart, s.captainAt(mgr), "he starts in the hall")
	for _, who := range []string{"alice", "bob"} {
		s.offeredIs(mgr, who, "at the start", "rude", "insult", "the-way-in")
	}

	// 1. Bob is merely rude; he is warned and stays.
	_, acts := s.say(mgr, "bob", "rude")
	s.Equal(mindYourTongue, s.captainSays(acts), "he warns bob")
	s.wait(mgr)
	s.Equal(knownStart, s.captainAt(mgr), "after one Wait he has not left his cell")
	s.offeredIs(mgr, "bob", "after being rude", "insult", "the-way-in")

	// 2. Alice insults him; the note itself wakes him.
	_, acts = s.say(mgr, "alice", "insult")
	s.Equal(outOfMyHall, s.captainSays(acts), "he bars alice")
	s.waitUntilAtTheDoor(mgr)

	// 3. Alice has lost every line; bob still has the question.
	s.offeredIs(mgr, "alice", "barred")
	s.offeredIs(mgr, "bob", "still warned only", "insult", "the-way-in")

	// A second manager over the same stores: everything after runs on it.
	return s.manager()
}

// captainDecisionsOnSeen counts the captain's behavior_decision entries on the
// seen beat in the session's saved log: how often the engine asked him about
// a sighting.
func (s *KnownOnSightWalkSuite) captainDecisionsOnSeen() int {
	s.T().Helper()
	stored, err := s.encounters.GetEncounter(context.Background(), "sess")
	s.Require().NoError(err)
	n := 0
	for _, entry := range stored.Log.Entries {
		var p struct {
			Beat   string `json:"beat"`
			Member string `json:"member"`
			On     string `json:"on"`
		}
		if json.Unmarshal(entry.Payload, &p) == nil && p.Beat == "behavior_decision" && p.Member == "captain" && p.On == "seen" {
			n++
		}
	}
	return n
}

func (s *KnownOnSightWalkSuite) TestKnownOnSight() {
	mgr := s.untilAliceIsBarred()
	s.Equal(knownAliceStart, s.where(mgr, "alice"), "alice has not moved")
	s.Equal(knownBobStart, s.where(mgr, "bob"), "bob has not moved")
	s.Equal(knownDoorSpot, s.captainAt(mgr), "he is still on his door spot after the reload")

	// The cells: aside he does not see her, on a seen cell he does.
	s.outAndAside(mgr, "alice")

	// 4. Back in his sight: he calls out within two Waits.
	beforeAlice := s.captainDecisionsOnSeen()
	s.Contains(s.backInSight(mgr, "alice", knownSeenCell), iSaidOut, "alice steps back into sight: the captain says 'I said out.'")

	s.Equal(beforeAlice+1, s.captainDecisionsOnSeen(), "alice's return asked the captain once")

	// 5. Bob does the same: he says nothing.
	s.outAndAside(mgr, "bob")
	beforeBob := s.captainDecisionsOnSeen()
	s.Empty(s.backInSight(mgr, "bob", knownDoorway), "bob steps back into sight: the captain says nothing")
	s.Equal(beforeBob, s.captainDecisionsOnSeen(), "the filter never asked the captain about bob")

	// 6. Alice again: he says it again.
	s.outAndAside(mgr, "alice")
	s.Contains(s.backInSight(mgr, "alice", knownSeenCell), iSaidOut, "alice again: the captain says 'I said out.' again")
}

func (s *KnownOnSightWalkSuite) TestTheOrderDoesNotMatter() {
	mgr := s.launch("alice", "bob")

	// Alice insults first; the note wakes him.
	_, acts := s.say(mgr, "alice", "insult")
	s.Equal(outOfMyHall, s.captainSays(acts), "he bars alice")
	s.waitUntilAtTheDoor(mgr)

	// Then bob is merely rude; he is warned, and stays at his door.
	_, acts = s.say(mgr, "bob", "rude")
	s.Equal(mindYourTongue, s.captainSays(acts), "he warns bob")
	s.wait(mgr)
	s.Equal(knownDoorSpot, s.captainAt(mgr), "he is still at his door")

	s.outAndAside(mgr, "alice")
	s.Contains(s.backInSight(mgr, "alice", knownSeenCell), iSaidOut, "he calls out for alice")
	s.outAndAside(mgr, "bob")
	s.Empty(s.backInSight(mgr, "bob", knownDoorway), "and not for bob")
}
