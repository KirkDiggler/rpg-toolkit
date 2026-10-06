// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// EffectState is how one effect bears on a declared action, as the player
// sees it (rpg-project#520). A closed set mirrored from the rulebook's own
// answer rather than imported across the seam (S2).
type EffectState string

const (
	// EffectApplies means the effect's rule applies to this action.
	EffectApplies EffectState = "applies"
	// EffectDoesNotApply means the rule does not apply, for the reason the row
	// carries. The row stays visible.
	EffectDoesNotApply EffectState = "does_not_apply"
	// EffectDepends means the rule needs a fact the actor does not know yet —
	// most often which target, or who stands beside it.
	EffectDepends EffectState = "depends"
	// EffectUnavailable means the effect's rule cannot yet answer at all. It is
	// never dropped and never shown as not applying.
	EffectUnavailable EffectState = "unavailable"
)

// EffectParticipation says whether an effect joins the action as it is taken
// or is offered as a choice afterwards.
type EffectParticipation string

const (
	// ContributesNow means the effect joins the action's own calculation.
	ContributesNow EffectParticipation = "contributes_now"
	// LaterChoice means the effect is offered later — Bardic Inspiration after
	// the roll — and is never shown as already added.
	LaterChoice EffectParticipation = "later_choice"
)

// EffectRow is one effect bearing on a declaration's action: which effect,
// what it does, whether it applies here, and why. Every string is the
// rulebook's own; this seam names no effect and authors no text.
//
// ROWS NEVER GRANT OR REFUSE. A declaration's Available and Why are decided
// without them, execution never reads them, and they are not selector
// material: a row changing leaves the declaration's ID where it was.
type EffectRow struct {
	// ID is opaque and unique within its declaration. A candidate's
	// [TargetEffect] names the row it replaces by this ID.
	ID string `json:"id"`

	// Ref is the effect's source reference.
	Ref string `json:"ref"`

	// Name is the effect's display name.
	Name string `json:"name"`

	// Description is what the effect does, authored beside its rule —
	// separate from Reason, which is why it does or does not apply here.
	Description string `json:"description"`

	// State is the rule's answer with no target in particular.
	State EffectState `json:"state"`

	// Reason is the rule's own reason for State.
	Reason string `json:"reason"`

	// Participation says whether the effect contributes now or later.
	Participation EffectParticipation `json:"participation"`

	// Benefit is the rule-authored line stating what the effect adds, empty
	// when there is no line — which is every row that does not apply.
	Benefit string `json:"benefit,omitempty"`
}

// TargetEffect is one row's answer for a particular target candidate. It
// replaces the declaration row with the same ID's State, Reason and Benefit
// wholesale; Description and Participation never change by target and are
// never repeated.
//
// A candidate carries one only where that target's answer differs from the
// declaration's row, so an empty list means "every row reads as declared".
type TargetEffect struct {
	// ID names the declaration row this answer replaces.
	ID string `json:"id"`

	// State is the rule's answer for this target.
	State EffectState `json:"state"`

	// Reason is the rule's own reason for State.
	Reason string `json:"reason"`

	// Benefit is the rule-authored benefit line for this target, empty when
	// there is none.
	Benefit string `json:"benefit,omitempty"`
}

// attachEffectsInput is what [attachEffects] reads: the encounter the actor
// observes from, the actor's own already-loaded sheet, and the offers Afford
// just compiled.
type attachEffectsInput struct {
	// Encounter is the loaded world Afford compiled from.
	Encounter *encounter.Encounter
	// Member is the acting member, whose observed context the rows draw on.
	Member string
	// Actor is the sheet compileOffersFor compiled from — never a second load,
	// and never attached to a bus.
	Actor *character.Character
	// Offers are Afford's compiled offers. Their declarations gain rows in
	// place; nothing else on them is touched.
	Offers []compiledOffer
}

// attachEffects writes effect rows onto the declarations Afford compiled
// (rpg-project#520). It runs ONLY in [Manager.Afford], after compileOffersFor
// and never on an execution caller of it, so a row can never refuse, alter or
// select a command: the rows are written after every gate and selector is
// already settled, and no verb ever compiles them.
//
// Session carries and projects; it decides nothing. The rows are
// [resolution.InformAttack]'s answers, drawn from the actor's own observed
// context — read at most once per call, and only when some offer has rows to
// carry — and the actor's own loaded effects. Each compiled Attack, and each
// compiled Cast whose definition makes a spell attack, is asked about its own
// candidates in candidate order. The no-target answers become the
// declaration's rows; a candidate carries a [TargetEffect] only where its
// answer differs from that row.
//
// Every other declaration is left without rows: blockers, which compiled no
// attack, and every verb whose action makes no attack roll.
//
// Errors, each failing Afford closed: the observed context cannot be read, a
// compiled Attack carries no attack profile, InformAttack refuses, or an
// answer arrives in a state or participation this seam has no word for.
func attachEffects(in *attachEffectsInput) error {
	var observed *encounter.ObservedContextOutput
	for i := range in.Offers {
		offer := &in.Offers[i]
		attack, informs := informedAttackOf(offer)
		if !informs {
			continue
		}
		if attack == nil {
			return fmt.Errorf("effect rows: compiled attack %s carries no attack profile: %w",
				offer.attack.Ref.String(), ErrBadAttack)
		}
		var err error
		if observed == nil {
			if observed, err = in.Encounter.ObservedContext(&encounter.ViewInput{
				Member: encounter.MemberID(in.Member),
			}); err != nil {
				return fmt.Errorf("effect rows: %w", translate(err))
			}
		}

		targets := make([]string, 0, len(offer.declaration.Candidates))
		for _, candidate := range offer.declaration.Candidates {
			targets = append(targets, candidate.Member)
		}
		informed, err := resolution.InformAttack(&resolution.InformAttackInput{
			Observed: observed, Actor: in.Actor, Attack: attack, Targets: targets,
		})
		if err != nil {
			return fmt.Errorf("effect rows for %s: %w", offer.declaration.Verb, err)
		}

		rows := make([]EffectRow, 0, len(informed.Effects))
		for _, effect := range informed.Effects {
			row, err := effectRowOf(effect)
			if err != nil {
				return err
			}
			rows = append(rows, row)
		}

		// A fresh candidate slice: the compiled variants of one verb may have
		// been projected from shared preflight, and each declaration's answers
		// are its own.
		candidates := make([]TargetCandidate, len(offer.declaration.Candidates))
		copy(candidates, offer.declaration.Candidates)
		for j := range candidates {
			answers, err := targetEffectsOf(rows, informed.ByTarget[candidates[j].Member])
			if err != nil {
				return fmt.Errorf("effect rows for %q: %w", candidates[j].Member, err)
			}
			candidates[j].Effects = answers
			held, err := heldRowsOf(rows, informed.HeldByTarget, candidates[j].Member)
			if err != nil {
				return fmt.Errorf("held effect rows for %q: %w", candidates[j].Member, err)
			}
			candidates[j].HeldEffects = held
		}

		offer.declaration.Effects = rows
		offer.declaration.Candidates = candidates
	}
	return nil
}

// heldRowsOf maps one candidate's held effects onto full rows, set only on
// that candidate (R18): declaration rows stay the actor's own, and an effect
// no candidate holds appears nowhere. Resolution answers every target asked,
// so a target missing from heldByTarget is a broken answer and fails the read,
// as does a held row whose ID collides with a declaration row's — the two
// lists are never joined, and a shared ID would let a client do exactly that.
// Nil when the target holds nothing that bears or its holdings are unknown.
func heldRowsOf(declared []EffectRow, heldByTarget map[string][]contributions.Effect, member string) ([]EffectRow, error) {
	held, answered := heldByTarget[member]
	if !answered {
		return nil, fmt.Errorf("resolution gave no held answer for this target: %w", ErrBadAttack)
	}
	if len(held) == 0 {
		return nil, nil
	}
	ids := make(map[string]bool, len(declared))
	for _, row := range declared {
		ids[row.ID] = true
	}
	rows := make([]EffectRow, 0, len(held))
	for _, effect := range held {
		row, err := effectRowOf(effect)
		if err != nil {
			return nil, err
		}
		if ids[row.ID] {
			return nil, fmt.Errorf("held row %q collides with a declaration row: %w", row.ID, ErrBadAttack)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// informedAttackOf is the attack profile an offer's rows are asked about, and
// whether the offer carries rows at all: every compiled Attack does, with its
// assembled weapon attack, as does a compiled Cast whose definition makes a
// spell attack. Every other offer — blockers included — carries none.
func informedAttackOf(offer *compiledOffer) (*combatActions.AttackProfile, bool) {
	switch {
	case offer.attack != nil:
		return offer.attack.Attack, true
	case offer.spell != nil && offer.spell.Cast != nil && offer.spell.Cast.Attack != nil:
		return offer.spell.Cast.Attack, true
	default:
		return nil, false
	}
}

// effectRowOf projects one no-target answer onto the wire row, field for field.
func effectRowOf(effect contributions.Effect) (EffectRow, error) {
	state, err := effectStateOf(effect.State)
	if err != nil {
		return EffectRow{}, fmt.Errorf("effect row %q: %w", effect.ID, err)
	}
	var participation EffectParticipation
	switch effect.Participation {
	case contributions.ContributesNow:
		participation = ContributesNow
	case contributions.LaterChoice:
		participation = LaterChoice
	default:
		return EffectRow{}, fmt.Errorf("effect row %q: %w: participation %q",
			effect.ID, ErrInvalidWorld, effect.Participation)
	}
	ref := ""
	if effect.Source.Ref != nil {
		ref = effect.Source.Ref.String()
	}
	return EffectRow{
		ID: effect.ID, Ref: ref, Name: effect.Source.Name, Description: effect.Description,
		State: state, Reason: effect.Reason, Participation: participation, Benefit: effect.Benefit,
	}, nil
}

// targetEffectsOf keeps one target's answers only where they differ from the
// declaration's row with the same ID. InformAttack answers every target with
// the declaration's IDs in the declaration's order; anything else is refused
// rather than overlaid onto the wrong row.
func targetEffectsOf(rows []EffectRow, answers []contributions.Effect) ([]TargetEffect, error) {
	if len(answers) != len(rows) {
		return nil, fmt.Errorf("%w: %d target answers for %d rows", ErrInvalidWorld, len(answers), len(rows))
	}
	var out []TargetEffect
	for i, answer := range answers {
		if answer.ID != rows[i].ID {
			return nil, fmt.Errorf("%w: target answer %d is %q, not %q", ErrInvalidWorld, i, answer.ID, rows[i].ID)
		}
		state, err := effectStateOf(answer.State)
		if err != nil {
			return nil, fmt.Errorf("target answer %q: %w", answer.ID, err)
		}
		if state == rows[i].State && answer.Reason == rows[i].Reason && answer.Benefit == rows[i].Benefit {
			continue
		}
		out = append(out, TargetEffect{ID: answer.ID, State: state, Reason: answer.Reason, Benefit: answer.Benefit})
	}
	return out, nil
}

// effectStateOf maps the rulebook's state onto this seam's closed set.
func effectStateOf(state contributions.EffectState) (EffectState, error) {
	switch state {
	case contributions.StateApplies:
		return EffectApplies, nil
	case contributions.StateDoesNotApply:
		return EffectDoesNotApply, nil
	case contributions.StateDepends:
		return EffectDepends, nil
	case contributions.StateUnavailable:
		return EffectUnavailable, nil
	default:
		return "", fmt.Errorf("%w: effect state %q", ErrInvalidWorld, state)
	}
}
