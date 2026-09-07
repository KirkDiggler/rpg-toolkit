// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// BardicInspiration is the bard's level-1 feature: a bonus action that hands
// an ally a die.
//
// # It grants across entities, and it is the second thing that does
//
// Help is the first (combatabilities/help.go): the ability publishes a
// ConditionAppliedEvent naming a target OTHER than the activator, and the
// recipient's own SheetKeeper is what applies it. This takes the same path,
// which is why it needs no new machinery to put a condition on somebody else's
// sheet.
//
// # What it does not do is decide whether the ally is reachable
//
// Range, line of sight, whether the ally is standing, and whether they already
// hold a die are all questions about the BOARD, and only the session holds
// positions and sight. This feature is handed a target and grants to it; the
// candidate universe upstream is where a target 65 feet away is refused, with
// a reason a player can read.
type BardicInspiration struct {
	id   string
	name string
}

// BardicInspirationData is the JSON structure for persisting the feature.
// The uses pool is owned by the Character, not by the feature.
type BardicInspirationData struct {
	Ref  *core.Ref `json:"ref"`
	ID   string    `json:"id"`
	Name string    `json:"name"`
}

// NewBardicInspiration creates the feature. It holds no character id because
// it owns no state about one: the uses are a Character resource and the die
// lands on whoever is targeted, so there is nothing here to key by owner.
func NewBardicInspiration() *BardicInspiration {
	return &BardicInspiration{
		id:   refs.Features.BardicInspiration().ID,
		name: conditions.InspiredName,
	}
}

// Ref returns the unique ref for the Bardic Inspiration feature.
func (b *BardicInspiration) Ref() *core.Ref { return refs.Features.BardicInspiration() }

// Name returns the display name.
func (b *BardicInspiration) Name() string { return b.name }

// GetID implements core.Entity.
func (b *BardicInspiration) GetID() string { return b.id }

// GetType implements core.Entity.
func (b *BardicInspiration) GetType() core.EntityType { return EntityTypeFeature }

// ActionType is a bonus action — RAW, and the shape the panel draws.
func (b *BardicInspiration) ActionType() combat.ActionType { return combat.ActionBonus }

// Status reports the character-owned inspiration pool. The feature does not own
// it — the Character does, the way Rage does not own its charges — so an owner
// that cannot report the pool is a malformed sheet and says so.
func (b *BardicInspiration) Status(in *StatusInput) (*StatusOutput, error) {
	if in == nil || in.Owner == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"bardic inspiration status requires an owner resource reader")
	}
	current, maximum, ok := in.Owner.ResourceStatus(resources.Inspiration)
	if !ok {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not carry inspiration")
	}
	if err := validateResourceBounds("bardic inspiration", current, maximum); err != nil {
		return nil, err
	}
	name := b.name
	if name == "" {
		name = conditions.InspiredName
	}
	return &StatusOutput{Status: &Status{
		Ref:  *refs.Features.BardicInspiration(),
		Name: name,
		Resource: &ResourceStatus{
			Key:     resources.Inspiration,
			Name:    conditions.InspiredName,
			Current: current,
			Maximum: maximum,
		},
	}}, nil
}

// CanActivate refuses when the pool is empty, and says nothing about the
// target.
//
// The target is deliberately not checked here: this runs on every Afford to
// decide whether the row is available, and it runs with an empty FeatureInput
// (character.buildAvailableAbilities). A target check would report every bard
// as unable to inspire anyone, always. Whether a target was named is enforced
// upstream, off the ability's own TargetKind, before anything is spent.
func (b *BardicInspiration) CanActivate(_ context.Context, owner core.Entity, _ FeatureInput) error {
	accessor, ok := owner.(coreResources.ResourceAccessor)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not implement ResourceAccessor")
	}
	if !accessor.IsResourceAvailable(resources.Inspiration) {
		return rpgerr.New(rpgerr.CodeResourceExhausted, "no bardic inspiration uses remaining")
	}
	return nil
}

// Activate spends one use and publishes the die onto the ally's sheet.
//
// The order is spend-then-publish, and it is the order that fails safely: a
// publish that errors leaves a use spent on a grant that did not land, which
// the caller reports; a grant that landed with nothing spent would be a free
// die nobody could find. The action slot is not touched here — the Character
// consumes it around this call, as it does for every feature.
func (b *BardicInspiration) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	if err := b.CanActivate(ctx, owner, input); err != nil {
		return err
	}
	if input.Bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus required for bardic inspiration")
	}
	if input.Target == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "target ally required for bardic inspiration")
	}
	if input.Target.GetID() == owner.GetID() {
		// RAW: "a creature other than yourself". Refused here as well as in
		// the candidate universe above, because this is the one place that
		// holds both identities at once and a self-grant that got this far
		// would spend a use on nothing.
		return rpgerr.New(rpgerr.CodeInvalidArgument, "bardic inspiration cannot target yourself")
	}

	accessor, ok := owner.(coreResources.ResourceAccessor)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "owner does not implement ResourceAccessor")
	}
	if err := accessor.UseResource(resources.Inspiration, 1); err != nil {
		return rpgerr.Wrapf(err, "failed to spend bardic inspiration")
	}

	inspired := conditions.NewInspiredCondition(
		input.Target.GetID(), owner.GetID(), conditions.InspiredDie,
	)
	if err := dnd5eEvents.ConditionAppliedTopic.On(input.Bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    input.Target,
		Type:      dnd5eEvents.ConditionInspired,
		Source:    dnd5eEvents.ConditionSourceFeature,
		Condition: inspired,
	}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish inspired condition")
	}

	return nil
}

// ToJSON converts the feature to JSON for persistence.
func (b *BardicInspiration) ToJSON() (json.RawMessage, error) {
	data := BardicInspirationData{Ref: b.Ref(), ID: b.id, Name: b.name}
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal bardic inspiration data: %w", err)
	}
	return bytes, nil
}

// loadJSON loads the feature back from JSON.
func (b *BardicInspiration) loadJSON(data json.RawMessage) error {
	var stored BardicInspirationData
	if err := json.Unmarshal(data, &stored); err != nil {
		return fmt.Errorf("failed to unmarshal bardic inspiration data: %w", err)
	}
	b.id = stored.ID
	if b.id == "" {
		b.id = refs.Features.BardicInspiration().ID
	}
	b.name = stored.Name
	if b.name == "" {
		b.name = conditions.InspiredName
	}
	return nil
}
