package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// ChoiceData represents a choice made during character creation using a sum type pattern
// Deprecated: Use the new Choice interface types instead
// This is kept for backward compatibility and JSON serialization
type ChoiceData struct {
	Category shared.ChoiceCategory `json:"category"`  // Type-safe category
	Source   shared.ChoiceSource   `json:"source"`    // Type-safe source
	ChoiceID string                `json:"choice_id"` // Specific choice identifier like "fighter_proficiencies_1"

	// Selection fields - only one should be populated based on Category
	NameSelection            *string                 `json:"name,omitempty"`               // For ChoiceName
	SkillSelection           []skills.Skill          `json:"skills,omitempty"`             // For ChoiceSkills
	LanguageSelection        []languages.Language    `json:"languages,omitempty"`          // For ChoiceLanguages
	AbilityScoreSelection    *shared.AbilityScores   `json:"ability_scores,omitempty"`     // For ChoiceAbilityScores
	FightingStyleSelection   *string                 `json:"fighting_style,omitempty"`     // For ChoiceFightingStyle
	EquipmentSelection       []string                `json:"equipment,omitempty"`          // For ChoiceEquipment
	RaceSelection            *RaceChoice             `json:"race,omitempty"`               // For ChoiceRace
	ClassSelection           *ClassChoice            `json:"class,omitempty"`              // For ChoiceClass
	BackgroundSelection      *backgrounds.Background `json:"background,omitempty"`         // For ChoiceBackground
	SpellSelection           []string                `json:"spells,omitempty"`             // For ChoiceSpells
	CantripSelection         []string                `json:"cantrips,omitempty"`           // For ChoiceCantrips
	ExpertiseSelection       []string                `json:"expertise,omitempty"`          // For ChoiceExpertise
	TraitSelection           []string                `json:"traits,omitempty"`             // For ChoiceTraits
	ToolProficiencySelection []string                `json:"tool_proficiencies,omitempty"` // For ChoiceToolProficiency
}
