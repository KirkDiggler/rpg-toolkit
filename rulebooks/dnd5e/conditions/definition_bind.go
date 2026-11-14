// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import "fmt"

// BindDefinition is the staging source adapter while old authoring callers move
// to CompileApplicationDefinition. Binding itself has one implementation.
func BindDefinition(in *BindDefinitionInput) (*BindDefinitionOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("definition binding is required")
	}
	if err := ValidateDefinitionReference(&ValidateDefinitionReferenceInput{Definitions: in.Definitions, DefinitionID: in.DefinitionID}); err != nil {
		return nil, err
	}
	def := in.Definitions.definitions[in.DefinitionID]
	return BindApplicationDefinition(&BindApplicationDefinitionInput{
		Definition: &CompiledApplicationDefinition{name: def.name, coordinator: def.coordinator, coordinatorParts: def.maintenance, recipientParts: def.benefits},
		UserID:     in.UserID, RecipientID: in.RecipientID, ApplicationID: in.ApplicationID, Origin: in.Origin,
	})
}
