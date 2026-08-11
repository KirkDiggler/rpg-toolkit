package damage_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/stretchr/testify/require"
)

func TestDamageSpecValidate(t *testing.T) {
	valid := damage.DamageSpec{Pools: []damage.Damage{{
		Dice: "1d6", Type: damage.Bludgeoning, FlatBonus: -1,
		Properties: []damage.Property{damage.PropertyCritEligible},
	}}}

	require.NoError(t, valid.Validate())
	require.Error(t, (&damage.DamageSpec{}).Validate())
	require.Error(t, (&damage.DamageSpec{Pools: []damage.Damage{{Dice: "bad", Type: damage.Acid}}}).Validate())
}

func TestDamageSpecValidateRejectsUnsupportedPoolMetadata(t *testing.T) {
	tests := []damage.Damage{
		{Dice: "1d6", Type: damage.None},
		{Dice: "1d6", Type: damage.Fire, Properties: []damage.Property{"unknown"}},
	}

	for _, pool := range tests {
		spec := damage.DamageSpec{Pools: []damage.Damage{pool}}
		require.Error(t, spec.Validate())
	}
}

func TestDamageSpecKeepsSaveMetadata(t *testing.T) {
	spec := damage.DamageSpec{Pools: []damage.Damage{{
		Dice: "8d6", Type: damage.Fire,
		Save: &damage.SaveSpec{Ability: abilities.DEX, DC: 14, Effect: damage.SaveEffectHalf},
	}}}

	require.NoError(t, spec.Validate())
}
