// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// InformAttackInput is everything effect information for one attack may draw
// from: the acting character's own observed context, its own sheet, the
// assembled attack, and the candidate targets to ask about.
//
// THERE IS NO FIELD FOR A TARGET SHEET OR THE CAST, and that absence is the
// guarantee (K4): information draws only from what the actor knows, so a
// target's hidden state has no way in. What the actor knows of the others is
// Observed — its sightings, the distances between them and the stances it
// believes.
type InformAttackInput struct {
	// Observed is the actor's own observed context
	// ([encounter.Encounter.ObservedContext]). REQUIRED.
	Observed *encounter.ObservedContextOutput

	// Actor is the acting character's loaded sheet; only its own loaded
	// effects are asked. REQUIRED, and its ID must be Observed's observer.
	Actor *character.Character

	// Attack is the assembled attack profile the actor would swing. REQUIRED.
	Attack *combatActions.AttackProfile

	// Targets are the candidate member IDs to ask about, in candidate order.
	// Each must be non-empty and appear once. Empty asks only the no-target
	// question.
	Targets []string
}

// InformAttackOutput is each loaded effect's answer for the attack. It holds
// one row per bearing effect and never a total.
type InformAttackOutput struct {
	// Effects are the answers with no target in particular, in the actor's
	// persisted condition order.
	Effects []contributions.Effect

	// ByTarget holds each candidate's answers. Every list carries the same IDs
	// in the same order as Effects; only state, reason and benefit may differ.
	// Non-nil, with one entry per target.
	ByTarget map[string][]contributions.Effect

	// HeldByTarget holds, per candidate, a row for each effect that candidate
	// was SEEN holding which bears on the attack
	// ([conditions.AssessTargetHeldEffects] over the same information frame
	// ByTarget used), in the order it was seen. A candidate whose holdings the
	// actor has not observed gets no rows, and so does one seen holding
	// nothing that bears. These rows are never in Effects or ByTarget, whose
	// rows are the actor's own (R18). Non-nil, with one entry per target.
	HeldByTarget map[string][]contributions.Effect
}

// InformAttack answers which of the actor's own effects bear on an attack and
// how: once with no target, and once per candidate target. Each answer comes
// from [conditions.AssessActionEffects] over an information frame built from
// Observed alone, so the rows are the same rule functions the swing will ask,
// answered from what the actor knows. Per candidate it also answers which
// effects that candidate was seen holding bear on the attack, from the same
// frame: what others hold comes only from the actor's sightings, never a
// target sheet; the actor's own holdings come from its own sheet.
//
// Reading spends, rolls and publishes nothing, and rows never grant or refuse
// the attack.
//
// Errors: a nil input, a nil Observed, Actor or Attack, an actor that is not
// Observed's observer, an empty or repeated target, an information frame that
// fails validation, or any error from [conditions.AssessActionEffects] or
// [conditions.AssessTargetHeldEffects].
func InformAttack(in *InformAttackInput) (*InformAttackOutput, error) {
	if in == nil || in.Observed == nil || in.Actor == nil || in.Attack == nil {
		return nil, fmt.Errorf("%w: inform attack needs an observed context, an actor and an attack", ErrNilInput)
	}
	if string(in.Observed.Observer) != in.Actor.GetID() {
		return nil, fmt.Errorf("inform attack: observed context is %q's, not the actor %q's",
			in.Observed.Observer, in.Actor.GetID())
	}
	loaded := in.Actor.GetConditions()
	actorHeld := heldAddresses(in.Actor.GetID(), loaded)

	assess := func(target string) ([]contributions.Effect, contributions.Frame, error) {
		framed, err := informationFrame(&informationFrameInput{
			Observed: in.Observed, Attack: in.Attack, Target: target, ActorHeld: actorHeld,
		})
		if err != nil {
			return nil, contributions.Frame{}, err
		}
		assessed, err := conditions.AssessActionEffects(&conditions.AssessActionEffectsInput{
			Conditions: loaded, Frame: framed.Frame,
		})
		if err != nil {
			return nil, contributions.Frame{}, fmt.Errorf("assess attack effects: %w", err)
		}
		return assessed.Effects, framed.Frame, nil
	}

	effects, _, err := assess("")
	if err != nil {
		return nil, err
	}
	out := &InformAttackOutput{
		Effects:      effects,
		ByTarget:     make(map[string][]contributions.Effect, len(in.Targets)),
		HeldByTarget: make(map[string][]contributions.Effect, len(in.Targets)),
	}
	for _, target := range in.Targets {
		if target == "" {
			return nil, fmt.Errorf("inform attack: an empty target")
		}
		if _, repeated := out.ByTarget[target]; repeated {
			return nil, fmt.Errorf("inform attack: target %q appears twice", target)
		}
		answers, frame, err := assess(target)
		if err != nil {
			return nil, err
		}
		if len(answers) != len(effects) {
			return nil, fmt.Errorf("inform attack: target %q answered %d effects, not %d", target, len(answers), len(effects))
		}
		for i := range answers {
			if answers[i].ID != effects[i].ID {
				return nil, fmt.Errorf("inform attack: target %q row %d is %q, not %q", target, i, answers[i].ID, effects[i].ID)
			}
		}
		out.ByTarget[target] = answers

		held, err := conditions.AssessTargetHeldEffects(&conditions.AssessTargetHeldEffectsInput{Frame: frame})
		if err != nil {
			return nil, fmt.Errorf("assess effects %q holds: %w", target, err)
		}
		out.HeldByTarget[target] = held.Effects
	}

	return out, nil
}
