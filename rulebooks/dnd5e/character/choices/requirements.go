package choices

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// getClassRequirementsInternal returns requirements for a specific class at level 1
func getClassRequirementsInternal(classID classes.Class) *Requirements {
	switch classID {
	case classes.Fighter:
		return getFighterLevel1Requirements()
	case classes.Rogue:
		return getRogueLevel1Requirements()
	case classes.Wizard:
		return getWizardLevel1Requirements()
	case classes.Barbarian:
		return getBarbarianLevel1Requirements()
	case classes.Bard:
		return getBardLevel1Requirements()
	case classes.Cleric:
		return getClericLevel1Requirements()
	case classes.Druid:
		return getDruidLevel1Requirements()
	case classes.Monk:
		return getMonkLevel1Requirements()
	case classes.Paladin:
		return getPaladinLevel1Requirements()
	case classes.Ranger:
		return getRangerLevel1Requirements()
	case classes.Sorcerer:
		return getSorcererLevel1Requirements()
	case classes.Warlock:
		return getWarlockLevel1Requirements()
	default:
		return nil
	}
}

// Fighter Requirements
func getFighterLevel1Requirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			Count: 2,
			Options: []skills.Skill{
				skills.Acrobatics, skills.AnimalHandling, skills.Athletics, skills.History,
				skills.Insight, skills.Intimidation, skills.Perception, skills.Survival,
			},
			Label: "Choose 2 skills",
		},
		FightingStyle: &FightingStyleRequirement{
			Options: []FightingStyle{
				FightingStyleArchery, FightingStyleDefense, FightingStyleDueling, FightingStyleGreatWeaponFighting,
				FightingStyleProtection, FightingStyleTwoWeaponFighting,
			},
			Label: "Choose a fighting style",
		},
		Equipment: []*EquipmentRequirement{
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "chain-mail",
						Items: []ItemSpec{
							{Type: "armor", ID: string(ChainMail), Quantity: 1},
						},
						Label: "Chain mail",
					},
					{
						ID: "leather-armor-set",
						Items: []ItemSpec{
							{Type: "armor", ID: string(LeatherArmor), Quantity: 1},
							{Type: "weapon", ID: string(weapons.Longbow), Quantity: 1},
							{Type: "ammunition", ID: string(weapons.Arrows20), Quantity: 1},
						},
						Label: "Leather armor, longbow, and 20 arrows",
					},
				},
				Label: "Choose armor",
			},
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "martial-and-shield",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.AnyMartialWeapon), Quantity: 1},
							{Type: "armor", ID: string(Shield), Quantity: 1},
						},
						Label: "A martial weapon and a shield",
					},
					{
						ID: "two-martials",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.AnyMartialWeapon), Quantity: 2},
						},
						Label: "Two martial weapons",
					},
				},
				Label: "Choose weapons",
			},
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "light-crossbow",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.LightCrossbow), Quantity: 1},
							{Type: "ammunition", ID: string(weapons.Bolts20), Quantity: 1},
						},
						Label: "Light crossbow and 20 bolts",
					},
					{
						ID: "two-handaxes",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.Handaxe), Quantity: 2},
						},
						Label: "Two handaxes",
					},
				},
				Label: "Choose ranged weapon",
			},
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "dungeoneers-pack",
						Items: []ItemSpec{
							{Type: "pack", ID: string(DungeoneersPack), Quantity: 1},
						},
						Label: "Dungeoneer's pack",
					},
					{
						ID: "explorers-pack",
						Items: []ItemSpec{
							{Type: "pack", ID: string(ExplorersPack), Quantity: 1},
						},
						Label: "Explorer's pack",
					},
				},
				Label: "Choose equipment pack",
			},
		},
	}
}

// Rogue Requirements
func getRogueLevel1Requirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			Count: 4,
			Options: []skills.Skill{
				skills.Acrobatics, skills.Athletics, skills.Deception, skills.Insight, skills.Intimidation,
				skills.Investigation, skills.Perception, skills.Performance, skills.Persuasion,
				skills.SleightOfHand, skills.Stealth,
			},
			Label: "Choose 4 skills",
		},
		Expertise: &ExpertiseRequirement{
			Count: 2,
			Label: "Choose 2 skills or thieves' tools for expertise",
		},
		Equipment: []*EquipmentRequirement{
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "rapier",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.Rapier), Quantity: 1},
						},
						Label: "Rapier",
					},
					{
						ID: "shortsword",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.Shortsword), Quantity: 1},
						},
						Label: "Shortsword",
					},
				},
				Label: "Choose weapon",
			},
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "shortbow-set",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.Shortbow), Quantity: 1},
							{Type: "gear", ID: "quiver", Quantity: 1},
							{Type: "ammunition", ID: string(weapons.Arrows20), Quantity: 1},
						},
						Label: "Shortbow and quiver of 20 arrows",
					},
					{
						ID: "shortsword-2",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.Shortsword), Quantity: 1},
						},
						Label: "Shortsword",
					},
				},
				Label: "Choose secondary weapon",
			},
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "burglars-pack",
						Items: []ItemSpec{
							{Type: "pack", ID: string(BurglarsPack), Quantity: 1},
						},
						Label: "Burglar's pack",
					},
					{
						ID: "dungeoneers-pack",
						Items: []ItemSpec{
							{Type: "pack", ID: string(DungeoneersPack), Quantity: 1},
						},
						Label: "Dungeoneer's pack",
					},
					{
						ID: "explorers-pack",
						Items: []ItemSpec{
							{Type: "pack", ID: string(ExplorersPack), Quantity: 1},
						},
						Label: "Explorer's pack",
					},
				},
				Label: "Choose equipment pack",
			},
		},
	}
}

// Wizard Requirements
func getWizardLevel1Requirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			Count: 2,
			Options: []skills.Skill{
				skills.Arcana, skills.History, skills.Insight, skills.Investigation,
				skills.Medicine, skills.Religion,
			},
			Label: "Choose 2 skills",
		},
		Cantrips: &SpellRequirement{
			Count: 3,
			Level: 0,
			Label: "Choose 3 cantrips from the Wizard spell list",
		},
		Spells: &SpellRequirement{
			Count: 6,
			Level: 1,
			Label: "Choose 6 1st-level spells for your spellbook",
		},
		Equipment: []*EquipmentRequirement{
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "quarterstaff",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.Quarterstaff), Quantity: 1},
						},
						Label: "Quarterstaff",
					},
					{
						ID: "dagger",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.Dagger), Quantity: 1},
						},
						Label: "Dagger",
					},
				},
				Label: "Choose weapon",
			},
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "component-pouch",
						Items: []ItemSpec{
							{Type: "focus", ID: string(ComponentPouch), Quantity: 1},
						},
						Label: "Component pouch",
					},
					{
						ID: "arcane-focus",
						Items: []ItemSpec{
							{Type: "focus", ID: string(ArcaneFocus), Quantity: 1},
						},
						Label: "Arcane focus",
					},
				},
				Label: "Choose spellcasting focus",
			},
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "scholars-pack",
						Items: []ItemSpec{
							{Type: "pack", ID: string(ScholarsPack), Quantity: 1},
						},
						Label: "Scholar's pack",
					},
					{
						ID: "explorers-pack",
						Items: []ItemSpec{
							{Type: "pack", ID: string(ExplorersPack), Quantity: 1},
						},
						Label: "Explorer's pack",
					},
				},
				Label: "Choose equipment pack",
			},
		},
	}
}

// Barbarian Requirements
func getBarbarianLevel1Requirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			Count: 2,
			Options: []skills.Skill{
				skills.AnimalHandling, skills.Athletics, skills.Intimidation, skills.Nature,
				skills.Perception, skills.Survival,
			},
			Label: "Choose 2 skills",
		},
		Equipment: []*EquipmentRequirement{
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "greataxe",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.Greataxe), Quantity: 1},
						},
						Label: "Greataxe",
					},
					{
						ID: "any-martial",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.AnyMartialWeapon), Quantity: 1},
						},
						Label: "Any martial melee weapon",
					},
				},
				Label: "Choose weapon",
			},
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "two-handaxes",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.Handaxe), Quantity: 2},
						},
						Label: "Two handaxes",
					},
					{
						ID: "any-simple",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.AnySimpleWeapon), Quantity: 1},
						},
						Label: "Any simple weapon",
					},
				},
				Label: "Choose secondary weapon",
			},
		},
	}
}

// Bard Requirements
func getBardLevel1Requirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			Count:   3,
			Options: nil, // Bard can choose ANY skills
			Label:   "Choose any 3 skills",
		},
		Cantrips: &SpellRequirement{
			Count: 2,
			Level: 0,
			Label: "Choose 2 cantrips from the Bard spell list",
		},
		Spells: &SpellRequirement{
			Count: 4,
			Level: 1,
			Label: "Choose 4 1st-level spells from the Bard spell list",
		},
		Instruments: &InstrumentRequirement{
			Count:   3,
			Options: nil, // Can choose any instruments
			Label:   "Choose 3 musical instruments",
		},
		Equipment: []*EquipmentRequirement{
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "rapier",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.Rapier), Quantity: 1},
						},
						Label: "Rapier",
					},
					{
						ID: "longsword",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.Longsword), Quantity: 1},
						},
						Label: "Longsword",
					},
					{
						ID: "any-simple",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.AnySimpleWeapon), Quantity: 1},
						},
						Label: "Any simple weapon",
					},
				},
				Label: "Choose weapon",
			},
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "diplomats-pack",
						Items: []ItemSpec{
							{Type: "pack", ID: string(DiplomatsPack), Quantity: 1},
						},
						Label: "Diplomat's pack",
					},
					{
						ID: "entertainers-pack",
						Items: []ItemSpec{
							{Type: "pack", ID: string(EntertainersPack), Quantity: 1},
						},
						Label: "Entertainer's pack",
					},
				},
				Label: "Choose equipment pack",
			},
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "lute",
						Items: []ItemSpec{
							{Type: "instrument", ID: string(Lute), Quantity: 1},
						},
						Label: "Lute",
					},
					{
						ID: "any-instrument",
						Items: []ItemSpec{
							{Type: "instrument", ID: string(AnyInstrument), Quantity: 1},
						},
						Label: "Any other musical instrument",
					},
				},
				Label: "Choose musical instrument",
			},
		},
	}
}

// Cleric Requirements
func getClericLevel1Requirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			Count: 2,
			Options: []skills.Skill{
				skills.History, skills.Insight, skills.Medicine, skills.Persuasion, skills.Religion,
			},
			Label: "Choose 2 skills",
		},
		Cantrips: &SpellRequirement{
			Count: 3,
			Level: 0,
			Label: "Choose 3 cantrips from the Cleric spell list",
		},
		Subclass: &SubclassRequirement{
			Options: []classes.Subclass{
				classes.LifeDomain,
				classes.LightDomain,
				classes.NatureDomain,
				classes.TempestDomain,
				classes.TrickeryDomain,
				classes.WarDomain,
				classes.KnowledgeDomain,
				classes.DeathDomain,
			},
			Label: "Choose your Divine Domain",
		},
		// Note: Clerics prepare spells, don't learn them
		Equipment: []*EquipmentRequirement{
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "mace",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.Mace), Quantity: 1},
						},
						Label: "Mace",
					},
					{
						ID: "warhammer",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.Warhammer), Quantity: 1},
						},
						Label: "Warhammer (if proficient)",
					},
				},
				Label: "Choose weapon",
			},
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "scale-mail",
						Items: []ItemSpec{
							{Type: "armor", ID: string(ScaleMail), Quantity: 1},
						},
						Label: "Scale mail",
					},
					{
						ID: "leather-armor",
						Items: []ItemSpec{
							{Type: "armor", ID: string(LeatherArmor), Quantity: 1},
						},
						Label: "Leather armor",
					},
					{
						ID: "chain-mail",
						Items: []ItemSpec{
							{Type: "armor", ID: string(ChainMail), Quantity: 1},
						},
						Label: "Chain mail (if proficient)",
					},
				},
				Label: "Choose armor",
			},
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "light-crossbow-set",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.LightCrossbow), Quantity: 1},
							{Type: "ammunition", ID: string(weapons.Bolts20), Quantity: 1},
						},
						Label: "Light crossbow and 20 bolts",
					},
					{
						ID: "any-simple",
						Items: []ItemSpec{
							{Type: "weapon", ID: string(weapons.AnySimpleWeapon), Quantity: 1},
						},
						Label: "Any simple weapon",
					},
				},
				Label: "Choose secondary weapon",
			},
			{
				Choose: 1,
				Options: []EquipmentOption{
					{
						ID: "priests-pack",
						Items: []ItemSpec{
							{Type: "pack", ID: string(PriestsPack), Quantity: 1},
						},
						Label: "Priest's pack",
					},
					{
						ID: "explorers-pack",
						Items: []ItemSpec{
							{Type: "pack", ID: string(ExplorersPack), Quantity: 1},
						},
						Label: "Explorer's pack",
					},
				},
				Label: "Choose equipment pack",
			},
		},
	}
}

// Additional class requirements...
func getDruidLevel1Requirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			Count: 2,
			Options: []skills.Skill{
				skills.Arcana, skills.AnimalHandling, skills.Insight, skills.Medicine,
				skills.Nature, skills.Perception, skills.Religion, skills.Survival,
			},
			Label: "Choose 2 skills",
		},
		Cantrips: &SpellRequirement{
			Count: 2,
			Level: 0,
			Label: "Choose 2 cantrips from the Druid spell list",
		},
	}
}

func getMonkLevel1Requirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			Count: 2,
			Options: []skills.Skill{
				skills.Acrobatics, skills.Athletics, skills.History, skills.Insight,
				skills.Religion, skills.Stealth,
			},
			Label: "Choose 2 skills",
		},
	}
}

func getPaladinLevel1Requirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			Count: 2,
			Options: []skills.Skill{
				skills.Athletics, skills.Insight, skills.Intimidation, skills.Medicine,
				skills.Persuasion, skills.Religion,
			},
			Label: "Choose 2 skills",
		},
	}
}

func getRangerLevel1Requirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			Count: 3,
			Options: []skills.Skill{
				skills.AnimalHandling, skills.Athletics, skills.Insight, skills.Investigation,
				skills.Nature, skills.Perception, skills.Stealth, skills.Survival,
			},
			Label: "Choose 3 skills",
		},
	}
}

func getSorcererLevel1Requirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			Count: 2,
			Options: []skills.Skill{
				skills.Arcana, skills.Deception, skills.Insight, skills.Intimidation,
				skills.Persuasion, skills.Religion,
			},
			Label: "Choose 2 skills",
		},
		Subclass: &SubclassRequirement{
			Options: []classes.Subclass{
				classes.DraconicBloodline,
				classes.WildMagic,
				classes.DivineSoul,
			},
			Label: "Choose your Sorcerous Origin",
		},
		Cantrips: &SpellRequirement{
			Count: 4,
			Level: 0,
			Label: "Choose 4 cantrips from the Sorcerer spell list",
		},
		Spells: &SpellRequirement{
			Count: 2,
			Level: 1,
			Label: "Choose 2 1st-level spells from the Sorcerer spell list",
		},
	}
}

// GetSubclassRequirements returns the complete requirements for a subclass (base + subclass specific)
func GetSubclassRequirements(subclassID classes.Subclass) *Requirements {
	// Get base class requirements
	baseReqs := GetClassRequirements(subclassID.Parent())
	if baseReqs == nil {
		return nil
	}

	// Remove the subclass requirement since it's already been chosen
	baseReqs.Subclass = nil

	// Add subclass-specific requirements
	switch subclassID {
	// Cleric subclasses
	case classes.KnowledgeDomain:
		// Knowledge Domain gets 2 extra languages and 2 extra skills
		baseReqs.Languages = &LanguageRequirement{
			Count: 2,
			Label: "Choose 2 languages (Knowledge Domain)",
		}
		// Add extra skill choices from specific list
		if baseReqs.Skills == nil {
			baseReqs.Skills = &SkillRequirement{}
		}
		// Knowledge Domain adds 2 skills from: Arcana, History, Nature, or Religion
		baseReqs.Skills.Count += 2
		// Add the Knowledge Domain specific skills to the allowed options
		knowledgeSkills := []skills.Skill{
			skills.Arcana,
			skills.History,
			skills.Nature,
			skills.Religion,
		}
		// Merge the knowledge domain skills with existing options
		for _, skill := range knowledgeSkills {
			// Check if skill is not already in the list
			found := false
			for _, existing := range baseReqs.Skills.Options {
				if existing == skill {
					found = true
					break
				}
			}
			if !found {
				baseReqs.Skills.Options = append(baseReqs.Skills.Options, skill)
			}
		}
		baseReqs.Skills.Label = "Choose 2 skills (base) + 2: Arcana/History/Nature/Religion (Knowledge Domain)"

	case classes.NatureDomain:
		// Nature Domain gets one druid cantrip and proficiency in one of: Animal Handling, Nature, or Survival
		if baseReqs.Cantrips != nil {
			baseReqs.Cantrips.Count++
			baseReqs.Cantrips.Label = "Choose 3 Cleric cantrips + 1 Druid cantrip (Nature Domain)"
		}
		// Add skill proficiency choice
		if baseReqs.Skills == nil {
			baseReqs.Skills = &SkillRequirement{}
		}
		baseReqs.Skills.Count += 1
		// Add Nature Domain specific skill options
		natureDomainSkills := []skills.Skill{
			skills.AnimalHandling,
			skills.Nature,
			skills.Survival,
		}
		// Merge with existing options
		for _, skill := range natureDomainSkills {
			found := false
			for _, existing := range baseReqs.Skills.Options {
				if existing == skill {
					found = true
					break
				}
			}
			if !found {
				baseReqs.Skills.Options = append(baseReqs.Skills.Options, skill)
			}
		}
		baseReqs.Skills.Label = "Choose 2 skills (base) + 1: Animal Handling/Nature/Survival (Nature Domain)"

	case classes.TrickeryDomain:
		// No additional requirements, but gets specific domain spells (handled elsewhere)

	// Most subclasses don't add requirements
	default:
		// No additional requirements
	}

	return baseReqs
}

func getWarlockLevel1Requirements() *Requirements {
	return &Requirements{
		Skills: &SkillRequirement{
			Count: 2,
			Options: []skills.Skill{
				skills.Arcana, skills.Deception, skills.History, skills.Intimidation,
				skills.Investigation, skills.Nature, skills.Religion,
			},
			Label: "Choose 2 skills",
		},
		Subclass: &SubclassRequirement{
			Options: []classes.Subclass{
				classes.Archfey,
				classes.Fiend,
				classes.GreatOldOne,
				classes.Hexblade,
			},
			Label: "Choose your Otherworldly Patron",
		},
		Cantrips: &SpellRequirement{
			Count: 2,
			Level: 0,
			Label: "Choose 2 cantrips from the Warlock spell list",
		},
		Spells: &SpellRequirement{
			Count: 2,
			Level: 1,
			Label: "Choose 2 1st-level spells from the Warlock spell list",
		},
	}
}

// Race Requirements
func getRaceRequirementsInternal(raceID races.Race) *Requirements {
	switch raceID {
	case races.HalfElf:
		return &Requirements{
			Skills: &SkillRequirement{
				Count:   2,
				Options: nil, // Can choose any skills
				Label:   "Choose any 2 skills",
			},
			Languages: &LanguageRequirement{
				Count:   1,
				Options: nil, // Can choose any language
				Label:   "Choose 1 additional language",
			},
		}
	case races.Dragonborn:
		return &Requirements{
			DraconicAncestry: &AncestryRequirement{
				Options: []AncestryID{
					AncestryBlack, AncestryBlue, AncestryBrass, AncestryBronze, AncestryCopper,
					AncestryGold, AncestryGreen, AncestryRed, Ancestrysilver, AncestryWhite,
				},
				Label: "Choose your draconic ancestry",
			},
		}
	case races.Elf:
		// High Elf subrace has choices
		// TODO: Handle subraces
		return nil
	default:
		// Most races have no choices
		return nil
	}
}

// Background Requirements
func getBackgroundRequirementsInternal(_ backgrounds.Background) *Requirements {
	// Most backgrounds have no choices, they just grant skills/tools/languages
	// Some backgrounds like Guild Artisan let you choose a guild
	// TODO: Add backgrounds with choices
	return nil
}

// mergeRequirements combines multiple requirement sets
func mergeRequirements(reqs ...*Requirements) *Requirements {
	if len(reqs) == 0 {
		return nil
	}

	// Start with the first non-nil requirements
	var merged *Requirements
	for _, r := range reqs {
		if r != nil {
			merged = &Requirements{
				Skills:                  r.Skills,
				Cantrips:                r.Cantrips,
				Spells:                  r.Spells,
				Equipment:               r.Equipment,
				Languages:               r.Languages,
				Tools:                   r.Tools,
				Instruments:             r.Instruments,
				FightingStyle:           r.FightingStyle,
				Expertise:               r.Expertise,
				DraconicAncestry:        r.DraconicAncestry,
				AbilityScoreImprovement: r.AbilityScoreImprovement,
				Feat:                    r.Feat,
			}
			break
		}
	}

	if merged == nil {
		return nil
	}

	// Merge additional requirements
	// For now, we just take the first non-nil value for each field
	// In the future, we might need more complex merging logic
	for _, r := range reqs[1:] {
		if r == nil {
			continue
		}
		if merged.Skills == nil && r.Skills != nil {
			merged.Skills = r.Skills
		}
		if merged.Languages == nil && r.Languages != nil {
			merged.Languages = r.Languages
		}
		if merged.DraconicAncestry == nil && r.DraconicAncestry != nil {
			merged.DraconicAncestry = r.DraconicAncestry
		}
		// Add more fields as needed
	}

	return merged
}
