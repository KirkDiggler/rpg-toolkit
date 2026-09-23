// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// WardingFlare offers a reaction before an attack against its owner is rolled. The
// owner holds its resource pool; this feature owns no second counter.
type WardingFlare struct {
	id, name, characterID string
	subscription          string
}

// WardingFlareData persists the passive feature identity, not resource state.
type WardingFlareData struct {
	Ref         *core.Ref `json:"ref"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CharacterID string    `json:"character_id"`
}

// Ref identifies this feature.
func (w *WardingFlare) Ref() *core.Ref { return refs.Features.WardingFlare() }

// Name returns the authored display name.
func (w *WardingFlare) Name() string { return w.name }

// GetID returns the feature's identity.
func (w *WardingFlare) GetID() string { return w.id }

// GetType identifies a class feature.
func (w *WardingFlare) GetType() core.EntityType { return EntityTypeFeature }

// ActionType identifies its reaction cost, never an ordinary activation.
func (w *WardingFlare) ActionType() coreCombat.ActionType { return coreCombat.ActionReaction }

// CanActivate refuses standalone use: only an offered attack reaction can pay for Warding Flare.
func (w *WardingFlare) CanActivate(context.Context, core.Entity, FeatureInput) error {
	return fmt.Errorf("warding flare is offered only before an attack roll")
}

// Activate refuses standalone use without spending anything.
func (w *WardingFlare) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	return w.CanActivate(ctx, owner, input)
}

// Status projects the owner's authoritative resource pool.
func (w *WardingFlare) Status(in *StatusInput) (*StatusOutput, error) {
	if in == nil || in.Owner == nil {
		return nil, fmt.Errorf("warding flare requires an owner resource reader")
	}
	current, maximum, ok := in.Owner.ResourceStatus(resources.WardingFlare)
	if !ok {
		return nil, fmt.Errorf("owner does not carry Warding Flare")
	}
	return &StatusOutput{Status: &Status{Ref: *w.Ref(), Name: w.name, Resource: &ResourceStatus{Key: resources.WardingFlare, Name: w.name, Current: current, Maximum: maximum}}}, nil
}

// Apply attaches the offer hook. The modifier receives the publisher's context.
func (w *WardingFlare) Apply(ctx context.Context, bus events.EventBus) error {
	if w.subscription != "" {
		return fmt.Errorf("warding flare already attached")
	}
	id, err := dndEvents.AttackChain.On(bus).SubscribeWithChain(ctx, w.onAttack)
	if err != nil {
		return err
	}
	w.subscription = id
	return nil
}

// Remove revokes the offer hook.
func (w *WardingFlare) Remove(ctx context.Context, bus events.EventBus) error {
	if w.subscription == "" {
		return nil
	}
	if err := bus.Unsubscribe(ctx, w.subscription); err != nil {
		return err
	}
	w.subscription = ""
	return nil
}
func (w *WardingFlare) onAttack(_ context.Context, e dndEvents.AttackChainEvent, c chain.Chain[dndEvents.AttackChainEvent]) (chain.Chain[dndEvents.AttackChainEvent], error) {
	if e.TargetID != w.characterID {
		return c, nil
	}
	err := c.Add(combat.StageConditions, "warding_flare_"+w.characterID, func(ctx context.Context, e dndEvents.AttackChainEvent) (dndEvents.AttackChainEvent, error) {
		cast, ok := gamectx.CastOf(ctx)
		if !ok {
			return e, nil
		}
		owner, ok := cast.Member(w.characterID)
		if !ok || combat.IsDown(owner) || !owner.CanReact() {
			return e, nil
		}
		state, ok := cast.(interface {
			ResourceStatus(string, coreResources.ResourceKey) (int, int, bool)
			HasCondition(string, *core.Ref) (bool, bool)
		})
		if !ok {
			return e, nil
		}
		if incapacitated, known := state.HasCondition(w.characterID, refs.Conditions.Incapacitated()); !known || incapacitated {
			return e, nil
		}
		current, _, known := state.ResourceStatus(w.characterID, resources.WardingFlare)
		if !known || current < 1 {
			return e, nil
		}

		sight, ok := cast.(interface {
			SeesWithin(string, string, int) (bool, bool)
		})
		if !ok {
			return e, nil
		}
		visible, known := sight.SeesWithin(w.characterID, e.AttackerID, 30)
		if !known || !visible {
			return e, nil
		}
		// Deferred: suppress this offer (and explain why) when the shared
		// condition-immunity model can report immunity to blindness. Eligibility
		// belongs here, before the reaction is offered, not in strike resolution.
		e.BeforeRollOffers = append(e.BeforeRollOffers, dndEvents.AttackRollOffer{
			ReactorID: w.characterID, Ref: *w.Ref(), Name: w.name,
			ResourceKey:  string(resources.WardingFlare),
			Disadvantage: dndEvents.AttackModifierSource{SourceRef: w.Ref(), SourceID: w.characterID, Reason: w.name},
		})
		return e, nil
	})
	return c, err
}

// ToJSON persists feature identity; character serialization owns its resource pool.
func (w *WardingFlare) ToJSON() (json.RawMessage, error) {
	return json.Marshal(WardingFlareData{Ref: w.Ref(), ID: w.id, Name: w.name, CharacterID: w.characterID})
}
func (w *WardingFlare) loadJSON(data json.RawMessage) error {
	var d WardingFlareData
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	if d.CharacterID == "" {
		return fmt.Errorf("warding flare requires its owner")
	}
	w.id, w.name, w.characterID = d.ID, d.Name, d.CharacterID
	return nil
}
