package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/types"
)

// MoveEvent is published when an entity moves through hexes in the encounter.
//
// Vision changes caused by the move are NOT embedded here — they ride on a
// parallel HexRevealedEvent published alongside this one. See the decoupled
// cause/effect decision in sdk-direction.md.
type MoveEvent struct {
	encID     types.EncounterID
	seq       uint64
	Mover     types.EntityID
	Path      []types.Hex
	PerPlayer map[types.PlayerID]MovePlayerSlice
}

// MovePlayerSlice is each viewer's projection of the move — which hexes
// of the path they saw the mover traverse.
//
// Vision changes from the move (newly-revealed hexes/entities) are NOT
// embedded here — they ride on a parallel HexRevealedEvent.
type MovePlayerSlice struct {
	SeenSegments []types.Hex `json:"seen_segments"`
}

// NewMoveEvent constructs a MoveEvent. The encounter is responsible for
// stamping the encounter ID and sequence number; PerPlayer is computed
// by perception.ProjectMove.
func NewMoveEvent(
	encID types.EncounterID,
	seq uint64,
	mover types.EntityID,
	path []types.Hex,
	perPlayer map[types.PlayerID]MovePlayerSlice,
) *MoveEvent {
	return &MoveEvent{
		encID:     encID,
		seq:       seq,
		Mover:     mover,
		Path:      path,
		PerPlayer: perPlayer,
	}
}

func (*MoveEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *MoveEvent) EncounterID() types.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *MoveEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive this event,
// derived from the keys of PerPlayer.
func (e *MoveEvent) Audience() types.AudienceSet { return audienceFromMap(e.PerPlayer) }

// moveEventWire is the on-wire shape — used only by MarshalJSON / UnmarshalJSON.
// Keeping this private (alongside the unexported encID/seq fields) preserves
// the construction invariant: only NewMoveEvent and UnmarshalJSON can set
// the encounter ID and sequence number.
type moveEventWire struct {
	EncID     types.EncounterID                  `json:"encounter_id"`
	Seq       uint64                             `json:"sequence"`
	Mover     types.EntityID                     `json:"mover"`
	Path      []types.Hex                        `json:"path"`
	PerPlayer map[types.PlayerID]MovePlayerSlice `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names without
// making the Go fields exported. Implements encoding/json.Marshaler.
func (e *MoveEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(moveEventWire{
		EncID:     e.encID,
		Seq:       e.seq,
		Mover:     e.Mover,
		Path:      e.Path,
		PerPlayer: e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *MoveEvent) UnmarshalJSON(b []byte) error {
	var w moveEventWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.encID = w.EncID
	e.seq = w.Seq
	e.Mover = w.Mover
	e.Path = w.Path
	e.PerPlayer = w.PerPlayer
	return nil
}
