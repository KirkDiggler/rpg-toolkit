// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
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

func (s *KnowledgeSuite) TestCapturedPresentationCrossesKnowledgeAndViewWithoutAliases() {
	no := false
	sc := scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("room", 0, 0, 3, 2)},
			Props: []encounter.PropInput{{ID: "book", Ref: "test:props:book", At: spatial.Position{X: 1}, Holdable: true, BlocksMovement: &no, BlocksLineOfSight: &no}},
			PropPresentations: []encounter.PropPresentation{{ID: "book", Ref: "test:props:book", Origin: spatial.Point{X: 5}, HeightScale: 1.5,
				PointLight: &encounter.PropPointLight{Color: "#abcdef", Range: 4}}}},
		Party:   []sceneSeat{seatAt("alice", 0, 0)},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	}
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(), Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: newFakeCharacters(armedFighter("alice")),
		Dice: testDice{}, TurnDriver: session.Pass{}, PresentationIDs: testPresentationIDs{}})
	s.Require().NoError(err)
	ctx := context.Background()
	launchScene(s.T(), mgr, sc)
	input := &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"}
	known, err := mgr.Knowledge(ctx, input)
	s.Require().NoError(err)
	s.Require().Len(known.View.Props, 1)
	s.Require().NotNil(known.View.Props[0].Presentation)
	s.Equal("book", known.View.Props[0].Presentation.ID)
	s.Empty(known.Atlas.PropPresentations)
	known.View.Props[0].Presentation.PointLight.Range = 999
	view, err := mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(4.0, view.Props[0].Presentation.PointLight.Range)
	_, err = mgr.Hold(ctx, &session.HoldInput{Session: "sess", Member: "alice", Target: "book"})
	s.Require().NoError(err)
	empty, err := mgr.Knowledge(ctx, input)
	s.Require().NoError(err)
	s.Require().Len(empty.View.Props, 1)
	s.True(empty.View.Props[0].ObservedEmpty)
	s.Nil(empty.View.Props[0].Presentation)
}

func (s *KnowledgeSuite) TestSnapshotUsesOneWorldAndTheExactOwnedSeat() {
	fixture := newRosterFixture(s.T())
	// A knows Bob, but has never observed the skeleton. Reading must not turn
	// current world placement into new knowledge.
	delete(fixture.encounters.byID["world"].Perception.Intel.Holdings["member|alice"], "member|skel-1")
	repo := &knowledgeEncounters{fakeEncounters: fixture.encounters}
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(), Sessions: fixture.sessions, Encounters: repo, Characters: fixture.characters,
		Dice: testDice{}, TurnDriver: session.Pass{}, PresentationIDs: testPresentationIDs{}})
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
	s.Zero(fixture.sessions.saves)
	s.Zero(fixture.encounters.saves)
	denied, err := mgr.Knowledge(ctx, &session.KnowledgeInput{Session: "sess", Member: "bob", Player: "player-alice"})
	s.ErrorIs(err, session.ErrNotSeated)
	s.Nil(denied)
}

// TestCellsRevealAndObservationsSurviveTheSessionPath: opening the gate
// teaches alice vault cells (rpg-project#586 R1), delivered as cells-revealed
// perceptions on her act stream, carrying only cells she newly learned.
// Applying them to her prior atlas lands exactly on the fresh Knowledge atlas,
// and no whole-room beat is produced.
func (s *KnowledgeSuite) TestCellsRevealAndObservationsSurviveTheSessionPath() {
	ctx := context.Background()
	stream := &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: stream, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(),
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: newFakeCharacters(armedFighter("alice")),
		Dice: testDice{}, TurnDriver: session.Pass{}, PresentationIDs: testPresentationIDs{},
	})
	s.Require().NoError(err)
	launchScene(s.T(), mgr, gatedWorld(encounter.DoorIsClosed()))
	input := &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"}
	before, err := mgr.Knowledge(ctx, input)
	s.Require().NoError(err)
	s.Require().Len(before.Atlas.Regions, 1)
	s.Equal("corridor", before.Atlas.Regions[0].ID)
	s.Require().Len(before.View.Doors, 1)
	s.Equal("closed", before.View.Doors[0].Door.State)
	stream.published = nil
	mark := lastSeq(storyBeats(s.T(), mgr, "sess", "alice"))
	_, err = mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "alice", Door: "gate"})
	s.Require().NoError(err)
	after, err := mgr.Knowledge(ctx, input)
	s.Require().NoError(err)
	s.Require().Len(after.Atlas.Regions, 2)
	s.Equal("open", after.View.Doors[0].Door.State)

	patched := map[string]map[spatial.Position]bool{}
	for _, region := range before.Atlas.Regions {
		patched[region.ID] = map[spatial.Position]bool{}
		for _, cell := range region.Cells {
			patched[region.ID][cell] = true
		}
	}
	var reveals int
	s.Empty(beatsNamed(storyBeatsAfter(s.T(), mgr, "sess", "alice", mark), "room_revealed"),
		"the whole-room beat is no longer produced")
	for _, act := range stream.published {
		perceived, ok := act.Body.(session.Perception)
		if !ok || perceived.Sense != session.SenseCells {
			continue
		}
		s.Equal("alice", act.Recipient, "a cells reveal is its learner's alone")
		body := perceived.Cells
		s.Require().NotNil(body)
		s.Empty(body.Props)
		for _, region := range body.Regions {
			if patched[region.ID] == nil {
				patched[region.ID] = map[spatial.Position]bool{}
			}
			for _, cell := range region.Cells {
				s.False(patched[region.ID][cell], "a reveal carries only newly known cells")
				patched[region.ID][cell] = true
			}
		}
		reveals++
	}
	s.Positive(reveals, "opening the gate teaches alice vault cells")
	for _, region := range after.Atlas.Regions {
		got := make([]spatial.Position, 0, len(patched[region.ID]))
		for cell := range patched[region.ID] {
			got = append(got, cell)
		}
		s.ElementsMatch(region.Cells, got, "prior atlas plus reveals is the fresh atlas for %q", region.ID)
	}
	s.Subset(after.Atlas.Cells, after.View.VisibleCells, "seen now is a subset of what she knows")
}
