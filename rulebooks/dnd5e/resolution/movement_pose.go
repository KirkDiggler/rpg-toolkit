// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// frozenReaction is what a stored reactor answers on a resumed step.
type frozenReaction struct {
	Definition combatActions.Definition `json:"definition"`
	Answer     ReactionAnswer           `json:"answer"`
}

// frozenMovement is a step stopped on one of its reactions. It holds the step,
// its fold and its triggers, and where it stopped — never the reactions that
// already settled, which were told with the pause.
type frozenMovement struct {
	Mover     string           `json:"mover"`
	MoverKind string           `json:"mover_kind,omitempty"`
	From      spatial.Position `json:"from"`
	To        spatial.Position `json:"to"`
	ForcedBy  *core.Ref        `json:"forced_by,omitempty"`

	Folded    *dndEvents.MovementChainEvent    `json:"folded"`
	Triggers  []dndEvents.ReactionTriggerEvent `json:"triggers"`
	Reactions map[string]frozenReaction        `json:"reactions"`
	Index     int                              `json:"index"`
	Inner     json.RawMessage                  `json:"inner"`
}

// storedReactionAttacks answers a resumed step from what was frozen: every
// reactor not stored answers [ReactionNone].
type storedReactionAttacks map[string]frozenReaction

// AttackFor implements [ReactionAttacks] over the frozen answers.
func (a storedReactionAttacks) AttackFor(id string) (combatActions.Definition, ReactionAnswer) {
	r, ok := a[id]
	if !ok {
		return combatActions.Definition{}, ReactionNone
	}
	return r.Definition, r.Answer
}

// freezeMovement turns the paused reaction's pause into the step's. Kind and
// price are the reaction's. What settled in this call — the reactions before
// it, the paused reaction's own hit when it paused after one, and every player
// asked so far — rides the pause to the driver, so it is told now, once.
func (m *movementMachine) freezeMovement(index int, definition combatActions.Definition, inner Pause) (Step, error) {
	reactions := map[string]frozenReaction{
		m.triggers[index].ReactorID: {Definition: definition, Answer: ReactionSwing},
	}
	for _, trigger := range m.triggers[index+1:] {
		if d, answer := m.in.Reactions.AttackFor(trigger.ReactorID); answer != ReactionNone {
			reactions[trigger.ReactorID] = frozenReaction{Definition: d, Answer: answer}
		}
	}
	raw, err := writeFrozen(machineMovement, inner.Kind, frozenMovement{
		Mover: string(m.in.Mover), MoverKind: m.in.MoverKind, From: m.in.From, To: m.in.To,
		ForcedBy: cloneCoreRef(m.in.ForcedBy), Folded: m.folded, Triggers: m.triggers,
		Reactions: reactions, Index: index, Inner: inner.Frozen,
	})
	if err != nil {
		return nil, err
	}
	return Pause{Kind: inner.Kind, Ask: inner.Ask, Cost: inner.Cost, Frozen: raw,
		settled: m.settledWith(index, definition, inner.settled)}, nil
}

// settledWith is the step as it settled in this call, with the paused
// reaction's own settled half appended when it has one. Nil when nothing
// settled and nobody was asked.
func (m *movementMachine) settledWith(index int, definition combatActions.Definition, inner Outcome) Outcome {
	out := m.outcome()
	out.Reactions = append([]ReactionOutcome(nil), m.reactions...)
	if struck, ok := inner.(StrikeOutcome); ok {
		out.Reactions = append(out.Reactions, m.reactionOf(m.triggers[index], definition, struck))
	}
	if len(out.Reactions) == 0 && len(out.Asked) == 0 {
		return nil
	}
	return out
}

// resumeMovement continues the reactions on a frozen step without announcing
// the step again or repeating reactions which already resolved.
func resumeMovement(h frozenHeader, in *ResumeInput) (Machine, error) {
	var f frozenMovement
	if err := decodeState(h, &f); err != nil {
		return nil, err
	}
	if f.Mover == "" || f.Folded == nil || f.Index < 0 || f.Index >= len(f.Triggers) {
		return nil, fmt.Errorf("%w: invalid movement continuation", ErrBadFrozen)
	}
	resumed, err := resumeInner(f.Inner, in, strikeMachines...)
	if err != nil {
		return nil, err
	}
	return &movementMachine{
		in: &MovementInput{
			Mover: encounter.MemberID(f.Mover), MoverKind: f.MoverKind, From: f.From, To: f.To,
			ForcedBy: f.ForcedBy, Reactions: storedReactionAttacks(f.Reactions), Roller: in.Roller,
		},
		folded: f.Folded, triggers: f.Triggers, resumed: resumed, resumeIndex: f.Index,
	}, nil
}

// frozenOpportunity is a player asked whether to swing at a mover: the step,
// the trigger that offered the swing, the attack, and its price.
type frozenOpportunity struct {
	Mover      string                         `json:"mover"`
	MoverKind  string                         `json:"mover_kind,omitempty"`
	From       spatial.Position               `json:"from"`
	To         spatial.Position               `json:"to"`
	ForcedBy   *core.Ref                      `json:"forced_by,omitempty"`
	Reactor    string                         `json:"reactor"`
	Trigger    dndEvents.ReactionTriggerEvent `json:"trigger"`
	Definition combatActions.Definition       `json:"definition"`
	Cost       *Cost                          `json:"cost"`
}

// askOpportunity poses one player's opportunity attack as a pause, at the
// price the one table states. The offer names its source: the trigger's
// condition ref and display name.
func (m *movementMachine) askOpportunity(
	trigger dndEvents.ReactionTriggerEvent, definition combatActions.Definition,
) (Pause, error) {
	ref, err := core.ParseString(trigger.ConditionRef)
	if err != nil {
		return Pause{}, fmt.Errorf("%w: reactor %q offered by %q: %w",
			ErrBadMovement, trigger.ReactorID, trigger.ConditionRef, err)
	}
	step := m.outcome()
	cost := reactionCost(trigger.ReactorID, "")
	frozen, err := writeFrozen(machineOpportunity, PauseOpportunity, frozenOpportunity{
		Mover: step.Mover, MoverKind: m.in.MoverKind, From: step.From, To: step.To,
		ForcedBy: cloneCoreRef(m.in.ForcedBy), Reactor: trigger.ReactorID, Trigger: trigger,
		Definition: definition, Cost: cost,
	})
	if err != nil {
		return Pause{}, err
	}
	return Pause{
		Kind:   PauseOpportunity,
		Ask:    Ask{Audience: trigger.ReactorID, Offer: Offer{Ref: *ref, Name: trigger.Name}},
		Cost:   cost,
		Frozen: frozen,
	}, nil
}

// resumeOpportunity answers an asked opportunity attack. Taken, the reactor
// swings at the frozen step — announced once already, when the question was
// asked, so not announced again — and pays the frozen price at the one door
// once the swing resolves; nobody else reacts. Declined, the step finishes
// with nothing to report and nothing is charged.
func resumeOpportunity(h frozenHeader, in *ResumeInput) (Machine, error) {
	var f frozenOpportunity
	if err := decodeState(h, &f); err != nil {
		return nil, err
	}
	if err := samePrice(f.Cost, in.Pause.Cost); err != nil {
		return nil, err
	}
	if f.Mover == "" || f.Reactor == "" || f.Trigger.ReactorID != f.Reactor || f.Trigger.SourceEntity != f.Mover || f.Cost == nil {
		return nil, fmt.Errorf("%w: invalid opportunity", ErrBadFrozen)
	}
	if !in.Answer.Taken() {
		return startedMachine{first: Done{Outcome: MovementOutcome{Mover: f.Mover, From: f.From, To: f.To}}}, nil
	}
	return &movementMachine{
		in: &MovementInput{
			Mover: encounter.MemberID(f.Mover), MoverKind: f.MoverKind, From: f.From, To: f.To,
			ForcedBy: f.ForcedBy, Roller: in.Roller,
			Reactions: storedReactionAttacks{f.Reactor: {Definition: f.Definition, Answer: ReactionSwing}},
		},
		triggers: []dndEvents.ReactionTriggerEvent{f.Trigger},
		answered: true,
		price:    f.Cost,
	}, nil
}
