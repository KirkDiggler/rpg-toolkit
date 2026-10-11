// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// UnarmoredDefenseType distinguishes between class variants of Unarmored Defense
type UnarmoredDefenseType string

const (
	// UnarmoredDefenseBarbarian uses CON modifier (10 + DEX + CON)
	UnarmoredDefenseBarbarian UnarmoredDefenseType = "barbarian"
	// UnarmoredDefenseMonk uses WIS modifier (10 + DEX + WIS)
	UnarmoredDefenseMonk UnarmoredDefenseType = "monk"
)

// UnarmoredDefenseData is the JSON structure for persisting unarmored defense condition state
type UnarmoredDefenseData struct {
	Ref      *core.Ref `json:"ref"`
	Type     string    `json:"type"`      // "barbarian" or "monk"
	MemberID string    `json:"member_id"` // ID of the character
	// Source is a ref string in "module:type:value" format (e.g., "dnd5e:classes:barbarian")
	Source string `json:"source"`
}

// UnarmoredDefenseCondition represents the Unarmored Defense feature.
// Barbarian: AC = 10 + DEX modifier + CON modifier
// Monk: AC = 10 + DEX modifier + WIS modifier
// Only applies when not wearing armor. A barbarian can still carry a shield; a
// monk holding one loses Unarmored Defense (RAW 2014: the monk's version
// requires no armor and no shield).
type UnarmoredDefenseCondition struct {
	MemberID        string
	Type            UnarmoredDefenseType
	Source          string // Ref string in "module:type:value" format (e.g., "dnd5e:classes:barbarian")
	subscriptionIDs []string
	bus             events.EventBus
}

// Ensure UnarmoredDefenseCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*UnarmoredDefenseCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (u *UnarmoredDefenseCondition) Ref() *core.Ref { return refs.Conditions.UnarmoredDefense() }

// UnarmoredDefenseInput provides configuration for creating an unarmored defense condition
type UnarmoredDefenseInput struct {
	MemberID string               // ID of the character
	Type     UnarmoredDefenseType // Barbarian (CON) or Monk (WIS)
	Source   string               // Ref string in "module:type:value" format (e.g., "dnd5e:classes:barbarian")
}

// NewUnarmoredDefenseCondition creates an unarmored defense condition from input
func NewUnarmoredDefenseCondition(input UnarmoredDefenseInput) *UnarmoredDefenseCondition {
	return &UnarmoredDefenseCondition{
		MemberID: input.MemberID,
		Type:     input.Type,
		Source:   input.Source,
	}
}

// IsApplied returns true if this condition is currently applied
func (u *UnarmoredDefenseCondition) IsApplied() bool {
	return u.bus != nil
}

// Apply subscribes this condition to AC chain events.
func (u *UnarmoredDefenseCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if u.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "unarmored defense already applied")
	}
	u.bus = bus

	// Subscribe to ACChain to add secondary ability modifier when unarmored
	acChain := combat.ACChain.On(bus)
	subID, err := acChain.SubscribeWithChain(ctx, u.onACChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to AC chain")
	}
	u.subscriptionIDs = append(u.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from events.
func (u *UnarmoredDefenseCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if u.bus == nil {
		return nil
	}

	total := len(u.subscriptionIDs)
	var errs []error
	for _, subID := range u.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	u.subscriptionIDs = nil
	u.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (u *UnarmoredDefenseCondition) ToJSON() (json.RawMessage, error) {
	data := UnarmoredDefenseData{
		Ref:      refs.Conditions.UnarmoredDefense(),
		Type:     string(u.Type),
		MemberID: u.MemberID,
		Source:   u.Source,
	}
	return json.Marshal(data)
}

// loadJSON loads unarmored defense condition state from JSON
func (u *UnarmoredDefenseCondition) loadJSON(data json.RawMessage) error {
	var udData UnarmoredDefenseData
	if err := json.Unmarshal(data, &udData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal unarmored defense data")
	}

	// A missing or unknown variant is refused, never read as barbarian: the
	// law the create path states holds for the stored blob too.
	switch UnarmoredDefenseType(udData.Type) {
	case UnarmoredDefenseBarbarian, UnarmoredDefenseMonk:
	default:
		return rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"stored unarmored defense names no variant of \"barbarian\" or \"monk\": got %q", udData.Type)
	}

	u.MemberID = udData.MemberID
	u.Type = UnarmoredDefenseType(udData.Type)
	u.Source = udData.Source

	return nil
}

// SecondaryAbility returns the secondary ability used for this type of
// unarmored defense. A condition built or loaded with a variant other than the
// two it names has no ability to answer, and says so rather than guessing one.
func (u *UnarmoredDefenseCondition) SecondaryAbility() (abilities.Ability, error) {
	switch u.Type {
	case UnarmoredDefenseBarbarian:
		return abilities.CON, nil
	case UnarmoredDefenseMonk:
		return abilities.WIS, nil
	}
	return "", rpgerr.Newf(rpgerr.CodeInvalidArgument,
		"unarmored defense variant %q has no secondary ability", u.Type)
}

// onACChain adds the secondary ability modifier to AC when unarmored.
func (u *UnarmoredDefenseCondition) onACChain(
	ctx context.Context,
	event *combat.ACChainEvent,
	c chain.Chain[*combat.ACChainEvent],
) (chain.Chain[*combat.ACChainEvent], error) {
	// Only modify AC for this character
	if event.CharacterID != u.MemberID {
		return c, nil
	}

	// Only apply when NOT wearing armor.
	if event.HasArmor {
		return c, nil
	}

	// A monk holding a shield loses the feature; a barbarian keeps it.
	if u.Type == UnarmoredDefenseMonk && event.HasShield {
		return c, nil
	}

	// Own sheet, looked up in the cast by this character's own ID — the same
	// call any other participant's sheet would come through.
	//
	// A cast that cannot name this character REFUSES the fold. This
	// contribution is part of the number, so leaving it out would answer base
	// armour for a character who has Unarmored Defense — the wrong AC that
	// rpg-api saved for monks and barbarians (rpg-toolkit#1965). See [member].
	me, ok := member(ctx, u.MemberID)
	if !ok {
		return c, rpgerr.Wrapf(gamectx.ErrNotInCast,
			"unarmored defense cannot read character %s's ability scores", u.MemberID)
	}

	// The secondary ability modifier (WIS for Monk, CON for Barbarian)
	secondary, err := u.SecondaryAbility()
	if err != nil {
		return c, err
	}
	secondaryMod := me.AbilityScores().Modifier(secondary)

	// Add secondary ability modifier at StageFeatures
	modifyAC := func(_ context.Context, e *combat.ACChainEvent) (*combat.ACChainEvent, error) {
		e.Breakdown.AddComponent(combat.ACComponent{
			Type:   combat.ACSourceFeature,
			Source: refs.Conditions.UnarmoredDefense(),
			Value:  secondaryMod,
		})
		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "unarmored_defense", modifyAC); err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply unarmored defense for character %s", u.MemberID)
	}

	return c, nil
}

// unarmoredDefenseDescriptor is the one entry for [UnarmoredDefenseCondition]; see [descriptors].
var unarmoredDefenseDescriptor = descriptor{
	ref:     refs.Conditions.UnarmoredDefense(),
	display: Display{Name: "Unarmored Defense"},
	load:    loaderOf[UnarmoredDefenseCondition](),
	create: func(in *CreateFromRefInput) (dnd5eEvents.ConditionBehavior, error) {
		return createdAs(createUnarmoredDefense(in.Config, in.MemberID, in.SourceRef))
	},
	ownAction:     notBearing,
	againstHolder: notBearing,
}
