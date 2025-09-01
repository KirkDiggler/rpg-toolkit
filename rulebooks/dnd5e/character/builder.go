// Package character provides D&D 5e character creation and management functionality
package character

import (
	"fmt"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Progress flags
const (
	ProgressName uint32 = 1 << iota
	ProgressRace
	ProgressClass
	ProgressBackground
	ProgressAbilityScores
	ProgressSkills
	ProgressLanguages
	ProgressEquipment
	ProgressSpells
)

// DraftData is the persistent representation of a draft
type DraftData struct {
	ID            string `json:"id"`
	PlayerID      string `json:"player_id"`
	Name          string `json:"name"`
	ProgressFlags uint32 `json:"progress_flags"`

	// Core identity choices
	RaceChoice         RaceChoice             `json:"race_choice"`
	ClassChoice        ClassChoice            `json:"class_choice"`
	BackgroundChoice   backgrounds.Background `json:"background_choice"`
	AbilityScoreChoice shared.AbilityScores   `json:"ability_score_choice"`

	// All choices with source tracking
	Choices []ChoiceData `json:"choices"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RaceChoice represents a race selection with optional subrace
type RaceChoice struct {
	RaceID    races.Race    `json:"race_id"`
	SubraceID races.Subrace `json:"subrace_id,omitempty"`
}

// MissingRequiredSubrace returns true if this race requires a subrace but none is selected
func (rc RaceChoice) MissingRequiredSubrace() bool {
	// These races require subrace selection per PHB
	requiresSubrace := rc.RaceID == races.Elf ||
		rc.RaceID == races.Dwarf ||
		rc.RaceID == races.Halfling ||
		rc.RaceID == races.Gnome

	return requiresSubrace && rc.SubraceID == ""
}

// HasSubraceOptions returns true if this race has subrace options available
func (rc RaceChoice) HasSubraceOptions() bool {
	return rc.RaceID == races.Elf ||
		rc.RaceID == races.Dwarf ||
		rc.RaceID == races.Halfling ||
		rc.RaceID == races.Gnome
	// Add more races with subraces as they're implemented
}

// GetAutomaticGrants returns the automatic grants for this race (and subrace if applicable)
func (rc RaceChoice) GetAutomaticGrants() races.AutomaticGrants {
	// GetAutomaticGrants returns a value, not a pointer
	baseGrants := races.GetAutomaticGrants(rc.RaceID)

	// If we have a subrace, merge its grants
	// Note: This would need enhancement in the races package to handle subrace grants
	// For now, just return base race grants
	return baseGrants
}

// GetProficiencies returns the proficiencies for this race choice (including subrace)
func (rc RaceChoice) GetProficiencies() (Proficiencies, error) {
	if rc.RaceID == "" {
		return Proficiencies{}, rpgerr.New(rpgerr.CodeInvalidArgument, "race ID is required",
			rpgerr.WithMeta("raceID", rc.RaceID))
	}

	grants := rc.GetAutomaticGrants()

	prof := Proficiencies{
		Skills:    grants.Skills,
		Languages: grants.Languages,
	}

	// Weapon proficiencies would need to come from race data
	// which we don't have here. They're handled separately in compileCharacter.
	// TODO: Consider if we should pass race data here or handle differently

	return prof, nil
}

// IsValid validates that the race choice is complete and correct
func (rc RaceChoice) IsValid() error {
	if rc.RaceID == "" {
		return fmt.Errorf("race must be selected")
	}

	if rc.MissingRequiredSubrace() {
		return fmt.Errorf("%s requires a subrace selection", rc.RaceID)
	}

	// Could add validation that subrace belongs to race if we have that data
	return nil
}

// ClassChoice represents a class selection with optional subclass
type ClassChoice struct {
	ClassID    classes.Class    `json:"class_id"`
	SubclassID classes.Subclass `json:"subclass_id,omitempty"`
}

// MissingSubclass returns true if this class requires a subclass at level 1 but none is selected
func (cc ClassChoice) MissingSubclass() bool {
	if (cc.ClassID == classes.Cleric ||
		cc.ClassID == classes.Sorcerer ||
		cc.ClassID == classes.Warlock) && cc.SubclassID == "" {
		return true
	}

	return false
}

// RequiresSubclassAtLevel returns true if this class requires a subclass at the given level
func (cc ClassChoice) RequiresSubclassAtLevel(level int) bool {
	switch cc.ClassID {
	case classes.Cleric, classes.Sorcerer, classes.Warlock:
		return level >= 1
	case classes.Barbarian, classes.Bard, classes.Druid, classes.Monk, classes.Ranger, classes.Rogue, classes.Wizard:
		return level >= 2
	case classes.Fighter, classes.Paladin:
		return level >= 3
	default:
		return false
	}
}

// Proficiencies represents all proficiencies that can be granted by any source
type Proficiencies struct {
	Armor        []proficiencies.Armor
	Weapons      []proficiencies.Weapon
	Tools        []proficiencies.Tool
	Skills       []skills.Skill
	Languages    []languages.Language
	SavingThrows []abilities.Ability
}

// GetProficiencies returns the proficiencies for this class choice (including subclass)
func (cc ClassChoice) GetProficiencies() (Proficiencies, error) {
	if cc.ClassID == "" {
		return Proficiencies{}, rpgerr.New(rpgerr.CodeInvalidArgument, "class ID is required",
			rpgerr.WithMeta("classID", cc.ClassID))
	}

	if cc.SubclassID != "" {
		// Get subclass-specific grants (includes base class grants)
		grants := classes.GetSubclassGrants(cc.SubclassID)
		if grants == nil {
			// Subclass doesn't exist or has no grants - this is an error
			return Proficiencies{}, rpgerr.NewfWithOpts(rpgerr.CodeNotFound,
				[]rpgerr.Option{
					rpgerr.WithMeta("classID", cc.ClassID),
					rpgerr.WithMeta("subclassID", cc.SubclassID),
				},
				"subclass %s not found or has no grants", cc.SubclassID)
		}

		return Proficiencies{
			Armor:        grants.ArmorProficiencies,
			Weapons:      grants.WeaponProficiencies,
			Tools:        grants.ToolProficiencies,
			SavingThrows: grants.SavingThrows,
		}, nil
	}

	// Fall back to base class grants
	grants := classes.GetAutomaticGrants(cc.ClassID)
	if grants == nil {
		// Class doesn't exist or has no grants - this is an error
		return Proficiencies{}, rpgerr.NewfWithOpts(rpgerr.CodeNotFound,
			[]rpgerr.Option{
				rpgerr.WithMeta("classID", cc.ClassID),
			},
			"class %s not found or has no grants", cc.ClassID)
	}

	return Proficiencies{
		Armor:        grants.ArmorProficiencies,
		Weapons:      grants.WeaponProficiencies,
		Tools:        grants.ToolProficiencies,
		SavingThrows: grants.SavingThrows,
	}, nil
}

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}
