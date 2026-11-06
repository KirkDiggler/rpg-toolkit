// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items_test

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/examples/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
)

const (
	rageHolder                      = "rage-wearer"
	rageGrant                       = "rage"
	rageFresh                       = "fresh"
	rageLive                        = "raging"
	rageEnded                       = "ended"
	rageWasHit                      = "was_hit_this_turn"
	rageAttacked                    = "did_attack_this_turn"
	rageSubject    machine.Selector = "holder"
	rageTurn       machine.Beat     = "turn_end"
	rageAttack     machine.Beat     = "attack_attempt"
	rageHit        machine.Beat     = "damage_received"
	rageZero       machine.Beat     = "dropped_to_zero"
	rageRest       machine.Beat     = "rest"
	rageCombatEnd  machine.Beat     = "combat_end"
	rageMelee      machine.FactName = "action.melee"
	rageWeapon     machine.FactName = "action.weapon_pool"
	rageAbility    machine.FactName = "action.ability"
	rageDamageKind effects.KindName = "damage_bonus"
	rageResistKind effects.KindName = "physical_resistance"
	rageSaveKind   effects.KindName = "save_advantage"
	rageCheckKind  effects.KindName = "check_advantage"
)

// These refs name this consumer's schemas, not production feature/condition loaders.
var rageRef = core.Ref{Module: "example", Type: "abilities", ID: "rage"}
var rageConditionRef = core.Ref{Module: "example", Type: "conditions", ID: "raging"}

type rageParameters struct {
	DamageBonus int `json:"damage_bonus"`
}

func rageItemDefinition() items.Definition {
	return items.Definition{
		Ref: core.Ref{Module: "example", Type: "items", ID: "raging_ring"}, Name: "Raging ring",
		Pools: map[string]items.PoolDefinition{chargePool: {Maximum: 1, RestoreOn: []items.Trigger{longRest}}},
		Grants: map[string]items.Grant{
			rageGrant: {Ref: rageRef, Parameters: json.RawMessage(`{"damage_bonus":2}`), Pool: chargePool, Uses: 1},
		},
	}
}

func rageItemConfig(id string) items.Config {
	return items.Config{
		ID: id, Definition: rageItemDefinition(),
		RecoveryTriggers: []items.Trigger{shortRest, longRest},
		ValidateGrant:    func(grant items.Grant) error { _, err := parseRage(grant); return err },
	}
}

func parseRage(grant items.Grant) (rageParameters, error) {
	if grant.Ref != rageRef {
		return rageParameters{}, fmt.Errorf("unknown Rage consumer ref %s", grant.Ref.String())
	}
	var p rageParameters
	decoder := json.NewDecoder(bytes.NewReader(grant.Parameters))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return p, err
	}
	if p.DamageBonus <= 0 {
		return p, fmt.Errorf("Rage example requires a positive authored damage bonus")
	}
	return p, nil
}

type rageActivation struct {
	Source items.Source
	Effect *effects.Composed
}

func useRage(ctx context.Context, item *items.Item, holder string) (*items.UseOutput[*rageActivation], error) {
	return items.Use(ctx, items.UseInput[*rageActivation]{
		Item: item, HolderID: holder, GrantID: rageGrant,
		Resolve: func(_ context.Context, offer items.Offer) (*rageActivation, error) {
			p, err := parseRage(offer.Grant)
			if err != nil {
				return nil, err
			}
			recipe, err := itemRageRecipe(p)
			if err != nil {
				return nil, err
			}
			sourceRef := offer.Source.ItemRef
			effect, err := effects.New(effects.Config{
				Recipe: recipe, MachineVocabulary: itemRageVocabulary(),
				OutputVocabulary: itemRageOutputs(offer.Source), HolderID: offer.Source.HolderID, SourceRef: &sourceRef,
			})
			if err != nil {
				return nil, err
			}
			return &rageActivation{Source: offer.Source, Effect: effect}, nil
		},
	})
}

// The state/transition data follows the existing effects prototype's Rage walk.
// The item supplies a fixed bonus; it never fabricates Barbarian class levels.
func itemRageRecipe(p rageParameters) (machine.Recipe, error) {
	fact := func(name machine.FactName, value any) machine.Guard {
		return machine.Guard{Fact: &machine.FactLeaf{Name: name, Op: machine.OpEq, Value: value}}
	}
	strength := fact(rageAbility, string(abilities.STR))
	outputs := []effects.Output{
		{Kind: rageDamageKind, When: &machine.Guard{All: []machine.Guard{
			fact(rageMelee, true), fact(rageWeapon, true), strength,
		}}, Amount: &effects.Amount{Fixed: &p.DamageBonus}},
		{Kind: rageResistKind},
		{Kind: rageSaveKind, When: &strength},
		{Kind: rageCheckKind, When: &strength},
	}
	encoded := make([]jsontext.Value, 0, len(outputs))
	for _, out := range outputs {
		data, err := json.Marshal(out)
		if err != nil {
			return machine.Recipe{}, err
		}
		encoded = append(encoded, jsontext.Value(data))
	}
	upkeep := []machine.Act{
		{AnchorRound: true}, {DeriveTurnsActive: true}, {ClearFlags: []string{rageWasHit, rageAttacked}},
	}
	endings := []machine.Transition{
		{On: rageZero, About: rageSubject, To: rageEnded, Reason: "unconscious"},
		{On: rageRest, About: rageSubject, To: rageEnded, Reason: "rest"},
		{On: rageCombatEnd, About: rageSubject, To: rageEnded, Reason: "combat_ended"},
	}
	return machine.Recipe{
		Ref: rageConditionRef, Stacks: machine.StacksByRef, Initial: rageFresh,
		Bag: machine.Bag{
			rageWasHit: false, rageAttacked: false, machine.KeySawTurnEnd: false,
			machine.KeyRoundActivated: 0, machine.KeyTurnsActive: 0,
		},
		Trackers: []machine.Tracker{
			{On: rageAttack, About: rageSubject, Set: rageAttacked},
			{On: rageHit, About: rageSubject, Set: rageWasHit},
		},
		States: map[string]machine.State{
			rageFresh: {Outputs: encoded, Transitions: append([]machine.Transition{
				{On: rageTurn, About: rageSubject, To: rageLive, Do: upkeep},
			}, endings...)},
			rageLive: {Outputs: encoded, Transitions: append([]machine.Transition{
				{On: rageTurn, About: rageSubject, When: &machine.Guard{Not: &machine.Guard{Any: []machine.Guard{
					{Flag: rageWasHit}, {Flag: rageAttacked},
				}}}, To: rageEnded, Reason: "no_combat_activity"},
				{On: rageTurn, About: rageSubject, When: &machine.Guard{Rounds: &machine.RoundsLeaf{
					SinceAnchor: &machine.IntOp{Op: machine.OpGte, N: 9},
				}}, To: rageEnded, Reason: "duration_expired"},
				{On: rageTurn, About: rageSubject, When: &machine.Guard{Rounds: &machine.RoundsLeaf{
					WentBackwards: true,
				}}, To: rageEnded, Reason: "clock_reset"},
				{On: rageTurn, About: rageSubject, Do: upkeep},
			}, endings...)},
			rageEnded: {Terminal: true},
		},
	}, nil
}
