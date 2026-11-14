// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"fmt"
	"strings"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// ApplicationAttachment selects custody, not a creature. Self-use still has two
// distinct attachments even when both participant roles select the same creature.
type ApplicationAttachment string

const (
	// CoordinatorAttachment holds the application's lifetime authority.
	CoordinatorAttachment ApplicationAttachment = "coordinator"
	// RecipientAttachment holds the delivered attachment, not lifetime authority.
	RecipientAttachment ApplicationAttachment = "recipient"
)

// ApplicationPlacement assigns one definition subtree to explicit custody and
// terminal scope. Path is relative to the canonical definition, never a ref ID.
type ApplicationPlacement struct {
	Key        string                `json:"key"`
	Path       []string              `json:"path,omitempty"`
	Attachment ApplicationAttachment `json:"attachment"`
	OnTerminal generic.EndScope      `json:"on_terminal"`
}

// ApplicationDefinition is a host's placement of ordinary resolved effect data.
// Inline values need no catalogue. References must resolve before compilation.
type ApplicationDefinition struct {
	Name        string
	Coordinator DefinitionRole
	Definition  effects.Definition
	Placements  []ApplicationPlacement
}

// CompiledApplicationDefinition owns validated concrete parts without actor,
// instance, payment or catalogue state. Binding returns new independent values.
type CompiledApplicationDefinition struct {
	name             string
	coordinator      DefinitionRole
	coordinatorParts []effects.PartDeclaration
	recipientParts   []effects.PartDeclaration
}

// CompileApplicationDefinitionInput supplies the complete unbound host mapping.
type CompileApplicationDefinitionInput struct{ Definition ApplicationDefinition }

// CompileApplicationDefinitionOutput returns one immutable application definition.
type CompileApplicationDefinitionOutput struct {
	Definition *CompiledApplicationDefinition
}

// CompileApplicationDefinition compiles with the canonical effect compiler and
// explicit placement. It refuses invalid roles, incomplete/overlapping mounts,
// unsupported scopes, empty attachments and impossible concentration custody.
// No world lookup, binding, runtime creation or payment occurs here.
func CompileApplicationDefinition(in *CompileApplicationDefinitionInput) (*CompileApplicationDefinitionOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("application definition input is required")
	}
	def := in.Definition
	if strings.TrimSpace(def.Name) == "" {
		return nil, fmt.Errorf("application definition name is required")
	}
	if def.Coordinator != DefinitionUser && def.Coordinator != DefinitionRecipient {
		return nil, fmt.Errorf("application coordinator must select user or recipient")
	}
	compiled, err := effects.Compile(&effects.CompileInput{Definition: def.Definition})
	if err != nil {
		return nil, err
	}
	mounts := make([]effects.Mount, 0, len(def.Placements))
	attachments := make(map[string]ApplicationAttachment, len(def.Placements))
	for _, placement := range def.Placements {
		switch placement.Attachment {
		case CoordinatorAttachment:
			if placement.OnTerminal == generic.EndAttachment {
				return nil, fmt.Errorf("coordinator expiry must end its application, not abandon its members")
			}
		case RecipientAttachment:
			if placement.OnTerminal == generic.EndApplication {
				return nil, fmt.Errorf("recipient cannot own application upkeep")
			}
		default:
			return nil, fmt.Errorf("unknown application attachment %q", placement.Attachment)
		}
		mounts = append(mounts, effects.Mount{Key: placement.Key, Path: placement.Path, OnTerminal: placement.OnTerminal})
		attachments[placement.Key] = placement.Attachment
	}
	placed, err := effects.Place(&effects.PlaceInput{Definition: compiled.Definition, Mounts: mounts})
	if err != nil {
		return nil, err
	}
	out := &CompiledApplicationDefinition{name: def.Name, coordinator: def.Coordinator}
	concentration := 0
	for _, part := range placed.Parts {
		declaration := part.Declaration
		info, err := effects.ValidateEffect(&effects.ValidateEffectInput{Effect: effects.EffectSpec{Kind: declaration.Kind, Params: declaration.Params, When: declaration.When, Amount: declaration.Amount}})
		if err != nil {
			return nil, err
		}
		attachment := attachments[part.MountKey]
		if info.Capabilities.Concentration {
			concentration++
			if attachment != CoordinatorAttachment || concentration > 1 {
				return nil, fmt.Errorf("application requires at most one coordinator-owned concentration claim")
			}
		}
		if attachment == CoordinatorAttachment {
			out.coordinatorParts = append(out.coordinatorParts, declaration.Clone())
		} else {
			out.recipientParts = append(out.recipientParts, declaration.Clone())
		}
	}
	if len(out.coordinatorParts) == 0 || len(out.recipientParts) == 0 {
		return nil, fmt.Errorf("application requires explicit nonempty coordinator and recipient parts")
	}
	return &CompileApplicationDefinitionOutput{Definition: out}, nil
}

// BindApplicationDefinitionInput supplies real invocation identities/provenance.
// Participant admission still belongs to PrepareApplication, not this binder.
type BindApplicationDefinitionInput struct {
	Definition    *CompiledApplicationDefinition
	UserID        string
	RecipientID   string
	ApplicationID string
	Origin        dndevents.ConditionOrigin
}

// BindApplicationDefinition substitutes actual roles into owned ApplicationSpec
// data. Missing/uncompiled values or invalid identities refuse; binding does not
// inspect the world, attach, roll, charge or replace anything.
func BindApplicationDefinition(in *BindApplicationDefinitionInput) (*BindDefinitionOutput, error) {
	if in == nil || in.Definition == nil {
		return nil, fmt.Errorf("compiled application definition is required")
	}
	def := in.Definition
	if def.name == "" || len(def.coordinatorParts) == 0 || len(def.recipientParts) == 0 {
		return nil, fmt.Errorf("application definition was not compiled")
	}
	if strings.TrimSpace(in.UserID) == "" || strings.TrimSpace(in.RecipientID) == "" || strings.TrimSpace(in.ApplicationID) == "" || strings.TrimSpace(in.Origin.EntityID) == "" {
		return nil, fmt.Errorf("definition binding requires user, recipient, application and origin identity")
	}
	if err := in.Origin.Ref.IsValid(); err != nil {
		return nil, fmt.Errorf("definition origin: %w", err)
	}
	coordinator := in.UserID
	if def.coordinator == DefinitionRecipient {
		coordinator = in.RecipientID
	}
	// A tagged tuple, not unescaped ID concatenation. The coordinator keeps the
	// ApplicationID; the recipient's qualifier stays different even on self-use.
	address, err := json.Marshal([]string{"recipient", in.ApplicationID})
	if err != nil {
		return nil, err
	}
	return &BindDefinitionOutput{CoordinatorID: coordinator, Spec: ApplicationSpec{Name: def.name, CoordinatorParts: cloneDeclarations(def.coordinatorParts), Recipients: []RecipientSpec{{MemberID: in.RecipientID, Ref: refs.Conditions.LinkedEffects().String(), Name: def.name, AddressSourceID: string(address), Parts: cloneDeclarations(def.recipientParts), Lifetime: LifetimeShared}}}}, nil
}
