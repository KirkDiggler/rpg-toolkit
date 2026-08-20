// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// continuityGrid returns a throwaway grid of the given family, valid ONLY
// as a Distance calculator over ABSOLUTE positions — Distance depends
// only on the two positions passed to it, never on the grid's own bounds
// (SquareGrid.Distance and AxialHexGrid.Distance's own implementations),
// so any instance of the right family computes the correct distance
// between two dungeon-absolute positions regardless of which room's
// config built it. #929 T4: this is what lets a scene assert continuity
// in absolute space without needing a specific room's live Grid.
func continuityGrid(family spatial.GridShape) spatial.Grid {
	if family == spatial.GridShapeHex {
		return spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 100000, SpanHeight: 100000})
	}
	return spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 100000, Height: 100000})
}

// continuityViolation returns the index i such that positions[i] and
// positions[i+1] are MORE than maxDist apart, or -1 if every consecutive
// pair is within maxDist. Written to localize a hostile probe run
// manually during T4 (a field built with W3 deliberately disabled, to
// confirm this function actually detects a break) and then deleted, not
// committed — a discontinuous field is unconstructible through the
// public API precisely BECAUSE W3 rejects it (#929 hardening round,
// test-gap closure item 7 — this is a strong property of the module,
// not merely an untested corner). The index return stays regardless: it
// makes a REAL assertion failure in this file diagnosable — which step
// broke, not just that one did — independent of that probe's own
// history.
func continuityViolation(family spatial.GridShape, positions []spatial.Position, maxDist float64) int {
	grid := continuityGrid(family)
	for i := 0; i < len(positions)-1; i++ {
		if grid.Distance(positions[i], positions[i+1]) > maxDist {
			return i
		}
	}
	return -1
}

// vaultChaseHexGate is the connection shared by the two places this file
// builds the hex vault-chase decider — vaultChaseHexSetup's initial
// construction (below) and TestVaultChaseAbsoluteContinuity's mid-chase
// reload, which re-attaches a decider carrying the identical topology —
// kept as its own value so both share the exact same gate without
// re-deriving it.
func vaultChaseHexGate() encounter.ConnectionInput {
	return encounter.ConnectionInput{
		ID: "gate", From: "corridor", To: "vault",
		FromPosition: spatial.Position{X: 4, Y: 1},
		ToPosition:   spatial.Position{X: -5, Y: -2},
	}
}

// vaultChaseHexSeamWall is the corridor's wall along its Q-max edge, open at
// the gate's row. The corridor's own last column is Q=4 and the vault's first
// is Q=5, so every edge here has one endpoint in each chamber — a sentence no
// room could say before the field became one canvas (rpg-toolkit#1106), and
// the reason this scene needs to say it: with one canvas and no wall, the two
// chambers share an open seam and there is nowhere in the vault to disappear
// to. R is the vault's own range, [-2,7], intersected with the corridor's
// [-5,4].
func vaultChaseHexSeamWall() []spatial.Boundary { return hexSeamWall(4, -2, 4, 1) }

// vaultChaseHexSetup is the hex-family sibling of chase_test.go's
// TestVaultChase fixture (#929 T4 — "the vault-chase fixture/story, or a
// sibling of it, in the game's family"): the SAME shape — a corridor and
// a vault joined by one gate, a pursuit decider, a sanctuary ending in
// the far room — re-derived for hex. The vault's Origin used to be a
// parameter so a hostile, shifted-origin variant could be built for a
// discriminating probe without duplicating this fixture — that probe
// was run manually during T4 and deleted, never committed (#929
// hardening round, test-gap closure item 7: the parameter had exactly
// one call site, always passing the same value below, so it was dead),
// leaving the vault's Origin fixed at the one geometry this scene ever
// actually uses.
//
// Geometry: corridor is 10x10 hex at Origin (0,0) — Q,R both in [-5,4].
// The vault's Origin is (10,3): vault's Q-range ([5,14]) is fully
// disjoint from corridor's ([-5,4]) regardless of R (W2), and the gate's
// endpoints — corridor (4,1) [Q-max edge] and vault local (-5,-2)
// [Q-min edge] — land on absolute (4,1) and (5,1): cube distance 1, a
// genuine axial neighbor via the standard (+1,0) offset, not a formula
// quirk (W3).
func vaultChaseHexSetup() *encounter.SetupInput {
	gate := vaultChaseHexGate()
	field := encounter.FieldInput{
		Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
		Rooms: []encounter.RoomInput{
			{ID: "corridor", Width: 10, Height: 10, Grid: spatial.GridShapeHex,
				Boundaries: vaultChaseHexSeamWall()},
			{ID: "vault", Width: 10, Height: 10, Grid: spatial.GridShapeHex, Origin: spatial.Position{X: 10, Y: 3}},
		},
		Connections: []encounter.ConnectionInput{gate},
	}
	pursuit := &pursuitDecider{doorways: doorwaysFrom(field), target: alice}
	return &encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: field,
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: "corridor", Position: spatial.Position{X: 1, Y: 1}},
			// One hex-step from the gate's corridor-side endpoint (4,1) via
			// the (0,+1) offset — close enough that the pursuit decider's
			// "jump to last-seen position" IS a genuine single-step move,
			// not a room-spanning teleport (#929 T4: every recorded beat in
			// this scene, not just the doorway, is a real single-cell hop).
			{ID: goblin, Kind: encounter.KindMonster, Room: "corridor",
				Position: spatial.Position{X: 4, Y: 0}, Decider: pursuit},
		},
		Endings: []encounter.EndingInput{
			{Key: "sanctuary", Trigger: encounter.TriggerReachedPosition{
				Room: "vault", Position: spatial.Position{X: -3, Y: -3}}},
		},
	}
}

// continuityProjector accumulates a human-readable, exact transcript and a
// plain position path as a story unfolds — the wave's payoff pin's own
// instrument.
//
// It projects with the FIXTURE's own arithmetic, from the layout this file
// authored, rather than asking the composition. It used to call Absolute and
// Locate; both are gone, because the authored rooms are compiled onto one
// canvas at construction and there is nothing left to project at runtime
// (rpg-toolkit#1106). Doing the arithmetic here is the stronger check anyway:
// the transcript is now independent of the code it is describing, so the two
// cannot agree by sharing a mistake.
type continuityProjector struct {
	t          *testing.T
	enc        *encounter.Encounter
	rooms      []encounter.RoomInput
	transcript []string
}

func newContinuityProjector(t *testing.T, enc *encounter.Encounter, rooms []encounter.RoomInput) *continuityProjector {
	return &continuityProjector{t: t, enc: enc, rooms: rooms}
}

// originOf is the authored anchor of a chamber in this fixture.
func (p *continuityProjector) originOf(room string) spatial.Position {
	for _, r := range p.rooms {
		if r.ID == room {
			return r.Origin
		}
	}
	require.FailNow(p.t, "no such room", room)
	return spatial.Position{}
}

// useEncounter repoints the projector at a reloaded Encounter — the
// transcript keeps accumulating in the SAME slice across the swap, so a
// reload mid-story never splits the pinned transcript in two.
func (p *continuityProjector) useEncounter(enc *encounter.Encounter) {
	p.enc = enc
}

// project turns one room-local position into the canvas cell the authored
// layout puts it at, records it in the human-readable transcript, and returns
// the absolute position for the caller's own path-continuity bookkeeping.
func (p *continuityProjector) project(member, verb, room string, pos spatial.Position) spatial.Position {
	p.t.Helper()
	absolute := pos.Add(p.originOf(room))
	p.transcript = append(p.transcript, fmt.Sprintf("%s %s: %s(%g,%g) -> absolute(%g,%g)",
		member, verb, room, pos.X, pos.Y, absolute.X, absolute.Y))
	return absolute
}

// locate is project's sibling for a position the composition ALREADY reports in
// dungeon-absolute space — a pump's monster step, or a closing outcome. It
// resolves the cell back to the chamber whose authored footprint holds it,
// writes the SAME transcript row project would have written for that
// room-local cell, and returns the absolute position unchanged.
//
// Reading the reported cell back is what makes a room-local value visible: a
// room-local cell either lands in the WRONG chamber or in none at all, and the
// transcript row says which.
func (p *continuityProjector) locate(member, verb string, absolute spatial.Position) spatial.Position {
	p.t.Helper()
	for _, r := range p.rooms {
		local := absolute.Subtract(r.Origin)
		qMin, qMax := hexSpan(r.Width)
		rMin, rMax := hexSpan(r.Height)
		if local.X < qMin || local.X > qMax || local.Y < rMin || local.Y > rMax {
			continue
		}
		p.transcript = append(p.transcript, fmt.Sprintf("%s %s: %s(%g,%g) -> absolute(%g,%g)",
			member, verb, r.ID, local.X, local.Y, absolute.X, absolute.Y))
		return absolute
	}
	require.FailNow(p.t, "cell belongs to no chamber",
		"%s %s: %v is not floor in this field", member, verb, absolute)
	return absolute
}

// hexSpan is an axial hex room's own [min,max] cell bounds along one axis: the
// origin-centred half-open span tools/spatial gives an AxialHexGrid.
func hexSpan(dim int) (min, max float64) {
	half := float64(dim) / 2
	return math.Ceil(-half), math.Ceil(half) - 1
}

// TestVaultChaseAbsoluteContinuity is the wave's payoff scene (#929 T4):
// W2 + W3 + the local/absolute bridge, proven as ONE property — a
// member's path, projected into dungeon-absolute space, is continuous
// across a doorway exactly as it is within a room. This is the hex
// sibling of chase_test.go's TestVaultChase — its TRANSCRIPT (the beats,
// positions, and assertions the story makes) is unchanged, though T1
// touched the file itself to give vaultRoom an explicit Origin (W2
// requires it; #929 hardening round H corrects an earlier claim of
// byte-identical that no longer held once that Origin was added). The
// story shape is deliberately the same (corridor, vault, one gate, a
// pursuit decider, sanctuary in the far room, a mid-chase reload) so the
// two scenes read as siblings, not vignettes.
//
// Every recorded position — including the goblin's — is a genuine
// single-hex-step from the one before it (the fixture is built for
// this: see vaultChaseHexSetup's doc comment), so the continuity claim
// holds over the WHOLE transcript, not just at the doorway; the doorway
// step is then asserted to be distance EXACTLY 1, same as every other
// step — the kiss made visible, indistinguishable by inspection from an
// ordinary move.
func TestVaultChaseAbsoluteContinuity(t *testing.T) {
	enc, err := encounter.NewEncounter(vaultChaseHexSetup())
	require.NoError(t, err, "the hex vault assembles")

	proj := newContinuityProjector(t, enc, vaultChaseHexSetup().Field.Rooms)
	var alicePath, goblinPath []spatial.Position

	// ---- Beat 1: starting placements, mutual sight ----------------------
	alicePath = append(alicePath, proj.project(string(alice), "start", "corridor", spatial.Position{X: 1, Y: 1}))
	goblinPath = append(goblinPath, proj.project(string(goblin), "start", "corridor", spatial.Position{X: 4, Y: 0}))

	st, _ := seen(t, enc, alice, goblin)
	require.Equal(t, intel.Current, st, "beat 1: alice sees the goblin across the open corridor")
	st, _ = seen(t, enc, goblin, alice)
	require.Equal(t, intel.Current, st, "beat 1: and the goblin sees her back")

	// Seeing each other started the fight (rpg-toolkit#964); she breaks off
	// before she runs.
	_, err = enc.Dissolve(&encounter.DissolveInput{Member: alice})
	require.NoError(t, err, "beat 1: alice breaks off to run")

	// ---- Beat 2: alice steps toward the gate, one hex at a time ----------
	for _, to := range []spatial.Position{{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 1}} {
		_, err = enc.Step(&encounter.StepInput{Member: alice, To: to})
		require.NoError(t, err, "alice steps toward the gate")
		alicePath = append(alicePath, proj.project(string(alice), "move", "corridor", to))
	}

	// Through the gate: one more step, to the cell on the other side. The
	// departure cell was already recorded as alicePath's last entry, so only
	// the arrival cell is new.
	throughTheGate := proj.project(string(alice), "arrive via gate", "vault", spatial.Position{X: -5, Y: -2})
	travOut, err := enc.Step(&encounter.StepInput{Member: alice, To: throughTheGate})
	require.NoError(t, err, "alice slips through the gate")
	require.Equal(t, "gate", travOut.Crossing, "the doorway is named, and decides nothing")
	require.Equal(t, spatial.Position{X: 4, Y: 1}, travOut.Stepped.From, "the departure cell matches the last recorded move")
	require.Equal(t, throughTheGate, travOut.Stepped.To)
	alicePath = append(alicePath, throughTheGate)

	// She takes one more step deeper into the vault before the pause, OFF the
	// gate's row, and THAT is what puts the wall between them: standing in the
	// opening she was still in plain view (rpg-toolkit#1106).
	deeper := proj.project(string(alice), "move", "vault", spatial.Position{X: -5, Y: -3})
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: deeper})
	require.NoError(t, err, "alice steps deeper into the vault")
	alicePath = append(alicePath, deeper)

	// The ghost forms at the goblin's LAST SIGHT of her, which the wall beside
	// the gate is what makes possible: a room boundary hid nothing here, and
	// standing in the opening she was in plain view.
	st, p := seen(t, enc, goblin, alice)
	require.Equal(t, intel.Held, st, "beat 2: the goblin's sight of alice fades — the wall took her")

	// ---- Beat 3: the pause (pause is free) --------------------------------
	// The projected path must survive the reload — the SAME claim T3 pinned
	// for Atlas (TestAtlasIdenticalAfterReload), now exercised as a live
	// path a host is actively rendering, not just a static snapshot.
	beforeReload := alicePath[len(alicePath)-1]
	data := enc.ToData()
	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, Data: data, Deciders: map[encounter.MemberID]encounter.Decider{
			goblin: &pursuitDecider{doorways: doorwaysFrom(vaultChaseHexSetup().Field), target: alice},
		}})
	require.NoError(t, err, "the suspended chase crosses a process boundary")
	enc = enc2
	proj.useEncounter(enc) // SAME projector, reloaded enc — the transcript keeps accumulating

	st, pAfter := seen(t, enc, goblin, alice)
	require.Equal(t, intel.Held, st, "beat 3: the ghost survived the reload")
	require.Equal(t, p, pAfter, "beat 3: at the same cell — loading never re-derives sight")

	// Re-project alice's CURRENT position on the reloaded encounter — it
	// must be identical to her last pre-reload position (distance 0,
	// trivially continuous): a reload never moves anyone.
	afterReload := proj.project(string(alice), "reload checkpoint", "vault", spatial.Position{X: -5, Y: -3})
	require.Equal(t, beforeReload, afterReload, "beat 3: the projected position is unchanged by the reload")

	// ---- Beat 4: the pursuit crosses too ----------------------------------
	pumpOut1, err := enc.Pump(&encounter.PumpInput{})
	require.NoError(t, err, "beat 4: the pursuit resumes")
	require.Len(t, pumpOut1.MonsterMoves, 1, "beat 4: the goblin steps toward the threshold")
	// The pump reports where it walked on the MAP — no room needed to read
	// it, and no arithmetic to redo (rpg-toolkit#1062). The corridor is
	// anchored at the origin, so this one cell reads the same either way;
	// the crossing below is where the two frames part company.
	require.Equal(t, spatial.Position{X: 4, Y: 1}, pumpOut1.MonsterMoves[0].To)
	goblinPath = append(goblinPath, proj.locate(string(goblin), "move", pumpOut1.MonsterMoves[0].To))

	pumpOut2, err := enc.Pump(&encounter.PumpInput{})
	require.NoError(t, err, "beat 4: the goblin follows her through")
	require.Len(t, pumpOut2.MonsterMoves, 1, "beat 4: the goblin comes through the gate")
	// vault-local (-5,-2) through the vault's (10,3) anchor: the same absolute
	// cell alice's own step landed on, and the same cell the movement beat
	// carries. ONE list, because there is one kind of step.
	require.Equal(t, spatial.Position{X: 5, Y: 1}, pumpOut2.MonsterMoves[0].To)
	goblinPath = append(goblinPath, proj.locate(string(goblin), "arrive via gate", pumpOut2.MonsterMoves[0].To))

	// ---- Beat 5: sanctuary ------------------------------------------------
	for _, to := range []spatial.Position{{X: -4, Y: -3}, {X: -3, Y: -3}} {
		absolute := proj.project(string(alice), "move", "vault", to)
		moveOut, mErr := enc.Step(&encounter.StepInput{Member: alice, To: absolute})
		require.NoError(t, mErr, "alice steps toward sanctuary")
		alicePath = append(alicePath, absolute)
		if to == (spatial.Position{X: -3, Y: -3}) {
			require.NotNil(t, moveOut.Outcome, "the ending fires on arrival")
			require.Equal(t, "sanctuary", moveOut.Outcome.Ending)

			var aliceOutcome, goblinOutcome encounter.MemberOutcome
			for _, m := range moveOut.Outcome.Members {
				switch m.ID {
				case alice:
					aliceOutcome = m
				case goblin:
					goblinOutcome = m
				}
			}
			// The final ending position (#929 T4): it must agree with the
			// live path's own last recorded entries. Read back through
			// Locate rather than projected, because the outcome now
			// reports absolute cells itself (#1068) — and Locate is the
			// stronger check anyway: a room-local cell here would resolve
			// to the wrong room or to no room at all.
			outAlice := proj.locate(string(alice), "outcome", aliceOutcome.Position)
			outGoblin := proj.locate(string(goblin), "outcome", goblinOutcome.Position)
			require.Equal(t, alicePath[len(alicePath)-1], outAlice, "the outcome's alice position matches her live projected path")
			require.Equal(t, goblinPath[len(goblinPath)-1], outGoblin, "the outcome's goblin position matches its live projected path")
		}
	}

	// ---- The continuity claim, proven -------------------------------------
	// Every consecutive pair in BOTH paths is at most one step apart — the
	// whole transcript is a genuine walk, not a sequence of jumps.
	require.Equal(t, -1, continuityViolation(spatial.GridShapeHex, alicePath, 1),
		"alice's absolute-space path must be continuous throughout, including the outcome projection")
	require.Equal(t, -1, continuityViolation(spatial.GridShapeHex, goblinPath, 1),
		"the goblin's absolute-space path must be continuous throughout, including the outcome projection")

	// The doorway step, specifically: distance EXACTLY 1, not 0, not 2 —
	// the room boundary is invisible in world space.
	grid := continuityGrid(spatial.GridShapeHex)
	require.Equal(t, 1.0, grid.Distance(alicePath[3], alicePath[4]), "alice's doorway crossing is distance exactly 1")
	require.Equal(t, 1.0, grid.Distance(goblinPath[1], goblinPath[2]), "the goblin's doorway crossing is distance exactly 1")

	// ---- The transcript, pinned exactly ------------------------------------
	// Human-readable in the house style, doubling as documentation of what
	// the host actually sees — the doorway rows (both "arrive via gate"
	// entries) read exactly like an ordinary move, by design: the room
	// boundary is invisible in world space.
	require.Equal(t, []string{
		"alice start: corridor(1,1) -> absolute(1,1)",
		"goblin start: corridor(4,0) -> absolute(4,0)",
		"alice move: corridor(2,1) -> absolute(2,1)",
		"alice move: corridor(3,1) -> absolute(3,1)",
		"alice move: corridor(4,1) -> absolute(4,1)",
		"alice arrive via gate: vault(-5,-2) -> absolute(5,1)",
		"alice move: vault(-5,-3) -> absolute(5,0)",
		"alice reload checkpoint: vault(-5,-3) -> absolute(5,0)",
		"goblin move: corridor(4,1) -> absolute(4,1)",
		"goblin arrive via gate: vault(-5,-2) -> absolute(5,1)",
		"alice move: vault(-4,-3) -> absolute(6,0)",
		"alice move: vault(-3,-3) -> absolute(7,0)",
		"alice outcome: vault(-3,-3) -> absolute(7,0)",
		"goblin outcome: vault(-5,-2) -> absolute(5,1)",
	}, proj.transcript, "the story IS the projected continuity, told in order")
}
