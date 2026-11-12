// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// DescribeSelectedRollContributionsInput supplies a recipient's ordered current
// condition snapshot and the actual roll-kind request.
type DescribeSelectedRollContributionsInput struct {
	Conditions []dndevents.ConditionBehavior
	Request    *dndevents.DescribeRollContributionsInput
}

// DescribeSelectedRollContributions selects the oldest applicable part in each
// group, then describes only selected parts. Information uses the same claimant.
func DescribeSelectedRollContributions(in *DescribeSelectedRollContributionsInput) (*dndevents.DescribeRollContributionsOutput, error) {
	if in == nil || in.Request == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "roll contribution request is required")
	}
	out := &dndevents.DescribeRollContributionsOutput{}
	groups := newRollGroupSelection()
	for _, condition := range in.Conditions {
		set, ok := condition.(dndevents.RollContributionSet)
		if !ok {
			continue
		}
		for _, provider := range set.RollContributors() {
			if provider == nil {
				return nil, fmt.Errorf("condition %T exposes a nil roll contributor", condition)
			}
			metadata, err := provider.RollContributionMetadata(in.Request)
			if err != nil {
				return nil, err
			}
			claim, err := groups.claim(&rollClaimInput{Metadata: metadata, Applies: metadata.Applicable})
			if err != nil {
				return nil, err
			}
			if !claim.Applicable || claim.Shadowed {
				continue
			}
			described, err := provider.DescribeRollContributions(in.Request)
			if err != nil {
				return nil, fmt.Errorf("describe roll contributions from %T: %w", condition, err)
			}
			if described == nil {
				return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "roll contribution provider %T returned nil output", provider)
			}
			out.Contributions = append(out.Contributions, described.Contributions...)
		}
	}
	return out, nil
}

type rollGroupSelection struct {
	holders map[string]string
}

func newRollGroupSelection() *rollGroupSelection {
	return &rollGroupSelection{holders: map[string]string{}}
}

type rollClaimInput struct {
	Metadata dndevents.RollContributionMetadataOutput
	Applies  bool
	Name     string
}

type rollClaimOutput struct {
	Applicable bool
	Shadowed   bool
	HeldBy     string
}

func (s *rollGroupSelection) claim(in *rollClaimInput) (*rollClaimOutput, error) {
	metadata := in.Metadata
	out := &rollClaimOutput{Applicable: metadata.Applicable}
	if !metadata.Applicable || !in.Applies {
		return out, nil
	}
	if metadata.Group == "" {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "applicable roll contribution %q declared an empty stacking group", in.Name)
	}
	if holder, held := s.holders[metadata.Group]; held {
		out.Shadowed, out.HeldBy = true, holder
		return out, nil
	}
	s.holders[metadata.Group] = in.Name
	return out, nil
}

// ConditionAddressOf derives exact identity on the named sheet. A current
// condition without a source qualifier names the exact empty-source address.
func ConditionAddressOf(memberID string, condition dndevents.ConditionBehavior) dndevents.ConditionAddress {
	if addressed, ok := condition.(dndevents.ConditionAddressProvider); ok {
		return addressed.ConditionAddress()
	}
	return dndevents.ConditionAddress{MemberID: memberID, ConditionRef: condition.Ref().String()}
}
