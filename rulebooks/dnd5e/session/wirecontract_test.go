// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestFalseIsAnAnswerOnTheWire pins that a blocking flag which is FALSE still
// appears in the JSON.
//
// # Why this needs a test of its own
//
// `omitempty` on a bool drops the key when the value is false, and the case
// that breaks is the one this seam exists to express: a pile of bones is walked
// THROUGH and seen OVER, so both of its answers are false, and with omitempty
// it serialises as a ref and a cell and nothing else. A non-Go client cannot
// tell that from a prop nobody declared blocking for — which is precisely the
// ambiguity the composition spends a *bool to prevent one layer down
// (rpg-toolkit#1033), arriving back at the wire.
//
// Found by Copilot on rpg-toolkit#1136. The fix shipped UNPINNED: putting
// omitempty back passed the entire suite, so this asserts the bytes rather than
// the Go value, because the Go value is identical either way.
func TestFalseIsAnAnswerOnTheWire(t *testing.T) {
	atlas := session.Atlas{
		Props: []session.AtlasProp{{
			Ref:               "bones",
			At:                spatial.Position{X: 1, Y: 2},
			BlocksMovement:    false,
			BlocksLineOfSight: false,
		}},
		Boundaries: []session.AtlasBoundary{{
			From:              spatial.Position{X: 0, Y: 0},
			To:                spatial.Position{X: 1, Y: 0},
			BlocksMovement:    false,
			BlocksLineOfSight: false,
		}},
	}

	raw, err := json.Marshal(atlas)
	require.NoError(t, err)

	// Decoded generically, the way a client that is not this package sees it.
	var wire struct {
		Props []map[string]json.RawMessage `json:"props"`
		Walls []map[string]json.RawMessage `json:"boundaries"`
	}
	require.NoError(t, json.Unmarshal(raw, &wire))
	require.Len(t, wire.Props, 1)
	require.Len(t, wire.Walls, 1)

	for _, field := range []string{"blocks_movement", "blocks_line_of_sight"} {
		require.Contains(t, wire.Props[0], field,
			"a prop that blocks neither must still SAY it blocks neither: %s", raw)
		require.Equal(t, "false", string(wire.Props[0][field]))

		require.Contains(t, wire.Walls[0], field,
			"and so must a boundary: %s", raw)
		require.Equal(t, "false", string(wire.Walls[0][field]))
	}
}
