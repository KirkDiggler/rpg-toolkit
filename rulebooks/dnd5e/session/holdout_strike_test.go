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
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
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

// launchTheChiefAtTheDoor launches the redis state as one board: the holder
// on the yard cell one step beyond the hut's doorway, with the letter lying
// there, the other player one step further along the same line, and the chief
// ON the doorway's hut-side cell — in plain sight of both through the
// doorway, so the launch forms a fight with him in it and his driven turn
// steps him into the yard. The holder then picks the letter up, on her own
// turn when it is hers, so the scene opens with it in hand.
//
// The old scene reached the same board through two walks, a Hold at the gate
// and a mid-run Spawn. Spawn is gone and no `arrives` form can bring a monster
// into a free-roam run after a walk (a round is a fight's clock, the camp's one
// fact is the one that turns it, nobody falls), so the board the walks
// produced is placed directly, and the letter lies where the holder carried
// it. The camp's own letter placement is moved on a copy of the field.
func (s *HoldOutSessionSuite) launchTheChiefAtTheDoor(opts campOptions, holder, other string) {
	s.T().Helper()
	in := s.prepare(opts)

	atlas, err := s.mgr.AtlasOf(context.Background(), &session.AtlasOfInput{Dungeon: in.Dungeon, DungeonKey: campKey})
	s.Require().NoError(err)
	var near, far spatial.Position
	found := false
	for _, dw := range atlas.Doorways {
		if dw.Door != yardHut {
			continue
		}
		near, far, found = dw.To, dw.From, true
		if regionOf(atlas, dw.From) == "yard" {
			near, far = dw.From, dw.To
		}
	}
	s.Require().True(found, "%s is not in the atlas", yardHut)
	step := spatial.Position{X: near.X - far.X, Y: near.Y - far.Y}
	beyond := spatial.Position{X: near.X + step.X, Y: near.Y + step.Y}
	further := spatial.Position{X: beyond.X + step.X, Y: beyond.Y + step.Y}

	seats := map[string]spatial.Position{holder: authoredOf(beyond), other: authoredOf(further)}
	in.Dungeon.PartyStart = []dungeonspec.Seat{{At: seats[in.Party[0]]}, {At: seats[in.Party[1]]}}

	props := append([]encounter.PropInput(nil), in.Dungeon.Field.Props...)
	moved := false
	for i := range props {
		if props[i].ID == campLetter {
			props[i].At, moved = authoredOf(beyond), true
		}
	}
	s.Require().True(moved, "the camp places no %s", campLetter)
	in.Dungeon.Field.Props = props

	chief := s.placement(campChief)
	chief.At = authoredOf(far)
	in.Dungeon.Monsters = append(in.Dungeon.Monsters, chief)

	out, err := s.mgr.Launch(context.Background(), in)
	s.Require().NoError(err)
	var formed *session.Formed
	for _, f := range out.Formed {
		if slices.Contains(f.Order, campChief) {
			formed = f
		}
	}
	s.Require().NotNil(formed, "the chief at the door sees the party in the yard")

	if s.turn(holder).Active == holder {
		s.hold(holder, campLetter)
	}
	s.stream.published = nil
}

// closingDriver is the fixture these three scenes drive with: strike an
// OPPOSED sighting you can reach, else walk toward it, else stand there.
//
// WRITTEN HERE RATHER THAN TAKEN OFF THE SHELF, for the reason
// [pursuingDriver] is (monster_turn_test.go). These tests are about the stance
// flip, the dissolution it causes, and the roster a strike reads after
// somebody Exits mid-fight — not about which policy the shipped content holds.
// They used to lean on the retired `behavior.Basic` for their walking, and the
// creature's table is rolled on the composition's own view rather than this
// twin, so it cannot be wired here at all (rpg-project#465).
//
// IT ASKS WHETHER A SIGHTING IS OPPOSED, which is what makes
// TestAChiefWhoseCampTurnedDoesNotSwing a real test rather than a coincidence:
// the camp turning does not move alice or make her invisible, it changes whose
// side she is on, and the driver has to be reading THAT to stop. Opposition is
// the stance graph's answer projected onto the view; a driver reading Kind
// instead would swing at her forever.
type closingDriver struct{}

func (closingDriver) Act(view session.MonsterView) (session.TurnIntent, error) {
	var target *session.SeenMember
	for i, seen := range view.Seen {
		if !seen.Opposed || !seen.Standing {
			continue
		}
		if target == nil || seen.DistanceCells < target.DistanceCells {
			target = &view.Seen[i]
		}
	}
	if target == nil {
		return session.Pass{}, nil
	}

	for _, action := range view.Actions {
		if target.InReach[action.Ref] {
			return session.Attack{Target: target.ID, Action: action.Ref}, nil
		}
	}
	if len(target.Path) > 0 {
		return session.Move{Path: target.Path}, nil
	}

	return session.Pass{}, nil
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
	s.launchTheChiefAtTheDoor(campOptions{withEnding: true, driver: closingDriver{},
		cast: []*character.Data{stout("alice"), stout("bob")}, spawn: []string{}}, "alice", "bob")

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
	s.launchTheChiefAtTheDoor(campOptions{withEnding: false, driver: closingDriver{},
		cast: []*character.Data{stout("alice"), stout("bob")}, spawn: []string{}}, "alice", "bob")

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
	s.launchTheChiefAtTheDoor(campOptions{withEnding: true, driver: closingDriver{},
		cast: []*character.Data{stout("alice"), stout("bob")}, spawn: []string{}}, "bob", "alice")
	s.Require().NoError(s.endTurnOf("alice"))
	s.Require().Equal("bob", s.turn("bob").Active)
	s.hold("bob", campLetter) // the letter in hand on his own turn, before he leaves

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
