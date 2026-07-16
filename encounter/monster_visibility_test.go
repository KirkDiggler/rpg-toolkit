package encounter_test

// monster_visibility_test.go covers rpg-toolkit#761: EntityAppeared /
// EntityDisappeared for MONSTERS when a player's move changes which
// monsters they can see. Mirrors visibility_transition_test.go's
// player-sees-player coverage, but for the player-sees-monster direction.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
)

type MonsterVisibilitySuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
	aliceSub  *encounter.Subscription
}

func TestMonsterVisibilitySuite(t *testing.T) {
	suite.Run(t, new(MonsterVisibilitySuite))
}

func (s *MonsterVisibilitySuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New(context.Background(), "enc-monster-vis", s.broker)

	var err error
	s.aliceSub, err = s.broker.Subscribe("enc-monster-vis", "alice")
	s.Require().NoError(err)
}

func (s *MonsterVisibilitySuite) TearDownTest() {
	_ = s.aliceSub.Close()
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TestMoveIntoMonsterLOS_AppearsOnTheSameMoveThatEntersCombat pins the
// issue's core scenario: a goblin sighted by the move that flips
// FREE_ROAM->TURN_BASED must APPEAR to that player on that same move, not a
// later one. Also asserts the ordering: the EntityAppearedEvent for the
// goblin precedes the ModeChangedEvent (cause before effect — the player
// sees the goblin, then combat starts because of it).
func (s *MonsterVisibilitySuite) TestMoveIntoMonsterLOS_AppearsOnTheSameMoveThatEntersCombat() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: -10, R: 0, S: 10}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-1", Position: core.Hex{Q: 0, R: 0, S: 0}, HP: 7, MaxHP: 7,
	}))
	s.Require().Equal(core.ModeFreeRoam, s.enc.Mode(), "goblin out of LoS at add time")

	// Alice moves to Q=3, within her sight range of 4 from the goblin's
	// position at the origin — this is also the move checkCombatEntry uses
	// to flip the encounter into TURN_BASED.
	pathEnd := core.Hex{Q: 3, R: 0, S: -3}
	s.Require().NoError(s.enc.Move("alice", []core.Hex{pathEnd}))

	s.Equal(core.ModeTurnBased, s.enc.Mode(), "the move must have triggered combat entry")

	aliceEvts := collectEventsTyped(s.aliceSub, 500*time.Millisecond)

	appearedIdx := -1
	modeChangedIdx := -1
	for i, evt := range aliceEvts {
		switch e := evt.(type) {
		case *events.EntityAppearedEvent:
			if e.Entity == core.EntityID("goblin-1") {
				appearedIdx = i
				s.Equal(core.Hex{Q: 0, R: 0, S: 0}, e.Position,
					"a stationary monster's Position must be its own fixed hex, not a hex on the mover's path")
				s.Contains(e.PerPlayer, core.PlayerID("alice"))
			}
		case *events.ModeChangedEvent:
			modeChangedIdx = i
		}
	}
	s.Require().GreaterOrEqual(appearedIdx, 0, "goblin-1 must have appeared to alice")
	s.Require().GreaterOrEqual(modeChangedIdx, 0, "combat entry must have published ModeChangedEvent")
	s.Less(appearedIdx, modeChangedIdx,
		"the goblin's EntityAppearedEvent must precede the ModeChangedEvent (cause before effect)")
}

// TestMoveOutOfMonsterLOS_Disappears covers the mid-combat case: a monster
// the player has actually SEEN appear must fire EntityDisappearedEvent when
// a later move takes the player out of range. This exercises the path
// OUTSIDE checkCombatEntry's FREE_ROAM gate, proving detection isn't tied to
// the entry transition. The setup deliberately keeps the goblin out of
// alice's range at AddMonster time (see rpg-toolkit#764: AddMonster DOES
// publish EntityAppearedEvent for a monster that's already visible when
// added) so the appearance under test is unambiguously the one caused by
// alice's own move below, not the add.
func (s *MonsterVisibilitySuite) TestMoveOutOfMonsterLOS_Disappears() {
	// Alice starts out of the goblin's sight range (distance 10 > sight 4),
	// so no combat entry and no appearance fire at setup time.
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 10, R: 0, S: -10}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-1", Position: core.Hex{Q: 0, R: 0, S: 0}, HP: 7, MaxHP: 7,
	}))
	s.Require().Equal(core.ModeFreeRoam, s.enc.Mode(), "goblin out of LoS at add time")

	// Alice moves into range: this is the move that both triggers combat
	// entry AND fires the goblin's EntityAppearedEvent (the scenario the
	// other test in this suite pins in detail). Drain those events so this
	// test can isolate the disappear behavior below.
	path := []core.Hex{
		{Q: 4, R: 0, S: -4}, // distance 4 — visible (edge)
		{Q: 2, R: 0, S: -2}, // distance 2 — clearly visible
	}
	s.Require().NoError(s.enc.Move("alice", path))
	s.Require().Equal(core.ModeTurnBased, s.enc.Mode(), "moving into LoS must have triggered combat entry")
	_ = collectEventsTyped(s.aliceSub, 300*time.Millisecond) // drain appeared + mode-change events

	// Now alice moves away, out of the goblin's range (distance 10 > sight 4).
	outPath := []core.Hex{
		{Q: 3, R: 0, S: -3},   // distance 3 — still visible
		{Q: 4, R: 0, S: -4},   // distance 4 — still visible (edge)
		{Q: 10, R: 0, S: -10}, // distance 10 — outside
	}
	s.Require().NoError(s.enc.Move("alice", outPath))

	aliceEvts := collectEventsTyped(s.aliceSub, 500*time.Millisecond)

	var found *events.EntityDisappearedEvent
	for _, evt := range aliceEvts {
		if e, ok := evt.(*events.EntityDisappearedEvent); ok && e.Entity == core.EntityID("goblin-1") {
			found = e
			break
		}
	}
	s.Require().NotNil(found, "goblin-1 must have disappeared from alice's view")
	s.Require().Contains(found.PerPlayer, core.PlayerID("alice"))
	s.Equal(core.Hex{Q: 0, R: 0, S: 0}, found.PerPlayer[core.PlayerID("alice")],
		"a stationary monster's last-known position must be its own fixed hex")
}

// TestStaysVisible_NoAppearDisappearEvents: a monster that was already
// visible and stays visible across a move must not re-fire
// EntityAppearedEvent (matches the player-sees-player "stays visible" case).
func (s *MonsterVisibilitySuite) TestStaysVisible_NoAppearDisappearEvents() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 1, R: 0, S: -1}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-1", Position: core.Hex{Q: 0, R: 0, S: 0}, HP: 7, MaxHP: 7,
	}))
	// AddMonster flips the mode AND publishes EntityAppearedEvent (the goblin
	// is visible at add time — rpg-toolkit#764) — drain both so the
	// assertions below are scoped to the move under test.
	_ = collectEventsTyped(s.aliceSub, 300*time.Millisecond)

	// Alice moves from Q=1 to Q=2, staying well inside sight range of the
	// stationary goblin at the origin.
	s.Require().NoError(s.enc.Move("alice", []core.Hex{{Q: 2, R: 0, S: -2}}))

	aliceEvts := collectEventsTyped(s.aliceSub, 500*time.Millisecond)
	for _, evt := range aliceEvts {
		switch e := evt.(type) {
		case *events.EntityAppearedEvent:
			s.NotEqual(core.EntityID("goblin-1"), e.Entity, "goblin was already visible — must not re-appear")
		case *events.EntityDisappearedEvent:
			s.NotEqual(core.EntityID("goblin-1"), e.Entity, "goblin stayed visible — must not disappear")
		}
	}
}

// TestPlayerPlayerMoveNeverEmitsMonsterEvents: with no monsters in the
// encounter, a player-sees-player transition must not spuriously produce
// any monster appear/disappear events (regression guard for the new code
// path being additive, not a behavior change to the existing player case).
func (s *MonsterVisibilitySuite) TestPlayerPlayerMoveNeverEmitsMonsterEvents() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: -10, R: 0, S: 10}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 4,
	}))

	s.Require().NoError(s.enc.Move("alice", []core.Hex{{Q: 3, R: 0, S: -3}}))

	aliceEvts := collectEventsTyped(s.aliceSub, 500*time.Millisecond)
	for _, evt := range aliceEvts {
		if _, ok := evt.(*events.EntityAppearedEvent); ok {
			s.Fail("no monsters exist in this encounter — EntityAppearedEvent must not fire")
		}
	}
}
