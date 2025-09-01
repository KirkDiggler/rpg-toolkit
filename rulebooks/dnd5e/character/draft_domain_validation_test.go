package character

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRaceChoice_Validation(t *testing.T) {
	t.Run("Elf Requires Subrace", func(t *testing.T) {
		rc := RaceChoice{
			RaceID: races.Elf,
			// No subrace selected
		}

		assert.True(t, rc.MissingRequiredSubrace(), "Elf should require subrace")
		assert.Error(t, rc.IsValid(), "Elf without subrace should be invalid")
	})

	t.Run("Human Does Not Require Subrace", func(t *testing.T) {
		rc := RaceChoice{
			RaceID: races.Human,
			// No subrace - this is fine
		}

		assert.False(t, rc.MissingRequiredSubrace(), "Human should not require subrace")
		assert.NoError(t, rc.IsValid(), "Human without subrace should be valid")
	})

	t.Run("High Elf Is Valid", func(t *testing.T) {
		rc := RaceChoice{
			RaceID:    races.Elf,
			SubraceID: races.HighElf,
		}

		assert.False(t, rc.MissingRequiredSubrace(), "High Elf should be complete")
		assert.NoError(t, rc.IsValid(), "High Elf should be valid")
	})

	t.Run("Draft Validates Missing Subrace", func(t *testing.T) {
		draft := &Draft{
			ID:   "test-draft",
			Name: "Test Elf",
			ClassChoice: ClassChoice{
				ClassID: classes.Fighter, // Doesn't require subclass at level 1
			},
			RaceChoice: RaceChoice{
				RaceID: races.Elf,
				// Missing subrace!
			},
			BackgroundChoice: backgrounds.Soldier,
		}

		result, err := draft.ValidateChoices()
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should have error about missing subrace
		assert.False(t, result.CanFinalize)
		assert.NotEmpty(t, draft.ValidationErrors)

		foundSubraceError := false
		for _, errMsg := range draft.ValidationErrors {
			if contains(errMsg, "requires a subrace") {
				foundSubraceError = true
				break
			}
		}
		assert.True(t, foundSubraceError, "Should have error about missing subrace")
	})
}

func TestClassChoice_Validation(t *testing.T) {
	t.Run("RequiresSubclassAtLevel", func(t *testing.T) {
		tests := []struct {
			name     string
			classID  classes.Class
			level    int
			expected bool
		}{
			{"Cleric at level 1", classes.Cleric, 1, true},
			{"Fighter at level 1", classes.Fighter, 1, false},
			{"Fighter at level 3", classes.Fighter, 3, true},
			{"Rogue at level 1", classes.Rogue, 1, false},
			{"Rogue at level 3", classes.Rogue, 3, true},
			{"Wizard at level 2", classes.Wizard, 2, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cc := ClassChoice{ClassID: tt.classID}
				assert.Equal(t, tt.expected, cc.RequiresSubclassAtLevel(tt.level))
			})
		}
	})

	t.Run("GetProficiencies With Subclass", func(t *testing.T) {
		cc := ClassChoice{
			ClassID:    classes.Cleric,
			SubclassID: classes.LifeDomain,
		}

		profs, err := cc.GetProficiencies()
		require.NoError(t, err)

		// Life Domain should have heavy armor
		hasHeavyArmor := false
		for _, a := range profs.Armor {
			if a == "heavy" {
				hasHeavyArmor = true
				break
			}
		}
		assert.True(t, hasHeavyArmor, "Life Domain should have heavy armor proficiency")

		// Should have saving throws
		assert.Contains(t, profs.SavingThrows, abilities.WIS)
		assert.Contains(t, profs.SavingThrows, abilities.CHA)
	})

	t.Run("GetProficiencies Without Subclass", func(t *testing.T) {
		cc := ClassChoice{
			ClassID: classes.Fighter,
			// No subclass yet
		}

		profs, err := cc.GetProficiencies()
		require.NoError(t, err)

		// Fighter should have all armor types
		assert.Contains(t, profs.Armor, proficiencies.ArmorLight)
		assert.Contains(t, profs.Armor, proficiencies.ArmorMedium)
		assert.Contains(t, profs.Armor, proficiencies.ArmorHeavy)
		assert.Contains(t, profs.Armor, proficiencies.ArmorShields)

		// Should have martial weapons
		assert.Contains(t, profs.Weapons, proficiencies.WeaponMartial)

		// Should have STR and CON saves
		assert.Contains(t, profs.SavingThrows, abilities.STR)
		assert.Contains(t, profs.SavingThrows, abilities.CON)
	})
}
