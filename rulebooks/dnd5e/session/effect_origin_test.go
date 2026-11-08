// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func TestEffectProjectionRetainsSourceEntityWithoutChangingRowIdentity(t *testing.T) {
	input := contributions.Effect{ID: "opaque-output", Source: contributions.Source{Ref: refs.Conditions.Raging(), Name: "Raging", SourceID: "ring-17"}, Description: "Damage bonus", State: contributions.StateApplies, Reason: "matching action", Participation: contributions.ContributesNow, Benefit: "+3 damage"}
	row, err := effectRowOf(input)
	require.NoError(t, err)
	require.Equal(t, "ring-17", row.SourceID)
	require.Equal(t, "opaque-output", row.ID, "origin is a value, not encoded into an opaque identifier")
	require.Equal(t, "dnd5e:conditions:raging", row.Ref)
}
