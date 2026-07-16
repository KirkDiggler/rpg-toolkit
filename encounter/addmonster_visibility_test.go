package encounter_test

// addmonster_visibility_test.go covers rpg-toolkit#764: AddMonster into a
// player's existing LoS starts combat but previously emitted no
// EntityAppearedEvent, leaving the client to enter combat against an entity
// it was never told exists. Companion to monster_visibility_test.go (#761),
// which covers the player-Move-triggered direction.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
)

type AddMonsterVisibilitySuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
	aliceSub  *encounter.Subscription
	bobSub    *encounter.Subscription
}

func TestAddMonsterVisibilitySuite(t *testing.T) {
	suite.Run(t, new(AddMonsterVisibilitySuite))
}

func (s *AddMonsterVisibilitySuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New(context.Background(), "enc-addmonster-vis", s.broker)

	var err error
	s.aliceSub, err = s.broker.Subscribe("enc-addmonster-vis", "alice")
	s.Require().NoError(err)
	s.bobSub, err = s.broker.Subscribe("enc-addmonster-vis", "bob")
	s.Require().NoError(err)
}

func (s *AddMonsterVisibilitySuite) TearDownTest() {
	_ = s.aliceSub.Close()
	_ = s.bobSub.Close()
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TestAddMonster_IntoVisibleRange_AppearsBeforeModeChanged pins the issue's
// core scenario: a monster spawned into a player's existing LoS fires
// EntityAppearedEvent, sequenced strictly before the ModeChangedEvent that
// same AddMonster call triggers — matching #762's ordering contract.
func (s *AddMonsterVisibilitySuite) TestAddMonster_IntoVisibleRange_AppearsBeforeModeChanged() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 4,
	}))
	s.Require().Equal(core.ModeFreeRoam, s.enc.Mode())

	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-1", Position: core.Hex{Q: 2, R: 0, S: -2}, HP: 7, MaxHP: 7,
	}))
	s.Equal(core.ModeTurnBased, s.enc.Mode(), "goblin visible at add time — combat must start")

	aliceEvts := collectEventsTyped(s.aliceSub, 500*time.Millisecond)

	appearedIdx := -1
	modeChangedIdx := -1
	for i, evt := range aliceEvts {
		switch e := evt.(type) {
		case *events.EntityAppearedEvent:
			if e.Entity == core.EntityID("goblin-1") {
				appearedIdx = i
				s.Equal(core.Hex{Q: 2, R: 0, S: -2}, e.Position,
					"appeared Position must be the monster's own fixed hex")
				s.Contains(e.PerPlayer, core.PlayerID("alice"))
			}
		case *events.ModeChangedEvent:
			modeChangedIdx = i
		}
	}
	s.Require().GreaterOrEqual(appearedIdx, 0, "goblin-1 must have appeared to alice")
	s.Require().GreaterOrEqual(modeChangedIdx, 0, "AddMonster must have triggered combat entry")
	s.Less(appearedIdx, modeChangedIdx,
		"EntityAppearedEvent must precede ModeChangedEvent (appearance before the mode change it caused)")
}

// TestAddMonster_OutOfRange_NoAppearedEvent is the regression guard: a
// monster added outside every player's LoS must not fire
// EntityAppearedEvent (and, unchanged from before #764, must not trigger
// combat entry either).
func (s *AddMonsterVisibilitySuite) TestAddMonster_OutOfRange_NoAppearedEvent() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-1", Position: core.Hex{Q: 20, R: 0, S: -20}, HP: 7, MaxHP: 7,
	}))
	s.Equal(core.ModeFreeRoam, s.enc.Mode(), "goblin out of range — must not start combat")

	aliceEvts := collectEventsTyped(s.aliceSub, 300*time.Millisecond)
	for _, evt := range aliceEvts {
		if _, ok := evt.(*events.EntityAppearedEvent); ok {
			s.Fail("goblin is out of alice's LoS — EntityAppearedEvent must not fire")
		}
	}
}

// TestAddMonster_MidCombatReinforcement_AppearsWithNoModeChange covers the
// wave-2 case the issue calls out directly: a monster added while the
// encounter is ALREADY TURN_BASED (a reinforcement, or a door opening onto
// an occupied room) must still fire EntityAppearedEvent for any player who
// can already see it, even though checkCombatEntry's ModeFreeRoam gate makes
// the mode-change part a no-op (no second ModeChangedEvent should fire).
func (s *AddMonsterVisibilitySuite) TestAddMonster_MidCombatReinforcement_AppearsWithNoModeChange() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: 0, R: 0, S: 0}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-1", Position: core.Hex{Q: 1, R: 0, S: -1}, HP: 7, MaxHP: 7,
	}))
	s.Require().Equal(core.ModeTurnBased, s.enc.Mode(), "first goblin starts combat")
	_ = collectEventsTyped(s.aliceSub, 300*time.Millisecond) // drain the first goblin's appear + mode change

	// A second goblin, also within alice's sight range, added mid-combat.
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-2", Position: core.Hex{Q: 2, R: 0, S: -2}, HP: 7, MaxHP: 7,
	}))
	s.Equal(core.ModeTurnBased, s.enc.Mode(), "still turn-based — no re-flip")

	aliceEvts := collectEventsTyped(s.aliceSub, 500*time.Millisecond)

	var found *events.EntityAppearedEvent
	for _, evt := range aliceEvts {
		switch e := evt.(type) {
		case *events.EntityAppearedEvent:
			if e.Entity == core.EntityID("goblin-2") {
				found = e
			}
		case *events.ModeChangedEvent:
			s.Fail("checkCombatEntry's mode gate must no-op mid-combat — no second ModeChangedEvent")
		}
	}
	s.Require().NotNil(found, "goblin-2 must have appeared to alice even though the mode gate no-ops")
	s.Equal(core.Hex{Q: 2, R: 0, S: -2}, found.Position)
	s.Contains(found.PerPlayer, core.PlayerID("alice"))
}

// TestAddMonster_VisibleToMultiplePlayers_GroupsIntoOneEvent: when a new
// monster is visible to more than one player at once (only possible via
// AddMonster — a single player Move can only ever change that ONE player's
// own visibility), all seeing players are grouped into a single
// EntityAppearedEvent's PerPlayer set, since the monster's Position is the
// same fixed hex for every viewer (mirroring the hex-grouping the
// player-sees-player path already does for its appearedByHex map).
func (s *AddMonsterVisibilitySuite) TestAddMonster_VisibleToMultiplePlayers_GroupsIntoOneEvent() {
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "char-alice",
		Position: core.Hex{Q: -1, R: 0, S: 1}, SightRange: 4,
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: "char-bob",
		Position: core.Hex{Q: 1, R: 0, S: -1}, SightRange: 4,
	}))

	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-1", Position: core.Hex{Q: 0, R: 0, S: 0}, HP: 7, MaxHP: 7,
	}))

	aliceEvts := collectEventsTyped(s.aliceSub, 500*time.Millisecond)
	bobEvts := collectEventsTyped(s.bobSub, 500*time.Millisecond)

	aliceAppeared := s.assertHasType(aliceEvts, "*events.EntityAppearedEvent").(*events.EntityAppearedEvent)
	bobAppeared := s.assertHasType(bobEvts, "*events.EntityAppearedEvent").(*events.EntityAppearedEvent)

	// Both subscriptions observe the SAME underlying event (same sequence,
	// same PerPlayer set) — the broker fans one publish out to every
	// audience member rather than the SDK publishing per-viewer duplicates.
	s.Equal(aliceAppeared.Sequence(), bobAppeared.Sequence())
	s.Contains(aliceAppeared.PerPlayer, core.PlayerID("alice"))
	s.Contains(aliceAppeared.PerPlayer, core.PlayerID("bob"))
	s.Len(aliceAppeared.PerPlayer, 2)
}

// assertHasType returns the first event of the given type name, or fails.
// Mirrors visibility_transition_test.go's helper of the same shape (that one
// is a *VisibilityTransitionSuite method; this suite needs its own).
func (s *AddMonsterVisibilitySuite) assertHasType(evts []events.EncounterEvent, typeName string) events.EncounterEvent {
	s.T().Helper()
	for _, e := range evts {
		if typeNameOf(e) == typeName {
			return e
		}
	}
	names := make([]string, 0, len(evts))
	for _, e := range evts {
		names = append(names, typeNameOf(e))
	}
	s.FailNowf("event not found", "want %s, got %v", typeName, names)
	return nil
}
