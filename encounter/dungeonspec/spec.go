// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dungeonspec decodes, validates, and compiles versioned YAML
// dungeon specs into the engine's DungeonParams + spawn plan (rpg-toolkit#842).
// Its authored wall endpoints are pointy-top odd-q offset [column,row] pairs,
// never even-q or axial coordinates; the rpg-project specimen authority owns
// the examples and coordinate errata.
package dungeonspec

import "github.com/KirkDiggler/rpg-toolkit/encounter/core"

// DungeonSpec is the top-level decoded shape of a dungeon spec file.
type DungeonSpec struct {
	Version int         `yaml:"version"`
	Key     string      `yaml:"key"`
	Name    string      `yaml:"name"`
	Theme   string      `yaml:"theme"`
	Height  int         `yaml:"height"`
	Canvas  *CanvasSpec `yaml:"canvas,omitempty"`
	// Start is an optional absolute [column,row] party-start anchor. Nil
	// represents both an omitted YAML field and an explicit YAML null.
	Start      *[2]int         `yaml:"start,omitempty"`
	Rooms      []RoomSpec      `yaml:"rooms"`
	Connectors []ConnectorSpec `yaml:"connectors"`
	// Walls is the optional dungeon-scoped authored physical-edge collection.
	// Nil represents both omitted and explicit YAML null; every present entry
	// must name two absolute pointy-top odd-q [column,row] floor cells (not
	// even-q or axial) and a solid or door kind.
	Walls []WallSpec `yaml:"walls,omitempty"`
	// Place accepts absolute placement only with an explicit canvas floor source.
	// Room-chain mode rejects every top-level entry at a field-specific
	// validation path.
	Place []PlacedEntry `yaml:"place,omitempty"`
	// Regions are authored semantic scopes over the structural floor. They are
	// deliberately separate from legacy Rooms: their cells are absolute and
	// never feed room-chain generation.
	Regions []RegionSpec `yaml:"regions,omitempty"`
	// roomsShape preserves the authored YAML form for canvas validation.
	// A nil Rooms slice alone cannot distinguish omitted, null, and [] input.
	roomsShape roomsShape
}

type roomsShape uint8

const (
	roomsOmitted roomsShape = iota
	roomsSequence
	roomsNull
	roomsInvalid
)

// CanvasSpec is the explicit structural floor source for canvas mode.
type CanvasSpec struct {
	Width       int    `yaml:"width"`
	Height      int    `yaml:"height"`
	FloorSource string `yaml:"floor_source,omitempty"`
}

// RegionSpec is one authored semantic scope. Cells are absolute odd-q
// [column,row] coordinates; nil/empty is a runnable empty scope.
type RegionSpec struct {
	ID        string   `yaml:"id"`
	Name      *string  `yaml:"name,omitempty"`
	Archetype *string  `yaml:"archetype,omitempty"`
	Cells     [][2]int `yaml:"cells"`
}

// RoomSpec is one room in a dungeon spec.
type RoomSpec struct {
	ID        string          `yaml:"id"`
	Archetype string          `yaml:"archetype"` // entrance|chamber|corridor|boss
	Width     int             `yaml:"width"`
	Pattern   string          `yaml:"pattern,omitempty"` // ""(=empty)|empty|scattered
	Monsters  []MonsterEntry  `yaml:"monsters,omitempty"`
	Boss      *BossEntry      `yaml:"boss,omitempty"`
	Obstacles []ObstacleEntry `yaml:"obstacles,omitempty"`
	Place     []PlacedEntry   `yaml:"place,omitempty"` // static placement — see PlacedEntry
}

// MonsterEntry is a count-based monster roll for a room.
type MonsterEntry struct {
	Ref   string `yaml:"ref"`
	Count int    `yaml:"count"`
}

// PlacementOffset is an optional authored [x,y,z] translation in canonical
// game-world axes relative to a placement's canonical origin. It is transport
// data only: facing never rotates it and toolkit mechanics never interpret it.
type PlacementOffset = core.PlacementOffset

// BossEntry designates a room's boss monster.
type BossEntry struct {
	Ref string  `yaml:"ref"`
	At  *[2]int `yaml:"at,omitempty"` // design delta: nil = unpinned (v1 behavior)
	// Facing is decoded solely so unsupported boss-facing input can fail at
	// its supplied field path. Boss-facing behavior is outside Slice #178.
	Facing *string `yaml:"facing,omitempty"`
	// Targeting overrides the boss's targeting strategy — same vocabulary
	// and nil-means-omitted shape as PlacedEntry.Targeting (rpg-toolkit#895).
	Targeting *string `yaml:"targeting,omitempty"`
	// Offset preserves omission independently from an explicit zero triple.
	Offset *PlacementOffset `yaml:"offset,omitempty"`
}

// ObstacleEntry is a count-based rolled obstacle for a room.
type ObstacleEntry struct {
	Ref            string `yaml:"ref"`
	Count          int    `yaml:"count"`
	BlocksMovement *bool  `yaml:"blocks_movement,omitempty"` // nil => true
	BlocksLoS      *bool  `yaml:"blocks_los,omitempty"`      // nil => true
	PreferBorder   bool   `yaml:"prefer_border,omitempty"`   // post-design delta: toolkit#840
}

// PlacedEntry is one static placement (static-placement delta, rpg-toolkit#842)
// — routed by Ref's `module:type:id` type segment at VALIDATE time (props →
// obstacle, monsters → spawn; see Validate), not at decode time, so a bad
// ref type fails validation with a clear message instead of a decode error.
type PlacedEntry struct {
	Ref            string `yaml:"ref"`
	At             [2]int `yaml:"at"` // [col, row], room-local in a room or absolute with a canvas floor source
	BlocksMovement *bool  `yaml:"blocks_movement,omitempty"`
	BlocksLoS      *bool  `yaml:"blocks_los,omitempty"`
	// Facing is the optional canonical YAML label. Nil represents both an
	// omitted field and explicit YAML null; explicit E remains non-nil and
	// compiles to the present numeric value zero.
	Facing *string `yaml:"facing,omitempty"`
	// Mount is decoded only to reject mounted-facing input at its supplied
	// field path. Slice #178 supports floor placements only and never
	// compiles mount behavior.
	Mount *string `yaml:"mount,omitempty"`
	// Targeting overrides the placed monster's targeting strategy (one of
	// "closest" | "lowest-health" | "lowest-ac" — monster.ParseTargetingStrategy,
	// rpg-toolkit#895). Nil represents both an omitted field and explicit
	// YAML null; only valid on a monsters ref, never a props ref (validate.go
	// mirrors blocks_movement's own props-only restriction, inverted).
	Targeting *string `yaml:"targeting,omitempty"`
	// Offset preserves omission independently from an explicit zero triple.
	Offset *PlacementOffset `yaml:"offset,omitempty"`
}

// WallSpec is one strict authored edge from the top-level walls grammar.
// Endpoints use absolute pointy-top odd-q offset [column,row] coordinates,
// not even-q or axial coordinates. Endpoint pointers preserve the difference
// between a supplied pair and a prohibited null/missing endpoint; explicit
// YAML null is valid only for the optional collection, not for either endpoint
// of an entry.
type WallSpec struct {
	From *[2]int       `yaml:"from"`
	To   *[2]int       `yaml:"to"`
	Kind string        `yaml:"kind"`
	Lock *WallLockSpec `yaml:"lock,omitempty"`
}

// ConnectorSpec joins two rooms, optionally behind a locked check.
type ConnectorSpec struct {
	From   string      `yaml:"from"`
	To     string      `yaml:"to"`
	Locked *LockedSpec `yaml:"locked,omitempty"`
}

// LockedSpec describes an ability check gating a connector.
type LockedSpec struct {
	DC      int    `yaml:"dc"`
	Ability string `yaml:"ability"`
}

// WallLockSpec preserves v0.3 lock grammar without binding executable behavior.
type WallLockSpec struct {
	Options []LockOptionSpec `yaml:"options"`
}

// LockOptionSpec is one authored, alternative ability check.
type LockOptionSpec struct {
	Ability string `yaml:"ability"`
	DC      int    `yaml:"dc"`
}
