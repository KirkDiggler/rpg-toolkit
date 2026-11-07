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
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type behaviorPauseSuite struct{ suite.Suite }

func TestBehaviorPauseSuite(t *testing.T) { suite.Run(t, new(behaviorPauseSuite)) }

// This supplied reaction boundary only announces/pauses. The production router,
// walk, pause, reload and Resume paths own every actual position and budget.
type behaviorPausingMover struct{ calls int }

func (m *behaviorPausingMover) Move(context.Context, *Encounter, MoveStep) error {
	m.calls++
	if m.calls == 2 || m.calls == 4 {
		return &StepPausedError{}
	}
	return nil
}

func (s *behaviorPauseSuite) TestSelectedWorkSurvivesTwoPausesWithoutRerolling() {
	raw, err := os.ReadFile("testdata/behavior-patrol-both.json")
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
	s.Require().NotNil(e.pause.turn.behavior)
	claim := workDataFrom(e.pause.turn.behavior)
	s.Equal(spatial.Position{X: 3}, e.pause.from)
	s.Equal(15, e.pause.turn.budget.MovementFeet)
	s.Equal(1, die.calls)

	for _, expected := range []struct {
		x      float64
		paused bool
		state  string
	}{
		{5, true, "item/0/walk"},
		{6, false, "item/1/walk"},
	} {
		encoded, err := json.Marshal(e.ToData())
		s.Require().NoError(err)
		var saved EncounterData
		s.Require().NoError(json.Unmarshal(encoded, &saved))
		s.Require().NotNil(saved.Pause.Turn.Behavior)
		s.Equal(claim, saved.Pause.Turn.Behavior)
		e, err = LoadEncounter(&LoadEncounterInput{Data: saved, Capabilities: in.Capabilities})
		s.Require().NoError(err)
		_, err = e.Resume(context.Background())
		s.Require().NoError(err)
		cell, err := e.cellOf(e.members["guard"])
		s.Require().NoError(err)
		s.Equal(spatial.Position{X: expected.x}, cell)
		s.Equal(expected.paused, e.Paused())
		s.Equal(expected.state, e.members["guard"].behaviors.composer.ToData().Instances[0].Blob.State)
		s.Equal(1, die.calls, "resume executes held work, it never chooses again")
		if expected.paused {
			s.Equal(5, e.pause.turn.budget.MovementFeet)
			s.Equal(claim, workDataFrom(e.pause.turn.behavior))
		}
	}
}
