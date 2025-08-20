package choices_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/fighter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/rogue"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFighterSkillChoices(t *testing.T) {
	choice := fighter.SkillChoices()
	
	assert.Equal(t, "fighter-skills", choice.ID)
	assert.Equal(t, choices.CategorySkill, choice.Category)
	assert.Equal(t, 2, choice.Choose)
	assert.Equal(t, choices.SourceClass, choice.Source)
	
	require.Len(t, choice.Options, 1)
	
	// Check it's a skill list option
	option := choice.Options[0]
	assert.Equal(t, choices.OptionTypeCategory, option.GetType())
	
	skillOption, ok := option.(choices.SkillListOption)
	require.True(t, ok)
	
	// Fighter should have 8 skills to choose from
	assert.Len(t, skillOption.Skills, 8)
	assert.Contains(t, skillOption.Skills, skills.Athletics)
	assert.Contains(t, skillOption.Skills, skills.Survival)
	assert.NotContains(t, skillOption.Skills, skills.SleightOfHand) // Rogue skill
}

func TestRogueSkillChoices(t *testing.T) {
	choice := rogue.SkillChoices()
	
	assert.Equal(t, "rogue-skills", choice.ID)
	assert.Equal(t, choices.CategorySkill, choice.Category)
	assert.Equal(t, 4, choice.Choose) // Rogues get 4 skills!
	assert.Equal(t, choices.SourceClass, choice.Source)
	
	require.Len(t, choice.Options, 1)
	
	skillOption, ok := choice.Options[0].(choices.SkillListOption)
	require.True(t, ok)
	
	// Rogue should have 11 skills to choose from
	assert.Len(t, skillOption.Skills, 11)
	assert.Contains(t, skillOption.Skills, skills.SleightOfHand)
	assert.Contains(t, skillOption.Skills, skills.Stealth)
	assert.NotContains(t, skillOption.Skills, skills.AnimalHandling) // Not a rogue skill
}

func TestSingleOptionValidation(t *testing.T) {
	tests := []struct {
		name    string
		option  choices.SingleOption
		wantErr bool
	}{
		{
			name: "valid option",
			option: choices.SingleOption{
				ItemType: choices.ItemTypeSkill,
				ItemID:   "athletics",
			},
			wantErr: false,
		},
		{
			name: "missing item ID",
			option: choices.SingleOption{
				ItemType: choices.ItemTypeSkill,
				ItemID:   "",
			},
			wantErr: true,
		},
		{
			name: "missing item type",
			option: choices.SingleOption{
				ItemType: "",
				ItemID:   "athletics",
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.option.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBundleOptionValidation(t *testing.T) {
	tests := []struct {
		name    string
		option  choices.BundleOption
		wantErr bool
	}{
		{
			name: "valid bundle",
			option: choices.BundleOption{
				ID: "test-bundle",
				Items: []choices.CountedItem{
					{ItemType: choices.ItemTypeGear, ItemID: "arrow", Quantity: 20},
				},
			},
			wantErr: false,
		},
		{
			name: "missing bundle ID",
			option: choices.BundleOption{
				ID: "",
				Items: []choices.CountedItem{
					{ItemType: choices.ItemTypeGear, ItemID: "arrow", Quantity: 20},
				},
			},
			wantErr: true,
		},
		{
			name: "empty items",
			option: choices.BundleOption{
				ID:    "test-bundle",
				Items: []choices.CountedItem{},
			},
			wantErr: true,
		},
		{
			name: "invalid quantity",
			option: choices.BundleOption{
				ID: "test-bundle",
				Items: []choices.CountedItem{
					{ItemType: choices.ItemTypeGear, ItemID: "arrow", Quantity: 0},
				},
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.option.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}