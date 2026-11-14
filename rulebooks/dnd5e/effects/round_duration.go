// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json/jsontext"
	"fmt"
	"strings"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

// KindRoundDuration measures an authored maximum from the first positive holder
// round observed. It is not a count of received turn ends or a new clock.
const KindRoundDuration generic.KindName = "round_duration"

const factTurnRoundAvailable machine.FactName = "turn.round_available"

type roundDurationParams struct {
	Rounds        int    `json:"rounds"`
	ExpiredReason string `json:"expired_reason"`
	ResetReason   string `json:"reset_reason"`
}

// RoundDurationInfo describes the part's sole anchored progress authority.
// Unanchored means RoundActivated/TurnsActive are zero, not a guessed round.
type RoundDurationInfo struct {
	Anchored       bool
	RoundActivated int
	TurnsActive    int
	TotalRounds    int
}

func parseRoundDurationParams(raw jsontext.Value) (roundDurationParams, error) {
	params, err := decodeParams[roundDurationParams](raw)
	if err != nil {
		return roundDurationParams{}, err
	}
	if params.Rounds <= 0 || strings.TrimSpace(params.ExpiredReason) == "" || strings.TrimSpace(params.ResetReason) == "" {
		return roundDurationParams{}, fmt.Errorf("round duration requires positive rounds and explicit expiry/reset reasons")
	}
	return params, nil
}
func validateRoundDurationEffect(spec EffectSpec) error {
	if err := validateMaintenanceFields(spec); err != nil {
		return err
	}
	_, err := parseRoundDurationParams(jsontext.Value(spec.Params))
	return err
}
func validateRoundDurationPart(part PartDeclaration) error {
	return validateRoundDurationEffect(EffectSpec{Kind: part.Kind, Params: part.Params, When: part.When, Amount: part.Amount})
}

func bindRoundDurationPart(in *BindPartInput) (boundPart, error) {
	params, err := parseRoundDurationParams(jsontext.Value(in.Declaration.Params))
	if err != nil {
		return boundPart{}, err
	}
	upkeep := []machine.Act{{AnchorRound: true}, {DeriveTurnsActive: true}}
	transitions := []machine.Transition{
		{On: BeatTurnEnd, About: Holder, When: &machine.Guard{Rounds: &machine.RoundsLeaf{SinceAnchor: &machine.IntOp{Op: machine.OpGte, N: params.Rounds - 1}}}, To: partStateEnded, Reason: params.ExpiredReason},
		{On: BeatTurnEnd, About: Holder, When: &machine.Guard{Rounds: &machine.RoundsLeaf{WentBackwards: true}}, To: partStateEnded, Reason: params.ResetReason},
	}
	if params.Rounds == 1 {
		// The first positive observation is already the whole one-round duration.
		// It cannot ask SinceAnchor before that observation has established an anchor.
		transitions = append(transitions, machine.Transition{On: BeatTurnEnd, About: Holder, When: &machine.Guard{Fact: &machine.FactLeaf{Name: factTurnRoundAvailable, Op: machine.OpEq, Value: true}}, To: partStateEnded, Reason: params.ExpiredReason, Do: upkeep})
	}
	transitions = append(transitions, machine.Transition{On: BeatTurnEnd, About: Holder, Do: upkeep})
	return boundPart{recipe: machine.Recipe{Ref: in.Ref, Initial: partStateActive, Bag: machine.Bag{machine.KeySawTurnEnd: false, machine.KeyRoundActivated: 0, machine.KeyTurnsActive: 0}, States: map[string]machine.State{
		partStateActive: {Transitions: transitions}, partStateEnded: {Terminal: true},
	}}}, nil
}

func inspectRoundDurationPart(in *InspectPartInput) (*PartInfo, error) {
	params, err := parseRoundDurationParams(jsontext.Value(in.Declaration.Params))
	if err != nil {
		return nil, err
	}
	state := in.Snapshot
	if state.State != partStateActive && state.State != partStateEnded {
		return nil, fmt.Errorf("unknown round-duration state %q", state.State)
	}
	if state.Bag == nil || len(state.Bag) != 3 {
		return nil, fmt.Errorf("round duration requires its complete progress bag")
	}
	saw, okSaw := state.Bag[machine.KeySawTurnEnd].(bool)
	anchor, okAnchor := state.Bag[machine.KeyRoundActivated].(int)
	turns, okTurns := state.Bag[machine.KeyTurnsActive].(int)
	if !okSaw || !okAnchor || !okTurns || anchor < 0 || turns < 0 {
		return nil, fmt.Errorf("round-duration progress has invalid types or bounds")
	}
	if (!saw && (anchor != 0 || turns != 0)) || (anchor == 0 && turns != 0) || (anchor > 0 && (!saw || turns == 0)) {
		return nil, fmt.Errorf("round-duration progress disagrees with its anchor")
	}
	if turns > params.Rounds || (state.State == partStateActive && turns >= params.Rounds) {
		return nil, fmt.Errorf("round-duration progress is inconsistent with its maximum/state")
	}
	if state.State == partStateEnded && anchor == 0 {
		return nil, fmt.Errorf("ended round duration has no anchor")
	}
	return &PartInfo{RoundDuration: &RoundDurationInfo{Anchored: anchor > 0, RoundActivated: anchor, TurnsActive: turns, TotalRounds: params.Rounds}, TurnEndObserved: &saw}, nil
}
