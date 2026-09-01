// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type longRestOutcome string

const (
	longRestRetain longRestOutcome = "retain"
	longRestReset  longRestOutcome = "reset"
	longRestRemove longRestOutcome = "remove"
)

type longRestCase struct {
	data          json.RawMessage
	ownerID       string
	expectedRef   *core.Ref
	outcome       longRestOutcome
	removalReason string
}

// longRestCases is deliberately authored independently of conditionLoaders.
// Its explicit persisted blobs and canonical refs make every loadable condition
// name its long-rest rule without deriving either the fixture or expectation
// from production dispatch.
var longRestCases = map[string]longRestCase{
	refs.Conditions.Raging().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"raging"},
			"member_id":"member-1","damage_bonus":2,"level":5,"source":"dnd5e:features:rage",
			"saw_turn_end":true,"round_activated":2,"turns_active":3,
			"was_hit_this_turn":true,"did_attack_this_turn":true
		}`),
		ownerID:       "member-1",
		expectedRef:   refs.Conditions.Raging(),
		outcome:       longRestRemove,
		removalReason: "rest",
	},
	refs.Conditions.BrutalCritical().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"brutal_critical"},
			"member_id":"member-1","level":13,"extra_dice":2
		}`),
		ownerID:     "member-1",
		expectedRef: refs.Conditions.BrutalCritical(),
		outcome:     longRestRetain,
	},
	refs.Conditions.UnarmoredDefense().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"unarmored_defense"},
			"type":"barbarian","member_id":"member-1","source":"dnd5e:classes:barbarian"
		}`),
		ownerID:     "member-1",
		expectedRef: refs.Conditions.UnarmoredDefense(),
		outcome:     longRestRetain,
	},
	refs.Conditions.FightingStyleArchery().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"fighting_style_archery"},
			"member_id":"member-1"
		}`),
		ownerID:     "member-1",
		expectedRef: refs.Conditions.FightingStyleArchery(),
		outcome:     longRestRetain,
	},
	refs.Conditions.FightingStyleDefense().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"fighting_style_defense"},
			"member_id":"member-1"
		}`),
		ownerID:     "member-1",
		expectedRef: refs.Conditions.FightingStyleDefense(),
		outcome:     longRestRetain,
	},
	refs.Conditions.FightingStyleDueling().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"fighting_style_dueling"},
			"member_id":"member-1"
		}`),
		ownerID:     "member-1",
		expectedRef: refs.Conditions.FightingStyleDueling(),
		outcome:     longRestRetain,
	},
	refs.Conditions.FightingStyleGreatWeaponFighting().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"fighting_style_great_weapon_fighting"},
			"member_id":"member-1"
		}`),
		ownerID:     "member-1",
		expectedRef: refs.Conditions.FightingStyleGreatWeaponFighting(),
		outcome:     longRestRetain,
	},
	refs.Conditions.FightingStyleProtection().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"fighting_style_protection"},
			"member_id":"member-1"
		}`),
		ownerID:     "member-1",
		expectedRef: refs.Conditions.FightingStyleProtection(),
		outcome:     longRestRetain,
	},
	refs.Conditions.FightingStyleTwoWeaponFighting().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"fighting_style_two_weapon_fighting"},
			"member_id":"member-1"
		}`),
		ownerID:     "member-1",
		expectedRef: refs.Conditions.FightingStyleTwoWeaponFighting(),
		outcome:     longRestRetain,
	},
	refs.Conditions.ImprovedCritical().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"improved_critical"},
			"member_id":"member-1","threshold":19
		}`),
		ownerID:     "member-1",
		expectedRef: refs.Conditions.ImprovedCritical(),
		outcome:     longRestRetain,
	},
	refs.Conditions.RecklessAttack().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"reckless_attack"},
			"member_id":"member-1"
		}`),
		ownerID:       "member-1",
		expectedRef:   refs.Conditions.RecklessAttack(),
		outcome:       longRestRemove,
		removalReason: "long rest",
	},
	refs.Conditions.MartialArts().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"martial_arts"},
			"member_id":"member-1","monk_level":3
		}`),
		ownerID:     "member-1",
		expectedRef: refs.Conditions.MartialArts(),
		outcome:     longRestRetain,
	},
	refs.Conditions.UnarmoredMovement().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"unarmored_movement"},
			"member_id":"member-1","monk_level":3
		}`),
		ownerID:     "member-1",
		expectedRef: refs.Conditions.UnarmoredMovement(),
		outcome:     longRestRetain,
	},
	refs.Features.SneakAttack().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"features","id":"sneak_attack"},
			"member_id":"member-1","level":3,"damage_dice":2,"used_this_turn":true
		}`),
		ownerID:     "member-1",
		expectedRef: refs.Features.SneakAttack(),
		outcome:     longRestReset,
	},
	refs.Conditions.Disengaging().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"disengaging"},
			"member_id":"member-1"
		}`),
		ownerID:       "member-1",
		expectedRef:   refs.Conditions.Disengaging(),
		outcome:       longRestRemove,
		removalReason: "long rest",
	},
	refs.Conditions.Dodging().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"dodging"},
			"member_id":"member-1"
		}`),
		ownerID:       "member-1",
		expectedRef:   refs.Conditions.Dodging(),
		outcome:       longRestRemove,
		removalReason: "long rest",
	},
	refs.Conditions.Prone().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"prone"},
			"member_id":"member-1"
		}`),
		ownerID:       "member-1",
		expectedRef:   refs.Conditions.Prone(),
		outcome:       longRestRemove,
		removalReason: "long rest",
	},
	refs.Conditions.Hidden().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"hidden"},
			"member_id":"member-1"
		}`),
		ownerID:       "member-1",
		expectedRef:   refs.Conditions.Hidden(),
		outcome:       longRestRemove,
		removalReason: "long rest",
	},
	refs.Conditions.Helped().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"helped"},
			"member_id":"member-1","helper_id":"helper-1"
		}`),
		ownerID:       "member-1",
		expectedRef:   refs.Conditions.Helped(),
		outcome:       longRestRemove,
		removalReason: "long rest",
	},
	refs.Conditions.Unconscious().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"unconscious"},
			"member_id":"member-1","successes":1,"failures":2,"stabilized":false,"dead":false
		}`),
		ownerID:       "member-1",
		expectedRef:   refs.Conditions.Unconscious(),
		outcome:       longRestRemove,
		removalReason: "long rest",
	},
	refs.Conditions.OpportunityAttack().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"opportunity_attack"},
			"member_id":"member-1","used_this_turn":true
		}`),
		ownerID:     "member-1",
		expectedRef: refs.Conditions.OpportunityAttack(),
		outcome:     longRestReset,
	},
	refs.Spells.Shield().String(): {
		data: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"spells","id":"shield"},
			"member_id":"member-1"
		}`),
		ownerID:       "member-1",
		expectedRef:   refs.Spells.Shield(),
		outcome:       longRestRemove,
		removalReason: "long rest",
	},
}

func TestLongRestRegistryIsExhaustive(t *testing.T) {
	require.ElementsMatch(t,
		slices.Collect(maps.Keys(conditionLoaders)),
		slices.Collect(maps.Keys(longRestCases)),
	)
}

func TestLongRestRegistryBehavior(t *testing.T) {
	for refString, testCase := range longRestCases {
		t.Run(refString, func(t *testing.T) {
			ctx := context.Background()
			bus := events.NewEventBus()

			var removed []dnd5eEvents.ConditionRemovedEvent
			_, err := dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(ctx,
				func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
					removed = append(removed, event)
					return nil
				})
			require.NoError(t, err)

			var changed []dnd5eEvents.ConditionStateChangedEvent
			_, err = dnd5eEvents.ConditionStateChangedTopic.On(bus).Subscribe(ctx,
				func(_ context.Context, event dnd5eEvents.ConditionStateChangedEvent) error {
					changed = append(changed, event)
					return nil
				})
			require.NoError(t, err)

			condition, err := LoadJSON(testCase.data)
			require.NoError(t, err)
			require.Equal(t, testCase.expectedRef.String(), condition.Ref().String())

			before, err := condition.ToJSON()
			require.NoError(t, err)
			require.NoError(t, condition.Apply(ctx, bus))
			t.Cleanup(func() { require.NoError(t, condition.Remove(ctx, bus)) })

			require.NoError(t, dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
				RestType:    coreResources.ResetLongRest,
				CharacterID: testCase.ownerID,
			}))

			after, err := condition.ToJSON()
			require.NoError(t, err)

			switch testCase.outcome {
			case longRestRetain:
				require.True(t, condition.IsApplied())
				require.JSONEq(t, string(before), string(after))
				require.Empty(t, removed)
				require.Empty(t, changed)

			case longRestReset:
				require.True(t, condition.IsApplied())
				require.Empty(t, removed)
				require.Len(t, changed, 1)
				require.Equal(t, testCase.ownerID, changed[0].MemberID)
				require.Equal(t, testCase.expectedRef.String(), changed[0].ConditionRef.String())

				var state struct {
					UsedThisTurn bool `json:"used_this_turn"`
				}
				require.NoError(t, json.Unmarshal(after, &state))
				require.False(t, state.UsedThisTurn)

			case longRestRemove:
				require.False(t, condition.IsApplied())
				require.JSONEq(t, string(before), string(after))
				require.Equal(t, []dnd5eEvents.ConditionRemovedEvent{{
					MemberID:     testCase.ownerID,
					ConditionRef: testCase.expectedRef.String(),
					Reason:       testCase.removalReason,
				}}, removed)
				require.Empty(t, changed)

			default:
				t.Fatalf("unknown long-rest outcome %q", testCase.outcome)
			}
		})
	}
}
