// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// ErrBadComposerData reports progress that cannot belong to the supplied
// original configuration. Loading never silently resets or rebinds an instance.
var ErrBadComposerData = errors.New("behavior: invalid composer data")

// ComposerData is the ordered continuation of a composer. The caller retains
// the original materialized configuration separately; programs and indexes are
// derived again, not persisted here. Order is also the authored precedence.
type ComposerData struct {
	Instances []InstanceData `json:"instances"`
}

// InstanceData identifies one attachment, the exact materialized definition
// its progress belongs to, and its private machine state. Definition is an
// integrity identity, not another mutable source of configuration.
type InstanceData struct {
	ID         string       `json:"id"`
	Definition string       `json:"definition"`
	Blob       machine.Blob `json:"blob"`
}

// ToData returns detached private progress. Mutating the result cannot change
// a live instance; the original configuration is not duplicated into this data.
func (c *Composer) ToData() ComposerData {
	out := ComposerData{Instances: make([]InstanceData, len(c.instances))}
	for i, instance := range c.instances {
		out.Instances[i] = InstanceData{
			ID: instance.spec.ID, Definition: instance.definition, Blob: cloneCompositionBlob(instance.blob),
		}
	}
	return out
}

// LoadComposerInput joins saved progress to the original materialized
// configuration and current supplied vocabulary. It does not resolve a mutable
// content registry or sample parameter values while loading.
type LoadComposerInput struct {
	Config ComposerConfig
	Data   ComposerData
}

// LoadComposer recompiles the configuration and restores validated private
// blobs. It refuses changed instance count/order/definition, an empty or unknown
// saved state, undeclared bag keys or wrong scalar kinds. Nil input returns
// ErrNilInput; configuration refusals retain ErrBadComposition, and invalid
// saved progress wraps ErrBadComposerData. No Step or die is invoked on load.
func LoadComposer(in *LoadComposerInput) (*Composer, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	out, err := NewComposer(&in.Config)
	if err != nil {
		return nil, err
	}
	if len(out.instances) != len(in.Data.Instances) {
		return nil, fmt.Errorf("%w: instance count differs from configuration", ErrBadComposerData)
	}
	for i, saved := range in.Data.Instances {
		instance := &out.instances[i]
		if saved.ID != instance.spec.ID || saved.Definition != instance.definition {
			return nil, fmt.Errorf("%w: instance %d identity/order/definition differs", ErrBadComposerData, i)
		}
		if err := checkCompositionBlob(instance.spec.Recipe, saved.Blob); err != nil {
			return nil, fmt.Errorf("%w: instance %q: %w", ErrBadComposerData, saved.ID, err)
		}
		instance.blob = cloneCompositionBlob(saved.Blob)
	}
	return out, nil
}

func checkCompositionBlob(recipe machine.Recipe, blob machine.Blob) error {
	if _, exists := recipe.States[blob.State]; blob.State == "" || !exists {
		return fmt.Errorf("saved state %q is not a named state", blob.State)
	}
	for _, name := range slices.Sorted(maps.Keys(blob.Bag)) {
		value := blob.Bag[name]
		zero, declared := recipe.Bag[name]
		if !declared {
			return fmt.Errorf("undeclared bag key %q", name)
		}
		if !sameCompositionScalar(zero, value) {
			return fmt.Errorf("bag key %q has wrong scalar kind %T", name, value)
		}
	}
	return nil
}

func sameCompositionScalar(zero, value any) bool {
	switch zero.(type) {
	case bool:
		_, ok := value.(bool)
		return ok
	case int:
		_, ok := value.(int)
		return ok
	case string:
		_, ok := value.(string)
		return ok
	default:
		return false
	}
}

func cloneCompositionBlob(blob machine.Blob) machine.Blob {
	out := blob
	out.Bag = blob.Bag.Clone()
	if blob.WakeAt != nil {
		at := *blob.WakeAt
		out.WakeAt = &at
	}
	if blob.Anchor != nil {
		at := *blob.Anchor
		out.Anchor = &at
	}
	return out
}
