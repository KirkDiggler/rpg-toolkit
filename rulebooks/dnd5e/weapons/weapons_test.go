package weapons_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWeaponLookup(t *testing.T) {
	tests := []struct {
		name     string
		weaponID string
		want     weapons.Weapon
		wantOK   bool
	}{
		{
			name:     "find longsword",
			weaponID: "longsword",
			want: weapons.Weapon{
				ID:         "longsword",
				Name:       "Longsword",
				Category:   weapons.CategoryMartialMelee,
				Cost:       "15 gp",
				Damage:     "1d8",
				DamageType: "slashing",
				Weight:     3,
				Properties: []weapons.WeaponProperty{weapons.PropertyVersatile},
			},
			wantOK: true,
		},
		{
			name:     "find dagger",
			weaponID: "dagger",
			want: weapons.Weapon{
				ID:         "dagger",
				Name:       "Dagger",
				Category:   weapons.CategorySimpleMelee,
				Cost:       "2 gp",
				Damage:     "1d4",
				DamageType: "piercing",
				Weight:     1,
				Properties: []weapons.WeaponProperty{weapons.PropertyFinesse, weapons.PropertyLight, weapons.PropertyThrown},
				Range:      &weapons.Range{Normal: 20, Long: 60},
			},
			wantOK: true,
		},
		{
			name:     "weapon not found",
			weaponID: "lightsaber",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := weapons.GetByID(tt.weaponID)
			if tt.wantOK {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestWeaponCategories(t *testing.T) {
	t.Run("get simple melee weapons", func(t *testing.T) {
		simple := weapons.GetByCategory(weapons.CategorySimpleMelee)
		require.NotEmpty(t, simple)

		// Check we have the expected weapons
		ids := make(map[string]bool)
		for _, w := range simple {
			ids[w.ID] = true
			assert.True(t, w.IsSimple())
			assert.True(t, w.IsMelee())
		}

		assert.True(t, ids["club"])
		assert.True(t, ids["dagger"])
		assert.True(t, ids["handaxe"])
		assert.True(t, ids["javelin"])
	})

	t.Run("get martial melee weapons", func(t *testing.T) {
		martial := weapons.GetByCategory(weapons.CategoryMartialMelee)
		require.NotEmpty(t, martial)

		ids := make(map[string]bool)
		for _, w := range martial {
			ids[w.ID] = true
			assert.True(t, w.IsMartial())
			assert.True(t, w.IsMelee())
		}

		assert.True(t, ids["longsword"])
		assert.True(t, ids["greatsword"])
		assert.True(t, ids["rapier"])
		assert.True(t, ids["shortsword"])
	})

	t.Run("get all simple weapons", func(t *testing.T) {
		simple := weapons.GetSimpleWeapons()
		require.NotEmpty(t, simple)

		for _, w := range simple {
			assert.True(t, w.IsSimple())
		}
	})

	t.Run("get all martial weapons", func(t *testing.T) {
		martial := weapons.GetMartialWeapons()
		require.NotEmpty(t, martial)

		for _, w := range martial {
			assert.True(t, w.IsMartial())
		}
	})
}

func TestWeaponProperties(t *testing.T) {
	t.Run("check weapon properties", func(t *testing.T) {
		dagger, err := weapons.GetByID("dagger")
		require.NoError(t, err)

		assert.True(t, dagger.HasProperty(weapons.PropertyFinesse))
		assert.True(t, dagger.HasProperty(weapons.PropertyLight))
		assert.True(t, dagger.HasProperty(weapons.PropertyThrown))
		assert.False(t, dagger.HasProperty(weapons.PropertyHeavy))
		assert.False(t, dagger.HasProperty(weapons.PropertyTwoHanded))
	})

	t.Run("greatsword is heavy and two-handed", func(t *testing.T) {
		greatsword, err := weapons.GetByID("greatsword")
		require.NoError(t, err)

		assert.True(t, greatsword.HasProperty(weapons.PropertyHeavy))
		assert.True(t, greatsword.HasProperty(weapons.PropertyTwoHanded))
		assert.False(t, greatsword.HasProperty(weapons.PropertyLight))
		assert.False(t, greatsword.HasProperty(weapons.PropertyFinesse))
	})
}

func TestWeaponRanges(t *testing.T) {
	t.Run("melee weapon with thrown property", func(t *testing.T) {
		dagger, err := weapons.GetByID("dagger")
		require.NoError(t, err)

		require.NotNil(t, dagger.Range)
		assert.Equal(t, 20, dagger.Range.Normal)
		assert.Equal(t, 60, dagger.Range.Long)
	})

	t.Run("ranged weapon", func(t *testing.T) {
		longbow, err := weapons.GetByID("longbow")
		require.NoError(t, err)

		require.NotNil(t, longbow.Range)
		assert.Equal(t, 150, longbow.Range.Normal)
		assert.Equal(t, 600, longbow.Range.Long)
	})

	t.Run("pure melee weapon has no range", func(t *testing.T) {
		greatsword, err := weapons.GetByID("greatsword")
		require.NoError(t, err)

		assert.Nil(t, greatsword.Range)
	})
}
