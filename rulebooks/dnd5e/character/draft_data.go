package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// DraftData represents the serializable form of a character draft
// This is what gets stored in the database during character creation
type DraftData struct {
	// Identity
	ID       string `json:"id"`
	PlayerID string `json:"player_id"`

	// Basic info
	Name string `json:"name,omitempty"`

	// Core choices
	Race       races.Race             `json:"race,omitempty"`
	Subrace    races.Subrace          `json:"subrace,omitempty"`
	Class      classes.Class          `json:"class,omitempty"`
	Subclass   classes.Subclass       `json:"subclass,omitempty"`
	Background backgrounds.Background `json:"background,omitempty"`

	// Ability scores (before racial modifiers)
	BaseAbilityScores shared.AbilityScores `json:"base_ability_scores,omitempty"`

	// Player choices stored for validation
	Choices []choices.ChoiceData `json:"choices,omitempty"`

	// Progress tracking
	Progress Progress `json:"progress"`
}

// ToData converts a Draft to its serializable form
func (d *Draft) ToData() *DraftData {
	if d == nil {
		return nil
	}

	return &DraftData{
		ID:                d.id,
		PlayerID:          d.playerID,
		Name:              d.name,
		Race:              d.race,
		Subrace:           d.subrace,
		Class:             d.class,
		Subclass:          d.subclass,
		Background:        d.background,
		BaseAbilityScores: d.baseAbilityScores,
		Choices:           d.choices,
		Progress:          d.progress,
	}
}

// LoadDraftFromData creates a Draft from its serialized form
func LoadDraftFromData(data *DraftData) *Draft {
	if data == nil {
		return nil
	}

	return &Draft{
		id:                data.ID,
		playerID:          data.PlayerID,
		name:              data.Name,
		race:              data.Race,
		subrace:           data.Subrace,
		class:             data.Class,
		subclass:          data.Subclass,
		background:        data.Background,
		baseAbilityScores: data.BaseAbilityScores,
		choices:           data.Choices,
		progress:          data.Progress,
	}
}
