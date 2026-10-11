// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// TestAtStakeReadsTheInstanceNotTheBlob: the declaration's parameter blob says
// "grovel" and the built instance says "flee". The effect carries the
// instance's word, so a detail filled from the blob or from cast input fails.
func TestAtStakeReadsTheInstanceNotTheBlob(t *testing.T) {
	blob, err := json.Marshal(map[string]string{spells.CommandWordParameter: spells.CommandWordGrovel})
	require.NoError(t, err)
	instance, err := conditions.NewCommandedCondition(
		"goblin", refs.Spells.Command().String(), "bard", spells.CommandWordFlee, 1)
	require.NoError(t, err)

	prepared := preparedCondition{
		declaration: combatActions.ConditionApplication{Ref: *refs.Conditions.Commanded(), Parameters: blob},
		behavior:    instance,
		name:        "Commanded",
	}

	require.Equal(t, spells.CommandWordFlee, prepared.atStake("goblin").Detail)
}

// TestADeliveredConditionCarriesTheCapturedDetail: deliveredEffects copies the
// collector's detail onto the effect it builds.
func TestADeliveredConditionCarriesTheCapturedDetail(t *testing.T) {
	applied, err := deliveredEffects(*refs.Spells.Command(), []ActivationEffect{{
		Kind: EffectConditionApplied, TargetID: "goblin",
		Ref: refs.Conditions.Commanded().String(), Name: "Commanded", Detail: "grovel",
	}})

	require.NoError(t, err)
	require.Len(t, applied, 1)
	require.Equal(t, "grovel", applied[0].Detail)
}
