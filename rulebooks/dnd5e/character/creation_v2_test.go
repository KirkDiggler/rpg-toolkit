package character

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/suite"
)

type CreationV2Suite struct {
	suite.Suite
	bus events.EventBus
}

func (s *CreationV2Suite) SetupTest() {
	s.bus = events.NewEventBus()
}

func TestCreationV2Suite(t *testing.T) {
	suite.Run(t, new(CreationV2Suite))
}

func (s *CreationV2Suite) TestLoadFromDef() {
	// Create a character def using refs
	def := &CharacterDef{
		ID:       "char_001",
		PlayerID: "player_001",
		Name:     "Thorin",
		Level:    5,

		// Core identity as refs
		RaceRef:       "dnd5e:race:dwarf",
		SubraceRef:    "dnd5e:subrace:mountain_dwarf",
		ClassRef:      "dnd5e:class:barbarian",
		BackgroundRef: "dnd5e:background:soldier",

		// Ability scores
		AbilityScores: shared.AbilityScores{
			constants.STR: 18,
			constants.DEX: 14,
			constants.CON: 16,
			constants.INT: 10,
			constants.WIS: 12,
			constants.CHA: 8,
		},

		// Combat stats
		HitPoints:    45,
		MaxHitPoints: 45,

		// Features as refs
		Features: []string{
			"dnd5e:features:rage",
			"dnd5e:features:reckless_attack",
			"dnd5e:features:danger_sense",
		},

		// Skills as refs
		Skills: []string{
			"dnd5e:skill:athletics",
			"dnd5e:skill:intimidation",
			"dnd5e:skill:survival",
		},

		// Languages as refs
		Languages: []string{
			"dnd5e:language:common",
			"dnd5e:language:dwarvish",
		},

		// Proficiencies as refs
		Proficiencies: []string{
			"dnd5e:proficiency:simple_weapons",
			"dnd5e:proficiency:martial_weapons",
			"dnd5e:proficiency:light_armor",
			"dnd5e:proficiency:medium_armor",
			"dnd5e:proficiency:shields",
		},

		// Equipment as refs
		Equipment: []string{
			"dnd5e:item:greataxe",
			"dnd5e:item:handaxe",
			"dnd5e:item:handaxe",
			"dnd5e:item:explorer_pack",
		},

		// Choices made during creation
		Choices: []CharacterChoice{
			{
				Ref:    "dnd5e:choice:barbarian_skills",
				Source: "dnd5e:class:barbarian",
				Selected: []string{
					"dnd5e:skill:athletics",
					"dnd5e:skill:intimidation",
				},
				Timestamp: "2025-01-01T10:00:00Z",
			},
			{
				Ref:    "dnd5e:choice:background_language",
				Source: "dnd5e:background:soldier",
				Selected: []string{
					"dnd5e:language:orcish",
				},
				Timestamp: "2025-01-01T10:01:00Z",
			},
		},
	}

	// Load character from def
	char, err := LoadFromDef(def, s.bus)
	s.Require().NoError(err)
	s.NotNil(char)

	// Verify basic properties
	s.Equal("char_001", char.id)
	s.Equal("player_001", char.playerID)
	s.Equal("Thorin", char.name)
	s.Equal(5, char.level)

	// Verify identity converted correctly
	s.Equal(constants.Race("dwarf"), char.raceID)
	s.Equal(constants.Class("barbarian"), char.classID)
	s.Equal(constants.Background("soldier"), char.backgroundID)

	// Verify skills
	s.Equal(shared.Proficient, char.skills[constants.SkillAthletics])
	s.Equal(shared.Proficient, char.skills[constants.SkillIntimidation])
	s.Equal(shared.Proficient, char.skills[constants.SkillSurvival])

	// Verify languages
	s.Contains(char.languages, constants.LanguageCommon)
	s.Contains(char.languages, constants.LanguageDwarvish)

	// Verify proficiencies
	s.Contains(char.proficiencies.Weapons, "simple_weapons")
	s.Contains(char.proficiencies.Weapons, "martial_weapons")
	s.Contains(char.proficiencies.Armor, "light_armor")
	s.Contains(char.proficiencies.Armor, "medium_armor")
	s.Contains(char.proficiencies.Armor, "shields")

	// Verify features stored as JSON
	s.Len(char.features, 3)

	// Verify equipment
	s.Contains(char.equipment, "dnd5e:item:greataxe")
	s.Contains(char.equipment, "dnd5e:item:handaxe")
	s.Contains(char.equipment, "dnd5e:item:explorer_pack")

	// Verify choices were preserved
	s.Len(char.choices, 2)
}

func (s *CreationV2Suite) TestLoadCharacterDefFromJSON() {
	// JSON representation using refs
	jsonData := `{
		"id": "char_002",
		"player_id": "player_002",
		"name": "Elara",
		"level": 1,
		"race_ref": "dnd5e:race:elf",
		"subrace_ref": "dnd5e:subrace:high_elf",
		"class_ref": "dnd5e:class:wizard",
		"background_ref": "dnd5e:background:sage",
		"ability_scores": {
			"Strength": 8,
			"Dexterity": 16,
			"Constitution": 14,
			"Intelligence": 17,
			"Wisdom": 13,
			"Charisma": 10
		},
		"hit_points": 8,
		"max_hit_points": 8,
		"features": [
			"dnd5e:features:arcane_recovery",
			"dnd5e:features:cantrip_formulas"
		],
		"skills": [
			"dnd5e:skill:arcana",
			"dnd5e:skill:history",
			"dnd5e:skill:investigation"
		],
		"languages": [
			"dnd5e:language:common",
			"dnd5e:language:elvish",
			"dnd5e:language:draconic",
			"dnd5e:language:dwarvish"
		],
		"proficiencies": [
			"dnd5e:proficiency:daggers",
			"dnd5e:proficiency:darts",
			"dnd5e:proficiency:slings",
			"dnd5e:proficiency:quarterstaffs",
			"dnd5e:proficiency:light_crossbows"
		],
		"equipment": [
			"dnd5e:item:quarterstaff",
			"dnd5e:item:component_pouch",
			"dnd5e:item:scholars_pack",
			"dnd5e:item:spellbook"
		],
		"choices": [
			{
				"ref": "dnd5e:choice:wizard_skills",
				"source": "dnd5e:class:wizard",
				"selected": ["dnd5e:skill:arcana", "dnd5e:skill:investigation"],
				"timestamp": "2025-01-02T14:00:00Z"
			}
		],
		"conditions": []
	}`

	// Load character from JSON
	char, err := LoadCharacterDef([]byte(jsonData), s.bus)
	s.Require().NoError(err)
	s.NotNil(char)

	// Verify it loaded correctly
	s.Equal("char_002", char.id)
	s.Equal("Elara", char.name)
	s.Equal(constants.Class("wizard"), char.classID)
	s.Equal(constants.Race("elf"), char.raceID)

	// Verify skills loaded
	s.Equal(shared.Proficient, char.skills[constants.SkillArcana])
	s.Equal(shared.Proficient, char.skills[constants.SkillHistory])
	s.Equal(shared.Proficient, char.skills[constants.SkillInvestigation])

	// Verify multiple languages
	s.Len(char.languages, 4)
	s.Contains(char.languages, constants.LanguageElvish)
	s.Contains(char.languages, constants.LanguageDraconic)
}

func (s *CreationV2Suite) TestToDef() {
	// Create a character the old way
	char := &Character{
		id:       "char_003",
		playerID: "player_003",
		name:     "Gimli",
		level:    3,

		raceID:       constants.Race("dwarf"),
		classID:      constants.Class("fighter"),
		backgroundID: constants.Background("soldier"),

		abilityScores: shared.AbilityScores{
			constants.STR: 17,
			constants.DEX: 13,
			constants.CON: 16,
			constants.INT: 10,
			constants.WIS: 12,
			constants.CHA: 8,
		},

		hitPoints:    28,
		maxHitPoints: 28,

		skills: map[constants.Skill]shared.ProficiencyLevel{
			constants.SkillAthletics:     shared.Proficient,
			constants.SkillIntimidation:  shared.Proficient,
			constants.SkillPerception:    shared.Proficient,
		},

		languages: []constants.Language{
			constants.LanguageCommon,
			constants.LanguageDwarvish,
		},

		proficiencies: shared.Proficiencies{
			Armor:   []string{"light_armor", "medium_armor", "heavy_armor", "shields"},
			Weapons: []string{"simple_weapons", "martial_weapons"},
			Tools:   []string{"smiths_tools"},
		},

		equipment: []string{
			"dnd5e:item:longsword",
			"dnd5e:item:shield",
			"dnd5e:item:chain_mail",
		},

		features: []json.RawMessage{
			json.RawMessage(`{"ref":"dnd5e:features:fighting_style","id":"feature_fighting_style"}`),
			json.RawMessage(`{"ref":"dnd5e:features:second_wind","id":"feature_second_wind"}`),
		},

		conditions: []json.RawMessage{},
	}

	// Convert to def
	def := char.ToDef()
	s.NotNil(def)

	// Verify conversion
	s.Equal("char_003", def.ID)
	s.Equal("player_003", def.PlayerID)
	s.Equal("Gimli", def.Name)
	s.Equal(3, def.Level)

	// Verify refs were created correctly
	s.Equal("dnd5e:race:dwarf", def.RaceRef)
	s.Equal("dnd5e:class:fighter", def.ClassRef)
	s.Equal("dnd5e:background:soldier", def.BackgroundRef)

	// Verify skills converted to refs
	s.Contains(def.Skills, "dnd5e:skill:athletics")
	s.Contains(def.Skills, "dnd5e:skill:intimidation")
	s.Contains(def.Skills, "dnd5e:skill:perception")

	// Verify languages converted to refs
	s.Contains(def.Languages, "dnd5e:language:common")
	s.Contains(def.Languages, "dnd5e:language:dwarvish")

	// Verify proficiencies converted to refs
	s.Contains(def.Proficiencies, "dnd5e:proficiency:simple_weapons")
	s.Contains(def.Proficiencies, "dnd5e:proficiency:martial_weapons")
	s.Contains(def.Proficiencies, "dnd5e:proficiency:heavy_armor")
	s.Contains(def.Proficiencies, "dnd5e:proficiency:smiths_tools")

	// Verify features extracted from JSON
	s.Contains(def.Features, "dnd5e:features:fighting_style")
	s.Contains(def.Features, "dnd5e:features:second_wind")

	// Verify equipment preserved
	s.Contains(def.Equipment, "dnd5e:item:longsword")
	s.Contains(def.Equipment, "dnd5e:item:shield")
	s.Contains(def.Equipment, "dnd5e:item:chain_mail")
}

func (s *CreationV2Suite) TestRoundTripConversion() {
	// Original def
	original := &CharacterDef{
		ID:       "char_004",
		PlayerID: "player_004",
		Name:     "Legolas",
		Level:    8,

		RaceRef:       "dnd5e:race:elf",
		SubraceRef:    "dnd5e:subrace:wood_elf",
		ClassRef:      "dnd5e:class:ranger",
		BackgroundRef: "dnd5e:background:outlander",

		AbilityScores: shared.AbilityScores{
			constants.STR: 13,
			constants.DEX: 18,
			constants.CON: 14,
			constants.INT: 12,
			constants.WIS: 16,
			constants.CHA: 10,
		},

		HitPoints:    60,
		MaxHitPoints: 60,

		Features: []string{
			"dnd5e:features:favored_enemy",
			"dnd5e:features:natural_explorer",
			"dnd5e:features:fighting_style",
			"dnd5e:features:primeval_awareness",
		},

		Skills: []string{
			"dnd5e:skill:athletics",
			"dnd5e:skill:insight",
			"dnd5e:skill:perception",
			"dnd5e:skill:stealth",
			"dnd5e:skill:survival",
		},

		Languages: []string{
			"dnd5e:language:common",
			"dnd5e:language:elvish",
			"dnd5e:language:orcish",
			"dnd5e:language:goblin",
		},

		Proficiencies: []string{
			"dnd5e:proficiency:simple_weapons",
			"dnd5e:proficiency:martial_weapons",
			"dnd5e:proficiency:light_armor",
			"dnd5e:proficiency:medium_armor",
			"dnd5e:proficiency:shields",
		},

		Equipment: []string{
			"dnd5e:item:longbow",
			"dnd5e:item:shortsword",
			"dnd5e:item:shortsword",
			"dnd5e:item:studded_leather",
		},

		Choices: []CharacterChoice{
			{
				Ref:    "dnd5e:choice:ranger_skills",
				Source: "dnd5e:class:ranger",
				Selected: []string{
					"dnd5e:skill:athletics",
					"dnd5e:skill:insight",
					"dnd5e:skill:stealth",
				},
				Timestamp: "2025-01-03T09:00:00Z",
			},
		},
	}

	// Convert to character
	char, err := LoadFromDef(original, s.bus)
	s.Require().NoError(err)

	// Convert back to def
	roundtrip := char.ToDef()

	// Verify core properties match
	s.Equal(original.ID, roundtrip.ID)
	s.Equal(original.PlayerID, roundtrip.PlayerID)
	s.Equal(original.Name, roundtrip.Name)
	s.Equal(original.Level, roundtrip.Level)

	// Verify refs match
	s.Equal(original.RaceRef, roundtrip.RaceRef)
	s.Equal(original.ClassRef, roundtrip.ClassRef)
	s.Equal(original.BackgroundRef, roundtrip.BackgroundRef)

	// Verify collections have same content (order may differ)
	s.ElementsMatch(original.Skills, roundtrip.Skills)
	s.ElementsMatch(original.Languages, roundtrip.Languages)
	s.ElementsMatch(original.Proficiencies, roundtrip.Proficiencies)
	s.ElementsMatch(original.Features, roundtrip.Features)
	s.ElementsMatch(original.Equipment, roundtrip.Equipment)
}