// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"encoding/json/jsontext"
	"fmt"
	"slices"
)

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

// RoundDurationDeclaration describes an anchored-round maximum, not a received
// turn-end count or current elapsed progress.
type RoundDurationDeclaration struct{ Rounds int }

// NamedRoundDurationDeclaration keeps each independent anchored clock distinct.
type NamedRoundDurationDeclaration struct {
	PartKey  string
	Duration RoundDurationDeclaration
}

// DescribeMaintenanceInput supplies source-authored parts without runtime identity.
type DescribeMaintenanceInput struct{ Parts []PartDeclaration }

// MaintenanceDescription reports capabilities without binding or rolling. Duration
// is present only for one clock; Durations preserves all independently authored clocks.
type MaintenanceDescription struct {
	Concentration  bool
	Duration       *DurationDeclaration
	Durations      []NamedDurationDeclaration
	RoundDuration  *RoundDurationDeclaration
	RoundDurations []NamedRoundDurationDeclaration
}

type partDescription struct {
	capabilities  PartCapabilities
	duration      *DurationDeclaration
	roundDuration *RoundDurationDeclaration
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
		if description.roundDuration != nil {
			result.RoundDurations = append(result.RoundDurations, NamedRoundDurationDeclaration{PartKey: part.Key, Duration: *description.roundDuration})
		}
	}
	if len(result.Durations) == 1 {
		duration := result.Durations[0].Duration
		result.Duration = &duration
	}
	if len(result.RoundDurations) == 1 {
		duration := result.RoundDurations[0].Duration
		result.RoundDuration = &duration
	}
	return result, nil
}

func describePart(part PartDeclaration) (partDescription, error) {
	if err := ValidatePart(&ValidatePartInput{Declaration: part}); err != nil {
		return partDescription{}, err
	}
	definition := kindTable[part.Kind]
	var result partDescription
	if definition.describePart != nil {
		description, err := definition.describePart(part)
		if err != nil {
			return partDescription{}, err
		}
		result = description
	}
	result.capabilities = definition.capabilities
	result.capabilities.AllowedScopes = slices.Clone(definition.capabilities.AllowedScopes)
	return result, nil
}

func describeRoundDurationPart(part PartDeclaration) (partDescription, error) {
	params, err := parseRoundDurationParams(jsontext.Value(part.Params))
	if err != nil {
		return partDescription{}, err
	}
	return partDescription{roundDuration: &RoundDurationDeclaration{Rounds: params.Rounds}}, nil
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
