// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The types below are this package's own twins of what the composition
// returns, and the functions translating into them are the price of S2.
//
// The translation is boring, and that is the point: it is what lets the
// encounter's own View, Story, Status and Atlas types change shape — or be
// replaced outright by a different implementation — without a single host
// source file changing. Re-exporting them would have been fewer lines and
// would have made every one of those modules permanently load-bearing on the
// wire.
//
// Everything here is also proto-shaped by construction: flat structs, string
// enums, no interfaces, nothing whose meaning varies by another field's value.

// GridKind names a room's coordinate family.
//
// A string rather than a mirror of spatial's iota. The composition already
// persists grid this way for the same reason: an iota is an in-process
// enumeration order, not a wire contract, and reordering it upstream would
// silently reinterpret every stored and transmitted value.
type GridKind string

const (
	// GridSquare is a square grid, where distance is Chebyshev.
	GridSquare GridKind = "square"

	// GridHex is a hex grid addressed in axial coordinates, where distance is
	// measured in cube space.
	GridHex GridKind = "hex"
)

// Atlas is the static world map in dungeon-absolute space.
//
// Construction truth: unchanged by movement, joins, exits, or endings. Cache
// it per encounter rather than fetching it per frame.
type Atlas struct {
	// Rooms is every room's absolute footprint, sorted by room ID.
	Rooms []AtlasRoom `json:"rooms,omitempty"`

	// Doorways is every connection's absolute endpoint pair, sorted by
	// connection ID.
	Doorways []AtlasDoorway `json:"doorways,omitempty"`
}

// AtlasRoom is one room's absolute-space footprint.
type AtlasRoom struct {
	// ID is the room's identifier.
	ID string `json:"id"`

	// Grid is the room's coordinate family.
	Grid GridKind `json:"grid"`

	// Origin is the room's dungeon-absolute anchor.
	Origin spatial.Position `json:"origin"`

	// Width is the room's horizontal dimension.
	Width int `json:"width"`

	// Height is the room's vertical dimension.
	Height int `json:"height"`

	// Cells is every cell of the room in dungeon-absolute space, occluded
	// cells included: occlusion is walkability, not ownership.
	Cells []spatial.Position `json:"cells,omitempty"`

	// Occluders is the subset of Cells that blocks line of sight, reported
	// separately so a host can render them distinctly.
	Occluders []spatial.Position `json:"occluders,omitempty"`

	// Boundaries is the room's walls and barriers, both endpoints absolute.
	Boundaries []AtlasBoundary `json:"boundaries,omitempty"`
}

// AtlasBoundary is one wall or barrier crossing between adjacent cells.
type AtlasBoundary struct {
	// From is one endpoint of the crossing, in dungeon-absolute space.
	From spatial.Position `json:"from"`

	// To is the other endpoint, in dungeon-absolute space.
	To spatial.Position `json:"to"`

	// BlocksMovement reports whether an entity may cross.
	BlocksMovement bool `json:"blocks_movement,omitempty"`

	// BlocksLineOfSight reports whether sight may cross.
	BlocksLineOfSight bool `json:"blocks_line_of_sight,omitempty"`
}

// AtlasDoorway is one connection's absolute endpoint pair. The two cells are
// adjacent in absolute space, so crossing one is an ordinary step.
type AtlasDoorway struct {
	// Connection is the connection's identifier.
	Connection string `json:"connection"`

	// From is the source room ID.
	From string `json:"from"`

	// FromCell is the endpoint in From, in dungeon-absolute space.
	FromCell spatial.Position `json:"from_cell"`

	// To is the destination room ID.
	To string `json:"to"`

	// ToCell is the endpoint in To, in dungeon-absolute space.
	ToCell spatial.Position `json:"to_cell"`
}

// Status reports whether an encounter is still running.
type Status struct {
	// Open reports whether the encounter is active.
	Open bool `json:"open"`

	// Outcome is the ending that fired, present only once closed.
	Outcome *Outcome `json:"outcome,omitempty"`
}

// Outcome describes how an encounter ended.
type Outcome struct {
	// Ending is the key of the ending that fired.
	Ending string `json:"ending"`

	// At is the clock reading when it fired.
	At uint64 `json:"at,omitempty"`

	// Members is where everyone stood when it closed.
	Members []MemberOutcome `json:"members,omitempty"`
}

// MemberOutcome is one member's final placement.
type MemberOutcome struct {
	// ID is the member's identifier.
	ID string `json:"id"`

	// Room is the room they were in.
	Room string `json:"room"`

	// Position is where they stood within it.
	Position spatial.Position `json:"position"`
}

// Sighting is one thing an observer currently perceives.
type Sighting struct {
	// Subject names what is perceived.
	Subject string `json:"subject"`

	// Payload is what the observer knows about it, encoded by the composition.
	Payload []byte `json:"payload,omitempty"`

	// Channel is how it was perceived.
	Channel string `json:"channel,omitempty"`

	// At is the clock reading when this knowledge was last refreshed.
	At uint64 `json:"at,omitempty"`

	// CurrentVia lists the channels currently carrying it. Empty means the
	// observer holds a memory rather than a live sighting.
	CurrentVia []string `json:"current_via,omitempty"`

	// Status distinguishes a live sighting from a stale memory.
	Status string `json:"status,omitempty"`
}

// StoryEntry is one beat of what an observer has witnessed.
type StoryEntry struct {
	// Seq is the beat's sequence: monotonic, gapless, never renumbered. A
	// client that notices a gap has missed a beat and should re-query.
	Seq uint64 `json:"seq"`

	// At is the clock reading when the beat was recorded.
	At uint64 `json:"at,omitempty"`

	// Correlation groups cause and effect across beats. Empty is legal.
	Correlation string `json:"correlation,omitempty"`

	// Tags is queryable metadata describing the beat.
	Tags map[string]string `json:"tags,omitempty"`

	// Payload is the beat itself, encoded by the composition.
	Payload []byte `json:"payload,omitempty"`
}

// projectGrid maps a room's grid family onto the wire enum.
//
// An unrecognised shape yields the empty string rather than a guess. It is
// unreachable — the composition rejects unknown grid families at both Setup
// and Load — but inventing a plausible value for something we do not
// understand would turn an impossible state into a wrong answer, which is
// strictly worse than an obviously absent one.
func projectGrid(shape spatial.GridShape) GridKind {
	switch shape {
	case spatial.GridShapeSquare:
		return GridSquare
	case spatial.GridShapeHex:
		return GridHex
	default:
		return ""
	}
}

func projectAtlas(in encounter.Atlas) Atlas {
	out := Atlas{
		Rooms:    make([]AtlasRoom, 0, len(in.Rooms)),
		Doorways: make([]AtlasDoorway, 0, len(in.Doorways)),
	}

	for _, room := range in.Rooms {
		projected := AtlasRoom{
			ID:         room.ID,
			Grid:       projectGrid(room.Grid),
			Origin:     room.Origin,
			Width:      room.Width,
			Height:     room.Height,
			Cells:      append([]spatial.Position(nil), room.Cells...),
			Occluders:  append([]spatial.Position(nil), room.Occluders...),
			Boundaries: make([]AtlasBoundary, 0, len(room.Boundaries)),
		}
		for _, b := range room.Boundaries {
			projected.Boundaries = append(projected.Boundaries, AtlasBoundary{
				From:              b.From,
				To:                b.To,
				BlocksMovement:    b.BlocksMovement,
				BlocksLineOfSight: b.BlocksLineOfSight,
			})
		}
		out.Rooms = append(out.Rooms, projected)
	}

	for _, d := range in.Doorways {
		out.Doorways = append(out.Doorways, AtlasDoorway{
			Connection: d.Connection,
			From:       d.From,
			FromCell:   d.FromCell,
			To:         d.To,
			ToCell:     d.ToCell,
		})
	}

	return out
}

func projectStatus(in *encounter.Status) *Status {
	if in == nil {
		return nil
	}
	out := &Status{Open: in.Open}
	if in.Outcome == nil {
		return out
	}

	outcome := &Outcome{
		Ending:  in.Outcome.Ending,
		At:      in.Outcome.At,
		Members: make([]MemberOutcome, 0, len(in.Outcome.Members)),
	}
	for _, m := range in.Outcome.Members {
		outcome.Members = append(outcome.Members, MemberOutcome{
			ID:       string(m.ID),
			Room:     m.Room,
			Position: m.Position,
		})
	}
	out.Outcome = outcome
	return out
}

func projectSightings(in []intel.Holding) []Sighting {
	out := make([]Sighting, 0, len(in))
	for _, h := range in {
		via := make([]string, 0, len(h.CurrentVia))
		for _, c := range h.CurrentVia {
			via = append(via, string(c))
		}
		out = append(out, Sighting{
			Subject:    string(h.Subject),
			Payload:    append([]byte(nil), h.Payload...),
			Channel:    string(h.Channel),
			At:         h.At,
			CurrentVia: via,
			Status:     string(h.Status),
		})
	}
	return out
}

// projectStory drops each entry's audience roster deliberately.
//
// A story is queried by one viewer, and the roster names every OTHER viewer a
// beat was addressed to. Handing that back would tell Alice which members
// exist and were present for something — including members she has never
// perceived, and members in rooms she has never entered. The audience is a
// delivery rule, not story content, and it is the composition's business
// rather than the host's.
func projectStory(in []record.Entry) []StoryEntry {
	out := make([]StoryEntry, 0, len(in))
	for _, e := range in {
		var tags map[string]string
		if len(e.Tags) > 0 {
			tags = make(map[string]string, len(e.Tags))
			for k, v := range e.Tags {
				tags[k] = v
			}
		}
		out = append(out, StoryEntry{
			Seq:         e.Seq,
			At:          e.At,
			Correlation: e.Correlation,
			Tags:        tags,
			Payload:     append([]byte(nil), e.Payload...),
		})
	}
	return out
}
