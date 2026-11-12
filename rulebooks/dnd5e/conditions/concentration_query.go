// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"

	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ConcentrationQueryInput names one holder's current condition collection.
type ConcentrationQueryInput struct{ Conditions []dndevents.ConditionBehavior }

// ConcentrationQueryOutput explicitly represents presence without a successful
// nil object. Info is a detached value and is meaningful only while Held is true.
type ConcentrationQueryOutput struct {
	Held bool
	Info ConcentrationInfo
}

// ConcentrationHeld asks maintenance capabilities rather than a concrete class
// or ref. Invalid providers and multiple claims are errors, not absence.
func ConcentrationHeld(in *ConcentrationQueryInput) (*ConcentrationQueryOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("concentration query requires conditions")
	}
	out := &ConcentrationQueryOutput{}
	for _, condition := range in.Conditions {
		if condition == nil {
			return nil, fmt.Errorf("concentration query contains a nil condition")
		}
		provider, ok := condition.(interface {
			MaintenanceInfo() (*MaintenanceInfo, error)
		})
		if !ok {
			continue
		}
		info, err := provider.MaintenanceInfo()
		if err != nil {
			return nil, err
		}
		if info == nil {
			return nil, fmt.Errorf("maintenance provider returned no information")
		}
		if info.Concentration == nil {
			continue
		}
		if out.Held {
			return nil, fmt.Errorf("holder has multiple concentration claims")
		}
		out.Held = true
		out.Info = *info.Concentration
	}
	return out, nil
}
