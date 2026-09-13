// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func stabilizationDetail(result character.StabilizeOutput) *encounter.StabilizationDetail {
	return &encounter.StabilizationDetail{
		Before: string(result.Before), After: string(result.After), HitPoints: result.HitPoints,
		Successes: result.Progress.Successes, Failures: result.Progress.Failures,
		SuccessesNeeded: result.Progress.SuccessesNeeded, FailuresRemaining: result.Progress.FailuresRemaining,
		Stabilized: result.Progress.Stabilized, Dead: result.Progress.Dead,
	}
}

func decodeStabilization(raw json.RawMessage) (*StabilizedBody, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok {
		return nil, false
	}
	required := []string{"before", "after", "hit_points", "successes", "failures", "successes_needed", "failures_remaining", "stabilized", "dead"}
	if len(fields) != len(required) {
		return nil, false
	}
	for _, key := range required {
		value, present := fields[key]
		if !present || isJSONNull(value) {
			return nil, false
		}
	}
	var detail encounter.StabilizationDetail
	if json.Unmarshal(raw, &detail) != nil || detail.Before == "" || detail.After == "" {
		return nil, false
	}
	return &StabilizedBody{
		Before: LifeState(detail.Before), After: LifeState(detail.After), HitPoints: detail.HitPoints,
		Progress: DeathSaveProgress{
			Successes: detail.Successes, Failures: detail.Failures, SuccessesNeeded: detail.SuccessesNeeded,
			FailuresRemaining: detail.FailuresRemaining, Stabilized: detail.Stabilized, Dead: detail.Dead,
		},
	}, true
}
