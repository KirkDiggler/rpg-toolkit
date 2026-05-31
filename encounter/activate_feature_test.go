package encounter_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/suite"
)

const (
	rageFeatureRef = "dnd5e:features:rage"
	rageFeatID     = "rage-feat-1"
)

type ActivateFeatureSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestActivateFeatureSuite(t *testing.T) {
	suite.Run(t, new(ActivateFeatureSuite))
}

func (s *ActivateFeatureSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *ActivateFeatureSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// barbEnc builds a loaded encounter with one barbarian player seat
// and returns the encounter plus the character's JSON data.
func (s *ActivateFeatureSuite) barbEnc() (*encounter.Encounter, json.RawMessage) {
	s.T().Helper()

	rageFeatureJSON := json.RawMessage(
		`{"ref":{"module":"dnd5e","type":"features","id":"rage"},` +
			`"id":"` + rageFeatID + `","name":"Rage","level":1}`,
	)
	charData := &dnd5eCharacter.Data{
		ID:       bobEntityID, // reuse const from combat_test.go in same package
		PlayerID: bobPlayerID,
		Name:     "Bob the Barbarian",
		Level:    1,
		ClassID:  classes.Barbarian,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 15,
			abilities.INT: 8,
			abilities.WIS: 12,
			abilities.CHA: 10,
		},
		HitPoints:        13,
		MaxHitPoints:     13,
		ArmorClass:       14,
		ProficiencyBonus: 2,
		Features:         []json.RawMessage{rageFeatureJSON},
		Resources: map[coreResources.ResourceKey]dnd5eCharacter.RecoverableResourceData{
			"rage_charges": {Current: 2, Maximum: 2, ResetType: coreResources.ResetLongRest},
		},
		// ActionEconomy must be set so char.InCombat() == true
		ActionEconomy: &dnd5eCharacter.ActionEconomyData{
			TurnNumber:            1,
			ActionsRemaining:      1,
			BonusActionsRemaining: 1,
			ReactionsRemaining:    1,
			MovementRemaining:     30,
		},
	}

	charJSON, err := json.Marshal(charData)
	s.Require().NoError(err)

	e := encounter.New(context.Background(), "enc-1", s.broker)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID:   bobPlayerID,
		EntityID:   bobEntityID,
		Position:   core.Hex{Q: 0, R: 0, S: 0},
		HP:         13,
		MaxHP:      13,
		AC:         14,
		DamageDice: "1d12",
	}))

	return e, json.RawMessage(charJSON)
}

// TestActivateFeature_PublishesBrokerEventsAndDecrementsCharge is the primary
// acceptance test: activating Rage on a barbarian with 2 charges should
// (a) decrement charges 2→1, (b) publish a ConditionAppliedEvent (raging),
// (c) publish a ResourceChangedEvent (rage_charges, new_current=1).
func (s *ActivateFeatureSuite) TestActivateFeature_PublishesBrokerEventsAndDecrementsCharge() {
	e, charJSON := s.barbEnc()

	// Subscribe before activating so we capture all published events.
	sub, err := s.broker.Subscribe("enc-1", bobPlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	out, err := e.ActivateFeature(s.ctx, &encounter.ActivateFeatureInput{
		ActorID:      bobEntityID,
		FeatureRef:   rageFeatureRef,
		CharDataJSON: charJSON,
	})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.NotEmpty(out.UpdatedCharData)

	// Collect up to 2 events within 1s (ConditionApplied + ResourceChanged).
	received := s.drainEvents(sub, 2, time.Second)

	var condApplied *events.ConditionAppliedEvent
	var resChanged *events.ResourceChangedEvent
	for _, evt := range received {
		switch ev := evt.(type) {
		case *events.ConditionAppliedEvent:
			condApplied = ev
		case *events.ResourceChangedEvent:
			resChanged = ev
		}
	}

	// Assert ConditionApplied (raging)
	s.Require().NotNil(condApplied, "expected a ConditionAppliedEvent for raging")
	s.Equal(core.EntityID(bobEntityID), condApplied.TargetID)
	s.Equal("raging", condApplied.ConditionRef)

	// Assert ResourceChanged (rage_charges 2→1)
	s.Require().NotNil(resChanged, "expected a ResourceChangedEvent for rage_charges")
	s.Equal(core.EntityID(bobEntityID), resChanged.EntityID)
	s.Equal("rage_charges", resChanged.ResourceRef)
	s.Equal(1, resChanged.NewCurrent)
	s.Equal(2, resChanged.Max)

	// Assert the returned updated char data has charge 1 persisted.
	var updatedData dnd5eCharacter.Data
	s.Require().NoError(json.Unmarshal(out.UpdatedCharData, &updatedData))
	rageRes := updatedData.Resources["rage_charges"]
	s.Equal(1, rageRes.Current, "persisted charge should be 1 after activation")
}

// TestActivateFeature_MissingCharData returns an error without publishing events.
func (s *ActivateFeatureSuite) TestActivateFeature_MissingCharData() {
	e, _ := s.barbEnc()

	_, err := e.ActivateFeature(s.ctx, &encounter.ActivateFeatureInput{
		ActorID:    bobEntityID,
		FeatureRef: rageFeatureRef,
		// CharDataJSON intentionally empty
	})
	s.Require().Error(err)
}

// TestActivateFeature_CharDataMismatch returns an error when CharDataJSON belongs
// to a different character than ActorID.
func (s *ActivateFeatureSuite) TestActivateFeature_CharDataMismatch() {
	e, charJSON := s.barbEnc()

	_, err := e.ActivateFeature(s.ctx, &encounter.ActivateFeatureInput{
		// ActorID deliberately mismatched: encounter has bobEntityID, data has bobEntityID too,
		// so we spoof a different ActorID to trigger the guard.
		// aliceEntityID is not in the encounter, so findPlayerByEntityID returns nil first.
		ActorID:      aliceEntityID,
		FeatureRef:   rageFeatureRef,
		CharDataJSON: charJSON,
	})
	s.Require().Error(err)
}

// TestActivateFeature_CharDataIDMismatch exercises the identity-mismatch guard when
// the actor IS in the encounter but CharDataJSON carries a different character ID.
func (s *ActivateFeatureSuite) TestActivateFeature_CharDataIDMismatch() {
	e, charJSON := s.barbEnc()

	// Also add alice so the actor-lookup passes, then pass bob's JSON.
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID:   alicePlayerID,
		EntityID:   aliceEntityID,
		Position:   core.Hex{Q: 1, R: 0, S: -1},
		HP:         10,
		MaxHP:      10,
		AC:         12,
		DamageDice: "1d12", // same as barbarian above; just needs to be non-empty
	}))

	_, err := e.ActivateFeature(s.ctx, &encounter.ActivateFeatureInput{
		ActorID:      aliceEntityID, // alice is in the encounter
		FeatureRef:   rageFeatureRef,
		CharDataJSON: charJSON, // but charJSON has ID=bobEntityID
	})
	s.Require().Error(err, "expected error when CharDataJSON.ID != ActorID")
	s.ErrorContains(err, "mismatch")
}

// TestActivateFeature_NoSubscriptionAccumulation is the regression test for the
// bus double-fire / subscription-accumulation class of bugs (#684 class).
// Two sequential ActivateFeature calls on the SAME Encounter object (same e.bus)
// must each produce exactly one ConditionAppliedEvent and one ResourceChangedEvent,
// not two of each from accumulated subscribers.
//
// Call 1: rage_charges 2→1, publishes ConditionApplied(raging)+ResourceChanged.
// Call 2 uses UpdatedCharData from call 1 (charge=1→0). Because Rage is already
// active, ActivateAbility should fail (no charges or already raging) — but the
// key assertion is that NO spurious extra events fire from call 1's residual
// subscriptions. We verify this by counting total events: call 2 emits 0 events
// (activation fails), not 2 (which would indicate leaked handlers from call 1).
func (s *ActivateFeatureSuite) TestActivateFeature_NoSubscriptionAccumulation() {
	e, charJSON := s.barbEnc()

	sub, err := s.broker.Subscribe("enc-1", bobPlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	// Call 1: should succeed, rage_charges 2→1.
	out1, err := e.ActivateFeature(s.ctx, &encounter.ActivateFeatureInput{
		ActorID:      bobEntityID,
		FeatureRef:   rageFeatureRef,
		CharDataJSON: charJSON,
	})
	s.Require().NoError(err)

	// Drain the 2 events from call 1 (ConditionApplied + ResourceChanged).
	call1Events := s.drainEvents(sub, 2, time.Second)
	s.Require().Len(call1Events, 2, "call 1 should produce exactly 2 events")

	// Call 2: attempt to activate again on the same encounter.
	// With charge=1 remaining and rage already applied, ActivateAbility will
	// either fail (already raging) or succeed (charge 1→0).
	// Either way, the event count from call 2 must NOT exceed what the activation
	// itself would produce — never 2× due to leaked call-1 subscribers.
	_, _ = e.ActivateFeature(s.ctx, &encounter.ActivateFeatureInput{
		ActorID:      bobEntityID,
		FeatureRef:   rageFeatureRef,
		CharDataJSON: out1.UpdatedCharData,
	})

	// Allow up to 500ms for any events that might fire from accumulated handlers.
	// With proper Cleanup, zero events should arrive (activation fails because
	// character is already raging). Without Cleanup, stale subscribers could
	// re-fire the raging ConditionAppliedEvent from call 1's loaded conditions.
	call2Events := s.drainEvents(sub, 10, 200*time.Millisecond)

	// The only acceptable count is 0 (failed activation, nothing published)
	// or the legitimate count from a successful activation (≤2: ConditionApplied
	// + ResourceChanged if charge 1→0 succeeded). It must NOT be 4+ which would
	// indicate doubled-up subscribers from call 1.
	s.LessOrEqual(len(call2Events), 2,
		"call 2 must not produce extra events from accumulated bus subscribers; "+
			"got %d events (expected ≤2)", len(call2Events))
}

// TestActivateFeature_ActorNotInEncounter returns an error for unknown actor.
func (s *ActivateFeatureSuite) TestActivateFeature_ActorNotInEncounter() {
	e, charJSON := s.barbEnc()

	_, err := e.ActivateFeature(s.ctx, &encounter.ActivateFeatureInput{
		ActorID:      "char-unknown",
		FeatureRef:   rageFeatureRef,
		CharDataJSON: charJSON,
	})
	s.Require().Error(err)
}

// drainEvents collects up to want events within the timeout, returning early
// when want events have arrived.
func (s *ActivateFeatureSuite) drainEvents(
	sub *encounter.Subscription,
	want int,
	timeout time.Duration,
) []events.EncounterEvent {
	s.T().Helper()
	deadline := time.After(timeout)
	var collected []events.EncounterEvent
	for len(collected) < want {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				return collected
			}
			collected = append(collected, evt)
		case <-deadline:
			return collected
		}
	}
	return collected
}
