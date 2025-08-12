// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"time"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// ApplyOptions holds configuration for applying a condition
type ApplyOptions struct {
	Source   string        // What caused this condition
	SaveDC   int           // DC for saves to end the condition
	Level    int           // Level of the condition (e.g., exhaustion)
	Duration time.Duration // How long the condition lasts
	Metadata map[string]interface{}
}

// ApplyOption is a function that modifies ApplyOptions
type ApplyOption func(*ApplyOptions)

// WithSource sets what caused this condition
func WithSource(source string) ApplyOption {
	return func(opts *ApplyOptions) {
		opts.Source = source
	}
}

// WithSaveDC sets the DC for saves to end the condition
func WithSaveDC(dc int) ApplyOption {
	return func(opts *ApplyOptions) {
		opts.SaveDC = dc
	}
}

// WithLevel sets the level of the condition (e.g., exhaustion levels)
func WithLevel(level int) ApplyOption {
	return func(opts *ApplyOptions) {
		opts.Level = level
	}
}

// WithDuration sets how long the condition lasts
func WithDuration(duration time.Duration) ApplyOption {
	return func(opts *ApplyOptions) {
		opts.Duration = duration
	}
}

// WithMetadata adds custom metadata to the condition
func WithMetadata(key string, value interface{}) ApplyOption {
	return func(opts *ApplyOptions) {
		if opts.Metadata == nil {
			opts.Metadata = make(map[string]interface{})
		}
		opts.Metadata[key] = value
	}
}

// WithConcentration marks this condition as requiring concentration
func WithConcentration() ApplyOption {
	return func(opts *ApplyOptions) {
		if opts.Metadata == nil {
			opts.Metadata = make(map[string]interface{})
		}
		opts.Metadata["concentration"] = true
	}
}

// WithRelatedEntity sets a related entity for the condition
func WithRelatedEntity(key string, entity core.Entity) ApplyOption {
	return func(opts *ApplyOptions) {
		if opts.Metadata == nil {
			opts.Metadata = make(map[string]interface{})
		}
		opts.Metadata[key] = entity
	}
}
