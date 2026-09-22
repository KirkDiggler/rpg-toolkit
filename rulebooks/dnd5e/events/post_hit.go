// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

// PostHitEvent announces a settled hit, including a hit that dealt zero damage.
// Subscribers append optional retaliations; they never roll, spend, or apply damage.
// The chain runs after the incoming hit's consequences have been applied.
type PostHitEvent struct {
	AttackerID string
	TargetID   string
	Offers     []PostHitOffer
}

// PostHitOffer is a provider-authored reaction and its resource price. It is
// data so a posed interaction can be persisted and resumed without a live bus.
type PostHitOffer struct {
	ReactorID   string          `json:"reactor_id"`
	Ref         core.Ref        `json:"ref"`
	Name        string          `json:"name"`
	ResourceKey string          `json:"resource_key"`
	Options     []PostHitOption `json:"options"`
}

// PostHitOption describes a retaliation save and damage, never its outcome.
// The selected option is validated before its reaction and resource are spent.
type PostHitOption struct {
	ID         string            `json:"id"`
	Label      string            `json:"label"`
	Ability    abilities.Ability `json:"ability"`
	DC         int               `json:"dc"`
	Dice       string            `json:"dice"`
	DamageType damage.Type       `json:"damage_type"`
	HalfOnSave bool              `json:"half_on_save"`
}

// PostHitChain collects optional reactions using the publisher's live context.
var PostHitChain = events.DefineChainedTopic[*PostHitEvent]("dnd5e.attack.hit.reactions")
