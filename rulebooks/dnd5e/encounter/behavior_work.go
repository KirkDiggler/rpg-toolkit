// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ErrBadBehaviorWork is a claim inconsistent with its original configuration.
var ErrBadBehaviorWork = errors.New("encounter: invalid behavior work")

// BehaviorWorkKind states the evidence required to complete selected work.
type BehaviorWorkKind string

const (
	// BehaviorArrival requires actual standing on the selected exact goal.
	BehaviorArrival BehaviorWorkKind = "arrival"
	// BehaviorDelivered completes a successfully delivered line or explicit hold.
	BehaviorDelivered BehaviorWorkKind = "delivered"
)

// BehaviorWork is the encounter's claim on a selected mapped answer. It does
// not own machine progress or grant another action opportunity on completion.
type BehaviorWork struct {
	Instance   string
	Definition string
	State      string
	Key        behavior.AnswerKey
	On         machine.Beat
	Kind       BehaviorWorkKind
	Goal       *spatial.Position
	// DecisionRecord is diagnostic correlation, never completion authority.
	DecisionRecord string
}

// BehaviorWorkData persists the same selected work through a held turn.
type BehaviorWorkData struct {
	Instance       string             `json:"instance"`
	Definition     string             `json:"definition"`
	State          string             `json:"state"`
	Key            behavior.AnswerKey `json:"key"`
	On             machine.Beat       `json:"on"`
	Kind           BehaviorWorkKind   `json:"kind"`
	Goal           *PositionData      `json:"goal,omitempty"`
	DecisionRecord string             `json:"decision_record,omitempty"`
}

type completionShape struct {
	Kind BehaviorWorkKind
	Goal *spatial.Position
}

func completionOf(row Answer) (completionShape, error) {
	var goal *spatial.Position
	switch {
	case row.Toward != nil && row.Toward.At != nil:
		goal = row.Toward.At
	case row.Dash != nil && row.Dash.At != nil:
		goal = row.Dash.At
	case row.Hold:
		return completionShape{Kind: BehaviorDelivered}, nil
	case row.Say != "" && row.Attack == nil && row.Toward == nil && row.Dash == nil && row.Away == nil && row.Search == nil && !row.Flee:
		return completionShape{Kind: BehaviorDelivered}, nil
	default:
		return completionShape{}, fmt.Errorf("mapped answer requires exact-cell movement, a line or explicit hold: %w", ErrBadBehaviorWork)
	}
	cell := *goal
	return completionShape{Kind: BehaviorArrival, Goal: &cell}, nil
}

func validateCompletionRows(rows []Answer) error {
	for _, row := range rows {
		if len(row.Rows) > 0 {
			if err := validateCompletionRows(row.Rows); err != nil {
				return err
			}
			continue
		}
		if _, err := completionOf(row); err != nil {
			return err
		}
	}
	return nil
}

type behaviorWorkSelection struct{ Work *BehaviorWork }

func (b *behaviorRuntime) workFor(chosen *behavior.DecideOutput) (behaviorWorkSelection, error) {
	if chosen.Pick == nil || chosen.Pick.Entry < 0 {
		return behaviorWorkSelection{}, nil
	}
	bound := b.input.Instances[b.byID[chosen.Instance]]
	on, exists := bound.Completions[chosen.Pick.Key]
	if !exists {
		return behaviorWorkSelection{}, nil
	}
	shape, err := completionOf(chosen.Pick.Answer)
	if err != nil {
		return behaviorWorkSelection{}, err
	}
	state, err := b.composer.StateOf(&behavior.InstanceStateInput{ID: chosen.Instance})
	if err != nil {
		return behaviorWorkSelection{}, err
	}
	return behaviorWorkSelection{Work: &BehaviorWork{Instance: chosen.Instance,
		Definition: state.Definition, State: state.State, Key: chosen.Pick.Key,
		On: on, Kind: shape.Kind, Goal: shape.Goal}}, nil
}

func (b *behaviorRuntime) validateWork(work *BehaviorWork) error {
	if work == nil {
		return nil
	}
	index, exists := b.byID[work.Instance]
	if !exists {
		return fmt.Errorf("attachment %q: %w", work.Instance, ErrBadBehaviorWork)
	}
	bound := b.input.Instances[index]
	state, err := b.composer.StateOf(&behavior.InstanceStateInput{ID: work.Instance})
	if err != nil {
		return err
	}
	if work.Definition != state.Definition || work.On == "" || bound.Completions[work.Key] != work.On {
		return fmt.Errorf("configuration/completion of %q: %w", work.Instance, ErrBadBehaviorWork)
	}
	declared, exists := bound.Instance.Recipe.States[work.State]
	if !exists {
		return fmt.Errorf("state %q: %w", work.State, ErrBadBehaviorWork)
	}
	listens := false
	for _, transition := range declared.Transitions {
		listens = listens || transition.On == work.On
	}
	if !listens || !rowsCompleteAs(bound.Instance.Table[work.Key], work) {
		return fmt.Errorf("state/goal of %q: %w", work.Instance, ErrBadBehaviorWork)
	}
	return nil
}

func rowsCompleteAs(rows []Answer, work *BehaviorWork) bool {
	for _, row := range rows {
		if len(row.Rows) > 0 {
			if rowsCompleteAs(row.Rows, work) {
				return true
			}
			continue
		}
		shape, err := completionOf(row)
		if err != nil || shape.Kind != work.Kind {
			continue
		}
		if shape.Goal == nil && work.Goal == nil || shape.Goal != nil && work.Goal != nil && *shape.Goal == *work.Goal {
			return true
		}
	}
	return false
}

func (e *Encounter) completeBehaviorWork(m *memberRecord, work *BehaviorWork, executionAct string) error {
	if work == nil {
		return nil
	}
	if err := m.behaviors.validateWork(work); err != nil {
		return err
	}
	state, err := m.behaviors.composer.StateOf(&behavior.InstanceStateInput{ID: work.Instance})
	if err != nil {
		return err
	}
	if state.State != work.State {
		// An inactive step's old completion cannot advance another step.
		return e.appendBehaviorWork(&behaviorWorkRecordInput{member: m, work: work, status: behaviorWorkAbandoned, reason: "state_changed", act: executionAct})
	}
	down, err := e.downNow()
	if err != nil {
		return err
	}
	if down[m.ID] {
		return e.appendBehaviorWork(&behaviorWorkRecordInput{member: m, work: work, status: behaviorWorkAbandoned, reason: "inactive", act: executionAct})
	}
	if work.Kind == BehaviorArrival {
		cell, err := e.cellOf(m)
		if err != nil {
			return err
		}
		if cell != *work.Goal {
			// Turn-end, a blocked route or a partial walk is not arrival.
			return e.appendBehaviorWork(&behaviorWorkRecordInput{member: m, work: work, status: behaviorWorkPartial, reason: "goal_not_reached", act: executionAct})
		}
	}
	now := uint64(e.clock.ToData().HighWater)
	out, err := m.behaviors.composer.Observe(&behavior.ObserveInput{
		Instance: work.Instance, On: work.On, Env: e.envFor(m.ID), Now: now, Trace: true,
	})
	if err != nil {
		return err // completion never retries with Decide
	}
	if err := e.behaviorStepped(m, out.Steps); err != nil {
		return err
	}
	if _, err := e.appendBehaviorDecision(&behaviorDecisionRecordInput{member: m, act: executionAct,
		trace: &behaviorDecisionTrace{on: work.On, now: now, operation: behaviorTraceComplete, outcomes: out.Outcomes}}); err != nil {
		return err
	}
	return e.appendBehaviorWork(&behaviorWorkRecordInput{member: m, work: work, status: behaviorWorkCompleted, act: executionAct})
}

func workDataFrom(work *BehaviorWork) *BehaviorWorkData {
	if work == nil {
		return nil
	}
	out := &BehaviorWorkData{Instance: work.Instance, Definition: work.Definition, State: work.State,
		Key: work.Key, On: work.On, Kind: work.Kind, DecisionRecord: work.DecisionRecord}
	if work.Goal != nil {
		out.Goal = &PositionData{X: work.Goal.X, Y: work.Goal.Y}
	}
	return out
}

func workFromData(data *BehaviorWorkData) *BehaviorWork {
	if data == nil {
		return nil
	}
	out := &BehaviorWork{Instance: data.Instance, Definition: data.Definition, State: data.State,
		Key: data.Key, On: data.On, Kind: data.Kind, DecisionRecord: data.DecisionRecord}
	if data.Goal != nil {
		out.Goal = &spatial.Position{X: data.Goal.X, Y: data.Goal.Y}
	}
	return out
}
