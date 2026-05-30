package encounter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ActivateFeatureInput is the request shape for Encounter.ActivateFeature.
//
// CharDataJSON is the caller-supplied serialized dnd5e character data (from the
// host's character store). It mirrors the MonsterInput/MonsterData.DataJSON
// pattern used by NPCAct — the encounter SDK does not store character data
// internally; the host (rpg-api) fetches it from its character store and passes
// it in, just as it does when seeding MonsterData.DataJSON for NPCAct.
type ActivateFeatureInput struct {
	// ActorID is the entity ID of the player character activating the feature.
	ActorID encountercore.EntityID

	// FeatureRef is the canonical ref string for the feature to activate,
	// e.g. "dnd5e:features:rage". Passed through to character.ActivateAbility.
	FeatureRef string

	// CharDataJSON is the serialized dnd5e character.Data for the actor.
	// Must include ActionEconomy (so the character is InCombat) and current
	// resource state. The verb loads the character from this data, runs
	// ActivateAbility, and returns the updated serialization in the output.
	CharDataJSON json.RawMessage
}

// ActivateFeatureOutput is the result shape for Encounter.ActivateFeature.
type ActivateFeatureOutput struct {
	// UpdatedCharData is the serialized dnd5e character.Data after the feature
	// activation (resource decremented, action economy consumed). The host
	// (rpg-api) persists this to its character store.
	UpdatedCharData json.RawMessage
}

// ActivateFeature activates a character feature in-encounter. It:
//  1. Validates the actor is a player seat in this encounter.
//  2. Loads the dnd5e character from CharDataJSON.
//  3. Subscribes to the encounter-scoped dnd5e bus to capture any
//     ConditionAppliedEvent that ActivateAbility publishes.
//  4. Calls char.ActivateAbility({AbilityRef: featureRef}).
//  5. Bridges each captured dnd5e ConditionAppliedEvent → broker
//     ConditionAppliedEvent (following the applyCapturedConditions template).
//  6. Diffs the full pre/post resource set and publishes a broker
//     ResourceChangedEvent for every resource whose current value changed.
//  7. Returns the updated character serialization for the host to persist.
//
// Design note: the encounter SDK already imports rulebooks/dnd5e/monster and
// calls monster.LoadFromData directly in NPCAct. ActivateFeature follows that
// same precedent, importing rulebooks/dnd5e/character and calling
// character.LoadFromData. Director accepted the NPCAct-precedent coupling for
// Wave 0; fully agnostic SDK is tracked separately.
func (e *Encounter) ActivateFeature(ctx context.Context, in *ActivateFeatureInput) (*ActivateFeatureOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("ActivateFeatureInput is nil")
	}
	if len(in.CharDataJSON) == 0 {
		return nil, fmt.Errorf("CharDataJSON is required")
	}

	// Validate the actor exists in this encounter.
	p := e.findPlayerByEntityID(in.ActorID)
	if p == nil {
		return nil, fmt.Errorf("actor %q not found in encounter", in.ActorID)
	}

	// Deserialize the character data.
	var charData dnd5eCharacter.Data
	if err := json.Unmarshal(in.CharDataJSON, &charData); err != nil {
		return nil, fmt.Errorf("unmarshal character data: %w", err)
	}

	// Validate that CharDataJSON belongs to the actor. This is the SDK's own
	// consistency guard: events are published under ActorID, so mismatched
	// data would produce condition/resource events for the wrong identity.
	// Player-ownership (ActorID == request.EntityID) stays at the rpg-api layer.
	if charData.ID != string(in.ActorID) {
		return nil, fmt.Errorf(
			"CharDataJSON identity mismatch: data.ID=%q does not match ActorID=%q",
			charData.ID, in.ActorID,
		)
	}

	// Snapshot the full resource set before activation so we can diff post.
	preSources := snapshotResources(charData.Resources)

	// Load the character onto the encounter-scoped dnd5e bus so that any
	// conditions already Applied at rehydration time (e.g. Sneak Attack) can
	// observe the activation event on the same persistent bus.
	char, err := dnd5eCharacter.LoadFromData(ctx, &charData, e.bus)
	if err != nil {
		return nil, fmt.Errorf("load character: %w", err)
	}
	// Tear down the character's bus subscriptions after the verb completes so
	// that repeated ActivateFeature calls on the same Encounter object (e.g. in
	// tests) do not accumulate duplicate subscribers on e.bus.
	// In production each RPC creates a fresh Encounter via LoadFromData (fresh
	// e.bus), but the SDK must be safe regardless. Mirrors the defer unsubCond()
	// pattern already used for the condition-capture subscriber.
	defer func() { _ = char.Cleanup(ctx) }()

	// Capture the condition the character's bus will emit during ActivateAbility.
	capturedCond, unsubCond, err := subscribeConditions(ctx, e.bus)
	if err != nil {
		return nil, fmt.Errorf("subscribe conditions: %w", err)
	}
	defer func() { _ = unsubCond() }()

	// Parse the feature ref string to a core.Ref.
	featureRef, err := core.ParseString(in.FeatureRef)
	if err != nil {
		return nil, fmt.Errorf("parse feature ref %q: %w", in.FeatureRef, err)
	}

	// Activate the feature via the character's own rules engine.
	out, err := char.ActivateAbility(ctx, &dnd5eCharacter.ActivateAbilityInput{
		AbilityRef: featureRef,
	})
	if err != nil {
		return nil, fmt.Errorf("activate ability: %w", err)
	}
	if !out.Success {
		return nil, fmt.Errorf("activate ability failed: %s", out.Error)
	}

	// Bridge captured dnd5e ConditionAppliedEvents → broker ConditionAppliedEvents.
	// Mirrors applyCapturedConditions (npc.go:776) — same template.
	if err := e.applyActivatedConditions(p, in.ActorID, *capturedCond); err != nil {
		return nil, fmt.Errorf("bridge conditions: %w", err)
	}

	// Serialize updated character data (post-activation).
	updatedData, err := json.Marshal(char.ToData())
	if err != nil {
		return nil, fmt.Errorf("marshal updated character: %w", err)
	}

	// Diff the full resource set and publish a ResourceChangedEvent for every
	// resource whose current value changed. Generic over all features — no
	// feature-specific hardcoding here.
	var postData dnd5eCharacter.Data
	if err := json.Unmarshal(updatedData, &postData); err != nil {
		return nil, fmt.Errorf("unmarshal updated character for resource diff: %w", err)
	}
	postSources := snapshotResources(postData.Resources)
	if err := e.publishChangedResources(in.ActorID, preSources, postSources); err != nil {
		return nil, fmt.Errorf("publish resource changes: %w", err)
	}

	return &ActivateFeatureOutput{
		UpdatedCharData: updatedData,
	}, nil
}

// resourceSnapshot is the pre/post shape for a single resource.
type resourceSnapshot struct {
	current int
	maximum int
}

// snapshotResources converts the character.Data.Resources map into a keyed
// snapshot map for pre/post diffing.
func snapshotResources(
	src map[coreResources.ResourceKey]dnd5eCharacter.RecoverableResourceData,
) map[coreResources.ResourceKey]resourceSnapshot {
	out := make(map[coreResources.ResourceKey]resourceSnapshot, len(src))
	for k, v := range src {
		out[k] = resourceSnapshot{current: v.Current, maximum: v.Maximum}
	}
	return out
}

// publishChangedResources emits a broker ResourceChangedEvent for every resource
// key whose Current value changed between pre and post snapshots. Generic over
// all features — no feature-specific key hardcoded.
func (e *Encounter) publishChangedResources(
	actorID encountercore.EntityID,
	pre, post map[coreResources.ResourceKey]resourceSnapshot,
) error {
	for key, postSnap := range post {
		preSnap, exists := pre[key]
		if !exists || postSnap.current == preSnap.current {
			continue
		}
		if err := e.publishResourceChanged(actorID, string(key), postSnap.current, postSnap.maximum); err != nil {
			return fmt.Errorf("publish resource changed for %q: %w", key, err)
		}
	}
	return nil
}

// applyActivatedConditions bridges dnd5e ConditionAppliedEvents emitted
// during a player feature activation to broker ConditionAppliedEvents.
// Template: applyCapturedConditions (npc.go:776).
func (e *Encounter) applyActivatedConditions(
	actor *PlayerData,
	actorID encountercore.EntityID,
	conds []dnd5eEvents.ConditionAppliedEvent,
) error {
	for _, cond := range conds {
		targetID := actorID
		condRef := string(cond.Type)

		condPerPlayer := make(map[encountercore.PlayerID]events.ConditionAppliedSlice)
		for viewerID, viewer := range e.data.Players {
			// Nil-guard: skip viewer if View is missing (malformed encounter data).
			if viewer == nil || viewer.View == nil {
				continue
			}
			// Nil-guard: skip if actor has no View (malformed encounter data).
			if actor == nil || actor.View == nil {
				break
			}
			if !e.viewerCanSee(viewer, actor.View.Position) {
				continue
			}
			condPerPlayer[viewerID] = events.ConditionAppliedSlice{Visible: true}
		}

		if err := e.broker.Publish(events.NewConditionAppliedEvent(
			e.data.ID, e.nextSeq(),
			targetID, actorID, condRef, 0, condPerPlayer,
		)); err != nil {
			return fmt.Errorf("publish condition applied: %w", err)
		}
	}
	return nil
}

// publishResourceChanged emits a broker ResourceChangedEvent for the given
// entity and resource. All players who can see the actor are included in
// the audience.
func (e *Encounter) publishResourceChanged(
	actorID encountercore.EntityID,
	resourceRef string,
	newCurrent, maxVal int,
) error {
	actor := e.findPlayerByEntityID(actorID)
	resPerPlayer := make(map[encountercore.PlayerID]events.ResourceChangedSlice)
	for viewerID, viewer := range e.data.Players {
		// Nil-guard: skip viewer or actor with missing View.
		if viewer == nil || viewer.View == nil {
			continue
		}
		if actor == nil || actor.View == nil {
			break
		}
		if e.viewerCanSee(viewer, actor.View.Position) {
			resPerPlayer[viewerID] = events.ResourceChangedSlice{Visible: true}
		}
	}

	return e.broker.Publish(events.NewResourceChangedEvent(
		e.data.ID, e.nextSeq(),
		actorID, resourceRef,
		newCurrent, maxVal,
		resPerPlayer,
	))
}
