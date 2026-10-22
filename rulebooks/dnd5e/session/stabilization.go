// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
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
