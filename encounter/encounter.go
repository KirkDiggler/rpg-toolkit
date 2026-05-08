package encounter

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
)

// Encounter is the transient SDK object for one ongoing encounter.
// Constructed per-call via LoadFromData; mutated by verbs; serialized via
// ToData and saved.
type Encounter struct {
	data   *Data
	broker *Broker
	roller dice.Roller
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

// PlayerInput populates a player seat at construction / AddPlayer time.
//
// Combat fields (HP / AC / AttackBonus / DamageDice / DamageType) are
// optional; when zero they remain unset on PlayerData and combat verbs
// that read them treat the player as a non-combatant.
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
	}
	for _, o := range opts {
		o(e)
	}
	return e
}

// LoadFromData rehydrates an encounter from persisted state.
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
	if data.Mode == core.ModeUnspecified {
		data.Mode = core.ModeFreeRoam
	}
	e := &Encounter{data: data, broker: b, roller: dice.NewRoller()}
	for _, o := range opts {
		o(e)
	}
	return e, nil
}

// AddPlayer registers a new player seat with a fresh PerceptionView.
// The player sees their starting position and surrounding hexes immediately.
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
	return nil
}

// AddDoor registers a door (slice scope; future slices use a richer entity
// system).
func (e *Encounter) AddDoor(id core.EntityID, position core.Hex, open bool) {
	e.data.Doors[id] = &DoorData{ID: id, Position: position, Open: open}
}

// AddMonster registers a monster seat. Mirrors AddPlayer / AddDoor and is
// the primary fixture verb for tests and orchestrator-driven seeding.
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
	return nil
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

	// 1. Capture starting position and compute the mover's reveal delta BEFORE
	//    mutating position/view.
	//    - moverStart is needed for visibility-transition detection (so viewers
	//      can determine if the mover was visible to them before the move).
	//    - The reveal delta = (visible-from-new-position) MINUS (already-revealed).
	//      Critical: if we apply the reveal first, the diff is always empty.
	moverStart := p.View.Position
	end := path[len(path)-1]
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
		SeenSegments: append([]core.Hex(nil), path...),
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
		moveSlice, revealSlice, visible := perception.ProjectMove(p.EntityID, path, other.View)
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
			moverStart, path, seenSegments, other.View, visible,
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
		e.data.ID, e.nextSeq(), p.EntityID, path, movePerPlayer,
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
