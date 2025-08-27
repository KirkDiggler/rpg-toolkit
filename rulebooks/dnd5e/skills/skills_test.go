package skills_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

func TestGetByID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected skills.Skill
		wantErr  bool
	}{
		{
			name:     "valid skill",
			id:       "athletics",
			expected: skills.Athletics,
			wantErr:  false,
		},
		{
			name:     "hyphenated skill",
			id:       "animal-handling",
			expected: skills.AnimalHandling,
			wantErr:  false,
		},
		{
			name:     "invalid skill",
			id:       "flying",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skill, err := skills.GetByID(tt.id)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, skill)
			}
		})
	}
}

func TestSkillAbility(t *testing.T) {
	tests := []struct {
		skill   skills.Skill
		ability abilities.Ability
	}{
		{skills.Athletics, abilities.STR},
		{skills.Acrobatics, abilities.DEX},
		{skills.SleightOfHand, abilities.DEX},
		{skills.Stealth, abilities.DEX},
		{skills.Arcana, abilities.INT},
		{skills.History, abilities.INT},
		{skills.Investigation, abilities.INT},
		{skills.Nature, abilities.INT},
		{skills.Religion, abilities.INT},
		{skills.AnimalHandling, abilities.WIS},
		{skills.Insight, abilities.WIS},
		{skills.Medicine, abilities.WIS},
		{skills.Perception, abilities.WIS},
		{skills.Survival, abilities.WIS},
		{skills.Deception, abilities.CHA},
		{skills.Intimidation, abilities.CHA},
		{skills.Performance, abilities.CHA},
		{skills.Persuasion, abilities.CHA},
	}

	for _, tt := range tests {
		t.Run(string(tt.skill), func(t *testing.T) {
			assert.Equal(t, tt.ability, tt.skill.Ability())
		})
	}
}

func TestSkillDisplay(t *testing.T) {
	tests := []struct {
		skill   skills.Skill
		display string
	}{
		{skills.Athletics, "Athletics"},
		{skills.AnimalHandling, "Animal Handling"},
		{skills.SleightOfHand, "Sleight of Hand"},
	}

	for _, tt := range tests {
		t.Run(string(tt.skill), func(t *testing.T) {
			assert.Equal(t, tt.display, tt.skill.Display())
		})
	}
}

func TestList(t *testing.T) {
	allSkills := skills.List()

	// Should have all 18 skills
	require.Len(t, allSkills, 18)

	// Check first and last
	assert.Equal(t, skills.Acrobatics, allSkills[0])
	assert.Equal(t, skills.Survival, allSkills[17])
}

func TestByAbility(t *testing.T) {
	// STR skills
	strSkills := skills.ByAbility(abilities.STR)
	require.Len(t, strSkills, 1)
	assert.Contains(t, strSkills, skills.Athletics)

	// DEX skills
	dexSkills := skills.ByAbility(abilities.DEX)
	require.Len(t, dexSkills, 3)
	assert.Contains(t, dexSkills, skills.Acrobatics)
	assert.Contains(t, dexSkills, skills.SleightOfHand)
	assert.Contains(t, dexSkills, skills.Stealth)

	// INT skills
	intSkills := skills.ByAbility(abilities.INT)
	require.Len(t, intSkills, 5)
	assert.Contains(t, intSkills, skills.Arcana)
	assert.Contains(t, intSkills, skills.History)
	assert.Contains(t, intSkills, skills.Investigation)
	assert.Contains(t, intSkills, skills.Nature)
	assert.Contains(t, intSkills, skills.Religion)
}
