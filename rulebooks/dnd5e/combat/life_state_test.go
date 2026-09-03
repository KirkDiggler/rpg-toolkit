// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

func TestClassifyLifeStateProvidesCompleteParticipation(t *testing.T) {
	tests := []struct {
		name  string
		input combat.LifeStateInput
		want  combat.Participation
	}{
		{
			name:  "conscious character acts normally",
			input: combat.LifeStateInput{Kind: combat.CombatantKindCharacter},
			want: combat.Participation{
				State:             combat.LifeStateConscious,
				CanActNormally:    true,
				RetainsInitiative: true,
				AttackTarget:      true,
				Conscious:         true,
			},
		},
		{
			name:  "dying character keeps a player turn",
			input: combat.LifeStateInput{Kind: combat.CombatantKindCharacter, Down: true},
			want: combat.Participation{
				State:             combat.LifeStateDying,
				Down:              true,
				NeedsDeathSave:    true,
				RetainsInitiative: true,
				AttackTarget:      true,
			},
		},
		{
			name: "stabilized character auto-passes in place",
			input: combat.LifeStateInput{
				Kind:       combat.CombatantKindCharacter,
				Down:       true,
				Stabilized: true,
			},
			want: combat.Participation{
				State:             combat.LifeStateStabilized,
				Down:              true,
				RetainsInitiative: true,
				AutoPassesTurn:    true,
				AttackTarget:      true,
			},
		},
		{
			name: "dead character is no longer a target or participant",
			input: combat.LifeStateInput{
				Kind: combat.CombatantKindCharacter,
				Down: true,
				Dead: true,
			},
			want: combat.Participation{State: combat.LifeStateDead, Down: true},
		},
		{
			name:  "conscious monster acts normally",
			input: combat.LifeStateInput{Kind: combat.CombatantKindMonster},
			want: combat.Participation{
				State:             combat.LifeStateConscious,
				CanActNormally:    true,
				RetainsInitiative: true,
				AttackTarget:      true,
				Conscious:         true,
			},
		},
		{
			name:  "down monster is defeated",
			input: combat.LifeStateInput{Kind: combat.CombatantKindMonster, Down: true},
			want:  combat.Participation{State: combat.LifeStateDefeated, Down: true},
		},
		{
			name:  "unknown input fails closed",
			input: combat.LifeStateInput{Kind: combat.CombatantKind("unknown")},
			want:  combat.Participation{State: combat.LifeStateUnknown},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := combat.ClassifyLifeState(tt.input)
			require.Equal(t, tt.want.State, state)
			require.Equal(t, tt.want, combat.ParticipationFor(state))
		})
	}
}

func TestPartyDefeatedRequiresMembersAndNoConsciousCharacter(t *testing.T) {
	tests := []struct {
		name  string
		party combat.PartyState
		want  bool
	}{
		{
			name: "one conscious party member",
			party: combat.PartyState{Members: []combat.Participation{
				combat.ParticipationFor(combat.LifeStateConscious),
			}},
			want: false,
		},
		{
			name: "dying and stabilized only",
			party: combat.PartyState{Members: []combat.Participation{
				combat.ParticipationFor(combat.LifeStateDying),
				combat.ParticipationFor(combat.LifeStateStabilized),
			}},
			want: true,
		},
		{
			name:  "empty party",
			party: combat.PartyState{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, combat.PartyDefeated(tt.party))
		})
	}
}
