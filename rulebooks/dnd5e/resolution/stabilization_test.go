// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// An arbitrary content identity proves delivery is selected by the profile.
func stabilizationDefinition() actions.Definition {
	return actions.Definition{Ref: core.Ref{Module: "test", Type: "spells", ID: "stabilize"}, Name: "Stabilize", Cost: oneAction(), Cast: &actions.CastProfile{
		Casting: &combat.SpellCasting{Level: 0, Time: combat.SpellCastingAction},
		Target:  actions.CastTargetTouch, RangeFeet: 5, MinTargets: 1, MaxTargets: 1, Stabilize: true,
	}}
}

func (s *CastActionTestSuite) TestStabilizationDeliveryAndRefusal() {
	for _, tc := range []struct {
		name                        string
		hp                          int
		state                       saves.DeathSaveState
		actions                     int
		far, wall, monster, missing bool
		wantErr                     error
	}{
		{name: "dying", state: saves.DeathSaveState{Successes: 1, Failures: 2}, actions: 1},
		{name: "already stable", state: saves.DeathSaveState{Stabilized: true}, actions: 1},
		{name: "positive HP", hp: 1, actions: 1, wantErr: ErrBadAction},
		{name: "dead", state: saves.DeathSaveState{Dead: true, Failures: 3}, actions: 1, wantErr: ErrBadAction},
		{name: "no action", state: saves.DeathSaveState{Failures: 1}, wantErr: ErrCannotPay},
		{name: "out of reach", actions: 1, far: true, wantErr: ErrOutOfRange},
		{name: "transparent wall", actions: 1, wall: true, wantErr: ErrOutOfRange},
		{name: "monster", actions: 1, monster: true, wantErr: ErrBadAction},
		{name: "missing character", actions: 1, missing: true, wantErr: ErrBadAction},
	} {
		s.Run(tc.name, func() {
			f := s.fixtures()
			caster := baneCaster(tc.actions, 2)
			caster.Conditions = []json.RawMessage{baneOwnerJSON(s.T(), bardID, 5)}
			target := f.saver(tc.hp)
			target.DeathSaveState = &tc.state
			world := f.world()
			if !tc.far {
				for i := range world.Members {
					if string(world.Members[i].ID) == heroID {
						world.Members[i].Cell = &encounter.PositionData{X: 2, Y: 1}
					}
				}
			}
			if tc.wall {
				world.Field.Walls = append(world.Field.Walls, encounter.BoundaryData{From: encounter.PositionData{X: 2, Y: 1}, To: encounter.PositionData{X: 3, Y: 1}, BlocksMovement: true})
			}
			participants := []Participant{{Character: caster}, {Monster: f.wolfData()}}
			if !tc.missing {
				participants = append(participants, Participant{Character: target})
			}
			targetID := heroID
			if tc.monster {
				targetID = wolfID
			}
			before, err := json.Marshal(participants)
			s.Require().NoError(err)
			roll := &countingCastRoller{}
			definition := stabilizationDefinition()
			machine, err := NewAction(&ActionInput{Definition: definition, AttackerID: bardID, TargetIDs: []string{targetID}, Roller: roll})
			s.Require().NoError(err)
			out, err := Resolve(s.ctx, &Input{World: world, Participants: participants, Machine: machine, Cost: castCost(), Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Roller: roll})
			after, marshalErr := json.Marshal(participants)
			s.Require().NoError(marshalErr)
			s.JSONEq(string(before), string(after), "the caller's persisted inputs remain untouched")
			s.Zero(roll.calls)
			if tc.wantErr != nil {
				s.ErrorIs(err, tc.wantErr)
				s.Nil(out)
				return
			}
			s.Require().NoError(err)
			result := s.castOutcome(out)
			s.Require().Len(result.Targets, 1)
			s.Nil(result.Targets[0].Save)
			s.Require().Len(result.Targets[0].Applied, 1)
			effect := result.Targets[0].Applied[0]
			s.Equal(ImposedStabilized, effect.Kind)
			s.Equal(heroID, effect.RecipientID)
			s.Equal(definition.Ref, *effect.Ref)
			expectedBefore := combat.LifeStateDying
			if tc.state.Stabilized {
				expectedBefore = combat.LifeStateStabilized
			}
			s.Equal(expectedBefore, effect.Stabilization.Before)
			s.Equal(combat.LifeStateStabilized, effect.Stabilization.After)
			s.Zero(effect.Stabilization.HitPoints)
			s.Zero(effect.Stabilization.Progress.Successes)
			s.Zero(effect.Stabilization.Progress.Failures)
			s.True(effect.Stabilization.Progress.Stabilized)
			s.Nil(effect.Calculation)
			paid := f.sheet(out, bardID)
			s.Zero(paid.ActionEconomy.ActionsRemaining)
			s.Equal(2, paid.Resources[resources.SpellSlotLevel1].Current)
			s.Require().Len(paid.Conditions, 1)
			s.JSONEq(string(caster.Conditions[0]), string(paid.Conditions[0]))
			encoded, err := json.Marshal(f.sheet(out, heroID))
			s.Require().NoError(err)
			var stored character.Data
			s.Require().NoError(json.Unmarshal(encoded, &stored))
			reloaded, err := character.Load(s.ctx, &stored)
			s.Require().NoError(err)
			s.Equal(combat.LifeStateStabilized, reloaded.LifeState())
			s.Zero(stored.HitPoints)
			s.Equal(&saves.DeathSaveState{Stabilized: true}, stored.DeathSaveState)
		})
	}
}

func (s *CastActionTestSuite) TestStabilizationTargets() {
	f := s.fixtures()
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{ID: "stabilization", Grid: spatial.NewHexGrid(spatial.HexGridConfig{Width: 8, Height: 8})})
	s.Require().NoError(room.PlaceEntity(activationTestEntity{id: bardID}, spatial.Position{X: 2, Y: 1}))
	s.Require().NoError(room.PlaceEntity(activationTestEntity{id: heroID}, spatial.Position{X: 3, Y: 1}))
	target := f.saver(0)
	target.DeathSaveState = &saves.DeathSaveState{Failures: 2}
	input := &StabilizationTargetsInput{Room: room, CasterID: bardID, Candidates: []string{heroID, bardID, wolfID, "missing"}, Participants: []Participant{{Character: target}, {Character: baneCaster(1, 2)}, {Monster: f.wolfData()}}}
	before, err := json.Marshal(target)
	s.Require().NoError(err)
	answers, err := StabilizationTargets(s.ctx, input)
	s.Require().NoError(err)
	s.Equal(map[string]bool{heroID: true}, answers)
	after, err := json.Marshal(target)
	s.Require().NoError(err)
	s.JSONEq(string(before), string(after))
	target.DeathSaveState = &saves.DeathSaveState{Stabilized: true}
	answers, err = StabilizationTargets(s.ctx, input)
	s.Require().NoError(err)
	s.True(answers[heroID])
	s.Require().NoError(room.RegisterBoundary(spatial.Boundary{From: spatial.Position{X: 2, Y: 1}, To: spatial.Position{X: 3, Y: 1}, BlocksMovement: true}))
	answers, err = StabilizationTargets(s.ctx, input)
	s.Require().NoError(err)
	s.False(answers[heroID])
	target.DeathSaveState = &saves.DeathSaveState{Dead: true, Failures: 3}
	answers, err = StabilizationTargets(s.ctx, input)
	s.Require().NoError(err)
	s.Empty(answers)
	_, err = StabilizationTargets(s.ctx, nil)
	s.ErrorIs(err, ErrNilInput)
}
