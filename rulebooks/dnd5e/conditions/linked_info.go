// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// LinkedApplicationInfo exposes copied identity/link facts, not runtime pointers.
type LinkedApplicationInfo struct {
	ID             string
	Role           LinkedRole
	Owner          dndevents.ConditionAddress
	MemberKey      string
	Members        []LinkedMemberBinding
	LiveMembers    []string
	PendingMembers []string
}

// ConcentrationInfo describes the source held by this coordinator.
type ConcentrationInfo struct {
	Owner      dndevents.ConditionAddress
	SourceRef  core.Ref
	SourceName string
	ChildCount int
}

// NamedDurationInfo keeps independently authored duration parts distinct.
type NamedDurationInfo struct {
	PartKey  string
	Progress dndfx.DurationInfo
}

// MaintenanceInfo contains only locally known maintenance. Duration is the
// convenient single-clock view when exactly one duration exists; Durations keeps
// every authored part without inventing a combined countdown.
type MaintenanceInfo struct {
	Concentration *ConcentrationInfo
	Duration      *dndfx.DurationInfo
	Durations     []NamedDurationInfo
}

// LinkedApplicationInfo returns detached address bindings and live keys.
func (c *LinkedCondition) LinkedApplicationInfo() LinkedApplicationInfo {
	info := LinkedApplicationInfo{ID: c.application, Role: c.role, MemberKey: c.bindings.MemberKey,
		Members: slices.Clone(c.bindings.Members)}
	if c.role == LinkedCoordinator {
		info.Owner = c.ConditionAddress()
		snapshot := c.lifetime.Snapshot()
		info.LiveMembers = snapshot.LiveMembers
		info.PendingMembers = snapshot.PendingMembers
	} else {
		info.Owner = *c.bindings.Owner
	}
	return info
}

// MaintenanceInfo asks each part's owning meaning to interpret its snapshot.
// A recipient does not infer its coordinator's countdown from a link.
func (c *LinkedCondition) MaintenanceInfo() (*MaintenanceInfo, error) {
	result := &MaintenanceInfo{}
	snapshot := c.assembly.Snapshot()
	for _, declaration := range c.bindings.Parts {
		info, err := dndfx.InspectPart(&dndfx.InspectPartInput{Declaration: declaration, Snapshot: snapshot.Parts[declaration.Key]})
		if err != nil {
			return nil, fmt.Errorf("linked part %q state: %w", declaration.Key, err)
		}
		if info.Concentration {
			if result.Concentration != nil || c.role != LinkedCoordinator {
				return nil, fmt.Errorf("invalid linked concentration custody")
			}
			result.Concentration = &ConcentrationInfo{Owner: c.ConditionAddress(), SourceRef: c.origin.Ref,
				SourceName: c.bindings.Name, ChildCount: len(c.lifetime.Snapshot().LiveMembers)}
		}
		if info.Duration != nil {
			result.Durations = append(result.Durations, NamedDurationInfo{PartKey: declaration.Key, Progress: *info.Duration})
		}
	}
	if len(result.Durations) == 1 {
		progress := result.Durations[0].Progress
		result.Duration = &progress
	}
	return result, nil
}

// DescribeConditionInput supplies canonical loaded behavior, never guessed JSON.
type DescribeConditionInput struct{ Condition dndevents.ConditionBehavior }

// DescribeConditionOutput is the rulebook-owned view a host maps without formulas.
type DescribeConditionOutput struct {
	Display  Display
	Origin   *dndevents.ConditionOrigin
	Duration *dndfx.DurationInfo
}

// DescribeCondition returns registry display plus instance-owned metadata where
// available. It refuses missing/unregistered conditions and invalid part state.
func DescribeCondition(in *DescribeConditionInput) (*DescribeConditionOutput, error) {
	if in == nil || in.Condition == nil || in.Condition.Ref() == nil {
		return nil, fmt.Errorf("condition description requires a condition")
	}
	entry, err := lookup(in.Condition.Ref().String())
	if err != nil {
		return nil, err
	}
	out := &DescribeConditionOutput{Display: entry.display}
	if origin, ok := in.Condition.(dndevents.ConditionOriginProvider); ok {
		copy := origin.ConditionOrigin()
		out.Origin = &copy
	}
	if linked, ok := in.Condition.(*LinkedCondition); ok {
		info, err := linked.MaintenanceInfo()
		if err != nil {
			return nil, err
		}
		out.Display.Name = linked.bindings.Name
		out.Duration = info.Duration
		if info.Concentration != nil {
			out.Display.Name = "Concentrating on " + info.Concentration.SourceName
			out.Display.Detail = "Requires concentration."
		}
		details := []string{out.Display.Detail}
		for _, clock := range info.Durations {
			text := fmt.Sprintf("%d maintainer turn ends remaining.", clock.Progress.Remaining)
			if len(info.Durations) > 1 {
				text = clock.PartKey + ": " + text
			}
			if clock.Progress.SkipNext {
				text += " The next turn end is a grace boundary."
			}
			details = append(details, text)
		}
		out.Display.Detail = strings.Join(details, " ")
	}
	return out, nil
}

type linkedRollProvider struct {
	output *generic.Bound
	origin dndevents.ConditionOrigin
	name   string
}

func (p linkedRollProvider) RollContributionMetadata(in *dndevents.DescribeRollContributionsInput) (dndevents.RollContributionMetadataOutput, error) {
	if in == nil || in.Kind == "" {
		return dndevents.RollContributionMetadataOutput{}, nil
	}
	out, err := dndfx.DescribeRoll(&dndfx.DescribeRollInput{Output: p.output, Origin: p.origin, DisplayName: p.name, Request: in})
	if err != nil {
		return dndevents.RollContributionMetadataOutput{}, err
	}
	return dndevents.RollContributionMetadataOutput{Applicable: out.Applicable, Group: out.Group}, nil
}

func (p linkedRollProvider) DescribeRollContributions(in *dndevents.DescribeRollContributionsInput) (*dndevents.DescribeRollContributionsOutput, error) {
	out, err := dndfx.DescribeRoll(&dndfx.DescribeRollInput{Output: p.output, Origin: p.origin, DisplayName: p.name, Request: in})
	if err != nil {
		return nil, err
	}
	return &dndevents.DescribeRollContributionsOutput{Contributions: out.Contributions}, nil
}

// RollContributors exposes each local roll part independently to the one holder
// selector. One condition can contain different groups without flattening them.
func (c *LinkedCondition) RollContributors() []dndevents.RollContributionProvider {
	var providers []dndevents.RollContributionProvider
	for _, part := range c.assembly.CurrentOutputs() {
		if part.Output.Kind() == dndfx.KindRollModifier {
			providers = append(providers, linkedRollProvider{output: part.Output, origin: c.origin, name: c.bindings.Name})
		}
	}
	return providers
}

// DescribeRollContributions uses the same ordered selector as a holder; it does
// not independently stack all dice merely because they share one condition.
func (c *LinkedCondition) DescribeRollContributions(in *dndevents.DescribeRollContributionsInput) (*dndevents.DescribeRollContributionsOutput, error) {
	return DescribeSelectedRollContributions(&DescribeSelectedRollContributionsInput{Conditions: []dndevents.ConditionBehavior{c}, Request: in})
}
