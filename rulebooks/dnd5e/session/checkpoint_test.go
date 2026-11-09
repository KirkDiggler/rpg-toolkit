// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// fencedStore is an in-memory FencedEncounterRepository over the ordinary
// fakes: a version per encounter, incremented on every save that lands, and
// a save that names any other version refused with ErrStaleCheckpoint. A
// landed save writes every piece it carries straight into the fakes' maps,
// so the fakes' own save counters count only plain writes, which a fenced
// host must never get.
type fencedStore struct {
	*fakeEncounters
	sessions *fakeSessions
	chars    *fakeCharacters
	seats    *fakeSeats
	versions map[string]uint64

	// expected is the Expected of every SaveFenced call, landed or not.
	expected []uint64

	// plainSaves counts SaveEncounter calls, which a fenced host never gets.
	plainSaves int
	// fencedSaves counts saves that landed; refusals counts stale ones.
	fencedSaves int
	refusals    int
	// sessionWrites counts session records written by a landed save.
	sessionWrites int

	// onGet, when set, runs once after a fenced read has taken its version:
	// another writer acting between this verb's load and its save.
	onGet func()
	// onGetSession, when set, runs once after the next GetSession has read.
	onGetSession func()
}

func newFencedStore() *fencedStore {
	return &fencedStore{
		fakeEncounters: newFakeEncounters(),
		sessions:       newFakeSessions(),
		chars:          testCharacters(),
		seats:          newFakeSeats(),
		versions:       map[string]uint64{},
	}
}

func (f *fencedStore) GetFencedEncounter(
	ctx context.Context, in *session.GetFencedEncounterInput,
) (*session.GetFencedEncounterOutput, error) {
	data, err := f.GetEncounter(ctx, in.Encounter)
	if err != nil {
		return nil, err
	}
	out := &session.GetFencedEncounterOutput{Data: data, Version: f.versions[in.Encounter]}
	if hook := f.onGet; hook != nil {
		f.onGet = nil
		hook()
	}
	return out, nil
}

func (f *fencedStore) SaveFenced(_ context.Context, in *session.SaveFencedInput) (*session.SaveFencedOutput, error) {
	f.expected = append(f.expected, in.Expected)
	if held := f.versions[in.Encounter]; held != in.Expected {
		f.refusals++
		return nil, fmt.Errorf("encounter %q at %d, save expected %d: %w",
			in.Encounter, held, in.Expected, session.ErrStaleCheckpoint)
	}
	f.byID[in.Encounter] = in.Data
	if in.Session != nil {
		f.sessions.byID[in.Session.ID] = in.Session
		f.sessionWrites++
	}
	for _, sheet := range in.Characters {
		f.chars.byID[sheet.ID] = sheet
	}
	f.seats.mu.Lock()
	for _, seat := range in.Seats {
		f.seats.byID[seat.Character] = *seat
	}
	f.seats.mu.Unlock()
	f.versions[in.Encounter]++
	f.fencedSaves++
	return &session.SaveFencedOutput{Version: f.versions[in.Encounter]}, nil
}

func (f *fencedStore) SaveEncounter(ctx context.Context, id string, data *encounter.EncounterData) error {
	f.plainSaves++
	return f.fakeEncounters.SaveEncounter(ctx, id, data)
}

// storeSessions is the session half of the fenced store, so a read through
// it sees what SaveFenced wrote and a plain SaveSession is counted apart.
type storeSessions struct{ store *fencedStore }

func (s storeSessions) GetSession(ctx context.Context, id string) (*session.SessionData, error) {
	data, err := s.store.sessions.GetSession(ctx, id)
	if hook := s.store.onGetSession; hook != nil {
		s.store.onGetSession = nil
		hook()
	}
	return data, err
}

func (s storeSessions) SaveSession(ctx context.Context, data *session.SessionData) error {
	return s.store.sessions.SaveSession(ctx, data)
}

// snapshot is the stored encounter as bytes, for comparing what the store
// holds before and after a refused save.
func (f *fencedStore) snapshot(id string) []byte {
	raw, err := json.Marshal(f.byID[id])
	if err != nil {
		panic(err)
	}
	return raw
}

type CheckpointFenceSuite struct {
	suite.Suite
	store *fencedStore
}

func TestCheckpointFenceSuite(t *testing.T) { suite.Run(t, new(CheckpointFenceSuite)) }

func (s *CheckpointFenceSuite) SetupTest() {
	s.store = newFencedStore()
}

// manager is one process over the shared store, with its own act stream.
func (s *CheckpointFenceSuite) manager(acts *fakeStream, runner session.ContinuationRunner) *session.Manager {
	return s.managerWith(acts, runner, testDice{})
}

func (s *CheckpointFenceSuite) managerWith(
	acts *fakeStream, runner session.ContinuationRunner, dice session.Roller,
) *session.Manager {
	cfg := &session.Config{
		Record: session.DiscardRecord{}, Acts: acts, Seats: s.store.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: dice, TurnDriver: session.Driver(),
		Sessions: storeSessions{store: s.store}, Encounters: s.store, Characters: s.store.chars,
	}
	if runner != nil {
		cfg.Continuations = runner
	}
	mgr, err := session.NewManager(cfg)
	s.Require().NoError(err)
	return mgr
}

func (s *CheckpointFenceSuite) moveAlice(mgr *session.Manager, to spatial.Position) (*session.MoveOutput, error) {
	return mgr.Move(context.Background(), &session.MoveInput{
		Session: testSession, Member: "alice", Path: []spatial.Position{to},
	})
}

// TestTheSecondWriterAtOneVersionFailsClosed is the fence's headline: two
// processes load the run at the same checkpoint version and both Move. The
// first to save lands; the second is refused with ErrStaleCheckpoint, the
// store keeps the first's world, and nothing of the loser's reaches its
// subscribers.
func (s *CheckpointFenceSuite) TestTheSecondWriterAtOneVersionFailsClosed() {
	loserActs, winnerActs := &fakeStream{}, &fakeStream{}
	loser, winner := s.manager(loserActs, nil), s.manager(winnerActs, nil)
	_, err := winner.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().NoError(err)
	winnerActs.published = nil
	s.Require().Equal(uint64(1), s.store.versions[testSession], "launch landed the first checkpoint")

	var winnerWorld []byte
	sessionWritesAfterWinner := 0
	s.store.onGet = func() {
		// The loser has loaded at version 1. The winner loads at 1 too,
		// saves, and moves the store to 2.
		_, err := s.moveAlice(winner, spatial.Position{X: 2, Y: 1})
		s.Require().NoError(err)
		winnerWorld = s.store.snapshot(testSession)
		sessionWritesAfterWinner = s.store.sessionWrites
	}

	_, err = s.moveAlice(loser, spatial.Position{X: 1, Y: 2})

	s.Require().ErrorIs(err, session.ErrStaleCheckpoint)
	s.ErrorIs(err, session.ErrSaveFailed, "a stale save is a failed save")
	var saveErr *session.SaveError
	s.Require().ErrorAs(err, &saveErr)
	s.Contains(saveErr.Report.Failed, "encounter:"+testSession)
	s.NotContains(saveErr.Report.Written, "encounter:"+testSession)

	s.Equal(1, s.store.refusals)
	s.Equal(uint64(2), s.store.versions[testSession], "only the winner's save advanced the version")
	s.Equal(winnerWorld, s.store.snapshot(testSession), "the store holds the winner's world")
	s.Equal(sessionWritesAfterWinner, s.store.sessionWrites,
		"pins the double: a refused save writes no session record")
	s.NotEmpty(winnerActs.published, "the winner's walk was delivered")
	s.Empty(loserActs.published, "no act of the loser's reached a subscriber")
	s.Zero(s.store.plainSaves, "a fenced host is never written around the fence")
}

// TestARacingLaunchWritesNoSessionRecord pins the session half: Launch
// touches the session record, and a fenced save carries it in the same
// transaction as the world. Two Launches of one run race; the one that loses
// expected no checkpoint, finds one, and writes neither blob.
func (s *CheckpointFenceSuite) TestARacingLaunchWritesNoSessionRecord() {
	loserActs, winnerActs := &fakeStream{}, &fakeStream{}
	loser, winner := s.manager(loserActs, nil), s.manager(winnerActs, nil)
	s.store.onGetSession = func() {
		// The loser has found no session; the winner launches the same run.
		_, err := winner.Launch(context.Background(), sceneInput(authoredWorld()))
		s.Require().NoError(err)
	}

	_, err := loser.Launch(context.Background(), sceneInput(authoredWorld()))

	s.Require().ErrorIs(err, session.ErrStaleCheckpoint)
	var saveErr *session.SaveError
	s.Require().ErrorAs(err, &saveErr)
	s.Contains(saveErr.Report.Failed, "session:"+testSession, "the session record failed with the world")
	s.Equal(1, s.store.sessionWrites, "only the winner's session record landed")
	s.Equal(uint64(1), s.store.versions[testSession])
	s.Zero(s.store.sessions.saves, "a fenced host's session record never goes through SaveSession")
	s.Zero(s.store.seats.saves, "nor its seats through SaveSeat")
	s.Zero(s.store.chars.saves, "nor its sheets through SaveCharacter")
	s.NotEmpty(winnerActs.published)
	s.Empty(loserActs.published, "the losing launch delivered nothing")
}

// TestABeatlessVerbStillAdvancesTheVersion is why the record's NextSeq is
// not the fence: declaring a sneak mode already held commits the world and
// appends no beat, so NextSeq stands still while the version moves.
func (s *CheckpointFenceSuite) TestABeatlessVerbStillAdvancesTheVersion() {
	mgr := s.manager(&fakeStream{}, nil)
	_, err := mgr.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().NoError(err)
	seqBefore := storedNextSeq(s.store.byID[testSession])
	versionBefore := s.store.versions[testSession]

	_, err = mgr.SetSneaking(context.Background(), &session.SetSneakingInput{
		Session: testSession, Member: "alice", Sneaking: false,
	})
	s.Require().NoError(err)

	s.Equal(seqBefore, storedNextSeq(s.store.byID[testSession]), "the verb appended no beat")
	s.Equal(versionBefore+1, s.store.versions[testSession], "but its save moved the checkpoint")
}

// TestAStaleContinuationPersistsNothing is the continuation's half: a
// continued Move saves only in its rest, on the version its answer loaded.
// A writer that lands in between (a lease that expired while the rest
// waited) makes the rest fail closed: nothing saved, nothing delivered.
func (s *CheckpointFenceSuite) TestAStaleContinuationPersistsNothing() {
	runner := &capturingRunner{}
	acts := &fakeStream{}
	mgr := s.manager(acts, runner)

	goblin := monsterAt("goblin", refs.Monsters.Goblin().String(), 11, 3)
	goblin.Faction = string(continuationCamp)
	goblin.Behaviors = ordinaryBehavior(encounter.Table{encounter.AnswerTime: {{Weight: 1, Hold: true}}})
	launchScene(s.T(), mgr, scene{
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 13, 4)},
			Factions: []encounter.FactionInput{{ID: continuationCamp}},
			Dispositions: []encounter.DispositionInput{
				{Between: [2]encounter.FactionID{continuationCamp, encounter.FactionParty}, Stance: encounter.StanceNeutral},
			},
		},
		Party:    []sceneSeat{seatAt("alice", 1, 1)},
		Monsters: []dungeonspec.MonsterPlacement{goblin},
		Endings:  []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	})
	acts.published = nil

	answer, err := mgr.Move(context.Background(), &session.MoveInput{
		Session: testSession, Member: "alice", Path: longWalk(),
	})
	s.Require().NoError(err)
	s.Require().True(answer.Continues, "the walk pays a round part way, so its rest is continued")
	s.Require().NotEmpty(runner.handed)

	// Another process saves while the rest waits.
	s.store.versions[testSession]++
	world := s.store.snapshot(testSession)
	landed := s.store.fencedSaves

	_, err = runner.handed[0].Continue(context.Background())

	s.Require().ErrorIs(err, session.ErrStaleCheckpoint)
	s.Equal(landed, s.store.fencedSaves, "the stale rest landed no save")
	s.Equal(world, s.store.snapshot(testSession), "the store keeps the other writer's world")
	s.Empty(acts.published, "a stale continuation delivers nothing")
}

// TestAFencedReadKeepsTheContract holds the capability to the repository
// contract fetchWorld already holds: success with no data is a broken
// repository, never an empty world.
func (s *CheckpointFenceSuite) TestAFencedReadKeepsTheContract() {
	mgr := s.manager(&fakeStream{}, nil)
	_, err := mgr.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().NoError(err)
	mgr, err = session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: &fakeStream{}, Seats: s.store.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Driver(),
		Sessions: storeSessions{store: s.store}, Encounters: emptyReadStore{s.store}, Characters: s.store.chars,
	})
	s.Require().NoError(err)

	_, err = s.moveAlice(mgr, spatial.Position{X: 2, Y: 1})

	s.ErrorIs(err, session.ErrBadRepository)
}

// emptyReadStore is a fenced store whose read reports success with no data.
type emptyReadStore struct{ *fencedStore }

func (e emptyReadStore) GetFencedEncounter(
	context.Context, *session.GetFencedEncounterInput,
) (*session.GetFencedEncounterOutput, error) {
	return &session.GetFencedEncounterOutput{Version: 1}, nil
}

// noAdvanceStore is a fenced store that reports success without moving its
// version: a contract violation the session refuses rather than trusts.
type noAdvanceStore struct{ *fencedStore }

func (n noAdvanceStore) SaveFenced(_ context.Context, in *session.SaveFencedInput) (*session.SaveFencedOutput, error) {
	return &session.SaveFencedOutput{Version: in.Expected}, nil
}

func (s *CheckpointFenceSuite) TestASaveThatDoesNotAdvanceTheVersionIsABrokenRepository() {
	mgr, err := session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: &fakeStream{}, Seats: s.store.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Driver(),
		Sessions: storeSessions{store: s.store}, Encounters: noAdvanceStore{s.store}, Characters: s.store.chars,
	})
	s.Require().NoError(err)

	_, err = mgr.Launch(context.Background(), sceneInput(authoredWorld()))

	s.ErrorIs(err, session.ErrBadRepository)
	s.False(errors.Is(err, session.ErrStaleCheckpoint), "a broken repository is not a stale writer")
}

// fight is aFight's scene over the fenced store: an armed alice beside a
// skeleton, the launch forms the fight, and alice's turn is first.
func (s *CheckpointFenceSuite) fight(rolls ...int) *session.Manager {
	s.store.chars.byID["alice"] = withFreeReactions(armedFighter("alice"))
	const initiativeRolls = 2
	roller := &sequenceDice{rolls: append(make([]int, initiativeRolls), rolls...)}
	mgr, err := session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: s.store.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: roller, TurnDriver: session.Pass{},
		Sessions: storeSessions{store: s.store}, Encounters: s.store, Characters: s.store.chars,
	})
	s.Require().NoError(err)
	launched, err := mgr.Launch(context.Background(), sceneInput(scene{
		Field:    encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Party:    []sceneSeat{seatAt("alice", 1, 1)},
		Monsters: []dungeonspec.MonsterPlacement{monsterAt("skeleton", refs.Monsters.Skeleton().String(), 2, 1)},
	}))
	s.Require().NoError(err)
	s.Require().NotEmpty(launched.Formed, "the launch forms the fight")
	roller.next = initiativeRolls
	return mgr
}

func (s *CheckpointFenceSuite) sheetBytes(id string) []byte {
	raw, err := json.Marshal(s.store.chars.byID[id])
	s.Require().NoError(err)
	return raw
}

func (s *CheckpointFenceSuite) walkAway(mgr *session.Manager) (*session.MoveOutput, error) {
	return mgr.Move(context.Background(), &session.MoveInput{
		Session: testSession, Member: "alice", Path: pathAwayFromSkeleton(),
		DeclarationID: currentMoveID(s.T(), mgr, testSession, "alice"),
	})
}

// TestAStaleFightMoveWritesNoSheet is the review's probe kept: a Move in a
// fight spends the walker's movement, a sheet write. Under the fence that
// sheet rides the one fenced save, so a writer whose version moved lands
// neither the world nor the sheet.
func (s *CheckpointFenceSuite) TestAStaleFightMoveWritesNoSheet() {
	mgr := s.fight(1, 1)
	sheetBefore := s.sheetBytes("alice")
	s.store.onGet = func() { s.store.versions[testSession]++ }

	_, err := s.walkAway(mgr)

	s.Require().ErrorIs(err, session.ErrStaleCheckpoint)
	var saveErr *session.SaveError
	s.Require().ErrorAs(err, &saveErr)
	s.NotContains(saveErr.Report.Written, "character:alice", "the walker's sheet did not land")
	s.Contains(saveErr.Report.Failed, "character:alice", "it failed with the world")
	s.Equal(sheetBefore, s.sheetBytes("alice"), "the store keeps the sheet the stale writer loaded")
	s.Zero(s.store.chars.saves, "no sheet went around the fence")

	// The refused walk's spent movement died with its guard: the same walk
	// again starts from the stored sheet, which still has the movement.
	_, err = s.walkAway(mgr)
	s.Require().NoError(err, "nothing the refused verb wrote is visible to the next")
}

// TestAFencedFightMoveLandsWhatAPlainOneDoes is the other half: the same
// Move, not stale, lands the walker's sheet in the fenced save, and the
// sheet and its report are what the plain path writes for the same scene.
// The sheets a verb defers are read back by that same verb from the
// overlay, so any difference here would be a read that missed its write.
func (s *CheckpointFenceSuite) TestAFencedFightMoveLandsWhatAPlainOneDoes() {
	mgr := s.fight(1, 1)
	sheetBefore := s.sheetBytes("alice")
	out, err := s.walkAway(mgr)
	s.Require().NoError(err)

	s.Contains(out.Saved.Written, "character:alice")
	s.NotEqual(sheetBefore, s.sheetBytes("alice"), "the walk spent movement on the sheet")
	s.Zero(s.store.chars.saves, "the sheet landed in the fenced save, not through SaveCharacter")

	plain, _, _, plainChars := aFight(s.T(), armedFighter("alice"), []int{1, 1})
	plainOut, err := plain.Move(context.Background(), &session.MoveInput{
		Session: testSession, Member: "alice", Path: pathAwayFromSkeleton(),
		DeclarationID: currentMoveID(s.T(), plain, testSession, "alice"),
	})
	s.Require().NoError(err)
	// The stamp is the clock's, not the walk's.
	plainAlice, fencedAlice := *plainChars.byID["alice"], *s.store.chars.byID["alice"]
	plainAlice.UpdatedAt, fencedAlice.UpdatedAt = time.Time{}, time.Time{}
	plainRaw, err := json.Marshal(plainAlice)
	s.Require().NoError(err)
	fencedRaw, err := json.Marshal(fencedAlice)
	s.Require().NoError(err)
	s.JSONEq(string(plainRaw), string(fencedRaw), "the fenced sheet persists as the plain one")
	s.ElementsMatch(plainOut.Saved.Written, out.Saved.Written, "and the report names the same aggregates")
}

// TestTheSaveIsFencedOnTheVersionTheVerbLoaded loads at a version Launch
// never leaves (7), so a session that handed back anything but what it read
// would be caught: the save names 7 and lands at 8.
func (s *CheckpointFenceSuite) TestTheSaveIsFencedOnTheVersionTheVerbLoaded() {
	mgr := s.manager(&fakeStream{}, nil)
	_, err := mgr.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().NoError(err)
	s.store.versions[testSession] = 7
	s.store.expected = nil

	_, err = s.moveAlice(mgr, spatial.Position{X: 2, Y: 1})

	s.Require().NoError(err)
	s.Equal([]uint64{7}, s.store.expected)
	s.Equal(uint64(8), s.store.versions[testSession])
}

// TestASeatedLevelRidesTheFence covers the one verb that writes a seated
// sheet without opening its run: under the fence the level lands in a fenced
// save of that run, and a stale one lands nothing.
func (s *CheckpointFenceSuite) TestASeatedLevelRidesTheFence() {
	s.store.chars.byID["alice"] = advancingFighter("alice")
	mgr := s.manager(&fakeStream{}, nil)
	_, err := mgr.Launch(context.Background(), sceneInput(authoredWorld()))
	s.Require().NoError(err)
	level := func() (*session.LevelUpOutput, error) {
		return mgr.LevelUp(context.Background(), &session.LevelUpInput{
			Character: "alice", HitPointMethod: session.HitPointsAverage,
		})
	}

	s.store.onGet = func() { s.store.versions[testSession]++ }
	sheetBefore := s.sheetBytes("alice")
	_, err = level()
	s.Require().ErrorIs(err, session.ErrStaleCheckpoint)
	s.Equal(sheetBefore, s.sheetBytes("alice"), "a stale level lands nothing")

	versionBefore := s.store.versions[testSession]
	out, err := level()
	s.Require().NoError(err)
	s.Contains(out.Saved.Written, "character:alice")
	s.Equal(2, s.store.chars.byID["alice"].Level)
	s.Equal(versionBefore+1, s.store.versions[testSession], "the level moved the run's checkpoint")
	s.Zero(s.store.chars.saves, "the level never went around the fence")
}
