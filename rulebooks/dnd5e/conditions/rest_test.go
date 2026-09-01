// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

const restTestMemberID = "member-1"

type temporaryConditionTestCase struct {
	newCondition func() dnd5eEvents.ConditionBehavior
	expectedRef  *core.Ref
}

func temporaryConditionTestCases() map[string]temporaryConditionTestCase {
	return map[string]temporaryConditionTestCase{
		"Reckless Attack": {
			newCondition: func() dnd5eEvents.ConditionBehavior {
				return NewRecklessAttackCondition(restTestMemberID)
			},
			expectedRef: refs.Conditions.RecklessAttack(),
		},
		"Dodging": {
			newCondition: func() dnd5eEvents.ConditionBehavior {
				return NewDodgingCondition(restTestMemberID)
			},
			expectedRef: refs.Conditions.Dodging(),
		},
		"Disengaging": {
			newCondition: func() dnd5eEvents.ConditionBehavior {
				return NewDisengagingCondition(restTestMemberID)
			},
			expectedRef: refs.Conditions.Disengaging(),
		},
		"Hidden": {
			newCondition: func() dnd5eEvents.ConditionBehavior {
				return NewHiddenCondition(restTestMemberID)
			},
			expectedRef: refs.Conditions.Hidden(),
		},
		"Helped": {
			newCondition: func() dnd5eEvents.ConditionBehavior {
				return NewHelpedCondition(restTestMemberID, "helper-1")
			},
			expectedRef: refs.Conditions.Helped(),
		},
		"Prone": {
			newCondition: func() dnd5eEvents.ConditionBehavior {
				return NewProneCondition(restTestMemberID)
			},
			expectedRef: refs.Conditions.Prone(),
		},
		"Unconscious": {
			newCondition: func() dnd5eEvents.ConditionBehavior {
				return NewUnconsciousCondition(restTestMemberID, nil)
			},
			expectedRef: refs.Conditions.Unconscious(),
		},
		"Shield": {
			newCondition: func() dnd5eEvents.ConditionBehavior {
				return NewShieldSpellCondition(restTestMemberID)
			},
			expectedRef: refs.Spells.Shield(),
		},
	}
}

func TestTemporaryConditionsEndOnLongRest(t *testing.T) {
	for name, testCase := range temporaryConditionTestCases() {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			bus := events.NewEventBus()
			condition := testCase.newCondition()

			var removed []dnd5eEvents.ConditionRemovedEvent
			_, err := dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(ctx,
				func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
					removed = append(removed, event)
					return nil
				})
			require.NoError(t, err)
			require.NoError(t, condition.Apply(ctx, bus))

			require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
				RestType:    coreResources.ResetLongRest,
				CharacterID: "other-member",
			}))
			require.Empty(t, removed, "another character's long rest must not remove the condition")
			require.True(t, condition.IsApplied())

			require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
				RestType:    coreResources.ResetShortRest,
				CharacterID: restTestMemberID,
			}))
			require.Empty(t, removed, "short rest must not remove the condition")
			require.True(t, condition.IsApplied())

			require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
				RestType:    coreResources.ResetLongRest,
				CharacterID: restTestMemberID,
			}))
			require.Equal(t, []dnd5eEvents.ConditionRemovedEvent{{
				MemberID:     restTestMemberID,
				ConditionRef: testCase.expectedRef.String(),
				Reason:       "long rest",
			}}, removed)
			require.False(t, condition.IsApplied(), "the owner's long rest must remove the condition's subscriptions")

			require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
				RestType:    coreResources.ResetLongRest,
				CharacterID: restTestMemberID,
			}))
			require.Len(t, removed, 1, "a removed condition must not publish a second removal")
		})
	}
}

func TestRagingConditionEndsOnAnyRestControl(t *testing.T) {
	for _, restType := range []coreResources.ResetType{
		coreResources.ResetShortRest,
		coreResources.ResetLongRest,
	} {
		t.Run(string(restType), func(t *testing.T) {
			ctx := context.Background()
			bus := events.NewEventBus()
			raging := &RagingCondition{CharacterID: restTestMemberID}

			var removed []dnd5eEvents.ConditionRemovedEvent
			_, err := dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(ctx,
				func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
					removed = append(removed, event)
					return nil
				})
			require.NoError(t, err)
			require.NoError(t, raging.Apply(ctx, bus))

			require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
				RestType:    restType,
				CharacterID: restTestMemberID,
			}))
			require.Equal(t, []dnd5eEvents.ConditionRemovedEvent{{
				MemberID:     restTestMemberID,
				ConditionRef: raging.Ref().String(),
				Reason:       "rest",
			}}, removed)
			require.False(t, raging.IsApplied())
		})
	}
}

var errConditionRemovalRefused = errors.New("condition removal publication refused")

func TestTemporaryConditionLongRestKeepsSubscriptionsWhenRemovalPublicationFails(t *testing.T) {
	for name, testCase := range temporaryConditionTestCases() {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			bus := events.NewEventBus()
			condition := testCase.newCondition()

			_, err := dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(ctx,
				func(_ context.Context, _ dnd5eEvents.ConditionRemovedEvent) error {
					return errConditionRemovalRefused
				})
			require.NoError(t, err)
			require.NoError(t, condition.Apply(ctx, bus))

			err = dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
				RestType:    coreResources.ResetLongRest,
				CharacterID: restTestMemberID,
			})
			require.ErrorIs(t, err, errConditionRemovalRefused)
			require.True(t, condition.IsApplied(), "failed removal publication must leave the condition applied")

			require.NoError(t, condition.Remove(ctx, bus))
		})
	}
}

var errRestSubscriptionRefused = errors.New("rest subscription refused")

type rejectRestSubscribeBus struct {
	events.EventBus
	successful   []string
	unsubscribed []string
}

func (b *rejectRestSubscribeBus) Subscribe(ctx context.Context, topic events.Topic, handler any) (string, error) {
	if topic == events.Topic("dnd5e.rest") {
		return "", errRestSubscriptionRefused
	}

	id, err := b.EventBus.Subscribe(ctx, topic, handler)
	if err == nil {
		b.successful = append(b.successful, id)
	}
	return id, err
}

func (b *rejectRestSubscribeBus) Unsubscribe(ctx context.Context, id string) error {
	b.unsubscribed = append(b.unsubscribed, id)
	return b.EventBus.Unsubscribe(ctx, id)
}

func TestTemporaryConditionApplyRollsBackWhenLongRestSubscriptionFails(t *testing.T) {
	for name, testCase := range temporaryConditionTestCases() {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			bus := &rejectRestSubscribeBus{EventBus: events.NewEventBus()}
			condition := testCase.newCondition()

			err := condition.Apply(ctx, bus)
			require.ErrorIs(t, err, errRestSubscriptionRefused)
			require.NotEmpty(t, bus.successful, "the rest subscription must be attempted after existing subscriptions")
			require.ElementsMatch(t, bus.successful, bus.unsubscribed)
			require.False(t, condition.IsApplied())

			retryBus := events.NewEventBus()
			require.NoError(t, condition.Apply(ctx, retryBus), "a rolled-back condition must be reusable")
			require.NoError(t, condition.Remove(ctx, retryBus))
		})
	}
}
