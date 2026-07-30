package encounter_test

// range_gate_test.go is the integration regression gate for rpg-toolkit#864:
// a 2026-07-30 QA walk found no range/reach validation anywhere in the
// action-resolution path — Interact opened (and unlocked) doors from 10-16
// hexes away, and a 1-hex-reach melee weapon landed a hit from 3 hexes away.
//
// Each pair of tests below proves BOTH halves of the fix: the distant call
// is rejected with encounter.ErrOutOfRange (and produces no side effect —
// door stays closed/locked, no prompt issued, no attack resolves), and the
// adjacent/in-range call still succeeds exactly as before. Covers all three
// call sites the issue's spec names (Interact/door, player melee attack,
// monster attack) plus the Reach weapon property's 2-hex extension.

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	monsteractions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// aliceCharacterName is the display name on the hydrated-character fixtures
// below (hydrateWeaponWielder) — extracted to a constant to satisfy goconst.
const aliceCharacterName = "Alice"

func newRangeGateBroker() (*encounter.InMemoryTransport, *encounter.Broker) {
	transport := encounter.NewInMemoryTransport()
	return transport, encounter.NewBroker(transport)
}

// --- OpenDoor (Interact, unlocked door) -------------------------------------

func TestOpenDoor_DistantPlayer_Rejected(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	enc := encounter.New(context.Background(), "enc-rg-door-distant", broker)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 20,
	}))
	require.NoError(t, enc.AddDoor("door-1", core.Hex{Q: 10, R: 0, S: -10}, false))

	err := enc.OpenDoor(alicePlayerID, "door-1")
	require.Error(t, err)
	require.True(t, errors.Is(err, encounter.ErrOutOfRange), "got: %v", err)
	require.False(t, enc.ToData().Doors["door-1"].Open, "door must remain closed when the actor is out of reach")
}

func TestOpenDoor_AdjacentPlayer_Succeeds(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	enc := encounter.New(context.Background(), "enc-rg-door-adjacent", broker)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 20,
	}))
	require.NoError(t, enc.AddDoor("door-1", core.Hex{Q: 1, R: 0, S: -1}, false))

	err := enc.OpenDoor(alicePlayerID, "door-1")
	require.NoError(t, err)
	require.True(t, enc.ToData().Doors["door-1"].Open)
}

// --- AttemptUnlock (Interact, locked door) ----------------------------------

func TestAttemptUnlock_DistantPlayer_RejectedBeforePromptIssued(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	resolver := &fakeResolver{abilityMod: 3, abilityOK: true, toolBonus: 2, toolOK: true}
	enc := encounter.New(context.Background(), "enc-rg-unlock-distant", broker,
		encounter.WithCharacterResolver(resolver),
	)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 20,
	}))
	require.NoError(t, enc.AddDoor("door-1", core.Hex{Q: 16, R: 0, S: -16}, false))
	door := enc.ToData().Doors["door-1"]
	door.Locked = true
	door.LockDC = 12
	door.LockAbility = abilityDEX

	_, err := enc.AttemptUnlock(alicePlayerID, "door-1")
	require.Error(t, err)
	require.True(t, errors.Is(err, encounter.ErrOutOfRange), "got: %v", err)
	require.NotContains(t, enc.ToData().PendingPrompts, core.PlayerID(alicePlayerID),
		"the skill-check prompt must never be issued when the actor is out of reach")
}

func TestAttemptUnlock_AdjacentPlayer_IssuesPrompt(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	resolver := &fakeResolver{abilityMod: 3, abilityOK: true, toolBonus: 2, toolOK: true}
	enc := encounter.New(context.Background(), "enc-rg-unlock-adjacent", broker,
		encounter.WithCharacterResolver(resolver),
	)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 20,
	}))
	require.NoError(t, enc.AddDoor("door-1", core.Hex{Q: 1, R: 0, S: -1}, false))
	door := enc.ToData().Doors["door-1"]
	door.Locked = true
	door.LockDC = 12
	door.LockAbility = abilityDEX

	issued, err := enc.AttemptUnlock(alicePlayerID, "door-1")
	require.NoError(t, err)
	require.Equal(t, 12, issued.DC)
	require.Contains(t, enc.ToData().PendingPrompts, core.PlayerID(alicePlayerID))
}

// TestSubmitCheck_PlayerWalksAwayBeforeSubmitting_LockSurvives is the
// gate-review regression for blocker 2 (rpg-toolkit#864): dispatchPromptAction
// (prompts.go) used to set door.Locked = false unconditionally before
// calling the now-fallible OpenDoor, and nothing re-checked reach at
// SubmitCheck time. Exploit sequence: AttemptUnlock while adjacent (issues
// the prompt), walk away, SubmitCheck with a roll that clears the DC —
// Locked got cleared, OpenDoor then failed on distance (door left
// closed-but-unlocked), and walking back let a later OpenDoor open it for
// free — the DC check permanently bypassed.
//
// The fix re-checks reach in dispatchPromptAction before committing
// Locked = false. This test drives the exact walk-away sequence and proves
// the lock survives: the dispatch fails (wrapping ErrOutOfRange) while the
// check itself is still reported as having succeeded (Success: true — the
// roll was genuinely good, only the "open it right now" dispatch failed),
// the prompt is consumed (not left stranded, matching the existing
// downstream-OpenDoor-error contract on SubmitCheck), and — the part that
// actually matters — the door is STILL Locked afterward, so a fresh
// AttemptUnlock from back in reach is required rather than a free unlock.
func TestSubmitCheck_PlayerWalksAwayBeforeSubmitting_LockSurvives(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	resolver := &fakeResolver{abilityMod: 3, abilityOK: true, toolBonus: 2, toolOK: true}
	enc := encounter.New(context.Background(), "enc-rg-unlock-walkaway", broker,
		encounter.WithCharacterResolver(resolver),
	)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 20,
	}))
	require.NoError(t, enc.AddDoor("door-1", core.Hex{Q: 1, R: 0, S: -1}, false))
	door := enc.ToData().Doors["door-1"]
	door.Locked = true
	door.LockDC = 12
	door.LockAbility = abilityDEX

	// Adjacent: AttemptUnlock issues the prompt normally.
	_, err := enc.AttemptUnlock(alicePlayerID, "door-1")
	require.NoError(t, err)

	// Walk away before resolving the check.
	require.NoError(t, enc.Move(alicePlayerID, []core.Hex{
		{Q: 2, R: 0, S: -2}, {Q: 3, R: 0, S: -3}, {Q: 4, R: 0, S: -4}, {Q: 5, R: 0, S: -5},
	}))

	// roll(15) + abilityMod(3) = 18 (no LockTool set, so toolBonus doesn't
	// apply), comfortably clears DC 12.
	res, err := enc.SubmitCheck(alicePlayerID, 15)
	require.Error(t, err, "the dispatch must fail once the player has left reach")
	require.True(t, errors.Is(err, encounter.ErrOutOfRange), "got: %v", err)
	require.True(t, res.Success, "the roll itself still succeeded — only the dispatch (opening the door right now) failed")
	require.Equal(t, 18, res.Total)

	doorAfter := enc.ToData().Doors["door-1"]
	require.True(t, doorAfter.Locked,
		"the door must remain genuinely locked — this is the free-unlock-bypass gate finding")
	require.False(t, doorAfter.Open)
	require.NotContains(t, enc.ToData().PendingPrompts, core.PlayerID(alicePlayerID),
		"the consumed attempt's prompt must not be left stranded")

	// Walk back and confirm the lock genuinely still requires a fresh
	// check — not a free, silent bypass.
	require.NoError(t, enc.Move(alicePlayerID, []core.Hex{
		{Q: 4, R: 0, S: -4}, {Q: 3, R: 0, S: -3}, {Q: 2, R: 0, S: -2}, {Q: 1, R: 0, S: -1},
	}))
	_, err = enc.AttemptUnlock(alicePlayerID, "door-1")
	require.NoError(t, err, "back in reach, a fresh AttemptUnlock must work normally")
}

// --- TakeAction melee attack (player path) ----------------------------------

// endTurnUntilActive advances turns until actorID is active. Fixtures here
// place the actor first in a two-entity roster often enough that this is a
// no-op, but it guards the flip either way (mirrors combat_test.go's
// identical inline pattern).
func endTurnUntilActive(t *testing.T, enc *encounter.Encounter, actorID core.EntityID) {
	t.Helper()
	for enc.ActiveActor() != actorID {
		_, _, err := enc.EndTurn(context.Background(), enc.ActiveActor())
		require.NoError(t, err)
	}
}

func TestTakeAction_MeleeAttackBeyondReach_Rejected(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	enc := encounter.New(context.Background(), "enc-rg-melee-distant", broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 20,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	require.NoError(t, enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 3, R: 0, S: -3},
		HP: 7, MaxHP: 7, AC: 15,
	}))
	endTurnUntilActive(t, enc, aliceEntityID)

	err := enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	require.Error(t, err)
	require.True(t, errors.Is(err, encounter.ErrOutOfRange), "got: %v", err)
}

func TestTakeAction_MeleeAttackAtReach_Succeeds(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	enc := encounter.New(context.Background(), "enc-rg-melee-adjacent", broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 20,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	require.NoError(t, enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15,
	}))
	endTurnUntilActive(t, enc, aliceEntityID)

	err := enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	require.NoError(t, err)
}

// --- TakeAction melee attack with a Reach weapon (rpg-toolkit#864 spec:
// "melee attacks use weapon reach") ------------------------------------------

// hydrateWeaponWielder builds+round-trips (via LoadFromData, mirroring
// combat_test.go's hydration pattern) a player whose equipped main-hand
// weapon is weaponID, at the origin, against a monster at monsterPos.
// Returns the loaded encounter with alice active. Shared by the glaive
// (melee Reach) and shortbow (ranged) fixtures below — only the weapon and
// sight range vary between them.
func hydrateWeaponWielder(
	t *testing.T, broker *encounter.Broker, id string, weaponID string, sightRange int, monsterPos core.Hex,
) *encounter.Encounter {
	t.Helper()
	charData := &dnd5eCharacter.Data{
		ID: aliceEntityID, Name: aliceCharacterName, Level: 1, ProficiencyBonus: 2,
		HitPoints: 12, MaxHitPoints: 12,
		Inventory: []dnd5eCharacter.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: weaponID, Quantity: 1},
		},
		EquipmentSlots: dnd5eCharacter.EquipmentSlots{
			dnd5eCharacter.SlotMainHand: weaponID,
		},
	}
	charJSON, err := json.Marshal(charData)
	require.NoError(t, err)

	enc := encounter.New(context.Background(), core.EncounterID(id), broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: sightRange,
		HP: 12, MaxHP: 12,
		DataJSON: charJSON,
	}))
	require.NoError(t, enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: monsterPos,
		HP: 7, MaxHP: 7, AC: 15,
	}))

	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data encounter.Data
	require.NoError(t, json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(context.Background(), &data, broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)
	require.NoError(t, err)
	endTurnUntilActive(t, loaded, aliceEntityID)
	return loaded
}

func TestTakeAction_GlaiveReach_TwoHexes_Succeeds(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	loaded := hydrateWeaponWielder(t, broker, "enc-rg-glaive-ok", weapons.Glaive, 20, core.Hex{Q: 2, R: 0, S: -2})

	err := loaded.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	require.NoError(t, err, "the glaive's Reach property must extend melee reach to 2 hexes")
}

func TestTakeAction_GlaiveReach_ThreeHexes_Rejected(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	loaded := hydrateWeaponWielder(t, broker, "enc-rg-glaive-reject", weapons.Glaive, 20, core.Hex{Q: 3, R: 0, S: -3})

	err := loaded.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	require.Error(t, err)
	require.True(t, errors.Is(err, encounter.ErrOutOfRange), "got: %v", err)
}

// --- TakeAction ranged attack with a Shortbow (rpg-toolkit#864 spec:
// "ranged attacks use the weapon's range") -----------------------------------

// shortbowRangeLongHexes is weapons.Shortbow's long range (320ft) converted
// to hexes (5ft/hex) — mirrors checkAttackRange's own conversion.
const shortbowRangeLongHexes = 320 / 5

// shortbowSightRange comfortably covers shortbowRangeLongHexes±4 so
// AddMonster's mutual-LoS check auto-transitions to TURN_BASED in both the
// success and rejection fixtures below.
const shortbowSightRange = shortbowRangeLongHexes + 10

func TestTakeAction_ShortbowRange_WithinLongRange_Succeeds(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	loaded := hydrateWeaponWielder(t, broker, "enc-rg-shortbow-ok", weapons.Shortbow, shortbowSightRange,
		core.Hex{Q: shortbowRangeLongHexes - 4, R: 0, S: -(shortbowRangeLongHexes - 4)})

	err := loaded.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	require.NoError(t, err, "a target well within the shortbow's long range must not be gated as melee")
}

func TestTakeAction_ShortbowRange_BeyondLongRange_Rejected(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	loaded := hydrateWeaponWielder(t, broker, "enc-rg-shortbow-reject", weapons.Shortbow, shortbowSightRange,
		core.Hex{Q: shortbowRangeLongHexes + 4, R: 0, S: -(shortbowRangeLongHexes + 4)})

	err := loaded.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	require.Error(t, err)
	require.True(t, errors.Is(err, encounter.ErrOutOfRange), "got: %v", err)
}

// --- NPCAct scripted attack (monster path, no DataJSON) ---------------------

// TestNPCAct_ScriptedAttackBeyondReach_PassesTurnWithoutWedging is the
// gate-review regression for blocker 1: npcActScripted originally returned
// the ErrOutOfRange from the shared gate directly, and NPCAct propagated it
// as a hard failure. rpg-api's driveNPCChain only calls EndTurn after a
// SUCCESSFUL NPCAct, so an out-of-reach monster (the closest player simply
// too far away) would error on every single retry and the encounter would
// be wedged forever — turn never advances, nobody can act. The fix: an
// out-of-reach target is "nothing to do this turn" (same as the existing
// target==nil case), not an error — NPCAct succeeds, no attack resolves,
// and the turn ends normally afterward.
func TestNPCAct_ScriptedAttackBeyondReach_PassesTurnWithoutWedging(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	enc := encounter.New(context.Background(), "enc-rg-npc-distant", broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 20,
		HP: 12, MaxHP: 12, AC: 14,
	}))
	require.NoError(t, enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 3, R: 0, S: -3},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
		// No DataJSON: NPCAct falls back to npcActScripted, the "different
		// entry point" the issue's spec calls out — it must be gated too.
	}))
	endTurnUntilActive(t, enc, gobEntityID)

	err := enc.NPCAct(context.Background(), gobEntityID)
	require.NoError(t, err, "an out-of-reach scripted attack must pass the turn, not error")
	require.Equal(t, 12, enc.ToData().Players[alicePlayerID].HP, "no attack resolved, so no damage")

	// The turn must be free to end normally — proving the encounter isn't
	// wedged (the exact failure mode blocker 1 fixes).
	_, _, endErr := enc.EndTurn(context.Background(), gobEntityID)
	require.NoError(t, endErr)
	require.Equal(t, core.EntityID(aliceEntityID), enc.ActiveActor(),
		"turn must advance to alice, not stay stuck on the goblin")
}

func TestNPCAct_ScriptedAttackAtReach_Succeeds(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	enc := encounter.New(context.Background(), "enc-rg-npc-adjacent", broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 20,
		HP: 12, MaxHP: 12, AC: 14,
	}))
	require.NoError(t, enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))
	endTurnUntilActive(t, enc, gobEntityID)

	err := enc.NPCAct(context.Background(), gobEntityID)
	require.NoError(t, err)
}

// --- applyCapturedAttacks skip semantics (hydrated monster path) -----------

// TestNPCAct_HydratedMonster_OutOfReachCapturedAttack_SkipsWithoutWedging is
// the gate-review regression for blocker 1's other call site
// (applyCapturedAttacks, the hydrated-monster path — the scripted-fallback
// path above covers npcActScripted).
//
// Builds a monster whose own melee action reports an inflated reach (3, via
// a weapon name — "unmatched-claw" — that doesn't match anything in the
// weapons catalog, so the shared gate's meleeReachForCombatant can't
// resolve a real weapon and falls back to the 1-hex default). This models
// a real mismatch class: a monster whose configured reach doesn't line up
// with what the shared catalog-driven gate can independently verify (e.g.
// a bug in the monster's own action config, or any reach not backed by a
// cataloged weapon).
//
// The monster's speed (2) is deliberately too small to close the initial
// 4-hex gap to adjacency in one turn (moveTowardEnemy always tries to close
// to adjacency first) — after moving 2 hexes it's 2 hexes from the player:
// within its OWN inflated reach (3), so monster.TakeTurn selects and swings
// the melee action, but beyond the shared gate's independently-resolved
// reach (1) — applyCapturedAttacks must skip this captured attack rather
// than error and wedge the encounter.
func TestNPCAct_HydratedMonster_OutOfReachCapturedAttack_SkipsWithoutWedging(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	meleeCfg := monsteractions.MeleeConfig{
		Name: "unmatched-claw", AttackBonus: 4, DamageDice: "1d6",
		Reach: 3, DamageType: damage.Slashing,
	}
	cfgJSON, err := json.Marshal(meleeCfg)
	require.NoError(t, err)
	monData := &monster.Data{
		ID: gobEntityID, Name: "Test Monster", HitPoints: 7, MaxHitPoints: 7, ArmorClass: 15,
		Senses: monster.SensesData{PassivePerception: 10},
		Actions: []monster.ActionData{
			{Ref: *refs.MonsterActions.Melee(), Config: cfgJSON},
		},
	}
	monJSON, err := json.Marshal(monData)
	require.NoError(t, err)

	enc := encounter.New(context.Background(), "enc-rg-npc-hydrated-skip", broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 20,
		HP: 12, MaxHP: 12, AC: 14,
	}))
	require.NoError(t, enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 4, R: 0, S: -4},
		HP: 7, MaxHP: 7, AC: 15, Speed: 2,
		DataJSON: monJSON,
	}))
	endTurnUntilActive(t, enc, gobEntityID)

	err = enc.NPCAct(context.Background(), gobEntityID)
	require.NoError(t, err, "an out-of-reach captured attack must be skipped, not error/wedge the turn")
	require.Equal(t, 12, enc.ToData().Players[alicePlayerID].HP, "the skipped attack must deal no damage")

	_, _, endErr := enc.EndTurn(context.Background(), gobEntityID)
	require.NoError(t, endErr)
	require.Equal(t, core.EntityID(aliceEntityID), enc.ActiveActor(),
		"turn must advance to alice, not stay stuck on the goblin")
}
