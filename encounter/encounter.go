package encounter

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	dnd5events "github.com/KirkDiggler/rpg-toolkit/events"
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
// TakeAction iff MaxHP > 0, AC > 0, and DamageDice is non-empty (see
// isPlayerCombatant in combat.go). AttackBonus may be 0 (no proficiency)
// and DamageType defaults to "untyped" when empty. Seats added without
// these fields cannot call combat verbs and TakeAction returns
// ErrNonCombatant for them.
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
func New(id core.EncounterID, b *Broker, opts ...Option) *Encounter {
	e := &Encounter{
		data:   NewData(id),
		broker: b,
		roller: dice.NewRoller(),
		bus:    dnd5events.NewEventBus(),
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

// LoadFromData rehydrates an encounter from persisted state.
//
// Wave 2.11c: a fresh encounter-scoped dnd5e EventBus is created at this
// point. Callers (typically rpg-api) then rehydrate character conditions
// via the bus exposed by EventBus() so subscriptions persist across attacks.
func LoadFromData(data *Data, b *Broker, opts ...Option) (*Encounter, error) {
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
		data:   data,
		broker: b,
		roller: dice.NewRoller(),
		bus:    dnd5events.NewEventBus(),
	}
	for _, o := range opts {
		o(e)
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
	view.ApplyReveal(perception.VisibleHexesAt(input.Position, input.SightRange))

	e.data.Players[input.PlayerID] = &PlayerData{
		ID:          input.PlayerID,
		EntityID:    input.EntityID,
		View:        view,
		HP:          input.HP,
		MaxHP:       input.MaxHP,
		AC:          input.AC,
		AttackBonus: input.AttackBonus,
		DamageDice:  input.DamageDice,
		DamageType:  input.DamageType,
	}
	// Seed default OA readiness for combatants. Free-cost reactions default
	// on so players do not need to opt in every fight.
	if input.DamageDice != "" && input.EntityID != "" {
		e.seedOAReadiness(input.EntityID)
	}
	return nil
}

// AddDoor registers a door (slice scope; future slices use a richer entity
// system).
func (e *Encounter) AddDoor(id core.EntityID, position core.Hex, open bool) {
	e.data.Doors[id] = &DoorData{ID: id, Position: position, Open: open}
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
	return nil
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
func (e *Encounter) ToData() *Data { return e.data }

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
// Slice scope: no action economy, no turn-order enforcement, no
// path-contiguity validation beyond non-empty.
func (e *Encounter) Move(playerID core.PlayerID, path []core.Hex) error {
	if len(path) == 0 {
		return errors.New("empty path")
	}
	p, ok := e.data.Players[playerID]
	if !ok {
		return fmt.Errorf("player %q not in encounter", playerID)
	}

	moverStart := p.View.Position

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
			// publish — position unchanged, no events fire.
			return nil
		}
	}

	return e.applyAndPublishMove(p, playerID, moverStart, traveledPath)
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
		result, stepErr := func() (*MovementStepResult, error) {
			// Install a buffered subscriber on ReactionTriggerTopic per
			// step. The subscriber catches any triggers that chain
			// subscribers publish during this step's resolver call. In
			// NPC-OA-only scope the SDK doesn't act on captured triggers
			// (the resolver impl resolves them inline), but the buffer is
			// installed for shape parity with TakeActionPhased and to
			// flush subscriptions cleanly per step.
			_, drainCleanup, err := e.installTriggerBuffer()
			if err != nil {
				return nil, fmt.Errorf("install trigger buffer: %w", err)
			}
			defer drainCleanup()

			return e.movementResolver.ResolveStep(MovementStepInput{
				EntityID: moverID,
				FromHex:  from,
				ToHex:    to,
				EventBus: e.bus,
			})
		}()
		if stepErr != nil {
			return traveled, fmt.Errorf("resolve movement step: %w", stepErr)
		}
		if result == nil {
			return traveled, fmt.Errorf("movement resolver: nil result with nil error")
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
func (e *Encounter) applyAndPublishMove(
	p *PlayerData, playerID core.PlayerID, moverStart core.Hex, traveledPath []core.Hex,
) error {
	// 1. Compute the mover's reveal delta BEFORE mutating position/view.
	//    - moverStart is needed for visibility-transition detection (so viewers
	//      can determine if the mover was visible to them before the move).
	//    - The reveal delta = (visible-from-new-position) MINUS (already-revealed).
	//      Critical: if we apply the reveal first, the diff is always empty.
	end := traveledPath[len(traveledPath)-1]
	newVisible := perception.VisibleHexesAt(end, p.View.SightRange)
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
		moveSlice, revealSlice, visible := perception.ProjectMove(p.EntityID, traveledPath, other.View)
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
	if err := e.broker.Publish(events.NewMoveEvent(
		e.data.ID, e.nextSeq(), p.EntityID, traveledPath, movePerPlayer,
	)); err != nil {
		return fmt.Errorf("publish move: %w", err)
	}
	if len(revealPerPlayer) > 0 {
		if err := e.broker.Publish(events.NewHexRevealedEvent(
			e.data.ID, e.nextSeq(), revealPerPlayer,
		)); err != nil {
			return fmt.Errorf("publish reveal: %w", err)
		}
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
			return fmt.Errorf("publish entity appeared: %w", err)
		}
	}

	// Emit EntityDisappearedEvent as a single event carrying per-viewer
	// last-known hexes (different viewers may have last seen the mover at
	// different hexes during a pass-through move).
	if len(disappearedPerPlayer) > 0 {
		if err := e.broker.Publish(events.NewEntityDisappearedEvent(
			e.data.ID, e.nextSeq(), p.EntityID, disappearedPerPlayer,
		)); err != nil {
			return fmt.Errorf("publish entity disappeared: %w", err)
		}
	}
	return nil
}

// OpenDoor applies an open-door action. Marks the door open and publishes
// the cause event (DoorOpenedEvent) plus a HexRevealedEvent for any viewer
// whose vision grew.
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

	doorPerPlayer := make(map[core.PlayerID]events.DoorOpenedPlayerSlice)
	revealPerPlayer := make(map[core.PlayerID]events.HexRevealedSlice)

	for viewerID, viewer := range e.data.Players {
		doorSlice, revealSlice := perception.ProjectDoorOpen(
			doorID, door.Position, p.EntityID, viewer.View,
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
