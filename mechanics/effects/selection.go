// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"fmt"
	"strings"
)

// SelectionMode names a contribution policy, not an instance-admission rule.
// Its zero value is invalid; callers must choose how applicable values compete.
type SelectionMode string

const (
	// AllApplicable selects each candidate independently and accepts no group.
	AllApplicable SelectionMode = "all_applicable"
	// OldestApplicable selects the first candidate in each declared group.
	// The caller supplies oldest-first order; selection does not read a clock.
	OldestApplicable SelectionMode = "oldest_applicable"
)

// SelectionRule declares how an applicable contribution participates. Group is
// an opaque exclusion key required only by OldestApplicable, not an identity
// or an instruction to replace another instance.
type SelectionRule struct {
	Mode  SelectionMode `json:"mode"`
	Group string        `json:"group,omitempty"`
}

// ValidateSelectionRuleInput supplies an authored policy without runtime IDs.
type ValidateSelectionRuleInput struct{ Rule SelectionRule }

// ValidateSelectionRule refuses absent input, unknown/empty modes, a missing
// oldest-first group or a group on AllApplicable. It neither normalizes keys
// nor constructs a candidate, so definition authors need no fake identities.
func ValidateSelectionRule(in *ValidateSelectionRuleInput) error {
	if in == nil {
		return fmt.Errorf("selection rule input is required")
	}
	switch in.Rule.Mode {
	case AllApplicable:
		if in.Rule.Group != "" {
			return fmt.Errorf("all-applicable selection forbids a group")
		}
	case OldestApplicable:
		if strings.TrimSpace(in.Rule.Group) == "" {
			return fmt.Errorf("oldest-applicable selection requires a group")
		}
	default:
		return fmt.Errorf("unsupported selection mode %q", in.Rule.Mode)
	}
	return nil
}

// SelectionCandidate is one already-applicable value in caller-owned priority
// order. ID names the contribution, not its whole owning condition or source.
type SelectionCandidate struct {
	ID   string
	Rule SelectionRule
}

// SelectionDecision describes one input candidate. SelectedInstead names the
// earlier winning candidate when Selected is false; it is empty for winners.
// Exclusion does not suspend, remove or otherwise mutate the owning instance.
type SelectionDecision struct {
	ID              string
	Selected        bool
	SelectedInstead string
}

// SelectContributionsInput supplies applicable candidates oldest-first, using
// a stable caller-owned order. No lifetime, quantity or runtime handle crosses
// this boundary. An empty candidate list is valid.
type SelectContributionsInput struct{ Candidates []SelectionCandidate }

// SelectContributionsOutput owns one decision per candidate in input order.
// A result retains no input slice or state for the next selection query.
type SelectContributionsOutput struct{ Decisions []SelectionDecision }

// SelectContributions validates the entire list, then applies each explicit
// policy without sorting, rolling or mutating. Nil input, malformed rules,
// blank IDs and duplicate IDs are errors with no partial output. Every call
// recomputes from its supplied applicable list; there is no stored winner.
func SelectContributions(in *SelectContributionsInput) (*SelectContributionsOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("contribution selection input is required")
	}
	seen := make(map[string]struct{}, len(in.Candidates))
	for i, candidate := range in.Candidates {
		if strings.TrimSpace(candidate.ID) == "" {
			return nil, fmt.Errorf("selection candidate %d requires an id", i)
		}
		if _, exists := seen[candidate.ID]; exists {
			return nil, fmt.Errorf("duplicate selection candidate id %q", candidate.ID)
		}
		seen[candidate.ID] = struct{}{}
		if err := ValidateSelectionRule(&ValidateSelectionRuleInput{Rule: candidate.Rule}); err != nil {
			return nil, fmt.Errorf("selection candidate %q: %w", candidate.ID, err)
		}
	}
	out := &SelectContributionsOutput{Decisions: make([]SelectionDecision, 0, len(in.Candidates))}
	winners := make(map[string]string)
	for _, candidate := range in.Candidates {
		decision := SelectionDecision{ID: candidate.ID, Selected: true}
		if candidate.Rule.Mode == OldestApplicable {
			if winner, exists := winners[candidate.Rule.Group]; exists {
				decision.Selected = false
				decision.SelectedInstead = winner
			} else {
				winners[candidate.Rule.Group] = candidate.ID
			}
		}
		out.Decisions = append(out.Decisions, decision)
	}
	return out, nil
}
