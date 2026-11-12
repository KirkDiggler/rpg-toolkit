// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// BindDefinition substitutes real roles into a validated definition. It refuses
// missing identity/provenance and unknown selections, but does not look up world
// participants, attach, roll, charge or replace anything. PrepareApplication
// remains responsible for participant admission and replacement description.
func BindDefinition(in *BindDefinitionInput) (*BindDefinitionOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("definition binding is required")
	}
	if err := ValidateDefinitionReference(&ValidateDefinitionReferenceInput{Definitions: in.Definitions, DefinitionID: in.DefinitionID}); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.UserID) == "" || strings.TrimSpace(in.RecipientID) == "" || strings.TrimSpace(in.ApplicationID) == "" || strings.TrimSpace(in.Origin.EntityID) == "" {
		return nil, fmt.Errorf("definition binding requires user, recipient, application and origin identity")
	}
	if err := in.Origin.Ref.IsValid(); err != nil {
		return nil, fmt.Errorf("definition origin: %w", err)
	}
	def := in.Definitions.definitions[in.DefinitionID]
	coordinator := in.UserID
	if def.coordinator == DefinitionRecipient {
		coordinator = in.RecipientID
	}
	// Encode a tagged tuple, never concatenate unescaped participant/source IDs.
	// The owner keeps ApplicationID; this longer encoding also separates self-use.
	address, err := json.Marshal([]string{"recipient", in.ApplicationID})
	if err != nil {
		return nil, err
	}
	return &BindDefinitionOutput{CoordinatorID: coordinator, Spec: ApplicationSpec{Name: def.name,
		CoordinatorParts: cloneDeclarations(def.maintenance),
		Recipients:       []RecipientSpec{{MemberID: in.RecipientID, Ref: refs.Conditions.LinkedEffects().String(), Name: def.name, AddressSourceID: string(address), Parts: cloneDeclarations(def.benefits), Lifetime: LifetimeShared}},
	}}, nil
}
