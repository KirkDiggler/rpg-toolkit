package encounter_test

// rpg-toolkit#752 regression coverage.
//
// Before this fix, checkEncounterEnd (death.go) transitioned the encounter
// to ModeEnded and published EncounterEndedEvent without touching any held
// player's conditions. A raging character whose encounter ended via the
// killing blow itself (rather than "no combat activity this turn") carried
// RagingCondition all the way to ToData() with the condition still present
// in the persisted character.Data.Conditions — and the next encounter's
// LoadFromData cascade silently re-Applied it, granting the rage damage
// bonus to a character who never activated rage and had no charges left.
//
// These tests exercise the fix on the real cascade-hydrated encounter bus
// (LoadFromData), not a bare condition-level unit test (that lives in
// conditions/raging_test.go): a barbarian enters the encounter already
// raging (DataJSON carries the condition, as it would mid-encounter after
// an earlier ActivateFeature call), lands the killing blow that ends the
// encounter, and the fix must (a) emit a broker StatusRemoved for raging
// before EncounterEndedEvent and (b) leave the persisted character data
// clean so a second, independent encounter never re-applies it.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	tkevents "github.com/KirkDiggler/rpg-toolkit/events"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

const (
	ecsEncounterID = encountercore.EncounterID("enc-condition-sweep")
	ecsDamageDice  = "1d12"
)

// EncounterEndConditionSweepSuite exercises the checkEncounterEnd condition
// sweep end to end (rpg-toolkit#752).
type EncounterEndConditionSweepSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestEncounterEndConditionSweepSuite(t *testing.T) {
	suite.Run(t, new(EncounterEndConditionSweepSuite))
}

func (s *EncounterEndConditionSweepSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *EncounterEndConditionSweepSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// ecsRagingBarbDataJSON builds a serialized dnd5e character.Data for a
// barbarian who is ALREADY raging — Conditions carries the raging condition
// blob, matching what an earlier in-encounter ActivateFeature("rage") call
// would have produced and rpg-api would have fed back into this seat's
// DataJSON. hp sets current HitPoints only (MaxHitPoints is always 16) —
// callers that need the barbarian to be the one going DOWN (tpk_test.go's
// TPK composition case, as opposed to this file's barb-kills-goblin case)
// pass a low value instead of the full 16.
//
// Shared (not duplicated per-file) deliberately, unlike most fixture
// helpers in this package (see turn_economy_downed_test.go's "no cross-
// suite coupling" precedent) — golangci-lint's dupl check flagged the
// near-identical copy this file and tpk_test.go each carried before this,
// and a raging-barbarian character.Data builder has no meaningful per-file
// variation beyond starting HP, which the parameter now covers.
func ecsRagingBarbDataJSON(t *testing.T, id, playerID string, hp int) json.RawMessage {
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
		HitPoints:    hp,
		MaxHitPoints: 16,
		ArmorClass:   14,
		Conditions:   []json.RawMessage{ragingJSON},
		ActionEconomy: &dnd5eCharacter.ActionEconomyData{
			TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
			ReactionsRemaining: 1, MovementRemaining: 30,
		},
	}
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	return raw
}

// loadRagingBarbVsGoblin builds a fresh encounter with one already-raging
// barbarian seat and one 1-HP goblin, then round-trips through
// ToData/LoadFromData so the cascade hydrates the barbarian's character
// (and its raging condition) onto the held bus exactly once — mirroring
// condition_removed_bridge_test.go's loadEncounter pattern. The goblin's 1
// HP guarantees the fixedMaxRoller-driven attack lands the killing blow that
// ends the encounter.
func (s *EncounterEndConditionSweepSuite) loadRagingBarbVsGoblin(charJSON json.RawMessage) *encounter.Encounter {
	s.T().Helper()
	enc := encounter.New(s.ctx, ecsEncounterID, s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
		encounter.WithStrikeResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: bobPlayerID, EntityID: bobEntityID,
		Position: encountercore.Hex{}, SightRange: 10,
		HP: 16, MaxHP: 16, AC: 14, AttackBonus: 5,
		DamageDice: ecsDamageDice, DamageType: damageSlashing,
		DataJSON: charJSON,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID:       gobEntityID,
		Position: encountercore.Hex{Q: 1, R: 0, S: -1},
		HP:       1, MaxHP: 7, AC: 5, Speed: 6,
		MonsterRef:  monsterRefGoblin,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
		DataJSON: testGoblinDataJSON(s.T(), gobEntityID),
	}))

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
		encounter.WithStrikeResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}))
	s.Require().NoError(err)

	// bob and the goblin are in mutual LoS, so AddMonster (above, on enc)
	// already auto-transitioned to TURN_BASED before the Data round-trip;
	// loaded inherits that mode. An explicit SetMode here would be
	// redundant and error. bob's DataJSON carries a pre-seeded
	// ActionEconomy (simulating a barbarian already mid-encounter), so this
	// fixture — unlike TestTakeAction_HydratedPlayerBypassesFlatSnapshotGate
	// — doesn't depend on SetMode's seedActorTurn call to populate economy.
	for i := 0; loaded.ActiveActor() != encountercore.EntityID(bobEntityID) && i < 4; i++ {
		_, _, err := loaded.EndTurn(s.ctx, loaded.ActiveActor())
		s.Require().NoError(err)
	}
	s.Require().Equal(encountercore.EntityID(bobEntityID), loaded.ActiveActor(), "setup must land on the barbarian's turn")
	return loaded
}

// TestKillingBlow_SweepsRagingCondition_BeforeEncounterEnded is the primary
// goal-behavior proof: a barbarian who is raging when their own attack ends
// the encounter (last hostile defeated) must have rage swept — a broker
// StatusRemoved (ConditionRemovedEvent) fires for "raging" with reason
// "combat_ended", sequenced BEFORE EncounterEndedEvent, and the persisted
// character data (ToData) no longer carries the condition.
func (s *EncounterEndConditionSweepSuite) TestKillingBlow_SweepsRagingCondition_BeforeEncounterEnded() {
	charJSON := ecsRagingBarbDataJSON(s.T(), bobEntityID, string(bobPlayerID), 16)
	enc := s.loadRagingBarbVsGoblin(charJSON)

	sub, err := s.broker.Subscribe(ecsEncounterID, bobPlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	s.Require().NoError(enc.TakeAction(bobPlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: gobEntityID},
	))

	evts := drainEvents(sub, time.Second)

	removed := findConditionRemoved(evts, encountercore.EntityID(bobEntityID))
	s.Require().NotNil(removed, "raging must be swept (StatusRemoved) when the killing blow ends the encounter")
	s.Equal("raging", removed.ConditionRef)
	s.Equal("combat_ended", removed.Reason)

	var ended *events.EncounterEndedEvent
	for _, evt := range evts {
		if e, ok := evt.(*events.EncounterEndedEvent); ok {
			ended = e
		}
	}
	s.Require().NotNil(ended, "encounter must end once the lone goblin dies")
	s.Less(removed.Sequence(), ended.Sequence(),
		"the StatusRemoved for raging must be sequenced before the terminal EncounterEndedEvent")

	// The persisted write-back (ToData) must no longer carry raging — this
	// is the actual leak vector: rpg-api reads PlayerData.DataJSON here and
	// persists it as the character's post-encounter state.
	persisted := enc.ToData()
	playerData := persisted.Players[bobPlayerID]
	s.Require().NotNil(playerData)
	var charData dnd5eCharacter.Data
	s.Require().NoError(json.Unmarshal(playerData.DataJSON, &charData))
	s.Empty(charData.Conditions, "raging must not survive into the persisted character data")
}

// TestTwoEncounterLeak_CleanedCharacterNeverReappliesRaging reproduces the
// rpg-toolkit#752 playtest evidence directly: take the character data that
// survives Encounter A's end (post-fix, clean), hydrate it fresh into an
// unrelated Encounter B, and prove the damage chain cannot include raging —
// because nothing Applied a RagingCondition onto Encounter B's bus, the same
// way the playtest's Encounter B fight would have.
func (s *EncounterEndConditionSweepSuite) TestTwoEncounterLeak_CleanedCharacterNeverReappliesRaging() {
	// --- Encounter A: rage active, killing blow ends the encounter ---
	charJSON := ecsRagingBarbDataJSON(s.T(), bobEntityID, string(bobPlayerID), 16)
	encA := s.loadRagingBarbVsGoblin(charJSON)
	s.Require().NoError(encA.TakeAction(bobPlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: gobEntityID},
	))
	cleanedCharJSON := encA.ToData().Players[bobPlayerID].DataJSON
	s.Require().NotEmpty(cleanedCharJSON)

	// --- Encounter B: a brand new encounter, seeded with the SAME character
	// data rpg-api would have persisted from Encounter A's end. Bob never
	// activated rage here and has no rage charges left, matching the
	// playtest's "no rage uses remaining ... yet his hit resolves
	// [... raging:2]" evidence. ---
	encB := encounter.New(s.ctx, "enc-condition-sweep-b", s.broker)
	s.Require().NoError(encB.AddPlayer(encounter.PlayerInput{
		PlayerID: bobPlayerID, EntityID: bobEntityID,
		Position: encountercore.Hex{}, SightRange: 10,
		HP: 16, MaxHP: 16, AC: 14, AttackBonus: 5,
		DamageDice: ecsDamageDice, DamageType: damageSlashing,
		DataJSON: cleanedCharJSON,
	}))
	raw, err := json.Marshal(encB.ToData())
	s.Require().NoError(err)
	var dataB encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &dataB))
	loadedB, err := encounter.LoadFromData(s.ctx, &dataB, s.broker)
	s.Require().NoError(err)

	// Publish a raw DamageChainEvent for bob's attack directly on Encounter
	// B's live bus, mirroring conditions/raging_test.go's own
	// executeDamageChain technique — this is the actual mechanism the
	// playtest's damage breakdown surfaced the leak through. If raging had
	// re-hydrated (the bug), RagingCondition.onDamageChain would append a
	// DamageSourceCondition component with SourceRef raging.
	weaponComp := dnd5eEvents.DamageComponent{
		Source: dnd5eEvents.DamageSourceWeapon, FlatBonus: 7,
		DamageType: damage.Slashing,
	}
	abilityComp := dnd5eEvents.DamageComponent{
		Source: dnd5eEvents.DamageSourceAbility, FlatBonus: 3,
		DamageType: damage.Slashing,
	}
	damageEvent := dnd5eEvents.NewDamageChainEvent(dnd5eEvents.DamageChainInput{
		AttackerID:       bobEntityID,
		TargetID:         "some-other-goblin",
		Components:       []dnd5eEvents.DamageComponent{weaponComp, abilityComp},
		WeaponDamageDice: ecsDamageDice,
		WeaponDamageType: damage.Slashing,
		AbilityUsed:      abilities.STR,
		IsMelee:          true,
	})
	chain := tkevents.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(loadedB.EventBus())
	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, chain)
	s.Require().NoError(err)
	final, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	s.Len(final.Components, 2,
		"damage chain must NOT include a raging component — bob never activated rage in this encounter")
	for _, comp := range final.Components {
		s.NotEqual(dnd5eEvents.DamageSourceCondition, comp.Source,
			"no condition-sourced damage component should exist without an active condition")
	}
}
