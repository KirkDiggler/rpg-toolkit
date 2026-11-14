// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package definitions

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// CatalogueInput requests explicit construction of the built-in definition set.
// It contains no runtime source, participant or mutable registration callback.
type CatalogueInput struct{}

// CatalogueOutput owns the same projected built-in values exposed by typed fields.
type CatalogueOutput struct{ Catalogue *effects.Catalogue }

// Catalogue registers the complete native Rage definition with live class scaling.
// A fixed-bonus variant is an explicit derived value, never inferred at lookup.
// Nil input or invalid built-in content returns an error, not an init-time panic.
func Catalogue(in *CatalogueInput) (*CatalogueOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("built-in catalogue input is required")
	}
	raging, err := Raging(&RagingInput{DamageBonus: RagingDamageBonus{BarbarianLevel: true}})
	if err != nil {
		return nil, err
	}
	entry, err := effects.DefineEntry(&effects.DefineEntryInput{Ref: *refs.Definitions.Raging(), Name: "Raging", Description: "Rage benefits and independent native upkeep", Definition: raging})
	if err != nil {
		return nil, err
	}
	catalogue, err := effects.BuildCatalogue(&effects.BuildCatalogueInput{Entries: []effects.CatalogueEntry{entry.Entry}})
	if err != nil {
		return nil, err
	}
	return &CatalogueOutput{Catalogue: catalogue.Catalogue}, nil
}
