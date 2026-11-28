// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// useRageFromItsDescription stands in for the resolver's effect machine
// (rulebooks/dnd5e/resolution/effect.go), which this module cannot import. It
// does what the door does, in the door's order, and reads everything from the
// DESCRIPTION: it asks the feature for its description (a refusal is returned,
// nothing built or paid), builds the status first from
// definition.Effect.Condition, pays definition.Cost at the gate (a fresh turn's
// one bonus action, then the owner's pool), and only then publishes the status
// to its user as a feature's. It returns the first refusal, and the status it
// built when it went through.
func useRageFromItsDescription(
	ctx context.Context, t *testing.T, rage *Rage, owner *StubEntity, bus events.EventBus,
) (dnd5eEvents.ConditionBehavior, error) {
	t.Helper()
	definition, err := rage.Definition(owner)
	if err != nil {
		return nil, err
	}
	require.NotNil(t, definition.Effect, "the description says what Rage puts on its user")
	require.NotNil(t, definition.Effect.Condition)
	status := definition.Effect.Condition
	built, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
		Ref:      status.Ref.String(),
		Config:   status.Parameters,
		MemberID: owner.GetID(),
		Origin:   &dnd5eEvents.ConditionOrigin{Ref: definition.Ref, EntityID: owner.GetID()},
	})
	require.NoError(t, err)
	if err := dnd5eCombat.Pay(&descriptionLedger{owner: owner, bonus: 1}, definition.Cost); err != nil {
		return nil, err
	}
	require.NoError(t, dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target: owner, Source: dnd5eEvents.ConditionSourceFeature, Condition: built.Condition,
	}))
	return built.Condition, nil
}
