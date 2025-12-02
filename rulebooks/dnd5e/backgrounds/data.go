package backgrounds

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Data contains the mechanical data for a background
type Data struct {
	ID Background // The background this data represents

	// Skill proficiencies
	SkillCount int            // Number of skills to choose
	Skills     []skills.Skill // Available skills to choose from

	// Languages
	LanguageCount int // Number of languages to choose

	// Equipment
	// TODO: Add equipment grants when equipment system is ready

	// Feature
	Feature string // The background feature name
	// TODO: Could expand this to a full Feature type later
}

// BackgroundData maps backgrounds to their mechanical data
var BackgroundData = map[Background]*Data{
	Acolyte: {
		ID:            Acolyte,
		SkillCount:    2,
		Skills:        []skills.Skill{skills.Insight, skills.Religion},
		LanguageCount: 2,
		Feature:       "Shelter of the Faithful",
	},

	Criminal: {
		ID:            Criminal,
		SkillCount:    2,
		Skills:        []skills.Skill{skills.Deception, skills.Stealth},
		LanguageCount: 0,
		Feature:       "Criminal Contact",
	},

	FolkHero: {
		ID:            FolkHero,
		SkillCount:    2,
		Skills:        []skills.Skill{skills.AnimalHandling, skills.Survival},
		LanguageCount: 0,
		Feature:       "Rustic Hospitality",
	},

	Noble: {
		ID:            Noble,
		SkillCount:    2,
		Skills:        []skills.Skill{skills.History, skills.Persuasion},
		LanguageCount: 1,
		Feature:       "Position of Privilege",
	},

	Sage: {
		ID:            Sage,
		SkillCount:    2,
		Skills:        []skills.Skill{skills.Arcana, skills.History},
		LanguageCount: 2,
		Feature:       "Researcher",
	},

	Soldier: {
		ID:            Soldier,
		SkillCount:    2,
		Skills:        []skills.Skill{skills.Athletics, skills.Intimidation},
		LanguageCount: 0,
		Feature:       "Military Rank",
	},

	Charlatan: {
		ID:            Charlatan,
		SkillCount:    2,
		Skills:        []skills.Skill{skills.Deception, skills.SleightOfHand},
		LanguageCount: 0,
		Feature:       "False Identity",
	},

	Entertainer: {
		ID:            Entertainer,
		SkillCount:    2,
		Skills:        []skills.Skill{skills.Acrobatics, skills.Performance},
		LanguageCount: 0,
		Feature:       "By Popular Demand",
	},

	GuildArtisan: {
		ID:            GuildArtisan,
		SkillCount:    2,
		Skills:        []skills.Skill{skills.Insight, skills.Persuasion},
		LanguageCount: 1,
		Feature:       "Guild Membership",
	},

	Hermit: {
		ID:            Hermit,
		SkillCount:    2,
		Skills:        []skills.Skill{skills.Medicine, skills.Religion},
		LanguageCount: 1,
		Feature:       "Discovery",
	},

	Outlander: {
		ID:            Outlander,
		SkillCount:    2,
		Skills:        []skills.Skill{skills.Athletics, skills.Survival},
		LanguageCount: 1,
		Feature:       "Wanderer",
	},

	Sailor: {
		ID:            Sailor,
		SkillCount:    2,
		Skills:        []skills.Skill{skills.Athletics, skills.Perception},
		LanguageCount: 0,
		Feature:       "Ship's Passage",
	},

	Urchin: {
		ID:            Urchin,
		SkillCount:    2,
		Skills:        []skills.Skill{skills.SleightOfHand, skills.Stealth},
		LanguageCount: 0,
		Feature:       "City Secrets",
	},

	// Variants share data with their base backgrounds
	Spy: {
		ID:            Spy,
		SkillCount:    2,
		Skills:        []skills.Skill{skills.Deception, skills.Stealth},
		LanguageCount: 0,
		Feature:       "Spy Contact", // Different feature than Criminal
	},

	Pirate: {
		SkillCount:    2,
		Skills:        []skills.Skill{skills.Athletics, skills.Perception},
		LanguageCount: 0,
		Feature:       "Bad Reputation", // Different feature than Sailor
	},

	Knight: {
		SkillCount:    2,
		Skills:        []skills.Skill{skills.History, skills.Persuasion},
		LanguageCount: 1,
		Feature:       "Retainers", // Different feature than Noble
	},

	GuildMerchant: {
		SkillCount:    2,
		Skills:        []skills.Skill{skills.Insight, skills.Persuasion},
		LanguageCount: 1,
		Feature:       "Guild Membership", // Same as Guild Artisan
	},
}

// GetData returns the mechanical data for a background
func GetData(bg Background) *Data {
	if data, ok := BackgroundData[bg]; ok {
		return data
	}
	return nil
}

// Name returns the display name of the background
func (d *Data) Name() string {
	return Name(d.ID)
}

// Description returns the description of the background
func (d *Data) Description() string {
	return Description(d.ID)
}
