// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

const (
	// BlessedName is the display name for a creature affected by Bless.
	BlessedName = "Blessed"
	// BlessedContributionGroup makes equal-potency Bless effects non-stacking.
	BlessedContributionGroup = "bless"
)

// BlessedConditionData is the persisted source-qualified Bless condition.
type BlessedConditionData struct {
	Ref       *core.Ref `json:"ref"`
	MemberID  string    `json:"member_id"`
	SourceID  string    `json:"source_id"`
	SourceRef *core.Ref `json:"source_ref"`
}

// NewBlessedConditionInput names the recipient, caster, and canonical spell that
// created a Blessed condition.
type NewBlessedConditionInput struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref
}

// BlessedCondition describes Bless's unresolved additive d4. It stores no die
// face and subscribes to no roll event; the recipient selects it and the later
// roll path evaluates its description.
type BlessedCondition struct {
	MemberID  string
	SourceID  string
	SourceRef *core.Ref

	bus       events.EventBus
	restSubID string
}

var (
	_ dnd5eEvents.ConditionBehavior        = (*BlessedCondition)(nil)
	_ dnd5eEvents.ConditionAddressProvider = (*BlessedCondition)(nil)
	_ dnd5eEvents.RollContributionProvider = (*BlessedCondition)(nil)
)

// NewBlessedCondition creates one source-qualified Bless effect.
func NewBlessedCondition(input NewBlessedConditionInput) (*BlessedCondition, error) {
	if input.MemberID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "blessed condition requires a recipient member")
	}
	if input.SourceID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "blessed condition requires a caster source id")
	}
	if input.SourceRef == nil || input.SourceRef.String() != refs.Spells.Bless().String() {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "blessed condition source ref must be Bless")
	}

	return &BlessedCondition{
		MemberID:  input.MemberID,
		SourceID:  input.SourceID,
		SourceRef: refs.Spells.Bless(),
	}, nil
}

// Ref returns the canonical Blessed condition ref.
func (b *BlessedCondition) Ref() *core.Ref { return refs.Conditions.Blessed() }

// ConditionAddress derives this condition's exact identity from its persisted state.
func (b *BlessedCondition) ConditionAddress() dnd5eEvents.ConditionAddress {
	return dnd5eEvents.ConditionAddress{
		MemberID:     b.MemberID,
		ConditionRef: b.Ref().String(),
		SourceID:     b.SourceID,
	}
}

// IsApplied reports whether the condition has joined an interaction bus.
func (b *BlessedCondition) IsApplied() bool { return b.bus != nil }

// Apply marks the condition active and subscribes only to long-rest cleanup.
// Bless contributes through its recipient's describe operation and deliberately
// installs no roll subscriber.
func (b *BlessedCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}
	if b.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "blessed condition already applied")
	}
	b.bus = bus
	restSubID, err := subscribeRemoveOnLongRest(ctx, bus, subscribeRemoveOnLongRestInput{
		Address: b.ConditionAddress(), Remove: b.Remove,
	})
	if err != nil {
		b.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe blessed condition to long rest")
	}
	b.restSubID = restSubID
	return nil
}

// Remove marks the condition inactive and revokes its cleanup subscription.
func (b *BlessedCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if b.restSubID != "" {
		if err := bus.Unsubscribe(ctx, b.restSubID); err != nil {
			return rpgerr.Wrap(err, "failed to unsubscribe blessed condition")
		}
	}
	b.restSubID = ""
	b.bus = nil
	return nil
}

// ToJSON serializes the source-qualified condition without runtime state.
func (b *BlessedCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(BlessedConditionData{
		Ref:       refs.Conditions.Blessed(),
		MemberID:  b.MemberID,
		SourceID:  b.SourceID,
		SourceRef: refs.Spells.Bless(),
	})
}

func (b *BlessedCondition) loadJSON(data json.RawMessage) error {
	var stored BlessedConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal blessed data")
	}
	loaded, err := NewBlessedCondition(NewBlessedConditionInput{
		MemberID: stored.MemberID, SourceID: stored.SourceID, SourceRef: stored.SourceRef,
	})
	if err != nil {
		return err
	}
	*b = *loaded
	return nil
}

// RollContributionMetadata declares Bless applicable to attacks and saving
// throws in its own equal-potency stacking group.
func (b *BlessedCondition) RollContributionMetadata(
	input *dnd5eEvents.DescribeRollContributionsInput,
) dnd5eEvents.RollContributionMetadataOutput {
	applicable := input != nil && (input.Kind == dnd5eEvents.RollKindAttack ||
		input.Kind == dnd5eEvents.RollKindSavingThrow)
	return dnd5eEvents.RollContributionMetadataOutput{
		Applicable: applicable,
		Group:      BlessedContributionGroup,
	}
}

// DescribeRollContributions describes one unresolved additive d4 for an
// applicable roll. Selection and rolling belong to the recipient and roll path.
func (b *BlessedCondition) DescribeRollContributions(
	input *dnd5eEvents.DescribeRollContributionsInput,
) (*dnd5eEvents.DescribeRollContributionsOutput, error) {
	if !b.RollContributionMetadata(input).Applicable {
		return &dnd5eEvents.DescribeRollContributionsOutput{}, nil
	}
	return &dnd5eEvents.DescribeRollContributionsOutput{
		Contributions: []dnd5eEvents.DiceContribution{{
			Source: dnd5eEvents.RollSource{
				Ref: refs.Spells.Bless(), Name: "Bless", SourceID: b.SourceID,
			},
			Dice: "1d4",
		}},
	}, nil
}
