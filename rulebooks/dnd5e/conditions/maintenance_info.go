// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	ruleeffects "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
)

type inspectMaintenanceInput struct {
	Parts    []ruleeffects.PartDeclaration
	Snapshot generic.AssemblySnapshot
}

// Inspection asks each kind to read its own state. It does not assert cross-part
// completion while a live occurrence is still notifying independent changes.
func inspectMaintenance(in *inspectMaintenanceInput) (*MaintenanceInfo, error) {
	result := &MaintenanceInfo{}
	for _, part := range in.Parts {
		state, ok := in.Snapshot.Parts[part.Key]
		if !ok {
			return nil, fmt.Errorf("part %q has no saved state", part.Key)
		}
		info, err := ruleeffects.InspectPart(&ruleeffects.InspectPartInput{Declaration: part, Snapshot: state})
		if err != nil {
			return nil, fmt.Errorf("part %q state: %w", part.Key, err)
		}
		if info.Concentration {
			result.concentrationParts++
		}
		if info.Duration != nil {
			result.Durations = append(result.Durations, NamedDurationInfo{PartKey: part.Key, Progress: *info.Duration})
		}
		if info.RoundDuration != nil {
			result.RoundDurations = append(result.RoundDurations, NamedRoundDurationInfo{PartKey: part.Key, Progress: *info.RoundDuration})
		}
	}
	if len(result.Durations) == 1 {
		progress := result.Durations[0].Progress
		result.Duration = &progress
	}
	if len(result.RoundDurations) == 1 {
		progress := result.RoundDurations[0].Progress
		result.RoundDuration = &progress
	}
	return result, nil
}

type assemblyRuntimeInput struct {
	Parts    []ruleeffects.PartDeclaration
	Snapshot generic.AssemblySnapshot
}

// Restore checks complete state and independent kind invariants. Required parts
// which report the same first-holder-end boundary must agree: restoring a fresh
// activity grace beside an already-observed clock would grant a second grace.
// This derived witness adds no shared mutable bag and is not checked mid-dispatch.
func validateAssemblyRuntime(in *assemblyRuntimeInput) error {
	if len(in.Parts) != len(in.Snapshot.Parts) {
		return fmt.Errorf("assembly runtime requires exact part coverage")
	}
	var seen *bool
	for _, part := range in.Parts {
		state, ok := in.Snapshot.Parts[part.Key]
		if !ok {
			return fmt.Errorf("assembly part %q has no runtime", part.Key)
		}
		info, err := ruleeffects.InspectPart(&ruleeffects.InspectPartInput{Declaration: part, Snapshot: state})
		if err != nil {
			return fmt.Errorf("assembly part %q: %w", part.Key, err)
		}
		if part.OnTerminal == generic.EndPart || info.TurnEndObserved == nil {
			continue
		}
		if seen != nil && *seen != *info.TurnEndObserved {
			return fmt.Errorf("required maintenance parts disagree on the first holder turn end")
		}
		observed := *info.TurnEndObserved
		seen = &observed
	}
	return nil
}
