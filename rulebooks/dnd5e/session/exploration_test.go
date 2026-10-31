// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// characterConfigStore is the one in-memory character configuration every
// test Config wires: Config.CharacterConfig is required, so no manager is
// built without one. A missing entry is a character who has not chosen. It
// counts reads, so a test can see the session ask at every write verb, and it
// has no write method because the session never writes character
// configuration (rpg-project#753 R2).
type characterConfigStore struct {
	mu      sync.Mutex
	sharing map[string]bool
	reads   map[string]int
	err     error
}

func newFakeCharacterConfig() *characterConfigStore {
	return &characterConfigStore{sharing: map[string]bool{}, reads: map[string]int{}}
}

func (r *characterConfigStore) DiscoverySharing(_ context.Context, id string) (*bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reads[id]++
	if r.err != nil {
		return nil, r.err
	}
	choice, ok := r.sharing[id]
	if !ok {
		return nil, nil
	}
	return &choice, nil
}

// choose is the host's character verb, standing in for the api's
// SetPreference: it writes outside any session.
func (r *characterConfigStore) choose(id string, sharing bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sharing[id] = sharing
}

func (r *characterConfigStore) readsOf(id string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reads[id]
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
	count := 0
	for _, beat := range beatsNamed(storyBeats(s.T(), mgr, run, "alice"), encounter.BeatDiscoveryChecked) {
		count++
		var body struct {
			Ability     string          `json:"ability"`
			Calculation json.RawMessage `json:"calculation"`
		}
		beat.decode(s.T(), &body)
		s.Equal("religion", body.Ability)
		s.NotEmpty(body.Calculation)
	}
	return count
}

func (s *AutomaticDiscoverySDKSuite) TestAPrivateChoiceSeedsANewEncounterWithoutAttempts() {
	ctx := context.Background()
	profiles := newFakeCharacterConfig()
	profiles.choose("alice", false)
	encounters := newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(),
		Sessions: newFakeSessions(), Encounters: encounters, Characters: testCharacters(),
		CharacterConfig: profiles, Dice: testDice{},
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
			profiles := newFakeCharacterConfig()
			sessions, encounters := newFakeSessions(), newFakeEncounters()
			cfg := &session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), Sessions: sessions, Encounters: encounters, Characters: testCharacters(),
				CharacterConfig: profiles, Dice: testDice{},
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
			profiles.choose("alice", false)
			start("first", seatAt("bob", 2, 4))
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
			again, err := encounters.GetEncounter(ctx, "second")
			s.Require().NoError(err)
			s.True(again.Discovery["alice"].Private, "only the sharing preference carries across runs")
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
	profiles *characterConfigStore
}

func newSharingRig(profiles *characterConfigStore) *sharingRig {
	return &sharingRig{profiles: profiles, cfg: &session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(),
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: testCharacters(),
		CharacterConfig: profiles, Dice: testDice{},
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
// reader rather than what the previous verb left on the encounter.
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

// TestResolveDiscoverySharing_FromReader: resolveDiscoverySharing reads the
// reader's one answer (rpg-project#753 R2). An explicit choice wins either
// way; a character who has not chosen discovers privately. A legacy record
// without a choice is the host's to resolve before it answers.
func TestResolveDiscoverySharing_FromReader(t *testing.T) {
	for _, tc := range []struct {
		name    string
		choice  *bool
		sharing bool
	}{
		{name: "nil is not chosen, so private", sharing: false},
		{name: "true is shared", choice: boolPtr(true), sharing: true},
		{name: "false is private", choice: boolPtr(false), sharing: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := new(AutomaticDiscoverySDKSuite)
			s.SetT(t)
			profiles := newFakeCharacterConfig()
			if tc.choice != nil {
				profiles.choose("alice", *tc.choice)
			}
			rig := newSharingRig(profiles)
			mgr := s.launchSharing(rig)
			s.Equal(tc.sharing, s.reportedSharing(mgr), "launch restores the reader's answer")
			s.Equal(tc.sharing, s.sharingAfterPrepare(rig), "a later write verb restores the same answer")
		})
	}
}

func (s *AutomaticDiscoverySDKSuite) TestAChangedChoiceTakesEffectAtTheNextWriteVerb() {
	rig := newSharingRig(newFakeCharacterConfig())
	mgr := s.launchSharing(rig)
	s.False(s.reportedSharing(mgr), "a character who never chose sees privately")
	ctx := context.Background()

	// The host's character verb changes the choice outside the session.
	rig.profiles.choose("alice", true)
	s.True(s.reportedSharing(mgr), "the knowledge read answers from the configuration at once")
	stored, err := rig.cfg.Encounters.GetEncounter(ctx, sharingRun)
	s.Require().NoError(err)
	s.True(stored.Discovery["alice"].Private, "the encounter's own copy waits for the next write verb")

	before := rig.profiles.readsOf("alice")
	s.True(s.sharingAfterPrepare(rig), "the next write verb restores the new choice")
	s.Greater(rig.profiles.readsOf("alice"), before, "every write verb asks the reader")
	stored, err = rig.cfg.Encounters.GetEncounter(ctx, sharingRun)
	s.Require().NoError(err)
	s.False(stored.Discovery["alice"].Private, "and the write verb saved the restored copy")

	rig.profiles.choose("alice", false)
	s.False(s.sharingAfterPrepare(rig), "and the next one restores the change back")
}

func (s *AutomaticDiscoverySDKSuite) TestAReaderErrorRefusesTheWriteVerb() {
	ctx := context.Background()
	rig := newSharingRig(newFakeCharacterConfig())
	mgr := s.launchSharing(rig)
	rig.profiles.err = errBroken
	_, err := mgr.Move(ctx, &session.MoveInput{Session: sharingRun, Member: "alice", Path: []spatial.Position{{X: 1, Y: 1}}})
	s.Require().ErrorIs(err, errBroken)
	s.ErrorContains(err, "character configuration")
}

func (s *AutomaticDiscoverySDKSuite) TestAReaderErrorRefusesTheKnowledgeRead() {
	rig := newSharingRig(newFakeCharacterConfig())
	mgr := s.launchSharing(rig)
	rig.profiles.err = errBroken
	_, err := mgr.Knowledge(context.Background(),
		&session.KnowledgeInput{Session: sharingRun, Member: "alice", Player: sharingPlayer})
	s.Require().ErrorIs(err, errBroken)
	s.ErrorContains(err, "character configuration")
}

func (s *AutomaticDiscoverySDKSuite) TestNotFoundIsNotChosen() {
	rig := newSharingRig(newFakeCharacterConfig())
	rig.profiles.err = session.ErrNotFound
	mgr := s.launchSharing(rig)
	s.False(s.reportedSharing(mgr), "a reader answering ErrNotFound reads as not chosen")
}

func boolPtr(v bool) *bool { return &v }
