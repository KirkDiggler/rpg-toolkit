package encounter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	dnd5events "github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// OAReactionRef is the canonical reaction ref string for Opportunity Attack.
// Wave 2.11c defines this ref so the readiness map can be seeded for melee
// combatants. The full OpportunityAttackCondition implementation ships in
// Wave 2.11d (#649); this constant aligns with the ref that condition will use.
const OAReactionRef = "dnd5e:conditions:opportunity_attack"

// Encounter is the transient SDK object for one ongoing encounter.
// Constructed per-call via LoadFromData; mutated by verbs; serialized via
// ToData and saved.
//
// Wave 2.11c: the encounter now owns a single dnd5e EventBus for its
// lifetime. Conditions Apply()'d during character rehydration subscribe
// to this bus once and stay subscribed across attacks, enabling persistent
// state (SneakAttack.UsedThisTurn, Protection reaction consumption) to
// accumulate correctly within a turn and reset at turn boundaries.
// The bus is reconstructed at LoadFromData — it is not serialized.
type Encounter struct {
	data             *Data
	broker           *Broker
	roller           dice.Roller
	resolver         CharacterResolver
	combatResolver   CombatResolver
	movementResolver MovementResolver
	// bus is the encounter-scoped dnd5e event bus. Conditions subscribe
	// once at rehydration and remain subscribed for the encounter's lifetime.
	// Reconstructed (not serialized) at each LoadFromData call.
	bus dnd5events.EventBus
	// pendingPhasedAttacks caches the in-flight PhasedAttackContext for each
	// reactor whose NPC-attack-triggered prompt is awaiting response. This is
	// in-process state, not serialized — the host marshals via its rulebook
	// adapter into PendingReactionPrompt.AttackContextJSON before saving the
	// encounter snapshot. Wave 2.11d.
	pendingPhasedAttacks map[core.PlayerID]*PhasedAttackContext

	// combatants holds the runtime rulebook entities (characters + monsters)
	// hydrated once by the LoadFromData cascade. #689: this is the single place
	// combatants are loaded and their conditions Apply()'d to e.bus, curing the
	// #684 double-subscribe class. Reconstructed (not serialized) at each
	// LoadFromData — exactly like e.bus. The combat/movement resolvers receive
	// the held entity (AttackInput.Attacker/Defender, MovementStepInput.Mover)
	// and never re-load. Empty when no entities carry rehydratable data.
	combatants map[core.EntityID]combat.Combatant

	// syncErr holds the most recent error from the ToData write-back cascade
	// (syncCombatantsToData). Surfaced via SyncErr() so a host can detect a
	// dropped persistence write before Save — a re-marshal failure is a
	// correctness loss (e.g. unsaved per-turn condition state). Nil when the
	// last ToData synced cleanly. #689.
	syncErr error

	// room and roomOrchestrator are the encounter's spatial room, built by
	// InitRoom (fresh encounter) or rebuildRoomFromData (LoadFromData) from
	// data.Space. Transient — reconstructed on every New/LoadFromData call,
	// exactly like bus and combatants, never serialized. Both remain nil for
	// encounters with no room (data.Space == nil); every room-aware call
	// site in this package nil-checks before using them.
	room             spatial.Room
	roomOrchestrator spatial.RoomOrchestrator
}

// Option configures an Encounter at construction.
type Option func(*Encounter)

// WithRoller injects a dice.Roller for combat verbs that need to roll
// (initiative, attacks, damage). If unset the encounter creates a default
// dice.NewRoller() at construction.
func WithRoller(r dice.Roller) Option {
	return func(e *Encounter) {
		if r != nil {
			e.roller = r
		}
	}
}

// WithCharacterResolver injects a CharacterResolver used by SubmitCheck
// to look up the ability/proficiency modifiers for the acting player.
// In production rpg-api wires this against its character store; tests
// supply a stub. SubmitCheck returns ErrNoCharacterResolver if a check
// is submitted without one.
func WithCharacterResolver(r CharacterResolver) Option {
	return func(e *Encounter) {
		e.resolver = r
	}
}

// WithCombatResolver injects a CombatResolver used by combat verbs
// (today: TakeAction's player-attack path; future waves extend) to
// evaluate attack mechanics through a rulebook implementation. The
// encounter SDK never embeds rulebook logic; the orchestrator (rpg-api)
// wires this against the dnd5e rulebook in production. Tests supply
// a stub. TakeAction returns ErrNoCombatResolver if invoked without one.
func WithCombatResolver(r CombatResolver) Option {
	return func(e *Encounter) {
		e.combatResolver = r
	}
}

// WithMovementResolver injects a MovementResolver used by Encounter.Move
// to delegate per-step movement mechanics (MovementChain execution, OA
// triggering) to a rulebook implementation. Wave 2.11e (#658).
//
// Optional — when not supplied, Encounter.Move uses the legacy single-
// jump behavior that mutates position to path[-1] without per-step chain
// execution. Non-combat encounters (free-roam, social) don't need a
// resolver and don't pay the per-step iteration cost.
func WithMovementResolver(r MovementResolver) Option {
	return func(e *Encounter) {
		e.movementResolver = r
	}
}

// PlayerInput populates a player seat at construction / AddPlayer time.
//
// Combat fields are optional. A seat is treated as a combatant for
// TakeAction iff MaxHP > 0 AND EITHER the seat is actually HELD — hydrated
// via the LoadFromData cascade, which happens when DataJSON was present at
// load time, NOT merely when DataJSON is set on this input (New()+AddPlayer
// never hydrates; only a LoadFromData round-trip does) — OR AC > 0 and
// DamageDice is non-empty (the stat-snapshot stand-in path). A held
// character resolves attacks through the real rules chain, which never reads
// the flat AC/DamageDice snapshot — see isPlayerCombatant in combat.go.
// AttackBonus may be 0 (no proficiency) and DamageType defaults to "untyped"
// when empty. Seats added without MaxHP and one of the two combat-readiness
// paths cannot call combat verbs and TakeAction returns ErrNonCombatant for
// them.
type PlayerInput struct {
	PlayerID   core.PlayerID
	EntityID   core.EntityID
	Position   core.Hex
	SightRange int

	HP          int
	MaxHP       int
	AC          int
	AttackBonus int
	DamageDice  string
	DamageType  string

	// DataJSON is the host-supplied serialized dnd5e character.Data for this
	// player. #689: when present, the LoadFromData cascade rehydrates the
	// character from it (character.LoadFromData) onto the encounter bus so its
	// conditions subscribe exactly once. Optional — a seat without it falls
	// back to the stat-snapshot stand-in path in the resolver.
	DataJSON json.RawMessage
}

// MonsterInput populates a monster seat at AddMonster time.
//
// MonsterRef and DataJSON are required if the orchestrator wants the
// encounter to drive AI via NPCAct (which rehydrates a *monster.Monster
// from DataJSON). The combat snapshot fields (AttackBonus / DamageDice /
// DamageType) feed NPCAct's stand-in attack resolution.
type MonsterInput struct {
	ID         core.EntityID
	Position   core.Hex
	HP         int
	MaxHP      int
	AC         int
	Speed      int
	MonsterRef string
	DataJSON   []byte

	AttackBonus int
	DamageDice  string
	DamageType  string
}

// New constructs a fresh encounter with the given ID.
//
// #689: New takes a ctx for signature parity with LoadFromData. The hydration
// cascade runs ONLY inside LoadFromData — New does not hydrate (a fresh
// encounter has no combatants yet). A freshly New'd encounter populated with
// AddPlayer/AddMonster carrying DataJSON is therefore not hydrated until its
// next round-trip through LoadFromData, matching the production lifecycle of
// create → persist → load-per-RPC.
func New(ctx context.Context, id core.EncounterID, b *Broker, opts ...Option) *Encounter {
	e := &Encounter{
		data:       NewData(id),
		broker:     b,
		roller:     dice.NewRoller(),
		bus:        dnd5events.NewEventBus(),
		combatants: make(map[core.EntityID]combat.Combatant),
	}
	// #733: permanent bridge from the rulebook's CharacterDiedTopic (3 failed
	// death saves) to the broker EntityDiedEvent — installed once, right after
	// the fresh bus is constructed, for the encounter's lifetime (see
	// subscribeCharacterDiedBridge's doc for why this can't be a transient
	// per-verb capture buffer like the rest of this package's subscriptions).
	// Subscribing against a just-constructed simpleEventBus cannot fail (its
	// Subscribe implementation only ever returns nil) — New's signature
	// returns no error, so this is deliberately swallowed rather than
	// widening New's contract for something structurally infallible here.
	_ = e.subscribeCharacterDiedBridge(ctx)
	// #741: same permanent-bridge shape and same "cannot fail" reasoning as
	// subscribeCharacterDiedBridge above — CharacterStabilizedTopic and
	// DeathSaveRolledTopic previously had zero production subscribers
	// anywhere in this package.
	_ = e.subscribeCharacterStabilizedBridge(ctx)
	_ = e.subscribeDeathSaveRolledBridge(ctx)
	// rpg-toolkit#772/#781/#784: syncs PlayerData.HP from held-character
	// healing (nat-20 death-save revival today; any future healing spell/
	// feature) — see subscribeHealingReceivedBridge's doc for why this
	// couldn't just be "read char.GetHitPoints() at ToData() time."
	_ = e.subscribeHealingReceivedBridge(ctx)
	e.subscribeConditionRemovedBridge(ctx)
	for _, o := range opts {
		o(e)
	}
	return e
}

// LoadFromData rehydrates an encounter from persisted state.
//
// #689: LoadFromData OWNS combatant hydration. A fresh encounter-scoped dnd5e
// EventBus is created here, then the cascade rehydrates every combatant that
// carries rehydratable data (PlayerData.DataJSON → character.LoadFromData;
// MonsterData.DataJSON → monster.LoadFromData) onto that bus exactly once,
// applying their persistent conditions + default reaction conditions. This
// single cascade is the only subscribe point — it cures the #684
// double-subscribe class that arose when the host loaded entities per attack.
// The held entities are exposed to the resolvers (AttackInput.Attacker/Defender,
// MovementStepInput.Mover); resolvers MUST NOT re-load.
//
// ctx flows into the rulebook loaders (they take a context). The bus + held
// combatants are reconstructed fresh on each call — they are not serialized.
func LoadFromData(ctx context.Context, data *Data, b *Broker, opts ...Option) (*Encounter, error) {
	if data == nil {
		return nil, errors.New("nil Data")
	}
	if data.Players == nil {
		data.Players = make(map[core.PlayerID]*PlayerData)
	}
	if data.Doors == nil {
		data.Doors = make(map[core.EntityID]*DoorData)
	}
	if data.Monsters == nil {
		data.Monsters = make(map[core.EntityID]*MonsterData)
	}
	if data.PendingPrompts == nil {
		data.PendingPrompts = make(map[core.PlayerID]*PendingPrompt)
	}
	if data.ReactionReadiness == nil {
		data.ReactionReadiness = make(map[core.EntityID]map[string]bool)
	}
	if data.PendingReactionPrompts == nil {
		data.PendingReactionPrompts = make(map[core.PlayerID]*PendingReactionPrompt)
	}
	if data.Mode == core.ModeUnspecified {
		data.Mode = core.ModeFreeRoam
	}
	e := &Encounter{
		data:       data,
		broker:     b,
		roller:     dice.NewRoller(),
		bus:        dnd5events.NewEventBus(),
		combatants: make(map[core.EntityID]combat.Combatant),
	}
	// #733: same permanent CharacterDied -> EntityDied bridge as New — see
	// subscribeCharacterDiedBridge's doc. Unlike New, LoadFromData already
	// returns an error, so failure here is propagated rather than swallowed.
	if err := e.subscribeCharacterDiedBridge(ctx); err != nil {
		return nil, fmt.Errorf("subscribe character died bridge: %w", err)
	}
	// #741: same permanent-bridge shape as subscribeCharacterDiedBridge —
	// CharacterStabilizedTopic and DeathSaveRolledTopic previously had zero
	// production subscribers anywhere in this package.
	if err := e.subscribeCharacterStabilizedBridge(ctx); err != nil {
		return nil, fmt.Errorf("subscribe character stabilized bridge: %w", err)
	}
	if err := e.subscribeDeathSaveRolledBridge(ctx); err != nil {
		return nil, fmt.Errorf("subscribe death save rolled bridge: %w", err)
	}
	// rpg-toolkit#772/#781/#784: same permanent-bridge shape as the others
	// above — see subscribeHealingReceivedBridge's doc.
	if err := e.subscribeHealingReceivedBridge(ctx); err != nil {
		return nil, fmt.Errorf("subscribe healing received bridge: %w", err)
	}
	e.subscribeConditionRemovedBridge(ctx)
	for _, o := range opts {
		o(e)
	}
	if err := e.hydrateCombatants(ctx); err != nil {
		return nil, fmt.Errorf("hydrate combatants: %w", err)
	}
	if err := e.rebuildRoomFromData(); err != nil {
		return nil, fmt.Errorf("rebuild room: %w", err)
	}
	// #757: catch up the active actor's turn seed when combat entry fired on
	// an un-hydrated encounter (see seedActiveActorIfUnseeded's doc).
	if err := e.seedActiveActorIfUnseeded(ctx); err != nil {
		return nil, fmt.Errorf("seed active actor: %w", err)
	}
	return e, nil
}

// AddPlayer registers a new player seat with a fresh PerceptionView.
// The player sees their starting position and surrounding hexes immediately.
//
// Wave 2.11c: if the player seat carries combat stats (DamageDice set), the
// Opportunity Attack reaction is seeded as ready-by-default. OA is a
// free-cost reaction; players should not need to opt in per-fight.
// Spell-cost reactions (Shield, Counterspell) are seeded false and require
// the player to opt in via SetReactionReady.
func (e *Encounter) AddPlayer(input PlayerInput) error {
	if _, exists := e.data.Players[input.PlayerID]; exists {
		return fmt.Errorf("player %q already in encounter", input.PlayerID)
	}
	view := perception.NewView(input.PlayerID, input.Position, input.SightRange)
	view.ApplyReveal(perception.VisibleHexesAt(input.Position, input.SightRange, e.room))

	hp, maxHP, dataJSON, err := restoreForNewSeat(input.HP, input.MaxHP, input.DataJSON)
	if err != nil {
		return fmt.Errorf("restore player %q for new seat: %w", input.PlayerID, err)
	}

	e.data.Players[input.PlayerID] = &PlayerData{
		ID:          input.PlayerID,
		EntityID:    input.EntityID,
		View:        view,
		HP:          hp,
		MaxHP:       maxHP,
		AC:          input.AC,
		AttackBonus: input.AttackBonus,
		DamageDice:  input.DamageDice,
		DamageType:  input.DamageType,
		DataJSON:    dataJSON,
	}
	// Seed default OA readiness for combatants. Free-cost reactions default
	// on so players do not need to opt in every fight.
	if input.DamageDice != "" && input.EntityID != "" {
		e.seedOAReadiness(input.EntityID)
	}
	return nil
}

// restoreForNewSeat applies arcade recovery (rpg-toolkit#785/#795,
// dnd5eCharacter.RestoreForNewEncounter) to an incoming player's DataJSON
// before AddPlayer stores it, and — when the restore actually fires — keeps
// the encounter-level HP/MaxHP snapshot (PlayerData.HP/MaxHP, what combat
// resolution reads directly) in sync with whatever the character-side
// restore decided (character.Data.HitPoints/MaxHitPoints, what the next
// LoadFromData hydration reads). rpg-toolkit#783/#784 already document
// these two HP representations as a live divergence-bug class; silently
// reviving the embedded character while leaving a caller-supplied hp=0
// snapshot in place would add a third, differently-shaped incoherent seat
// instead of fixing the one #785 is about.
//
// rpg-toolkit#795 widened what "the restore actually fires" means: resource
// pools (rage charges, ki, hit dice) now refresh at EVERY new seating,
// regardless of HP, not only when the HP/death-save branch also fires. In
// practice this means RestoreForNewEncounter returns true — and this
// function re-marshals dataJSON — for most resource-bearing characters on
// most new seatings, where before it was true only for a revived seat. That
// does not change this function's own logic (still "conditionally
// re-marshal based on the bool"), only how often the "unchanged passthrough"
// branch below is actually taken.
//
// AddPlayer is the only call site. That matters: it fires exactly once per
// new seat (guarded by the "already in encounter" check above) and is never
// reached by LoadFromData's per-RPC rehydration of an EXISTING seat — see
// RestoreForNewEncounter's own contract doc for why that distinction is the
// whole point, resources included: resuming an existing seat must never
// refresh its resources either, the same as it must never heal it.
//
// A seat with no DataJSON (a flat stat-snapshot player) has no rulebook
// object to restore; hp/maxHP/dataJSON pass through unchanged.
func restoreForNewSeat(hp, maxHP int, dataJSON json.RawMessage) (int, int, json.RawMessage, error) {
	if len(dataJSON) == 0 {
		return hp, maxHP, dataJSON, nil
	}
	var data dnd5eCharacter.Data
	if err := json.Unmarshal(dataJSON, &data); err != nil {
		return 0, 0, nil, fmt.Errorf("unmarshal character data: %w", err)
	}
	if !dnd5eCharacter.RestoreForNewEncounter(&data) {
		return hp, maxHP, dataJSON, nil
	}
	restored, err := json.Marshal(data)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("marshal restored character data: %w", err)
	}
	return data.HitPoints, data.MaxHitPoints, restored, nil
}

// AddDoor registers a door (slice scope; future slices use a richer entity
// system).
//
// rpg-toolkit#790: if the encounter already has a room (InitRoom/
// LoadFromData ran), a closed door must immediately start blocking
// movement/LoS, so this rebuilds e.room from the now-updated Data.Doors
// (see rebuildRoomFromData). A no-op, not an error, when there's no room
// yet — InitRoom's own rebuild will pick up any door added before it.
//
// On rebuild failure, the door registration is rolled back (removed from
// Data.Doors) rather than left half-applied — a Copilot catch on this PR:
// without the rollback, a failed rebuild would leave the door persisted
// but not reflected in e.room, so a subsequent OpenDoor/rebuild could
// behave inconsistently with what ToData() reports.
//
// rebuildRoomFromData replaces e.room/e.roomOrchestrator with fresh
// objects rather than mutating the existing ones — see OpenDoor's doc for
// why a caller holding an old Room()/RoomOrchestrator() reference across
// this call would observe stale geometry.
func (e *Encounter) AddDoor(id core.EntityID, position core.Hex, open bool) error {
	e.data.Doors[id] = &DoorData{ID: id, Position: position, Open: open}
	if e.data.Space == nil {
		return nil
	}
	if err := e.rebuildRoomFromData(); err != nil {
		delete(e.data.Doors, id)
		return fmt.Errorf("rebuild room after adding door %q: %w", id, err)
	}
	return nil
}

// AddMonster registers a monster seat. Mirrors AddPlayer / AddDoor and is
// the primary fixture verb for tests and orchestrator-driven seeding.
//
// Wave 2.11c: if the monster carries combat stats (DamageDice set), the
// Opportunity Attack reaction is seeded as ready-by-default. NPCs with
// melee capability auto-fire their OA reaction — no prompt needed for NPC
// reactors per the wave's architectural call.
func (e *Encounter) AddMonster(input MonsterInput) error {
	if input.ID == "" {
		return errors.New("monster ID required")
	}
	if _, exists := e.data.Monsters[input.ID]; exists {
		return fmt.Errorf("monster %q already in encounter", input.ID)
	}
	e.data.Monsters[input.ID] = &MonsterData{
		ID:          input.ID,
		Position:    input.Position,
		HP:          input.HP,
		MaxHP:       input.MaxHP,
		AC:          input.AC,
		Speed:       input.Speed,
		MonsterRef:  input.MonsterRef,
		DataJSON:    input.DataJSON,
		AttackBonus: input.AttackBonus,
		DamageDice:  input.DamageDice,
		DamageType:  input.DamageType,
	}
	// Seed default OA readiness for combatant monsters.
	if input.DamageDice != "" {
		e.seedOAReadiness(input.ID)
	}
	// A monster added while combat is already running joins the initiative
	// order — appended to the end, acting last in the round (the common
	// reinforcement house rule; a full mid-round initiative roll + insertion
	// is a future refinement). Without this, sequential monster seeding
	// (rpg-api's StartEncounter adds its goblins one AddMonster at a time)
	// would strand every monster after the first visible one: checkCombatEntry
	// below flips the mode on the FIRST visible add, and rollInitiative only
	// includes combatants present at that moment (Copilot review, PR #759).
	// Appending never disturbs ActiveIdx — existing turn order is untouched.
	if e.data.Mode == core.ModeTurnBased {
		e.data.Initiative = append(e.data.Initiative, input.ID)
	}

	// EntityAppearedEvent for any player who can already see the newly-added
	// monster (rpg-toolkit#764) — published BEFORE checkCombatEntry so a
	// triggering add's ModeChangedEvent is preceded by the appearance that
	// caused it, matching #762's ordering contract (appearance before mode
	// change on the triggering action). Not gated on mode — see
	// playersWhoCanSee's doc comment for why a mid-combat reinforcement must
	// still appear even when checkCombatEntry itself is a no-op.
	mon := e.data.Monsters[input.ID]
	if viewers := e.playersWhoCanSee(mon); len(viewers) > 0 {
		if err := e.broker.Publish(events.NewEntityAppearedEvent(
			e.data.ID, e.nextSeq(), mon.ID, mon.Position, viewers,
		)); err != nil {
			return fmt.Errorf("publish monster appeared on add: %w", err)
		}
	}

	// Combat-entry check (rpg-toolkit#757): a newly-added monster may already
	// be visible to an existing player (e.g. spawned inside an already-open
	// LoS), so this mutation site needs the same check as Move.
	return e.checkCombatEntry()
}

// seedOAReadiness initialises an entity's readiness map (if needed) and
// sets Opportunity Attack to ready=true. Idempotent — safe to call multiple
// times for the same entity.
func (e *Encounter) seedOAReadiness(id core.EntityID) {
	if e.data.ReactionReadiness[id] == nil {
		e.data.ReactionReadiness[id] = make(map[string]bool)
	}
	e.data.ReactionReadiness[id][OAReactionRef] = true
}

// Mode returns the encounter's current mode.
func (e *Encounter) Mode() core.EncounterMode { return e.data.Mode }

// ActiveActor returns the entity id whose turn it currently is. Returns
// the empty string when Mode != ModeTurnBased or initiative is empty.
func (e *Encounter) ActiveActor() core.EntityID {
	if e.data.Mode != core.ModeTurnBased || len(e.data.Initiative) == 0 {
		return ""
	}
	idx := e.data.ActiveIdx
	if idx < 0 || idx >= len(e.data.Initiative) {
		return ""
	}
	return e.data.Initiative[idx]
}

// IsNPC reports whether the given entity id refers to a monster (NPC) in
// this encounter — i.e. not a player. Used by orchestrators to decide
// whether to call NPCAct after EndTurn.
func (e *Encounter) IsNPC(id core.EntityID) bool {
	_, ok := e.data.Monsters[id]
	return ok
}

// ID returns the encounter's identifier.
func (e *Encounter) ID() core.EncounterID { return e.data.ID }

// SnapshotFor returns the read-only view a player's gRPC handler ships
// on connect/reconnect.
func (e *Encounter) SnapshotFor(playerID core.PlayerID) Snapshot {
	p, ok := e.data.Players[playerID]
	if !ok || p.View == nil {
		return Snapshot{}
	}
	revealed := make(core.HexSet, len(p.View.RevealedHexes))
	for h := range p.View.RevealedHexes {
		revealed[h] = struct{}{}
	}
	return Snapshot{
		PlayerID:      playerID,
		Position:      p.View.Position,
		RevealedHexes: revealed,
	}
}

// Snapshot is the slice-1 read-only view. Future slices add visible
// entities, turn state, action economy, etc.
type Snapshot struct {
	PlayerID      core.PlayerID
	Position      core.Hex
	RevealedHexes core.HexSet
}

// ToData returns the persisted shape. Caller saves this to the KV store.
//
// #689: ToData first cascades held-combatant state back into the snapshot —
// the mirror of the LoadFromData hydration cascade. Each held entity's ToData()
// is re-serialized into the owning PlayerData.DataJSON / MonsterData.DataJSON so
// the next load sees the current state (e.g. SneakAttack.UsedThisTurn). This
// replaces the host's scattered save-back (rpg-api's saveAttackerConditionState
// + publishTurnEndAndPersistReset). The SDK still never STORES — the caller
// saves the returned *Data; the encounter is the entity-aware authority and
// Data is its serialization view.
//
// The write-back is UNCONDITIONAL, not gated on IsDirty(): the rulebook
// entities' dirty flag tracks only HP-shaped mutations, NOT condition-state
// changes like UsedThisTurn — which is exactly the per-turn state #689 must
// round-trip. Gating on IsDirty() would silently drop it. A dirty flag that
// covers condition mutations is the future enhancement tracked in toolkit#692;
// until then we always re-serialize held entities. See ADR-0030 + hydration.go.
//
// ToData MUTATES e.data in place (overwriting the held entities' DataJSON) — it
// is intentionally NOT a pure read. This keeps Data a faithful serialization
// view of the entity-aware authority rather than a rival source that drifts.
// The mutation is idempotent (re-serializing the same live entities yields the
// same bytes) and the package is not safe for concurrent use: the host drives
// one encounter through load → verbs → ToData → Save per RPC on a single
// goroutine, so there is no concurrent ToData/verb access to guard. A host that
// shares an Encounter across goroutines must serialize access itself.
//
// A write-back marshal failure is recorded on the encounter; check SyncErr()
// after ToData and before Save to detect a dropped persistence write.
func (e *Encounter) ToData() *Data {
	e.syncErr = e.syncCombatantsToData()
	return e.data
}

// SyncErr returns the error (if any) from the most recent ToData write-back
// cascade — a non-nil value means at least one held entity failed to
// re-serialize into its DataJSON, so the returned *Data carries that entity's
// PRIOR blob (its latest in-memory state was not persisted). Hosts that care
// about persistence integrity check this after ToData and before Save. Nil when
// the last ToData (or a never-called ToData) synced cleanly. #689.
func (e *Encounter) SyncErr() error { return e.syncErr }

// EventBus returns the encounter-scoped dnd5e event bus. Callers (rpg-api)
// use this bus to Apply() character conditions during rehydration so that
// subscriptions survive across attacks within the encounter lifetime.
//
// The bus is NOT serialized — it is reconstructed fresh at each LoadFromData.
// Condition state that must survive serialization (e.g. SneakAttack level)
// is persisted via character.Data.Conditions JSON blobs and re-Apply()'d
// each time the encounter is rehydrated from the KV store.
func (e *Encounter) EventBus() dnd5events.EventBus { return e.bus }

// SetReactionReady sets the readiness flag for a specific reaction on a
// specific entity. Returns an error if the entity is not in the encounter.
//
// charID must be an EntityID (not a PlayerID) — the same identifier used
// in AttackInput.AttackerID / TargetID.
//
// reactionRef is the canonical ref string for the reaction (e.g.
// OAReactionRef for Opportunity Attack, or "dnd5e:conditions:shield" for
// the Shield spell). The convention matches core.Ref.String().
//
// This setter is the toolkit-side implementation of the SetReactionReady RPC
// (Wave 2.11d, #531). The RPC handler in rpg-api calls this method after
// rehydrating the encounter from the KV store and before saving it back.
func (e *Encounter) SetReactionReady(charID core.EntityID, reactionRef string, ready bool) error {
	if charID == "" {
		return errors.New("charID must not be empty")
	}
	if reactionRef == "" {
		return errors.New("reactionRef must not be empty")
	}
	// Validate the entity exists in the encounter (player entity or monster).
	if !e.entityExists(charID) {
		return fmt.Errorf("entity %q not in encounter", charID)
	}
	if e.data.ReactionReadiness[charID] == nil {
		e.data.ReactionReadiness[charID] = make(map[string]bool)
	}
	e.data.ReactionReadiness[charID][reactionRef] = ready
	return nil
}

// IsReactionReady reports whether the named reaction is currently ready for
// the given entity. Returns false for any unknown entity or reaction ref —
// not-ready is the safe default (no accidental reaction fires).
func (e *Encounter) IsReactionReady(charID core.EntityID, reactionRef string) bool {
	m, ok := e.data.ReactionReadiness[charID]
	if !ok {
		return false
	}
	return m[reactionRef]
}

// entityExists reports whether an entity ID belongs to a player seat or
// a monster seat in this encounter.
func (e *Encounter) entityExists(id core.EntityID) bool {
	for _, p := range e.data.Players {
		if p.EntityID == id {
			return true
		}
	}
	_, isMonster := e.data.Monsters[id]
	return isMonster
}

// nextSeq advances and returns the encounter's monotonic sequence counter.
// Used to stamp events on publish.
func (e *Encounter) nextSeq() uint64 {
	e.data.Sequence++
	return e.data.Sequence
}

// moveFeetPerHex is the D&D 5e grid unit: one hex step costs 5 feet of
// movement (mirrors combat.FeetPerGridUnit for the encounter SDK's own
// cube-coordinate hexes, which the combat package's grid units don't share a
// type with).
const moveFeetPerHex = 5

// pathCostFeet sums the cube-coordinate hex distance between consecutive
// waypoints — moverStart, then each entry in path in turn — and converts to
// feet. Summing distances (not counting slice length) keeps the cost correct
// regardless of whether the caller supplies a dense hex-by-hex path or a
// sparse endpoints-only path (#714): the total feet moved is the same either
// way, even though the per-step chain iteration below treats each path entry
// as one resolver step.
func pathCostFeet(moverStart core.Hex, path []core.Hex) int {
	hexes := 0
	from := moverStart
	for _, to := range path {
		hexes += perception.HexDistance(from, to)
		from = to
	}
	return hexes * moveFeetPerHex
}

// Move applies a move action by playerID along path. Validates, mutates
// player position, and publishes the cause event (MoveEvent) plus a
// HexRevealedEvent for any viewer whose vision grew.
//
// Wave 2.11e: when a MovementResolver is wired (WithMovementResolver),
// Move iterates per-step calling resolver.ResolveStep per hex with a
// fresh buffered subscriber on ReactionTriggerTopic installed and torn
// down around each step (see iterateMovementSteps). This is the seam
// that lets the rulebook run MovementChain per step (so OA conditions
// fire) without the encounter SDK importing rulebook packages. When no
// resolver is wired, the legacy single-jump behavior is preserved for
// non-combat encounters.
//
// #714: when the mover has a hydrated, in-combat character (a seeded turn
// economy), Move enforces and spends its movement budget. The requested
// path's cost is checked against MovementRemaining BEFORE running any
// per-step chain — an over-budget request is rejected outright
// (ErrInsufficientMovement), with no state mutation, no OA triggering, no
// partial move. Once the move resolves (traveledPath may be shorter than the
// request if a chain subscriber like Disengage stops it early), the ACTUAL
// traveled distance is spent from the budget and a TurnStateChangedEvent
// pushes the new movement_remaining to the client (Invariant 12). A mover
// with no hydrated character, or one not currently in combat (no seeded
// economy — free-roam/pre-combat), is not gated: movement stays free, as
// before.
//
// Remaining slice scope: no turn-order enforcement, no path-contiguity
// validation beyond non-empty.
func (e *Encounter) Move(playerID core.PlayerID, path []core.Hex) error {
	if len(path) == 0 {
		return errors.New("empty path")
	}
	p, ok := e.data.Players[playerID]
	if !ok {
		return fmt.Errorf("player %q not in encounter", playerID)
	}

	moverStart := p.View.Position

	// Wall-blocked movement (rpg-toolkit#757): truncate the requested path at
	// the first wall-occupied hex before ANY downstream cost/resolver logic
	// sees it, so a blocked path is never partially budgeted or chain-run.
	// Nil room (no SpaceData) is a no-op — pre-wave-1 unblocked movement.
	path = e.truncateAtWall(path)
	if len(path) == 0 {
		// Blocked at the very first requested hex. Nothing to publish —
		// position unchanged, no events fire, nothing spent. Mirrors the
		// movementResolver's own "prevented at first step" early return below.
		return nil
	}

	// rpg-toolkit#808: movement budgeting is structurally a turn-based-only
	// concept, gated on Mode as well as InCombat(). InCombat() alone isn't
	// enough — checkPocketCleared (combat.go) now tears down the held
	// player's economy on a non-terminal pocket exit (the primary #808
	// fix), but this Mode check is defense-in-depth: any FUTURE path that
	// leaves a stale in-combat economy behind in FREE_ROAM must still not
	// gate movement, rather than resurrecting this exact bug.
	char := e.heldCharacter(p.EntityID)
	tracksMovement := char != nil && char.InCombat() && e.data.Mode == core.ModeTurnBased
	if tracksMovement {
		requestedCost := pathCostFeet(moverStart, path)
		remaining := char.GetActionEconomy().MovementRemaining
		if requestedCost > remaining {
			return fmt.Errorf("%w: path costs %dft, %dft remaining",
				ErrInsufficientMovement, requestedCost, remaining)
		}
	}

	// Determine the actually-traveled path. Without a resolver, the legacy
	// single-jump path is the entire requested path. With a resolver, the
	// per-step iteration may truncate when chain subscribers prevent a
	// step (Disengage no-op, etc.).
	traveledPath := path
	if e.movementResolver != nil {
		traveled, err := e.iterateMovementSteps(p, moverStart, path)
		if err != nil {
			return err
		}
		traveledPath = traveled
		if len(traveledPath) == 0 {
			// Movement was prevented at the very first step. Nothing to
			// publish — position unchanged, no events fire, nothing spent.
			return nil
		}
	}

	// Spend the movement budget BEFORE mutating encounter-side state or
	// publishing the MoveEvent (#714 review). The affordability gate is the
	// pre-check above — it runs before ANY side effect and is the only place an
	// over-budget move is rejected ("rejected outright, no state mutation").
	// This spend is structurally guaranteed to succeed for a mover who is
	// still conscious (actualCost <= requestedCost, already checked against
	// remaining), so ordering it first means an unreachable spend failure
	// cannot leave a published MoveEvent behind it. actualCost may be less
	// than requestedCost when a chain subscriber truncated the path; a
	// zero-cost move (a degenerate path back to the same hex) spends nothing
	// and is not sent to the rules layer, which rejects non-positive
	// distances.
	//
	// Full atomicity is not achievable at this seam and is not claimed: the
	// MovementResolver already applied physical side effects (room position,
	// OA damage) during iterateMovementSteps, before the traveled distance —
	// and thus the spend — could be known. The pre-check is the atomic gate;
	// this is the accounting that follows a move that physically happened.
	//
	// #781: the p.HP > 0 guard handles the one way the spend CAN now
	// legitimately fail post-pre-check: an opportunity attack captured
	// during iterateMovementSteps above dropped the mover to 0 HP mid-move.
	// applyUnconsciousOnZeroHP (death.go) already zeroed their action
	// economy — including MovementRemaining — via the same char.EndTurn
	// call seedActorTurn's own downed-at-turn-start branch uses, the moment
	// they went unconscious. Without this guard, the spend below would fail
	// with a "0ft remaining" ErrInsufficientMovement even though the move
	// DID physically happen (position updates below; damage/Unconscious
	// already published inside iterateMovementSteps) — turning a real,
	// already-observable mutation into what looks like a no-op failure to
	// the caller. There is nothing left to spend against for an
	// incapacitated mover this turn regardless (their whole economy is
	// already zero), so skipping the spend is a pure no-op on the economy
	// side, not a bypass of anything.
	//
	// Reads p.HP (the encounter's own PlayerData snapshot) rather than
	// char.GetHitPoints(): p.HP is exactly the signal applyDamageToTarget /
	// applyCapturedDamage already used to decide a >0->0 transition
	// occurred and call applyUnconsciousOnZeroHP in the first place (npc.go)
	// — the same authoritative fact this guard needs, not a second,
	// potentially-unsynced source of truth.
	spentMovement := false
	if tracksMovement && p.HP > 0 {
		if actualCost := pathCostFeet(moverStart, traveledPath); actualCost > 0 {
			out, err := char.ExecuteAction(context.Background(), &dnd5eCharacter.ExecuteActionInput{
				ActionRef: refs.Actions.Move(),
				Distance:  actualCost,
			})
			if err != nil {
				return fmt.Errorf("spend movement economy: %w", err)
			}
			if !out.Success {
				// Structurally unreachable for a still-conscious mover:
				// actualCost <= requestedCost, and requestedCost was already
				// checked against remaining above; the only other way to
				// reach 0 remaining (going unconscious mid-move) is now
				// excluded by the guard on this if-statement. Surfaced
				// rather than swallowed so a future divergence between the
				// pre-check and the spend is loud, not silent.
				return fmt.Errorf("%w: %s", ErrInsufficientMovement, out.Error)
			}
			spentMovement = true
		}
	}

	corrID, err := e.applyAndPublishMove(p, playerID, moverStart, traveledPath)
	if err != nil {
		return err
	}

	// Push the post-move economy refresh so the client sees the new
	// movement_remaining live (Invariant 12), correlated to the MoveEvent that
	// caused it (Invariant 8). Only when movement was actually spent — a
	// zero-cost move changes no economy, so there is nothing to refresh.
	if spentMovement {
		if err := e.publishTurnStateChanged(p.EntityID, corrID); err != nil {
			return fmt.Errorf("publish turn-state changed: %w", err)
		}
	}

	// Combat-entry check (rpg-toolkit#757): the mover's view just updated,
	// so this is a mutation site where a player-monster visibility pair can
	// newly form. Runs after the MoveEvent/reveal/economy publishes above so
	// a client sees "you moved" before "combat starts" (cause before effect).
	if err := e.checkCombatEntry(); err != nil {
		return err
	}

	return nil
}

// iterateMovementSteps walks the path one hex at a time for the given
// PlayerData mover, calling the resolver per step and draining the
// ReactionTriggerTopic buffer. Returns the actually-traveled segment of
// the path (may be shorter than the input if a step was Prevented).
//
// Wave 2.11e NPC-OA-only scope (per director signoff on #658): the SDK
// drains triggers but does not partition or act on them. NPC OAs are
// resolved inline by the resolver impl (combat.MoveEntity →
// triggerOpportunityAttack → ResolveAttack runs end-to-end). Player-pause
// branch deferred to #665.
//
// Thin player-mover wrapper around iterateMovementStepsForEntity — kept so
// the player-direction call site (Encounter.Move) reads as before, while
// the NPC-direction (applyNPCMovement via #668) reuses the shared
// entity-agnostic walker.
func (e *Encounter) iterateMovementSteps(
	p *PlayerData, moverStart core.Hex, path []core.Hex,
) ([]core.Hex, error) {
	return e.iterateMovementStepsForEntity(p.EntityID, moverStart, path)
}

// iterateMovementStepsForEntity is the entity-agnostic per-step walker
// added in Wave 2.11e (#668). Same shape as iterateMovementSteps but
// takes the mover's EntityID directly so the NPC-direction caller
// (applyNPCMovement) can reuse the iteration mechanics without a
// PlayerData wrapper.
//
// Per #658 Q4 signoff: MovementStepInput carries no EntityType field —
// the SDK is direction-agnostic. The resolver impl differentiates by
// looking the entity up itself.
func (e *Encounter) iterateMovementStepsForEntity(
	moverID core.EntityID, moverStart core.Hex, path []core.Hex,
) ([]core.Hex, error) {
	traveled := make([]core.Hex, 0, len(path))
	from := moverStart
	for _, to := range path {
		// Each step runs inside an inner func so the buffered subscriber
		// is torn down via defer even if the resolver panics. Without
		// this, a panic in the rulebook chain would leak the subscription
		// and pollute subsequent encounter operations.
		type stepCapture struct {
			result *MovementStepResult
			rolls  []dnd5eEvents.PostAttackRollEvent
			dmg    []dnd5eEvents.DamageReceivedEvent
		}
		capture, stepErr := func() (stepCapture, error) {
			// Install a buffered subscriber on ReactionTriggerTopic per
			// step. The subscriber catches any triggers that chain
			// subscribers publish during this step's resolver call. In
			// NPC-OA-only scope the SDK doesn't act on captured triggers
			// (the resolver impl resolves them inline), but the buffer is
			// installed for shape parity with TakeActionPhased and to
			// flush subscriptions cleanly per step.
			_, drainCleanup, err := e.installTriggerBuffer()
			if err != nil {
				return stepCapture{}, fmt.Errorf("install trigger buffer: %w", err)
			}
			defer drainCleanup()

			// Wave 2.11e (#675): capture DamageReceivedEvent published by
			// the resolver's chain (combat.MoveEntity →
			// triggerOpportunityAttack → ResolveAttack). OA fires AND
			// resolves end-to-end inside the rulebook; the encounter SDK
			// observes the damage post-hoc on the bus and applies HP delta
			// + encounter-side DamageDealtEvent below.
			ctx := context.Background()
			dmgCaptured, unsubDmg, err := subscribeDamage(ctx, e.bus)
			if err != nil {
				return stepCapture{}, fmt.Errorf("subscribe move damage: %w", err)
			}
			defer func() { _ = unsubDmg() }()

			// #715: capture PostAttackRollEvent published by
			// combat.ResolveAttackHit — unconditionally, hit or miss —
			// alongside the damage subscription above. This is the only
			// bus-observable signal that an OA resolved with the roll
			// detail (roll, bonus, AC) needed to publish AttackResolvedEvent
			// in the same shape as the normal attack path (attackResolved
			// always, damage only on hit). The resolver interface itself
			// returns only Prevented/PreventReason — it does not surface
			// attack outcomes directly.
			rollsCaptured, unsubRolls, err := subscribeAttackRolls(ctx, e.bus)
			if err != nil {
				return stepCapture{}, fmt.Errorf("subscribe move attack rolls: %w", err)
			}
			defer func() { _ = unsubRolls() }()

			res, err := e.movementResolver.ResolveStep(MovementStepInput{
				EntityID: moverID,
				FromHex:  from,
				ToHex:    to,
				EventBus: e.bus,
				Mover:    e.combatantFor(moverID),
			})
			out := stepCapture{result: res}
			if dmgCaptured != nil {
				out.dmg = *dmgCaptured
			}
			if rollsCaptured != nil {
				out.rolls = *rollsCaptured
			}
			return out, err
		}()
		if stepErr != nil {
			return traveled, fmt.Errorf("resolve movement step: %w", stepErr)
		}
		result := capture.result
		if result == nil {
			return traveled, fmt.Errorf("movement resolver: nil result with nil error")
		}

		// Apply any OA attack outcomes captured during this step. Done
		// before the Prevented check because OAs fire whether or not the
		// chain ends up preventing the step — the attack (and any damage)
		// applies either way.
		//
		// context.Background(): iterateMovementStepsForEntity and Move (its
		// only caller) don't thread a ctx through this call chain today,
		// mirroring the ctx := context.Background() already used a few
		// lines up in this same function's per-step closure (#733: needed
		// by applyMoveAttackOutcomes -> applyMoveDamage -> the new
		// applyUnconsciousOnZeroHP helper's ConditionAppliedTopic publish).
		if len(capture.rolls) > 0 || len(capture.dmg) > 0 {
			if err := e.applyMoveAttackOutcomes(context.Background(), capture.rolls, capture.dmg); err != nil {
				return traveled, fmt.Errorf("apply move attack outcomes: %w", err)
			}
		}

		if result.Prevented {
			// Chain subscriber blocked the step. Stop here; do not advance
			// to `to`. The traveled slice so far is the actually-moved path.
			break
		}

		traveled = append(traveled, to)
		from = to
	}
	return traveled, nil
}

// applyAndPublishMove mutates the player's position to the final hex of
// traveledPath, computes per-viewer projections, and publishes the move +
// reveal + visibility-transition events. Shared between the legacy
// single-jump path (traveledPath = full input) and the resolver-mediated
// per-step path (traveledPath = actually-moved segments, possibly
// truncated by chain prevention).
//
// Wave 2.11e (#658 Q3): events carry the truncated traveled path, not the
// requested path. Wire clients see the actual outcome, not the intent.
//
// Returns the correlation id derived from the published MoveEvent's sequence
// (#714) so the caller can tie a follow-on TurnStateChanged economy refresh
// to the move that caused it (Invariant 8), the same shape
// publishActionResolved uses for other verbs.
func (e *Encounter) applyAndPublishMove(
	p *PlayerData, playerID core.PlayerID, moverStart core.Hex, traveledPath []core.Hex,
) (core.CorrelationID, error) {
	// 1. Compute the mover's reveal delta BEFORE mutating position/view.
	//    - moverStart is needed for visibility-transition detection (so viewers
	//      can determine if the mover was visible to them before the move).
	//    - The reveal delta = (visible-from-new-position) MINUS (already-revealed).
	//      Critical: if we apply the reveal first, the diff is always empty.
	end := traveledPath[len(traveledPath)-1]
	newVisible := perception.VisibleHexesAt(end, p.View.SightRange, e.room)
	moverNewHexes := diffHexes(p.View.RevealedHexes, newVisible)

	// 2. Mutate state: position, then apply the reveal delta we just computed.
	p.View.Position = end
	p.View.ApplyReveal(moverNewHexes)

	// 3. Per-player projection.
	movePerPlayer := make(map[core.PlayerID]events.MovePlayerSlice)
	revealPerPlayer := make(map[core.PlayerID]events.HexRevealedSlice)

	// The mover always sees their own move; their reveal is the delta we
	// just computed.
	movePerPlayer[playerID] = events.MovePlayerSlice{
		SeenSegments: append([]core.Hex(nil), traveledPath...),
	}
	if len(moverNewHexes) > 0 {
		revealPerPlayer[playerID] = events.HexRevealedSlice{Hexes: moverNewHexes}
	}

	// Other players: project the move from their current view.
	//
	// Also accumulate visibility-transition data for EntityAppeared /
	// EntityDisappeared events. appearedByHex maps the hex where the mover
	// became visible to the set of viewers who see the appearance at that hex.
	// disappearedPerPlayer maps each viewer to the hex where they last saw the mover.
	appearedByHex := make(map[core.Hex]map[core.PlayerID]struct{})
	disappearedPerPlayer := make(map[core.PlayerID]core.Hex)

	for otherID, other := range e.data.Players {
		if otherID == playerID {
			continue
		}
		// ProjectMove returns the visible set so we can pass it directly to
		// ProjectVisibilityTransition without recomputing VisibleHexesAt.
		moveSlice, revealSlice, visible := perception.ProjectMove(p.EntityID, traveledPath, other.View, e.room)
		if moveSlice != nil {
			movePerPlayer[otherID] = *moveSlice
		}
		if revealSlice != nil {
			if revealSlice.Hexes != nil {
				other.View.ApplyReveal(revealSlice.Hexes)
			}
			revealPerPlayer[otherID] = *revealSlice
		}

		// Determine visibility transitions for this viewer.
		var seenSegments []core.Hex
		if moveSlice != nil {
			seenSegments = moveSlice.SeenSegments
		}
		appearedAt, disappearedAt := perception.ProjectVisibilityTransition(
			moverStart, traveledPath, seenSegments, other.View, visible,
		)
		if appearedAt != nil {
			if appearedByHex[*appearedAt] == nil {
				appearedByHex[*appearedAt] = make(map[core.PlayerID]struct{})
			}
			appearedByHex[*appearedAt][otherID] = struct{}{}
		}
		if disappearedAt != nil {
			disappearedPerPlayer[otherID] = *disappearedAt
		}
	}

	// 4. Publish — cause event always; effect event only when someone's
	//    vision changed. The two events get sequential sequence numbers.
	moveSeq := e.nextSeq()
	corrID := e.correlationFor(moveSeq)
	if err := e.broker.Publish(events.NewMoveEvent(
		e.data.ID, moveSeq, p.EntityID, moverStart, traveledPath, movePerPlayer,
	)); err != nil {
		return "", fmt.Errorf("publish move: %w", err)
	}
	if len(revealPerPlayer) > 0 {
		if err := e.broker.Publish(events.NewHexRevealedEvent(
			e.data.ID, e.nextSeq(), revealPerPlayer,
		)); err != nil {
			return "", fmt.Errorf("publish reveal: %w", err)
		}
	}
	if err := e.publishRevealedObstacles(revealPerPlayer); err != nil {
		return "", err
	}

	// Emit EntityAppearedEvent once per distinct appeared-at hex, grouping
	// viewers who share the same appearance position. Under the endpoints-only
	// model this is typically a single hex (path[len-1] for enter-LoS), but
	// pass-through viewers at different positions can yield different
	// SeenSegments[0] hexes, producing distinct groups.
	for hex, viewers := range appearedByHex {
		if err := e.broker.Publish(events.NewEntityAppearedEvent(
			e.data.ID, e.nextSeq(), p.EntityID, hex, viewers,
		)); err != nil {
			return "", fmt.Errorf("publish entity appeared: %w", err)
		}
	}

	// Emit EntityDisappearedEvent as a single event carrying per-viewer
	// last-known hexes (different viewers may have last seen the mover at
	// different hexes during a pass-through move).
	if len(disappearedPerPlayer) > 0 {
		if err := e.broker.Publish(events.NewEntityDisappearedEvent(
			e.data.ID, e.nextSeq(), p.EntityID, disappearedPerPlayer,
		)); err != nil {
			return "", fmt.Errorf("publish entity disappeared: %w", err)
		}
	}

	// Emit EntityAppeared/EntityDisappeared for MONSTERS whose visibility to
	// playerID (the mover) changed during this move (rpg-toolkit#761).
	// Published here — not from inside checkCombatEntry — because
	// checkCombatEntry's mode gate short-circuits once the encounter has left
	// FREE_ROAM (it exists to make repeated Move/AddMonster calls idempotent
	// for the ENTRY transition only); wiring detection there would silently
	// stop firing appear/disappear events for the rest of the encounter the
	// moment combat starts, missing the ongoing-combat case (a player
	// rounding a corner mid-fight) this issue also needs. applyAndPublishMove
	// runs on every Move regardless of mode and already computes the
	// identical player-sees-player transition just above, so the monster
	// case is wired in right alongside it, sharing the same machinery.
	monsterAppeared, monsterDisappeared := e.monsterVisibilityTransitions(
		p.EntityID, p.View.SightRange, moverStart, traveledPath,
	)
	for _, m := range monsterAppeared {
		if err := e.broker.Publish(events.NewEntityAppearedEvent(
			e.data.ID, e.nextSeq(), m.ID, m.Position,
			map[core.PlayerID]struct{}{playerID: {}},
		)); err != nil {
			return "", fmt.Errorf("publish monster appeared: %w", err)
		}
	}
	for _, m := range monsterDisappeared {
		if err := e.broker.Publish(events.NewEntityDisappearedEvent(
			e.data.ID, e.nextSeq(), m.ID,
			map[core.PlayerID]core.Hex{playerID: m.Position},
		)); err != nil {
			return "", fmt.Errorf("publish monster disappeared: %w", err)
		}
	}
	return corrID, nil
}

// OpenDoor applies an open-door action. Marks the door open, rebuilds
// e.room so the door's cell stops blocking movement/LoS (rpg-toolkit#790 —
// see rebuildRoomFromData), and publishes the cause event (DoorOpenedEvent)
// plus a HexRevealedEvent for any viewer whose vision grew — the room
// rebuild happens BEFORE the per-viewer ProjectDoorOpen calls below so
// their VisibleHexesAt sees through the now-open doorway. On rebuild
// failure, door.Open is rolled back to false (a Copilot catch on this PR)
// so DoorData and e.room can't disagree about whether the door blocks.
//
// Stale-reference caveat: rebuildRoomFromData replaces e.room and
// e.roomOrchestrator with FRESH objects rather than mutating the existing
// ones in place. A caller that fetched Room()/RoomOrchestrator() before
// this call and holds onto that reference (e.g. a host wiring the spawn
// engine to "the encounter's room" once at startup) will keep observing
// the pre-open geometry — it must re-fetch Room()/RoomOrchestrator() after
// any OpenDoor/AddDoor call rather than caching them long-term. No caller
// does this yet (doors are toolkit-only through Slice 1), but it's a live
// trap for Slice 2+ wiring.
func (e *Encounter) OpenDoor(playerID core.PlayerID, doorID core.EntityID) error {
	p, ok := e.data.Players[playerID]
	if !ok {
		return fmt.Errorf("player %q not in encounter", playerID)
	}
	door, ok := e.data.Doors[doorID]
	if !ok {
		return fmt.Errorf("door %q not in encounter", doorID)
	}
	if door.Open {
		return fmt.Errorf("door %q already open", doorID)
	}

	door.Open = true
	if e.data.Space != nil {
		if err := e.rebuildRoomFromData(); err != nil {
			door.Open = false
			return fmt.Errorf("rebuild room after opening door %q: %w", doorID, err)
		}
	}

	doorPerPlayer := make(map[core.PlayerID]events.DoorOpenedPlayerSlice)
	revealPerPlayer := make(map[core.PlayerID]events.HexRevealedSlice)

	for viewerID, viewer := range e.data.Players {
		doorSlice, revealSlice := perception.ProjectDoorOpen(
			doorID, door.Position, p.EntityID, viewer.View, e.room,
		)
		if doorSlice != nil {
			doorPerPlayer[viewerID] = *doorSlice
		}
		if revealSlice != nil {
			if revealSlice.Hexes != nil {
				viewer.View.ApplyReveal(revealSlice.Hexes)
			}
			revealPerPlayer[viewerID] = *revealSlice
		}
	}

	if err := e.broker.Publish(events.NewDoorOpenedEvent(
		e.data.ID, e.nextSeq(), doorID, p.EntityID, doorPerPlayer,
	)); err != nil {
		return fmt.Errorf("publish door: %w", err)
	}
	if len(revealPerPlayer) > 0 {
		if err := e.broker.Publish(events.NewHexRevealedEvent(
			e.data.ID, e.nextSeq(), revealPerPlayer,
		)); err != nil {
			return fmt.Errorf("publish reveal: %w", err)
		}
	}
	if err := e.publishRevealedObstacles(revealPerPlayer); err != nil {
		return err
	}
	return nil
}

// publishRevealedObstacles announces each static obstacle exactly once for a
// reveal transition. Obstacles use sticky explored-map semantics: only viewers
// whose newly revealed hex delta contains an obstacle position are included,
// and the caller has already persisted that delta into their View.
func (e *Encounter) publishRevealedObstacles(reveals map[core.PlayerID]events.HexRevealedSlice) error {
	if e.data.Space == nil || len(reveals) == 0 || len(e.data.Space.Obstacles) == 0 {
		return nil
	}

	obstacles := append([]ObstacleData(nil), e.data.Space.Obstacles...)
	sort.Slice(obstacles, func(i, j int) bool {
		if obstacles[i].ID != obstacles[j].ID {
			return obstacles[i].ID < obstacles[j].ID
		}
		if obstacles[i].Position.Q != obstacles[j].Position.Q {
			return obstacles[i].Position.Q < obstacles[j].Position.Q
		}
		if obstacles[i].Position.R != obstacles[j].Position.R {
			return obstacles[i].Position.R < obstacles[j].Position.R
		}
		return obstacles[i].Position.S < obstacles[j].Position.S
	})

	for _, obstacle := range obstacles {
		audience := make(map[core.PlayerID]struct{})
		for playerID, reveal := range reveals {
			if reveal.Hexes.Has(obstacle.Position) {
				audience[playerID] = struct{}{}
			}
		}
		if len(audience) == 0 {
			continue
		}
		if err := e.broker.Publish(events.NewEntityAppearedEvent(
			e.data.ID, e.nextSeq(), obstacle.ID, obstacle.Position, audience,
		)); err != nil {
			return fmt.Errorf("publish revealed obstacle %q: %w", obstacle.ID, err)
		}
	}
	return nil
}

// diffHexes returns hexes in candidate that are not already in current.
func diffHexes(current, candidate core.HexSet) core.HexSet {
	out := make(core.HexSet)
	for h := range candidate {
		if !current.Has(h) {
			out[h] = struct{}{}
		}
	}
	return out
}
