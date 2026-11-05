// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

// EntityTypeItem identifies item instances through the shared entity interface.
const EntityTypeItem core.EntityType = "item"

// ErrUnavailable identifies a detached, wrong-holder, absent or exhausted grant.
var ErrUnavailable = errors.New("item grant unavailable")

const (
	kindGrant    effects.KindName = "items.grant"
	kindRecover  effects.KindName = "items.recover_pool"
	statePresent                  = "present"
)

// RecoveryEvent reports a consumer-defined occurrence for one holder. The item
// selects its configured recovery policy; it does not know what a rest means.
type RecoveryEvent struct {
	HolderID string
	Trigger  Trigger
}

// RecoveryTopic carries recovery occurrences on the caller-owned interaction bus.
var RecoveryTopic = events.DefineTypedTopic[RecoveryEvent]("items.recovery")

// Item composes attached grants and recovery outputs around instance-owned pools.
// Like effects.Composed, it is synchronous and not safe for concurrent use.
// This is an experimental example API, not a production equipment contract.
type Item struct {
	id       string
	def      Definition
	pools    map[string]*resources.Resource
	recipe   machine.Recipe
	active   map[string]bool
	composed *effects.Composed
	bus      events.EventBus
	holder   string
}

var _ core.Entity = (*Item)(nil)

// GetID returns this item's instance identity, not its content definition ref.
func (i *Item) GetID() string { return i.id }

// GetType returns the shared entity category for item instances.
func (i *Item) GetType() core.EntityType { return EntityTypeItem }

// New validates and copies configuration, then creates or strictly restores the
// instance. It does not attach to a bus, interpret an ability or mutate a character.
func New(config Config) (*Item, error) {
	def, pools, err := prepare(config)
	if err != nil {
		return nil, err
	}
	i := &Item{id: config.ID, def: def, pools: pools, active: map[string]bool{}}
	var outputs []jsontext.Value
	for _, key := range slices.Sorted(maps.Keys(def.Grants)) {
		out, err := bindingOutput(kindGrant, key)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, out)
	}
	for _, key := range slices.Sorted(maps.Keys(def.Pools)) {
		if len(def.Pools[key].RestoreOn) == 0 {
			continue
		}
		out, err := bindingOutput(kindRecover, key)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, out)
	}
	i.recipe = machine.Recipe{
		Ref: def.Ref, Initial: statePresent, Bag: machine.Bag{},
		States: map[string]machine.State{statePresent: {Outputs: outputs}},
	}
	return i, nil
}

type binding struct {
	Key string `json:"key"`
}

func bindingOutput(kind effects.KindName, key string) (jsontext.Value, error) {
	params, err := json.Marshal(binding{Key: key})
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(effects.Output{Kind: kind, Params: jsontext.Value(params)})
	return jsontext.Value(out), err
}

func bindingKey(raw jsontext.Value) (string, error) {
	var b binding
	if err := json.Unmarshal(raw, &b); err != nil {
		return "", err
	}
	if b.Key == "" {
		return "", fmt.Errorf("output binding key is missing")
	}
	return b.Key, nil
}

// AttachInput binds the item to an explicitly eligible holder on an existing bus.
// Determining worn/carried/attuned eligibility belongs to the consumer, not this API.
type AttachInput struct {
	Bus      events.EventBus
	HolderID string
}

// Attach creates real effects wiring for this association. A second attachment
// is refused; failed effects attachment uses the provider's existing rollback.
func (i *Item) Attach(ctx context.Context, input AttachInput) error {
	if input.Bus == nil || strings.TrimSpace(input.HolderID) == "" {
		return fmt.Errorf("%w: attachment requires bus and holder", ErrInvalidConfig)
	}
	if i.composed != nil {
		return machine.ErrAlreadyApplied
	}
	c, err := effects.New(effects.Config{
		Recipe: i.recipe, OutputVocabulary: i.vocabulary(), HolderID: input.HolderID, SourceRef: &i.def.Ref,
	})
	if err != nil {
		return err
	}
	if err := c.Apply(ctx, input.Bus); err != nil {
		return err
	}
	i.composed, i.bus, i.holder = c, input.Bus, input.HolderID
	return nil
}

// Detach removes grant/recovery wiring but preserves every pool's remaining uses.
// It is idempotent when already detached and reports provider detach failures.
func (i *Item) Detach(ctx context.Context) error {
	if i.composed == nil {
		return nil
	}
	if err := i.composed.Remove(ctx, i.bus); err != nil {
		return err
	}
	i.composed, i.bus, i.holder = nil, nil, ""
	return nil
}

// Source distinguishes a content definition from the particular item and holder.
type Source struct {
	ItemID   string
	ItemRef  core.Ref
	Name     string
	HolderID string
}

// Offer is a copied, inspectable grant snapshot, not a live mutation authority.
type Offer struct {
	ID        string
	Grant     Grant
	Source    Source
	Remaining int // Item-pool snapshot at inspection, before a subsequent use.
	// Affordable concerns this item's pool only, not actor action economy or
	// rulebook eligibility. The consuming rules layer must supply those checks.
	Affordable bool
}

// OffersInput identifies the holder inspecting this item's grants.
type OffersInput struct{ HolderID string }

// OffersOutput contains grants in stable local-ID order, including exhausted ones.
type OffersOutput struct{ Grants []Offer }

// Offers returns no grants when detached or associated with a different holder.
// Empty holder identity is refused rather than interpreted as a wildcard.
func (i *Item) Offers(input OffersInput) (*OffersOutput, error) {
	if strings.TrimSpace(input.HolderID) == "" {
		return nil, fmt.Errorf("%w: holder is required", ErrInvalidConfig)
	}
	out := &OffersOutput{}
	if i.composed == nil || input.HolderID != i.holder {
		return out, nil
	}
	for _, key := range slices.Sorted(maps.Keys(i.active)) {
		out.Grants = append(out.Grants, i.offer(key))
	}
	return out, nil
}

func (i *Item) offer(key string) Offer {
	g := i.def.Grants[key]
	remaining := i.pools[g.Pool].Current
	return Offer{
		ID: key, Grant: cloneGrant(g), Remaining: remaining, Affordable: remaining >= g.Uses,
		Source: Source{ItemID: i.id, ItemRef: i.def.Ref, Name: i.def.Name, HolderID: i.holder},
	}
}

// Data snapshots irreducible state in a detached map. It never saves attachment,
// a projected offer or the generated one-state effects runtime.
func (i *Item) Data() Data {
	data := Data{ID: i.id, DefinitionRef: i.def.Ref, Pools: make(map[string]int, len(i.pools))}
	for key, pool := range i.pools {
		data.Pools[key] = pool.Current
	}
	return data
}

func (i *Item) vocabulary() effects.OutputVocabulary {
	assess := func(frame any, bound *effects.Bound) (effects.Row, error) {
		verdict, reason := bound.Answer(frame)
		return effects.Row{Kind: bound.Kind(), Verdict: verdict, Reason: reason}, nil
	}
	return effects.OutputVocabulary{Kinds: map[effects.KindName]effects.KindSpec{
		kindGrant: {
			Check: func(raw jsontext.Value) error {
				key, err := bindingKey(raw)
				if err != nil {
					return err
				}
				if _, ok := i.def.Grants[key]; !ok {
					return fmt.Errorf("unknown grant binding %q", key)
				}
				return nil
			},
			Attach: func(_ context.Context, _ events.EventBus, bound *effects.Bound) (effects.Detach, error) {
				key, err := bindingKey(bound.Params())
				if err != nil {
					return nil, err
				}
				i.active[key] = true
				return func(context.Context) error { delete(i.active, key); return nil }, nil
			},
			Assess: assess,
		},
		kindRecover: {
			Check: func(raw jsontext.Value) error {
				key, err := bindingKey(raw)
				if err != nil {
					return err
				}
				if _, ok := i.pools[key]; !ok {
					return fmt.Errorf("unknown pool binding %q", key)
				}
				return nil
			},
			Attach: func(ctx context.Context, bus events.EventBus, bound *effects.Bound) (effects.Detach, error) {
				key, err := bindingKey(bound.Params())
				if err != nil {
					return nil, err
				}
				id, err := RecoveryTopic.On(bus).Subscribe(ctx, func(_ context.Context, event RecoveryEvent) error {
					if event.HolderID == bound.Host().Holder && slices.Contains(i.def.Pools[key].RestoreOn, event.Trigger) {
						i.pools[key].RestoreToFull()
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
	}}
}
