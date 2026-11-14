// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// DescribeSelectedRollContributionsInput supplies a recipient's ordered current
// condition snapshot and the actual roll-kind request.
type DescribeSelectedRollContributionsInput struct {
	Conditions []dndevents.ConditionBehavior
	Request    *dndevents.DescribeRollContributionsInput
}

// DescribeSelectedRollContributions gathers applicable output identities in
// persisted condition/part order, applies their explicit neutral selection rules,
// and describes only selected contributions. It neither rolls nor mutates owners.
// Information consumes the same neutral selector, not another policy loop.
func DescribeSelectedRollContributions(in *DescribeSelectedRollContributionsInput) (*dndevents.DescribeRollContributionsOutput, error) {
	if in == nil || in.Request == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "roll contribution request is required")
	}
	var candidates []generic.SelectionCandidate
	providers := map[string]dndevents.RollContributionProvider{}
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
			if !metadata.Applicable {
				continue
			}
			candidates = append(candidates, generic.SelectionCandidate{ID: metadata.ID, Rule: metadata.Selection})
			providers[metadata.ID] = provider
		}
	}
	selected, err := generic.SelectContributions(&generic.SelectContributionsInput{Candidates: candidates})
	if err != nil {
		return nil, err
	}
	out := &dndevents.DescribeRollContributionsOutput{}
	for _, decision := range selected.Decisions {
		if !decision.Selected {
			continue
		}
		provider := providers[decision.ID]
		described, err := provider.DescribeRollContributions(in.Request)
		if err != nil {
			return nil, fmt.Errorf("describe roll contributions from %T: %w", provider, err)
		}
		if described == nil {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "roll contribution provider %T returned nil output", provider)
		}
		out.Contributions = append(out.Contributions, described.Contributions...)
	}
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
