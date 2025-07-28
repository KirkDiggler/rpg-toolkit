package constants

// Skill represents a D&D 5e skill
type Skill string

// Skill constants
const (
	SkillAcrobatics     Skill = "acrobatics"
	SkillAnimalHandling Skill = "animal-handling"
	SkillArcana         Skill = "arcana"
	SkillAthletics      Skill = "athletics"
	SkillDeception      Skill = "deception"
	SkillHistory        Skill = "history"
	SkillInsight        Skill = "insight"
	SkillIntimidation   Skill = "intimidation"
	SkillInvestigation  Skill = "investigation"
	SkillMedicine       Skill = "medicine"
	SkillNature         Skill = "nature"
	SkillPerception     Skill = "perception"
	SkillPerformance    Skill = "performance"
	SkillPersuasion     Skill = "persuasion"
	SkillReligion       Skill = "religion"
	SkillSleightOfHand  Skill = "sleight-of-hand"
	SkillStealth        Skill = "stealth"
	SkillSurvival       Skill = "survival"
)

// Display returns the human-readable name of the skill
func (s Skill) Display() string {
	switch s {
	case SkillAcrobatics:
		return "Acrobatics"
	case SkillAnimalHandling:
		return "Animal Handling"
	case SkillArcana:
		return "Arcana"
	case SkillAthletics:
		return "Athletics"
	case SkillDeception:
		return "Deception"
	case SkillHistory:
		return "History"
	case SkillInsight:
		return "Insight"
	case SkillIntimidation:
		return "Intimidation"
	case SkillInvestigation:
		return "Investigation"
	case SkillMedicine:
		return "Medicine"
	case SkillNature:
		return "Nature"
	case SkillPerception:
		return "Perception"
	case SkillPerformance:
		return "Performance"
	case SkillPersuasion:
		return "Persuasion"
	case SkillReligion:
		return "Religion"
	case SkillSleightOfHand:
		return "Sleight of Hand"
	case SkillStealth:
		return "Stealth"
	case SkillSurvival:
		return "Survival"
	default:
		return string(s)
	}
}

// Ability returns the ability score this skill is based on
func (s Skill) Ability() Ability {
	switch s {
	case SkillAthletics:
		return STR
	case SkillAcrobatics, SkillSleightOfHand, SkillStealth:
		return DEX
	case SkillArcana, SkillHistory, SkillInvestigation, SkillNature, SkillReligion:
		return INT
	case SkillAnimalHandling, SkillInsight, SkillMedicine, SkillPerception, SkillSurvival:
		return WIS
	case SkillDeception, SkillIntimidation, SkillPerformance, SkillPersuasion:
		return CHA
	default:
		return ""
	}
}

// AllSkills returns all skills in alphabetical order
func AllSkills() []Skill {
	return []Skill{
		SkillAcrobatics,
		SkillAnimalHandling,
		SkillArcana,
		SkillAthletics,
		SkillDeception,
		SkillHistory,
		SkillInsight,
		SkillIntimidation,
		SkillInvestigation,
		SkillMedicine,
		SkillNature,
		SkillPerception,
		SkillPerformance,
		SkillPersuasion,
		SkillReligion,
		SkillSleightOfHand,
		SkillStealth,
		SkillSurvival,
	}
}
