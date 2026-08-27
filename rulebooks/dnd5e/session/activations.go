// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// buildActivationOffers compiles one offer per thing this member can activate.
//
// # A projection, not a second pricing engine
//
// character.AvailableAbilities already answers, per ability: its ref, its
// display name, which economy shape it draws, what target it prompts for,
// whether it can be used right now, and how many charges are left. That is a
// declaration in everything but name, assembled in the rulebook for a menu
// that no longer exists and never once crossing this seam.
//
// So this compiles nothing and prices nothing. It projects, adds a selector,
// and converts one prose field into a structured one — see [activationWhy].
// Afford's "read the price off the SAME profile the door would charge" rule is
// kept by construction here rather than by care: the shape comes from the
// ability's own ActionType, which IS what the door charges (rpg-project#301 §4).
//
// # Attack is deliberately not among them
//
// Every character carries a combat ability called Attack, and it is excluded.
// Swinging is already [VerbAttack] at this seam, priced off a compiled
// definition; emitting the Attack ACTION as an eighth activation would put two
// buttons on the panel for one thing and invite a player to spend an action
// banking swings the seam already banked. The six that remain plus the
// character's features are the seven a level-1 player actually reaches.
//
// # Every ability gets a row, available or not
//
// An unavailable ability keeps its row with Available false and a reason, the
// way a budget-blocked Attack keeps its candidates. A UI that only learned
// about Dodge on the turns it could Dodge would have a menu that changes size
// as the turn goes on.
func (m *Manager) buildActivationOffers(
	session, member string, sheet *character.Character,
) ([]compiledOffer, error) {
	available := sheet.AvailableAbilities()
	economy := sheet.GetActionEconomy()

	offers := make([]compiledOffer, 0, len(available))
	for _, ability := range available {
		if ability.Ref == nil {
			// Fail closed. A nameless ability cannot be selected, echoed back,
			// or executed, and quietly dropping it would leave a button
			// missing from a panel with no trace of why.
			return nil, fmt.Errorf("member %q offers an ability with no ref: %w", member, ErrBadCharacter)
		}
		if ability.Ref.ID == refs.CombatAbilities.Attack().ID {
			continue
		}

		slot := slotOfEconomySlot(ability.EconomySlot)
		ref := ability.Ref.String()

		id, variant, err := selectorIDFor(session, member, VerbActivate, slot, nil, ref)
		if err != nil {
			return nil, err
		}

		declaration := Declaration{
			Verb:       VerbActivate,
			Slot:       slot,
			Available:  ability.CanUse,
			Ability:    &AbilityRef{Ref: ref, Name: ability.Name},
			TargetKind: targetKindOfAbility(ability.TargetKind),
			ID:         id,
			Candidates: []TargetCandidate{},
		}
		if !ability.CanUse {
			why := activationWhy(ability, slot, economy)
			declaration.Why = &why
		}

		offers = append(offers, compiledOffer{
			declaration: declaration,
			sheet:       sheet,
			verb:        VerbActivate,
			slot:        slot,
			variant:     variant,
		})
	}

	// Sorted by ref so the panel's order is a fact about the character rather
	// than about the order two slices happened to be concatenated in. Ranking
	// across verbs is sortDeclarations' job and stays there; this is the
	// within-verb tiebreak that verb ranking alone cannot provide now that one
	// verb has seven rows.
	sort.SliceStable(offers, func(i, j int) bool {
		return offers[i].declaration.Ability.Ref < offers[j].declaration.Ability.Ref
	})
	return offers, nil
}

// activationWhy turns the rulebook's prose refusal into the structured one this
// seam owes a client.
//
// # Authored, never parsed
//
// AvailableAbility carries a single Reason STRING covering three different
// refusals: the slot is spent, the charges are gone, or the ability's own
// precondition said no. Discriminating them by matching that string would be
// the exact inversion of this seam's law — the SDK renders Text FROM the
// structure, never the reverse — and it would break the first time a feature
// reworded its own error.
//
// So the discrimination is made from state the seam can see for itself:
//
//  1. Ask the ECONOMY whether this shape is spent. If it is, the refusal is a
//     budget one and every figure is known.
//  2. Otherwise the economy allowed it and the ABILITY refused. If it carries a
//     resource that has run out, that is a ledger too — just not one of the
//     turn's three.
//  3. Otherwise it is a precondition: already raging, already at full hit
//     points. Nothing ran out and waiting will not help.
//
// # Where the prose survives, and why
//
// Case 1 renders its own text, because reason/currency/needed/left determine it
// completely. Cases 2 and 3 keep the ABILITY'S OWN words, because the structure
// deliberately does not name which resource ran out — this seam does not
// enumerate the rulebook's resource keys — so "no rage uses remaining" carries
// the one fact a client cannot reconstruct. That is Text doing its documented
// job of narrating what the structure cannot.
func activationWhy(
	ability character.AvailableAbility, slot Slot, economy *character.ActionEconomyData,
) Shortfall {
	if economy != nil && slot != SlotNone && remainingInSlot(slot, economy) <= 0 {
		currency := currencyOfSlot(slot)
		return Shortfall{
			Reason:   ShortfallNoBudget,
			Currency: currency,
			Needed:   1,
			Left:     0,
			Text:     fmt.Sprintf("%s: 1 needed, 0 left", currency),
		}
	}

	if ability.ResourceMax > 0 && ability.ResourceCurrent <= 0 {
		return Shortfall{
			Reason:   ShortfallNoBudget,
			Currency: CurrencyCharges,
			Needed:   1,
			Left:     0,
			Text:     ability.Reason,
		}
	}

	return Shortfall{Reason: ShortfallUnavailable, Text: ability.Reason}
}

// remainingInSlot reads one shape's remaining count off the economy.
//
// SlotNone never reaches here: an ability that lights no shape cannot be
// refused for want of one, and asking the economy about a shape that does not
// exist would answer zero — which reads as "spent" and would mark every free
// ability unaffordable.
func remainingInSlot(slot Slot, economy *character.ActionEconomyData) int {
	switch slot {
	case SlotAction:
		return economy.ActionsRemaining
	case SlotBonus:
		return economy.BonusActionsRemaining
	case SlotReaction:
		return economy.ReactionsRemaining
	case SlotNone:
		return 1
	default:
		return 1
	}
}

// slotOfEconomySlot maps the rulebook's economy shape onto the seam's.
//
// The rulebook has two values this seam does not: Movement, which is a ledger
// rather than a shape and belongs to Move, and Free, which is the rulebook's
// way of saying "lights nothing" — the same claim SlotNone makes. Unspecified
// maps to SlotNone as well, because an ability whose author said nothing is
// not thereby spending an action, and inventing one would make a free ability
// look expensive.
func slotOfEconomySlot(slot character.EconomySlot) Slot {
	switch slot {
	case character.EconomySlotAction:
		return SlotAction
	case character.EconomySlotBonusAction:
		return SlotBonus
	case character.EconomySlotReaction:
		return SlotReaction
	case character.EconomySlotMovement, character.EconomySlotFree, character.EconomySlotUnspecified:
		return SlotNone
	default:
		return SlotNone
	}
}

// targetKindOfAbility maps the rulebook's target prompt onto the seam's.
//
// SIX RULEBOOK VALUES BECOME THREE, and one collapse is worth naming: Self and
// None both become TargetNone. They are the same instruction to a client — do
// not prompt — and the distinction the rulebook keeps is about WHOSE SHEET the
// effect lands on, which is the rulebook's business and not the panel's.
//
// Position and Area have no seam value and become TargetNone rather than an
// invented one, because nothing at level 1 uses them and a target kind arrives
// here with a proven executor, never in advance.
func targetKindOfAbility(kind character.TargetKind) TargetKind {
	switch kind {
	case character.TargetKindSingleEntity:
		return TargetMember
	default:
		return TargetNone
	}
}
