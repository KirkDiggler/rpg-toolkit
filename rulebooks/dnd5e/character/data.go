package character

import (
	"context"
	"encoding/json"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
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

	// Death saves (only persisted if character is at 0 HP making death saves)
	DeathSaveState *saves.DeathSaveState `json:"death_save_state,omitempty"`

	// Proficiencies and skills
	Skills              map[skills.Skill]shared.ProficiencyLevel      `json:"skills"`
	SavingThrows        map[abilities.Ability]shared.ProficiencyLevel `json:"saving_throws"`
	Languages           []languages.Language                          `json:"languages"`
	ArmorProficiencies  []proficiencies.Armor                         `json:"armor_proficiencies"`
	WeaponProficiencies []proficiencies.Weapon                        `json:"weapon_proficiencies"`
	ToolProficiencies   []proficiencies.Tool                          `json:"tool_proficiencies"`

	// Equipment and resources
	Inventory      []InventoryItemData                                   `json:"inventory"`
	EquipmentSlots EquipmentSlots                                        `json:"equipment_slots,omitempty"`
	SpellSlots     map[int]SpellSlotData                                 `json:"spell_slots,omitempty"`
	ClassResources map[shared.ClassResourceType]ResourceData             `json:"class_resources,omitempty"`
	Resources      map[coreResources.ResourceKey]RecoverableResourceData `json:"resources,omitempty"`

	// Features (rage, second wind, etc)
	Features []json.RawMessage `json:"features,omitempty"`

	// Conditions (raging, poisoned, stunned, etc)
	Conditions []json.RawMessage `json:"conditions,omitempty"`

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

// RecoverableResourceData represents serializable recoverable resource state
type RecoverableResourceData struct {
	Current   int                     `json:"current"`
	Maximum   int                     `json:"maximum"`
	ResetType coreResources.ResetType `json:"reset_type"`
}

// LoadFromData creates a Character from persistent data
func LoadFromData(ctx context.Context, d *Data, bus events.EventBus) (*Character, error) {
	if bus == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}

	char := &Character{
		id:                  d.ID,
		playerID:            d.PlayerID,
		name:                d.Name,
		level:               d.Level,
		proficiencyBonus:    d.ProficiencyBonus,
		raceID:              d.RaceID,
		subraceID:           d.SubraceID,
		classID:             d.ClassID,
		subclassID:          d.SubclassID,
		abilityScores:       d.AbilityScores,
		hitPoints:           d.HitPoints,
		maxHitPoints:        d.MaxHitPoints,
		armorClass:          d.ArmorClass,
		deathSaveState:      d.DeathSaveState,
		skills:              d.Skills,
		savingThrows:        d.SavingThrows,
		languages:           d.Languages,
		armorProficiencies:  d.ArmorProficiencies,
		weaponProficiencies: d.WeaponProficiencies,
		toolProficiencies:   d.ToolProficiencies,
		equipmentSlots:      d.EquipmentSlots,
		bus:                 bus,
		subscriptionIDs:     make([]string, 0),
		resources:           make(map[coreResources.ResourceKey]*combat.RecoverableResource),
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

	// Load conditions from persisted JSON data (following same pattern as features)
	char.conditions = make([]dnd5eEvents.ConditionBehavior, 0, len(d.Conditions))
	for _, rawCondition := range d.Conditions {
		// Load the actual condition implementation
		condition, err := conditions.LoadJSON(rawCondition)
		if err != nil {
			// Log error but continue loading other conditions
			// TODO: Consider how to handle condition loading errors
			continue
		}

		// Re-apply the condition so it subscribes to events
		if err := condition.Apply(ctx, bus); err != nil {
			// Clean up any partial subscriptions to avoid resource leaks
			_ = condition.Remove(ctx, bus)
			// Log error but continue loading other conditions
			// TODO: Consider how to handle condition apply errors
			continue
		}

		char.conditions = append(char.conditions, condition)
	}

	// Load resources from persisted data
	for key, resData := range d.Resources {
		resource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          string(key),
			Maximum:     resData.Maximum,
			CharacterID: char.id,
			ResetType:   resData.ResetType,
		})

		// Set current value if different from maximum
		if resData.Current != resData.Maximum {
			deficit := resData.Maximum - resData.Current
			_ = resource.Use(deficit) // Ignore error - we know the value is valid
		}

		// Apply resource to subscribe to rest events
		if err := resource.Apply(ctx, bus); err != nil {
			// Clean up on failure
			_ = resource.Remove(ctx, bus)
			// Log error but continue loading other resources
			// TODO: Consider how to handle resource apply errors
			continue
		}

		char.resources[key] = resource
	}

	// Subscribe to events - character comes out fully initialized
	if err := char.subscribeToEvents(ctx); err != nil {
		return nil, rpgerr.Wrapf(err, "failed to subscribe to events")
	}

	return char, nil
}
