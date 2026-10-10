// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// TestASequenceWhoseStoryNamesAnotherDefinitionIsRefused: the sequence marker
// is the story's own definition, so a story for another action would label
// every swing with the wrong sequence. The refusal comes before any unit is
// built, so nothing is recorded: the encounter is nil here, and reaching it
// would panic.
func TestASequenceWhoseStoryNamesAnotherDefinitionIsRefused(t *testing.T) {
	story := windowStory{
		Kind: storyAttack, Attacker: "boss", Target: "bob",
		Definition: combatActions.Definition{Ref: *refs.Weapons.Scimitar(), Name: "Scimitar"},
		// The component IS carried, so only the definition check can refuse.
		Components: []combatActions.Definition{{Ref: *refs.Weapons.Scimitar(), Name: "Scimitar"}},
	}
	sequence := resolution.SequenceOutcome{
		Action: *refs.MonsterActions.GoblinBossMultiattack(),
		Steps:  []resolution.SequenceStepOutcome{{Action: *refs.Weapons.Scimitar(), Strike: resolution.StrikeOutcome{}}},
	}

	_, err := (&Manager{}).recordAttack(nil, story, sequence, concentration{})
	require.ErrorIs(t, err, ErrBadAttack)
}

// TestAContinuedStrikeWithNothingToCarryItsConcentrationIsRefused: concentration
// is told behind a unit's beat. A continued strike with no retaliation builds
// no unit, so a save it forced has nowhere to ride, and dropping it would lose
// the roll.
func TestAContinuedStrikeWithNothingToCarryItsConcentrationIsRefused(t *testing.T) {
	story := windowStory{
		Kind: storyAttack, Attacker: "alice", Target: "bob",
		Definition: combatActions.Definition{Ref: *refs.Weapons.Longsword(), Name: "Longsword"},
	}
	told := concentration{Checks: []encounter.ConcentrationCheck{{}}}

	_, err := (&Manager{}).recordAttack(nil, story, resolution.StrikeOutcome{Continued: true}, told)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidWorld))

	got, err := (&Manager{}).recordAttack(nil, story, resolution.StrikeOutcome{Continued: true}, concentration{})
	require.NoError(t, err, "and with nothing to tell, nothing is recorded")
	require.Nil(t, got)
}
