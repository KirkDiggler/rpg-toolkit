// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// DamageCustodyTestSuite covers the fold this package took custody of in #965
// slice 2: resolution publishes and executes the damage chain itself and calls
// the bus-free combat.FinalDamage, where slice 1 handed its bus to
// combat.ResolveDamage and let the other module do both.
//
// The behavior must not have moved. What these pin is that the fold still
// reaches every subscriber, that the multiplier arithmetic still applies, and
// that both are true through resolution's own step rather than through
// combat's entry point.
type DamageCustodyTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestDamageCustodySuite(t *testing.T) {
	suite.Run(t, new(DamageCustodyTestSuite))
}

func (s *DamageCustodyTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// wolfBiteAt runs the catalog wolf's bite at the hero on a bus the caller
// holds, so a test can install its own chain subscriber first.
func (s *DamageCustodyTestSuite) biteOnBus(
	bus events.EventBus, target *monster.Data, roller *sequenceRoller,
) (*Output, error) {
	data := monsters.NewWolf(wolfID).ToData()
	attack, err := AttackFromMonsterAction(data.Actions[0])
	s.Require().NoError(err)

	return resolveOn(s.ctx, &Input{
		World:        s.roomWith(encounter.MemberID(wolfID), encounter.MemberID(target.ID)),
		Participants: []Participant{{Monster: data}, {Monster: target}},
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   target.ID,
			Attack:     attack,
			Roller:     roller,
		}),
	}, newSurface(bus))
}

// THE HEADLINE. Damage lands with the multipliers applied, through
// resolution's own fold and combat's exported arithmetic.
//
// The bite is 2d4+2. Scripted [3 4] gives 3+4+2 = 9. Resistance halves the
// TYPE's total after grouping — 9 halved is 4, truncating — which is exactly
// what a fold that skipped FinalDamage would get wrong by reporting 9.
func (s *DamageCustodyTestSuite) TestResistanceHalvesDamageThroughTheOwnedFold() {
	bus := events.NewEventBus()
	s.halveOnBus(bus, damage.Piercing)

	out, err := s.biteOnBus(bus, monsters.NewWolf(secondWolfID).ToData(),
		&sequenceRoller{singles: []int{hitRoll, 18}, pair: []int{3, 4}, fallback: 2})
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Require().True(outcome.Hit)
	s.Require().Equal(4, outcome.Damage, "2d4[3 4]+2 = 9, halved to 4 — the arithmetic, not the raw pool")

	s.Require().Len(out.DirtyMonsters, 1)
	s.Require().Equal(11-4, out.DirtyMonsters[0].HitPoints)
}

// THE BORN-CORRECT PAYOFF. An immune target takes nothing.
//
// This is why the module pins dnd5e v0.94.1 rather than v0.94.0: immunity is
// authored as a zero factor, and until #1012 the dispatch read zero as "no
// modifier" and let full damage through. Pinning the earlier tag would have
// given this fold the bug at birth AND pinned the wrong number here as
// expected behavior.
func (s *DamageCustodyTestSuite) TestImmunityNegatesDamageThroughTheOwnedFold() {
	bus := events.NewEventBus()
	s.multiplyOnBus(bus, damage.Piercing, 0)

	out, err := s.biteOnBus(bus, monsters.NewWolf(secondWolfID).ToData(),
		&sequenceRoller{singles: []int{hitRoll, 18}, pair: []int{3, 4}, fallback: 2})
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Require().True(outcome.Hit, "the blow lands")
	s.Require().Zero(outcome.Damage, "and does nothing — immunity, not resistance")

	// The sheet is still handed back (ApplyDamage marks it seen either way);
	// what matters is that its hit points did not move.
	s.Require().Len(out.DirtyMonsters, 1)
	s.Require().Equal(11, out.DirtyMonsters[0].HitPoints, "full health — nothing got through")
}

// The fold reaches subscribers on THIS package's bus. A modifier installed by
// the test contributes, and its ref rides the component — which is the whole
// reason custody mattered: the fold has to happen where the subscribers are.
func (s *DamageCustodyTestSuite) TestASubscriberOnResolutionsBusContributes() {
	bus := events.NewEventBus()
	s.addFlatOnBus(bus, 5, damage.Piercing)

	out, err := s.biteOnBus(bus, monsters.NewWolf(secondWolfID).ToData(),
		&sequenceRoller{singles: []int{hitRoll, 18}, pair: []int{3, 4}, fallback: 2})
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Require().Equal(3+4+2+5, outcome.Damage, "the pool's 9 plus the fold's 5")
}

// Two modifiers of different types stay separate: 5e resists a TYPE, so a
// resistance to one does not touch the other, and both land.
func (s *DamageCustodyTestSuite) TestTypesAreGroupedSeparatelyThroughTheFold() {
	bus := events.NewEventBus()
	s.addFlatOnBus(bus, 6, damage.Fire)
	s.halveOnBus(bus, damage.Piercing)

	out, err := s.biteOnBus(bus, monsters.NewWolf(secondWolfID).ToData(),
		&sequenceRoller{singles: []int{hitRoll, 18}, pair: []int{3, 4}, fallback: 2})
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	// Piercing 9 halved to 4; fire 6 untouched.
	s.Require().Equal(4+6, outcome.Damage, "each type resolves on its own")
}

// The event's own DamageType is load-bearing, not decoration: Rage's
// resistance predicates on it twice — event.DamageType.IsPhysical() decides
// whether to resist at all, and the multiplier component it appends carries
// event.DamageType so the grouping puts it with the right damage.
//
// Real content, and the case a synthetic subscriber cannot pin: a raging
// barbarian takes half from the wolf's piercing bite.
func (s *DamageCustodyTestSuite) TestARagingTargetsResistanceReadsTheEventsDamageType() {
	world, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 5, Y: 5}},
			{ID: wolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 8, Y: 5}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	data := monsters.NewWolf(wolfID).ToData()
	attack, err := AttackFromMonsterAction(data.Actions[0])
	s.Require().NoError(err)

	raging, err := (&conditions.RagingCondition{
		CharacterID: heroID,
		DamageBonus: 2,
		Level:       1,
		Source:      "rage",
	}).ToJSON()
	s.Require().NoError(err)

	out, err := Resolve(s.ctx, &Input{
		World: world.ToData(),
		Participants: []Participant{
			{Character: s.ragingHero(raging)},
			{Monster: data},
		},
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   heroID,
			Attack:     attack,
			Roller:     &sequenceRoller{singles: []int{hitRoll, 18}, pair: []int{3, 4}, fallback: 2},
		}),
	})
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Require().True(outcome.Hit)
	s.Require().Equal(4, outcome.Damage,
		"2d4[3 4]+2 = 9 piercing, halved by rage to 4 — resistance the event's type selected")

	s.Require().Len(out.DirtyCharacters, 1)
	s.Require().Equal(14-4, out.DirtyCharacters[0].HitPoints)
}

// ragingHero is a barbarian who can take the hit: AC 14, 14 hit points.
func (s *DamageCustodyTestSuite) ragingHero(conds ...json.RawMessage) *character.Data {
	return &character.Data{
		ID:       heroID,
		PlayerID: "player-1",
		Name:     "Grog",
		Level:    1,
		ClassID:  classes.Barbarian,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:        14,
		MaxHitPoints:     14,
		ArmorClass:       14,
		ProficiencyBonus: 2,
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
		},
		Conditions: conds,
	}
}

// --- chain-subscriber helpers -------------------------------------------------

func (s *DamageCustodyTestSuite) onDamageChain(
	bus events.EventBus, name string, modify func(*dnd5eEvents.DamageChainEvent),
) {
	_, err := dnd5eEvents.DamageChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, _ *dnd5eEvents.DamageChainEvent,
			c chain.Chain[*dnd5eEvents.DamageChainEvent],
		) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
			err := c.Add(combat.StageFinal, name,
				func(_ context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
					modify(e)
					return e, nil
				})

			return c, err
		})
	s.Require().NoError(err)
}

// multiplyOnBus installs a modifier component carrying the given factor —
// the shape resistance, vulnerability, and immunity all take.
func (s *DamageCustodyTestSuite) multiplyOnBus(bus events.EventBus, t damage.Type, factor float64) {
	s.onDamageChain(bus, "test_multiplier_"+string(t), func(e *dnd5eEvents.DamageChainEvent) {
		e.Components = append(e.Components, dnd5eEvents.DamageComponent{
			Source:     dnd5eEvents.DamageSourceCondition,
			SourceRef:  &core.Ref{Module: "test", Type: "conditions", ID: "multiplier"},
			Multiplier: dnd5eEvents.Multiply(factor),
			DamageType: t,
		})
	})
}

func (s *DamageCustodyTestSuite) halveOnBus(bus events.EventBus, t damage.Type) {
	s.multiplyOnBus(bus, t, 0.5)
}

// addFlatOnBus contributes extra damage of a type, the way Rage or a Divine
// Smite does.
func (s *DamageCustodyTestSuite) addFlatOnBus(bus events.EventBus, amount int, t damage.Type) {
	s.onDamageChain(bus, "test_flat_"+string(t), func(e *dnd5eEvents.DamageChainEvent) {
		e.Components = append(e.Components, dnd5eEvents.DamageComponent{
			Source:     dnd5eEvents.DamageSourceCondition,
			SourceRef:  refs.Conditions.Raging(),
			FlatBonus:  amount,
			DamageType: t,
		})
	})
}

// roomWith puts two monsters a cell apart. Monster-versus-monster on purpose:
// it keeps a character's AC chain out of the arithmetic under test, and the
// damage fold is the same fold whoever is swinging.
func (s *DamageCustodyTestSuite) roomWith(attacker, target encounter.MemberID) encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: attacker, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 5, Y: 5}},
			{ID: target, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 5, Y: 6}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}
