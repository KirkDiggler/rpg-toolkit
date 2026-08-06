package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// EntityAppearedEvent is published when a moving entity enters one or more
// viewers' lines of sight during a move, or when a static obstacle enters one
// or more viewers' sticky explored-map reveal delta. This event lets consumers
// show entity markers only to viewers who newly learned about the entity.
//
// Position is the hex where a mover became visible or the static obstacle's
// fixed position. Under the endpoints-only visibility model, all viewers in
// the audience will see a mover appear at the same hex (path[len-1] for
// enter-LoS; SeenSegments[0] for pass-through). If viewers share different
// appearedAt hexes (only possible in pass-through when viewers sit at different
// positions and thus differ in which hex was first visible), Move() emits one
// EntityAppearedEvent per distinct Position with the viewers grouped into
// PerPlayer. Static obstacles group viewers whose newly revealed hex delta
// contains the same fixed obstacle position.
type EntityAppearedEvent struct {
	eventMeta
	encID     core.EncounterID
	seq       uint64
	Entity    core.EntityID
	Position  core.Hex
	PerPlayer map[core.PlayerID]struct{}
	// Observations is each recipient's immutable-at-publish observation of Position.
	Observations map[core.PlayerID]KnownHex `json:"observations,omitempty"`
}

// NewEntityAppearedEvent constructs an EntityAppearedEvent. The encounter is
// responsible for stamping the encounter ID and sequence number.
func NewEntityAppearedEvent(
	encID core.EncounterID,
	seq uint64,
	entity core.EntityID,
	position core.Hex,
	perPlayer map[core.PlayerID]struct{},
	observations ...map[core.PlayerID]KnownHex,
) *EntityAppearedEvent {
	var observed map[core.PlayerID]KnownHex
	if len(observations) > 0 {
		observed = observations[0]
	}
	return &EntityAppearedEvent{
		encID:        encID,
		seq:          seq,
		Entity:       entity,
		Position:     position,
		PerPlayer:    perPlayer,
		Observations: observed,
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
	metaWire
	EncID        core.EncounterID           `json:"encounter_id"`
	Seq          uint64                     `json:"sequence"`
	Entity       core.EntityID              `json:"entity"`
	Position     core.Hex                   `json:"position"`
	PerPlayer    map[core.PlayerID]struct{} `json:"per_player"`
	Observations map[core.PlayerID]KnownHex `json:"observations,omitempty"`
}

// MarshalJSON exposes encID and seq under stable JSON field names without
// making the Go fields exported. Implements encoding/json.Marshaler.
func (e *EntityAppearedEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(entityAppearedWire{
		metaWire:     e.toWire(),
		EncID:        e.encID,
		Seq:          e.seq,
		Entity:       e.Entity,
		Position:     e.Position,
		PerPlayer:    e.PerPlayer,
		Observations: e.Observations,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *EntityAppearedEvent) UnmarshalJSON(b []byte) error {
	var w entityAppearedWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.Entity = w.Entity
	e.Position = w.Position
	e.PerPlayer = w.PerPlayer
	e.Observations = w.Observations
	return nil
}
