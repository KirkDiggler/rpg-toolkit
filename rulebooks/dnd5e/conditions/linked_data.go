// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// LinkedRole distinguishes lifetime custody from receiving a contribution.
type LinkedRole string

const (
	// LinkedCoordinator is the sole owner of a shared lifetime.
	LinkedCoordinator LinkedRole = "coordinator"
	// LinkedRecipient owns only its local parts and an immutable owner link.
	LinkedRecipient LinkedRole = "recipient"
)

// LinkMode distinguishes current backlink-bearing nodes from current payloads
// adopted by exact address. It does not select a historical saved-data reader.
type LinkMode string

const (
	// LinkComposed requires a matching application and coordinator backlink.
	LinkComposed LinkMode = "composed"
	// LinkAdopted names an unmigrated current payload by exact address.
	LinkAdopted LinkMode = "adopted"
)

// LinkedMemberBinding maps one opaque lifetime key to one exact attachment.
type LinkedMemberBinding struct {
	Key     string                     `json:"key"`
	Address dndevents.ConditionAddress `json:"address"`
	Mode    LinkMode                   `json:"mode"`
}

// LinkedBindings are immutable authored parts and role-specific address bindings.
// Members are the coordinator's initial bindings, not a duplicate live-member set.
type LinkedBindings struct {
	Name      string                      `json:"name"`
	Parts     []dndfx.PartDeclaration     `json:"parts"`
	Owner     *dndevents.ConditionAddress `json:"owner,omitempty"`
	MemberKey string                      `json:"member_key,omitempty"`
	Members   []LinkedMemberBinding       `json:"members,omitempty"`
}

// LinkedConditionConfig creates current linked data through CreateFromRef.
// AddressSourceID is replacement identity, independently of Origin.EntityID.
type LinkedConditionConfig struct {
	ApplicationID   string          `json:"application_id"`
	Role            LinkedRole      `json:"role"`
	AddressSourceID string          `json:"address_source_id"`
	Bindings        *LinkedBindings `json:"bindings"`
}

// LinkedRuntimeData stores each local part once and lifetime state only on its owner.
type LinkedRuntimeData struct {
	Parts    *generic.AssemblySnapshot `json:"parts"`
	Lifetime *generic.LifetimeSnapshot `json:"lifetime,omitempty"`
}

// LinkedConditionData is the sole current linked-condition representation.
// It is not an alternative reader for historical Blessed/Baned/Concentrating data.
type LinkedConditionData struct {
	Ref             core.Ref                   `json:"ref"`
	MemberID        string                     `json:"member_id"`
	AddressSourceID string                     `json:"address_source_id"`
	ApplicationID   string                     `json:"application_id"`
	Role            LinkedRole                 `json:"role"`
	Origin          *dndevents.ConditionOrigin `json:"origin"`
	Bindings        *LinkedBindings            `json:"bindings"`
	Runtime         *LinkedRuntimeData         `json:"runtime"`
}

func cloneLinkedBindings(in LinkedBindings) LinkedBindings {
	out := in
	out.Parts = slices.Clone(in.Parts)
	for i := range out.Parts {
		out.Parts[i].Params = slices.Clone(in.Parts[i].Params)
	}
	out.Members = slices.Clone(in.Members)
	if in.Owner != nil {
		owner := *in.Owner
		out.Owner = &owner
	}
	return out
}

func validLinkedAddress(address dndevents.ConditionAddress) error {
	if strings.TrimSpace(address.MemberID) == "" {
		return fmt.Errorf("linked address requires a member")
	}
	if _, err := core.ParseString(address.ConditionRef); err != nil {
		return fmt.Errorf("linked address ref: %w", err)
	}
	return nil
}

func validateLinkedData(data LinkedConditionData) error {
	if err := data.Ref.IsValid(); err != nil {
		return fmt.Errorf("linked ref: %w", err)
	}
	if strings.TrimSpace(data.MemberID) == "" || strings.TrimSpace(data.ApplicationID) == "" || strings.TrimSpace(data.AddressSourceID) == "" {
		return fmt.Errorf("linked member, application and address qualifier are required")
	}
	if data.Origin == nil || data.Origin.EntityID == "" {
		return fmt.Errorf("linked origin is required")
	}
	if err := data.Origin.Ref.IsValid(); err != nil {
		return fmt.Errorf("linked origin ref: %w", err)
	}
	if data.Bindings == nil || strings.TrimSpace(data.Bindings.Name) == "" || data.Bindings.Parts == nil {
		return fmt.Errorf("linked name and explicit part declarations are required")
	}
	own := dndevents.ConditionAddress{MemberID: data.MemberID, ConditionRef: data.Ref.String(), SourceID: data.AddressSourceID}
	switch data.Role {
	case LinkedCoordinator:
		if data.AddressSourceID != data.ApplicationID || data.Bindings.Owner != nil || data.Bindings.MemberKey != "" {
			return fmt.Errorf("coordinator identity/bindings disagree with its application")
		}
		keys := map[string]bool{}
		addresses := map[dndevents.ConditionAddress]bool{}
		for _, member := range data.Bindings.Members {
			if strings.TrimSpace(member.Key) == "" || keys[member.Key] || addresses[member.Address] || member.Address == own {
				return fmt.Errorf("linked membership repeats or lacks an exact non-owner address/key")
			}
			if err := validLinkedAddress(member.Address); err != nil {
				return err
			}
			if member.Mode != LinkComposed && member.Mode != LinkAdopted {
				return fmt.Errorf("unknown linked member mode %q", member.Mode)
			}
			keys[member.Key], addresses[member.Address] = true, true
		}
	case LinkedRecipient:
		if data.Bindings.Owner == nil || data.Bindings.MemberKey == "" || len(data.Bindings.Members) != 0 {
			return fmt.Errorf("recipient requires only its exact owner/member link")
		}
		if err := validLinkedAddress(*data.Bindings.Owner); err != nil {
			return err
		}
		if data.Bindings.Owner.SourceID != data.ApplicationID || *data.Bindings.Owner == own {
			return fmt.Errorf("recipient link disagrees with its application or points to itself")
		}
		for _, part := range data.Bindings.Parts {
			if part.OnTerminal != generic.EndPart {
				return fmt.Errorf("recipient parts cannot own application upkeep")
			}
		}
	default:
		return fmt.Errorf("unknown linked role %q", data.Role)
	}
	return nil
}

func createLinked(in *CreateFromRefInput) (dndevents.ConditionBehavior, error) {
	if in.SourceRef != "" {
		return nil, fmt.Errorf("linked creation requires current Origin, not SourceRef")
	}
	var config LinkedConditionConfig
	if err := strictJSON(in.Config, &config); err != nil {
		return nil, fmt.Errorf("linked config: %w", err)
	}
	ref, err := core.ParseString(in.Ref)
	if err != nil {
		return nil, err
	}
	return createdAs(buildLinked(LinkedConditionData{Ref: *ref, MemberID: in.MemberID, Origin: in.Origin,
		ApplicationID: config.ApplicationID, AddressSourceID: config.AddressSourceID,
		Role: config.Role, Bindings: config.Bindings}, false))
}

func loadLinked(raw json.RawMessage) (dndevents.ConditionBehavior, error) {
	var data LinkedConditionData
	if err := strictJSON(raw, &data); err != nil {
		return nil, fmt.Errorf("linked data: %w", err)
	}
	if data.Runtime == nil || data.Runtime.Parts == nil || data.Runtime.Parts.Parts == nil {
		return nil, fmt.Errorf("linked restore requires explicit complete runtime state")
	}
	return createdAs(buildLinked(data, true))
}
