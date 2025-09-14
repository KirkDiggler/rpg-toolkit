package choices

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
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

	// Validate additional skills (from subclass)
	for _, skillReq := range requirements.AdditionalSkills {
		if err := v.validateSkills(skillReq, submissions); err != nil {
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
		for _, langReq := range requirements.Languages {
			if err := v.validateLanguages(langReq, submissions); err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, *err)
			}
		}
	}

	// Validate tools
	if requirements.Tools != nil {
		if err := v.validateTools(requirements.Tools, submissions); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
		}
	}

	// Validate subclass
	if requirements.Subclass != nil {
		if err := v.validateSubclass(requirements.Subclass, submissions); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
		}
	}

	// Validate cantrips
	if requirements.Cantrips != nil {
		if err := v.validateCantrips(requirements.Cantrips, submissions); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
		}
	}

	// Validate fighting style
	if requirements.FightingStyle != nil {
		if err := v.validateFightingStyle(requirements.FightingStyle, submissions); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
		}
	}

	// Validate expertise (for rogues)
	if requirements.Expertise != nil {
		if err := v.validateExpertise(requirements.Expertise, submissions); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
		}
	}

	// Validate spellbook (for wizards)
	if requirements.Spellbook != nil {
		if err := v.validateSpellbook(requirements.Spellbook, submissions); err != nil {
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

	// Count skills chosen for THIS specific requirement
	totalChosen := 0
	chosenSkills := make(map[skills.Skill]bool)
	found := false

	for _, sub := range skillSubs {
		if sub.ChoiceID == req.ID {
			found = true
			totalChosen += len(sub.Values)
			for _, skillID := range sub.Values {
				chosenSkills[skillID] = true
			}
		}
	}

	if !found {
		return &ValidationError{
			Category: shared.ChoiceSkills,
			ChoiceID: req.ID,
			Message:  fmt.Sprintf("%s: Must choose %d skills", req.Label, req.Count),
		}
	}

	if totalChosen != req.Count {
		return &ValidationError{
			Category: shared.ChoiceSkills,
			ChoiceID: req.ID,
			Message:  fmt.Sprintf("%s: Must choose exactly %d skills, got %d", req.Label, req.Count, totalChosen),
		}
	}

	// If options are specified, validate against them
	if len(req.Options) > 0 {
		// Build allowed set for O(1) lookup
		allowedSkills := make(map[skills.Skill]bool)
		for _, skill := range req.Options {
			allowedSkills[skill] = true
		}

		// Check each chosen skill is allowed
		for skillID := range chosenSkills {
			if !allowedSkills[skillID] {
				return &ValidationError{
					Category: shared.ChoiceSkills,
					ChoiceID: req.ID,
					Message:  fmt.Sprintf("Skill '%s' is not in the allowed options", skillID),
				}
			}
		}
	}
	// If req.Options is nil, any skill is allowed (no validation needed)

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

			// Validate that the option ID is valid
			if sub.OptionID != "" {
				validOption := false
				for _, option := range req.Options {
					if option.ID == sub.OptionID {
						validOption = true
						break
					}
				}
				if !validOption {
					return &ValidationError{
						Category: shared.ChoiceEquipment,
						ChoiceID: req.ID,
						Message:  fmt.Sprintf("Invalid equipment option '%s'", sub.OptionID),
					}
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

func (v *Validator) validateSubclass(req *SubclassRequirement, submissions *Submissions) *ValidationError {
	// Find subclass submissions (using ChoiceClass category)
	subclassSubs := submissions.GetByCategory(shared.ChoiceClass)

	// Look for a submission with the subclass choice ID
	found := false
	for _, sub := range subclassSubs {
		if sub.ChoiceID == req.ID {
			found = true
			if len(sub.Values) != 1 {
				return &ValidationError{
					Category: shared.ChoiceClass,
					ChoiceID: req.ID,
					Message:  "Must choose exactly one subclass",
				}
			}

			// Validate the chosen subclass is in the allowed options
			if len(req.Options) > 0 {
				chosenSubclass := sub.Values[0]
				validSubclass := false
				for _, option := range req.Options {
					if option == chosenSubclass {
						validSubclass = true
						break
					}
				}
				if !validSubclass {
					return &ValidationError{
						Category: shared.ChoiceClass,
						ChoiceID: req.ID,
						Message:  fmt.Sprintf("Invalid subclass choice '%s'", chosenSubclass),
					}
				}
			}
			break
		}
	}

	if !found {
		return &ValidationError{
			Category: shared.ChoiceClass,
			ChoiceID: req.ID,
			Message:  fmt.Sprintf("%s required", req.Label),
		}
	}

	return nil
}

func (v *Validator) validateCantrips(req *CantripRequirement, submissions *Submissions) *ValidationError {
	// Find cantrip submissions
	cantripSubs := submissions.GetByCategory(shared.ChoiceCantrips)

	found := false
	for _, sub := range cantripSubs {
		if sub.ChoiceID == req.ID {
			found = true
			if len(sub.Values) != req.Count {
				return &ValidationError{
					Category: shared.ChoiceCantrips,
					ChoiceID: req.ID,
					Message:  fmt.Sprintf("Must choose exactly %d cantrips, got %d", req.Count, len(sub.Values)),
				}
			}

			// Validate chosen cantrips are in the allowed options
			if len(req.Options) > 0 {
				allowedCantrips := make(map[spells.Spell]bool)
				for _, cantrip := range req.Options {
					allowedCantrips[cantrip] = true
				}

				for _, chosenCantrip := range sub.Values {
					if !allowedCantrips[chosenCantrip] {
						return &ValidationError{
							Category: shared.ChoiceCantrips,
							ChoiceID: req.ID,
							Message:  fmt.Sprintf("Cantrip '%s' is not in the allowed options", chosenCantrip),
						}
					}
				}
			}
			break
		}
	}

	if !found {
		return &ValidationError{
			Category: shared.ChoiceCantrips,
			ChoiceID: req.ID,
			Message:  fmt.Sprintf("%s required", req.Label),
		}
	}

	return nil
}

func (v *Validator) validateFightingStyle(req *FightingStyleRequirement, submissions *Submissions) *ValidationError {
	// Find fighting style submissions
	styleSubs := submissions.GetByCategory(shared.ChoiceFightingStyle)

	found := false
	for _, sub := range styleSubs {
		if sub.ChoiceID == req.ID {
			found = true
			if len(sub.Values) != 1 {
				return &ValidationError{
					Category: shared.ChoiceFightingStyle,
					ChoiceID: req.ID,
					Message:  "Must choose exactly one fighting style",
				}
			}

			// Validate the chosen style is in the allowed options
			if len(req.Options) > 0 {
				chosenStyle := sub.Values[0]
				validStyle := false
				for _, option := range req.Options {
					if option == chosenStyle {
						validStyle = true
						break
					}
				}
				if !validStyle {
					return &ValidationError{
						Category: shared.ChoiceFightingStyle,
						ChoiceID: req.ID,
						Message:  fmt.Sprintf("Invalid fighting style '%s'", chosenStyle),
					}
				}
			}
			break
		}
	}

	if !found {
		return &ValidationError{
			Category: shared.ChoiceFightingStyle,
			ChoiceID: req.ID,
			Message:  fmt.Sprintf("%s required", req.Label),
		}
	}

	return nil
}

func (v *Validator) validateExpertise(req *ExpertiseRequirement, submissions *Submissions) *ValidationError {
	// Find expertise submissions
	expertiseSubs := submissions.GetByCategory(shared.ChoiceExpertise)

	found := false
	totalChosen := 0
	for _, sub := range expertiseSubs {
		if sub.ChoiceID == req.ID {
			found = true
			totalChosen += len(sub.Values)
		}
	}

	if !found {
		return &ValidationError{
			Category: shared.ChoiceExpertise,
			ChoiceID: req.ID,
			Message:  fmt.Sprintf("%s: Must choose %d skills or tools for expertise", req.Label, req.Count),
		}
	}

	if totalChosen != req.Count {
		return &ValidationError{
			Category: shared.ChoiceExpertise,
			ChoiceID: req.ID,
			Message:  fmt.Sprintf("%s: Must choose exactly %d for expertise, got %d", req.Label, req.Count, totalChosen),
		}
	}

	return nil
}

func (v *Validator) validateSpellbook(req *SpellbookRequirement, submissions *Submissions) *ValidationError {
	// Find spellbook submissions
	spellSubs := submissions.GetByCategory(shared.ChoiceSpells)

	found := false
	totalChosen := 0
	for _, sub := range spellSubs {
		if sub.ChoiceID == req.ID {
			found = true
			totalChosen += len(sub.Values)

			// Validate chosen spells are in the allowed options
			if len(req.Options) > 0 {
				allowedSpells := make(map[spells.Spell]bool)
				for _, spell := range req.Options {
					allowedSpells[spell] = true
				}

				for _, chosenSpell := range sub.Values {
					if !allowedSpells[chosenSpell] {
						return &ValidationError{
							Category: shared.ChoiceSpells,
							ChoiceID: req.ID,
							Message:  fmt.Sprintf("Spell '%s' is not in the allowed options", chosenSpell),
						}
					}
				}
			}
		}
	}

	if !found {
		return &ValidationError{
			Category: shared.ChoiceSpells,
			ChoiceID: req.ID,
			Message:  fmt.Sprintf("%s: Must choose %d spells", req.Label, req.Count),
		}
	}

	if totalChosen != req.Count {
		return &ValidationError{
			Category: shared.ChoiceSpells,
			ChoiceID: req.ID,
			Message:  fmt.Sprintf("%s: Must choose exactly %d spells, got %d", req.Label, req.Count, totalChosen),
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

		// Merge languages (append all)
		if req.Languages != nil {
			merged.Languages = append(merged.Languages, req.Languages...)
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
