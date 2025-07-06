package dndbot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

func TestProficiencyIntegration(t *testing.T) {
	// Create event bus
	bus := events.NewBus()

	// Create proficiency integration
	profIntegration := NewProficiencyIntegration(bus)

	t.Run("add fighter proficiencies", func(t *testing.T) {
		// Add proficiencies for a level 5 fighter
		err := profIntegration.AddCharacterProficiencies("fighter-123", 5)
		require.NoError(t, err)

		// Check proficiency bonus calculation
		assert.Equal(t, 3, GetProficiencyBonus(5), "Level 5 should have +3 proficiency bonus")

		// Check weapon proficiencies
		assert.True(t, profIntegration.CheckProficiency("fighter-123", "longsword"))
		assert.True(t, profIntegration.CheckProficiency("fighter-123", "simple-weapons"))
		assert.True(t, profIntegration.CheckProficiency("fighter-123", "martial-weapons"))

		// Check weapon categories
		assert.True(t, profIntegration.CheckProficiency("fighter-123", "dagger"), "Should be proficient with simple weapons")
		assert.True(t, profIntegration.CheckProficiency("fighter-123", "greatsword"),
			"Should be proficient with martial weapons")

		// Check armor proficiencies
		assert.True(t, profIntegration.CheckProficiency("fighter-123", "heavy-armor"))
		assert.True(t, profIntegration.CheckProficiency("fighter-123", "shields"))

		// Check skill proficiencies
		assert.True(t, profIntegration.CheckProficiency("fighter-123", "athletics"))
		assert.True(t, profIntegration.CheckProficiency("fighter-123", "intimidation"))
		assert.False(t, profIntegration.CheckProficiency("fighter-123", "arcana"), "Should not be proficient in arcana")

		// Check saving throw proficiencies
		assert.True(t, profIntegration.CheckProficiency("fighter-123", "strength-save"))
		assert.True(t, profIntegration.CheckProficiency("fighter-123", "constitution-save"))
		assert.False(t, profIntegration.CheckProficiency("fighter-123", "wisdom-save"))
	})

	t.Run("proficiency bonus by level", func(t *testing.T) {
		testCases := []struct {
			level int
			bonus int
		}{
			{1, 2}, // Levels 1-4: +2
			{4, 2},
			{5, 3}, // Levels 5-8: +3
			{8, 3},
			{9, 4}, // Levels 9-12: +4
			{12, 4},
			{13, 5}, // Levels 13-16: +5
			{16, 5},
			{17, 6}, // Levels 17-20: +6
			{20, 6},
		}

		for _, tc := range testCases {
			assert.Equal(t, tc.bonus, GetProficiencyBonus(tc.level),
				"Level %d should have +%d bonus", tc.level, tc.bonus)
		}
	})

	t.Run("weapon category helpers", func(t *testing.T) {
		// Simple weapons
		assert.True(t, isSimpleWeapon("dagger"))
		assert.True(t, isSimpleWeapon("club"))
		assert.False(t, isSimpleWeapon("longsword"))

		// Martial weapons
		assert.True(t, isMartialWeapon("longsword"))
		assert.True(t, isMartialWeapon("greatsword"))
		assert.False(t, isMartialWeapon("club"))
	})
}

// Example of how to integrate with existing DND bot code
func Example_proficiencyMigration() {
	// In the DND bot, when checking if a character can use a weapon:
	/*
		// Old code:
		func (c *Character) HasWeaponProficiency(weapon string) bool {
			for _, prof := range c.Proficiencies[ProficiencyTypeWeapon] {
				if prof.Key == weapon {
					return true
				}
			}
			return false
		}

		// New code using toolkit:
		func (c *Character) HasWeaponProficiency(weapon string) bool {
			return profIntegration.CheckProficiency(c.ID, weapon)
		}

		// Or during attack calculations:
		func (c *Character) CalculateAttackBonus(weapon *Weapon) int {
			bonus := c.GetAbilityModifier(weapon.AbilityScore)

			// Add proficiency bonus if proficient
			if profIntegration.CheckProficiency(c.ID, weapon.Key) {
				bonus += GetProficiencyBonus(c.Level)
			}

			return bonus
		}
	*/
}
