// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type behaviorConsumerSuite struct{ suite.Suite }

func TestBehaviorConsumerSuite(t *testing.T) { suite.Run(t, new(behaviorConsumerSuite)) }

type behaviorRolls struct{ remaining, calls int }

func (d *behaviorRolls) Roll(_ context.Context, sides int) (int, error) {
	if d.remaining <= 0 || sides != 100 {
		return 0, fmt.Errorf("unexpected d%d, remaining scripted d100 faces: %d", sides, d.remaining)
	}
	d.remaining--
	d.calls++
	return 1, nil
}

func (d *behaviorRolls) RollN(context.Context, int, int) ([]int, error) {
	return nil, fmt.Errorf("unexpected RollN in behavior selection")
}

func (s *behaviorConsumerSuite) TestCompletionWaitsForRealArrivalAndSurvivesReload() {
	raw, err := os.ReadFile("testdata/behavior-patrol.json")
	s.Require().NoError(err)
	var definition behavior.SequenceDefinition
	s.Require().NoError(jsonv2.Unmarshal(raw, &definition))
	bound, err := behavior.ExpandSequence(&behavior.ExpandSequenceInput{
		ID: "duty", Definition: definition, Parameters: map[string]any{"weight": 1},
		Items:      []jsontext.Value{[]byte(`{"x":6,"y":0}`), []byte(`{"x":2,"y":0}`)},
		Vocabulary: machineVocabulary(),
	})
	s.Require().NoError(err)
	guard := MemberInput{ID: "guard", Kind: KindMonster, Position: spatial.Position{X: 2},
		Behaviors: &BehaviorInput{Instances: []BoundBehavior{{Instance: bound.Instance, Completions: bound.Completions}}}}
	in := wakeSetup(BehaviorDriver{}, guard)
	in.Sheets = sheetFacts{"guard": {SpeedFeet: 10}, "alice": {SpeedFeet: 30}}
	die := &behaviorRolls{remaining: 3}
	in.Roller = die
	e, err := NewEncounter(in)
	s.Require().NoError(err)

	// A real world opportunity runs the real router and walk. Ten feet buys
	// two cells, not arrival at the four-cell-away goal.
	s.Require().NoError(e.spendWorldAction("alice"))
	cell, err := e.cellOf(e.members["guard"])
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 4}, cell)
	s.Equal("item/0/walk", e.members["guard"].behaviors.composer.ToData().Instances[0].Blob.State)
	s.Equal(1, die.calls)

	stored := e.ToData()
	encoded, err := json.Marshal(stored)
	s.Require().NoError(err)
	var decoded EncounterData
	s.Require().NoError(json.Unmarshal(encoded, &decoded))
	// Changing author input cannot rebind a saved run.
	guard.Behaviors.Instances[0].Instance.Table["item/0/move"][0].Toward.At.X = 18
	e, err = LoadEncounter(&LoadEncounterInput{Data: decoded, Capabilities: in.Capabilities})
	s.Require().NoError(err)
	s.Equal(1, die.calls, "loading does not choose or execute")
	s.Require().NoError(e.spendWorldAction("alice"))
	cell, err = e.cellOf(e.members["guard"])
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 6}, cell)
	s.Equal("item/1/walk", e.members["guard"].behaviors.composer.ToData().Instances[0].Blob.State)
	s.Equal(2, die.calls, "arrival completion grants no extra choice")
	s.Require().NoError(e.spendWorldAction("alice"))
	cell, err = e.cellOf(e.members["guard"])
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 4}, cell)
	s.Equal("item/1/walk", e.members["guard"].behaviors.composer.ToData().Instances[0].Blob.State)
	s.Equal(3, die.calls)
}

func (s *behaviorConsumerSuite) TestOldMachineTableInputRefusesInsteadOfInstallingAnotherRunner() {
	_, err := NewEncounter(wakeSetup(BehaviorDriver{}, MemberInput{ID: "old", Kind: KindMonster, Position: spatial.Position{X: 2}, Table: holds()}))
	s.ErrorIs(err, ErrBadMember)
	s.ErrorContains(err, "behaviors")
}

func (s *behaviorConsumerSuite) TestNeedsProgressDoesNotCompeteForMovement() {
	s.assertNeedsAndDuty(10, 4, 1)
}

func (s *behaviorConsumerSuite) TestDataOnlyProgressSurvivesSelectedRoleHold() {
	s.assertNeedsAndDuty(0, 2, 0)
}

func (s *behaviorConsumerSuite) assertNeedsAndDuty(speed int, wantX float64, rolls int) {
	raw, err := os.ReadFile("testdata/behavior-patrol.json")
	s.Require().NoError(err)
	var definition behavior.SequenceDefinition
	s.Require().NoError(jsonv2.Unmarshal(raw, &definition))
	bound, err := behavior.ExpandSequence(&behavior.ExpandSequenceInput{
		ID: "duty", Definition: definition, Parameters: map[string]any{"weight": 1},
		Items: []jsontext.Value{[]byte(`{"x":6,"y":0}`), []byte(`{"x":2,"y":0}`)}, Vocabulary: machineVocabulary(),
	})
	s.Require().NoError(err)
	needs := BoundBehavior{Instance: behavior.InstanceSpec{ID: "needs", Recipe: machine.Recipe{
		Ref: core.Ref{Module: "test", Type: "behaviors", ID: "needs"}, Initial: "living", Bag: machine.Bag{"hungry": false},
		States: map[string]machine.State{"living": {Transitions: []machine.Transition{
			{On: beatTime, Do: []machine.Act{{Set: &machine.SetAct{Key: "hungry", Value: true}}}},
		}}},
	}}}
	guard := MemberInput{ID: "guard", Kind: KindMonster, Position: spatial.Position{X: 2}, Behaviors: &BehaviorInput{
		Instances: []BoundBehavior{needs, {Instance: bound.Instance, Completions: bound.Completions}},
	}}
	in := wakeSetup(BehaviorDriver{}, guard)
	in.Sheets = sheetFacts{"guard": {SpeedFeet: speed}, "alice": {SpeedFeet: 30}}
	die := &behaviorRolls{remaining: rolls}
	in.Roller = die
	e, err := NewEncounter(in)
	s.Require().NoError(err)
	s.Require().NoError(e.spendWorldAction("alice"))
	cell, err := e.cellOf(e.members["guard"])
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: wantX}, cell)
	progress := e.members["guard"].behaviors.composer.ToData()
	s.Equal(true, progress.Instances[0].Blob.Bag["hungry"])
	s.Equal("item/0/walk", progress.Instances[1].Blob.State)
	s.Equal(rolls, die.calls)
}

func (s *behaviorConsumerSuite) TestWorldDutyIsNotOfferedAFightTurn() {
	guard := listener("guard", 4, 0, recipe(s.T(), "ready", map[string]machine.State{
		"ready": {Transitions: []machine.Transition{{On: beatTime, To: "spent", Do: []machine.Act{{Pick: string(AnswerTime)}}}}},
		"spent": {},
	}))
	in := wakeSetup(BehaviorDriver{}, guard)
	die := &behaviorRolls{}
	in.Roller = die
	e, err := NewEncounter(in)
	s.Require().NoError(err)
	_, err = e.form(&FormInput{Order: []MemberID{"alice", "guard"}})
	s.Require().NoError(err)
	_, err = e.EndTurn(&EndTurnInput{Member: "alice"})
	s.Require().NoError(err)
	s.Equal("ready", e.members["guard"].bodyState())
	s.Zero(die.calls)
}

func (s *behaviorConsumerSuite) TestNotificationCannotAuthorizeSelection() {
	guard := listener("guard", 4, 0, recipe(s.T(), "ready", map[string]machine.State{
		"ready": {Transitions: []machine.Transition{{On: clockBeat(clockFight), To: "spent", Do: []machine.Act{{Pick: string(AnswerTime)}}}}},
		"spent": {},
	}))
	in := wakeSetup(BehaviorDriver{}, guard)
	die := &behaviorRolls{}
	in.Roller = die
	e, err := NewEncounter(in)
	s.Require().NoError(err)
	before := e.members["guard"].behaviors.composer.ToData()
	e.landClock(clockFight, FactionMonsters, 1)
	s.Require().ErrorIs(e.matchLanded(), behavior.ErrSelectionRequired)
	s.Equal(before, e.members["guard"].behaviors.composer.ToData())
	s.Zero(die.calls)
}

func (s *behaviorConsumerSuite) TestTwoRoutesInvestigateAndResumeTheUnfinishedLeg() {
	raw, err := os.ReadFile("testdata/behavior-patrol.json")
	s.Require().NoError(err)
	var patrol behavior.SequenceDefinition
	s.Require().NoError(jsonv2.Unmarshal(raw, &patrol))
	raw, err = os.ReadFile("testdata/behavior-investigation.json")
	s.Require().NoError(err)
	var investigation behavior.InstanceSpec
	s.Require().NoError(jsonv2.Unmarshal(raw, &investigation))
	west, err := behavior.ExpandSequence(&behavior.ExpandSequenceInput{
		ID: "duty", Definition: patrol, Parameters: map[string]any{"weight": 1},
		Items: []jsontext.Value{[]byte(`{"x":6,"y":0}`), []byte(`{"x":2,"y":0}`)}, Vocabulary: machineVocabulary(),
	})
	s.Require().NoError(err)
	court, err := behavior.ExpandSequence(&behavior.ExpandSequenceInput{
		ID: "duty", Definition: patrol, Parameters: map[string]any{"weight": 1},
		Items: []jsontext.Value{[]byte(`{"x":14,"y":0}`), []byte(`{"x":16,"y":0}`), []byte(`{"x":12,"y":0}`)}, Vocabulary: machineVocabulary(),
	})
	s.Require().NoError(err)
	in := wakeSetup(BehaviorDriver{},
		MemberInput{ID: "west", Kind: KindMonster, Position: spatial.Position{X: 2}, Behaviors: &BehaviorInput{
			Instances: []BoundBehavior{{Instance: investigation}, {Instance: west.Instance, Completions: west.Completions}},
		}},
		MemberInput{ID: "court", Kind: KindMonster, Position: spatial.Position{X: 12}, Behaviors: &BehaviorInput{
			Instances: []BoundBehavior{{Instance: investigation}, {Instance: court.Instance, Completions: court.Completions}},
		}},
	)
	in.Sheets = sheetFacts{"west": {SpeedFeet: 10, PassivePerception: 10}, "court": {SpeedFeet: 10, PassivePerception: 4}, "alice": {SpeedFeet: 30}}
	die := &behaviorRolls{remaining: 10}
	in.Roller = die
	e, err := NewEncounter(in)
	s.Require().NoError(err)
	s.Require().NoError(e.spendWorldAction("alice"))
	pending := e.members["west"].behaviors.composer.ToData().Instances[1]
	s.Equal("item/0/walk", pending.Blob.State)
	cell, err := e.cellOf(e.members["west"])
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 4}, cell)

	// Both occurrences happen before the raise. The later voice must not
	// erase the qualifying clang; the court guard's actual hearing fails DC 5.
	s.Require().NoError(e.makeSound(noiseAttack, "alice", 1))
	s.Require().NoError(e.makeSound(noiseSay, "alice", 1))
	s.Require().Len(e.landed, 2)
	for _, landed := range e.landed {
		s.Equal(MemberID("west"), landed.Member)
	}
	for round := 2; round <= 3; round++ {
		s.Require().NoError(e.spendWorldAction("alice"))
		s.Equal(pending, e.members["west"].behaviors.composer.ToData().Instances[1])
		cell, err = e.cellOf(e.members["west"])
		s.Require().NoError(err)
		s.Equal(spatial.Position{X: 2, Y: 2}, cell)
		s.Equal("idle", e.members["court"].behaviors.composer.ToData().Instances[0].Blob.State)
	}
	// Reload mid-investigation, keeping the unfinished original leg.
	encoded, err := json.Marshal(e.ToData())
	s.Require().NoError(err)
	var saved EncounterData
	s.Require().NoError(json.Unmarshal(encoded, &saved))
	e, err = LoadEncounter(&LoadEncounterInput{Data: saved, Capabilities: in.Capabilities})
	s.Require().NoError(err)
	s.Require().NoError(e.spendWorldAction("alice"))
	cell, err = e.cellOf(e.members["west"])
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 4}, cell, "resume toward pending six, not the nearer old waypoint two")
	s.Equal("item/0/walk", e.members["west"].behaviors.composer.ToData().Instances[1].Blob.State)
	s.Require().NoError(e.spendWorldAction("alice"))
	cell, err = e.cellOf(e.members["west"])
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 6}, cell)
	s.Equal("item/1/walk", e.members["west"].behaviors.composer.ToData().Instances[1].Blob.State)
	cell, err = e.cellOf(e.members["court"])
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 14}, cell)
	s.Equal("item/1/walk", e.members["court"].behaviors.composer.ToData().Instances[1].Blob.State)
	s.Equal(10, die.calls)
}

func (s *behaviorConsumerSuite) TestBehaviorOnlyAlarmFactSurvivesReload() {
	guard := monster("guard", 4, 0, Table{AnswerTime: {{Weight: 1, Rows: []Answer{{Weight: 1, Say: "To arms!", Fact: "behavior-only-alarm"}}}}})
	in := wakeSetup(BehaviorDriver{}, guard)
	in.Sheets = sheetFacts{"guard": {SpeedFeet: 10, PassivePerception: 10}, "alice": {SpeedFeet: 30, PassivePerception: 10}}
	die := &behaviorRolls{remaining: 2}
	in.Roller = die
	e, err := NewEncounter(in)
	s.Require().NoError(err)
	s.Require().NoError(e.spendWorldAction("alice"))
	s.True(e.world.knowsFact("alice", "behavior-only-alarm"), "the actual voice taught its hearer")
	raw, err := json.Marshal(e.ToData())
	s.Require().NoError(err)
	var saved EncounterData
	s.Require().NoError(json.Unmarshal(raw, &saved))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: saved, Capabilities: in.Capabilities})
	s.Require().NoError(err, "a fact authored only in a nested behavior row belongs to this run")
	s.True(loaded.world.knowsFact("alice", "behavior-only-alarm"))
	s.Equal(2, die.calls)
}

func (s *behaviorConsumerSuite) TestNoPolicyNeedsNoDie() {
	in := wakeSetup(BehaviorDriver{}, MemberInput{ID: "guard", Kind: KindMonster, Position: spatial.Position{X: 4}})
	in.Roller = nil
	e, err := NewEncounter(in)
	s.Require().NoError(err)
	_, err = e.form(&FormInput{Order: []MemberID{"alice", "guard"}})
	s.Require().NoError(err)
	_, err = e.EndTurn(&EndTurnInput{Member: "alice"})
	s.Require().NoError(err, "an empty composition makes no choice and requires no randomness")
}

func (s *behaviorConsumerSuite) TestCompulsionPrecedesTheOrdinaryLane() {
	guard := monster("guard", 4, 0, holds())
	in := wakeSetup(passDriver{}, guard)
	die := &behaviorRolls{}
	in.Roller = die
	e, err := NewEncounter(in)
	s.Require().NoError(err)
	before := e.members["guard"].behaviors.composer.ToData()
	s.Require().NoError(e.spendWorldAction("alice"))
	s.Equal(before, e.members["guard"].behaviors.composer.ToData())
	s.Zero(die.calls, "a concrete imposed decision never consults ordinary choice")
}
