// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// pieces_test.go pins SplitPieces and JoinPieces (rpg-project#897, leg b) on
// the small keep's garrison: the pieces a store holds, carried forward save
// by save with the marks of the last one, always join back to the rest byte
// for byte, and a save writes the pieces that changed and nothing else.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// pieceStore is what a store holds: every piece's bytes and the marks the
// session carries forward.
type pieceStore struct {
	pieces map[encounter.PieceKey][]byte
	marks  map[encounter.PieceKey]encounter.Mark
	since  uint64
	prev   []byte // the last rest's JSON, for the oracle
}

// save checkpoints enc, splits the rest against the held marks, writes what
// came out, and holds the new marks. It returns the split and the rest.
func (s *pieceStore) save(t *testing.T, enc *encounter.Encounter) (*encounter.SplitPiecesOutput, []byte) {
	t.Helper()
	ck, err := enc.ToCheckpoint(&encounter.ToCheckpointInput{Since: s.since})
	require.NoError(t, err)
	s.since = ck.Rest.Log.NextSeq
	out, err := encounter.SplitPieces(&encounter.SplitPiecesInput{Data: &ck.Rest, Held: s.marks})
	require.NoError(t, err)
	if s.pieces == nil {
		s.pieces = map[encounter.PieceKey][]byte{}
	}
	for key, p := range out.Pieces {
		s.pieces[key] = p.Bytes
	}
	for _, key := range out.Removed {
		delete(s.pieces, key)
	}
	s.marks = out.Marks
	rest, err := json.Marshal(ck.Rest)
	require.NoError(t, err)
	return out, rest
}

// requireJoinsToRest joins every piece the store holds and holds the
// result to the rest, byte for byte.
func (s *pieceStore) requireJoinsToRest(t *testing.T, rest []byte, after string) {
	t.Helper()
	joined, err := encounter.JoinPieces(&encounter.JoinPiecesInput{Pieces: s.pieces})
	require.NoError(t, err, "join after %s", after)
	raw, err := json.Marshal(joined)
	require.NoError(t, err)
	require.True(t, bytes.Equal(rest, raw), "the joined pieces are the rest byte for byte after %s", after)
}

// changedPieces is the oracle: the keys whose JSON moved between two rests,
// read off the rests' own JSON trees rather than through SplitPieces, plus
// the head (written on every save). Removed keys are those prev has and cur
// does not.
func changedPieces(t *testing.T, prev, cur []byte) (changed, removed []encounter.PieceKey) {
	t.Helper()
	a, b := pieceTree(t, prev), pieceTree(t, cur)
	for key, v := range b {
		if key == encounter.PieceHead || !bytes.Equal(a[key], v) {
			changed = append(changed, key)
		}
	}
	for key := range a {
		if _, ok := b[key]; !ok {
			removed = append(removed, key)
		}
	}
	slices.Sort(changed)
	slices.Sort(removed)
	return changed, removed
}

// pieceTree reads a rest's JSON into its pieces' content by key, from the
// top-level JSON names: a member per id, an observer per holdings key with
// its member's sweep, the journal, and the shared roster and knowledge
// fields. The head's content is not compared; it is written every save.
func pieceTree(t *testing.T, raw []byte) map[encounter.PieceKey][]byte {
	t.Helper()
	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &top))
	out := map[encounter.PieceKey][]byte{encounter.PieceHead: nil, encounter.PieceJournal: top["journal"]}
	shared := func(key encounter.PieceKey, names ...string) {
		var buf bytes.Buffer
		for _, n := range names {
			buf.WriteString(n + "=")
			buf.Write(top[n])
			buf.WriteString(";")
		}
		out[key] = buf.Bytes()
	}
	shared(encounter.PieceRoster, "outcome", "doors", "sight_areas", "interactions", "reserve", "ever_members", "pause")
	shared(encounter.PieceKnowledge, "discovery", "sneak_verdicts")
	var members []json.RawMessage
	require.NoError(t, json.Unmarshal(top["members"], &members))
	for _, m := range members {
		var id struct {
			ID encounter.MemberID `json:"id"`
		}
		require.NoError(t, json.Unmarshal(m, &id))
		out[encounter.MemberPiece(id.ID)] = m
	}
	var per struct {
		Intel struct {
			Holdings map[string]json.RawMessage `json:"holdings"`
		} `json:"intel"`
	}
	require.NoError(t, json.Unmarshal(top["perception"], &per))
	observers := map[string][]byte{}
	for obs, h := range per.Intel.Holdings {
		observers[obs] = append([]byte("holdings="), h...)
	}
	var sweeps map[encounter.MemberID]string
	if raw, ok := top["sight_sweeps"]; ok {
		require.NoError(t, json.Unmarshal(raw, &sweeps))
	}
	for id, sweep := range sweeps {
		obs := string(perception.Qualify("member", id))
		observers[obs] = append(observers[obs], []byte(";sweep="+sweep)...)
	}
	for obs, v := range observers {
		out[encounter.ObserverPiece(encounter.MemberID(obs))] = v
	}
	return out
}

// keysOf is the sorted keys of a split's pieces.
func keysOf(out *encounter.SplitPiecesOutput) []encounter.PieceKey {
	keys := make([]encounter.PieceKey, 0, len(out.Pieces))
	for k := range out.Pieces {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// byShape counts keys by shape and prefix, for the log: head, roster `_`,
// members, knowledge `_`, observers, journal.
func byShape(keys []encounter.PieceKey) string {
	n := map[string]int{}
	for _, k := range keys {
		switch {
		case strings.HasPrefix(string(k), "m:"):
			n["members"]++
		case strings.HasPrefix(string(k), "o:"):
			n["observers"]++
		default:
			n[string(k)]++
		}
	}
	names := make([]string, 0, len(n))
	for k := range n {
		names = append(names, k)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, k := range names {
		parts = append(parts, fmt.Sprintf("%s=%d", k, n[k]))
	}
	return strings.Join(parts, " ")
}

// TestPieces_SavesWhatChanged carries a store forward save by save on the
// small keep's garrison, the marks of each save held for the next, through a
// step, three four-cell paces (some raise the world clock and the patrol walks,
// some do not), a step to a new cell and a player's exit. After every save
// the pieces the store holds join back to the rest byte for byte, and the
// pieces that came out are exactly the ones whose JSON moved, read off the
// rests' own JSON by the oracle, plus the head: their count is the oracle's.
// A pace that raises no round, where the walker ends where he began, writes
// the head and the walker alone. A step writes the head, the walker and the
// observers who saw him, and nothing else. The exit names the leaver's
// piece removed. Last, the walker's stored piece loses a byte and the join
// refuses it by name.
func TestPieces_SavesWhatChanged(t *testing.T) {
	enc := openDrivenSeeing(t, smallKeep(t, true), 8)
	s := &pieceStore{}
	out, rest := s.save(t, enc)
	require.Len(t, out.Pieces, len(out.Marks), "a store with no marks gets every piece")
	require.Empty(t, out.Removed)
	s.requireJoinsToRest(t, rest, "launch")
	prev := rest

	walker := keepPlayers[0]
	walkerPiece := encounter.MemberPiece(walker)
	start := whereIs(t, enc, walker)
	var beside spatial.Position
	for _, d := range hexSteps {
		cell := spatial.Position{X: start.X + d.X, Y: start.Y + d.Y}
		if _, err := enc.Step(&encounter.StepInput{Member: walker, To: cell, EndWalk: true}); err == nil {
			beside = cell
			break
		}
	}
	require.NotEqual(t, start, beside, "the walker has a cell to pace to")
	out, rest = s.save(t, enc)
	s.requireJoinsToRest(t, rest, "the probe step")
	prev = rest

	verb := func(name string, do func()) (*encounter.SplitPiecesOutput, bool) {
		t.Helper()
		round := enc.WorldRound()
		do()
		out, rest := s.save(t, enc)
		s.requireJoinsToRest(t, rest, name)
		changed, removed := changedPieces(t, prev, rest)
		t.Logf("%s, round %d to %d: %s, removed %v", name, round, enc.WorldRound(), byShape(keysOf(out)), out.Removed)
		require.Equal(t, changed, keysOf(out), "%s writes the pieces whose JSON moved and the head", name)
		require.Equal(t, removed, out.Removed, "%s removes the pieces the rest lost", name)
		prev = rest
		return out, enc.WorldRound() > round
	}
	stepTo := func(cell spatial.Position, end bool) {
		_, err := enc.Step(&encounter.StepInput{Member: walker, To: cell, EndWalk: end})
		require.NoError(t, err)
	}

	out, raised := verb("a step back", func() { stepTo(start, true) })
	require.False(t, raised)
	requireStepPieces(t, out, walkerPiece)

	quiet, rounds := 0, 0
	for i := 0; i < 3; i++ {
		out, raised := verb(fmt.Sprintf("pace %d", i), func() {
			for j, cell := range []spatial.Position{beside, start, beside, start} {
				stepTo(cell, j == 3)
			}
		})
		if raised {
			rounds++
			continue
		}
		quiet++
		require.Equal(t, []encounter.PieceKey{encounter.PieceHead, walkerPiece}, keysOf(out),
			"a pace that raised no round writes the head and the walker alone")
	}
	require.Positive(t, quiet, "some pace raised no round")
	require.Positive(t, rounds, "some pace raised a round, so the garrison's pieces moved")

	out, raised = verb("a step to a new cell", func() { stepTo(beside, true) })
	require.False(t, raised)
	requireStepPieces(t, out, walkerPiece)

	leaver := keepPlayers[1]
	out, _ = verb("an exit", func() {
		_, err := enc.Exit(&encounter.ExitInput{Member: leaver})
		require.NoError(t, err)
	})
	require.Contains(t, out.Removed, encounter.MemberPiece(leaver), "the leaver's piece is removed")

	s.pieces[walkerPiece] = s.pieces[walkerPiece][:len(s.pieces[walkerPiece])-1]
	_, err := encounter.JoinPieces(&encounter.JoinPiecesInput{Pieces: s.pieces})
	require.ErrorIs(t, err, encounter.ErrBadPiece, "a corrupt piece fails the join")
	require.ErrorContains(t, err, string(walkerPiece), "by name")
}

// requireStepPieces holds a walker's step to the head, the walker and the
// observers who saw him: at least one observer, and nothing else.
func requireStepPieces(t *testing.T, out *encounter.SplitPiecesOutput, walker encounter.PieceKey) {
	t.Helper()
	observers := 0
	for _, key := range keysOf(out) {
		switch {
		case key == encounter.PieceHead, key == walker:
		case key.Shape() == encounter.ShapeKnowledge && key != encounter.PieceKnowledge:
			observers++
		default:
			// The first look at a cell can append a knowledge fact.
			require.Equal(t, encounter.PieceJournal, key, "a step writes no %q", key)
		}
	}
	require.Contains(t, out.Pieces, encounter.PieceHead)
	require.Contains(t, out.Pieces, walker)
	require.Positive(t, observers, "the observers who saw the step are written")
}

// TestPieces_Refused names what is wrong: a rest carrying the record is not
// split, a missing piece is named, and a piece that does not decode, a key
// that names nothing, a member under another's key and an observer holding
// nothing are each refused by name.
func TestPieces_Refused(t *testing.T) {
	rest := encounter.EncounterData{
		SiteHash: "site", Members: []encounter.MemberData{{ID: "a"}, {ID: "b"}},
		EverMembers: []encounter.MemberID{"a", "b"},
		SightSweeps: map[encounter.MemberID]string{"a": "sweep"},
	}
	out, err := encounter.SplitPieces(&encounter.SplitPiecesInput{Data: &rest})
	require.NoError(t, err)
	held := map[encounter.PieceKey][]byte{}
	for key, p := range out.Pieces {
		held[key] = p.Bytes
	}
	joined, err := encounter.JoinPieces(&encounter.JoinPiecesInput{Pieces: held})
	require.NoError(t, err)
	want, err := json.Marshal(rest)
	require.NoError(t, err)
	got, err := json.Marshal(joined)
	require.NoError(t, err)
	require.Equal(t, string(want), string(got), "the pieces join to the rest")

	join := func(edit func(map[encounter.PieceKey][]byte)) error {
		pieces := map[encounter.PieceKey][]byte{}
		for k, v := range held {
			pieces[k] = v
		}
		edit(pieces)
		_, err := encounter.JoinPieces(&encounter.JoinPiecesInput{Pieces: pieces})
		return err
	}
	for _, key := range []encounter.PieceKey{encounter.PieceHead, encounter.PieceRoster, encounter.PieceKnowledge, encounter.PieceJournal} {
		err := join(func(p map[encounter.PieceKey][]byte) { delete(p, key) })
		require.ErrorIs(t, err, encounter.ErrMissingPiece, "without %s", key)
		require.ErrorContains(t, err, string(key))
	}
	member := encounter.MemberPiece("a")
	for name, tc := range map[string]struct {
		edit func(map[encounter.PieceKey][]byte)
		key  string
	}{
		"corrupt member":          {func(p map[encounter.PieceKey][]byte) { p[member] = p[member][:len(p[member])/2] }, string(member)},
		"corrupt head":            {func(p map[encounter.PieceKey][]byte) { p[encounter.PieceHead] = []byte("{") }, "head"},
		"unknown key":             {func(p map[encounter.PieceKey][]byte) { p["x:1"] = []byte("{}") }, "x:1"},
		"member under another":    {func(p map[encounter.PieceKey][]byte) { p[encounter.MemberPiece("c")] = p[member] }, "m:c"},
		"observer holding nought": {func(p map[encounter.PieceKey][]byte) { p[encounter.ObserverPiece("member|z")] = []byte("{}") }, "o:member|z"},
		"sweep on no member":      {func(p map[encounter.PieceKey][]byte) { p[encounter.ObserverPiece("prop|z")] = []byte(`{"sweep":"s"}`) }, "o:prop|z"},
	} {
		err := join(tc.edit)
		require.ErrorIs(t, err, encounter.ErrBadPiece, name)
		require.ErrorContains(t, err, tc.key, name)
	}

	carrying := rest
	carrying.Log.Entries = make([]record.EntryData, 1)
	_, err = encounter.SplitPieces(&encounter.SplitPiecesInput{Data: &carrying})
	require.ErrorIs(t, err, encounter.ErrRestCarriesRecord)
	err = join(func(p map[encounter.PieceKey][]byte) { p[encounter.PieceHead] = []byte(`{"log":{"entries":[{}]}}`) })
	require.ErrorIs(t, err, encounter.ErrRestCarriesRecord)
	_, err = encounter.SplitPieces(nil)
	require.ErrorIs(t, err, encounter.ErrNilInput)
	_, err = encounter.JoinPieces(nil)
	require.ErrorIs(t, err, encounter.ErrNilInput)
}

// TestPieces_UnchangedRestWritesTheHeadAlone splits the same rest twice:
// with the first split's marks held, only the head comes out; a piece the
// store lost its mark for comes out again; a held key the rest lacks is
// removed.
func TestPieces_UnchangedRestWritesTheHeadAlone(t *testing.T) {
	rest := encounter.EncounterData{SiteHash: "site", Members: []encounter.MemberData{{ID: "a"}}}
	first, err := encounter.SplitPieces(&encounter.SplitPiecesInput{Data: &rest})
	require.NoError(t, err)
	held := map[encounter.PieceKey]encounter.Mark{}
	for k, m := range first.Marks {
		held[k] = m
	}
	again, err := encounter.SplitPieces(&encounter.SplitPiecesInput{Data: &rest, Held: held})
	require.NoError(t, err)
	require.Equal(t, []encounter.PieceKey{encounter.PieceHead}, keysOf(again))
	require.Empty(t, again.Removed)
	require.Equal(t, first.Marks, again.Marks)

	delete(held, encounter.MemberPiece("a"))
	held[encounter.MemberPiece("gone")] = 1
	lost, err := encounter.SplitPieces(&encounter.SplitPiecesInput{Data: &rest, Held: held})
	require.NoError(t, err)
	require.Equal(t, []encounter.PieceKey{encounter.PieceHead, encounter.MemberPiece("a")}, keysOf(lost))
	require.Equal(t, []encounter.PieceKey{encounter.MemberPiece("gone")}, lost.Removed)
}

// BenchmarkSplitPiecesGarrison is what a save's split costs and writes on
// the 10k keep's posts-asleep garrison (F26's keep): one op is the garrison
// bench's Move, four cells paced beside the party start, then the save's
// ToCheckpoint (untimed) and SplitPieces against the last save's marks
// (timed). A Move that raised the world clock is a round Move; its pieces'
// bytes are reported apart from a Move that raised none. split-all-ms is one
// split with no marks, every piece digested and returned, as at Launch.
// The record's segment is not a piece and is not counted.
//
//	go test -run '^$' -bench BenchmarkSplitPiecesGarrison -benchtime 30x .
func BenchmarkSplitPiecesGarrison(b *testing.B) {
	enc := openGarrisonKeep(b, true)
	player := keepPlayers[0]
	start := whereIs(b, enc, player)
	beside := besideStart(b, enc, player, start)
	walk := []spatial.Position{beside, start, beside, start}

	ck, err := enc.ToCheckpoint(&encounter.ToCheckpointInput{})
	if err != nil {
		b.Fatal(err)
	}
	since := ck.Rest.Log.NextSeq
	started := time.Now()
	all, err := encounter.SplitPieces(&encounter.SplitPiecesInput{Data: &ck.Rest})
	if err != nil {
		b.Fatal(err)
	}
	allTook := time.Since(started)
	allBytes := 0
	for _, p := range all.Pieces {
		allBytes += len(p.Bytes)
	}
	held := all.Marks

	var splitNs time.Duration
	var roundMoves, quietMoves, roundBytes, quietBytes, roundPieces, quietPieces int
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		round := enc.WorldRound()
		for j, cell := range walk {
			if _, err := enc.Step(&encounter.StepInput{Member: player, To: cell, EndWalk: j == len(walk)-1}); err != nil {
				b.Fatalf("op %d step %d to %v: %v", i, j, cell, err)
			}
		}
		ck, err := enc.ToCheckpoint(&encounter.ToCheckpointInput{Since: since})
		if err != nil {
			b.Fatal(err)
		}
		since = ck.Rest.Log.NextSeq
		b.StartTimer()
		started := time.Now()
		out, err := encounter.SplitPieces(&encounter.SplitPiecesInput{Data: &ck.Rest, Held: held})
		splitNs += time.Since(started)
		if err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		held = out.Marks
		written := 0
		for _, p := range out.Pieces {
			written += len(p.Bytes)
		}
		if enc.WorldRound() != round {
			roundMoves++
			roundBytes += written
			roundPieces += len(out.Pieces)
		} else {
			quietMoves++
			quietBytes += written
			quietPieces += len(out.Pieces)
		}
		b.StartTimer()
	}
	b.StopTimer()
	b.ReportMetric(float64(splitNs)/float64(b.N)/float64(time.Millisecond), "split-ms")
	b.ReportMetric(float64(allTook)/float64(time.Millisecond), "split-all-ms")
	b.ReportMetric(float64(allBytes)/1024, "all-pieces-KB")
	b.ReportMetric(float64(len(all.Pieces)), "all-pieces")
	b.ReportMetric(float64(roundMoves), "round-moves")
	b.ReportMetric(float64(quietMoves), "quiet-moves")
	if roundMoves > 0 {
		b.ReportMetric(float64(roundBytes)/float64(roundMoves)/1024, "round-KB")
		b.ReportMetric(float64(roundPieces)/float64(roundMoves), "round-pieces")
	}
	if quietMoves > 0 {
		b.ReportMetric(float64(quietBytes)/float64(quietMoves)/1024, "quiet-KB")
		b.ReportMetric(float64(quietPieces)/float64(quietMoves), "quiet-pieces")
	}
}
