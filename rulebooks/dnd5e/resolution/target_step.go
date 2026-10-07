// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// The labels the trace's own lines carry, so a player reading the breakdown
// sees why the number moved. A dealt line keeps the label its rule gave it.
const (
	reducedLabel = "reduced"
	flooredLabel = "cannot fall below zero"
)

// targetStepInput is what one damage source hands the target step: what it
// rolled, how to halve it, and what to do once the target has taken it.
type targetStepInput struct {
	// Dealt is the dealt fold's event before it folds: the source's rolled
	// components and the action's frame, the target known.
	Dealt *dnd5eEvents.DamageChainEvent

	// Halving is set when a made save meets a Half gate, and names the save's
	// cause for the halving line. Nil for every other delivery.
	Halving *damageHalving

	Cast       *Participants
	IsCritical bool

	// Apply is the sheet application. Nil applies through the target's own
	// sheet ([applyToSheet]).
	Apply applyDamageFunc

	// Cause is what the damage taken report says dealt the damage.
	Cause  dnd5eEvents.SaveCause
	Roller dice.Roller

	// Received records what the target received, before the report runs.
	Received func(receivedDamage)
	// FollowUp records each check the report came back with.
	FollowUp func(FollowUpOutcome)
	// Then runs after the last follow-up.
	Then func(context.Context) (Step, error)
}

// damageHalving is a made save's halving: the cause whose save was made and
// the name its line on the trace carries.
type damageHalving struct {
	Cause      dnd5eEvents.SaveCause
	SourceName string
}

// receivedDamage is what one target received from one damage source, in the
// one shape a strike and a contest both report.
type receivedDamage struct {
	// Trace is every line that explains the number: the dealt components, the
	// halving, the target's reductions, a floor where a type fell below zero,
	// and one line per multiplied type naming the multiplier that decided it.
	// No line carries a multiplier factor; Calculation totals Requested.
	Trace       []dnd5eEvents.DamageComponent
	Calculation *dnd5eEvents.RollCalculation

	// Instances are the settlement's landing instances, sorted by type.
	Instances []damage.Instance

	// Requested is what the settlement decided the target takes, before the
	// sheet had a say.
	Requested int

	// Applied is the sheet's own report of the application.
	Applied *combat.ApplyDamageResult
}

// foldDamage is a strike's damage phase: one step that runs the target step
// on this interaction's bus.
func foldDamage(in targetStepInput) Gather {
	return Gather{
		name: "damage chain",
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			return receiveDamage(ctx, bus, in)
		},
	}
}

// receiveDamage is the target step every damage source delivers through: a
// strike's blow and a contest's damage alike. In order:
//
//  1. The dealt fold, on [dnd5eEvents.DamageChain]: the source's rules add
//     what it deals. Nothing the target answers is folded here.
//  2. A made save's halving, when one met a Half gate ([halveDamage]).
//  3. The incoming fold, on [dnd5eEvents.IncomingDamageChain]: one clone of
//     the event sent is published and the fold executes over another, so the
//     target's rules append reductions and multipliers and cannot touch what
//     was sent. A fold that changed what it was handed is refused.
//  4. Combat's settlement ([combat.SettleDamage]), per damage type.
//  5. The trace ([receivedTrace]), and the refusal to apply a number it does
//     not explain.
//  6. The sheet applies the landing instances.
//  7. The damage taken report, and its follow-ups.
//
// The step is a fold and poses no question: a reaction that changes the damage
// answers in a window that closes before it opens.
//
// Errors: the dealt or incoming fold failing, a halving that cannot be built
// ([halveDamage]), an incoming fold that altered what it was handed (wrapping
// [dnd5eEvents.ErrTargetAnswerAltered]), a malformed target answer (wrapping
// [contributions.ErrRuleCannotAnswer] and [dnd5eEvents.ErrMalformedTargetAnswer]),
// a settlement combat refuses, or a
// trace that does not explain the settled number ([ErrBadAction]).
func receiveDamage(ctx context.Context, bus events.EventBus, in targetStepInput) (Step, error) {
	target, err := combatantFor(in.Cast, in.Dealt.TargetID)
	if err != nil {
		return nil, err
	}
	frame := in.Dealt.Frame.Clone()

	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.DamageChain.On(bus).PublishWithChain(ctx, in.Dealt, chain)
	if err != nil {
		return nil, fmt.Errorf("publish damage chain: %w", err)
	}
	folded, err := modified.Execute(ctx, in.Dealt)
	if err != nil {
		return nil, fmt.Errorf("execute damage chain: %w", err)
	}

	dealt := dnd5eEvents.CloneDamageComponents(folded.Components)
	if in.Halving != nil {
		dealt, err = halveDamage(dealt, in.Halving.Cause, in.Halving.SourceName)
		if err != nil {
			return nil, err
		}
	}

	sent, err := dnd5eEvents.NewIncomingDamageEvent(dnd5eEvents.IncomingDamageInput{
		TargetID: in.Dealt.TargetID,
		SourceID: in.Dealt.AttackerID,
		Dealt:    dealt,
		Frame:    frame,
	})
	if err != nil {
		return nil, targetAnswerError(fmt.Errorf("incoming damage: %w", err))
	}
	// sent never leaves this function: subscribers are handed one clone and
	// the fold runs over another, so nothing a handler does can rewrite what
	// CheckUnaltered compares against.
	incoming := events.NewStagedChain[*dnd5eEvents.IncomingDamageEvent](combat.ModifierStages)
	answering, err := dnd5eEvents.IncomingDamageChain.On(bus).PublishWithChain(ctx, sent.Clone(), incoming)
	if err != nil {
		return nil, fmt.Errorf("publish incoming damage: %w", err)
	}
	answered, err := answering.Execute(ctx, sent.Clone())
	if err != nil {
		return nil, fmt.Errorf("execute incoming damage: %w", err)
	}
	if err := answered.CheckUnaltered(sent); err != nil {
		return nil, targetAnswerError(fmt.Errorf("incoming damage on %q: %w", in.Dealt.TargetID, err))
	}

	settlement, err := combat.SettleDamage(&combat.SettleDamageInput{
		Dealt:       sent.Dealt(),
		Reductions:  answered.Reductions,
		Multipliers: answered.Multipliers,
	})
	if err != nil {
		return nil, targetAnswerError(fmt.Errorf("settle damage on %q: %w", in.Dealt.TargetID, err))
	}
	trace := receivedTrace(sent.Dealt(), answered.Reductions, settlement)
	calculation, err := damageCalculation(trace)
	if err != nil {
		return nil, err
	}
	final, total := settlement.FinalDamage()
	if total != calculation.Total {
		// A record built from this would show lines that do not add up to the
		// damage the target took.
		return nil, fmt.Errorf("%w: %q settled at %d, and the damage trace explains %d",
			ErrBadAction, in.Dealt.TargetID, total, calculation.Total)
	}

	received := receivedDamage{
		Trace:       trace,
		Calculation: calculation,
		Instances:   make([]damage.Instance, 0, len(final)),
		Requested:   total,
	}
	instances := make([]combat.DamageInstance, 0, len(final))
	for _, instance := range final {
		received.Instances = append(received.Instances, damage.Instance{Amount: instance.Amount, Type: instance.Type})
		instances = append(instances, combat.DamageInstance{Amount: instance.Amount, Type: string(instance.Type)})
	}

	apply := in.Apply
	if apply == nil {
		apply = applyToSheet
	}
	// Bus-free: applying damage is the sheet's own business, and the same
	// call for every source, so a blow that drops somebody to zero flows
	// through the death-save and downed transitions the sheet owns.
	received.Applied = apply(ctx, target, &combat.ApplyDamageInput{
		Instances:  instances,
		IsCritical: in.IsCritical,
	})
	if in.Received != nil {
		in.Received(received)
	}

	return reportDamage(reportDamageInput{
		MemberID:      in.Dealt.TargetID,
		Amount:        received.Applied.TotalDamage,
		DamageType:    primaryDamageType(received.Instances),
		DroppedToZero: received.Applied.PreviousHP > 0 && received.Applied.CurrentHP == 0,
		Cause:         in.Cause,
	}, func(reported context.Context, ups []dnd5eEvents.FollowUp) (Step, error) {
		record := in.FollowUp
		if record == nil {
			record = func(FollowUpOutcome) {}
		}
		return runFollowUps(reported, ups, 0, in.Roller, record, in.Then)
	}), nil
}

// targetAnswerError classifies a refusal caused by a target rule's malformed
// answer ([dnd5eEvents.ErrMalformedTargetAnswer]) as a rule that cannot
// answer ([contributions.ErrRuleCannotAnswer]): the defect is in the rule
// that answered, not in the world or the action. Every other error passes
// through unchanged.
func targetAnswerError(err error) error {
	if errors.Is(err, dnd5eEvents.ErrMalformedTargetAnswer) {
		return fmt.Errorf("%w: %w", contributions.ErrRuleCannotAnswer, err)
	}
	return err
}

// receivedTrace builds the one trace from the settlement: the dealt
// components as they were sent, one line per target reduction in fold order,
// then per settled type a floor line when its total fell below zero and one
// line naming the multiplier that decided it, carrying the change it made.
// Each type's lines sum to what it took, so the trace totals the settlement.
//
// A type falls below zero two ways: its reductions sank it, and the floor line
// names the first reduction on it; or its dealt total was negative (a 1 rolled
// beside a -3 modifier) with no reduction at all, and the floor line names the
// last dealt component of that type. Either way the type lands at zero.
//
// A multiplied type gets its line even when the change is zero — immunity to
// a type already floored is still the rule that decided it. A type whose
// factors cancel has no deciding multiplier and no line.
func receivedTrace(
	dealt []dnd5eEvents.DamageComponent, reductions []dnd5eEvents.DamageReduction,
	settlement *combat.SettleDamageOutput,
) []dnd5eEvents.DamageComponent {
	trace := dnd5eEvents.CloneDamageComponents(dealt)
	floorSource := make(map[damage.Type]dnd5eEvents.DamageComponent)
	for _, component := range dealt {
		floorSource[component.DamageType] = component
	}
	reduced := make(map[damage.Type]bool)
	for _, reduction := range reductions {
		if !reduced[reduction.DamageType] {
			reduced[reduction.DamageType] = true
			floorSource[reduction.DamageType] = dnd5eEvents.DamageComponent{
				Source: reduction.Category, Roll: dnd5eEvents.RollComponent{Source: reduction.Source},
			}
		}
		trace = append(trace, answerLine(reduction.Category, reduction.Source, reducedLabel,
			reduction.DamageType, reduction.Modifier))
	}

	for _, settled := range settlement.Types {
		if settled.Floor != 0 {
			source := floorSource[settled.Type]
			trace = append(trace, answerLine(source.Source, source.Roll.Source, flooredLabel,
				settled.Type, settled.Floor))
		}
		if settled.DecidedBy != nil {
			trace = append(trace, answerLine(settled.DecidedBy.Category, settled.DecidedBy.Source,
				multipliedLabel(settled.Factor), settled.Type, settled.Change))
		}
	}

	return trace
}

// answerLine is one modifier-only trace line for a target's answer, sourced
// to the rule that gave it.
func answerLine(
	category dnd5eEvents.DamageSourceType, source dnd5eEvents.RollSource, label string,
	damageType damage.Type, amount int,
) dnd5eEvents.DamageComponent {
	line := dnd5eEvents.CloneRollSource(source)
	line.Label = label
	return dnd5eEvents.DamageComponent{
		Source:     category,
		Roll:       dnd5eEvents.RollComponent{Source: line, Modifier: &amount},
		DamageType: damageType,
	}
}

// multipliedLabel is what a multiplier's line on the trace calls itself.
func multipliedLabel(factor float64) string {
	switch factor {
	case dnd5eEvents.DamageFactorImmunity:
		return "immune"
	case dnd5eEvents.DamageFactorResistance:
		return "resisted"
	default:
		return "vulnerable"
	}
}
