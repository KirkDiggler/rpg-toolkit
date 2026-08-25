// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// stubResourceReader is a minimal ResourceReader for tests: it hands back
// fixed current/maximum values for the keys it knows, and reports not-found
// (ok=false) for anything else. It never touches persistence JSON.
type stubResourceReader struct {
	values map[coreResources.ResourceKey][2]int // current, maximum
}

func (r stubResourceReader) ResourceStatus(key coreResources.ResourceKey) (int, int, bool) {
	v, ok := r.values[key]
	if !ok {
		return 0, 0, false
	}
	return v[0], v[1], true
}

func TestSecondWindStatusReportsPrivateResourceWithoutJSON(t *testing.T) {
	sw := newSecondWindForTest("second-wind-feature", 3, "fighter-1")

	out, err := sw.Status(&StatusInput{})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Status)

	assert.Equal(t, *refs.Features.SecondWind(), out.Status.Ref, "ref must be the canonical SecondWind ref")
	assert.NotEmpty(t, out.Status.Name, "name must never be empty")
	assert.Empty(t, out.Status.Detail, "detail is server-authored and may be empty")

	require.NotNil(t, out.Status.Resource, "Second Wind owns a private resource")
	assert.Equal(t, resources.SecondWind, out.Status.Resource.Key)
	assert.Equal(t, "Second Wind", out.Status.Resource.Name)
	assert.Equal(t, 1, out.Status.Resource.Current)
	assert.Equal(t, 1, out.Status.Resource.Maximum)
}

func TestActionSurgeStatusReportsPrivateResourceWithoutJSON(t *testing.T) {
	as := &ActionSurge{
		id:          refs.Features.ActionSurge().ID,
		name:        "Action Surge",
		characterID: "fighter-1",
		resource: combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          refs.Features.ActionSurge().ID,
			Maximum:     1,
			CharacterID: "fighter-1",
			ResetType:   coreResources.ResetShortRest,
		}),
	}

	out, err := as.Status(&StatusInput{})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Status)

	assert.Equal(t, *refs.Features.ActionSurge(), out.Status.Ref)
	assert.NotEmpty(t, out.Status.Name)

	require.NotNil(t, out.Status.Resource)
	assert.Equal(t, resources.ActionSurge, out.Status.Resource.Key)
	assert.Equal(t, "Action Surge", out.Status.Resource.Name)
	assert.Equal(t, 1, out.Status.Resource.Current)
	assert.Equal(t, 1, out.Status.Resource.Maximum)
}

func TestKiFeaturesReportOneSharedResourceKey(t *testing.T) {
	owner := stubResourceReader{values: map[coreResources.ResourceKey][2]int{
		resources.Ki: {3, 5},
	}}

	flurry := &FlurryOfBlows{id: "flurry", name: "Flurry of Blows", characterID: "monk-1"}
	patient := &PatientDefense{id: "patient", name: "Patient Defense", characterID: "monk-1"}
	step := &StepOfTheWind{id: "step", name: "Step of the Wind", characterID: "monk-1"}

	for _, tc := range []struct {
		name    string
		feature StatusProvider
		ref     core.Ref
	}{
		{"flurry_of_blows", flurry, *refs.Features.FlurryOfBlows()},
		{"patient_defense", patient, *refs.Features.PatientDefense()},
		{"step_of_the_wind", step, *refs.Features.StepOfTheWind()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.feature.Status(&StatusInput{Owner: owner})
			require.NoError(t, err)
			require.NotNil(t, out)
			require.NotNil(t, out.Status)

			assert.Equal(t, tc.ref, out.Status.Ref)
			assert.NotEmpty(t, out.Status.Name)

			require.NotNil(t, out.Status.Resource, "Ki features report the shared Ki pool")
			assert.Equal(t, resources.Ki, out.Status.Resource.Key)
			assert.Equal(t, "Ki", out.Status.Resource.Name)
			assert.Equal(t, 3, out.Status.Resource.Current)
			assert.Equal(t, 5, out.Status.Resource.Maximum)
		})
	}
}

func TestKiFeatureStatusErrorsWithoutOwner(t *testing.T) {
	flurry := &FlurryOfBlows{id: "flurry", name: "Flurry of Blows", characterID: "monk-1"}

	_, err := flurry.Status(&StatusInput{})
	require.Error(t, err)
}

func TestRageStatusReportsOwnerOwnedRageCharges(t *testing.T) {
	owner := stubResourceReader{values: map[coreResources.ResourceKey][2]int{
		resources.RageCharges: {2, 4},
	}}

	rage := &Rage{id: "rage", name: "Rage", level: 5}

	out, err := rage.Status(&StatusInput{Owner: owner})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Status)

	assert.Equal(t, *refs.Features.Rage(), out.Status.Ref)
	assert.NotEmpty(t, out.Status.Name)

	require.NotNil(t, out.Status.Resource)
	assert.Equal(t, resources.RageCharges, out.Status.Resource.Key)
	assert.Equal(t, 2, out.Status.Resource.Current)
	assert.Equal(t, 4, out.Status.Resource.Maximum)
}

func TestResourcelessFeaturesReportNoResource(t *testing.T) {
	reckless := &RecklessAttack{id: "reckless", name: "Reckless Attack", characterID: "barbarian-1"}
	deflect := &DeflectMissiles{id: "deflect", name: "Deflect Missiles", characterID: "monk-1"}

	for _, tc := range []struct {
		name    string
		feature StatusProvider
		ref     core.Ref
	}{
		{"reckless_attack", reckless, *refs.Features.RecklessAttack()},
		{"deflect_missiles", deflect, *refs.Features.DeflectMissiles()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.feature.Status(&StatusInput{})
			require.NoError(t, err)
			require.NotNil(t, out)
			require.NotNil(t, out.Status)

			assert.Equal(t, tc.ref, out.Status.Ref)
			assert.NotEmpty(t, out.Status.Name)
			assert.Nil(t, out.Status.Resource, "this feature owns no resource")
		})
	}
}
