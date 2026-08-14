package character

import (
	"context"
	"encoding/json"
	"time"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
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

	// Action economy state (nil outside combat)
	ActionEconomy *ActionEconomyData `json:"action_economy,omitempty"`

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

// LoadFromData creates a Character from persistent data and puts it on the bus.
//
// It is [Load] followed by [Attach], and nothing else. The two halves are
// separately callable — a sheet loaded with Load exists without a bus at all —
// and this signature stays for the callers that have it, until
// rpg-toolkit#965 and #966 retire the verb methods that need the character to
// be holding a bus of its own.
//
// This path is forgiving where Load is strict: a condition, feature, or
// inventory item it cannot make sense of is dropped and the load continues,
// which is the behaviour every existing caller has. Read that as a warning
// rather than a feature — it means a sheet can come back from a round trip
// missing something nobody removed (rpg-toolkit#948). Load refuses instead,
// and names the blob.
func LoadFromData(ctx context.Context, d *Data, bus events.EventBus) (*Character, error) {
	if bus == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}

	char, err := loadSheet(d, lenientEffects)
	if err != nil {
		return nil, err
	}

	if err := Attach(ctx, char, bus); err != nil {
		return nil, err
	}

	return char, nil
}
