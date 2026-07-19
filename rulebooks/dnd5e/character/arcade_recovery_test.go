// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// ArcadeRecoveryTestSuite exercises RestoreForNewEncounter (rpg-toolkit#785).
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
	s.False(RestoreForNewEncounter(nil))
}

func (s *ArcadeRecoveryTestSuite) TestAboveZeroHP_IsNoOp() {
	data := &Data{
		HitPoints:      5,
		MaxHitPoints:   20,
		DeathSaveState: &saves.DeathSaveState{Failures: 2},
		Conditions:     []json.RawMessage{unconsciousBlob(s.T(), "char-1", 2, false)},
	}
	before := *data

	restored := RestoreForNewEncounter(data)

	s.False(restored, "a character above 0 HP must not be touched by arcade recovery")
	s.Equal(before.HitPoints, data.HitPoints)
	s.Equal(before.DeathSaveState, data.DeathSaveState)
	s.Equal(before.Conditions, data.Conditions)
}

func (s *ArcadeRecoveryTestSuite) TestZeroHP_TPKDeath_RestoresFullHPAndClearsDeathState() {
	data := &Data{
		ID:             "char-1",
		HitPoints:      0,
		MaxHitPoints:   16,
		DeathSaveState: &saves.DeathSaveState{Failures: 3, Dead: true},
		Conditions:     []json.RawMessage{unconsciousBlob(s.T(), "char-1", 3, true)},
	}

	restored := RestoreForNewEncounter(data)

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

	restored := RestoreForNewEncounter(data)

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

	restored := RestoreForNewEncounter(data)

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
// HP -- not just that RestoreForNewEncounter mutated the struct in memory.
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

	s.Require().True(RestoreForNewEncounter(data))

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
