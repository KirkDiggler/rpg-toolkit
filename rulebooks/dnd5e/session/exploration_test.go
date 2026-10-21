// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type explorationStore struct {
	data map[string]*session.ExplorationData
}

// newFakeExplorations is the one in-memory exploration repository every test
// Config wires: Config.Explorations is required, so no manager is built
// without one.
func newFakeExplorations() *explorationStore {
	return &explorationStore{data: map[string]*session.ExplorationData{}}
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

const runSecret = "same-dungeon/secret"

func discoveryRunWorld(dc int) scene {
	field := encounter.FieldInput{
		Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 6)},
		Concealments: []encounter.ConcealmentInput{{
			ID: runSecret, Checks: []encounter.CheckApproach{{Ability: "religion", DC: dc}},
			Cells: []spatial.Position{{X: 3, Y: 1}},
		}},
	}

	return scene{
		Field:   field,
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	}
}

func (s *AutomaticDiscoverySDKSuite) checkCount(mgr *session.Manager, run string) int {
	story, err := mgr.Story(context.Background(), &session.StoryInput{Session: run, Member: "alice"})
	s.Require().NoError(err)
	count := 0
	for _, event := range story {
		if event.Kind == session.EventDiscoveryChecked {
			count++
			body := event.Body.(session.DiscoveryCheckedBody)
			s.Equal("religion", body.Ability)
			s.NotNil(body.Calculation)
		}
	}
	return count
}

func (s *AutomaticDiscoverySDKSuite) TestLegacyCharacterChecksCannotSeedANewEncounter() {
	ctx := context.Background()
	var legacy session.ExplorationData
	s.Require().NoError(json.Unmarshal([]byte(`{"character":"alice","private":true,"checks":{"same-dungeon/secret":{"used":9,"learned":true}}}`), &legacy))
	profiles := &explorationStore{data: map[string]*session.ExplorationData{"alice": &legacy}}
	encounters := newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Seats: newFakeSeats(),
		Sessions: newFakeSessions(), Encounters: encounters, Characters: testCharacters(),
		Explorations: profiles, Events: session.DiscardEvents{}, Dice: testDice{},
		PresentationIDs: testPresentationIDs{}, TurnDriver: session.Pass{},
	})
	s.Require().NoError(err)
	sc := discoveryRunWorld(99)
	sc.Session = "fresh"
	sc.Party = []sceneSeat{seatAt("alice", 0, 1)}
	launchScene(s.T(), mgr, sc)
	stored, err := encounters.GetEncounter(ctx, "fresh")
	s.Require().NoError(err)
	s.Empty(stored.Discovery["alice"].Attempts)
	s.True(stored.Discovery["alice"].Private)
	atlas, err := mgr.Atlas(ctx, &session.AtlasInput{Session: "fresh", Member: "alice"})
	s.Require().NoError(err)
	s.NotContains(atlas.Cells, spatial.Position{X: 3, Y: 1})
}

func (s *AutomaticDiscoverySDKSuite) TestDiscoveryBelongsToTheEncounterNotTheCharacterProfile() {
	for _, tc := range []struct {
		name    string
		dc      int
		learned bool
	}{
		{name: "failed attempt", dc: 99},
		{name: "learned secret", dc: 1, learned: true},
	} {
		s.Run(tc.name, func() {
			ctx := context.Background()
			profiles := newFakeExplorations()
			sessions, encounters := newFakeSessions(), newFakeEncounters()
			cfg := &session.Config{Record: session.DiscardRecord{}, Seats: newFakeSeats(), Sessions: sessions, Encounters: encounters, Characters: testCharacters(),
				Explorations: profiles, Events: session.DiscardEvents{}, Dice: testDice{},
				PresentationIDs: testPresentationIDs{}, TurnDriver: session.Pass{}}
			mgr, err := session.NewManager(cfg)
			s.Require().NoError(err)
			world := discoveryRunWorld(tc.dc)
			start := func(run string, party ...sceneSeat) {
				sc := world
				sc.Session = run
				sc.Party = append([]sceneSeat{seatAt("alice", 0, 1)}, party...)
				in := sceneInput(sc)
				in.DungeonKey = "same-dungeon"
				_, startErr := mgr.Launch(ctx, in)
				s.Require().NoError(startErr)
			}
			approach := func(run string) {
				_, moveErr := mgr.Move(ctx, &session.MoveInput{Session: run, Member: "alice", Path: []spatial.Position{{X: 1, Y: 1}, {X: 2, Y: 1}}})
				s.Require().NoError(moveErr)
			}
			// A second party member (bob, at axial (0,4)) keeps the encounter
			// open during exit/rejoin.
			start("first", seatAt("bob", 2, 4))
			_, err = mgr.SetDiscoverySharing(ctx, &session.SetDiscoverySharingInput{Session: "first", Member: "alice", Sharing: false})
			s.Require().NoError(err)
			approach("first")
			s.Equal(1, s.checkCount(mgr, "first"))
			saved, err := encounters.GetEncounter(ctx, "first")
			s.Require().NoError(err)
			s.Equal(1, saved.Discovery["alice"].Attempts[runSecret].Used, "attempt state is saved with the encounter")
			atlas, err := mgr.Atlas(ctx, &session.AtlasInput{Session: "first", Member: "alice"})
			s.Require().NoError(err)
			if tc.learned {
				s.Contains(atlas.Cells, spatial.Position{X: 3, Y: 1})
			} else {
				s.NotContains(atlas.Cells, spatial.Position{X: 3, Y: 1})
			}

			// A new Manager is a repository reload, not a new playthrough.
			mgr, err = session.NewManager(cfg)
			s.Require().NoError(err)
			_, err = mgr.Move(ctx, &session.MoveInput{Session: "first", Member: "alice", Path: []spatial.Position{{X: 1, Y: 1}, {X: 0, Y: 1}}})
			s.Require().NoError(err)
			approach("first")
			s.Equal(1, s.checkCount(mgr, "first"), "reload preserves spent attempt/learned knowledge")
			_, err = mgr.Exit(ctx, &session.ExitInput{Session: "first", Member: "alice"})
			s.Require().NoError(err)
			_, err = mgr.Join(ctx, &session.JoinInput{Session: "first", Member: "alice", Position: spatial.Position{X: 0, Y: 1}})
			s.Require().NoError(err)
			approach("first")
			s.Equal(1, s.checkCount(mgr, "first"), "exit/rejoin of the same encounter is not a new playthrough")

			// A character is seated in one run at a time (rpg-project#542, the
			// seat): alice leaves the first run before she enters the second.
			_, err = mgr.Exit(ctx, &session.ExitInput{Session: "first", Member: "alice"})
			s.Require().NoError(err)
			start("second") // SAME template and SAME character, new encounter identity
			fresh, err := encounters.GetEncounter(ctx, "second")
			s.Require().NoError(err)
			s.Empty(fresh.Discovery["alice"].Attempts, "new encounter must not import old attempt history")
			freshAtlas, err := mgr.Atlas(ctx, &session.AtlasInput{Session: "second", Member: "alice"})
			s.Require().NoError(err)
			s.NotContains(freshAtlas.Cells, spatial.Position{X: 3, Y: 1}, "new encounter starts undiscovered")
			approach("second")
			s.Equal(1, s.checkCount(mgr, "second"), "the new playthrough has its own attempt")
			s.Require().NotNil(profiles.data["alice"].DiscoverySharing, "only the sharing preference carries across runs")
			s.False(*profiles.data["alice"].DiscoverySharing, "only the sharing preference carries across runs")
			raw, err := json.Marshal(profiles.data["alice"])
			s.Require().NoError(err)
			s.NotContains(string(raw), "checks", "character profile must not export encounter discoveries")
		})
	}
}

const (
	sharingRun    = "run"
	sharingPlayer = "player-alice"
)

// sharingRig is one manager over shared in-memory repositories, so a second
// manager built from the same Config is a reload of the same saved state.
type sharingRig struct {
	cfg      *session.Config
	profiles *explorationStore
}

func newSharingRig(profiles *explorationStore) *sharingRig {
	return &sharingRig{profiles: profiles, cfg: &session.Config{Record: session.DiscardRecord{}, Seats: newFakeSeats(),
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: testCharacters(),
		Explorations: profiles, Events: session.DiscardEvents{}, Dice: testDice{},
		PresentationIDs: testPresentationIDs{}, TurnDriver: session.Pass{},
	}}
}

func (s *AutomaticDiscoverySDKSuite) launchSharing(rig *sharingRig) *session.Manager {
	mgr, err := session.NewManager(rig.cfg)
	s.Require().NoError(err)
	sc := discoveryRunWorld(99)
	sc.Session = sharingRun
	sc.Party = []sceneSeat{seatAt("alice", 0, 1), seatAt("bob", 2, 4)}
	launchScene(s.T(), mgr, sc)
	return mgr
}

// sharingAfterPrepare reloads the manager and runs a write verb, so the
// reported preference is whatever prepareExploration resolved from the
// repository rather than what the previous verb left on the encounter.
func (s *AutomaticDiscoverySDKSuite) sharingAfterPrepare(rig *sharingRig) bool {
	ctx := context.Background()
	mgr, err := session.NewManager(rig.cfg)
	s.Require().NoError(err)
	knowledge, err := mgr.Knowledge(ctx,
		&session.KnowledgeInput{Session: sharingRun, Member: "alice", Player: sharingPlayer})
	s.Require().NoError(err)
	step := spatial.Position{X: 1, Y: 1}
	if knowledge.Where.Position == step {
		step = spatial.Position{X: 0, Y: 1}
	}
	_, err = mgr.Move(ctx, &session.MoveInput{Session: sharingRun, Member: "alice", Path: []spatial.Position{step}})
	s.Require().NoError(err)
	return s.reportedSharing(mgr)
}

func (s *AutomaticDiscoverySDKSuite) reportedSharing(mgr *session.Manager) bool {
	knowledge, err := mgr.Knowledge(context.Background(),
		&session.KnowledgeInput{Session: sharingRun, Member: "alice", Player: sharingPlayer})
	s.Require().NoError(err)
	s.Require().NotNil(knowledge.DiscoverySharing)
	return *knowledge.DiscoverySharing
}

func (s *AutomaticDiscoverySDKSuite) TestUnchosenSharingResolvesPrivate() {
	rig := newSharingRig(newFakeExplorations())
	mgr := s.launchSharing(rig)
	s.False(s.reportedSharing(mgr), "a character who never chose sees privately")
	s.False(s.sharingAfterPrepare(rig), "a later prepare resolves the same default")
	s.NotContains(rig.profiles.data, "alice", "resolution never writes a default down as a choice")
}

func (s *AutomaticDiscoverySDKSuite) TestLegacyRecordWithoutPrivateStaysShared() {
	for _, tc := range []struct {
		name    string
		legacy  string
		sharing bool
	}{
		{name: "absent private meant shared", legacy: `{"character":"alice"}`, sharing: true},
		{name: "private true stays private", legacy: `{"character":"alice","private":true}`, sharing: false},
	} {
		s.Run(tc.name, func() {
			var legacy session.ExplorationData
			s.Require().NoError(json.Unmarshal([]byte(tc.legacy), &legacy))
			rig := newSharingRig(&explorationStore{data: map[string]*session.ExplorationData{"alice": &legacy}})
			mgr := s.launchSharing(rig)
			s.Equal(tc.sharing, s.reportedSharing(mgr))
			s.Equal(tc.sharing, s.sharingAfterPrepare(rig))
			s.Nil(rig.profiles.data["alice"].DiscoverySharing, "reading a legacy record does not rewrite it as a choice")
		})
	}
}

func (s *AutomaticDiscoverySDKSuite) TestChoiceIsSavedAndSurvivesReload() {
	ctx := context.Background()
	rig := newSharingRig(newFakeExplorations())
	mgr := s.launchSharing(rig)
	_, err := mgr.SetDiscoverySharing(ctx,
		&session.SetDiscoverySharingInput{Session: sharingRun, Member: "alice", Sharing: true})
	s.Require().NoError(err)
	raw, err := json.Marshal(rig.profiles.data["alice"])
	s.Require().NoError(err)
	s.JSONEq(`{"character":"alice","discovery_sharing":true}`, string(raw))
	s.True(s.sharingAfterPrepare(rig), "the saved choice resolves shared after a reload")

	_, err = mgr.SetDiscoverySharing(ctx,
		&session.SetDiscoverySharingInput{Session: sharingRun, Member: "alice", Sharing: false})
	s.Require().NoError(err)
	raw, err = json.Marshal(rig.profiles.data["alice"])
	s.Require().NoError(err)
	s.JSONEq(`{"character":"alice","discovery_sharing":false,"private":true}`, string(raw))
	s.False(s.sharingAfterPrepare(rig), "the second choice resolves private after a reload")
}

func (s *AutomaticDiscoverySDKSuite) TestChoosingTheDefaultIsStillAChoice() {
	rig := newSharingRig(newFakeExplorations())
	mgr := s.launchSharing(rig)
	_, err := mgr.SetDiscoverySharing(context.Background(),
		&session.SetDiscoverySharingInput{Session: sharingRun, Member: "alice", Sharing: false})
	s.Require().NoError(err)
	s.Require().Contains(rig.profiles.data, "alice", "choosing private is written even though private is the default")
	s.Require().NotNil(rig.profiles.data["alice"].DiscoverySharing)
	s.False(*rig.profiles.data["alice"].DiscoverySharing)
}
