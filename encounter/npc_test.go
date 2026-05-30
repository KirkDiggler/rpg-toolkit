package encounter_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	encevents "github.com/KirkDiggler/rpg-toolkit/encounter/events"
	dnd5events "github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// monsterRefGoblin is the canonical monster ref used across NPC fixtures.
const monsterRefGoblin = "dnd5e:monsters:goblin"

// NPCSuite covers the full NPCAct path through monster.TakeTurn —
// distinct from CombatSuite which uses the scripted DataJSON-less path.
type NPCSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
	aliceSub  *encounter.Subscription
}

func TestNPCSuite(t *testing.T) {
	suite.Run(t, new(NPCSuite))
}

func (s *NPCSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New("enc-npc", s.broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 4, damageType: damageSlashing}),
	)

	// Build a goblin and serialize it.
	gob := monster.NewGoblin(gobEntityID)
	gobData := gob.ToData()
	dataJSON, err := json.Marshal(gobData)
	s.Require().NoError(err)

	// Alice adjacent to goblin so the goblin can attack on its turn.
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14,
	}))
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID:       gobEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       7, MaxHP: 7, AC: 15, Speed: 6,
		MonsterRef:  monsterRefGoblin,
		DataJSON:    dataJSON,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	s.aliceSub, err = s.broker.Subscribe("enc-npc", alicePlayerID)
	s.Require().NoError(err)
}

func (s *NPCSuite) TearDownTest() {
	if s.aliceSub != nil {
		_ = s.aliceSub.Close()
	}
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// NPCAct via monster.TakeTurn captures the dnd5e AttackEvent the goblin's
// scimitar action emits and re-publishes it as an encounter
// AttackResolvedEvent.
func (s *NPCSuite) TestNPCAct_GoblinTakeTurn_PublishesAttack() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	for s.enc.ActiveActor() != gobEntityID {
		_, _, err := s.enc.EndTurn(s.enc.ActiveActor())
		s.Require().NoError(err)
	}
	drainSub(s.aliceSub, 100*time.Millisecond)

	err := s.enc.NPCAct(s.ctx, gobEntityID)
	s.Require().NoError(err)

	seen := collectTypes(s.aliceSub, time.Second)
	s.Contains(seen, "*events.AttackResolvedEvent")
}

// NPCAct outside TURN_BASED returns ErrNotTurnBased.
func (s *NPCSuite) TestNPCAct_RequiresTurnBased() {
	err := s.enc.NPCAct(s.ctx, gobEntityID)
	s.ErrorIs(err, encounter.ErrNotTurnBased)
}

// NPCAct against an unknown id returns ErrNotYourTurn (the active-actor
// check fires before the existence check).
func (s *NPCSuite) TestNPCAct_RejectsUnknownNPC() {
	s.Require().NoError(s.enc.SetMode(core.ModeTurnBased))
	// Cycle to goblin to satisfy the active-actor gate, then try to act
	// for a non-existent npc — the test exercises ErrNotYourTurn (the
	// active-actor check fires before the existence check).
	for s.enc.ActiveActor() != gobEntityID {
		_, _, err := s.enc.EndTurn(s.enc.ActiveActor())
		s.Require().NoError(err)
	}
	err := s.enc.NPCAct(s.ctx, "ghost")
	s.ErrorIs(err, encounter.ErrNotYourTurn)
}

// NPCAct returns ErrNoCombatResolver when no CombatResolver is wired.
// Mirrors the guard on TakeAction (player path) — production must wire one
// via WithCombatResolver.
func (s *NPCSuite) TestNPCAct_ErrNoCombatResolver() {
	gob := monster.NewGoblin(gobEntityID)
	gobData := gob.ToData()
	dataJSON, err := json.Marshal(gobData)
	s.Require().NoError(err)

	enc := encounter.New("enc-npc-no-resolver", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID:       gobEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       7, MaxHP: 7, AC: 15, Speed: 6,
		MonsterRef: monsterRefGoblin,
		DataJSON:   dataJSON,
	}))
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != gobEntityID {
		_, _, endErr := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(endErr)
	}

	err = enc.NPCAct(s.ctx, gobEntityID)
	s.ErrorIs(err, encounter.ErrNoCombatResolver)
}

// NPCAct (scripted path — no DataJSON) returns ErrNoCombatResolver when
// no resolver is wired.
func (s *NPCSuite) TestNPCAct_Scripted_ErrNoCombatResolver() {
	enc := encounter.New("enc-npc-scripted-no-resolver", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14,
	}))
	// No DataJSON — triggers the scripted path.
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID:       gobEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != gobEntityID {
		_, _, endErr := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(endErr)
	}

	err := enc.NPCAct(s.ctx, gobEntityID)
	s.ErrorIs(err, encounter.ErrNoCombatResolver)
}

// TestNPCAct_MovementOA_AppliesDamageOnce is the production-path regression
// guard from PR #677 director review: NPCAct installs outer subscribeAttack/
// subscribeDamage listeners for the entire call, and applyNPCMovement runs
// INSIDE that window. Without per-window scoping, an OA-triggered
// DamageReceivedEvent published during the movement step is captured by
// BOTH the inner per-step subscriber (iterateMovementStepsForEntity, #675)
// and the outer NPCAct subscriber — HP delta then applies twice when
// applyCapturedDamage runs after applyNPCMovement returns. A 5-damage OA
// drops a 7-HP goblin to -3 HP and fires the kill chain twice.
//
// The earlier TestNPCMove_NPCMoves_OADamagesMonster used MoveNPCSteps
// which bypasses NPCAct's outer subscribers entirely, so the double-apply
// passed test review unnoticed. This test exercises NPCAct directly with
// a MovementResolver wired, simulating an OA during movement, and asserts
// HP delta applies exactly once.
func (s *NPCSuite) TestNPCAct_MovementOA_AppliesDamageOnce() {
	gob := monster.NewGoblin(gobEntityID)
	gobData := gob.ToData()
	dataJSON, err := json.Marshal(gobData)
	s.Require().NoError(err)

	// Stub MovementResolver that publishes a DamageReceivedEvent targeting
	// the goblin (mover) during step 0 — simulating an OA the player has
	// already taken against the retreating monster. combat.ResolveAttack
	// would do this in production.
	const oaDamage = 5
	resolver := &stubMovementResolver{
		publishOnStep: func(bus dnd5events.EventBus, stepIdx int) {
			if stepIdx == 0 {
				topic := dnd5eEvents.DamageReceivedTopic.On(bus)
				_ = topic.Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
					TargetID:   string(gobEntityID),
					SourceID:   string(aliceEntityID),
					Amount:     oaDamage,
					DamageType: damage.Slashing,
				})
			}
		},
	}

	// Rebuild an encounter with both the combat resolver (NPCAct guard)
	// AND the movement resolver wired. Alice at distance 10 so the goblin
	// AI walks toward her but doesn't enter scimitar reach this turn —
	// keeps the test focused on the movement-OA path (no attack publishes
	// to confound the captured-damage slice post-movement).
	enc := encounter.New("enc-npcact-move-oa", s.broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 4, damageType: damageSlashing}),
		encounter.WithMovementResolver(resolver),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{Q: 10, R: 0, S: -10}, SightRange: 12,
		HP: 12, MaxHP: 12, AC: 14,
	}))
	const startHP = 7
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID:       gobEntityID,
		Position: core.Hex{Q: 0, R: 0, S: 0},
		HP:       startHP, MaxHP: startHP, AC: 15, Speed: 6,
		MonsterRef:  monsterRefGoblin,
		DataJSON:    dataJSON,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != gobEntityID {
		_, _, endErr := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(endErr)
	}

	err = enc.NPCAct(s.ctx, gobEntityID)
	s.Require().NoError(err)

	// HP applied exactly once: 7 - 5 = 2. If the outer subscriber
	// double-applied, HP would be -3 (clamped to 0) and the kill chain
	// would have fired.
	mon := enc.ToData().Monsters[gobEntityID]
	s.Require().NotNil(mon, "goblin must still be present (5 damage on 7 HP is non-lethal)")
	s.Equal(startHP-oaDamage, mon.HP,
		"OA damage must apply exactly once on the NPCAct path (no double-apply)")
}

// busPublishingResolver is a test CombatResolver that mimics the production
// dnd5e resolver: it returns an AttackOutcome AND publishes a
// DamageReceivedEvent on the encounter bus (the "notify" step that
// combat.ApplyAttackOutcome performs after running the damage chain).
// This is the minimal reproducer for #684 — the standard alwaysHitResolver
// does NOT publish on the bus and so cannot trigger the double-apply.
type busPublishingResolver struct {
	damage     int
	damageType string
}

func (r busPublishingResolver) ResolveAttack(input encounter.AttackInput) (*encounter.AttackOutcome, error) {
	if input.EventBus != nil {
		topic := dnd5eEvents.DamageReceivedTopic.On(input.EventBus)
		_ = topic.Publish(context.Background(), dnd5eEvents.DamageReceivedEvent{
			TargetID:   string(input.TargetID),
			SourceID:   string(input.AttackerID),
			Amount:     r.damage,
			DamageType: damage.Type(r.damageType),
		})
	}
	return &encounter.AttackOutcome{
		Hit:         true,
		AttackRoll:  20,
		AttackBonus: 4,
		TargetAC:    10,
		Damage:      r.damage,
		DamageType:  r.damageType,
	}, nil
}

// TestNPCAct_SingleDamageDealtEvent_PerAttack is the regression guard for
// #684: a single NPC attack must produce exactly one DamageDealtEvent and
// apply HP exactly once. Before the fix, the resolver's internal
// DamageReceivedEvent notify was captured by the outer subscribeDamage
// listener in NPCAct and then re-applied by applyCapturedDamage, producing
// a second DamageDealtEvent with empty Components and a second HP mutation.
//
// This test uses busPublishingResolver which reproduces the production
// resolver pattern: returns AttackOutcome AND publishes DamageReceivedEvent
// on the encounter bus. Without the #684 fix, alice (12 HP) would take 10
// HP of damage from a 5-damage hit and be at 2 HP; with the fix she is at 7.
func (s *NPCSuite) TestNPCAct_SingleDamageDealtEvent_PerAttack() {
	const attackDamage = 5
	const aliceStartHP = 12

	gob := monster.NewGoblin(gobEntityID)
	gobData := gob.ToData()
	dataJSON, err := json.Marshal(gobData)
	s.Require().NoError(err)

	// Build a fresh encounter with the bus-publishing resolver so the
	// DamageReceivedEvent notify fires during the attack-resolution window.
	enc := encounter.New("enc-npc-single-dmg", s.broker,
		encounter.WithCombatResolver(busPublishingResolver{damage: attackDamage, damageType: damageSlashing}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: aliceStartHP, MaxHP: aliceStartHP, AC: 14,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID:       gobEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       7, MaxHP: 7, AC: 15, Speed: 6,
		MonsterRef:  monsterRefGoblin,
		DataJSON:    dataJSON,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))
	sub, subErr := s.broker.Subscribe("enc-npc-single-dmg", alicePlayerID)
	s.Require().NoError(subErr)
	defer func() { _ = sub.Close() }()

	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != gobEntityID {
		_, _, endErr := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(sub, 100*time.Millisecond)

	s.Require().NoError(enc.NPCAct(s.ctx, gobEntityID))

	// Collect all events delivered to alice from the NPC attack.
	var dmgEvents []*encevents.DamageDealtEvent
	deadline := time.After(time.Second)
drainLoop:
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				break drainLoop
			}
			if de, isDmg := evt.(*encevents.DamageDealtEvent); isDmg {
				dmgEvents = append(dmgEvents, de)
			}
		case <-deadline:
			break drainLoop
		}
	}

	// ASSERT: exactly one DamageDealtEvent (not two).
	s.Require().Len(dmgEvents, 1,
		"exactly one DamageDealtEvent per NPC attack (#684: before fix, two events fire — "+
			"one from publishAttackOutcome with Components, one from applyCapturedDamage without)")

	// ASSERT: HP applied exactly once — alice goes from 12 → 7, not 12 → 2.
	aliceAfter := enc.ToData().Players[alicePlayerID]
	s.Equal(aliceStartHP-attackDamage, aliceAfter.HP,
		"HP must drop by damage exactly once; if 12→2, applyCapturedDamage double-applied (#684)")
}
