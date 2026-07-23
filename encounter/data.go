package encounter

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
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

	// Space is the encounter's room snapshot (walls + dimensions), populated
	// once from environments.QuickRoom at encounter creation (see
	// Encounter.InitRoom). Nil for encounters with no spatial room —
	// pre-wave-1 fixtures and non-spatial (social/free-form) encounters —
	// which fall back to the pure-radius LoS and unblocked movement that
	// predates this field. Persists as a snapshot, not a regeneration seed:
	// wave 2+ wall destruction / door state would otherwise be unable to
	// replay from a pattern.
	Space *SpaceData `json:"space,omitempty"`
}

// SpaceData is the persisted snapshot of an encounter's room: the wall
// layout and grid dimensions needed to reconstruct the spatial.Room (and
// its wall entities) on every LoadFromData, without re-running the
// (non-deterministic) room generator. Walls reuse environments.WallSegmentData
// rather than a toolkit-local type — same shape environments.EnvironmentData
// already persists, no reason to duplicate it.
type SpaceData struct {
	// Walls are the room's wall segments in absolute cube coordinates. Wave 1
	// stores one degenerate (Start == End) segment per discretized wall hex —
	// geometry fidelity beyond per-hex blocking isn't needed until per-viewer
	// wall reveal (wave 2+).
	Walls []environments.WallSegmentData `json:"walls"`

	// Width and Height are the room's grid dimensions (offset-coordinate
	// bounds), needed to reconstruct a HexGrid that agrees with the original
	// on IsValidPosition — without these, a reconstructed room would need a
	// guessed bound, silently accepting or rejecting edge positions wrong.
	Width  int `json:"width"`
	Height int `json:"height"`

	// Entrance is the designated spawn-anchor cell for chamber 1, set by
	// multi-chamber generators (Wave 2 Slice 2's InitTwoChamberRoom).
	// Replaces the roomCenterHex() placeholder downstream (rpg-api#648).
	// Zero value (the zero Hex) for single-room InitRoom spaces, which have
	// no entrance concept — callers fall back to their own placement (e.g.
	// room center) when Entrance is the zero Hex.
	Entrance core.Hex `json:"entrance"`

	// Regions tags every hex by chamber for multi-chamber spaces (Wave 2
	// Slice 2) — scopes spawn placement and, via LoS, combat pockets
	// (rpg-toolkit#796). Empty for single-room InitRoom spaces, which have
	// exactly one implicit region.
	Regions []RegionData `json:"regions,omitempty"`

	// Theme is opaque cosmetic metadata (e.g. "crypt") that InitDungeon
	// carries through from DungeonParams.Theme without interpreting —
	// rpg-toolkit#814's Approved Slice 3 corrections are explicit that the
	// generator must never branch on Theme's value; it exists purely so a
	// host/client can key dressing off it downstream. Distinct from
	// RegionData.Archetype, which identifies region ROLE (a fixed generic
	// vocabulary), not cosmetic theme. Empty for InitRoom/InitTwoChamberRoom
	// spaces and any InitDungeon call that doesn't set it.
	Theme string `json:"theme,omitempty"`

	// Obstacles are generic, content-agnostic static obstacle instances —
	// rpg-toolkit#818. Same category as a Wall (a static blocker the room
	// must agree with on BlocksMovement/BlocksLoS) but an INSTANCE at one
	// hex, not a boundary-edge segment, and with a stable ID/Ref a host or
	// client can key off across ticks/reloads — a set-piece (crypt
	// sarcophagus, altar, pillar, ...) rather than room geometry. The
	// toolkit never interprets Ref or branches on it; placement policy
	// (WHERE obstacles go for a given theme/archetype) is implemented at
	// InitDungeon's DungeonRegionParams.Obstacles + placeRegionObstacles
	// (rpg-toolkit#819 — the crypt-placement issue this comment used to
	// point at as future work; see CryptDungeonParams for the crypt
	// template's own composition). This field itself remains the single,
	// generic, content-agnostic representation regardless of caller —
	// #818's shape is unchanged by #819's addition. Order-preserving
	// slice, not a map: AddObstacle enforces unique, non-empty IDs itself
	// (see validateObstacles) rather than relying on map-key overwrite
	// semantics the way Doors does. Empty for every space predating #818
	// — omitted from the wire when empty so old snapshots round-trip
	// byte-for-byte.
	Obstacles []ObstacleData `json:"obstacles,omitempty"`
}

// ObstacleData persists one static obstacle instance: a generic,
// content-agnostic blocker at a single hex — rpg-toolkit#818. Distinct
// from a Wall (a boundary-edge segment discretized per-hex, generator-
// owned) and from a Door (a connector with open/locked state): an
// obstacle is an INSTANCE a host places (e.g. via AddObstacle) at a
// specific position, with no notion of "edge" or "connects two regions".
type ObstacleData struct {
	// ID is this obstacle's stable identifier — unique within its
	// SpaceData's Obstacles — so a host or client can reference the SAME
	// obstacle across ticks/reloads (e.g. to target it with an interaction
	// verb in a later issue). Required; enforced non-empty and unique by
	// AddObstacle/validateObstacles, never auto-generated.
	ID core.EntityID `json:"id"`

	// Ref is an opaque content identifier for this obstacle (e.g.
	// "dnd5e:obstacles:sarcophagus") — mirrors MonsterData.MonsterRef's
	// plain-opaque-string convention. The encounter SDK never interprets
	// Ref; it exists so a host/client can look up dressing, interaction
	// verbs, or other content-specific behavior keyed off it downstream,
	// without the toolkit needing to know what a "sarcophagus" is.
	Ref string `json:"ref"`

	// Position is this obstacle's absolute cube-coordinate-backed hex in
	// the encounter's Space — the same coordinate space Walls/Doors/
	// Regions already use (core.Hex, ->ToCube()/->ToPosition() convert to
	// the spatial package's types rebuildRoomFromData places into).
	Position core.Hex `json:"position"`

	// BlocksMovement mirrors environments.WallProperties.BlocksMovement —
	// when true, the reconstructed spatial.Room rejects placing/moving a
	// blocking entity onto this obstacle's hex, exactly like a wall.
	BlocksMovement bool `json:"blocks_movement"`

	// BlocksLoS mirrors environments.WallProperties.BlocksLoS — when
	// true, the reconstructed spatial.Room's IsLineOfSightBlocked reports
	// true for any line of sight passing through this obstacle's hex,
	// exactly like a wall.
	BlocksLoS bool `json:"blocks_los"`
}

// RegionData tags a named set of hexes as one chamber/region within a
// SpaceData. IDs are generator-defined (InitTwoChamberRoom uses
// RegionChamber1/RegionChamber2); hosts key spawn/seeding decisions off
// them without knowing generator internals.
type RegionData struct {
	ID    string      `json:"id"`
	Hexes core.HexSet `json:"hexes"`

	// Archetype identifies this region's generic ROLE (entrance | chamber |
	// corridor | boss) — rpg-toolkit#814. Distinct from SpaceData.Theme:
	// Archetype is a fixed, reusable vocabulary a host keys spawn tables
	// off (or a client keys dressing off) without knowing generator
	// internals or theme content. Omitted from the wire when empty — the
	// zero value for regions predating #814 (none exist yet, but this
	// mirrors every other additive field in this package).
	Archetype RegionArchetype `json:"archetype,omitempty"`
}

// RegionArchetype identifies a region's generic ROLE within a dungeon.
// This is a fixed, reusable vocabulary — NOT a per-template or per-theme
// enum — so hosts and clients can key behavior off it without
// understanding any specific generator's internals (rpg-toolkit#814,
// Approved Slice 3 corrections).
type RegionArchetype string

const (
	// ArchetypeEntrance tags the region containing SpaceData.Entrance —
	// the dungeon's starting region.
	ArchetypeEntrance RegionArchetype = "entrance"
	// ArchetypeChamber tags a generic interior region with no more
	// specific role — remains in the vocabulary for future templates even
	// though the first crypt template (#814's done bar) doesn't emit it.
	ArchetypeChamber RegionArchetype = "chamber"
	// ArchetypeCorridor tags a connecting region between two chambers. Not
	// a distinct geometry primitive (#814 explicitly defers that) — a
	// corridor is generated exactly like any other region, just narrower.
	ArchetypeCorridor RegionArchetype = "corridor"
	// ArchetypeBoss tags the dungeon's climactic region. Any region tagged
	// ArchetypeBoss must satisfy the boss-room scale invariant: its
	// primary playable axis (min(region width, the dungeon's shared
	// height)) must exceed 6 hex steps — enforced at generation time by
	// InitDungeon, not left to eyeballing.
	ArchetypeBoss RegionArchetype = "boss"
)

// RegionAt returns the region ID containing hex, and whether one was
// found. Nil-receiver safe — Data.Space is nil for encounters with no
// spatial room (pre-wave-1 fixtures, non-spatial encounters), and this is
// an exported helper hosts call directly off ToData().Space without
// necessarily nil-checking first — so a nil *SpaceData behaves exactly
// like a SpaceData with no regions ("", false) rather than panicking.
// O(n) over the region hex sets otherwise — fine at slice-2 scale (a
// couple dozen hexes per chamber); revisit with an index if regions grow
// large.
func (sd *SpaceData) RegionAt(h core.Hex) (string, bool) {
	if sd == nil {
		return "", false
	}
	for _, r := range sd.Regions {
		if r.Hexes.Has(h) {
			return r.ID, true
		}
	}
	return "", false
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

	// DataJSON is the host-supplied serialized dnd5e character.Data for this
	// player. #689: the encounter's LoadFromData cascade rehydrates the
	// character from this blob (character.LoadFromData) so its conditions
	// subscribe to the encounter bus exactly once — the single subscribe point
	// that cures the #684 double-subscribe class. The host fetches the blob
	// from its character store and attaches it before LoadFromData; it is a
	// transient serialization seam (mirrors MonsterData.DataJSON and
	// ActivateFeatureInput.CharDataJSON). Omitted from the wire when empty —
	// players without a blob fall back to the stat-snapshot stand-in path.
	DataJSON json.RawMessage `json:"data_json,omitempty"`

	// ActiveConditions lists the canonical ref strings (e.g.
	// "dnd5e:conditions:raging") of every condition currently applied to
	// this player's held character (rpg-toolkit#754). Refreshed by ToData's
	// write-back cascade from the held character's GetConditions(), the same
	// accessor checkEncounterEnd's sweep would use — not read back from
	// DataJSON, so it reflects the LIVE in-memory state at snapshot time, not
	// whatever was last persisted. Nil (omitted from the wire) when the seat
	// has no held character or no active conditions. Exists so a client that
	// (re)connects mid-encounter learns about a condition that was hydrated
	// in via LoadFromData or applied while it was disconnected — previously
	// only the live StatusApplied/StatusRemoved broker stream carried this,
	// which a reconnecting viewer never saw. Deliberately minimal (ref only,
	// no duration/source) — matches ConditionAppliedEvent's own cause-event
	// shape; the consumer enriches display_name/icon_hint itself, the same
	// division of labor translateConditionAppliedEvent already uses for the
	// live path.
	ActiveConditions []string `json:"active_conditions,omitempty"`

	// Dead marks a player-seat entity as having died within THIS encounter
	// (either 3 failed death saves via the CharacterDied bridge, or the
	// non-hydrated flat-snapshot instant-death fallback — see
	// publishPlayerDied in death.go, the single site that sets this).
	// rpg-toolkit#772/#782: drives checkEncounterEnd's TPK predicate — once
	// every seated player carries Dead=true, the encounter transitions to
	// ModeEnded with Reason "tpk", mirroring the monster-side
	// len(Monsters)==0 predicate.
	//
	// Deliberately does NOT mean "unconscious" — a player at HP<=0 who is
	// still death-saving (could nat-20 revive) has Dead==false; only a
	// CONFIRMED death sets it. This preserves the death-save mechanic's
	// (rpg-toolkit#729/#741) chance to matter even for the last conscious
	// player, rather than ending the encounter the instant everyone hits 0
	// HP.
	//
	// Terminal within the encounter: nothing in the toolkit today clears
	// Dead back to false — there is no in-encounter revival mechanic (no
	// Raise Dead / Revivify verb exists yet). It is set at most once and
	// persists for the rest of THIS encounter's lifetime.
	//
	// Scoped to the encounter snapshot ONLY — never written into the held
	// character's own DataJSON/character.Data, so it cannot leak across
	// encounters. A character who died in encounter A and is later seated
	// (narratively revived, or simply re-added for a fresh fight/test) in
	// encounter B gets a brand-new PlayerData with Dead defaulting to
	// false; cross-encounter cleanliness is the character record's job
	// (mirrors the raging-condition leak fix, rpg-toolkit#752/PR#753 —
	// see encounter_end_condition_sweep_test.go), not this flag's.
	//
	// Persistence / restart safety: set and checked within one synchronous
	// call stack — publishPlayerDied sets Dead=true and then calls
	// checkEncounterEnd() inline, before the triggering verb call returns
	// to the host. There is no intermediate point where Dead=true could be
	// persisted without the resulting Mode==ModeEnded transition (when
	// Dead=true completes the TPK predicate) also being persisted alongside
	// it — a host only calls ToData() after a verb call fully returns.
	// LoadFromData therefore does NOT need a defensive re-check of this
	// predicate the way seedActiveActorIfUnseeded (turn_economy.go)
	// re-checks turn-start seeding — that gap exists because SetMode and
	// seedActorTurn are two separate entry points that can race a reload;
	// here the mutation and the predicate check are the same function call,
	// so the "mutated but not yet checked" state this field's persistence
	// story worries about is unreachable, not merely untested.
	Dead bool `json:"dead,omitempty"`
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

	// ActiveConditions lists the canonical ref strings (e.g.
	// "dnd5e:monster_traits:pack_tactics") of every condition currently
	// applied to this monster (rpg-toolkit#754) — same mechanics and same
	// rationale as PlayerData.ActiveConditions, refreshed from the held
	// monster's GetConditions() at ToData() time. Monster "conditions" in
	// this rulebook include monstertraits (Immunity/Vulnerability/
	// PackTactics/UndeadFortitude) alongside anything a live encounter
	// applies (poisoned, etc.) — both are genuinely-Applied
	// dnd5eEvents.ConditionBehavior instances once a monster has been
	// through any LoadFromData cycle (AddTraitData's pre-bus staging only
	// matters before the first such cycle), so this field does not
	// distinguish "innate trait" from "battlefield condition"; neither does
	// the toolkit's own condition model. Nil when the seat has no held
	// monster or no active conditions.
	ActiveConditions []string `json:"active_conditions,omitempty"`
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
