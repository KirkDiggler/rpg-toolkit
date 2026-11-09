// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// Kind names the supported reusable mechanical meanings, not named conditions.
const (
	KindDamageBonus generic.KindName = "damage_bonus"
	KindResistance  generic.KindName = "resistance"
	KindAdvantage   generic.KindName = "advantage"
)

// BindInput supplies immutable instance attribution for the rulebook vocabulary.
// Origin is not a condition's stacking qualifier; DisplayName is authored by its
// descriptor rather than inferred from a ref or a particular source type.
type BindInput struct {
	Origin      dndevents.ConditionOrigin
	DisplayName string
}

// BindOutput supplies the two generic vocabularies. The condition owner supplies
// its own End/Changed hooks; a kind does not decide condition address or custody.
type BindOutput struct {
	Machine machine.Vocabulary
	Outputs generic.OutputVocabulary
}

// AssessInput asks one bound output about one typed rulebook context. No context
// requests its description without an applied benefit. No operation mutates it.
type AssessInput struct {
	Query       Query
	Output      *generic.Bound
	Origin      dndevents.ConditionOrigin
	DisplayName string
}

// AssessOutput carries one domain-honest information row, never an action grant.
type AssessOutput struct{ Effect contributions.Effect }

func validateBinding(in *BindInput) error {
	if in == nil {
		return fmt.Errorf("effect vocabulary requires input")
	}
	if err := in.Origin.Ref.IsValid(); err != nil {
		return fmt.Errorf("effect origin ref: %w", err)
	}
	if in.Origin.EntityID == "" {
		return fmt.Errorf("effect origin entity_id is required")
	}
	if in.DisplayName == "" {
		return fmt.Errorf("effect display name is required")
	}
	return nil
}
