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

// TestWallHeightOmitemptyIsDeliberateOnTheWire pins the height field's BYTES
// both ways, because its contract is the opposite of its two bool siblings
// above: for blocks_movement/blocks_line_of_sight, false must be SAID; for
// height, zero must be UNSAID. That asymmetry is safe only because 0 is
// unauthorable (the YAML bounds are [1,3], raise-only — rpg-project#273), so
// an absent key can only ever mean "not authored, render the standard
// height". A nonzero authored multiplier must be emitted verbatim, and the
// default case must omit the key — a tag regression in either direction
// passes every Go-value test, so this asserts the bytes.
func TestWallHeightOmitemptyIsDeliberateOnTheWire(t *testing.T) {
	atlas := session.Atlas{
		Boundaries: []session.AtlasBoundary{
			{From: spatial.Position{X: 0, Y: 0}, To: spatial.Position{X: 1, Y: 0},
				BlocksMovement: true, BlocksLineOfSight: true, Height: 2.5},
			{From: spatial.Position{X: 0, Y: 1}, To: spatial.Position{X: 1, Y: 1},
				BlocksMovement: true, BlocksLineOfSight: true},
		},
	}

	raw, err := json.Marshal(atlas)
	require.NoError(t, err)

	var wire struct {
		Walls []map[string]json.RawMessage `json:"boundaries"`
	}
	require.NoError(t, json.Unmarshal(raw, &wire))
	require.Len(t, wire.Walls, 2)

	require.Contains(t, wire.Walls[0], "height", "an authored multiplier is emitted: %s", raw)
	require.Equal(t, "2.5", string(wire.Walls[0]["height"]), "verbatim, never rescaled")
	require.NotContains(t, wire.Walls[1], "height",
		"no authored height omits the key — a reader maps absence to the standard height: %s", raw)
}

// TestEmptyDeclarationsIsAnAnswerOnTheWire is TestFalseIsAnAnswerOnTheWire's
// own claim asked of a LIST rather than a bool: on the world clock,
// AffordOutput.Declarations is empty because the economy does not apply, not
// because nobody set the field. `omitempty` on a slice drops the key on nil
// exactly the way it drops a bool on false, and a non-Go client reading a
// missing "declarations" key cannot tell "the world clock, honestly empty"
// from "an older server that never had this field" or "a bug that forgot to
// set it". So the empty case still marshals the key, as "[]" rather than a
// missing entry or "null".
func TestEmptyDeclarationsIsAnAnswerOnTheWire(t *testing.T) {
	out := session.AffordOutput{Clock: session.ClockWorld, Declarations: []session.Declaration{}}

	raw, err := json.Marshal(out)
	require.NoError(t, err)

	var wire map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &wire))
	require.Contains(t, wire, "declarations",
		"the world clock's empty answer must still be a key on the wire: %s", raw)
	require.JSONEq(t, "[]", string(wire["declarations"]))
}
