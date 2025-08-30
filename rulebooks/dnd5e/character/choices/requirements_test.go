package choices

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

func TestGetClassRequirements(t *testing.T) {
	tests := []struct {
		name     string
		classID  classes.Class
		validate func(t *testing.T, reqs *Requirements)
	}{
		{
			name:    "Fighter Level 1",
			classID: classes.Fighter,
			validate: func(t *testing.T, reqs *Requirements) {
				require.NotNil(t, reqs)

				// Check skills
				require.NotNil(t, reqs.Skills)
				assert.Equal(t, 2, reqs.Skills.Count)
				assert.Contains(t, reqs.Skills.Options, skills.Athletics)
				assert.Contains(t, reqs.Skills.Options, skills.Intimidation)
				assert.Len(t, reqs.Skills.Options, 8)

				// Check fighting style
				require.NotNil(t, reqs.FightingStyle)
				assert.Len(t, reqs.FightingStyle.Options, 6)
				assert.Contains(t, reqs.FightingStyle.Options, FightingStyleDefense)

				// Check equipment
				assert.Len(t, reqs.Equipment, 4) // armor, weapons, ranged, pack
			},
		},
		{
			name:    "Rogue Level 1",
			classID: classes.Rogue,
			validate: func(t *testing.T, reqs *Requirements) {
				require.NotNil(t, reqs)

				// Check skills - Rogue gets 4
				require.NotNil(t, reqs.Skills)
				assert.Equal(t, 4, reqs.Skills.Count)
				assert.Contains(t, reqs.Skills.Options, skills.Stealth)
				assert.Contains(t, reqs.Skills.Options, skills.SleightOfHand)

				// Check expertise
				require.NotNil(t, reqs.Expertise)
				assert.Equal(t, 2, reqs.Expertise.Count)

				// No fighting style for Rogue
				assert.Nil(t, reqs.FightingStyle)
			},
		},
		{
			name:    "Wizard Level 1",
			classID: classes.Wizard,
			validate: func(t *testing.T, reqs *Requirements) {
				require.NotNil(t, reqs)

				// Check skills
				require.NotNil(t, reqs.Skills)
				assert.Equal(t, 2, reqs.Skills.Count)
				assert.Contains(t, reqs.Skills.Options, skills.Arcana)
				assert.Contains(t, reqs.Skills.Options, skills.Investigation)

				// Check cantrips
				require.NotNil(t, reqs.Cantrips)
				assert.Equal(t, 3, reqs.Cantrips.Count)
				assert.Equal(t, 0, reqs.Cantrips.Level)

				// Check spells
				require.NotNil(t, reqs.Spells)
				assert.Equal(t, 6, reqs.Spells.Count)
				assert.Equal(t, 1, reqs.Spells.Level)
			},
		},
		{
			name:    "Bard Level 1",
			classID: classes.Bard,
			validate: func(t *testing.T, reqs *Requirements) {
				require.NotNil(t, reqs)

				// Check skills - Bard can choose ANY 3 skills
				require.NotNil(t, reqs.Skills)
				assert.Equal(t, 3, reqs.Skills.Count)
				assert.Nil(t, reqs.Skills.Options) // nil means any skill

				// Check instruments
				require.NotNil(t, reqs.Instruments)
				assert.Equal(t, 3, reqs.Instruments.Count)
				assert.Nil(t, reqs.Instruments.Options) // can choose any
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqs := GetClassRequirements(tt.classID)
			tt.validate(t, reqs)
		})
	}
}

func TestGetRaceRequirements(t *testing.T) {
	tests := []struct {
		name     string
		raceID   races.Race
		validate func(t *testing.T, reqs *Requirements)
	}{
		{
			name:   "Half-Elf Has Choices",
			raceID: races.HalfElf,
			validate: func(t *testing.T, reqs *Requirements) {
				require.NotNil(t, reqs)

				// Half-Elf chooses 2 skills
				require.NotNil(t, reqs.Skills)
				assert.Equal(t, 2, reqs.Skills.Count)
				assert.Nil(t, reqs.Skills.Options) // can choose any

				// Half-Elf chooses 1 language
				require.NotNil(t, reqs.Languages)
				assert.Equal(t, 1, reqs.Languages.Count)
				assert.Nil(t, reqs.Languages.Options) // can choose any
			},
		},
		{
			name:   "Dragonborn Has Ancestry Choice",
			raceID: races.Dragonborn,
			validate: func(t *testing.T, reqs *Requirements) {
				require.NotNil(t, reqs)

				// Dragonborn chooses ancestry
				require.NotNil(t, reqs.DraconicAncestry)
				assert.Len(t, reqs.DraconicAncestry.Options, 10)
				assert.Contains(t, reqs.DraconicAncestry.Options, AncestryRed)
				assert.Contains(t, reqs.DraconicAncestry.Options, AncestryGold)
			},
		},
		{
			name:   "Human Has No Choices",
			raceID: races.Human,
			validate: func(t *testing.T, reqs *Requirements) {
				assert.Nil(t, reqs)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqs := GetRaceRequirements(tt.raceID)
			tt.validate(t, reqs)
		})
	}
}

func TestGetRequirements_Combined(t *testing.T) {
	// Test combining class and race requirements
	reqs := GetRequirements(classes.Fighter, races.HalfElf)
	require.NotNil(t, reqs)

	// Should have Fighter's skills requirement
	require.NotNil(t, reqs.Skills)
	assert.Equal(t, 2, reqs.Skills.Count)
	assert.Len(t, reqs.Skills.Options, 8) // Fighter's specific options

	// Should have Fighter's fighting style
	require.NotNil(t, reqs.FightingStyle)

	// Should have Half-Elf's language requirement
	require.NotNil(t, reqs.Languages)
	assert.Equal(t, 1, reqs.Languages.Count)
}

func TestEquipmentRequirements(t *testing.T) {
	reqs := GetClassRequirements(classes.Fighter)
	require.NotNil(t, reqs)
	require.Len(t, reqs.Equipment, 4)

	// Check first equipment choice (armor)
	armorChoice := reqs.Equipment[0]
	assert.Equal(t, 1, armorChoice.Choose)
	assert.Len(t, armorChoice.Options, 2)

	// Check chain mail option
	chainMail := armorChoice.Options[0]
	assert.Equal(t, "chain-mail", chainMail.ID)
	assert.Len(t, chainMail.Items, 1)
	assert.Equal(t, "armor", chainMail.Items[0].Type)
	assert.Equal(t, "chain-mail", chainMail.Items[0].ID)

	// Check leather armor set option
	leatherSet := armorChoice.Options[1]
	assert.Equal(t, "leather-armor-set", leatherSet.ID)
	assert.Len(t, leatherSet.Items, 3) // leather, longbow, arrows
}

func TestAllClassesHaveLevel1Requirements(t *testing.T) {
	allClasses := []classes.Class{
		classes.Barbarian,
		classes.Bard,
		classes.Cleric,
		classes.Druid,
		classes.Fighter,
		classes.Monk,
		classes.Paladin,
		classes.Ranger,
		classes.Rogue,
		classes.Sorcerer,
		classes.Warlock,
		classes.Wizard,
	}

	for _, classID := range allClasses {
		t.Run(string(classID), func(t *testing.T) {
			reqs := GetClassRequirements(classID)
			require.NotNil(t, reqs, "Every class should have level 1 requirements")

			// Every class should have skill choices
			require.NotNil(t, reqs.Skills, "%s should have skill choices", classID)
			assert.Greater(t, reqs.Skills.Count, 0)

			// Bard is special - can choose any skills
			if classID == classes.Bard {
				assert.Nil(t, reqs.Skills.Options)
			} else {
				assert.NotNil(t, reqs.Skills.Options)
				assert.Greater(t, len(reqs.Skills.Options), 0)
			}
		})
	}
}
