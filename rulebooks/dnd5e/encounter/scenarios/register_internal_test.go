// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package scenarios

// register_internal_test.go exercises the registry's own rules
// (rpg-toolkit#1499 review).
//
// INTERNAL, and it has to be: [register] panics from a scenario package's
// init, which is a process that has already died by the time any test could
// look at it. [registerInto] is that decision with the panic lifted off, so
// the rules can be asserted rather than described.
//
// The finding this file exists for is worth stating plainly, because it is a
// test-quality lesson as much as a code one: the external suite already
// asserted that no two scenarios share an id, and that assertion COULD NOT
// FAIL. It walked [All], which walks the map — so a duplicate id silently
// overwrote the first scenario and the survivor was, trivially, unique. The
// check has to be at the write, not over the reads.

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeScenario is the least a registry entry can be.
type fakeScenario struct {
	id, name string
}

func (f fakeScenario) ID() string      { return f.id }
func (f fakeScenario) Name() string    { return f.name }
func (f fakeScenario) Fields() []Field { return nil }
func (f fakeScenario) New(map[string]string, *DungeonFacts) (Declared, error) {
	return Declared{}, nil
}

func TestTheRegistryRefusesWhatWouldVanish(t *testing.T) {
	t.Run("a first registration lands", func(t *testing.T) {
		into := map[string]Scenario{}
		require.NoError(t, registerInto(into, fakeScenario{id: "recover", name: "Recover"}))
		require.Len(t, into, 1)
	})

	t.Run("a duplicate id is refused, naming the scenario that has it", func(t *testing.T) {
		// The whole point: without this, the second write wins, the first
		// scenario stops existing, and nothing anywhere says so.
		into := map[string]Scenario{}
		require.NoError(t, registerInto(into, fakeScenario{id: "recover", name: "Recover"}))
		err := registerInto(into, fakeScenario{id: "recover", name: "Recover Again"})
		require.ErrorContains(t, err, "already registered")
		require.ErrorContains(t, err, "Recover")
		require.Equal(t, "Recover", into["recover"].Name(), "and the first one is still the one there")
		require.Len(t, into, 1)
	})

	t.Run("an empty id is refused", func(t *testing.T) {
		require.ErrorContains(t, registerInto(map[string]Scenario{}, fakeScenario{}),
			"no id")
	})

	t.Run("a nil scenario is refused rather than panicking on ID()", func(t *testing.T) {
		require.ErrorContains(t, registerInto(map[string]Scenario{}, nil), "nil scenario")
	})

	t.Run("the real registry registered exactly what it declares", func(t *testing.T) {
		// register panics on a collision, so reaching this line at all is
		// the assertion: every scenario package's init ran and none of them
		// collided.
		require.NotEmpty(t, registry)
		for id, s := range registry {
			require.Equal(t, id, s.ID(), "the map key is the scenario's own id")
		}
	})
}
