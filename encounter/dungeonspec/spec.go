// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dungeonspec decodes, validates, and compiles versioned YAML
// dungeon specs into the engine's DungeonParams + spawn plan (rpg-toolkit#842).
package dungeonspec

// DungeonSpec is the top-level decoded shape of a dungeon spec file.
type DungeonSpec struct {
	Version    int             `yaml:"version"`
	Key        string          `yaml:"key"`
	Name       string          `yaml:"name"`
	Theme      string          `yaml:"theme"`
	Height     int             `yaml:"height"`
	Rooms      []RoomSpec      `yaml:"rooms"`
	Connectors []ConnectorSpec `yaml:"connectors"`
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
	Place     []PlacedEntry   `yaml:"place,omitempty"` // design delta — see PlacedEntry
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
}

// ObstacleEntry is a count-based rolled obstacle for a room.
type ObstacleEntry struct {
	Ref            string `yaml:"ref"`
	Count          int    `yaml:"count"`
	BlocksMovement *bool  `yaml:"blocks_movement,omitempty"` // nil => true
	BlocksLoS      *bool  `yaml:"blocks_los,omitempty"`      // nil => true
	PreferBorder   bool   `yaml:"prefer_border,omitempty"`   // post-design delta: toolkit#840
}

// PlacedEntry is one static placement (design.md §Design delta) — routed by
// Ref's `module:type:id` type segment at VALIDATE time (props → obstacle,
// monsters → spawn; see Validate), not at decode time, so a bad ref type
// fails validation with a clear message instead of a decode error.
type PlacedEntry struct {
	Ref            string `yaml:"ref"`
	At             [2]int `yaml:"at"` // [col, row], room-local — see design.md §Design delta
	BlocksMovement *bool  `yaml:"blocks_movement,omitempty"`
	BlocksLoS      *bool  `yaml:"blocks_los,omitempty"`
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
