// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// FightingStyleProtectionData is the JSON structure for persisting protection condition state
type FightingStyleProtectionData struct {
	Ref      *core.Ref `json:"ref"`
	MemberID string    `json:"member_id"`
}

// FightingStyleProtectionCondition imposes disadvantage on attacks against adjacent allies.
// When a creature you can see attacks a target other than you that is within 5 feet of you,
// you can use your reaction to impose disadvantage on the attack roll. You must be wielding a shield.
type FightingStyleProtectionCondition struct {
	MemberID        string
	subscriptionIDs []string
	bus             events.EventBus
}

// Ensure FightingStyleProtectionCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*FightingStyleProtectionCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (f *FightingStyleProtectionCondition) Ref() *core.Ref {
	return refs.Conditions.FightingStyleProtection()
}

// NewFightingStyleProtectionCondition creates a new Protection fighting style condition.
func NewFightingStyleProtectionCondition(characterID string) *FightingStyleProtectionCondition {
	return &FightingStyleProtectionCondition{
		MemberID: characterID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (f *FightingStyleProtectionCondition) IsApplied() bool {
	return f.bus != nil
}

// Apply subscribes this condition to attack chain events.
func (f *FightingStyleProtectionCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if f.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "protection fighting style already applied")
	}
	f.bus = bus

	// Subscribe to AttackChain to impose disadvantage when eligible
	attackChain := dnd5eEvents.AttackChain.On(bus)
	subID, err := attackChain.SubscribeWithChain(ctx, f.onAttackChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to attack chain")
	}
	f.subscriptionIDs = append(f.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from events.
func (f *FightingStyleProtectionCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if f.bus == nil {
		return nil
	}

	total := len(f.subscriptionIDs)
	var errs []error
	for _, subID := range f.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	f.subscriptionIDs = nil
	f.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON converts the condition to JSON for persistence.
func (f *FightingStyleProtectionCondition) ToJSON() (json.RawMessage, error) {
	data := FightingStyleProtectionData{
		Ref:      refs.Conditions.FightingStyleProtection(),
		MemberID: f.MemberID,
	}
	return json.Marshal(data)
}

// loadJSON loads protection condition state from JSON.
func (f *FightingStyleProtectionCondition) loadJSON(data json.RawMessage) error {
	var protectionData FightingStyleProtectionData
	if err := json.Unmarshal(data, &protectionData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal protection data")
	}

	f.MemberID = protectionData.MemberID
	return nil
}

// onAttackChain imposes disadvantage on attacks against nearby allies when using shield and reaction.
//
// rpg-toolkit#1178: this used to fire on the protector's OWN outgoing
// attacks too, because it excluded only "target is me" and never "attacker
// is me" — but Protection is a REACTION to someone else's attack (the doc
// comment above says so: "a creature ... attacks a target other than
// you"), never a response to your own swing. That gap put every armed
// attack by a Protection-wielding character on the code path below, which
// depends on gamectx.RequireCharacters — a live registry the session stack
// never installs — turning an unrelated eligibility bug into a crash on
// every one of that character's own attacks.
func (f *FightingStyleProtectionCondition) onAttackChain(
	ctx context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	// Never a reaction to my own attack.
	if event.AttackerID == f.MemberID {
		return c, nil
	}

	// Only triggers for attacks on OTHER creatures (not self)
	if event.TargetID == f.MemberID {
		return c, nil
	}

	// Only triggers for melee attacks
	if !event.IsMelee {
		return c, nil
	}

	// This protector's own sheet, read out of the cast the way it reads
	// anybody else's — in place of the handle a loader used to pass in at
	// attach time (rpg-toolkit#1178), which was silently absent whenever a
	// loader forgot.
	//
	// A protector nobody can look up is NOT ELIGIBLE — the old nil-owner
	// branch preserved exactly, and the same answer the opportunity attack
	// gives to the same question. A cast is installed on every path that folds
	// anything, so a fold without one is assembled wrong rather than describing
	// a participant with nothing to say.
	self, ok := member(ctx, f.MemberID)
	if !ok {
		return c, nil
	}

	// Must be wielding a shield.
	if !self.HasShieldEquipped() {
		return c, nil
	}

	// Must have a reaction available. Same question the opportunity attack
	// asks, asked of the same surface, so the two cannot drift into answering
	// "can I react" differently.
	if !self.CanReact() {
		return c, nil
	}

	// Check if we're within 5 feet of the target. Positions are genuinely
	// world state no single character's sheet carries, so this stays on
	// [gamectx.RequireRoom] rather than moving to the cast the way the shield
	// and reaction reads above did.
	//
	// It is not the only registry in play. resolution.installTruth installs
	// three — the room, the cast, and reaction readiness — and this condition
	// reads two of them: the cast at the self lookup above, the room here.
	room, err := gamectx.RequireRoom(ctx)
	if err != nil {
		return c, err
	}

	// Get positions of fighter and target
	fighterPos, fighterExists := room.GetEntityPosition(f.MemberID)
	targetPos, targetExists := room.GetEntityPosition(event.TargetID)
	if !fighterExists || !targetExists {
		return c, nil
	}

	// Check if within 5 feet (adjacent on grid = distance 1)
	grid := room.GetGrid()
	distance := grid.Distance(fighterPos, targetPos)
	if distance > 1 {
		return c, nil
	}

	// All conditions met - add modifier to impose disadvantage at StageFeatures
	modifyAttack := func(ctx context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		e.DisadvantageSources = append(e.DisadvantageSources, dnd5eEvents.AttackModifierSource{
			SourceID:  f.MemberID,
			SourceRef: refs.Conditions.FightingStyleProtection(),
			Reason:    "protection_fighting_style",
		})

		// Actually consume the reaction, and do it HERE rather than above:
		// the bill falls due when the stage runs, which is where it fell due
		// before. The keeper applies it at the instant of the publish — the
		// bus is synchronous — so the debit lands where the direct call landed.
		//
		// The request cannot be refused. The eligibility check above already
		// ran, and Ledger's own contract (combat/gate.go) is that a debit past
		// a passed check cannot fail; what can fail is the publish reaching
		// nobody, and a reaction consumed that nothing recorded is the failure
		// this whole shape exists to end.
		if err := publishSpendRequested(
			ctx, f.bus, f.MemberID, coreCombat.ActionReaction, 1, f.Ref(),
		); err != nil {
			return e, rpgerr.Wrap(err, "failed to publish protection reaction spend")
		}

		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "protection", modifyAttack); err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply protection for character %s", f.MemberID)
	}

	return c, nil
}
