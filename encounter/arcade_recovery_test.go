package encounter_test

// rpg-toolkit#785 — arcade recovery goal-behavior proof: a character
// entering a NEW encounter at 0 HP (a post-TPK persisted record) is
// restored to full HP with clean death-save state, and does NOT re-hydrate
// with the Unconscious condition. A negative control proves the restore
// never fires on a per-RPC reload of an EXISTING seat within the SAME
// encounter — only AddPlayer's first-seating path may revive; resuming is
// not re-seating.
//
// rpg-toolkit#795 extends the same goal-behavior + negative-control shape
// to resource pools (rage charges, ki, hit dice): a new seating refreshes
// them regardless of HP, and a mid-encounter reload of an EXISTING seat
// must never touch them either — the same "resuming is not re-seating"
// contract, now for resources too.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	dnd5eResources "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

const (
	arcBobPlayerID = core.PlayerID("arc-bob")
	arcBobEntityID = core.EntityID("char-arc-bob")
	arcGoblinID    = core.EntityID("goblin-arc")
)

// ArcadeRecoverySuite exercises Encounter.AddPlayer's arcade-recovery
// wiring (rpg-toolkit#785) end to end, on the real cascade-hydrated
// encounter bus — not a bare character.RestoreForNewEncounter unit test
// (that lives in rulebooks/dnd5e/character/arcade_recovery_test.go).
type ArcadeRecoverySuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestArcadeRecoverySuite(t *testing.T) {
	suite.Run(t, new(ArcadeRecoverySuite))
}

func (s *ArcadeRecoverySuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *ArcadeRecoverySuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// postTPKCharDataJSON builds the exact shape a character record carries out
// of a TPK: 0 HP, DeathSaveState at 3 failures/Dead, and the Unconscious
// condition still embedded in Conditions — #753's end-of-combat sweep does
// NOT remove it (conditions.UnconsciousCondition never subscribes to
// CombatEndTopic), so this is what a real post-TPK ToData() snapshot looks
// like today, not an inflated synthetic worst case.
func (s *ArcadeRecoverySuite) postTPKCharDataJSON() json.RawMessage {
	s.T().Helper()
	unconsciousJSON, err := json.Marshal(conditions.UnconsciousData{
		Ref:         refs.Conditions.Unconscious(),
		CharacterID: string(arcBobEntityID),
		Failures:    3,
		Dead:        true,
	})
	s.Require().NoError(err)

	data := &dnd5eCharacter.Data{
		ID:               string(arcBobEntityID),
		PlayerID:         string(arcBobPlayerID),
		Name:             string(arcBobEntityID),
		Level:            3,
		ProficiencyBonus: 2,
		ClassID:          classes.Fighter,
		RaceID:           races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 14, abilities.DEX: 12, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 10,
		},
		HitPoints:      0,
		MaxHitPoints:   16,
		ArmorClass:     15,
		DeathSaveState: &saves.DeathSaveState{Failures: 3, Dead: true},
		Conditions:     []json.RawMessage{unconsciousJSON},
	}
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	return raw
}

// aliveArcBobCharDataJSON builds bob alive at 1 HP — mirrors
// turn_economy_downed_test.go's aliceAliveSeededCharDataJSON pattern, for
// the negative-control test below where bob must go down via REAL combat
// within an already-running encounter, not via a synthesized 0-HP seat.
func (s *ArcadeRecoverySuite) aliveArcBobCharDataJSON() json.RawMessage {
	s.T().Helper()
	data := &dnd5eCharacter.Data{
		ID:               string(arcBobEntityID),
		PlayerID:         string(arcBobPlayerID),
		Name:             string(arcBobEntityID),
		Level:            3,
		ProficiencyBonus: 2,
		ClassID:          classes.Fighter,
		RaceID:           races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 14, abilities.DEX: 12, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 10,
		},
		HitPoints:    1,
		MaxHitPoints: 16,
		ArmorClass:   15,
	}
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	return raw
}

// TestPostTPKCharacter_EntersNewEncounter_FullyRestored is the goal-behavior
// proof for #785: seat a post-TPK character record into a brand-new
// encounter, round-trip it exactly like rpg-api's per-RPC load (ToData ->
// marshal -> unmarshal -> LoadFromData), and confirm it comes back whole:
// full HP on both the encounter-level snapshot AND the hydrated character,
// no Unconscious condition, and able to act on their own turn.
func (s *ArcadeRecoverySuite) TestPostTPKCharacter_EntersNewEncounter_FullyRestored() {
	encID := core.EncounterID("enc-arc-new")
	resolver := encounter.WithCombatResolver(alwaysHitResolver{damage: 1, damageType: damageSlashing})

	enc := encounter.New(s.ctx, encID, s.broker, resolver)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: arcBobPlayerID, EntityID: arcBobEntityID,
		Position: core.Hex{}, SightRange: 10,
		// The caller (rpg-api) passes through whatever it last persisted —
		// per the issue's own live observation, that's the dead 0-HP
		// snapshot. AddPlayer, not the caller, is what must fix this.
		HP: 0, MaxHP: 16, AC: 15, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
		DataJSON: s.postTPKCharDataJSON(),
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: arcGoblinID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 5, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	// PlayerData.HP must already read the restored value straight off
	// AddPlayer — rpg-api reads ToData() before ever calling LoadFromData
	// again, so this snapshot has to be right immediately, not only after
	// a hydration cascade (rpg-toolkit#783/#784's two-representations trap).
	s.Equal(16, enc.ToData().Players[arcBobPlayerID].HP,
		"AddPlayer must sync the encounter-level HP snapshot to the restored value")

	// The exact per-RPC shape: marshal, unmarshal, LoadFromData.
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker, resolver)
	s.Require().NoError(err)

	s.Equal(16, loaded.ToData().Players[arcBobPlayerID].HP)

	for _, ref := range loaded.ToData().Players[arcBobPlayerID].ActiveConditions {
		s.NotEqual(refs.Conditions.Unconscious().String(), ref,
			"a restored character must not re-hydrate with Unconscious applied")
	}

	// Able to act: attack the goblin on bob's own turn. A player still
	// gated by a leftover Unconscious condition or a zeroed downed-actor
	// economy would fail this with ErrActionUnaffordable.
	for loaded.ActiveActor() != arcBobEntityID {
		_, _, endErr := loaded.EndTurn(s.ctx, loaded.ActiveActor())
		s.Require().NoError(endErr)
	}
	err = loaded.TakeAction(arcBobPlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: arcGoblinID},
	)
	s.Require().NoError(err, "a restored character must be able to act on their own turn")
}

// TestExistingSeat_MidEncounterReload_DoesNotHeal is the negative control —
// the trap this design has to avoid: a player who goes down via REAL combat
// WITHIN an already-running encounter must stay down across a per-RPC
// LoadFromData reload of that SAME encounter. The restore must never fire
// on resume, only on AddPlayer's first seating; the toolkit has no other
// signal to distinguish "brand new seat" from "same seat, next RPC" than
// which function was called. Mirrors
// TestDownedPlayerViaRealCombat_StaysZeroedAcrossMultipleTurns'
// (turn_economy_downed_test.go) real-combat-knockdown pattern.
func (s *ArcadeRecoverySuite) TestExistingSeat_MidEncounterReload_DoesNotHeal() {
	encID := core.EncounterID("enc-arc-resume")
	roller := encounter.WithRoller(alwaysFailDeathSaveRoller{})
	resolver := encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing})

	enc := encounter.New(s.ctx, encID, s.broker, roller, resolver)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: arcBobPlayerID, EntityID: arcBobEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 1, MaxHP: 16, AC: 15, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
		DataJSON: s.aliveArcBobCharDataJSON(),
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: arcGoblinID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 5, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker, roller, resolver)
	s.Require().NoError(err)

	for loaded.ActiveActor() != arcGoblinID {
		_, _, endErr := loaded.EndTurn(s.ctx, loaded.ActiveActor())
		s.Require().NoError(endErr)
	}
	// Real combat drops bob (1 HP) to 0 — applies Unconscious, matching
	// production, not a synthesized fixture.
	s.Require().NoError(loaded.NPCAct(s.ctx, arcGoblinID))

	s.Equal(0, loaded.ToData().Players[arcBobPlayerID].HP,
		"a real in-encounter knockdown must not be revived by this same encounter's own ToData")

	// Simulate rpg-api's per-RPC reload of the SAME encounter — this must
	// be indistinguishable from any other mid-encounter LoadFromData call,
	// and must NOT run AddPlayer's restore (AddPlayer is never called
	// again for bob's already-seated entry).
	rawReload, err := json.Marshal(loaded.ToData())
	s.Require().NoError(err)
	var reloadData encounter.Data
	s.Require().NoError(json.Unmarshal(rawReload, &reloadData))
	reloaded, err := encounter.LoadFromData(s.ctx, &reloadData, s.broker, roller, resolver)
	s.Require().NoError(err)

	s.Equal(0, reloaded.ToData().Players[arcBobPlayerID].HP,
		"a mid-encounter LoadFromData reload must never heal an existing seat")

	foundUnconscious := false
	for _, ref := range reloaded.ToData().Players[arcBobPlayerID].ActiveConditions {
		if ref == refs.Conditions.Unconscious().String() {
			foundUnconscious = true
		}
	}
	s.True(foundUnconscious,
		"the Unconscious condition must still be applied across a mid-encounter reload — resuming is not re-seating")
}

// ─── rpg-toolkit#795: resource pool restoration, end to end ────────────────

const arcBarbEntityID = core.EntityID("char-arc-barb")

// barbarianDataJSON builds a barbarian character record with rage charges
// spent down to 1 of 3, at the given HP -- the exact shape a persisted
// record carries after Rage.Activate consumed from Data.Resources
// mid-encounter, whether the character survived (alive, some HP left) or
// died still raging (0 HP, death-save state, Unconscious condition).
func (s *ArcadeRecoverySuite) barbarianDataJSON(hp int, dead bool) json.RawMessage {
	s.T().Helper()
	data := &dnd5eCharacter.Data{
		ID:               string(arcBarbEntityID),
		PlayerID:         string(arcBobPlayerID),
		Name:             string(arcBarbEntityID),
		Level:            3,
		ProficiencyBonus: 2,
		ClassID:          classes.Barbarian,
		RaceID:           races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 12, abilities.CON: 15,
			abilities.INT: 8, abilities.WIS: 10, abilities.CHA: 10,
		},
		HitPoints:    hp,
		MaxHitPoints: 20,
		ArmorClass:   14,
		Resources: map[coreResources.ResourceKey]dnd5eCharacter.RecoverableResourceData{
			dnd5eResources.RageCharges: {
				Current: 1, Maximum: 3, ResetType: coreResources.ResetLongRest,
			},
		},
	}
	if dead {
		data.DeathSaveState = &saves.DeathSaveState{Failures: 3, Dead: true}
		data.Conditions = []json.RawMessage{
			mustMarshal(s.T(), conditions.UnconsciousData{
				Ref: refs.Conditions.Unconscious(), CharacterID: string(arcBarbEntityID),
				Failures: 3, Dead: true,
			}),
		}
	}
	return mustMarshal(s.T(), data)
}

func mustMarshal(t interface {
	Helper()
	Errorf(string, ...any)
}, v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		t.Errorf("marshal: %v", err)
	}
	return raw
}

// rageChargesCurrent unmarshals a seated player's DataJSON and returns its
// RageCharges Resources entry's Current value, failing the test if the
// entry is missing entirely (a missing entry is itself a bug worth failing
// loudly on, not silently treating as "0 charges").
func (s *ArcadeRecoverySuite) rageChargesCurrent(dataJSON json.RawMessage) int {
	s.T().Helper()
	var data dnd5eCharacter.Data
	s.Require().NoError(json.Unmarshal(dataJSON, &data))
	res, ok := data.Resources[dnd5eResources.RageCharges]
	s.Require().True(ok, "RageCharges must round-trip through DataJSON")
	return res.Current
}

// TestAliveBarbarian_SpentRage_NewSeating_RageRestoredHPUntouched is #795's
// goal-behavior proof for the scope change over #785: an ALIVE barbarian
// (above 0 HP) seated into a brand-new encounter gets rage charges back
// even though nothing about their HP needed fixing.
func (s *ArcadeRecoverySuite) TestAliveBarbarian_SpentRage_NewSeating_RageRestoredHPUntouched() {
	encID := core.EncounterID("enc-arc-rage-alive")
	enc := encounter.New(s.ctx, encID, s.broker)

	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: arcBobPlayerID, EntityID: arcBarbEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 14, MaxHP: 20, AC: 14,
		DataJSON: s.barbarianDataJSON(14, false),
	}))

	s.Equal(14, enc.ToData().Players[arcBobPlayerID].HP,
		"an alive character's encounter-level HP snapshot must not change at seating")
	s.Equal(3, s.rageChargesCurrent(enc.ToData().Players[arcBobPlayerID].DataJSON),
		"rage charges must refresh to their maximum even though the character was never down")
}

// TestDeadBarbarian_SpentRage_NewSeating_FullHPAndFullRage combines #785's
// HP recovery with #795's resource recovery in one seating -- a barbarian
// who died mid-rage comes back whole on both axes.
func (s *ArcadeRecoverySuite) TestDeadBarbarian_SpentRage_NewSeating_FullHPAndFullRage() {
	encID := core.EncounterID("enc-arc-rage-dead")
	enc := encounter.New(s.ctx, encID, s.broker)

	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: arcBobPlayerID, EntityID: arcBarbEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 0, MaxHP: 20, AC: 14,
		DataJSON: s.barbarianDataJSON(0, true),
	}))

	s.Equal(20, enc.ToData().Players[arcBobPlayerID].HP, "HP must restore alongside rage")
	s.Equal(3, s.rageChargesCurrent(enc.ToData().Players[arcBobPlayerID].DataJSON))
}

// TestExistingSeat_MidEncounterReload_DoesNotRestoreRage is the resource
// counterpart to TestExistingSeat_MidEncounterReload_DoesNotHeal: an alive
// barbarian seated with spent rage must stay spent across a mid-encounter
// LoadFromData reload of that SAME seat — resuming is not re-seating, for
// resources exactly as much as for HP.
func (s *ArcadeRecoverySuite) TestExistingSeat_MidEncounterReload_DoesNotRestoreRage() {
	encID := core.EncounterID("enc-arc-rage-resume")
	enc := encounter.New(s.ctx, encID, s.broker)

	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: arcBobPlayerID, EntityID: arcBarbEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 14, MaxHP: 20, AC: 14,
		DataJSON: s.barbarianDataJSON(14, false),
	}))
	// AddPlayer's own restore already refreshed rage to 3/3 (proven by
	// TestAliveBarbarian_SpentRage_NewSeating_RageRestoredHPUntouched
	// above) -- overwrite the persisted DataJSON with a freshly-spent 1/3
	// blob before reloading, standing in for whatever real mid-encounter
	// Rage.Activate calls would have done. The reload must see THIS spent
	// state and leave it alone, exactly like rpg-api's per-RPC load would.
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	data.Players[arcBobPlayerID].DataJSON = s.barbarianDataJSON(14, false)

	reloaded, err := encounter.LoadFromData(s.ctx, &data, s.broker)
	s.Require().NoError(err)

	s.Equal(1, s.rageChargesCurrent(reloaded.ToData().Players[arcBobPlayerID].DataJSON),
		"a mid-encounter LoadFromData reload must never refresh an existing seat's resources")
}
