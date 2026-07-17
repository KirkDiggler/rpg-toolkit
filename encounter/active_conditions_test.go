package encounter_test

// active_conditions_test.go covers rpg-toolkit#754: Encounter.Data carries
// no projection of a held entity's active conditions, so a condition that
// was hydrated in via LoadFromData (rather than applied while a client was
// connected and watching the live StatusApplied/StatusRemoved stream) is
// mechanically real but invisible to a client that (re)connects mid-fight —
// found during the #752 fix (leaked rage added a damage component with no
// 🔥 badge anywhere).
//
// These tests deliberately call NO verb after LoadFromData — the scenario
// under test is a pure reconnect: a client that connects and asks for a
// snapshot before doing anything else. If ActiveConditions only appeared
// after some other action ran, that would prove the LIVE stream still
// works (already true, not the gap), not that the SNAPSHOT itself carries
// the condition.
//
// Also covers rpg-toolkit#778: ActiveConditions must exclude conditions
// attached permanently at construction (a Monk's MartialArts, a goblin's
// PackTactics) since those are structurally never announced on the live
// broker stream either — including them would make ActiveConditions a
// strict superset of what a continuously-connected client ever sees. See
// TestReconnect_MonsterTrait_ExcludedFromSnapshot and
// permanent_conditions_test.go's golden-list regression test.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type ActiveConditionsSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestActiveConditionsSuite(t *testing.T) {
	suite.Run(t, new(ActiveConditionsSuite))
}

func (s *ActiveConditionsSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *ActiveConditionsSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// acRagingBarbDataJSON builds a serialized dnd5e character.Data for a
// barbarian who is already raging — same shape as
// encounter_end_condition_sweep_test.go's ecsRagingBarbDataJSON, duplicated
// locally (rather than reused across files) since that fixture's ActionEconomy
// seeding is specific to #767's combat scenario and not needed here; this
// test never calls a combat verb.
func acRagingBarbDataJSON(t *testing.T, id, playerID string) json.RawMessage {
	t.Helper()
	ragingJSON, err := json.Marshal(conditions.RagingData{
		Ref:         refs.Conditions.Raging(),
		CharacterID: id,
		DamageBonus: 2,
		Level:       3,
		Source:      "dnd5e:features:rage",
	})
	require.NoError(t, err)
	data := &dnd5eCharacter.Data{
		ID:               id,
		PlayerID:         playerID,
		Name:             id,
		Level:            3,
		ProficiencyBonus: 2,
		ClassID:          classes.Barbarian,
		RaceID:           races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 12, abilities.CON: 15,
			abilities.INT: 8, abilities.WIS: 10, abilities.CHA: 10,
		},
		HitPoints:    16,
		MaxHitPoints: 16,
		ArmorClass:   14,
		Conditions:   []json.RawMessage{ragingJSON},
	}
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	return raw
}

// loadEncounterFromData round-trips a fresh Encounter through ToData/
// json-marshal/LoadFromData — the same "persist then reload" cycle a real
// reconnect goes through — WITHOUT calling any verb in between, so the
// returned Encounter's snapshot reflects exactly what a client asking for a
// snapshot immediately after connecting would see.
func (s *ActiveConditionsSuite) loadEncounterFromData(enc *encounter.Encounter) *encounter.Encounter {
	s.T().Helper()
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker)
	s.Require().NoError(err)
	return loaded
}

// TestReconnect_PlayerCondition_VisibleInSnapshot_NoVerbCalled is the primary
// goal-behavior proof: a barbarian who is raging (hydrated via DataJSON, the
// same shape rpg-api would feed back after an earlier session) has that
// condition appear in the SNAPSHOT the moment the encounter is loaded and
// ToData is called — before any action, before any live StatusApplied event
// could possibly have fired.
func (s *ActiveConditionsSuite) TestReconnect_PlayerCondition_VisibleInSnapshot_NoVerbCalled() {
	charJSON := acRagingBarbDataJSON(s.T(), "char-bob", "bob")
	enc := encounter.New(s.ctx, "enc-active-conditions", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: encountercore.Hex{}, SightRange: 10,
		HP: 16, MaxHP: 16, AC: 14,
		DataJSON: charJSON,
	}))

	loaded := s.loadEncounterFromData(enc)

	persisted := loaded.ToData()
	playerData := persisted.Players["bob"]
	s.Require().NotNil(playerData)
	s.Contains(playerData.ActiveConditions, "dnd5e:conditions:raging",
		"a condition hydrated in via LoadFromData must appear in the snapshot without any verb call")
}

// TestReconnect_MonkMartialArts_ExcludedFromSnapshot pins rpg-toolkit#778's
// motivating example directly: a Monk's MartialArts (a Grant.Conditions
// entry, attached once at Draft.Finalize — character creation, never
// through Encounter.ActivateFeature) must NOT appear in ActiveConditions.
// Contrast with TestReconnect_PlayerCondition_VisibleInSnapshot_NoVerbCalled
// above: Raging is genuinely runtime-activated (never in any class's
// Grant.Conditions list) and must still appear; MartialArts is
// Grant-attached and must not — the same snapshot, two condition refs,
// opposite outcomes, is the actual behavior rpg-toolkit#778 fixes.
func (s *ActiveConditionsSuite) TestReconnect_MonkMartialArts_ExcludedFromSnapshot() {
	martialArtsJSON, err := json.Marshal(conditions.MartialArtsData{
		Ref: refs.Conditions.MartialArts(), CharacterID: "char-finn", MonkLevel: 1,
	})
	s.Require().NoError(err)

	charData := &dnd5eCharacter.Data{
		ID: "char-finn", PlayerID: "finn", Name: "finn",
		Level: 1, ProficiencyBonus: 2,
		ClassID: classes.Monk, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12, abilities.DEX: 16, abilities.CON: 13,
			abilities.INT: 10, abilities.WIS: 14, abilities.CHA: 8,
		},
		HitPoints: 10, MaxHitPoints: 10, ArmorClass: 15,
		Conditions: []json.RawMessage{martialArtsJSON},
	}
	raw, err := json.Marshal(charData)
	s.Require().NoError(err)

	enc := encounter.New(s.ctx, "enc-active-conditions-monk", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "finn", EntityID: "char-finn",
		Position: encountercore.Hex{}, SightRange: 10,
		HP: 10, MaxHP: 10, AC: 15,
		DataJSON: raw,
	}))

	loaded := s.loadEncounterFromData(enc)
	persisted := loaded.ToData()
	playerData := persisted.Players["finn"]
	s.Require().NotNil(playerData)
	s.Nil(playerData.ActiveConditions,
		"MartialArts is Grant-attached (rpg-toolkit#778) — it must not appear in ActiveConditions, "+
			"the same way a live broker viewer would never see it announced")
}

// TestReconnect_PlayerNoConditions_ActiveConditionsOmitted is the negative
// case: a player with no conditions must have a nil (omitted) ActiveConditions,
// not an empty-but-present slice.
func (s *ActiveConditionsSuite) TestReconnect_PlayerNoConditions_ActiveConditionsOmitted() {
	charData := &dnd5eCharacter.Data{
		ID: "char-alice", PlayerID: "alice", Name: "alice",
		Level: 1, ProficiencyBonus: 2,
		ClassID: classes.Fighter, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 14, abilities.DEX: 12, abilities.CON: 13,
			abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 8,
		},
		HitPoints: 12, MaxHitPoints: 12, ArmorClass: 16,
	}
	raw, err := json.Marshal(charData)
	s.Require().NoError(err)

	enc := encounter.New(s.ctx, "enc-active-conditions-empty", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: encountercore.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 16,
		DataJSON: raw,
	}))

	loaded := s.loadEncounterFromData(enc)
	persisted := loaded.ToData()
	playerData := persisted.Players["alice"]
	s.Require().NotNil(playerData)
	s.Nil(playerData.ActiveConditions, "a character with no conditions must have ActiveConditions nil, not an empty slice")

	wire, err := json.Marshal(playerData)
	s.Require().NoError(err)
	s.NotContains(string(wire), "active_conditions",
		"empty ActiveConditions must be omitted from the wire (omitempty), not sent as []")
}

// TestReconnect_MonsterTrait_ExcludedFromSnapshot pins rpg-toolkit#778's
// fix for the sibling monster-side case: PackTactics is attached at monster
// construction (AddTraitData's pre-bus staging, unified into a genuine
// ConditionBehavior on the first LoadFromData cycle — mechanically
// identical to a character's Grant.Conditions), never through
// Encounter.ActivateFeature's live broker bridge, so it must NOT appear in
// ActiveConditions — otherwise every goblin would carry a permanent
// "PackTactics" badge on every snapshot forever, the same disease as a
// Monk's MartialArts leaking through (#778's motivating example).
//
// This test used to assert the OPPOSITE (PackTactics present) under #754's
// original unfiltered ActiveConditions — inverted here, not just relaxed,
// once #778 established that monster traits are structurally build-time
// -only, same as a character's Grant.Conditions entries.
func (s *ActiveConditionsSuite) TestReconnect_MonsterTrait_ExcludedFromSnapshot() {
	packTacticsJSON, err := json.Marshal(monstertraits.PackTacticsData{
		Ref:     refs.MonsterTraits.PackTactics(),
		OwnerID: "goblin-1",
	})
	s.Require().NoError(err)

	monData := &monster.Data{
		ID: "goblin-1", Name: "Goblin", HitPoints: 7, MaxHitPoints: 7, ArmorClass: 12,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8, abilities.DEX: 14, abilities.CON: 10,
			abilities.INT: 10, abilities.WIS: 8, abilities.CHA: 8,
		},
		Senses:     monster.SensesData{PassivePerception: 9},
		Conditions: []json.RawMessage{packTacticsJSON},
	}
	monJSON, err := json.Marshal(monData)
	s.Require().NoError(err)

	enc := encounter.New(s.ctx, "enc-active-conditions-monster", s.broker)
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-1", Position: encountercore.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 12,
		DataJSON: monJSON,
	}))

	loaded := s.loadEncounterFromData(enc)
	persisted := loaded.ToData()
	monsterData := persisted.Monsters["goblin-1"]
	s.Require().NotNil(monsterData)
	s.Nil(monsterData.ActiveConditions,
		"PackTactics is structurally build-time-only (rpg-toolkit#778) — it must not appear in ActiveConditions, "+
			"the same way a live broker viewer would never see it announced")
}
