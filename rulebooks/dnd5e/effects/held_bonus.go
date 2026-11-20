// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json/jsontext"
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

// The held bonuses are query-only: nothing is attached to a bus. The sheet that
// holds the condition asks for the amount when it reads the number it changes,
// so a removed condition is simply no longer asked and nothing can go stale.
const (
	// KindHitPointMaximum raises the holder's hit point maximum by a fixed
	// positive whole number while the part is held.
	KindHitPointMaximum generic.KindName = "hit_point_maximum"
	// KindInitiativeBonus adds a fixed positive whole number to the holder's
	// initiative while the part is held.
	KindInitiativeBonus generic.KindName = "initiative_bonus"
)

const (
	domainHitPointMaximum domain = "hit_point_maximum"
	domainInitiative      domain = "initiative"
)

// heldBonus is the one shape both kinds share; only their words differ.
type heldBonus struct {
	kind   generic.KindName
	domain domain
	// noun names the kind in a refusal: "hit point maximum raise".
	noun string
	// describe is the information row's plain sentence for an amount.
	describe func(int) string
	// general is the sentence when no amount is in hand (the kind's meaning).
	general string
}

var (
	hitPointMaximumBonus = heldBonus{kind: KindHitPointMaximum, domain: domainHitPointMaximum,
		noun:     "hit point maximum raise",
		describe: func(n int) string { return fmt.Sprintf("Raises the holder's hit point maximum by %d.", n) },
		general:  "Raises the holder's hit point maximum."}
	initiativeBonus = heldBonus{kind: KindInitiativeBonus, domain: domainInitiative,
		noun:     "initiative bonus",
		describe: func(n int) string { return fmt.Sprintf("Adds %d to the holder's initiative.", n) },
		general:  "Adds to the holder's initiative."}
)

func (b heldBonus) meaning(raw jsontext.Value) (meaning, error) {
	if len(raw) > 0 {
		if _, err := decodeParams[struct{}](raw); err != nil {
			return meaning{}, fmt.Errorf("%s takes no params: %w", b.noun, err)
		}
	}
	return meaning{domain: b.domain, description: b.general}, nil
}

// check refuses everything but a fixed positive whole number and no guard.
// A fraction never reaches here: the amount's fixed field is an int, so the
// definition's own decoding refuses it.
func (b heldBonus) check(out generic.Output) error {
	if _, err := b.meaning(out.Params); err != nil {
		return err
	}
	if out.When != nil {
		return fmt.Errorf("%s does not take a guard: it is in force while held", b.noun)
	}
	if out.Amount == nil {
		return fmt.Errorf("%s requires an amount", b.noun)
	}
	name, err := out.Amount.Name()
	if err != nil {
		return err
	}
	if name != generic.AmountFixed {
		return fmt.Errorf("%s requires a fixed whole number, not %q", b.noun, name)
	}
	if *out.Amount.Fixed < 1 {
		return fmt.Errorf("%s must be a positive whole number, got %d", b.noun, *out.Amount.Fixed)
	}
	return nil
}

func (b heldBonus) payload(decl PartDeclaration) (generic.Output, error) {
	if decl.OnTerminal != generic.EndPart {
		return generic.Output{}, fmt.Errorf("%s payload requires local part scope", b.noun)
	}
	out := generic.Output{Kind: b.kind, Params: jsontext.Value(decl.Params), When: decl.When, Amount: decl.Amount}
	if err := b.check(out); err != nil {
		return generic.Output{}, err
	}
	return out, nil
}

func (b heldBonus) validatePart(decl PartDeclaration) error {
	_, err := b.payload(decl)
	return err
}

func (b heldBonus) bindPart(in *BindPartInput) (boundPart, error) {
	out, err := b.payload(in.Declaration)
	if err != nil {
		return boundPart{}, err
	}
	return payloadRecipe(in, out)
}

func (b heldBonus) inspectPart(in *InspectPartInput) (*PartInfo, error) {
	if err := b.validatePart(in.Declaration); err != nil {
		return nil, err
	}
	return inspectPayloadState(in.Snapshot)
}

// assess takes the ordinary path (no action, attack, save or check is this
// kind's domain, so the row never claims to apply to one) and then names the
// amount in the description.
func (b heldBonus) assess(in *AssessInput, m meaning) (assessment, error) {
	result, err := assessMeaning(in, m)
	if err != nil {
		return result, err
	}
	n, err := b.amount(in.Output)
	if err != nil {
		return result, err
	}
	result.effect.Description = b.describe(n)
	return result, nil
}

func (b heldBonus) amount(out *generic.Bound) (int, error) {
	definition := out.Definition()
	if err := b.check(definition); err != nil {
		return 0, err
	}
	return *definition.Amount.Fixed, nil
}

func (b heldBonus) definition() kindDefinition {
	return kindDefinition{class: PartClassBenefit, payload: b.payload, validatePart: b.validatePart, part: b.bindPart,
		inspectPart: b.inspectPart, execution: generic.KindQuery, check: b.check, meaning: b.meaning, assess: b.assess}
}

// HeldBonusInput asks one bound output what it adds to one held number.
type HeldBonusInput struct {
	Output *generic.Bound
}

// HeldBonusOutput is the whole number the output adds. Zero means the output
// is of another kind and adds nothing to this number.
type HeldBonusOutput struct {
	Amount int
}

func readHeldBonus(b heldBonus, in *HeldBonusInput) (*HeldBonusOutput, error) {
	if in == nil || in.Output == nil {
		return nil, fmt.Errorf("%s read requires a bound output", b.noun)
	}
	if in.Output.Kind() != b.kind {
		return &HeldBonusOutput{}, nil
	}
	n, err := b.amount(in.Output)
	if err != nil {
		return nil, err
	}
	return &HeldBonusOutput{Amount: n}, nil
}

// HitPointMaximumOf answers how much one bound output raises its holder's hit
// point maximum. Errors: a nil input or output, or a hit-point-maximum output
// whose amount is not a fixed positive whole number.
func HitPointMaximumOf(in *HeldBonusInput) (*HeldBonusOutput, error) {
	return readHeldBonus(hitPointMaximumBonus, in)
}

// InitiativeBonusOf answers how much one bound output adds to its holder's
// initiative. Errors as [HitPointMaximumOf].
func InitiativeBonusOf(in *HeldBonusInput) (*HeldBonusOutput, error) {
	return readHeldBonus(initiativeBonus, in)
}
