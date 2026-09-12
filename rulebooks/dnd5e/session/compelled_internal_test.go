// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// compelled_internal_test.go pins the two translations the verb cannot show
// from outside: an obeyed word's effects becoming one intent, and a host
// driver's own Routed crossing the seam.
//
// The refusals are the half worth having here. A compelled turn that produced
// two moves, or a policy the composition cannot walk, is a wiring fault no
// content can express — so the door is the only place it can be observed.

// imposedMove is an ImposedMove effect carrying the directive Obey produces
// for a walking word.
func imposedMove(policy combatActions.MovePolicy, anchor string) resolution.ImposedEffect {
	return resolution.ImposedEffect{
		Kind:        resolution.ImposedMove,
		RecipientID: "skeleton",
		Move: &resolution.MoveDirective{
			Policy: policy, AnchorID: anchor, Turn: true, Provokes: true,
		},
	}
}

func TestOneImposedMoveBecomesARoutedTurnNamingTheCompulsion(t *testing.T) {
	intent, err := compelledIntent("skeleton", []resolution.ImposedEffect{
		imposedMove(combatActions.MoveToward, "bard"),
	})
	require.NoError(t, err)
	require.Equal(t, encounter.Routed{
		Policy: encounter.MoveToward,
		Anchor: "bard",
		Cause:  *refs.Conditions.Commanded(),
	}, intent, "the anchor and the policy come from the word; the cause is what gave it")

	away, err := compelledIntent("skeleton", []resolution.ImposedEffect{
		imposedMove(combatActions.MoveAway, "bard"),
	})
	require.NoError(t, err)
	require.Equal(t, encounter.MoveAway, away.(encounter.Routed).Policy,
		"a translation that hardcoded one direction would pass the test above alone")
}

// TestAWordThatImposesNoMoveEndsTheTurn is Grovel, and every later word whose
// whole turn is what it left behind: the condition is already applied, so the
// turn has nothing further to spend.
func TestAWordThatImposesNoMoveEndsTheTurn(t *testing.T) {
	intent, err := compelledIntent("skeleton", []resolution.ImposedEffect{{
		Kind: resolution.ImposedCondition, RecipientID: "skeleton",
		Ref: refs.Conditions.Prone(),
	}})
	require.NoError(t, err)
	require.Equal(t, encounter.Pass{}, intent)

	empty, err := compelledIntent("skeleton", nil)
	require.NoError(t, err)
	require.Equal(t, encounter.Pass{}, empty,
		"and a word that imposed nothing at all still spends the turn")
}

// TestAnEffectKindThisSeamWasNeverTaughtIsRefused is the one the doc promised
// and the code did not do until a reviewer probed it: an unknown kind used to
// be skipped, so the turn ended as though nothing had been asked of the
// creature and nothing said a translation had been dropped.
//
// The three kinds that ARE known must keep passing through, which is why the
// fix is an allow-list and not a bare default — Grovel rides on two of them.
func TestAnEffectKindThisSeamWasNeverTaughtIsRefused(t *testing.T) {
	intent, err := compelledIntent("skeleton", []resolution.ImposedEffect{
		{Kind: "teleport", RecipientID: "skeleton"},
	})
	require.ErrorIs(t, err, ErrBadTurnOutcome)
	require.Nil(t, intent, "and no turn is invented for it")
	require.Contains(t, err.Error(), "teleport", "the refusal names what it could not translate")

	for _, kind := range []resolution.ImposedEffectKind{
		resolution.ImposedCondition,
		resolution.ImposedConditionRemoved,
		resolution.ImposedDamage,
	} {
		t.Run(string(kind), func(t *testing.T) {
			spent, spentErr := compelledIntent("skeleton", []resolution.ImposedEffect{
				{Kind: kind, RecipientID: "skeleton"},
			})
			require.NoError(t, spentErr,
				"resolution already applied this one; the turn has nothing to carry out")
			require.Equal(t, encounter.Pass{}, spent)
		})
	}

	// And an unknown kind is refused even when a walk arrived beside it, so a
	// word that grew a second consequence cannot hide it behind its move.
	beside, err := compelledIntent("skeleton", []resolution.ImposedEffect{
		imposedMove(combatActions.MoveToward, "bard"),
		{Kind: "teleport", RecipientID: "skeleton"},
	})
	require.ErrorIs(t, err, ErrBadTurnOutcome)
	require.Nil(t, beside)
}

// TestTheLookupSkipsABlobItCannotReadRatherThanStoppingAtIt is the driver's
// own half of the rule participation keeps.
//
// It is a UNIT test on purpose. The reviewer showed the scenario cannot be
// built through the verbs — resolution's attach refuses a record carrying an
// unreadable condition one seam earlier — so this is the only place the
// defence can be observed at all, and the doc on commandedIn now says that is
// what it is.
//
// The ORDER is the assertion: the corrupt blob goes first, which is exactly
// the arrangement that makes a whole-list decode answer "nothing here".
func TestTheLookupSkipsABlobItCannotReadRatherThanStoppingAtIt(t *testing.T) {
	real, err := conditions.NewCommandedCondition(
		"skeleton", refs.Spells.Command().String(), "bard", "flee", 1,
	)
	require.NoError(t, err)
	raw, err := real.ToJSON()
	require.NoError(t, err)

	data, held := commandedIn([]json.RawMessage{json.RawMessage(`{"ref":`), raw})
	require.True(t, held, "a corrupt blob in front of a compulsion must not hide it")
	require.Equal(t, "flee", data.Word)
	require.Equal(t, "bard", data.CasterID)

	_, none := commandedIn([]json.RawMessage{json.RawMessage(`{"ref":`)})
	require.False(t, none, "and a sheet with nothing readable on it holds no compulsion")
}

// TestATranslationThisSeamCannotMakeIsRefused — each of these is a wiring
// fault rather than anything content can express, and a turn that quietly
// passed instead would be indistinguishable from a creature that was never
// compelled.
func TestATranslationThisSeamCannotMakeIsRefused(t *testing.T) {
	cases := map[string][]resolution.ImposedEffect{
		"two moves is two answers to where the turn goes": {
			imposedMove(combatActions.MoveToward, "bard"),
			imposedMove(combatActions.MoveAway, "bard"),
		},
		"an imposed move carrying no directive": {{
			Kind: resolution.ImposedMove, RecipientID: "skeleton",
		}},
		"a policy this composition cannot walk": {{
			Kind: resolution.ImposedMove, RecipientID: "skeleton",
			Move: &resolution.MoveDirective{Policy: "sideways", AnchorID: "bard", Turn: true},
		}},
	}

	for name, effects := range cases {
		t.Run(name, func(t *testing.T) {
			intent, err := compelledIntent("skeleton", effects)
			require.ErrorIs(t, err, ErrBadTurnOutcome)
			require.Nil(t, intent, "a refusal invents no turn")
			require.Contains(t, err.Error(), `"skeleton"`, "and names whose turn it was")
		})
	}
}

// TestAHostsOwnRoutedCrossesTheSeam is the other half of the intent: the
// compelled driver is not the only customer. A monster that decides to run
// hands back a Routed of its own, and this is the translation that carries it.
func TestAHostsOwnRoutedCrossesTheSeam(t *testing.T) {
	intent, err := routedToEncounter("skeleton", Routed{
		Policy: MoveAway, Anchor: "fighter",
		Cause: refs.Conditions.Commanded().String(),
	})
	require.NoError(t, err)
	require.Equal(t, encounter.Routed{
		Policy: encounter.MoveAway,
		Anchor: "fighter",
		Cause:  *refs.Conditions.Commanded(),
	}, intent)
}

// TestARoutedCauseThatWillNotParseIsRefused — refused HERE rather than passed
// on empty, because the composition's own refusal would name a missing cause
// while the truth is a malformed one, and a driver author reading "no cause"
// beside the cause they wrote has been told the wrong thing.
func TestARoutedCauseThatWillNotParseIsRefused(t *testing.T) {
	for name, cause := range map[string]string{
		"empty":     "",
		"not a ref": "commanded",
	} {
		t.Run(name, func(t *testing.T) {
			intent, err := routedToEncounter("skeleton", Routed{
				Policy: MoveToward, Anchor: "bard", Cause: cause,
			})
			require.ErrorIs(t, err, ErrBadTurnOutcome)
			require.Nil(t, intent)
		})
	}
}
