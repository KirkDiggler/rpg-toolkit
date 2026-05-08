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
type DoorData struct {
	ID       core.EntityID `json:"id"`
	Position core.Hex      `json:"position"`
	Open     bool          `json:"open"`
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
		ID:       id,
		Players:  make(map[core.PlayerID]*PlayerData),
		Doors:    make(map[core.EntityID]*DoorData),
		Monsters: make(map[core.EntityID]*MonsterData),
		Mode:     core.ModeFreeRoam,
	}
}
