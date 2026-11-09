// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/examples/items"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
)

// The rulebook vocabulary and rules stay in this consumer, not in items.
const (
	shortRest  items.Trigger = "rest.short"
	longRest   items.Trigger = "rest.long"
	chargePool               = "charges"
	windGrant                = "second_wind"
)

// This ref belongs to the prototype consumer's parameter schema. It is not a
// claim that the production feature loader accepts a ring-level grant payload.
var windRef = core.Ref{Module: "example", Type: "abilities", ID: "second_wind"}

type windParameters struct {
	Level int `json:"level"`
}

func ringDefinition(level int) items.Definition {
	parameters, err := json.Marshal(windParameters{Level: level})
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

func parseWind(grant items.Grant) (windParameters, error) {
	if grant.Ref != windRef {
		return windParameters{}, fmt.Errorf("unknown consumer ability %s", grant.Ref.String())
	}
	var p windParameters
	decoder := json.NewDecoder(bytes.NewReader(grant.Parameters))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return p, err
	}
	if p.Level <= 0 {
		return p, fmt.Errorf("Second Wind requires a positive supplied level")
	}
	return p, nil
}

func validateSecondWind(grant items.Grant) error {
	_, err := parseWind(grant)
	return err
}

func useRing(ctx context.Context, item *items.Item, holder string, roller dice.Roller) (
	*items.UseOutput[*dndevents.RollCalculation], error,
) {
	return items.Use(ctx, items.UseInput[*dndevents.RollCalculation]{
		Item: item, HolderID: holder, GrantID: windGrant,
		Resolve: func(ctx context.Context, offer items.Offer) (*dndevents.RollCalculation, error) {
			p, err := parseWind(offer.Grant)
			if err != nil {
				return nil, err
			}
			abilityRef, itemRef := offer.Grant.Ref, offer.Source.ItemRef
			source := dndevents.RollSource{Ref: &abilityRef, Name: offer.Source.Name, SourceID: offer.Source.ItemID}
			declaration := healing.Declaration{
				Dice: "1d10",
				Modifiers: []healing.Modifier{{Amount: p.Level, Source: dndevents.RollSource{
					Ref: &itemRef, Name: offer.Source.Name, SourceID: offer.Source.ItemID, Label: "Ring level",
				}}},
			}
			return healing.Resolve(ctx, declaration, source, roller)
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
