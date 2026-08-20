package encounter_test

// initiative_rolled_test.go covers rpg-toolkit#765: SetMode publishes
// InitiativeRolledEvent{Order} on the FREE_ROAM->TURN_BASED transition, so
// consumers get the rolled roster on the wire instead of synthesizing it via
// a racy repo read-back (rpg-api#647).

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
)

type InitiativeRolledSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
	aliceSub  *encounter.Subscription
}

func TestInitiativeRolledSuite(t *testing.T) {
	suite.Run(t, new(InitiativeRolledSuite))
}

func (s *InitiativeRolledSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	// fixedMaxRoller (death_test.go) makes every d20 roll 20, so initiative
	// ties are broken deterministically by rollInitiative's ascending-id
	// tiebreak ("char-alice" < "goblin-1") — alice always acts first. Without
	// this, a real roll can put the goblin first, and TestEndTurn's EndTurn
	// call would need a wired StrikeResolver to run NPCAct on it; determinism
	// here is simpler and correct than exercising both branches.
	s.enc = encounter.New(context.Background(), "enc-initiative-rolled", s.broker,
		encounter.WithRoller(fixedMaxRoller{}))

	var err error
	s.aliceSub, err = s.broker.Subscribe("enc-initiative-rolled", "alice")
	s.Require().NoError(err)
}

func (s *InitiativeRolledSuite) TearDownTest() {
	_ = s.aliceSub.Close()
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TestSetMode_InitiativeRolled_SequencedBetweenModeChangedAndTurnStarted
// pins the FULL ordering contract this AddMonster call produces: the
// goblin's EntityAppearedEvent (#764) -> ModeChanged -> InitiativeRolled ->
// TurnStarted, all from the same FREE_ROAM -> TURN_BASED transition
// (triggered here via AddMonster's checkCombatEntry call, the same
// combat-entry path #764 exercises). Consumers bind to this order, so all
// four are pinned together in one place rather than split across tests.
func (s *InitiativeRolledSuite) TestSetMode_InitiativeRolled_SequencedBetweenModeChangedAndTurnStarted() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-1", Position: core.Hex{Q: 1, R: 0, S: -1}, HP: 7, MaxHP: 7,
		DataJSON: testGoblinDataJSON(s.T(), "goblin-1"),
	}))
	s.Require().Equal(core.ModeTurnBased, s.enc.Mode())

	aliceEvts := collectEventsTyped(s.aliceSub, 500*time.Millisecond)

	appearedIdx, modeChangedIdx, rolledIdx, turnStartedIdx := -1, -1, -1, -1
	var rolled *events.InitiativeRolledEvent
	for i, evt := range aliceEvts {
		switch e := evt.(type) {
		case *events.EntityAppearedEvent:
			if e.Entity == core.EntityID("goblin-1") {
				appearedIdx = i
			}
		case *events.ModeChangedEvent:
			modeChangedIdx = i
		case *events.InitiativeRolledEvent:
			rolledIdx = i
			rolled = e
		case *events.TurnStartedEvent:
			turnStartedIdx = i
		}
	}
	s.Require().GreaterOrEqual(appearedIdx, 0, "goblin-1's EntityAppearedEvent must have fired")
	s.Require().GreaterOrEqual(modeChangedIdx, 0, "ModeChangedEvent must have fired")
	s.Require().GreaterOrEqual(rolledIdx, 0, "InitiativeRolledEvent must have fired")
	s.Require().GreaterOrEqual(turnStartedIdx, 0, "TurnStartedEvent must have fired")
	s.Less(appearedIdx, modeChangedIdx, "the goblin's appearance must precede the ModeChanged it caused")
	s.Less(modeChangedIdx, rolledIdx, "InitiativeRolled must come after ModeChanged")
	s.Less(rolledIdx, turnStartedIdx, "InitiativeRolled must come before TurnStarted")

	// The roster must match the encounter's actual rolled initiative and
	// contain both combatants.
	s.Require().NotNil(rolled)
	s.ElementsMatch([]core.EntityID{"char-alice", "goblin-1"}, rolled.Order)
	s.Equal(s.enc.ToData().Initiative, rolled.Order)
	s.Contains(rolled.PerPlayer, core.PlayerID("alice"))
	s.True(rolled.PerPlayer["alice"].Visible)
}

// TestEndTurn_DoesNotRepublishInitiativeRolled: the roster doesn't change
// turn to turn, so InitiativeRolledEvent must fire exactly once at the
// FREE_ROAM->TURN_BASED transition, never again on later EndTurn advances.
// The suite's fixedMaxRoller makes alice deterministically win the initiative
// tiebreak (ascending entity id), so EndTurn always runs the player path
// here — no NPCAct / StrikeResolver dependency.
func (s *InitiativeRolledSuite) TestEndTurn_DoesNotRepublishInitiativeRolled() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-1", Position: core.Hex{Q: 1, R: 0, S: -1}, HP: 7, MaxHP: 7,
		DataJSON: testGoblinDataJSON(s.T(), "goblin-1"),
	}))
	s.Require().Equal(core.ModeTurnBased, s.enc.Mode())
	s.Require().Equal(core.EntityID("char-alice"), s.enc.ActiveActor(),
		"fixedMaxRoller's ascending-id tiebreak must put alice first")
	_ = collectEventsTyped(s.aliceSub, 300*time.Millisecond) // drain the entry transition's events

	_, _, err := s.enc.EndTurn(context.Background(), "char-alice")
	s.Require().NoError(err)

	aliceEvts := collectEventsTyped(s.aliceSub, 300*time.Millisecond)
	for _, evt := range aliceEvts {
		if _, ok := evt.(*events.InitiativeRolledEvent); ok {
			s.Fail("InitiativeRolledEvent must not republish on a later EndTurn advance")
		}
	}
}

// TestSetMode_FreeRoamTransition_NoInitiativeRolled: the reverse transition
// (TURN_BASED -> FREE_ROAM, e.g. combat ending) clears Initiative and must
// not publish InitiativeRolledEvent — there is no roster to report.
func (s *InitiativeRolledSuite) TestSetMode_FreeRoamTransition_NoInitiativeRolled() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-1", Position: core.Hex{Q: 1, R: 0, S: -1}, HP: 7, MaxHP: 7,
		DataJSON: testGoblinDataJSON(s.T(), "goblin-1"),
	}))
	s.Require().Equal(core.ModeTurnBased, s.enc.Mode())
	_ = collectEventsTyped(s.aliceSub, 300*time.Millisecond) // drain the entry transition's events

	s.Require().NoError(s.enc.SetMode(core.ModeFreeRoam))

	aliceEvts := collectEventsTyped(s.aliceSub, 300*time.Millisecond)
	for _, evt := range aliceEvts {
		if _, ok := evt.(*events.InitiativeRolledEvent); ok {
			s.Fail("InitiativeRolledEvent must not fire on TURN_BASED->FREE_ROAM (Initiative is cleared, not rolled)")
		}
	}
}
