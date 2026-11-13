// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	ruleeffects "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// RagingBindings is the immutable construction input for the shared definition.
// It selects a fixed authored amount or the rulebook's live Barbarian scaling.
type RagingBindings struct {
	DamageBonus RagingDamageBonus `json:"damage_bonus"`
}

// RagingDamageBonus selects exactly one amount strategy. Fixed must be positive;
// BarbarianLevel must be true when selected and never stores a copied level.
type RagingDamageBonus struct {
	Fixed          *int `json:"fixed,omitempty"`
	BarbarianLevel bool `json:"barbarian_level,omitempty"`
}

// rageDurationRounds is the rulebook's ten-round Rage duration.
const rageDurationRounds = 10

var ragingDescriptor = composedDescriptor(compositionDefinition{
	ref:     *refs.Conditions.Raging(),
	display: Display{Name: "Raging", Detail: "Adds your rage damage bonus to melee weapon attacks that use Strength. Also grants advantage on Strength-based skill checks and Strength saving throws, and resistance to bludgeoning, piercing, and slashing damage."},
	bind:    bindRaging, validateState: validateRagingSnapshot,
})

const (
	ragingFresh     = "fresh"
	ragingLive      = "raging"
	ragingEnded     = "ended"
	ragingWasHit    = "was_hit_this_turn"
	ragingDidAttack = "did_attack_this_turn"
)

func bindRaging(raw json.RawMessage) (boundComposition, error) {
	var bindings RagingBindings
	if err := strictJSON(raw, &bindings); err != nil {
		return boundComposition{}, fmt.Errorf("raging bindings: %w", err)
	}
	amount, err := ragingDamageAmount(bindings.DamageBonus)
	if err != nil {
		return boundComposition{}, err
	}
	canonical, err := json.Marshal(bindings)
	if err != nil {
		return boundComposition{}, err
	}
	recipe, err := ragingRecipe(amount)
	if err != nil {
		return boundComposition{}, err
	}
	return boundComposition{bindings: canonical, recipe: recipe}, nil
}

func ragingRecipe(amount generic.Amount) (machine.Recipe, error) {
	parts, err := ruleeffects.CompileParts(&ruleeffects.CompilePartsInput{Parts: ragingBenefitParts(amount)})
	if err != nil {
		return machine.Recipe{}, err
	}
	outputs, err := ruleeffects.CompileOutputs(&ruleeffects.CompileOutputsInput{Parts: parts.Parts})
	if err != nil {
		return machine.Recipe{}, err
	}
	encoded := make([]jsontext.Value, 0, len(outputs.Outputs))
	for _, out := range outputs.Outputs {
		data, err := json.Marshal(out)
		if err != nil {
			return machine.Recipe{}, err
		}
		encoded = append(encoded, data)
	}
	upkeep := []machine.Act{{AnchorRound: true}, {DeriveTurnsActive: true}, {ClearFlags: []string{ragingWasHit, ragingDidAttack}}}
	endings := []machine.Transition{
		{On: ruleeffects.BeatDroppedToZero, About: ruleeffects.Holder, To: ragingEnded, Reason: "unconscious"},
		{On: ruleeffects.BeatRest, About: ruleeffects.Holder, To: ragingEnded, Reason: "rest"},
		{On: ruleeffects.BeatCombatEnd, About: ruleeffects.Holder, To: ragingEnded, Reason: "combat_ended"},
	}
	return machine.Recipe{Ref: *refs.Conditions.Raging(), Stacks: machine.StacksByRef, Initial: ragingFresh,
		Bag:      machine.Bag{ragingWasHit: false, ragingDidAttack: false, machine.KeySawTurnEnd: false, machine.KeyRoundActivated: 0, machine.KeyTurnsActive: 0},
		Trackers: []machine.Tracker{{On: ruleeffects.BeatAttackAttempt, About: ruleeffects.Holder, Set: ragingDidAttack}, {On: ruleeffects.BeatDamageReceived, About: ruleeffects.Holder, Set: ragingWasHit}},
		States: map[string]machine.State{
			ragingFresh: {Outputs: encoded, Transitions: append([]machine.Transition{{On: ruleeffects.BeatTurnEnd, About: ruleeffects.Holder, To: ragingLive, Do: upkeep}}, endings...)},
			ragingLive: {Outputs: encoded, Transitions: append([]machine.Transition{
				{On: ruleeffects.BeatTurnEnd, About: ruleeffects.Holder, When: &machine.Guard{Not: &machine.Guard{Any: []machine.Guard{{Flag: ragingWasHit}, {Flag: ragingDidAttack}}}}, To: ragingEnded, Reason: "no_combat_activity"},
				{On: ruleeffects.BeatTurnEnd, About: ruleeffects.Holder, When: &machine.Guard{Rounds: &machine.RoundsLeaf{SinceAnchor: &machine.IntOp{Op: machine.OpGte, N: rageDurationRounds - 1}}}, To: ragingEnded, Reason: "duration_expired"},
				{On: ruleeffects.BeatTurnEnd, About: ruleeffects.Holder, When: &machine.Guard{Rounds: &machine.RoundsLeaf{WentBackwards: true}}, To: ragingEnded, Reason: "clock_reset"},
				{On: ruleeffects.BeatTurnEnd, About: ruleeffects.Holder, Do: upkeep},
			}, endings...)},
			ragingEnded: {Terminal: true},
		},
	}, nil
}

func validateRagingSnapshot(snapshot generic.Snapshot) error {
	// Compose has already checked every scalar's kind and normalized whole numbers.
	saw := snapshot.Bag[machine.KeySawTurnEnd].(bool)
	anchor := snapshot.Bag[machine.KeyRoundActivated].(int)
	turns := snapshot.Bag[machine.KeyTurnsActive].(int)
	if anchor < 0 || turns < 0 || turns >= rageDurationRounds {
		return fmt.Errorf("raging runtime has invalid round/progress")
	}
	switch snapshot.State {
	case ragingFresh:
		if saw || anchor != 0 || turns != 0 {
			return fmt.Errorf("fresh Raging runtime cannot carry completed-turn progress")
		}
	case ragingLive:
		if !saw || (anchor == 0 && turns != 0) || (anchor > 0 && turns == 0) {
			return fmt.Errorf("raging runtime state and progress disagree")
		}
	default:
		return fmt.Errorf("raging runtime state %q is not live", snapshot.State)
	}
	return nil
}
