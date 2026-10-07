// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

// The factors a target's multiplier answer may carry. The stacking rules
// know these three and no others: immunity wins over everything, resistance
// and vulnerability cancel, and neither stacks.
const (
	// DamageFactorImmunity lets nothing of the type through.
	DamageFactorImmunity = 0.0
	// DamageFactorResistance halves the type, rounding down.
	DamageFactorResistance = 0.5
	// DamageFactorVulnerability doubles the type.
	DamageFactorVulnerability = 2.0
)

// ErrTargetAnswerAltered reports an incoming fold that changed what it was
// handed: the target, the source, the frame, a dealt component, or an answer
// already present when the fold opened. A target answers by appending a
// reduction or a multiplier; it never rewrites the damage dealt to it.
var ErrTargetAnswerAltered = errors.New("incoming damage fold altered what it was handed")

// DamageReduction is one target answer that lowers one damage type by a fixed
// amount before any multiplier applies. Modifier is negative: a reduction
// adds nothing, and damage a source adds belongs on the dealt fold.
//
// It sits on one damage type, the grain the settlement folds on.
type DamageReduction struct {
	// Category is what kind of rule answered: a condition, a feature, a
	// monster trait.
	Category DamageSourceType
	// Source names the rule that answered. Ref is required: the trace line
	// for the reduction names it.
	Source     RollSource
	DamageType damage.Type
	Modifier   int
}

// Validate refuses a reduction the settlement cannot fold: no damage type, no
// named source, or a modifier that does not reduce.
func (r DamageReduction) Validate() error {
	if r.DamageType == "" {
		return fmt.Errorf("damage reduction names no damage type")
	}
	if r.Source.Ref == nil {
		return fmt.Errorf("%s damage reduction names no source", r.DamageType)
	}
	if r.Modifier >= 0 {
		return fmt.Errorf("%s damage reduction from %s is %d, not negative",
			r.DamageType, r.Source.Ref, r.Modifier)
	}
	return nil
}

// DamageMultiplier is one target answer that scales one damage type after
// every other modifier: immunity, resistance or vulnerability. Factor is one
// of [DamageFactorImmunity], [DamageFactorResistance] and
// [DamageFactorVulnerability]; zero is immunity, never "no multiplier".
type DamageMultiplier struct {
	// Category is what kind of rule answered: a condition, a feature, a
	// monster trait.
	Category DamageSourceType
	// Source names the rule that answered. Ref is required: the trace line
	// for a multiplied type names the multiplier that decided it.
	Source     RollSource
	DamageType damage.Type
	Factor     float64
}

// Validate refuses a multiplier the stacking rules cannot fold: no damage
// type, no named source, or a factor that is not one of the three.
func (m DamageMultiplier) Validate() error {
	if m.DamageType == "" {
		return fmt.Errorf("damage multiplier names no damage type")
	}
	if m.Source.Ref == nil {
		return fmt.Errorf("%s damage multiplier names no source", m.DamageType)
	}
	switch m.Factor {
	case DamageFactorImmunity, DamageFactorResistance, DamageFactorVulnerability:
		return nil
	default:
		return fmt.Errorf("%s damage multiplier from %s has factor %v, not immunity, resistance or vulnerability",
			m.DamageType, m.Source.Ref, m.Factor)
	}
}

// IncomingDamageInput names what the target step hands the incoming fold.
type IncomingDamageInput struct {
	// TargetID is the member receiving the damage.
	TargetID string
	// SourceID is the acting member who dealt it, the frame's actor.
	SourceID string
	// Dealt is what the source deals after the dealt fold and the save's
	// halving: damage only, no multiplier component.
	Dealt []DamageComponent
	// Frame is the action's frame with the target known.
	Frame contributions.Frame
	// Reductions and Multipliers are target answers settled before the fold
	// opens, such as a damage-changing reaction rolled by the machine that
	// held its window. Usually empty.
	Reductions  []DamageReduction
	Multipliers []DamageMultiplier
}

// IncomingDamageEvent is the target's half of received damage: the damage
// dealt to it, read-only, and the answers its own rules give. It is published
// on [IncomingDamageChain] by the target step, after the dealt fold and the
// save's halving and before combat's settlement.
//
// The target, source, frame and dealt components are read through methods
// that return copies, so a subscriber cannot rewrite them in place. A
// subscriber answers by appending to Reductions or Multipliers; the step
// refuses a fold that returned anything else, with [IncomingDamageEvent.CheckUnaltered].
//
// Never persisted, like [DamageChainEvent]: a Fact does not marshal.
type IncomingDamageEvent struct {
	targetID string
	sourceID string
	dealt    []DamageComponent
	frame    contributions.Frame

	// Reductions are the target's fixed reductions, one damage type each.
	Reductions []DamageReduction
	// Multipliers are the target's immunities, resistances and
	// vulnerabilities, in fold order. Fold order decides which multiplier a
	// trace line names when more than one carries the effective factor.
	Multipliers []DamageMultiplier
}

// NewIncomingDamageEvent builds the event the target step publishes.
//
// Errors: an empty target or source; a frame that is invalid, whose actor is
// not the source, or whose target is not known as the target; a dealt
// component with no damage type or carrying a multiplier (a target answer on
// the dealt side); or a malformed answer.
func NewIncomingDamageEvent(input IncomingDamageInput) (*IncomingDamageEvent, error) {
	if input.TargetID == "" {
		return nil, fmt.Errorf("incoming damage names no target")
	}
	if input.SourceID == "" {
		return nil, fmt.Errorf("incoming damage on %s names no source", input.TargetID)
	}
	if err := input.Frame.Validate(); err != nil {
		return nil, fmt.Errorf("incoming damage on %s: %w", input.TargetID, err)
	}
	if input.Frame.Actor != input.SourceID {
		return nil, fmt.Errorf("incoming damage on %s: frame actor %q is not the source %q",
			input.TargetID, input.Frame.Actor, input.SourceID)
	}
	if target, known := input.Frame.Target.Get(); !known || target != input.TargetID {
		return nil, fmt.Errorf("incoming damage on %s: frame does not know the target", input.TargetID)
	}
	for i, component := range input.Dealt {
		if component.DamageType == "" {
			return nil, fmt.Errorf("incoming damage on %s: dealt component %d names no damage type",
				input.TargetID, i)
		}
		if component.Multiplier != nil {
			return nil, fmt.Errorf("incoming damage on %s: dealt component %d carries a multiplier; "+
				"a target's answer belongs on the incoming fold", input.TargetID, i)
		}
	}
	for _, reduction := range input.Reductions {
		if err := reduction.Validate(); err != nil {
			return nil, fmt.Errorf("incoming damage on %s: %w", input.TargetID, err)
		}
	}
	for _, multiplier := range input.Multipliers {
		if err := multiplier.Validate(); err != nil {
			return nil, fmt.Errorf("incoming damage on %s: %w", input.TargetID, err)
		}
	}
	return &IncomingDamageEvent{
		targetID:    input.TargetID,
		sourceID:    input.SourceID,
		dealt:       CloneDamageComponents(input.Dealt),
		frame:       input.Frame.Clone(),
		Reductions:  cloneReductions(input.Reductions),
		Multipliers: cloneMultipliers(input.Multipliers),
	}, nil
}

// TargetID is the member receiving the damage.
func (e *IncomingDamageEvent) TargetID() string { return e.targetID }

// SourceID is the acting member who dealt the damage.
func (e *IncomingDamageEvent) SourceID() string { return e.sourceID }

// Frame returns a copy of the action's frame, the target known. A target rule
// that depends on how the damage arrived — a weapon attack, a saving throw —
// reads this, never a component's source stamp.
func (e *IncomingDamageEvent) Frame() contributions.Frame { return e.frame.Clone() }

// Dealt returns a copy of the dealt components. Changing the copy changes
// nothing the step settles.
func (e *IncomingDamageEvent) Dealt() []DamageComponent { return CloneDamageComponents(e.dealt) }

// DealtTypes lists each damage type dealt, once, in the order the dealt
// components first name it. A target answer names one of these.
func (e *IncomingDamageEvent) DealtTypes() []damage.Type {
	var types []damage.Type
	for _, component := range e.dealt {
		if !slices.Contains(types, component.DamageType) {
			types = append(types, component.DamageType)
		}
	}
	return types
}

// Clone returns an independently owned copy. The target step publishes one
// event and executes the fold over its clone, so the event it sent stays what
// it sent and [IncomingDamageEvent.CheckUnaltered] has something to compare
// against.
func (e *IncomingDamageEvent) Clone() *IncomingDamageEvent {
	return &IncomingDamageEvent{
		targetID:    e.targetID,
		sourceID:    e.sourceID,
		dealt:       CloneDamageComponents(e.dealt),
		frame:       e.frame.Clone(),
		Reductions:  cloneReductions(e.Reductions),
		Multipliers: cloneMultipliers(e.Multipliers),
	}
}

// CheckUnaltered refuses a folded event that is not sent plus appended
// answers: the target, source, frame and dealt components must be sent's, and
// sent's own answers must open each answer list unchanged. Every appended
// answer must be well formed. Errors wrap [ErrTargetAnswerAltered], or name
// the malformed answer.
func (e *IncomingDamageEvent) CheckUnaltered(sent *IncomingDamageEvent) error {
	if e == nil || sent == nil {
		return fmt.Errorf("%w: an incoming damage event is missing", ErrTargetAnswerAltered)
	}
	if e.targetID != sent.targetID || e.sourceID != sent.sourceID {
		return fmt.Errorf("%w: target or source changed", ErrTargetAnswerAltered)
	}
	if !reflect.DeepEqual(e.frame, sent.frame) {
		return fmt.Errorf("%w: frame changed", ErrTargetAnswerAltered)
	}
	if !reflect.DeepEqual(e.dealt, sent.dealt) {
		return fmt.Errorf("%w: a dealt component changed", ErrTargetAnswerAltered)
	}
	if !opensWith(e.Reductions, sent.Reductions) {
		return fmt.Errorf("%w: a reduction present before the fold changed", ErrTargetAnswerAltered)
	}
	if !opensWith(e.Multipliers, sent.Multipliers) {
		return fmt.Errorf("%w: a multiplier present before the fold changed", ErrTargetAnswerAltered)
	}
	for _, reduction := range e.Reductions[len(sent.Reductions):] {
		if err := reduction.Validate(); err != nil {
			return err
		}
	}
	for _, multiplier := range e.Multipliers[len(sent.Multipliers):] {
		if err := multiplier.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// opensWith reports whether answers begins with every one of before,
// unchanged. An empty before opens every list.
func opensWith[T any](answers, before []T) bool {
	if len(answers) < len(before) {
		return false
	}
	return slices.EqualFunc(answers[:len(before)], before, func(a, b T) bool {
		return reflect.DeepEqual(a, b)
	})
}

// CloneDamageComponents returns an independently owned copy of components:
// every roll fact, property list and multiplier pointer is the copy's own.
func CloneDamageComponents(components []DamageComponent) []DamageComponent {
	if components == nil {
		return nil
	}
	clones := make([]DamageComponent, len(components))
	for i, component := range components {
		clones[i] = component
		clones[i].Roll = RollComponent{
			Source:       cloneRollSource(component.Roll.Source),
			Dice:         cloneDiceTrace(component.Roll.Dice),
			Modifier:     cloneInt(component.Roll.Modifier),
			SubtractDice: component.Roll.SubtractDice,
		}
		clones[i].Properties = slices.Clone(component.Properties)
		if component.Multiplier != nil {
			factor := *component.Multiplier
			clones[i].Multiplier = &factor
		}
	}
	return clones
}

func cloneReductions(reductions []DamageReduction) []DamageReduction {
	if reductions == nil {
		return nil
	}
	clones := make([]DamageReduction, len(reductions))
	for i, reduction := range reductions {
		clones[i] = reduction
		clones[i].Source = cloneRollSource(reduction.Source)
	}
	return clones
}

func cloneMultipliers(multipliers []DamageMultiplier) []DamageMultiplier {
	if multipliers == nil {
		return nil
	}
	clones := make([]DamageMultiplier, len(multipliers))
	for i, multiplier := range multipliers {
		clones[i] = multiplier
		clones[i].Source = cloneRollSource(multiplier.Source)
	}
	return clones
}
