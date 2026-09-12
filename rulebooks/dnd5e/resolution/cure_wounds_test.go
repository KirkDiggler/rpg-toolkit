// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

func TestHealingPublicationFailureDoesNotPromiseRollback(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	stored := baneCaster(1, 2)
	stored.HitPoints = 1
	sheet, err := character.Load(ctx, stored)
	require.NoError(t, err)
	require.NoError(t, character.Attach(ctx, sheet, bus))
	t.Cleanup(func() { require.NoError(t, sheet.Cleanup(ctx)) })
	fault := errors.New("applied subscriber failed")
	_, err = dnd5eEvents.HealingAppliedTopic.On(bus).Subscribe(ctx, func(context.Context, dnd5eEvents.HealingAppliedEvent) error { return fault })
	require.NoError(t, err)
	heal := preparedHealing{targetID: bardID, declaration: healing.Declaration{Dice: "1d8"}, roller: facedRoller{other: 5}, source: dnd5eEvents.RollSource{Ref: refs.Spells.CureWounds(), Name: "Cure Wounds"}}
	require.ErrorIs(t, heal.deliver(ctx, bus, ""), fault)
	require.Equal(t, 6, sheet.ToData().HitPoints, "a later subscriber failure does not undo live HP mutation")
	require.Equal(t, 1, stored.HitPoints, "the caller's persisted input remains untouched")
}

func (s *CastActionTestSuite) TestCureWoundsMonsterTypesAndConcentration() {
	for _, tc := range []struct {
		name         string
		ref          *core.Ref
		creatureType string
		amount       int
		noEffect     bool
	}{
		{name: "beast", ref: refs.Monsters.Wolf(), amount: 8},
		{name: "undead", ref: refs.Monsters.Skeleton(), noEffect: true},
		{name: "construct", ref: refs.Monsters.AnimatedArmor(), noEffect: true},
		{name: "missing ref and type", amount: 8},
		{name: "unknown custom ref", ref: &core.Ref{Module: "custom", Type: "monsters", ID: "friend"}, amount: 8},
		{name: "explicit undead", creatureType: "undead", noEffect: true},
		{name: "explicit construct", creatureType: "construct", noEffect: true},
		{name: "explicit beast", creatureType: "beast", amount: 8},
	} {
		s.Run(tc.name, func() {
			f := s.fixtures()
			caster := baneCaster(1, 2)
			caster.Conditions = []json.RawMessage{baneOwnerJSON(s.T(), bardID, 5)}
			ch, err := character.Load(s.ctx, caster)
			s.Require().NoError(err)
			definition := ch.CastDefinition(spells.CureWounds)
			roll := &countingCastRoller{facedRoller: facedRoller{other: 5}}
			machine, err := NewAction(&ActionInput{Definition: *definition, AttackerID: bardID, TargetIDs: []string{wolfID}, Roller: roll})
			s.Require().NoError(err)
			target := f.wolfData()
			target.Ref = tc.ref
			target.CreatureType = tc.creatureType
			target.HitPoints = 1
			out, err := Resolve(s.ctx, &Input{World: f.world(), Participants: []Participant{{Character: caster}, {Character: f.saver(14)}, {Monster: target}}, Machine: machine,
				Cost:       &Cost{SpellTurn: "scene/round-1/bard", PayerID: bardID, Profile: definition.Cost, Turn: &Turn{Number: mockeryTurn, Speed: mockerySpeed}},
				Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Roller: dice.NewRoller()})
			s.Require().NoError(err)
			outcome := s.castOutcome(out)
			s.Require().Len(outcome.Targets, 1)
			s.Nil(outcome.Targets[0].Save)
			s.Require().Len(outcome.Targets[0].Applied, 1)
			applied := outcome.Targets[0].Applied[0]
			s.Equal(ImposedHealing, applied.Kind)
			s.Equal(tc.amount, applied.Amount)
			s.Equal(1, applied.Before)
			s.Equal(1+tc.amount, applied.After)
			if tc.noEffect {
				s.Zero(roll.calls)
			} else {
				s.Equal(1, roll.calls)
				s.Require().Len(out.DirtyMonsters, 1)
				s.Equal(1+tc.amount, out.DirtyMonsters[0].HitPoints)
				s.Equal(tc.creatureType, out.DirtyMonsters[0].CreatureType, "healing does not invent classification")
			}
			paid := f.sheet(out, bardID)
			s.Equal(1, paid.Resources[resources.SpellSlotLevel1].Current)
			s.Require().Len(paid.Conditions, 1)
			s.JSONEq(string(caster.Conditions[0]), string(paid.Conditions[0]), "healing preserves concentration")
		})
	}
}

func (s *CastActionTestSuite) TestCureWoundsPhysicalBarrierRefusesBeforePayment() {
	f := s.fixtures()
	caster := baneCaster(1, 2)
	ch, err := character.Load(s.ctx, caster)
	s.Require().NoError(err)
	definition := ch.CastDefinition(spells.CureWounds)
	roll := &countingCastRoller{facedRoller: facedRoller{other: 5}}
	machine, err := NewAction(&ActionInput{Definition: *definition, AttackerID: bardID, TargetIDs: []string{wolfID}, Roller: roll})
	s.Require().NoError(err)
	world := f.world()
	world.Field.Walls = append(world.Field.Walls, encounter.BoundaryData{From: encounter.PositionData{X: 2, Y: 1}, To: encounter.PositionData{X: 3, Y: 1}, BlocksMovement: true, BlocksLineOfSight: true})
	out, err := Resolve(s.ctx, &Input{World: world, Participants: []Participant{{Character: caster}, {Character: f.saver(14)}, {Monster: f.wolfData()}}, Machine: machine,
		Cost:       &Cost{SpellTurn: "scene/round-1/bard", PayerID: bardID, Profile: definition.Cost, Turn: &Turn{Number: mockeryTurn, Speed: mockerySpeed}},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Roller: dice.NewRoller()})
	s.ErrorIs(err, ErrOutOfRange)
	s.Nil(out)
	s.Zero(roll.calls)
	s.Equal(2, caster.Resources[resources.SpellSlotLevel1].Current)
}

func (s *CastActionTestSuite) TestHealingTargetsDoesNotRequireCreatureClassification() {
	for _, tc := range []struct {
		name string
		ref  *core.Ref
		hp   int
		want bool
	}{
		{name: "untyped custom monster", hp: 1, want: true},
		{name: "unknown ref", ref: &core.Ref{Module: "custom", Type: "monsters", ID: "friend"}, hp: 1, want: true},
		{name: "undead remains a valid paid declaration", ref: refs.Monsters.Skeleton(), hp: 1, want: true},
		{name: "construct remains a valid paid declaration", ref: refs.Monsters.AnimatedArmor(), hp: 1, want: true},
		{name: "defeated untyped monster", hp: 0},
	} {
		s.Run(tc.name, func() {
			f := s.fixtures()
			target := f.wolfData()
			target.Ref = tc.ref
			target.CreatureType = ""
			target.HitPoints = tc.hp
			room := spatial.NewBasicRoom(spatial.BasicRoomConfig{ID: "healing-targets", Grid: spatial.NewHexGrid(spatial.HexGridConfig{Width: 8, Height: 8})})
			s.Require().NoError(room.PlaceEntity(activationTestEntity{id: bardID}, spatial.Position{X: 2, Y: 1}))
			s.Require().NoError(room.PlaceEntity(activationTestEntity{id: wolfID}, spatial.Position{X: 3, Y: 1}))
			answers, err := HealingTargets(s.ctx, &HealingTargetsInput{Room: room, CasterID: bardID, Candidates: []string{wolfID}, Participants: []Participant{{Character: baneCaster(1, 2)}, {Monster: target}}})
			s.Require().NoError(err)
			if tc.want {
				s.Equal(map[string]bool{wolfID: true}, answers)
			} else {
				s.Empty(answers)
			}
		})
	}
}

func TestTouchReachUsesPhysicalBoundaryWithoutSightPredicate(t *testing.T) {
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{ID: "touch", Grid: spatial.NewHexGrid(spatial.HexGridConfig{Width: 8, Height: 8})})
	from, to := spatial.Position{X: 2, Y: 1}, spatial.Position{X: 3, Y: 1}
	require.NoError(t, room.PlaceEntity(activationTestEntity{id: "caster"}, from))
	require.NoError(t, room.PlaceEntity(activationTestEntity{id: "target"}, to))
	reachable, err := TouchReach(room, "caster", "target")
	require.NoError(t, err)
	require.True(t, reachable)
	require.NoError(t, room.RegisterBoundary(spatial.Boundary{From: from, To: to, BlocksMovement: true}))
	require.False(t, room.IsLineOfSightBlocked(from, to), "transparent boundary permits sight")
	reachable, err = TouchReach(room, "caster", "target")
	require.NoError(t, err)
	require.False(t, reachable)
	reachable, err = TouchReach(room, "caster", "caster")
	require.NoError(t, err)
	require.True(t, reachable)
}
