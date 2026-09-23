// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

const tricksterBlessingName = "Blessing of the Trickster (Not yet implemented)"

// BlessingOfTheTrickster exposes the level-1 grant without offering a working
// activation. Mechanics are deferred until the player-facing Stealth workflow.
// Agreed rules: one action, self or a willing creature within touch range,
// advantage on Stealth checks, no slot/uses/concentration. The blessing ends
// when its originating cleric takes a long rest or blesses another recipient;
// it does not count clock ticks. No effect is applied by this placeholder.
type BlessingOfTheTrickster struct{}

// Ref identifies the domain feature.
func (*BlessingOfTheTrickster) Ref() *core.Ref { return refs.Features.BlessingOfTheTrickster() }

// Name includes the implementation status on every existing display surface.
func (*BlessingOfTheTrickster) Name() string { return tricksterBlessingName }

// GetID returns the stable feature identity.
func (b *BlessingOfTheTrickster) GetID() string { return b.Ref().ID }

// GetType identifies a class feature.
func (*BlessingOfTheTrickster) GetType() core.EntityType { return EntityTypeFeature }

// ActionType describes the intended cost; activation remains unavailable.
func (*BlessingOfTheTrickster) ActionType() combat.ActionType { return combat.ActionStandard }

// Status projects the grant without inventing a limited-use pool.
func (b *BlessingOfTheTrickster) Status(*StatusInput) (*StatusOutput, error) {
	return &StatusOutput{Status: &Status{Ref: *b.Ref(), Name: b.Name(), Detail: "Not yet implemented. Will grant advantage on Stealth checks to yourself or a willing creature within touch range, until you finish a long rest or bless another target. Costs an action; no spell slot or concentration."}}, nil
}

// CanActivate keeps the placeholder unavailable without spending anything.
func (*BlessingOfTheTrickster) CanActivate(context.Context, core.Entity, FeatureInput) error {
	return fmt.Errorf("Blessing of the Trickster is not yet implemented")
}

// Activate refuses even direct calls; an NYI grant cannot silently succeed.
func (b *BlessingOfTheTrickster) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
	return b.CanActivate(ctx, owner, input)
}

// ToJSON persists the canonical grant for the shared loader.
func (b *BlessingOfTheTrickster) ToJSON() (json.RawMessage, error) {
	return json.Marshal(struct {
		Ref  *core.Ref `json:"ref"`
		ID   string    `json:"id"`
		Name string    `json:"name"`
	}{b.Ref(), b.GetID(), b.Name()})
}
