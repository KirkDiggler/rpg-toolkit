package encounter_test

// rpg-project#75 (Beat 2) / rpg-toolkit#699 — turn-start revival regression
// coverage. seedActorTurn (turn_economy.go) now publishes dnd5eEvents.
// TurnStartTopic on the encounter-held bus, mirroring EndTurn's existing
// TurnEndTopic publish. This wakes THREE dormant subscribers, not just
// Dodging: DodgingCondition's self-removal, Barbarian RecklessAttackCondition's
// self-removal, and UnconsciousCondition's auto-death-save roll (the mechanism
// behind rpg-api#612's death-gate playtest beat). These tests prove all three
// fire at the OWNING character's own next turn start and NOT at another
// actor's turn start, on the real cascade-hydrated encounter bus (not a bare
// condition-level unit test).
//
// Initiative order is roll-dependent (see hydration_test.go's own comment on
// this), so tests either drive to a known actor via advanceUntilActive before
// asserting, or branch on which actor rolled first rather than assuming a
// fixed order.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	tkenc "github.com/KirkDiggler/rpg-toolkit/encounter"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
	dnd5events "github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eConditions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

const (
	tsrDodgerPlayerID = encountercore.PlayerID("tsr-dodger")
	tsrDodgerEntityID = encountercore.EntityID("char-tsr-dodger")
	tsrBuddyPlayerID  = encountercore.PlayerID("tsr-buddy")
	tsrBuddyEntityID  = encountercore.EntityID("char-tsr-buddy")
	tsrBarbPlayerID   = encountercore.PlayerID("tsr-barb")
	tsrBarbEntityID   = encountercore.EntityID("char-tsr-barb")
	tsrDownedPlayerID = encountercore.PlayerID("tsr-downed")
	tsrDownedEntityID = encountercore.EntityID("char-tsr-downed")
)

// TurnStartRevivalSuite exercises the revived TurnStartTopic publish end to
// end on the real cascade-hydrated encounter bus.
type TurnStartRevivalSuite struct {
	suite.Suite
	ctx       context.Context
	transport *tkenc.InMemoryTransport
	broker    *tkenc.Broker
}

func TestTurnStartRevivalSuite(t *testing.T) {
	suite.Run(t, new(TurnStartRevivalSuite))
}

func (s *TurnStartRevivalSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = tkenc.NewInMemoryTransport()
	s.broker = tkenc.NewBroker(s.transport)
}

func (s *TurnStartRevivalSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// charDataJSON builds a minimal serialized dnd5e character.Data for a
// combat-ready fighter (Dodge is a universal combat ability, wired onto every
// character regardless of class), optionally carrying persisted conditions so
// the LoadFromData cascade reconstitutes + Apply()s them onto the held bus.
func (s *TurnStartRevivalSuite) charDataJSON(id, playerID string, conds ...json.RawMessage) json.RawMessage {
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
		ActionEconomy: &dnd5eCharacter.ActionEconomyData{
			TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
			ReactionsRemaining: 1, MovementRemaining: 30,
		},
		Conditions: conds,
	}
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	return raw
}

// loadEncounter builds a fresh encounter, adds the given players, then
// round-trips through ToData/LoadFromData so the cascade hydrates each
// player's character (and any persisted conditions) onto the held bus exactly
// once — mirroring hydration_test.go's loadEncounterWithRogue pattern.
func (s *TurnStartRevivalSuite) loadEncounter(players ...tkenc.PlayerInput) *tkenc.Encounter {
	s.T().Helper()
	enc := tkenc.New(s.ctx, "enc-turnstart-revival", s.broker)
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
func (s *TurnStartRevivalSuite) advanceUntilActive(enc *tkenc.Encounter, target encountercore.EntityID) {
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

// attackHasDisadvantageFromDodging publishes a synthetic AttackChainEvent
// targeting the given entity and reports whether Dodging's disadvantage
// modifier is present on the resolved chain — the same publish/execute
// pattern conditions/dodging_test.go uses, run here on the encounter-held bus.
func (s *TurnStartRevivalSuite) attackHasDisadvantageFromDodging(
	bus dnd5events.EventBus, targetID encountercore.EntityID,
) bool {
	s.T().Helper()
	evt := dnd5eEvents.AttackChainEvent{
		AttackerID: "attacker-x",
		TargetID:   string(targetID),
		IsMelee:    true,
	}
	chain := dnd5events.NewStagedChain[dnd5eEvents.AttackChainEvent](dnd5eCombat.ModifierStages)
	modified, err := dnd5eEvents.AttackChain.On(bus).PublishWithChain(s.ctx, evt, chain)
	s.Require().NoError(err)
	final, err := modified.Execute(s.ctx, evt)
	s.Require().NoError(err)
	for _, src := range final.DisadvantageSources {
		if src.SourceRef != nil && src.SourceRef.String() == refs.Conditions.Dodging().String() {
			return true
		}
	}
	return false
}

// attackHasRecklessAdvantage publishes a synthetic AttackChainEvent targeting
// the barbarian and reports whether Reckless Attack's "target is reckless"
// advantage modifier is present.
func (s *TurnStartRevivalSuite) attackHasRecklessAdvantage(
	bus dnd5events.EventBus, targetID encountercore.EntityID,
) bool {
	s.T().Helper()
	evt := dnd5eEvents.AttackChainEvent{
		AttackerID: "attacker-x",
		TargetID:   string(targetID),
		IsMelee:    true,
	}
	chain := dnd5events.NewStagedChain[dnd5eEvents.AttackChainEvent](dnd5eCombat.ModifierStages)
	modified, err := dnd5eEvents.AttackChain.On(bus).PublishWithChain(s.ctx, evt, chain)
	s.Require().NoError(err)
	final, err := modified.Execute(s.ctx, evt)
	s.Require().NoError(err)
	for _, src := range final.AdvantageSources {
		if src.SourceRef != nil && src.SourceRef.String() == refs.Conditions.RecklessAttack().String() {
			return true
		}
	}
	return false
}

// TestDodge_LiveActivation_ExpiresAtOwnersNextTurnStart_NotAtBuddysTurnStart
// is the goal-behavior proof: Dodge, activated live through TakeAction on the
// real encounter, grants disadvantage immediately, survives another actor's
// turn start, and is removed exactly at the dodger's own next turn start.
func (s *TurnStartRevivalSuite) TestDodge_LiveActivation_ExpiresAtOwnersNextTurnStart_NotAtBuddysTurnStart() {
	dodgerData := s.charDataJSON(string(tsrDodgerEntityID), string(tsrDodgerPlayerID))
	buddyData := s.charDataJSON(string(tsrBuddyEntityID), string(tsrBuddyPlayerID))

	enc := s.loadEncounter(
		tkenc.PlayerInput{
			PlayerID: tsrDodgerPlayerID, EntityID: tsrDodgerEntityID,
			Position: encountercore.Hex{}, SightRange: 10,
			HP: 20, MaxHP: 20, AC: 15, DataJSON: dodgerData,
		},
		tkenc.PlayerInput{
			PlayerID: tsrBuddyPlayerID, EntityID: tsrBuddyEntityID,
			Position: encountercore.Hex{Q: 1, R: 0, S: -1}, SightRange: 10,
			HP: 20, MaxHP: 20, AC: 15, DataJSON: buddyData,
		},
	)
	s.Require().NoError(enc.SetMode(encountercore.ModeTurnBased))
	s.advanceUntilActive(enc, tsrDodgerEntityID)

	dodgeRef := refs.CombatAbilities.Dodge()
	err := enc.TakeAction(tsrDodgerPlayerID,
		tkenc.ActionRef{Module: dodgeRef.Module, Type: dodgeRef.Type, ID: dodgeRef.ID},
		tkenc.ActionTarget{})
	s.Require().NoError(err)

	bus := enc.EventBus()
	s.Require().NotNil(bus)
	s.True(s.attackHasDisadvantageFromDodging(bus, tsrDodgerEntityID),
		"attack against the dodger should carry disadvantage right after Dodge")

	var removed *dnd5eEvents.ConditionRemovedEvent
	_, err = dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, e dnd5eEvents.ConditionRemovedEvent) error {
			if e.CharacterID == string(tsrDodgerEntityID) {
				removed = &e
			}
			return nil
		})
	s.Require().NoError(err)

	// Buddy's turn starts next — Dodging must NOT be removed.
	_, _, err = enc.EndTurn(s.ctx, tsrDodgerEntityID)
	s.Require().NoError(err)
	s.Equal(tsrBuddyEntityID, enc.ActiveActor())
	s.Nil(removed, "dodging must not be removed at another actor's turn start")
	s.True(s.attackHasDisadvantageFromDodging(bus, tsrDodgerEntityID),
		"dodging must still be active during buddy's turn")

	// The dodger's own next turn starts — Dodging must be removed.
	_, _, err = enc.EndTurn(s.ctx, tsrBuddyEntityID)
	s.Require().NoError(err)
	s.Equal(tsrDodgerEntityID, enc.ActiveActor())
	s.Require().NotNil(removed, "dodging must be removed at the dodger's own next turn start")
	s.Equal(refs.Conditions.Dodging().String(), removed.ConditionRef)
	s.False(s.attackHasDisadvantageFromDodging(bus, tsrDodgerEntityID),
		"dodging should no longer impose disadvantage")
}

// TestRecklessAttack_RegressionRemovedAtOwnersNextTurnStart_NotAtBuddysTurnStart
// is the R2-required regression check: Reckless Attack's own code is
// untouched by this wave, but its self-removal now actually fires once
// turn-start is live. A persisted RecklessAttackCondition (simulating "used on
// a prior turn") must survive another actor's turn start and be removed
// exactly at the barbarian's own next turn start.
func (s *TurnStartRevivalSuite) TestRecklessAttack_RegressionRemovedAtOwnersNextTurnStart_NotAtBuddysTurnStart() {
	recklessCond := dnd5eConditions.NewRecklessAttackCondition(string(tsrBarbEntityID))
	recklessJSON, err := recklessCond.ToJSON()
	s.Require().NoError(err)

	barbData := s.charDataJSON(string(tsrBarbEntityID), string(tsrBarbPlayerID), recklessJSON)
	buddyData := s.charDataJSON(string(tsrBuddyEntityID), string(tsrBuddyPlayerID))

	enc := s.loadEncounter(
		tkenc.PlayerInput{
			PlayerID: tsrBarbPlayerID, EntityID: tsrBarbEntityID,
			Position: encountercore.Hex{}, SightRange: 10,
			HP: 20, MaxHP: 20, AC: 15, DataJSON: barbData,
		},
		tkenc.PlayerInput{
			PlayerID: tsrBuddyPlayerID, EntityID: tsrBuddyEntityID,
			Position: encountercore.Hex{Q: 1, R: 0, S: -1}, SightRange: 10,
			HP: 20, MaxHP: 20, AC: 15, DataJSON: buddyData,
		},
	)

	bus := enc.EventBus()
	s.Require().NotNil(bus)

	var removed *dnd5eEvents.ConditionRemovedEvent
	_, err = dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, e dnd5eEvents.ConditionRemovedEvent) error {
			if e.CharacterID == string(tsrBarbEntityID) {
				removed = &e
			}
			return nil
		})
	s.Require().NoError(err)

	// SetMode seeds the first actor's turn — which one rolled first is
	// non-deterministic, so branch rather than assume an order.
	s.Require().NoError(enc.SetMode(encountercore.ModeTurnBased))

	if enc.ActiveActor() == tsrBarbEntityID {
		// The barbarian rolled first: SetMode's own seed IS the barbarian's own
		// turn start, so the persisted condition is correctly removed immediately.
		s.Require().NotNil(removed, "reckless attack must be removed at the barbarian's own turn start")
		s.Equal(refs.Conditions.RecklessAttack().String(), removed.ConditionRef)
		s.False(s.attackHasRecklessAdvantage(bus, tsrBarbEntityID))
		return
	}

	// Buddy rolled first: the persisted condition must survive buddy's turn start.
	s.Nil(removed, "reckless attack must not be removed at another actor's turn start")
	s.True(s.attackHasRecklessAdvantage(bus, tsrBarbEntityID),
		"reckless attack advantage should still be active during buddy's turn")

	_, _, err = enc.EndTurn(s.ctx, tsrBuddyEntityID)
	s.Require().NoError(err)
	s.Equal(tsrBarbEntityID, enc.ActiveActor())
	s.Require().NotNil(removed, "reckless attack must be removed at the barbarian's own next turn start")
	s.Equal(refs.Conditions.RecklessAttack().String(), removed.ConditionRef)
	s.False(s.attackHasRecklessAdvantage(bus, tsrBarbEntityID))
}

// TestUnconscious_RegressionAutoDeathSaveAtOwnersNextTurnStart_NotAtBuddysTurnStart
// proves the R2-named connection to rpg-api#612: an unconscious character's
// death save now actually auto-rolls at the start of their own turn on the
// live encounter path, and does not roll on another actor's turn start.
func (s *TurnStartRevivalSuite) TestUnconscious_RegressionAutoDeathSaveAtOwnersNextTurnStart_NotAtBuddysTurnStart() {
	unconsciousCond := &dnd5eConditions.UnconsciousCondition{CharacterID: string(tsrDownedEntityID)}
	unconsciousJSON, err := unconsciousCond.ToJSON()
	s.Require().NoError(err)

	downedData := s.charDataJSON(string(tsrDownedEntityID), string(tsrDownedPlayerID), unconsciousJSON)
	buddyData := s.charDataJSON(string(tsrBuddyEntityID), string(tsrBuddyPlayerID))

	enc := s.loadEncounter(
		tkenc.PlayerInput{
			PlayerID: tsrDownedPlayerID, EntityID: tsrDownedEntityID,
			Position: encountercore.Hex{}, SightRange: 10,
			HP: 0, MaxHP: 20, AC: 15, DataJSON: downedData,
		},
		tkenc.PlayerInput{
			PlayerID: tsrBuddyPlayerID, EntityID: tsrBuddyEntityID,
			Position: encountercore.Hex{Q: 1, R: 0, S: -1}, SightRange: 10,
			HP: 20, MaxHP: 20, AC: 15, DataJSON: buddyData,
		},
	)

	bus := enc.EventBus()
	s.Require().NotNil(bus)

	var rolled []dnd5eEvents.DeathSaveRolledEvent
	_, err = dnd5eEvents.DeathSaveRolledTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, e dnd5eEvents.DeathSaveRolledEvent) error {
			rolled = append(rolled, e)
			return nil
		})
	s.Require().NoError(err)

	s.Require().NoError(enc.SetMode(encountercore.ModeTurnBased))

	if enc.ActiveActor() == tsrDownedEntityID {
		// Downed rolled first: SetMode's own seed IS their own turn start.
		s.Require().Len(rolled, 1, "a death save should auto-roll at the downed character's own turn start")
		s.Equal(string(tsrDownedEntityID), rolled[0].CharacterID)
		return
	}

	// Buddy rolled first: no death save should roll on buddy's turn start.
	s.Empty(rolled, "no death save should auto-roll on another actor's turn start")

	_, _, err = enc.EndTurn(s.ctx, tsrBuddyEntityID)
	s.Require().NoError(err)
	s.Equal(tsrDownedEntityID, enc.ActiveActor())
	s.Require().Len(rolled, 1, "a death save should auto-roll at the downed character's own next turn start")
	s.Equal(string(tsrDownedEntityID), rolled[0].CharacterID)
}
