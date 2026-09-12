package choices

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
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

// ValidateCharacterCreation validates all choices for character creation.
//
// Validates each source's Requirements against that source's own
// submissions independently, rather than merging them into a single
// Requirements struct first. A merge is lossy for fields like Tools: two
// sources can each carry their own ToolRequirement (different ID,
// different Options), and merging them into one struct can only keep one
// source's ID/Options while summing both sources' Count — silently
// validating against the wrong requirement. Per-source ChoiceIDs are
// unique by construction, so validating each source's Requirements
// against the shared Submissions pool naturally isolates that source's
// own choices without needing to merge anything.
func (v *Validator) ValidateCharacterCreation(
	classID classes.Class,
	raceID races.Race,
	backgroundID backgrounds.Background,
	submissions *Submissions,
) *ValidationResult {
	return v.validatePerSource(submissions,
		GetClassRequirements(classID), GetRaceRequirements(raceID), GetBackgroundRequirements(backgroundID))
}

// validatePerSource validates each of reqs independently against the same
// Submissions pool and combines the results. nil entries are skipped.
func (v *Validator) validatePerSource(submissions *Submissions, reqs ...*Requirements) *ValidationResult {
	result := &ValidationResult{Valid: true, Errors: []ValidationError{}}

	for _, req := range reqs {
		if req == nil {
			continue
		}
		sub := v.Validate(req, submissions)
		if !sub.Valid {
			result.Valid = false
			result.Errors = append(result.Errors, sub.Errors...)
		}
	}

	return result
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

	if len(chosenSkills) != totalChosen {
		return &ValidationError{
			Category: shared.ChoiceSkills,
			ChoiceID: req.ID,
			Message:  "Skill choices must be distinct",
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

// EligibleEquipment returns the complete ordered set of equipment allowed by a
// category choice. Requirement expansion and every category-selection validator
// must use this function so clients are never offered a value the rules reject.
func EligibleEquipment(
	equipType shared.EquipmentType,
	categories []shared.EquipmentCategory,
) ([]equipment.Equipment, error) {
	return equipment.GetByCategory(equipType, categories)
}

func (v *Validator) validateEquipmentCategory(
	req *EquipmentCategoryRequirement,
	submissions *Submissions,
) *ValidationError {
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

			// Get all valid equipment IDs through the same membership path used
			// to expand category choices for clients.
			validEquipment, err := EligibleEquipment(req.Type, req.Categories)
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
	// Find tool submissions for THIS specific requirement — a shared
	// Submissions pool can carry another source's ChoiceToolProficiency
	// entries too (e.g. a race and a background each with their own tool
	// choice), so filtering by req.ID matters here exactly as it already
	// does in validateSkills.
	toolSubs := submissions.GetByCategory(shared.ChoiceToolProficiency)

	totalChosen := 0
	chosenTools := make(map[shared.SelectionID]bool)
	found := false

	for _, sub := range toolSubs {
		if sub.ChoiceID == req.ID {
			found = true
			totalChosen += len(sub.Values)
			for _, toolID := range sub.Values {
				chosenTools[toolID] = true
			}
		}
	}

	if !found {
		return &ValidationError{
			Category: shared.ChoiceToolProficiency,
			ChoiceID: req.ID,
			Message:  fmt.Sprintf("%s: Must choose %d tools", req.Label, req.Count),
		}
	}

	if totalChosen != req.Count {
		return &ValidationError{
			Category: shared.ChoiceToolProficiency,
			ChoiceID: req.ID,
			Message:  fmt.Sprintf("%s: Must choose exactly %d tools, got %d", req.Label, req.Count, totalChosen),
		}
	}

	if len(chosenTools) != totalChosen {
		return &ValidationError{
			Category: shared.ChoiceToolProficiency,
			ChoiceID: req.ID,
			Message:  fmt.Sprintf("%s: tool selections must be unique", req.Label),
		}
	}

	if len(req.Options) > 0 {
		allowed := make(map[shared.SelectionID]bool, len(req.Options))
		for _, option := range req.Options {
			allowed[option] = true
		}
		for toolID := range chosenTools {
			if !allowed[toolID] {
				return &ValidationError{
					Category: shared.ChoiceToolProficiency,
					ChoiceID: req.ID,
					Message:  fmt.Sprintf("Tool '%s' is not in the allowed options", toolID),
				}
			}
		}
	}

	return nil
}

type validateChoiceInput struct {
	Submissions []Submission
	ChoiceID    ChoiceID
	Options     []shared.SelectionID
	Label       string
	Category    shared.ChoiceCategory
	ItemName    string
	Count       int // Expected number of selections (1 for single choice, >1 for multiple)
}

// validateChoice validates that the correct number of choices were made from allowed options
func (v *Validator) validateChoice(input validateChoiceInput) *ValidationError {
	// Default to 1 if not specified (for backward compatibility)
	if input.Count == 0 {
		input.Count = 1
	}

	found := false
	for _, sub := range input.Submissions {
		if sub.ChoiceID == input.ChoiceID {
			found = true
			if len(sub.Values) != input.Count {
				var message string
				if input.Count == 1 {
					message = fmt.Sprintf("Must choose exactly one %s", input.ItemName)
				} else {
					message = fmt.Sprintf("Must choose exactly %d %ss, got %d", input.Count, input.ItemName, len(sub.Values))
				}
				return &ValidationError{
					Category: input.Category,
					ChoiceID: input.ChoiceID,
					Message:  message,
				}
			}

			seen := make(map[shared.SelectionID]bool, len(sub.Values))
			for _, value := range sub.Values {
				if seen[value] {
					return &ValidationError{
						Category: input.Category,
						ChoiceID: input.ChoiceID,
						Message:  fmt.Sprintf("Duplicate %s choice '%s'", input.ItemName, value),
					}
				}
				seen[value] = true
			}

			// Validate all chosen options are allowed
			if len(input.Options) > 0 {
				// Build a map for O(1) lookups
				allowedOptions := make(map[shared.SelectionID]bool)
				for _, option := range input.Options {
					allowedOptions[option] = true
				}

				for _, chosen := range sub.Values {
					if !allowedOptions[chosen] {
						return &ValidationError{
							Category: input.Category,
							ChoiceID: input.ChoiceID,
							Message:  fmt.Sprintf("Invalid %s choice '%s'", input.ItemName, chosen),
						}
					}
				}
			}
			break
		}
	}

	if !found {
		return &ValidationError{
			Category: input.Category,
			ChoiceID: input.ChoiceID,
			Message:  fmt.Sprintf("%s required", input.Label),
		}
	}

	return nil
}

func (v *Validator) validateSubclass(req *SubclassRequirement, submissions *Submissions) *ValidationError {
	return v.validateChoice(validateChoiceInput{
		Submissions: submissions.GetByCategory(shared.ChoiceClass),
		ChoiceID:    req.ID,
		Options:     req.Options,
		Label:       req.Label,
		Category:    shared.ChoiceClass,
		ItemName:    "subclass",
		Count:       1,
	})
}

func (v *Validator) validateCantrips(req *CantripRequirement, submissions *Submissions) *ValidationError {
	return v.validateChoice(validateChoiceInput{
		Submissions: submissions.GetByCategory(shared.ChoiceCantrips),
		ChoiceID:    req.ID,
		Options:     req.Options,
		Label:       req.Label,
		Category:    shared.ChoiceCantrips,
		ItemName:    "cantrip",
		Count:       req.Count, // Use the actual count from requirements
	})
}

func (v *Validator) validateFightingStyle(req *FightingStyleRequirement, submissions *Submissions) *ValidationError {
	return v.validateChoice(validateChoiceInput{
		Submissions: submissions.GetByCategory(shared.ChoiceFightingStyle),
		ChoiceID:    req.ID,
		Options:     req.Options,
		Label:       req.Label,
		Category:    shared.ChoiceFightingStyle,
		ItemName:    "fighting style",
		Count:       1,
	})
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
	seen := make(map[spells.Spell]bool)
	for _, sub := range spellSubs {
		if sub.ChoiceID == req.ID {
			found = true
			totalChosen += len(sub.Values)
			for _, spell := range sub.Values {
				if seen[spell] {
					return &ValidationError{Category: shared.ChoiceSpells, ChoiceID: req.ID,
						Message: fmt.Sprintf("Spell '%s' was chosen more than once", spell)}
				}
				seen[spell] = true
			}

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
