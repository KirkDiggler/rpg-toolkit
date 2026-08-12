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

func TestDamageSpecValidateRejectsDiceModifiers(t *testing.T) {
	for _, notation := range []string{"1d6+2", "1d6-1"} {
		spec := damage.DamageSpec{Pools: []damage.Damage{{Dice: notation, Type: damage.Bludgeoning}}}
		require.Error(t, spec.Validate(), notation)
	}
}

func TestDamageSpecValidateUsesTermsWhenPresent(t *testing.T) {
	spec := damage.DamageSpec{Pools: []damage.Damage{{
		Dice:  "not legacy dice",
		Terms: []damage.DiceTerm{{Dice: "1d6", Sign: 1}, {Dice: "1d4", Sign: -1}},
		Type:  damage.Fire,
	}}}

	require.NoError(t, spec.Validate())
}

func TestDamageSpecValidateRejectsInvalidTerms(t *testing.T) {
	for _, terms := range [][]damage.DiceTerm{
		{{Dice: "1d6+1d4", Sign: 1}},
		{{Dice: "1d6", Sign: 0}},
		{{Dice: "1d6", Sign: -1}},
	} {
		spec := damage.DamageSpec{Pools: []damage.Damage{{Dice: "1d6", Terms: terms, Type: damage.Fire}}}
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
