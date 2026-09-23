// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import "github.com/KirkDiggler/rpg-toolkit/core"

// AttackRollOffer is an optional, provider-authored disadvantage reaction before
// this attack's dice are rolled. The owner pays its reaction and one resource
// only on acceptance. The offer is data so an unresolved attack can be persisted.
type AttackRollOffer struct {
	ReactorID    string               `json:"reactor_id"`
	Ref          core.Ref             `json:"ref"`
	Name         string               `json:"name"`
	ResourceKey  string               `json:"resource_key"`
	Disadvantage AttackModifierSource `json:"disadvantage"`
}
