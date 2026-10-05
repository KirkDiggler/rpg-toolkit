// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/weaponattack"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// HeldWeapon identifies one occupied hand, not every copy of a catalog weapon.
// Inventory currently stores quantities by equipment ID, so the slot is part
// of the binding. Labels are presentation only; neither label nor client data
// determines which weapon receives the enchantment.
type HeldWeapon struct {
	Slot   string `json:"slot"`
	ItemID string `json:"item_id"`
	Label  string `json:"label,omitempty"`
}

// ShillelaghConfig carries the server-compiled eligible weapons. The generic
// cast option binding writes WeaponSlot when more than one hand qualifies.
type ShillelaghConfig struct {
	Weapons    []HeldWeapon      `json:"weapons"`
	WeaponSlot string            `json:"weapon_slot"`
	Ability    abilities.Ability `json:"ability"`
}

// ShillelaghCondition persists the selected held weapon and its own clock.
// The weapon catalog stays immutable; attack assembly asks this condition for
// an override before any attack or damage dice are rolled.
type ShillelaghCondition struct {
	MemberID         string            `json:"member_id"`
	Weapon           HeldWeapon        `json:"weapon"`
	Ability          abilities.Ability `json:"ability"`
	TurnEndsLeft     int               `json:"turn_ends_left"`
	SkipFirstTurnEnd bool              `json:"skip_first_turn_end"`
	bus              events.EventBus
	subscriptions    []string
}

// NewShillelaghCondition selects only from authoritative held-weapon choices.
func NewShillelaghCondition(memberID string, config ShillelaghConfig) (*ShillelaghCondition, error) {
	if memberID == "" || config.Ability == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "Shillelagh requires a caster and spellcasting ability")
	}
	for _, weapon := range config.Weapons {
		if weapon.Slot == config.WeaponSlot && weapon.Slot != "" && weapon.ItemID != "" {
			return &ShillelaghCondition{MemberID: memberID, Weapon: weapon, Ability: config.Ability, TurnEndsLeft: 10, SkipFirstTurnEnd: true}, nil
		}
	}
	return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "Shillelagh requires a selected held club or quarterstaff")
}

// Ref identifies the condition.
func (s *ShillelaghCondition) Ref() *core.Ref { return refs.Conditions.Shillelagh() }

// ConditionAddress identifies the caster's one enchantment.
func (s *ShillelaghCondition) ConditionAddress() dnd5eEvents.ConditionAddress {
	return dnd5eEvents.ConditionAddress{MemberID: s.MemberID, ConditionRef: s.Ref().String(), SourceID: s.MemberID}
}

// EquipmentBinding tells the sheet which occupied hand must remain equipped.
func (s *ShillelaghCondition) EquipmentBinding() (string, string) {
	return s.Weapon.Slot, s.Weapon.ItemID
}

// MoveEquipmentBinding preserves an identified hand-to-hand transfer.
func (s *ShillelaghCondition) MoveEquipmentBinding(slot string) { s.Weapon.Slot = slot }

// WeaponAttackOverride affects only attacks with the selected held weapon.
func (s *ShillelaghCondition) WeaponAttackOverride(slot, itemID string) *weaponattack.Override {
	if s.TurnEndsLeft <= 0 || !s.binds(slot, itemID) {
		return nil
	}
	return &weaponattack.Override{Dice: "1d8", Ability: s.Ability, Magical: true}
}

// binds reports whether a hand and the equipment ID it holds are the
// enchanted weapon — the one predicate attack assembly and AssessAction share.
// Inventory stores a weapon by its catalogue ID, so the frame's weapon ID is
// the equipment ID assembly was handed.
func (s *ShillelaghCondition) binds(slot, itemID string) bool {
	return slot == s.Weapon.Slot && itemID == s.Weapon.ItemID
}

// IsApplied reports whether the clock is attached to a bus.
func (s *ShillelaghCondition) IsApplied() bool { return s.bus != nil }

// Apply attaches the independent duration. Reattaching a persisted condition
// does not reset the clock or count as a recast.
func (s *ShillelaghCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}
	if s.bus != nil {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "Shillelagh already applied")
	}
	s.bus = bus
	subscribe := []func() (string, error){
		func() (string, error) { return dnd5eEvents.TurnEndTopic.On(bus).Subscribe(ctx, s.onTurnEnd) },
		func() (string, error) {
			return dnd5eEvents.CombatEndTopic.On(bus).Subscribe(ctx, func(ctx context.Context, e dnd5eEvents.CombatEndEvent) error {
				if e.SubjectID == s.MemberID {
					return s.end(ctx, "combat ended")
				}
				return nil
			})
		},
		func() (string, error) {
			return dnd5eEvents.RestTopic.On(bus).Subscribe(ctx, func(ctx context.Context, e dnd5eEvents.RestEvent) error {
				if e.CharacterID == s.MemberID {
					return s.end(ctx, "rest")
				}
				return nil
			})
		},
	}
	for _, attach := range subscribe {
		id, err := attach()
		if err != nil {
			_ = s.Remove(ctx, bus)
			return err
		}
		s.subscriptions = append(s.subscriptions, id)
	}
	return nil
}

// Remove detaches runtime subscriptions without altering persisted facts.
func (s *ShillelaghCondition) Remove(ctx context.Context, bus events.EventBus) error {
	var errs []error
	for _, id := range s.subscriptions {
		if err := bus.Unsubscribe(ctx, id); err != nil {
			errs = append(errs, err)
		}
	}
	s.subscriptions = nil
	s.bus = nil
	return errors.Join(errs...)
}

func (s *ShillelaghCondition) onTurnEnd(ctx context.Context, e dnd5eEvents.TurnEndEvent) error {
	if e.SubjectID != s.MemberID {
		return nil
	}
	if s.SkipFirstTurnEnd {
		s.SkipFirstTurnEnd = false
	} else {
		s.TurnEndsLeft--
	}
	if s.TurnEndsLeft <= 0 {
		return s.end(ctx, "expired")
	}
	return dnd5eEvents.ConditionStateChangedTopic.On(s.bus).Publish(ctx, dnd5eEvents.ConditionStateChangedEvent{MemberID: s.MemberID, ConditionRef: s.Ref()})
}

func (s *ShillelaghCondition) end(ctx context.Context, reason string) error {
	if s.bus == nil {
		return nil
	}
	bus := s.bus
	if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{MemberID: s.MemberID, ConditionRef: s.Ref().String(), SourceID: s.MemberID, Reason: reason}); err != nil {
		return err
	}
	return s.Remove(ctx, bus)
}

// ToJSON includes the canonical ref used by the shared condition loader.
func (s *ShillelaghCondition) ToJSON() (json.RawMessage, error) {
	type state ShillelaghCondition
	return json.Marshal(struct {
		Ref *core.Ref `json:"ref"`
		*state
	}{s.Ref(), (*state)(s)})
}

func (s *ShillelaghCondition) loadJSON(data json.RawMessage) error {
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}
	if s.MemberID == "" || s.Weapon.Slot == "" || s.Weapon.ItemID == "" || s.Ability == "" || s.TurnEndsLeft <= 0 {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "invalid persisted Shillelagh")
	}
	return nil
}
