// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// StaleTargetPolicy is the host's choice for known-creature casts aimed at an
// outdated remembered location. It is configuration, not a player spell option.
type StaleTargetPolicy string

const (
	// StaleTargetRefuse refuses the whole cast before spending.
	StaleTargetRefuse StaleTargetPolicy = "refuse"
	// StaleTargetAttempt pays once and reports misses for displaced recipients.
	StaleTargetAttempt StaleTargetPolicy = "attempt"
)

const missingStaleTargetPolicy = "Known-creature casting requires a stale-target policy"

// This new target shape binds host policy into its selector. An offer made
// under refusal must not become a paid attempt after a host reconfiguration.
func knownCastSelector(input *compileCastOfferInput, slot Slot, definition json.RawMessage, policy StaleTargetPolicy) (string, json.RawMessage, error) {
	raw, err := json.Marshal(struct {
		Definition json.RawMessage   `json:"definition"`
		Policy     StaleTargetPolicy `json:"stale_target_policy"`
	}{definition, policy})
	if err != nil {
		return "", nil, err
	}
	variant, err := canonicalSelectorVariant(raw)
	if err != nil {
		return "", nil, err
	}
	id, err := declarationIDWithVariant(declarationIDInput{Session: input.SessionID, Member: input.Member, Verb: VerbCast, Slot: slot}, variant)
	return id, variant, err
}

func knownCastCandidates(ctx context.Context, input *compileCastOfferInput, policy StaleTargetPolicy) ([]targetPreflight, error) {
	ids := []string{input.Member}
	seen := map[string]bool{input.Member: true}
	for _, holding := range excludeWorldNPCs(input.Holdings, rosterKinds(input.Roster)) {
		id := string(holding.Subject)
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	sort.Strings(ids)
	room, err := input.Encounter.Canvas()
	if err != nil {
		return nil, err
	}
	answers, err := resolution.KnownCreatureTargets(ctx, &resolution.KnownCreatureTargetsInput{
		Room: room, Encounter: input.Encounter, CasterID: input.Member,
		Candidates: ids, Participants: input.Participants, RangeFeet: input.Definition.Cast.RangeFeet,
		StaleTargetPolicy: resolution.StaleTargetPolicy(policy),
	})
	if err != nil {
		return nil, translateResolution(err)
	}
	out := make([]targetPreflight, 0, len(answers))
	for _, id := range ids {
		available, eligible := answers[id]
		if !eligible {
			continue
		}
		candidate := targetPreflight{member: id, available: available}
		if !available {
			candidate.why = &Shortfall{Reason: ShortfallTargetOutOfReach, Text: "Known target is unavailable"}
		}
		out = append(out, candidate)
	}
	return out, nil
}
