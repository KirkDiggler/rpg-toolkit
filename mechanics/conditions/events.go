// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Event refs for condition events
var (
	ConditionAppliedEventRef = mustParseRef("condition:applied")
	ConditionRemovedEventRef = mustParseRef("condition:removed")
	ConditionTickEventRef    = mustParseRef("condition:tick")
)

func mustParseRef(s string) *core.Ref {
	ref, err := core.ParseString(s)
	if err != nil {
		panic(err)
	}
	return ref
}

// ConditionAppliedEvent is published when a condition is applied
type ConditionAppliedEvent struct {
	events.BaseEvent
	Condition Condition
	EntityID  string
}

// ConditionRemovedEvent is published when a condition is removed
type ConditionRemovedEvent struct {
	events.BaseEvent
	ConditionID string
	EntityID    string
	Reason      string
}

// ConditionTickEvent is published when conditions should process their tick
type ConditionTickEvent struct {
	events.BaseEvent
	Round int
}