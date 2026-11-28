// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/combat"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// actionSurgeAbility is Action Surge's entry in the table of named abilities:
// one use, and it gives one more action this turn. It charges no slot (it is
// free on your turn), so it is counted: its giver must pay it from a count.
var actionSurgeAbility = namedAbility{
	word:    "action surge",
	ref:     func() core.Ref { return *refs.Features.ActionSurge() },
	label:   "Action Surge",
	counted: true,
	check: func(raw json.RawMessage) error {
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("{}")) || bytes.Equal(trimmed, []byte("null")) {
			return nil
		}
		return fmt.Errorf("action surge takes no values")
	},
	describe: func(in *describeInput) (*actions.Definition, error) {
		return &actions.Definition{
			Ref:         *refs.Features.ActionSurge(),
			Name:        "Action Surge",
			Description: "Take one more action this turn",
			Cost: &dnd5eCombat.SpendProfile{
				Pools:      in.pools,
				GrantSlots: map[combat.ActionType]int{combat.ActionStandard: 1},
			},
		}, nil
	},
}
