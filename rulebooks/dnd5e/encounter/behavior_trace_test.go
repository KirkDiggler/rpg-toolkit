// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type behaviorTraceSuite struct{ suite.Suite }

func TestBehaviorTraceSuite(t *testing.T) { suite.Run(t, new(behaviorTraceSuite)) }

// This only loads a complete authored definition through the real expander.
// The production composer/router/pause paths do all runtime work.
func (s *behaviorTraceSuite) sequence(file string) BoundBehavior {
	raw, err := os.ReadFile("testdata/" + file)
	s.Require().NoError(err)
	var definition behavior.SequenceDefinition
	s.Require().NoError(jsonv2.Unmarshal(raw, &definition))
	bound, err := behavior.ExpandSequence(&behavior.ExpandSequenceInput{
		ID: "duty", Definition: definition, Parameters: map[string]any{"weight": 1},
		Items: []jsontext.Value{[]byte(`{"x":6,"y":0}`), []byte(`{"x":2,"y":0}`)}, Vocabulary: machineVocabulary(),
	})
	s.Require().NoError(err)
	return BoundBehavior{Instance: bound.Instance, Completions: bound.Completions}
}

type capturedBehavior struct {
	entry record.Entry
	body  struct {
		Beat           string            `json:"beat"`
		Schema         int               `json:"schema"`
		Member         MemberID          `json:"member"`
		Config         string            `json:"config"`
		Configuration  *BehaviorInput    `json:"configuration"`
		Operation      string            `json:"operation"`
		PickRecord     string            `json:"pick_record"`
		DecisionRecord string            `json:"decision_record"`
		Status         string            `json:"status"`
		Work           *BehaviorWorkData `json:"work"`
		Outcomes       []struct {
			ID        string `json:"id"`
			Before    string `json:"before"`
			After     string `json:"after"`
			Selected  bool   `json:"selected"`
			Committed bool   `json:"committed"`
		} `json:"outcomes"`
	}
}

func (s *behaviorTraceSuite) captured(e *Encounter, kind string) []capturedBehavior {
	out, err := e.ReadRecord(&ReadRecordInput{})
	s.Require().NoError(err)
	var rows []capturedBehavior
	for _, entry := range out.Entries {
		row := capturedBehavior{entry: entry}
		s.Require().NoError(jsonv2.Unmarshal(entry.Payload, &row.body))
		if row.body.Beat == kind {
			rows = append(rows, row)
		}
	}
	return rows
}

func (s *behaviorTraceSuite) TestConfigurationPrecedesDecisionAndUsesOriginalBinding() {
	bound := s.sequence("behavior-patrol.json")
	in := wakeSetup(BehaviorDriver{}, MemberInput{ID: "guard", Kind: KindMonster, Position: spatial.Position{X: 2}, Behaviors: &BehaviorInput{Instances: []BoundBehavior{bound}}})
	in.Retention = RetentionUnbounded
	in.Sheets = sheetFacts{"guard": {SpeedFeet: 10}, "alice": {SpeedFeet: 30}}
	die := &behaviorRolls{remaining: 1}
	in.Roller = die
	e, err := NewEncounter(in)
	s.Require().NoError(err)
	s.Require().NoError(e.spendWorldAction("alice"))
	configs := s.captured(e, BeatBehaviorConfig)
	decisions := s.captured(e, BeatBehaviorDecision)
	s.Require().Len(configs, 1)
	s.Require().NotEmpty(decisions)
	s.Less(configs[0].entry.ID, decisions[0].entry.ID)
	s.Equal(1, configs[0].body.Schema)
	s.Equal(e.members["guard"].behaviors.definition, configs[0].body.Config)
	s.Require().NotNil(configs[0].body.Configuration)
	s.Equal(e.members["guard"].behaviors.input, *configs[0].body.Configuration)
	s.NotContains(string(configs[0].entry.Payload), `"progress"`)
	s.Empty(configs[0].entry.Audience, "configuration symbols are GM/debug evidence")
	s.Empty(decisions[0].entry.Audience, "private alternatives are not player knowledge")
	s.Require().NotEmpty(decisions[0].body.PickRecord)
	picks := s.captured(e, BeatAnswered)
	s.Require().Len(picks, 1)
	s.Equal(picks[0].entry.ID, decisions[0].body.PickRecord)
	s.Equal(picks[0].entry.Tags[ActTag], decisions[0].entry.Tags[ActTag])
	work := s.captured(e, BeatBehaviorWork)
	s.Require().Len(work, 2)
	s.Equal("selected", work[0].body.Status)
	s.Equal("partial", work[1].body.Status)
	for _, row := range work {
		s.Equal(decisions[0].entry.ID, row.body.DecisionRecord)
	}
	s.Equal(1, die.calls)
}

func (s *behaviorTraceSuite) TestConfigurationRecordHasDeterministicBytes() {
	bound := s.sequence("behavior-patrol.json")
	in := wakeSetup(BehaviorDriver{}, MemberInput{ID: "guard", Kind: KindMonster, Position: spatial.Position{X: 2}, Behaviors: &BehaviorInput{Instances: []BoundBehavior{bound}}})
	in.Sheets = sheetFacts{"guard": {SpeedFeet: 10}, "alice": {SpeedFeet: 30}}
	var first []byte
	for range 24 {
		in.Roller = &behaviorRolls{remaining: 1}
		e, err := NewEncounter(in)
		s.Require().NoError(err)
		s.Require().NoError(e.spendWorldAction("alice"))
		rows := s.captured(e, BeatBehaviorConfig)
		s.Require().Len(rows, 1)
		if first == nil {
			first = rows[0].entry.Payload
		} else {
			s.Require().Equal(string(first), string(rows[0].entry.Payload), "identical symbol values must have identical captured bytes")
		}
	}
}

func (s *behaviorTraceSuite) TestReloadKeepsSymbolsAndRollCount() {
	bound := s.sequence("behavior-patrol.json")
	input := &BehaviorInput{Instances: []BoundBehavior{bound}}
	in := wakeSetup(BehaviorDriver{}, MemberInput{ID: "guard", Kind: KindMonster, Position: spatial.Position{X: 2}, Behaviors: input})
	in.Retention = RetentionUnbounded
	in.Sheets = sheetFacts{"guard": {SpeedFeet: 10}, "alice": {SpeedFeet: 30}}
	die := &behaviorRolls{remaining: 2}
	in.Roller = die
	e, err := NewEncounter(in)
	s.Require().NoError(err)
	s.Require().NoError(e.spendWorldAction("alice"))
	first := s.captured(e, BeatBehaviorConfig)
	s.Require().Len(first, 1)
	encoded, err := json.Marshal(e.ToData())
	s.Require().NoError(err)
	var stored EncounterData
	s.Require().NoError(json.Unmarshal(encoded, &stored))
	input.Instances[0].Instance.Table["item/0/move"][0].Toward.At.X = 18
	e, err = LoadEncounter(&LoadEncounterInput{Data: stored, Capabilities: in.Capabilities})
	s.Require().NoError(err)
	s.Require().NoError(e.spendWorldAction("alice"))
	configs := s.captured(e, BeatBehaviorConfig)
	s.Require().Len(configs, 1, "unchanged reload must not copy symbols again")
	s.Equal(first[0].entry.Payload, configs[0].entry.Payload)
	cell, err := e.cellOf(e.members["guard"])
	s.Require().NoError(err)
	s.Equal(spatial.Position{X: 6}, cell)
	s.Equal("item/1/walk", e.members["guard"].behaviors.composer.ToData().Instances[0].Blob.State)
	s.Equal(2, die.calls)
	work := s.captured(e, BeatBehaviorWork)
	s.Require().Len(work, 4)
	s.Equal("completed", work[3].body.Status)
}

func (s *behaviorTraceSuite) TestPartialAndPausedWalkNeverClaimCompletion() {
	bound := s.sequence("behavior-patrol-both.json")
	in := wakeSetup(BehaviorDriver{}, MemberInput{ID: "guard", Kind: KindMonster, Position: spatial.Position{X: 2}, Behaviors: &BehaviorInput{Instances: []BoundBehavior{bound}}})
	in.Retention = RetentionUnbounded
	in.Sheets = sheetFacts{"guard": {SpeedFeet: 20}, "alice": {SpeedFeet: 30}}
	die, mover := &behaviorRolls{remaining: 1}, &behaviorPausingMover{}
	in.Roller, in.Mover = die, mover
	e, err := NewEncounter(in)
	s.Require().NoError(err)
	_, err = e.form(&FormInput{Order: []MemberID{"alice", "guard"}})
	s.Require().NoError(err)
	_, err = e.EndTurn(&EndTurnInput{Member: "alice"})
	s.Require().NoError(err)
	s.Require().True(e.Paused())
	initial := s.captured(e, BeatBehaviorWork)
	s.Require().Len(initial, 2)
	s.Equal("selected", initial[0].body.Status)
	s.Equal("paused", initial[1].body.Status)
	decision := initial[0].body.DecisionRecord
	s.Require().NotEmpty(decision)
	for range 2 {
		encoded, err := json.Marshal(e.ToData())
		s.Require().NoError(err)
		var stored EncounterData
		s.Require().NoError(json.Unmarshal(encoded, &stored))
		e, err = LoadEncounter(&LoadEncounterInput{Data: stored, Capabilities: in.Capabilities})
		s.Require().NoError(err)
		_, err = e.Resume(context.Background())
		s.Require().NoError(err)
	}
	work := s.captured(e, BeatBehaviorWork)
	s.Require().Len(work, 4)
	for i, status := range []string{"selected", "paused", "paused", "completed"} {
		s.Equal(status, work[i].body.Status)
		s.Equal(decision, work[i].body.DecisionRecord)
		s.Equal(initial[0].entry.Tags[ActTag], work[i].entry.Tags[ActTag], "the held walk keeps its execution act")
	}
	s.Equal(1, die.calls)
	s.False(e.Paused())
}

func (s *behaviorTraceSuite) TestInvestigationDefersPatrolWithoutCommittingItsPreview() {
	bound := s.sequence("behavior-patrol.json")
	raw, err := os.ReadFile("testdata/behavior-investigation.json")
	s.Require().NoError(err)
	var investigation behavior.InstanceSpec
	s.Require().NoError(jsonv2.Unmarshal(raw, &investigation))
	in := wakeSetup(BehaviorDriver{}, MemberInput{ID: "guard", Kind: KindMonster, Position: spatial.Position{X: 2}, Behaviors: &BehaviorInput{Instances: []BoundBehavior{{Instance: investigation}, bound}}})
	in.Retention = RetentionUnbounded
	in.Sheets = sheetFacts{"guard": {SpeedFeet: 10, PassivePerception: 10}, "alice": {SpeedFeet: 30}}
	die := &behaviorRolls{remaining: 2}
	in.Roller = die
	e, err := NewEncounter(in)
	s.Require().NoError(err)
	s.Require().NoError(e.spendWorldAction("alice"))
	before := e.members["guard"].behaviors.composer.ToData().Instances[1]
	s.Require().NoError(e.makeSound(noiseAttack, "alice", 1))
	s.Require().NoError(e.spendWorldAction("alice"))
	found := false
	for _, d := range s.captured(e, BeatBehaviorDecision) {
		if d.body.Operation != "decide" || len(d.body.Outcomes) != 2 || !d.body.Outcomes[0].Selected {
			continue
		}
		found = true
		s.Equal("investigate", d.body.Outcomes[0].ID)
		s.False(d.body.Outcomes[1].Selected)
		s.False(d.body.Outcomes[1].Committed)
		s.Equal("item/0/walk", d.body.Outcomes[1].After)
	}
	s.True(found, "the real disturbance must produce a trace for both roles")
	s.Equal(before, e.members["guard"].behaviors.composer.ToData().Instances[1])
	s.Equal(2, die.calls)
}

func (s *behaviorTraceSuite) TestDriverBypassDoesNotInventMachineEvaluation() {
	bound := s.sequence("behavior-patrol.json")
	in := wakeSetup(passDriver{}, MemberInput{ID: "guard", Kind: KindMonster, Position: spatial.Position{X: 2}, Behaviors: &BehaviorInput{Instances: []BoundBehavior{bound}}})
	in.Retention = RetentionUnbounded
	die := &behaviorRolls{}
	in.Roller = die
	e, err := NewEncounter(in)
	s.Require().NoError(err)
	s.Require().NoError(e.spendWorldAction("alice"))
	rows := s.captured(e, BeatBehaviorDecision)
	s.Require().NotEmpty(rows)
	for _, row := range rows {
		s.Equal("driver", row.body.Operation)
		s.Empty(row.body.Outcomes)
	}
	s.Zero(die.calls)
	s.Equal("item/0/walk", e.members["guard"].behaviors.composer.ToData().Instances[0].Blob.State)
}
