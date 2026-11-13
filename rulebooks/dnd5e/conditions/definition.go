// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"

	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// DefinitionRole selects a participant supplied at invocation, never a saved ID.
type DefinitionRole string

const (
	// DefinitionUser selects the activating source's user as coordinator.
	DefinitionUser DefinitionRole = "user"
	// DefinitionRecipient selects the beneficiary as coordinator.
	DefinitionRecipient DefinitionRole = "recipient"
)

// PartSelection names exactly one leaf or non-nested benefit set under a local key.
type PartSelection struct {
	Key  string `json:"key"`
	Part string `json:"part,omitempty"`
	Set  string `json:"set,omitempty"`
}

// EffectDefinition selects benefits and independent coordinator maintenance.
type EffectDefinition struct {
	Name        string          `json:"name"`
	Coordinator DefinitionRole  `json:"coordinator"`
	Maintenance []PartSelection `json:"maintenance"`
	Benefits    []PartSelection `json:"benefits"`
}

// DefinitionLibrary is the single authored format, with no runtime IDs or state.
type DefinitionLibrary struct {
	Parts       map[string]dndfx.PartTemplate `json:"parts"`
	BenefitSets map[string][]string           `json:"benefit_sets"`
	Definitions map[string]EffectDefinition   `json:"definitions"`
}

type compiledDefinition struct {
	name        string
	coordinator DefinitionRole
	maintenance []dndfx.PartDeclaration
	benefits    []dndfx.PartDeclaration
}

// CompiledDefinitions owns validated expanded declarations. It exposes no mutable
// catalogue; binding returns fresh values and active effects save those values.
type CompiledDefinitions struct{ definitions map[string]compiledDefinition }

// CompileDefinitionsInput supplies complete authored JSON for strict validation.
type CompileDefinitionsInput struct{ Data json.RawMessage }

// CompileDefinitionsOutput contains an immutable validated catalogue.
type CompileDefinitionsOutput struct{ Definitions *CompiledDefinitions }

// ValidateDefinitionReferenceInput validates source selection without fake actors.
type ValidateDefinitionReferenceInput struct {
	Definitions  *CompiledDefinitions
	DefinitionID string
}

// BindDefinitionInput supplies real invocation roles and immutable provenance.
// Binding remains pure; participant admission belongs to PrepareApplication.
type BindDefinitionInput struct {
	Definitions   *CompiledDefinitions
	DefinitionID  string
	UserID        string
	RecipientID   string
	ApplicationID string
	Origin        dndevents.ConditionOrigin
}

// BindDefinitionOutput supplies owned concrete data to PrepareApplication.
type BindDefinitionOutput struct {
	CoordinatorID string
	Spec          ApplicationSpec
}
