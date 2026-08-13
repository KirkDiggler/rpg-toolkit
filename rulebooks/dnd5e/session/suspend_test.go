// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SuspendTestSuite covers the interrupt spine: a resolution that stops mid-way,
// persists as data, and resumes — possibly in a different process.
type SuspendTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func (s *SuspendTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = testCharacters()
	s.mgr = managerOver(s.T(), s.sessions, s.encounters)
}

// SetupSubTest gives every s.Run() its own stores. Without it a table would
// share a session across rows, and the second row would fail to start one
// rather than testing what it claims to test.
func (s *SuspendTestSuite) SetupSubTest() {
	s.SetupTest()
}

func managerOver(t fataler, sessions *fakeSessions, encounters *fakeEncounters) *session.Manager {
	return managerOverRepos(t, sessions, encounters)
}

// managerOverRepos builds a manager over any repositories, so a test can swap
// in one that fails on demand without rebuilding the world it already set up.
func managerOverRepos(
	t fataler, sessions session.SessionRepository, encounters session.EncounterRepository,
) *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		Sessions: sessions, Encounters: encounters, Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	if err != nil {
		t.Fatalf("building manager: %v", err)
	}
	return mgr
}

// ambushWorld is a hall split by a wall of occluders with a single gap at y=3.
//
// Alice starts at (1,1) with no line of sight to anything; the ogre waits at
// (5,3) behind the wall. Walking north to (2,3) lines her up with the gap and
// the ogre comes into view — first contact, mid-path, with a cell still to go.
// That is scene 2 with no combat in it: perception alone is enough to stop the
// world, which is why this wave lands before entities.
func ambushWorld(t fataler, extra ...encounter.MemberInput) *encounter.EncounterData {
	occluders := make([]spatial.Position, 0, 7)
	for y := 0; y < 8; y++ {
		if y == 3 {
			continue // the gap
		}
		occluders = append(occluders, spatial.Position{X: 3, Y: float64(y)})
	}

	members := []encounter.MemberInput{
		{ID: "alice", Kind: encounter.KindPlayer, Room: "hall", Position: spatial.Position{X: 1, Y: 1}},
		{ID: "ogre", Kind: encounter.KindMonster, Room: "hall", Position: spatial.Position{X: 5, Y: 3}},
	}
	members = append(members, extra...)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Rooms: []encounter.RoomInput{
			{ID: "hall", Width: 8, Height: 8, Occluders: occluders},
		}},
		Members: members,
		Endings: []encounter.EndingInput{
			{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
				Room: "hall", Position: spatial.Position{X: 7, Y: 7},
			}},
		},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building ambush world: %v", err)
	}
	data := enc.ToData()
	return &data
}

func (s *SuspendTestSuite) startAmbush(extra ...encounter.MemberInput) {
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: ambushWorld(s.T(), extra...),
	})
	s.Require().NoError(err)
}

// walkIntoTheAmbush walks the three-cell path that trips the checkpoint on cell
// two, and returns the output.
func (s *SuspendTestSuite) walkIntoTheAmbush() *session.MoveOutput {
	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 2}, {X: 2, Y: 3}, {X: 2, Y: 4}},
	})
	s.Require().NoError(err)
	return out
}

// TestPerceptionStopsTheWalk is the headline of this wave, and scene 2.
//
// Alice asks for three cells and gets two, because the second brought the ogre
// into view. The walk did not fail — it suspended, which is a different thing
// and is reported differently: no error, a short Steps, and a window naming who
// owes an answer.
func (s *SuspendTestSuite) TestPerceptionStopsTheWalk() {
	s.startAmbush()
	out := s.walkIntoTheAmbush()

	s.Require().Len(out.Steps, 2, "the walk stopped where perception changed")
	s.Equal(spatial.Position{X: 2, Y: 3}, out.Steps[1].Position)

	s.Require().NotNil(out.Pending, "and the world is waiting on a decision")
	s.Equal("alice", out.Pending.Audience, "the walker owes the answer")
	s.Equal([]string{"ogre"}, out.Pending.Prompt.Sighted, "and is shown what she just saw")
	s.Equal(spatial.Position{X: 2, Y: 3}, out.Pending.Prompt.At, "where she stopped, not where she was headed")
	s.Equal("hall", out.Pending.Prompt.Room)

	kinds := make([]session.OptionKind, 0, len(out.Pending.Options))
	for _, opt := range out.Pending.Options {
		kinds = append(kinds, opt.Kind)
	}
	s.Equal([]session.OptionKind{session.OptionContinue, session.OptionStop}, kinds)
}

// TestSeeingNothingNewDoesNotStopTheWalk is the negative control.
//
// Same world, same path, same ogre — but alice already holds it, because a
// first walk revealed it and a second cannot reveal it again. The walk runs to
// the end. Without this, every assertion above is equally consistent with "any
// walk near a monster stops", which is not the rule and would be a much worse
// game.
func (s *SuspendTestSuite) TestSeeingNothingNewDoesNotStopTheWalk() {
	s.startAmbush()
	ctx := context.Background()

	first := s.walkIntoTheAmbush()
	s.Require().NotNil(first.Pending, "precondition: the first sighting suspends")
	_, err := s.mgr.Answer(ctx, &session.AnswerInput{
		Session: "sess", Window: first.Pending.Window,
		Member: "alice", Option: string(session.OptionStop),
	})
	s.Require().NoError(err)

	// She is at (2,3) holding the ogre. Walking on cannot be first contact.
	out, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 4}, {X: 2, Y: 5}},
	})
	s.Require().NoError(err)
	s.Nil(out.Pending, "a subject already known is not a discovery")
	s.Len(out.Steps, 2, "so the whole path is walked")
}

// TestNoWindowOnTheFinalCell pins that a checkpoint with nothing left to decide
// is not opened.
//
// If the last cell of a path reveals something, continuing and stopping are the
// same act. Freezing the world to offer a choice between two identical
// outcomes would be a pause the player cannot make wrong and cannot skip.
func (s *SuspendTestSuite) TestNoWindowOnTheFinalCell() {
	s.startAmbush()

	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 2}, {X: 2, Y: 3}}, // sighting lands on the last cell
	})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, 2, "the walk finished")
	s.Nil(out.Pending, "and nothing was left to ask about")

	seen, err := s.mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Len(seen, 1, "she did see the ogre — the sighting is real, the window is not")
	s.Equal("ogre", seen[0].Subject)
}

// TestSightedIsSortedRatherThanIterationOrdered pins C8 at this seam.
//
// Two subjects revealed by the same step must be enumerated in an order that is
// a function of persisted data. Iterating a map would pass this test most of
// the time, which is exactly why it is asserted rather than assumed: a resumed
// resolution that enumerated differently would break "identical inputs yield
// identical outputs" only under load, only in production.
func (s *SuspendTestSuite) TestSightedIsSortedRatherThanIterationOrdered() {
	// "aardvark" sorts before "ogre"; declared after it, so insertion order and
	// sorted order disagree.
	s.startAmbush(encounter.MemberInput{
		ID: "aardvark", Kind: encounter.KindMonster,
		Room: "hall", Position: spatial.Position{X: 5, Y: 3},
	})

	out := s.walkIntoTheAmbush()
	s.Require().NotNil(out.Pending)
	s.Equal([]string{"aardvark", "ogre"}, out.Pending.Prompt.Sighted,
		"enumeration is sorted, not whatever order the composition reported")
}

// TestContinueResumesFromWhereItStopped is scene 3's second half.
func (s *SuspendTestSuite) TestContinueResumesFromWhereItStopped() {
	s.startAmbush()
	ctx := context.Background()
	out := s.walkIntoTheAmbush()
	s.Require().NotNil(out.Pending)

	resumed, err := s.mgr.Answer(ctx, &session.AnswerInput{
		Session: "sess", Window: out.Pending.Window,
		Member: "alice", Option: string(session.OptionContinue),
	})
	s.Require().NoError(err)
	s.Require().Len(resumed.Steps, 1, "exactly the cell that was left")
	s.Equal(spatial.Position{X: 2, Y: 4}, resumed.Steps[0].Position)
	s.Nil(resumed.Pending, "and the window is closed")

	s.Greater(resumed.Steps[0].Seq, out.Steps[1].Seq,
		"the story continues rather than restarting")
}

// TestStopAbandonsTheRemainderWithoutUndoing pins that stopping is not rewinding.
//
// The two steps she took are not taken back. A player who spots an ogre and
// halts is standing where she halted, not back where she started — anything
// else would make the pause itself change the world.
func (s *SuspendTestSuite) TestStopAbandonsTheRemainderWithoutUndoing() {
	s.startAmbush()
	ctx := context.Background()
	out := s.walkIntoTheAmbush()
	s.Require().NotNil(out.Pending)

	stopped, err := s.mgr.Answer(ctx, &session.AnswerInput{
		Session: "sess", Window: out.Pending.Window,
		Member: "alice", Option: string(session.OptionStop),
	})
	s.Require().NoError(err)
	s.Empty(stopped.Steps, "nothing further was walked")

	// And the world runs again: a verb that would change it now succeeds.
	moved, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 2}},
	})
	s.Require().NoError(err, "answering unfroze the world")
	s.Require().Len(moved.Steps, 1)
	s.Equal(spatial.Position{X: 2, Y: 2}, moved.Steps[0].Position,
		"she resumed from (2,3), which is where she stopped")
}

// TestEveryChangeVerbIsRefusedWhileFrozen is T2.2, asserted over the whole
// surface rather than the one verb that happens to have suspended.
//
// The table is the point: a future verb that forgets the freeze is only caught
// if the rule is stated once for all of them.
func (s *SuspendTestSuite) TestEveryChangeVerbIsRefusedWhileFrozen() {
	ctx := context.Background()

	attempts := map[string]func(*session.Manager) error{
		"Move": func(m *session.Manager) error {
			_, err := m.Move(ctx, &session.MoveInput{
				Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 4}},
			})
			return err
		},
		"Traverse": func(m *session.Manager) error {
			_, err := m.Traverse(ctx, &session.TraverseInput{
				Session: "sess", Member: "alice", Connection: "anything",
			})
			return err
		},
		"Join": func(m *session.Manager) error {
			_, err := m.Join(ctx, &session.JoinInput{
				Session: "sess", Member: "bob", Kind: session.KindPlayer,
				Room: "hall", Position: spatial.Position{X: 0, Y: 0},
			})
			return err
		},
		"Exit": func(m *session.Manager) error {
			_, err := m.Exit(ctx, &session.ExitInput{Session: "sess", Member: "alice"})
			return err
		},
		"End": func(m *session.Manager) error {
			_, err := m.End(ctx, &session.EndInput{Session: "sess", Ending: "stairs"})
			return err
		},
	}

	for name, attempt := range attempts {
		s.Run(name, func() {
			s.startAmbush()
			out := s.walkIntoTheAmbush()
			s.Require().NotNil(out.Pending, "precondition: the world is frozen")

			err := attempt(s.mgr)
			s.Require().Error(err, "%s changes the world, so it must be refused", name)
			s.Require().ErrorIs(err, session.ErrFrozen)

			var frozen *session.FrozenError
			s.Require().ErrorAs(err, &frozen,
				"the rejection carries the window, so the caller is not sent hunting for it")
			s.Equal(out.Pending.Window, frozen.Window)
			s.Equal("alice", frozen.Audience)
		})
	}
}

// TestReadVerbsSurviveTheFreeze is the other half of T2.2.
//
// A freeze that blinded everyone would be worse than no freeze: a client cannot
// render "waiting on Alice" without asking what the world looks like. What is
// frozen is change, not observation.
func (s *SuspendTestSuite) TestReadVerbsSurviveTheFreeze() {
	s.startAmbush()
	ctx := context.Background()
	out := s.walkIntoTheAmbush()
	s.Require().NotNil(out.Pending)

	_, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess"})
	s.NoError(err, "Atlas")

	_, err = s.mgr.Status(ctx, &session.StatusInput{Session: "sess"})
	s.NoError(err, "Status")

	_, err = s.mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "alice"})
	s.NoError(err, "View")

	_, err = s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice"})
	s.NoError(err, "Story")

	pending, err := s.mgr.Pending(ctx, &session.PendingInput{Session: "sess"})
	s.Require().NoError(err, "Pending")
	s.Require().Len(pending.Windows, 1, "and it reports the window blocking everything else")
	s.Equal(out.Pending.Window, pending.Windows[0].Window)
	s.Equal([]string{"ogre"}, pending.Windows[0].Prompt.Sighted,
		"including what the audience was shown when it opened")
}

// TestSuspensionSurvivesAProcessRestart is S7, and the reason a frozen
// resolution is data rather than a parked goroutine.
//
// Both stores are marshalled to JSON and read back into fresh repositories
// behind a fresh Manager — everything in memory is gone, exactly as it would be
// if the answer arrived the next morning on another machine. The window is
// still open, still knows what it was waiting on, and still resumes correctly.
func (s *SuspendTestSuite) TestSuspensionSurvivesAProcessRestart() {
	s.startAmbush()
	ctx := context.Background()
	out := s.walkIntoTheAmbush()
	s.Require().NotNil(out.Pending)

	sessions, encounters := s.roundTripStores()
	restarted := managerOver(s.T(), sessions, encounters)

	pending, err := restarted.Pending(ctx, &session.PendingInput{Session: "sess"})
	s.Require().NoError(err)
	s.Require().Len(pending.Windows, 1, "the window outlived the process")
	s.Equal(out.Pending.Window, pending.Windows[0].Window, "and kept its identity")
	s.Equal([]string{"ogre"}, pending.Windows[0].Prompt.Sighted)

	resumed, err := restarted.Answer(ctx, &session.AnswerInput{
		Session: "sess", Window: pending.Windows[0].Window,
		Member: "alice", Option: string(session.OptionContinue),
	})
	s.Require().NoError(err)
	s.Require().Len(resumed.Steps, 1, "and the walk picked up exactly where it stopped")
	s.Equal(spatial.Position{X: 2, Y: 4}, resumed.Steps[0].Position)
}

// roundTripStores marshals both repositories through JSON into fresh ones,
// which is the closest a test can get to "a different process picked this up".
func (s *SuspendTestSuite) roundTripStores() (*fakeSessions, *fakeEncounters) {
	sessions := newFakeSessions()
	for id, data := range s.sessions.byID {
		raw, err := json.Marshal(data)
		s.Require().NoError(err)
		var reloaded session.SessionData
		s.Require().NoError(json.Unmarshal(raw, &reloaded))
		sessions.byID[id] = &reloaded
	}

	encounters := newFakeEncounters()
	for id, data := range s.encounters.byID {
		raw, err := json.Marshal(data)
		s.Require().NoError(err)
		var reloaded encounter.EncounterData
		s.Require().NoError(json.Unmarshal(raw, &reloaded))
		encounters.byID[id] = &reloaded
	}
	return sessions, encounters
}

// TestTheStoryIsOneContinuousSequence is T2.4's transcript claim.
//
// A suspension is invisible in the record: the beats before the freeze and the
// beats after it form one unbroken, ascending sequence. That is what makes S10
// work — a client that reconnects mid-freeze and re-queries Story cannot tell
// from the log that anything stopped.
func (s *SuspendTestSuite) TestTheStoryIsOneContinuousSequence() {
	s.startAmbush()
	ctx := context.Background()
	out := s.walkIntoTheAmbush()
	s.Require().NotNil(out.Pending)

	resumed, err := s.mgr.Answer(ctx, &session.AnswerInput{
		Session: "sess", Window: out.Pending.Window,
		Member: "alice", Option: string(session.OptionContinue),
	})
	s.Require().NoError(err)
	s.Require().Len(resumed.Steps, 1)

	story, err := s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().NotEmpty(story)

	for i := 1; i < len(story); i++ {
		s.Greater(story[i].Seq, story[i-1].Seq, "sequence %d does not advance", i)
	}

	last := story[len(story)-1].Seq
	s.Equal(resumed.Steps[0].Seq, last,
		"the resumed step is the newest beat, in the same sequence as the earlier ones")
}

// positionJSON renders a position the way the frozen payload stores it, without
// a test having to know its field tags.
func positionJSON(t fataler, p spatial.Position) any {
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshalling position: %v", err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshalling position: %v", err)
	}
	return v
}

// tamperFrozenPath rewrites the stored path inside the open window's frozen
// payload, standing in for a blob that does not match the world it refers to.
func (s *SuspendTestSuite) tamperFrozenPath(mutate func(path []any)) {
	stored := s.sessions.byID["sess"]
	s.Require().Len(stored.Windows.Windows, 1, "precondition: one open window")

	var payload map[string]any
	s.Require().NoError(json.Unmarshal(stored.Windows.Windows[0].Payload, &payload))
	path, ok := payload["path"].([]any)
	s.Require().True(ok, "the frozen payload carries the path")

	mutate(path)
	payload["path"] = path

	raw, err := json.Marshal(payload)
	s.Require().NoError(err)
	stored.Windows.Windows[0].Payload = raw
}

func (s *SuspendTestSuite) answerContinue(window string) (*session.AnswerOutput, error) {
	return s.mgr.Answer(context.Background(), &session.AnswerInput{
		Session: "sess", Window: window,
		Member: "alice", Option: string(session.OptionContinue),
	})
}

// TestResumeRejectsAPathThatNoLongerLinesUp pins the re-validation in resume.
//
// A window and a world are two stored records, and nothing guarantees a stored
// window describes the world that actually loaded — a hand-edited blob is
// enough. Resuming on trust would walk a member from where she stands to
// somewhere across the room in one step, because the phase index says there is
// a cell left and the cell says (7,7).
func (s *SuspendTestSuite) TestResumeRejectsAPathThatNoLongerLinesUp() {
	s.startAmbush()
	out := s.walkIntoTheAmbush()
	s.Require().NotNil(out.Pending)

	// The one unwalked cell is moved across the room.
	s.tamperFrozenPath(func(path []any) {
		path[2] = positionJSON(s.T(), spatial.Position{X: 7, Y: 7})
	})

	_, err := s.answerContinue(out.Pending.Window)
	s.Require().Error(err, "a resume that does not line up is refused, not walked")
	s.ErrorIs(err, session.ErrBrokenPath)
}

// TestResumeChecksOnlyWhatIsLeftToWalk is the other half, and the reason the
// check is over a suffix rather than the whole path.
//
// The cells already walked are behind her. Re-validating them against where she
// now stands would reject legitimate resumes for describing a journey that has
// already happened — the walk is driven by the phase index, and the phase index
// is the only part of the path that is still ahead.
func (s *SuspendTestSuite) TestResumeChecksOnlyWhatIsLeftToWalk() {
	s.startAmbush()
	out := s.walkIntoTheAmbush()
	s.Require().NotNil(out.Pending)

	// The already-walked first cell becomes nonsense. It must not matter.
	s.tamperFrozenPath(func(path []any) {
		path[0] = positionJSON(s.T(), spatial.Position{X: 7, Y: 7})
	})

	resumed, err := s.answerContinue(out.Pending.Window)
	s.Require().NoError(err, "the walked prefix is history, not a precondition")
	s.Require().Len(resumed.Steps, 1)
	s.Equal(spatial.Position{X: 2, Y: 4}, resumed.Steps[0].Position)
}

// TestOnlyTheAudienceMayAnswer pins that a window is owed by someone specific.
func (s *SuspendTestSuite) TestOnlyTheAudienceMayAnswer() {
	s.startAmbush()
	ctx := context.Background()
	out := s.walkIntoTheAmbush()
	s.Require().NotNil(out.Pending)

	_, err := s.mgr.Answer(ctx, &session.AnswerInput{
		Session: "sess", Window: out.Pending.Window,
		Member: "ogre", Option: string(session.OptionContinue),
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotAudience)

	pending, err := s.mgr.Pending(ctx, &session.PendingInput{Session: "sess"})
	s.Require().NoError(err)
	s.Len(pending.Windows, 1, "a rejected answer leaves the window open")
}

// TestUnofferedChoiceIsRejected pins that the option list is the menu, not a
// suggestion.
func (s *SuspendTestSuite) TestUnofferedChoiceIsRejected() {
	s.startAmbush()
	ctx := context.Background()
	out := s.walkIntoTheAmbush()
	s.Require().NotNil(out.Pending)

	_, err := s.mgr.Answer(ctx, &session.AnswerInput{
		Session: "sess", Window: out.Pending.Window,
		Member: "alice", Option: "teleport",
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotOffered)
}

// TestOneAnswerPerWindow pins that answering is not idempotent.
//
// A retried answer must not resume a walk twice. The second attempt finds no
// such window, which is the correct answer: the window is gone because it was
// used.
func (s *SuspendTestSuite) TestOneAnswerPerWindow() {
	s.startAmbush()
	ctx := context.Background()
	out := s.walkIntoTheAmbush()
	s.Require().NotNil(out.Pending)

	answer := &session.AnswerInput{
		Session: "sess", Window: out.Pending.Window,
		Member: "alice", Option: string(session.OptionStop),
	}
	_, err := s.mgr.Answer(ctx, answer)
	s.Require().NoError(err)

	_, err = s.mgr.Answer(ctx, answer)
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNoWindow)
}

// TestUnknownWindowIsRejected covers the identifiers a caller can get wrong.
func (s *SuspendTestSuite) TestUnknownWindowIsRejected() {
	ctx := context.Background()

	cases := map[string]struct {
		window string
		expect error
	}{
		"empty":        {"", session.ErrNoWindowID},
		"not a number": {"window-1", session.ErrNoWindowID},
		"zero":         {"0", session.ErrNoWindowID},
		"never opened": {"77", session.ErrNoWindow},
	}
	for name, tc := range cases {
		s.Run(name, func() {
			s.startAmbush()
			_, err := s.mgr.Answer(ctx, &session.AnswerInput{
				Session: "sess", Window: tc.window,
				Member: "alice", Option: string(session.OptionContinue),
			})
			s.Require().Error(err)
			s.ErrorIs(err, tc.expect)
		})
	}
}

// TestAWalkThatSuspendsNothingWritesOnlyTheEncounter pins write proportionality.
//
// Session state did not change, so the session blob is not rewritten. The cost
// of windows existing must be paid by walks that open one, not by every walk.
func (s *SuspendTestSuite) TestAWalkThatSuspendsNothingWritesOnlyTheEncounter() {
	s.startAmbush()

	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 1, Y: 2}},
	})
	s.Require().NoError(err)
	s.Require().Nil(out.Pending, "precondition: nothing suspended")
	s.Equal([]string{"encounter:world"}, out.Saved.Written,
		"the session blob is left alone when session state did not change")
}

// TestASuspendingWalkWritesBothAggregates is the other side of that rule, and
// the first time SaveReport names more than one thing.
func (s *SuspendTestSuite) TestASuspendingWalkWritesBothAggregates() {
	s.startAmbush()
	out := s.walkIntoTheAmbush()

	s.Require().NotNil(out.Pending)
	s.Equal([]string{"encounter:world", "session:sess"}, out.Saved.Written,
		"the world first, then what is owed — the order persist chose deliberately")
	s.Empty(out.Saved.Failed)
}

// TestAPartialSaveTellsTheCallerWhichHalfLanded is S6 reaching a caller, which
// is not the same as S6 being computed.
//
// A verb returns no output when it returns an error, so a report that only
// exists inside persist is a report nobody can act on. This is the first
// genuinely reachable partial save in the module: the encounter is durable and
// the window that was owed is not, and the caller has to be able to tell that
// from "nothing was written" — one is a retry, the other is a repair.
func (s *SuspendTestSuite) TestAPartialSaveTellsTheCallerWhichHalfLanded() {
	s.startAmbush()

	// Arm the session store to fail only now, after the world is in place.
	sessions := &failingSessions{fakeSessions: s.sessions, saveErr: errBroken}
	mgr := managerOverRepos(s.T(), sessions, s.encounters)

	_, err := mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 2}, {X: 2, Y: 3}, {X: 2, Y: 4}},
	})
	s.Require().Error(err, "the walk suspended, so the session had to be written")
	s.Require().ErrorIs(err, session.ErrSaveFailed, "the condition is matchable")
	s.ErrorIs(err, errBroken, "and so is the store's own failure")

	var saved *session.SaveError
	s.Require().ErrorAs(err, &saved, "the report must survive the error, not die in persist")
	s.Equal([]string{"encounter:world"}, saved.Report.Written,
		"the world landed — the steps she took are real")
	s.Equal([]string{"session:sess"}, saved.Report.Failed,
		"the window she owes did not — this is a repair, not a retry")
}

// TestATotalSaveFailureNamesOnlyWhatWasAttempted is the contrast that gives the
// test above its meaning: a report naming one failure and nothing written is a
// different situation from one naming a success and a failure.
func (s *SuspendTestSuite) TestATotalSaveFailureNamesOnlyWhatWasAttempted() {
	s.startAmbush()

	encounters := &failingEncounters{fakeEncounters: s.encounters, saveErr: errBroken}
	mgr := managerOverRepos(s.T(), s.sessions, encounters)

	_, err := mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 2}},
	})
	s.Require().Error(err)

	var saved *session.SaveError
	s.Require().ErrorAs(err, &saved)
	s.Empty(saved.Report.Written, "nothing landed")
	s.Equal([]string{"encounter:world"}, saved.Report.Failed)
}

// TestHostileSessionBlobIsRejectedNotCrashed pins reject-never-crash on the new
// persisted surface.
//
// A ledger that could not have been written by this module is refused. The
// alternative — repairing it into something plausible — would resume a
// resolution that never happened, moving a member through cells on the strength
// of a guess.
func (s *SuspendTestSuite) TestHostileSessionBlobIsRejectedNotCrashed() {
	s.startAmbush()
	ctx := context.Background()
	s.Require().NotNil(s.walkIntoTheAmbush().Pending)

	// A window claiming an ID at or above next_id cannot have been issued.
	stored := s.sessions.byID["sess"]
	stored.Windows.Windows[0].ID = 9999

	_, err := s.mgr.Pending(ctx, &session.PendingInput{Session: "sess"})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrInvalidSession)

	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 2, Y: 4}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrInvalidSession, "and every verb refuses the same way")
}

// TestFrozenErrorMatchesBothWays pins the error's contract: the sentinel for
// detecting the condition, the type for acting on it.
func (s *SuspendTestSuite) TestFrozenErrorMatchesBothWays() {
	frozen := &session.FrozenError{Window: "7", Audience: "alice"}

	s.True(errors.Is(frozen, session.ErrFrozen), "errors.Is finds the condition")

	var recovered *session.FrozenError
	s.True(errors.As(error(frozen), &recovered), "errors.As recovers the detail")
	s.Equal("7", recovered.Window)
	s.Contains(frozen.Error(), "alice", "and the message names who is holding things up")
}

// TestPromptCarriesNoCheckpointReason pins the asymmetry S5 depends on.
//
// The client renders the moment and branches on OptionKind. It is never told
// which kind of checkpoint fired, so new checkpoint kinds — a trap, an offered
// reaction, whatever a later wave invents — arrive without any client learning
// a new reason code. A field named Reason or Kind here would quietly become the
// thing clients switch on, and then it could never change.
func (s *SuspendTestSuite) TestPromptCarriesNoCheckpointReason() {
	s.Equal([]string{"Member", "Room", "At", "Sighted"}, structFields(session.Prompt{}),
		"Prompt describes what the player sees, never why the resolution stopped")
}

func TestSuspendSuite(t *testing.T) {
	suite.Run(t, new(SuspendTestSuite))
}
