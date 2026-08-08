package encounter_test

// dungeon_completion_test.go is the end-to-end regression-evidence gate for
// rpg-toolkit#817 (wave 2, "The Dungeon" — whole-dungeon completion): killing
// the last hostile ANYWHERE in an N-region InitDungeon (#814) encounter ends
// the encounter in victory; clearing a non-final combat pocket (#794/#796)
// exits non-terminally to FREE_ROAM while hostiles remain elsewhere; a TPK
// (#772/#782) ends with the canonical defeat reason regardless of how many
// hostiles are still alive in unexplored regions; and the resulting
// ModeEnded/terminal state survives a ToData/LoadFromData round trip to the
// extent the toolkit persists it.
//
// #817's "Approved Slice 3 corrections" comment settled the open defeat
// question: the existing EncounterEndedReasonTPK declaration (verified in a
// real production playtest) is preserved as-is — this file does not invent
// or duplicate defeat detection. It also does not re-derive the victory
// predicate: checkEncounterEnd's `len(e.data.Monsters) == 0` (death.go) is
// already whole-dungeon-scoped by construction (Monsters is one flat map
// spanning every region/pocket, not scoped per pocket), and
// checkPocketCleared's FREE_ROAM exit (combat.go) already runs strictly
// AFTER checkEncounterEnd, only when the encounter is still ModeTurnBased.
// What was missing before this file is END-TO-END evidence that this
// already-generic pair of rules actually holds against the InitDungeon
// generator specifically, with more than one non-final combat pocket — every
// existing test exercising the pocket-clear/whole-dungeon-end lifecycle
// (combat_pockets_test.go) used a hand-built single-door room, and every
// existing InitDungeon test (dungeon_test.go, #814) only covers
// geometry/validation, never AddMonster/TakeAction/NPCAct.
//
// Fixture: reuses dungeon_test.go's shared 3-region (entrance/corridor/boss)
// row/column helpers and ID constants (dungeonHeight, dungeonEntranceWidth,
// dungeonCorridorWidth, dungeonBossWidth, dungeonRegionID*, dungeonDoor0ID,
// dungeonDoor1ID, dungeonRegionNearEdgeHex, dungeonRegionFarEdgeHex) so the
// region/door geometry here is provably the same shape #814 already proved
// out — but swaps every region's Pattern to PatternEmpty (dungeon_test.go
// itself uses PatternRandom for entrance/boss) so line-of-sight and movement
// across each pocket are deterministic and this file's assertions are about
// the encounter lifecycle, not about wall-generation randomness (already
// covered by dungeon_test.go's own DungeonSuite).
//
// Movement convention: every hex in this fixture sits on the shared middle
// row (X varies, Y fixed at dungeonHeight/2) — dungeonRowHex(x) below is the
// same offset->cube conversion dungeon_test.go's own Near/FarEdgeHex helpers
// use, generalized to an arbitrary global X.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	dcSeed = 424242

	dcEntranceGoblinID = core.EntityID("dc-goblin-entrance")
	dcCorridorGoblinID = core.EntityID("dc-goblin-corridor")
	dcBossGoblinID     = core.EntityID("dc-goblin-boss")

	// dcSightRange comfortably covers the full 27-hex span of the fixture
	// (entrance width 10 + corridor width 5 + boss width 10, plus the 2
	// door columns) so every LoS assertion in this file is gated purely by
	// closed doors / open doors, never by sight-range distance.
	dcSightRange = 60
)

// dcDungeonParams builds the same entrance/corridor/boss region shape and
// door IDs as dungeon_test.go's threeRegionDungeonParams, but with
// PatternEmpty for every region (instead of PatternRandom for entrance/boss)
// so this file's LoS/movement assertions are deterministic regardless of
// wall-generation randomness — that randomness is dungeon_test.go's concern,
// not this file's.
func dcDungeonParams(seed int64) encounter.DungeonParams {
	return encounter.DungeonParams{
		Height:     dungeonHeight,
		RandomSeed: seed,
		Theme:      dungeonThemeCrypt,
		Regions: []encounter.DungeonRegionParams{
			{
				ID: dungeonRegionIDEntrance, Archetype: encounter.ArchetypeEntrance,
				Width: dungeonEntranceWidth, Pattern: environments.PatternEmpty,
			},
			{
				ID: dungeonRegionIDCorridor, Archetype: encounter.ArchetypeCorridor,
				Width: dungeonCorridorWidth, Pattern: environments.PatternEmpty,
			},
			{
				ID: dungeonRegionIDBoss, Archetype: encounter.ArchetypeBoss,
				Width: dungeonBossWidth, Pattern: environments.PatternEmpty,
			},
		},
		Connectors: []encounter.DungeonConnectorParams{
			{DoorID: dungeonDoor0ID},
			{DoorID: dungeonDoor1ID},
		},
	}
}

// dcRowHex returns the global hex at column x on this fixture's shared
// middle row — the same offset->cube conversion dungeon_test.go's
// dungeonRegionNearEdgeHex/dungeonRegionFarEdgeHex use, generalized to an
// arbitrary column rather than just each region's two boundary columns.
func dcRowHex(x int) core.Hex {
	return core.HexFromPosition(spatial.Position{X: float64(x), Y: float64(dungeonHeight / 2)})
}

// dcPathTo returns every hex strictly after fromX up to and including toX on
// the shared middle row — a movement path Move() can walk one column at a
// time. A toX less than fromX (right-to-left movement) is never exercised by
// this file — movement is always rightward, entrance->corridor->boss.
func dcPathTo(fromX, toX int) []core.Hex {
	path := make([]core.Hex, 0, toX-fromX)
	for x := fromX + 1; x <= toX; x++ {
		path = append(path, dcRowHex(x))
	}
	return path
}

// dcEndTurnUntilActive cycles EndTurn (no action taken) until entityID is
// the active actor, bounded so a bug that never lands on entityID fails the
// test instead of hanging — same pattern combat_pockets_test.go and
// tpk_test.go already use.
func dcEndTurnUntilActive(ctx context.Context, s *suite.Suite, enc *encounter.Encounter, entityID core.EntityID) {
	for i := 0; enc.ActiveActor() != entityID && i < 8; i++ {
		_, _, err := enc.EndTurn(ctx, enc.ActiveActor())
		s.Require().NoError(err)
	}
	s.Require().Equal(entityID, enc.ActiveActor(), "setup must be able to reach this actor's turn")
}

// DungeonCompletionSuite covers rpg-toolkit#817's done bar end-to-end
// against the real InitDungeon generator (#814), not a hand-built room.
type DungeonCompletionSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestDungeonCompletionSuite(t *testing.T) {
	suite.Run(t, new(DungeonCompletionSuite))
}

func (s *DungeonCompletionSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *DungeonCompletionSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TestClearingEarlyPockets_ThenVictoryOnLastHostile_ReloadSafe walks the
// full done-bar sequence from #817's issue body: clear chambers 1..N-1 (here
// N=3: entrance and corridor) -> each pocket exits to FREE_ROAM with the
// encounter still live and hostiles remaining elsewhere -> kill the boss
// (the last hostile anywhere) -> the encounter ends with the canonical
// all-hostiles-defeated reason -> a ToData/LoadFromData reload lands the
// encounter in a completed, endable state rather than a resumed fight.
func (s *DungeonCompletionSuite) TestClearingEarlyPockets_ThenVictoryOnLastHostile_ReloadSafe() {
	roller := encounter.WithRoller(fixedMaxRoller{})
	resolver := encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing})
	enc := encounter.New(s.ctx, "enc-dungeon-completion-victory", s.broker, roller, resolver)

	s.Require().NoError(enc.InitDungeon(dcDungeonParams(dcSeed)))
	entrance := enc.ToData().Space.Entrance
	s.Require().Equal(dungeonRegionNearEdgeHex(0), entrance,
		"fixture premise: player spawns at the entrance region's near edge")

	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: entrance, SightRange: dcSightRange,
		HP: 20, MaxHP: 20, AC: 14, AttackBonus: 5,
		DamageDice: dice1d6, DamageType: damageSlashing,
	}))

	// Boss and corridor goblins are seeded BEFORE the entrance goblin,
	// mirroring combat_pockets_test.go's fixture-order comment: adding them
	// first means their own AddMonster calls trigger no combat entry (both
	// closed doors block LoS to them), and the entrance goblin's AddMonster
	// call is what actually exercises rollInitiative's LoS-scoping to just
	// the entrance pocket.
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: dcBossGoblinID, Position: dungeonRegionFarEdgeHex(2),
		HP: 1, MaxHP: 7, AC: 5, Speed: 6, MonsterRef: monsterRefGoblin,
		DataJSON: testGoblinDataJSON(s.T(), dcBossGoblinID),
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: dcCorridorGoblinID, Position: dungeonRegionFarEdgeHex(1),
		HP: 1, MaxHP: 7, AC: 5, Speed: 6, MonsterRef: monsterRefGoblin,
		DataJSON: testGoblinDataJSON(s.T(), dcCorridorGoblinID),
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: dcEntranceGoblinID, Position: dungeonRegionFarEdgeHex(0),
		HP: 1, MaxHP: 7, AC: 5, Speed: 6, MonsterRef: monsterRefGoblin,
		DataJSON: testGoblinDataJSON(s.T(), dcEntranceGoblinID),
	}))

	s.Require().Equal(core.ModeTurnBased, enc.Mode(), "sighting the entrance goblin must auto-start the first pocket")

	// --- Pocket 1 (entrance): clear it. Corridor + boss goblins remain,
	// both still hidden behind closed doors -- clearing must exit to
	// FREE_ROAM, not end the encounter. ---
	sub, err := s.broker.Subscribe("enc-dungeon-completion-victory", alicePlayerID)
	s.Require().NoError(err)

	dcEndTurnUntilActive(s.ctx, &s.Suite, enc, aliceEntityID)
	// rpg-toolkit#864: TakeAction now gates melee attacks on reach — alice
	// spawns at the entrance's near edge (X=0), 9 hexes from the goblin at
	// the far edge (X=9), so she must walk up to X=8 (adjacent) first. A
	// stat-snapshot player (no DataJSON) isn't gated on movement budget
	// (Move's tracksMovement check requires a held character), so this is a
	// free repositioning, not a turn-economy change.
	s.Require().NoError(enc.Move(alicePlayerID, dcPathTo(0, 8)))
	s.Require().NoError(enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: dcEntranceGoblinID},
	))

	s.Equal(core.ModeFreeRoam, enc.Mode(),
		"clearing the entrance pocket with hostiles remaining elsewhere must exit to FREE_ROAM, not end the encounter")
	s.Empty(enc.ToData().Initiative)
	s.Contains(enc.ToData().Monsters, dcCorridorGoblinID, "corridor goblin must still be alive")
	s.Contains(enc.ToData().Monsters, dcBossGoblinID, "boss goblin must still be alive")

	seen := collectTypes(sub, 500*time.Millisecond)
	s.Contains(seen, "*events.ModeChangedEvent")
	s.NotContains(seen, "*events.EncounterEndedEvent", "clearing one of several pockets must not fire the terminal event")
	_ = sub.Close()

	// --- Pocket 2 (corridor): open door 0, move into the corridor to gain
	// LoS on the corridor goblin (boss goblin stays hidden behind the still-
	// closed door 1), clear it. Boss goblin remains -- must exit to
	// FREE_ROAM again, still not end the encounter. ---
	// rpg-toolkit#864: OpenDoor requires adjacency too — alice is at X=8
	// (from the entrance-goblin attack above), door0 sits at X=10, so she
	// steps up to X=9 first. The subsequent move into the corridor now
	// starts from X=9 (her real position), not X=0.
	s.Require().NoError(enc.Move(alicePlayerID, dcPathTo(8, 9)))
	s.Require().NoError(enc.OpenDoor(alicePlayerID, dungeonDoor0ID))
	s.Require().NoError(enc.Move(alicePlayerID, dcPathTo(9, dungeonRegionStarts()[1])))
	s.Require().Equal(core.ModeTurnBased, enc.Mode(), "moving into LoS of the corridor goblin must start a fresh pocket")
	s.True(func() bool {
		for _, id := range enc.ToData().Initiative {
			if id == dcCorridorGoblinID {
				return true
			}
		}
		return false
	}(), "the fresh pocket must be scoped to the corridor goblin")

	sub2, err := s.broker.Subscribe("enc-dungeon-completion-victory", alicePlayerID)
	s.Require().NoError(err)

	dcEndTurnUntilActive(s.ctx, &s.Suite, enc, aliceEntityID)
	// rpg-toolkit#864: alice is at the corridor's near edge (X=11) after the
	// move above; the corridor goblin sits at the far edge (X=15), so she
	// steps up to X=14 (adjacent) before attacking.
	s.Require().NoError(enc.Move(alicePlayerID, dcPathTo(dungeonRegionStarts()[1], 14)))
	s.Require().NoError(enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: dcCorridorGoblinID},
	))

	s.Equal(core.ModeFreeRoam, enc.Mode(),
		"clearing the corridor pocket with the boss still alive must exit to FREE_ROAM again, not end the encounter")
	s.NotContains(enc.ToData().Monsters, dcCorridorGoblinID)
	s.Contains(enc.ToData().Monsters, dcBossGoblinID, "boss goblin must still be alive and untouched")

	seen2 := collectTypes(sub2, 500*time.Millisecond)
	s.NotContains(seen2, "*events.EncounterEndedEvent",
		"clearing the second-to-last pocket must still not fire the terminal event")
	_ = sub2.Close()

	// --- Final pocket (boss): open door 1, move into the boss region to
	// gain LoS on the boss goblin -- the LAST hostile anywhere -- and kill
	// it. This must end the whole dungeon in victory. ---
	// rpg-toolkit#864: alice is at X=14 (from the corridor-goblin attack
	// above); door1 sits at X=16, so she steps up to X=15 (adjacent) first.
	s.Require().NoError(enc.Move(alicePlayerID, dcPathTo(14, 15)))
	s.Require().NoError(enc.OpenDoor(alicePlayerID, dungeonDoor1ID))
	s.Require().NoError(enc.Move(alicePlayerID, dcPathTo(15, dungeonRegionStarts()[2])))
	s.Require().Equal(core.ModeTurnBased, enc.Mode(), "moving into LoS of the boss goblin must start the final pocket")

	sub3, err := s.broker.Subscribe("enc-dungeon-completion-victory", alicePlayerID)
	s.Require().NoError(err)

	dcEndTurnUntilActive(s.ctx, &s.Suite, enc, aliceEntityID)
	// rpg-toolkit#864: alice is at the boss region's near edge (X=17); the
	// boss goblin sits at the far edge (X=26), so she steps up to X=25
	// (adjacent) before attacking.
	s.Require().NoError(enc.Move(alicePlayerID, dcPathTo(dungeonRegionStarts()[2], 25)))
	s.Require().NoError(enc.TakeAction(alicePlayerID,
		encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		encounter.ActionTarget{EntityID: dcBossGoblinID},
	))

	s.Equal(core.ModeEnded, enc.Mode(), "killing the last hostile anywhere in the dungeon must end the whole encounter")
	s.Empty(enc.ToData().Monsters, "every hostile must be gone")

	var ended *events.EncounterEndedEvent
	for _, evt := range collectEventsTyped(sub3, 500*time.Millisecond) {
		if e, ok := evt.(*events.EncounterEndedEvent); ok {
			ended = e
		}
	}
	s.Require().NotNil(ended, "the whole-dungeon clear must publish EncounterEndedEvent")
	s.Equal(encounter.EncounterEndedReasonAllHostilesDefeated, ended.Reason,
		"the whole-dungeon clear must use the canonical victory/all-hostiles-defeated reason, not a new one")
	_ = sub3.Close()

	s.dcAssertReloadSafe(enc)
}

// TestTPKMidDungeon_EndsWithCanonicalReason_HostilesRemainElsewhere proves
// #817's Approved Slice 3 corrections directly: a TPK ends the encounter
// with the existing, canonical EncounterEndedReasonTPK — composed correctly
// with the N-region/combat-pocket machinery, i.e. it fires regardless of how
// many hostiles are still alive and unengaged elsewhere in the dungeon (the
// corridor and boss goblins here are never even sighted), proving TPK is NOT
// gated on hostile-clearing the way victory is, and does not fire the
// victory reason instead.
func (s *DungeonCompletionSuite) TestTPKMidDungeon_EndsWithCanonicalReason_HostilesRemainElsewhere() {
	roller := encounter.WithRoller(fixedMaxRoller{})
	resolver := encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing})
	enc := encounter.New(s.ctx, "enc-dungeon-completion-tpk", s.broker, roller, resolver)

	s.Require().NoError(enc.InitDungeon(dcDungeonParams(dcSeed)))
	entrance := enc.ToData().Space.Entrance

	// Solo, flat-snapshot (no DataJSON) seat at 1 HP -- the same instant-
	// death fallback tpk_test.go's own multi-player test uses -- so a single
	// hit from the entrance goblin confirms death immediately, no death-save
	// sequence needed.
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: entrance, SightRange: dcSightRange,
		HP: 1, MaxHP: 20, AC: 14, AttackBonus: 5,
		DamageDice: dice1d6, DamageType: damageSlashing,
	}))

	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: dcBossGoblinID, Position: dungeonRegionFarEdgeHex(2),
		HP: 7, MaxHP: 7, AC: 5, Speed: 6, MonsterRef: monsterRefGoblin,
		DataJSON: testGoblinDataJSON(s.T(), dcBossGoblinID),
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: dcCorridorGoblinID, Position: dungeonRegionFarEdgeHex(1),
		HP: 7, MaxHP: 7, AC: 5, Speed: 6, MonsterRef: monsterRefGoblin,
		DataJSON: testGoblinDataJSON(s.T(), dcCorridorGoblinID),
	}))
	// rpg-toolkit#864: NPCAct's scripted fallback (no DataJSON — this
	// goblin's shape) now gates melee attacks on reach and has no movement
	// logic of its own, so the goblin spawns adjacent to alice (X=1, vs.
	// alice's entrance at X=0) rather than at the far edge (X=9) — nothing
	// else in this test depends on the goblin's exact starting column, only
	// that it's within LoS (dcSightRange comfortably covers the whole
	// fixture regardless).
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: dcEntranceGoblinID, Position: dcRowHex(1),
		HP: 7, MaxHP: 7, AC: 5, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
		DataJSON: testGoblinDataJSON(s.T(), dcEntranceGoblinID),
	}))
	s.Require().Equal(core.ModeTurnBased, enc.Mode(), "sighting the entrance goblin must start the first pocket")

	sub, err := s.broker.Subscribe("enc-dungeon-completion-tpk", alicePlayerID)
	s.Require().NoError(err)

	dcEndTurnUntilActive(s.ctx, &s.Suite, enc, dcEntranceGoblinID)
	s.Require().NoError(enc.NPCAct(s.ctx, dcEntranceGoblinID))

	s.Equal(core.ModeEnded, enc.Mode(), "the last living player dying must end the encounter")
	s.True(enc.ToData().Players[alicePlayerID].Dead)
	s.Empty(enc.ToData().Initiative)

	// The whole point of "TPK, not victory": the corridor and boss goblins
	// were never engaged and must still be exactly as seeded.
	s.Contains(enc.ToData().Monsters, dcCorridorGoblinID, "corridor goblin, never engaged, must still be alive")
	s.Contains(enc.ToData().Monsters, dcBossGoblinID, "boss goblin, never engaged, must still be alive")

	var ended *events.EncounterEndedEvent
	for _, evt := range collectEventsTyped(sub, 500*time.Millisecond) {
		if e, ok := evt.(*events.EncounterEndedEvent); ok {
			ended = e
		}
	}
	s.Require().NotNil(ended, "the TPK must publish EncounterEndedEvent")
	s.Equal(encounter.EncounterEndedReasonTPK, ended.Reason,
		"a TPK mid-dungeon must use the canonical TPK reason, not the victory reason")
	_ = sub.Close()

	s.dcAssertReloadSafe(enc)
}

// dcAssertReloadSafe is the shared reload-safety assertion #817's done bar
// calls for: "reconnect lands in a completed, endable state" — round-trips
// the ended encounter through ToData/JSON/LoadFromData and confirms Mode and
// Initiative, the fields the toolkit actually persists on Data, survive the
// round trip as terminal, and that a post-reload verb call is rejected with
// ErrEncounterEnded rather than resuming as a live, inescapable fight.
//
// Reason is deliberately NOT asserted here post-reload: it is a field on the
// published EncounterEndedEvent only (events/encounter_ended.go), never
// written onto Data, so there is nothing for LoadFromData to round-trip —
// any reason projection for a resumed/reconnecting client is rpg-api's job
// (#690, explicitly out of scope for this toolkit-owned issue). This
// comment, and the assertions below, are the "to the extent the toolkit
// owns it" boundary #817 draws.
func (s *DungeonCompletionSuite) dcAssertReloadSafe(enc *encounter.Encounter) {
	s.Require().Equal(core.ModeEnded, enc.Mode(), "test premise: encounter must already be ended before reload")

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))

	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker)
	s.Require().NoError(err)

	s.Equal(core.ModeEnded, loaded.Mode(), "ModeEnded must survive a ToData/LoadFromData round trip")
	s.Empty(loaded.ToData().Initiative, "a reloaded ended encounter must not carry a stale initiative order")

	// A reconnecting player must land in an endable state, not an
	// inescapable fight: any post-reload verb call must be rejected with
	// the terminal sentinel, not silently resume combat.
	_, _, err = loaded.EndTurn(s.ctx, loaded.ActiveActor())
	s.ErrorIs(err, encounter.ErrEncounterEnded, "a reloaded ended encounter must reject verbs, not resume as a live fight")
}
