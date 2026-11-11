// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// checkpoint_pieces_test.go pins the pieces capability (rpg-project#897 leg
// b) end to end over an in-memory store that holds the rest as pieces: what
// each verb's save carries, against an oracle read off the stored worlds' own
// JSON, and that a load through the pieces is the load a whole-rest host
// gives.

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/keepgen"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// storedPiece is one piece as the store holds it: its bytes and the mark
// written beside them, as a host prefixes its value with the mark.
type storedPiece struct {
	bytes []byte
	mark  encounter.Mark
}

// piecesStore is an in-memory PiecesEncounterRepository over the appending
// store: it keeps each encounter's rest as pieces by key with their marks,
// and a fenced read joins them (encounter.JoinPieces) and then the record's
// segments (record.JoinData), as a host's read does. A save applies the
// pieces it carries and drops the removed ones on a copy, joins the copy, and
// hands the joined rest to the appending store's own save, which checks the
// version and writes the segment, the session record, sheets and seats; only
// a landed save keeps the copy.
type piecesStore struct {
	*appendingStore
	pieces map[string]map[encounter.PieceKey]storedPiece

	// calls is every SavePieces call, landed or not, in order.
	calls []piecesCall
	// appendedSaves counts SaveAppended calls, which a pieces host never
	// gets from the session.
	appendedSaves int
}

// piecesCall is what one SavePieces carried.
type piecesCall struct {
	keys     []encounter.PieceKey
	removed  []encounter.PieceKey
	marks    map[encounter.PieceKey]encounter.Mark
	bytes    int
	expected uint64
	dataSent bool
	landed   bool
}

func newPiecesStore() *piecesStore {
	return &piecesStore{
		appendingStore: newAppendingStore(),
		pieces:         map[string]map[encounter.PieceKey]storedPiece{},
	}
}

func (p *piecesStore) SaveAppended(
	ctx context.Context, in *session.SaveAppendedInput,
) (*session.SaveFencedOutput, error) {
	p.appendedSaves++
	return p.appendingStore.SaveAppended(ctx, in)
}

func (p *piecesStore) SavePieces(
	ctx context.Context, in *session.SavePiecesInput,
) (*session.SaveFencedOutput, error) {
	call := piecesCall{
		keys: slices.Sorted(maps.Keys(in.Pieces)), removed: in.Removed, marks: in.Marks,
		expected: in.Expected, dataSent: in.Data != nil,
	}
	for _, piece := range in.Pieces {
		call.bytes += len(piece.Bytes)
	}

	// A first save starts the store over, as a host's DEL does.
	next := map[encounter.PieceKey]storedPiece{}
	if in.Expected != 0 {
		maps.Copy(next, p.pieces[in.Encounter])
	}
	for key, piece := range in.Pieces {
		next[key] = storedPiece{bytes: piece.Bytes, mark: piece.Mark}
	}
	for _, key := range in.Removed {
		delete(next, key)
	}
	rest, err := joinStored(next)
	if err != nil {
		p.calls = append(p.calls, call)
		return nil, err
	}
	appended := in.SaveAppendedInput
	appended.Data = rest
	out, err := p.appendingStore.SaveAppended(ctx, &appended)
	if err != nil {
		p.calls = append(p.calls, call)
		return nil, err
	}
	call.landed = true
	p.calls = append(p.calls, call)
	p.pieces[in.Encounter] = next
	return out, nil
}

// GetFencedEncounter reads through the pieces: the stored pieces joined, the
// segments joined onto the head, and the marks stored beside the pieces.
func (p *piecesStore) GetFencedEncounter(
	ctx context.Context, in *session.GetFencedEncounterInput,
) (*session.GetFencedEncounterOutput, error) {
	stored, ok := p.pieces[in.Encounter]
	if !ok {
		return p.appendingStore.GetFencedEncounter(ctx, in)
	}
	rest, err := joinStored(stored)
	if err != nil {
		return nil, err
	}
	rest.Log, err = joinSegments(rest.Log, p.floors[in.Encounter], p.segments[in.Encounter])
	if err != nil {
		return nil, err
	}
	marks := map[encounter.PieceKey]encounter.Mark{}
	for key, piece := range stored {
		marks[key] = piece.mark
	}
	out := &session.GetFencedEncounterOutput{Data: rest, Version: p.versions[in.Encounter], Marks: marks}
	if hook := p.onGet; hook != nil {
		p.onGet = nil
		hook()
	}
	return out, nil
}

// joinStored joins every stored piece into a rest.
func joinStored(stored map[encounter.PieceKey]storedPiece) (*encounter.EncounterData, error) {
	raw := make(map[encounter.PieceKey][]byte, len(stored))
	for key, piece := range stored {
		raw[key] = piece.bytes
	}
	return encounter.JoinPieces(&encounter.JoinPiecesInput{Pieces: raw})
}

func (p *piecesStore) lastCall() piecesCall { return p.calls[len(p.calls)-1] }

// appendingOnly hides the pieces capability behind a wrapper that forwards
// the appending methods alone: the host that has not adopted it.
type appendingOnly struct{ fencedOnly }

func (a appendingOnly) SaveAppended(
	ctx context.Context, in *session.SaveAppendedInput,
) (*session.SaveFencedOutput, error) {
	return a.store.SaveAppended(ctx, in)
}

var (
	smallKeepOnce     sync.Once
	smallKeepCompiled dungeonspec.Compiled
	errSmallKeep      error
)

// smallKeep is the encounter suite's small keep garrison (rpg-project#897
// leg b): keepgen's plan shrunk to 1,230 hexes, seed 1, four monsters per 100
// hexes, every one given orders, posts listening, one walking a patrol. A
// garrison small enough to launch in a test that still has observers who see
// a step and a world clock that raises rounds.
func smallKeep(t testing.TB) dungeonspec.Compiled {
	t.Helper()
	smallKeepOnce.Do(func() {
		plan := keepgen.DefaultPlan()
		for i := range plan.Rooms {
			r := &plan.Rooms[i]
			r.Width, r.Depth = 5, 3
			switch r.ID {
			case "hidden-stash", "tower":
				r.Width = 3
			case "larder":
				r.Depth = 6
			}
		}
		plan.Grounds.West.Width, plan.Grounds.South.Depth = 3, 3
		opts := keepgen.DefaultOptions()
		opts.Hexes, opts.Seed, opts.Key = 1230, 1, "small-keep"
		opts.MonsterDensity = 4
		_, report, err := keepgen.Generate(plan, opts)
		if err != nil {
			errSmallKeep = err
			return
		}
		opts.Garrison, opts.Patrols, opts.PostsListen = report.Monsters, 1, true
		raw, _, err := keepgen.Generate(plan, opts)
		if err != nil {
			errSmallKeep = err
			return
		}
		smallKeepCompiled, errSmallKeep = dungeonspec.Load(raw)
	})
	if errSmallKeep != nil {
		t.Fatal(errSmallKeep)
	}
	return smallKeepCompiled
}

// neighbourSteps are the six neighbours of a cell, in the axial frame verbs
// speak.
var neighbourSteps = []spatial.Position{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}, {X: 1, Y: -1}, {X: -1, Y: 1}}

// watchedHall is a hall wide enough to walk in with alice and bob seated,
// each watching the other: members, observers and knowledge to split, and no
// fight to start.
func watchedHall() scene {
	return scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 10, 6)}},
		Party: []sceneSeat{seatAt("alice", 1, 1), seatAt("bob", 1, 3)},
	}
}

// piecesBase is what both pieces suites share: the store and the helpers.
type piecesBase struct {
	suite.Suite
	store *piecesStore
}

// PiecesCheckpointSuite is the pieces capability on small authored halls.
type PiecesCheckpointSuite struct{ piecesBase }

func TestPiecesCheckpointSuite(t *testing.T) { suite.Run(t, new(PiecesCheckpointSuite)) }

func (s *piecesBase) SetupTest() {
	s.store = newPiecesStore()
}

func (s *piecesBase) managerOver(store *appendingStore, encounters session.EncounterRepository) *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: &fakeStream{}, Seats: store.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Driver(),
		Sessions: storeSessions{store: store.fencedStore}, Encounters: encounters, Characters: store.chars,
	})
	s.Require().NoError(err)
	return mgr
}

// launchKeep launches the small keep with alice and bob in its first seats.
func (s *piecesBase) launchKeep(store *appendingStore, encounters session.EncounterRepository) *session.Manager {
	mgr := s.managerOver(store, encounters)
	compiled := smallKeep(s.T())
	_, err := mgr.Launch(context.Background(), &session.LaunchInput{
		Session: testSession, Dungeon: &compiled, Party: []string{"alice", "bob"},
	})
	s.Require().NoError(err)
	return mgr
}

// where is a member's cell and the world round.
func (s *piecesBase) where(mgr *session.Manager, member string) (spatial.Position, int) {
	enc, err := mgr.LoadWorldForTest(context.Background(), testSession)
	s.Require().NoError(err)
	for _, m := range enc.WorldView().Members {
		if string(m.ID) == member && m.Cell != nil {
			return spatial.Position{X: m.Cell.X, Y: m.Cell.Y}, int(enc.WorldRound())
		}
	}
	s.FailNow(member + " is not placed")
	return spatial.Position{}, 0
}

// storedJSON is the world the pieces store assembles, as JSON.
func (s *piecesBase) storedJSON() []byte {
	out, err := s.store.GetFencedEncounter(context.Background(), &session.GetFencedEncounterInput{Encounter: testSession})
	s.Require().NoError(err)
	raw, err := json.Marshal(out.Data)
	s.Require().NoError(err)
	return raw
}

// step finds a neighbour of from that alice can walk to.
func (s *piecesBase) step(mgr *session.Manager, from spatial.Position) spatial.Position {
	for _, d := range neighbourSteps {
		to := spatial.Position{X: from.X + d.X, Y: from.Y + d.Y}
		_, err := mgr.Move(context.Background(), &session.MoveInput{
			Session: testSession, Member: "alice", Path: []spatial.Position{to},
		})
		if err == nil {
			return to
		}
	}
	s.FailNow("alice has no cell to step to")
	return spatial.Position{}
}

// TestLaunchWritesEveryPiece: the first save of a run holds no marks, so it
// sends every piece the rest has, at version 0, with no rest beside them,
// and the marks the store keeps are the marks the save named.
func (s *PiecesCheckpointSuite) TestLaunchWritesEveryPiece() {
	mgr := s.managerOver(s.store.appendingStore, s.store)
	_, err := mgr.Launch(context.Background(), sceneInput(watchedHall()))
	s.Require().NoError(err)

	s.Require().Len(s.store.calls, 1)
	launch := s.store.calls[0]
	s.True(launch.landed)
	s.Zero(launch.expected)
	s.False(launch.dataSent, "the pieces replace the rest")
	s.Equal(slices.Sorted(maps.Keys(launch.marks)), launch.keys, "every piece the rest has")
	s.Empty(launch.removed)
	for _, key := range []encounter.PieceKey{
		encounter.PieceHead, encounter.PieceRoster, encounter.PieceKnowledge, encounter.PieceJournal,
		encounter.MemberPiece("alice"), encounter.MemberPiece("bob"),
		encounter.ObserverPiece("member|alice"), encounter.ObserverPiece("member|bob"),
	} {
		s.Contains(launch.keys, key)
	}
	s.Equal(launch.marks, s.storedMarks(), "the store holds the marks the save named")
	s.Zero(s.store.appendedSaves, "a pieces host is never handed the whole rest")
	s.Zero(s.store.wholeSaves)
	s.Zero(s.store.plainSaves)
}

// storedMarks is the marks the store returns on a fenced read.
func (s *piecesBase) storedMarks() map[encounter.PieceKey]encounter.Mark {
	out, err := s.store.GetFencedEncounter(context.Background(), &session.GetFencedEncounterInput{Encounter: testSession})
	s.Require().NoError(err)
	return out.Marks
}

// PiecesKeepSuite is the pieces capability on the small keep garrison, apart
// from the hall's suite so its launch is priced once and alone.
type PiecesKeepSuite struct{ piecesBase }

func TestPiecesKeepSuite(t *testing.T) { suite.Run(t, new(PiecesKeepSuite)) }

// TestAVerbWritesWhatMoved is the leg's headline on the small keep: after
// Launch, every verb's save carries exactly the pieces whose JSON moved in
// the stored world, read off that JSON by an oracle that never calls
// SplitPieces, plus the head. A step writes the head, the walker and the
// observers who saw her, no other member. A pace out and back that raises no
// round changes nothing but the walker's pace and writes the head and the
// walker alone. Every landed save's marks are what the next load returns.
func (s *PiecesKeepSuite) TestAVerbWritesWhatMoved() {
	began := time.Now()
	mgr := s.launchKeep(s.store.appendingStore, s.store)
	s.T().Logf("launch: %d pieces, %d bytes, %v", len(s.store.calls[0].keys), s.store.calls[0].bytes,
		time.Since(began).Round(time.Millisecond))
	prev := s.storedJSON()
	verb := func(name string, do func()) (piecesCall, bool) {
		round := worldRound(s.T(), prev)
		calls := len(s.store.calls)
		began := time.Now()
		do()
		took := time.Since(began)
		s.Require().Len(s.store.calls, calls+1, "%s saved once", name)
		call := s.store.lastCall()
		s.Require().True(call.landed, name)
		cur := s.storedJSON()
		changed, removed := changedPieces(s.T(), prev, cur)
		s.Require().Equal(changed, call.keys, "%s writes the pieces whose JSON moved and the head", name)
		s.Require().Equal(removed, call.removed, "%s", name)
		s.Require().Equal(call.marks, s.storedMarks(), "%s: the next load holds the save's marks", name)
		s.Require().False(call.dataSent)
		prev = cur
		s.T().Logf("%s, round %d to %d: %d pieces, %d bytes, %v", name, round, worldRound(s.T(), cur),
			len(call.keys), call.bytes, took.Round(time.Millisecond))
		return call, worldRound(s.T(), cur) > round
	}

	start, _ := s.where(mgr, "alice")
	var beside spatial.Position
	call, raised := verb("a step", func() { beside = s.step(mgr, start) })
	if !raised {
		s.requireStep(call)
	}

	quiet := 0
	for i := 0; i < 4 && quiet == 0; i++ {
		call, raised := verb("a pace", func() {
			_, err := mgr.Move(context.Background(), &session.MoveInput{
				Session: testSession, Member: "alice", Path: []spatial.Position{start, beside},
			})
			s.Require().NoError(err)
		})
		if raised {
			continue
		}
		quiet++
		s.Equal([]encounter.PieceKey{encounter.PieceHead, encounter.MemberPiece("alice")}, call.keys,
			"a pace that raised no round writes the head and the walker alone")
		s.Empty(call.removed)
	}
	s.Positive(quiet, "some pace raised no round")
	s.Zero(s.store.appendedSaves)
}

// requireStep holds a step's save to the head, the walker and at least one
// observer who saw her; the journal may move on a first look, nothing else.
func (s *piecesBase) requireStep(call piecesCall) {
	observers := 0
	for _, key := range call.keys {
		switch {
		case key == encounter.PieceHead, key == encounter.MemberPiece("alice"), key == encounter.PieceJournal:
		case key.Shape() == encounter.ShapeKnowledge && key != encounter.PieceKnowledge:
			observers++
		default:
			s.Failf("a step writes no other piece", "%q", key)
		}
	}
	s.Contains(call.keys, encounter.PieceHead)
	s.Contains(call.keys, encounter.MemberPiece("alice"))
	s.Positive(observers, "the observers who saw the step are written")
}

// TestALeaverIsRemoved: a player's exit names the leaver's piece removed,
// and the joined world no longer has the member.
func (s *PiecesCheckpointSuite) TestALeaverIsRemoved() {
	hall := authoredWorld()
	hall.Party = append(hall.Party, seatAt("bob", 3, 3))
	mgr := s.managerOver(s.store.appendingStore, s.store)
	_, err := mgr.Launch(context.Background(), sceneInput(hall))
	s.Require().NoError(err)

	_, err = mgr.Exit(context.Background(), &session.ExitInput{Session: testSession, Member: "bob"})
	s.Require().NoError(err)

	call := s.store.lastCall()
	s.Require().True(call.landed)
	s.Contains(call.removed, encounter.MemberPiece("bob"), "the leaver's piece is removed")
	s.NotContains(call.keys, encounter.MemberPiece("bob"))
	s.NotContains(s.storedMarks(), encounter.MemberPiece("bob"))
	out, err := s.store.GetFencedEncounter(context.Background(), &session.GetFencedEncounterInput{Encounter: testSession})
	s.Require().NoError(err)
	for _, m := range out.Data.Members {
		s.NotEqual(encounter.MemberID("bob"), m.ID, "the joined world no longer holds the leaver")
	}
}

// TestAStaleSaveLandsNothing: a writer that saves between this verb's load
// and its save leaves the pieces save refused, and the store keeps every
// piece and mark it held.
func (s *PiecesCheckpointSuite) TestAStaleSaveLandsNothing() {
	mgr := s.managerOver(s.store.appendingStore, s.store)
	_, err := mgr.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().NoError(err)
	before := maps.Clone(s.store.pieces[testSession])
	world := s.storedJSON()
	s.store.onGet = func() { s.store.versions[testSession]++ }

	_, err = mgr.Move(context.Background(), &session.MoveInput{
		Session: testSession, Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})

	s.Require().ErrorIs(err, session.ErrStaleCheckpoint)
	s.False(s.store.lastCall().landed)
	s.Equal(before, s.store.pieces[testSession], "the store keeps every piece it held")
	s.Equal(world, s.storedJSON())
}

// TestThePiecesLoadIsTheWholeLoad: the same run and verbs over a pieces host
// and over a host that stores the rest whole load the same world, byte for
// byte but for the ids each run mints, after Launch and after every Move, on
// a hall where bob watches alice walk.
func (s *PiecesCheckpointSuite) TestThePiecesLoadIsTheWholeLoad() {
	whole := newAppendingStore()
	launch := func(store *appendingStore, encounters session.EncounterRepository) *session.Manager {
		mgr := s.managerOver(store, encounters)
		_, err := mgr.Launch(context.Background(), sceneInput(watchedHall()))
		s.Require().NoError(err)
		return mgr
	}
	piecesMgr := launch(s.store.appendingStore, s.store)
	wholeMgr := launch(whole, whole)
	same := func(after string) {
		got := s.storedJSON()
		data, err := whole.GetEncounter(context.Background(), testSession)
		s.Require().NoError(err)
		want, err := json.Marshal(data)
		s.Require().NoError(err)
		s.Require().True(bytes.Equal(withoutEntryIDs(s.T(), want), withoutEntryIDs(s.T(), got)),
			"the pieces load is the whole load after %s", after)
	}
	same("launch")

	start, _ := s.where(piecesMgr, "alice")
	beside := s.step(piecesMgr, start)
	s.Equal(beside, s.step(wholeMgr, start))
	same("a step")
	for range 3 {
		for _, mgr := range []*session.Manager{piecesMgr, wholeMgr} {
			_, err := mgr.Move(context.Background(), &session.MoveInput{
				Session: testSession, Member: "alice", Path: []spatial.Position{start, beside},
			})
			s.Require().NoError(err)
		}
		same("a pace")
	}
	s.NotEmpty(whole.appends, "the twin saved through the appending path")
}

// TestASeatedLevelWritesTheHeadAlone: LevelUp writes the run's world back as
// it read it, so against the marks the load returned only the head moves.
func (s *PiecesCheckpointSuite) TestASeatedLevelWritesTheHeadAlone() {
	s.store.chars.byID["alice"] = advancingFighter("alice")
	mgr := s.managerOver(s.store.appendingStore, s.store)
	_, err := mgr.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().NoError(err)

	out, err := mgr.LevelUp(context.Background(), &session.LevelUpInput{
		Character: "alice", HitPointMethod: session.HitPointsAverage,
	})
	s.Require().NoError(err)

	s.Contains(out.Saved.Written, "character:alice")
	level := s.store.lastCall()
	s.True(level.landed)
	s.Equal([]encounter.PieceKey{encounter.PieceHead}, level.keys, "a world written back unchanged sends the head")
	s.Empty(level.removed)
}

// TestAHostWithoutTheCapabilityAppends: a wrapper that forwards the appending
// methods alone hides SavePieces, and the session saves through SaveAppended
// with the whole rest, as before this leg.
func (s *PiecesCheckpointSuite) TestAHostWithoutTheCapabilityAppends() {
	mgr := s.managerOver(s.store.appendingStore, appendingOnly{fencedOnly{store: s.store.appendingStore}})
	_, err := mgr.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().NoError(err)
	_, err = mgr.Move(context.Background(), &session.MoveInput{
		Session: testSession, Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})
	s.Require().NoError(err)

	s.Empty(s.store.calls, "a hidden capability is never called")
	s.Len(s.store.appends, 2, "every save went through SaveAppended")
	s.Zero(s.store.wholeSaves)
}

// withoutEntryIDs is a stored world's JSON with its record entries' minted
// ids cleared (the wire id, the act correlation and its tag): two runs of the
// same verbs mint different ids (time-ordered), and nothing else between them
// may differ.
func withoutEntryIDs(t *testing.T, raw []byte) []byte {
	t.Helper()
	var data encounter.EncounterData
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	for i := range data.Log.Entries {
		entry := &data.Log.Entries[i]
		entry.ID, entry.Correlation = "", ""
		delete(entry.Tags, "act")
	}
	data.Log.LastID = ""
	out, err := json.Marshal(&data)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// worldRound is the world clock's high water in a stored world's JSON: the
// rounds the world has raised.
func worldRound(t *testing.T, raw []byte) int {
	t.Helper()
	var top struct {
		Clock struct {
			HighWater int `json:"high_water"`
		} `json:"clock"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	return top.Clock.HighWater
}

// changedPieces is the oracle: the keys whose JSON moved between two stored
// worlds, read off the worlds' own JSON trees rather than through
// SplitPieces, plus the head (written on every save). Removed keys are those
// prev has and cur does not.
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

// pieceTree reads a world's JSON into its pieces' content by key, from the
// top-level JSON names: a member per id, an observer per holdings key with
// its member's sweep, the journal, and the shared roster and knowledge
// fields. The head's content is not compared; it is written every save.
func pieceTree(t *testing.T, raw []byte) map[encounter.PieceKey][]byte {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
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
	if err := json.Unmarshal(top["members"], &members); err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		var id struct {
			ID encounter.MemberID `json:"id"`
		}
		if err := json.Unmarshal(m, &id); err != nil {
			t.Fatal(err)
		}
		out[encounter.MemberPiece(id.ID)] = m
	}
	var per struct {
		Intel struct {
			Holdings map[string]json.RawMessage `json:"holdings"`
		} `json:"intel"`
	}
	if raw, ok := top["perception"]; ok {
		if err := json.Unmarshal(raw, &per); err != nil {
			t.Fatal(err)
		}
	}
	observers := map[string][]byte{}
	for obs, h := range per.Intel.Holdings {
		observers[obs] = append([]byte("holdings="), h...)
	}
	var sweeps map[string]string
	if raw, ok := top["sight_sweeps"]; ok {
		if err := json.Unmarshal(raw, &sweeps); err != nil {
			t.Fatal(err)
		}
	}
	for id, sweep := range sweeps {
		obs := "member|" + id
		observers[obs] = append(observers[obs], []byte(";sweep="+sweep)...)
	}
	for obs, v := range observers {
		out[encounter.PieceKey("o:"+obs)] = v
	}
	return out
}
