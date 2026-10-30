// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Vocabulary is the rulebook's plug into the machine: the facts a guard may
// read and the beats a transition or tracker may listen to. Validate checks
// every recipe against the vocabulary it is given.
type Vocabulary struct {
	Facts map[FactName]FactSpec
	Beats map[Beat]BeatSpec
}

// ScalarKind is the kind of a fact or bag value.
type ScalarKind string

const (
	// KindBool is a bool scalar.
	KindBool ScalarKind = "bool"
	// KindInt is an int scalar.
	KindInt ScalarKind = "int"
	// KindString is a string scalar.
	KindString ScalarKind = "string"
)

// FactSpec declares one fact: its kind, and Read, which returns the fact's
// value from an opaque frame or event and whether it is known. A fact Read
// reports unknown makes its leaf answer Depends.
type FactSpec struct {
	Kind ScalarKind
	Read func(frame any) (any, bool)
}

// BeatSpec declares one beat.
//
//   - Selectors are the About values a recipe may use on this beat.
//   - Subscribe attaches h to the beat's topic on bus and returns the
//     subscription id; the machine unsubscribes it through bus.
//   - About returns the id the selector names on the event, and false when
//     the event carries none.
//   - Round returns the beat's round and whether it carries one; nil means
//     the beat never does, and Validate refuses a rounds leaf on it.
type BeatSpec struct {
	Selectors []Selector
	Subscribe func(ctx context.Context, bus events.EventBus, h func(ctx context.Context, ev any) error) (string, error)
	About     func(ev any, sel Selector) (string, bool)
	Round     func(ev any) (int, bool)
}

// supports reports whether the beat lists sel.
func (b BeatSpec) supports(sel Selector) bool {
	for _, s := range b.Selectors {
		if s == sel {
			return true
		}
	}
	return false
}

// kindOf returns the scalar kind of a bag or leaf value, and false for any
// other type.
func kindOf(v any) (ScalarKind, bool) {
	switch v.(type) {
	case bool:
		return KindBool, true
	case int:
		return KindInt, true
	case string:
		return KindString, true
	}
	return "", false
}
