// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/examples/items"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// The rulebook vocabulary and rules stay in this consumer, not in items.
const (
	shortRest  items.Trigger = "rest.short"
	longRest   items.Trigger = "rest.long"
	chargePool               = "charges"
	windGrant                = "second_wind"
)

// The ring names the production ability: the same ref a fighter's class
// feature asks the rulebook for.
var windRef = *refs.Features.SecondWind()

func ringDefinition(level int) items.Definition {
	parameters, err := json.Marshal(features.SecondWindParameters{Level: level})
	if err != nil {
		panic(err)
	}
	return items.Definition{
		Ref:  core.Ref{Module: "example", Type: "items", ID: fmt.Sprintf("second_wind_ring_%d", level)},
		Name: "Second Wind ring",
		Pools: map[string]items.PoolDefinition{
			chargePool: {Maximum: 1, RestoreOn: []items.Trigger{shortRest, longRest}},
		},
		Grants: map[string]items.Grant{
			windGrant: {Ref: windRef, Parameters: parameters, Pool: chargePool, Uses: 1},
		},
	}
}

func ringConfig(id string, level int, state *items.Data) items.Config {
	return items.Config{
		ID: id, Definition: ringDefinition(level), State: state,
		RecoveryTriggers: []items.Trigger{shortRest, longRest}, ValidateGrant: validateSecondWind,
	}
}

func validateSecondWind(grant items.Grant) error {
	return features.ValidateParameters(&features.ValidateParametersInput{Ref: grant.Ref, Parameters: grant.Parameters})
}

func useRing(ctx context.Context, item *items.Item, holder string, roller dice.Roller) (
	*items.UseOutput[*dndevents.RollCalculation], error,
) {
	return items.Use(ctx, items.UseInput[*dndevents.RollCalculation]{
		Item: item, HolderID: holder, GrantID: windGrant,
		Resolve: func(ctx context.Context, offer items.Offer) (*dndevents.RollCalculation, error) {
			abilityRef, itemRef := offer.Grant.Ref, offer.Source.ItemRef
			// The item charges its own count in items.Use, so no pools are passed.
			definition, err := features.DescribeFromRef(&features.DescribeFromRefInput{
				Ref:        offer.Grant.Ref,
				Parameters: offer.Grant.Parameters,
				Giver: dndevents.RollSource{
					Ref: &itemRef, Name: offer.Source.Name, Label: "Ring level", SourceID: offer.Source.ItemID,
				},
			})
			if err != nil {
				return nil, err
			}
			source := dndevents.RollSource{Ref: &abilityRef, Name: offer.Source.Name, SourceID: offer.Source.ItemID}
			return healing.Resolve(ctx, *definition.Effect.Healing, source, roller)
		},
	})
}

type fixedRoller struct {
	face  int
	calls int
	err   error
}

func (r *fixedRoller) Roll(ctx context.Context, size int) (int, error) {
	faces, err := r.RollN(ctx, 1, size)
	if err != nil {
		return 0, err
	}
	return faces[0], nil
}

func (r *fixedRoller) RollN(_ context.Context, count, size int) ([]int, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	if count != 1 || size != 10 || r.face < 1 || r.face > 10 {
		return nil, fmt.Errorf("fixture expects exactly one d10, got %dd%d", count, size)
	}
	return []int{r.face}, nil
}

// The ring's grant is checked by the rulebook, so the rulebook's own sentences
// refuse a bad grant when the item is built.
func TestRulebookRefusesABadRingGrant(t *testing.T) {
	for name, tc := range map[string]struct {
		parameters string
		contains   string
	}{
		"level zero":     {`{"level": 0}`, "second wind level 0 must be at least 1"},
		"unknown member": {`{"level": 1, "bonus": 2}`, "second wind parameters"},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := ringConfig("ring-bad", 1, nil)
			grant := cfg.Definition.Grants[windGrant]
			grant.Parameters = json.RawMessage(tc.parameters)
			cfg.Definition.Grants[windGrant] = grant
			_, err := items.New(cfg)
			require.Error(t, err)
			require.ErrorContains(t, err, tc.contains)
		})
	}
}
