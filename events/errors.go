// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"fmt"
	"reflect"
)

// ErrDuplicateRef indicates multiple event types are using the same ref string
type ErrDuplicateRef struct {
	RefString      string
	ExistingType   string
	IncomingType   string
}

func (e *ErrDuplicateRef) Error() string {
	return fmt.Sprintf(
		"duplicate event ref %q: already registered by %s, attempted by %s",
		e.RefString,
		e.ExistingType,
		e.IncomingType,
	)
}

// ErrRefMismatch indicates an event's ref doesn't match its TypedRef
// This should never happen if packages are well-formed
type ErrRefMismatch struct {
	EventType    string
	EventRef     string
	ExpectedRef  string
}

func (e *ErrRefMismatch) Error() string {
	return fmt.Sprintf(
		"BUG: %s has ref mismatch - event.EventRef() returns %q but TypedRef has %q",
		e.EventType,
		e.EventRef,
		e.ExpectedRef,
	)
}

// Internal registry to detect duplicate refs
var eventRegistry = make(map[string]reflect.Type)

// RegisterEventType records an event type for its ref string
// Returns error if another type already uses this ref
func RegisterEventType(refString string, eventType reflect.Type) error {
	if existing, exists := eventRegistry[refString]; exists {
		if existing != eventType {
			return &ErrDuplicateRef{
				RefString:    refString,
				ExistingType: existing.String(),
				IncomingType: eventType.String(),
			}
		}
	}
	eventRegistry[refString] = eventType
	return nil
}