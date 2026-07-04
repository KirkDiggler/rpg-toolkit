package encounter_test

// rpg-toolkit#733 — design intent (rpg-project#75 Beat 2 mechanical-effects
// design doc, "Turn-start is dead code"): "ActionEconomy for an unconscious
// actor should not offer actions." seedActorTurn (turn_economy.go) used to
// unconditionally call char.StartTurn on any player whose turn began,
// including a downed one — reseeding a fresh 1/1/1/30 economy for a player
// who cannot act. This file proves the fix: a downed player's own turn-start
// zeroes the economy (via char.EndTurn) instead of reseeding it, and the
// pushed TurnStateChangedEvent reflects that.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

const (
	tedDownedPlayerID = core.PlayerID("ted-downed")
	tedDownedEntityID = core.EntityID("char-ted-downed")
	tedBuddyPlayerID  = core.PlayerID("ted-buddy")
	tedBuddyEntityID  = core.EntityID("char-ted-buddy")
)

// TurnEconomyDownedSuite proves the seedActorTurn fix.
type TurnEconomyDownedSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestTurnEconomyDownedSuite(t *testing.T) {
	suite.Run(t, new(TurnEconomyDownedSuite))
}

func (s *TurnEconomyDownedSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *TurnEconomyDownedSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// charDataJSONWithSeededEconomy builds a hydratable dnd5e character.Data blob
// carrying a PRE-SEEDED ActionEconomy (1/1/1/30, TurnNumber 1) — simulating a
// character already mid-combat (a real StartTurn already ran) before going
// down. This makes the regression meaningful: pre-fix, seedActorTurn would
// call char.StartTurn and RESEED 1/1/1/30 (visibly nonzero) at the downed
// player's next turn start; post-fix, char.EndTurn zeroes it instead.
func (s *TurnEconomyDownedSuite) charDataJSONWithSeededEconomy(id, playerID string) json.RawMessage {
	s.T().Helper()
	data := &dnd5eCharacter.Data{
		ID:               id,
		PlayerID:         playerID,
		Name:             id,
		Level:            3,
		ProficiencyBonus: 2,
		ClassID:          classes.Fighter,
		RaceID:           races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 14, abilities.DEX: 12, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 10,
		},
		HitPoints:    0,
		MaxHitPoints: 20,
		ArmorClass:   15,
		ActionEconomy: &dnd5eCharacter.ActionEconomyData{
			TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
			ReactionsRemaining: 1, MovementRemaining: 30,
		},
	}
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	return raw
}

// buddyCharDataJSON builds a plain hydratable Fighter with no special state —
// just needs to be a valid seat so the encounter has >1 combatant to cycle
// through.
func (s *TurnEconomyDownedSuite) buddyCharDataJSON(id, playerID string) json.RawMessage {
	s.T().Helper()
	data := &dnd5eCharacter.Data{
		ID:               id,
		PlayerID:         playerID,
		Name:             id,
		Level:            3,
		ProficiencyBonus: 2,
		ClassID:          classes.Fighter,
		RaceID:           races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 14, abilities.DEX: 12, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 10,
		},
		HitPoints:    20,
		MaxHitPoints: 20,
		ArmorClass:   15,
	}
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	return raw
}

// TestDownedPlayerTurnStart_ZeroesEconomy_NotReseeds is the goal-behavior
// proof: at the downed player's own turn start, the pushed
// TurnStateChangedEvent shows ZERO actions/bonus/reactions/movement — not
// the normal freshly-seeded 1/1/1/30 values.
func (s *TurnEconomyDownedSuite) TestDownedPlayerTurnStart_ZeroesEconomy_NotReseeds() {
	downedData := s.charDataJSONWithSeededEconomy(string(tedDownedEntityID), string(tedDownedPlayerID))
	buddyData := s.buddyCharDataJSON(string(tedBuddyEntityID), string(tedBuddyPlayerID))

	encID := core.EncounterID("enc-ted-1")
	enc := encounter.New(s.ctx, encID, s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: tedDownedPlayerID, EntityID: tedDownedEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 0, MaxHP: 20, AC: 15, DataJSON: downedData,
	}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: tedBuddyPlayerID, EntityID: tedBuddyEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1}, SightRange: 10,
		HP: 20, MaxHP: 20, AC: 15, DataJSON: buddyData,
	}))

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker)
	s.Require().NoError(err)

	sub, err := s.broker.Subscribe(encID, tedDownedPlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	// Subscribe before flipping to TURN_BASED so we catch the push
	// regardless of whether the downed player rolls first (SetMode itself
	// seeds the first actor) or second (via one EndTurn cycle).
	s.Require().NoError(loaded.SetMode(core.ModeTurnBased))
	for loaded.ActiveActor() != tedDownedEntityID {
		_, _, endErr := loaded.EndTurn(s.ctx, loaded.ActiveActor())
		s.Require().NoError(endErr)
	}

	seen := collectEventsTyped(sub, 500*time.Millisecond)
	var ts *events.TurnStateChangedEvent
	for _, e := range seen {
		if t, ok := e.(*events.TurnStateChangedEvent); ok {
			ts = t
		}
	}
	s.Require().NotNil(ts, "the downed player's turn start must still push a TurnStateChangedEvent")
	s.Equal(tedDownedEntityID, ts.ActorID)
	s.Equal(0, ts.State.ActionsRemaining, "a downed player's turn start must NOT reseed the standard action")
	s.Equal(0, ts.State.BonusActionsRemaining, "a downed player's turn start must NOT reseed the bonus action")
	s.Equal(0, ts.State.ReactionsRemaining, "a downed player's turn start must NOT reseed the reaction")
	s.Equal(0, ts.State.MovementRemaining, "a downed player's turn start must NOT reseed movement")
}

// TestAlivePlayerTurnStart_StillSeedsNormalEconomy is the negative control:
// an ALIVE player's turn start is completely unaffected by this fix — still
// gets the normal freshly-seeded 1/1/1/30 economy.
func (s *TurnEconomyDownedSuite) TestAlivePlayerTurnStart_StillSeedsNormalEconomy() {
	aliveData := s.buddyCharDataJSON(string(tedBuddyEntityID), string(tedBuddyPlayerID))

	encID := core.EncounterID("enc-ted-2")
	enc := encounter.New(s.ctx, encID, s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: tedBuddyPlayerID, EntityID: tedBuddyEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 20, MaxHP: 20, AC: 15, DataJSON: aliveData,
	}))

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker)
	s.Require().NoError(err)

	sub, err := s.broker.Subscribe(encID, tedBuddyPlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	s.Require().NoError(loaded.SetMode(core.ModeTurnBased))
	s.Require().Equal(tedBuddyEntityID, loaded.ActiveActor())

	seen := collectEventsTyped(sub, 500*time.Millisecond)
	var ts *events.TurnStateChangedEvent
	for _, e := range seen {
		if t, ok := e.(*events.TurnStateChangedEvent); ok {
			ts = t
		}
	}
	s.Require().NotNil(ts)
	s.Equal(1, ts.State.ActionsRemaining)
	s.Equal(1, ts.State.BonusActionsRemaining)
	s.Equal(1, ts.State.ReactionsRemaining)
	s.Positive(ts.State.MovementRemaining)
}
