package encounter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	monsteractions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	floorMaskDamageDice = "1d6"
	floorMaskActionType = "action"
	floorMaskAttackID   = "attack"
	floorMaskRefModule  = "dnd5e"
)

func scopedBoundsWithWall(t *testing.T, id core.EncounterID, broker *encounter.Broker) *encounter.Encounter {
	t.Helper()
	left := core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	right := core.HexFromPosition(spatial.Position{X: 1, Y: 0})
	enc := encounter.New(context.Background(), id, broker,
		encounter.WithStrikeResolver(alwaysHitResolver{damage: 4, damageType: damageSlashing}),
	)
	require.NoError(t, enc.InitDungeon(encounter.DungeonParams{
		Key: string(id), FloorSource: encounter.FloorSourceCanvas, Width: 2, Height: 1,
		PartyStart:    encounter.PartyStartParams{Anchor: &left, SeatCount: 1},
		AuthoredEdges: []encounter.AuthoredEdge{{From: left, To: right, Kind: encounter.GeneratedEdgeKindSolid}},
	}))
	return enc
}

func realGoblinJSON(t *testing.T, id core.EntityID) []byte {
	t.Helper()
	raw, err := json.Marshal(monsters.NewGoblin(string(id)).ToData())
	require.NoError(t, err)
	return raw
}

func realRangedMonsterJSON(t *testing.T, id core.EntityID) []byte {
	t.Helper()
	config, err := json.Marshal(monsteractions.RangedConfig{
		Name: "test-bow", AttackBonus: 4,
		Damage:      []damage.Damage{{Dice: floorMaskDamageDice, Type: damage.Piercing}},
		RangeNormal: 30, RangeLong: 120,
	})
	require.NoError(t, err)
	raw, err := json.Marshal(&monster.Data{
		ID: string(id), Name: "Ranged Test Monster", HitPoints: 7, MaxHitPoints: 7, ArmorClass: 12,
		Senses:  monster.SensesData{PassivePerception: 10},
		Actions: []monster.ActionData{{Ref: *refs.MonsterActions.Ranged(), Config: config}},
	})
	require.NoError(t, err)
	return raw
}

func TestBoundsCanvasPlayerAttackPreservesV03BoundaryBehavior(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()
	enc := scopedBoundsWithWall(t, "bounds-player-boundary", broker)
	left := core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	right := core.HexFromPosition(spatial.Position{X: 1, Y: 0})
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID, Position: left, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	require.NoError(t, enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: right, HP: 7, MaxHP: 7, AC: 12, Speed: 30,
		MonsterRef: monsterRefGoblin, DataJSON: realGoblinJSON(t, gobEntityID),
	}))
	require.NoError(t, enc.SetMode(core.ModeTurnBased))
	endTurnUntilActive(t, enc, aliceEntityID)
	require.NoError(t, enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: floorMaskRefModule, Type: floorMaskActionType, ID: floorMaskAttackID},
		encounter.ActionTarget{EntityID: gobEntityID},
	))
	require.Equal(t, 3, enc.ToData().Monsters[gobEntityID].HP,
		"Wave A must not add a bounds/wall LoS gate to the established player attack path")
}

func TestRoomChainPlayerAttackPreservesV03Behavior(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()
	enc := encounter.New(context.Background(), "room-chain-player", broker,
		encounter.WithStrikeResolver(alwaysHitResolver{damage: 4, damageType: damageSlashing}),
	)
	require.NoError(t, enc.InitRoom(2, 1, "empty"))
	left := core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	right := core.HexFromPosition(spatial.Position{X: 1, Y: 0})
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID, Position: left, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	require.NoError(t, enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: right, HP: 7, MaxHP: 7, AC: 12, Speed: 30,
		MonsterRef: monsterRefGoblin, DataJSON: realGoblinJSON(t, gobEntityID),
	}))
	endTurnUntilActive(t, enc, aliceEntityID)
	require.NoError(t, enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: floorMaskRefModule, Type: floorMaskActionType, ID: floorMaskAttackID},
		encounter.ActionTarget{EntityID: gobEntityID},
	))
	require.Equal(t, 3, enc.ToData().Monsters[gobEntityID].HP)
}

func TestBoundsCanvasNPCActionPreservesV03BoundaryBehavior(t *testing.T) {
	transport, broker := newRangeGateBroker()
	defer func() { _ = broker.Close(); _ = transport.Close() }()
	enc := scopedBoundsWithWall(t, "bounds-npc-boundary", broker)
	left := core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	right := core.HexFromPosition(spatial.Position{X: 1, Y: 0})
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID, Position: right, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14,
	}))
	require.NoError(t, enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: left, HP: 7, MaxHP: 7, AC: 12, Speed: 0,
		MonsterRef: monsterRefGoblin, DataJSON: realRangedMonsterJSON(t, gobEntityID),
	}))
	snapshot := enc.ToData()
	snapshot.Mode = core.ModeTurnBased
	snapshot.Initiative = []core.EntityID{gobEntityID, aliceEntityID}
	snapshot.ActiveIdx = 0
	snapshot.Round = 1
	loaded, err := encounter.LoadFromData(context.Background(), snapshot, broker,
		encounter.WithStrikeResolver(alwaysHitResolver{damage: 4, damageType: damageSlashing}),
	)
	require.NoError(t, err)
	require.NoError(t, loaded.NPCAct(context.Background(), gobEntityID))
	require.Less(t, loaded.ToData().Players[alicePlayerID].HP, 12,
		"real hydrated NPC action must retain bounds behavior; region-only mask gate must not suppress it")
}
