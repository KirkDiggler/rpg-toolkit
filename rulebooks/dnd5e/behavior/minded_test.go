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

// handGrudge is the grudge a test spells out when it builds a Retaliator
// directly rather than taking one from a word: the bow skeleton's own rule,
// and long enough to cover every clock in this file.
//
// It names the verb it answers, because since rpg-project#454 a grudge that
// names none answers nothing — an omission here used to be invisible and is
// now a fixture that provokes nobody.
var handGrudge = behavior.Grudge{
	Patience: 3, Excuse: behavior.ExcuseUnarmed,
	Provokes: []string{encounter.DeedAttack},
}

const (
	skeleton = encounter.MemberID("skeleton")
	alice    = encounter.MemberID("alice")
	bob      = encounter.MemberID("bob")
	// zara is alice with one property the others lack: an id that sorts
	// AFTER "deeds|", so a bundle of her sighting and her deed puts the
	// deed's handle first. Every id in this suite used to sort before it,
	// which is why the suite could not see the ghost bug.
	zara = encounter.MemberID("zara")
)

var (
	skeletonAt = spatial.Position{X: 6, Y: 4}
	aliceAt    = spatial.Position{X: 6, Y: 0} // far, with a bow
	bobAt      = spatial.Position{X: 5, Y: 4} // next to the skeleton
	zaraAt     = spatial.Position{X: 0, Y: 4} // across the room, with a bow
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
// skeleton where it can see them, from alice's spot.
func shotBy(who encounter.MemberID, at uint64) perception.Holding {
	return shotFrom(who, at, aliceAt)
}

// shotFrom is shotBy for a shooter standing somewhere else. The place is the
// deed's own and not decoration: a contact takes its place from its freshest
// placed holding, so a deed filed where its actor never stood would move the
// figure it is bundled with.
func shotFrom(who encounter.MemberID, at uint64, where spatial.Position) perception.Holding {
	return perception.Holding{
		Subject: deed.Subject(who), Channel: deed.Channel, Observed: at, Confirmed: at,
		Payload: deed.Encode(deed.Deed{Verb: encounter.DeedAttack, Actor: who, Target: skeleton, Where: where.String()}),
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
		Space:  flatSpace{aliceAt.String(): 4, bobAt.String(): 1},
		Grudge: handGrudge,
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

// zaraView is the reviewer's own scene: one shooter, the crossbow she shot
// with, and the deed she left behind. current says whether the skeleton can
// see her this turn — false is the turn she spends as a ghost, true is her
// return to the open.
func (s *MindedTestSuite) zaraView(at uint64, current bool) encounter.MonsterView {
	sight := perception.Holding{
		Subject: zara, Channel: perception.Sight, Observed: 3, Confirmed: 3,
		Payload: armed(s.T(), zaraAt, weapons.LightCrossbow, ""),
	}

	var seen []encounter.SeenMember

	if current {
		sight.Confirmed = at
		sight.CurrentVia = []perception.Channel{perception.Sight}
		seen = []encounter.SeenMember{{
			ID: zara, Kind: encounter.KindPlayer, Standing: true, Position: zaraAt, DistanceCells: 6,
			InReach: map[core.Ref]bool{bowRef: true, meleeRef: false},
		}}
	}

	return encounter.MonsterView{
		Self:     skeleton,
		Position: skeletonAt,
		Mind:     behavior.MindRetaliator,
		Actions: []encounter.ActionView{
			{Ref: meleeRef, RangeFeet: 5},
			{Ref: bowRef, RangeFeet: 80},
		},
		Holdings: []perception.Holding{sight, shotFrom(zara, 3, zaraAt)},
		At:       at,
		Seen:     seen,
		Budget:   encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
	}
}

// The ghost's return. The shooter was seen at the shot and gone by the time
// the skeleton's turn came, so the first word it ever has for her is spoken
// over a bundle whose sorted-first holding is the deed's own handle. When she
// steps back into the open with the crossbow still up, in bowshot and inside
// the grudge, the skeleton has to be able to aim at her.
//
// It can only do that by the plain member id. A name or a target taken from
// the handle the store files deeds under matches no member the encounter ever
// offers, and the skeleton stands still while its prey shoots it.
func (s *MindedTestSuite) TestAShooterFirstMetAsAGhostIsStillAttackable() {
	s.Require().Greater(string(zara), string(deed.Subject(zara)),
		"this proof only bites while the member id sorts after its own deed handle")

	d := s.driver()

	first, err := d.Act(s.zaraView(3, false))
	s.Require().NoError(err)
	s.Require().Equal(encounter.Pass{}, first, "a memory is no target, and the encounter offers no step toward one")

	second, err := d.Act(s.zaraView(4, true))
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: zara, Action: bowRef}, second, "back in the open, crossbow up, grudge fresh")
}

// What a mind calls a contact is a member id or nothing. The deeds handle is
// the store's filing system, not testimony, and deed's own rule says a mind
// does not read handles — so a contact it cannot put an id to is a contact it
// has no word for yet, which the ladder then declines to aim at.
func (s *MindedTestSuite) TestAContactIsNamedByItsMemberId() {
	sight := mind.Holding{Holding: perception.Holding{
		Subject: zara, Channel: perception.Sight, Observed: 3, Confirmed: 3,
		Payload: armed(s.T(), zaraAt, weapons.LightCrossbow, ""),
	}}

	shot := func(actor encounter.MemberID) mind.Holding {
		h := shotFrom(zara, 3, zaraAt)
		h.Payload = deed.Encode(deed.Deed{
			Verb: encounter.DeedAttack, Actor: actor, Target: skeleton, Where: zaraAt.String(),
		})

		return mind.Holding{Holding: h}
	}

	cases := []struct {
		scene    string
		holdings []mind.Holding
		want     mind.Name
		named    bool
	}{
		{
			scene: "a ghost and the deed bundled onto her: the handle sorts first",
			// The fold's own order, which is what Name is handed.
			holdings: []mind.Holding{shot(zara), sight},
			want:     mind.Name(zara),
			named:    true,
		},
		{
			scene:    "a deed alone, from somebody it was holding at the time",
			holdings: []mind.Holding{shot(zara)},
			want:     mind.Name(zara),
			named:    true,
		},
		{
			scene:    "a deed alone, from nobody it could see",
			holdings: []mind.Holding{shot("")},
			named:    false,
		},
	}

	r := &behavior.Retaliator{Space: flatSpace{}}

	for _, tc := range cases {
		s.Run(tc.scene, func() {
			out, err := r.Name(&mind.NameInput{Contact: mind.Contact{Holdings: tc.holdings}})
			s.Require().NoError(err)
			s.Equal(tc.named, out.Named)
			s.Equal(tc.want, out.Name)
		})
	}
}

// skittish is an authored mind: the Retaliator with one judgment changed. It
// is this test's mind and not the rulebook's, which is the point — it comes
// in through the driver's door, the way a game's own mind would.
//
// Keep is the changed judgment, and the only thing that reaches the ladder's
// rung 0. Two cells, so a figure standing next to it is nearer than it keeps.
type skittish struct {
	*behavior.Retaliator
}

// Keep says how much room it wants. The Retaliator keeps none.
func (skittish) Keep(*mind.KeepInput) (*mind.KeepOutput, error) {
	return &mind.KeepOutput{Steps: 2}, nil
}

// skittishDriver is a driver that answers the given word with the keeping
// mind, and is otherwise the rulebook's own.
func (s *MindedTestSuite) skittishDriver(word string) *behavior.Minded {
	d, err := behavior.NewMinded(&behavior.NewMindedInput{
		Minds: map[string]mind.Mind{word: skittish{&behavior.Retaliator{
			Space:  flatSpace{bobAt.String(): 1},
			Grudge: handGrudge,
		}}},
	})
	s.Require().NoError(err)

	return d
}

// cornered is one live player standing next to the skeleton and nobody else:
// the scene a mind that wants room has to answer. away is the step the
// encounter precomputed, and nil is the corner — the encounter is what knows
// where the walls are, and the driver never decides that for itself.
func (s *MindedTestSuite) cornered(mindName string, away []spatial.Position) encounter.MonsterView {
	return encounter.MonsterView{
		Self:     skeleton,
		Position: skeletonAt,
		Mind:     mindName,
		Actions: []encounter.ActionView{
			{Ref: meleeRef, RangeFeet: 5},
			{Ref: bowRef, RangeFeet: 80},
		},
		Holdings: []perception.Holding{{
			Subject: bob, Channel: perception.Sight, Observed: 3, Confirmed: 3,
			Payload:    armed(s.T(), bobAt, weapons.Longsword, ""),
			CurrentVia: []perception.Channel{perception.Sight},
		}},
		At:     3,
		Seen:   []encounter.SeenMember{seenPlayer(bob, bobAt, 1, nil, away)},
		Budget: encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
	}
}

// A mind that keeps its distance steps away, and the step is the encounter's
// own. This is the ladder's rung 0 reaching the driver's Away arm, which no
// mind the rulebook ships can reach: the Retaliator keeps nothing.
func (s *MindedTestSuite) TestAnAuthoredMindStepsAway() {
	away := []spatial.Position{{X: 7, Y: 4}}

	intent, err := s.skittishDriver("skittish").Act(s.cornered("skittish", away))
	s.Require().NoError(err)
	s.Equal(encounter.Move{Path: away}, intent, "one cell away from bob, the way the encounter laid it out")
}

// Cornered, the same mind fights. Rung 0 asks the encounter for a step away,
// the encounter has none, and keeping range is a preference rather than a
// compulsion — so it falls to rung 1 and swings at what it could not avoid.
func (s *MindedTestSuite) TestAnAuthoredMindWithNowhereToGoFights() {
	intent, err := s.skittishDriver("skittish").Act(s.cornered("skittish", nil))
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: bob, Action: meleeRef}, intent, "no way out: it swings")
}

// An authored mind adds a word. The ones the rulebook ships still answer on
// the same driver.
func (s *MindedTestSuite) TestAnAuthoredMindDoesNotDisplaceTheBuiltIns() {
	intent, err := s.skittishDriver("skittish").Act(s.view(3, shotBy(alice, 3)))
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: alice, Action: bowRef}, intent, "still the rulebook's retaliator")
}

// An authored mind under a built-in's word wins it: a game tries its own
// retaliator without waiting for a rulebook release.
func (s *MindedTestSuite) TestAnAuthoredMindOverridesABuiltInWord() {
	away := []spatial.Position{{X: 7, Y: 4}}
	d := s.skittishDriver(behavior.MindRetaliator)

	intent, err := d.Act(s.cornered(behavior.MindRetaliator, away))
	s.Require().NoError(err)
	s.Equal(encounter.Move{Path: away}, intent, "the caller's mind, under the rulebook's word")
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
