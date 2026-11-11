// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import "fmt"

// DurationDeclaration describes an authored maximum and grace, not saved progress.
type DurationDeclaration struct {
	TurnEnds         int
	SkipFirstTurnEnd bool
}

// NamedDurationDeclaration preserves each independently authored clock's identity.
type NamedDurationDeclaration struct {
	PartKey  string
	Duration DurationDeclaration
}

// DescribeMaintenanceInput supplies source-authored parts without runtime identity.
type DescribeMaintenanceInput struct{ Parts []PartDeclaration }

// MaintenanceDescription reports capabilities without binding or rolling. Duration
// is present only for one clock; Durations preserves all independently authored clocks.
type MaintenanceDescription struct {
	Concentration bool
	Duration      *DurationDeclaration
	Durations     []NamedDurationDeclaration
}

type partDescription struct {
	capabilities PartCapabilities
	duration     *DurationDeclaration
}

// DescribeMaintenance validates parts and reports their maintenance requirements.
// Missing input, duplicate keys, invalid parts and multiple concentration claims
// are errors. Neither a source ref nor a spell identity is needed to describe parts.
func DescribeMaintenance(in *DescribeMaintenanceInput) (*MaintenanceDescription, error) {
	if in == nil {
		return nil, fmt.Errorf("maintenance description input is required")
	}
	result := &MaintenanceDescription{}
	seen := make(map[string]bool, len(in.Parts))
	for _, part := range in.Parts {
		if seen[part.Key] {
			return nil, fmt.Errorf("duplicate maintenance part key %q", part.Key)
		}
		seen[part.Key] = true
		description, err := describePart(part)
		if err != nil {
			return nil, fmt.Errorf("maintenance part %q: %w", part.Key, err)
		}
		if description.capabilities.Concentration {
			if result.Concentration {
				return nil, fmt.Errorf("multiple concentration claims")
			}
			result.Concentration = true
		}
		if description.duration != nil {
			result.Durations = append(result.Durations, NamedDurationDeclaration{PartKey: part.Key, Duration: *description.duration})
		}
	}
	if len(result.Durations) == 1 {
		duration := result.Durations[0].Duration
		result.Duration = &duration
	}
	return result, nil
}

func describePart(part PartDeclaration) (partDescription, error) {
	if err := ValidatePart(&ValidatePartInput{Declaration: part}); err != nil {
		return partDescription{}, err
	}
	definition := kindTable[part.Kind]
	if definition.describePart != nil {
		return definition.describePart(part)
	}
	return partDescription{}, nil
}

func describeConcentrationPart(_ PartDeclaration) (partDescription, error) {
	return partDescription{capabilities: PartCapabilities{Concentration: true}}, nil
}

func describeDurationPart(part PartDeclaration) (partDescription, error) {
	params, err := parseDuration(part)
	if err != nil {
		return partDescription{}, err
	}
	return partDescription{capabilities: PartCapabilities{Duration: true}, duration: &DurationDeclaration{TurnEnds: params.TurnEnds, SkipFirstTurnEnd: params.SkipFirst}}, nil
}
