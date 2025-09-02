package character

import (
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
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

	// Background
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
	Languages    []string                                      `json:"languages"`

	// Equipment and resources
	Inventory      []InventoryItemData                       `json:"inventory"`
	SpellSlots     map[int]SpellSlotData                     `json:"spell_slots,omitempty"`
	ClassResources map[shared.ClassResourceType]ResourceData `json:"class_resources,omitempty"`

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

// ToCharacter converts the data to a domain Character
func (d *Data) ToCharacter() *Character {
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

	// Convert languages (stored as strings) back to typed constants
	// TODO: Implement proper language string to constant conversion

	// Convert spell slots
	char.spellSlots = make(map[int]SpellSlot)
	for level, slot := range d.SpellSlots {
		char.spellSlots[level] = SpellSlot{
			Max:  slot.Max,
			Used: slot.Used,
		}
	}

	// Convert class resources
	char.classResources = make(map[shared.ClassResourceType]Resource)
	for resourceType, resource := range d.ClassResources {
		char.classResources[resourceType] = Resource{
			Name:    resource.Name,
			Max:     resource.Max,
			Current: resource.Current,
			Resets:  resource.Resets,
		}
	}

	return char
}

// FromCharacter creates Data from a domain Character
func FromCharacter(c *Character) *Data {
	data := &Data{
		ID:               c.id,
		PlayerID:         c.playerID,
		Name:             c.name,
		Level:            c.level,
		ProficiencyBonus: c.proficiencyBonus,
		RaceID:           c.raceID,
		SubraceID:        c.subraceID,
		ClassID:          c.classID,
		SubclassID:       c.subclassID,
		AbilityScores:    c.abilityScores,
		HitPoints:        c.hitPoints,
		MaxHitPoints:     c.maxHitPoints,
		ArmorClass:       c.armorClass,
		Skills:           c.skills,
		SavingThrows:     c.savingThrows,
		UpdatedAt:        time.Now(),
	}

	// Convert inventory to data
	data.Inventory = make([]InventoryItemData, 0, len(c.inventory))
	for _, item := range c.inventory {
		data.Inventory = append(data.Inventory, InventoryItemData{
			Type:     item.Equipment.EquipmentType(),
			ID:       item.Equipment.EquipmentID(),
			Quantity: item.Quantity,
		})
	}

	// Convert languages to strings
	// TODO: Convert typed language constants to strings

	// Convert spell slots
	data.SpellSlots = make(map[int]SpellSlotData)
	for level, slot := range c.spellSlots {
		data.SpellSlots[level] = SpellSlotData{
			Max:  slot.Max,
			Used: slot.Used,
		}
	}

	// Convert class resources
	data.ClassResources = make(map[shared.ClassResourceType]ResourceData)
	for resourceType, resource := range c.classResources {
		data.ClassResources[resourceType] = ResourceData{
			Name:    resource.Name,
			Max:     resource.Max,
			Current: resource.Current,
			Resets:  resource.Resets,
		}
	}

	return data
}
