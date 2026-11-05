// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items_test

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// Everything here is consumer-test wiring, not an exported item runtime.
type restKind string

const (
	shortRest    restKind         = "short"
	longRest     restKind         = "long"
	grantKind    effects.KindName = "grant_healing"
	recoveryKind effects.KindName = "recover_pool"
	presentState                  = "present"
)

type restEvent struct {
	Holder string
	Kind   restKind
}

var restTopic = events.DefineTypedTopic[restEvent]("items.probe.rest")

type definition struct {
	Level    int
	Capacity int
	Recovery []restKind
}

func ringDefinition(level int) definition {
	return definition{Level: level, Capacity: 1, Recovery: []restKind{shortRest, longRest}}
}

// Only irreducible item state is saved. Authored configuration is supplied again
// on reconstruction; the bus, holder and attached offer are interaction wiring.
type instanceData struct {
	ID        string `json:"id"`
	Remaining int    `json:"remaining"`
}

type composeItemInput struct {
	Definition definition
	State      instanceData
}

type composeItemOutput struct{ Item *item }

type healingOffer struct {
	Holder      string
	Declaration healing.Declaration
	Source      dndevents.RollSource
}

type item struct {
	id       string
	def      definition
	pool     *resources.Resource
	composed *effects.Composed
	bus      events.EventBus
	offer    *healingOffer
	outputID string
}

func composeItem(in composeItemInput) (*composeItemOutput, error) {
	if in.State.ID == "" || in.Definition.Level <= 0 || in.Definition.Capacity <= 0 {
		return nil, fmt.Errorf("probe requires an item id, positive level and capacity")
	}
	if in.State.Remaining < 0 || in.State.Remaining > in.Definition.Capacity {
		return nil, fmt.Errorf("saved item charge count is outside its capacity")
	}
	pool := resources.NewResource(in.State.ID, in.Definition.Capacity)
	pool.SetCurrent(in.State.Remaining)
	def := in.Definition
	def.Recovery = slices.Clone(def.Recovery)
	return &composeItemOutput{Item: &item{id: in.State.ID, def: def, pool: pool}}, nil
}

func (i *item) data() instanceData {
	return instanceData{ID: i.id, Remaining: i.pool.Current}
}

func (i *item) available() bool { return i.offer != nil && i.pool.IsAvailable() }

func (i *item) attach(ctx context.Context, bus events.EventBus, holder string) error {
	if holder == "" || bus == nil {
		return fmt.Errorf("probe attachment requires holder and bus")
	}
	if i.composed != nil && i.composed.IsApplied() {
		return fmt.Errorf("item is already attached")
	}
	grantParams, err := json.Marshal(struct {
		Dice string `json:"dice"`
	}{Dice: "1d10"})
	if err != nil {
		return err
	}
	recoveryParams, err := json.Marshal(struct {
		Modes []restKind `json:"modes"`
	}{Modes: i.def.Recovery})
	if err != nil {
		return err
	}
	outs := []effects.Output{
		{Kind: grantKind, Amount: &effects.Amount{Fixed: &i.def.Level}, Params: jsontext.Value(grantParams)},
		{Kind: recoveryKind, Params: jsontext.Value(recoveryParams)},
	}
	encoded := make([]jsontext.Value, 0, len(outs))
	for _, out := range outs {
		data, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return marshalErr
		}
		encoded = append(encoded, jsontext.Value(data))
	}
	ref := core.Ref{Module: "example", Type: "items", ID: "second_wind_ring"}
	recipe := machine.Recipe{
		Ref: ref, Initial: presentState, Bag: machine.Bag{},
		States: map[string]machine.State{presentState: {Outputs: encoded}},
	}
	composed, err := effects.Compose(recipe, machine.Vocabulary{}, i.vocabulary(), holder, &ref, nil, "")
	if err != nil {
		return err
	}
	if err := composed.Apply(ctx, bus); err != nil {
		return err
	}
	i.composed = composed
	i.bus = bus
	return nil
}

func (i *item) detach(ctx context.Context) error {
	if i.composed == nil {
		return nil
	}
	if err := i.composed.Remove(ctx, i.bus); err != nil {
		return err
	}
	i.bus = nil
	return nil
}

func assess(frame any, out *effects.Bound) (effects.Row, error) {
	verdict, reason := out.Answer(frame)
	row := effects.Row{Kind: out.Kind(), Verdict: verdict, Reason: reason}
	if verdict == machine.Applies && out.HasAmount() {
		amount, err := out.Amount(frame)
		if err != nil {
			return row, err
		}
		row.Amount = amount
	}
	return row, nil
}

func (i *item) vocabulary() effects.OutputVocabulary {
	return effects.OutputVocabulary{
		Amounts: map[effects.AmountName]effects.AmountFn{
			effects.AmountFixed: func(_ any, amount effects.Amount) (int, error) { return *amount.Fixed, nil },
		},
		Kinds: map[effects.KindName]effects.KindSpec{
			grantKind: {
				Check: func(raw jsontext.Value) error {
					var p struct {
						Dice string `json:"dice"`
					}
					if err := json.Unmarshal(raw, &p); err != nil {
						return err
					}
					return (healing.Declaration{Dice: p.Dice}).Validate()
				},
				Attach: func(_ context.Context, _ events.EventBus, out *effects.Bound) (effects.Detach, error) {
					var p struct {
						Dice string `json:"dice"`
					}
					if err := json.Unmarshal(out.Params(), &p); err != nil {
						return nil, err
					}
					level, err := out.Amount(nil)
					if err != nil {
						return nil, err
					}
					sourceRef := *refs.Features.SecondWind()
					source := dndevents.RollSource{Ref: &sourceRef, Name: "Second Wind ring", SourceID: i.id}
					modifierSource := source
					modifierSource.Label = "Ring level"
					i.offer = &healingOffer{
						Holder: out.Host().Holder, Source: source,
						Declaration: healing.Declaration{Dice: p.Dice,
							Modifiers: []healing.Modifier{{Source: modifierSource, Amount: level}}},
					}
					i.outputID = out.ID()
					return func(context.Context) error { i.offer = nil; return nil }, nil
				},
				Assess: assess,
			},
			recoveryKind: {
				Check: func(raw jsontext.Value) error {
					var p struct {
						Modes []restKind `json:"modes"`
					}
					if err := json.Unmarshal(raw, &p); err != nil {
						return err
					}
					if len(p.Modes) == 0 {
						return fmt.Errorf("recovery modes are required in this probe")
					}
					for _, mode := range p.Modes {
						if mode != shortRest && mode != longRest {
							return fmt.Errorf("unknown probe rest %q", mode)
						}
					}
					return nil
				},
				Attach: func(ctx context.Context, bus events.EventBus, out *effects.Bound) (effects.Detach, error) {
					var p struct {
						Modes []restKind `json:"modes"`
					}
					if err := json.Unmarshal(out.Params(), &p); err != nil {
						return nil, err
					}
					id, err := restTopic.On(bus).Subscribe(ctx, func(_ context.Context, event restEvent) error {
						if event.Holder == out.Host().Holder && slices.Contains(p.Modes, event.Kind) {
							i.pool.RestoreToFull()
						}
						return nil
					})
					if err != nil {
						return nil, err
					}
					return func(ctx context.Context) error { return bus.Unsubscribe(ctx, id) }, nil
				},
				Assess: assess,
			},
		},
	}
}

type useInput struct {
	Holder string
	Roller dice.Roller
}

type useOutput struct{ Calculation *dndevents.RollCalculation }

func (i *item) use(in useInput) (*useOutput, error) {
	if i.offer == nil || in.Holder != i.offer.Holder || !i.pool.IsAvailable() {
		return nil, fmt.Errorf("no available grant for holder %q", in.Holder)
	}
	if err := i.offer.Declaration.Validate(); err != nil {
		return nil, err
	}
	if err := i.pool.Use(1); err != nil {
		return nil, err
	}
	calculation, err := healing.Resolve(context.Background(), i.offer.Declaration, i.offer.Source, in.Roller)
	if err != nil {
		// Nothing was delivered or mutated outside this item: refund the local
		// reservation, not an alleged rollback of arbitrary effects.
		i.pool.Restore(1)
		return nil, err
	}
	return &useOutput{Calculation: calculation}, nil
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
