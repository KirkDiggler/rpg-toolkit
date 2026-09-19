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
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// WrathOfTheStorm offers a reaction after the owner's qualifying hit. The
// owner holds its resource pool; this feature owns no second counter.
type WrathOfTheStorm struct {
	id, name, characterID string
	subscription          string
}

// WrathOfTheStormData persists the passive feature identity, not resource state.
type WrathOfTheStormData struct {
	Ref         *core.Ref `json:"ref"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CharacterID string    `json:"character_id"`
}

// Ref identifies this feature.
func (w *WrathOfTheStorm) Ref() *core.Ref { return refs.Features.WrathOfTheStorm() }

// Name returns the authored display name.
func (w *WrathOfTheStorm) Name() string { return w.name }

// GetID returns the feature's identity.
func (w *WrathOfTheStorm) GetID() string { return w.id }

// GetType identifies a class feature.
func (w *WrathOfTheStorm) GetType() core.EntityType { return EntityTypeFeature }

// ActionType identifies its reaction cost, never an ordinary activation.
func (w *WrathOfTheStorm) ActionType() coreCombat.ActionType { return coreCombat.ActionReaction }

// CanActivate refuses standalone use: only an offered hit reaction can pay for Wrath.
func (w *WrathOfTheStorm) CanActivate(context.Context, core.Entity, FeatureInput) error {
	return fmt.Errorf("Wrath of the Storm is offered only after a hit")
}

// Activate refuses standalone use without spending anything.
func (w *WrathOfTheStorm) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	return w.CanActivate(ctx, owner, input)
}

// Status projects the owner's authoritative resource pool.
func (w *WrathOfTheStorm) Status(in *StatusInput) (*StatusOutput, error) {
	if in == nil || in.Owner == nil {
		return nil, fmt.Errorf("Wrath of the Storm requires an owner resource reader")
	}
	current, maximum, ok := in.Owner.ResourceStatus(resources.WrathOfTheStorm)
	if !ok {
		return nil, fmt.Errorf("owner does not carry Wrath of the Storm")
	}
	return &StatusOutput{Status: &Status{Ref: *w.Ref(), Name: w.name, Resource: &ResourceStatus{Key: resources.WrathOfTheStorm, Name: w.name, Current: current, Maximum: maximum}}}, nil
}

// Apply attaches the offer hook. The modifier receives the publisher's context.
func (w *WrathOfTheStorm) Apply(ctx context.Context, bus events.EventBus) error {
	if w.subscription != "" {
		return fmt.Errorf("Wrath of the Storm already attached")
	}
	id, err := dndEvents.PostHitChain.On(bus).SubscribeWithChain(ctx, w.onHit)
	if err != nil {
		return err
	}
	w.subscription = id
	return nil
}

// Remove revokes the offer hook.
func (w *WrathOfTheStorm) Remove(ctx context.Context, bus events.EventBus) error {
	if w.subscription == "" {
		return nil
	}
	if err := bus.Unsubscribe(ctx, w.subscription); err != nil {
		return err
	}
	w.subscription = ""
	return nil
}
func (w *WrathOfTheStorm) onHit(_ context.Context, e *dndEvents.PostHitEvent, c chain.Chain[*dndEvents.PostHitEvent]) (chain.Chain[*dndEvents.PostHitEvent], error) {
	if e == nil || e.TargetID != w.characterID {
		return c, nil
	}
	err := c.Add(combat.StageConditions, "wrath_of_the_storm_"+w.characterID, func(ctx context.Context, e *dndEvents.PostHitEvent) (*dndEvents.PostHitEvent, error) {
		cast, ok := gamectx.CastOf(ctx)
		if !ok {
			return e, nil
		}
		owner, ok := cast.Member(w.characterID)
		if !ok || combat.IsDown(owner) || !owner.CanReact() {
			return e, nil
		}
		if conditions, ok := owner.(interface{ HasCondition(*core.Ref) bool }); ok && conditions.HasCondition(refs.Conditions.Incapacitated()) {
			return e, nil
		}
		reader, ok := owner.(ResourceReader)
		if !ok {
			return e, nil
		}
		current, _, known := reader.ResourceStatus(resources.WrathOfTheStorm)
		if !known || current < 1 {
			return e, nil
		}
		sight, ok := cast.(interface {
			SeesWithin(string, string, int) (bool, bool)
		})
		if !ok {
			return e, nil
		}
		visible, known := sight.SeesWithin(w.characterID, e.AttackerID, 5)
		if !known || !visible {
			return e, nil
		}
		dc := 8 + owner.ProficiencyBonus() + owner.AbilityScores().Modifier(abilities.WIS)
		e.Offers = append(e.Offers, dndEvents.PostHitOffer{ReactorID: w.characterID, Ref: *w.Ref(), Name: w.name, ResourceKey: string(resources.WrathOfTheStorm), Options: []dndEvents.PostHitOption{
			{ID: "lightning", Label: "Lightning", Ability: abilities.DEX, DC: dc, Dice: "2d8", DamageType: damage.Lightning, HalfOnSave: true},
			{ID: "thunder", Label: "Thunder", Ability: abilities.DEX, DC: dc, Dice: "2d8", DamageType: damage.Thunder, HalfOnSave: true},
		}})
		return e, nil
	})
	return c, err
}

// ToJSON persists feature identity; character serialization owns its resource pool.
func (w *WrathOfTheStorm) ToJSON() (json.RawMessage, error) {
	return json.Marshal(WrathOfTheStormData{Ref: w.Ref(), ID: w.id, Name: w.name, CharacterID: w.characterID})
}
func (w *WrathOfTheStorm) loadJSON(data json.RawMessage) error {
	var d WrathOfTheStormData
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	if d.CharacterID == "" {
		return fmt.Errorf("Wrath of the Storm requires its owner")
	}
	w.id, w.name, w.characterID = d.ID, d.Name, d.CharacterID
	return nil
}
