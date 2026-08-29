// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type placed struct {
	id   string
	kind core.EntityType
}

func (p *placed) GetID() string            { return p.id }
func (p *placed) GetType() core.EntityType { return p.kind }

// carriedOA is the blob a seated opportunity attack leaves on a monster's
// sheet, written by the condition itself rather than by hand so the test cannot
// drift from the shape ToData actually produces.
func carriedOA(t *testing.T, id string) json.RawMessage {
	t.Helper()
	raw, err := conditions.NewOpportunityAttackCondition(id).ToJSON()
	require.NoError(t, err)

	return raw
}

// A monster's sheet has always been documented as carrying "runtime state:
// poisoned, hidden, etc.", and this loader knew four TRAITS and errored on
// everything else — so a monster that ever persisted an ordinary condition
// could not be loaded again.
//
// Nothing had put one there, which is why the gap was invisible. Seating the
// universal opportunity attack puts one there, and without this the first
// interaction writes an OA blob onto a wolf and the second refuses to load it,
// breaking every monster in the game.
func TestAMonsterCanCarryAnOrdinaryCondition(t *testing.T) {
	data := &monster.Data{
		ID: "wolf-1", Name: "Wolf", HitPoints: 11, MaxHitPoints: 11, ArmorClass: 13,
		Conditions: []json.RawMessage{carriedOA(t, "wolf-1")},
	}

	m, err := monster.Load(context.Background(), data)
	require.NoError(t, err, "a sheet carrying a condition must load")

	bus := events.NewEventBus()
	require.NoError(t, AttachMonster(context.Background(), m, bus, dice.NewRoller()),
		"and must attach without the trait loader refusing its ref")

	require.Len(t, m.GetConditions(), 1, "the condition is on the sheet, not merely loadable")
	require.Equal(t, refs.Conditions.OpportunityAttack().String(), m.GetConditions()[0].Ref().String())
}

// The owner handoff character.Attach has made since rpg-toolkit#1178, now that
// a monster can carry a condition that needs it.
//
// The opportunity attack's once-per-turn meter is stored on the condition and
// serialized as part of this sheet, so nothing else notices when it changes:
// without the handoff the flag is set, the sheet never goes dirty, the save is
// dropped, and a wolf reacts again on the very next call. The meter would exist
// and mean nothing.
func TestACarriedConditionIsHandedItsOwnSheet(t *testing.T) {
	ctx := context.Background()
	data := &monster.Data{
		ID: "wolf-1", Name: "Wolf", HitPoints: 11, MaxHitPoints: 11, ArmorClass: 13,
		Conditions: []json.RawMessage{carriedOA(t, "wolf-1")},
	}

	m, err := monster.Load(ctx, data)
	require.NoError(t, err)
	require.False(t, m.IsDirty(), "a freshly loaded sheet has nothing to save")

	bus := events.NewEventBus()
	require.NoError(t, AttachMonster(ctx, m, bus, dice.NewRoller()))

	// A room with the wolf next to a rogue who then runs, which is the
	// condition's whole predicate.
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID: "r", Type: "dungeon",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 20, Height: 20}),
	})
	require.NoError(t, room.PlaceEntity(&placed{id: "wolf-1", kind: "monster"}, spatial.Position{X: 5, Y: 5}))
	require.NoError(t, room.PlaceEntity(&placed{id: "rogue-1", kind: "character"}, spatial.Position{X: 5, Y: 6}))

	fired := 0
	_, err = dnd5eEvents.ReactionTriggerTopic.On(bus).Subscribe(ctx,
		func(_ context.Context, _ dnd5eEvents.ReactionTriggerEvent) error { fired++; return nil })
	require.NoError(t, err)

	// The wolf's own sheet in the cast, the way resolution's one door installs
	// it. The reaction gate reads the reactor's sheet now, and a monster's
	// answer — nothing here refuses a reaction — is the sheet's to give.
	runCtx := gamectx.WithReactionReadiness(gamectx.WithRoom(castOf(ctx, m), room),
		gamectx.ReactionReadinessMap{"wolf-1": {refs.Conditions.OpportunityAttack().String(): true}})

	event := &dnd5eEvents.MovementChainEvent{
		EntityID:   "rogue-1",
		EntityType: "character",
		// ONE CELL. MovementChainEvent documents a single step, and the
		// predicate does not need more than one: the wolf is at (5,5), so
		// (5,6) is inside its reach and (5,7) is outside it. A three-cell jump
		// exercised a shape the event contract does not permit and would hide a
		// step-specific bug behind the extra distance.
		FromPosition: dnd5eEvents.Position{X: 5, Y: 6},
		ToPosition:   dnd5eEvents.Position{X: 5, Y: 7},
	}
	staged := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
	folded, err := dnd5eEvents.MovementChain.On(bus).PublishWithChain(runCtx, event, staged)
	require.NoError(t, err)
	_, err = folded.Execute(runCtx, event)
	require.NoError(t, err)

	require.Equal(t, 1, fired, "the carried condition really is live on the bus")
	require.True(t, m.IsDirty(),
		"the condition spent its meter and said so — a silent update is a dropped save")
}

// The substitution this package now depends on: a loaded entry names itself
// with the SAME ref its persisted blob routes on.
//
// Attribution used to be taken by peeking the ref back out of the JSON, and is
// now taken from ConditionBehavior.Ref (rpg-toolkit#971). Those are only
// interchangeable if every routed entry agrees with its own blob — a trait whose
// Ref returned something else would silently file its subscriptions under the
// wrong effect, which no behavioural test would notice because the subscriptions
// still work.
func TestEveryLoadedEntryNamesItselfWithItsPersistedRef(t *testing.T) {
	built := []dnd5eEvents.ConditionBehavior{
		Immunity("wolf-1", damage.Fire),
		Vulnerability("wolf-1", damage.Cold),
		PackTactics("wolf-1"),
		UndeadFortitude("wolf-1", 3, dice.NewRoller()),
		conditions.NewOpportunityAttackCondition("wolf-1"),
	}
	require.Len(t, built, len(AllTraitRefs())+1,
		"every routed trait is covered here, plus the condition route this PR adds")

	for _, entry := range built {
		blob, err := entry.ToJSON()
		require.NoError(t, err)

		var peek struct {
			Ref core.Ref `json:"ref"`
		}
		require.NoError(t, json.Unmarshal(blob, &peek))

		loaded, err := LoadJSON(blob, dice.NewRoller())
		require.NoError(t, err, "%s must round-trip through the loader", peek.Ref)

		require.NotNil(t, loaded.Ref(), "%s: Ref must never return nil", peek.Ref)
		require.Equal(t, peek.Ref.String(), loaded.Ref().String(),
			"%s names itself differently than its blob routes on — attribution would be filed wrong",
			peek.Ref)
	}
}

// The monster half of Kirk's ruling: a monster has NO action economy at all, so
// the condition's own UsedThisTurn is the only meter there is — and a meter that
// is not written down is a wolf that reacts again on the very next call.
//
// So unlike a character, whose carried reaction stays live-but-unwritten because
// ActionEconomy.ReactionsRemaining is already its persisted meter, a monster's
// joins the sheet and is serialized by ToData.
func TestAnAttachedMonsterCarriesItsFreeReactionOnTheSheet(t *testing.T) {
	ctx := context.Background()
	data := &monster.Data{ID: "wolf-1", Name: "Wolf", HitPoints: 11, MaxHitPoints: 11, ArmorClass: 13}

	m, err := monster.Load(ctx, data)
	require.NoError(t, err)
	require.Empty(t, m.ToData().Conditions, "load is a pure read and grants nothing")

	require.NoError(t, AttachMonster(ctx, m, events.NewEventBus(), dice.NewRoller()))

	require.Len(t, m.GetConditions(), 1)
	require.Equal(t, refs.Conditions.OpportunityAttack().String(), m.GetConditions()[0].Ref().String())
	require.Len(t, m.ToData().Conditions, 1,
		"written down, because UsedThisTurn is the only meter a monster has")
	require.False(t, m.IsDirty(),
		"but gaining it is not a change worth saving — only spending the meter is")
}

// A monster that already persisted one — every monster, from its second attach
// onward — is not given a duplicate on top of it.
func TestAMonsterIsNotGivenASecondCopyOfWhatItAlreadyCarries(t *testing.T) {
	ctx := context.Background()
	data := &monster.Data{
		ID: "wolf-1", Name: "Wolf", HitPoints: 11, MaxHitPoints: 11, ArmorClass: 13,
		Conditions: []json.RawMessage{carriedOA(t, "wolf-1")},
	}

	m, err := monster.Load(ctx, data)
	require.NoError(t, err)
	require.NoError(t, AttachMonster(ctx, m, events.NewEventBus(), dice.NewRoller()))

	require.Len(t, m.GetConditions(), 1, "the persisted one is kept and no second is carried on top")
	require.Len(t, m.ToData().Conditions, 1)
}
