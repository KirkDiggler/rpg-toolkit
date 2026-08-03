package encounter

// knowledge_internal_test.go — rpg-toolkit#851 white-box test for the
// witnessed-removal memory refresh wired into killEntity (death.go). Lives
// in package encounter (not encounter_test) so it can call the unexported
// killEntity directly, rather than driving the full TakeAction combat
// machinery (held characters, dice rollers, resolvers) just to remove a
// monster — this test is about the memory-refresh wiring, not combat
// resolution, which is covered elsewhere (move_resolver_test.go,
// combat_phased_test.go).

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

// Test-package fixture identifiers (extracted to satisfy goconst — mirrors
// combat_test.go's identical convention in the encounter_test package one
// file tree over).
const (
	knowledgeAlicePlayerID = "alice"
	knowledgeAliceEntityID = "char-alice"
	knowledgeGobEntityID   = "goblin-1"
)

type KnowledgeInternalSuite struct {
	suite.Suite
	transport *InMemoryTransport
	broker    *Broker
}

func TestKnowledgeInternalSuite(t *testing.T) {
	suite.Run(t, new(KnowledgeInternalSuite))
}

func (s *KnowledgeInternalSuite) SetupTest() {
	s.transport = NewInMemoryTransport()
	s.broker = NewBroker(s.transport)
}

func (s *KnowledgeInternalSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TestKnownHexes_WitnessedRemoval_UpdatesImmediately proves a removal
// alice WATCHES happen updates her memory of that hex right away — no
// tombstone, just a fresh Visible record that no longer lists the thing
// (HexObservation's "nothing is ever deleted" contract).
func (s *KnowledgeInternalSuite) TestKnownHexes_WitnessedRemoval_UpdatesImmediately() {
	e := New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(PlayerInput{
		PlayerID: knowledgeAlicePlayerID, EntityID: knowledgeAliceEntityID,
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 5,
	}))
	goblinHex := core.Hex{Q: 2, R: -2, S: 0}
	s.Require().NoError(e.AddMonster(MonsterInput{
		ID: knowledgeGobEntityID, Position: goblinHex, HP: 1, MaxHP: 1, AC: 10,
	}))

	before := e.KnownHexes(knowledgeAlicePlayerID)
	s.Require().Contains(before, goblinHex)
	s.Equal([]core.EntityID{knowledgeGobEntityID}, contentEntityIDs(before[goblinHex]))

	s.Require().NoError(e.killEntity(knowledgeGobEntityID, ""))

	after := e.KnownHexes(knowledgeAlicePlayerID)
	s.Require().Contains(after, goblinHex, "the hex itself is never deleted")
	s.Equal(perception.KnowledgeStateVisible, after[goblinHex].State,
		"alice is still looking right at the hex — it must stay Visible, not flip to Remembered")
	s.Empty(contentEntityIDs(after[goblinHex]),
		"a witnessed removal must clear Contents immediately, without waiting for alice's next move")
}

// TestKnownHexes_WitnessedRemoval_OnlyRefreshesWitnesses proves the killer
// (who may see only the killer's position, not the dying monster's hex) is
// not force-refreshed for a hex they never had knowledge of, and a
// completely unrelated bystander with no LoS to either gets nothing.
func (s *KnowledgeInternalSuite) TestKnownHexes_WitnessedRemoval_OnlyRefreshesWitnesses() {
	e := New(context.Background(), "enc-1", s.broker)
	goblinHex := core.Hex{Q: 0, R: 0, S: 0}
	s.Require().NoError(e.AddMonster(MonsterInput{
		ID: knowledgeGobEntityID, Position: goblinHex, HP: 1, MaxHP: 1, AC: 10,
	}))
	// bystander is far from the goblin and never sees it at all.
	s.Require().NoError(e.AddPlayer(PlayerInput{
		PlayerID: "bystander", EntityID: "char-bystander",
		Position: core.Hex{Q: 50, R: -50, S: 0}, SightRange: 2,
	}))

	s.Require().NoError(e.killEntity(knowledgeGobEntityID, ""))

	known := e.KnownHexes("bystander")
	s.NotContains(known, goblinHex, "a viewer with no LoS to the dying monster must gain no knowledge of its hex")
}

// TestRefreshObservations_CanonicalizationFailurePreservesMemory proves a
// malformed physical-edge conflict aborts a refresh before Memory.Observe can
// replace any established geometry with empty or partial edges.
func (s *KnowledgeInternalSuite) TestRefreshObservations_CanonicalizationFailurePreservesMemory() {
	e := New(context.Background(), "enc-1", s.broker)
	origin := core.Hex{}
	s.Require().NoError(e.AddPlayer(PlayerInput{
		PlayerID: knowledgeAlicePlayerID, EntityID: knowledgeAliceEntityID,
		Position: origin, SightRange: 1,
	}))
	before := e.KnownHexes(knowledgeAlicePlayerID)

	neighbor := core.Hex{Q: 1, R: -1, S: 0}
	e.data.Space = &SpaceData{Walls: []environments.WallSegmentData{
		{Start: origin.ToCube(), End: neighbor.ToCube(), BlocksMovement: true, BlocksLoS: true},
		{Start: neighbor.ToCube(), End: origin.ToCube(), BlocksMovement: true, BlocksLoS: false},
	}}

	err := e.refreshObservations(e.data.Players[knowledgeAlicePlayerID].View, core.NewHexSet(origin, neighbor))
	s.Require().Error(err)
	s.Contains(err.Error(), "conflicting generated edges")
	s.Equal(before, e.KnownHexes(knowledgeAlicePlayerID),
		"failed canonicalization must not replace remembered observations")
}

func contentEntityIDs(obs perception.HexObservation) []core.EntityID {
	out := make([]core.EntityID, 0, len(obs.Contents))
	for _, p := range obs.Contents {
		out = append(out, p.EntityID)
	}
	return out
}
