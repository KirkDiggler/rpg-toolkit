// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type conditionHolder struct {
	held []dnd5eEvents.ConditionBehavior
}

func (h *conditionHolder) GetID() string                                  { return "holder-1" }
func (h *conditionHolder) GetType() core.EntityType                       { return EntityTypeFeature }
func (h *conditionHolder) GetConditions() []dnd5eEvents.ConditionBehavior { return h.held }

// mute holds no way to report its conditions.
type mute struct{}

func (mute) GetID() string            { return "mute-1" }
func (mute) GetType() core.EntityType { return EntityTypeFeature }

func ragingCondition(t *testing.T) dnd5eEvents.ConditionBehavior {
	t.Helper()
	built, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
		Ref: refs.Conditions.Raging().String(), Config: json.RawMessage(`{"damage_bonus":{"fixed":2}}`), MemberID: "holder-1",
		Origin: &dnd5eEvents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: "holder-1"},
	})
	require.NoError(t, err)
	return built.Condition
}

func rageDescription(t *testing.T) *actions.Definition {
	t.Helper()
	d, err := describeRageFor(t, `{"damage_bonus": 2}`, map[coreResources.ResourceKey]int{"ring:charges": 1})
	require.NoError(t, err)
	return d
}

func describeFor(t *testing.T, ref core.Ref, params string, pools map[coreResources.ResourceKey]int) *actions.Definition {
	t.Helper()
	d, err := DescribeFromRef(&DescribeFromRefInput{Ref: ref, Parameters: json.RawMessage(params), Giver: rageItemGiver(t), Pools: pools})
	require.NoError(t, err)
	return d
}

// Test 1: which status. Fails before the change (no such method); not a pin.
func TestStatusOnUserNamesTheStatusADescriptionPutsOnItsUser(t *testing.T) {
	rage := rageDescription(t)
	ref, ok := rage.StatusOnUser()
	assert.True(t, ok)
	assert.Equal(t, *refs.Conditions.Raging(), ref)

	for name, d := range map[string]*actions.Definition{
		"second wind":  describeFor(t, *refs.Features.SecondWind(), `{"level":1}`, nil),
		"action surge": describeFor(t, *refs.Features.ActionSurge(), `{}`, map[coreResources.ResourceKey]int{"ring:charges": 1}),
		"an attack": {Ref: core.Ref{Module: "dnd5e", Type: "actions", ID: "strike"}, Name: "Strike",
			Attack: &actions.AttackProfile{}},
		"a status put on another": {Effect: &actions.EffectProfile{To: actions.EffectTarget("another"),
			Condition: &actions.ConditionApplication{Ref: *refs.Conditions.Raging()}}},
	} {
		_, ok := d.StatusOnUser()
		assert.False(t, ok, name)
	}
}

// Test 2: the question, live.
func TestRefuseHeldStatusAsksALiveHolder(t *testing.T) {
	rage := rageDescription(t)
	heal := describeFor(t, *refs.Features.SecondWind(), `{"level":1}`, nil)
	raging := &conditionHolder{held: []dnd5eEvents.ConditionBehavior{ragingCondition(t)}}

	assert.NoError(t, RefuseHeldStatus(&RefuseHeldStatusInput{Definition: rage, Holder: &conditionHolder{}}))

	err := RefuseHeldStatus(&RefuseHeldStatusInput{Definition: rage, Holder: raging})
	require.Error(t, err)
	assert.Equal(t, "already raging", err.Error())
	assert.Equal(t, rpgerr.CodeConflictingState, rpgerr.GetCode(err))

	assert.NoError(t, RefuseHeldStatus(&RefuseHeldStatusInput{Definition: heal, Holder: raging}),
		"a description that puts no status on its user is not refused while she rages")

	assert.Error(t, RefuseHeldStatus(&RefuseHeldStatusInput{Definition: rage, Holder: mute{}}),
		"a holder that cannot report its conditions is refused")
}

// Test 3: the question, stored.
func TestRefuseHeldStatusStoredAsksAStoredSheet(t *testing.T) {
	rage := rageDescription(t)
	blob, err := ragingCondition(t).ToJSON()
	require.NoError(t, err)

	err = RefuseHeldStatusStored(&RefuseHeldStatusStoredInput{Definition: rage, Stored: []json.RawMessage{blob}})
	require.Error(t, err)
	assert.Equal(t, "already raging", err.Error())
	assert.Equal(t, rpgerr.CodeConflictingState, rpgerr.GetCode(err))

	assert.NoError(t, RefuseHeldStatusStored(&RefuseHeldStatusStoredInput{Definition: rage}))

	err = RefuseHeldStatusStored(&RefuseHeldStatusStoredInput{Definition: rage, Stored: []json.RawMessage{json.RawMessage(`not json`)}})
	require.Error(t, err, "a list HoldsRef cannot read is an error, not nil")
	assert.NotEqual(t, rpgerr.CodeConflictingState, rpgerr.GetCode(err))
}

// A status with no display reads by its ref.
func TestAHeldStatusWithNoDisplayReadsByItsRef(t *testing.T) {
	ref := core.Ref{Module: "dnd5e", Type: "conditions", ID: "no_such_status"}
	assert.Equal(t, "already holding dnd5e:conditions:no_such_status", heldStatusReason(ref))
}
