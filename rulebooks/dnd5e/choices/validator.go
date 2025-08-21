package choices

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// ValidateSelection validates that a selection is valid for a given choice
func ValidateSelection(choice Choice, selections []string) error {
	return ValidateSelectionCtx(context.Background(), choice, selections)
}

// ValidateSelectionCtx validates that a selection is valid for a given choice with context
func ValidateSelectionCtx(ctx context.Context, choice Choice, selections []string) error {
	// Add choice context that will be included in all errors
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("choice_id", string(choice.ID)),
		rpgerr.Meta("category", string(choice.Category)),
		rpgerr.Meta("choose", choice.Choose),
	)

	// Check count
	if len(selections) != choice.Choose {
		ctx := rpgerr.WithMetadata(ctx,
			rpgerr.Meta("expected", choice.Choose),
			rpgerr.Meta("got", len(selections)),
		)
		return rpgerr.NewCtx(ctx, rpgerr.CodeInvalidArgument, "invalid selection count")
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, sel := range selections {
		if seen[sel] {
			ctx := rpgerr.WithMetadata(ctx, rpgerr.Meta("selection", sel))
			return rpgerr.NewCtx(ctx, rpgerr.CodeInvalidArgument, "duplicate selection")
		}
		seen[sel] = true
	}

	// Validate each selection against available options
	for _, selection := range selections {
		selCtx := rpgerr.WithMetadata(ctx, rpgerr.Meta("selection", selection))
		if err := validateSingleSelection(selCtx, choice, selection); err != nil {
			return rpgerr.WrapCtx(selCtx, err, "invalid selection")
		}
	}

	return nil
}

// validateSingleSelection checks if a single selection is valid for the choice
func validateSingleSelection(ctx context.Context, choice Choice, selection string) error {
	// Context already has choice_id, category, and selection

	// For category options, we need to check the actual items
	for _, opt := range choice.Options {
		switch o := opt.(type) {
		case SingleOption:
			if o.ItemID == selection {
				return nil
			}

		case BundleOption:
			if o.ID == selection {
				return nil
			}

		case SkillListOption:
			// Check if selection is a valid skill from the list
			for _, skill := range o.Skills {
				if string(skill) == selection {
					return nil
				}
			}

		case LanguageListOption:
			if o.AllowAny {
				// Any language is valid - just check it exists
				if _, err := languages.GetByID(selection); err == nil {
					return nil
				}
			} else {
				// Check against specific list
				for _, lang := range o.Languages {
					if string(lang) == selection {
						return nil
					}
				}
			}

		case WeaponCategoryOption:
			// Check against actual weapon list
			weapon, err := weapons.GetByID(selection)
			if err == nil && weapon.Category == o.Category {
				return nil
			}
		}
	}

	return rpgerr.NewCtx(ctx, rpgerr.CodeInvalidArgument, "selection not found in available options")
}

// GetAvailableOptions returns all valid selections for a choice
func GetAvailableOptions(choice Choice) ([]string, error) {
	return GetAvailableOptionsCtx(context.Background(), choice)
}

// GetAvailableOptionsCtx returns all valid selections for a choice with context
func GetAvailableOptionsCtx(ctx context.Context, choice Choice) ([]string, error) {
	// Add choice context
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("choice_id", string(choice.ID)),
		rpgerr.Meta("category", string(choice.Category)),
	)

	var options []string

	for _, opt := range choice.Options {
		switch o := opt.(type) {
		case SingleOption:
			options = append(options, o.ItemID)

		case BundleOption:
			options = append(options, o.ID)

		case SkillListOption:
			for _, skill := range o.Skills {
				options = append(options, string(skill))
			}

		case LanguageListOption:
			if o.AllowAny {
				// Return all languages
				for _, lang := range languages.StandardLanguages() {
					options = append(options, string(lang))
				}
				for _, lang := range languages.ExoticLanguages() {
					options = append(options, string(lang))
				}
			} else {
				for _, lang := range o.Languages {
					options = append(options, string(lang))
				}
			}

		case WeaponCategoryOption:
			// Return actual weapons from category
			categoryWeapons := weapons.GetByCategory(o.Category)
			for _, weapon := range categoryWeapons {
				options = append(options, weapon.ID)
			}

		default:
			ctx := rpgerr.WithMetadata(ctx, rpgerr.Meta("type", fmt.Sprintf("%T", o)))
			return nil, rpgerr.NewCtx(ctx, rpgerr.CodeInternal, "unknown option type")
		}
	}

	if len(options) == 0 {
		return nil, rpgerr.NewCtx(ctx, rpgerr.CodeInvalidArgument, "no options available")
	}

	return options, nil
}

// ValidateChoice validates that a choice itself is properly constructed
func ValidateChoice(choice Choice) error {
	return ValidateChoiceCtx(context.Background(), choice)
}

// ValidateChoiceCtx validates that a choice itself is properly constructed with context
func ValidateChoiceCtx(ctx context.Context, choice Choice) error {
	if choice.ID == "" {
		return rpgerr.NewCtx(ctx, rpgerr.CodeInvalidArgument, "choice ID is required")
	}

	// Add choice context for all subsequent errors
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("choice_id", string(choice.ID)),
		rpgerr.Meta("category", string(choice.Category)),
	)

	if choice.Category == "" {
		return rpgerr.NewCtx(ctx, rpgerr.CodeInvalidArgument, "choice category is required")
	}

	if choice.Choose < 1 {
		ctx := rpgerr.WithMetadata(ctx, rpgerr.Meta("choose", choice.Choose))
		return rpgerr.NewCtx(ctx, rpgerr.CodeInvalidArgument, "choose count must be at least 1")
	}

	if len(choice.Options) == 0 {
		return rpgerr.NewCtx(ctx, rpgerr.CodeInvalidArgument, "choice must have at least one option")
	}

	// Validate each option
	for i, opt := range choice.Options {
		if err := opt.Validate(); err != nil {
			optCtx := rpgerr.WithMetadata(ctx, rpgerr.Meta("option_index", i))
			return rpgerr.WrapCtx(optCtx, err, "invalid option")
		}
	}

	// Special validation for skill choices
	if choice.Category == CategorySkill {
		availableCount := 0
		for _, opt := range choice.Options {
			if skillOpt, ok := opt.(SkillListOption); ok {
				availableCount += len(skillOpt.Skills)
			}
		}
		if availableCount < choice.Choose {
			ctx := rpgerr.WithMetadata(ctx,
				rpgerr.Meta("choose", choice.Choose),
				rpgerr.Meta("available", availableCount),
			)
			return rpgerr.NewCtx(ctx, rpgerr.CodeInvalidArgument, "not enough skills available to satisfy choice")
		}
	}

	return nil
}

// ValidateSelectionForSkills is a helper for validating skill selections
func ValidateSelectionForSkills(skillList []skills.Skill, selections []string) error {
	return ValidateSelectionForSkillsCtx(context.Background(), skillList, selections)
}

// ValidateSelectionForSkillsCtx is a helper for validating skill selections with context
func ValidateSelectionForSkillsCtx(ctx context.Context, skillList []skills.Skill, selections []string) error {
	validSkills := make(map[string]bool)
	for _, skill := range skillList {
		validSkills[string(skill)] = true
	}

	for _, selection := range selections {
		if !validSkills[selection] {
			// Try to provide helpful error about what IS valid
			validOptions := make([]string, 0, len(skillList))
			for _, s := range skillList {
				validOptions = append(validOptions, string(s))
			}

			ctx := rpgerr.WithMetadata(ctx,
				rpgerr.Meta("selection", selection),
				rpgerr.Meta("valid_options", validOptions),
			)
			return rpgerr.NewCtx(ctx, rpgerr.CodeInvalidArgument, "invalid skill selection")
		}
	}

	return nil
}

// ValidateWeaponSelection validates that a weapon selection is valid for a category
func ValidateWeaponSelection(category weapons.WeaponCategory, selection string) error {
	return ValidateWeaponSelectionCtx(context.Background(), category, selection)
}

// ValidateWeaponSelectionCtx validates that a weapon selection is valid for a category with context
func ValidateWeaponSelectionCtx(ctx context.Context, category weapons.WeaponCategory, selection string) error {
	// Add category context
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("weapon_category", string(category)),
		rpgerr.Meta("selection", selection),
	)

	weapon, err := weapons.GetByID(selection)
	if err != nil {
		return rpgerr.WrapCtx(ctx, err, "weapon not found")
	}

	if weapon.Category != category {
		ctx := rpgerr.WithMetadata(ctx, rpgerr.Meta("actual_category", string(weapon.Category)))
		return rpgerr.NewCtx(ctx, rpgerr.CodeInvalidArgument, "weapon not in category")
	}

	return nil
}
