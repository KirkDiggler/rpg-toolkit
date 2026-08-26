// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// PackTacticsData is the JSON structure for persisting pack tactics trait state
type PackTacticsData struct {
	Ref     *core.Ref `json:"ref"`
	OwnerID string    `json:"owner_id"`
}

// packTacticsCondition represents a creature's Pack Tactics ability.
// Pack Tactics grants advantage on attack rolls against a creature if at least
// one of the attacker's allies is within 5 feet of the target and not incapacitated.
//
// This was a stub until rpg-toolkit#1251. It could not be written before,
// because a trait had no way to ask who anybody was to anybody else — the
// registry that would have answered was never installed, and the only other
// signal available was entity type, which cannot tell one monster faction from
// another. gamectx.Cast answers it now.
type packTacticsCondition struct {
	ownerID string
	bus     events.EventBus
	subID   string
}

// Ensure packTacticsCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*packTacticsCondition)(nil)

// Ref returns the canonical ref this trait names itself by — the same ref its
// ToJSON embeds and its loader routes on.
func (p *packTacticsCondition) Ref() *core.Ref { return refs.MonsterTraits.PackTactics() }

// PackTactics creates a new pack tactics trait
func PackTactics(ownerID string) dnd5eEvents.ConditionBehavior {
	return &packTacticsCondition{
		ownerID: ownerID,
	}
}

// IsApplied returns true if this condition is currently applied
func (p *packTacticsCondition) IsApplied() bool {
	return p.bus != nil
}

// Apply subscribes this condition to relevant combat events
func (p *packTacticsCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if p.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "pack tactics condition already applied")
	}
	p.bus = bus

	// Subscribe to attack chain to grant advantage when ally is adjacent to target
	attackChain := dnd5eEvents.AttackChain.On(bus)
	subID, err := attackChain.SubscribeWithChain(ctx, p.onAttackChain)
	if err != nil {
		return err
	}
	p.subID = subID

	return nil
}

// Remove unsubscribes this condition from events
func (p *packTacticsCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if p.bus == nil {
		return nil // Not applied, nothing to remove
	}

	if p.subID != "" {
		err := bus.Unsubscribe(ctx, p.subID)
		if err != nil {
			return err
		}
	}

	p.subID = ""
	p.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (p *packTacticsCondition) ToJSON() (json.RawMessage, error) {
	data := PackTacticsData{
		Ref:     refs.MonsterTraits.PackTactics(),
		OwnerID: p.ownerID,
	}
	return json.Marshal(data)
}

// loadJSON loads pack tactics condition state from JSON
func (p *packTacticsCondition) loadJSON(data json.RawMessage) error {
	var tacticsData PackTacticsData
	if err := json.Unmarshal(data, &tacticsData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal pack tactics data")
	}

	p.ownerID = tacticsData.OwnerID

	return nil
}

// packTacticsReachCells is 5 feet in grid cells, widened to 1.5 so the four
// diagonals count as adjacent.
const packTacticsReachCells = 1.5

// onAttackChain grants advantage when an ally of the attacker is within five
// feet of the target.
//
// Asks IsAllied rather than IsHostile. Those are momentarily each other's
// complement — both answer "same MemberKind" today — but they are different
// questions and the rule has to ask the one it means. The moment a third
// faction can be neutral, "not my enemy" would start counting bystanders as
// pack-mates.
//
// A question that cannot be answered leaves the chain untouched. Never an
// error: this folds into the attack chain, and an errored fold discards every
// other contribution to that roll along with this one (rpg-toolkit#1254).
func (p *packTacticsCondition) onAttackChain(
	ctx context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	if event.AttackerID != p.ownerID {
		return c, nil
	}
	if !p.allyAdjacentToTarget(ctx, event) {
		return c, nil
	}

	modifyAttack := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		e.AdvantageSources = append(e.AdvantageSources, dnd5eEvents.AttackModifierSource{
			SourceRef: refs.MonsterTraits.PackTactics(),
			SourceID:  p.ownerID,
			Reason:    "Pack Tactics - ally adjacent to target",
		})
		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "pack_tactics", modifyAttack); err != nil {
		return c, rpgerr.Wrapf(err, "error applying pack tactics for owner %s", p.ownerID)
	}

	return c, nil
}

// allyAdjacentToTarget reports whether any ally of this creature stands within
// five feet of the target.
//
// TODO(rpg-toolkit): RAW adds "and isn't incapacitated". That clause cannot be
// written yet — Incapacitated is one of thirteen standard conditions with no
// implementation, so there is nothing truthful to test. Deliberately left
// unenforced rather than approximated by something that happens to be nearby
// (downed, say), which would be a different rule wearing this one's name.
func (p *packTacticsCondition) allyAdjacentToTarget(
	ctx context.Context,
	event dnd5eEvents.AttackChainEvent,
) bool {
	room, ok := gamectx.Room(ctx)
	if !ok {
		return false
	}
	cast, ok := gamectx.CastOf(ctx)
	if !ok {
		return false
	}

	targetPos, found := room.GetEntityPosition(event.TargetID)
	if !found {
		return false
	}

	for _, entity := range room.GetEntitiesInRange(targetPos, packTacticsReachCells) {
		id := entity.GetID()
		if id == event.TargetID || id == p.ownerID {
			continue
		}
		if allied, known := cast.IsAllied(p.ownerID, id); known && allied {
			return true
		}
	}

	return false
}
