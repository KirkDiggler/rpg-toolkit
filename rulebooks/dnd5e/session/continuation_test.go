// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// continuation_test.go is rpg-project#879 S1 through the session: a Move
// answers with the walker's own steps up to the step that pays a world round,
// and the rest of the verb (that step, the raise inside it, the steps after,
// the save and the delivery) runs as a continuation the host's runner holds,
// under the session's guard. Order is unchanged: a continued walk delivers
// exactly what a whole walk of the same path delivers.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// detachingRunner continues at once, the way a host's runner documented on
// [session.RunContinuationInput.Continue] does: under its Run context with
// the request's cancel removed. It keeps the report so a test can read it.
type detachingRunner struct {
	report *session.ContinuationReport
	err    error
}

func (r *detachingRunner) Run(ctx context.Context, in *session.RunContinuationInput) error {
	r.report, r.err = in.Continue(context.WithoutCancel(ctx))
	return nil
}

type requestKey struct{}

// valueCharacters reads a request value on every sheet read, the way a host
// repository reading a trace id from its context does.
type valueCharacters struct {
	*fakeCharacters
	seen *[]any
}

func (c valueCharacters) GetCharacter(ctx context.Context, id string) (*character.Data, error) {
	*c.seen = append(*c.seen, ctx.Value(requestKey{}))
	return c.fakeCharacters.GetCharacter(ctx, id)
}

// capturingRunner holds the continuation it is handed without running it, so
// a test decides when the rest of the verb happens.
type capturingRunner struct {
	handed []*session.RunContinuationInput
	err    error
}

func (r *capturingRunner) Run(_ context.Context, in *session.RunContinuationInput) error {
	if r.err != nil {
		return r.err
	}
	r.handed = append(r.handed, in)
	return nil
}

// slotLocker is real session exclusion that honours cancellation while it
// waits, so a verb that cannot have the guard fails with its context's error
// rather than hanging the test.
type slotLocker struct {
	slot     chan struct{}
	releases int
}

func newSlotLocker() *slotLocker { return &slotLocker{slot: make(chan struct{}, 1)} }

func (l *slotLocker) LockSession(ctx context.Context, _ *session.LockSessionInput) (*session.LockSessionOutput, error) {
	select {
	case l.slot <- struct{}{}:
		return &session.LockSessionOutput{Release: func() {
			l.releases++
			<-l.slot
		}}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// LockCharacter grants every character guard: these scenes are about the
// session's.
func (l *slotLocker) LockCharacter(context.Context, *session.LockCharacterInput) (*session.LockCharacterOutput, error) {
	return &session.LockCharacterOutput{Release: func() {}}, nil
}

// contextCharacters refuses a sheet read under a finished context, the way a
// host repository honouring its context would. A continuation still calling
// the host under the request's context after the request ended fails here.
type contextCharacters struct{ *fakeCharacters }

func (c contextCharacters) GetCharacter(ctx context.Context, id string) (*character.Data, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.fakeCharacters.GetCharacter(ctx, id)
}

// continuedRun is one manager over its own stores.
type continuedRun struct {
	mgr        *session.Manager
	encounters *fakeEncounters
	stream     *fakeStream
	locker     *slotLocker
	runner     *capturingRunner
}

type ContinuationSuite struct{ suite.Suite }

func TestContinuationSuite(t *testing.T) { suite.Run(t, new(ContinuationSuite)) }

// The goblin camp is NEUTRAL to the party, so the walk stays on the world
// clock in sight of a creature the raise asks for its time.
const continuationCamp encounter.FactionID = "camp"

// launch starts the scene on a manager with the given runner (nil for none):
// one long hall, alice (a dwarf, twenty-five feet: five cells a world round)
// at its west end, and a goblin who holds when it has time.
func (s *ContinuationSuite) launch(runner *capturingRunner) *continuedRun {
	run := &continuedRun{
		encounters: newFakeEncounters(), stream: &fakeStream{}, locker: newSlotLocker(), runner: runner,
	}
	cfg := &session.Config{
		Record: session.DiscardRecord{}, Acts: run.stream, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Driver(), Sessions: newFakeSessions(),
		Encounters: run.encounters, Characters: contextCharacters{testCharacters()},
		Locker: run.locker,
	}
	if runner != nil {
		cfg.Continuations = runner
	}
	mgr, err := session.NewManager(cfg)
	s.Require().NoError(err)
	run.mgr = mgr

	goblin := monsterAt("goblin", refs.Monsters.Goblin().String(), 11, 3)
	goblin.Faction = string(continuationCamp)
	goblin.Table = encounter.Table{encounter.AnswerTime: {{Weight: 1, Hold: true}}}
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
	run.stream.published = nil
	return run
}

// eastFrom is the row-1 cells east of alice's start, from column 2 to last.
func eastFrom(last int) []spatial.Position {
	var path []spatial.Position
	for col := 2; col <= last; col++ {
		path = append(path, hexCell(col, 1))
	}
	return path
}

// longWalk is longer than a world round at alice's pace, so a pace comes due
// part way along it.
func longWalk() []spatial.Position { return eastFrom(10) }

func (s *ContinuationSuite) move(run *continuedRun, ctx context.Context, path []spatial.Position) (*session.MoveOutput, error) {
	return run.mgr.Move(ctx, &session.MoveInput{Session: testSession, Member: "alice", Path: path})
}

// record is the whole stored record, oldest first.
func (s *ContinuationSuite) record(run *continuedRun) []session.RecordEvent {
	out, err := run.mgr.Record(context.Background(), &session.RecordReadInput{Session: testSession})
	s.Require().NoError(err)
	return out.Events
}

// alicesMoves is every moved beat of alice's in the record, oldest first.
func (s *ContinuationSuite) alicesMoves(run *continuedRun) []session.RecordEvent {
	var moves []session.RecordEvent
	for _, event := range s.record(run) {
		if event.Beat != encounter.BeatMoved {
			continue
		}
		var body struct {
			Member string `json:"member"`
		}
		s.Require().NoError(json.Unmarshal(event.Fields, &body))
		if body.Member == "alice" {
			moves = append(moves, event)
		}
	}
	return moves
}

// TestAContinuedWalkDeliversWhatAWholeWalkDelivers is the slice's headline:
// under a runner the walk answers before the step that pays the round, with
// nothing saved or delivered; the continuation, run after the request's own
// context has ended, saves and delivers exactly the acts and the record a
// whole walk of the same path makes, in the same order.
func (s *ContinuationSuite) TestAContinuedWalkDeliversWhatAWholeWalkDelivers() {
	whole := s.launch(nil)
	wholeOut, err := s.move(whole, context.Background(), longWalk())
	s.Require().NoError(err)
	s.Require().False(wholeOut.Continues, "without a runner a walk never continues")
	s.Require().Equal(session.MovementCompleted, wholeOut.Status)

	continued := s.launch(&capturingRunner{})
	savesBefore := continued.encounters.saves
	request, endRequest := context.WithCancel(context.Background())
	answer, err := s.move(continued, request, longWalk())
	s.Require().NoError(err)
	endRequest()

	s.True(answer.Continues)
	s.Equal(session.MovementContinuing, answer.Status)
	s.NotEmpty(answer.Steps, "the steps before the due one are the answer")
	s.Less(len(answer.Steps), len(longWalk()), "the answer stops before the step that pays the round")
	s.Equal(wholeOut.Steps[:len(answer.Steps)], answer.Steps)
	s.NotEmpty(answer.Through)
	s.Nil(answer.Outcome)
	s.Nil(answer.Formed)
	s.Zero(answer.Saved, "nothing is saved before the rest runs")
	s.Zero(answer.Delivery, "nothing is delivered before the save")
	s.Equal(savesBefore, continued.encounters.saves, "the record is not saved at the answer")
	s.Empty(continued.stream.published, "no act reaches the stream at the answer")

	s.Require().NotEmpty(continued.runner.handed, "the rest was handed to the runner")
	handed := continued.runner.handed[0]
	s.Equal(testSession, handed.Session)
	s.Equal(session.VerbMove, handed.Verb)

	report, err := handed.Continue(context.Background())
	s.Require().NoError(err)
	s.Greater(continued.encounters.saves, savesBefore, "the continuation saved the record")
	s.NotZero(report.Saved)
	s.Require().NotNil(report.Move)
	s.Equal(session.MovementCompleted, report.Move.Status)
	s.Equal(wholeOut.Steps, append(append([]session.Step(nil), answer.Steps...), report.Move.Steps...),
		"the answer's steps and the rest's are the whole walk's")

	s.Equal(canonicalActs(whole.stream.published), canonicalActs(continued.stream.published),
		"a continued walk delivers exactly the acts a whole walk does, in order")
	s.Equal(canonicalRecord(s.record(whole)), canonicalRecord(s.record(continued)),
		"and records exactly the same story")

	moves := s.alicesMoves(continued)
	walked := moves[len(moves)-len(longWalk()):]
	s.Equal(walked[len(answer.Steps)-1].ID, answer.Through,
		"Through is the moved beat of the answer's last step")
	ticked := false
	for _, event := range s.record(continued) {
		if event.ID > answer.Through && event.Beat == encounter.BeatTick {
			ticked = true
		}
	}
	s.True(ticked, "the world's round was raised after the answer, in the rest")
}

// TestAWalkThatPaysNoRoundAnswersWhole: under a runner, a path along which no
// pace comes due is the whole verb before the answer, exactly as without one
// (the write tier for every verb is a later slice): the runner is never
// handed anything, and the answer carries the save.
func (s *ContinuationSuite) TestAWalkThatPaysNoRoundAnswersWhole() {
	run := s.launch(&capturingRunner{})
	savesBefore, releasesBefore := run.encounters.saves, run.locker.releases
	out, err := s.move(run, context.Background(), eastFrom(3))
	s.Require().NoError(err)
	s.False(out.Continues)
	s.Empty(out.Through)
	s.Equal(session.MovementCompleted, out.Status)
	s.Equal(eastFrom(3), positionsOf(out.Steps))
	s.Empty(run.runner.handed, "nothing was left to continue")
	s.Greater(run.encounters.saves, savesBefore, "the answer waited for the save")
	s.NotZero(out.Saved)
	s.NotEmpty(run.stream.published)
	s.Equal(releasesBefore+1, run.locker.releases, "the verb gave its guard back")
}

// TestAContinuingAnswerWithNoStepsStillNamesItsResumePoint: a walker whose
// very next step pays the round answers with no steps, and Through is still
// a resume point: the newest record entry at the answer, before every moved
// beat the continuation records.
func (s *ContinuationSuite) TestAContinuingAnswerWithNoStepsStillNamesItsResumePoint() {
	run := s.launch(&capturingRunner{})
	// Four cells of alice's five: no round is paid, and her next step pays it.
	first, err := s.move(run, context.Background(), eastFrom(5))
	s.Require().NoError(err)
	s.Require().False(first.Continues, "four cells pay no round")

	rest := []spatial.Position{hexCell(6, 1), hexCell(7, 1), hexCell(8, 1)}
	answer, err := s.move(run, context.Background(), rest)
	s.Require().NoError(err)
	s.Require().True(answer.Continues)
	s.Empty(answer.Steps, "the first step is the one that pays the round")
	s.NotEmpty(answer.Through, "an answer with no steps still names where it covers up to")

	report, err := run.runner.handed[0].Continue(context.Background())
	s.Require().NoError(err)
	s.Require().NotNil(report.Move)
	s.Equal(rest, positionsOf(report.Move.Steps))

	var after []session.RecordEvent
	for _, move := range s.alicesMoves(run) {
		if move.ID > answer.Through {
			after = append(after, move)
		}
	}
	s.Require().NotEmpty(after)
	moves := s.alicesMoves(run)
	s.Equal(moves[len(moves)-len(rest):], after,
		"every moved beat of the continuation, and only those, come after Through")
}

// TestTheContinuationHoldsTheGuard: until the rest has run, the session is
// the continuation's, and the next verb waits; once it has, the next verb
// proceeds.
func (s *ContinuationSuite) TestTheContinuationHoldsTheGuard() {
	run := s.launch(&capturingRunner{})
	answer, err := s.move(run, context.Background(), longWalk())
	s.Require().NoError(err)
	s.Require().True(answer.Continues)

	waiting, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = run.mgr.Status(waiting, &session.StatusInput{Session: testSession})
	s.Require().Error(err)
	s.ErrorIs(err, context.DeadlineExceeded, "the next verb waits for the world to finish")

	_, err = run.runner.handed[0].Continue(context.Background())
	s.Require().NoError(err)

	_, err = run.mgr.Status(context.Background(), &session.StatusInput{Session: testSession})
	s.Require().NoError(err, "the continuation gave the guard back")
}

// TestAContinuationRunsOnce: a second Continue has nothing left to do and no
// guard to give back.
func (s *ContinuationSuite) TestAContinuationRunsOnce() {
	run := s.launch(&capturingRunner{})
	_, err := s.move(run, context.Background(), longWalk())
	s.Require().NoError(err)
	handed := run.runner.handed[0]
	_, err = handed.Continue(context.Background())
	s.Require().NoError(err)
	released := run.locker.releases

	_, err = handed.Continue(context.Background())
	s.ErrorIs(err, session.ErrContinuationSpent)
	s.Equal(released, run.locker.releases, "the guard was given back once")
}

// TestARefusedHandOffReleasesTheGuard: a runner that will not take the rest
// fails the verb with its own error, saves nothing, and leaves the session
// free.
func (s *ContinuationSuite) TestARefusedHandOffReleasesTheGuard() {
	runner := &capturingRunner{}
	run := s.launch(runner)
	savesBefore := run.encounters.saves
	full := errors.New("queue full")
	runner.err = full

	_, err := s.move(run, context.Background(), longWalk())
	s.Require().Error(err)
	s.ErrorIs(err, full)
	s.Equal(savesBefore, run.encounters.saves, "nothing was saved")
	s.Empty(run.stream.published)

	waiting, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = run.mgr.Status(waiting, &session.StatusInput{Session: testSession})
	s.Require().NoError(err, "the refused verb gave its guard back")
}

// TestARefusalBeforeTheHandOffReleasesTheGuard: a walk refused before any
// step (here a path that does not start beside the walker) is an ordinary
// refusal under a runner too: the runner is handed nothing and the session
// is free.
func (s *ContinuationSuite) TestARefusalBeforeTheHandOffReleasesTheGuard() {
	run := s.launch(&capturingRunner{})
	_, err := s.move(run, context.Background(), []spatial.Position{hexCell(9, 1)})
	s.Require().Error(err)
	s.Empty(run.runner.handed)

	waiting, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = run.mgr.Status(waiting, &session.StatusInput{Session: testSession})
	s.Require().NoError(err, "the refused verb gave its guard back")
}

// ordinals renames ids by their order of first appearance. Record ids are
// minted from the clock, so two runs of one story differ in them and in
// nothing else; the ordinal keeps what an id means to a reader (which
// entries share one, and their order) and drops the instant it was minted.
type ordinals map[string]string

func (o ordinals) of(id string) string {
	if id == "" {
		return ""
	}
	if _, ok := o[id]; !ok {
		o[id] = fmt.Sprintf("#%d", len(o)+1)
	}
	return o[id]
}

// canonicalActs is acts with their record and act ids made ordinal.
func canonicalActs(acts []session.Act) []session.Act {
	ids, act := ordinals{}, ordinals{}
	out := make([]session.Act, 0, len(acts))
	for _, a := range acts {
		a.ID, a.Act = ids.of(a.ID), act.of(a.Act)
		out = append(out, a)
	}
	return out
}

// canonicalRecord is a record with its ids made ordinal. An act id also
// appears inside a beat's fields, so those are compared with every id in
// them made ordinal too.
func canonicalRecord(events []session.RecordEvent) []session.RecordEvent {
	ids, acts := ordinals{}, ordinals{}
	out := make([]session.RecordEvent, 0, len(events))
	for _, e := range events {
		fields := string(e.Fields)
		if e.Act != "" {
			fields = strings.ReplaceAll(fields, e.Act, acts.of(e.Act))
		}
		e.ID, e.Act, e.Fields = ids.of(e.ID), acts.of(e.Act), json.RawMessage(fields)
		out = append(out, e)
	}
	return out
}

// TestARunnerContinuingUnderItsRunContextCompletes: a runner that continues
// under context.WithoutCancel of the context Run was handed (the documented
// shape) completes the walk, and the host's collaborators still read the
// request's values through it. Run being handed the verb's own context made
// that context its own parent, and the first value read never returned.
func (s *ContinuationSuite) TestARunnerContinuingUnderItsRunContextCompletes() {
	runner := &detachingRunner{}
	var seen []any
	mgr, err := session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: &fakeStream{}, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Driver(), Sessions: newFakeSessions(),
		Encounters: newFakeEncounters(), Characters: valueCharacters{testCharacters(), &seen},
		Continuations: runner,
	})
	s.Require().NoError(err)
	launchScene(s.T(), mgr, scene{
		Field:   encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 13, 4)}},
		Party:   []sceneSeat{seatAt("alice", 1, 1)},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	})
	seen = nil

	request := context.WithValue(context.Background(), requestKey{}, "trace-1")
	answer, err := mgr.Move(request, &session.MoveInput{Session: testSession, Member: "alice", Path: longWalk()})
	s.Require().NoError(err)
	s.Require().True(answer.Continues)
	s.Require().NoError(runner.err)
	s.Require().NotNil(runner.report)
	s.Require().NotNil(runner.report.Move)
	s.Equal(session.MovementCompleted, runner.report.Move.Status)
	s.NotZero(runner.report.Saved)
	s.Require().NotEmpty(seen, "the walk read sheets")
	for _, value := range seen {
		s.Equal("trace-1", value, "every sheet read carried the request's value")
	}
}

// positionsOf is the cells a walk's steps entered.
func positionsOf(steps []session.Step) []spatial.Position {
	out := make([]spatial.Position, 0, len(steps))
	for _, step := range steps {
		out = append(out, step.Position)
	}
	return out
}
