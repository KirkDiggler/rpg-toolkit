package choices

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// ValidationResult represents the result of validating choices
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Source   shared.ChoiceSource   `json:"source"`
	Category shared.ChoiceCategory `json:"category"`
	ChoiceID ChoiceID              `json:"choice_id"`
	Message  string                `json:"message"`
}

// Validator validates character choices against requirements
type Validator struct {
	// Could add context like available content, house rules, etc.
}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{}
}

// Validate validates submissions against requirements
func (v *Validator) Validate(requirements *Requirements, submissions *Submissions) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	// Validate skills
	if requirements.Skills != nil {
		if err := v.validateSkills(requirements.Skills, submissions); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
		}
	}

	// Validate equipment
	for _, equipReq := range requirements.Equipment {
		if err := v.validateEquipment(equipReq, submissions); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
		}
	}

	// Validate equipment category choices
	for _, catReq := range requirements.EquipmentCategories {
		if err := v.validateEquipmentCategory(catReq, submissions); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
		}
	}

	// Validate languages
	if requirements.Languages != nil {
		if err := v.validateLanguages(requirements.Languages, submissions); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
		}
	}

	// Validate tools
	if requirements.Tools != nil {
		if err := v.validateTools(requirements.Tools, submissions); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
		}
	}

	return result
}

// ValidateCharacterCreation validates all choices for character creation
func (v *Validator) ValidateCharacterCreation(
	classID classes.Class,
	raceID races.Race,
	submissions *Submissions,
) *ValidationResult {
	// Get requirements
	classReqs := GetClassRequirements(classID)
	raceReqs := GetRaceRequirements(raceID)

	// Merge requirements
	merged := mergeRequirements(classReqs, raceReqs)

	// Validate against merged requirements
	return v.Validate(merged, submissions)
}

func (v *Validator) validateSkills(req *SkillRequirement, submissions *Submissions) *ValidationError {
	// Find skill submissions
	skillSubs := submissions.GetByCategory(shared.ChoiceSkills)
	if len(skillSubs) == 0 {
		return &ValidationError{
			Category: shared.ChoiceSkills,
			Message:  fmt.Sprintf("Must choose %d skills", req.Count),
		}
	}

	// Count total skills chosen
	totalChosen := 0
	for _, sub := range skillSubs {
		totalChosen += len(sub.Values)
	}

	if totalChosen != req.Count {
		return &ValidationError{
			Category: shared.ChoiceSkills,
			Message:  fmt.Sprintf("Must choose exactly %d skills, got %d", req.Count, totalChosen),
		}
	}

	// TODO: Validate that chosen skills are in the allowed options

	return nil
}

func (v *Validator) validateEquipment(req *EquipmentRequirement, submissions *Submissions) *ValidationError {
	// Check if this equipment choice was made
	equipSubs := submissions.GetByCategory(shared.ChoiceEquipment)

	found := false
	for _, sub := range equipSubs {
		if sub.ChoiceID == req.ID { // Using proper ID
			found = true
			if len(sub.Values) != req.Choose {
				return &ValidationError{
					Category: shared.ChoiceEquipment,
					ChoiceID: req.ID,
					Message:  fmt.Sprintf("Must choose exactly %d options, got %d", req.Choose, len(sub.Values)),
				}
			}
		}
	}

	if !found {
		return &ValidationError{
			Category: shared.ChoiceEquipment,
			ChoiceID: req.ID,
			Message:  fmt.Sprintf("%s required", req.Label),
		}
	}

	return nil
}

func (v *Validator) validateEquipmentCategory(req *EquipmentCategoryRequirement, submissions *Submissions) *ValidationError {
	// Check if this equipment category choice was made
	equipSubs := submissions.GetByCategory(shared.ChoiceEquipment)

	found := false
	for _, sub := range equipSubs {
		if sub.ChoiceID == req.ID {
			found = true

			// Validate the number of choices
			if len(sub.Values) != req.Choose {
				return &ValidationError{
					Category: shared.ChoiceEquipment,
					ChoiceID: req.ID,
					Message:  fmt.Sprintf("Must choose exactly %d items, got %d", req.Choose, len(sub.Values)),
				}
			}

			// Get all valid equipment IDs for the categories
			validEquipment, err := equipment.GetByCategory(req.Type, req.Categories)
			if err != nil {
				return &ValidationError{
					Category: shared.ChoiceEquipment,
					ChoiceID: req.ID,
					Message:  fmt.Sprintf("Failed to validate equipment categories: %v", err),
				}
			}

			// Create a set of valid IDs for quick lookup
			validIDs := make(map[string]bool)
			for _, equip := range validEquipment {
				validIDs[equip.EquipmentID()] = true
			}

			// Validate each chosen item is from the allowed categories
			for _, chosenID := range sub.Values {
				if !validIDs[chosenID] {
					return &ValidationError{
						Category: shared.ChoiceEquipment,
						ChoiceID: req.ID,
						Message:  fmt.Sprintf("Invalid equipment choice '%s' - must be from specified categories", chosenID),
					}
				}
			}

			// Check for duplicates if choosing multiple items
			if req.Choose > 1 {
				seen := make(map[string]bool)
				for _, chosenID := range sub.Values {
					if seen[chosenID] {
						return &ValidationError{
							Category: shared.ChoiceEquipment,
							ChoiceID: req.ID,
							Message:  fmt.Sprintf("Cannot choose the same item '%s' multiple times", chosenID),
						}
					}
					seen[chosenID] = true
				}
			}
		}
	}

	if !found {
		return &ValidationError{
			Category: shared.ChoiceEquipment,
			ChoiceID: req.ID,
			Message:  fmt.Sprintf("%s required", req.Label),
		}
	}

	return nil
}

func (v *Validator) validateLanguages(req *LanguageRequirement, submissions *Submissions) *ValidationError {
	// Find language submissions
	langSubs := submissions.GetByCategory(shared.ChoiceLanguages)
	if len(langSubs) == 0 {
		return &ValidationError{
			Category: shared.ChoiceLanguages,
			Message:  fmt.Sprintf("Must choose %d languages", req.Count),
		}
	}

	// Count total languages chosen
	totalChosen := 0
	for _, sub := range langSubs {
		totalChosen += len(sub.Values)
	}

	if totalChosen != req.Count {
		return &ValidationError{
			Category: shared.ChoiceLanguages,
			Message:  fmt.Sprintf("Must choose exactly %d languages, got %d", req.Count, totalChosen),
		}
	}

	return nil
}

func (v *Validator) validateTools(req *ToolRequirement, submissions *Submissions) *ValidationError {
	// Find tool submissions
	toolSubs := submissions.GetByCategory(shared.ChoiceToolProficiency)
	if len(toolSubs) == 0 {
		return &ValidationError{
			Category: shared.ChoiceToolProficiency,
			Message:  fmt.Sprintf("Must choose %d tools", req.Count),
		}
	}

	// Count total tools chosen
	totalChosen := 0
	for _, sub := range toolSubs {
		totalChosen += len(sub.Values)
	}

	if totalChosen != req.Count {
		return &ValidationError{
			Category: shared.ChoiceToolProficiency,
			Message:  fmt.Sprintf("Must choose exactly %d tools, got %d", req.Count, totalChosen),
		}
	}

	return nil
}

// mergeRequirements merges multiple requirement sets
func mergeRequirements(reqs ...*Requirements) *Requirements {
	merged := &Requirements{}

	for _, req := range reqs {
		if req == nil {
			continue
		}

		// Merge skills (take the one with more choices)
		if req.Skills != nil {
			if merged.Skills == nil || req.Skills.Count > merged.Skills.Count {
				merged.Skills = req.Skills
			}
		}

		// Merge equipment (append all)
		merged.Equipment = append(merged.Equipment, req.Equipment...)

		// Merge equipment categories (append all)
		merged.EquipmentCategories = append(merged.EquipmentCategories, req.EquipmentCategories...)

		// Merge languages (sum the counts)
		if req.Languages != nil {
			if merged.Languages == nil {
				merged.Languages = req.Languages
			} else {
				merged.Languages.Count += req.Languages.Count
			}
		}

		// Merge tools
		if req.Tools != nil {
			if merged.Tools == nil {
				merged.Tools = req.Tools
			} else {
				merged.Tools.Count += req.Tools.Count
			}
		}

		// Take first fighting style requirement
		if req.FightingStyle != nil && merged.FightingStyle == nil {
			merged.FightingStyle = req.FightingStyle
		}

		// Take first expertise requirement
		if req.Expertise != nil && merged.Expertise == nil {
			merged.Expertise = req.Expertise
		}

		// Take first subclass requirement
		if req.Subclass != nil && merged.Subclass == nil {
			merged.Subclass = req.Subclass
		}
	}

	return merged
}

// ValidateChoice validates a single choice is valid
func ValidateChoice(choice ChoiceData) error {
	if choice.Category == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "choice missing category")
	}

	if choice.Source == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "choice missing source")
	}

	// Validate based on category
	switch choice.Category {
	case shared.ChoiceName:
		if choice.NameSelection == nil || *choice.NameSelection == "" {
			return rpgerr.New(rpgerr.CodeInvalidArgument, "name choice requires name selection")
		}
	case shared.ChoiceSkills:
		if len(choice.SkillSelection) == 0 {
			return rpgerr.New(rpgerr.CodeInvalidArgument, "skills choice requires skill selection")
		}
	case shared.ChoiceEquipment:
		if len(choice.EquipmentSelection) == 0 {
			return rpgerr.New(rpgerr.CodeInvalidArgument, "equipment choice requires equipment selection")
		}
	}

	return nil
}
