package encounter_test

// rpg-toolkit#714 — the Move verb never accounted movement economy:
// movement_remaining never decremented, no budget cap (a second move could
// land in full after the mover's speed was already spent), and no
// TurnStateChanged followed a move so the client never saw the new budget.
//
// These tests exercise the real hydrated path (LoadFromData → held
// *character.Character, mirrors TurnStateSuite's loadedMonkEncounter) rather
// than a stub, because the bug lives in how Encounter.Move reads/writes the
// character's own action economy — a stub combatant has none.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// revealVisible seeds view's memory with a minimal VISIBLE observation for
// every hex within sightRange of pos — a fixture convenience shared by this
// file and turn_state_test.go for tests that need SOME reveal state to
// exist (so LoS-gated checks pass) but aren't testing fog-of-war content
// itself. Production code never does this: every real reveal path builds
// full world-truth observations via Encounter.refreshObservations
// (rpg-toolkit#851) — this only sets Position/State, deliberately leaving
// Terrain/ZoneID/Edges/Contents at their zero values, since these fixtures
// don't need or check them.
func revealVisible(view *perception.View, pos core.Hex, sightRange int) {
	for h := range perception.VisibleHexesAt(pos, sightRange, nil) {
		view.Observe(perception.HexObservation{Position: h, State: perception.KnowledgeStateVisible})
	}
}

const (
	moveEconPlayerID = core.PlayerID("charli")
	moveEconEntityID = core.EntityID("char-charli")
	moveEconDamage   = "1d8"
)

// MoveEconomySuite proves #714: a hydrated, in-combat mover's Move() spends
// its distance from MovementRemaining, rejects an over-budget request
// outright, and pushes a TurnStateChanged carrying the new budget.
type MoveEconomySuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestMoveEconomySuite(t *testing.T) {
	suite.Run(t, new(MoveEconomySuite))
}

func (s *MoveEconomySuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *MoveEconomySuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// charliCharJSON builds a level-1 Human Fighter character.Data blob (30ft
// base speed) without a seeded ActionEconomy — SetMode(TurnBased) seeds it.
func (s *MoveEconomySuite) charliCharJSON() json.RawMessage {
	s.T().Helper()
	charData := &dnd5eCharacter.Data{
		ID:       string(moveEconEntityID),
		PlayerID: string(moveEconPlayerID),
		Name:     "Charli the Fighter",
		Level:    1,
		ClassID:  classes.Fighter,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:        12,
		MaxHitPoints:     12,
		ArmorClass:       16,
		ProficiencyBonus: 2,
	}
	raw, err := json.Marshal(charData)
	s.Require().NoError(err)
	return raw
}

// loadedEncounter builds a TURN_BASED encounter with one hydrated Fighter
// seat via LoadFromData, then flips to turn-based so StartTurn seeds the
// economy (30ft movement) from the character's own speed.
func (s *MoveEconomySuite) loadedEncounter() *encounter.Encounter {
	s.T().Helper()
	view := perception.NewView(moveEconPlayerID, core.Hex{}, 10)
	revealVisible(view, core.Hex{}, 10)
	data := encounter.NewData("enc-move-econ")
	data.Players[moveEconPlayerID] = &encounter.PlayerData{
		ID:         moveEconPlayerID,
		EntityID:   moveEconEntityID,
		View:       view,
		HP:         12,
		MaxHP:      12,
		AC:         16,
		DamageDice: moveEconDamage,
		DamageType: damageSlashing,
		DataJSON:   s.charliCharJSON(),
	}
	enc, err := encounter.LoadFromData(s.ctx, data, s.broker)
	s.Require().NoError(err)
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	s.Require().Equal(moveEconEntityID, enc.ActiveActor())
	return enc
}

// hexPath returns a straight-line, hex-by-hex path of n adjacent steps along
// the Q axis starting from the origin — n hexes, n * 5ft.
func hexPath(n int) []core.Hex {
	path := make([]core.Hex, 0, n)
	for i := 1; i <= n; i++ {
		path = append(path, core.Hex{Q: i, R: 0, S: -i})
	}
	return path
}

// TestMove_DeductsDistanceFromMovementRemaining is the headline fix: a move
// must spend its distance, not leave the budget untouched (issue evidence:
// movement_remaining stayed 30 for the entire turn).
func (s *MoveEconomySuite) TestMove_DeductsDistanceFromMovementRemaining() {
	enc := s.loadedEncounter()

	pre := enc.ActorTurnState(moveEconEntityID)
	s.Require().NotNil(pre.Economy)
	s.Equal(30, pre.Economy.MovementRemaining, "Human fighter seeds 30ft")

	// One hex = 5ft.
	s.Require().NoError(enc.Move(moveEconPlayerID, hexPath(1)))

	post := enc.ActorTurnState(moveEconEntityID)
	s.Require().NotNil(post.Economy)
	s.Equal(25, post.Economy.MovementRemaining, "a 1-hex move must spend 5ft")
}

// TestMove_OverBudget_RejectedOutright reproduces the exact playtest bug: a
// 7-hex (35ft) move must not land in full on a 30ft speed. It must be
// rejected — no position change, no economy mutation — not silently accepted.
func (s *MoveEconomySuite) TestMove_OverBudget_RejectedOutright() {
	enc := s.loadedEncounter()

	err := enc.Move(moveEconPlayerID, hexPath(7)) // 35ft > 30ft budget
	s.Require().Error(err, "a move costing more than the remaining budget must be rejected")
	s.ErrorIs(err, encounter.ErrInsufficientMovement)

	post := enc.ActorTurnState(moveEconEntityID)
	s.Require().NotNil(post.Economy)
	s.Equal(30, post.Economy.MovementRemaining, "a rejected move must not touch the budget")
}

// TestMove_SequentialMoves_AccumulateAgainstBudget is the precise repro from
// the playtest: 1 hex spent (25ft left), then a 7-hex (35ft) move — which the
// bug accepted in full for 40ft total on a 30ft speed. It must now be
// rejected against the 25ft actually remaining, and the position must stay
// at the end of the first (accepted) move.
func (s *MoveEconomySuite) TestMove_SequentialMoves_AccumulateAgainstBudget() {
	enc := s.loadedEncounter()

	s.Require().NoError(enc.Move(moveEconPlayerID, hexPath(1)))
	mid := enc.ActorTurnState(moveEconEntityID)
	s.Require().Equal(25, mid.Economy.MovementRemaining)

	err := enc.Move(moveEconPlayerID, hexPath(8)) // from hex 1: 7 more hexes = 35ft
	s.Require().Error(err)
	s.ErrorIs(err, encounter.ErrInsufficientMovement)

	post := enc.ActorTurnState(moveEconEntityID)
	s.Equal(25, post.Economy.MovementRemaining, "the rejected second move must not spend further")
}

// TestMove_PublishesTurnStateChangedAfterSuccessfulMove proves the client
// sees the new movement_remaining live (North-Star Invariant 12) instead of
// going silently stale — the issue's third symptom.
func (s *MoveEconomySuite) TestMove_PublishesTurnStateChangedAfterSuccessfulMove() {
	enc := s.loadedEncounter()

	sub, err := s.broker.Subscribe("enc-move-econ", moveEconPlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	s.Require().NoError(enc.Move(moveEconPlayerID, hexPath(2))) // 10ft

	var ts *events.TurnStateChangedEvent
	deadline := time.After(500 * time.Millisecond)
	for ts == nil {
		select {
		case evt := <-sub.Events():
			if e, ok := evt.(*events.TurnStateChangedEvent); ok {
				ts = e
			}
		case <-deadline:
			s.FailNow("no TurnStateChangedEvent received after Move")
		}
	}
	s.Equal(moveEconEntityID, ts.ActorID)
	s.Equal(20, ts.State.MovementRemaining, "pushed snapshot must carry the post-move budget")
}

// TestMove_NotInCombat_SkipsEconomyEnforcement proves Move stays usable for a
// hydrated character with no seeded turn economy (free-roam / pre-combat) —
// the fix must not regress non-combat movement.
func (s *MoveEconomySuite) TestMove_NotInCombat_SkipsEconomyEnforcement() {
	view := perception.NewView(moveEconPlayerID, core.Hex{}, 10)
	revealVisible(view, core.Hex{}, 10)
	data := encounter.NewData("enc-move-econ-free")
	data.Players[moveEconPlayerID] = &encounter.PlayerData{
		ID:         moveEconPlayerID,
		EntityID:   moveEconEntityID,
		View:       view,
		HP:         12,
		MaxHP:      12,
		AC:         16,
		DamageDice: moveEconDamage,
		DamageType: damageSlashing,
		DataJSON:   s.charliCharJSON(),
	}
	enc, err := encounter.LoadFromData(s.ctx, data, s.broker)
	s.Require().NoError(err)
	// Mode intentionally left as-is (not TurnBased) — no economy seeded.

	s.Require().NoError(enc.Move(moveEconPlayerID, hexPath(20)), "no economy to enforce outside combat")

	ts := enc.ActorTurnState(moveEconEntityID)
	s.Nil(ts.Economy, "still no economy after a free-roam move")
}
