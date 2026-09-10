// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func TestBanedConditionRoundTripAndRegistration(t *testing.T) {
	condition, err := NewBanedCondition(NewBanedConditionInput{
		MemberID:  "target-1",
		SourceID:  "bard-a",
		SourceRef: refs.Spells.Bane(),
	})
	require.NoError(t, err)
	require.Equal(t, refs.Conditions.Baned(), condition.Ref())
	require.Equal(t, dnd5eEvents.ConditionAddress{
		MemberID:     "target-1",
		ConditionRef: refs.Conditions.Baned().String(),
		SourceID:     "bard-a",
	}, condition.ConditionAddress())

	raw, err := condition.ToJSON()
	require.NoError(t, err)
	require.NotContains(t, string(raw), "face", "a description carries notation, never a rolled face")

	loaded, err := LoadJSON(raw)
	require.NoError(t, err)
	back, ok := loaded.(*BanedCondition)
	require.True(t, ok)
	require.Equal(t, condition.ConditionAddress(), back.ConditionAddress())
	require.Equal(t, refs.Spells.Bane(), back.SourceRef)

	display, ok := DisplayFor(*refs.Conditions.Baned())
	require.True(t, ok)
	require.Equal(t, BanedName, display.Name)
}

func TestBanedConditionRequiresQualifiedCanonicalBaneSource(t *testing.T) {
	tests := map[string]NewBanedConditionInput{
		"recipient":  {SourceID: "bard-a", SourceRef: refs.Spells.Bane()},
		"source id":  {MemberID: "target-1", SourceRef: refs.Spells.Bane()},
		"source ref": {MemberID: "target-1", SourceID: "bard-a"},
		"bane ref": {
			MemberID: "target-1", SourceID: "bard-a", SourceRef: refs.Spells.TrueStrike(),
		},
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewBanedCondition(input)
			require.Error(t, err)
		})
	}
}

func TestBanedConditionDescribesUnresolvedAttackAndSavePenalty(t *testing.T) {
	condition, err := NewBanedCondition(NewBanedConditionInput{
		MemberID: "target-1", SourceID: "bard-a", SourceRef: refs.Spells.Bane(),
	})
	require.NoError(t, err)

	for _, kind := range []dnd5eEvents.RollKind{
		dnd5eEvents.RollKindAttack,
		dnd5eEvents.RollKindSavingThrow,
	} {
		input := &dnd5eEvents.DescribeRollContributionsInput{Kind: kind}
		metadata := condition.RollContributionMetadata(input)
		require.True(t, metadata.Applicable)
		require.Equal(t, BanedContributionGroup, metadata.Group)

		output, err := condition.DescribeRollContributions(input)
		require.NoError(t, err)
		require.Equal(t, []dnd5eEvents.DiceContribution{{
			Source: dnd5eEvents.RollSource{
				Ref: refs.Spells.Bane(), Name: "Bane", SourceID: "bard-a",
			},
			Dice: "1d4", Subtract: true,
		}}, output.Contributions)
	}

	metadata := condition.RollContributionMetadata(
		&dnd5eEvents.DescribeRollContributionsInput{Kind: dnd5eEvents.RollKind("damage")})
	require.False(t, metadata.Applicable)
}

func TestDescribeSelectedRollContributionsKeepsOldestPerConditionGroup(t *testing.T) {
	baneA, err := NewBanedCondition(NewBanedConditionInput{
		MemberID: "target-1", SourceID: "bard-a", SourceRef: refs.Spells.Bane(),
	})
	require.NoError(t, err)
	baneB, err := NewBanedCondition(NewBanedConditionInput{
		MemberID: "target-1", SourceID: "bard-b", SourceRef: refs.Spells.Bane(),
	})
	require.NoError(t, err)
	blessing := &syntheticContributionCondition{memberID: "target-1"}

	output, err := DescribeSelectedRollContributions(&DescribeSelectedRollContributionsInput{
		Conditions: []dnd5eEvents.ConditionBehavior{baneA, baneB, blessing},
		Request:    &dnd5eEvents.DescribeRollContributionsInput{Kind: dnd5eEvents.RollKindAttack},
	})
	require.NoError(t, err)
	require.Equal(t, []dnd5eEvents.DiceContribution{
		{
			Source: dnd5eEvents.RollSource{
				Ref: refs.Spells.Bane(), Name: "Bane", SourceID: "bard-a",
			},
			Dice: "1d4", Subtract: true,
		},
		{
			Source: dnd5eEvents.RollSource{
				Ref: syntheticContributionRef, Name: "Blessing", SourceID: "cleric-a",
			},
			Dice: "1d6",
		},
	}, output.Contributions)

	output, err = DescribeSelectedRollContributions(&DescribeSelectedRollContributionsInput{
		Conditions: []dnd5eEvents.ConditionBehavior{baneB, blessing},
		Request:    &dnd5eEvents.DescribeRollContributionsInput{Kind: dnd5eEvents.RollKindAttack},
	})
	require.NoError(t, err)
	require.Equal(t, "bard-b", output.Contributions[0].Source.SourceID,
		"removing the oldest hands the group to the next persisted condition")
}

var syntheticContributionRef = &core.Ref{Module: "test", Type: "conditions", ID: "blessing"}

type syntheticContributionCondition struct {
	memberID string
	applied  bool
}

func (s *syntheticContributionCondition) Ref() *core.Ref  { return syntheticContributionRef }
func (s *syntheticContributionCondition) IsApplied() bool { return s.applied }
func (s *syntheticContributionCondition) Apply(_ context.Context, _ events.EventBus) error {
	s.applied = true
	return nil
}
func (s *syntheticContributionCondition) Remove(_ context.Context, _ events.EventBus) error {
	s.applied = false
	return nil
}
func (s *syntheticContributionCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(struct {
		Ref *core.Ref `json:"ref"`
	}{Ref: syntheticContributionRef})
}
func (s *syntheticContributionCondition) RollContributionMetadata(
	input *dnd5eEvents.DescribeRollContributionsInput,
) dnd5eEvents.RollContributionMetadataOutput {
	return dnd5eEvents.RollContributionMetadataOutput{
		Applicable: input != nil && input.Kind == dnd5eEvents.RollKindAttack,
		Group:      "synthetic-blessing",
	}
}
func (s *syntheticContributionCondition) DescribeRollContributions(
	_ *dnd5eEvents.DescribeRollContributionsInput,
) (*dnd5eEvents.DescribeRollContributionsOutput, error) {
	return &dnd5eEvents.DescribeRollContributionsOutput{Contributions: []dnd5eEvents.DiceContribution{{
		Source: dnd5eEvents.RollSource{
			Ref: syntheticContributionRef, Name: "Blessing", SourceID: "cleric-a",
		},
		Dice: "1d6",
	}}}, nil
}
