// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
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
	fixed := bindings.DamageBonus.Fixed
	byClass := bindings.DamageBonus.BarbarianLevel
	if (fixed != nil) == byClass {
		return boundComposition{}, fmt.Errorf("raging damage_bonus requires exactly one fixed or barbarian_level strategy")
	}
	if fixed != nil && *fixed <= 0 {
		return boundComposition{}, fmt.Errorf("raging fixed damage bonus must be positive")
	}
	amount := generic.Amount{Fixed: fixed}
	if byClass {
		amount = generic.Amount{ByClassLevel: &generic.ClassSteps{Class: "barbarian", Steps: map[int]int{1: 2, 9: 3, 16: 4}}}
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
	fact := func(name machine.FactName, value any) machine.Guard {
		return machine.Guard{Fact: &machine.FactLeaf{Name: name, Op: machine.OpEq, Value: value}}
	}
	strength := fact(ruleeffects.FactAbility, string(abilities.STR))
	outputs := []generic.Output{
		{Kind: ruleeffects.KindDamageBonus, When: &machine.Guard{All: []machine.Guard{fact(ruleeffects.FactMelee, true), fact(ruleeffects.FactWeaponPool, true), strength}}, Amount: &amount},
		{Kind: ruleeffects.KindResistance, Params: jsontext.Value(`{"types":["bludgeoning","piercing","slashing"]}`)},
		{Kind: ruleeffects.KindAdvantage, When: &strength, Params: jsontext.Value(`{"on":"saving_throw"}`)},
		{Kind: ruleeffects.KindAdvantage, When: &strength, Params: jsontext.Value(`{"on":"ability_check"}`)},
	}
	encoded := make([]jsontext.Value, 0, len(outputs))
	for _, out := range outputs {
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
