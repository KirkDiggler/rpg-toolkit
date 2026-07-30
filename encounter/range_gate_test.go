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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

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

// hydrateGlaiveWielder builds+round-trips (via LoadFromData, mirroring
// combat_test.go's hydration pattern) a player whose equipped main-hand
// weapon is the Reach-property glaive, at pos, against a monster at
// monsterPos. Returns the loaded encounter with alice active.
func hydrateGlaiveWielder(t *testing.T, broker *encounter.Broker, id string, monsterPos core.Hex) *encounter.Encounter {
	t.Helper()
	charData := &dnd5eCharacter.Data{
		ID: aliceEntityID, Name: "Alice", Level: 1, ProficiencyBonus: 2,
		HitPoints: 12, MaxHitPoints: 12,
		Inventory: []dnd5eCharacter.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: weapons.Glaive, Quantity: 1},
		},
		EquipmentSlots: dnd5eCharacter.EquipmentSlots{
			dnd5eCharacter.SlotMainHand: weapons.Glaive,
		},
	}
	charJSON, err := json.Marshal(charData)
	require.NoError(t, err)

	enc := encounter.New(context.Background(), core.EncounterID(id), broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 20,
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

	loaded := hydrateGlaiveWielder(t, broker, "enc-rg-glaive-ok", core.Hex{Q: 2, R: 0, S: -2})

	err := loaded.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	require.NoError(t, err, "the glaive's Reach property must extend melee reach to 2 hexes")
}

func TestTakeAction_GlaiveReach_ThreeHexes_Rejected(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()

	loaded := hydrateGlaiveWielder(t, broker, "enc-rg-glaive-reject", core.Hex{Q: 3, R: 0, S: -3})

	err := loaded.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	require.Error(t, err)
	require.True(t, errors.Is(err, encounter.ErrOutOfRange), "got: %v", err)
}

// --- NPCAct scripted attack (monster path, no DataJSON) ---------------------

func TestNPCAct_ScriptedAttackBeyondReach_Rejected(t *testing.T) {
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
	require.Error(t, err)
	require.True(t, errors.Is(err, encounter.ErrOutOfRange), "got: %v", err)
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
