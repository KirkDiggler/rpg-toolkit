// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type attachedLongRestOutcome string

const (
	attachedLongRestRetain attachedLongRestOutcome = "retain"
	attachedLongRestReset  attachedLongRestOutcome = "reset"
	attachedLongRestRemove attachedLongRestOutcome = "remove"
)

type attachedLongRestCase struct {
	expectedRef *core.Ref
	outcome     attachedLongRestOutcome
	persisted   json.RawMessage
}

// attachedLongRestCases is authored independently of the conditions package's
// loader registry and tests. Each entry carries its own persisted fixture,
// canonical ref, and expected attached-character outcome.
var attachedLongRestCases = []attachedLongRestCase{
	{
		expectedRef: refs.Conditions.Raging(),
		outcome:     attachedLongRestRemove,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"raging"},
			"member_id":"condition-rest-member","damage_bonus":2,"level":5,
			"source":"dnd5e:features:rage","saw_turn_end":true,"round_activated":2,
			"turns_active":3,"was_hit_this_turn":true,"did_attack_this_turn":true
		}`),
	},
	{
		expectedRef: refs.Conditions.BrutalCritical(),
		outcome:     attachedLongRestRetain,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"brutal_critical"},
			"member_id":"condition-rest-member","level":13,"extra_dice":2
		}`),
	},
	{
		expectedRef: refs.Conditions.UnarmoredDefense(),
		outcome:     attachedLongRestRetain,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"unarmored_defense"},
			"type":"barbarian","member_id":"condition-rest-member","source":"dnd5e:classes:barbarian"
		}`),
	},
	{
		expectedRef: refs.Conditions.FightingStyleArchery(),
		outcome:     attachedLongRestRetain,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"fighting_style_archery"},
			"member_id":"condition-rest-member"
		}`),
	},
	{
		expectedRef: refs.Conditions.FightingStyleDefense(),
		outcome:     attachedLongRestRetain,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"fighting_style_defense"},
			"member_id":"condition-rest-member"
		}`),
	},
	{
		expectedRef: refs.Conditions.FightingStyleDueling(),
		outcome:     attachedLongRestRetain,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"fighting_style_dueling"},
			"member_id":"condition-rest-member"
		}`),
	},
	{
		expectedRef: refs.Conditions.FightingStyleGreatWeaponFighting(),
		outcome:     attachedLongRestRetain,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"fighting_style_great_weapon_fighting"},
			"member_id":"condition-rest-member"
		}`),
	},
	{
		expectedRef: refs.Conditions.FightingStyleProtection(),
		outcome:     attachedLongRestRetain,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"fighting_style_protection"},
			"member_id":"condition-rest-member"
		}`),
	},
	{
		expectedRef: refs.Conditions.FightingStyleTwoWeaponFighting(),
		outcome:     attachedLongRestRetain,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"fighting_style_two_weapon_fighting"},
			"member_id":"condition-rest-member"
		}`),
	},
	{
		expectedRef: refs.Conditions.ImprovedCritical(),
		outcome:     attachedLongRestRetain,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"improved_critical"},
			"member_id":"condition-rest-member","threshold":19
		}`),
	},
	{
		expectedRef: refs.Conditions.RecklessAttack(),
		outcome:     attachedLongRestRemove,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"reckless_attack"},
			"member_id":"condition-rest-member"
		}`),
	},
	{
		expectedRef: refs.Conditions.MartialArts(),
		outcome:     attachedLongRestRetain,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"martial_arts"},
			"member_id":"condition-rest-member","monk_level":3
		}`),
	},
	{
		expectedRef: refs.Conditions.UnarmoredMovement(),
		outcome:     attachedLongRestRetain,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"unarmored_movement"},
			"member_id":"condition-rest-member","monk_level":3
		}`),
	},
	{
		expectedRef: refs.Features.SneakAttack(),
		outcome:     attachedLongRestReset,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"features","id":"sneak_attack"},
			"member_id":"condition-rest-member","level":3,"damage_dice":2,"used_this_turn":true
		}`),
	},
	{
		expectedRef: refs.Conditions.Disengaging(),
		outcome:     attachedLongRestRemove,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"disengaging"},
			"member_id":"condition-rest-member"
		}`),
	},
	{
		expectedRef: refs.Conditions.Dodging(),
		outcome:     attachedLongRestRemove,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"dodging"},
			"member_id":"condition-rest-member"
		}`),
	},
	{
		expectedRef: refs.Conditions.Prone(),
		outcome:     attachedLongRestRemove,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"prone"},
			"member_id":"condition-rest-member"
		}`),
	},
	{
		expectedRef: refs.Conditions.Hidden(),
		outcome:     attachedLongRestRemove,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"hidden"},
			"member_id":"condition-rest-member"
		}`),
	},
	{
		expectedRef: refs.Conditions.Helped(),
		outcome:     attachedLongRestRemove,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"helped"},
			"member_id":"condition-rest-member","helper_id":"helper-1"
		}`),
	},
	{
		expectedRef: refs.Conditions.Unconscious(),
		outcome:     attachedLongRestRemove,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"unconscious"},
			"member_id":"condition-rest-member","successes":1,"failures":2,
			"stabilized":false,"dead":false
		}`),
	},
	{
		expectedRef: refs.Conditions.OpportunityAttack(),
		outcome:     attachedLongRestReset,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"conditions","id":"opportunity_attack"},
			"member_id":"condition-rest-member","used_this_turn":true
		}`),
	},
	{
		expectedRef: refs.Spells.Shield(),
		outcome:     attachedLongRestRemove,
		persisted: json.RawMessage(`{
			"ref":{"module":"dnd5e","type":"spells","id":"shield"},
			"member_id":"condition-rest-member"
		}`),
	},
}

func TestLongRestPersistsEveryConditionOutcomeOnAttachedCharacter(t *testing.T) {
	const (
		ownerID              = "condition-rest-member"
		expectedCaseCount    = 22
		expectedRetainCount  = 11
		expectedResetCount   = 2
		expectedRemovalCount = 9
	)

	require.Len(t, attachedLongRestCases, expectedCaseCount,
		"the current condition-loader audit has exactly 22 explicit cases")
	seen := make(map[string]struct{}, expectedCaseCount)
	outcomeCounts := make(map[attachedLongRestOutcome]int, 3)

	for _, testCase := range attachedLongRestCases {
		refString := testCase.expectedRef.String()
		require.NotContains(t, seen, refString, "each canonical condition ref must appear exactly once")
		seen[refString] = struct{}{}
		outcomeCounts[testCase.outcome]++

		t.Run(refString, func(t *testing.T) {
			ctx := context.Background()
			data := plainFighter(ownerID)
			data.Conditions = []json.RawMessage{testCase.persisted}

			sheet, err := Load(ctx, data)
			require.NoError(t, err)
			require.False(t, sheet.IsDirty(), "strict load must not invent a persistence write")

			before := sheet.ToData()
			beforeByRef := persistedConditionBlobsByRef(t, before)
			require.Len(t, before.Conditions, 1, "the fixture must load as exactly one persisted condition")
			require.Len(t, beforeByRef, 1)
			require.Len(t, beforeByRef[testCase.expectedRef.String()], 1,
				"the strict loader must preserve the fixture's canonical ref exactly once")
			if testCase.outcome == attachedLongRestReset {
				require.True(t, persistedUsedThisTurn(t, beforeByRef[testCase.expectedRef.String()][0]),
					"reset fixtures must enter Attach with a seeded spent meter")
			}

			bus := events.NewEventBus()

			// Character.LongRest marks its own HP/resource writes dirty before it
			// publishes RestEvent. This first RestTopic observer creates a
			// checkpoint after those writes and before Attach registers condition
			// handlers. Any dirty flag after the publish must therefore come from
			// the keeper handling this condition's removal/state-change fact.
			_, err = dnd5eEvents.RestTopic.On(bus).Subscribe(ctx,
				func(_ context.Context, _ dnd5eEvents.RestEvent) error {
					sheet.dirty = false
					return nil
				})
			require.NoError(t, err)

			require.NoError(t, Attach(ctx, sheet, bus))
			cleaned := false
			t.Cleanup(func() {
				if !cleaned {
					require.NoError(t, sheet.Cleanup(ctx))
				}
			})

			require.False(t, sheet.IsDirty(), "Attach and its free reaction must not dirty a loaded sheet")
			require.Len(t, sheet.GetConditions(), 1,
				"a free opportunity attack must neither duplicate nor replace the persisted target condition")

			require.NoError(t, sheet.LongRest(ctx))
			if testCase.outcome == attachedLongRestRetain {
				require.False(t, sheet.IsDirty(),
					"an unchanged passive condition must publish no persistence fact after the checkpoint")
			} else {
				require.True(t, sheet.IsDirty(),
					"a reset or removal fact must make the sheet persistence-visible after the checkpoint")
			}

			after := sheet.ToData()
			afterByRef := persistedConditionBlobsByRef(t, after)

			switch testCase.outcome {
			case attachedLongRestRetain:
				require.Len(t, after.Conditions, 1,
					"a free reaction must not make a missing passive condition look retained")
				require.Len(t, afterByRef, 1)
				require.Len(t, afterByRef[testCase.expectedRef.String()], 1)
				require.JSONEq(t,
					string(beforeByRef[testCase.expectedRef.String()][0]),
					string(afterByRef[testCase.expectedRef.String()][0]),
					"a retained passive condition must preserve its serialized state")

			case attachedLongRestReset:
				require.Len(t, after.Conditions, 1,
					"the retained meter must be the only persisted condition, not a free duplicate")
				require.Len(t, afterByRef, 1)
				require.Len(t, afterByRef[testCase.expectedRef.String()], 1)
				require.False(t, persistedUsedThisTurn(t, afterByRef[testCase.expectedRef.String()][0]),
					"the seeded spent meter must persist as available after long rest")

			case attachedLongRestRemove:
				require.Empty(t, after.Conditions,
					"a runtime-only free reaction must not mask a temporary condition that failed to leave persistence")
				require.Empty(t, afterByRef)

			default:
				t.Fatalf("unknown attached long-rest outcome %q", testCase.outcome)
			}

			require.NoError(t, sheet.Cleanup(ctx))
			cleaned = true
		})
	}

	require.Len(t, seen, expectedCaseCount)
	require.Equal(t, expectedRetainCount, outcomeCounts[attachedLongRestRetain])
	require.Equal(t, expectedResetCount, outcomeCounts[attachedLongRestReset])
	require.Equal(t, expectedRemovalCount, outcomeCounts[attachedLongRestRemove])
}

func persistedConditionBlobsByRef(t *testing.T, data *Data) map[string][]json.RawMessage {
	t.Helper()

	byRef := make(map[string][]json.RawMessage, len(data.Conditions))
	for _, raw := range data.Conditions {
		var envelope struct {
			Ref core.Ref `json:"ref"`
		}
		require.NoError(t, json.Unmarshal(raw, &envelope))
		byRef[envelope.Ref.String()] = append(byRef[envelope.Ref.String()], raw)
	}

	return byRef
}

func persistedUsedThisTurn(t *testing.T, raw json.RawMessage) bool {
	t.Helper()

	var state struct {
		UsedThisTurn bool `json:"used_this_turn"`
	}
	require.NoError(t, json.Unmarshal(raw, &state))

	return state.UsedThisTurn
}
