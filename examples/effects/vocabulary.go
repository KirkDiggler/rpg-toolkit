// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"context"
	"encoding/json/jsontext"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// OutputVocabulary is the rulebook's plug into the composer: the output
// kinds and amounts a recipe may name, and the hooks a condition calls on
// its way out and when its bag changes. The machine knows no removal topic;
// End is where the rulebook publishes one.
type OutputVocabulary struct {
	Kinds   map[KindName]KindSpec
	Amounts map[AmountName]AmountFn

	// End is called when the machine enters its terminal state, before any
	// output detaches, with the transition's reason. Nil publishes nothing.
	End func(ctx context.Context, bus events.EventBus, h Host, reason string) error

	// Changed is called when a tracker or a staying transition changes the
	// bag. Nil publishes nothing.
	Changed func(ctx context.Context, bus events.EventBus, h Host) error
}

// KindSpec is one output kind: the Go code that knows its chain and stage.
//
//   - Check validates the output's params at Compose; nil accepts any.
//   - Attach subscribes the output on bus and returns how to detach it.
//   - Assess answers the output's information row for a frame; the same
//     guard answers execution and information.
type KindSpec struct {
	Check  func(params jsontext.Value) error
	Attach func(ctx context.Context, bus events.EventBus, o *Bound) (Detach, error)
	Assess func(frame any, o *Bound) (Row, error)
}

// AmountFn computes an amount from a frame; a is the output's Amount, whose
// one set field is the function's parameters. It returns an error when the
// frame cannot answer.
type AmountFn func(frame any, a Amount) (int, error)

// Detach undoes one Attach.
type Detach func(ctx context.Context) error

// Host is the condition as its outputs see it: who holds it, its ref, and
// the ref that applied it.
type Host struct {
	Holder string
	Ref    *core.Ref
	Source *core.Ref
}
