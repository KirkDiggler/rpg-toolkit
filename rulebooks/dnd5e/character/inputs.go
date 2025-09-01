package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// SetNameInput contains the input for setting a character's name
type SetNameInput struct {
	Name string `json:"name"`
}

// SetRaceInput contains the input for setting a character's race
type SetRaceInput struct {
	RaceID    races.Race    `json:"race_id"`
	SubraceID races.Subrace `json:"subrace_id,omitempty"`
	Choices   RaceChoices   `json:"choices,omitempty"`
}

// RaceChoices contains optional choices when selecting a race
type RaceChoices struct {
	Languages []languages.Language `json:"languages,omitempty"`
	Skills    []skills.Skill       `json:"skills,omitempty"`
	Cantrips  []spells.Spell       `json:"cantrips,omitempty"`
}

// SetClassInput contains the input for setting a character's class
type SetClassInput struct {
	ClassID    classes.Class    `json:"class_id"`
	SubclassID classes.Subclass `json:"subclass_id,omitempty"`
	Choices    ClassChoices     `json:"choices"`
}

// ClassChoices contains choices when selecting a class
type ClassChoices struct {
	Skills        []skills.Skill               `json:"skills"`
	FightingStyle fightingstyles.FightingStyle `json:"fighting_style,omitempty"`
	Cantrips      []spells.Spell               `json:"cantrips,omitempty"`
	Spells        []spells.Spell               `json:"spells,omitempty"`
}

// SetBackgroundInput contains the input for setting a character's background
type SetBackgroundInput struct {
	BackgroundID backgrounds.Background `json:"background_id"`
	Choices      BackgroundChoices      `json:"choices,omitempty"`
}

// BackgroundChoices contains optional choices when selecting a background
type BackgroundChoices struct {
	Languages []languages.Language `json:"languages,omitempty"`
}

// SetAbilityScoresInput contains the input for setting ability scores
type SetAbilityScoresInput struct {
	Scores shared.AbilityScores `json:"scores"`
	Method string               `json:"method"` // "standard", "point_buy", "rolled"
}