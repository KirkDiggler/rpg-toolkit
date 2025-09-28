package character

import (
	"encoding/json"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Data represents the serializable form of a character
// This is what gets stored in the database
type Data struct {
	// Identity
	ID       string `json:"id"`
	PlayerID string `json:"player_id"`
	Name     string `json:"name"`

	// Core attributes
	Level            int `json:"level"`
	ProficiencyBonus int `json:"proficiency_bonus"`

	// Race and class
	RaceID     races.Race       `json:"race_id"`
	SubraceID  races.Subrace    `json:"subrace_id,omitempty"`
	ClassID    classes.Class    `json:"class_id"`
	SubclassID classes.Subclass `json:"subclass_id,omitempty"`

	// BackgroundData
	BackgroundID backgrounds.Background `json:"background_id"`

	// Ability scores (final values including racial modifiers)
	AbilityScores shared.AbilityScores `json:"ability_scores"`

	// Combat stats
	HitPoints    int `json:"hit_points"`
	MaxHitPoints int `json:"max_hit_points"`
	ArmorClass   int `json:"armor_class"`

	// Proficiencies and skills
	Skills       map[skills.Skill]shared.ProficiencyLevel      `json:"skills"`
	SavingThrows map[abilities.Ability]shared.ProficiencyLevel `json:"saving_throws"`
	Languages    []languages.Language                          `json:"languages"`

	// Equipment and resources
	Inventory      []InventoryItemData                       `json:"inventory"`
	SpellSlots     map[int]SpellSlotData                     `json:"spell_slots,omitempty"`
	ClassResources map[shared.ClassResourceType]ResourceData `json:"class_resources,omitempty"`

	// Features (rage, second wind, etc)
	Features []json.RawMessage `json:"features,omitempty"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// InventoryItemData represents serializable inventory item
type InventoryItemData struct {
	Type     shared.EquipmentType `json:"type"` // weapon, armor, tool, pack, item, ammunition
	ID       string               `json:"id"`   // The specific item ID (e.g., "longsword", "leather_armor")
	Quantity int                  `json:"quantity"`
}

// SpellSlotData represents serializable spell slot info
type SpellSlotData struct {
	Max  int `json:"max"`
	Used int `json:"used"`
}

// ResourceData represents serializable class resource info
type ResourceData struct {
	Name    string           `json:"name"`
	Max     int              `json:"max"`
	Current int              `json:"current"`
	Resets  shared.ResetType `json:"resets"`
}

// LoadFromData creates a Character from persistent data
func LoadFromData(d *Data) (*Character, error) {
	char := &Character{
		id:               d.ID,
		playerID:         d.PlayerID,
		name:             d.Name,
		level:            d.Level,
		proficiencyBonus: d.ProficiencyBonus,
		raceID:           d.RaceID,
		subraceID:        d.SubraceID,
		classID:          d.ClassID,
		subclassID:       d.SubclassID,
		abilityScores:    d.AbilityScores,
		hitPoints:        d.HitPoints,
		maxHitPoints:     d.MaxHitPoints,
		armorClass:       d.ArmorClass,
		skills:           d.Skills,
		savingThrows:     d.SavingThrows,
	}

	// Get hit dice from class data
	if classData := classes.GetData(d.ClassID); classData != nil {
		char.hitDice = classData.HitDice
	}

	// Convert inventory data back to Equipment items
	char.inventory = make([]InventoryItem, 0, len(d.Inventory))
	for _, itemData := range d.Inventory {
		// Use the unified GetByID function
		equip, err := equipment.GetByID(itemData.ID)
		if err != nil {
			// Log error but continue loading other items
			// TODO: Consider how to handle missing equipment
			continue
		}
		char.inventory = append(char.inventory, InventoryItem{
			Equipment: equip,
			Quantity:  itemData.Quantity,
		})
	}

	// Load features from persisted JSON data
	char.features = make([]features.Feature, 0, len(d.Features))
	for _, rawFeature := range d.Features {
		// Peek at the ref to check module
		var peek struct {
			Ref core.Ref `json:"ref"`
		}
		if err := json.Unmarshal(rawFeature, &peek); err != nil {
			// Skip malformed features
			continue
		}

		// Check if this is a dnd5e feature (for now, only module we support)
		if peek.Ref.Module == "dnd5e" {
			// Load the actual feature implementation
			feature, err := features.LoadJSON(rawFeature)
			if err != nil {
				// Log error but continue loading other features
				// TODO: Consider how to handle feature loading errors
				continue
			}
			char.features = append(char.features, feature)
		}
		// Silently skip non-dnd5e features for now
		// In the future, this would route to a module registry
	}

	return char, nil
}
