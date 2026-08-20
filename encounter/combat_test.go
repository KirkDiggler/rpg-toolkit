package encounter_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	toolkitcore "github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
)

// Test-package fixture identifiers (extracted to satisfy goconst).
const (
	alicePlayerID  = "alice"
	bobPlayerID    = "bob"
	aliceEntityID  = "char-alice"
	bobEntityID    = "char-bob"
	gobEntityID    = "goblin-1"
	gob2EntityID   = "goblin-2"
	damageSlashing = "slashing"
	refModuleDnd5e = "dnd5e"
	refTypeAction  = "action"
)

// CombatSuite covers the Wave 2.8 verbs (SetMode, EndTurn, TakeAction,
// NPCAct) and the new combat events. Fixture: alice + bob + goblin-1.
type CombatSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
	aliceSub  *encounter.Subscription
	bobSub    *encounter.Subscription
}

func TestCombatSuite(t *testing.T) {
	suite.Run(t, new(CombatSuite))
}

func (s *CombatSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New(context.Background(), "enc-combat", s.broker,
		encounter.WithStrikeResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)

	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: "1d8+2", DamageType: damageSlashing,
	}))
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: bobEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1}, SightRange: 10,
		HP: 10, MaxHP: 10, AC: 13, AttackBonus: 3,
		DamageDice: "1d6+1", DamageType: damagePiercing,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID,
		// Adjacent (distance 1) to BOTH alice {0,0,0} and bob {1,0,-1} —
		// their shared hex-grid common neighbor — rather than co-located
		// with bob (the original position, distance 0 from him). NPCAct now
		// runs the real monster.TakeTurn AI (rpg-toolkit#895 no-fallback
		// rider deleted the scripted fallback this fixture used to exercise
		// instead), and PerceivedEntity.Adjacent is strictly distance==1, so
		// a co-located target never reads as adjacent and CanActivate
		// rejects it as out of melee range. Distance 1 from alice also
		// keeps the player-attacks-goblin tests in this suite (e.g.
		// TestTakeAction_PublishesStrikeOutcome) in melee reach.
		Position: core.Hex{Q: 1, R: -1, S: 0},
		HP:       7, MaxHP: 7, AC: 15, Speed: 6,
		MonsterRef:  "dnd5e:monsters:goblin",
		DataJSON:    testGoblinDataJSON(s.T(), gobEntityID),
		AttackBonus: 4, DamageDice: "1d6+2", DamageType: damageSlashing,
	}))

	var err error
	s.aliceSub, err = s.broker.Subscribe("enc-combat", "alice")
	s.Require().NoError(err)
	s.bobSub, err = s.broker.Subscribe("enc-combat", "bob")
	s.Require().NoError(err)
}

func (s *CombatSuite) TearDownTest() {
	if s.aliceSub != nil {
		_ = s.aliceSub.Close()
	}
	if s.bobSub != nil {
		_ = s.bobSub.Close()
	}
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// SetMode flip to TURN_BASED rolls initiative, fires ModeChangedEvent +
// TurnStartedEvent, and gates verbs on mode.
// The shared s.enc fixture (SetupTest) has alice/bob/goblin in mutual LoS,
// so AddMonster auto-transitions it to TURN_BASED before any test body runs.
// This test verifies the FreeRoam->TurnBased flip itself, so it needs its
// own encounter with no monster (checkCombatEntry no-ops with zero monsters)
// to observe the actual FreeRoam starting state.
func (s *CombatSuite) TestSetMode_FlipsAndPublishes() {
	enc := encounter.New(context.Background(), "enc-mode-flip", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: "1d8+2", DamageType: damageSlashing,
	}))
	sub, err := s.broker.Subscribe(enc.ID(), "alice")
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	s.Equal(core.ModeFreeRoam, enc.Mode())
	s.Equal(core.EntityID(""), enc.ActiveActor())

	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	s.Equal(core.ModeTurnBased, enc.Mode())
	s.NotEqual(core.EntityID(""), enc.ActiveActor())

	seen := collectTypes(sub, 500*time.Millisecond)
	s.Contains(seen, "*events.ModeChangedEvent")
	s.Contains(seen, "*events.TurnStartedEvent")
}

// SetMode rejects redundant flips. Own monster-less encounter for the same
// reason as TestSetMode_FlipsAndPublishes: the shared fixture is already
// TURN_BASED by the time this test body runs.
func (s *CombatSuite) TestSetMode_RejectsRedundant() {
	enc := encounter.New(context.Background(), "enc-mode-redundant", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
	}))
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	s.Error(enc.SetMode(core.ModeTurnBased))
}

// TakeAction in FreeRoam mode returns ErrNotTurnBased. Own monster-less
// encounter so the fixture stays FreeRoam (the shared s.enc is already
// TURN_BASED by setup time); TakeAction's mode gate fires before any target
// lookup, so no monster needs to exist for this assertion.
func (s *CombatSuite) TestTakeAction_RejectedOutsideTurnBased() {
	enc := encounter.New(context.Background(), "enc-mode-notb", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
	}))
	err := enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.ErrorIs(err, encounter.ErrNotTurnBased)
}

// TakeAction by a non-active player returns ErrNotYourTurn. s.enc is
// already TURN_BASED by SetupTest (alice/bob/goblin start in mutual LoS).
func (s *CombatSuite) TestTakeAction_RejectedWhenNotYourTurn() {
	active := s.enc.ActiveActor()
	// Find the OTHER player and try to act.
	var attackerID core.PlayerID
	if active == aliceEntityID {
		attackerID = "bob"
	} else {
		attackerID = "alice"
	}
	err := s.enc.TakeAction(attackerID,
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.ErrorIs(err, encounter.ErrNotYourTurn)
}

// TakeAction with an unknown action ref returns ErrUnsupportedAction.
// s.enc is already TURN_BASED by SetupTest.
func (s *CombatSuite) TestTakeAction_RejectsUnknownAction() {
	active := s.enc.ActiveActor()
	playerID := s.playerIDFor(active)
	if playerID == "" {
		s.T().Skip("active actor is an NPC; this test only covers player turns")
	}
	// #697: the attack-only hard gate is gone — non-attack refs now delegate to
	// the held character's rules engine. This suite's seats are flat
	// stat-snapshots (no DataJSON → no hydrated character), so a non-attack ref
	// is rejected with ErrNonCombatant ("no character to take menu actions
	// with"), not ErrUnsupportedAction. Unknown-ref rejection on a HYDRATED
	// character (the ErrUnsupportedAction path) is covered by
	// TurnStateSuite.TestTakeAction_RejectsUnknownRefOnHydratedCharacter.
	err := s.enc.TakeAction(playerID,
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "shove"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.ErrorIs(err, encounter.ErrNonCombatant)
}

// TakeAction publishes AttackResolvedEvent (always); on hit a
// DamageDealtEvent rides alongside.
// s.enc is already TURN_BASED by SetupTest (alice/bob/goblin start in
// mutual LoS, auto-triggering combat entry at AddMonster time).
func (s *CombatSuite) TestTakeAction_PublishesStrikeOutcome() {
	for s.enc.ActiveActor() != aliceEntityID {
		_, _, err := s.enc.EndTurn(context.Background(), s.enc.ActiveActor())
		s.Require().NoError(err)
	}
	drainSub(s.aliceSub, 100*time.Millisecond)
	drainSub(s.bobSub, 100*time.Millisecond)

	err := s.enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.Require().NoError(err)

	seen := collectEventsTyped(s.aliceSub, 500*time.Millisecond)
	var sawAttack, sawAction bool
	for _, e := range seen {
		switch ev := e.(type) {
		case *events.AttackResolvedEvent:
			sawAttack = true
		case *events.ActionResolvedEvent:
			sawAction = true
			// Player-taken attacks are a human choice, not an AI targeting
			// decision — TargetRationale must be empty (rpg-toolkit#895;
			// the NPC-attack path is covered by
			// npc_test.go's TestNPCAct_AttackPublishesTargetRationale).
			s.Empty(ev.TargetRationale, "player-path attacks must emit an empty decision rationale")
		}
	}
	s.True(sawAttack, "expected *events.AttackResolvedEvent")
	s.True(sawAction, "expected *events.ActionResolvedEvent")
}

// TakeAction copies HasAdvantage/HasDisadvantage/AdvantageSources/
// DisadvantageSources from the resolver's StrikeOutcome onto the published
// AttackResolvedEvent verbatim (#726). The stub resolver here hands back
// canned values the same way rpg-api's real resolver copies them from
// combat.AttackResult -- this proves the encounter-side plumbing does not
// drop or recompute them, not that any rule fired.
func (s *CombatSuite) TestTakeAction_PublishesAdvantageDisadvantageOnAttackResolved() {
	dodgingRef := &toolkitcore.Ref{Module: "dnd5e", Type: "conditions", ID: "dodging"}
	enc := encounter.New(context.Background(), "enc-adv", s.broker,
		encounter.WithStrikeResolver(alwaysHitResolver{
			damage: 8, damageType: damageSlashing,
			hasDisadvantage:     true,
			disadvantageSources: []*toolkitcore.Ref{dodgingRef},
		}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: "1d8+2", DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15,
		DataJSON: testGoblinDataJSON(s.T(), gobEntityID),
	}))
	// alice and the goblin are in mutual LoS, so AddMonster already
	// auto-transitioned to TURN_BASED; an explicit SetMode here would be
	// redundant and error.
	for enc.ActiveActor() != aliceEntityID {
		_, _, err := enc.EndTurn(context.Background(), enc.ActiveActor())
		s.Require().NoError(err)
	}

	sub, err := s.broker.Subscribe(enc.ID(), "alice")
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	s.Require().NoError(enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	))

	var attackEvt *events.AttackResolvedEvent
	deadline := time.After(2 * time.Second)
drainLoop:
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				break drainLoop
			}
			if ae, isAttack := evt.(*events.AttackResolvedEvent); isAttack {
				attackEvt = ae
				break drainLoop
			}
		case <-deadline:
			break drainLoop
		}
	}
	s.Require().NotNil(attackEvt, "AttackResolvedEvent should have been published")
	s.False(attackEvt.HasAdvantage)
	s.True(attackEvt.HasDisadvantage)
	s.Require().Len(attackEvt.DisadvantageSources, 1)
	s.Equal(dodgingRef, attackEvt.DisadvantageSources[0])
	s.Empty(attackEvt.AdvantageSources)
}

// TakeAction returns ErrNonCombatant when the active player has no
// combat snapshot. Documents the PlayerInput contract: zero combat
// fields opt the seat out of combat verbs.
func (s *CombatSuite) TestTakeAction_RejectsNonCombatant() {
	// Build a fresh encounter with a non-combatant alice.
	enc := encounter.New(context.Background(), "enc-noncomb", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		// No HP / AC / DamageDice — non-combatant.
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15,
		DataJSON: testGoblinDataJSON(s.T(), gobEntityID),
	}))
	// alice and the goblin are in mutual LoS, so AddMonster already
	// auto-transitioned to TURN_BASED; an explicit SetMode here would be
	// redundant and error.
	for enc.ActiveActor() != aliceEntityID {
		_, _, err := enc.EndTurn(context.Background(), enc.ActiveActor())
		s.Require().NoError(err)
	}
	err := enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.ErrorIs(err, encounter.ErrNonCombatant)
}

// TestTakeAction_HydratedPlayerBypassesFlatSnapshotGate is the #634 gate
// relaxation: a player seat carrying NO flat AC/DamageDice snapshot — the
// honest state a host lands in when it hydrates real characters but has no
// rules-legitimate way to also invent an attack-bonus/damage-dice snapshot
// (rpg-api's lobby StartEncounter, see rpg-api#634) — still passes
// isPlayerCombatant once hydrated via DataJSON. alwaysHitResolver is a stub
// that ignores its input, so this proves the GATE specifically, mirroring
// TestTakeAction_RejectsNonCombatant's scope for the un-hydrated case.
func (s *CombatSuite) TestTakeAction_HydratedPlayerBypassesFlatSnapshotGate() {
	charData := &dnd5eCharacter.Data{
		ID:               aliceEntityID,
		Name:             "Alice",
		Level:            1,
		ProficiencyBonus: 2,
		HitPoints:        12,
		MaxHitPoints:     12,
	}
	charJSON, err := json.Marshal(charData)
	s.Require().NoError(err)

	enc := encounter.New(context.Background(), "enc-hydrated-gate", s.broker,
		encounter.WithStrikeResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, // No AC, no DamageDice — the honest StartEncounter snapshot.
		DataJSON: charJSON,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15,
		DataJSON: testGoblinDataJSON(s.T(), gobEntityID),
	}))

	// Round-trip through Data so the hydration cascade runs — New/AddPlayer
	// never hydrate; only LoadFromData does (mirrors hydration_test.go).
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(context.Background(), &data, s.broker,
		encounter.WithStrikeResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)
	s.Require().NoError(err)

	// AddMonster auto-transitioned to TURN_BASED (mutual LoS, #757) back on
	// the pre-round-trip encounter; LoadFromData's catch-up seeded the active
	// actor's economy (the seed was structurally impossible at the flip —
	// nothing was hydrated yet).
	for loaded.ActiveActor() != aliceEntityID {
		_, _, endErr := loaded.EndTurn(context.Background(), loaded.ActiveActor())
		s.Require().NoError(endErr)
	}

	err = loaded.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.Require().NoError(err, "hydrated seat must pass the combatant gate without a flat AC/DamageDice snapshot")
}

// TestTakeAction_DataJSONWithoutLoadFromData_StillRejected is the regression
// guard for Copilot review on rpg-toolkit#751: DataJSON being SET on a
// PlayerInput is not the same as a seat being HYDRATED — New()+AddPlayer
// never hydrate; only a LoadFromData round-trip's hydrateCombatants cascade
// does (see hydration.go). A seat built via New()+AddPlayer(DataJSON: ...)
// and used directly, with no LoadFromData round-trip, must still be
// rejected — e.heldCharacter returns nil for it, and it carries no flat
// AC/DamageDice snapshot either.
func (s *CombatSuite) TestTakeAction_DataJSONWithoutLoadFromData_StillRejected() {
	charData := &dnd5eCharacter.Data{
		ID: aliceEntityID, Name: "Alice", Level: 1,
		HitPoints: 12, MaxHitPoints: 12,
	}
	charJSON, err := json.Marshal(charData)
	s.Require().NoError(err)

	enc := encounter.New(context.Background(), "enc-unhydrated-datajson", s.broker,
		encounter.WithStrikeResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, // No AC, no DamageDice.
		DataJSON: charJSON, // Set, but never round-tripped through LoadFromData — not actually hydrated.
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15,
		DataJSON: testGoblinDataJSON(s.T(), gobEntityID),
	}))
	// alice and the goblin are in mutual LoS, so AddMonster already
	// auto-transitioned to TURN_BASED; an explicit SetMode here would be
	// redundant and error.
	for enc.ActiveActor() != aliceEntityID {
		_, _, endErr := enc.EndTurn(context.Background(), enc.ActiveActor())
		s.Require().NoError(endErr)
	}

	err = enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.ErrorIs(err, encounter.ErrNonCombatant, "DataJSON alone (no LoadFromData round-trip) must not satisfy the gate")
}

// EndTurn publishes TurnEnded + TurnStarted; rotates Initiative. s.enc is
// already TURN_BASED by SetupTest.
func (s *CombatSuite) TestEndTurn_AdvancesInitiative() {
	first := s.enc.ActiveActor()
	drainSub(s.aliceSub, 100*time.Millisecond)
	drainSub(s.bobSub, 100*time.Millisecond)

	next, _, err := s.enc.EndTurn(context.Background(), first)
	s.Require().NoError(err)
	s.NotEqual(first, next)
	s.Equal(next, s.enc.ActiveActor())

	seen := collectTypes(s.aliceSub, 500*time.Millisecond)
	s.Contains(seen, "*events.TurnEndedEvent")
	s.Contains(seen, "*events.TurnStartedEvent")
}

// EndTurn called by a non-active actor errors with ErrNotYourTurn. s.enc is
// already TURN_BASED by SetupTest.
func (s *CombatSuite) TestEndTurn_RejectsWrongActor() {
	active := s.enc.ActiveActor()
	other := core.EntityID(aliceEntityID)
	if active == aliceEntityID {
		other = bobEntityID
	}
	_, _, err := s.enc.EndTurn(context.Background(), other)
	s.ErrorIs(err, encounter.ErrNotYourTurn)
}

// EndTurn outside TURN_BASED returns ErrNotTurnBased. Own monster-less
// encounter so the fixture stays FreeRoam (the shared s.enc is already
// TURN_BASED by setup time).
func (s *CombatSuite) TestEndTurn_RequiresTurnBased() {
	enc := encounter.New(context.Background(), "enc-mode-endturn-notb", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
	}))
	_, _, err := enc.EndTurn(context.Background(), aliceEntityID)
	s.ErrorIs(err, encounter.ErrNotTurnBased)
}

// EndTurn returns ErrNoCombatants — does not panic — when initiative
// is empty (e.g. SetMode(TurnBased) flipped on an empty encounter).
// Regression test for the out-of-range panic Copilot flagged in #638.
func (s *CombatSuite) TestEndTurn_GuardsEmptyInitiative() {
	enc := encounter.New(context.Background(), "enc-empty", s.broker)
	// SetMode would normally roll initiative, but with no players or
	// monsters the Initiative slice ends up empty.
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	s.Empty(enc.ActiveActor())

	_, _, err := enc.EndTurn(context.Background(), "anyone")
	s.ErrorIs(err, encounter.ErrNoCombatants)
}

// NPCAct emits an attack event when a player is reachable. Formerly named
// TestNPCAct_ScriptedAttackPublishes and exercised via a no-DataJSON
// monster (npcActScripted); that fallback is deleted (rpg-toolkit#895
// no-fallback rider), so s.enc's goblin now carries real DataJSON and this
// runs the normal hydrated NPCAct path instead.
// s.enc is already TURN_BASED by SetupTest.
func (s *CombatSuite) TestNPCAct_AttackPublishes() {
	for s.enc.ActiveActor() != gobEntityID {
		_, _, err := s.enc.EndTurn(context.Background(), s.enc.ActiveActor())
		s.Require().NoError(err)
	}
	drainSub(s.aliceSub, 100*time.Millisecond)
	drainSub(s.bobSub, 100*time.Millisecond)

	err := s.enc.NPCAct(s.ctx, gobEntityID)
	s.Require().NoError(err)

	seen := collectTypes(s.aliceSub, 500*time.Millisecond)
	s.Contains(seen, "*events.AttackResolvedEvent")
}

// TakeAction omits non-viewers (out-of-LoS players) from PerPlayer
// entirely so the broker does not deliver to them. Mirrors Move /
// OpenDoor audience-routing.
func (s *CombatSuite) TestTakeAction_OmitsNonViewersFromAudience() {
	enc := encounter.New(context.Background(), "enc-combat-2", s.broker,
		encounter.WithStrikeResolver(alwaysHitResolver{damage: 8, damageType: damageSlashing}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: "1d8+2", DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: bobEntityID,
		Position: core.Hex{Q: 50, R: -25, S: -25}, SightRange: 5,
		HP: 10, MaxHP: 10, AC: 13,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID:       gob2EntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       7, MaxHP: 7, AC: 15,
		DataJSON: testGoblinDataJSON(s.T(), gob2EntityID),
	}))
	farAliceSub, err := s.broker.Subscribe("enc-combat-2", "alice")
	s.Require().NoError(err)
	defer func() { _ = farAliceSub.Close() }()
	farBobSub, err := s.broker.Subscribe("enc-combat-2", "bob")
	s.Require().NoError(err)
	defer func() { _ = farBobSub.Close() }()

	// alice is in LoS of gob2 (bob is far away and out of range), so
	// AddMonster already auto-transitioned to TURN_BASED; an explicit
	// SetMode here would be redundant and error.
	for enc.ActiveActor() != aliceEntityID {
		_, _, endErr := enc.EndTurn(context.Background(), enc.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(farAliceSub, 100*time.Millisecond)
	drainSub(farBobSub, 100*time.Millisecond)

	err = enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gob2EntityID},
	)
	s.Require().NoError(err)

	// Alice can see her attack: she's in PerPlayer (Visible: true) and
	// her subscription delivers.
	aliceEvent := waitForAttackResolved(s.T(), farAliceSub, time.Second)
	s.Require().NotNil(aliceEvent, "alice should receive AttackResolvedEvent")
	s.True(aliceEvent.PerPlayer["alice"].Visible)
	_, bobInAudience := aliceEvent.PerPlayer["bob"]
	s.False(bobInAudience, "bob is out of LoS; he should be omitted from PerPlayer entirely")

	// Bob is out of LoS to both attacker and target — broker should NOT
	// deliver any AttackResolvedEvent to bob's subscription.
	bobEvent := waitForAttackResolved(s.T(), farBobSub, 200*time.Millisecond)
	s.Nil(bobEvent, "bob is out of LoS; no AttackResolvedEvent should be delivered")
}

// MonsterData round-trips through ToData / LoadFromData. s.enc is already
// TURN_BASED by SetupTest.
func (s *CombatSuite) TestMonsterData_RoundTrips() {
	persisted := s.enc.ToData()
	s.Require().Contains(persisted.Monsters, core.EntityID(gobEntityID))

	rehydrated, err := encounter.LoadFromData(context.Background(), persisted, s.broker)
	s.Require().NoError(err)
	s.Equal(core.ModeTurnBased, rehydrated.Mode())
	s.NotEqual(core.EntityID(""), rehydrated.ActiveActor())
}

// Helpers — match patterns from encounter_test.go / integration_test.go.

func (s *CombatSuite) playerIDFor(entityID core.EntityID) core.PlayerID {
	switch entityID {
	case aliceEntityID:
		return "alice"
	case bobEntityID:
		return "bob"
	}
	return ""
}

func waitForAttackResolved(
	t *testing.T, sub *encounter.Subscription, timeout time.Duration,
) *events.AttackResolvedEvent {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				return nil
			}
			if ar, ok := evt.(*events.AttackResolvedEvent); ok {
				return ar
			}
		case <-deadline:
			return nil
		}
	}
}
