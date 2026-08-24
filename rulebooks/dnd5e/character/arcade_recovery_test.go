// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	dnd5eResources "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// ArcadeRecoveryTestSuite exercises RestoreForLaunch (rpg-toolkit#785).
type ArcadeRecoveryTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *ArcadeRecoveryTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func TestArcadeRecoveryTestSuite(t *testing.T) {
	suite.Run(t, new(ArcadeRecoveryTestSuite))
}

// unconsciousBlob builds the raw JSON a dead/downed character would carry
// in Data.Conditions -- the exact shape conditions.UnconsciousCondition.ToJSON
// produces, matching what an encounter's ToData/DataJSON round-trip would
// have persisted after a TPK.
func unconsciousBlob(t *testing.T, characterID string, failures int, dead bool) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(conditions.UnconsciousData{
		Ref:         refs.Conditions.Unconscious(),
		CharacterID: characterID,
		Failures:    failures,
		Dead:        dead,
	})
	require.NoError(t, err)
	return raw
}

// conditionRef peeks a live condition's persisted Ref by round-tripping
// through its own ToJSON, mirroring encounter.activeConditionRefs' peek
// pattern rather than type-asserting against a concrete condition type.
func conditionRef(t *testing.T, cond interface {
	ToJSON() (json.RawMessage, error)
}) *core.Ref {
	t.Helper()
	raw, err := cond.ToJSON()
	require.NoError(t, err)
	var wire struct {
		Ref *core.Ref `json:"ref"`
	}
	require.NoError(t, json.Unmarshal(raw, &wire))
	return wire.Ref
}

func (s *ArcadeRecoveryTestSuite) TestNilData_NoPanic_ReturnsFalse() {
	s.False(RestoreForLaunch(nil))
}

// TestWounded_RestoredToFull: the #1225 ungate (Kirk's ruling,
// rpg-project#253) -- a character above 0 HP but below max is healed to
// full at launch, with any lingering death-save state and Unconscious blob
// removed. Before #1225 this exact input was the documented no-op case.
func (s *ArcadeRecoveryTestSuite) TestWounded_RestoredToFull() {
	data := &Data{
		HitPoints:      5,
		MaxHitPoints:   20,
		DeathSaveState: &saves.DeathSaveState{Failures: 2},
		Conditions:     []json.RawMessage{unconsciousBlob(s.T(), "char-1", 2, false)},
	}

	restored := RestoreForLaunch(data)

	s.True(restored, "a wounded character is restored at launch since #1225")
	s.Equal(20, data.HitPoints, "HP must restore to MaxHitPoints, not just for the downed")
	s.Nil(data.DeathSaveState)
	s.Empty(data.Conditions, "a lingering Unconscious blob is stripped regardless of HP")
}

// TestHealthy_LingeringUnconsciousBlob_IsStrippedAndReported: the strip
// runs unconditionally, so a coherent-looking record (full HP, no
// death-save state) that still carries an Unconscious blob from a broken
// earlier state is cleaned AND reported true — the caller must persist
// the strip, or the next hydration re-Applies the condition onto a
// healthy character.
func (s *ArcadeRecoveryTestSuite) TestHealthy_LingeringUnconsciousBlob_IsStrippedAndReported() {
	data := &Data{
		HitPoints:    20,
		MaxHitPoints: 20,
		Conditions:   []json.RawMessage{unconsciousBlob(s.T(), "char-1", 1, false)},
	}

	restored := RestoreForLaunch(data)

	s.True(restored, "a stripped blob alone must still tell the caller to persist")
	s.Equal(20, data.HitPoints)
	s.Empty(data.Conditions)
}

// TestFullyHealthy_IsNoOp: what remains of the old above-zero no-op after
// #1225 -- a character already at max HP with no death-save state, no
// Unconscious blob and full pools reports false, so the caller knows the
// record does not need persisting.
func (s *ArcadeRecoveryTestSuite) TestFullyHealthy_IsNoOp() {
	data := &Data{
		HitPoints:    20,
		MaxHitPoints: 20,
	}

	restored := RestoreForLaunch(data)

	s.False(restored, "nothing to restore, nothing to persist")
	s.Equal(20, data.HitPoints)
	s.Nil(data.DeathSaveState)
}

func (s *ArcadeRecoveryTestSuite) TestZeroHP_TPKDeath_RestoresFullHPAndClearsDeathState() {
	data := &Data{
		ID:             "char-1",
		HitPoints:      0,
		MaxHitPoints:   16,
		DeathSaveState: &saves.DeathSaveState{Failures: 3, Dead: true},
		Conditions:     []json.RawMessage{unconsciousBlob(s.T(), "char-1", 3, true)},
	}

	restored := RestoreForLaunch(data)

	s.True(restored)
	s.Equal(16, data.HitPoints, "HP must restore to MaxHitPoints")
	s.Nil(data.DeathSaveState, "death save state must be cleared")
	s.Empty(data.Conditions, "the Unconscious condition must be stripped so it never re-hydrates")
}

func (s *ArcadeRecoveryTestSuite) TestNegativeHP_TreatedSameAsZero() {
	// Defensive: nothing in the combat path is known to persist a negative
	// HitPoints today (ApplyDamage clamps at 0), but the gate should not
	// require exactly 0 to trigger a restore for an equally-broken record.
	data := &Data{HitPoints: -4, MaxHitPoints: 12}

	restored := RestoreForLaunch(data)

	s.True(restored)
	s.Equal(12, data.HitPoints)
}

func (s *ArcadeRecoveryTestSuite) TestPreservesOtherConditions() {
	ragingJSON, err := json.Marshal(conditions.RagingData{
		Ref:         refs.Conditions.Raging(),
		CharacterID: "char-1",
		DamageBonus: 2,
		Level:       3,
	})
	s.Require().NoError(err)

	data := &Data{
		HitPoints:    0,
		MaxHitPoints: 16,
		Conditions: []json.RawMessage{
			unconsciousBlob(s.T(), "char-1", 3, true),
			ragingJSON,
		},
	}

	restored := RestoreForLaunch(data)

	s.True(restored)
	s.Require().Len(data.Conditions, 1, "only the Unconscious condition should be stripped")

	var wire struct {
		Ref *core.Ref `json:"ref"`
	}
	s.Require().NoError(json.Unmarshal(data.Conditions[0], &wire))
	s.True(wire.Ref.Equals(refs.Conditions.Raging()), "the surviving condition must be Raging, not Unconscious")
}

// TestRoundTrip_HydratesCleanly proves the restored Data survives the exact
// ToData/LoadFromData shape a new encounter's hydration cascade exercises:
// marshal the restored Data, unmarshal it, LoadFromData it, and confirm the
// resulting live Character has no Unconscious condition subscribed and full
// HP -- not just that RestoreForLaunch mutated the struct in memory.
func (s *ArcadeRecoveryTestSuite) TestRoundTrip_HydratesCleanly() {
	data := &Data{
		ID:               "char-1",
		Name:             "Bob",
		Level:            3,
		ProficiencyBonus: 2,
		HitPoints:        0,
		MaxHitPoints:     16,
		ArmorClass:       14,
		DeathSaveState:   &saves.DeathSaveState{Failures: 3, Dead: true},
		Conditions:       []json.RawMessage{unconsciousBlob(s.T(), "char-1", 3, true)},
	}

	s.Require().True(RestoreForLaunch(data))

	raw, err := json.Marshal(data)
	s.Require().NoError(err)

	var reloaded Data
	s.Require().NoError(json.Unmarshal(raw, &reloaded))

	bus := events.NewEventBus()
	char, err := LoadFromData(s.ctx, &reloaded, bus)
	s.Require().NoError(err)

	s.Equal(16, char.GetHitPoints())
	s.Equal(&saves.DeathSaveState{}, char.GetDeathSaveState())
	for _, cond := range char.GetConditions() {
		ref := conditionRef(s.T(), cond)
		s.False(ref.Equals(refs.Conditions.Unconscious()),
			"a restored character must not re-hydrate with Unconscious applied")
	}
}

// ─── rpg-toolkit#795: resource pool restoration ────────────────────────────

// spentRageResources builds a Data.Resources map for a barbarian who has
// spent 2 of 3 rage charges -- the exact shape Character.ToData() persists
// after Rage.Activate consumes from the resource via UseResource
// (character.go:833).
func spentRageResources() map[coreResources.ResourceKey]RecoverableResourceData {
	return map[coreResources.ResourceKey]RecoverableResourceData{
		dnd5eResources.RageCharges: {
			Current: 1, Maximum: 3, ResetType: coreResources.ResetLongRest,
		},
	}
}

// TestDeadBarbarian_SpentRage_SeatedAlive_FullHPAndFullRage: rpg-toolkit#795's
// primary goal-behavior proof, combining #785's death recovery with #795's
// resource recovery in one seating -- a barbarian who died mid-rage (0 HP,
// death-save failures, rage down to 1/3) must come back whole on both axes,
// not just HP.
func (s *ArcadeRecoveryTestSuite) TestDeadBarbarian_SpentRage_SeatedAlive_FullHPAndFullRage() {
	data := &Data{
		ID:             "barb-1",
		HitPoints:      0,
		MaxHitPoints:   20,
		DeathSaveState: &saves.DeathSaveState{Failures: 3, Dead: true},
		Conditions:     []json.RawMessage{unconsciousBlob(s.T(), "barb-1", 3, true)},
		Resources:      spentRageResources(),
	}

	restored := RestoreForLaunch(data)

	s.True(restored)
	s.Equal(20, data.HitPoints, "HP must restore to MaxHitPoints")
	s.Nil(data.DeathSaveState, "death save state must be cleared")
	s.Empty(data.Conditions, "the Unconscious condition must be stripped")
	s.Equal(3, data.Resources[dnd5eResources.RageCharges].Current,
		"rage charges must refresh to their maximum alongside HP")
}

// TestAliveBarbarian_SpentRage_SeatedWithFullRageAndHP: an ALIVE barbarian
// who spent rage and took some hits earlier in the run gets both back at
// launch -- rage via #795's ungated pools, HP via #1225's ungated heal.
func (s *ArcadeRecoveryTestSuite) TestAliveBarbarian_SpentRage_SeatedWithFullRageAndHP() {
	data := &Data{
		ID:           "barb-2",
		HitPoints:    14,
		MaxHitPoints: 20,
		Resources:    spentRageResources(),
	}

	restored := RestoreForLaunch(data)

	s.True(restored)
	s.Equal(20, data.HitPoints, "since #1225 launch heals the wounded to full, not just the downed")
	s.Nil(data.DeathSaveState)
	s.Equal(3, data.Resources[dnd5eResources.RageCharges].Current,
		"rage charges must refresh alongside the heal")
}

// TestFullRage_AboveZeroHP_IsFullNoOp: the negative control for the negative
// control -- a character already at full HP AND full resources must report
// false and be left byte-for-byte untouched, so restoreForNewSeat's caller
// (encounter.go) correctly skips an unnecessary re-marshal.
func (s *ArcadeRecoveryTestSuite) TestFullRage_AboveZeroHP_IsFullNoOp() {
	data := &Data{
		HitPoints:    20,
		MaxHitPoints: 20,
		Resources: map[coreResources.ResourceKey]RecoverableResourceData{
			dnd5eResources.RageCharges: {Current: 3, Maximum: 3, ResetType: coreResources.ResetLongRest},
		},
	}
	// before is a real, independent snapshot, not a shallow struct copy --
	// *data alone would leave before.Resources pointing at the SAME map as
	// data.Resources, making the s.Equal below compare the map to itself
	// (trivially "equal" no matter what RestoreForLaunch does to it,
	// proving nothing about the no-op claim). Copilot catch on PR #801.
	before := *data
	before.Resources = maps.Clone(data.Resources)

	restored := RestoreForLaunch(data)

	s.False(restored, "a character already whole on both HP and resources must be a full no-op")
	s.Equal(before.HitPoints, data.HitPoints)
	s.Equal(before.Resources, data.Resources)
}

// TestMultipleResourcePools_AllRestoreIndependently: ki and hit dice
// restore the same way rage does -- proving the generic loop over
// d.Resources, not a rage-specific special case.
func (s *ArcadeRecoveryTestSuite) TestMultipleResourcePools_AllRestoreIndependently() {
	data := &Data{
		HitPoints:    10,
		MaxHitPoints: 10,
		Resources: map[coreResources.ResourceKey]RecoverableResourceData{
			dnd5eResources.RageCharges: {Current: 0, Maximum: 3, ResetType: coreResources.ResetLongRest},
			dnd5eResources.Ki:          {Current: 1, Maximum: 5, ResetType: coreResources.ResetShortRest},
			dnd5eResources.HitDice:     {Current: 2, Maximum: 6, ResetType: coreResources.ResetLongRest},
		},
	}

	restored := RestoreForLaunch(data)

	s.True(restored)
	s.Equal(3, data.Resources[dnd5eResources.RageCharges].Current)
	s.Equal(5, data.Resources[dnd5eResources.Ki].Current)
	s.Equal(6, data.Resources[dnd5eResources.HitDice].Current)
}
