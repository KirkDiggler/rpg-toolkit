// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tomb returns validM1YAML's boss room (the last room), so mutate funcs
// don't repeat magic indices as the fixture grows.
func tomb(s *dungeonspec.DungeonSpec) *dungeonspec.RoomSpec {
	return &s.Rooms[len(s.Rooms)-1]
}

// Shared substrings used by more than one table row below (goconst).
const (
	errRefShape              = "must be shaped like module:type:id"
	errM1Rolled              = "rolled monster placement lands in M2"
	errOutOfBounds           = "out of bounds"
	errUnsupportedCapability = "unsupported capability"
)

// TestValidate_FacingStrictlyScopesCanonicalFloorProps proves facing is neither
// stripped nor broadly accepted: only floor props may carry it in room-chain
// or canvas mode, while unsupported forms name the exact supplied field path.
func TestValidate_FacingStrictlyScopesCanonicalFloorProps(t *testing.T) {
	decode := func(t *testing.T) *dungeonspec.DungeonSpec {
		t.Helper()
		spec, err := dungeonspec.Decode([]byte(placedTombYAML))
		require.NoError(t, err)
		return spec
	}

	for _, label := range []string{"E", "NE", "NW", "W", "SW", "SE"} {
		t.Run("floor prop accepts "+label, func(t *testing.T) {
			spec := decode(t)
			spec.Rooms[1].Place[0].Facing = &label
			require.NoError(t, dungeonspec.Validate(spec))
		})
	}

	cases := []struct {
		name     string
		mutate   func(*dungeonspec.DungeonSpec)
		wantPath string
		wantErr  string
	}{
		{
			name: "floor prop rejects unknown label with vocabulary",
			mutate: func(spec *dungeonspec.DungeonSpec) {
				label := "N"
				spec.Rooms[1].Place[0].Facing = &label
			},
			wantPath: "rooms[1].place[0].facing",
			wantErr:  `must be "E", "NE", "NW", "W", "SW", or "SE"`,
		},
		{
			name: "monster place facing is unsupported",
			mutate: func(spec *dungeonspec.DungeonSpec) {
				label := "E"
				spec.Rooms[1].Place[5].Facing = &label
			},
			wantPath: "rooms[1].place[5].facing",
			wantErr:  errUnsupportedCapability,
		},
		{
			name: "boss facing is unsupported",
			mutate: func(spec *dungeonspec.DungeonSpec) {
				label := "SW"
				spec.Rooms[1].Boss.Facing = &label
			},
			wantPath: "rooms[1].boss.facing",
			wantErr:  errUnsupportedCapability,
		},
		{
			name: "mounted prop facing is unsupported",
			mutate: func(spec *dungeonspec.DungeonSpec) {
				label, mount := "NE", "wall"
				spec.Rooms[1].Place[0].Facing = &label
				spec.Rooms[1].Place[0].Mount = &mount
			},
			wantPath: "rooms[1].place[0].facing",
			wantErr:  errUnsupportedCapability,
		},
		{
			name: "top level facing is unsupported at its own entry path",
			mutate: func(spec *dungeonspec.DungeonSpec) {
				label := "E"
				spec.Place = []dungeonspec.PlacedEntry{
					{Ref: "dnd5e:props:candles", At: [2]int{1, 1}},
					{Ref: "dnd5e:props:candles", At: [2]int{2, 1}, Facing: &label},
				}
			},
			wantPath: "place[1].facing",
			wantErr:  errUnsupportedCapability,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := decode(t)
			tc.mutate(spec)
			err := dungeonspec.Validate(spec)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantPath)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestValidate_KeyVocabulary(t *testing.T) {
	for _, key := range []string{"Uppercase", "under_score", "has space"} {
		t.Run(key, func(t *testing.T) {
			spec, err := dungeonspec.Decode([]byte(validM1YAML))
			require.NoError(t, err)
			spec.Key = key
			require.ErrorContains(t, dungeonspec.Validate(spec), "lowercase letters, digits, and hyphens")
		})
	}
	spec, err := dungeonspec.Decode([]byte(validM1YAML))
	require.NoError(t, err)
	spec.Key = "valid-key-123"
	require.NoError(t, dungeonspec.Validate(spec))
}

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
		{"connector count mismatch rejected", "", func(s *dungeonspec.DungeonSpec) { s.Connectors = s.Connectors[:1] },
			"linear chain"}, // distinct branch from "broken chain": the count check, not the per-connector match
		{"room width below minimum rejected", "", func(s *dungeonspec.DungeonSpec) { s.Rooms[0].Width = 3 },
			"width must be at least"},
		{"invalid archetype rejected", "",
			func(s *dungeonspec.DungeonSpec) { s.Rooms[1].Archetype = "chambre" }, //nolint:misspell
			"invalid archetype"},
		// gallery (rooms[1]) is a plain chamber room today; neither of these
		// mutations trips any OTHER invariant (only "boss" is special-cased
		// elsewhere in Validate) — chamber and boss themselves are already
		// exercised, as-is, by every row that doesn't touch rooms[1]/[3].
		{"archetype entrance accepted on a non-entrance room", "", func(s *dungeonspec.DungeonSpec) {
			s.Rooms[1].Archetype = "entrance"
		}, ""},
		{"archetype corridor accepted on a non-corridor room", "", func(s *dungeonspec.DungeonSpec) {
			s.Rooms[1].Archetype = "corridor"
		}, ""},
		{"invalid pattern rejected", "", func(s *dungeonspec.DungeonSpec) { s.Rooms[0].Pattern = "bogus" },
			"invalid pattern"},
		// Ordering guard: boss-axis must run before any per-cell place-bounds
		// check, so shrinking the boss room reports "primary axis" and not an
		// incidental "out of bounds" on one of its now-too-far-out place
		// entries (the tomb room's place list uses col 9 for the altar).
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
		{"boss ref must be a monster ref rejected", "", func(s *dungeonspec.DungeonSpec) {
			tomb(s).Boss.Ref = "dnd5e:props:coffin"
		}, "must be a monster ref"},
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
		}, errOutOfBounds},
		{"place at out of bounds (row)", "", func(s *dungeonspec.DungeonSpec) {
			tomb(s).Place[0].At = [2]int{6, 99}
		}, errOutOfBounds},
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
		{"boss.at on reserved row (height/2) rejected", "", func(s *dungeonspec.DungeonSpec) {
			at := [2]int{7, s.Height / 2}
			tomb(s).Boss.At = &at
		}, "reserved row"},
		{"boss.at out of bounds rejected", "", func(s *dungeonspec.DungeonSpec) {
			at := [2]int{99, 5}
			tomb(s).Boss.At = &at
		}, errOutOfBounds},
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
		{"blocks_movement set on a monster place entry is rejected", "", func(s *dungeonspec.DungeonSpec) {
			b := true
			tomb(s).Place[5].BlocksMovement = &b // Place[5] is the skeleton (monster) entry
		}, "blocks_movement only valid on props"},
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
		// Ordering guard: boss-required (non-nil Boss) must be checked in
		// validateBossCardinality before anything downstream dereferences
		// bossRoom.Boss (axis, ref resolution, M1 at-pinning, place-block) —
		// this row proves the guard fires first instead of a nil dereference.
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
