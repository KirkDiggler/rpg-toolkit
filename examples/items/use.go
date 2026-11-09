// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items

import (
	"context"
	"fmt"
)

// UseInput selects an existing grant, never caller-supplied cost or parameters.
// Resolve must validate/compute a typed outcome without delivering world mutations
// or re-entering/mutating the item. The caller delivers the returned result once.
// A failure can then return the locally reserved charge without pretending to
// roll back arbitrary effects. This boundary remains an example experiment.
type UseInput[Result any] struct {
	Item     *Item
	HolderID string
	GrantID  string
	Resolve  func(context.Context, Offer) (Result, error)
}

// UseOutput returns the consumer's typed result and settled item-budget facts.
// Items never interprets Result or serializes it through an untyped intermediate.
type UseOutput[Result any] struct {
	Result    Result
	Source    Source
	Remaining int
}

// Use checks the current association/grant/budget, reserves its authored price,
// invokes the pure consumer resolver, and returns the typed outcome. Missing or
// unavailable inputs refuse before resolution. Resolver failure refunds this
// item's reservation; success keeps it spent. Panics and concurrent/reentrant
// mutation are not transactional guarantees of this experimental API.
func Use[Result any](ctx context.Context, input UseInput[Result]) (*UseOutput[Result], error) {
	if input.Item == nil || input.Resolve == nil {
		return nil, fmt.Errorf("%w: item and resolver are required", ErrInvalidConfig)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	i := input.Item
	if i.composed == nil || input.HolderID != i.holder || !i.active[input.GrantID] {
		return nil, fmt.Errorf("%w: grant %q for holder %q", ErrUnavailable, input.GrantID, input.HolderID)
	}
	offer := i.offer(input.GrantID)
	if !offer.Affordable {
		return nil, fmt.Errorf("%w: exhausted pool %q", ErrUnavailable, offer.Grant.Pool)
	}
	pool := i.pools[offer.Grant.Pool]
	if err := pool.Use(offer.Grant.Uses); err != nil {
		return nil, err
	}
	result, err := input.Resolve(ctx, offer)
	if err != nil {
		pool.Restore(offer.Grant.Uses)
		return nil, err
	}
	return &UseOutput[Result]{Result: result, Source: offer.Source, Remaining: pool.Current}, nil
}
