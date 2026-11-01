// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// not_test.go is THE ONE NEGATION (rpg-project#719): `{ not: <predicate> }`
// read on an option's requires and a leave ending's if, waited for on a
// stance an until or an ending names, judged at the departure on a left, and
// refused wherever it could never fire.

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// caughtInKitchen is the fact a failed pilfer teaches whoever saw it.
const caughtInKitchen = encounter.FactID("caught-in-kitchen")

// cookOptions is the castle cook's theft and the line it closes: pilfer
// teaches the fact on a failure, and chat is withheld from whoever knows it.
func cookOptions() []encounter.Option {
	return []encounter.Option{
		{
			ID: "pilfer", Say: "(lift a pie from the sill)",
			Check:   []encounter.CheckApproach{{Ability: "sleight-of-hand", DC: 13}},
			Success: []encounter.Answer{{Weight: 1, Say: "The cook never looks round."}},
			Failure: []encounter.Answer{{Weight: 1, Say: "Thief!", Fact: caughtInKitchen}},
		},
		{
			ID: "chat", Say: "Smells good in here.",
			Requires: encounter.TriggerNot{Inner: encounter.TriggerFact{Fact: caughtInKitchen}},
			Check:    []encounter.CheckApproach{{Ability: "persuasion", DC: 10}},
			Success:  []encounter.Answer{{Weight: 1, Say: "Stew for the guard."}},
		},
	}
}

// D3 (ruled 2026-10-29): `requires` reads the SPEAKER's knowledge, so the
// not-fact line is offered to both before the theft, withheld from alice once
// she is caught, and still offered to billy, who was behind the wall — and
// the withholding survives a reload.
func (s *OptionTestSuite) TestANotFactOptionIsWithheldFromTheSpeakerWhoWasCaught() {
	enc := s.sceneWith(cookOptions())

	before, err := enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"pilfer", "chat"}, ids(before), "nobody has been caught yet")

	_, err = enc.Speak(s.ctx, &encounter.SpeakInput{
		Range: speakReach,
		Actor: alice, Target: goblin, OptionID: "pilfer", Skill: "sleight-of-hand",
		Beaten: false, DC: 13, Total: 5, Roller: rollsLowest{},
	})
	s.Require().NoError(err)

	after, err := enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"pilfer"}, ids(after), "alice was caught, and the cook's other line is gone for her")

	billys, err := enc.OptionsFor(billy, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"pilfer", "chat"}, ids(billys), "billy saw nothing and the speaker's knowledge is what is read")

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Capabilities: s.capabilities(),
	})
	s.Require().NoError(err)
	again, err := reloaded.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Equal([]string{"pilfer"}, ids(again), "the not and the fact both came back")
}

// A not of a holding is offered while the speaker's hands are empty of it.
func (s *OptionTestSuite) TestANotHoldsOptionIsOfferedOnlyWhileTheSpeakerLacksTheProp() {
	options := cookOptions()
	options[1].Requires = encounter.TriggerNot{Inner: encounter.TriggerHolds{Prop: thePurse}}
	enc := s.sceneWith(options)

	before, err := enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.Contains(ids(before), "chat")

	_, err = enc.Hold(&encounter.HoldInput{Member: alice, Target: thePurse})
	s.Require().NoError(err)
	held, err := enc.OptionsFor(alice, goblin)
	s.Require().NoError(err)
	s.NotContains(ids(held), "chat", "alice is carrying the purse now")
}

// A requirement's not is refused at the door when it negates nothing, another
// not, or a form an option does not read.
func (s *OptionTestSuite) TestANotRequirementThatCannotBeReadIsRefusedAtConstruction() {
	for name, requires := range map[string]encounter.Trigger{
		"nothing":    encounter.TriggerNot{},
		"a not":      encounter.TriggerNot{Inner: encounter.TriggerNot{Inner: encounter.TriggerFact{Fact: "x"}}},
		"a round":    encounter.TriggerNot{Inner: encounter.TriggerRound{Round: 2}},
		"a lantern":  encounter.TriggerNot{Inner: encounter.TriggerHolds{Prop: "lantern"}},
		"empty fact": encounter.TriggerNot{Inner: encounter.TriggerFact{}},
	} {
		s.Run(name, func() {
			options := cookOptions()
			options[1].Requires = requires
			_, err := encounter.NewEncounter(&encounter.SetupInput{
				Field: encounter.FieldInput{
					Canvas:  openAir(),
					Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 6, 6)},
					Props:   []encounter.PropInput{holdableProp(thePurse, "test:props:purse", spatial.Position{X: 1, Y: 1})},
				},
				Members: []encounter.MemberInput{
					{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 1}},
					{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1}, Options: options},
				},
				Endings:      []encounter.EndingInput{withdrawn()},
				Capabilities: s.capabilities(),
			})
			s.Require().ErrorIs(err, encounter.ErrBadMember)
		})
	}
}

// An until on a not of a stance turns its pair when the other pair turns
// AWAY from that stance: the goblins stop being civil on round 3, and the
// dogs, who were civil only while the goblins were, turn with them.
func (s *BothWaysSuite) TestAnUntilOnANotStanceTurnsWhenThePairStopsBeingIt() {
	enc := s.open(
		s.yard(
			[]encounter.FactionInput{{ID: bwGoblins}, {ID: bwDogs}, {ID: bwKobolds}},
			[]encounter.DispositionInput{
				neutralUntil(encounter.TriggerRound{Round: 3}),
				{
					Between: [2]encounter.FactionID{bwDogs, encounter.FactionParty},
					Stance:  encounter.StanceNeutral,
					Until: []encounter.UntilStep{{When: encounter.TriggerNot{Inner: encounter.TriggerStance{
						Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty},
						Stance:  encounter.StanceNeutral,
					}}}},
				},
			},
			false,
		),
		[]encounter.MemberInput{
			player(alice, 0, 1),
			monster(bwKobold, bwKobolds, 4, 4),
			monster(bwChief, bwGoblins, 4, 1),
			monster(bwDog, bwDogs, 5, 1),
		},
	)

	end, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().True(end.RoundWrapped)
	s.Equal(encounter.StanceNeutral, s.stance(enc, bwDogs, encounter.FactionParty), "round two: the goblins are still civil")

	end, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().True(end.RoundWrapped)
	s.Equal(encounter.StanceHostile, s.stance(enc, bwGoblins, encounter.FactionParty))
	s.Equal(encounter.StanceHostile, s.stance(enc, bwDogs, encounter.FactionParty))
	dogs := s.turnedTo(enc, alice, bwDogs, encounter.FactionParty)
	s.Equal("goblins and party stopped being neutral", dogs["cause"], "the beat says why the dogs turned")
}

// An ending on a not of a stance fires in the fold where the pair turns away
// from it, and survives the blob on the way.
func (s *BothWaysSuite) TestAnEndingOnANotStanceFiresWhenThePairTurnsAway() {
	field := s.yard(
		[]encounter.FactionInput{{ID: bwGoblins}},
		[]encounter.DispositionInput{{
			Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty},
			Stance:  encounter.StanceNeutral,
		}},
		false,
	)
	members := []encounter.MemberInput{player(alice, 0, 1), monster(bwScout, bwGoblins, 4, 1)}
	truceBroken := encounter.EndingInput{
		Key: "truce-broken", Result: encounter.ResultFailure,
		Trigger: encounter.TriggerNot{Inner: encounter.TriggerStance{
			Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty}, Stance: encounter.StanceNeutral,
		}},
	}
	enc := s.openWith(field, members, truceBroken)

	data := enc.ToData()
	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Capabilities: encounter.Capabilities{
			Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{},
			Conditions: encounter.UnobservedEquipment{}, Sheets: zeroSheets{}, Participation: s.standing,
			Initiative: orderAsGiven{}, Driver: passDriver{}, CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
			Actors: encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	})
	s.Require().NoError(err, "a not ending reloads")

	_, err = recordOne(reloaded, &encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{bwScout},
		Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 7},
	})
	s.Require().NoError(err)
	status, err := reloaded.Status()
	s.Require().NoError(err)
	s.Require().NotNil(status.Outcome, "the swing broke the truce and the run ended on it")
	s.Equal("truce-broken", status.Outcome.Ending)
}

// A waited-for not is refused wherever it could never fire: a fact, a round
// and a fall hold from the start, a stance the pair is not in from the start
// holds from the start too, a holding is only ever required, and a not of a
// not is the predicate itself.
func (s *BothWaysSuite) TestAWaitedForNotThatCannotFireIsRefused() {
	goblinsNeutral := encounter.TriggerStance{
		Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty}, Stance: encounter.StanceNeutral,
	}
	goblinsHostile := goblinsNeutral
	goblinsHostile.Stance = encounter.StanceHostile
	for _, tc := range []struct {
		name    string
		trigger encounter.Trigger
		says    string
	}{
		{"a fact", encounter.TriggerNot{Inner: encounter.TriggerFact{Fact: bwFact}}, "holds from the start"},
		{"a round", encounter.TriggerNot{Inner: encounter.TriggerRound{Round: 3}}, "holds from the start"},
		{"a fall", encounter.TriggerNot{Inner: encounter.TriggerMemberDown{Member: bwScout}}, "holds from the start"},
		{"a stance the pair is not in", encounter.TriggerNot{Inner: goblinsHostile}, "which they are not from the start"},
		{"a holding", encounter.TriggerNot{Inner: encounter.TriggerHolds{Prop: "purse"}}, "only something an option requires"},
		{"a not", encounter.TriggerNot{Inner: encounter.TriggerNot{Inner: goblinsNeutral}}, "a not of a not"},
		{"nothing", encounter.TriggerNot{}, "names nothing to negate"},
	} {
		s.Run(tc.name, func() {
			_, err := encounter.NewEncounter(&encounter.SetupInput{
				Field: s.yard([]encounter.FactionInput{{ID: bwGoblins}}, []encounter.DispositionInput{{
					Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty}, Stance: encounter.StanceNeutral,
				}}, false),
				Members:      []encounter.MemberInput{player(alice, 0, 1), monster(bwScout, bwGoblins, 4, 1)},
				Endings:      []encounter.EndingInput{{Key: "never", Trigger: tc.trigger}},
				Capabilities: encounter.Capabilities{Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{}, Conditions: encounter.UnobservedEquipment{}, Sheets: zeroSheets{}, Participation: s.standing, Initiative: orderAsGiven{}, Driver: passDriver{}, Actors: encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}}},
			})
			s.Require().ErrorIs(err, encounter.ErrNoEnding)
			s.Contains(err.Error(), tc.says)
		})
	}
}

// A leave ending's if is a state read, so its not is the read negated: the
// king alive when the last one leaves is the failure, whatever comes after.
func (s *LeaveSuite) TestANotIfIsTheReadNegated() {
	endings := []encounter.EndingInput{
		{Key: "king-lives", Trigger: encounter.TriggerLeft{Exit: leaveDoor},
			If: encounter.TriggerNot{Inner: encounter.TriggerMemberDown{Member: leaveKing}}, Result: encounter.ResultFailure},
		{Key: "king-slain", Trigger: encounter.TriggerLeft{Exit: leaveDoor}, Result: encounter.ResultSuccess},
	}

	s.Run("the king stands", func() {
		enc, err := s.scene(endings)
		s.Require().NoError(err)
		_, last := s.leave(enc)
		s.Require().NotNil(last)
		s.Equal("king-lives", last.Ending)
	})
	s.Run("the king is down", func() {
		enc, err := s.scene(endings)
		s.Require().NoError(err)
		s.standing.down = []encounter.MemberID{leaveKing}
		_, last := s.leave(enc)
		s.Require().NotNil(last)
		s.Equal("king-slain", last.Ending, "the not did not hold, so the next ending at the door is the one")
	})
	s.Run("and it survives the blob", func() {
		enc, err := s.scene(endings)
		s.Require().NoError(err)
		reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data: enc.ToData(), Capabilities: s.capabilities(),
		})
		s.Require().NoError(err)
		_, last := s.leave(reloaded)
		s.Require().NotNil(last)
		s.Equal("king-lives", last.Ending)
	})
}

// A not of a left is judged at the same departure as the left and answers
// the opposite: the last one out by the back way, not the front door.
func (s *LeaveSuite) TestANotLeftIsTheLastOneOutByAnyOtherDoor() {
	backDoor := encounter.ExitID("back-door")
	open := func() *encounter.Encounter {
		s.standing = &nobodyIsInContact{}
		f := s.field()
		f.Exits = append(f.Exits, encounter.FieldExit{ID: backDoor, At: besideTheDoor})
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Field: f,
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: leaveDoorCell},
				{ID: bob, Kind: encounter.KindPlayer, Position: besideTheDoor},
			},
			Endings: []encounter.EndingInput{{
				Key: "slipped-out", Trigger: encounter.TriggerNot{Inner: encounter.TriggerLeft{Exit: leaveDoor}},
				Result: encounter.ResultFailure,
			}},
			Retention:    encounter.RetentionUnbounded,
			Capabilities: s.capabilities(),
		})
		s.Require().NoError(err)
		return enc
	}

	s.Run("the last one out by the back", func() {
		enc := open()
		out, err := enc.Exit(&encounter.ExitInput{Member: alice})
		s.Require().NoError(err)
		s.Nil(out.Closed, "bob is still inside")
		out, err = enc.Exit(&encounter.ExitInput{Member: bob})
		s.Require().NoError(err)
		s.Require().NotNil(out.Closed)
		s.Equal("slipped-out", out.Closed.Ending)
	})
	s.Run("the last one out by the front is not it", func() {
		enc := open()
		_, err := enc.Exit(&encounter.ExitInput{Member: bob})
		s.Require().NoError(err)
		out, err := enc.Exit(&encounter.ExitInput{Member: alice})
		s.Require().NoError(err)
		s.Require().NotNil(out.Closed)
		s.Equal("abandoned", out.Closed.Ending, "no authored ending held, so the run is abandoned as ever")
	})
	s.Run("refused when the door named is the only way out", func() {
		_, err := s.scene([]encounter.EndingInput{{
			Key: "slipped-out", Trigger: encounter.TriggerNot{Inner: encounter.TriggerLeft{Exit: leaveDoor}},
			Result: encounter.ResultFailure,
		}})
		s.Require().ErrorIs(err, encounter.ErrNoEnding)
		s.Contains(err.Error(), "it is the only way out")
	})
	s.Run("a not left anywhere but an ending is refused in left's words", func() {
		s.standing = &nobodyIsInContact{}
		f := s.field()
		f.Factions = []encounter.FactionInput{{ID: "court"}}
		f.Dispositions = []encounter.DispositionInput{{
			Between: [2]encounter.FactionID{"court", encounter.FactionParty}, Stance: encounter.StanceNeutral,
			Until: []encounter.UntilStep{{When: encounter.TriggerNot{Inner: encounter.TriggerLeft{Exit: leaveDoor}}}},
		}}
		_, err := encounter.NewEncounter(&encounter.SetupInput{
			Field:        f,
			Members:      []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: leaveDoorCell}},
			Endings:      []encounter.EndingInput{withdrawn()},
			Capabilities: s.capabilities(),
		})
		s.Require().ErrorIs(err, encounter.ErrNoFaction)
		s.Contains(err.Error(), "only an ending waits for that")
	})
}
