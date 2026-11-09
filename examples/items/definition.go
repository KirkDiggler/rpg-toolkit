// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

// ErrInvalidConfig identifies refused authored configuration or saved instance data.
var ErrInvalidConfig = errors.New("invalid item configuration")

// Trigger is a consumer-defined recovery occurrence, not a rulebook rest enum.
type Trigger string

// PoolDefinition declares a finite capacity and the occurrences that refill it.
type PoolDefinition struct {
	Maximum   int       `json:"maximum"`
	RestoreOn []Trigger `json:"restore_on,omitempty"`
}

// Grant is inert consumer-owned ability data and its price in an item-owned pool.
// Items interprets Pool and Uses; the consumer validates/interprets Ref and Parameters.
type Grant struct {
	Ref        core.Ref        `json:"ref"`
	Parameters json.RawMessage `json:"parameters,omitempty"`
	Pool       string          `json:"pool"`
	Uses       int             `json:"uses"`
}

// Definition is authored item content, separate from each instance's remaining uses.
// This exploratory cut supports finite charged grants, not passive-item recipes.
type Definition struct {
	Ref    core.Ref                  `json:"ref"`
	Name   string                    `json:"name"`
	Pools  map[string]PoolDefinition `json:"pools"`
	Grants map[string]Grant          `json:"grants"`
}

// Data is the irreducible state of one instance. Attachment is not persisted.
// Pool capacity comes from Definition; a restored record must name every pool.
type Data struct {
	ID            string         `json:"id"`
	DefinitionRef core.Ref       `json:"definition_ref"`
	Pools         map[string]int `json:"pools"`
}

// Config supplies authored data, identity, and runtime consumer validation.
// State nil explicitly creates a new full-budget item. A non-nil State is restored
// strictly, never clamped or refilled. ValidateGrant must reject unsupported rules.
type Config struct {
	ID               string
	Definition       Definition
	State            *Data
	RecoveryTriggers []Trigger
	ValidateGrant    func(Grant) error
}

func cloneGrant(g Grant) Grant {
	g.Parameters = bytes.Clone(g.Parameters)
	return g
}

func prepare(config Config) (Definition, map[string]*resources.Resource, error) {
	def := config.Definition
	if strings.TrimSpace(config.ID) == "" || strings.TrimSpace(def.Name) == "" {
		return Definition{}, nil, fmt.Errorf("%w: instance id and name are required", ErrInvalidConfig)
	}
	if err := def.Ref.IsValid(); err != nil {
		return Definition{}, nil, fmt.Errorf("%w: definition ref: %w", ErrInvalidConfig, err)
	}
	if len(def.Grants) == 0 || config.ValidateGrant == nil {
		return Definition{}, nil, fmt.Errorf("%w: grants and a consumer validator are required", ErrInvalidConfig)
	}
	if state := config.State; state != nil {
		if state.ID != config.ID || state.DefinitionRef != def.Ref || len(state.Pools) != len(def.Pools) {
			return Definition{}, nil, fmt.Errorf("%w: saved identity, definition or pool set differs", ErrInvalidConfig)
		}
	}
	owned := Definition{Ref: def.Ref, Name: def.Name, Pools: map[string]PoolDefinition{}, Grants: map[string]Grant{}}
	pools := make(map[string]*resources.Resource, len(def.Pools))
	for key, p := range def.Pools {
		if strings.TrimSpace(key) == "" || p.Maximum <= 0 {
			return Definition{}, nil, fmt.Errorf("%w: pool %q needs a positive maximum", ErrInvalidConfig, key)
		}
		for _, trigger := range p.RestoreOn {
			if strings.TrimSpace(string(trigger)) == "" || !slices.Contains(config.RecoveryTriggers, trigger) {
				return Definition{}, nil, fmt.Errorf("%w: pool %q names unsupported recovery %q", ErrInvalidConfig, key, trigger)
			}
		}
		remaining := p.Maximum
		if state := config.State; state != nil {
			var ok bool
			remaining, ok = state.Pools[key]
			if !ok || remaining < 0 || remaining > p.Maximum {
				return Definition{}, nil, fmt.Errorf("%w: saved pool %q is missing or out of bounds", ErrInvalidConfig, key)
			}
		}
		p.RestoreOn = slices.Clone(p.RestoreOn)
		owned.Pools[key] = p
		pool := resources.NewResource(key, p.Maximum)
		pool.SetCurrent(remaining)
		pools[key] = pool
	}
	for key, g := range def.Grants {
		pool, ok := def.Pools[g.Pool]
		if strings.TrimSpace(key) == "" || !ok || g.Uses <= 0 || g.Uses > pool.Maximum {
			return Definition{}, nil, fmt.Errorf("%w: grant %q has an invalid pool or price", ErrInvalidConfig, key)
		}
		if err := g.Ref.IsValid(); err != nil {
			return Definition{}, nil, fmt.Errorf("%w: grant %q ref: %w", ErrInvalidConfig, key, err)
		}
		if len(g.Parameters) > 0 && !json.Valid(g.Parameters) {
			return Definition{}, nil, fmt.Errorf("%w: grant %q parameters are not JSON", ErrInvalidConfig, key)
		}
		if err := config.ValidateGrant(cloneGrant(g)); err != nil {
			return Definition{}, nil, fmt.Errorf("%w: grant %q: %w", ErrInvalidConfig, key, err)
		}
		owned.Grants[key] = cloneGrant(g)
	}
	return owned, pools, nil
}
