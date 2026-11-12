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
	"testing"

	"github.com/stretchr/testify/require"

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

	// eastBeatPost is gate-2's beat post, where she stands with her back to
	// the gate, watching the incomers.
	eastBeatPost = castleCell(1, 10)

	// sneakCompanion is the sneak's second party member, outside beside the
	// stranger (besideTheStranger, an offset cell), whose word to him makes
	// the noise.
	sneakCompanion    = encounter.MemberID("companion")
	besideTheStranger = spatial.Position{X: 6, Y: 14}
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

	// Piece 4, re-aimed by rpg-project#836 (R4 A): gate-1 holds the crossing,
	// gate-2 walks the loop. Posted, the dash is stopped by his body, which
	// writes nothing. Drawn off by a voice (a companion outside asks the
	// stranger about the bustle), he leaves the crossing for a round, and the
	// dash crosses on gate-2's blind beat. Each lands the same log replayed.
	toTheDash := func(w *world) {
		w.walkTo(hero, besideTheCorner)
		if w.at(hero) != besideTheCorner {
			require.NoError(w.t, w.step(hero, besideTheCorner))
		}
		require.NoError(w.t, w.step(hero, theCorner))
		require.NoError(w.t, w.sneak(hero, true), "behind the corner nobody sees the hero declare")
		require.NoError(w.t, w.step(hero, besideTheCorner))
		// gate-2 has just come to her beat post: she stands there this round
		// and the next, with her back to the gate.
		w.paceUntil(hero, besideTheCorner, besideThat, 60, func() bool {
			d := w.dwellOf(kingGate2)
			return d != nil && d.Rounds > 0 && w.at(kingGate2) == eastBeatPost
		})
	}
	t.Run("piece 4: posted, the dash stops at the guard", func(t *testing.T) {
		t.Parallel()
		log := replayRun(t, castleOpener(gatehouseSeat), func(w *world) {
			toTheDash(w)
			require.Equal(t, beneathTheGate, w.at(kingGate1), "precondition: gate-1 holds the crossing")
			for _, cell := range theDash[:len(theDash)-1] {
				if w.at(hero) != cell {
					require.NoError(t, w.step(hero, cell))
				}
			}
			// A bump is answered, not refused (rpg-project#842): the step
			// is taken as a stop, and the hero stays put.
			require.NoError(t, w.step(hero, beneathTheGate), "the posted body stops the dash")
			require.Equal(t, theDash[len(theDash)-2], w.at(hero), "one cell short of him")
			require.False(t, w.fighting(hero), "the civil gate forms no fight")
			require.NoError(t, w.sneak(hero, false))
		})
		require.Equal(t, 2, countBeats(t, log, encounter.BeatSneaking), "precondition: on and off are on the record")
	})
	t.Run("piece 4: drawn off by a shout, the dash in gate-2's window", func(t *testing.T) {
		t.Parallel()
		// GUARDS ANSWER SHOUTS, NOT TALK (rpg-project#933): the companion's
		// line is raised to a shout.
		log := replayRun(t, castleOpenerSaying(gatehouseSeat, map[string]string{"tonight": "shout"}, encounter.MemberInput{
			ID: sneakCompanion, Kind: encounter.KindPlayer, Position: besideTheStranger,
		}), func(w *world) {
			toTheDash(w)
			require.NoError(t, w.speak(encounter.SpeakInput{
				Actor: sneakCompanion, Target: kingPedlar, OptionID: "tonight", Skill: "persuasion", Beaten: true, DC: 15, Total: 20,
			}), "the companion shouts to the stranger")
			start := w.round()
			w.paceUntil(hero, besideTheCorner, besideThat, 12, func() bool { return w.round() > start })
			require.NotEqual(t, beneathTheGate, w.at(kingGate1), "the shout drew gate-1 off the crossing")
			require.Equal(t, eastBeatPost, w.at(kingGate2), "precondition: gate-2 still stands her blind beat")
			for _, cell := range theDash {
				if w.at(hero) != cell {
					require.NoError(t, w.step(hero, cell))
				}
			}
			require.Equal(t, beneathTheGate, w.at(hero))
			require.False(t, w.fighting(hero), "the civil gate forms no fight")
			require.NoError(t, w.sneak(hero, false))
		})
		require.Equal(t, 2, countBeats(t, log, encounter.BeatSneaking), "precondition: on and off are on the record")
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
