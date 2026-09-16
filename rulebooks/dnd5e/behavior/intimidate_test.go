// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

// intimidate_test.go is what a threat is WORTH to each of the three words
// (rpg-project#454, ideas/shenanigans/intimidate.md). The deed is the
// encounter's; this is the mind reading it.

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	mind "github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type FearTestSuite struct {
	suite.Suite
}

func TestFearSuite(t *testing.T) {
	suite.Run(t, new(FearTestSuite))
}

// threatenedBy is the deed the encounter lands when somebody beats an
// Intimidate check against the skeleton, from alice's spot.
func threatenedBy(who encounter.MemberID, at uint64, where spatial.Position) perception.Holding {
	return perception.Holding{
		Subject: deed.Subject(who), Channel: deed.Channel, Observed: at, Confirmed: at,
		Payload: deed.Encode(deed.Deed{
			Verb: encounter.DeedIntimidate, Actor: who, Target: skeleton, Where: where.String(),
		}),
	}
}

// alone is a coward's turn with ONE player in it: alice, four cells off with
// a bow trained on her, and a step away available. Nobody is adjacent, so
// the coward's own two-step room never fires and anything that happens is
// the threat's doing.
func (s *FearTestSuite) alone(word string, at uint64, extra ...perception.Holding) encounter.MonsterView {
	holdings := []perception.Holding{{
		Subject: alice, Payload: armed(s.T(), aliceAt, weapons.LightCrossbow, ""),
		Channel: perception.Sight, Observed: 0, Confirmed: at,
		CurrentVia: []perception.Channel{perception.Sight},
	}}

	return encounter.MonsterView{
		Self:     skeleton,
		Position: skeletonAt,
		Mind:     word,
		Actions:  []encounter.ActionView{{Ref: meleeRef, RangeFeet: 5}, {Ref: bowRef, RangeFeet: 80}},
		Holdings: append(holdings, extra...),
		At:       at,
		Seen: []encounter.SeenMember{
			seenPlayer(alice, aliceAt, 4, []spatial.Position{{X: 6, Y: 3}}, []spatial.Position{{X: 6, Y: 5}}),
		},
		Budget: encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
	}
}

// crowded is [FearTestSuite.alone] with bob standing on the monster, sword
// up: the scene where "who does it go for" is a real question, because
// somebody nearer is always available.
func (s *FearTestSuite) crowded(word string, at uint64, extra ...perception.Holding) encounter.MonsterView {
	v := s.alone(word, at, extra...)
	v.Holdings = append(v.Holdings, perception.Holding{
		Subject: bob, Payload: armed(s.T(), bobAt, weapons.Longsword, ""),
		Channel: perception.Sight, Observed: 0, Confirmed: at,
		CurrentVia: []perception.Channel{perception.Sight},
	})
	v.Seen = append(v.Seen, seenPlayer(bob, bobAt, 1, nil, []spatial.Position{{X: 7, Y: 4}}))

	return v
}

func (s *FearTestSuite) driver() *behavior.Minded {
	d, err := behavior.NewMinded(nil)
	s.Require().NoError(err)
	return d
}

// (a) A cowed coward runs. Alice is four cells away and squarely in bowshot,
// so it could shoot her; it steps away instead, and the step is the
// encounter's own precomputed one.
func (s *FearTestSuite) TestACowedCowardRunsInsteadOfShooting() {
	intent, err := s.driver().Act(s.alone(behavior.MindCoward, 3, threatenedBy(alice, 3, aliceAt)))
	s.Require().NoError(err)
	s.Equal(encounter.Move{Path: []spatial.Position{{X: 6, Y: 5}}}, intent,
		"away from her, one cell, the step the encounter handed it")
}

// The same coward, unthreatened, shoots. This is the control: everything
// about the scene except the deed is identical.
func (s *FearTestSuite) TestAnUnthreatenedCowardShoots() {
	intent, err := s.driver().Act(s.alone(behavior.MindCoward, 3))
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: alice, Action: bowRef}, intent)
}

// (b) Out of sight, it stops running from her. Rung 0 only ever considers a
// CREATURE, and a creature is a figure some current holding reads as one —
// so the moment she is a memory the fear has nobody to be about, and the
// goblin goes back to what it would do about a remembered figure.
func (s *FearTestSuite) TestItStopsWhenSheIsOutOfSight() {
	v := s.alone(behavior.MindCoward, 3, threatenedBy(alice, 3, aliceAt))
	v.Holdings[0].CurrentVia = nil
	v.Seen = nil
	v.Remembered = []encounter.RememberedMember{{
		ID: alice, Kind: encounter.KindPlayer, Position: aliceAt,
		DistanceCells: 4, Path: []spatial.Position{{X: 6, Y: 3}},
	}}

	intent, err := s.driver().Act(v)
	s.Require().NoError(err)
	s.NotEqual(encounter.Move{Path: []spatial.Position{{X: 6, Y: 5}}}, intent,
		"it is not fleeing a memory")
	s.Equal(encounter.Move{Path: []spatial.Position{{X: 6, Y: 3}}}, intent,
		"a ghost is never fled and never attacked, so the ladder walks toward it")
}

// (c) Fear runs out. The same threat, three ticks on, and the coward has its
// bow up again.
func (s *FearTestSuite) TestFearWearsOff() {
	s.Run("still working on the last tick", func() {
		intent, err := s.driver().Act(s.alone(behavior.MindCoward, 5, threatenedBy(alice, 3, aliceAt)))
		s.Require().NoError(err)
		s.Equal(encounter.Move{Path: []spatial.Position{{X: 6, Y: 5}}}, intent)
	})
	s.Run("spent on the next", func() {
		intent, err := s.driver().Act(s.alone(behavior.MindCoward, 6, threatenedBy(alice, 3, aliceAt)))
		s.Require().NoError(err)
		s.Equal(encounter.Attack{Target: alice, Action: bowRef}, intent,
			"patience 3 answers the tick it lands and the two after, and no more")
	})
}

// The room STAYS. Intimidation adds fear on top of the coward's two-step
// flinch rather than replacing it, so an unthreatened coward still backs
// away from whoever closes on it.
func (s *FearTestSuite) TestTheCowardStillFlinches() {
	intent, err := s.driver().Act(s.crowded(behavior.MindCoward, 3))
	s.Require().NoError(err)
	s.Equal(encounter.Move{Path: []spatial.Position{{X: 7, Y: 4}}}, intent,
		"bob is inside its two steps of room, and nobody threatened anybody")
}

// (d) The berserker takes it personally. Alice threatened it from across the
// room; bob is standing on it with a sword. It goes for alice.
func (s *FearTestSuite) TestTheBerserkerChargesWhoeverThreatenedIt() {
	intent, err := s.driver().Act(s.crowded(behavior.MindBerserker, 3, threatenedBy(alice, 3, aliceAt)))
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: alice, Action: bowRef}, intent,
		"a threat is a provocation to this one, and bob is merely closer")
}

// (e) The retaliator shrugs. The identical scene, one word different, and
// its ranking is exactly what it would have been with no deed at all: its
// excuse is about hands, and a threat is not a swing.
func (s *FearTestSuite) TestTheRetaliatorIsUnmovedByAThreat() {
	withThreat, err := s.driver().Act(s.crowded(behavior.MindRetaliator, 3, threatenedBy(alice, 3, aliceAt)))
	s.Require().NoError(err)
	withNothing, err := s.driver().Act(s.crowded(behavior.MindRetaliator, 3))
	s.Require().NoError(err)

	s.Equal(encounter.Attack{Target: bob, Action: meleeRef}, withThreat)
	s.Equal(withNothing, withThreat, "the deed changed nothing about what it does")
}

// (f) A mind with no Fear is unchanged by the deed: the zero value tells the
// truth. Built by hand, because no word the rulebook ships is a coward
// without fear.
func (s *FearTestSuite) TestAMindWithNoFearIsNeverCowed() {
	threatened := mind.Contact{
		Holdings: []mind.Holding{{
			Holding: perception.Holding{
				Subject: deed.Subject(alice), Channel: deed.Channel, Confirmed: 3,
				Payload: deed.Encode(deed.Deed{
					Verb: encounter.DeedIntimidate, Actor: alice, Target: skeleton, Where: aliceAt.String(),
				}),
			},
		}},
		Name: mind.Name(alice), Named: true, Bearer: alice,
	}
	situation := mind.Situation{Actor: skeleton, At: 3, Self: mind.Self{Where: skeletonAt.String()}}

	fearless := &behavior.Retaliator{Room: 2}
	out, err := fearless.Keep(&mind.KeepInput{Situation: situation, Contact: threatened})
	s.Require().NoError(err)
	s.Equal(2, out.Steps, "it holds the threat and keeps exactly the room it always kept")

	cowed := &behavior.Retaliator{Room: 2, Fear: behavior.Fear{Patience: 3}}
	out, err = cowed.Keep(&mind.KeepInput{Situation: situation, Contact: threatened})
	s.Require().NoError(err)
	s.Greater(out.Steps, 2, "and the same contact, read by a mind that can be frightened, is every step there is")
}

// A threat against somebody else is not a threat against me — the same rule
// the grudge keeps, asked of fear.
func (s *FearTestSuite) TestAThreatAgainstSomebodyElseIsNotMine() {
	threat := threatenedBy(alice, 3, aliceAt)
	threat.Payload = deed.Encode(deed.Deed{
		Verb: encounter.DeedIntimidate, Actor: alice, Target: bob, Where: aliceAt.String(),
	})

	intent, err := s.driver().Act(s.alone(behavior.MindCoward, 3, threat))
	s.Require().NoError(err)
	s.Equal(encounter.Attack{Target: alice, Action: bowRef}, intent)
}

// Judge reads no verb, and it had to stop: a coward's grudge is the zero, so
// a verb-filtered Judge would have left the threat unattached — a deeds
// handle floating beside the figure instead of on her — and Keep would be
// asked about a contact holding no memory of the one thing that happened.
func (s *FearTestSuite) TestJudgeAttachesAThreatToTheFigureWhoMadeIt() {
	r := &behavior.Retaliator{}
	out, err := r.Judge(&mind.JudgeInput{Holdings: []mind.Holding{
		{Holding: perception.Holding{Subject: alice, Channel: perception.Sight}},
		{Holding: perception.Holding{
			Subject: deed.Subject(alice), Channel: deed.Channel,
			Payload: deed.Encode(deed.Deed{
				Verb: encounter.DeedIntimidate, Actor: alice, Target: skeleton, Where: aliceAt.String(),
			}),
		}},
	}})
	s.Require().NoError(err)
	s.Equal([]mind.Pair{{A: core.EntityID(deed.Subject(alice)), B: alice}}, out.Same)
}
