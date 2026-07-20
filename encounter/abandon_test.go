package encounter_test

// rpg-toolkit#797 (rpg-api#663) — administrative Encounter.End(reason),
// reusing checkEncounterEnd's terminal transition rather than a parallel
// path. Coverage:
//
//  1. Mid-combat: End("abandoned") drives the same Mode -> ModeEnded /
//     Initiative-cleared / EncounterEndedEvent transition a natural
//     victory or TPK end does, the state persists as ended through
//     ToData/LoadFromData, and post-load the encounter refuses further
//     verbs (TakeAction/EndTurn/NPCAct) exactly like a naturally-ended one.
//  2. FREE_ROAM: the actual reported bug shape (#483's movement deadlock)
//     — an encounter that never entered combat, so checkEncounterEnd's
//     victory/TPK predicates (only evaluated from killEntity/
//     publishPlayerDied) can never fire. End must work anyway, since it is
//     unconditional on Mode.
//  3. Negative control: End against an already-ended encounter (whether
//     ended by a prior End call or by a natural end) returns
//     ErrEncounterEnded and does not re-publish EncounterEndedEvent.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// AbandonSuite covers the administrative End(reason) path.
type AbandonSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestAbandonSuite(t *testing.T) {
	suite.Run(t, new(AbandonSuite))
}

func (s *AbandonSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *AbandonSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// TestEnd_MidCombat_Abandoned_EndsEncounter_PersistsThroughReload_RejectsPostLoadVerbs
// is the goal-behavior proof: a mid-combat encounter (player + monster in
// mutual LoS, auto-entered TURN_BASED) is administratively ended via
// End("abandoned"). Asserts the full observable contract: Mode flips,
// Initiative/ActiveIdx/Round clear, EncounterEndedEvent carries Reason
// "abandoned", the ended state round-trips through ToData -> JSON ->
// LoadFromData, and — critically for the resume/liveness bug this closes —
// the reloaded encounter rejects TakeAction/EndTurn/NPCAct with
// ErrEncounterEnded exactly as a naturally-ended encounter does
// (TestSlice_PostEndRoundTrips, death_test.go).
func (s *AbandonSuite) TestEnd_MidCombat_Abandoned_EndsEncounter_PersistsThroughReload_RejectsPostLoadVerbs() {
	encID := core.EncounterID("enc-abandon-midcombat")
	enc := encounter.New(s.ctx, encID, s.broker, encounter.WithRoller(fixedMaxRoller{}),
		encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))
	s.Require().Equal(core.ModeTurnBased, enc.Mode(),
		"alice is in LoS of the goblin — AddMonster must auto-enter combat")

	sub, err := s.broker.Subscribe(encID, "alice")
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	endErr := enc.End(encounter.EncounterEndedReasonAbandoned)
	s.Require().NoError(endErr)

	s.Equal(core.ModeEnded, enc.Mode())
	persisted := enc.ToData()
	s.Empty(persisted.Initiative)
	s.Equal(0, persisted.ActiveIdx)
	s.Equal(0, persisted.Round)
	// The monster is untouched by an abandon — nobody won or lost, so
	// unlike a victory end there is no reason to have cleared Monsters.
	s.Contains(persisted.Monsters, core.EntityID(gobEntityID))

	ended := waitForEncounterEnded(s.T(), sub)
	s.Require().NotNil(ended, "End must publish EncounterEndedEvent")
	s.Equal(encounter.EncounterEndedReasonAbandoned, ended.Reason)

	// Persistence: ended state survives a real JSON round-trip + LoadFromData,
	// the same path rpg-api's per-RPC load/save cycle exercises.
	raw, err := json.Marshal(persisted)
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.Equal(core.ModeEnded, data.Mode, "Mode=ENDED must be the persisted marker — no separate EndedAt field exists")

	reloaded, err := encounter.LoadFromData(s.ctx, &data, s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
		encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}))
	s.Require().NoError(err)
	s.Equal(core.ModeEnded, reloaded.Mode(), "ended state must survive LoadFromData")

	// Post-load: the reloaded, abandoned encounter refuses every combat verb
	// exactly like a naturally-ended one — this is the actual fix for the
	// reported bug (resume/liveness re-imprisoning players in a stuck
	// encounter with no escape).
	err = reloaded.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.ErrorIs(err, encounter.ErrEncounterEnded)

	_, _, err = reloaded.EndTurn(s.ctx, aliceEntityID)
	s.ErrorIs(err, encounter.ErrEncounterEnded)

	err = reloaded.NPCAct(s.ctx, gobEntityID)
	s.ErrorIs(err, encounter.ErrEncounterEnded)
}

// TestEnd_FreeRoam_Abandoned_EndsEncounter proves End works on the actual
// shape of the reported bug: an encounter that never entered combat at all
// (no monster ever added, Mode stays the New()-default ModeFreeRoam).
// checkEncounterEnd's victory/TPK predicates are only ever evaluated from
// killEntity/publishPlayerDied — a FREE_ROAM encounter with no combat
// activity structurally never reaches either, so this proves End's "works
// from ANY mode, not just TURN_BASED" doc claim rather than just asserting
// it.
func (s *AbandonSuite) TestEnd_FreeRoam_Abandoned_EndsEncounter() {
	encID := core.EncounterID("enc-abandon-freeroam")
	enc := encounter.New(s.ctx, encID, s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
	}))
	s.Require().Equal(core.ModeFreeRoam, enc.Mode(), "setup sanity: no monster ever added, must stay FREE_ROAM")

	sub, err := s.broker.Subscribe(encID, "alice")
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	s.Require().NoError(enc.End(encounter.EncounterEndedReasonAbandoned))
	s.Equal(core.ModeEnded, enc.Mode())

	ended := waitForEncounterEnded(s.T(), sub)
	s.Require().NotNil(ended, "End must publish EncounterEndedEvent even from FREE_ROAM")
	s.Equal(encounter.EncounterEndedReasonAbandoned, ended.Reason)
}

// TestEnd_AlreadyEnded_ReturnsErrEncounterEnded_NegativeControl is the
// negative control: End against an already-ended encounter — whether ended
// by a prior End call or by reaching a natural conclusion — returns
// ErrEncounterEnded rather than silently re-transitioning or double-
// publishing EncounterEndedEvent.
func (s *AbandonSuite) TestEnd_AlreadyEnded_ReturnsErrEncounterEnded_NegativeControl() {
	s.Run("already ended by a prior End call", func() {
		encID := core.EncounterID("enc-abandon-double-end")
		enc := encounter.New(s.ctx, encID, s.broker)
		s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
			PlayerID: "alice", EntityID: aliceEntityID,
			Position: core.Hex{}, SightRange: 10,
		}))

		sub, err := s.broker.Subscribe(encID, "alice")
		s.Require().NoError(err)
		defer func() { _ = sub.Close() }()

		s.Require().NoError(enc.End(encounter.EncounterEndedReasonAbandoned))
		drainSub(sub, 100*time.Millisecond)

		err = enc.End(encounter.EncounterEndedReasonAbandoned)
		s.ErrorIs(err, encounter.ErrEncounterEnded)

		// No second EncounterEndedEvent — the guard must short-circuit
		// before endWithReason runs a second time, not merely error after
		// re-publishing.
		seen := collectTypes(sub, 300*time.Millisecond)
		s.NotContains(seen, "*events.EncounterEndedEvent",
			"a rejected second End must not re-publish the terminal event")
	})

	s.Run("already ended naturally (victory)", func() {
		encID := core.EncounterID("enc-abandon-after-victory")
		enc := encounter.New(s.ctx, encID, s.broker, encounter.WithRoller(fixedMaxRoller{}),
			encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}))
		s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
			PlayerID: "alice", EntityID: aliceEntityID,
			Position: core.Hex{}, SightRange: 10,
			HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
			DamageDice: damage1d8plus2, DamageType: damageSlashing,
		}))
		s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
			ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
			HP: 1, MaxHP: 7, AC: 15, Speed: 6,
			AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
		}))

		sub, err := s.broker.Subscribe(encID, "alice")
		s.Require().NoError(err)
		defer func() { _ = sub.Close() }()

		s.Require().NoError(enc.TakeAction("alice",
			encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
			encounter.ActionTarget{EntityID: gobEntityID},
		))
		s.Require().Equal(core.ModeEnded, enc.Mode(), "setup must reach a natural victory end")

		original := waitForEncounterEnded(s.T(), sub)
		s.Require().NotNil(original)
		s.Equal(encounter.EncounterEndedReasonAllHostilesDefeated, original.Reason,
			"setup sanity: must have ended as a victory, not by whatever this test is trying to reject")

		endErr := enc.End(encounter.EncounterEndedReasonAbandoned)
		s.ErrorIs(endErr, encounter.ErrEncounterEnded,
			"abandoning an encounter that already ended naturally must reject the same way")
		s.Equal(core.ModeEnded, enc.Mode())

		// The rejected abandon attempt must not have published a second
		// EncounterEndedEvent overwriting the original victory reason.
		seen := collectTypes(sub, 300*time.Millisecond)
		s.NotContains(seen, "*events.EncounterEndedEvent",
			"a rejected End after a natural end must not re-publish the terminal event")
	})
}
