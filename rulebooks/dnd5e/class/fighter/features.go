package fighter

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// GetFeatures returns Fighter features by level
func GetFeatures() map[int][]class.FeatureData {
	return map[int][]class.FeatureData{
		1: {
			{
				ID:          "fighting-style",
				Name:        "Fighting Style",
				Level:       1,
				Description: "You adopt a particular style of fighting as your specialty",
				Choice: &class.ChoiceData{
					ID:          "fighting-style-choice",
					Type:        "fighting_style",
					Description: "Choose a fighting style",
					Choose:      1,
					From: []string{
						"archery",
						"defense",
						"dueling",
						"great-weapon-fighting",
						"protection",
						"two-weapon-fighting",
					},
				},
			},
			{
				ID:          "second-wind",
				Name:        "Second Wind",
				Level:       1,
				Description: "You have a limited well of stamina that you can draw on to protect yourself from harm. On your turn, you can use a bonus action to regain hit points equal to 1d10 + your fighter level. Once you use this feature, you must finish a short or long rest before you can use it again.",
			},
		},
		2: {
			{
				ID:          "action-surge",
				Name:        "Action Surge",
				Level:       2,
				Description: "Starting at 2nd level, you can push yourself beyond your normal limits for a moment. On your turn, you can take one additional action. Once you use this feature, you must finish a short or long rest before you can use it again. Starting at 17th level, you can use it twice before a rest, but only once on the same turn.",
			},
		},
		3: {
			{
				ID:          "martial-archetype",
				Name:        "Martial Archetype",
				Level:       3,
				Description: "At 3rd level, you choose an archetype that you strive to emulate in your combat styles and techniques",
			},
		},
		4: {
			{
				ID:          "ability-score-improvement-4",
				Name:        "Ability Score Improvement",
				Level:       4,
				Description: "When you reach 4th level, and again at 6th, 8th, 12th, 14th, 16th, and 19th level, you can increase one ability score of your choice by 2, or you can increase two ability scores of your choice by 1. As normal, you can't increase an ability score above 20 using this feature.",
			},
		},
		5: {
			{
				ID:          "extra-attack",
				Name:        "Extra Attack",
				Level:       5,
				Description: "Beginning at 5th level, you can attack twice, instead of once, whenever you take the Attack action on your turn. The number of attacks increases to three when you reach 11th level in this class and to four when you reach 20th level in this class.",
			},
		},
		6: {
			{
				ID:          "ability-score-improvement-6",
				Name:        "Ability Score Improvement",
				Level:       6,
				Description: "You can increase one ability score of your choice by 2, or you can increase two ability scores of your choice by 1.",
			},
		},
		8: {
			{
				ID:          "ability-score-improvement-8",
				Name:        "Ability Score Improvement",
				Level:       8,
				Description: "You can increase one ability score of your choice by 2, or you can increase two ability scores of your choice by 1.",
			},
		},
		9: {
			{
				ID:          "indomitable",
				Name:        "Indomitable",
				Level:       9,
				Description: "Beginning at 9th level, you can reroll a saving throw that you fail. If you do so, you must use the new roll, and you can't use this feature again until you finish a long rest. You can use this feature twice between long rests starting at 13th level and three times between long rests starting at 17th level.",
			},
		},
		11: {
			{
				ID:          "extra-attack-2",
				Name:        "Extra Attack (2)",
				Level:       11,
				Description: "You can attack three times whenever you take the Attack action on your turn.",
			},
		},
		12: {
			{
				ID:          "ability-score-improvement-12",
				Name:        "Ability Score Improvement",
				Level:       12,
				Description: "You can increase one ability score of your choice by 2, or you can increase two ability scores of your choice by 1.",
			},
		},
		13: {
			{
				ID:          "indomitable-2",
				Name:        "Indomitable (2 uses)",
				Level:       13,
				Description: "You can use Indomitable twice between long rests.",
			},
		},
		14: {
			{
				ID:          "ability-score-improvement-14",
				Name:        "Ability Score Improvement",
				Level:       14,
				Description: "You can increase one ability score of your choice by 2, or you can increase two ability scores of your choice by 1.",
			},
		},
		16: {
			{
				ID:          "ability-score-improvement-16",
				Name:        "Ability Score Improvement",
				Level:       16,
				Description: "You can increase one ability score of your choice by 2, or you can increase two ability scores of your choice by 1.",
			},
		},
		17: {
			{
				ID:          "action-surge-2",
				Name:        "Action Surge (2 uses)",
				Level:       17,
				Description: "You can use Action Surge twice before a rest, but only once on the same turn.",
			},
			{
				ID:          "indomitable-3",
				Name:        "Indomitable (3 uses)",
				Level:       17,
				Description: "You can use Indomitable three times between long rests.",
			},
		},
		19: {
			{
				ID:          "ability-score-improvement-19",
				Name:        "Ability Score Improvement",
				Level:       19,
				Description: "You can increase one ability score of your choice by 2, or you can increase two ability scores of your choice by 1.",
			},
		},
		20: {
			{
				ID:          "extra-attack-3",
				Name:        "Extra Attack (3)",
				Level:       20,
				Description: "You can attack four times whenever you take the Attack action on your turn.",
			},
		},
	}
}

// GetResources returns Fighter-specific resources
func GetResources() []class.ResourceData {
	return []class.ResourceData{
		{
			Type:       shared.ClassResourceSecondWind,
			Name:       "Second Wind",
			MaxFormula: "1", // Always 1 use
			Resets:     shared.ResetTypeShortRest,
		},
		{
			Type:       shared.ClassResourceActionSurge,
			Name:       "Action Surge",
			MaxFormula: "level >= 17 ? 2 : 1",
			Resets:     shared.ResetTypeShortRest,
			UsesPerLevel: map[int]int{
				1:  1,
				17: 2,
			},
		},
		{
			Type:       shared.ClassResourceIndomitable,
			Name:       "Indomitable",
			MaxFormula: "level >= 17 ? 3 : level >= 13 ? 2 : 1",
			Resets:     shared.ResetTypeLongRest,
			UsesPerLevel: map[int]int{
				9:  1,
				13: 2,
				17: 3,
			},
		},
	}
}