package character

import (
	"context"
	"encoding/json"
	"time"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/customization"
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

	// Core attributes.
	//
	// Both are PROJECTIONS of Levels, not truth (design §2 R2.2). ToData
	// writes them from the record so the wire and every existing reader are
	// unaffected; nothing in the toolkit reads them back as the character's
	// level. Two representations that could disagree must not, so LoadCharacter
	// refuses a sheet whose Level does not match its record (R2.3).
	Level            int `json:"level"`
	ProficiencyBonus int `json:"proficiency_bonus"`

	// Levels is the record of how this character reached its current level, in
	// order. Levels[0] is level 1. This is the source of truth for the
	// character's level (R2.2).
	Levels []LevelEntry `json:"levels"`

	// Experience is the total experience this character has earned.
	//
	// Cumulative and never debited (design R4.8), and read-only over the wire
	// (R4.12): nothing in the toolkit awards it yet and no service call writes
	// it, so a fixture seeds a levelled character by writing this field on the
	// persisted sheet. The level it entitles the character to is DERIVED from
	// it and never stored — see [Character.EntitledLevel].
	//
	// A sheet written before experience existed loads with zero, and load does
	// not re-check entitlement against it (R4.12a): Advance enforces the rule
	// at the moment a level is taken, and a later change to the threshold table
	// must not refuse every stored sheet that was legal when it was written.
	Experience int `json:"experience"`

	// Race and class
	RaceID     races.Race       `json:"race_id"`
	SubraceID  races.Subrace    `json:"subrace_id,omitempty"`
	ClassID    classes.Class    `json:"class_id"`
	SubclassID classes.Subclass `json:"subclass_id,omitempty"`

	// BackgroundData
	BackgroundID backgrounds.Background `json:"background_id"`

	// Appearance
	Appearance *customization.Appearance `json:"appearance,omitempty"`

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
	Inventory []InventoryItemData `json:"inventory"`

	// Wallet is the character's coin purse — a field beside Inventory, not
	// embedded in it (rpg-toolkit#1275: "gold should not be modeled as a
	// regular inventory item for purchasing power"). Zero value (Money{}) is
	// legal and means broke, not "never had a wallet" — there is no separate
	// presence flag to distinguish the two, matching every other zero-value
	// numeric field on this struct.
	Wallet currency.Money `json:"wallet"`

	EquipmentSlots EquipmentSlots `json:"equipment_slots,omitempty"`

	// KnownCantrips and KnownSpells are what this character knows, as
	// canonical content refs ("dnd5e:spells:vicious-mockery"), written by the
	// choice pipeline at creation.
	//
	// REFS RATHER THAN NAMES OR ENUM VALUES, because a known spell is content
	// and this sheet holds an identity for it rather than a copy of it
	// (rpg-project#391 §5.2). They are authorization facts rather than copied
	// behavior: the compiler that mints Cast declarations reads them and emits
	// only spells with complete definitions.
	//
	// Two fields rather than one keyed by level, because the two are chosen
	// separately, counted separately by every class table, and refilled by
	// different rules — a cantrip is never forgotten and a known spell can be
	// swapped on level-up. One list would need a level beside every entry to
	// answer a question the split already answers.
	KnownCantrips  []string                                              `json:"known_cantrips,omitempty"`
	KnownSpells    []string                                              `json:"known_spells,omitempty"`
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

// HitPointMethod names how a level's hit point gain was produced.
//
// Stored on the entry rather than re-derived, because a roll cannot be rolled
// again: the same call authored wall runs already make. Everything else a
// level did is derived from the entry plus the current rules, so correcting a
// rule corrects every character (design §7.1).
type HitPointMethod string

const (
	// HitPointMethodMax takes the full hit die. Level 1 only — every later
	// level rolls or averages.
	HitPointMethodMax HitPointMethod = "max"

	// HitPointMethodRolled rolls the class hit die.
	HitPointMethodRolled HitPointMethod = "rolled"

	// HitPointMethodAverage takes the class's fixed average for its hit die,
	// which is half the die plus one (PHB p.15).
	HitPointMethodAverage HitPointMethod = "average"
)

// LevelEntry records one level this character has taken. Append-only.
//
// It holds the INPUTS to that level, never the effects: what a level granted
// is derived from the entry plus the current rules, so a corrected rule
// corrects every character. A record of effects reads better and rots — the
// day a grant is fixed, every stored account of it becomes a confident,
// permanent description of something that should not have happened, with no
// way to reconcile it against the sheet. See design §7.1.
type LevelEntry struct {
	// Level is the character level this entry produced. Entry n has Level n+1.
	Level int `json:"level"`

	// ClassID is the class the level was taken in. Always equal to the
	// character's class until multiclassing exists (R2.4).
	ClassID classes.Class `json:"class_id"`

	// HitPointGain is the hit points this level added, and the method that
	// produced it. Stored because a roll cannot be re-derived.
	HitPointGain   int            `json:"hit_point_gain"`
	HitPointMethod HitPointMethod `json:"hit_point_method"`

	// Choices are the choices this level required, if any. Empty for a level
	// that requires none — which is every level 2 of the four SRD classes.
	Choices []choices.ChoiceData `json:"choices,omitempty"`
}

// InventoryItemData represents serializable inventory item
type InventoryItemData struct {
	Type     shared.EquipmentType `json:"type"` // weapon, armor, tool, pack, item, ammunition
	ID       string               `json:"id"`   // The specific item ID (e.g., "longsword", "leather_armor")
	Quantity int                  `json:"quantity"`
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
// This path is forgiving where Load is strict: a condition, feature, unknown
// inventory item, inventory row with a nonpositive quantity, or character-owned
// resource with malformed bounds is dropped and the load continues, which is
// the behaviour every existing caller has. Malformed quantities are never
// defaulted to one, and malformed resource counts are never clamped into a
// valid-looking entry. Read that as a warning
// rather than a feature — it means a sheet can come back from a round trip
// missing something nobody removed (rpg-toolkit#948). Load refuses instead,
// and names the blob.
func LoadFromData(ctx context.Context, d *Data, bus events.EventBus) (*Character, error) {
	return LoadFromDataWithRoller(ctx, d, bus, nil)
}

// LoadFromDataWithRoller is [LoadFromData] with the runtime dice roller the
// interaction owns: the sheet is built and attached exactly as [LoadFromData]
// builds and attaches it, and the roller is offered to every condition that
// accepts one, so the lenient path reaches the interaction's randomness
// without reimplementing Character loading policy. Read [AttachWithRoller]
// for what supplying a roller does and does not change; nil is
// [LoadFromData] exactly.
func LoadFromDataWithRoller(
	ctx context.Context, d *Data, bus events.EventBus, roller dice.Roller,
) (*Character, error) {
	if bus == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}

	char, err := loadSheet(d, lenientEffects)
	if err != nil {
		return nil, err
	}

	if err := AttachWithRoller(ctx, char, bus, roller); err != nil {
		return nil, err
	}

	return char, nil
}
