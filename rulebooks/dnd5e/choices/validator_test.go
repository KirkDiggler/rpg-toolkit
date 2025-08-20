package choices_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/fighter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSelection(t *testing.T) {
	fighterChoice := fighter.SkillChoices()

	tests := []struct {
		name       string
		choice     choices.Choice
		selections []string
		wantErr    bool
		errContext string
	}{
		{
			name:       "valid fighter skill selection",
			choice:     fighterChoice,
			selections: []string{"athletics", "perception"},
			wantErr:    false,
		},
		{
			name:       "too many selections",
			choice:     fighterChoice,
			selections: []string{"athletics", "perception", "survival"},
			wantErr:    true,
			errContext: "invalid selection count",
		},
		{
			name:       "too few selections",
			choice:     fighterChoice,
			selections: []string{"athletics"},
			wantErr:    true,
			errContext: "invalid selection count",
		},
		{
			name:       "duplicate selection",
			choice:     fighterChoice,
			selections: []string{"athletics", "athletics"},
			wantErr:    true,
			errContext: "duplicate selection",
		},
		{
			name:       "invalid skill for fighter",
			choice:     fighterChoice,
			selections: []string{"athletics", "sleight-of-hand"}, // Rogue skill
			wantErr:    true,
			errContext: "invalid selection",
		},
		{
			name:       "completely invalid skill",
			choice:     fighterChoice,
			selections: []string{"athletics", "flying"},
			wantErr:    true,
			errContext: "invalid selection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := choices.ValidateSelection(tt.choice, tt.selections)
			if tt.wantErr {
				require.Error(t, err)
				// Check it's an rpgerr with context
				var rpgErr *rpgerr.Error
				require.ErrorAs(t, err, &rpgErr)
				assert.Contains(t, err.Error(), tt.errContext)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetAvailableOptions(t *testing.T) {
	tests := []struct {
		name     string
		choice   choices.Choice
		expected []string
		wantErr  bool
	}{
		{
			name:   "fighter skills",
			choice: fighter.SkillChoices(),
			expected: []string{
				"acrobatics",
				"animal-handling",
				"athletics",
				"history",
				"insight",
				"intimidation",
				"perception",
				"survival",
			},
			wantErr: false,
		},
		{
			name: "single option",
			choice: choices.Choice{
				ID:       choices.ChoiceID("test-single"),
				Category: choices.CategoryEquipment,
				Choose:   1,
				Options: []choices.Option{
					choices.SingleOption{
						ItemType: choices.ItemTypeArmor,
						ItemID:   "chain-mail",
					},
				},
			},
			expected: []string{"chain-mail"},
			wantErr:  false,
		},
		{
			name: "bundle option",
			choice: choices.Choice{
				ID:       choices.ChoiceID("test-bundle"),
				Category: choices.CategoryEquipment,
				Choose:   1,
				Options: []choices.Option{
					choices.BundleOption{
						ID: "starter-pack",
						Items: []choices.CountedItem{
							{ItemType: choices.ItemTypeGear, ItemID: "rope", Quantity: 1},
							{ItemType: choices.ItemTypeGear, ItemID: "torch", Quantity: 5},
						},
					},
				},
			},
			expected: []string{"starter-pack"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options, err := choices.GetAvailableOptions(tt.choice)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, tt.expected, options)
			}
		})
	}
}

func TestValidateChoice(t *testing.T) {
	tests := []struct {
		name    string
		choice  choices.Choice
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid fighter skills choice",
			choice:  fighter.SkillChoices(),
			wantErr: false,
		},
		{
			name: "missing ID",
			choice: choices.Choice{
				Category: choices.CategorySkill,
				Choose:   1,
				Options: []choices.Option{
					choices.SkillListOption{
						Skills: []skills.Skill{skills.Athletics},
					},
				},
			},
			wantErr: true,
			errMsg:  "choice ID is required",
		},
		{
			name: "missing category",
			choice: choices.Choice{
				ID:     choices.ChoiceID("test"),
				Choose: 1,
				Options: []choices.Option{
					choices.SkillListOption{
						Skills: []skills.Skill{skills.Athletics},
					},
				},
			},
			wantErr: true,
			errMsg:  "choice category is required",
		},
		{
			name: "invalid choose count",
			choice: choices.Choice{
				ID:       choices.ChoiceID("test"),
				Category: choices.CategorySkill,
				Choose:   0,
				Options: []choices.Option{
					choices.SkillListOption{
						Skills: []skills.Skill{skills.Athletics},
					},
				},
			},
			wantErr: true,
			errMsg:  "choose count must be at least 1",
		},
		{
			name: "no options",
			choice: choices.Choice{
				ID:       choices.ChoiceID("test"),
				Category: choices.CategorySkill,
				Choose:   1,
				Options:  []choices.Option{},
			},
			wantErr: true,
			errMsg:  "choice must have at least one option",
		},
		{
			name: "not enough skills available",
			choice: choices.Choice{
				ID:       choices.ChoiceID("test"),
				Category: choices.CategorySkill,
				Choose:   3,
				Options: []choices.Option{
					choices.SkillListOption{
						Skills: []skills.Skill{
							skills.Athletics,
							skills.Perception,
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "not enough skills available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := choices.ValidateChoice(tt.choice)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSelectionForSkills(t *testing.T) {
	fighterSkills := []skills.Skill{
		skills.Acrobatics,
		skills.AnimalHandling,
		skills.Athletics,
		skills.History,
		skills.Insight,
		skills.Intimidation,
		skills.Perception,
		skills.Survival,
	}

	tests := []struct {
		name       string
		skillList  []skills.Skill
		selections []string
		wantErr    bool
	}{
		{
			name:       "valid selections",
			skillList:  fighterSkills,
			selections: []string{"athletics", "perception"},
			wantErr:    false,
		},
		{
			name:       "invalid skill",
			skillList:  fighterSkills,
			selections: []string{"athletics", "sleight-of-hand"},
			wantErr:    true,
		},
		{
			name:       "empty selection",
			skillList:  fighterSkills,
			selections: []string{},
			wantErr:    false, // Empty is valid, just nothing selected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := choices.ValidateSelectionForSkills(tt.skillList, tt.selections)
			if tt.wantErr {
				require.Error(t, err)
				// Check it has context
				var rpgErr *rpgerr.Error
				if assert.ErrorAs(t, err, &rpgErr) {
					// Should have valid_options metadata
					meta := rpgerr.GetMeta(err)
					assert.NotNil(t, meta["valid_options"])
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
