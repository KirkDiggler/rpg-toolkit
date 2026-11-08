// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json/jsontext"
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

const (
	partStateFresh  = "fresh"
	partStateActive = "active"
	partStateEnded  = "ended"
	keyRemaining    = "remaining"
	reasonDuration  = "duration"
)

type durationParams struct {
	TurnEnds  int  `json:"turn_ends"`
	SkipFirst bool `json:"skip_first,omitempty"`
}

func parseDuration(declaration PartDeclaration) (durationParams, error) {
	if declaration.OnTerminal != generic.EndApplication {
		return durationParams{}, fmt.Errorf("duration must end its application")
	}
	params, err := decodeParams[durationParams](jsontext.Value(declaration.Params))
	if err != nil {
		return durationParams{}, err
	}
	if params.TurnEnds <= 0 {
		return durationParams{}, fmt.Errorf("duration requires positive turn_ends")
	}
	return params, nil
}

func bindDurationPart(in *BindPartInput) (boundPart, error) {
	params, err := parseDuration(in.Declaration)
	if err != nil {
		return boundPart{}, err
	}
	recipe := machine.Recipe{
		Ref: in.Ref, Initial: partStateActive, Bag: machine.Bag{keyRemaining: params.TurnEnds},
		States: map[string]machine.State{
			partStateActive: {Transitions: []machine.Transition{
				{On: BeatTurnEnd, About: Holder, When: &machine.Guard{Bag: &machine.BagLeaf{Key: keyRemaining, Op: machine.OpEq, Value: 1}}, To: partStateEnded, Reason: reasonDuration},
				{On: BeatTurnEnd, About: Holder, Do: []machine.Act{{Add: &machine.AddAct{Key: keyRemaining, Delta: -1}}}},
			}},
			partStateEnded: {Terminal: true},
		},
	}
	if params.SkipFirst {
		recipe.Initial = partStateFresh
		recipe.States[partStateFresh] = machine.State{Transitions: []machine.Transition{
			{On: BeatTurnEnd, About: Holder, To: partStateActive},
		}}
	}
	return boundPart{recipe: recipe, capabilities: PartCapabilities{Duration: true}}, nil
}

func inspectDurationPart(in *InspectPartInput) (*PartInfo, error) {
	params, err := parseDuration(in.Declaration)
	if err != nil {
		return nil, err
	}
	remaining, ok := in.Snapshot.Bag[keyRemaining].(int)
	if !ok || len(in.Snapshot.Bag) != 1 || remaining < 1 || remaining > params.TurnEnds {
		return nil, fmt.Errorf("duration requires its complete valid remaining counter")
	}
	switch in.Snapshot.State {
	case partStateFresh:
		if !params.SkipFirst || remaining != params.TurnEnds {
			return nil, fmt.Errorf("duration fresh state disagrees with its grace binding")
		}
		return &PartInfo{Duration: &DurationInfo{Remaining: remaining, SkipNext: true}}, nil
	case partStateActive:
		return &PartInfo{Duration: &DurationInfo{Remaining: remaining}}, nil
	case partStateEnded:
		return &PartInfo{Duration: &DurationInfo{}}, nil
	default:
		return nil, fmt.Errorf("unknown duration state %q", in.Snapshot.State)
	}
}
