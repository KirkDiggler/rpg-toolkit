package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
)

// Data is the persisted shape of an Encounter. The orchestrator
// stores this in Redis (or any KV store) and rehydrates the live Encounter
// via LoadFromData.
//
// Wave 2.8 adds Monsters + Mode + turn-state. The encounter SDK is the
// source of truth for whose turn it is — the orchestrator does not mirror.
//
// Wave 2.9 adds PendingPrompts: at most one in-flight prompt per player,
// resolved via SubmitCheck.
//
// Wave 2.11c adds ReactionReadiness: per-entity, per-reaction readiness
// flags that persist across attacks within an encounter session.
type Data struct {
	ID       core.EncounterID               `json:"id"`
	Sequence uint64                         `json:"sequence"`
	Players  map[core.PlayerID]*PlayerData  `json:"players"`
	Doors    map[core.EntityID]*DoorData    `json:"doors"`
	Monsters map[core.EntityID]*MonsterData `json:"monsters"`

	// Mode + turn state. Initiative / ActiveIdx / Round are only meaningful
	// when Mode == ModeTurnBased; serialized as omitempty otherwise.
	Mode       core.EncounterMode `json:"mode"`
	Initiative []core.EntityID    `json:"initiative,omitempty"`
	ActiveIdx  int                `json:"active_idx,omitempty"`
	Round      int                `json:"round,omitempty"`

	// PendingPrompts holds at most one in-flight prompt per player. A prompt
	// is set by a verb that needs a player decision/check (AttemptUnlock,
	// future dialogue/target-select) and cleared by SubmitCheck regardless
	// of outcome. Omitted from the wire when empty.
	PendingPrompts map[core.PlayerID]*PendingPrompt `json:"pending_prompts,omitempty"`

	// ReactionReadiness tracks per-entity readiness for each named reaction.
	// Keys are entity IDs; values are maps from reaction ref strings
	// (e.g. "dnd5e:conditions:opportunity_attack") to ready booleans.
	// Free-cost reactions (OA) default to true for melee combatants;
	// spell-cost reactions (Shield, Counterspell) default to false.
	// Survives ToData/LoadFromData round-trips; omitted from JSON when empty.
	ReactionReadiness map[core.EntityID]map[string]bool `json:"reaction_readiness,omitempty"`

	// PendingReactionPrompts holds, per reactor PlayerID, the in-flight
	// phase-1 attack state plus the trigger details for which reaction the
	// reactor is being asked about. The orchestrator persists this across
	// the (TakeAction → wait for SubmitCheck{take_reaction} → CompleteTakeAction)
	// flow.
	//
	// Keyed by REACTOR playerID (not the original caller's playerID): the
	// reactor responds via SubmitCheck on their own session.
	//
	// Cleared when the reactor's SubmitCheck resolves (take or skip).
	PendingReactionPrompts map[core.PlayerID]*PendingReactionPrompt `json:"pending_reaction_prompts,omitempty"`
}

// PendingReactionPrompt is the persisted shape of a reaction prompt waiting
// on a player's SubmitCheck{take_reaction} response. The orchestrator sets
// this when TakeActionPhased returns reactions for a player reactor; it is
// cleared (and consumed) when CompleteTakeAction runs after the reactor's
// response.
//
// AttackContext is opaque to the encounter SDK — the resolver implementation
// (rpg-api's Dnd5eCombatResolver) interprets it. The field is json.RawMessage
// to keep the SDK rulebook-agnostic; rpg-api marshals/unmarshals via its own
// type when persisting and reloading.
type PendingReactionPrompt struct {
	// ReactorEntityID is the entity that can react. Used by the orchestrator
	// to map back to the reactor's PlayerID for prompt routing.
	ReactorEntityID core.EntityID `json:"reactor_entity_id"`

	// ConditionRef identifies which reaction this prompt is asking about
	// (e.g. "dnd5e:conditions:opportunity_attack", "dnd5e:spells:shield").
	ConditionRef string `json:"condition_ref"`

	// TriggerKind mirrors the rulebook's TriggerKind enum
	// (e.g. "post_hit", "movement_oa", "post_damage").
	TriggerKind string `json:"trigger_kind"`

	// SourceEntity is the entity whose action triggered this prompt
	// (the attacker for post_hit, the mover for movement_oa).
	SourceEntity core.EntityID `json:"source_entity"`

	// AttackContextJSON is the serialized phase-1 attack state. The
	// resolver implementation owns the schema; the SDK treats it as opaque
	// bytes to keep the boundary clean. CompleteTakeAction unmarshals it
	// into the resolver's PhasedAttackContext shape before phase 2.
	AttackContextJSON []byte `json:"attack_context_json"`
}

// PlayerData persists a single player seat.
//
// HP / MaxHP / AC / AttackBonus / DamageDice are Wave 2.8 additions used
// by combat verbs. They are intentionally minimal; full character /
// ResolveAttack chain integration (ability scores, weapons, EffectiveAC)
// is tracked as a followup.
type PlayerData struct {
	ID       core.PlayerID    `json:"id"`
	EntityID core.EntityID    `json:"entity_id"`
	View     *perception.View `json:"view"`

	// Combat snapshot — used by TakeAction / NPCAct.
	HP          int    `json:"hp,omitempty"`
	MaxHP       int    `json:"max_hp,omitempty"`
	AC          int    `json:"ac,omitempty"`
	AttackBonus int    `json:"attack_bonus,omitempty"`
	DamageDice  string `json:"damage_dice,omitempty"`
	DamageType  string `json:"damage_type,omitempty"`
}

// DoorData persists a door entity.
//
// Wave 2.9 adds the locked-door snapshot. Locked is the orchestrator's
// signal that the door requires AttemptUnlock + SubmitCheck rather than
// OpenDoor. The toolkit does NOT gate OpenDoor on Locked — orchestrators
// (and the verb router on the wire side) route player intent to
// AttemptUnlock when a door is Locked. SubmitCheck on success clears
// Locked before calling OpenDoor internally so the door round-trips as
// unlocked-and-open. All lock fields are omitempty so legacy DoorData
// (Wave 2.7) round-trips as an unlocked door.
type DoorData struct {
	ID       core.EntityID `json:"id"`
	Position core.Hex      `json:"position"`
	Open     bool          `json:"open"`

	// Locked-door state (Wave 2.9). Locked must be true for AttemptUnlock
	// to issue a prompt (returns ErrDoorNotLocked otherwise). LockDC,
	// LockAbility, and LockTool feed the SkillCheck prompt that resolution
	// runs through. LockAbility uses 3-letter codes ("DEX", "STR"). LockTool
	// is a toolkit ref (e.g. "dnd5e:item:thieves-tools"); empty means no
	// tool proficiency applies.
	Locked      bool   `json:"locked,omitempty"`
	LockDC      int    `json:"lock_dc,omitempty"`
	LockAbility string `json:"lock_ability,omitempty"`
	LockTool    string `json:"lock_tool,omitempty"`
}

// MonsterData persists a monster entity, including the serialized
// monster.Data blob that the dnd5e rulebook rehydrates per-call.
//
// MonsterRef identifies the monster type (e.g. "dnd5e:monsters:goblin").
// DataJSON is the full monster.Data marshaled to JSON; LoadFromData
// rehydrates it via monster.LoadFromData on a per-call dnd5e bus.
//
// AttackBonus / DamageDice / DamageType are a snapshot of the monster's
// primary attack used by NPCAct's stand-in resolution. The proper integration
// reads these from the loaded monster's actions; the snapshot exists so an
// orchestrator can seed a monster without rehydrating the rulebook.
type MonsterData struct {
	ID         core.EntityID `json:"id"`
	Position   core.Hex      `json:"position"`
	HP         int           `json:"hp"`
	MaxHP      int           `json:"max_hp"`
	AC         int           `json:"ac"`
	Speed      int           `json:"speed"`
	MonsterRef string        `json:"monster_ref"`
	DataJSON   []byte        `json:"data_json,omitempty"`

	AttackBonus int    `json:"attack_bonus,omitempty"`
	DamageDice  string `json:"damage_dice,omitempty"`
	DamageType  string `json:"damage_type,omitempty"`
}

// NewData constructs an empty Data with a fresh ID. Mode defaults to
// ModeFreeRoam; turn-state fields remain at their zero values.
func NewData(id core.EncounterID) *Data {
	return &Data{
		ID:                     id,
		Players:                make(map[core.PlayerID]*PlayerData),
		Doors:                  make(map[core.EntityID]*DoorData),
		Monsters:               make(map[core.EntityID]*MonsterData),
		Mode:                   core.ModeFreeRoam,
		PendingPrompts:         make(map[core.PlayerID]*PendingPrompt),
		ReactionReadiness:      make(map[core.EntityID]map[string]bool),
		PendingReactionPrompts: make(map[core.PlayerID]*PendingReactionPrompt),
	}
}
