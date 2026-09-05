// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// holdout_strike_test.go is Kirk's walk on the raider camp (2026-09-05) at
// the seam: the chief's DRIVEN turn once he is in the fight and the letter
// is in the yard. The redis state at the failure: the chief on the hut's
// doorway cell, the letter-holder in the yard, the holder's turn to end.
//
// Three scenes, one mechanism. A driven turn is several intents — a step,
// then a swing — and the chief's own step into the yard is the presence
// that teaches him the fact (design §3.6): the camp turns on HIS step, the
// fight dissolves by stance, and the hold-out ends. What these scenes pin is
// that the turn stops there: nothing swings after the run closed, and a
// chief who is opposed to nobody does not strike.

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/scenarios"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// stout is a fighter who survives a few skeleton turns: the scenes here are
// about the chief's turn, not about anybody going down.
func stout(id string) *character.Data {
	c := armedFighter(id)
	c.HitPoints, c.MaxHitPoints = 200, 200
	return c
}

// arrangeTheChiefAtTheDoor is the redis state: the holder carries the letter
// to the yard cell one step beyond the hut's doorway, the other player one
// step further along the same line, and then the chief is spawned ON the
// doorway's hut-side cell — in plain sight of both through the doorway, so
// a fight forms with him in it and his driven turn steps him into the yard.
func (s *HoldOutSessionSuite) arrangeTheChiefAtTheDoor(holder, other string) {
	s.T().Helper()
	near, far := s.doorway(yardHut, "yard")
	step := spatial.Position{X: near.X - far.X, Y: near.Y - far.Y}
	beyond := spatial.Position{X: near.X + step.X, Y: near.Y + step.Y}
	further := spatial.Position{X: beyond.X + step.X, Y: beyond.Y + step.Y}

	s.hold(holder, campLetter)
	s.Require().Nil(s.walk(holder, beyond).Formed, "nobody to fight yet")
	s.Require().Nil(s.walk(other, further).Formed)

	chief := s.placement(campChief)
	out, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: campSession, ID: chief.ID, Ref: chief.Ref, Position: far,
		Holds: chief.Holds, Faction: chief.Faction,
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "the chief at the door sees the party in the yard")
	s.Require().Contains(out.Formed.Order, campChief)
	s.stream.published = nil
}

// endTurnOf ends a player's turn through the verb, driving whoever the
// clock lands on next.
func (s *HoldOutSessionSuite) endTurnOf(who string) error {
	s.T().Helper()
	s.Require().Equal(who, s.turn(who).Active, "precondition: it is %s's turn", who)
	_, err := s.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: campSession, Member: who,
		DeclarationID: currentEndTurnID(s.T(), s.mgr, campSession, who),
	})
	return err
}

// after is the kinds on a recipient's stream from the first of a kind on.
func after(kinds []session.EventKind, from session.EventKind) []session.EventKind {
	for i, k := range kinds {
		if k == from {
			return kinds[i:]
		}
	}
	return nil
}

// TestTheChiefsOwnStepTurnsTheCampAndEndsHisTurn is the EndTurn failure
// Kirk walked into: the chief steps out of the hut into the yard where the
// letter is, the camp turns on his step, the fight dissolves by stance and
// the hold-out ends — and his turn ends THERE. The verb succeeds, the
// ending reaches everyone, and nothing swings after the run closed.
func (s *HoldOutSessionSuite) TestTheChiefsOwnStepTurnsTheCampAndEndsHisTurn() {
	s.startWith(campOptions{withEnding: true, driver: session.Behavior(),
		cast: []*character.Data{stout("alice"), stout("bob")}, spawn: []string{}})
	s.arrangeTheChiefAtTheDoor("alice", "bob")

	s.Require().NoError(s.endTurnOf("alice"))
	s.Require().NoError(s.endTurnOf("bob"), "the chief's driven turn is inside this verb")

	status := s.status()
	s.False(status.Open, "the camp turned on the chief's own step, and that is the hold-out")
	s.Require().NotNil(status.Outcome)
	s.Equal(scenarios.HoldOutID, status.Outcome.Ending)

	for _, who := range []string{"alice", "bob"} {
		s.Equal([]session.EventKind{session.EventStanceChanged, session.EventFightEnded, session.EventEnded},
			after(s.kinds(who), session.EventStanceChanged),
			"%s: the flip, the fight ending by stance, the run ending — and no swing after", who)
		s.Equal(session.FightEndedBody{Cause: session.DissolveByStance}, s.bodyOf(who, session.EventFightEnded))
	}
}

// TestAChiefWhoseCampTurnedDoesNotSwing is the same driven turn on a run
// nothing ends: the chief steps into the yard, the camp turns, the fight
// dissolves — and a chief opposed to nobody does not strike the player he
// was walking toward. Everyone is back on the world clock, unstruck.
func (s *HoldOutSessionSuite) TestAChiefWhoseCampTurnedDoesNotSwing() {
	s.startWith(campOptions{withEnding: false, driver: session.Behavior(),
		cast: []*character.Data{stout("alice"), stout("bob")}, spawn: []string{}})
	s.arrangeTheChiefAtTheDoor("alice", "bob")

	s.Require().NoError(s.endTurnOf("alice"))
	s.Require().NoError(s.endTurnOf("bob"))

	s.Equal([]session.EventKind{session.EventStanceChanged, session.EventFightEnded},
		after(s.kinds("alice"), session.EventStanceChanged),
		"the flip and the dissolution, and nothing after: no struck, no missed")
	s.Equal(session.ClockWorld, s.turn("alice").Clock)
	s.Equal(session.ClockWorld, s.turn(campChief).Clock)
	s.True(s.status().Open, "nothing was declared to end on the flip")
}

// TestTheActiveHolderExitingMidFightLetsTheChiefSwing is the Exit failure
// Kirk walked into: the letter-holder, whose turn it is, leaves the run —
// dropping the letter in the yard — and the clock moves on to the chief,
// who steps into the yard and strikes the player still there. The departure
// must not leave a half-removed member on the roster the strike reads.
func (s *HoldOutSessionSuite) TestTheActiveHolderExitingMidFightLetsTheChiefSwing() {
	s.startWith(campOptions{withEnding: true, driver: session.Behavior(),
		cast: []*character.Data{stout("alice"), stout("bob")}, spawn: []string{}})
	s.arrangeTheChiefAtTheDoor("bob", "alice")
	s.Require().NoError(s.endTurnOf("alice"))
	s.Require().Equal("bob", s.turn("bob").Active)

	out, err := s.mgr.Exit(context.Background(), &session.ExitInput{Session: campSession, Member: "bob"})
	s.Require().NoError(err, "bob leaves; the chief's driven turn is inside this verb")
	s.Nil(out.Closed, "the letter lies in the yard, unheld; the camp has not turned")

	kinds := s.kinds("alice")
	s.Contains(kinds, session.EventExited)
	s.Contains(kinds, session.EventDropped, "the letter, dropped where bob stood")
	s.NotContains(kinds, session.EventStanceChanged, "nobody carried it to the chief")
	swung := false
	for _, k := range kinds {
		if k == session.EventStruck || k == session.EventMissed {
			swung = true
		}
	}
	s.True(swung, "the chief reached alice and swung: %v", kinds)
	s.Equal(encounter.FactionParty, s.roster()["alice"].Faction)
}
