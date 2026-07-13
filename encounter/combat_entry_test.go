package encounter_test

// combat_entry_test.go covers rpg-toolkit#757's inline combat-entry
// self-transition: a player-monster visibility pair forming while FREE_ROAM
// flips the encounter to TURN_BASED by itself, mirroring checkEncounterEnd's
// self-transition at combat's other edge. See death.go / visibility_transition_test.go
// for the analogous pattern at combat end.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

type CombatEntrySuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
}

func TestCombatEntrySuite(t *testing.T) {
	suite.Run(t, new(CombatEntrySuite))
}

func (s *CombatEntrySuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New(context.Background(), "enc-combat-entry", s.broker)
}

// A player added within sight of an already-present monster: AddMonster
// itself is the mutation site that must notice the pair (the monster didn't
// exist a moment ago, so its visibility to any player is inherently "newly
// formed").
func (s *CombatEntrySuite) TestAddMonster_VisibleToExistingPlayer_EntersCombat() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 5,
	}))
	s.Equal(core.ModeFreeRoam, s.enc.Mode())

	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-1", Position: core.Hex{Q: 2, R: -2, S: 0}, HP: 7, MaxHP: 7,
	}))

	s.Equal(core.ModeTurnBased, s.enc.Mode())
	s.NotEmpty(s.enc.ToData().Initiative, "SetMode's initiative roll must have run")
}

// A player who moves into an already-present monster's LoS: Move is the
// mutation site.
func (s *CombatEntrySuite) TestMove_IntoMonsterLOS_EntersCombat() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: -10, R: 10, S: 0}, SightRange: 5, // far from the goblin
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-1", Position: core.Hex{Q: 0, R: 0, S: 0}, HP: 7, MaxHP: 7,
	}))
	s.Equal(core.ModeFreeRoam, s.enc.Mode(), "goblin out of LoS at add time — must not have entered combat yet")

	s.Require().NoError(s.enc.Move("alice", []core.Hex{
		{Q: -5, R: 5, S: 0}, {Q: 0, R: 0, S: 0},
	}))

	s.Equal(core.ModeTurnBased, s.enc.Mode())
}

// Player-player visibility must never trigger combat entry — "hostile" ==
// "is a monster" for wave 1.
func (s *CombatEntrySuite) TestPlayerPlayerVisibility_NeverEntersCombat() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 5,
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: core.Hex{Q: 1, R: -1, S: 0}, SightRange: 5,
	}))

	s.Equal(core.ModeFreeRoam, s.enc.Mode())

	s.Require().NoError(s.enc.Move("alice", []core.Hex{{Q: 1, R: 0, S: -1}}))
	s.Equal(core.ModeFreeRoam, s.enc.Mode(), "no monster in the encounter — must stay FREE_ROAM")
}

// Once combat has started, further mutation-site calls must not re-fire
// SetMode (which would error "mode is already TURN_BASED") — the mode gate
// at the top of checkCombatEntry makes it idempotent.
func (s *CombatEntrySuite) TestIdempotent_AddMonsterAndMoveAfterCombatStarted() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 5,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-1", Position: core.Hex{Q: 1, R: -1, S: 0}, HP: 7, MaxHP: 7,
	}))
	s.Require().Equal(core.ModeTurnBased, s.enc.Mode())

	// A second monster, also visible, must not error even though the
	// encounter is already TURN_BASED.
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-2", Position: core.Hex{Q: 2, R: -2, S: 0}, HP: 7, MaxHP: 7,
	}))
	s.Equal(core.ModeTurnBased, s.enc.Mode())

	// A further move by the same player must not error either.
	s.Require().NoError(s.enc.Move("alice", []core.Hex{{Q: 0, R: 1, S: -1}}))
	s.Equal(core.ModeTurnBased, s.enc.Mode())
}
