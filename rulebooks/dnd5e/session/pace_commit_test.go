// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// pace_commit_test.go is rpg-project#879 S13 (ruling R7, option D): a
// continued walk commits at each pace boundary. After every step that paid a
// world round, the raise that step fired is saved and delivered before the
// walk's next step, and the remaining steps go in the next commit. Nothing is
// delivered twice, nothing is missing, and a stale save mid-continuation
// fails closed with the landed commits standing.

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// batchStream keeps every PublishActs call as its own batch: one batch is one
// commit's delivery.
type batchStream struct {
	mu      sync.Mutex
	batches [][]session.Act
}

func (b *batchStream) PublishActs(_ context.Context, acts []session.Act) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.batches = append(b.batches, append([]session.Act(nil), acts...))
	return nil
}

// flat is every batch's acts in delivery order.
func (b *batchStream) flat() []session.Act {
	var out []session.Act
	for _, batch := range b.batches {
		out = append(out, batch...)
	}
	return out
}

// host is one repository shape a pace commit must work over.
type host string

const (
	hostPlain     host = "plain"
	hostFenced    host = "fenced-only"
	hostAppending host = "appending-only"
	hostPieces    host = "pieces"
)

// pacedRun is one manager over one host shape, with a runner that holds the
// continuation until the test runs it.
type pacedRun struct {
	mgr    *session.Manager
	stream *batchStream
	runner *capturingRunner
	plain  *fakeEncounters
	store  *piecesStore
}

// PaceCommitSuite drives a continued walk over each host shape.
type PaceCommitSuite struct{ suite.Suite }

func TestPaceCommitSuite(t *testing.T) { suite.Run(t, new(PaceCommitSuite)) }

// launch starts the continuation hall (alice at its west end, twenty-five
// feet a round, and a neutral goblin who walks west when a round wakes it) over shape, with a runner
// when continued is true. The launch's own acts are dropped from the stream.
func (s *PaceCommitSuite) launch(shape host, continued bool) *pacedRun {
	run := &pacedRun{stream: &batchStream{}, plain: newFakeEncounters(), store: newPiecesStore()}
	cfg := &session.Config{
		Record: session.DiscardRecord{}, Acts: run.stream,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Driver(),
	}
	switch shape {
	case hostPlain:
		cfg.Seats, cfg.Sessions, cfg.Encounters, cfg.Characters = newFakeSeats(), newFakeSessions(), run.plain, testCharacters()
	default:
		store := run.store.appendingStore
		cfg.Seats, cfg.Sessions, cfg.Characters = store.seats, storeSessions{store: store.fencedStore}, store.chars
		switch shape {
		case hostFenced:
			cfg.Encounters = fencedOnly{store: store}
		case hostAppending:
			cfg.Encounters = appendingOnly{fencedOnly{store: store}}
		default:
			cfg.Encounters = run.store
		}
	}
	if continued {
		run.runner = &capturingRunner{}
		cfg.Continuations = run.runner
	}
	mgr, err := session.NewManager(cfg)
	s.Require().NoError(err)
	run.mgr = mgr

	goblin := monsterAt("goblin", refs.Monsters.Goblin().String(), 11, 3)
	goblin.Faction = string(continuationCamp)
	// The goblin WALKS when a round wakes it, toward the hall's west end in
	// alice's sight, so a creature's walk run sits beside hers at every pace
	// boundary (act.go: a run never spans a commit).
	goblin.Behaviors = ordinaryBehavior(encounter.Table{encounter.AnswerTime: {
		{Weight: 1, Toward: &encounter.Selector{Path: []spatial.Position{hexCell(1, 3)}}},
	}})
	launchScene(s.T(), mgr, scene{
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 14, 4)},
			Factions: []encounter.FactionInput{{ID: continuationCamp}},
			Dispositions: []encounter.DispositionInput{
				{Between: [2]encounter.FactionID{continuationCamp, encounter.FactionParty}, Stance: encounter.StanceNeutral},
			},
		},
		Party:    []sceneSeat{seatAt("alice", 1, 1)},
		Monsters: []dungeonspec.MonsterPlacement{goblin},
		Endings:  []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	})
	run.stream.batches = nil
	return run
}

func (s *PaceCommitSuite) move(run *pacedRun, path []spatial.Position) *session.MoveOutput {
	out, err := run.mgr.Move(context.Background(), &session.MoveInput{Session: testSession, Member: "alice", Path: path})
	s.Require().NoError(err)
	return out
}

// continueRest runs the continuation the runner holds.
func (s *PaceCommitSuite) continueRest(run *pacedRun) (*session.ContinuationReport, error) {
	s.Require().Len(run.runner.handed, 1, "the walk continued")
	return run.runner.handed[0].Continue(context.Background())
}

// oneRaise is a walk whose rest pays one world round part way along it: the
// answer stops before the fifth cell, which pays, and four cells follow.
func oneRaise() []spatial.Position { return eastFrom(10) }

// twoRaises pays a second round on its tenth cell, with a cell after it.
func twoRaises() []spatial.Position { return eastFrom(12) }

// perRecipient is acts grouped by recipient, each in delivery order, ids made
// ordinal per recipient: what one listener receives, which is the stream's
// contract (each recipient's acts in record order).
func perRecipient(acts []session.Act) map[string][]session.Act {
	out := map[string][]session.Act{}
	for _, a := range acts {
		out[a.Recipient] = append(out[a.Recipient], a)
	}
	for who, list := range out {
		out[who] = canonicalActs(list)
	}
	return out
}

// goblinWalks reports whether a batch carries the goblin's own walk for
// alice: a creature's run, woken by the raise.
func goblinWalks(batch []session.Act) bool {
	for _, a := range batch {
		if w, ok := a.Body.(session.WalkAct); ok && a.Recipient == "alice" && w.Mover.ID == "goblin" {
			return true
		}
	}
	return false
}

// hasTick reports whether a batch carries a world round's tick.
func hasTick(batch []session.Act) bool {
	for _, a := range batch {
		if c, ok := a.Body.(session.ClockEntry); ok && c.Mark == session.ClockTick {
			return true
		}
	}
	return false
}

// TestAPaceCommitDeliversTheRaiseBeforeTheRest is the slice's headline, over
// every host shape: a one-raise continued walk publishes twice. The first
// batch holds the round's tick; the union of both batches, per listener, is
// exactly what a whole walk of the same path delivers, with no act id
// delivered twice. The record the store holds is the whole walk's record.
func (s *PaceCommitSuite) TestAPaceCommitDeliversTheRaiseBeforeTheRest() {
	for _, shape := range []host{hostPlain, hostFenced, hostAppending, hostPieces} {
		s.Run(string(shape), func() {
			whole := s.launch(shape, false)
			wholeOut := s.move(whole, oneRaise())
			s.Require().Equal(session.MovementCompleted, wholeOut.Status)
			s.Require().Len(whole.stream.batches, 1, "a whole walk commits once")

			run := s.launch(shape, true)
			answer := s.move(run, oneRaise())
			s.Require().True(answer.Continues)
			s.Empty(run.stream.batches, "nothing is delivered at the answer")

			report, err := s.continueRest(run)
			s.Require().NoError(err)
			s.Equal(2, report.Commits, "one pace commit and the final one")
			s.Require().Len(run.stream.batches, 2, "each commit delivers its own batch")
			s.True(hasTick(run.stream.batches[0]), "the first batch carries the raise")
			s.True(goblinWalks(run.stream.batches[0]), "and the goblin's walk the raise woke")
			s.False(hasTick(run.stream.batches[1]), "the second carries only the remaining steps")
			s.Equal(session.MovementCompleted, report.Move.Status)

			s.Equal(perRecipient(whole.stream.flat()), perRecipient(run.stream.flat()),
				"per listener, the two batches are exactly the whole walk's delivery")
			for who, acts := range perRecipient(run.stream.flat()) {
				seen := map[string]bool{}
				for _, a := range acts {
					key := a.ID + "|" + a.Act
					s.False(seen[key], "%s: act %s delivered twice", who, key)
					seen[key] = true
				}
			}
			s.Equal(canonicalRecord(storedRecord(s.T(), whole.mgr)), canonicalRecord(storedRecord(s.T(), run.mgr)),
				"and the store holds exactly the whole walk's record")

			newest := storedRecord(s.T(), run.mgr)
			s.Equal(newest[len(newest)-1].ID, report.SavedThrough, "the report names the newest id saved")
		})
	}
}

// TestCatchUpEqualsLiveAcrossAPaceCommit is #541 R2 across the new boundary:
// a listener that reads catch-up after the move reads exactly what it was
// handed live across both commits, and resuming after any delivered act's
// id gives exactly the live suffix after it.
func (s *PaceCommitSuite) TestCatchUpEqualsLiveAcrossAPaceCommit() {
	run := s.launch(hostPieces, true)
	before := map[string]string{}
	for _, member := range []string{"alice", "goblin"} {
		caught := actsOf(s.T(), run.mgr, member, "")
		if n := len(caught.Acts); n > 0 {
			before[member] = caught.Acts[n-1].ID
		}
	}
	s.move(run, oneRaise())
	_, err := s.continueRest(run)
	s.Require().NoError(err)
	s.Require().Len(run.stream.batches, 2)

	for _, member := range []string{"alice", "goblin"} {
		var live []session.Act
		for _, a := range run.stream.flat() {
			if a.Recipient == member {
				live = append(live, a)
			}
		}
		s.Require().NotEmpty(live, member)
		s.Equal(live, actsOf(s.T(), run.mgr, member, before[member]).Acts, "%s: live is catch-up", member)
		for i, a := range live {
			last := i
			for j := i; j < len(live); j++ {
				if live[j].ID == a.ID {
					last = j
				}
			}
			want := live[last+1:]
			got := actsOf(s.T(), run.mgr, member, a.ID).Acts
			if len(want) == 0 {
				s.Empty(got, "%s after %s", member, a.ID)
				continue
			}
			s.Equal(want, got, "%s after %s", member, a.ID)
		}
	}
}

// TestAOneRaiseWalkLandsTwoFencedSaves: on a pieces host the continuation
// makes two SavePieces calls, fenced N then N+1. The second appends the
// record from where the first ended (its Since is the first's NextSeq), and
// it sends only the pieces whose marks moved since the first landed.
func (s *PaceCommitSuite) TestAOneRaiseWalkLandsTwoFencedSaves() {
	run := s.launch(hostPieces, true)
	s.move(run, oneRaise())
	callsBefore, appendsBefore := len(run.store.calls), len(run.store.appends)
	_, err := s.continueRest(run)
	s.Require().NoError(err)

	calls := run.store.calls[callsBefore:]
	appends := run.store.appends[appendsBefore:]
	s.Require().Len(calls, 2, "a pace commit and the final one")
	s.Require().Len(appends, 2)
	s.True(calls[0].landed)
	s.True(calls[1].landed)
	s.Equal(calls[0].expected+1, calls[1].expected, "the second save is fenced on the version the first made")
	s.Equal(appends[0].restNext, appends[1].since, "the second save appends from where the first ended")
	s.NotEmpty(appends[0].seqs, "the first save carries the raise")
	s.NotEmpty(appends[1].seqs, "the second carries the remaining steps")
	s.Less(appends[0].seqs[len(appends[0].seqs)-1], appends[1].seqs[0], "no entry is appended twice")
	s.Less(len(calls[1].keys), len(calls[1].marks), "the second save sends only the pieces that moved")
	for key, mark := range calls[1].marks {
		if !slices.Contains(calls[1].keys, key) {
			s.Equal(calls[0].marks[key], mark, "an unsent piece keeps the first save's mark: %s", key)
		}
	}
}

// TestADueLastStepCommitsOnce: a walk whose due step is its last cell ends
// with that step, so the rest makes one commit, the final one, and one
// publish; a pace commit there would save and publish nothing new.
func (s *PaceCommitSuite) TestADueLastStepCommitsOnce() {
	run := s.launch(hostPieces, true)
	path := eastFrom(6)
	answer := s.move(run, path)
	s.Require().True(answer.Continues)
	s.Require().Len(answer.Steps, len(path)-1, "control: the due step is the walk's last")
	callsBefore := len(run.store.calls)

	report, err := s.continueRest(run)
	s.Require().NoError(err)
	s.Equal(1, report.Commits)
	s.Len(run.stream.batches, 1)
	s.True(hasTick(run.stream.batches[0]))
	s.Len(run.store.calls[callsBefore:], 1, "one save")
}

// TestATwoRaiseWalkMakesThreeCommits: a walk whose rest pays two rounds
// commits after each, and once more at its end.
func (s *PaceCommitSuite) TestATwoRaiseWalkMakesThreeCommits() {
	whole := s.launch(hostPieces, false)
	s.move(whole, twoRaises())

	run := s.launch(hostPieces, true)
	s.move(run, twoRaises())
	report, err := s.continueRest(run)
	s.Require().NoError(err)
	s.Equal(3, report.Commits)
	s.Require().Len(run.stream.batches, 3)
	s.True(hasTick(run.stream.batches[0]))
	s.True(hasTick(run.stream.batches[1]))
	s.False(hasTick(run.stream.batches[2]))
	s.Equal(perRecipient(whole.stream.flat()), perRecipient(run.stream.flat()))
}

// TestAStaleSecondSaveFailsClosed: another writer takes the world between the
// pace commit and the final one. The final save is refused stale; nothing is
// retried; the first commit stands in the store and was delivered; the walk's
// remaining steps are neither saved nor delivered; and the report beside the
// error names the one landed commit and the newest id it saved.
func (s *PaceCommitSuite) TestAStaleSecondSaveFailsClosed() {
	run := s.launch(hostPieces, true)
	answer := s.move(run, oneRaise())
	saves := 0
	run.store.onSave = func(in *session.SaveFencedInput) {
		saves++
		if saves == 2 {
			run.store.versions[in.Encounter]++ // another writer landed first
		}
	}
	report, err := s.continueRest(run)
	s.Require().ErrorIs(err, session.ErrStaleCheckpoint)
	s.Require().NotNil(report, "the landed commit is reported beside the error")
	s.Equal(1, report.Commits)
	s.Nil(report.Move, "the walk's end never landed")
	s.Equal(2, saves, "nothing retried the refused save")
	s.Require().Len(run.stream.batches, 1, "only the landed commit was delivered")
	s.True(hasTick(run.stream.batches[0]))

	stored := storedRecord(s.T(), run.mgr)
	s.Equal(stored[len(stored)-1].ID, report.SavedThrough, "the store stops where the report says")
	for _, a := range run.stream.batches[0] {
		s.LessOrEqual(a.ID, report.SavedThrough, "nothing delivered is past what the store holds")
	}
	at := s.cellOf(run.mgr, "alice")
	path := oneRaise()
	s.Equal(path[len(answer.Steps)], at, "alice stands where the pace commit left her: the due cell")
}

// cellOf is a member's stored cell.
func (s *PaceCommitSuite) cellOf(mgr *session.Manager, member string) spatial.Position {
	enc, err := mgr.LoadWorldForTest(context.Background(), testSession)
	s.Require().NoError(err)
	for _, m := range enc.WorldView().Members {
		if string(m.ID) == member && m.Cell != nil {
			return spatial.Position{X: m.Cell.X, Y: m.Cell.Y}
		}
	}
	s.FailNow(member + " is not placed")
	return spatial.Position{}
}

// storedRecord is a manager's whole stored record.
func storedRecord(t *testing.T, mgr *session.Manager) []session.RecordEvent {
	t.Helper()
	out, err := mgr.Record(context.Background(), &session.RecordReadInput{Session: testSession})
	if err != nil {
		t.Fatal(err)
	}
	return out.Events
}
