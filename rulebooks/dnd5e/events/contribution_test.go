// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func TestRollContributionVocabularyKeepsEntityIdentityOnItsSource(t *testing.T) {
	contribution := DiceContribution{
		Source: RollSource{
			Ref:      refs.Spells.Bane(),
			Name:     "Bane",
			SourceID: "bard-a",
		},
		Dice:     "1d4",
		Subtract: true,
	}

	require.Equal(t, "bard-a", contribution.Source.SourceID)
	require.Equal(t, "1d4", contribution.Dice)
	require.True(t, contribution.Subtract)
	require.Equal(t, RollKind("attack"), RollKindAttack)
	require.Equal(t, RollKind("saving_throw"), RollKindSavingThrow)
}

func TestConditionAddressIsExactlyMemberConditionAndSource(t *testing.T) {
	address := ConditionAddress{
		MemberID:     "target-1",
		ConditionRef: refs.Conditions.Baned().String(),
		SourceID:     "bard-a",
	}

	require.Equal(t, "target-1", address.MemberID)
	require.Equal(t, refs.Conditions.Baned().String(), address.ConditionRef)
	require.Equal(t, "bard-a", address.SourceID)
}

func TestCloneRollCalculationPreservesSourceEntityIdentity(t *testing.T) {
	modifier := 2
	original := &RollCalculation{
		Components: []RollComponent{{
			Source: RollSource{
				Ref: refs.Spells.Bane(), Name: "Bane", SourceID: "bard-a",
			},
			Modifier: &modifier,
		}},
		Total: 2,
	}

	clone := CloneRollCalculation(original)
	require.Equal(t, "bard-a", clone.Components[0].Source.SourceID)
}
