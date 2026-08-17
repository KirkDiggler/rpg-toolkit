// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// sentinels_test.go pins the refusals a caller can DRIVE this seam into. This
// file pins the ones it cannot (rpg-toolkit#1058).
//
// Some composition sentinels arrive only through state no blob LoadEncounter
// accepts can produce — Members reporting that the roster and the spatial field
// disagree about who is placed, most concretely. There is no verb call that
// reaches those today, and a test that faked one would be pinning the fake.
// Testing translate directly is the honest version: it is where the mapping
// lives, and an arm that is missing there is a leak the moment its path opens.
func TestTranslateLetsNoCompositionSentinelThrough(t *testing.T) {
	// Every arm translate carries, asserted in both directions. The second
	// assertion is the load-bearing one: a translation that returned
	// fmt.Errorf("%w: %w", ours, theirs) would satisfy the first and leak
	// exactly as badly as no translation at all.
	cases := []struct {
		name  string
		inner error
		want  error
	}{
		{"trimmed story", encounter.ErrTrimmed, ErrStoryTrimmed},
		{"empty member id", encounter.ErrNoMember, ErrNoMember},
		{"not a member", encounter.ErrNotMember, ErrNoMember},
		{"closed encounter", encounter.ErrClosed, ErrClosed},
		{"undeclared ending", encounter.ErrNoEnding, ErrNoEnding},
		{"unknown connection", encounter.ErrNoConnection, ErrNoConnection},
		{"malformed connection", encounter.ErrBadConnection, ErrNoConnection},
		{"bad placement", encounter.ErrBadPlacement, ErrBadPosition},
		{"already in a fight", encounter.ErrInBubble, ErrInBubble},
		{"not in a fight", encounter.ErrNoBubble, ErrNotInFight},
		// The two the reads needed. Members fails with ErrNoField when the
		// roster and the field disagree about who is placed, which is what
		// Where, whereIs and Attack all read through; ErrInvalidData is what a
		// blob that cannot be reconstituted carries.
		{"defective field", encounter.ErrNoField, ErrInvalidWorld},
		{"unreadable blob", encounter.ErrInvalidData, ErrInvalidWorld},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Wrapped, the way a composition verb really returns it, so the
			// arms are exercised through errors.Is rather than by identity.
			out := translate(fmt.Errorf("members: alice: %w", tc.inner))

			require.ErrorIs(t, out, tc.want,
				"the host is answered in this package's vocabulary")
			require.NotErrorIs(t, out, tc.inner,
				"and must not be able to reach the composition's sentinel — matching on it "+
					"would couple every host to a module we intend to replace (S2)")
		})
	}
}

// TestTranslateResolutionLetsNoResolutionSentinelThrough is the same table over
// the swing's translation (rpg-toolkit#1066).
//
// One of these arms IS reachable from Attack and is driven there, in
// sentinels_test.go, where a real host mistake produces it. The rest are not:
// a member with no stored sheet is refused by name before the strike runs, and
// Attack always builds an input and always hands over a machine, so
// ErrNoCombatant, ErrNilInput and ErrNoMachine have no path from a verb. They
// keep their arms because an unmapped sentinel is a leak the moment its path
// opens, and this is where that promise is checkable without a fake standing in
// for the module that would have produced it.
func TestTranslateResolutionLetsNoResolutionSentinelThrough(t *testing.T) {
	cases := []struct {
		name  string
		inner error
		want  error
	}{
		// Driven for real in sentinels_test.go: one stored sheet answering to
		// two member IDs.
		{"one sheet under two ids", resolution.ErrBadParticipant, ErrBadCharacter},
		// The backstop: a member with no stored sheet is refused by name
		// before the strike runs, so this arm is the one that catches a
		// combatant the cast turned out not to hold.
		{"combatant not in the cast", resolution.ErrNoCombatant, ErrNoSheet},
		// Defects here rather than in the call, and unreachable for that
		// reason.
		{"no input at all", resolution.ErrNilInput, ErrNilInput},
		{"nothing to resolve", resolution.ErrNoMachine, ErrNilInput},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := translateResolution(fmt.Errorf("resolution: attach character %q: %w", "alice", tc.inner))

			require.ErrorIs(t, out, tc.want,
				"the host is answered in this package's vocabulary")
			require.NotErrorIs(t, out, tc.inner,
				"and must not be able to reach the resolution module's sentinel — matching on "+
					"it would couple every host to the module the strike machine lives in (S2)")
			require.Contains(t, out.Error(), tc.inner.Error(),
				"while the reason itself survives as text, for whoever debugs it")
		})
	}
}
