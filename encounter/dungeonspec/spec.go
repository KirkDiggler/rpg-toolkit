// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dungeonspec decodes, validates, and compiles versioned YAML
// dungeon specs into the engine's DungeonParams + spawn plan (rpg-toolkit#842).
// Its authored wall endpoints are pointy-top odd-q offset [column,row] pairs,
// never even-q or axial coordinates; the rpg-project specimen authority owns
// the examples and coordinate errata.
package dungeonspec

// DungeonSpec is the top-level decoded shape of a dungeon spec file.
type DungeonSpec struct {
	Version int    `yaml:"version"`
	Key     string `yaml:"key"`
	Name    string `yaml:"name"`
	Theme   string `yaml:"theme"`
	Height  int    `yaml:"height"`
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
	// Place is decoded solely to reject unsupported top-level placement,
	// including facing, at a field-specific validation path. Slice #178
	// supports only existing room-scoped floor props.
	Place []PlacedEntry `yaml:"place,omitempty"`
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

// BossEntry designates a room's boss monster.
type BossEntry struct {
	Ref string  `yaml:"ref"`
	At  *[2]int `yaml:"at,omitempty"` // design delta: nil = unpinned (v1 behavior)
	// Facing is decoded solely so unsupported boss-facing input can fail at
	// its supplied field path. Boss-facing behavior is outside Slice #178.
	Facing *string `yaml:"facing,omitempty"`
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
	At             [2]int `yaml:"at"` // [col, row], room-local — static-placement delta, rpg-toolkit#842
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
}

// WallSpec is one strict authored edge from the top-level walls grammar.
// Endpoints use absolute pointy-top odd-q offset [column,row] coordinates,
// not even-q or axial coordinates. Endpoint pointers preserve the difference
// between a supplied pair and a prohibited null/missing endpoint; explicit
// YAML null is valid only for the optional collection, not for either endpoint
// of an entry.
type WallSpec struct {
	From *[2]int `yaml:"from"`
	To   *[2]int `yaml:"to"`
	Kind string  `yaml:"kind"`
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
