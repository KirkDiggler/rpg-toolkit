package encounter_test

// rpg-toolkit#772/#782 — TPK end-condition coverage beyond the solo-player
// cases already exercised in death_test.go
// (TestSlice_PlayerDeath_NotRePublishedOnReHit, flat-snapshot instant death)
// and unconscious_zero_hp_test.go (TestThreeFailedDeathSaves_BridgesToEntityDied
// / ..._AcrossPerRPCReloads_BridgesToEntityDied, hydrated 3-failed-saves
// death). This file covers:
//
//  1. Multi-player: the encounter must NOT end when one of several players
//     dies but at least one other is still alive, and must end (as "tpk")
//     only once the LAST living player is also confirmed dead.
//  2. Composition: a TPK-triggered end must run the same #752 condition
//     sweep and #767 ExitCombat/economy clear a victory-triggered end
//     already gets — checkEncounterEnd's terminal-transition code is
//     reason-agnostic, so this proves that holds for defeat too, not just
//     documents it.
//  3. Persistence: PlayerData.Dead and the resulting Mode/Initiative/Reason
//     state round-trip through JSON + a fresh LoadFromData exactly like any
//     other terminal-state field.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	tkevents "github.com/KirkDiggler/rpg-toolkit/events"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

const (
	tpkAliceEntityID = encountercore.EntityID("char-tpk-alice")
	tpkBobPlayerID   = encountercore.PlayerID("tpk-bob")
	tpkBobEntityID   = encountercore.EntityID("char-tpk-bob")
	tpkGoblinID      = encountercore.EntityID("tpk-goblin")
	tpkDamageDice    = "1d8+2"
)

// TPKSuite covers the multi-player and composition/persistence TPK cases.
type TPKSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestTPKSuite(t *testing.T) {
	suite.Run(t, new(TPKSuite))
}

func (s *TPKSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *TPKSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TestMultiPlayer_OneDeath_DoesNotEndEncounter_BothDeaths_EndsAsTPK is the
// two-player goal-behavior proof: alice and bob are both flat-snapshot
// seats (instant death on 0 HP, #733's non-hydrated fallback) facing one
// goblin. The goblin kills alice first — encounter continues (bob alive) —
// then, on its NEXT turn, closestPlayer retargets to bob (alice is now
// permanently excluded) and kills him too — only THEN must the encounter
// end, with Reason "tpk".
func (s *TPKSuite) TestMultiPlayer_OneDeath_DoesNotEndEncounter_BothDeaths_EndsAsTPK() {
	encID := encountercore.EncounterID("enc-tpk-multi")
	enc := encounter.New(s.ctx, encID, s.broker, encounter.WithRoller(fixedMaxRoller{}),
		encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}))

	// alice is closer to the goblin than bob, so closestPlayer targets her
	// first; once she's down (HP<=0, excluded), the goblin's next act must
	// retarget to bob.
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: tpkAliceEntityID,
		Position: encountercore.Hex{}, SightRange: 10,
		HP: 1, MaxHP: 12, AC: 10, AttackBonus: 4,
		DamageDice: tpkDamageDice, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: tpkBobEntityID,
		// Strictly farther from the goblin than alice's distance-1 spot, so
		// closestPlayer's tie-break (map iteration order, non-deterministic)
		// can never accidentally pick bob first.
		Position: encountercore.Hex{Q: 4, R: 0, S: -4}, SightRange: 10,
		HP: 1, MaxHP: 12, AC: 10, AttackBonus: 4,
		DamageDice: tpkDamageDice, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: tpkGoblinID, Position: encountercore.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	sub, err := s.broker.Subscribe(encID, "bob")
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	for enc.ActiveActor() != tpkGoblinID {
		_, _, endErr := enc.EndTurn(s.ctx, enc.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(sub, 100*time.Millisecond)

	// First goblin act: kills alice. Encounter must still be running.
	s.Require().NoError(enc.NPCAct(s.ctx, tpkGoblinID))
	s.Equal(encountercore.ModeTurnBased, enc.Mode(),
		"bob is still alive — alice's death alone must not end the encounter")
	s.True(enc.ToData().Players["alice"].Dead)
	s.False(enc.ToData().Players["bob"].Dead)

	for enc.ActiveActor() != tpkGoblinID {
		_, _, endErr := enc.EndTurn(s.ctx, enc.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(sub, 100*time.Millisecond)

	// Second goblin act: closestPlayer excludes downed alice, so this must
	// hit bob — the last living player. The encounter must now end as tpk.
	s.Require().NoError(enc.NPCAct(s.ctx, tpkGoblinID))
	s.Equal(encountercore.ModeEnded, enc.Mode())
	s.True(enc.ToData().Players["bob"].Dead)
	s.Empty(enc.ToData().Initiative)

	seen := collectEventsTyped(sub, 500*time.Millisecond)
	var ended *events.EncounterEndedEvent
	for _, e := range seen {
		if e2, ok := e.(*events.EncounterEndedEvent); ok {
			ended = e2
		}
	}
	s.Require().NotNil(ended, "the second, last-living-player death must publish EncounterEndedEvent")
	s.Equal(encounter.EncounterEndedReasonTPK, ended.Reason)
}

// TestTPK_RunsEndOfCombatSweep_ExitCombatClearsEconomy proves checkEncounterEnd's
// terminal-transition code (death.go) is exactly as reason-agnostic in
// practice as it is by construction: it still calls endCombatForPlayers
// (char.EndCombat + char.ExitCombat for every held player) on a TPK ending,
// not just a victory ending.
//
// This does NOT use raging as the probe, despite rpg-toolkit#752's victory-
// side test doing so (TestKillingBlow_SweepsRagingCondition_BeforeEncounterEnded,
// encounter_end_condition_sweep_test.go) — a real, load-bearing discovery
// made writing this test: RagingCondition.onConditionApplied (raging.go)
// unconditionally self-removes rage the instant Unconscious is applied to
// the same character (RAW-correct — you cannot keep raging while
// unconscious), which fires at the KNOCKDOWN hit, not at combat's end. Every
// hydrated character's HP-zero transition routes through Unconscious first
// (#733), so a raging character who is ABOUT to die always has rage
// stripped several turns before death, not at the TPK transition itself —
// there is structurally no way to reach checkEncounterEnd with rage still
// active on the player who is dying, whether the ending is a TPK or (per
// #752's own test, which relies on the ATTACKER being the raging one, not
// the target) a victory. Raging is also the ONLY condition in this
// rulebook that self-terminates on CombatEndTopic (grep confirms no other
// conditions/*.go file subscribes to it), so there is no substitute
// condition available to probe "is anything left to sweep" — #767's
// ExitCombat/economy-clear is the meaningful, actually-reachable half of
// the composition guarantee this test can verify for TPK specifically.
func (s *TPKSuite) TestTPK_RunsEndOfCombatSweep_ExitCombatClearsEconomy() {
	charJSON := ecsRagingBarbDataJSON(s.T(), string(tpkBobEntityID), string(tpkBobPlayerID), 1)

	roller := encounter.WithRoller(alwaysFailDeathSaveRoller{})
	resolver := encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing})

	enc := encounter.New(s.ctx, "enc-tpk-sweep", s.broker, roller, resolver)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: tpkBobPlayerID, EntityID: tpkBobEntityID,
		Position: encountercore.Hex{}, SightRange: 10,
		HP: 1, MaxHP: 16, AC: 14, AttackBonus: 5,
		DamageDice: tpkDamageDice, DamageType: damageSlashing,
		DataJSON: charJSON,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: tpkGoblinID, Position: encountercore.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 5, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker, roller, resolver)
	s.Require().NoError(err)
	s.Require().Contains(loaded.ToData().Players[tpkBobPlayerID].ActiveConditions, "dnd5e:conditions:raging",
		"setup sanity: bob must start raging")

	sub, err := s.broker.Subscribe("enc-tpk-sweep", tpkBobPlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	// bob and the goblin are in mutual LoS, so AddMonster (above, on enc)
	// already auto-transitioned to TURN_BASED; the Data round-trip inherits
	// that mode. Cycle to the goblin's turn.
	for loaded.ActiveActor() != tpkGoblinID {
		_, _, endErr := loaded.EndTurn(s.ctx, loaded.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(sub, 100*time.Millisecond)

	// Goblin's attack drops bob (1 HP) to 0 — Unconscious applied, which
	// (per this test's doc comment) already strips raging right here, well
	// before combat ends.
	s.Require().NoError(loaded.NPCAct(s.ctx, tpkGoblinID))
	drainSub(sub, 100*time.Millisecond)
	s.NotContains(loaded.ToData().Players[tpkBobPlayerID].ActiveConditions, "dnd5e:conditions:raging",
		"raging must already be gone at knockdown, before combat ends — confirms the premise this test's doc explains")

	// Cycle turns until bob's own turn-start has fired 3 times — 3
	// consecutive plain death-save failures (alwaysFailDeathSaveRoller never
	// rolls a success/crit) — the 3rd bridges to CharacterDiedTopic ->
	// publishPlayerDied -> Dead=true -> checkEncounterEnd (bob is solo, so
	// this is a TPK) inside that same EndTurn call.
	bobTurnStarts := 0
	active := loaded.ActiveActor()
	for i := 0; i < 20 && bobTurnStarts < 3; i++ {
		next, _, endErr := loaded.EndTurn(s.ctx, active)
		s.Require().NoError(endErr)
		active = next
		if active == tpkBobEntityID {
			bobTurnStarts++
		}
	}
	s.Require().Equal(3, bobTurnStarts, "must reach bob's own turn 3 times to accumulate 3 failures")

	evts := collectEventsTyped(sub, 500*time.Millisecond)
	var ended *events.EncounterEndedEvent
	for _, evt := range evts {
		if e, ok := evt.(*events.EncounterEndedEvent); ok {
			ended = e
		}
	}
	s.Require().NotNil(ended, "bob's confirmed death (solo player) must end the encounter")
	s.Equal(encounter.EncounterEndedReasonTPK, ended.Reason)

	// The persisted write-back must show bob marked Dead — this is the
	// actual persistence surface rpg-api reads back.
	persisted := loaded.ToData()
	playerData := persisted.Players[tpkBobPlayerID]
	s.Require().NotNil(playerData)
	s.True(playerData.Dead)
	var charData dnd5eCharacter.Data
	s.Require().NoError(json.Unmarshal(playerData.DataJSON, &charData))
	// Unconscious itself is NOT expected to be swept here — unlike raging,
	// it has no CombatEndTopic subscription (it only self-removes on
	// stabilize/revive/death, saves package) — so it legitimately survives
	// into the persisted data as the record of how bob died. Only raging
	// (already confirmed absent above, at knockdown time) is the sweep
	// target this test is about.
	s.Len(charData.Conditions, 1, "only the unconscious condition should remain — raging is already gone")

	// #767: ExitCombat must have cleared bob's ActionEconomy on the TPK end,
	// same as any other encounter-end — checkEncounterEnd's terminal
	// transition (Initiative/ActiveIdx/Round clearing, endCombatForPlayers)
	// is unconditional on WHICH reason fired. A raised/revived bob in a
	// future encounter (out of this wave's scope, but the persisted field
	// must be honest regardless) must not carry a stale, depleted economy
	// forward.
	s.Nil(charData.ActionEconomy, "ExitCombat must clear ActionEconomy on TPK end, same as a victory end")
}

// TestTPK_DeadFlagDoesNotLeakAcrossEncounters mirrors
// TestTwoEncounterLeak_CleanedCharacterNeverReappliesRaging
// (encounter_end_condition_sweep_test.go): take the character data that
// survives a TPK-ending encounter (post-sweep, clean), seat it fresh in an
// unrelated second encounter, and prove the new seat's PlayerData.Dead
// defaults to false — a character who died in encounter A and is later
// re-seated (narratively revived, or just re-added for a fresh fight/test)
// in encounter B is not carrying a stale Dead=true forward. Dead is never
// written into DataJSON (see PlayerData.Dead's doc, data.go) so this is
// true by construction, but the round-trip is exercised explicitly rather
// than left as an unverified claim.
func (s *TPKSuite) TestTPK_DeadFlagDoesNotLeakAcrossEncounters() {
	charJSON := ecsRagingBarbDataJSON(s.T(), string(tpkBobEntityID), string(tpkBobPlayerID), 1)

	roller := encounter.WithRoller(alwaysFailDeathSaveRoller{})
	resolver := encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing})

	encA := encounter.New(s.ctx, "enc-tpk-leak-a", s.broker, roller, resolver)
	s.Require().NoError(encA.AddPlayer(encounter.PlayerInput{
		PlayerID: tpkBobPlayerID, EntityID: tpkBobEntityID,
		Position: encountercore.Hex{}, SightRange: 10,
		HP: 1, MaxHP: 16, AC: 14, AttackBonus: 5,
		DamageDice: tpkDamageDice, DamageType: damageSlashing,
		DataJSON: charJSON,
	}))
	s.Require().NoError(encA.AddMonster(encounter.MonsterInput{
		ID: tpkGoblinID, Position: encountercore.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 5, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))
	raw, err := json.Marshal(encA.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loadedA, err := encounter.LoadFromData(s.ctx, &data, s.broker, roller, resolver)
	s.Require().NoError(err)

	for loadedA.ActiveActor() != tpkGoblinID {
		_, _, endErr := loadedA.EndTurn(s.ctx, loadedA.ActiveActor())
		s.Require().NoError(endErr)
	}
	s.Require().NoError(loadedA.NPCAct(s.ctx, tpkGoblinID))

	active := loadedA.ActiveActor()
	for i := 0; i < 20 && loadedA.Mode() != encountercore.ModeEnded; i++ {
		next, _, endErr := loadedA.EndTurn(s.ctx, active)
		s.Require().NoError(endErr)
		active = next
	}
	s.Require().Equal(encountercore.ModeEnded, loadedA.Mode(), "setup must reach TPK before proceeding")

	cleanedCharJSON := loadedA.ToData().Players[tpkBobPlayerID].DataJSON
	s.Require().NotEmpty(cleanedCharJSON)

	// --- Encounter B: a brand new encounter seeded with the SAME character
	// data rpg-api would have persisted from Encounter A's end. bob never
	// died in THIS encounter. ---
	encB := encounter.New(s.ctx, "enc-tpk-leak-b", s.broker)
	s.Require().NoError(encB.AddPlayer(encounter.PlayerInput{
		PlayerID: tpkBobPlayerID, EntityID: tpkBobEntityID,
		Position: encountercore.Hex{}, SightRange: 10,
		HP: 16, MaxHP: 16, AC: 14, AttackBonus: 5,
		DamageDice: tpkDamageDice, DamageType: damageSlashing,
		DataJSON: cleanedCharJSON,
	}))

	s.False(encB.ToData().Players[tpkBobPlayerID].Dead,
		"a fresh seat in an unrelated encounter must not inherit Dead=true from the character's prior death")

	// Same reasoning as TestTwoEncounterLeak_CleanedCharacterNeverReappliesRaging:
	// prove the damage chain has no raging component either, since the
	// character never activated rage here — the sweep from encounter A's
	// end must have actually cleaned the persisted data, not just this
	// flag.
	rawB, err := json.Marshal(encB.ToData())
	s.Require().NoError(err)
	var dataB encounter.Data
	s.Require().NoError(json.Unmarshal(rawB, &dataB))
	loadedB, err := encounter.LoadFromData(s.ctx, &dataB, s.broker)
	s.Require().NoError(err)

	weaponComp := dnd5eEvents.DamageComponent{
		Source: dnd5eEvents.DamageSourceWeapon, FlatBonus: 7,
		DamageType: damage.Slashing,
	}
	damageEvent := &dnd5eEvents.DamageChainEvent{
		AttackerID:   string(tpkBobEntityID),
		TargetID:     "some-other-goblin",
		Components:   []dnd5eEvents.DamageComponent{weaponComp},
		DamageType:   damage.Slashing,
		WeaponDamage: tpkDamageDice,
		AbilityUsed:  abilities.STR,
		IsMelee:      true,
	}
	chain := tkevents.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	damageTopic := dnd5eEvents.DamageChain.On(loadedB.EventBus())
	modifiedChain, err := damageTopic.PublishWithChain(s.ctx, damageEvent, chain)
	s.Require().NoError(err)
	final, err := modifiedChain.Execute(s.ctx, damageEvent)
	s.Require().NoError(err)

	for _, comp := range final.Components {
		s.NotEqual(dnd5eEvents.DamageSourceCondition, comp.Source,
			"no condition-sourced damage component should exist without an active condition")
	}
}
