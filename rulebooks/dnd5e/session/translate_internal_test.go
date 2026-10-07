// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
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
		{"malformed reach", encounter.ErrBadReach, ErrBadReach},
		{"closed encounter", encounter.ErrClosed, ErrClosed},
		{"automatic discovery retires search", encounter.ErrSearchRetired, ErrSearchRetired},
		{"undeclared ending", encounter.ErrNoEnding, ErrNoEnding},
		// A faction the dungeon does not declare, or a mind arriving outside
		// its faction (rpg-project#375): a naming mistake in the host's
		// forwarding, and its own sentinel so the host can tell it from a
		// record it forgot to mint (ErrNoIntel) or a cell nobody owns.
		{"no such faction", encounter.ErrNoFaction, ErrNoFaction},
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
		// The other half of the rpg-toolkit#1135 split, landed: a walk into a
		// merely-shut door arrives on the composition's own sentinel now, not
		// as a placement refusal with the state in text. (This table once
		// carried `ErrNoCrossing` for the sourceless version of this case;
		// #1135 is what gave the distinction a source again.)
		{"shut door", encounter.ErrDoorShut, ErrDoorShut},
		// A turn stopped mid-walk waiting on an answer. The composition and
		// this seam name the same fact from either side of the boundary, and
		// the arm exists for the half-failed save that is the only way to
		// reach it — see translate's own note.
		{"turn paused", encounter.ErrTurnPaused, ErrWindowOpen},
		{"bad placement", encounter.ErrBadPlacement, ErrBadPosition},
		{"already in a fight", encounter.ErrInBubble, ErrInBubble},
		{"not in a fight", encounter.ErrNoBubble, ErrNotInFight},
		// The two the reads needed. Members fails with ErrNoField when the
		// roster and the field disagree about who is placed, which is what
		// Where, whereIs and Attack all read through; ErrInvalidData is what a
		// blob that cannot be reconstituted carries.
		{"defective field", encounter.ErrNoField, ErrInvalidWorld},
		{"unreadable blob", encounter.ErrInvalidData, ErrInvalidWorld},
		// The holdings verbs' four refusals (rpg-project#368). ErrNotDown is
		// ordinary — a body is visible and being on the floor is not a
		// secret. The three prop refusals say what they mean only for a prop
		// the member can SEE; for one they cannot, the composition already
		// answers all of them with ErrNoProp, which is the probe law and is
		// pinned as bytes in holdings_test.go rather than here.
		{"body still standing", encounter.ErrNotDown, ErrNotDown},
		// An attack pointed at a world NPC (rpg-project#493, R4). Not driven
		// from a verb: this seam refuses the same swing one layer earlier, by
		// name, because its candidate universe never held the merchant in the
		// first place. The arm is here for the reason the resolution arms
		// below are — the exclusion above it is a convenience, and an
		// unmapped sentinel leaks the moment that convenience moves.
		{"npc is not a target", encounter.ErrNotATarget, ErrNotATarget},
		{"no such prop", encounter.ErrNoProp, ErrNoProp},
		{"prop is scenery", encounter.ErrNotHoldable, ErrNotHoldable},
		{"prop already carried", encounter.ErrAlreadyHeld, ErrAlreadyHeld},
		// The sheet capability's two refusals (rpg-project#538): an answer
		// that skipped a member, and a compile-only world asked to pace or
		// reach one. Neither is reachable while the sheet seam answers every
		// member it is asked about or refuses by its own name first; the arms
		// keep the composition's words behind the boundary if that changes.
		{"sheet answer skipped a member", encounter.ErrNoSheets, ErrNoSheet},
		{"compiled world asked for sheets", encounter.ErrRefusingSheets, ErrInvalidWorld},
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
		// Out of range on every verb but a swing, which translateAttack owns.
		{"target beyond range", resolution.ErrOutOfRange, ErrOutOfRange},
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
		// The activation pair, split for the economy pair's reason and worth
		// the same note about what each can actually be reached by.
		//
		// ErrBadActivation IS reachable from a caller: Manager.Activate passes
		// Target through, and naming one for an ability that takes none is
		// refused at the machine's preflight. Driven end to end by
		// TestAMalformedActivationIsNotAnAbilityRefusing.
		{"an activation nobody could run", resolution.ErrBadActivation, ErrBadActivation},
		// ErrActivationRefused is NOT reachable through Manager.Activate today,
		// and saying so is better than implying a path. Afford consults the same
		// gates ActivateAbility does, so an unavailable ability is refused as a
		// stale selector before the sheet is ever asked; the one combat ability
		// with a precondition beyond the economy is Help, whose missing target
		// the machine catches first as ErrBadActivation.
		//
		// The arm exists anyway, and not as a placeholder. The sheet's contract
		// genuinely answers refusals as (output{Success:false}, nil), so a verb
		// that assumed the two gates always agree would report a refusal as a
		// SUCCESSFUL activation that did nothing. This is that assumption
		// declined, and it is tested here rather than through a scenario
		// because inventing one would mean building a disagreement that does
		// not exist.
		{"an ability that said no", resolution.ErrActivationRefused, ErrCannotActivate},
		// The ward carries no DC: a Sanctuary written before wards recorded
		// their caster's spell save DC at cast refuses rather than reading as
		// DC 0 (rpg-toolkit#1965). Bad stored data on the holder's sheet.
		{"a ward that carries no DC", resolution.ErrWardUnreadable, ErrBadCharacter},
		// A class-scaled rule handed a frame it cannot answer from — the
		// actor's sheet holds no levels in the class its effect scales with
		// (rpg-project#538). The sheet's problem, so this package's word for a
		// sheet it cannot use; the effect fails rather than reading level one.
		{"a class-scaled rule with no levels to read", contributions.ErrRuleCannotAnswer, ErrBadCharacter},
		// The sheet capability not supplied, or its answer skipping a member,
		// as resolution reports them back out of its own load.
		{"no sheets capability", resolution.ErrNoSheets, ErrNoSheet},
		{"sheet answer skipped a member", encounter.ErrNoSheets, ErrNoSheet},
		{"compiled world asked for sheets", encounter.ErrRefusingSheets, ErrInvalidWorld},
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

// TestASheetTheSeamCouldNotReadIsTheCause: the sheet seam is consulted from
// inside a resolution — a cost's witnesses ask every member's sight — and
// resolution wraps the seam's refusal in its own sentinel for what it was
// doing at the time. The host's repair is the sheet's, so the seam's own word
// wins, and the resolution sentinel it was wrapped in stays unreachable.
func TestASheetTheSeamCouldNotReadIsTheCause(t *testing.T) {
	for _, own := range []error{
		ErrNoCharacter, ErrBadCharacter, ErrNoSheet, ErrBadRepository, ErrInvalidSession, ErrBadAttack,
	} {
		t.Run(own.Error(), func(t *testing.T) {
			inner := fmt.Errorf("%w: witnesses: sight: member %q: %w", resolution.ErrBadCost, "bob", own)

			out := translateResolution(inner)
			require.ErrorIs(t, out, own)
			require.NotErrorIs(t, out, resolution.ErrBadCost, "no resolution sentinel reaches the host (S2)")
			require.NotErrorIs(t, out, ErrBadCost, "and the cost is not blamed for a sheet")

			priced := badCostUnlessSheet(inner)
			require.ErrorIs(t, priced, own)
			require.NotErrorIs(t, priced, ErrBadCost)
		})
	}

	require.ErrorIs(t, badCostUnlessSheet(errors.New("ledger refused")), ErrBadCost,
		"a pricing failure that is not a sheet is still the cost's")
}

// TestADependencyRefusalNamesASheetOnlyWhenOneIsNamed: an unreadable
// participant's refusal is a sheet's only when a member is named and a sheet
// sentinel says so. A failure against no member is a cast this package built
// wrongly, and it keeps resolution's translated word rather than sending a
// host to repair a sheet that is fine.
func TestADependencyRefusalNamesASheetOnlyWhenOneIsNamed(t *testing.T) {
	unnamed := dependencyRefusal(resolutionDependencyFailure{
		member: "", err: fmt.Errorf("%w: empty cast", resolution.ErrNilInput),
	})
	require.ErrorIs(t, unnamed, ErrNilInput)
	require.NotErrorIs(t, unnamed, ErrBadCharacter, "no member named, no sheet blamed")
	require.NotErrorIs(t, unnamed, resolution.ErrNilInput, "and resolution's own word stays behind the boundary")

	for _, own := range []error{ErrNoSheet, ErrNoCharacter, ErrBadCharacter, ErrBadRepository} {
		named := dependencyRefusal(resolutionDependencyFailure{member: "bob", err: fmt.Errorf("load: %w", own)})
		require.ErrorIs(t, named, own)
		require.Contains(t, named.Error(), `"bob"`)
	}

	translated := dependencyRefusal(resolutionDependencyFailure{
		member: "bob", err: fmt.Errorf("attach: %w", resolution.ErrBadParticipant),
	})
	require.ErrorIs(t, translated, ErrBadCharacter, "resolution's participant refusal, in this package's word")
	require.NotErrorIs(t, translated, resolution.ErrBadParticipant)
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
		{"the sides stopped being sides", encounter.ByStance(), DissolveByStance},
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
