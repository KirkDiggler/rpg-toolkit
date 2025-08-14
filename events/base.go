// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import "github.com/KirkDiggler/rpg-toolkit/core"

// BaseEvent provides a standard implementation of the Event interface.
// Domain events can embed this to get the standard behavior.
type BaseEvent struct {
	ref     *core.Ref
	context *EventContext
}

// NewBaseEvent creates a new base event with the given ref
func NewBaseEvent(ref *core.Ref) *BaseEvent {
	return &BaseEvent{
		ref:     ref,
		context: NewEventContext(),
	}
}

// EventRef implements the Event interface
func (e *BaseEvent) EventRef() *core.Ref {
	return e.ref
}

// Context implements the Event interface
func (e *BaseEvent) Context() *EventContext {
	return e.context
}

// WithContext sets a specific context (useful for tests)
func (e *BaseEvent) WithContext(ctx *EventContext) *BaseEvent {
	e.context = ctx
	return e
}
