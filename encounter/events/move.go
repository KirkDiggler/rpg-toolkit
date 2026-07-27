package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// MoveEvent is published when an entity moves through hexes in the encounter.
//
// Vision changes caused by the move are NOT embedded here — they ride on a
// parallel HexRevealedEvent published alongside this one. See the decoupled
// cause/effect decision in sdk-direction.md.
type MoveEvent struct {
	eventMeta
	encID core.EncounterID
	seq   uint64
	Mover core.EntityID
	// From is the mover's position BEFORE this move (#714). Path is the
	// traveled destinations — the encounter's own publish paths build it from
	// completed steps and do not prepend the origin — so a consumer deriving
	// "from" via Path[0] risks reading the first destination, not the origin.
	// (The SDK does not validate Path contents supplied by callers, so treat
	// this as guidance, not an invariant.) Use From for the true starting hex
	// regardless of what Path contains.
	From      core.Hex
	Path      []core.Hex
	PerPlayer map[core.PlayerID]MovePlayerSlice
	// MoverPlayerID is the player who controls Mover, when Mover is
	// player-controlled — empty for an NPC/monster mover. Set directly from
	// the same playerID the encounter's own Move(playerID, ...) call
	// already has in hand, so a consumer deciding "is THIS viewer the
	// mover's own viewer" (to attach MoverKnownHexes) never needs its own
	// entity-id -> player-id repository lookup either — the same
	// stale-read class MoverKnownHexes itself exists to close, just for a
	// smaller, easier-to-miss piece of data.
	MoverPlayerID core.PlayerID
	// MoverKnownHexes is the MOVER's OWN complete current knowledge
	// (KnownHexes(Mover)'s wire projection — see KnownHex's doc), computed
	// at the exact moment this event is published, i.e. immediately after
	// the mover's position/view mutation above and before anything else can
	// observe or persist this encounter.
	//
	// This exists because HexRevealedEvent — the toolkit's only other
	// vision-change signal — fires exclusively on vision GAIN
	// (perception.ProjectMove never reports a LOSS). A move that purely
	// loses sight of a hex (stepping behind a pillar, backing out of a
	// room) publishes no HexRevealedEvent at all, so a consumer with no
	// other signal never learns that hex demoted to remembered.
	//
	// A consumer that needs the mover's current knowledge (e.g. to restate
	// it to the mover's own client) MUST read this field rather than doing
	// its own out-of-band re-load-and-KnownHexes(Mover) call: this
	// encounter's in-memory state at publish time is the ONLY moment
	// guaranteed to reflect this exact move — a caller's own repository
	// read racing against the SAME move's not-yet-executed persist would
	// see pre-move state (KirkDiggler/rpg-api#737: exactly this race, caught live —
	// the restatement reported the mover's old hex as still VISIBLE with
	// them on it, in the very event meant to correct that).
	//
	// Empty (not nil) when Mover has no view (e.g. a non-player-controlled
	// mover) — never nil, so a consumer can range over it unconditionally.
	MoverKnownHexes []KnownHex
}

// MovePlayerSlice is each viewer's projection of the move — which hexes
// of the path they saw the mover traverse.
//
// Vision changes from the move (newly-revealed hexes/entities) are NOT
// embedded here — they ride on a parallel HexRevealedEvent.
type MovePlayerSlice struct {
	SeenSegments []core.Hex `json:"seen_segments"`
}

// NewMoveEvent constructs a MoveEvent. The encounter is responsible for
// stamping the encounter ID and sequence number; PerPlayer is computed by
// perception.ProjectMove. moverPlayerID is empty for a non-player-controlled
// mover (an NPC/monster). moverKnownHexes is the mover's own
// KnownHexes(mover), projected to KnownHex by the caller — see
// MoverKnownHexes' doc for why this must be read at publish time, not
// re-derived later. A nil moverKnownHexes is normalized to an empty slice.
func NewMoveEvent(
	encID core.EncounterID,
	seq uint64,
	mover core.EntityID,
	from core.Hex,
	path []core.Hex,
	perPlayer map[core.PlayerID]MovePlayerSlice,
	moverPlayerID core.PlayerID,
	moverKnownHexes []KnownHex,
) *MoveEvent {
	if moverKnownHexes == nil {
		moverKnownHexes = []KnownHex{}
	}
	return &MoveEvent{
		encID:           encID,
		seq:             seq,
		Mover:           mover,
		From:            from,
		Path:            path,
		PerPlayer:       perPlayer,
		MoverPlayerID:   moverPlayerID,
		MoverKnownHexes: moverKnownHexes,
	}
}

func (*MoveEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *MoveEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *MoveEvent) Sequence() uint64 { return e.seq }

// Audience returns the set of players who can perceive this event,
// derived from the keys of PerPlayer.
func (e *MoveEvent) Audience() AudienceSet { return audienceFromMap(e.PerPlayer) }

// moveEventWire is the on-wire shape — used only by MarshalJSON / UnmarshalJSON.
// Keeping this private (alongside the unexported encID/seq fields) preserves
// the construction invariant: only NewMoveEvent and UnmarshalJSON can set
// the encounter ID and sequence number.
type moveEventWire struct {
	metaWire
	EncID           core.EncounterID                  `json:"encounter_id"`
	Seq             uint64                            `json:"sequence"`
	Mover           core.EntityID                     `json:"mover"`
	From            core.Hex                          `json:"from"`
	Path            []core.Hex                        `json:"path"`
	PerPlayer       map[core.PlayerID]MovePlayerSlice `json:"per_player"`
	MoverPlayerID   core.PlayerID                     `json:"mover_player_id,omitempty"`
	MoverKnownHexes []KnownHex                        `json:"mover_known_hexes,omitempty"`
}

// MarshalJSON exposes encID and seq under stable JSON field names without
// making the Go fields exported. Implements encoding/json.Marshaler.
func (e *MoveEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(moveEventWire{
		metaWire:        e.toWire(),
		EncID:           e.encID,
		Seq:             e.seq,
		Mover:           e.Mover,
		From:            e.From,
		Path:            e.Path,
		PerPlayer:       e.PerPlayer,
		MoverPlayerID:   e.MoverPlayerID,
		MoverKnownHexes: e.MoverKnownHexes,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *MoveEvent) UnmarshalJSON(b []byte) error {
	var w moveEventWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.Mover = w.Mover
	e.From = w.From
	e.Path = w.Path
	e.PerPlayer = w.PerPlayer
	e.MoverPlayerID = w.MoverPlayerID
	e.MoverKnownHexes = w.MoverKnownHexes
	if e.MoverKnownHexes == nil {
		e.MoverKnownHexes = []KnownHex{}
	}
	return nil
}
