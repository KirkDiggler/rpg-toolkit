// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validM1YAML is the M1-valid variant of referenceYAML's shape (rpg-toolkit#842):
// same 4-room/3-connector chain (entrance -> gallery -> trap-crossing -> tomb,
// same ids/widths/archetypes/connectors/lock DC 12), but M1-valid: entrance/
// gallery drop their count-based monsters: blocks (their count-based
// obstacles: stay, unaffected by the M1 restriction); the tomb room's boss:
// gains at: [7, 5] and its obstacles: are replaced by the design delta's full
// place: list (coffin/altar/statue-reaper/brazier x2/skeleton), occupying
// rows 1, 2, 3, 5, and 6 — clear of doorRow (row 4, height/2).
const validM1YAML = `
version: 1
key: sunken-crypt
name: The Sunken Crypt
theme: crypt
height: 8
rooms:
  - id: entrance
    archetype: entrance
    width: 10
    obstacles:
      - { ref: "dnd5e:props:obelisk", count: 1 }
      - { ref: "dnd5e:props:pillar", count: 2 }
  - id: gallery
    archetype: chamber
    width: 8
  - id: trap-crossing
    archetype: corridor
    width: 6
  - id: tomb
    archetype: boss
    width: 12
    boss: { ref: "dnd5e:monsters:skeleton-captain", at: [7, 5] }
    place:
      - { ref: "dnd5e:props:coffin",        at: [6, 3], blocks_los: false }
      - { ref: "dnd5e:props:altar",         at: [9, 3] }
      - { ref: "dnd5e:props:statue-reaper", at: [1, 1] }
      - { ref: "dnd5e:props:brazier",       at: [3, 1] }
      - { ref: "dnd5e:props:brazier",       at: [3, 6] }
      - { ref: "dnd5e:monsters:skeleton",   at: [4, 2] }
connectors:
  - { from: entrance, to: gallery }
  - { from: gallery, to: trap-crossing }
  - { from: trap-crossing, to: tomb, locked: { dc: 12, ability: dex } }
`

// tomb returns validM1YAML's boss room (the last room), so mutate funcs
// don't repeat magic indices as the fixture grows.
func tomb(s *dungeonspec.DungeonSpec) *dungeonspec.RoomSpec {
	return &s.Rooms[len(s.Rooms)-1]
}

// Shared substrings used by more than one table row below (goconst).
const (
	errRefShape = "must be shaped like module:type:id"
	errM1Rolled = "rolled monster placement lands in M2"
)

func TestValidate_Table(t *testing.T) {
	cases := []struct {
		name    string
		base    string // defaults to validM1YAML when empty
		mutate  func(*dungeonspec.DungeonSpec)
		wantErr string // substring; "" = valid
	}{
		// --- v1 core rules ---
		{"valid reference spec", "", func(_ *dungeonspec.DungeonSpec) {}, ""},
		{"version 2 rejected", "", func(s *dungeonspec.DungeonSpec) { s.Version = 2 }, "unsupported spec version"},
		{"key empty rejected", "", func(s *dungeonspec.DungeonSpec) { s.Key = "" }, "key must not be empty"},
		{"name empty rejected", "", func(s *dungeonspec.DungeonSpec) { s.Name = "" }, "name must not be empty"},
		{"height below minimum rejected", "", func(s *dungeonspec.DungeonSpec) { s.Height = 3 },
			"height must be at least"},
		{"fewer than two rooms rejected", "", func(s *dungeonspec.DungeonSpec) { s.Rooms = s.Rooms[:1] },
			"must have at least"},
		{"duplicate room id rejected", "", func(s *dungeonspec.DungeonSpec) { s.Rooms[1].ID = s.Rooms[0].ID },
			"duplicate room id"},
		{"broken chain", "", func(s *dungeonspec.DungeonSpec) { s.Connectors[1].To = "tomb" }, "linear chain"},
		{"room width below minimum rejected", "", func(s *dungeonspec.DungeonSpec) { s.Rooms[0].Width = 3 },
			"width must be at least"},
		{"invalid pattern rejected", "", func(s *dungeonspec.DungeonSpec) { s.Rooms[0].Pattern = "bogus" },
			"invalid pattern"},
		{"boss width 5 fails axis rule", "", func(s *dungeonspec.DungeonSpec) { s.Rooms[3].Width = 5 }, "primary axis"},
		{"zero boss rooms rejected", "", func(s *dungeonspec.DungeonSpec) {
			s.Rooms[3].Archetype = "chamber"
			s.Rooms[3].Boss = nil
		}, "exactly one boss room"},
		{"two boss rooms rejected", "", func(s *dungeonspec.DungeonSpec) {
			at := [2]int{2, 1}
			s.Rooms[2].Archetype = "boss"
			s.Rooms[2].Boss = &dungeonspec.BossEntry{Ref: "dnd5e:monsters:skeleton-captain", At: &at}
		}, "exactly one boss room"},
		{"boss entry only on the boss room", "", func(s *dungeonspec.DungeonSpec) {
			at := [2]int{2, 2}
			s.Rooms[0].Boss = &dungeonspec.BossEntry{Ref: "dnd5e:monsters:skeleton-captain", At: &at}
		}, "boss entry only on the boss room"},
		{"obstacle count below minimum rejected", "", func(s *dungeonspec.DungeonSpec) {
			s.Rooms[0].Obstacles[0].Count = 0
		}, "count must be at least"},
		{"obstacle ref bad shape rejected", "", func(s *dungeonspec.DungeonSpec) {
			s.Rooms[0].Obstacles[0].Ref = "bad-ref"
		}, errRefShape},
		{"boss ref bad shape rejected", "", func(s *dungeonspec.DungeonSpec) {
			tomb(s).Boss.Ref = "not-a-real-ref"
		}, errRefShape},
		{"boss ref unresolvable rejected", "", func(s *dungeonspec.DungeonSpec) {
			tomb(s).Boss.Ref = "dnd5e:monsters:beholder"
		}, "unknown monster"},
		{"lock dc out of range rejected", "", func(s *dungeonspec.DungeonSpec) {
			s.Connectors[2].Locked.DC = 31
		}, "dc must be between"},
		{"lock ability unknown rejected", "", func(s *dungeonspec.DungeonSpec) {
			s.Connectors[2].Locked.Ability = "luck"
		}, "invalid ability"},

		// --- design delta: place / boss.at ---
		{"place at out of bounds (col)", "", func(s *dungeonspec.DungeonSpec) {
			// row 3 deliberately, NOT the entry's real row 4 — height/2
			// (doorRow) is ALSO row 4 (height:8), so col-99 at row 4 would
			// double-break (col OOB + reserved row) and this row wouldn't
			// isolate which check actually fired.
			tomb(s).Place[0].At = [2]int{99, 3}
		}, "out of bounds"},
		{"place at out of bounds (row)", "", func(s *dungeonspec.DungeonSpec) {
			tomb(s).Place[0].At = [2]int{6, 99}
		}, "out of bounds"},
		{"place ref bad shape rejected", "", func(s *dungeonspec.DungeonSpec) {
			tomb(s).Place[0].Ref = "no-colons-here"
		}, errRefShape},
		{"place collides with another placed entry", "", func(s *dungeonspec.DungeonSpec) {
			tomb(s).Place[1].At = tomb(s).Place[0].At
		}, "already placed"},
		{"place collides with boss.at", "", func(s *dungeonspec.DungeonSpec) {
			tomb(s).Place[0].At = *tomb(s).Boss.At
		}, "already placed"},
		{"place on reserved row (height/2) rejected", "", func(s *dungeonspec.DungeonSpec) {
			tomb(s).Place[0].At = [2]int{6, s.Height / 2}
		}, "reserved row"},
		{"place ref of unknown type rejected", "", func(s *dungeonspec.DungeonSpec) {
			// Place[2] (statue-reaper) deliberately, NOT Place[0] (coffin,
			// which sets blocks_los): this row must isolate the ref-type
			// rule alone, not also trip "blocks_los only valid on props" on
			// the same mutated entry.
			tomb(s).Place[2].Ref = "dnd5e:traps:pit"
		}, "must be props or monsters"},
		{"blocks_los set on a monster place entry is rejected", "", func(s *dungeonspec.DungeonSpec) {
			f := false
			tomb(s).Place[5].BlocksLoS = &f // Place[5] is the skeleton (monster) entry
		}, "blocks_los only valid on props"},
		{"boss ref duplicated in place is rejected", "", func(s *dungeonspec.DungeonSpec) {
			tomb(s).Place = append(tomb(s).Place, dungeonspec.PlacedEntry{Ref: tomb(s).Boss.Ref, At: [2]int{0, 0}})
		}, "boss ref may not also appear in place"},
		{"place monster ref unresolvable", "", func(s *dungeonspec.DungeonSpec) {
			tomb(s).Place[5].Ref = "dnd5e:monsters:beholder"
		}, "unknown monster"},
		{"place rejected when room pattern is scattered", "", func(s *dungeonspec.DungeonSpec) {
			// tomb has both place entries AND a pinned boss.at — either
			// alone triggers this rule; Validate's OR condition is
			// exercised either way.
			tomb(s).Pattern = "scattered"
		}, "place/boss.at not allowed with pattern: scattered"},
		{"count-based monsters: entry rejected in M1", "", func(s *dungeonspec.DungeonSpec) {
			s.Rooms[0].Monsters = append(s.Rooms[0].Monsters,
				dungeonspec.MonsterEntry{Ref: "dnd5e:monsters:skeleton", Count: 2})
		}, errM1Rolled},
		{"unpinned boss (no at) rejected in M1", "", func(s *dungeonspec.DungeonSpec) {
			tomb(s).Boss.At = nil
		}, errM1Rolled},
		{"boss-archetype room with no boss: entry rejected — permanent, not M1-only", "",
			func(s *dungeonspec.DungeonSpec) { tomb(s).Boss = nil }, "boss room must declare boss"},

		// --- fixture consequence: referenceYAML's own count-based monsters
		// are M1-invalid (Task B2's documented fixture consequence). Uses
		// referenceYAML (Task B1's decode-only fixture) instead of
		// validM1YAML for this one row only. Removed (not flipped to ""),
		// not kept, once M2's Task C0 lifts the M1 restriction.
		{"referenceYAML's own count-based monsters are M1-invalid", referenceYAML,
			func(_ *dungeonspec.DungeonSpec) {}, errM1Rolled},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := tc.base
			if base == "" {
				base = validM1YAML
			}
			spec, err := dungeonspec.Decode([]byte(base))
			require.NoError(t, err)

			tc.mutate(spec)

			err = dungeonspec.Validate(spec)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
