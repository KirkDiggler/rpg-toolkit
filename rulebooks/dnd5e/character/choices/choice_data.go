// Package choices provides character creation choice validation for D&D 5e
package choices

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// ChoiceData represents a choice made during character creation
// This uses a sum type pattern where only one selection field should be populated
type ChoiceData struct {
	Category shared.ChoiceCategory `json:"category"`
	Source   shared.ChoiceSource   `json:"source"`
	ChoiceID ChoiceID              `json:"choice_id,omitempty"`
	OptionID string                `json:"option_id,omitempty"` // For equipment bundles, tracks which option was selected

	// Selection fields - only one should be populated based on Category
	NameSelection          *string                       `json:"name,omitempty"`
	SkillSelection         []skills.Skill                `json:"skills,omitempty"`
	LanguageSelection      []languages.Language          `json:"languages,omitempty"`
	AbilityScoreSelection  shared.AbilityScores          `json:"ability_scores,omitempty"`
	FightingStyleSelection *fightingstyles.FightingStyle `json:"fighting_style,omitempty"`
	EquipmentSelection     []shared.SelectionID          `json:"equipment,omitempty"`
	BackgroundSelection    *backgrounds.Background       `json:"background,omitempty"`
	SpellSelection         []spells.Spell                `json:"spells,omitempty"`
	ToolSelection          []proficiencies.Tool          `json:"tools,omitempty"`
	ExpertiseSelection     []skills.Skill                `json:"expertise,omitempty"`
	TraitSelection         []string                      `json:"traits,omitempty"`
	Method                 string                        `json:"method,omitempty"` // For ability score generation
}
