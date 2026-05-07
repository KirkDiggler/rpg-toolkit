package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// EntityAppearedEvent is published when a moving entity enters one or more
// viewers' lines of sight during a move. This event lets consumers show entity
// markers for viewers who could not see the mover before.
//
// Position is the hex where the mover became visible. Under the endpoints-only
// visibility model, all viewers in the audience will see the mover appear at
// the same hex (path[len-1] for enter-LoS; SeenSegments[0] for pass-through).
// If two viewers share different appearedAt hexes (only possible in pass-through
// when viewers sit at different positions and thus differ in which hex was the
// first visible one), Move() emits one EntityAppearedEvent per distinct Position
// with the viewers for that position grouped into PerPlayer.
type EntityAppearedEvent struct {
	encID     core.EncounterID
	seq       uint64
	Entity    core.EntityID
	Position  core.Hex
	PerPlayer map[core.PlayerID]struct{}
}

// NewEntityAppearedEvent constructs an EntityAppearedEvent. The encounter is
// responsible for stamping the encounter ID and sequence number.
func NewEntityAppearedEvent(
	encID core.EncounterID,
	seq uint64,
	entity core.EntityID,
	position core.Hex,
	perPlayer map[core.PlayerID]struct{},
) *EntityAppearedEvent {
	return &EntityAppearedEvent{
		encID:     encID,
		seq:       seq,
		Entity:    entity,
		Position:  position,
		PerPlayer: perPlayer,
	}
}

func (*EntityAppearedEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *EntityAppearedEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *EntityAppearedEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who newly see the entity, derived from
// the keys of PerPlayer.
func (e *EntityAppearedEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

// entityAppearedWire is the on-wire shape — used only by MarshalJSON / UnmarshalJSON.
type entityAppearedWire struct {
	EncID     core.EncounterID           `json:"encounter_id"`
	Seq       uint64                     `json:"sequence"`
	Entity    core.EntityID              `json:"entity"`
	Position  core.Hex                   `json:"position"`
	PerPlayer map[core.PlayerID]struct{} `json:"per_player"`
}

// MarshalJSON exposes encID and seq under stable JSON field names without
// making the Go fields exported. Implements encoding/json.Marshaler.
func (e *EntityAppearedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(entityAppearedWire{
		EncID:     e.encID,
		Seq:       e.seq,
		Entity:    e.Entity,
		Position:  e.Position,
		PerPlayer: e.PerPlayer,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *EntityAppearedEvent) UnmarshalJSON(b []byte) error {
	var w entityAppearedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.encID = w.EncID
	e.seq = w.Seq
	e.Entity = w.Entity
	e.Position = w.Position
	e.PerPlayer = w.PerPlayer
	return nil
}
