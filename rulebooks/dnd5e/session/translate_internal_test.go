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
		// Doors are things with identity and state (the S4 slice), and since
		// rpg-project#256 they are the only crossing with a name: a connection
		// was a room-chain artefact, and its sentinel left with the rooms.
		{"no such door", encounter.ErrNoDoor, ErrNoConnection},
		{"malformed door", encounter.ErrBadDoor, ErrNoConnection},
		// A locked door is a FICTION BEAT and gets its own sentinel: there is a
		// DC behind it and something for a player to do about it. It must not
		// arrive as ErrBadPosition — "that is not a place" and "the way is
		// shut" send whoever reads them somewhere different.
		{"locked door", encounter.ErrLocked, ErrLocked},
		// NOTE: this table used to carry `ErrNoCrossing` for a step into an
		// adjacent cell with no doorway joining it. The composition no longer
		// distinguishes that case — a walk into a shut door comes back as a
		// placement refusal carrying the door's state in TEXT — so the sentinel
		// was removed rather than kept as one nothing can produce.
		// rpg-toolkit#1135 is what would give it a source again.
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
		{"target beyond delivery", resolution.ErrOutOfRange, ErrOutOfReach},
		// Driven for real in sentinels_test.go: a second swing in a turn that
		// bought one. The PLAYER-facing arm, and the only one of the economy's
		// three a caller can reach.
		{"an actor who has run out", resolution.ErrCannotPay, ErrCannotAfford},
		// The economy's programmer-facing pair, unreachable because this package
		// compiles the only prices it charges and names an attacker who is always
		// in the cast. They are kept APART from the arm above deliberately: E2
		// split them at the door so that a malformed profile could never reach a
		// client as "out of actions", and flattening them here would undo that
		// split one layer further out (rpg-toolkit#1097).
		{"a price nobody could be charged", resolution.ErrBadCost, ErrBadCost},
		{"a payer this cast cannot charge", resolution.ErrNoPayer, ErrBadCost},
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

// unknownCause is a cause from a composition NEWER than this build.
//
// It is built by embedding a real one, and that is not a trick to get around
// the seal — it is the only honest way to model the case, because the seal
// really works. This package cannot declare a third cause: isDissolveCause is
// unexported in the composition, so a hand-rolled struct does not satisfy the
// interface and does not compile. Embedding borrows a genuine case's seal and
// overrides only the answer, which is exactly the shape of the thing this arm
// guards against: a value that satisfies the interface because the composition
// made it, reporting a kind this build has no name for.
//
// That is not hypothetical. It is what THIS slice was: the composition grew
// ByDefeat (rpg-toolkit#1078) and this package had to grow its twin. Had it not,
// every defeat would have arrived here as an unrecognised kind.
type unknownCause struct {
	encounter.DissolveCause
}

func (unknownCause) Kind() encounter.DissolveKind { return "surrendered" }

// TestCauseOfIsTotalOverTheCompositionsCauses is the cause translation's twin
// of the two tables above, and it is asserting something they are not.
//
// Those tables are about what a host must NOT be able to reach. This one is
// about completeness: a fight ends two ways, both of them arrive through this
// one function, and a mapping that quietly answered "decision" for a fight lost
// by defeat would narrate the wrong thing forever with nothing failing. That is
// kindOf's documented hazard — silent degradation — and the difference here is
// that this function can refuse instead.
//
// The defeat row cannot be driven through Manager.Dissolve, and that is not a
// gap. The composition's Dissolve VERB is the decision, so it only ever reports
// one cause; defeat reaches a client through the story, where the end-to-end
// scene in death_test.go pins it. This is where the other half of the sealed
// set is checked to exist and to map.
func TestCauseOfIsTotalOverTheCompositionsCauses(t *testing.T) {
	cases := []struct {
		name string
		in   encounter.DissolveCause
		want DissolveKind
	}{
		{"the party broke off", encounter.ByDecision(), DissolveByDecision},
		{"a side stopped standing", encounter.ByDefeat(), DissolveByDefeat},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := causeOf(tc.in)

			require.NoError(t, err)
			require.Equal(t, tc.want, out.Kind(),
				"the composition's account, said in this package's own words")
		})
	}

	t.Run("a cause this build has no name for", func(t *testing.T) {
		out, err := causeOf(unknownCause{DissolveCause: encounter.ByDecision()})

		require.Nil(t, out)
		require.ErrorIs(t, err, ErrInvalidWorld,
			"refused rather than flattened onto a cause we happen to know")
		require.Contains(t, err.Error(), "surrendered",
			"and the unrecognised cause is named, so the gap is findable")
	})
}
