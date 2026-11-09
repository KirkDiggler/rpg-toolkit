// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/examples/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

const (
	rageHolder = "rage-wearer"
	rageGrant  = "rage"
	rageFresh  = "fresh"
	rageLive   = "raging"
)

// The grant is still experimental; its resulting condition uses the canonical
// shared rulebook creator and that creator's binding schema.
var rageRef = core.Ref{Module: "example", Type: "abilities", ID: "rage"}
var rageConditionRef = *refs.Conditions.Raging()

func rageItemDefinition() items.Definition {
	return items.Definition{Ref: core.Ref{Module: "example", Type: "items", ID: "raging_ring"}, Name: "Raging ring",
		Pools:  map[string]items.PoolDefinition{chargePool: {Maximum: 1, RestoreOn: []items.Trigger{longRest}}},
		Grants: map[string]items.Grant{rageGrant: {Ref: rageRef, Parameters: json.RawMessage(`{"damage_bonus":{"fixed":2}}`), Pool: chargePool, Uses: 1}},
	}
}
func rageItemConfig(id string) items.Config {
	return items.Config{ID: id, Definition: rageItemDefinition(), RecoveryTriggers: []items.Trigger{shortRest, longRest}, ValidateGrant: validateRageGrant}
}
func validateRageGrant(grant items.Grant) error {
	if grant.Ref != rageRef {
		return fmt.Errorf("unknown Rage consumer ref %s", grant.Ref.String())
	}
	return conditions.ValidateBindings(&conditions.ValidateBindingsInput{Ref: rageConditionRef.String(), Bindings: grant.Parameters})
}

type rageActivation struct {
	Source items.Source
	Effect dndevents.ConditionBehavior
}

func useRage(ctx context.Context, item *items.Item, holder string) (*items.UseOutput[*rageActivation], error) {
	return items.Use(ctx, items.UseInput[*rageActivation]{Item: item, HolderID: holder, GrantID: rageGrant,
		Resolve: func(_ context.Context, offer items.Offer) (*rageActivation, error) {
			if err := validateRageGrant(offer.Grant); err != nil {
				return nil, err
			}
			created, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: rageConditionRef.String(), Config: offer.Grant.Parameters, MemberID: offer.Source.HolderID,
				Origin: &dndevents.ConditionOrigin{Ref: offer.Source.ItemRef, EntityID: offer.Source.ItemID},
			})
			if err != nil {
				return nil, err
			}
			return &rageActivation{Source: offer.Source, Effect: created.Condition}, nil
		},
	})
}
