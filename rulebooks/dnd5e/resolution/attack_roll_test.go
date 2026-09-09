// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAttackRollIsReportedBeforeAnyPostRollChoice(t *testing.T) {
	for _, tc := range []struct {
		name     string
		inspired bool
		roll     int
		total    int
	}{
		{name: "ordinary completed attack", roll: 15, total: 19},
		{name: "posed inspiration attack", inspired: true, roll: 8, total: 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hero := actionHero()
			if tc.inspired {
				hero = inspiredHero(t)
			}
			out, err := heroSwings(t, hero, &actionRoller{
				singles: []int{tc.roll}, damage: [][]int{{3}},
			})
			require.NoError(t, err)
			require.Equal(t, &AttackRoll{
				AttackerID: heroID, TargetID: wolfID, Roll: tc.roll, Total: tc.total,
			}, out.AttackRoll)
			if tc.inspired {
				require.NotNil(t, out.Posed)
				require.Nil(t, out.Outcome, "the early roll is not a final result")
			} else {
				require.Nil(t, out.Posed)
				require.NotNil(t, out.Outcome)
			}
		})
	}
}

func TestAttackRollIsNotReportedAgainWhenTheChoiceResumes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		answer    OfferAnswer
		wantTotal int
	}{
		{name: "keep", answer: OfferKeep, wantTotal: 12},
		{name: "spend", answer: OfferSpend, wantTotal: 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posed, err := heroSwings(t, inspiredHero(t), &actionRoller{singles: []int{8}})
			require.NoError(t, err)
			require.NotNil(t, posed.Posed)
			machine, err := NewStrikeResumed(&StrikeResumeInput{
				Frozen: posed.Posed.Frozen, Answer: tc.answer,
				Roller: &actionRoller{singles: []int{4}, damage: [][]int{{3}}},
			})
			require.NoError(t, err)
			resumed, err := resolveHeroStrike(t, inspiredHero(t), machine)
			require.NoError(t, err)
			require.Nil(t, resumed.AttackRoll, "answering the offer is not another d20")
			outcome := resumed.Outcome.(StrikeOutcome)
			require.Equal(t, 8, outcome.Roll)
			require.Equal(t, tc.wantTotal, outcome.Total)
			require.Equal(t, &AttackRoll{
				AttackerID: heroID, TargetID: wolfID, Roll: 8, Total: 12,
			}, posed.AttackRoll, "the original snapshot does not become the final total")
		})
	}
}

func TestAttackRollFailureReturnsNoPartialOutput(t *testing.T) {
	out, err := heroSwings(t, inspiredHero(t), &actionRoller{})
	require.ErrorContains(t, err, "roll attack: no scripted single")
	require.Nil(t, out, "a failed d20 must not expose a zero-valued roll or a partial pose")
}

func TestAttackRollSnapshotDoesNotAliasMachineState(t *testing.T) {
	machine := strikeFor(t, &actionRoller{singles: []int{8}})
	out, err := resolveHeroStrike(t, inspiredHero(t), machine)
	require.NoError(t, err)
	require.NotNil(t, out.AttackRoll)
	out.AttackRoll.Roll = 20
	out.AttackRoll.Total = 99
	require.Equal(t, &AttackRoll{
		AttackerID: heroID, TargetID: wolfID, Roll: 8, Total: 12,
	}, reportedAttackRoll(machine), "a caller can edit its copy, not the machine's capture")
}
