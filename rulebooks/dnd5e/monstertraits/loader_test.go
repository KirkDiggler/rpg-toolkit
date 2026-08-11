// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage/affinity"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// TestAllTraitRefs_NoPhantomEntries is the gate-recommended tripwire for
// AllTraitRefs (rpg-toolkit#778 PR #779 review): builds a real JSON blob
// for every ref AllTraitRefs claims to know about and confirms LoadJSON
// actually recognizes it (no error). This catches a "phantom entry" —
// AllTraitRefs listing a ref LoadJSON's dispatch switch doesn't really
// have a case for.
//
// This does NOT catch the other drift direction: a NEW case added to
// LoadJSON's switch that AllTraitRefs forgets to list (a "missing entry").
// That direction can't be tested by iterating AllTraitRefs's own output —
// there's nothing to iterate that would reveal an entry AllTraitRefs
// doesn't know about. Closing that direction needs LoadJSON restructured
// around an enumerable registry (a real refactor, judged out of scope for
// #778); tracked as rpg-toolkit#780.
func TestAllTraitRefs_NoPhantomEntries(t *testing.T) {
	roller := &mockRoller{nextRoll: 15}

	blobs := map[string]json.RawMessage{}

	for _, ref := range []*core.Ref{
		refs.DamageAffinities.Resistance(),
		refs.DamageAffinities.Vulnerability(),
		refs.DamageAffinities.Immunity(),
	} {
		affinityJSON, err := json.Marshal(affinity.Data{
			Ref: ref, OwnerID: "test-monster", DamageType: damage.Slashing,
		})
		require.NoError(t, err)
		blobs[ref.String()] = affinityJSON
	}

	immunityJSON, err := json.Marshal(ImmunityData{
		Ref: refs.MonsterTraits.Immunity(), OwnerID: "test-monster", DamageType: damage.Slashing,
	})
	require.NoError(t, err)
	blobs[refs.MonsterTraits.Immunity().String()] = immunityJSON

	vulnerabilityJSON, err := json.Marshal(VulnerabilityData{
		Ref: refs.MonsterTraits.Vulnerability(), OwnerID: "test-monster", DamageType: damage.Slashing,
	})
	require.NoError(t, err)
	blobs[refs.MonsterTraits.Vulnerability().String()] = vulnerabilityJSON

	conditionImmunityJSON, err := json.Marshal(ConditionImmunityData{
		Ref: refs.MonsterTraits.ConditionImmunity(), OwnerID: "test-monster",
		Conditions: []dnd5eEvents.ConditionType{dnd5eEvents.ConditionBlinded},
	})
	require.NoError(t, err)
	blobs[refs.MonsterTraits.ConditionImmunity().String()] = conditionImmunityJSON

	packTacticsJSON, err := json.Marshal(PackTacticsData{
		Ref: refs.MonsterTraits.PackTactics(), OwnerID: "test-monster",
	})
	require.NoError(t, err)
	blobs[refs.MonsterTraits.PackTactics().String()] = packTacticsJSON

	undeadFortitudeJSON, err := json.Marshal(UndeadFortitudeData{
		Ref: refs.MonsterTraits.UndeadFortitude(), OwnerID: "test-monster", ConModifier: 3,
	})
	require.NoError(t, err)
	blobs[refs.MonsterTraits.UndeadFortitude().String()] = undeadFortitudeJSON

	all := AllTraitRefs()
	require.Len(t, all, len(blobs),
		"this test and AllTraitRefs must cover the same 4 known trait refs — "+
			"update both together if a trait is added")

	for _, ref := range all {
		blob, ok := blobs[ref]
		require.True(t, ok, "AllTraitRefs claims %q but this test has no fixture for it — "+
			"add one so the phantom-entry check actually covers it", ref)

		condition, err := LoadJSON(blob, roller)
		require.NoError(t, err, "AllTraitRefs claims %q but LoadJSON rejected it — phantom entry", ref)
		require.NotNil(t, condition)
	}
}
