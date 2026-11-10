// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

// TestRebindBindsEveryCapability holds Rebind to "what a load binds": every
// member of [Capabilities] (and of its embedded [Actors]) is held on the
// encounter under its own name lowered, and Rebind sets each one. A
// capability added to Capabilities and not to Rebind leaves its field as the
// test cleared it and fails here by name. The Witness is held only on a
// concealed field, as at Load.
func TestRebindBindsEveryCapability(t *testing.T) {
	enc, err := NewEncounter(wakeSetup(&askedDriver{inner: BehaviorDriver{}}))
	require.NoError(t, err)
	caps := wakeCapabilities(&askedDriver{inner: BehaviorDriver{}})
	caps.Disengager = quietDisengager{}
	caps.CheckResolver = RefusingCheckResolver{}
	caps.Witness = nobodyPerceives{}

	names := capabilityNames(reflect.TypeOf(caps))
	self := reflect.ValueOf(enc).Elem()
	for _, name := range names {
		field := heldField(t, self, name)
		field.Set(reflect.Zero(field.Type()))
	}

	require.NoError(t, enc.Rebind(&RebindInput{Capabilities: caps}))

	given := reflect.ValueOf(caps)
	for _, name := range names {
		field := heldField(t, self, name)
		want := given.FieldByName(name)
		if name == "Witness" {
			require.True(t, field.IsNil(), "a plain field holds no witness")
			continue
		}
		require.False(t, field.IsNil(), "Rebind binds %s", name)
		require.True(t, field.Interface() == want.Interface(), "Rebind binds the %s it was given", name)
	}
}

// capabilityNames is every interface member of Capabilities, flattening the
// embedded Actors.
func capabilityNames(t reflect.Type) []string {
	var out []string
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Anonymous {
			out = append(out, capabilityNames(f.Type)...)
			continue
		}
		out = append(out, f.Name)
	}
	return out
}

// heldField is the encounter's field for capability name, readable and
// settable from the test.
func heldField(t *testing.T, self reflect.Value, name string) reflect.Value {
	t.Helper()
	held := strings.ToLower(name[:1]) + name[1:]
	f := self.FieldByName(held)
	require.True(t, f.IsValid(), "the encounter holds capability %s as %s", name, held)
	return reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem() //nolint:gosec // a test reaching its own fields
}

// quietDisengager is a Disengager that does nothing, so the optional actor
// has a value to bind.
type quietDisengager struct{}

func (quietDisengager) Disengage(context.Context, *Encounter, MemberID) error { return nil }
