// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	mind "github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
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

// sighting places a figure and says nothing about their hands: the skeleton
// saw somebody there, not what they were carrying.
func sighting(t *testing.T, at spatial.Position) []byte {
	t.Helper()
	payload, err := encounter.EncodeSightTestimony(encounter.SightTestimony{State: encounter.LocationKnown, Position: at})
	require.NoError(t, err)
	return payload
}

// armed places a figure AND reports what was seen in their hands, which is
// what the grudge now turns on. Item ids, as the rulebook mints them.
func armed(t *testing.T, at spatial.Position, mainHand, offHand string) []byte {
	t.Helper()
	payload, err := encounter.EncodeSightTestimony(encounter.SightTestimony{
		State:     encounter.LocationKnown,
		Position:  at,
		Equipment: &encounter.HeldEquipment{MainHand: mainHand, OffHand: offHand},
	})
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

// view is the skeleton's turn: alice far with a crossbow up and bob adjacent
// with a sword, both seen and both in bowshot; the skeleton has a bow and a
// sword.
func (s *MindedTestSuite) view(at uint64, extra ...perception.Holding) encounter.MonsterView {
	return s.viewSeeing(at, armed(s.T(), aliceAt, weapons.LightCrossbow, ""), extra...)
}

// viewSeeing is [MindedTestSuite.view] with alice seen exactly as the given
// sight payload says — the one thing the walk's finding turns on.
func (s *MindedTestSuite) viewSeeing(at uint64, aliceSight []byte, extra ...perception.Holding) encounter.MonsterView {
	holdings := []perception.Holding{
		{Subject: alice, Payload: aliceSight, Channel: perception.Sight, Observed: 0, Confirmed: at, CurrentVia: []perception.Channel{perception.Sight}},
		{Subject: bob, Payload: armed(s.T(), bobAt, weapons.Longsword, ""), Channel: perception.Sight, Observed: 0, Confirmed: at, CurrentVia: []perception.Channel{perception.Sight}},
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

// The arrow, first half: alice shoots the skeleton from across the room and
// still has the crossbow up. Bob is closer, and the skeleton turns on alice
// anyway.
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

// A grudge fades. The same shot from the same still-armed alice, older than
// patience, no longer ranks her first: the timer is the other half of the
// rule and it still runs.
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

// The walk's finding, and the rule it bought: the fighter shot the skeleton
// with a crossbow, put it away and closed with a longsword — and the
// skeleton went on shooting past a nearer barbarian until the clock ran out.
// A grudge holds only while the bow is up.
func (s *MindedTestSuite) TestTheGrudgeEndsWhenTheBowIsPutAway() {
	v := s.viewSeeing(3, armed(s.T(), aliceAt, weapons.Longsword, ""), shotBy(alice, 3))

	intent, err := s.driver().Act(v)
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: bob, Action: meleeRef}, intent, "bob: alice cannot shoot back any more")
}

// Hands it did not see are not a bow. An empty [encounter.HeldEquipment] is a
// player standing there with nothing in their hands; a nil one is a figure
// whose hands were never observed, and neither is a reason to keep shooting.
func (s *MindedTestSuite) TestHandsItDidNotSeeAreNotABow() {
	v := s.viewSeeing(3, sighting(s.T(), aliceAt), shotBy(alice, 3))

	intent, err := s.driver().Act(v)
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: bob, Action: meleeRef}, intent, "bob: nothing was seen in alice's hands")
}

// A bow in the off hand is still a bow. Both hands are asked, because a hand
// crossbow is held in one.
func (s *MindedTestSuite) TestABowInTheOffHandIsStillABow() {
	v := s.viewSeeing(3, armed(s.T(), aliceAt, "", weapons.HandCrossbow), shotBy(alice, 3))

	intent, err := s.driver().Act(v)
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: alice, Action: bowRef}, intent, "alice: the off hand can shoot too")
}

// What counts as ranged is the driver's to give — the first authoring knob on
// a mind, and really just data. A driver that calls everything ranged keeps
// the grudge on a swordsman, with no change to the Retaliator.
func (s *MindedTestSuite) TestTheRangedKnobIsTheDriversToGive() {
	d, err := behavior.NewMinded(&behavior.NewMindedInput{Ranged: func(string) bool { return true }})
	s.Require().NoError(err)

	v := s.viewSeeing(3, armed(s.T(), aliceAt, weapons.Longsword, ""), shotBy(alice, 3))

	intent, err := d.Act(v)
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: alice, Action: bowRef}, intent, "alice: this driver says a longsword shoots")
}

// A shooter it cannot see any more is still remembered as one. The weapon
// rule asks what is in a shooter's hands, and a ghost has no hands to check —
// so the clock is what remains, and alice still ranks ahead of the live man
// beside it while the clock runs. She is remembered holding a SWORD on
// purpose: what ranks her first is being out of sight, not being armed.
//
// Rank is asked directly because this driver never attacks a ghost — live
// beats remembered, and a remembered figure reads as no creature at all — so
// an intent would hide the ordering rather than show it.
func (s *MindedTestSuite) TestARememberedShooterIsStillRankedFirst() {
	ghost := mind.Contact{
		Bearer: alice, Name: mind.Name(alice), Named: true,
		Holdings: []mind.Holding{
			{
				Holding: perception.Holding{
					Subject: alice, Channel: perception.Sight, Observed: 1, Confirmed: 1,
					Payload: armed(s.T(), aliceAt, weapons.Longsword, ""),
				},
				Reading: mind.Reading{Where: aliceAt.String()},
			},
			{
				Holding: perception.Holding{
					Subject: deed.Subject(alice), Channel: deed.Channel, Observed: 3, Confirmed: 3,
					Payload: deed.Encode(deed.Deed{
						Verb: encounter.DeedAttack, Actor: alice, Target: skeleton, Where: aliceAt.String(),
					}),
				},
			},
		},
	}

	live := mind.Contact{
		Bearer: bob, Name: mind.Name(bob), Named: true,
		Holdings: []mind.Holding{{
			Holding: perception.Holding{
				Subject: bob, Channel: perception.Sight, Observed: 3, Confirmed: 3,
				Payload:    armed(s.T(), bobAt, weapons.Longsword, ""),
				CurrentVia: []perception.Channel{perception.Sight},
			},
			Reading: mind.Reading{Where: bobAt.String(), Creature: true},
		}},
	}

	r := &behavior.Retaliator{
		Space:    flatSpace{aliceAt.String(): 4, bobAt.String(): 1},
		Patience: behavior.DefaultPatience,
	}

	out, err := r.Rank(&mind.RankInput{Situation: mind.Situation{
		Actor:    skeleton,
		At:       3,
		Self:     mind.Self{Where: skeletonAt.String()},
		Contacts: []mind.Contact{live, ghost},
	}})
	s.Require().NoError(err)
	s.Require().Len(out.Ranked, 2)
	s.Equal(core.EntityID(alice), out.Ranked[0].Bearer, "the remembered shooter, ahead of the man in its face")
}

// flatSpace answers distance from a table and nothing else. Rank is the only
// judgment under test above, so the geometry is a fixture rather than a board.
type flatSpace map[string]int

func (f flatSpace) Distance(in *mind.DistanceInput) (*mind.DistanceOutput, error) {
	steps, ok := f[in.To]

	return &mind.DistanceOutput{Steps: steps, Known: ok}, nil
}

func (flatSpace) Toward(*mind.TowardInput) (*mind.TowardOutput, error) {
	return &mind.TowardOutput{}, nil
}

func (flatSpace) Away(*mind.AwayInput) (*mind.AwayOutput, error) {
	return &mind.AwayOutput{}, nil
}
