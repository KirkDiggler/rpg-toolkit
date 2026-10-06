// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
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
// Who is whose ally and who stands beside the target are frame facts
// resolution measures; the trait reads them and nothing else.
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

// onAttackChain grants advantage when an ally of the attacker is within five
// feet of the target.
//
// Who stands where and who is on whose side are read from the event's
// attack-roll frame, which resolution builds from authoritative state: the
// ally→target distance and the attacker→ally stance on the disposition graph
// at the moment of asking (R5). The rule asks for an ALLIED stance, not "not
// hostile": a neutral faction beside the target is no packmate.
//
// A frame that cannot answer fails the attack (R13): an invalid or incomplete
// frame — a partial set of pairs never proves no packmate stands there — or an
// unknown stance or distance for a member beside the target. It is never read
// as "no ally", which would switch the trait off silently.
func (p *packTacticsCondition) onAttackChain(
	_ context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	if event.AttackerID != p.ownerID {
		return c, nil
	}
	adjacent, err := p.allyAdjacentToTarget(event.Frame, event.TargetID)
	if err != nil {
		return c, err
	}
	if !adjacent {
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

// allyAdjacentToTarget reports whether the frame shows any ally of this
// creature within five feet of the target. Every member the frame pairs with
// the target is a candidate; errors wrap contributions.ErrRuleCannotAnswer.
//
// TODO(rpg-toolkit): RAW adds "and isn't incapacitated". That clause cannot be
// written yet — Incapacitated is one of thirteen standard conditions with no
// implementation, so there is nothing truthful to test. Deliberately left
// unenforced rather than approximated by something that happens to be nearby
// (downed, say), which would be a different rule wearing this one's name.
func (p *packTacticsCondition) allyAdjacentToTarget(frame contributions.Frame, target string) (bool, error) {
	if err := frame.Validate(); err != nil {
		return false, fmt.Errorf("pack tactics: %w: %w", contributions.ErrRuleCannotAnswer, err)
	}
	if !frame.Complete {
		return false, fmt.Errorf("pack tactics: %w: the frame's pairs do not cover every member",
			contributions.ErrRuleCannotAnswer)
	}
	for _, pair := range frame.Pairs {
		if pair.To != target || pair.From == p.ownerID {
			continue
		}
		stance, known := frame.Pair(p.ownerID, pair.From).Stance.Get()
		if !known {
			return false, fmt.Errorf("pack tactics: %w: stance from %q to %q is unknown",
				contributions.ErrRuleCannotAnswer, p.ownerID, pair.From)
		}
		if stance != contributions.StanceAllied {
			continue
		}
		distance, known := pair.DistanceCells.Get()
		if !known {
			return false, fmt.Errorf("pack tactics: %w: distance from %q to %q is unknown",
				contributions.ErrRuleCannotAnswer, pair.From, target)
		}
		if distance <= combat.AdjacentCells {
			return true, nil
		}
	}
	return false, nil
}
