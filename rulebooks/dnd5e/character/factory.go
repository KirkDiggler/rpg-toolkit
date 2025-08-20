// Package character provides D&D 5e character creation and management functionality
package character

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// CharacterFactory provides simple character creation without the complexity of Draft/Builder
type CharacterFactory struct {
	bus events.EventBus
}

// NewCharacterFactory creates a new character factory
func NewCharacterFactory(bus events.EventBus) *CharacterFactory {
	return &CharacterFactory{
		bus: bus,
	}
}

// CreateBarbarianInput contains the data needed to create a barbarian
type CreateBarbarianInput struct {
	ID            string
	PlayerID      string
	Name          string
	AbilityScores shared.AbilityScores
	SkillChoices  []string // Skill refs like "dnd5e:skill:athletics"
	Background    string   // Background ref like "dnd5e:background:soldier"
	Equipment     []string // Equipment refs
}

// CreateBarbarian creates a level 1 barbarian character
func (f *CharacterFactory) CreateBarbarian(ctx context.Context, input CreateBarbarianInput) (*Character, error) {
	// Build the character def
	def := &CharacterDef{
		ID:            input.ID,
		PlayerID:      input.PlayerID,
		Name:          input.Name,
		Level:         1,
		ClassRef:      "dnd5e:class:barbarian",
		RaceRef:       "dnd5e:race:human", // Default, would be specified
		BackgroundRef: input.Background,
		AbilityScores: input.AbilityScores,

		// Barbarian features at level 1
		Features: []string{
			"dnd5e:features:rage",
			"dnd5e:features:unarmored_defense",
		},

		// Skills from choices plus any background skills
		Skills: input.SkillChoices,

		// Standard barbarian proficiencies
		Proficiencies: []string{
			"dnd5e:proficiency:simple_weapons",
			"dnd5e:proficiency:martial_weapons",
			"dnd5e:proficiency:light_armor",
			"dnd5e:proficiency:medium_armor",
			"dnd5e:proficiency:shields",
		},

		// Equipment
		Equipment: input.Equipment,

		// Calculate HP (12 + CON modifier)
		MaxHitPoints: 12 + input.AbilityScores.Modifier(constants.CON),
		HitPoints:    12 + input.AbilityScores.Modifier(constants.CON),

		// Record the choices made
		Choices: []CharacterChoice{
			{
				Ref:      "dnd5e:choice:barbarian_skills",
				Source:   "dnd5e:class:barbarian",
				Selected: input.SkillChoices,
			},
		},
	}

	// Load the character
	return LoadFromDef(def, f.bus)
}

// CreateWizardInput contains the data needed to create a wizard
type CreateWizardInput struct {
	ID            string
	PlayerID      string
	Name          string
	AbilityScores shared.AbilityScores
	SkillChoices  []string // Skill refs
	Background    string   // Background ref
	Cantrips      []string // Cantrip spell refs
	Spells        []string // 1st level spell refs
}

// CreateWizard creates a level 1 wizard character
func (f *CharacterFactory) CreateWizard(ctx context.Context, input CreateWizardInput) (*Character, error) {
	// Build the character def
	def := &CharacterDef{
		ID:            input.ID,
		PlayerID:      input.PlayerID,
		Name:          input.Name,
		Level:         1,
		ClassRef:      "dnd5e:class:wizard",
		RaceRef:       "dnd5e:race:elf", // Default high elf for INT bonus
		BackgroundRef: input.Background,
		AbilityScores: input.AbilityScores,

		// Wizard features at level 1
		Features: []string{
			"dnd5e:features:arcane_recovery",
			"dnd5e:features:spellcasting",
		},

		// Skills from choices
		Skills: input.SkillChoices,

		// Wizard proficiencies (limited)
		Proficiencies: []string{
			"dnd5e:proficiency:daggers",
			"dnd5e:proficiency:darts",
			"dnd5e:proficiency:slings",
			"dnd5e:proficiency:quarterstaffs",
			"dnd5e:proficiency:light_crossbows",
		},

		// Standard wizard equipment
		Equipment: []string{
			"dnd5e:item:quarterstaff",
			"dnd5e:item:component_pouch",
			"dnd5e:item:scholars_pack",
			"dnd5e:item:spellbook",
		},

		// Calculate HP (6 + CON modifier)
		MaxHitPoints: 6 + input.AbilityScores.Modifier(constants.CON),
		HitPoints:    6 + input.AbilityScores.Modifier(constants.CON),

		// Record choices
		Choices: []CharacterChoice{
			{
				Ref:      "dnd5e:choice:wizard_skills",
				Source:   "dnd5e:class:wizard",
				Selected: input.SkillChoices,
			},
			{
				Ref:      "dnd5e:choice:wizard_cantrips",
				Source:   "dnd5e:class:wizard",
				Selected: input.Cantrips,
			},
			{
				Ref:      "dnd5e:choice:wizard_spells",
				Source:   "dnd5e:class:wizard",
				Selected: input.Spells,
			},
		},
	}

	// Load the character
	return LoadFromDef(def, f.bus)
}

// QuickCreateInput provides minimal data for quick character creation
type QuickCreateInput struct {
	Name  string
	Class string // "barbarian", "wizard", "fighter", etc.
}

// QuickCreate creates a character with sensible defaults
func (f *CharacterFactory) QuickCreate(ctx context.Context, input QuickCreateInput) (*Character, error) {
	// Generate a simple ID
	id := fmt.Sprintf("char_%s_%s", input.Class, input.Name)

	switch input.Class {
	case "barbarian":
		return f.CreateBarbarian(ctx, CreateBarbarianInput{
			ID:       id,
			PlayerID: "quick_player",
			Name:     input.Name,
			AbilityScores: shared.AbilityScores{
				constants.STR: 16,
				constants.DEX: 14,
				constants.CON: 15,
				constants.INT: 10,
				constants.WIS: 12,
				constants.CHA: 8,
			},
			SkillChoices: []string{
				"dnd5e:skill:athletics",
				"dnd5e:skill:intimidation",
			},
			Background: "dnd5e:background:soldier",
			Equipment: []string{
				"dnd5e:item:greataxe",
				"dnd5e:item:handaxe",
				"dnd5e:item:handaxe",
				"dnd5e:item:explorer_pack",
			},
		})

	case "wizard":
		return f.CreateWizard(ctx, CreateWizardInput{
			ID:       id,
			PlayerID: "quick_player",
			Name:     input.Name,
			AbilityScores: shared.AbilityScores{
				constants.STR: 8,
				constants.DEX: 14,
				constants.CON: 14,
				constants.INT: 16,
				constants.WIS: 13,
				constants.CHA: 10,
			},
			SkillChoices: []string{
				"dnd5e:skill:arcana",
				"dnd5e:skill:investigation",
			},
			Background: "dnd5e:background:sage",
			Cantrips: []string{
				"dnd5e:spell:fire_bolt",
				"dnd5e:spell:mage_hand",
				"dnd5e:spell:prestidigitation",
			},
			Spells: []string{
				"dnd5e:spell:magic_missile",
				"dnd5e:spell:shield",
				"dnd5e:spell:detect_magic",
				"dnd5e:spell:identify",
				"dnd5e:spell:sleep",
				"dnd5e:spell:burning_hands",
			},
		})

	default:
		return nil, fmt.Errorf("unknown class: %s", input.Class)
	}
}