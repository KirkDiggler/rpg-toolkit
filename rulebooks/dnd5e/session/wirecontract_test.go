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

// TestTheHoldingsWireNamesMatchTheContract pins the JSON names of everything
// rpg-project#368 added to this seam.
//
// # Why the bytes rather than the Go values
//
// A tag regression is INVISIBLE to every other test in this package: the Go
// field keeps its name, every scene keeps passing, and the only thing that
// changes is what a host reading these as JSON sees. The names here are the
// proto's names — `holding`, `exit`, `looter`, `body`, `holder`, `prop`, `at`
// — and a body whose key silently stopped matching would leave the beat
// undecodable on the far side while the whole suite stayed green. Found by
// the mutation pass: renaming ExitedBody's `holding` tag killed nothing.
//
// The two claims are separate on purpose. `holdable` must be SAID when false,
// on AtlasProp's own stated rule — a thing nobody declared holdable is
// scenery, and a client reading a missing key cannot tell that from an older
// server. `holding` and `exit` must be OMITTED when empty, because an
// ordinary departure carries neither and spending the keys on every exit
// would say "this departure had an empty list" where the truth is "this
// departure was not that kind of thing".
func TestTheHoldingsWireNamesMatchTheContract(t *testing.T) {
	keys := func(v any) map[string]json.RawMessage {
		raw, err := json.Marshal(v)
		require.NoError(t, err)
		var out map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &out))
		return out
	}

	t.Run("the three new bodies", func(t *testing.T) {
		looted := keys(session.LootedBody{Looter: "alice", Body: "captain"})
		require.Equal(t, `"alice"`, string(looted["looter"]))
		require.Equal(t, `"captain"`, string(looted["body"]))

		held := keys(session.HeldBody{Holder: "alice", Prop: "heirloom"})
		require.Equal(t, `"alice"`, string(held["holder"]))
		require.Equal(t, `"heirloom"`, string(held["prop"]))

		dropped := keys(session.DroppedBody{
			Member: "bob", Prop: "chalice", At: spatial.Position{X: 4, Y: 1}})
		require.Equal(t, `"bob"`, string(dropped["member"]))
		require.Equal(t, `"chalice"`, string(dropped["prop"]))
		require.Contains(t, dropped, "at",
			"the cell is `at` here, though the composition writes its beats with `position`")
	})

	t.Run("the departure, carrying something and carrying nothing", func(t *testing.T) {
		carried := keys(session.ExitedBody{
			Member: "alice", Holding: []string{"heirloom"}, Exit: "front-gate"})
		require.Equal(t, `["heirloom"]`, string(carried["holding"]))
		require.Equal(t, `"front-gate"`, string(carried["exit"]))

		ordinary := keys(session.ExitedBody{Member: "bob"})
		require.NotContains(t, ordinary, "holding",
			"an ordinary departure carried nothing, and says so by saying nothing")
		require.NotContains(t, ordinary, "exit",
			"and used no authored way out")
	})

	t.Run("an exit on the map", func(t *testing.T) {
		exit := keys(session.AtlasExit{ID: "front-gate", At: spatial.Position{X: 1, Y: 1}})
		require.Equal(t, `"front-gate"`, string(exit["id"]))
		require.Contains(t, exit, "at")
	})

	t.Run("a prop that cannot be picked up still says so", func(t *testing.T) {
		scenery := keys(session.AtlasProp{Ref: "pillar", Holdable: false})
		require.Contains(t, scenery, "holdable",
			"false is the ANSWER — a thing nobody declared holdable IS scenery — and a "+
				"client reading a missing key cannot tell that from an older server")
		require.Equal(t, "false", string(scenery["holdable"]))
		require.NotContains(t, scenery, "id",
			"but an unnamed placement spends no key: most props have no author's name, "+
				"and empty and absent are the same fact")

		named := keys(session.AtlasProp{Ref: "reliquary", ID: "heirloom", Holdable: true})
		require.Equal(t, `"heirloom"`, string(named["id"]))
		require.Equal(t, "true", string(named["holdable"]))
	})
}
