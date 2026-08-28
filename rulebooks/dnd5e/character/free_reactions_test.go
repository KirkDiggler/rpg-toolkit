// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func plainFighter(id string) *character.Data {
	return &character.Data{
		ID: id, PlayerID: "p", Name: "Plain", Level: 1, ClassID: "fighter", RaceID: "human",
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 12, MaxHitPoints: 12, ArmorClass: 16, ProficiencyBonus: 2,
	}
}

// inFight is the same sheet with a turn's worth of economy on it, which is what
// a character has once they have acted in a fight.
func inFight(data *character.Data) *character.Data {
	data.ActionEconomy = &character.ActionEconomyData{
		TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
		ReactionsRemaining: 1, MovementRemaining: 30,
	}

	return data
}

func carries(conds []dnd5eEvents.ConditionBehavior, ref string) bool {
	for _, c := range conds {
		if got := c.Ref(); got != nil && got.String() == ref {
			return true
		}
	}

	return false
}

// An opportunity attack is not a class feature — every melee combatant has one,
// monsters included, which is why it is correctly absent from all twelve grant
// lists. A character who was authored with no conditions at all still has one.
func TestACharacterWithNoConditionsStillCarriesItsFreeReactions(t *testing.T) {
	ctx := context.Background()
	sheet, err := character.Load(ctx, inFight(plainFighter("fighter-1")))
	require.NoError(t, err)
	require.Empty(t, sheet.GetConditions(), "load is a pure read and grants nothing")

	bus := events.NewEventBus()
	require.NoError(t, character.Attach(ctx, sheet, bus))

	require.Equal(t, 1, triggersOnAStepAway(t, bus, "fighter-1"),
		"the carried reaction is LIVE — one that was merely constructed could never fire")
}

// triggersOnAStepAway runs a real movement fold with an enemy leaving the
// fighter's reach and counts the reaction triggers it produced.
//
// A liveness proof rather than a subscription count: the only thing that
// matters about a carried condition is whether it FIRES, and a test that
// inspected the bus would pass just as happily for a condition subscribed to
// the wrong topic.
func triggersOnAStepAway(t *testing.T, bus events.EventBus, reactor string) int {
	t.Helper()
	ctx := context.Background()

	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID: "r", Type: "dungeon",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 20, Height: 20}),
	})
	require.NoError(t, room.PlaceEntity(&placedEntity{id: reactor, kind: "character"}, spatial.Position{X: 5, Y: 5}))
	require.NoError(t, room.PlaceEntity(&placedEntity{id: "wolf-1", kind: "monster"}, spatial.Position{X: 5, Y: 6}))

	fired := 0
	_, err := dnd5eEvents.ReactionTriggerTopic.On(bus).Subscribe(ctx,
		func(_ context.Context, _ dnd5eEvents.ReactionTriggerEvent) error { fired++; return nil })
	require.NoError(t, err)

	runCtx := gamectx.WithReactionReadiness(gamectx.WithRoom(ctx, room),
		gamectx.ReactionReadinessMap{reactor: {refs.Conditions.OpportunityAttack().String(): true}})

	event := &dnd5eEvents.MovementChainEvent{
		EntityID:     "wolf-1",
		EntityType:   "monster",
		FromPosition: dnd5eEvents.Position{X: 5, Y: 6},
		ToPosition:   dnd5eEvents.Position{X: 5, Y: 7},
	}
	staged := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
	folded, err := dnd5eEvents.MovementChain.On(bus).PublishWithChain(runCtx, event, staged)
	require.NoError(t, err)
	_, err = folded.Execute(runCtx, event)
	require.NoError(t, err)

	return fired
}

// placedEntity is a body on the map, which is all the geometry predicate needs.
type placedEntity struct {
	id   string
	kind core.EntityType
}

func (p *placedEntity) GetID() string            { return p.id }
func (p *placedEntity) GetType() core.EntityType { return p.kind }

// The character half of Kirk's ruling: characters PAY, so their meter is the
// persisted reaction slot and the condition itself is never written down.
//
// This is what keeps a sheet that merely joined an interaction from being
// rewritten — resolution refuses to write back a participant nothing happened
// to, and gaining a reaction is not something that happened.
func TestACarriedReactionIsNeverWrittenToTheCharacterSheet(t *testing.T) {
	ctx := context.Background()
	data := plainFighter("fighter-1")

	sheet, err := character.Load(ctx, data)
	require.NoError(t, err)
	require.NoError(t, character.Attach(ctx, sheet, events.NewEventBus()))

	after := sheet.ToData()
	require.Empty(t, after.Conditions,
		"a character's meter is ActionEconomy.ReactionsRemaining, so nothing needs writing "+
			"and the sheet writes back exactly what it was built from")
	require.False(t, sheet.IsDirty(), "gaining a reaction is not a change worth saving")
}

// A COLD SHEET CANNOT REACT, and this is the honest consequence of Kirk's
// ruling that characters pay: the purse a character pays from is the action
// economy, a sheet that has not acted in a fight has none, and combat.Ledger
// answers "nothing left" rather than "unlimited" for a holder who is not in
// combat.
//
// So a character carries the condition from the moment they attach, and it
// stays silent until they have taken a turn. In initiative order that is the
// window before their first turn.
//
// Pinned rather than fixed because fixing it is a ruling about when a sheet is
// lit, not a bug in this file — the session lights a sheet when an actor on the
// fight clock first acts (rpg-toolkit#1091), and moving that is its own
// decision. Written down so it is found deliberately instead of reported as a
// mystery.
func TestACharacterWhoHasNotActedYetCarriesTheReactionButCannotSpendIt(t *testing.T) {
	ctx := context.Background()
	sheet, err := character.Load(ctx, plainFighter("fighter-1"))
	require.NoError(t, err)

	bus := events.NewEventBus()
	require.NoError(t, character.Attach(ctx, sheet, bus))

	require.Equal(t, 0, triggersOnAStepAway(t, bus, "fighter-1"),
		"no economy means no reaction slot to pay from, so the swing is declined")
}

// A character who somehow already persisted one — a sheet written by an older
// build, or a monster-shaped fixture — is not given a duplicate.
func TestAnAlreadyCarriedReactionIsNotDuplicated(t *testing.T) {
	ctx := context.Background()
	data := plainFighter("fighter-1")
	blob, err := conditions.NewOpportunityAttackCondition("fighter-1").ToJSON()
	require.NoError(t, err)
	data.Conditions = append(data.Conditions, blob)

	sheet, err := character.Load(ctx, data)
	require.NoError(t, err)
	require.NoError(t, character.Attach(ctx, sheet, events.NewEventBus()))

	count := 0
	for _, c := range sheet.GetConditions() {
		if c.Ref() != nil && c.Ref().String() == refs.Conditions.OpportunityAttack().String() {
			count++
		}
	}
	require.Equal(t, 1, count, "the persisted one is kept and no second is carried on top")
	require.True(t, carries(sheet.GetConditions(), refs.Conditions.OpportunityAttack().String()))
}
