// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"slices"

	mind "github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// The scene the three words answer (rpg-toolkit#1745).
//
// ONE fixture, three minds, and the body never changes: the member is
// called skeleton in all of it because what is under test is the word on
// its sheet, not the monster wearing it. Alice shot it with a crossbow from
// across the room two ticks ago, has since put the crossbow away, drawn a
// longsword and closed to three cells. Bob has stood two cells off the
// whole time and has attacked nobody. The bow reaches both of them and the
// blade reaches neither.
//
// The scene is chosen so the three words cannot agree by accident: a mind
// that answers the shot picks alice, one that does not picks bob, and one
// that wants room picks neither and steps back. Three derivations put the
// crossbow back in alice's hands, bring bob into touching distance, and
// land the shot on the tick being answered — each named for the one thing
// it changes, and each copying the slices it writes through, since a
// MonsterView is a struct of slices and a derivation that edited in place
// would rewrite the scene for every test after it.

var (
	// aliceClosedAt is three cells out: she closed, and is still farther
	// than the bystander, which is what makes the grudge visible.
	aliceClosedAt = spatial.Position{X: 6, Y: 1}
	// bobBackAt is two cells out — nearer than the shooter, and exactly
	// far enough that a coward's room is not disturbed.
	bobBackAt = spatial.Position{X: 6, Y: 2}
	// bobAwayStep is the one cell the encounter precomputed for backing
	// away from bob. The driver never decides where the walls are.
	bobAwayStep = []spatial.Position{{X: 7, Y: 4}}
)

const (
	// theShot is the tick alice pulled the trigger.
	theShot uint64 = 3
	// theTurn is the tick the skeleton gets to answer, two ticks later — a
	// deed every profile here still counts as fresh, so what separates
	// their answers is never the clock.
	theTurn uint64 = 5
)

// theSwap is the scene above, as the encounter would hand it to a driver.
func (s *MindedTestSuite) theSwap(word string) encounter.MonsterView {
	return encounter.MonsterView{
		Self:     skeleton,
		Position: skeletonAt,
		Mind:     word,
		Actions: []encounter.ActionView{
			{Ref: meleeRef, RangeFeet: 5},
			{Ref: bowRef, RangeFeet: 80},
		},
		Holdings: []perception.Holding{
			{
				Subject: alice, Payload: armed(s.T(), aliceClosedAt, weapons.Longsword, ""),
				Channel: perception.Sight, Observed: 0, Confirmed: theTurn,
				CurrentVia: []perception.Channel{perception.Sight},
			},
			{
				Subject: bob, Payload: armed(s.T(), bobBackAt, weapons.Longsword, ""),
				Channel: perception.Sight, Observed: 0, Confirmed: theTurn,
				CurrentVia: []perception.Channel{perception.Sight},
			},
			shotFrom(alice, theShot, aliceAt),
		},
		At: theTurn,
		Seen: []encounter.SeenMember{
			seenPlayer(alice, aliceClosedAt, 3, []spatial.Position{{X: 6, Y: 3}}, []spatial.Position{{X: 6, Y: 5}}),
			seenPlayer(bob, bobBackAt, 2, []spatial.Position{{X: 6, Y: 3}}, bobAwayStep),
		},
		Budget: encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
	}
}

// stillArmed is theSwap with one thing changed: alice never put the
// crossbow away. It is the control for the grudge ITSELF rather than for
// the excuse — with a shooter nothing could excuse, a mind that holds a
// grudge and a mind that holds none finally choose different people.
func (s *MindedTestSuite) stillArmed(v encounter.MonsterView) encounter.MonsterView {
	v.Holdings = slices.Clone(v.Holdings)

	for i := range v.Holdings {
		if v.Holdings[i].Subject == alice && v.Holdings[i].Channel == perception.Sight {
			v.Holdings[i].Payload = armed(s.T(), aliceClosedAt, weapons.LightCrossbow, "")
		}
	}

	return v
}

// crowded is theSwap with one thing changed: bob steps up to touching
// distance. That is inside a coward's room and nothing at all to the other
// two, so it is the control for Room.
func (s *MindedTestSuite) crowded(v encounter.MonsterView) encounter.MonsterView {
	v.Holdings, v.Seen = slices.Clone(v.Holdings), slices.Clone(v.Seen)

	for i := range v.Holdings {
		if v.Holdings[i].Subject == bob && v.Holdings[i].Channel == perception.Sight {
			v.Holdings[i].Payload = armed(s.T(), bobAt, weapons.Longsword, "")
		}
	}

	for i := range v.Seen {
		if v.Seen[i].ID == bob {
			v.Seen[i] = seenPlayer(bob, bobAt, 1, nil, bobAwayStep)
		}
	}

	return v
}

// justShot is theSwap with one thing changed: the shot lands on the tick
// the skeleton is answering. A deed of age zero is the youngest a deed can
// be, and the one a Patience of zero must still refuse.
func (s *MindedTestSuite) justShot(v encounter.MonsterView) encounter.MonsterView {
	v.Holdings = slices.Clone(v.Holdings)

	for i := range v.Holdings {
		if v.Holdings[i].Channel == deed.Channel {
			v.Holdings[i].Observed = v.At
			v.Holdings[i].Confirmed = v.At
		}
	}

	return v
}

// The skeleton lets the shooter go once the sword is seen, and answers her
// while the crossbow is up. Its profile is unchanged by #1745: patience is
// a span now rather than a maximum age, and 3 says the same thing the old
// number said.
func (s *MindedTestSuite) TestTheRetaliatorLetsAnUnarmedShooterGo() {
	base := s.theSwap(behavior.MindRetaliator)

	s.Run("the sword is seen", func() {
		intent, err := s.driver().Act(base)
		s.Require().NoError(err)
		s.Equal(encounter.Attack{Target: bob, Action: bowRef}, intent,
			"bob: alice cannot shoot back any more, so the nearest live target wins")
	})

	s.Run("the crossbow is still up", func() {
		intent, err := s.driver().Act(s.stillArmed(base))
		s.Require().NoError(err)
		s.Equal(encounter.Attack{Target: alice, Action: bowRef}, intent,
			"alice: same shot, same clock, and this time nothing excuses her")
	})

	s.Run("somebody is in its face", func() {
		intent, err := s.driver().Act(s.crowded(base))
		s.Require().NoError(err)
		s.Equal(encounter.Attack{Target: bob, Action: meleeRef}, intent,
			"it keeps no room at all: bob is adjacent and it swings")
	})
}

// The berserker keeps coming for the shooter. Putting the crossbow away
// does nothing, and neither does a nearer player: only the clock talks it
// off a grudge, and the clock has not run.
func (s *MindedTestSuite) TestTheBerserkerDoesNotCareWhatTheShooterIsHolding() {
	base := s.theSwap(behavior.MindBerserker)

	s.Run("the sword is seen", func() {
		intent, err := s.driver().Act(base)
		s.Require().NoError(err)
		s.Equal(encounter.Attack{Target: alice, Action: bowRef}, intent,
			"alice: the same scene the skeleton reads as over")
	})

	s.Run("the crossbow is still up", func() {
		intent, err := s.driver().Act(s.stillArmed(base))
		s.Require().NoError(err)
		s.Equal(encounter.Attack{Target: alice, Action: bowRef}, intent)
	})

	s.Run("somebody is in its face", func() {
		intent, err := s.driver().Act(s.crowded(base))
		s.Require().NoError(err)
		s.Equal(encounter.Attack{Target: alice, Action: bowRef}, intent,
			"alice, past the man standing next to it: a grudge outranks a distance")
	})
}

// The coward never ranks a grudge and never lets anybody stand next to it.
// It shoots whoever is nearest from where it is, and backs off the moment
// that becomes touching distance.
func (s *MindedTestSuite) TestTheCowardHoldsNoGrudgeAndKeepsItsRoom() {
	base := s.theSwap(behavior.MindCoward)

	s.Run("the sword is seen", func() {
		intent, err := s.driver().Act(base)
		s.Require().NoError(err)
		s.Equal(encounter.Attack{Target: bob, Action: bowRef}, intent,
			"bob: the nearest, with the bow — two cells is outside its room")
	})

	s.Run("the crossbow is still up", func() {
		intent, err := s.driver().Act(s.stillArmed(base))
		s.Require().NoError(err)
		s.Equal(encounter.Attack{Target: bob, Action: bowRef}, intent,
			"still bob: this is the scene both grudges answer, and it has none")
	})

	s.Run("somebody is in its face", func() {
		intent, err := s.driver().Act(s.crowded(base))
		s.Require().NoError(err)
		s.Equal(encounter.Move{Path: bobAwayStep}, intent,
			"one cell back from bob, the way the encounter laid it out")
	})
}

// A grudge of zero patience refuses even the shot that just landed.
//
// This is the zero value's own claim, and age zero is the only case that
// could break it: a Patience read as a maximum AGE would answer a deed of
// age zero, so a mind with no grudge would hold one for exactly as long as
// anybody was looking. Patience is a SPAN, and a span of zero covers
// nothing.
//
// The retaliator is the control: the same deed, at the same instant, on the
// same board.
func (s *MindedTestSuite) TestAZeroGrudgeRefusesTheShotThatJustLanded() {
	s.Run("the coward", func() {
		intent, err := s.driver().Act(s.justShot(s.stillArmed(s.theSwap(behavior.MindCoward))))
		s.Require().NoError(err)
		s.Equal(encounter.Attack{Target: bob, Action: bowRef}, intent,
			"bob: it was shot this very tick and holds nothing against anybody")
	})

	s.Run("the retaliator, for contrast", func() {
		intent, err := s.driver().Act(s.justShot(s.stillArmed(s.theSwap(behavior.MindRetaliator))))
		s.Require().NoError(err)
		s.Equal(encounter.Attack{Target: alice, Action: bowRef}, intent,
			"alice: a mind with a grudge answers exactly the deed the coward ignored")
	})
}

// The zero [behavior.Grudge] holds nothing, asked of Rank directly.
//
// The driver proof above runs the coward, whose grudge happens to be the
// zero; this one runs the zero itself against the hardest contact there is
// — a remembered shooter, who has no hands to check and whom
// [MindedTestSuite.TestARememberedShooterIsStillRankedFirst] proves a real
// grudge ranks ahead of the live man in its face.
func (s *MindedTestSuite) TestTheZeroGrudgeRanksNobodyFirst() {
	ghost := mind.Contact{
		Bearer: alice, Name: mind.Name(alice), Named: true,
		Holdings: []mind.Holding{
			{
				Holding: perception.Holding{
					Subject: alice, Channel: perception.Sight, Observed: 1, Confirmed: 1,
					Payload: armed(s.T(), aliceAt, weapons.LightCrossbow, ""),
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

	situation := mind.Situation{
		Actor:    skeleton,
		At:       3,
		Self:     mind.Self{Where: skeletonAt.String()},
		Contacts: []mind.Contact{live, ghost},
	}

	cases := []struct {
		mind   string
		grudge behavior.Grudge
		first  string
	}{
		{mind: "a grudge that stands", grudge: handGrudge, first: string(alice)},
		{mind: "the zero grudge", grudge: behavior.Grudge{}, first: string(bob)},
	}

	for _, tc := range cases {
		s.Run(tc.mind, func() {
			r := &behavior.Retaliator{
				Space:  flatSpace{aliceAt.String(): 4, bobAt.String(): 1},
				Grudge: tc.grudge,
			}

			out, err := r.Rank(&mind.RankInput{Situation: situation})
			s.Require().NoError(err)
			s.Require().Len(out.Ranked, 2)
			s.Equal(tc.first, string(out.Ranked[0].Bearer))
		})
	}
}
