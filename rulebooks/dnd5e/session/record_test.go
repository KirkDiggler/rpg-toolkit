// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// record_test.go pins the record stream's SDK half (rpg-project#628,
// ideas/event-stream R5, R12, R13, R15, R27, Law 5): every beat a commit
// appended is published as written, in id order, under its v2 act id, and
// the same beats are readable after any id.

import (
	"context"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// fakeRecord is a RecordStream that keeps every batch it is handed.
type fakeRecord struct {
	batches [][]session.RecordEvent
}

func (f *fakeRecord) PublishRecord(_ context.Context, events []session.RecordEvent) error {
	f.batches = append(f.batches, append([]session.RecordEvent(nil), events...))
	return nil
}

func (f *fakeRecord) last() []session.RecordEvent {
	if len(f.batches) == 0 {
		return nil
	}
	return f.batches[len(f.batches)-1]
}

// v2Act is the v1alpha2 act form: a verb, a colon, a 26-character id.
var v2Act = regexp.MustCompile(`^[a-z_]+:[0-9A-HJKMNP-TV-Z]{26}$`)

func recordOf(beats []session.RecordEvent, beat string) []session.RecordEvent {
	var out []session.RecordEvent
	for _, e := range beats {
		if e.Beat == beat {
			out = append(out, e)
		}
	}
	return out
}

func recordIDs(events []session.RecordEvent) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.ID
	}
	return out
}

func readRecord(mgr *session.Manager, after string) (*session.RecordReadOutput, error) {
	return mgr.Record(context.Background(), &session.RecordReadInput{Session: "sess", AfterID: after})
}

// TestRecordEventCarriesV2ActID: a three-cell walk publishes three moved
// beats under one v2 act id, `move:<id>`, while the v1 events keep E1's
// `move:<first seq>` — both forms, one act.
func (s *MoveTestSuite) TestRecordEventCarriesV2ActID() {
	s.startCorridor()

	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 3, Y: 2}},
	})
	s.Require().NoError(err)

	moved := recordOf(s.record.last(), encounter.BeatMoved)
	s.Require().Len(moved, 3)
	s.Equal("sess", moved[0].Session, "a record event names its session, which a host routes by")
	act := moved[0].Act
	s.Regexp(v2Act, act)
	s.True(strings.HasPrefix(act, "move:"))
	for _, m := range moved {
		s.Equal(act, m.Act)
	}

	told := beatsNamed(storyBeats(s.T(), s.mgr, "sess", "alice"), encounter.BeatMoved)
	s.Require().Len(told, 3)
	for _, b := range told {
		s.Equal(act, b.Correlation, "alice's story carries the same one act id")
	}
}

// TestOutsideActHasNoAct: the scene opening is nobody's act.
func (s *MoveTestSuite) TestOutsideActHasNoAct() {
	s.startCorridor()

	out, err := readRecord(s.mgr, "")
	s.Require().NoError(err)
	opened := recordOf(out.Events, encounter.BeatSceneOpened)
	s.Require().Len(opened, 1)
	s.Empty(opened[0].Act)
}

// TestRecordReadAfterID: the read after an id is every later beat, in id
// order, byte-equal to what was published live.
func (s *MoveTestSuite) TestRecordReadAfterID() {
	s.startCorridor()
	before, err := readRecord(s.mgr, "")
	s.Require().NoError(err)
	s.Require().NotEmpty(before.Events)
	mark := before.Events[len(before.Events)-1].ID

	_, err = s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}, {X: 3, Y: 1}},
	})
	s.Require().NoError(err)

	after, err := readRecord(s.mgr, mark)
	s.Require().NoError(err)
	s.False(after.Restarted)
	s.Equal(s.record.last(), after.Events, "catch-up is the live batch")
	for _, e := range after.Events {
		s.Greater(e.ID, mark)
	}

	s.False(after.More, "an uncapped read has nothing more")
	limited, err := s.mgr.Record(context.Background(), &session.RecordReadInput{Session: "sess", AfterID: mark, Limit: 1})
	s.Require().NoError(err)
	s.Equal(after.Events[:1], limited.Events)
	s.True(limited.More, "a page cut short says more remain")
	exact, err := s.mgr.Record(context.Background(), &session.RecordReadInput{Session: "sess", AfterID: mark, Limit: len(after.Events)})
	s.Require().NoError(err)
	s.Len(exact.Events, len(after.Events))
	s.False(exact.More, "a page that holds the rest says no more")
}

// TestRecordReadRestarted: an id older than what the stored record retains
// restarts the read from the oldest retained beat.
func (s *MoveTestSuite) TestRecordReadRestarted() {
	s.startCorridor()
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}, {X: 3, Y: 1}},
	})
	s.Require().NoError(err)
	whole, err := readRecord(s.mgr, "")
	s.Require().NoError(err)
	s.Require().Greater(len(whole.Events), 3)

	// Age the oldest two beats out of the stored record, as retention does.
	s.Require().Len(s.encounters.byID, 1)
	for _, data := range s.encounters.byID {
		data.Log.Entries = data.Log.Entries[2:]
	}

	out, err := readRecord(s.mgr, whole.Events[0].ID)
	s.Require().NoError(err)
	s.True(out.Restarted)
	s.Equal(recordIDs(whole.Events[2:]), recordIDs(out.Events))
}

// TestCommitPublishesRecordInOrder: a lethal swing's commit publishes its
// beats in ascending id order, struck and downed under one attack act, and
// exactly the beats the commit appended.
func (s *AttackTestSuite) TestCommitPublishesRecordInOrder() {
	mgr := s.duel(&sequenceDice{rolls: []int{15, 5}})
	s.characters.byID["bob"].HitPoints = 1
	before, err := readRecord(mgr, "")
	s.Require().NoError(err)
	mark := before.Events[len(before.Events)-1].ID
	published := len(s.record.batches)

	_, err = s.swing(mgr)
	s.Require().NoError(err)

	s.Require().Len(s.record.batches, published+1, "one batch per commit")
	batch := s.record.last()
	s.True(sort.StringsAreSorted(recordIDs(batch)))
	after, err := readRecord(mgr, mark)
	s.Require().NoError(err)
	s.Equal(recordIDs(after.Events), recordIDs(batch), "exactly this commit's beats")

	struck := recordOf(batch, string(encounter.OutcomeStruck))
	downed := recordOf(batch, string(encounter.OutcomeDown))
	s.Require().Len(struck, 1)
	s.Require().Len(downed, 1)
	s.Regexp(v2Act, struck[0].Act)
	s.True(strings.HasPrefix(struck[0].Act, "attack:"))
	s.Equal(struck[0].Act, downed[0].Act)
}

// TestBeatIsVerbatim: the record names a beat by the encounter's own word,
// not the SDK's kind.
func (s *SpeakWorldSuite) TestBeatIsVerbatim() {
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		line := appealLine()
		line.Success = []encounter.Answer{{Weight: 1, Say: "Fine."}}
		in.Options = []encounter.Option{line}
	}
	mgr := s.front([]int{10, 1, 1})
	_, err := s.appeal(mgr)
	s.Require().NoError(err)

	out, err := mgr.Record(context.Background(), &session.RecordReadInput{Session: "sess"})
	s.Require().NoError(err)
	answered := recordOf(out.Events, "answered")
	s.Require().NotEmpty(answered)
	s.Contains(string(answered[0].Fields), `"beat":"answered"`)
	s.Regexp(v2Act, answered[0].Act)
	s.True(strings.HasPrefix(answered[0].Act, "speak:"))
}

// TestNilRecordStreamRefused: a manager without a record stream is refused
// at construction, naming the field.
func (s *ManagerTestSuite) TestNilRecordStreamRefused() {
	_, err := session.NewManager(&session.Config{
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: testCharacters(),
		Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(), Acts: session.DiscardActs{},
		Dice: testDice{}, PresentationIDs: testPresentationIDs{}, TurnDriver: session.Pass{},
	})
	s.ErrorIs(err, session.ErrIncompleteConfig)
	s.Contains(err.Error(), "Record")
}

// TestRecordEventCarriesNoSeq: the record's dense global seq never rides a
// RecordEvent — the wire id is its only order (R27, R30). The fields are
// exactly these; a new one must be argued for here.
func (s *MoveTestSuite) TestRecordEventCarriesNoSeq() {
	typ := reflect.TypeOf(session.RecordEvent{})
	var names []string
	for i := 0; i < typ.NumField(); i++ {
		names = append(names, typ.Field(i).Name)
		s.NotContains(strings.ToLower(typ.Field(i).Name), "seq")
	}
	s.Equal([]string{"Session", "ID", "At", "Act", "Beat", "Fields"}, names)
}

// TestRecordRefusesABadAfterID: an AfterID that is not a record id is the
// caller's error, ErrBadRecordID — never an empty "caught up" read, and not a
// broken world.
func (s *MoveTestSuite) TestRecordRefusesABadAfterID() {
	s.startCorridor()
	held, err := readRecord(s.mgr, "")
	s.Require().NoError(err)
	for _, after := range []string{"zzz", strings.ToLower(held.Events[0].ID)} {
		_, err := readRecord(s.mgr, after)
		s.ErrorIs(err, session.ErrBadRecordID, "after %q", after)
		s.NotErrorIs(err, session.ErrInvalidWorld)
	}
}

// TestRecordNilInput is the package's nil-input law.
func (s *MoveTestSuite) TestRecordNilInput() {
	_, err := s.mgr.Record(context.Background(), nil)
	s.ErrorIs(err, session.ErrNilInput)
}

// TestWindowCarriesTheWalksV2Act is Law 2 on the record: a skeleton's walk is
// held by the fighter's window, saved, and answered; the opportunity attack
// and every cell of the walk carry the walk's v2 act, which the posed window
// kept beside its v1 id.
func (s *ReactWindowSuite) TestWindowCarriesTheWalksV2Act() {
	mgr := s.managerWith(&pathWalker{
		paths:  map[string][]spatial.Position{"skel-1": {hexCell(5, 0), hexCell(6, 0)}},
		walked: map[string]bool{},
	})
	s.start(mgr, []sceneSeat{seatAt("fighter", 3, 0)}, reactSkeleton("skel-1", 4, 0))

	s.endTurn(mgr, "fighter")
	s.react(mgr, "fighter", session.Take(""))

	out, err := readRecord(mgr, "")
	s.Require().NoError(err)
	opened := recordOf(out.Events, encounter.BeatWindowOpened)
	s.Require().Len(opened, 1)
	walk := opened[0].Act
	s.Regexp(v2Act, walk)
	s.True(strings.HasPrefix(walk, "move:"))

	var swings []session.RecordEvent
	swings = append(swings, recordOf(out.Events, string(encounter.OutcomeStruck))...)
	swings = append(swings, recordOf(out.Events, string(encounter.OutcomeMissed))...)
	s.Require().Len(swings, 1)
	s.Equal(walk, swings[0].Act, "the reaction carries its trigger's v2 act")

	var cells int
	for _, m := range recordOf(out.Events, encounter.BeatMoved) {
		if strings.Contains(string(m.Fields), `"member":"skel-1"`) {
			cells++
			s.Equal(walk, m.Act)
		}
	}
	s.Equal(2, cells)
}
