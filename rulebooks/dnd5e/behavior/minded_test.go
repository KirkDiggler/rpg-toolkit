// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// The bow skeleton's scene, as a driver sees it: the view the encounter
// would hand it, and nothing else. Every proof here is #1725's arrow at
// the driver seam.

var bowRef = core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "shortbow"}

const (
	skeleton = encounter.MemberID("skeleton")
	alice    = encounter.MemberID("alice")
	bob      = encounter.MemberID("bob")
)

var (
	skeletonAt = spatial.Position{X: 6, Y: 4}
	aliceAt    = spatial.Position{X: 6, Y: 0} // far, with a bow
	bobAt      = spatial.Position{X: 5, Y: 4} // next to the skeleton
)

type MindedTestSuite struct {
	suite.Suite
}

func TestMindedSuite(t *testing.T) {
	suite.Run(t, new(MindedTestSuite))
}

func sighting(t *testing.T, at spatial.Position) []byte {
	t.Helper()
	payload, err := encounter.EncodeSightTestimony(encounter.SightTestimony{State: encounter.LocationKnown, Position: at})
	require.NoError(t, err)
	return payload
}

func seenPlayer(id encounter.MemberID, at spatial.Position, dist float64, path, away []spatial.Position) encounter.SeenMember {
	return encounter.SeenMember{
		ID: id, Kind: encounter.KindPlayer, Standing: true, Position: at, DistanceCells: dist,
		InReach:  map[core.Ref]bool{bowRef: true, meleeRef: dist <= 1},
		Path:     path,
		AwayPath: away,
	}
}

// view is the skeleton's turn: alice far and bob adjacent, both seen and
// both in bowshot; the skeleton has a bow and a sword.
func (s *MindedTestSuite) view(at uint64, extra ...perception.Holding) encounter.MonsterView {
	holdings := []perception.Holding{
		{Subject: alice, Payload: sighting(s.T(), aliceAt), Channel: perception.Sight, Observed: 0, Confirmed: at, CurrentVia: []perception.Channel{perception.Sight}},
		{Subject: bob, Payload: sighting(s.T(), bobAt), Channel: perception.Sight, Observed: 0, Confirmed: at, CurrentVia: []perception.Channel{perception.Sight}},
	}
	holdings = append(holdings, extra...)

	return encounter.MonsterView{
		Self:     skeleton,
		Position: skeletonAt,
		Mind:     behavior.MindRetaliator,
		Actions: []encounter.ActionView{
			{Ref: meleeRef, RangeFeet: 5},
			{Ref: bowRef, RangeFeet: 80},
		},
		Holdings: holdings,
		At:       at,
		Seen: []encounter.SeenMember{
			seenPlayer(alice, aliceAt, 4, []spatial.Position{{X: 6, Y: 3}}, []spatial.Position{{X: 6, Y: 5}}),
			seenPlayer(bob, bobAt, 1, nil, []spatial.Position{{X: 7, Y: 4}}),
		},
		Budget: encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
	}
}

// shotBy is the deed the encounter lands when somebody attacks the
// skeleton where it can see them.
func shotBy(who encounter.MemberID, at uint64) perception.Holding {
	return perception.Holding{
		Subject: deed.Subject(who), Channel: deed.Channel, Observed: at, Confirmed: at,
		Payload: deed.Encode(deed.Deed{Verb: encounter.DeedAttack, Actor: who, Target: skeleton, Where: aliceAt.String()}),
	}
}

func (s *MindedTestSuite) driver() *behavior.Minded {
	d, err := behavior.NewMinded(nil)
	s.Require().NoError(err)
	return d
}

// The arrow, first half: alice shoots the skeleton from across the room.
// Bob is closer, and the skeleton turns on alice anyway.
func (s *MindedTestSuite) TestItTurnsOnWhoeverShotIt() {
	intent, err := s.driver().Act(s.view(3, shotBy(alice, 3)))
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: alice, Action: bowRef}, intent, "at alice, with the action that reaches her")
}

// The arrow, second half: nobody has shot it lately, so it goes for the
// closest — bob, next to it.
func (s *MindedTestSuite) TestItGoesBackToClosest() {
	intent, err := s.driver().Act(s.view(3))
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: bob, Action: meleeRef}, intent)
}

// A grudge fades. The same shot, older than patience, no longer ranks
// alice first.
func (s *MindedTestSuite) TestAGrudgeFades() {
	intent, err := s.driver().Act(s.view(10, shotBy(alice, 3)))
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: bob, Action: meleeRef}, intent, "bob again: the shot was an age ago")
}

// A shot at somebody else is not a grudge.
func (s *MindedTestSuite) TestAShotAtSomebodyElseIsNotAGrudge() {
	shot := shotBy(alice, 3)
	shot.Payload = deed.Encode(deed.Deed{Verb: encounter.DeedAttack, Actor: alice, Target: bob, Where: aliceAt.String()})

	intent, err := s.driver().Act(s.view(3, shot))
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: bob, Action: meleeRef}, intent)
}

// Out of reach with nobody nearer, it walks: the encounter's own first step
// toward the target. (With bob adjacent it would swing at bob first — the
// ladder attacks what is in reach before it walks toward what it prefers,
// and a grudge only orders the ranking.)
func (s *MindedTestSuite) TestOutOfReachItWalksTheEncountersStep() {
	v := s.view(3, shotBy(alice, 3))
	v.Actions = []encounter.ActionView{{Ref: meleeRef, RangeFeet: 5}}
	v.Seen = v.Seen[:1]
	v.Seen[0].InReach = map[core.Ref]bool{meleeRef: false}

	intent, err := s.driver().Act(v)
	s.Require().NoError(err)
	s.Equal(encounter.Move{Path: []spatial.Position{{X: 6, Y: 3}}}, intent, "toward alice, one cell")
}

// A member whose sheet names no mind is driven exactly as Basic drives it.
func (s *MindedTestSuite) TestNoMindIsBasic() {
	v := s.view(3, shotBy(alice, 3))
	v.Mind = ""

	minded, err := s.driver().Act(v)
	s.Require().NoError(err)
	basic, err := (behavior.Basic{}).Act(v)
	s.Require().NoError(err)
	s.Equal(basic, minded)
	s.Equal(encounter.Attack{Target: bob, Action: meleeRef}, basic, "closest, grudge or no grudge")
}

// A mind the driver has never heard of is a wiring fault.
func (s *MindedTestSuite) TestAnUnknownMindFailsLoudly() {
	v := s.view(3)
	v.Mind = "genius"

	_, err := s.driver().Act(v)
	s.Require().ErrorIs(err, behavior.ErrUnknownMind)
}

// Names persist: the same driver, two turns, the same word for alice.
func (s *MindedTestSuite) TestTheGrudgeSurvivesATurn() {
	d := s.driver()

	first, err := d.Act(s.view(3, shotBy(alice, 3)))
	s.Require().NoError(err)
	second, err := d.Act(s.view(4, shotBy(alice, 3)))
	s.Require().NoError(err)
	s.Equal(first, second, "still alice, one tick later")
}
