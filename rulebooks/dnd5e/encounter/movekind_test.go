package encounter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// tableChaseRef stands in for a table driver's own routed walk: a chase, a
// search, a patrol rejoin. It names a cause and is the creature's own walk.
var tableChaseRef = core.Ref{Module: "encounter", Type: "table", ID: "chase"}

// directOnce runs one directive inside its own act, as a host verb does.
func directOnce(t *testing.T, enc *encounter.Encounter, in encounter.DirectInput) (encounter.DirectOutput, error) {
	t.Helper()
	_, err := enc.OpenAct(&encounter.OpenActInput{Verb: "push"})
	require.NoError(t, err)
	defer func() { require.NoError(t, enc.CloseAct()) }()
	return enc.Direct(context.Background(), in)
}

// TestAMovedBeatNamesItsKind is the law of rpg-project#825 R1 on the record:
// whoever decides the walk names the kind, `kind` is written only for
// compelled and displaced, and `forced` is written exactly when the kind is
// displaced. A cause never names the kind: a Dash and a table chase carry a
// cause and are own.
func TestAMovedBeatNamesItsKind(t *testing.T) {
	away := encounter.Routed{Policy: encounter.MoveAway, Anchor: alice}
	tableChase, compelled := away, away
	tableChase.Cause = tableChaseRef
	compelled.Cause, compelled.Kind = commandedRef, encounter.MoveCompelled

	turns := []struct {
		name      string
		intent    encounter.TurnIntent
		wantCause bool
		wantKind  string
	}{
		{"an own Move", encounter.Move{Path: []spatial.Position{cellAt(7, 2)}}, false, ""},
		{"a Dash", encounter.Move{Path: []spatial.Position{cellAt(7, 2)}, Dash: true}, true, ""},
		{"a table chase (Routed, zero kind)", tableChase, true, ""},
		{"a compelled Routed", compelled, true, "compelled"},
	}
	for _, tc := range turns {
		t.Run(tc.name, func(t *testing.T) {
			driver := &scriptedDriver{intents: []encounter.TurnIntent{tc.intent}}
			enc := routedScene(t, quietMover{}, &downList{}, driver)
			_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
			require.NoError(t, err)

			moved := movedBeatsOf(t, enc, alice)
			require.NotEmpty(t, moved, "the walk wrote its beats")
			for _, beat := range moved {
				require.NotContains(t, beat, "forced", "a turn's walk is never displaced")
				if tc.wantKind == "" {
					require.NotContains(t, beat, "kind", "own is absent")
				} else {
					require.Equal(t, tc.wantKind, beat["kind"])
				}
				if tc.wantCause {
					require.Contains(t, beat, "cause", "the cause still names what moved the body")
				}
			}
		})
	}

	directives := []struct {
		kind       encounter.MoveKind
		wantForced bool
	}{
		{encounter.MoveDisplaced, true},
		{encounter.MoveCompelled, false},
	}
	for _, tc := range directives {
		t.Run("Direct "+string(tc.kind), func(t *testing.T) {
			enc := routedScene(t, quietMover{}, &downList{}, passDriver{})
			_, err := directOnce(t, enc, encounter.DirectInput{
				Mover: goblin, Cause: thunderwaveRef, Route: fleeRoute(), Kind: tc.kind,
			})
			require.NoError(t, err)

			moved := movedBeatsOf(t, enc, alice)
			require.Len(t, moved, len(fleeRoute()))
			for _, beat := range moved {
				require.Equal(t, string(tc.kind), beat["kind"])
				if tc.wantForced {
					require.Equal(t, true, beat["forced"], "forced mirrors displaced")
				} else {
					require.NotContains(t, beat, "forced", "forced is written only for displaced")
				}
			}
		})
	}
}

// TestADirectiveWithNoKindIsRefused: a directive is somebody else's decision,
// so it names compelled or displaced. Empty and own are refused (fail
// closed), and nothing moves.
func TestADirectiveWithNoKindIsRefused(t *testing.T) {
	for _, kind := range []encounter.MoveKind{"", encounter.MoveOwn, "sideways"} {
		t.Run(string(kind), func(t *testing.T) {
			mover := &recordingMover{}
			enc := routedScene(t, mover, &downList{}, passDriver{})
			_, err := directOnce(t, enc, encounter.DirectInput{
				Mover: goblin, Cause: thunderwaveRef, Route: fleeRoute(), Kind: kind,
			})
			require.ErrorIs(t, err, encounter.ErrBadMoveKind)
			require.Empty(t, mover.calls, "nothing was announced")
			require.Equal(t, cellAt(6, 2), positionOfMember(t, enc, goblin), "and nothing moved")
			require.Empty(t, movedBeatsOf(t, enc, alice))
		})
	}
}

// TestARoutedTurnIsNeverDisplaced: a turn walks on its own legs, so a Routed
// intent naming displaced is a malformed intent, refused before any step.
func TestARoutedTurnIsNeverDisplaced(t *testing.T) {
	mover := &recordingMover{}
	driver := &scriptedDriver{intents: []encounter.TurnIntent{encounter.Routed{
		Policy: encounter.MoveAway, Anchor: alice, Cause: commandedRef, Kind: encounter.MoveDisplaced,
	}}}
	enc := routedScene(t, mover, &downList{}, driver)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.ErrorIs(t, err, encounter.ErrBadMoveKind)
	require.Empty(t, mover.calls, "nothing was announced")
	require.Equal(t, cellAt(6, 2), positionOfMember(t, enc, goblin), "and nothing moved")
}

// TestAHeldDirectiveResumesWithItsKind: a compelled directive paused on a
// window keeps its kind on every beat after the resume, through a reload.
func TestAHeldDirectiveResumesWithItsKind(t *testing.T) {
	mover := &pausingMover{pauseAt: map[int]bool{1: true}}
	enc := routedScene(t, mover, &downList{}, passDriver{})
	out, err := directOnce(t, enc, encounter.DirectInput{
		Mover: goblin, Cause: whispersRef, Route: fleeRoute(), Kind: encounter.MoveCompelled,
	})
	require.NoError(t, err)
	require.True(t, out.Paused)

	data := enc.ToData()
	require.NotNil(t, data.Pause)
	require.Equal(t, "compelled", data.Pause.MoveKind, "the hold stores its kind")
	require.False(t, data.Pause.Forced, "and no forced: it is not displaced")

	loaded, lerr := encounter.LoadEncounter(loadInput(data, mover, &downList{}))
	require.NoError(t, lerr)
	_, rerr := loaded.Resume(context.Background())
	require.NoError(t, rerr)

	for _, call := range mover.calls {
		require.Equal(t, encounter.MoveCompelled, call.Kind, "every announced cell, before and after the hold")
	}
	moved := movedBeatsOf(t, loaded, alice)
	require.Len(t, moved, len(fleeRoute()))
	for i, beat := range moved {
		require.Equal(t, "compelled", beat["kind"], "beat %d keeps the kind across the hold", i)
		require.NotContains(t, beat, "forced")
	}
}

// TestAPausedRoutedTurnResumesWithItsKind: a compelled Routed turn paused on a
// window stores its intent's kind and resumes with it, through a reload; a
// table Routed turn (zero kind) resumes with no kind.
func TestAPausedRoutedTurnResumesWithItsKind(t *testing.T) {
	cases := []struct {
		name     string
		cause    core.Ref
		kind     encounter.MoveKind
		announce encounter.MoveKind
		wantKind string
	}{
		{"compelled", commandedRef, encounter.MoveCompelled, encounter.MoveCompelled, "compelled"},
		{"a table walk", tableChaseRef, "", encounter.MoveOwn, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mover := &pausingMover{pauseAt: map[int]bool{1: true}}
			driver := &scriptedDriver{intents: []encounter.TurnIntent{encounter.Routed{
				Policy: encounter.MoveAway, Anchor: alice, Cause: tc.cause, Kind: tc.kind,
			}}}
			enc := routedScene(t, mover, &downList{}, driver)
			_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
			require.NoError(t, err)
			require.True(t, pausedAs(enc, encounter.PauseTurn), "the second step's window holds the turn")

			data := enc.ToData()
			require.Equal(t, tc.wantKind, data.Pause.MoveKind)
			require.False(t, data.Pause.Forced)

			loaded, lerr := encounter.LoadEncounter(loadInput(data, mover, &downList{}))
			require.NoError(t, lerr)
			_, rerr := loaded.Resume(context.Background())
			require.NoError(t, rerr)

			moved := movedBeatsOf(t, loaded, alice)
			require.Greater(t, len(moved), 2, "the resumed half walked on")
			for i, beat := range moved {
				if tc.wantKind == "" {
					require.NotContains(t, beat, "kind", "beat %d: own is absent", i)
				} else {
					require.Equal(t, tc.wantKind, beat["kind"], "beat %d keeps the kind across the pause", i)
				}
				require.NotContains(t, beat, "forced", "beat %d: a turn is never displaced", i)
			}
			for i, call := range mover.calls {
				require.Equal(t, tc.announce, call.Kind, "announced step %d carries the intent's kind", i)
			}
		})
	}
}

// TestAStoredPauseLoadsItsKind: a stored `move_kind` round trips; a pause
// from before the kind derives it from `forced` alone (true is displaced,
// else own).
func TestAStoredPauseLoadsItsKind(t *testing.T) {
	held := func(t *testing.T, kind encounter.MoveKind) (encounter.EncounterData, *pausingMover) {
		mover := &pausingMover{pauseAt: map[int]bool{0: true}}
		enc := routedScene(t, mover, &downList{}, passDriver{})
		out, err := directOnce(t, enc, encounter.DirectInput{
			Mover: goblin, Cause: whispersRef, Route: fleeRoute(), Kind: kind,
		})
		require.NoError(t, err)
		require.True(t, out.Paused)
		return enc.ToData(), mover
	}
	resumedKinds := func(t *testing.T, data encounter.EncounterData, mover *pausingMover) []any {
		loaded, err := encounter.LoadEncounter(loadInput(data, mover, &downList{}))
		require.NoError(t, err)
		_, err = loaded.Resume(context.Background())
		require.NoError(t, err)
		var kinds []any
		for _, beat := range movedBeatsOf(t, loaded, alice) {
			kinds = append(kinds, beat["kind"])
		}
		return kinds
	}

	t.Run("move_kind round trips", func(t *testing.T) {
		for _, kind := range []encounter.MoveKind{encounter.MoveCompelled, encounter.MoveDisplaced} {
			data, mover := held(t, kind)
			require.Equal(t, string(kind), data.Pause.MoveKind)
			require.Equal(t, kind.Displaced(), data.Pause.Forced, "forced mirrors displaced")
			for _, k := range resumedKinds(t, data, mover) {
				require.Equal(t, string(kind), k)
			}
		}
	})
	t.Run("legacy forced true is displaced", func(t *testing.T) {
		data, mover := held(t, encounter.MoveDisplaced)
		data.Pause.MoveKind = ""
		for _, k := range resumedKinds(t, data, mover) {
			require.Equal(t, "displaced", k)
		}
	})
	t.Run("legacy without either is own", func(t *testing.T) {
		data, mover := held(t, encounter.MoveCompelled)
		data.Pause.MoveKind, data.Pause.Forced = "", false
		for _, k := range resumedKinds(t, data, mover) {
			require.Nil(t, k, "own is absent")
		}
	})
}
