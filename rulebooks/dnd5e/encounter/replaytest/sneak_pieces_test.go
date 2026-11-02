// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package replaytest_test

// sneak_pieces_test.go is THE SCRIPT'S SNEAKING PIECES, REPLAYED (castletest's
// castle_king_script_test.go, pieces 4 and 5). A sneak declaration is on the
// record (sneaking R10 as amended by the-world-runs R11, rpg-project#768):
// an accepted one writes a `sneaking` beat the replay re-issues as a
// SetSneaking, and every roll the sneak asks is a `snuck` beat the replay's
// host answers from. A refused declaration writes nothing and changes
// nothing, so the replay, which never re-issues it, lands the same log.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The gatehouse's corner and the gate patrol's posts (castletest's script).
var (
	// theCorner is the cell behind the corner wall, in the gatehouse's
	// north-west, out of the gate's sight.
	theCorner = castleCell(-3, 7)

	// besideTheCorner is the floor just south of the corner, in the open.
	besideTheCorner = castleCell(-3, 8)

	// besideThat is besideTheCorner's west neighbour, for pacing.
	besideThat = castleCell(-4, 8)

	// theDash is the way from the corner to the cell under the gate, one
	// adjacent cell at a time.
	theDash = []spatial.Position{castleCell(-3, 8), castleCell(-2, 8), castleCell(-2, 7), castleCell(-1, 7), castleCell(0, 7)}

	// The posts: each gate guard stands at a door post watching the cell
	// under the gate, then at a beat post watching the incomers.
	westDoorPost = castleCell(-1, 8)
	eastDoorPost = castleCell(2, 8)
	westBeatPost = castleCell(0, 10)
	eastBeatPost = castleCell(1, 10)
)

// gate1Passive and gate2Passive are the gate guards' passives on
// [castleOpener]'s host, so a roll's DC names the guard it was against.
const (
	gate1Passive = 11
	gate2Passive = 12
)

// dwellOf is a member's dwell as the run persists it, or nil.
func (w *world) dwellOf(id encounter.MemberID) *encounter.DwellData {
	w.t.Helper()
	for _, m := range w.enc.ToData().Members {
		if m.ID == id {
			return m.Dwell
		}
	}
	require.FailNowf(w.t, "not in the run", "%s", id)

	return nil
}

// justArrived is whether both gate guards stand at the given posts with a
// round of the dwell still to stand.
func (w *world) justArrived(west, east spatial.Position) bool {
	w.t.Helper()
	g1, g2 := w.dwellOf(kingGate1), w.dwellOf(kingGate2)

	return g1 != nil && g2 != nil && g1.Rounds > 0 && g2.Rounds > 0 &&
		w.at(kingGate1) == west && w.at(kingGate2) == east
}

// paceUntil walks a member out and back between two adjacent cells until a
// condition holds, and requires that it came to hold.
func (w *world) paceUntil(member encounter.MemberID, a, b spatial.Position, steps int, until func() bool) {
	w.t.Helper()
	require.Equal(w.t, 1.0, w.enc.Distance(a, b), "pace walks between adjacent cells")
	for i := 0; i < steps && !until(); i++ {
		to := a
		if w.at(member) == a {
			to = b
		}
		require.NoError(w.t, w.step(member, to))
	}
	require.True(w.t, until(), "the condition came to hold within %d steps", steps)
}

// snuckDC is the DC a `snuck` beat's roll faced.
func snuckDC(t *testing.T, payload []byte) int {
	t.Helper()
	var b struct {
		DC int `json:"dc"`
	}
	require.NoError(t, json.Unmarshal(payload, &b))

	return b.DC
}

// toTheCorner walks the hero, not sneaking, from the gatehouse to the cell
// beside the corner and onto the corner.
func (w *world) toTheCorner() {
	w.t.Helper()
	w.walkTo(hero, besideTheCorner)
	if w.at(hero) != besideTheCorner {
		require.NoError(w.t, w.step(hero, besideTheCorner))
	}
	require.NoError(w.t, w.step(hero, theCorner))
}

// TestReplay_CastleSneakPieces replays the QA castle script's two sneaking
// pieces from their record.
func TestReplay_CastleSneakPieces(t *testing.T) {
	t.Parallel()

	// Piece 4, the sneak window at the door: the hero declares behind the
	// corner once both gate guards have just come to the given posts, dashes
	// to the cell under the gate, and declares off there. At the beat posts
	// nothing rolls; at the door posts the dash rolls against both guards,
	// on the host's own die, won or lost.
	dash := func(t *testing.T, west, east spatial.Position) []record.EntryData {
		t.Helper()
		return replayRun(t, castleOpener(gatehouseSeat), func(w *world) {
			w.walkTo(hero, besideTheCorner)
			if w.at(hero) != besideTheCorner {
				require.NoError(t, w.step(hero, besideTheCorner))
			}
			w.paceUntil(hero, besideTheCorner, besideThat, 60, func() bool { return w.justArrived(west, east) })
			require.NoError(t, w.step(hero, theCorner))
			require.NoError(t, w.sneak(hero, true), "behind the corner nobody sees the hero declare")
			for _, cell := range theDash {
				require.NoError(t, w.step(hero, cell))
			}
			require.Equal(t, beneathTheGate, w.at(hero))
			require.False(t, w.fighting(hero), "the civil gate forms no fight")
			require.NoError(t, w.sneak(hero, false))
		})
	}
	t.Run("piece 4: the dash in the window", func(t *testing.T) {
		t.Parallel()
		log := dash(t, westBeatPost, eastBeatPost)
		require.Equal(t, 2, countBeats(t, log, encounter.BeatSneaking), "precondition: on and off are on the record")
		require.Zero(t, countBeats(t, log, encounter.BeatSnuck), "precondition: in the window nothing rolled")
	})
	t.Run("piece 4: the dash at the door posts", func(t *testing.T) {
		t.Parallel()
		log := dash(t, westDoorPost, eastDoorPost)
		require.Equal(t, 2, countBeats(t, log, encounter.BeatSneaking), "precondition: on and off are on the record")
		dcs := map[int]bool{}
		for _, e := range log {
			if beatOf(t, e.Payload) == encounter.BeatSnuck {
				dcs[snuckDC(t, e.Payload)] = true
			}
		}
		require.True(t, dcs[gate1Passive] && dcs[gate2Passive], "precondition: the dash rolled against both gate guards")
	})

	// Piece 5, sneaking in plain sight: declared in the gatehouse, in the
	// gate's sight, sneaking is refused and writes nothing. The hero walks
	// to the corner and declares again until the patrol's back lets it,
	// then declares off. The replay re-issues only the accepted
	// declaration, and lands the same log.
	t.Run("piece 5: refused in plain sight, then declared unseen", func(t *testing.T) {
		t.Parallel()
		refused := 0
		log := replayRun(t, castleOpener(gatehouseSeat), func(w *world) {
			next := w.enc.ToData().Log.NextSeq
			err := w.sneak(hero, true)
			require.ErrorIs(t, err, encounter.ErrSeen, "in the gate's sight the declaration is refused")
			require.Equal(t, next, w.enc.ToData().Log.NextSeq, "and writes nothing")
			refused++
			w.toTheCorner()
			for i := 0; i < 40; i++ {
				err := w.sneak(hero, true)
				if err == nil {
					break
				}
				require.ErrorIs(t, err, encounter.ErrSeen)
				refused++
				require.NoError(t, w.step(hero, besideTheCorner))
				require.NoError(t, w.step(hero, theCorner))
			}
			require.True(t, w.enc.Sneaking(hero), "the hero declared behind the corner")
			require.NoError(t, w.sneak(hero, false))
		})
		require.Positive(t, refused, "precondition: a declaration was refused")
		require.Equal(t, 2, countBeats(t, log, encounter.BeatSneaking), "precondition: only the accepted on, and the off")
	})
}
