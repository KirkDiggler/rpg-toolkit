// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// sightFrame is an execution frame holding only the a↔b sight facts.
func sightFrame(aSeesB, bSeesA contributions.Fact[bool]) contributions.Frame {
	return contributions.Frame{
		Actor: "a",
		Pairs: []contributions.PairFacts{
			{From: "a", To: "b", Sees: aSeesB},
			{From: "b", To: "a", Sees: bSeesA},
		},
	}
}

// The sight rule reads both directions off the frame: an unseen target is
// disadvantage, an unseen attacker advantage, and both cancel by landing.
func TestSightAttackModifiersReadTheFrame(t *testing.T) {
	seen, unseen := contributions.Known(true), contributions.Known(false)
	for _, tc := range []struct {
		name                string
		aSeesB, bSeesA      contributions.Fact[bool]
		advantage, disadvan bool
	}{
		{"clear", seen, seen, false, false},
		{"unseen target", unseen, seen, false, true},
		{"unseen attacker", seen, unseen, true, false},
		{"both obscured", unseen, unseen, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := dndEvents.AttackChainEvent{}
			require.NoError(t, applySightAttackModifiers(sightFrame(tc.aSeesB, tc.bSeesA), &e, "a", "b", refs.Spells.GuidingBolt()))
			require.Equal(t, tc.advantage, hasReason(e.AdvantageSources, "target cannot see attacker"))
			require.Equal(t, tc.disadvan, hasReason(e.DisadvantageSources, "attacker cannot see target"))
		})
	}
}

// At execution an undecided sight in either direction fails the strike (R13);
// it is never read as seen.
func TestSightAttackModifiersRefuseAnUnknownDirection(t *testing.T) {
	unknown, seen := contributions.Unknown[bool](), contributions.Known(true)
	for name, frame := range map[string]contributions.Frame{
		"attacker to target": sightFrame(unknown, seen),
		"target to attacker": sightFrame(seen, unknown),
	} {
		t.Run(name, func(t *testing.T) {
			e := dndEvents.AttackChainEvent{}
			err := applySightAttackModifiers(frame, &e, "a", "b", refs.Spells.GuidingBolt())
			require.ErrorIs(t, err, contributions.ErrRuleCannotAnswer)
			require.Empty(t, e.AdvantageSources)
			require.Empty(t, e.DisadvantageSources)
		})
	}
}

func hasReason(sources []dndEvents.AttackModifierSource, reason string) bool {
	for _, source := range sources {
		if source.Reason == reason {
			return true
		}
	}
	return false
}
