// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/stretchr/testify/suite"
)

type knowledgeEncounters struct {
	*fakeEncounters
	reads int
}

func (r *knowledgeEncounters) GetEncounter(ctx context.Context, id string) (*encounter.EncounterData, error) {
	r.reads++
	return r.fakeEncounters.GetEncounter(ctx, id)
}

type KnowledgeSuite struct{ suite.Suite }

func TestKnowledgeSuite(t *testing.T) { suite.Run(t, new(KnowledgeSuite)) }

func (s *KnowledgeSuite) TestSnapshotUsesOneWorldAndTheExactOwnedSeat() {
	fixture := newRosterFixture(s.T())
	// A knows Bob, but has never observed the skeleton. Reading must not turn
	// current world placement into new knowledge.
	delete(fixture.encounters.byID["world"].Perception.Intel.Holdings["member|alice"], "member|skel-1")
	repo := &knowledgeEncounters{fakeEncounters: fixture.encounters}
	mgr, err := session.NewManager(&session.Config{Sessions: fixture.sessions, Encounters: repo, Characters: fixture.characters,
		Events: session.DiscardEvents{}, Dice: testDice{}, TurnDriver: session.Pass{}, PresentationIDs: testPresentationIDs{}})
	s.Require().NoError(err)
	ctx := context.Background()
	snapshot, err := mgr.Knowledge(ctx, &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"})
	s.Require().NoError(err)
	s.Equal(1, repo.reads, "all snapshot sections use one loaded world")
	var identities []string
	for _, member := range snapshot.Roster.Members {
		identities = append(identities, member.ID)
	}
	s.Equal([]string{"alice", "bob"}, identities)
	for _, sighting := range snapshot.View.Sightings {
		s.NotEqual("skel-1", sighting.Subject)
	}
	s.Empty(snapshot.Holding)
	story, err := mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().NotEmpty(story)
	s.Equal(story[len(story)-1].Seq, snapshot.Seq)
	s.Zero(fixture.sessions.saves)
	s.Zero(fixture.encounters.saves)
	denied, err := mgr.Knowledge(ctx, &session.KnowledgeInput{Session: "sess", Member: "bob", Player: "player-alice"})
	s.ErrorIs(err, session.ErrNotSeated)
	s.Nil(denied)
}

func (s *KnowledgeSuite) TestRoomRevealAndObservationsSurviveTheSessionPath() {
	ctx := context.Background()
	stream := &fakeStream{}
	mgr, err := session.NewManager(&session.Config{
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: newFakeCharacters(armedFighter("alice")),
		Events: stream, Dice: testDice{}, TurnDriver: session.Pass{}, PresentationIDs: testPresentationIDs{},
	})
	s.Require().NoError(err)
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: gatedWorld(s.T(), encounter.DoorIsClosed())})
	s.Require().NoError(err)
	input := &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"}
	before, err := mgr.Knowledge(ctx, input)
	s.Require().NoError(err)
	s.Require().Len(before.Atlas.Regions, 1)
	s.Equal("corridor", before.Atlas.Regions[0].ID)
	s.Require().Len(before.View.Doors, 1)
	s.Equal("closed", before.View.Doors[0].Door.State)
	stream.published = nil
	_, err = mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "alice", Door: "gate"})
	s.Require().NoError(err)
	after, err := mgr.Knowledge(ctx, input)
	s.Require().NoError(err)
	s.Len(after.Atlas.Regions, 2)
	s.Equal("open", after.View.Doors[0].Door.State)
	var reveals int
	for _, event := range stream.published {
		if event.Kind == session.EventRoomRevealed {
			body, ok := event.Body.(session.RoomRevealedBody)
			s.Require().True(ok)
			s.Equal("vault", body.Region.ID)
			s.Len(body.Region.Cells, 36)
			s.Empty(body.Props)
			reveals++
		}
	}
	s.Equal(1, reveals)
	replay, err := mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice", FromSeq: before.Seq + 1})
	s.Require().NoError(err)
	s.Equal(eventsFor(stream.published, "alice"), replay, "live and replay project the same stored room payload")
	s.Equal(replay[len(replay)-1].Seq, after.Seq)
}
