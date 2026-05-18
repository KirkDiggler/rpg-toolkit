package encounter_test

// Wave 2.11d — phased combat orchestration tests.
//
// These tests exercise Encounter.TakeActionPhased + CompleteTakeAction with
// a stubbed PhasedCombatResolver. The stub controls whether ReactionTrigger
// events are published during phase 1 and whether the reactor is a player
// (surface) or NPC (auto-resolve).

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	tkenc "github.com/KirkDiggler/rpg-toolkit/encounter"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
	dnd5events "github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// shieldRefStr is the canonical core.Ref string for the Shield spell, used
// in this file to construct trigger events and modifier expectations.
const shieldRefStr = "dnd5e:spells:shield"

// actionIDAttackTest mirrors the encounter SDK's package-private actionIDAttack
// constant for use in test ActionRefs.
const actionIDAttackTest = "attack"

// damage1d6plus1 is the goblin's default natural-attack dice notation.
const damage1d6plus1 = "1d6+1"

// stubPhasedResolver implements PhasedCombatResolver and records calls.
// It also lets the test publish ReactionTriggerEvents during ResolveAttackHit
// so the encounter SDK's buffered subscriber sees them.
type stubPhasedResolver struct {
	hitCalls     []tkenc.AttackInput
	outcomeCalls []*tkenc.PhasedAttackContext

	// publishOnHit, when set, is called during ResolveAttackHit to simulate
	// post-roll subscribers publishing ReactionTriggerEvents on the bus.
	publishOnHit func(bus dnd5events.EventBus)

	// resolverTriggers are returned directly from ResolveAttackHit in
	// addition to whatever the buffered subscriber catches.
	resolverTriggers []tkenc.ReactionTrigger

	// hitReturn / outcomeReturn control the return values.
	hitReturn     *tkenc.PhasedAttackContext
	outcomeReturn *tkenc.AttackOutcome
}

func (s *stubPhasedResolver) ResolveAttack(_ tkenc.AttackInput) (*tkenc.AttackOutcome, error) {
	// Legacy path. Phased-path tests do not reach this; the legacy fallback
	// test uses stubLegacyResolver instead.
	return s.outcomeReturn, nil
}

func (s *stubPhasedResolver) ResolveAttackHit(
	input tkenc.AttackInput,
) (*tkenc.PhasedAttackContext, []tkenc.ReactionTrigger, error) {
	s.hitCalls = append(s.hitCalls, input)
	if s.publishOnHit != nil && input.EventBus != nil {
		s.publishOnHit(input.EventBus)
	}
	if s.hitReturn == nil {
		s.hitReturn = &tkenc.PhasedAttackContext{
			Rulebook:   "stub",
			AttackerID: input.AttackerID,
			TargetID:   input.TargetID,
		}
	}
	return s.hitReturn, s.resolverTriggers, nil
}

func (s *stubPhasedResolver) ApplyAttackOutcome(
	ctx *tkenc.PhasedAttackContext, _ []tkenc.ReactionModifier,
) (*tkenc.AttackOutcome, error) {
	s.outcomeCalls = append(s.outcomeCalls, ctx)
	if s.outcomeReturn == nil {
		s.outcomeReturn = &tkenc.AttackOutcome{
			Hit:         true,
			AttackRoll:  14,
			AttackBonus: 3,
			TargetAC:    14,
			Damage:      5,
			DamageType:  damageSlashing,
		}
	}
	return s.outcomeReturn, nil
}

type PhasedTakeActionSuite struct {
	suite.Suite
	transport *tkenc.InMemoryTransport
	broker    *tkenc.Broker
	enc       *tkenc.Encounter
	resolver  *stubPhasedResolver
}

func TestPhasedTakeActionSuite(t *testing.T) {
	suite.Run(t, new(PhasedTakeActionSuite))
}

func (s *PhasedTakeActionSuite) SetupTest() {
	s.transport = tkenc.NewInMemoryTransport()
	s.broker = tkenc.NewBroker(s.transport)
	s.resolver = &stubPhasedResolver{}
	s.enc = tkenc.New("enc-1", s.broker, tkenc.WithCombatResolver(s.resolver))

	// Two players so we can route a reactor that isn't the attacker.
	s.Require().NoError(s.enc.AddPlayer(tkenc.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: encountercore.Hex{}, SightRange: 10,
		HP: 30, MaxHP: 30, AC: 14, AttackBonus: 5,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(s.enc.AddPlayer(tkenc.PlayerInput{
		PlayerID: bobPlayerID, EntityID: bobEntityID,
		Position: encountercore.Hex{Q: 1, R: 0, S: -1}, SightRange: 10,
		HP: 18, MaxHP: 18, AC: 12, AttackBonus: 4,
		DamageDice: "1d4", DamageType: damageSlashing,
	}))
	s.Require().NoError(s.enc.AddMonster(tkenc.MonsterInput{
		ID: gobEntityID, Position: encountercore.Hex{Q: 2, R: 0, S: -2},
		HP: 10, MaxHP: 10, AC: 13, Speed: 30, AttackBonus: 4,
		DamageDice: damage1d6plus1, DamageType: damageSlashing,
	}))
	// Initiative is random per-roll; tests cycle EndTurn until alice is active.
	s.Require().NoError(s.enc.SetMode(encountercore.ModeTurnBased))
}

// makeAliceActive cycles EndTurn until alice is the active actor.
func (s *PhasedTakeActionSuite) makeAliceActive() {
	for i := 0; i < 5; i++ {
		if s.enc.ActiveActor() == aliceEntityID {
			return
		}
		_, _, err := s.enc.EndTurn(s.enc.ActiveActor())
		s.Require().NoError(err)
	}
	s.Require().Equal(encountercore.EntityID(aliceEntityID), s.enc.ActiveActor(),
		"alice must be active for this test")
}

// attackRef is the canonical attack action ref for tests.
func attackRef() tkenc.ActionRef {
	return tkenc.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest}
}

func (s *PhasedTakeActionSuite) TestNoTriggers_ResolvedInline() {
	s.makeAliceActive()
	out, err := s.enc.TakeActionPhased(alicePlayerID, attackRef(),
		tkenc.ActionTarget{EntityID: gobEntityID})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.True(out.Resolved, "expected Resolved=true when no triggers fired")
	s.Empty(out.Reactions)
	s.Len(s.resolver.hitCalls, 1, "phase 1 should be called once")
	s.Len(s.resolver.outcomeCalls, 1, "phase 2 should be called inline once")
}

func (s *PhasedTakeActionSuite) TestNPCTrigger_ResolvedInlineWithModifier() {
	s.makeAliceActive()
	// NPC reactor (goblin-1) publishes a Shield trigger. Stub partitions
	// goblin-1 as NPC (not in players map).
	s.resolver.publishOnHit = func(bus dnd5events.EventBus) {
		topic := dnd5eEvents.ReactionTriggerTopic.On(bus)
		_ = topic.Publish(context.Background(), dnd5eEvents.ReactionTriggerEvent{
			ReactorID:    gobEntityID,
			ConditionRef: shieldRefStr,
			TriggerKind:  dnd5eEvents.TriggerKindPostHit,
			SourceEntity: aliceEntityID,
		})
	}

	out, err := s.enc.TakeActionPhased(alicePlayerID, attackRef(),
		tkenc.ActionTarget{EntityID: gobEntityID})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.True(out.Resolved, "NPC triggers should be resolved inline")
	s.Empty(out.Reactions, "no player triggers expected")
	s.Require().Len(s.resolver.outcomeCalls, 1)
}

func (s *PhasedTakeActionSuite) TestPlayerTrigger_SurfacedToCaller() {
	s.makeAliceActive()
	// Player reactor (bob) publishes a Shield trigger. Bob is in players
	// map → partitioned as player → surfaced.
	s.resolver.publishOnHit = func(bus dnd5events.EventBus) {
		topic := dnd5eEvents.ReactionTriggerTopic.On(bus)
		_ = topic.Publish(context.Background(), dnd5eEvents.ReactionTriggerEvent{
			ReactorID:    bobEntityID,
			ConditionRef: shieldRefStr,
			TriggerKind:  dnd5eEvents.TriggerKindPostHit,
			SourceEntity: aliceEntityID,
		})
	}

	out, err := s.enc.TakeActionPhased(alicePlayerID, attackRef(),
		tkenc.ActionTarget{EntityID: gobEntityID})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.False(out.Resolved, "player trigger present → not yet resolved")
	s.Require().Len(out.Reactions, 1)
	s.Equal(encountercore.EntityID(bobEntityID), out.Reactions[0].ReactorID)
	s.Equal(shieldRefStr, out.Reactions[0].ConditionRef)
	s.NotNil(out.AttackContext, "AttackContext must be populated for resume")
	s.Empty(s.resolver.outcomeCalls, "phase 2 must not run before reactor responds")
}

func (s *PhasedTakeActionSuite) TestCompleteTakeAction_RunsPhase2WithModifiers() {
	s.makeAliceActive()
	s.resolver.publishOnHit = func(bus dnd5events.EventBus) {
		topic := dnd5eEvents.ReactionTriggerTopic.On(bus)
		_ = topic.Publish(context.Background(), dnd5eEvents.ReactionTriggerEvent{
			ReactorID:    bobEntityID,
			ConditionRef: shieldRefStr,
			TriggerKind:  dnd5eEvents.TriggerKindPostHit,
			SourceEntity: aliceEntityID,
		})
	}

	out, err := s.enc.TakeActionPhased(alicePlayerID, attackRef(),
		tkenc.ActionTarget{EntityID: gobEntityID})
	s.Require().NoError(err)
	s.Require().NotNil(out.AttackContext)

	err = s.enc.CompleteTakeAction(out.AttackContext, []tkenc.ReactionModifier{
		{ConditionRef: shieldRefStr, ACBonus: 5},
	})
	s.Require().NoError(err)
	s.Len(s.resolver.outcomeCalls, 1, "phase 2 should run once via CompleteTakeAction")
}

// stubLegacyResolver only implements CombatResolver (no phased path). Verifies
// TakeActionPhased gracefully falls back to the monolithic single-phase path.
type stubLegacyResolver struct {
	calls []tkenc.AttackInput
}

func (s *stubLegacyResolver) ResolveAttack(input tkenc.AttackInput) (*tkenc.AttackOutcome, error) {
	s.calls = append(s.calls, input)
	return &tkenc.AttackOutcome{
		Hit: false, AttackRoll: 5, AttackBonus: 3, TargetAC: 14,
	}, nil
}

func (s *PhasedTakeActionSuite) TestLegacyResolverFallback() {
	transport := tkenc.NewInMemoryTransport()
	broker := tkenc.NewBroker(transport)
	resolver := &stubLegacyResolver{}
	enc := tkenc.New("enc-2", broker, tkenc.WithCombatResolver(resolver))
	s.Require().NoError(enc.AddPlayer(tkenc.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: encountercore.Hex{}, SightRange: 10,
		HP: 30, MaxHP: 30, AC: 14, AttackBonus: 5,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(tkenc.MonsterInput{
		ID: gobEntityID, Position: encountercore.Hex{Q: 2, R: 0, S: -2},
		HP: 10, MaxHP: 10, AC: 13, Speed: 30, AttackBonus: 4,
		DamageDice: damage1d6plus1, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.SetMode(encountercore.ModeTurnBased))
	for i := 0; i < 5; i++ {
		if enc.ActiveActor() == aliceEntityID {
			break
		}
		_, _, err := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(err)
	}

	out, err := enc.TakeActionPhased(alicePlayerID, attackRef(),
		tkenc.ActionTarget{EntityID: gobEntityID})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.True(out.Resolved, "legacy resolver always Resolved=true")
	s.Len(resolver.calls, 1)
}
