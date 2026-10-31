// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// speak_reach_test.go is THE SPEAK MENU WITHIN REACH (rpg-project#753, slice
// S2, rulings R1 and R4): a member is offered the lines of what stands within
// [session.InteractionReach] of them, sighted (a creature) or seen now (a
// prop), and of nothing farther; a far target is walked up to first, and a
// line said to one from afar is refused at the door before anything is paid.
//
// Every scene stands on rows 0 and 1 of a pointy-top hall, where the authored
// offset frame and the axial frame a verb speaks are the same cells, so a
// position below means the same thing to the scene and to a Move.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type SpeakReachSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
}

func TestSpeakReachSuite(t *testing.T) {
	suite.Run(t, new(SpeakReachSuite))
}

func (s *SpeakReachSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(talkingFighter("alice"))
}

// aHall opens a 12x2 hall with alice at (1,1), the given props on its floor
// and the given creatures in it, neutral, so the clock is the world's.
func (s *SpeakReachSuite) aHall(props []encounter.PropInput, monsters ...dungeonspec.MonsterPlacement) *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{}, Dice: &sequenceDice{rolls: []int{10, 10, 10, 10, 10, 10}},
		TurnDriver: session.Pass{},
		Sessions:   s.sessions, Encounters: s.encounters,
		Characters: s.characters,
	})
	s.Require().NoError(err)

	for i := range monsters {
		monsters[i].Faction = "goblins"
	}
	launchScene(s.T(), mgr, scene{
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 12, 2)},
			Props:    props,
			Factions: []encounter.FactionInput{{ID: "goblins"}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{"goblins", encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
			}},
		},
		Party:    []sceneSeat{seatAt("alice", 1, 1)},
		Monsters: monsters,
	})

	return mgr
}

// guardAt is a goblin carrying the threat line.
func guardAt(id string, col, row int) dungeonspec.MonsterPlacement {
	g := monsterAt(id, refs.Monsters.Goblin().String(), col, row)
	g.Options = []encounter.Option{threatLine()}
	return g
}

// readLine is the one line every prop in these scenes carries.
func readLine() encounter.Option {
	return encounter.Option{
		ID: "read", Say: "I read it aloud.",
		Check: []encounter.CheckApproach{{Ability: "intimidation", DC: 5}},
	}
}

// aPropWithALine is a see-through, walk-over prop on a cell, carrying [readLine].
func aPropWithALine(id string, col, row int) encounter.PropInput {
	open := false
	return encounter.PropInput{
		ID: encounter.PropID(id), Ref: "altar", At: spatial.Position{X: float64(col), Y: float64(row)},
		BlocksMovement: &open, BlocksLineOfSight: &open,
		Options: []encounter.Option{readLine()},
	}
}

func (s *SpeakReachSuite) row(mgr *session.Manager) session.Declaration {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	for _, d := range out.Declarations {
		if d.Verb == session.VerbSpeak {
			return d
		}
	}
	s.Require().Fail("no Speak row")
	return session.Declaration{}
}

// offered is every target the Speak row lists a line for, creatures by id and
// props as "prop:<id>".
func (s *SpeakReachSuite) offered(mgr *session.Manager) map[string]bool {
	out := map[string]bool{}
	for _, line := range menu(s.row(mgr)) {
		out[line[0]] = true
	}
	return out
}

// distanceTo is how far alice stands from a member, by the encounter's one
// distance.
func (s *SpeakReachSuite) distanceTo(mgr *session.Manager, member encounter.MemberID) float64 {
	enc, err := mgr.LoadWorldForTest(context.Background(), "sess")
	s.Require().NoError(err)
	return enc.Distance(standingAt(enc, "alice"), standingAt(enc, member))
}

// walk moves alice on the world clock, where a move carries no declaration.
func (s *SpeakReachSuite) walk(mgr *session.Manager, path ...spatial.Position) {
	_, err := mgr.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "alice", Path: path})
	s.Require().NoError(err)
}

func (s *SpeakReachSuite) say(mgr *session.Manager, target, prop, option string) error {
	_, err := mgr.Speak(context.Background(), &session.SpeakInput{
		Session: "sess", Member: "alice", Target: target, Prop: prop, OptionID: option,
	})
	return err
}

// A GUARD FOUR HEXES OFF IS NOT OFFERED; WALK ONE STEP AND HE IS. The guard
// sees alice and she sees him the whole time — only the distance changes.
func (s *SpeakReachSuite) TestSpeakMenu_WithinReachOnly() {
	mgr := s.aHall(nil, guardAt("guard", 5, 1))
	s.Require().InDelta(4, s.distanceTo(mgr, "guard"), 0, "the scene: four hexes apart")

	row := s.row(mgr)
	s.False(s.offered(mgr)["guard"], "four hexes off is beyond reach, so he is not on the menu")
	s.NotContains(projectCandidateIDs(row), "guard", "nor a candidate")
	s.Require().NotNil(row.Why)
	s.Equal(session.ShortfallNoTargetInReach, row.Why.Reason, "nobody within reach is the reason, not a missing line")

	s.walk(mgr, spatial.Position{X: 2, Y: 1})
	s.Require().InDelta(3, s.distanceTo(mgr, "guard"), 0, "one step closer: three hexes")

	row = s.row(mgr)
	s.True(row.Available, "within reach the row is open")
	s.Contains(menu(row), [2]string{"guard", "threaten"}, "and his line appears on arrival")
	s.Contains(projectCandidateIDs(row), "guard")
	s.NoError(s.say(mgr, "guard", "", "threaten"), "the line the menu offered is heard")
}

// A PROP WITHIN REACH IS OFFERED ONLY WHEN IT IS SHOWN: a cache under an
// undiscovered secret two hexes off is no more on the menu than an id that
// names nothing, the altar beside it is, and an altar across the hall —
// shown, seen, but eight hexes off — is not.
func (s *SpeakReachSuite) TestSpeakMenu_HiddenPropNotOffered() {
	mgr := s.aHallWithASecret(
		aPropWithALine("altar", 2, 0),
		aPropWithALine("cache", 3, 0),
		aPropWithALine("far-altar", 9, 1),
	)

	offered := s.offered(mgr)
	s.True(offered["prop:altar"], "the shown altar within reach is on the menu")
	s.False(offered["prop:cache"], "the hidden cache within reach is not")
	s.False(offered["prop:far-altar"], "the shown altar beyond reach is not")

	err := s.say(mgr, "", "cache", "read")
	s.Require().ErrorIs(err, session.ErrNoProp, "a hidden prop is the probe law's answer at the door")
	s.NotErrorIs(err, session.ErrOutOfRange)
}

// aHallWithASecret is [SpeakReachSuite.aHall] with the "cache" prop hidden
// under a secret nobody can find.
func (s *SpeakReachSuite) aHallWithASecret(props ...encounter.PropInput) *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{}, Dice: &sequenceDice{rolls: []int{10, 10, 10, 10, 10, 10}},
		TurnDriver: session.Pass{},
		Sessions:   s.sessions, Encounters: s.encounters,
		Characters: s.characters,
	})
	s.Require().NoError(err)
	launchScene(s.T(), mgr, scene{
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 12, 2)},
			Props:   props,
			Concealments: []encounter.ConcealmentInput{{
				ID:     "the-cache",
				Checks: []encounter.CheckApproach{{Ability: "perception", DC: 30}},
				Props:  []encounter.PropID{"cache"},
			}},
		},
		Party: []sceneSeat{seatAt("alice", 1, 1)},
	})
	return mgr
}

// A CREATURE WITHIN REACH THAT ALICE DOES NOT SIGHT IS NOT OFFERED: a lurker
// two hexes off behind a wall is absent — proximity reveals nothing a map
// does not show — while the watcher on her side of the wall is offered.
func (s *SpeakReachSuite) TestSpeakMenu_UnsightedCreatureNotOffered() {
	mgr := s.aHall(
		occludingProps(spatial.Position{X: 2, Y: 0}, spatial.Position{X: 2, Y: 1}),
		guardAt("lurker", 3, 1), guardAt("watcher", 0, 0),
	)
	s.Require().LessOrEqual(s.distanceTo(mgr, "lurker"), float64(session.InteractionReach), "the lurker is within reach")
	s.Require().LessOrEqual(s.distanceTo(mgr, "watcher"), float64(session.InteractionReach))

	offered := s.offered(mgr)
	s.False(offered["lurker"], "unsighted within reach is not offered")
	s.True(offered["watcher"], "sighted within reach is")
}

// A THING IN YOUR HAND IS WITHIN REACH. The encounter's within-reach read
// leaves out what the member carries; the door admits it, so the menu does
// too: a scroll alice picked up is still hers to read aloud across the hall.
func (s *SpeakReachSuite) TestSpeakMenu_CarriedPropOffered() {
	scroll := aPropWithALine("scroll", 2, 1)
	scroll.Holdable = true
	mgr := s.aHall([]encounter.PropInput{scroll})

	_, err := mgr.Hold(context.Background(), &session.HoldInput{Session: "sess", Member: "alice", Target: "scroll"})
	s.Require().NoError(err)
	s.walk(mgr, spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1}, spatial.Position{X: 4, Y: 1},
		spatial.Position{X: 5, Y: 1}, spatial.Position{X: 6, Y: 1}, spatial.Position{X: 7, Y: 1})

	s.True(s.offered(mgr)["prop:scroll"], "the carried scroll is on the menu wherever she stands")
	s.NoError(s.say(mgr, "", "scroll", "read"), "and the door hears it")
}

// A LINE SAID FROM AFAR IS REFUSED AT THE DOOR, BEFORE THE PRICE. On the turn
// clock, a stale client speaking to a guard four hexes off is told the
// target is out of range, and alice keeps her action; a shown altar out of
// reach is refused the same way.
func (s *SpeakReachSuite) TestSpeak_OutOfReachRefused() {
	mgr := s.aHall([]encounter.PropInput{aPropWithALine("far-altar", 9, 1)}, guardAt("guard", 5, 1))
	authorTurnClock(s.T(), s.encounters, testSession, []string{"alice", "guard"}, 0)
	turn, err := mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn.Clock, "alice is on the turn clock, where a line costs her action")

	s.ErrorIs(s.say(mgr, "guard", "", "threaten"), session.ErrOutOfRange, "four hexes off is out of reach")
	s.ErrorIs(s.say(mgr, "", "far-altar", "read"), session.ErrOutOfRange, "so is the far altar")

	stored, err := s.characters.GetCharacter(context.Background(), "alice")
	s.Require().NoError(err)
	// A sheet never readied for a turn carries no economy, and a charged one
	// carries zero actions left; either way a spend shows.
	if stored.ActionEconomy != nil {
		s.Equal(1, stored.ActionEconomy.ActionsRemaining, "nothing was charged on the way to the refusal")
	}
}

// projectCandidateIDs is the Speak row's candidates by member id.
func projectCandidateIDs(row session.Declaration) []string {
	out := make([]string, 0, len(row.Candidates))
	for _, c := range row.Candidates {
		out = append(out, c.Member)
	}
	return out
}
