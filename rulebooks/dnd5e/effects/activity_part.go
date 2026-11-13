// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json/jsontext"
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

// KindCombatActivity sustains upkeep from the holder's attacks or applied damage.
// It owns neither a duration nor concentration; those are separate parts.
const KindCombatActivity generic.KindName = "combat_activity"

const (
	activityAttack = "attacked"
	activityDamage = "damaged"
)

type activityParams struct {
	SkipFirst bool `json:"skip_first"`
}

func parseActivity(decl PartDeclaration) (activityParams, error) {
	if decl.OnTerminal != generic.EndApplication {
		return activityParams{}, fmt.Errorf("activity upkeep must end its application")
	}
	return decodeParams[activityParams](jsontext.Value(decl.Params))
}
func validateActivityPart(decl PartDeclaration) error { _, err := parseActivity(decl); return err }
func bindActivityPart(in *BindPartInput) (boundPart, error) {
	params, err := parseActivity(in.Declaration)
	if err != nil {
		return boundPart{}, err
	}
	clearFlags := []machine.Act{{ClearFlags: []string{activityAttack, activityDamage}}}
	recipe := machine.Recipe{Ref: in.Ref, Initial: partStateActive, Bag: machine.Bag{activityAttack: false, activityDamage: false},
		Trackers: []machine.Tracker{{On: BeatAttackAttempt, About: Holder, Set: activityAttack}, {On: BeatDamageReceived, About: Holder, Set: activityDamage}},
		States: map[string]machine.State{
			partStateActive: {Transitions: []machine.Transition{
				{On: BeatTurnEnd, About: Holder, When: &machine.Guard{Not: &machine.Guard{Any: []machine.Guard{{Flag: activityAttack}, {Flag: activityDamage}}}}, To: partStateEnded, Reason: "no_combat_activity"},
				{On: BeatTurnEnd, About: Holder, Do: clearFlags},
			}},
			partStateEnded: {Terminal: true},
		},
	}
	if params.SkipFirst {
		recipe.Initial = partStateFresh
		recipe.States[partStateFresh] = machine.State{Transitions: []machine.Transition{{On: BeatTurnEnd, About: Holder, To: partStateActive, Do: clearFlags}}}
	}
	return boundPart{recipe: recipe}, nil
}
func inspectActivityPart(in *InspectPartInput) (*PartInfo, error) {
	params, err := parseActivity(in.Declaration)
	if err != nil {
		return nil, err
	}
	if len(in.Snapshot.Bag) != 2 {
		return nil, fmt.Errorf("activity requires its complete flag bag")
	}
	for _, key := range []string{activityAttack, activityDamage} {
		if _, ok := in.Snapshot.Bag[key].(bool); !ok {
			return nil, fmt.Errorf("activity flag %q must be boolean", key)
		}
	}
	switch in.Snapshot.State {
	case partStateFresh:
		if !params.SkipFirst {
			return nil, fmt.Errorf("activity grace disagrees with its declaration")
		}
	case partStateActive, partStateEnded:
	default:
		return nil, fmt.Errorf("unknown activity state %q", in.Snapshot.State)
	}
	return &PartInfo{}, nil
}
