// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	json "encoding/json/v2"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

const (
	// BeatBehaviorConfig describes the exact bound configuration for diagnostics.
	BeatBehaviorConfig = "behavior_config"
	// BeatBehaviorDecision records actual evaluation and composer dispositions.
	BeatBehaviorDecision = "behavior_decision"
	// BeatBehaviorWork records execution evidence, never a selection as arrival.
	BeatBehaviorWork = "behavior_work"
)

const (
	behaviorTraceDecide   = "decide"
	behaviorTraceObserve  = "observe"
	behaviorTraceComplete = "complete"
	behaviorTraceDriver   = "driver"
	behaviorWorkSelected  = "selected"
	behaviorWorkPartial   = "partial"
	behaviorWorkPaused    = "paused"
	behaviorWorkCompleted = "completed"
	behaviorWorkAbandoned = "abandoned"
)

// behaviorDecisionTrace holds returned values only: no event/env object escapes
// into the record. A choice is recorded under its own act by runTurnIntents.
type behaviorDecisionTrace struct {
	on        machine.Beat
	now       uint64
	operation string
	outcomes  []behavior.InstanceOutcome
}

type behaviorRecordInput struct {
	member *memberRecord
	kind   string
	act    string
	fields map[string]any
}

func (e *Encounter) appendBehaviorRecord(in *behaviorRecordInput) (*record.AppendOutput, error) {
	in.fields["beat"], in.fields["schema"] = in.kind, 1
	in.fields["member"], in.fields["config"] = in.member.ID, in.member.behaviors.definition
	payload, err := json.Marshal(in.fields, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("behavior record: %w", err)
	}
	// Empty audience is intentional: these are GM/debug records, not facts
	// any player acquires by seeing an NPC. RecordService is the host gate.
	var out *record.AppendOutput
	err = e.underAct(in.act, func() error {
		var appendErr error
		out, appendErr = e.appendBeat(&record.AppendInput{
			At: uint64(e.clock.ToData().HighWater), Tags: map[string]string{"tag": in.kind}, Payload: payload,
		})
		return appendErr
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (e *Encounter) describeBehavior(m *memberRecord) error {
	if m.behaviors == nil || len(m.behaviors.input.Instances) == 0 || m.behaviors.symbolsRecorded {
		return nil
	}
	_, err := e.appendBehaviorRecord(&behaviorRecordInput{member: m, kind: BeatBehaviorConfig,
		fields: map[string]any{"configuration": m.behaviors.input}})
	if err != nil {
		return err
	}
	m.behaviors.symbolsRecorded = true
	return nil
}

type behaviorDecisionRecordInput struct {
	member     *memberRecord
	trace      *behaviorDecisionTrace
	pick       *Pick
	pickRecord string
	act        string
}

type behaviorDecisionRecordOutput struct{ ID string }

func (e *Encounter) appendBehaviorDecision(in *behaviorDecisionRecordInput) (*behaviorDecisionRecordOutput, error) {
	if in.trace == nil {
		return &behaviorDecisionRecordOutput{}, nil
	}
	if err := e.describeBehavior(in.member); err != nil {
		return nil, err
	}
	fields := map[string]any{
		"operation": in.trace.operation, "on": in.trace.on, "now": in.trace.now,
		"outcomes": behaviorOutcomeFields(in.trace.outcomes), "pick_record": in.pickRecord,
	}
	if in.pick != nil && in.pickRecord == "" {
		// The ordinary story suppresses repeated empty holds. Their actual row
		// verdict still belongs in diagnostics, without reevaluating the table.
		fields["selection"] = answeredBeatBody(answeredBy{creature: in.member.ID}, in.pick)
	}
	out, err := e.appendBehaviorRecord(&behaviorRecordInput{member: in.member, kind: BeatBehaviorDecision, act: in.act, fields: fields})
	if err != nil {
		return nil, err
	}
	return &behaviorDecisionRecordOutput{ID: out.ID}, nil
}

type behaviorWorkRecordInput struct {
	member *memberRecord
	work   *BehaviorWork
	status string
	reason string
	act    string
}

func (e *Encounter) appendBehaviorWork(in *behaviorWorkRecordInput) error {
	if in.work == nil {
		return nil
	}
	if err := e.describeBehavior(in.member); err != nil {
		return err
	}
	_, err := e.appendBehaviorRecord(&behaviorRecordInput{member: in.member, kind: BeatBehaviorWork, act: in.act, fields: map[string]any{
		"status": in.status, "reason": in.reason, "decision_record": in.work.DecisionRecord,
		"work": workDataFrom(in.work),
	}})
	return err
}

func behaviorOutcomeFields(outcomes []behavior.InstanceOutcome) []map[string]any {
	out := make([]map[string]any, len(outcomes))
	for i, outcome := range outcomes {
		var step any
		if outcome.Step != nil {
			step = map[string]any{
				"fired": outcome.Step.Step.Fired, "from": outcome.Step.Step.From, "to": outcome.Step.Step.To,
				"reason": outcome.Step.Step.Reason, "acts": outcome.Step.Step.Acts,
				"before": outcome.Step.Trace.Before, "after": outcome.Step.Trace.After,
				"on": outcome.Step.Trace.On, "now": outcome.Step.Trace.Now,
				"transitions": behaviorTransitionFields(outcome.Step.Trace.Transitions),
			}
		}
		out[i] = map[string]any{
			"id": outcome.ID, "definition": outcome.Definition, "before": outcome.Before, "after": outcome.After,
			"requested": outcome.Requested, "selected": outcome.Selected, "committed": outcome.Committed,
			"skipped": outcome.Skipped, "step": step,
		}
	}
	return out
}

func behaviorTransitionFields(transitions []machine.TransitionTrace) []map[string]any {
	out := make([]map[string]any, len(transitions))
	for i, t := range transitions {
		out[i] = map[string]any{"index": t.Index, "on": t.On, "evaluated": t.Evaluated,
			"selected": t.Selected, "guard": behaviorGuardFields(t.Guard)}
	}
	return out
}

func behaviorGuardFields(g machine.GuardTrace) map[string]any {
	children := make([]map[string]any, len(g.Children))
	for i, child := range g.Children {
		children[i] = behaviorGuardFields(child)
	}
	return map[string]any{
		"evaluated": g.Evaluated, "verdict": g.Verdict.String(), "reason": g.Reason,
		"read": g.Read, "known": g.Known, "scalar": g.Scalar, "value": g.Value, "children": children,
	}
}
