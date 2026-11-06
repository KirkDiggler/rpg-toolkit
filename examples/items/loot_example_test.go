// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// This adapter is the example HOST, not item-library persistence. The two paths
// make catalog content and existing instance state distinct. No encounter is
// needed to build, store or retrieve either. It is not a production store layout.
type itemArtifactPaths struct{ Definition, Instance string }

func artifactPaths(dir string) itemArtifactPaths {
	return itemArtifactPaths{Definition: filepath.Join(dir, "definition.json"), Instance: filepath.Join(dir, "instance.json")}
}

func saveArtifact(path string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o600)
}

func retrieveBuiltItem(paths itemArtifactPaths) (*items.Item, error) {
	definitionBytes, err := os.ReadFile(paths.Definition)
	if err != nil {
		return nil, err
	}
	instanceBytes, err := os.ReadFile(paths.Instance)
	if err != nil {
		return nil, err
	}
	var definition items.Definition
	if err := json.Unmarshal(definitionBytes, &definition); err != nil {
		return nil, err
	}
	var state items.Data
	if err := json.Unmarshal(instanceBytes, &state); err != nil {
		return nil, err
	}
	// State is ALWAYS non-nil on retrieval. Missing/null/incomplete storage
	// must not be treated as permission to mint a fresh full-budget item.
	return items.New(items.Config{
		ID: state.ID, Definition: definition, State: &state,
		RecoveryTriggers: []items.Trigger{shortRest, longRest},
		ValidateGrant: func(grant items.Grant) error {
			switch grant.Ref {
			case rageRef:
				_, err := parseRage(grant)
				return err
			case windRef:
				return validateSecondWind(grant)
			default:
				return fmt.Errorf("unknown example grant %s", grant.Ref.String())
			}
		},
	})
}

func TestBuiltItemIsRetrievedBeforeAnEncounterAndLootPreservesIt(t *testing.T) {
	ctx := context.Background()
	cfg := rageItemConfig("vault-ring-17")
	// An authored variation proves retrieval consumes the saved definition,
	// rather than silently rebuilding the default fixture with a +2 bonus.
	grant := cfg.Definition.Grants[rageGrant]
	grant.Parameters = json.RawMessage(`{"damage_bonus":3}`)
	cfg.Definition.Grants[rageGrant] = grant
	built, err := items.New(cfg)
	require.NoError(t, err)
	paths := artifactPaths(t.TempDir())
	require.NoError(t, saveArtifact(paths.Definition, cfg.Definition))
	require.NoError(t, saveArtifact(paths.Instance, built.Data()))
	looted, err := retrieveBuiltItem(paths)
	require.NoError(t, err)
	require.Equal(t, built.GetID(), looted.GetID())
	require.Equal(t, cfg.Definition.Ref, looted.Data().DefinitionRef)
	outside, err := looted.Offers(items.OffersInput{HolderID: rageHolder})
	require.NoError(t, err)
	require.Empty(t, outside.Grants, "a retrieved item is not an encounter association")

	// Only now does an encounter-like consumer exist and give this existing
	// instance to a holder. No encounter/session package is imported.
	bus := events.NewEventBus()
	require.NoError(t, looted.Attach(ctx, items.AttachInput{Bus: bus, HolderID: rageHolder}))
	t.Cleanup(func() { require.NoError(t, looted.Detach(ctx)) })
	used, err := useRage(ctx, looted, rageHolder)
	require.NoError(t, err)
	require.NoError(t, used.Result.Effect.Apply(ctx, bus))
	t.Cleanup(func() { require.NoError(t, used.Result.Effect.Remove(ctx, bus)) })
	dealt, err := rageDamage(ctx, bus, rageHolder, abilities.STR, true, true)
	require.NoError(t, err)
	total := 0
	for _, component := range dealt.Components {
		total += component.Total()
	}
	require.Equal(t, 13, total, "the retrieved +3 item, not a default +2 item")
	require.Equal(t, "vault-ring-17", used.Source.ItemID)
	require.NoError(t, saveArtifact(paths.Instance, looted.Data()))
	require.NoError(t, dndevents.CombatEndTopic.On(bus).Publish(ctx, dndevents.CombatEndEvent{SubjectID: rageHolder}))
	require.NoError(t, looted.Detach(ctx))

	again, err := retrieveBuiltItem(paths) // back outside any encounter
	require.NoError(t, err)
	require.Equal(t, "vault-ring-17", again.GetID())
	require.Zero(t, again.Data().Pools[chargePool])
	secondBus := events.NewEventBus()
	require.NoError(t, again.Attach(ctx, items.AttachInput{Bus: secondBus, HolderID: "next-looter"}))
	t.Cleanup(func() { require.NoError(t, again.Detach(ctx)) })
	_, err = useRage(ctx, again, "next-looter")
	require.ErrorIs(t, err, items.ErrUnavailable, "retrieving/giving loot is not a recharge")
}

func TestMissingOrNullStoredInstanceDoesNotCreateFreshLoot(t *testing.T) {
	paths := artifactPaths(t.TempDir())
	require.NoError(t, saveArtifact(paths.Definition, rageItemDefinition()))
	_, err := retrieveBuiltItem(paths)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, os.WriteFile(paths.Instance, []byte("null"), 0o600))
	_, err = retrieveBuiltItem(paths)
	require.ErrorIs(t, err, items.ErrInvalidConfig)
}

var errRefusedSubscription = errors.New("example bus refused attachment")

type refusingSubscriptions struct{ events.EventBus }

func (b refusingSubscriptions) Subscribe(context.Context, events.Topic, any) (string, error) {
	return "", errRefusedSubscription
}

func TestHostCommitsItemStateOnlyAfterEffectDeliverySucceeds(t *testing.T) {
	ctx := context.Background()
	original, err := items.New(rageItemConfig("ring"))
	require.NoError(t, err)
	paths := artifactPaths(t.TempDir())
	require.NoError(t, saveArtifact(paths.Definition, rageItemDefinition()))
	require.NoError(t, saveArtifact(paths.Instance, original.Data()))
	working, err := retrieveBuiltItem(paths)
	require.NoError(t, err)
	bus := events.NewEventBus()
	require.NoError(t, working.Attach(ctx, items.AttachInput{Bus: bus, HolderID: rageHolder}))
	prepared, err := useRage(ctx, working, rageHolder)
	require.NoError(t, err)
	require.Zero(t, working.Data().Pools[chargePool], "Use has already spent the runtime charge")
	err = prepared.Result.Effect.Apply(ctx, refusingSubscriptions{EventBus: events.NewEventBus()})
	require.ErrorIs(t, err, errRefusedSubscription)
	require.False(t, prepared.Result.Effect.IsApplied())
	require.NoError(t, working.Detach(ctx))
	// The host does NOT save this failed operation's working copy. This is
	// not a claim that Use magically undid a later Apply failure.
	retried, err := retrieveBuiltItem(paths)
	require.NoError(t, err)
	require.Equal(t, 1, retried.Data().Pools[chargePool])
	require.NoError(t, retried.Attach(ctx, items.AttachInput{Bus: bus, HolderID: rageHolder}))
	t.Cleanup(func() { require.NoError(t, retried.Detach(ctx)) })
	success, err := useRage(ctx, retried, rageHolder)
	require.NoError(t, err)
	require.NoError(t, success.Result.Effect.Apply(ctx, bus))
	t.Cleanup(func() { require.NoError(t, success.Result.Effect.Remove(ctx, bus)) })
	require.NoError(t, saveArtifact(paths.Instance, retried.Data()))
	committed, err := retrieveBuiltItem(paths)
	require.NoError(t, err)
	require.Zero(t, committed.Data().Pools[chargePool])
}

func Example_rageLoot() {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "rpg-item-loot-example-")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			panic(err)
		}
	}()
	paths := artifactPaths(dir)
	built, err := items.New(rageItemConfig("loot-ring-17"))
	if err != nil {
		panic(err)
	}
	if err := saveArtifact(paths.Definition, rageItemDefinition()); err != nil {
		panic(err)
	}
	if err := saveArtifact(paths.Instance, built.Data()); err != nil {
		panic(err)
	}
	looted, err := retrieveBuiltItem(paths)
	if err != nil {
		panic(err)
	}
	fmt.Printf("retrieved outside encounter: %s; charges %d\n", looted.GetID(), looted.Data().Pools[chargePool])
	bus := events.NewEventBus()
	if err := looted.Attach(ctx, items.AttachInput{Bus: bus, HolderID: rageHolder}); err != nil {
		panic(err)
	}
	result, err := useRage(ctx, looted, rageHolder)
	if err != nil {
		panic(err)
	}
	fmt.Printf("prepared Rage: applied %t; charges %d\n", result.Result.Effect.IsApplied(), result.Remaining)
	if err := result.Result.Effect.Apply(ctx, bus); err != nil {
		panic(err)
	}
	dealt, err := rageDamage(ctx, bus, rageHolder, abilities.STR, true, true)
	if err != nil {
		panic(err)
	}
	total := 0
	for _, component := range dealt.Components {
		total += component.Total()
	}
	fmt.Printf("raging Strength melee damage: %d\n", total)
	if err := looted.Detach(ctx); err != nil {
		panic(err)
	}
	fmt.Printf("source detached; active effect remains: %t\n", result.Result.Effect.IsApplied())
	if err := dndevents.CombatEndTopic.On(bus).Publish(ctx, dndevents.CombatEndEvent{SubjectID: rageHolder}); err != nil {
		panic(err)
	}
	fmt.Printf("combat ended; effect active: %t; charges %d\n", result.Result.Effect.IsApplied(), looted.Data().Pools[chargePool])
	if err := saveArtifact(paths.Instance, looted.Data()); err != nil {
		panic(err)
	}
	retrievedAgain, err := retrieveBuiltItem(paths)
	if err != nil {
		panic(err)
	}
	fmt.Printf("retrieved after encounter: %s; charges %d\n", retrievedAgain.GetID(), retrievedAgain.Data().Pools[chargePool])
	// Output:
	// retrieved outside encounter: loot-ring-17; charges 1
	// prepared Rage: applied false; charges 0
	// raging Strength melee damage: 12
	// source detached; active effect remains: true
	// combat ended; effect active: false; charges 0
	// retrieved after encounter: loot-ring-17; charges 0
}
