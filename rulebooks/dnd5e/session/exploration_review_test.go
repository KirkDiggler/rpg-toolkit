// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func (s *AutomaticDiscoverySDKSuite) TestSharingRefusalsKeepTheVerbAndSentinel() {
	ctx := context.Background()
	sessions := newFakeSessions()
	encounters := &failingEncounters{fakeEncounters: newFakeEncounters()}
	cfg := &session.Config{
		Sessions: sessions, Encounters: encounters, Characters: testCharacters(),
		Explorations: &explorationStore{data: map[string]*session.ExplorationData{}},
		Events:       session.DiscardEvents{}, Dice: testDice{},
		PresentationIDs: testPresentationIDs{}, TurnDriver: session.Pass{},
	}
	mgr, err := session.NewManager(cfg)
	s.Require().NoError(err)
	_, err = mgr.SetDiscoverySharing(ctx, nil)
	s.ErrorIs(err, session.ErrNilInput)
	s.ErrorContains(err, "set discovery sharing:")
	_, err = mgr.SetDiscoverySharing(ctx, &session.SetDiscoverySharingInput{Session: "missing", Member: "alice"})
	s.ErrorIs(err, session.ErrNoSession)
	s.ErrorContains(err, "set discovery sharing:")

	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "run", Encounter: "enc", World: authoredWorld(s.T())})
	s.Require().NoError(err)
	_, err = mgr.SetDiscoverySharing(ctx, &session.SetDiscoverySharingInput{Session: "run", Member: "stranger"})
	s.ErrorIs(err, session.ErrNoMember)
	s.ErrorContains(err, "set discovery sharing:")

	encounters.saveErr = errBroken
	_, err = mgr.SetDiscoverySharing(ctx, &session.SetDiscoverySharingInput{Session: "run", Member: "alice", Sharing: false})
	s.ErrorIs(err, session.ErrSaveFailed)
	s.ErrorContains(err, "set discovery sharing:")
	var partial *session.SaveError
	s.Require().ErrorAs(err, &partial)
	s.Contains(partial.Report.Written, "exploration:alice", "context wrapping must preserve the partial-save report")

	cfg.Explorations = nil
	legacy, err := session.NewManager(cfg)
	s.Require().NoError(err)
	_, err = legacy.SetDiscoverySharing(ctx, &session.SetDiscoverySharingInput{Session: "run", Member: "alice"})
	s.ErrorIs(err, session.ErrIncompleteConfig)
	s.ErrorContains(err, "set discovery sharing:")

	cfg.Locker = &observingLocker{err: context.Canceled}
	blocked, err := session.NewManager(cfg)
	s.Require().NoError(err)
	_, err = blocked.SetDiscoverySharing(ctx, &session.SetDiscoverySharingInput{Session: "run", Member: "alice"})
	s.ErrorIs(err, context.Canceled)
	s.ErrorContains(err, "set discovery sharing:")
}

type discoveryReviewDice struct{ calls int }

func (d *discoveryReviewDice) Roll(_ context.Context, _ int) (int, error) {
	d.calls++
	return 10, nil
}

func (s *AutomaticDiscoverySDKSuite) TestJoinDiscoveryUsesTheRestedRecordRatherThanTheStagedRage() {
	for _, rageAfterJoin := range []bool{false, true} {
		name := "first admission clears staged rage before the automatic check"
		if rageAfterJoin {
			name = "control: an actually raging character rolls strength with advantage"
		}
		s.Run(name, func() {
			ctx := context.Background()
			characters := newFakeCharacters(dwarfCharacter("alice"), ragingDwarf("bob"))
			dice := &discoveryReviewDice{}
			mgr, err := session.NewManager(&session.Config{
				Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: characters,
				Explorations: &explorationStore{data: map[string]*session.ExplorationData{}},
				Events:       session.DiscardEvents{}, Dice: dice,
				PresentationIDs: testPresentationIDs{}, TurnDriver: session.Pass{},
			})
			s.Require().NoError(err)
			world := authoredWorld(s.T())
			world.Field.Concealments = []encounter.ConcealmentData{{
				ID: "hall/strength-secret", Checks: []encounter.CheckApproachData{{Ability: "athletics", DC: 99}},
				Cells: []encounter.PositionData{{X: 3, Y: 1}},
			}}
			_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "run", Encounter: "enc", World: world})
			s.Require().NoError(err)
			position := spatial.Position{X: 2, Y: 1}
			if rageAfterJoin {
				position = spatial.Position{} // out of range during admission/rest
			}
			_, err = mgr.Join(ctx, &session.JoinInput{Session: "run", Member: "bob", Position: position})
			s.Require().NoError(err)
			s.Nil(conditionByRef(s.T(), characters.byID["bob"].Conditions, refs.Conditions.Raging().String()),
				"first admission actually removed the old rage")
			if rageAfterJoin {
				s.Zero(dice.calls)
				characters.byID["bob"] = ragingDwarf("bob")
				_, err = mgr.Move(ctx, &session.MoveInput{Session: "run", Member: "bob", Path: []spatial.Position{{X: 1}, {X: 2}, {X: 3}}})
				s.Require().NoError(err)
				s.Equal(2, dice.calls, "the control proves stale rage would change the check")
			} else {
				s.Equal(1, dice.calls, "Join is not a walker: the consult must refresh its pre-rest staging")
			}
			story, err := mgr.Story(ctx, &session.StoryInput{Session: "run", Member: "bob"})
			s.Require().NoError(err)
			checks := 0
			for _, event := range story {
				if event.Kind == session.EventDiscoveryChecked {
					checks++
				}
			}
			s.Equal(1, checks)
		})
	}
}
