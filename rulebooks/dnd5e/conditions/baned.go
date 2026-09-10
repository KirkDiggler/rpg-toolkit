// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

const (
	// BanedName is the display name for a creature affected by Bane.
	BanedName = "Baned"
	// BanedContributionGroup makes equal-potency Bane effects non-stacking.
	BanedContributionGroup = "bane"
)

// BanedConditionData is the persisted source-qualified Bane condition.
type BanedConditionData struct {
	Ref       *core.Ref `json:"ref"`
	MemberID  string    `json:"member_id"`
	SourceID  string    `json:"source_id"`
	SourceRef *core.Ref `json:"source_ref"`
}

// NewBanedConditionInput names the recipient, caster, and canonical spell that
// created a Baned condition.
type NewBanedConditionInput struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref
}

// BanedCondition describes Bane's unresolved subtractive d4. It stores no die
// face and subscribes to no roll event; the recipient selects it and the later
// roll path evaluates its description.
type BanedCondition struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref

	bus       events.EventBus
	restSubID string
}

var (
	_ dnd5eEvents.ConditionBehavior        = (*BanedCondition)(nil)
	_ dnd5eEvents.ConditionAddressProvider = (*BanedCondition)(nil)
	_ dnd5eEvents.RollContributionProvider = (*BanedCondition)(nil)
)

// NewBanedCondition creates one source-qualified Bane effect.
func NewBanedCondition(input NewBanedConditionInput) (*BanedCondition, error) {
	if input.MemberID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "baned condition requires a recipient member")
	}
	if input.SourceID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "baned condition requires a caster source id")
	}
	if input.SourceRef == nil || input.SourceRef.String() != refs.Spells.Bane().String() {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "baned condition source ref must be Bane")
	}

	return &BanedCondition{
		MemberID:  input.MemberID,
		SourceID:  input.SourceID,
		SourceRef: refs.Spells.Bane(),
	}, nil
}

// Ref returns the canonical Baned condition ref.
func (b *BanedCondition) Ref() *core.Ref { return refs.Conditions.Baned() }

// ConditionAddress derives this condition's exact identity from its persisted state.
func (b *BanedCondition) ConditionAddress() dnd5eEvents.ConditionAddress {
	return dnd5eEvents.ConditionAddress{
		MemberID:     b.MemberID,
		ConditionRef: b.Ref().String(),
		SourceID:     b.SourceID,
	}
}

// IsApplied reports whether the condition has joined an interaction bus.
func (b *BanedCondition) IsApplied() bool { return b.bus != nil }

// Apply marks the condition active and subscribes only to long-rest cleanup.
// Bane contributes through its recipient's describe operation and deliberately
// installs no roll subscriber.
func (b *BanedCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}
	if b.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "baned condition already applied")
	}
	b.bus = bus
	restSubID, err := subscribeRemoveOnLongRest(ctx, bus, subscribeRemoveOnLongRestInput{
		Address: b.ConditionAddress(), Remove: b.Remove,
	})
	if err != nil {
		b.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe baned condition to long rest")
	}
	b.restSubID = restSubID
	return nil
}

// Remove marks the condition inactive and revokes its cleanup subscription.
func (b *BanedCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if b.restSubID != "" {
		if err := bus.Unsubscribe(ctx, b.restSubID); err != nil {
			return rpgerr.Wrap(err, "failed to unsubscribe baned condition")
		}
	}
	b.restSubID = ""
	b.bus = nil
	return nil
}

// ToJSON serializes the source-qualified condition without runtime state.
func (b *BanedCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(BanedConditionData{
		Ref:       refs.Conditions.Baned(),
		MemberID:  b.MemberID,
		SourceID:  b.SourceID,
		SourceRef: refs.Spells.Bane(),
	})
}

func (b *BanedCondition) loadJSON(data json.RawMessage) error {
	var stored BanedConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal baned data")
	}
	loaded, err := NewBanedCondition(NewBanedConditionInput{
		MemberID: stored.MemberID, SourceID: stored.SourceID, SourceRef: stored.SourceRef,
	})
	if err != nil {
		return err
	}
	*b = *loaded
	return nil
}

// RollContributionMetadata declares Bane applicable to attacks and saving
// throws in its own equal-potency stacking group.
func (b *BanedCondition) RollContributionMetadata(
	input *dnd5eEvents.DescribeRollContributionsInput,
) dnd5eEvents.RollContributionMetadataOutput {
	applicable := input != nil && (input.Kind == dnd5eEvents.RollKindAttack ||
		input.Kind == dnd5eEvents.RollKindSavingThrow)
	return dnd5eEvents.RollContributionMetadataOutput{
		Applicable: applicable,
		Group:      BanedContributionGroup,
	}
}

// DescribeRollContributions describes one unresolved subtractive d4 for an
// applicable roll. Selection and rolling belong to the recipient and roll path.
func (b *BanedCondition) DescribeRollContributions(
	input *dnd5eEvents.DescribeRollContributionsInput,
) (*dnd5eEvents.DescribeRollContributionsOutput, error) {
	if !b.RollContributionMetadata(input).Applicable {
		return &dnd5eEvents.DescribeRollContributionsOutput{}, nil
	}
	return &dnd5eEvents.DescribeRollContributionsOutput{
		Contributions: []dnd5eEvents.DiceContribution{{
			Source: dnd5eEvents.RollSource{
				Ref: refs.Spells.Bane(), Name: "Bane", SourceID: b.SourceID,
			},
			Dice: "1d4", Subtract: true,
		}},
	}, nil
}

// DescribeSelectedRollContributionsInput supplies a recipient's current
// persisted-order condition snapshot and the roll-kind request.
type DescribeSelectedRollContributionsInput struct {
	Conditions []dnd5eEvents.ConditionBehavior
	Request    *dnd5eEvents.DescribeRollContributionsInput
}

// DescribeSelectedRollContributions selects the oldest applicable provider in
// each provider-declared group, then asks only those providers to describe.
func DescribeSelectedRollContributions(
	input *DescribeSelectedRollContributionsInput,
) (*dnd5eEvents.DescribeRollContributionsOutput, error) {
	if input == nil || input.Request == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "roll contribution request is required")
	}

	output := &dnd5eEvents.DescribeRollContributionsOutput{}
	selectedGroups := make(map[string]struct{})
	for _, condition := range input.Conditions {
		provider, ok := condition.(dnd5eEvents.RollContributionProvider)
		if !ok {
			continue
		}
		metadata := provider.RollContributionMetadata(input.Request)
		if !metadata.Applicable {
			continue
		}
		if metadata.Group == "" {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"roll contribution provider %T declared an empty stacking group", condition)
		}
		if _, selected := selectedGroups[metadata.Group]; selected {
			continue
		}

		described, err := provider.DescribeRollContributions(input.Request)
		if err != nil {
			return nil, fmt.Errorf("describe roll contributions from %T: %w", condition, err)
		}
		if described == nil {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"roll contribution provider %T returned nil output", condition)
		}
		selectedGroups[metadata.Group] = struct{}{}
		output.Contributions = append(output.Contributions, described.Contributions...)
	}
	return output, nil
}

// ConditionAddressOf derives a condition's exact identity on memberID's sheet.
// Legacy conditions without the optional capability retain exact empty source identity.
func ConditionAddressOf(
	memberID string, condition dnd5eEvents.ConditionBehavior,
) dnd5eEvents.ConditionAddress {
	if addressed, ok := condition.(dnd5eEvents.ConditionAddressProvider); ok {
		return addressed.ConditionAddress()
	}
	return dnd5eEvents.ConditionAddress{
		MemberID: memberID, ConditionRef: condition.Ref().String(),
	}
}
