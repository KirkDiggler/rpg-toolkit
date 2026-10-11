// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func refsOf(t *testing.T, stored []json.RawMessage) []string {
	t.Helper()
	out := make([]string, 0, len(stored))
	for _, blob := range stored {
		var named struct {
			Ref string `json:"ref"`
		}
		require.NoError(t, json.Unmarshal(blob, &named))
		out = append(out, named.Ref)
	}
	return out
}

// TestAMintedMonsterStoresItsFreeReactions: the monster mint writes the
// reactions every combatant has into the new sheet's stored conditions, the
// way the character draft does, because a member holds only what its stored
// record says. A mint that skipped this would make a monster that never takes
// an opportunity attack.
func TestAMintedMonsterStoresItsFreeReactions(t *testing.T) {
	skeleton, err := instantiate("skel-1", "dnd5e:monsters:skeleton", nil, nil)
	require.NoError(t, err)

	held := refsOf(t, skeleton.Conditions)
	require.Contains(t, held, refs.Conditions.OpportunityAttack().String())
	once := 0
	for _, ref := range held {
		if ref == refs.Conditions.OpportunityAttack().String() {
			once++
		}
	}
	require.Equal(t, 1, once, "named once, so the encounter never sees it held twice")
}

// TestAMintedMonsterIsSeenHoldingItsFreeReactions closes the loop through the
// reader: what the mint stored is what the conditions seam observes.
func TestAMintedMonsterIsSeenHoldingItsFreeReactions(t *testing.T) {
	skeleton, err := instantiate("skel-1", "dnd5e:monsters:skeleton", nil, nil)
	require.NoError(t, err)
	seam := equipmentBeside(standingSeam{
		ctx:   context.Background(),
		data:  &SessionData{NPCs: []monster.Data{*skeleton}},
		kinds: map[string]encounter.MemberKind{"skel-1": encounter.KindMonster},
	})

	got, err := seam.Conditions([]encounter.MemberID{"skel-1"})
	require.NoError(t, err)

	require.NotNil(t, got["skel-1"])
	require.Contains(t, got["skel-1"].Conditions,
		encounter.ConditionKey{ConditionRef: conditions.NewOpportunityAttackCondition("skel-1").Ref().String()})
}
