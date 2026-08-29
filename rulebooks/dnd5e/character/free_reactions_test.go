// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func plainFighter(id string) *Data {
	return &Data{
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
func inFight(data *Data) *Data {
	data.ActionEconomy = &ActionEconomyData{
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
	sheet, err := Load(ctx, inFight(plainFighter("fighter-1")))
	require.NoError(t, err)
	require.Empty(t, sheet.GetConditions(), "load is a pure read and grants nothing")

	bus := events.NewEventBus()
	require.NoError(t, Attach(ctx, sheet, bus))

	require.Equal(t, 1, triggersOnAStepAway(t, bus, sheet),
		"the carried reaction is LIVE — one that was merely constructed could never fire")
}

// triggersOnAStepAway runs a real movement fold with an enemy leaving the
// fighter's reach and counts the reaction triggers it produced.
//
// A liveness proof rather than a subscription count: the only thing that
// matters about a carried condition is whether it FIRES, and a test that
// inspected the bus would pass just as happily for a condition subscribed to
// the wrong topic.
func triggersOnAStepAway(t *testing.T, bus events.EventBus, reactor combat.Member) int {
	t.Helper()

	// The reactor's own sheet, installed the way resolution's one door
	// installs it on every path that folds anything. A carried reaction reads
	// its slot off the cast now, so this is no longer scenery: it is where the
	// answer below comes from, and a fold without it has no sheet to ask.
	ctx := castOf(context.Background(), reactor)

	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID: "r", Type: "dungeon",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 20, Height: 20}),
	})
	require.NoError(t, room.PlaceEntity(&placedEntity{id: reactor.GetID(), kind: "character"}, spatial.Position{X: 5, Y: 5}))
	require.NoError(t, room.PlaceEntity(&placedEntity{id: "wolf-1", kind: "monster"}, spatial.Position{X: 5, Y: 6}))

	fired := 0
	_, err := dnd5eEvents.ReactionTriggerTopic.On(bus).Subscribe(ctx,
		func(_ context.Context, _ dnd5eEvents.ReactionTriggerEvent) error { fired++; return nil })
	require.NoError(t, err)

	runCtx := gamectx.WithReactionReadiness(gamectx.WithRoom(ctx, room),
		gamectx.ReactionReadinessMap{reactor.GetID(): {refs.Conditions.OpportunityAttack().String(): true}})

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

	sheet, err := Load(ctx, data)
	require.NoError(t, err)
	require.NoError(t, Attach(ctx, sheet, events.NewEventBus()))

	after := sheet.ToData()
	require.Empty(t, after.Conditions,
		"a character's meter is ActionEconomy.ReactionsRemaining, so nothing needs writing "+
			"and the sheet writes back exactly what it was built from")
	require.False(t, sheet.IsDirty(), "gaining a reaction is not a change worth saving")
}

// A SHEET WITH NO ECONOMY CANNOT REACT, which is this package answering
// honestly rather than a rule about fights.
//
// The purse a character pays from is the action economy, and combat.Ledger
// answers "nothing left" rather than "unlimited" for a holder who is not in
// combat — the same refusal that stops an out-of-combat sheet spending an
// action it does not have.
//
// WHOSE JOB IT IS TO MAKE SURE THIS NEVER HAPPENS IN A FIGHT: the session's.
// Kirk ruled 2026-08-28 (rpg-project#316) that "characters should start with
// their full economy... when we go into a combat bubble, all players should
// have economy", which supersedes the lazy ignition recorded in
// session/economy.go ("the session lights the sheet when an actor on the fight
// clock first acts"). A combatant is lit when the bubble forms, so the state
// this test describes does not arise in play.
//
// It is still pinned, because the sheet must keep giving this answer for a
// holder who genuinely has no economy — and because "cannot consume it when it
// is not your turn" is the turn gate's job, not this refusal's. A reaction is
// spent on somebody else's turn by definition, so the two must not be confused.
func TestASheetWithNoEconomyCarriesTheReactionButCannotSpendIt(t *testing.T) {
	ctx := context.Background()
	sheet, err := Load(ctx, plainFighter("fighter-1"))
	require.NoError(t, err)

	bus := events.NewEventBus()
	require.NoError(t, Attach(ctx, sheet, bus))

	require.Equal(t, 0, triggersOnAStepAway(t, bus, sheet),
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

	sheet, err := Load(ctx, data)
	require.NoError(t, err)
	require.NoError(t, Attach(ctx, sheet, events.NewEventBus()))

	count := 0
	for _, c := range sheet.GetConditions() {
		if c.Ref() != nil && c.Ref().String() == refs.Conditions.OpportunityAttack().String() {
			count++
		}
	}
	require.Equal(t, 1, count, "the persisted one is kept and no second is carried on top")
	require.True(t, carries(sheet.GetConditions(), refs.Conditions.OpportunityAttack().String()))
}
