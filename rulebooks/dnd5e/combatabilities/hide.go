// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combatabilities

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/checks"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Hide represents the Hide combat ability (PHB p.192).
// When activated, it consumes 1 action, publishes a HideActivatedEvent, and
// makes a Dexterity (Stealth) check via checks.MakeAbilityCheck against the
// highest passive Perception among input.ObserverPassivePerceptions (binary
// success/fail — not per-observer; see the design doc's "Hide's success
// granularity" decision). On success the character becomes Hidden
// (HiddenCondition): advantage on their attacks, disadvantage on attacks
// against them, until they attack.
//
// The DC is the caller's responsibility to gather (the encounter SDK
// computes the observer set via perception.CanSeeAt and calls each
// observer's Combatant.PassivePerception() — Hide only does the "beat the
// highest" comparison, never the position/LOS work).
type Hide struct {
	*BaseCombatAbility
}

// stealthChecker is implemented by entities that can make a Stealth check.
// Character satisfies this. The interface keeps Hide decoupled from a direct
// dependency on the character package (owner arrives as core.Entity).
type stealthChecker interface {
	GetSkillModifier(skill skills.Skill) int
}

// HideData is the JSON structure for persisting Hide ability state.
type HideData struct {
	Ref *core.Ref `json:"ref"`
	ID  string    `json:"id"`
}

// NewHide creates a new Hide combat ability that uses a standard action.
// This is the default Hide action available to all characters.
func NewHide(id string) *Hide {
	return &Hide{
		BaseCombatAbility: NewBaseCombatAbility(BaseCombatAbilityConfig{
			ID:          id,
			Name:        "Hide",
			Description: "Make a Stealth check to become hidden from creatures that can't see you.",
			ActionType:  coreCombat.ActionStandard,
			Ref:         refs.CombatAbilities.Hide(),
		}),
	}
}

// CanActivate checks if the Hide ability can be activated.
// Requires an available action, an event bus, and an owner that can make
// skill checks.
func (h *Hide) CanActivate(ctx context.Context, owner core.Entity, input CombatAbilityInput) error {
	if err := h.BaseCombatAbility.CanActivate(ctx, owner, input); err != nil {
		return err
	}
	if input.Bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus required for Hide")
	}
	if _, ok := owner.(stealthChecker); !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "Hide requires a character that can make skill checks")
	}
	return nil
}

// Activate consumes 1 action, publishes a HideActivatedEvent, makes the
// Stealth check, and — on success — applies HiddenCondition. A failed check
// still consumes the action (RAW: no condition applied, no do-over).
func (h *Hide) Activate(ctx context.Context, owner core.Entity, input CombatAbilityInput) error {
	if input.Bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus required for Hide")
	}
	sc, ok := owner.(stealthChecker)
	if !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "Hide requires a character that can make skill checks")
	}
	if err := h.BaseCombatAbility.Activate(ctx, owner, input); err != nil {
		return err
	}

	dc := highestPassivePerception(input.ObserverPassivePerceptions)

	result, err := checks.MakeAbilityCheck(ctx, &checks.AbilityCheckInput{
		Roller:    input.Roller,
		EventBus:  input.Bus,
		CheckerID: owner.GetID(),
		Skill:     skills.Stealth,
		DC:        dc,
		Modifier:  sc.GetSkillModifier(skills.Stealth),
	})
	if err != nil {
		return fmt.Errorf("failed to make stealth check: %w", err)
	}

	if err := dnd5eEvents.HideActivatedTopic.On(input.Bus).Publish(ctx, dnd5eEvents.HideActivatedEvent{
		CharacterID: owner.GetID(),
	}); err != nil {
		return fmt.Errorf("failed to publish hide activated event: %w", err)
	}

	if !result.Success {
		return nil
	}

	hiddenCondition := conditions.NewHiddenCondition(owner.GetID())
	if err := dnd5eEvents.ConditionAppliedTopic.On(input.Bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target:    owner,
		Type:      dnd5eEvents.ConditionHidden,
		Source:    dnd5eEvents.ConditionSourceCombatAbility,
		Condition: hiddenCondition,
	}); err != nil {
		return fmt.Errorf("failed to publish hidden condition applied event: %w", err)
	}

	return nil
}

// highestPassivePerception returns the highest value in observerPP, or 0 if
// empty (no observers means DC 0 — the check still rolls, but a Stealth
// total below 0 is essentially unreachable in practice, not structurally
// impossible). Hide's own "beat the highest" comparison — the caller only
// gathers the raw values via Combatant.PassivePerception(), never the DC
// itself.
func highestPassivePerception(observerPP []int) int {
	highest := 0
	for _, pp := range observerPP {
		if pp > highest {
			highest = pp
		}
	}
	return highest
}

// ToJSON converts the Hide ability to JSON for persistence.
func (h *Hide) ToJSON() (json.RawMessage, error) {
	data := HideData{Ref: h.Ref(), ID: h.GetID()}
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hide ability data: %w", err)
	}
	return bytes, nil
}

// loadJSON deserializes a Hide ability from JSON.
func (h *Hide) loadJSON(data json.RawMessage) error {
	var hideData HideData
	if err := json.Unmarshal(data, &hideData); err != nil {
		return fmt.Errorf("failed to unmarshal hide ability data: %w", err)
	}
	h.BaseCombatAbility = NewBaseCombatAbility(BaseCombatAbilityConfig{
		ID:          hideData.ID,
		Name:        "Hide",
		Description: "Make a Stealth check to become hidden from creatures that can't see you.",
		ActionType:  coreCombat.ActionStandard,
		Ref:         refs.CombatAbilities.Hide(),
	})
	return nil
}
