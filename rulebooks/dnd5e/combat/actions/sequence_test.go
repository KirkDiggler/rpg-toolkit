// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// scimitarStrike is a component an actor carries: an ordinary attack
// definition, indistinguishable from one nothing scripts.
func scimitarStrike() combatActions.Definition {
	profile := validAttackProfile()
	return combatActions.Definition{
		Ref:    *refs.Weapons.Scimitar(),
		Name:   "Scimitar",
		Attack: &profile,
	}
}

func javelinStrike() combatActions.Definition {
	profile := validAttackProfile()
	return combatActions.Definition{
		Ref:    *refs.Weapons.Javelin(),
		Name:   "Javelin",
		Attack: &profile,
	}
}

func twoScimitars() combatActions.Definition {
	return combatActions.Definition{
		Ref:  *refs.MonsterActions.GoblinBossMultiattack(),
		Name: "Multiattack",
		Sequence: &combatActions.SequenceProfile{
			Steps: []combatActions.SequenceStep{
				{Action: *refs.Weapons.Scimitar()},
				{Action: *refs.Weapons.Scimitar(), Disadvantage: "second attack of a multiattack"},
			},
		},
	}
}

func TestSequenceIsAThirdArmAndOnlyOne(t *testing.T) {
	sequence := twoScimitars()
	require.NoError(t, sequence.Validate())

	attackToo := twoScimitars()
	profile := validAttackProfile()
	attackToo.Attack = &profile
	require.ErrorContains(t, attackToo.Validate(), "exactly one profile",
		"a definition carrying both a sequence and an attack names two machines")
}

func TestSequenceValidationRefusals(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*combatActions.Definition)
		message string
	}{
		{
			name: "a sequence of one is the component action itself",
			mutate: func(d *combatActions.Definition) {
				d.Sequence.Steps = d.Sequence.Steps[:1]
			},
			message: "at least 2 steps",
		},
		{
			name: "a sequence with no steps declares nothing to run",
			mutate: func(d *combatActions.Definition) {
				d.Sequence.Steps = nil
			},
			message: "at least 2 steps",
		},
		{
			name: "a step naming nothing cannot be resolved later",
			mutate: func(d *combatActions.Definition) {
				d.Sequence.Steps[1].Action = core.Ref{}
			},
			message: "step 1 action ref is invalid",
		},
		{
			name: "disadvantage with a blank reason is an anonymous die",
			mutate: func(d *combatActions.Definition) {
				d.Sequence.Steps[1].Disadvantage = "   "
			},
			message: "step 1 imposes disadvantage with a blank reason",
		},
		{
			name: "a step naming the sequence itself never terminates",
			mutate: func(d *combatActions.Definition) {
				d.Sequence.Steps[0].Action = *refs.MonsterActions.GoblinBossMultiattack()
			},
			message: "step 0 names the sequence itself",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			definition := twoScimitars()
			tc.mutate(&definition)
			require.ErrorContains(t, definition.Validate(), tc.message)
		})
	}
}

func TestSequenceSurvivesJSONAndDoesNotAliasOnClone(t *testing.T) {
	original := twoScimitars()

	raw, err := json.Marshal(original)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"sequence"`)
	assert.Contains(t, string(raw), `"second attack of a multiattack"`)

	var decoded combatActions.Definition
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.NoError(t, decoded.Validate())
	assert.Equal(t, original, decoded)

	clone := original.Clone()
	clone.Sequence.Steps[1].Disadvantage = "rewritten"
	assert.Equal(t, "second attack of a multiattack", original.Sequence.Steps[1].Disadvantage,
		"a clone that shared its step list would let a caller rewrite a running script")
}

func TestSequenceStepWithNoDisadvantageOmitsIt(t *testing.T) {
	raw, err := json.Marshal(combatActions.SequenceStep{Action: *refs.Weapons.Scimitar()})
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "disadvantage",
		"the zero step is a plain swing, and the wire should say so by silence")
}

func TestResolveSequenceMatchesStepsToComponentsInOrder(t *testing.T) {
	sequence := twoScimitars()
	repertoire := []combatActions.Definition{sequence, scimitarStrike(), javelinStrike()}

	components, err := combatActions.ResolveSequence(sequence, repertoire)
	require.NoError(t, err)
	require.Len(t, components, len(sequence.Sequence.Steps))

	assert.Equal(t, *refs.Weapons.Scimitar(), components[0].Definition.Ref)
	assert.Empty(t, components[0].Step.Disadvantage, "the first swing is unpenalised")
	assert.Equal(t, *refs.Weapons.Scimitar(), components[1].Definition.Ref)
	assert.Equal(t, "second attack of a multiattack", components[1].Step.Disadvantage,
		"the second swing carries the reason the die will be kept low under")
}

func TestResolveSequenceClonesTheComponentsItReturns(t *testing.T) {
	sequence := twoScimitars()
	scimitar := scimitarStrike()
	repertoire := []combatActions.Definition{sequence, scimitar}

	components, err := combatActions.ResolveSequence(sequence, repertoire)
	require.NoError(t, err)

	components[0].Definition.Attack.AttackBonus = 99
	assert.Equal(t, validAttackProfile().AttackBonus, scimitar.Attack.AttackBonus,
		"a resolved script must not alias the repertoire it was read off")
}

func TestResolveSequenceRefusesAStepTheActorDoesNotCarry(t *testing.T) {
	sequence := twoScimitars()

	_, err := combatActions.ResolveSequence(sequence, []combatActions.Definition{sequence, javelinStrike()})
	require.ErrorContains(t, err, "which the actor does not carry",
		"the multiattack this replaces skipped a sub-action it could not find")
	require.ErrorContains(t, err, refs.Weapons.Scimitar().String())
}

func TestResolveSequenceRefusesAStepThatIsItselfASequence(t *testing.T) {
	inner := combatActions.Definition{
		Ref:  *refs.MonsterActions.ThugMultiattack(),
		Name: "Inner Multiattack",
		Sequence: &combatActions.SequenceProfile{
			Steps: []combatActions.SequenceStep{
				{Action: *refs.Weapons.Scimitar()},
				{Action: *refs.Weapons.Scimitar()},
			},
		},
	}
	outer := combatActions.Definition{
		Ref:  *refs.MonsterActions.GoblinBossMultiattack(),
		Name: "Outer Multiattack",
		Sequence: &combatActions.SequenceProfile{
			Steps: []combatActions.SequenceStep{
				{Action: *refs.MonsterActions.ThugMultiattack()},
				{Action: *refs.Weapons.Scimitar()},
			},
		},
	}

	_, err := combatActions.ResolveSequence(outer, []combatActions.Definition{outer, inner, scimitarStrike()})
	require.ErrorContains(t, err, "which is itself a sequence")
}

func TestResolveSequenceRefusesADefinitionWithNoSequence(t *testing.T) {
	_, err := combatActions.ResolveSequence(scimitarStrike(), []combatActions.Definition{scimitarStrike()})
	require.ErrorContains(t, err, "declares no sequence")
}
