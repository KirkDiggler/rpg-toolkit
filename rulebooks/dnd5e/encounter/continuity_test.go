// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"fmt"
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
		FromPosition: spatial.Position{X: 9, Y: 5},
		ToPosition:   spatial.Position{X: 0, Y: 5},
	}
}

// vaultChaseHexSeamWall is the corridor's wall along its last column, open at
// the gate's row. The corridor's own last column is 9 and the vault's first is
// 10, so every edge here has one endpoint in each chamber — a sentence no room
// could say before the field became one canvas (rpg-toolkit#1106), and the
// reason this scene needs to say it: with one canvas and no wall, the two
// chambers share an open seam and there is nowhere in the vault to disappear
// to.
//
// Counted in the chamber's own offset rows since rpg-toolkit#1127, so the range
// is simply its full height.
func vaultChaseHexSeamWall() []spatial.Boundary {
	return hexOffsetSeamWall(encounter.HexesArePointyTop(), 9, 0, 9, 5)
}

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
// Geometry, in the frame a hex chamber is authored in since
// rpg-toolkit#1127 — OFFSET columns and rows, counted from the chamber's own
// corner. Corridor is 10x10 at Origin (0,0), so columns 0-9 and rows 0-9;
// the vault is 10x10 at Origin (10,0), so columns 10-19. The two column
// ranges do not meet, so the chambers share no cell (W2) — and unlike the
// rhombus reading this replaces, that is now true under EITHER orientation,
// because a chamber's footprint no longer depends on which way its hexes
// point.
//
// The gate joins corridor (9,5) — its own last column — to vault local (0,5),
// which is absolute column 10, row 5. Same-row neighbouring columns are
// adjacent in odd-r for either parity, so those two cells kiss (W3) as
// spatial's own conversion has it, not as a formula this fixture rolled.
func vaultChaseHexSetup() *encounter.SetupInput {
	gate := vaultChaseHexGate()
	field := encounter.FieldInput{
		Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
		Rooms: []encounter.RoomInput{
			{ID: "corridor", Width: 10, Height: 10, Grid: spatial.GridShapeHex,
				Boundaries: vaultChaseHexSeamWall()},
			{ID: "vault", Width: 10, Height: 10, Grid: spatial.GridShapeHex, Origin: spatial.Position{X: 10, Y: 0}},
		},
		Connections: []encounter.ConnectionInput{gate},
	}
	pursuit := &pursuitDecider{doorways: doorwaysFrom(field), target: alice}
	return &encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Field: field,
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Room: "corridor", Position: spatial.Position{X: 6, Y: 5}},
			// One hex-step from the gate's corridor-side endpoint (9,5), and
			// BESIDE the seam rather than in line with it. Both halves are
			// load-bearing, and neither is arbitrary.
			//
			// One step, because [Encounter.Step] does not check adjacency
			// (its doc comment says so, deliberately) and neither does the
			// silent stepTo a Pump moves a monster with — so the pursuit
			// decider's "jump to the last-seen cell" lands wherever that cell
			// is, in one go. It is the FIXTURE that makes every recorded beat
			// a real single-cell hop, not the verb, which is exactly why this
			// scene can claim continuity over the whole transcript.
			//
			// Beside, because the seam wall leaves one edge open — (9,5) to
			// (10,5) — and a watcher off that line cannot see through it: the
			// ray from (9,4) into the vault crosses a walled edge in every
			// direction. So this goblin watches her walk the corridor and
			// loses her AT the threshold, which is what leaves the ghost on
			// the corridor's own gate cell one step away, and what makes the
			// decider reach for the doorway on its second think.
			{ID: goblin, Kind: encounter.KindMonster, Room: "corridor",
				Position: spatial.Position{X: 9, Y: 4}, Decider: pursuit},
		},
		Endings: []encounter.EndingInput{
			{Key: "sanctuary", Trigger: encounter.TriggerReachedPosition{
				Room: "vault", Position: spatial.Position{X: 2, Y: 6}}},
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
//
// The arithmetic is offset-then-convert since rpg-toolkit#1127: a hex
// chamber's local cell and its anchor are both counted in authored columns and
// rows, and the SUM is what becomes an axial cell. Adding the anchor in axial —
// what this used to do — is the shear that made a chamber a rhombus, one level
// up.
func (p *continuityProjector) project(member, verb, room string, pos spatial.Position) spatial.Position {
	p.t.Helper()
	origin := p.originOf(room)
	absolute := encounter.HexCellAt(encounter.HexesArePointyTop(),
		int(pos.X)+int(origin.X), int(pos.Y)+int(origin.Y))
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
	col, row := hexOffsetOfCell(absolute)
	for _, r := range p.rooms {
		localCol, localRow := col-int(r.Origin.X), row-int(r.Origin.Y)
		if localCol < 0 || localCol >= r.Width || localRow < 0 || localRow >= r.Height {
			continue
		}
		p.transcript = append(p.transcript, fmt.Sprintf("%s %s: %s(%d,%d) -> absolute(%g,%g)",
			member, verb, r.ID, localCol, localRow, absolute.X, absolute.Y))
		return absolute
	}
	require.FailNow(p.t, "cell belongs to no chamber",
		"%s %s: %v is not floor in this field", member, verb, absolute)
	return absolute
}

// hexOffsetOfCell is [encounter.HexCellAt] run backwards: the authored
// [col,row] a dungeon-absolute cell came from.
//
// The fixture's own arithmetic, deliberately — this file projects with the
// layout it authored rather than asking the composition, so the transcript
// stays independent of the code it describes and the two cannot agree by
// sharing a mistake. What it may borrow is tools/spatial's conversion, which
// belongs to neither side of that comparison.
func hexOffsetOfCell(cell spatial.Position) (col, row int) {
	offset := spatial.AxialToCube(cell).
		ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
	return int(offset.X), int(offset.Y)
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
	alicePath = append(alicePath, proj.project(string(alice), "start", "corridor", spatial.Position{X: 6, Y: 5}))
	goblinPath = append(goblinPath, proj.project(string(goblin), "start", "corridor", spatial.Position{X: 9, Y: 4}))

	st, _ := seen(t, enc, alice, goblin)
	require.Equal(t, intel.Current, st, "beat 1: alice sees the goblin across the open corridor")
	st, _ = seen(t, enc, goblin, alice)
	require.Equal(t, intel.Current, st, "beat 1: and the goblin sees her back")

	// Seeing each other started the fight (rpg-toolkit#964); she breaks off
	// before she runs.
	_, err = enc.Dissolve(&encounter.DissolveInput{Member: alice})
	require.NoError(t, err, "beat 1: alice breaks off to run")

	// ---- Beat 2: alice steps toward the gate, one hex at a time ----------
	for _, to := range []spatial.Position{{X: 7, Y: 5}, {X: 8, Y: 5}, {X: 9, Y: 5}} {
		absolute := proj.project(string(alice), "move", "corridor", to)
		_, err = enc.Step(&encounter.StepInput{Member: alice, To: absolute})
		require.NoError(t, err, "alice steps toward the gate")
		alicePath = append(alicePath, absolute)
	}

	// Through the gate: one more step, to the cell on the other side. The
	// departure cell was already recorded as alicePath's last entry, so only
	// the arrival cell is new.
	throughTheGate := proj.project(string(alice), "arrive via gate", "vault", spatial.Position{X: 0, Y: 5})
	travOut, err := enc.Step(&encounter.StepInput{Member: alice, To: throughTheGate})
	require.NoError(t, err, "alice slips through the gate")
	require.Equal(t, "gate", travOut.Crossing, "the doorway is named, and decides nothing")
	require.Equal(t, alicePath[len(alicePath)-1], travOut.Stepped.From, "the departure cell matches the last recorded move")
	require.Equal(t, throughTheGate, travOut.Stepped.To)
	alicePath = append(alicePath, throughTheGate)

	// She takes one more step deeper into the vault before the pause, off the
	// gate's row. The wall took her one step earlier, at the threshold itself:
	// the goblin watches from beside the seam, so crossing the opening is
	// already out of its sight (see vaultChaseHexSetup's placement note). This
	// step is what puts a second cell between the ghost and where she actually
	// is, so the pursuit has somewhere to be wrong about.
	deeper := proj.project(string(alice), "move", "vault", spatial.Position{X: 0, Y: 6})
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: deeper})
	require.NoError(t, err, "alice steps deeper into the vault")
	alicePath = append(alicePath, deeper)

	// The ghost forms at the goblin's LAST SIGHT of her, which the wall beside
	// the gate is what makes possible: a room boundary hid nothing here
	// (rpg-toolkit#1106), and without a wall on the seam the vault would be in
	// plain view from the corridor and there would be nowhere to disappear to.
	st, p := seen(t, enc, goblin, alice)
	require.Equal(t, intel.Held, st, "beat 2: the goblin's sight of alice fades — the wall took her")

	// ---- Beat 3: the pause (pause is free) --------------------------------
	// The projected path must survive the reload — the SAME claim T3 pinned
	// for Atlas (TestAtlasIdenticalAfterReload), now exercised as a live
	// path a host is actively rendering, not just a static snapshot.
	beforeReload := alicePath[len(alicePath)-1]
	data := enc.ToData()
	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Data: data, Deciders: map[encounter.MemberID]encounter.Decider{
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
	afterReload := proj.project(string(alice), "reload checkpoint", "vault", spatial.Position{X: 0, Y: 6})
	require.Equal(t, beforeReload, afterReload, "beat 3: the projected position is unchanged by the reload")

	// ---- Beat 4: the pursuit crosses too ----------------------------------
	pumpOut1, err := enc.Pump(&encounter.PumpInput{})
	require.NoError(t, err, "beat 4: the pursuit resumes")
	require.Len(t, pumpOut1.MonsterMoves, 1, "beat 4: the goblin steps toward the threshold")
	// The pump reports where it walked on the MAP — no room needed to read it,
	// and no arithmetic to redo (rpg-toolkit#1062). It went to the ghost, and
	// the ghost stands on the last cell it saw her on: the corridor's own gate
	// cell, which is the fourth thing her path recorded. Asserted as that
	// entry rather than as a literal, so the two cannot drift apart — and note
	// that the absolute cell is NOT the pair either of them was authored as
	// (rpg-toolkit#1127), which is exactly why the pump reports absolute.
	require.Equal(t, alicePath[3], pumpOut1.MonsterMoves[0].To)
	goblinPath = append(goblinPath, proj.locate(string(goblin), "move", pumpOut1.MonsterMoves[0].To))

	pumpOut2, err := enc.Pump(&encounter.PumpInput{})
	require.NoError(t, err, "beat 4: the goblin follows her through")
	require.Len(t, pumpOut2.MonsterMoves, 1, "beat 4: the goblin comes through the gate")
	// Standing on the ghost and she is not there, the decider reaches for the
	// door it is standing in — so vault-local (0,5) through the vault's (10,0)
	// anchor: the same absolute cell alice's own step landed on, and the same
	// cell the movement beat carries. ONE list, because there is one kind of
	// step.
	require.Equal(t, throughTheGate, pumpOut2.MonsterMoves[0].To)
	goblinPath = append(goblinPath, proj.locate(string(goblin), "arrive via gate", pumpOut2.MonsterMoves[0].To))

	// ---- Beat 5: sanctuary ------------------------------------------------
	for _, to := range []spatial.Position{{X: 1, Y: 6}, {X: 2, Y: 6}} {
		absolute := proj.project(string(alice), "move", "vault", to)
		moveOut, mErr := enc.Step(&encounter.StepInput{Member: alice, To: absolute})
		require.NoError(t, mErr, "alice steps toward sanctuary")
		alicePath = append(alicePath, absolute)
		if to == (spatial.Position{X: 2, Y: 6}) {
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
		// EVERY ABSOLUTE HERE MOVED when rpg-toolkit#1141 corrected the hex
		// offset schemes, and again when rpg-toolkit#1150 corrected the axial
		// basis itself (Q is cube X, R is cube Z, not cube Y) -- every
		// AUTHORED cell stayed put throughout both fixes: corridor(6,5) is
		// still corridor(6,5). That split is the reassuring part: the scene
		// plays out identically and only the projection underneath it changed.
		//
		// The continuity this test is named for still holds and is still what
		// the comparison enforces: one authored cell projects to ONE absolute
		// cell everywhere in the story. corridor(9,5) is absolute(7,5) for
		// alice and for the goblin; vault(0,5) is absolute(8,5) at both
		// arrivals and at the goblin's outcome; vault(0,6) survives a reload
		// unchanged.
		"alice start: corridor(6,5) -> absolute(4,5)",
		"goblin start: corridor(9,4) -> absolute(7,4)",
		"alice move: corridor(7,5) -> absolute(5,5)",
		"alice move: corridor(8,5) -> absolute(6,5)",
		"alice move: corridor(9,5) -> absolute(7,5)",
		"alice arrive via gate: vault(0,5) -> absolute(8,5)",
		"alice move: vault(0,6) -> absolute(7,6)",
		"alice reload checkpoint: vault(0,6) -> absolute(7,6)",
		"goblin move: corridor(9,5) -> absolute(7,5)",
		"goblin arrive via gate: vault(0,5) -> absolute(8,5)",
		"alice move: vault(1,6) -> absolute(8,6)",
		"alice move: vault(2,6) -> absolute(9,6)",
		"alice outcome: vault(2,6) -> absolute(9,6)",
		"goblin outcome: vault(0,5) -> absolute(8,5)",
	}, proj.transcript, "the story IS the projected continuity, told in order")
}
