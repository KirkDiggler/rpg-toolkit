// Package skills provides D&D 5e skill constants and utilities
package skills

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
)

// Skill represents a D&D 5e skill
type Skill string

// All D&D 5e skills
const (
	Acrobatics     Skill = "acrobatics"
	AnimalHandling Skill = "animal-handling"
	Arcana         Skill = "arcana"
	Athletics      Skill = "athletics"
	Deception      Skill = "deception"
	History        Skill = "history"
	Insight        Skill = "insight"
	Intimidation   Skill = "intimidation"
	Investigation  Skill = "investigation"
	Medicine       Skill = "medicine"
	Nature         Skill = "nature"
	Perception     Skill = "perception"
	Performance    Skill = "performance"
	Persuasion     Skill = "persuasion"
	Religion       Skill = "religion"
	SleightOfHand  Skill = "sleight-of-hand"
	Stealth        Skill = "stealth"
	Survival       Skill = "survival"
)

// All contains all skills mapped by ID for O(1) lookup
var All = map[string]Skill{
	"acrobatics":      Acrobatics,
	"animal-handling": AnimalHandling,
	"arcana":          Arcana,
	"athletics":       Athletics,
	"deception":       Deception,
	"history":         History,
	"insight":         Insight,
	"intimidation":    Intimidation,
	"investigation":   Investigation,
	"medicine":        Medicine,
	"nature":          Nature,
	"perception":      Perception,
	"performance":     Performance,
	"persuasion":      Persuasion,
	"religion":        Religion,
	"sleight-of-hand": SleightOfHand,
	"stealth":         Stealth,
	"survival":        Survival,
}

// GetByID returns a skill by its ID
func GetByID(id string) (Skill, error) {
	skill, ok := All[id]
	if !ok {
		validSkills := make([]string, 0, len(All))
		for k := range All {
			validSkills = append(validSkills, k)
		}
		return "", rpgerr.New(rpgerr.CodeInvalidArgument, "invalid skill",
			rpgerr.WithMeta("provided", id),
			rpgerr.WithMeta("valid_options", validSkills))
	}
	return skill, nil
}

// List returns all skills in alphabetical order
func List() []Skill {
	return []Skill{
		Acrobatics,
		AnimalHandling,
		Arcana,
		Athletics,
		Deception,
		History,
		Insight,
		Intimidation,
		Investigation,
		Medicine,
		Nature,
		Perception,
		Performance,
		Persuasion,
		Religion,
		SleightOfHand,
		Stealth,
		Survival,
	}
}

// Ability returns the ability score this skill is based on
func (s Skill) Ability() abilities.Ability {
	switch s {
	case Athletics:
		return abilities.STR
	case Acrobatics, SleightOfHand, Stealth:
		return abilities.DEX
	case Arcana, History, Investigation, Nature, Religion:
		return abilities.INT
	case AnimalHandling, Insight, Medicine, Perception, Survival:
		return abilities.WIS
	case Deception, Intimidation, Performance, Persuasion:
		return abilities.CHA
	default:
		return ""
	}
}

// Display returns the human-readable name of the skill
func (s Skill) Display() string {
	switch s {
	case Acrobatics:
		return "Acrobatics"
	case AnimalHandling:
		return "Animal Handling"
	case Arcana:
		return "Arcana"
	case Athletics:
		return "Athletics"
	case Deception:
		return "Deception"
	case History:
		return "History"
	case Insight:
		return "Insight"
	case Intimidation:
		return "Intimidation"
	case Investigation:
		return "Investigation"
	case Medicine:
		return "Medicine"
	case Nature:
		return "Nature"
	case Perception:
		return "Perception"
	case Performance:
		return "Performance"
	case Persuasion:
		return "Persuasion"
	case Religion:
		return "Religion"
	case SleightOfHand:
		return "Sleight of Hand"
	case Stealth:
		return "Stealth"
	case Survival:
		return "Survival"
	default:
		return string(s)
	}
}

// ByAbility returns all skills that use the given ability
func ByAbility(ability abilities.Ability) []Skill {
	var result []Skill
	for _, skill := range List() {
		if skill.Ability() == ability {
			result = append(result, skill)
		}
	}
	return result
}
