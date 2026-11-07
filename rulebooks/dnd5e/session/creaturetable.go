// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/table"
)

// resolvedTemper looks up the rulebook's profiles without choosing a policy or
// rolling a mixture. Canonical behavior configuration is already compiled and
// crosses Launch unchanged; there is no implicit kind-table layer here.
func resolvedTemper(in encounter.Temper) (encounter.Temper, error) {
	if in.Word != "" {
		profile, err := profileOf(in.Word)
		if err != nil {
			return encounter.Temper{}, err
		}
		return encounter.Temper{Word: in.Word, Profile: profile}, nil
	}
	if len(in.Mix) == 0 {
		return encounter.Temper{}, nil
	}
	profiles := make(map[string]encounter.TemperProfile, len(in.Mix))
	for word := range in.Mix {
		profile, err := profileOf(word)
		if err != nil {
			return encounter.Temper{}, fmt.Errorf("in the faction's mix: %w", err)
		}
		profiles[word] = profile
	}
	return encounter.Temper{Mix: in.Mix, Profiles: profiles}, nil
}

// profileOf returns the meaning of one authored temperament word from its owner.
func profileOf(word string) (encounter.TemperProfile, error) {
	parsed, err := table.ParseTemper(word)
	if err != nil {
		return encounter.TemperProfile{}, fmt.Errorf("temper: %w", err)
	}
	profile, err := parsed.Profile()
	if err != nil {
		return encounter.TemperProfile{}, fmt.Errorf("temper: %w", err)
	}
	return encounter.TemperProfile{Attack: profile.Attack, Toward: profile.Toward, Away: profile.Away, Flee: profile.Flee, Hold: profile.Hold}, nil
}
