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
			mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(),
				Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: characters,
				CharacterConfig: newFakeCharacterConfig(),
				Dice:            dice,
				PresentationIDs: testPresentationIDs{}, TurnDriver: session.Pass{},
			})
			s.Require().NoError(err)
			world := authoredWorld()
			world.Session = "run"
			world.Field.Concealments = []encounter.ConcealmentInput{{
				ID: "hall/strength-secret", Checks: []encounter.CheckApproach{{Ability: "athletics", DC: 99}},
				Cells: []spatial.Position{{X: 3, Y: 1}},
			}}
			launchScene(s.T(), mgr, world)
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
			checks := beatsNamed(storyBeats(s.T(), mgr, "run", "bob"), encounter.BeatDiscoveryChecked)
			s.Len(checks, 1)
		})
	}
}
