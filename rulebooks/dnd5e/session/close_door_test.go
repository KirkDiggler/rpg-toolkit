// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// close_door_test.go covers rpg-project#527: the composition's existing
// CloseDoor verb, exposed through the seam. It is the mirrored half of the door
// suite in doors_test.go, and it is deliberately about EXPOSURE rather than a
// new rule — every closing decision (probe, membership, reach, state) is the
// composition's, and this file proves the seam carries each across intact.
//
// The fixture is a real footprint door standing on one cell: a rectangle, not a
// crossing. It blocks the way onto its own cell while shut and stops blocking
// the instant it is opened, which is what makes "close" a fact a walk and a
// sight read can both observe rather than a string in a response.

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// leafDoorID is the one footprint door every scene here acts on.
const leafDoorID = "leaf"

// footprintLeafWorld is one six-by-six hall with a single footprint door
// standing on hexCell(4, 1) in the caller's state. alice stands one cell west
// of it — within reach — bob one cell east, so a sighting or a refused step
// has somebody to observe, and carol stands far away for the reach refusal.
//
// FOOTPRINT-DOOR OBSERVATION IS O1 AND STILL RED, so this file never asserts a
// distant member's DoorSightings; it asserts what closing CHANGES (a step, a
// sighting), which the canvas answers from the live state alone.
func footprintLeafWorld(t fataler, state encounter.DoorState) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 6)},
			Doors: []encounter.DoorInput{{
				ID:        leafDoorID,
				Placement: aLeafOn(footprintCell()),
				State:     state,
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: cell(3, 1)},
			{ID: "bob", Kind: encounter.KindPlayer, Position: cell(5, 1)},
			{ID: "carol", Kind: encounter.KindPlayer, Position: cell(0, 4)},
		},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sheets:        encStandStill{},
			Sight:         encEveryoneSees{},
			Equipment:     encNoHandsObserved{},
			Initiative:    encOrderAsGiven{},
			Driver:        encPassDriver{},
			Standing:      encEveryoneStanding{},
			CheckResolver: encNeverResolves{},
			Witness:       encNeverWitnesses{},
			Actors: encounter.Actors{
				Striker:   encounter.RefusingStriker{},
				Mover:     encounter.RefusingMover{},
				Announcer: encQuietAnnouncer{},
			},
		},
	})
	if err != nil {
		t.Fatalf("building footprint leaf world: %v", err)
	}
	data := enc.ToData()

	return &data
}

type CloseDoorSuite struct {
	suite.Suite

	stream     *fakeStream
	sessions   *fakeSessions
	encounters *fakeEncounters
	mgr        *session.Manager
}

func TestCloseDoorSuite(t *testing.T) { suite.Run(t, new(CloseDoorSuite)) }

// startWith wires a fresh manager over the given world and leaves the counters
// at zero, so a failure test's "nothing was written" assertion is about the
// operation and not about StartSession's own initial save.
func (s *CloseDoorSuite) startWith(world *encounter.EncounterData) {
	s.stream = &fakeStream{}
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: testCharacters(), Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: world,
	})
	s.Require().NoError(err)

	s.sessions.saves = 0
	s.encounters.saves = 0
	s.stream.published = nil
}

// bobIsCurrentlySeen reports whether alice's view of bob is a LIVE sighting
// rather than a held memory — the seam's own reading of a door that blocks
// sight.
func (s *CloseDoorSuite) bobIsCurrentlySeen() bool {
	s.T().Helper()
	view, err := s.mgr.View(context.Background(), &session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	for _, sighting := range view.Sightings {
		if sighting.Subject == "bob" {
			return sighting.Status == "current"
		}
	}
	return false
}

// walkOntoLeaf walks alice onto the footprint door's own cell and returns the
// walk's outcome. Session Move reports a blocked step as a STOPPED WALK rather
// than a verb error ([MoveOutput]'s own contract), so a door refusal is read off
// the status and reason, not off err.
func (s *CloseDoorSuite) walkOntoLeaf(mgr *session.Manager) *session.MoveOutput {
	s.T().Helper()
	out, err := mgr.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "alice",
		Path: []spatial.Position{footprintCell()}})
	s.Require().NoError(err, "a blocked walk is an outcome, not a verb error")
	return out
}

// TestAnOpenFootprintDoorClosesAndBlocksAgain is the headline: a real
// footprint door is opened, crossed, closed, and refuses the crossing once
// more — with sight changing with it, because closing is the same refresh
// running backwards.
func (s *CloseDoorSuite) TestAnOpenFootprintDoorClosesAndBlocksAgain() {
	ctx := context.Background()
	s.startWith(footprintLeafWorld(s.T(), encounter.DoorIsClosed()))

	// SHUT TO BEGIN: the way onto the door's own cell is refused as a DOOR,
	// by this seam's word, and it is a wall to sight.
	shut := s.walkOntoLeaf(s.mgr)
	s.Equal(session.MovementStopped, shut.Status)
	s.Contains(shut.StopReason, "shut", "a shut leaf refuses as a door, not as a nameless placement")
	s.False(s.bobIsCurrentlySeen(), "and it blocks sight while shut")

	// OPEN IT: the existing verb, unchanged.
	opened, err := s.mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "alice", Door: leafDoorID})
	s.Require().NoError(err)
	s.Equal(session.Door{ID: leafDoorID, State: "open"}, opened.Door)

	walked := s.walkOntoLeaf(s.mgr)
	s.Equal(session.MovementCompleted, walked.Status, "an open door offers the crossing it refused")
	s.NotEmpty(walked.Steps)
	s.True(s.bobIsCurrentlySeen(), "and sight crosses it while open")

	// BACK OFF THE CELL so the close is a reach-judged act from beside the
	// door, not from on top of it.
	_, err = s.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice",
		Path: []spatial.Position{hexCell(3, 1)}})
	s.Require().NoError(err)

	// AND CLOSE IT — the verb under test.
	closed, err := s.mgr.CloseDoor(ctx, &session.CloseDoorInput{Session: "sess", Member: "alice", Door: leafDoorID})
	s.Require().NoError(err)
	s.Equal(session.Door{ID: leafDoorID, State: "closed"}, closed.Door)
	s.NotEmpty(closed.Saved.Written, "the closed world was saved")
	s.Contains(closed.Saved.Written, "encounter:world", "the world carries the new state")
	s.Require().Contains(closed.Discovered["alice"].Faded, "bob", "the close result carries Alice's lost sighting")
	s.Require().Contains(closed.Discovered["bob"].Faded, "alice", "the close result carries Bob's lost sighting")

	shutAgain := s.walkOntoLeaf(s.mgr)
	s.Equal(session.MovementStopped, shutAgain.Status, "the crossing is refused again")
	s.Contains(shutAgain.StopReason, "shut")
	s.False(s.bobIsCurrentlySeen(), "and sight is blocked again")
}

// TestAClosedDoorSurvivesARepositoryReload pins that the state the verb wrote
// is repository truth, not a value the verb kept: a fresh manager over the same
// repositories loads the shut door and still refuses the crossing.
func (s *CloseDoorSuite) TestAClosedDoorSurvivesARepositoryReload() {
	ctx := context.Background()
	s.startWith(footprintLeafWorld(s.T(), encounter.DoorIsClosed()))
	_, err := s.mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "alice", Door: leafDoorID})
	s.Require().NoError(err)
	_, err = s.mgr.CloseDoor(ctx, &session.CloseDoorInput{Session: "sess", Member: "alice", Door: leafDoorID})
	s.Require().NoError(err)

	reloaded, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: testCharacters(), Events: &fakeStream{},
	})
	s.Require().NoError(err)

	reloadedOut, err := reloaded.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice",
		Path: []spatial.Position{footprintCell()}})
	s.Require().NoError(err)
	s.Equal(session.MovementStopped, reloadedOut.Status, "the reloaded shut door still refuses the way through")
	s.Contains(reloadedOut.StopReason, "shut")
}

// TestTheCloseBeatNamesTheActorAndState is the delivery half: the existing
// door beat reaches the actor typed, with the actor and the new state, and the
// output's Seq is that beat's own number in the actor's dense stream.
func (s *CloseDoorSuite) TestTheCloseBeatNamesTheActorAndState() {
	ctx := context.Background()
	s.startWith(footprintLeafWorld(s.T(), encounter.DoorIsClosed()))
	_, err := s.mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "alice", Door: leafDoorID})
	s.Require().NoError(err)
	s.stream.published = nil

	out, err := s.mgr.CloseDoor(ctx, &session.CloseDoorInput{Session: "sess", Member: "alice", Door: leafDoorID})
	s.Require().NoError(err)

	var beats []session.Event
	for _, event := range s.stream.published {
		if event.Kind == session.EventDoor && event.Recipient == "alice" {
			body, ok := event.Body.(session.DoorBody)
			s.Require().True(ok, "a door event carries its typed body")
			s.Equal(session.DoorBody{Door: leafDoorID, State: "closed", Actor: "alice"}, body,
				"the beat names the door, the new state, and whose hands")
			s.Nil(body.Calculation, "closing rolls nothing, and says so")
			beats = append(beats, event)
		}
	}
	s.Require().Len(beats, 1, "one door beat for the close")
	s.Equal(out.Seq, beats[0].Seq, "the output's Seq is the actor's own number for that beat")
}

// Closing changes door state, not the known room or its floor. Structural-wall
// integration coverage remains on the separate structural-layout branch.
func (s *CloseDoorSuite) TestTheKnownRoomAndFloorAreUntouched() {
	ctx := context.Background()
	s.startWith(footprintLeafWorld(s.T(), encounter.DoorIsOpen()))
	before, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().NotEmpty(before.Cells)
	s.Require().NotEmpty(before.Regions)

	_, err = s.mgr.CloseDoor(ctx, &session.CloseDoorInput{Session: "sess", Member: "alice", Door: leafDoorID})
	s.Require().NoError(err)
	after, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(before.Cells, after.Cells)
	s.Equal(before.Regions, after.Regions)
}

// TestCloseDoorRefusalsCarryNoWrite is the failure half: every refusal the
// composition already owns crosses the seam and writes nothing.
func (s *CloseDoorSuite) TestCloseDoorRefusalsCarryNoWrite() {
	ctx := context.Background()

	cases := []struct {
		name  string
		world func(t fataler) *encounter.EncounterData
		in    *session.CloseDoorInput
		want  error
	}{
		{
			name:  "no member",
			world: func(t fataler) *encounter.EncounterData { return footprintLeafWorld(t, encounter.DoorIsClosed()) },
			in:    &session.CloseDoorInput{Session: "sess", Door: leafDoorID},
			want:  session.ErrNoMemberID,
		},
		{
			name:  "unknown member",
			world: func(t fataler) *encounter.EncounterData { return footprintLeafWorld(t, encounter.DoorIsOpen()) },
			in:    &session.CloseDoorInput{Session: "sess", Member: "stranger", Door: leafDoorID},
			want:  session.ErrNoMember,
		},
		{
			name:  "no session",
			world: func(t fataler) *encounter.EncounterData { return footprintLeafWorld(t, encounter.DoorIsClosed()) },
			in:    &session.CloseDoorInput{Member: "alice", Door: leafDoorID},
			want:  session.ErrNoSessionID,
		},
		{
			name:  "no door",
			world: func(t fataler) *encounter.EncounterData { return footprintLeafWorld(t, encounter.DoorIsClosed()) },
			in:    &session.CloseDoorInput{Session: "sess", Member: "alice"},
			want:  session.ErrNoConnection,
		},
		{
			name:  "already closed",
			world: func(t fataler) *encounter.EncounterData { return footprintLeafWorld(t, encounter.DoorIsClosed()) },
			in:    &session.CloseDoorInput{Session: "sess", Member: "alice", Door: leafDoorID},
			want:  session.ErrNoConnection,
		},
		{
			name:  "locked",
			world: func(t fataler) *encounter.EncounterData { return footprintLeafWorld(t, tombLock()) },
			in:    &session.CloseDoorInput{Session: "sess", Member: "alice", Door: leafDoorID},
			want:  session.ErrNoConnection,
		},
		{
			name:  "unknown door",
			world: func(t fataler) *encounter.EncounterData { return footprintLeafWorld(t, encounter.DoorIsOpen()) },
			in:    &session.CloseDoorInput{Session: "sess", Member: "alice", Door: "no-such-door"},
			want:  session.ErrNoConnection,
		},
		{
			name:  "out of reach",
			world: func(t fataler) *encounter.EncounterData { return footprintLeafWorld(t, encounter.DoorIsOpen()) },
			in:    &session.CloseDoorInput{Session: "sess", Member: "carol", Door: leafDoorID},
			want:  session.ErrOutOfRange,
		},
		{
			name:  "unfound concealed door",
			world: hiddenLeafWorld,
			in:    &session.CloseDoorInput{Session: "sess", Member: "alice", Door: leafDoorID},
			want:  session.ErrNoConnection,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.startWith(tc.world(s.T()))
			_, err := s.mgr.CloseDoor(ctx, tc.in)
			s.Require().ErrorIs(err, tc.want)
			s.Zero(s.sessions.saves, "a refusal does not save the session")
			s.Zero(s.encounters.saves, "nor the encounter")
			s.Empty(s.stream.published, "and a refusal is not a beat")
		})
	}

	s.Run("nil input", func() {
		s.startWith(footprintLeafWorld(s.T(), encounter.DoorIsClosed()))
		_, err := s.mgr.CloseDoor(ctx, nil)
		s.Require().ErrorIs(err, session.ErrNilInput)
		s.Zero(s.sessions.saves)
		s.Zero(s.encounters.saves)
		s.Empty(s.stream.published)
	})
}

// TestCloseDoorRunsUnderTheSessionLock is the coverage half, exercised rather
// than inferred: the operation's first repository read must already hold the
// host's guard, and the guard is released however the call ends.
func (s *CloseDoorSuite) TestCloseDoorRunsUnderTheSessionLock() {
	ctx := context.Background()
	s.startWith(footprintLeafWorld(s.T(), encounter.DoorIsOpen()))

	locker := &observingLocker{}
	accesses := 0
	probe := func() {
		accesses++
		s.True(locker.held, "repository and delivery operations must hold the session guard")
	}
	stream := &guardedStream{probe: probe}
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions:   guardedSessions{SessionRepository: s.sessions, probe: probe},
		Encounters: guardedEncounters{EncounterRepository: s.encounters, probe: probe},
		Characters: testCharacters(), Events: stream, Locker: locker,
	})
	s.Require().NoError(err)

	_, err = mgr.CloseDoor(ctx, &session.CloseDoorInput{Session: "sess", Member: "alice", Door: leafDoorID})
	s.Require().NoError(err)
	s.Equal([]string{"sess"}, locker.calls)
	s.False(locker.held, "the guard is released after the operation")
	s.Equal(1, locker.releases)
	s.Positive(accesses)
	s.Positive(stream.published)

	_, err = mgr.CloseDoor(ctx, &session.CloseDoorInput{Session: "sess", Member: "alice", Door: leafDoorID})
	s.Require().ErrorIs(err, session.ErrNoConnection)
	s.False(locker.held, "the guard is also released after a refusal")
	s.Equal(2, locker.releases)
}

// TestClosingADoorTellsNobodyToLookAgain pins how closing meets condition
// freshness (rpg-project#520, R19): the commit runs the freshness step, and a
// close changes no member's conditions, so the only sight beats it sends are
// the sightings it takes away — never a "changed" beat telling an observer to
// look again at somebody it can still see.
func (s *CloseDoorSuite) TestClosingADoorTellsNobodyToLookAgain() {
	s.startWith(footprintLeafWorld(s.T(), encounter.DoorIsOpen()))
	s.Require().True(s.bobIsCurrentlySeen(), "precondition: the open door lets alice see bob")

	_, err := s.mgr.CloseDoor(context.Background(),
		&session.CloseDoorInput{Session: "sess", Member: "alice", Door: leafDoorID})
	s.Require().NoError(err)

	lost := false
	for _, event := range s.stream.published {
		body, ok := event.Body.(session.SightedBody)
		if !ok {
			continue
		}
		s.Empty(body.Changed, "nobody's conditions changed: %+v to %s", body, event.Recipient)
		if event.Recipient == "alice" && slices.Contains(body.Lost, "bob") {
			lost = true
		}
	}
	s.True(lost, "the close's own sight beat still reaches alice: bob left her view")
}
