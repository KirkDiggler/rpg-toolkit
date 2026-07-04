package encounter_test

// rpg-toolkit#734 (rpg-project#75, Beat 2) regression coverage.
//
// Before this fix, dnd5eEvents.ConditionRemovedTopic had zero subscribers
// anywhere in encounter/*.go: a condition's self-removal (e.g. Dodge at the
// dodger's own next turn start) was only ever observable by forcing a fresh
// reconnect and reading it off a snapshot. A client watching the live event
// stream saw the condition get applied (ConditionAppliedTopic was already
// bridged) but never saw it removed.
//
// These tests exercise the fix on the real cascade-hydrated encounter bus
// (LoadFromData), not a bare condition-level unit test: a player activates
// Dodge live via TakeAction, the broker's StatusApplied (ConditionAppliedEvent)
// is observed, turns advance to the dodger's own next turn start, and the
// broker's StatusRemoved (ConditionRemovedEvent) must appear — carrying a
// ConditionRef in the SAME format as the earlier StatusApplied, so a client
// can correlate the pair — and only to viewers with LoS to the dodger.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	tkenc "github.com/KirkDiggler/rpg-toolkit/encounter"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

const (
	crbEncounterID    = encountercore.EncounterID("enc-cond-removed-bridge")
	crbDodgerPlayerID = encountercore.PlayerID("crb-dodger")
	crbDodgerEntityID = encountercore.EntityID("char-crb-dodger")
	crbBuddyPlayerID  = encountercore.PlayerID("crb-buddy")
	crbBuddyEntityID  = encountercore.EntityID("char-crb-buddy")
	crbFarPlayerID    = encountercore.PlayerID("crb-far")
	crbFarEntityID    = encountercore.EntityID("char-crb-far")
)

// ConditionRemovedBridgeSuite exercises the ConditionRemovedTopic -> broker
// ConditionRemovedEvent (StatusRemoved) bridge end to end.
type ConditionRemovedBridgeSuite struct {
	suite.Suite
	ctx       context.Context
	transport *tkenc.InMemoryTransport
	broker    *tkenc.Broker
}

func TestConditionRemovedBridgeSuite(t *testing.T) {
	suite.Run(t, new(ConditionRemovedBridgeSuite))
}

func (s *ConditionRemovedBridgeSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = tkenc.NewInMemoryTransport()
	s.broker = tkenc.NewBroker(s.transport)
}

func (s *ConditionRemovedBridgeSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// crbCharDataJSON builds a minimal serialized dnd5e character.Data for a
// combat-ready fighter (Dodge is a universal combat ability, wired onto
// every character regardless of class).
func crbCharDataJSON(t *testing.T, id, playerID string) json.RawMessage {
	t.Helper()
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
		ActionEconomy: &dnd5eCharacter.ActionEconomyData{
			TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
			ReactionsRemaining: 1, MovementRemaining: 30,
		},
	}
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	return raw
}

// loadEncounter builds a fresh encounter, adds the given players, then
// round-trips through ToData/LoadFromData so the cascade hydrates each
// player's character onto the held bus exactly once — mirroring
// turn_start_revival_test.go's loadEncounter pattern.
func (s *ConditionRemovedBridgeSuite) loadEncounter(players ...tkenc.PlayerInput) *tkenc.Encounter {
	s.T().Helper()
	enc := tkenc.New(s.ctx, crbEncounterID, s.broker)
	for _, p := range players {
		s.Require().NoError(enc.AddPlayer(p))
	}
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data tkenc.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := tkenc.LoadFromData(s.ctx, &data, s.broker)
	s.Require().NoError(err)
	return loaded
}

// advanceUntilActive cycles EndTurn (whoever is currently active) until the
// target entity becomes the active actor.
func (s *ConditionRemovedBridgeSuite) advanceUntilActive(enc *tkenc.Encounter, target encountercore.EntityID) {
	s.T().Helper()
	for i := 0; i < 8; i++ {
		if enc.ActiveActor() == target {
			return
		}
		active := enc.ActiveActor()
		_, _, err := enc.EndTurn(s.ctx, active)
		s.Require().NoError(err)
	}
	s.FailNow("never reached target's turn", "target=%s", target)
}

// advanceToOwnersNextTurn assumes target is currently active, ends its turn,
// then keeps ending whoever is active until control cycles all the way back
// to target — i.e. target's OWN next turn start, regardless of how many
// other actors sit between them in initiative order.
func (s *ConditionRemovedBridgeSuite) advanceToOwnersNextTurn(enc *tkenc.Encounter, target encountercore.EntityID) {
	s.T().Helper()
	s.Require().Equal(target, enc.ActiveActor(), "must be target's turn before advancing to their own NEXT turn")
	for i := 0; i < 8; i++ {
		active := enc.ActiveActor()
		_, _, err := enc.EndTurn(s.ctx, active)
		s.Require().NoError(err)
		if enc.ActiveActor() == target {
			return
		}
	}
	s.FailNow("never returned to target's own next turn", "target=%s", target)
}

// findConditionApplied returns the first *events.ConditionAppliedEvent in
// evts targeting targetID, or nil.
func findConditionApplied(
	evts []events.EncounterEvent, targetID encountercore.EntityID,
) *events.ConditionAppliedEvent {
	for _, evt := range evts {
		if ca, ok := evt.(*events.ConditionAppliedEvent); ok && ca.TargetID == targetID {
			return ca
		}
	}
	return nil
}

// findConditionRemoved returns the first *events.ConditionRemovedEvent in
// evts targeting targetID, or nil.
func findConditionRemoved(
	evts []events.EncounterEvent, targetID encountercore.EntityID,
) *events.ConditionRemovedEvent {
	for _, evt := range evts {
		if cr, ok := evt.(*events.ConditionRemovedEvent); ok && cr.TargetID == targetID {
			return cr
		}
	}
	return nil
}

// TestDodgeRemoval_BridgesToBroker_RefMatchesAppliedEvent_AndAudienceRespectsLoS
// is the goal-behavior proof for rpg-toolkit#734: Dodge activated live through
// TakeAction produces a broker StatusApplied event a buddy with LoS observes;
// advancing to the dodger's own next turn start then produces a broker
// StatusRemoved event on the SAME live stream, with a ConditionRef that
// matches the earlier StatusApplied's format (proving the applied/removed
// pair correlates) — and a viewer with no LoS to the dodger never receives
// either event.
func (s *ConditionRemovedBridgeSuite) TestDodgeRemoval_BridgesToBroker_RefMatchesAppliedEvent_AndAudienceRespectsLoS() {
	dodgerData := crbCharDataJSON(s.T(), string(crbDodgerEntityID), string(crbDodgerPlayerID))

	enc := s.loadEncounter(
		tkenc.PlayerInput{
			PlayerID: crbDodgerPlayerID, EntityID: crbDodgerEntityID,
			Position: encountercore.Hex{Q: 0, R: 0, S: 0}, SightRange: 10,
			HP: 20, MaxHP: 20, AC: 15, DataJSON: dodgerData,
		},
		tkenc.PlayerInput{
			// Adjacent to the dodger — has LoS.
			PlayerID: crbBuddyPlayerID, EntityID: crbBuddyEntityID,
			Position: encountercore.Hex{Q: 1, R: 0, S: -1}, SightRange: 10,
			HP: 20, MaxHP: 20, AC: 15,
		},
		tkenc.PlayerInput{
			// Far away with a short sight range — no LoS to the dodger.
			PlayerID: crbFarPlayerID, EntityID: crbFarEntityID,
			Position: encountercore.Hex{Q: 20, R: 0, S: -20}, SightRange: 1,
			HP: 20, MaxHP: 20, AC: 15,
		},
	)
	s.Require().NoError(enc.SetMode(encountercore.ModeTurnBased))
	s.advanceUntilActive(enc, crbDodgerEntityID)

	buddySub, err := s.broker.Subscribe(crbEncounterID, crbBuddyPlayerID)
	s.Require().NoError(err)
	defer func() { _ = buddySub.Close() }()

	farSub, err := s.broker.Subscribe(crbEncounterID, crbFarPlayerID)
	s.Require().NoError(err)
	defer func() { _ = farSub.Close() }()

	dodgeRef := refs.CombatAbilities.Dodge()
	err = enc.TakeAction(crbDodgerPlayerID,
		tkenc.ActionRef{Module: dodgeRef.Module, Type: dodgeRef.Type, ID: dodgeRef.ID},
		tkenc.ActionTarget{})
	s.Require().NoError(err)

	// StatusApplied: buddy (LoS) must observe it; far (no LoS) must not.
	buddyAppliedEvts := drainEvents(buddySub, time.Second)
	applied := findConditionApplied(buddyAppliedEvts, crbDodgerEntityID)
	s.Require().NotNil(applied, "buddy (LoS to dodger) must observe the StatusApplied event for dodge")
	s.Equal("dodging", applied.ConditionRef, "StatusApplied carries the short ConditionType keyword form")

	farAppliedEvts := drainEvents(farSub, 200*time.Millisecond)
	s.Nil(findConditionApplied(farAppliedEvts, crbDodgerEntityID),
		"far viewer (no LoS to dodger) must not observe the StatusApplied event")

	// Advance to the dodger's own next turn start — Dodging self-removes.
	s.advanceToOwnersNextTurn(enc, crbDodgerEntityID)

	buddyRemovedEvts := drainEvents(buddySub, time.Second)
	removed := findConditionRemoved(buddyRemovedEvts, crbDodgerEntityID)
	s.Require().NotNil(removed, "buddy (LoS to dodger) must observe the StatusRemoved event on the live broker stream")
	s.Equal(crbDodgerEntityID, removed.TargetID)
	s.Equal(applied.ConditionRef, removed.ConditionRef,
		"StatusRemoved's ConditionRef must match StatusApplied's format so a client can correlate the pair")
	s.Equal("turn_start", removed.Reason)

	farRemovedEvts := drainEvents(farSub, 200*time.Millisecond)
	s.Nil(findConditionRemoved(farRemovedEvts, crbDodgerEntityID),
		"far viewer (no LoS to dodger) must not observe the StatusRemoved event")
}
