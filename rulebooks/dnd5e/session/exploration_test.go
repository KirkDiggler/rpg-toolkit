// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
	"testing"
)

type explorationStore struct {
	data map[string]*session.ExplorationData
}

func (r *explorationStore) GetExploration(_ context.Context, id string) (*session.ExplorationData, error) {
	if value := r.data[id]; value != nil {
		return copyOf(value)
	}
	return nil, session.ErrNotFound
}
func (r *explorationStore) SaveExploration(_ context.Context, in *session.ExplorationData) error {
	value, err := copyOf(in)
	if err != nil {
		return err
	}
	r.data[in.Character] = value
	return nil
}

type AutomaticDiscoverySDKSuite struct{ suite.Suite }

func TestAutomaticDiscoverySDKSuite(t *testing.T) { suite.Run(t, new(AutomaticDiscoverySDKSuite)) }
func (s *AutomaticDiscoverySDKSuite) TestRealSheetRollPersistsAcrossRunsAndPreferenceReload() {
	ctx := context.Background()
	profiles := &explorationStore{data: map[string]*session.ExplorationData{}}
	sessions, encounters, characters := newFakeSessions(), newFakeEncounters(), testCharacters()
	cfg := &session.Config{Sessions: sessions, Encounters: encounters, Characters: characters, Explorations: profiles, Events: session.DiscardEvents{}, Dice: testDice{}, PresentationIDs: testPresentationIDs{}, TurnDriver: session.Pass{}}
	mgr, err := session.NewManager(cfg)
	s.Require().NoError(err)
	world := authoredWorld(s.T())
	world.Field.Concealments = []encounter.ConcealmentData{{ID: "hall/secret", Checks: []encounter.CheckApproachData{{Ability: "religion", DC: 99}}, Cells: []encounter.PositionData{{X: 3, Y: 1}}}}
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "run", Encounter: "enc", World: world})
	s.Require().NoError(err)
	_, err = mgr.SetDiscoverySharing(ctx, &session.SetDiscoverySharingInput{Session: "run", Member: "alice", Sharing: false})
	s.Require().NoError(err)
	_, err = mgr.Move(ctx, &session.MoveInput{Session: "run", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}}})
	s.Require().NoError(err)
	story, err := mgr.Story(ctx, &session.StoryInput{Session: "run", Member: "alice"})
	s.Require().NoError(err)
	checks := 0
	for _, event := range story {
		if event.Kind == session.EventDiscoveryChecked {
			checks++
			body, ok := event.Body.(session.DiscoveryCheckedBody)
			s.Require().True(ok)
			s.Equal("religion", body.Ability)
			s.False(body.Beaten)
			s.NotNil(body.Calculation)
		}
	}
	s.Equal(1, checks)
	s.Equal(1, profiles.data["alice"].Checks["hall/secret"].Used)
	s.True(profiles.data["alice"].PrivateDiscoveries)
	mgr, err = session.NewManager(cfg)
	s.Require().NoError(err)
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "second", Encounter: "enc2", World: world})
	s.Require().NoError(err)
	_, err = mgr.Move(ctx, &session.MoveInput{Session: "second", Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}}})
	s.Require().NoError(err)
	story, err = mgr.Story(ctx, &session.StoryInput{Session: "second", Member: "alice"})
	s.Require().NoError(err)
	for _, event := range story {
		s.NotEqual(session.EventDiscoveryChecked, event.Kind)
	}
	s.Equal(1, profiles.data["alice"].Checks["hall/secret"].Used)
	_, err = mgr.Search(ctx, &session.SearchInput{Session: "second", Member: "alice", Region: "hall"})
	s.Error(err, "automatic hosts cannot use the old search path")
}
