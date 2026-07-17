package encounter_test

// exit_combat_test.go covers rpg-toolkit#767: checkEncounterEnd's
// endCombatForPlayers sweep now also calls character.Character.ExitCombat
// for every held player, clearing ActionEconomy back to nil. Before this
// fix, a character who finished a fight carried their depleted combat state
// (e.g. movement_remaining: 0) into every SUBSEQUENT encounter forever,
// because ActionEconomy is a flat, encounter-unscoped field that ToData()/
// LoadFromData round-trip verbatim.
//
// Adds a test method to EncounterEndConditionSweepSuite (defined in
// encounter_end_condition_sweep_test.go, #752) rather than building a
// parallel fixture: that suite's loadRagingBarbVsGoblin already seeds a
// LIVE ActionEconomy on the barbarian (simulating "mid-turn when the
// encounter ends") alongside an active raging condition, and already
// exercises the exact "killing blow ends the encounter" path this fix
// touches. Testing both #752's condition sweep and #767's economy clear
// through the SAME fixture at the SAME call site (endCombatForPlayers) is
// the natural regression guard: it proves adding the new ExitCombat call
// alongside the existing EndCombat call didn't disturb either.

import (
	"encoding/json"

	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// TestKillingBlow_ClearsActionEconomy_AndStillSweepsRagingCondition is the
// primary goal-behavior proof for #767, plus the #752 regression guard, in
// one assertion pair: a barbarian who is BOTH raging AND mid-turn (live
// ActionEconomy) when their own attack ends the encounter must come out of
// ToData() with ActionEconomy == nil (this fix) AND Conditions empty
// (#752, unregressed).
func (s *EncounterEndConditionSweepSuite) TestKillingBlow_ClearsActionEconomy_AndStillSweepsRagingCondition() {
	charJSON := ecsRagingBarbDataJSON(s.T(), bobEntityID, string(bobPlayerID))
	enc := s.loadRagingBarbVsGoblin(charJSON)

	s.Require().NoError(enc.TakeAction(bobPlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: gobEntityID},
	))

	persisted := enc.ToData()
	playerData := persisted.Players[bobPlayerID]
	s.Require().NotNil(playerData)
	var charData dnd5eCharacter.Data
	s.Require().NoError(json.Unmarshal(playerData.DataJSON, &charData))

	s.Nil(charData.ActionEconomy,
		"ActionEconomy must be cleared back to nil when the encounter ends (rpg-toolkit#767) — "+
			"otherwise the character carries depleted combat state into every subsequent encounter")
	s.Empty(charData.Conditions,
		"raging must still be swept (rpg-toolkit#752) — adding ExitCombat must not regress EndCombat")
}

// TestKillingBlow_ExitCombat_IdempotentOnNonHeldSeats: a PLAYER seat with no
// hydrated *character.Character (a flat stat-snapshot seat — no DataJSON,
// so heldCharacter returns nil for it) must not cause endCombatForPlayers
// to error just because ExitCombat now runs alongside EndCombat in the same
// loop. This is the same "flat stat-snapshot seats are skipped" guarantee
// #752 already relied on; re-asserted here because the new ExitCombat call
// is a second call site added to that same skip-on-nil branch.
func (s *EncounterEndConditionSweepSuite) TestKillingBlow_ExitCombat_IdempotentOnNonHeldSeats() {
	// bob carries no DataJSON — a flat stat-snapshot seat, not a held
	// character — so heldCharacter(bob) is nil and both EndCombat/ExitCombat
	// must be skipped without error.
	enc := encounter.New(s.ctx, "enc-exit-combat-flat-seat", s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
		encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: bobPlayerID, EntityID: bobEntityID,
		Position: encountercore.Hex{}, SightRange: 10,
		HP: 16, MaxHP: 16, AC: 14, AttackBonus: 5,
		DamageDice: ecsDamageDice, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID:       gobEntityID,
		Position: encountercore.Hex{Q: 1, R: 0, S: -1},
		HP:       1, MaxHP: 7, AC: 5,
		MonsterRef:  monsterRefGoblin,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))
	// The killing blow triggers checkEncounterEnd -> endCombatForPlayers,
	// which now calls ExitCombat alongside EndCombat for every held player.
	// bob has no held character (heldCharacter returns nil), so both calls
	// must be skipped without error — this assertion IS the test.
	s.Require().NoError(enc.TakeAction(bobPlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: gobEntityID},
	))
}
