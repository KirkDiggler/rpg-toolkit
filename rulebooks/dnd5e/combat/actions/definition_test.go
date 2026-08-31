package actions_test

import (
	"encoding/json"
	"testing"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

func validAttackProfile() combatActions.AttackProfile {
	return combatActions.AttackProfile{
		Category: combatActions.AttackCategoryWeapon,
		Delivery: combatActions.AttackDelivery{
			Melee: &combatActions.MeleeDelivery{ReachFeet: 5},
		},
		AttackBonus: 5,
		Ability: &combatActions.AbilityContribution{
			Ability:  abilities.STR,
			Modifier: 3,
		},
		Weapon: &combatActions.WeaponContext{Ref: refs.Weapons.Longsword()},
		Damage: []damage.Damage{{
			Dice:       "1d8",
			Type:       damage.Slashing,
			Properties: []damage.Property{damage.AddsAttackAbilityModifier},
		}},
	}
}

func validDefinition() combatActions.Definition {
	profile := validAttackProfile()
	return combatActions.Definition{
		Ref:    *refs.Weapons.Longsword(),
		Name:   "Longsword",
		Cost:   &combat.SpendProfile{Capacity: map[combat.CapacityType]int{combat.CapacityAttack: 1}},
		Attack: &profile,
	}
}

func TestDefinitionRoundTrip(t *testing.T) {
	original := validDefinition()
	original.Attack.IsOffHandAttack = true
	original.Attack.OnHit = []combatActions.ConditionApplication{{
		Ref:  *refs.Conditions.Prone(),
		Save: saves.NewSaveGate(abilities.STR, 11),
	}}

	raw, err := json.Marshal(original)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"capacity"`)
	assert.NotContains(t, string(raw), `"Capacity"`)

	var decoded combatActions.Definition
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.NoError(t, decoded.Validate())
	assert.Equal(t, original, decoded)
}

func TestDefinitionRoundTripUsesExplicitSpendProfileKeys(t *testing.T) {
	original := validDefinition()
	original.Cost = &combat.SpendProfile{
		Slots:    map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Capacity: map[combat.CapacityType]int{combat.CapacityAttack: 1},
		Grants:   map[combat.CapacityType]int{combat.CapacityMovement: 5},
		Pools:    map[coreResources.ResourceKey]int{"ki": 1},
		Requires: map[combat.CapacityType]int{combat.CapacityAttack: 1},
	}

	raw, err := json.Marshal(original)
	require.NoError(t, err)
	for _, key := range []string{"slots", "capacity", "grants", "pools", "requires"} {
		assert.Contains(t, string(raw), `"`+key+`"`)
	}

	var decoded combatActions.Definition
	require.NoError(t, json.Unmarshal(raw, &decoded))
	assert.Equal(t, original, decoded)
}

func TestDefinitionValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*combatActions.Definition)
		message string
	}{
		{
			name:    "invalid ref",
			mutate:  func(def *combatActions.Definition) { def.Ref.ID = "" },
			message: "ref",
		},
		{
			name:    "empty name",
			mutate:  func(def *combatActions.Definition) { def.Name = "" },
			message: "name",
		},
		{
			name:    "no profile",
			mutate:  func(def *combatActions.Definition) { def.Attack = nil },
			message: "exactly one profile",
		},
		{
			name: "invalid cost",
			mutate: func(def *combatActions.Definition) {
				def.Cost = &combat.SpendProfile{Capacity: map[combat.CapacityType]int{combat.CapacityNone: 1}}
			},
			message: "cost",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			def := validDefinition()
			tc.mutate(&def)

			err := def.Validate()

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.message)
		})
	}
}

func TestDefinitionValidationAllowsNilCost(t *testing.T) {
	def := validDefinition()
	def.Cost = nil

	require.NoError(t, def.Validate())
}

func TestOffHandAttackValidation(t *testing.T) {
	t.Run("light-melee eligibility remains the producer's rule", func(t *testing.T) {
		def := validDefinition()
		def.Attack.IsOffHandAttack = true

		require.NoError(t, def.Validate())
	})

	t.Run("requires weapon context", func(t *testing.T) {
		def := validDefinition()
		def.Attack.IsOffHandAttack = true
		def.Attack.Weapon = nil

		err := def.Validate()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "weapon context")
	})

	t.Run("requires melee delivery", func(t *testing.T) {
		def := validDefinition()
		def.Attack.IsOffHandAttack = true
		def.Attack.Delivery = combatActions.AttackDelivery{
			Ranged: &combatActions.RangedDelivery{NormalFeet: 30, LongFeet: 120},
		}

		err := def.Validate()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "melee")
	})
}

func TestDefinitionCloneDoesNotAliasNestedData(t *testing.T) {
	original := validDefinition()
	original.Attack.IsOffHandAttack = true
	original.Cost = &combat.SpendProfile{
		Slots:    map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Capacity: map[combat.CapacityType]int{combat.CapacityAttack: 1},
		Grants:   map[combat.CapacityType]int{combat.CapacityMovement: 5},
		Pools:    map[coreResources.ResourceKey]int{"ki": 1},
		Requires: map[combat.CapacityType]int{combat.CapacityAttack: 1},
	}
	original.Attack.Weapon.OffHandWeaponRef = refs.Weapons.Dagger()
	original.Attack.OnHit = []combatActions.ConditionApplication{{
		Ref:        *refs.Conditions.Prone(),
		Parameters: json.RawMessage(`{"duration":1}`),
		Save:       saves.NewSaveGate(abilities.STR, 11),
	}}

	clone := original.Clone()
	clone.Attack.IsOffHandAttack = false
	clone.Cost.Slots[coreCombat.ActionStandard] = 2
	clone.Cost.Capacity[combat.CapacityAttack] = 2
	clone.Cost.Grants[combat.CapacityMovement] = 10
	clone.Cost.Pools["ki"] = 2
	clone.Cost.Requires[combat.CapacityAttack] = 2
	clone.Attack.Ability.Ability = abilities.DEX
	clone.Attack.Weapon.Ref.ID = "rapier"
	clone.Attack.Weapon.OffHandWeaponRef.ID = "club"
	clone.Attack.Damage[0].Properties[0] = damage.DoesNotCrit
	clone.Attack.OnHit[0].Ref.ID = "stunned"
	clone.Attack.OnHit[0].Parameters[0] = '['
	clone.Attack.OnHit[0].Save.Abilities[0] = abilities.DEX

	assert.True(t, original.Attack.IsOffHandAttack)
	assert.Equal(t, 1, original.Cost.Slots[coreCombat.ActionStandard])
	assert.Equal(t, 1, original.Cost.Capacity[combat.CapacityAttack])
	assert.Equal(t, 5, original.Cost.Grants[combat.CapacityMovement])
	assert.Equal(t, 1, original.Cost.Pools["ki"])
	assert.Equal(t, 1, original.Cost.Requires[combat.CapacityAttack])
	assert.Equal(t, abilities.STR, original.Attack.Ability.Ability)
	assert.Equal(t, "longsword", original.Attack.Weapon.Ref.ID)
	assert.Equal(t, "dagger", original.Attack.Weapon.OffHandWeaponRef.ID)
	assert.Equal(t, damage.AddsAttackAbilityModifier, original.Attack.Damage[0].Properties[0])
	assert.Equal(t, "prone", original.Attack.OnHit[0].Ref.ID)
	assert.JSONEq(t, `{"duration":1}`, string(original.Attack.OnHit[0].Parameters))
	assert.Equal(t, abilities.STR, original.Attack.OnHit[0].Save.Abilities[0])
}
