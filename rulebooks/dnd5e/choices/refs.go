package choices

import "github.com/KirkDiggler/rpg-toolkit/core"

// Ref creates a choice reference from a choice type
func Ref(choiceType string) *core.Ref {
	return core.MustNewRef(core.RefInput{
		Module: "dnd5e",
		Type:   "choice",
		Value:  choiceType,
	})
}

// Common choice refs as constants
var (
	// Core character creation choices
	Race          = Ref("race")
	Class         = Ref("class")
	Background    = Ref("background")
	AbilityScores = Ref("ability_scores")
	Name          = Ref("name")

	// Class-specific skill choices
	BarbarianSkills = Ref("barbarian_skills")
	BardSkills      = Ref("bard_skills")
	ClericSkills    = Ref("cleric_skills")
	DruidSkills     = Ref("druid_skills")
	FighterSkills   = Ref("fighter_skills")
	MonkSkills      = Ref("monk_skills")
	PaladinSkills   = Ref("paladin_skills")
	RangerSkills    = Ref("ranger_skills")
	RogueSkills     = Ref("rogue_skills")
	SorcererSkills  = Ref("sorcerer_skills")
	WarlockSkills   = Ref("warlock_skills")
	WizardSkills    = Ref("wizard_skills")

	// Fighting styles
	FighterFightingStyle = Ref("fighter_fighting_style")
	PaladinFightingStyle = Ref("paladin_fighting_style")
	RangerFightingStyle  = Ref("ranger_fighting_style")

	// Spells and cantrips
	ClericCantrips  = Ref("cleric_cantrips")
	DruidCantrips   = Ref("druid_cantrips")
	SorcererCantrips = Ref("sorcerer_cantrips")
	WarlockCantrips = Ref("warlock_cantrips")
	WizardCantrips  = Ref("wizard_cantrips")
	
	WizardSpells    = Ref("wizard_spells")
	ClericSpells    = Ref("cleric_spells")
	
	// Equipment choices
	FighterEquipment = Ref("fighter_equipment")
	WizardEquipment  = Ref("wizard_equipment")
	
	// Language choices
	HumanLanguage     = Ref("human_language")
	HalfElfLanguages  = Ref("half_elf_languages")
)