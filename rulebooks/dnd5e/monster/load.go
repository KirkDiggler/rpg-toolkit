// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// conditionPolicy decides what a load does with the condition blobs a monster
// was persisted with.
type conditionPolicy int

const (
	// carryConditions keeps the blobs on the monster, unapplied, so that
	// ToData writes back what it read. This is the zero value: carrying is
	// what a loader should do, and losing them takes a deliberate choice.
	carryConditions conditionPolicy = iota

	// dropConditions ignores them, which is what [LoadFromData] has always
	// done — its callers pass the same blobs to
	// monstertraits.LoadMonsterConditions themselves, and carrying them here
	// too would write every condition back twice.
	dropConditions
)

// Load turns persisted data into a monster, with no event bus involved.
//
// Nothing subscribes, nothing is applied; the monster is inert until a
// [SheetKeeper] puts it on a bus. Unlike [LoadFromData], Load does not throw
// away the condition blobs it was handed: this package cannot route them to
// their loaders (monstertraits imports monster, so monster cannot import
// monstertraits), but it can carry them, and carrying them is what makes
// Load(d).ToData() the data it was given. monstertraits.AttachMonster is what
// turns them into behaviour.
//
// Load is strict about those blobs to the extent it can be without a loader:
// one that is not JSON, or that names no ref for a loader to route on, fails
// the load and the error names it. Whether the ref resolves to a trait this
// build knows is monstertraits' answer to give, at attach time.
//
// Actions are already inert shared definitions. Load validates and clones them
// directly; no behavior loader or second representation is involved.
//
// Two fields of [Data] survive no loader: Features and Inventory have nowhere
// to live on the monster and ToData does not write them. That is a gap in the
// sheet rather than in this loader, pinned by TestKnownRoundTripGaps so it
// cannot be mistaken for a guarantee.
//
// ctx is unused: a pure load has nothing to cancel. It is in the signature so
// that the pure and legacy loaders read alike at a call site.
func Load(_ context.Context, d *Data) (*Monster, error) {
	if d == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "monster data is required")
	}

	return loadMonster(d, carryConditions)
}

// TakeUnappliedConditions returns the condition blobs this monster is carrying
// that no bus has seen, and clears them from the monster.
//
// Draining rather than reading, because ToData serializes both the applied
// conditions and the unapplied blobs: an attach helper that loaded the blobs
// into behaviours without clearing them would write the monster back carrying
// every condition twice.
func (m *Monster) TakeUnappliedConditions() []json.RawMessage {
	pending := m.traitData
	m.traitData = nil

	return pending
}

// loadMonster builds the sheet both loaders hand back. Everything that reads
// [Data] lives here; everything that touches a bus lives in [SheetKeeper].
func loadMonster(d *Data, policy conditionPolicy) (*Monster, error) {
	// Handle proficiency bonus - default to 2 if not set
	profBonus := d.ProficiencyBonus
	if profBonus == 0 {
		profBonus = 2
	}

	m := &Monster{
		id:               d.ID,
		name:             d.Name,
		ref:              d.Ref,
		hp:               d.HitPoints,
		maxHP:            d.MaxHitPoints,
		ac:               d.ArmorClass,
		abilityScores:    d.AbilityScores,
		proficiencyBonus: profBonus,
		speed:            d.Speed,
		senses:           d.Senses,
		targeting:        d.Targeting,
		subscriptionIDs:  make([]string, 0),
		actions:          make([]combatActions.Definition, 0, len(d.Actions)),
		proficiencies:    make(map[string]int),
		conditions:       make([]dnd5eEvents.ConditionBehavior, 0, len(d.Conditions)),
	}

	for index, definition := range d.Actions {
		if err := m.AddAction(definition); err != nil {
			return nil, rpgerr.Wrapf(err, "failed to load monster action %d", index)
		}
	}

	// Load proficiencies
	for _, prof := range d.Proficiencies {
		m.proficiencies[prof.Skill] = prof.Bonus
	}

	if policy == dropConditions {
		return m, nil
	}

	for i, raw := range d.Conditions {
		var peek struct {
			Ref core.Ref `json:"ref"`
		}
		if err := json.Unmarshal(raw, &peek); err != nil {
			return nil, rpgerr.Wrapf(err, "failed to read the ref of condition %d: %s", i, raw)
		}
		if peek.Ref.ID == "" {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"condition %d names no ref for a loader to route on: %s", i, raw)
		}
	}
	m.traitData = append(m.traitData, d.Conditions...)

	return m, nil
}

// SheetKeeper is a monster's own reaction to the world, made attachable.
//
// A monster sheet keeps damage, healing, and live condition applications, but the principle is
// the same one rpg-toolkit#985 is about: that is behaviour, and behaviour
// belongs in something a caller can attach, name in a registration ledger, and
// take back, rather than in wiring hidden inside a constructor.
//
// Get one from [Monster.SheetKeeper].
type SheetKeeper struct {
	monster *Monster

	// subscriptionIDs are the hooks this keeper granted, so Remove revokes
	// exactly those and nothing else.
	subscriptionIDs []string
}

// SheetKeeper returns the attachable carrying this monster's own behaviour.
//
// The keeper is created once and kept, so two callers asking a monster for its
// keeper get the same one and cannot accidentally subscribe it twice.
func (m *Monster) SheetKeeper() *SheetKeeper {
	if m.keeper == nil {
		m.keeper = &SheetKeeper{monster: m}
	}

	return m.keeper
}

// Apply subscribes the monster's damage, healing, and condition handlers to bus.
//
// The handlers close over this bus rather than reading one off the monster: a
// purely loaded monster has none of its own.
//
// **A failed Apply is a no-op.** A monster subscribed to damage but not healing
// would take every hit and heal from nothing, and a keeper left holding that
// half-attachment would refuse every later Apply as already applied — a leak
// that also cannot be retried. So whatever went on comes back off, the monster
// gets back the bus it was holding, and the keeper is appliable again.
func (k *SheetKeeper) Apply(ctx context.Context, bus events.EventBus) error {
	if k.monster == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "sheet keeper has no monster")
	}
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}
	if len(k.subscriptionIDs) > 0 {
		// Attaching twice would double the handlers: one damage event would
		// take its damage twice.
		return rpgerr.New(rpgerr.CodeAlreadyExists, "sheet keeper is already applied")
	}

	m := k.monster
	previousBus := m.bus

	// Park the bus on the monster for Cleanup, which is the only method still
	// reading it. It goes away with the rest of the migration
	// (rpg-toolkit#965, #966); the handlers below never read it.
	m.bus = bus

	handlers := []struct {
		what      string
		subscribe func() (string, error)
	}{
		// This row treats a notification as an instruction — it calls TakeDamage
		// on an event that reports damage already landed — which is why
		// resolution refuses to publish DamageReceivedEvent at all. That refusal
		// is argued in full at resolution/strike.go's afterDamage, in a different
		// module from the row that causes it, and rpg-toolkit#977 owns the fix.
		// Read it before changing anything here.
		{"damage received", func() (string, error) {
			return dnd5eEvents.DamageReceivedTopic.On(bus).Subscribe(ctx, m.onDamageReceived)
		}},
		{"healing received", func() (string, error) {
			return dnd5eEvents.HealingReceivedTopic.On(bus).Subscribe(ctx,
				func(ctx context.Context, event dnd5eEvents.HealingReceivedEvent) error {
					return m.onHealingReceived(ctx, bus, event)
				})
		}},
		{"condition applied", func() (string, error) {
			return dnd5eEvents.ConditionAppliedTopic.On(bus).Subscribe(ctx,
				func(ctx context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
					return m.onConditionApplied(ctx, bus, event)
				})
		}},
		{"condition removed", func() (string, error) {
			return dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(ctx, m.onConditionRemoved)
		}},
		{"condition state changed", func() (string, error) {
			return dnd5eEvents.ConditionStateChangedTopic.On(bus).Subscribe(ctx, m.onConditionStateChanged)
		}},
	}

	for _, handler := range handlers {
		subID, err := handler.subscribe()
		if err != nil {
			k.unsubscribeSelf(ctx, bus)
			m.bus = previousBus

			return rpgerr.Wrapf(err, "failed to subscribe to %s", handler.what)
		}

		k.track(subID)
	}

	return nil
}

// unsubscribeSelf revokes the subscriptions this keeper granted and forgets
// them, leaving the keeper unapplied and therefore appliable again.
func (k *SheetKeeper) unsubscribeSelf(ctx context.Context, bus events.EventBus) {
	for _, subID := range k.subscriptionIDs {
		_ = bus.Unsubscribe(ctx, subID)
	}

	k.monster.forgetSubscriptions(k.subscriptionIDs)
	k.subscriptionIDs = nil
}

// Remove revokes every subscription this keeper granted, and nothing else.
func (k *SheetKeeper) Remove(ctx context.Context, bus events.EventBus) error {
	if k.monster == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "sheet keeper has no monster")
	}
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}

	var firstErr error

	for _, subID := range k.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil && firstErr == nil {
			firstErr = rpgerr.Wrapf(err, "failed to unsubscribe")
		}
	}

	k.monster.forgetSubscriptions(k.subscriptionIDs)
	k.subscriptionIDs = nil

	return firstErr
}

// track records a subscription on the keeper and on the monster: the keeper
// needs to know what to revoke, and Cleanup — which predates the keeper —
// revokes whatever the monster is carrying.
func (k *SheetKeeper) track(subID string) {
	k.subscriptionIDs = append(k.subscriptionIDs, subID)
	k.monster.subscriptionIDs = append(k.monster.subscriptionIDs, subID)
}

// forgetSubscriptions drops the given subscription IDs from the monster's
// list, so a Cleanup after a [SheetKeeper.Remove] does not try to revoke hooks
// that are already gone.
func (m *Monster) forgetSubscriptions(revoked []string) {
	if len(revoked) == 0 || len(m.subscriptionIDs) == 0 {
		return
	}

	gone := make(map[string]struct{}, len(revoked))
	for _, id := range revoked {
		gone[id] = struct{}{}
	}

	kept := make([]string, 0, len(m.subscriptionIDs))
	for _, id := range m.subscriptionIDs {
		if _, ok := gone[id]; !ok {
			kept = append(kept, id)
		}
	}
	m.subscriptionIDs = kept
}
