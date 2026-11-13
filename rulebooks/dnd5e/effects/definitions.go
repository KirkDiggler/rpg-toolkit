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

// Definition is typed, unbound rulebook content. CompileParts lowers these
// values into canonical PartDeclaration data and returns validation errors.
// The private adapter method admits no author-supplied runtime callbacks.
type Definition interface{ effectTemplate() (PartTemplate, error) }

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

func (d AddToRoll) effectTemplate() (PartTemplate, error) {
	return definitionTemplate(definitionTemplateInput{Kind: KindAddToRoll, Amount: &generic.Amount{Dice: d.Amount.Notation}, Params: addToRollParams{Polarity: d.Amount.Polarity, Rolls: d.Rolls, Selection: d.Selection}})
}

// EndOn ends the scope declared by its enclosing PartSpec on a holder beat.
// It requires a supported occurrence and an explicit descriptive reason.
type EndOn struct {
	Beat   machine.Beat
	Reason string
}

func (d EndOn) effectTemplate() (PartTemplate, error) {
	return definitionTemplate(definitionTemplateInput{Kind: KindEndOn, Params: endOnParams(d)})
}

// DamageBonus adds the supported scalar amount to matching outgoing damage.
// When is declarative frame-fact data, not a callback or private state access.
type DamageBonus struct {
	Amount generic.Amount
	When   *machine.Guard
}

func (d DamageBonus) effectTemplate() (PartTemplate, error) {
	return definitionTemplate(definitionTemplateInput{Kind: KindDamageBonus, Amount: &d.Amount, When: d.When})
}

// Resistance contributes resistance for its nonempty, unique damage-type list.
// It includes no activity, concentration or other ending policy.
type Resistance struct{ Types []damage.Type }

func (d Resistance) effectTemplate() (PartTemplate, error) {
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

func (d Advantage) effectTemplate() (PartTemplate, error) {
	params := struct {
		On   AdvantageDomain `json:"on"`
		Side AttackSide      `json:"side,omitempty"`
	}{On: d.On, Side: d.Side}
	return definitionTemplate(definitionTemplateInput{Kind: KindAdvantage, Params: params, When: d.When})
}

// Concentration maintains the application through the holder's existing
// concentration meaning. It adds no duration, rest or recipient-activity rule.
type Concentration struct{}

func (Concentration) effectTemplate() (PartTemplate, error) {
	return PartTemplate{Kind: KindConcentration}, nil
}

// UntilEmpty ends an application on actual last-member loss, not empty creation.
type UntilEmpty struct{}

func (UntilEmpty) effectTemplate() (PartTemplate, error) {
	return PartTemplate{Kind: KindUntilEmpty}, nil
}

type definitionTemplateInput struct {
	Kind   generic.KindName
	Params any
	When   *machine.Guard
	Amount *generic.Amount
}

func definitionTemplate(in definitionTemplateInput) (PartTemplate, error) {
	var params json.RawMessage
	if in.Params != nil {
		raw, err := json.Marshal(in.Params)
		if err != nil {
			return PartTemplate{}, err
		}
		params = raw
	}
	return PartTemplate{Kind: in.Kind, Params: params, When: in.When, Amount: in.Amount}, nil
}
