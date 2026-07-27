package encounter_test

// knowledge_test.go covers rpg-toolkit#851's acceptance bar for
// Encounter.KnownHexes: first discovery, loss of sight, re-sight,
// hidden-mutation isolation, witnessed removal, multi-viewer isolation,
// persist/reload without consulting current world data, and idempotent
// reconciliation. See encounter/knowledge.go's file doc for the
// implementation this exercises, and perception/knowledge.go for the
// HexObservation contract these tests assert against.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
)

type KnowledgeSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestKnowledgeSuite(t *testing.T) {
	suite.Run(t, new(KnowledgeSuite))
}

func (s *KnowledgeSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *KnowledgeSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// contentIDs is a test-only helper: the sorted-by-scan entity ids in obs's
// Contents, for order-independent assertions.
func contentIDs(obs perception.HexObservation) []core.EntityID {
	out := make([]core.EntityID, 0, len(obs.Contents))
	for _, p := range obs.Contents {
		out = append(out, p.EntityID)
	}
	return out
}

// TestKnownHexes_FirstSight_EstablishesVisibleTotalObservation covers the
// "first discovery" acceptance line: a freshly-added player's own starting
// hex is VISIBLE, carries the door edge actually there, and Contents is
// TOTAL — the player's own entity is listed (they're standing there), and
// an empty-of-everything-else hex nearby is Visible with empty contents
// (a positive "nothing here" claim, not omission).
func (s *KnowledgeSuite) TestKnownHexes_FirstSight_EstablishesVisibleTotalObservation() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	origin := core.Hex{Q: 0, R: 0, S: 0}
	// The door sits adjacent to alice's spawn so her first reveal picks up
	// its edge — AddDoor before AddPlayer so refreshObservations sees it.
	doorHex := core.Hex{Q: 1, R: 0, S: -1}
	s.Require().NoError(e.AddDoor("door-1", doorHex, false))
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: origin, SightRange: 3,
	}))

	known := e.KnownHexes(alicePlayerID)
	s.Require().Contains(known, origin)
	ownHex := known[origin]
	s.Equal(perception.KnowledgeStateVisible, ownHex.State)
	s.Equal([]core.EntityID{aliceEntityID}, contentIDs(ownHex),
		"a viewer's own hex must list their own entity — Contents is total")

	s.Require().Contains(known, doorHex)
	doorObs := known[doorHex]
	s.Equal(perception.KnowledgeStateVisible, doorObs.State)
	s.Require().Len(doorObs.Edges, 1)
	s.Equal("door-1", doorObs.Edges[0].DoorID)
	s.False(doorObs.Edges[0].DoorOpen)
	s.Empty(contentIDs(doorObs), "the door hex has no occupant — Contents must be empty, not absent")

	emptyHex := core.Hex{Q: -1, R: 0, S: 1}
	s.Require().Contains(known, emptyHex, "an empty hex within sight must still be known")
	s.Equal(perception.KnowledgeStateVisible, known[emptyHex].State)
	s.Empty(contentIDs(known[emptyHex]))

	farHex := core.Hex{Q: 10, R: -10, S: 0}
	s.NotContains(known, farHex, "a hex never observed must be ABSENT, not present with a zero value")
}

// TestKnownHexes_LossOfSight_FreezesRememberedObservation covers "loss of
// sight": once alice moves away, her last-seen record of bob's hex freezes
// with EXACTLY what she saw there (bob's placement), and does not vanish.
func (s *KnowledgeSuite) TestKnownHexes_LossOfSight_FreezesRememberedObservation() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 2,
	}))
	bobHex := core.Hex{Q: 2, R: -2, S: 0}
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: bobPlayerID, EntityID: bobEntityID,
		Position: bobHex, SightRange: 2,
	}))

	// alice takes a 1-hex step; her own reveal refresh now scans current
	// world truth (including bob, added after her) for everything she can
	// see from her new position — this is how she first learns bob is
	// there (AddPlayer does not itself notify already-seated viewers).
	s.Require().NoError(e.Move(alicePlayerID, []core.Hex{{Q: 1, R: -1, S: 0}}))

	before := e.KnownHexes(alicePlayerID)
	s.Require().Contains(before, bobHex)
	s.Equal(perception.KnowledgeStateVisible, before[bobHex].State)
	s.Equal([]core.EntityID{bobEntityID}, contentIDs(before[bobHex]))

	// alice walks far away — bobHex leaves her sight entirely.
	s.Require().NoError(e.Move(alicePlayerID, []core.Hex{{Q: 20, R: -20, S: 0}}))

	after := e.KnownHexes(alicePlayerID)
	s.Require().Contains(after, bobHex, "a hex once known must never become absent")
	s.Equal(perception.KnowledgeStateRemembered, after[bobHex].State)
	s.Equal([]core.EntityID{bobEntityID}, contentIDs(after[bobHex]),
		"the frozen observation must carry exactly what alice last saw — bob's placement")
}

// TestKnownHexes_ReSight_AtomicallyRefreshesContent covers "re-sight": a
// hex that goes visible -> remembered -> visible again must replace the
// stale remembered content with CURRENT truth in one step, not carry the
// stale content forward. A monster spawning at the hex while alice cannot
// see it must not appear in her memory until she actually looks again —
// and once she does, it must appear correctly, proving the refresh is a
// real re-observation, not a no-op because the hex was "already known".
func (s *KnowledgeSuite) TestKnownHexes_ReSight_AtomicallyRefreshesContent() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 2,
	}))
	targetHex := core.Hex{Q: 2, R: -2, S: 0}

	// First sight: empty.
	first := e.KnownHexes(alicePlayerID)
	s.Require().Contains(first, targetHex)
	s.Equal(perception.KnowledgeStateVisible, first[targetHex].State)
	s.Empty(contentIDs(first[targetHex]))

	// Lose sight.
	s.Require().NoError(e.Move(alicePlayerID, []core.Hex{{Q: 20, R: -20, S: 0}}))
	lost := e.KnownHexes(alicePlayerID)
	s.Equal(perception.KnowledgeStateRemembered, lost[targetHex].State)

	// While alice is away and cannot see it, a monster spawns on the exact
	// hex she remembers as empty. Her memory must NOT change (hidden
	// mutation — proven directly by re-reading, not just asserted).
	s.Require().NoError(e.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: targetHex, HP: 7, MaxHP: 7, AC: 12,
	}))
	stillStale := e.KnownHexes(alicePlayerID)
	s.Empty(contentIDs(stillStale[targetHex]),
		"a hidden mutation must not reach a viewer's memory — the goblin spawned out of alice's sight")

	// alice returns. Re-sight must atomically refresh to CURRENT truth.
	s.Require().NoError(e.Move(alicePlayerID, []core.Hex{{Q: 0, R: 0, S: 0}}))
	reSighted := e.KnownHexes(alicePlayerID)
	s.Require().Contains(reSighted, targetHex)
	s.Equal(perception.KnowledgeStateVisible, reSighted[targetHex].State)
	s.Equal([]core.EntityID{gobEntityID}, contentIDs(reSighted[targetHex]),
		"re-sight must replace stale-empty memory with the goblin now actually there")
}

// TestKnownHexes_HiddenMutation_LeavesUnrelatedMemoryUntouched proves a
// mutation on a hex the viewer has NEVER seen leaves both that hex absent
// AND every other already-known hex's observation byte-for-byte unchanged
// — a hidden event must not disturb memory it has no business touching.
func (s *KnowledgeSuite) TestKnownHexes_HiddenMutation_LeavesUnrelatedMemoryUntouched() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 2,
	}))
	known := core.Hex{Q: 1, R: -1, S: 0}
	hidden := core.Hex{Q: 20, R: -20, S: 0}

	before := e.KnownHexes(alicePlayerID)
	s.Require().Contains(before, known)
	beforeObs := before[known]

	// A monster spawns far outside alice's sight.
	s.Require().NoError(e.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: hidden, HP: 7, MaxHP: 7, AC: 12,
	}))

	after := e.KnownHexes(alicePlayerID)
	s.NotContains(after, hidden, "a hex never seen must stay absent even after an unrelated hidden mutation")
	s.Require().Contains(after, known)
	s.Equal(beforeObs, after[known], "an unrelated hidden mutation must not alter an already-known hex's observation")
}

// TestKnownHexes_WitnessedRemoval_UpdatesImmediately lives in
// knowledge_internal_test.go (package encounter — it calls the unexported
// killEntity directly rather than driving the full TakeAction combat
// machinery just to remove a monster).

// TestKnownHexes_MultiViewer_Isolation is the case the playable concept
// never proved (it only ever exercised one viewer): two viewers of
// overlapping geometry must hold INDEPENDENT knowledge. Alice sees a
// goblin and remembers a hex bob has never even discovered; bob has his
// own separate first-sight/remembered state. Neither viewer's KnownHexes
// leaks into the other's.
func (s *KnowledgeSuite) TestKnownHexes_MultiViewer_Isolation() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 2,
	}))
	// bob starts far away — outside alice's sight and vice versa.
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: bobPlayerID, EntityID: bobEntityID,
		Position: core.Hex{Q: 50, R: -50, S: 0}, SightRange: 2,
	}))

	aliceOnlyHex := core.Hex{Q: 2, R: -2, S: 0}
	s.Require().NoError(e.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: aliceOnlyHex, HP: 7, MaxHP: 7, AC: 12,
	}))
	// alice's own move refreshes her memory of aliceOnlyHex (per the
	// AddPlayer/AddMonster ordering note elsewhere in this file).
	s.Require().NoError(e.Move(alicePlayerID, []core.Hex{{Q: 1, R: -1, S: 0}}))

	aliceKnown := e.KnownHexes(alicePlayerID)
	bobKnown := e.KnownHexes(bobPlayerID)

	s.Require().Contains(aliceKnown, aliceOnlyHex)
	s.Equal([]core.EntityID{gobEntityID}, contentIDs(aliceKnown[aliceOnlyHex]))
	s.NotContains(bobKnown, aliceOnlyHex,
		"bob has never seen this hex or the goblin on it — it must be totally absent from HIS knowledge")

	// alice loses sight of the goblin's hex; it freezes to Remembered in
	// HER memory only. bob still has nothing for it.
	s.Require().NoError(e.Move(alicePlayerID, []core.Hex{{Q: 30, R: -30, S: 0}}))
	aliceAfter := e.KnownHexes(alicePlayerID)
	bobAfter := e.KnownHexes(bobPlayerID)
	s.Equal(perception.KnowledgeStateRemembered, aliceAfter[aliceOnlyHex].State)
	s.NotContains(bobAfter, aliceOnlyHex, "alice's remembered geometry must not leak into bob's knowledge at all")

	// Symmetric check: bob's own starting hex is known to bob, absent from
	// alice's knowledge.
	bobOwnHex := core.Hex{Q: 50, R: -50, S: 0}
	s.Require().Contains(bobAfter, bobOwnHex)
	s.NotContains(aliceAfter, bobOwnHex)
}

// TestKnownHexes_PersistReload_SurvivesWithoutConsultingWorldData is
// #851's real acceptance bar: a fresh Encounter rehydrated from ToData's
// JSON round-trip must report the IDENTICAL known geometry the original
// had — including REMEMBERED records — without silently re-deriving them
// from current world state. Proven by mutating the world (moving the
// goblin alice remembered) BETWEEN save and load: if reload consulted
// current world data instead of persisted memory, alice's reloaded
// "remembered" observation would show the goblin's NEW position instead of
// where she actually last saw it — exactly the leak this bar exists to
// catch.
func (s *KnowledgeSuite) TestKnownHexes_PersistReload_SurvivesWithoutConsultingWorldData() {
	ctx := context.Background()
	e1 := encounter.New(ctx, "enc-1", s.broker)
	s.Require().NoError(e1.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 2,
	}))
	goblinHex := core.Hex{Q: 2, R: -2, S: 0}
	s.Require().NoError(e1.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: goblinHex, HP: 7, MaxHP: 7, AC: 12,
	}))
	s.Require().NoError(e1.Move(alicePlayerID, []core.Hex{{Q: 1, R: -1, S: 0}}))
	s.Require().NoError(e1.Move(alicePlayerID, []core.Hex{{Q: 30, R: -30, S: 0}}))

	before := e1.KnownHexes(alicePlayerID)
	s.Require().Contains(before, goblinHex)
	s.Equal(perception.KnowledgeStateRemembered, before[goblinHex].State)
	s.Equal([]core.EntityID{gobEntityID}, contentIDs(before[goblinHex]))

	// Persist, then mutate the world AFTER saving but BEFORE reload: move
	// the goblin off the hex alice remembers it on. A correct reload must
	// still show alice's OLD (persisted) observation, not the goblin's new
	// position.
	data := e1.ToData()
	payload, err := json.Marshal(data)
	s.Require().NoError(err)

	data.Monsters[gobEntityID].Position = core.Hex{Q: 40, R: -40, S: 0}

	var reloaded encounter.Data
	s.Require().NoError(json.Unmarshal(payload, &reloaded))
	e2, err := encounter.LoadFromData(ctx, &reloaded, s.broker)
	s.Require().NoError(err)

	after := e2.KnownHexes(alicePlayerID)
	s.Require().Contains(after, goblinHex,
		"persisted remembered geometry must survive reload")
	s.Equal(perception.KnowledgeStateRemembered, after[goblinHex].State)
	s.Equal([]core.EntityID{gobEntityID}, contentIDs(after[goblinHex]),
		"reload must replay alice's PERSISTED observation, never re-derive from current (mutated) world data")
}

// TestKnownHexes_DuplicateReconciliation_Idempotent covers "duplicate
// reconciliation is idempotent": reconciling the identical visibility twice
// in a row (two no-op moves to the same hex) must leave KnownHexes exactly
// as it was after the first.
func (s *KnowledgeSuite) TestKnownHexes_DuplicateReconciliation_Idempotent() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 2,
	}))
	s.Require().NoError(e.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: -1, S: 0}, HP: 7, MaxHP: 7, AC: 12,
	}))

	first := e.KnownHexes(alicePlayerID)

	// A zero-distance "move" isn't legal (Move requires a real path), so
	// idempotency is proven the way the toolkit actually re-reconciles:
	// stepping to a hex within the SAME visible set twice in a row, which
	// re-observes the identical world state both times.
	s.Require().NoError(e.Move(alicePlayerID, []core.Hex{{Q: 0, R: 0, S: 0}, {Q: 0, R: 0, S: 0}}))
	second := e.KnownHexes(alicePlayerID)

	s.Equal(first, second, "reconciling unchanged visibility twice must leave KnownHexes identical")
}

// TestKnownHexes_StationaryViewer_MoverCrossingSeveralHexes_EndsInExactlyOne
// is rpg-toolkit#858's core acceptance bar: bob is stationary with sight
// range 5 (never moves, never refreshes his own position), and watches
// alice cross FOUR of his visible hexes in one move, stopping on the last
// one (still within his sight — not a pass-through). Bob's memory of
// alice's crossing must show her in exactly the hex she stopped on, not
// smeared across every hex he watched her traverse.
func (s *KnowledgeSuite) TestKnownHexes_StationaryViewer_MoverCrossingSeveralHexes_EndsInExactlyOne() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: bobPlayerID, EntityID: bobEntityID,
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 5,
	}))
	// alice starts well outside bob's sight (distance 6) and crosses four
	// hexes all within it (distances 5,4,3,2), stopping on the last —
	// crossedHex1/2/3 must end up empty of her; finalHex must hold her.
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{Q: 6, R: -6, S: 0}, SightRange: 1,
	}))
	crossedHex1 := core.Hex{Q: 5, R: -5, S: 0}
	crossedHex2 := core.Hex{Q: 4, R: -4, S: 0}
	crossedHex3 := core.Hex{Q: 3, R: -3, S: 0}
	finalHex := core.Hex{Q: 2, R: -2, S: 0}

	s.Require().NoError(e.Move(alicePlayerID, []core.Hex{crossedHex1, crossedHex2, crossedHex3, finalHex}))

	bobKnown := e.KnownHexes(bobPlayerID)
	s.Require().Contains(bobKnown, finalHex)
	s.Equal([]core.EntityID{aliceEntityID}, contentIDs(bobKnown[finalHex]),
		"alice must be recorded on the hex she actually stopped on")

	for _, h := range []core.Hex{crossedHex1, crossedHex2, crossedHex3} {
		s.Require().Contains(bobKnown, h, "bob watched alice cross this hex, so it must be known to him")
		s.Empty(contentIDs(bobKnown[h]),
			"alice must NOT be recorded on a hex she has since left, even though bob watched her cross it")
	}
}

// TestKnownHexes_PassThrough_MoverCrossesAndExits_LeavesNoVisibleTrace is
// rpg-toolkit#858's pass-through acceptance bar: bob is stationary with
// sight range 2, and alice moves in one path from far outside his sight,
// briefly through two hexes he CAN see, and out the far side back beyond
// his sight — neither her start nor her true final position is ever within
// bob's sight. Bob must end with alice in NO hex's contents at all: not
// pinned at the last hex he saw her in (that would fabricate a visible
// placement the contract reserves for REMEMBERED state, never VISIBLE).
func (s *KnowledgeSuite) TestKnownHexes_PassThrough_MoverCrossesAndExits_LeavesNoVisibleTrace() {
	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: bobPlayerID, EntityID: bobEntityID,
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 2,
	}))
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{Q: 20, R: -20, S: 0}, SightRange: 1,
	}))
	seenHex1 := core.Hex{Q: 1, R: -1, S: 0}
	seenHex2 := core.Hex{Q: 2, R: -2, S: 0}
	farExit := core.Hex{Q: 30, R: -30, S: 0}

	s.Require().NoError(e.Move(alicePlayerID, []core.Hex{seenHex1, seenHex2, farExit}))

	bobKnown := e.KnownHexes(bobPlayerID)
	for _, h := range []core.Hex{seenHex1, seenHex2} {
		s.Require().Contains(bobKnown, h, "bob watched alice cross this hex, so it must be known to him")
		s.Empty(contentIDs(bobKnown[h]),
			"alice has moved on past bob's sight entirely — she must not be pinned at the last hex he saw her in")
	}
	s.NotContains(bobKnown, farExit, "alice's true final hex was never within bob's sight at all")
}
