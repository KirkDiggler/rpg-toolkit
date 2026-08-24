package actions_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

func TestAttackProfileValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*combatActions.AttackProfile)
		message string
	}{
		{
			name:    "unknown category",
			mutate:  func(profile *combatActions.AttackProfile) { profile.Category = "siege" },
			message: "category",
		},
		{
			name:    "no delivery",
			mutate:  func(profile *combatActions.AttackProfile) { profile.Delivery = combatActions.AttackDelivery{} },
			message: "exactly one delivery",
		},
		{
			name: "two deliveries",
			mutate: func(profile *combatActions.AttackProfile) {
				profile.Delivery.Ranged = &combatActions.RangedDelivery{NormalFeet: 20}
			},
			message: "exactly one delivery",
		},
		{
			name:    "invalid melee reach",
			mutate:  func(profile *combatActions.AttackProfile) { profile.Delivery.Melee.ReachFeet = 0 },
			message: "positive reach",
		},
		{
			name: "invalid ranged normal range",
			mutate: func(profile *combatActions.AttackProfile) {
				profile.Delivery = combatActions.AttackDelivery{Ranged: &combatActions.RangedDelivery{}}
			},
			message: "positive normal range",
		},
		{
			name: "invalid ranged bracket",
			mutate: func(profile *combatActions.AttackProfile) {
				profile.Delivery = combatActions.AttackDelivery{Ranged: &combatActions.RangedDelivery{NormalFeet: 80, LongFeet: 40}}
			},
			message: "long range",
		},
		{
			name: "spell with weapon context",
			mutate: func(profile *combatActions.AttackProfile) {
				profile.Category = combatActions.AttackCategorySpell
			},
			message: "spell attack",
		},
		{
			name: "unknown ability",
			mutate: func(profile *combatActions.AttackProfile) {
				profile.Ability.Ability = "luck"
			},
			message: "ability",
		},
		{
			name: "ability without marked pool",
			mutate: func(profile *combatActions.AttackProfile) {
				profile.Damage[0].Properties = nil
			},
			message: "exactly one ability-marked damage pool",
		},
		{
			name: "marker without ability",
			mutate: func(profile *combatActions.AttackProfile) {
				profile.Ability = nil
			},
			message: "no ability-marked damage pool",
		},
		{
			name: "invalid damage",
			mutate: func(profile *combatActions.AttackProfile) {
				profile.Damage[0].Dice = "8"
			},
			message: "damage",
		},
		{
			name: "empty outcome",
			mutate: func(profile *combatActions.AttackProfile) {
				profile.Ability = nil
				profile.Damage = nil
				profile.OnHit = nil
			},
			message: "damage or an on-hit condition",
		},
		{
			name: "wrong condition namespace",
			mutate: func(profile *combatActions.AttackProfile) {
				profile.OnHit = []combatActions.ConditionApplication{{Ref: *refs.Weapons.Club()}}
			},
			message: "condition ref",
		},
		{
			name: "condition save for half",
			mutate: func(profile *combatActions.AttackProfile) {
				profile.OnHit = []combatActions.ConditionApplication{{
					Ref: *refs.Conditions.Prone(),
					Save: &saves.SaveGate{
						Abilities: []abilities.Ability{abilities.STR},
						DC:        saves.DCStatic(11),
						OnSuccess: saves.Half,
					},
				}}
			},
			message: "condition save must negate",
		},
		{
			name: "invalid condition save",
			mutate: func(profile *combatActions.AttackProfile) {
				profile.OnHit = []combatActions.ConditionApplication{{
					Ref:  *refs.Conditions.Prone(),
					Save: saves.NewSaveGate(abilities.STR, 0),
				}}
			},
			message: "save",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			profile := validAttackProfile()
			tc.mutate(&profile)

			err := profile.Validate()

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.message)
		})
	}
}

func TestAttackProfileValidationAllowsConditionOnlyAttack(t *testing.T) {
	profile := validAttackProfile()
	profile.Ability = nil
	profile.Damage = nil
	profile.OnHit = []combatActions.ConditionApplication{{Ref: *refs.Conditions.Restrained()}}

	require.NoError(t, profile.Validate())
}

func TestAttackProfileValidationAllowsPrecomputedAttackWithoutEvidence(t *testing.T) {
	profile := validAttackProfile()
	profile.Ability = nil
	profile.Weapon = nil
	profile.Damage[0].Properties = nil

	require.NoError(t, profile.Validate())
}

func TestAttackDeliveryHelpers(t *testing.T) {
	tests := []struct {
		name        string
		delivery    combatActions.AttackDelivery
		isMelee     bool
		normalRange int
		maxRange    int
	}{
		{
			name:        "melee",
			delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 10}},
			isMelee:     true,
			normalRange: 10,
			maxRange:    10,
		},
		{
			name:        "ranged without long bracket",
			delivery:    combatActions.AttackDelivery{Ranged: &combatActions.RangedDelivery{NormalFeet: 30}},
			normalRange: 30,
			maxRange:    30,
		},
		{
			name:        "ranged with long bracket",
			delivery:    combatActions.AttackDelivery{Ranged: &combatActions.RangedDelivery{NormalFeet: 80, LongFeet: 320}},
			normalRange: 80,
			maxRange:    320,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.isMelee, tc.delivery.IsMelee())
			assert.Equal(t, tc.normalRange, tc.delivery.NormalRangeFeet())
			assert.Equal(t, tc.maxRange, tc.delivery.MaxRangeFeet())
		})
	}
}

func TestConditionApplicationValidationRequiresDND5EConditionRef(t *testing.T) {
	tests := []struct {
		name string
		ref  core.Ref
	}{
		{name: "empty", ref: core.Ref{}},
		{name: "other module", ref: core.Ref{Module: "expansion", Type: refs.TypeConditions, ID: "rooted"}},
		{name: "other type", ref: *refs.Weapons.Club()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := (combatActions.ConditionApplication{Ref: tc.ref}).Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "condition ref")
		})
	}
}
