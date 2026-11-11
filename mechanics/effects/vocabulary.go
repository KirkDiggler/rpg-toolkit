// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"context"

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
	// bag, or after a nonterminal transition successfully attaches its new
	// outputs. Terminal transitions use End instead. Nil publishes nothing.
	Changed func(ctx context.Context, bus events.EventBus, h Host) error
}

// KindSpec is one output kind: the Go code that knows its chain and stage.
//
//   - Check validates the complete output at Compose, including requirements
//     on its amount and params; nil adds no kind-specific validation.
//   - Attach subscribes the output on bus and returns how to detach it.
//   - Assess answers the output's information row for a frame; the same
//     guard answers execution and information.
type KindSpec struct {
	// Execution declares whether this kind subscribes or contributes by query.
	Execution KindExecution
	Check     func(output Output) error
	Attach    func(ctx context.Context, bus events.EventBus, o *Bound) (Detach, error)
	Assess    func(frame any, o *Bound) (Row, error)
}

// KindExecution declares the delivery mechanism of an output kind.
type KindExecution string

const (
	// KindSubscribed is the default and requires an Attach implementation.
	KindSubscribed KindExecution = ""
	// KindQuery describes a contribution on demand and forbids Attach.
	KindQuery KindExecution = "query"
)

// AmountFn describes an amount from a frame. Scalars are settled; dice remain
// unresolved until the consuming roll path evaluates them. The function must not
// roll or substitute zero for a frame it cannot answer.
type AmountFn func(frame any, a Amount) (AmountResult, error)

// Detach undoes one Attach.
type Detach func(ctx context.Context) error

// Host is the condition as its outputs see it: who holds it, its ref, and
// the ref that applied it.
type Host struct {
	Holder string    // the entity id the condition is on
	Ref    *core.Ref // the recipe's ref, never nil
	Source *core.Ref // what applied the condition, possibly nil
}
