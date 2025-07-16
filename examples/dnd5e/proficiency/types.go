// Package proficiency implements D&D 5e proficiency system using rpg-toolkit
package proficiency

// ProficiencyType represents different types of proficiencies in D&D 5e
type ProficiencyType string

const (
	// Core proficiency types
	ProficiencyTypeArmor       ProficiencyType = "armor"
	ProficiencyTypeWeapon      ProficiencyType = "weapon"
	ProficiencyTypeTool        ProficiencyType = "tool"
	ProficiencyTypeSavingThrow ProficiencyType = "saving-throw"
	ProficiencyTypeSkill       ProficiencyType = "skill"
	ProficiencyTypeLanguage    ProficiencyType = "language"
)

// WeaponCategory represents weapon proficiency categories
type WeaponCategory string

const (
	WeaponCategorySimple  WeaponCategory = "simple"
	WeaponCategoryMartial WeaponCategory = "martial"
)

// ArmorCategory represents armor proficiency categories
type ArmorCategory string

const (
	ArmorCategoryLight   ArmorCategory = "light"
	ArmorCategoryMedium  ArmorCategory = "medium"
	ArmorCategoryHeavy   ArmorCategory = "heavy"
	ArmorCategoryShields ArmorCategory = "shields"
)

// Skill represents D&D 5e skills
type Skill string

const (
	// Skills by attribute
	SkillAcrobatics     Skill = "acrobatics"      // DEX
	SkillAnimalHandling Skill = "animal-handling" // WIS
	SkillArcana         Skill = "arcana"          // INT
	SkillAthletics      Skill = "athletics"       // STR
	SkillDeception      Skill = "deception"       // CHA
	SkillHistory        Skill = "history"         // INT
	SkillInsight        Skill = "insight"         // WIS
	SkillIntimidation   Skill = "intimidation"    // CHA
	SkillInvestigation  Skill = "investigation"   // INT
	SkillMedicine       Skill = "medicine"        // WIS
	SkillNature         Skill = "nature"          // INT
	SkillPerception     Skill = "perception"      // WIS
	SkillPerformance    Skill = "performance"     // CHA
	SkillPersuasion     Skill = "persuasion"      // CHA
	SkillReligion       Skill = "religion"        // INT
	SkillSleightOfHand  Skill = "sleight-of-hand" // DEX
	SkillStealth        Skill = "stealth"         // DEX
	SkillSurvival       Skill = "survival"        // WIS
)

// SavingThrow represents D&D 5e saving throw types
type SavingThrow string

const (
	SavingThrowStrength     SavingThrow = "strength"
	SavingThrowDexterity    SavingThrow = "dexterity"
	SavingThrowConstitution SavingThrow = "constitution"
	SavingThrowIntelligence SavingThrow = "intelligence"
	SavingThrowWisdom       SavingThrow = "wisdom"
	SavingThrowCharisma     SavingThrow = "charisma"
)

// GetSkillAttribute returns which attribute a skill uses
func GetSkillAttribute(skill Skill) string {
	skillAttributes := map[Skill]string{
		SkillAcrobatics:     "dexterity",
		SkillAnimalHandling: "wisdom",
		SkillArcana:         "intelligence",
		SkillAthletics:      "strength",
		SkillDeception:      "charisma",
		SkillHistory:        "intelligence",
		SkillInsight:        "wisdom",
		SkillIntimidation:   "charisma",
		SkillInvestigation:  "intelligence",
		SkillMedicine:       "wisdom",
		SkillNature:         "intelligence",
		SkillPerception:     "wisdom",
		SkillPerformance:    "charisma",
		SkillPersuasion:     "charisma",
		SkillReligion:       "intelligence",
		SkillSleightOfHand:  "dexterity",
		SkillStealth:        "dexterity",
		SkillSurvival:       "wisdom",
	}
	return skillAttributes[skill]
}

// GetProficiencyBonus calculates the D&D 5e proficiency bonus for a given level
func GetProficiencyBonus(level int) int {
	if level < 1 {
		level = 1
	}
	return 2 + ((level - 1) / 4)
}
