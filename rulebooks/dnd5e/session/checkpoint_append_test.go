// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// appendingStore is an in-memory AppendingEncounterRepository over the
// fenced store: it keeps each encounter's rest and its record segments, and
// a read assembles them with the record's own join. The fenced store's
// version check and its writes of the session record, sheets and seats are
// reused as they are, so the two capabilities differ only in the record.
type appendingStore struct {
	*fencedStore
	rests    map[string]*encounter.EncounterData
	segments map[string][]record.LogData
	// floors is each encounter's retention floor as its last save named it.
	floors map[string]uint64

	// appends is every SaveAppended call, landed or not, in order.
	appends []appendCall
	// wholeSaves counts SaveFenced calls, which an appending host never gets
	// from the session.
	wholeSaves int
}

// appendCall is what one SaveAppended carried.
type appendCall struct {
	since, floor, expected uint64
	// seqs is the appended segment's entry seqs, in order.
	seqs []uint64
	// restEntries is how many entries the rest's own log carried (want none).
	restEntries int
	// restNext is the rest's log NextSeq as stored.
	restNext uint64
	landed   bool
}

func newAppendingStore() *appendingStore {
	return &appendingStore{
		fencedStore: newFencedStore(),
		rests:       map[string]*encounter.EncounterData{},
		segments:    map[string][]record.LogData{},
		floors:      map[string]uint64{},
	}
}

func (a *appendingStore) SaveFenced(ctx context.Context, in *session.SaveFencedInput) (*session.SaveFencedOutput, error) {
	a.wholeSaves++
	return a.fencedStore.SaveFenced(ctx, in)
}

func (a *appendingStore) SaveAppended(
	ctx context.Context, in *session.SaveAppendedInput,
) (*session.SaveFencedOutput, error) {
	call := appendCall{
		since: in.Since, floor: in.Floor, expected: in.Expected,
		restEntries: len(in.Data.Log.Entries), restNext: in.Data.Log.NextSeq,
	}
	for _, e := range in.Appended.Entries {
		call.seqs = append(call.seqs, e.Seq)
	}

	// The store's own shape: Since 1 starts the record over, an empty
	// segment is skipped, and a read joins the head with what is held.
	held := a.segments[in.Encounter]
	if in.Since == 1 {
		held = nil
	}
	segments := append([]record.LogData(nil), held...)
	if len(in.Appended.Entries) > 0 {
		segments = append(segments, in.Appended)
	}
	// The floor drops every segment wholly below it, as a host's store does.
	for len(segments) > 0 && segmentLast(segments[0]) < in.Floor {
		segments = segments[1:]
	}
	joined, err := joinSegments(in.Data.Log, in.Floor, segments)
	if err != nil {
		a.appends = append(a.appends, call)
		return nil, err
	}

	// The fenced store holds the assembled world, so every read (and every
	// helper over byID) sees what a host's join returns.
	whole := *in.Data
	whole.Log = joined
	fenced := in.SaveFencedInput
	fenced.Data = &whole
	out, err := a.fencedStore.SaveFenced(ctx, &fenced)
	if err != nil {
		a.appends = append(a.appends, call)
		return nil, err
	}
	call.landed = true
	a.appends = append(a.appends, call)
	a.rests[in.Encounter] = in.Data
	a.segments[in.Encounter] = segments
	a.floors[in.Encounter] = in.Floor
	return out, nil
}

// segmentLast is a non-empty segment's last seq.
func segmentLast(seg record.LogData) uint64 {
	return seg.Entries[len(seg.Entries)-1].Seq
}

// joinSegments is the record's join, in one place so the call shape lives
// here alone.
func joinSegments(head record.LogData, floor uint64, segments []record.LogData) (record.LogData, error) {
	return record.JoinData(&record.JoinDataInput{Head: head, Floor: floor, Segments: segments})
}

// lastAppend is the most recent SaveAppended call.
func (a *appendingStore) lastAppend() appendCall {
	return a.appends[len(a.appends)-1]
}

// fencedOnly hides the appending capability behind a wrapper that forwards
// only the fenced methods: the host that has not adopted it.
type fencedOnly struct{ store *appendingStore }

func (f fencedOnly) GetEncounter(ctx context.Context, id string) (*encounter.EncounterData, error) {
	return f.store.GetEncounter(ctx, id)
}

func (f fencedOnly) SaveEncounter(ctx context.Context, id string, data *encounter.EncounterData) error {
	return f.store.SaveEncounter(ctx, id, data)
}

func (f fencedOnly) GetFencedEncounter(
	ctx context.Context, in *session.GetFencedEncounterInput,
) (*session.GetFencedEncounterOutput, error) {
	return f.store.GetFencedEncounter(ctx, in)
}

func (f fencedOnly) SaveFenced(ctx context.Context, in *session.SaveFencedInput) (*session.SaveFencedOutput, error) {
	return f.store.SaveFenced(ctx, in)
}

type AppendedRecordSuite struct {
	suite.Suite
	store *appendingStore
}

func TestAppendedRecordSuite(t *testing.T) { suite.Run(t, new(AppendedRecordSuite)) }

func (s *AppendedRecordSuite) SetupTest() {
	s.store = newAppendingStore()
}

func (s *AppendedRecordSuite) managerOver(encounters session.EncounterRepository) *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: &fakeStream{}, Seats: s.store.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Driver(),
		Sessions: storeSessions{store: s.store.fencedStore}, Encounters: encounters, Characters: s.store.chars,
	})
	s.Require().NoError(err)
	return mgr
}

func (s *AppendedRecordSuite) launched() *session.Manager {
	mgr := s.managerOver(s.store)
	_, err := mgr.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().NoError(err)
	return mgr
}

func (s *AppendedRecordSuite) move(mgr *session.Manager, to spatial.Position) {
	_, err := mgr.Move(context.Background(), &session.MoveInput{
		Session: testSession, Member: "alice", Path: []spatial.Position{to},
	})
	s.Require().NoError(err)
}

// stored is the assembled world the store returns, as a host's read would.
func (s *AppendedRecordSuite) stored() *encounter.EncounterData {
	data, err := s.store.GetEncounter(context.Background(), testSession)
	s.Require().NoError(err)
	return data
}

// TestLaunchAppendsTheWholeRecordFromSeqOne: the first save of a run holds
// no record to append to, so it sends every beat Launch wrote, since 1, at
// version 0; the rest carries the head alone.
func (s *AppendedRecordSuite) TestLaunchAppendsTheWholeRecordFromSeqOne() {
	s.launched()

	s.Require().Len(s.store.appends, 1)
	launch := s.store.appends[0]
	s.True(launch.landed)
	s.Equal(uint64(1), launch.since, "launch appends since seq 1")
	s.Equal(uint64(0), launch.expected, "on a store that holds no checkpoint")
	s.Zero(launch.restEntries, "the rest carries the record's head, no entries")
	s.Require().NotEmpty(launch.seqs, "launch wrote beats")
	s.Equal(uint64(1), launch.seqs[0], "the segment is the whole record")
	s.Equal(storedNextSeq(s.stored())-1, launch.seqs[len(launch.seqs)-1])
	s.Zero(s.store.wholeSaves, "an appending host is never handed the whole world")
	s.Zero(s.store.plainSaves)
}

// TestAMoveAppendsOnlyWhatTheStoreDoesNotHold is the leg's headline: a Move
// after Launch sends the entries from the stored record's NextSeq onward and
// nothing older, and the next Move's Since is the first save's NextSeq.
func (s *AppendedRecordSuite) TestAMoveAppendsOnlyWhatTheStoreDoesNotHold() {
	mgr := s.launched()
	afterLaunch := s.store.lastAppend().restNext

	s.move(mgr, spatial.Position{X: 2, Y: 1})
	first := s.store.lastAppend()
	s.Require().True(first.landed)
	s.Equal(afterLaunch, first.since, "the Move sends from what Launch's save left")
	s.Require().NotEmpty(first.seqs, "a walk writes beats")
	s.Equal(first.since, first.seqs[0], "the segment starts at Since")
	for _, seq := range first.seqs {
		s.GreaterOrEqual(seq, first.since, "no entry the store already holds is sent again")
	}
	s.Zero(first.restEntries)

	s.move(mgr, spatial.Position{X: 3, Y: 1})
	second := s.store.lastAppend()
	s.Require().True(second.landed)
	s.Equal(first.restNext, second.since, "the second Move's Since is the first save's NextSeq")
	s.Equal(second.since, second.seqs[0])

	// The assembled world is the whole record: every seq from 1, unbroken.
	whole := s.stored()
	s.Require().NotEmpty(whole.Log.Entries)
	s.Equal(uint64(1), whole.Log.Entries[0].Seq)
	s.Len(whole.Log.Entries, int(storedNextSeq(whole)-1), "the joined record holds every beat")
	s.Len(s.store.segments[testSession], 3, "one segment per landed save that wrote beats")
	s.Zero(s.store.wholeSaves)
}

// TestABeatlessVerbAppendsNothingAndStillMovesTheVersion: declaring the
// sneak mode already held commits a world that wrote no beat. Its segment is
// empty and its Since stands where the last save left it, while the version
// moves, as on the whole-record path.
func (s *AppendedRecordSuite) TestABeatlessVerbAppendsNothingAndStillMovesTheVersion() {
	mgr := s.launched()
	since := s.store.lastAppend().restNext
	versionBefore := s.store.versions[testSession]
	segmentsBefore := len(s.store.segments[testSession])

	_, err := mgr.SetSneaking(context.Background(), &session.SetSneakingInput{
		Session: testSession, Member: "alice", Sneaking: false,
	})
	s.Require().NoError(err)

	beatless := s.store.lastAppend()
	s.True(beatless.landed)
	s.Equal(since, beatless.since)
	s.Empty(beatless.seqs, "the verb appended no beat, so it sends none")
	s.Equal(since, beatless.restNext, "the record did not move")
	s.Equal(versionBefore+1, s.store.versions[testSession], "but the checkpoint did")
	s.Len(s.store.segments[testSession], segmentsBefore, "the store holds no new segment")
}

// TestAStaleAppendLandsNothing: a writer that saves between this verb's load
// and its save leaves the appending save refused like a fenced one, and the
// store keeps the other writer's rest and segments.
func (s *AppendedRecordSuite) TestAStaleAppendLandsNothing() {
	mgr := s.launched()
	s.store.onGet = func() { s.store.versions[testSession]++ }
	world := s.store.snapshot(testSession)
	segments := len(s.store.segments[testSession])

	_, err := mgr.Move(context.Background(), &session.MoveInput{
		Session: testSession, Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}},
	})

	s.Require().ErrorIs(err, session.ErrStaleCheckpoint)
	s.False(s.store.lastAppend().landed)
	s.Equal(world, s.store.snapshot(testSession), "the store keeps the world it held")
	s.Len(s.store.segments[testSession], segments, "and appended nothing")
}

// TestASeatedLevelAppendsNothing: LevelUp writes the run's world back as it
// read it, so under the appending capability it sends the head as the rest
// and an empty segment from the stored NextSeq, and the record stays whole.
func (s *AppendedRecordSuite) TestASeatedLevelAppendsNothing() {
	s.store.chars.byID["alice"] = advancingFighter("alice")
	s.launched()
	mgr := s.managerOver(s.store)
	before := s.stored()

	out, err := mgr.LevelUp(context.Background(), &session.LevelUpInput{
		Character: "alice", HitPointMethod: session.HitPointsAverage,
	})
	s.Require().NoError(err)

	s.Contains(out.Saved.Written, "character:alice")
	level := s.store.lastAppend()
	s.True(level.landed)
	s.Equal(storedNextSeq(before), level.since)
	s.Empty(level.seqs, "a level writes no beat")
	s.Zero(level.restEntries)
	s.Equal(before.Log.Entries, s.stored().Log.Entries, "the record is what it was")
	s.Zero(s.store.wholeSaves)
}

// TestAHostWithoutTheCapabilityGetsTheWholeWorld: a wrapper that forwards
// only the fenced methods hides SaveAppended, and the session falls back to
// SaveFenced with the whole record on every save, as before this leg.
func (s *AppendedRecordSuite) TestAHostWithoutTheCapabilityGetsTheWholeWorld() {
	var saved []*session.SaveFencedInput
	s.store.onSave = func(in *session.SaveFencedInput) { saved = append(saved, in) }
	mgr := s.managerOver(fencedOnly{store: s.store})
	_, err := mgr.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().NoError(err)
	s.move(mgr, spatial.Position{X: 2, Y: 1})

	s.Empty(s.store.appends, "a hidden capability is never called")
	s.Equal(2, s.store.wholeSaves)
	s.Require().Len(saved, 2)
	last := saved[1].Data.Log
	s.Require().NotEmpty(last.Entries)
	s.Equal(uint64(1), last.Entries[0].Seq, "the fenced save carries the whole record")
	s.Len(last.Entries, int(last.NextSeq-1))
}

// TestABoundedRecordLoadsWhatItsWindowKeeps pins the floor end to end: a run
// whose record keeps a window of five beats saves through the appending
// capability, the store drops every segment wholly below the floor the save
// names, and the joined record is exactly the window: the entries from the
// floor through the last beat, which is what ToData keeps. A save that named
// floor 0 would leave the store every old segment, and the join would start
// at seq 1.
func (s *AppendedRecordSuite) TestABoundedRecordLoadsWhatItsWindowKeeps() {
	mgr := s.launched()
	const window = 5
	// The launched run keeps everything; give the stored world a window, the
	// way a run built with a bounded Retention would carry it.
	s.store.byID[testSession].Retention = window
	s.store.rests[testSession].Retention = window

	for i, to := range []spatial.Position{{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 2, Y: 1}, {X: 3, Y: 1}} {
		s.move(mgr, to)
		save := s.store.lastAppend()
		s.Require().True(save.landed, "move %d", i)
		s.Require().Positive(save.floor, "move %d: a bounded record names its floor", i)

		whole := s.stored().Log
		next := storedNextSeq(s.stored())
		s.Require().NotEmpty(whole.Entries, "move %d", i)
		s.Equal(save.floor, whole.Entries[0].Seq, "move %d: the record starts at the floor", i)
		s.Equal(next-1, whole.Entries[len(whole.Entries)-1].Seq, "move %d", i)
		s.Len(whole.Entries, int(next-save.floor), "move %d: every beat of the window, gapless", i)
		s.LessOrEqual(len(whole.Entries), window, "move %d: no more than the window", i)
	}
	s.Equal(s.store.floors[testSession], s.store.lastAppend().floor)
	for _, seg := range s.store.segments[testSession] {
		s.GreaterOrEqual(segmentLast(seg), s.store.floors[testSession], "no stored segment lies wholly below the floor")
	}
}
