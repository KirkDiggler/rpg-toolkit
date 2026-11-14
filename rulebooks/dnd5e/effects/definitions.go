// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

// Definition projects typed, unbound content to canonical definition data.
// Projection is pure and finite: no world lookup, binding, payment or runtime
// state. Compile propagates projection errors and retains only owned data,
// never this interface or a running callback.
type Definition interface {
	DefinitionData() (DefinitionData, error)
}

// SignedDice describes an unresolved homogeneous pool and its explicit polarity.
// Notation contains no sign or fixed modifier; construction never rolls it.
type SignedDice struct {
	Notation string
	Polarity Polarity
}

// AddToRoll contributes signed dice to declared attack/save domains, subject to
// an explicit contribution-selection rule. It has no implicit lifetime policy.
type AddToRoll struct {
	Amount    SignedDice
	Rolls     []contributions.RollKind
	Selection generic.SelectionRule
}

// DefinitionData projects the configured signed roll contribution without binding.
func (d AddToRoll) DefinitionData() (DefinitionData, error) {
	return definitionTemplate(definitionTemplateInput{Kind: KindAddToRoll, Amount: &generic.Amount{Dice: d.Amount.Notation}, Params: addToRollParams{Polarity: d.Amount.Polarity, Rolls: d.Rolls, Selection: d.Selection}})
}

// EndOn ends the scope declared by its enclosing PartSpec on a holder beat.
// It requires a supported occurrence and an explicit descriptive reason.
type EndOn struct {
	Beat   machine.Beat
	Reason string
}

// DefinitionData projects the occurrence/reason without choosing an ending scope.
func (d EndOn) DefinitionData() (DefinitionData, error) {
	return definitionTemplate(definitionTemplateInput{Kind: KindEndOn, Params: endOnParams(d)})
}

// DamageBonus adds the supported scalar amount to matching outgoing damage.
// When is declarative frame-fact data, not a callback or private state access.
type DamageBonus struct {
	Amount generic.Amount
	When   *machine.Guard
}

// DefinitionData projects the amount/guard; Compile owns the resulting copy.
func (d DamageBonus) DefinitionData() (DefinitionData, error) {
	return definitionTemplate(definitionTemplateInput{Kind: KindDamageBonus, Amount: &d.Amount, When: d.When})
}

// Resistance contributes resistance for its nonempty, unique damage-type list.
// It includes no activity, concentration or other ending policy.
type Resistance struct{ Types []damage.Type }

// DefinitionData projects the configured damage types without runtime effects.
func (d Resistance) DefinitionData() (DefinitionData, error) {
	return definitionTemplate(definitionTemplateInput{Kind: KindResistance, Params: resistanceParams(d)})
}

// AdvantageDomain names the operation to which an advantage definition applies.
type AdvantageDomain string

const (
	// AdvantageOnAttacks requires an explicit by/against-holder side.
	AdvantageOnAttacks AdvantageDomain = AdvantageDomain(domainAttack)
	// AdvantageOnSavingThrows applies to the holder's matching saving throws.
	AdvantageOnSavingThrows AdvantageDomain = AdvantageDomain(domainSave)
	// AdvantageOnAbilityChecks applies to the holder's matching ability checks.
	AdvantageOnAbilityChecks AdvantageDomain = AdvantageDomain(domainCheck)
)

// AttackSide names the holder's bearing on an attack, without swapping actors.
type AttackSide string

const (
	// AttackByHolder applies to attacks made by the effect's holder.
	AttackByHolder AttackSide = AttackSide(sideByHolder)
	// AttackAgainstHolder applies to attacks against the effect's holder.
	AttackAgainstHolder AttackSide = AttackSide(sideAgainstHolder)
)

// Advantage configures the existing advantage meaning with supported frame facts.
// Side is required only for attacks and forbidden for saves/checks.
type Advantage struct {
	On   AdvantageDomain
	Side AttackSide
	When *machine.Guard
}

// DefinitionData projects domain/side/guard without assessing a live action.
func (d Advantage) DefinitionData() (DefinitionData, error) {
	params := struct {
		On   AdvantageDomain `json:"on"`
		Side AttackSide      `json:"side,omitempty"`
	}{On: d.On, Side: d.Side}
	return definitionTemplate(definitionTemplateInput{Kind: KindAdvantage, Params: params, When: d.When})
}

// Concentration maintains the application through the holder's existing
// concentration meaning. It adds no duration, rest or recipient-activity rule.
type Concentration struct{}

// DefinitionData projects concentration meaning without inventing an owner.
func (Concentration) DefinitionData() (DefinitionData, error) {
	return DefinitionData{Effect: &EffectSpec{Kind: KindConcentration}}, nil
}

// Duration counts positive holder turn ends independently of payloads or
// contribution selection. SkipFirst explicitly selects activation grace.
// The existing duration meaning owns counting and application termination.
type Duration struct {
	TurnEnds  int
	SkipFirst bool
}

// DefinitionData projects the authored counter/grace, not current progress.
func (d Duration) DefinitionData() (DefinitionData, error) {
	return definitionTemplate(definitionTemplateInput{Kind: KindDuration, Params: durationParams(d)})
}

// UntilEmpty ends an application on actual last-member loss, not empty creation.
type UntilEmpty struct{}

// DefinitionData projects last-member upkeep without inventing membership.
func (UntilEmpty) DefinitionData() (DefinitionData, error) {
	return DefinitionData{Effect: &EffectSpec{Kind: KindUntilEmpty}}, nil
}

type definitionTemplateInput struct {
	Kind   generic.KindName
	Params any
	When   *machine.Guard
	Amount *generic.Amount
}

func definitionTemplate(in definitionTemplateInput) (DefinitionData, error) {
	var params json.RawMessage
	if in.Params != nil {
		raw, err := json.Marshal(in.Params)
		if err != nil {
			return DefinitionData{}, err
		}
		params = raw
	}
	return DefinitionData{Effect: &EffectSpec{Kind: in.Kind, Params: params, When: in.When, Amount: in.Amount}}, nil
}
