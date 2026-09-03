// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
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
	attack := data.Actions[0]

	return resolveOn(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        s.roomWith(encounter.MemberID(wolfID), encounter.MemberID(target.ID)),
		Participants: []Participant{{Monster: data}, {Monster: target}},
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   target.ID,
			Definition: attack,
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
		&sequenceRoller{singles: []int{hitRoll, 18}, pair: []int{3, 4}})
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
		&sequenceRoller{singles: []int{hitRoll, 18}, pair: []int{3, 4}})
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
		&sequenceRoller{singles: []int{hitRoll, 18}, pair: []int{3, 4}})
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
		&sequenceRoller{singles: []int{hitRoll, 18}, pair: []int{3, 4}})
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	// Piercing 9 halved to 4; fire 6 untouched.
	s.Require().Equal(4+6, outcome.Damage, "each type resolves on its own")
}

// THE OUTCOME OWNS ITS TRACES. The folded damage event is publisher-owned
// state a chain subscriber can retain, so the outcome deep-clones every
// component at capture: mutating every roll fact on the publisher's event
// after the strike has reported cannot rewrite what it reported, and nothing
// the fold appended afterwards leaks in.
func (s *DamageCustodyTestSuite) TestTheStrikeOutcomeOwnsThePublisherTraceAfterTheFold() {
	var folded *dnd5eEvents.DamageChainEvent
	bus := events.NewEventBus()
	_, err := dnd5eEvents.DamageChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, e *dnd5eEvents.DamageChainEvent,
			c chain.Chain[*dnd5eEvents.DamageChainEvent],
		) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
			folded = e
			return c, nil
		})
	s.Require().NoError(err)

	definition := oozeProfile(damage.Damage{
		Dice: "1d8", Type: damage.Bludgeoning, FlatBonus: 2,
		Properties: []damage.Property{damage.AddsAttackAbilityModifier},
	})
	definition.Attack.Ability = &combatActions.AbilityContribution{Ability: abilities.STR, Modifier: 3}

	target := monsters.NewWolf(secondWolfID).ToData()
	out, err := resolveOn(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        s.roomWith(encounter.MemberID(wolfID), encounter.MemberID(target.ID)),
		Participants: []Participant{{Monster: monsters.NewWolf(wolfID).ToData()}, {Monster: target}},
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   target.ID,
			Definition: definition,
			Roller:     &sequenceRoller{singles: []int{15}, pair: []int{5}},
		}),
	}, newSurface(bus))
	s.Require().NoError(err)

	struck, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Require().Len(struck.DamageComponents, 2)
	s.Require().NotNil(folded)

	// MUTATE EVERY PUBLISHER-OWNED FACT after the fold captured them: faces,
	// subtotal, reroll history, provider identity, modifier pointer, damage
	// properties — and append a component the outcome must never see.
	weaponEvent := &folded.Components[0]
	weaponEvent.Roll.Dice.OriginalRolls[0] = 99
	weaponEvent.Roll.Dice.FinalRolls[0] = 99
	weaponEvent.Roll.Dice.Subtotal = 999
	weaponEvent.Roll.Dice.Rerolls = append(weaponEvent.Roll.Dice.Rerolls, dnd5eEvents.DiceReroll{
		DieIndex: 0, Before: 5, After: 1,
		Source: dnd5eEvents.RollSource{Ref: refs.Conditions.Raging(), Name: "Raging"},
	})
	weaponEvent.Roll.Source.Name = "tampered"
	weaponEvent.Roll.Source.Ref.ID = "tampered"
	*weaponEvent.Roll.Modifier = 99
	weaponEvent.Properties[0] = damage.DoesNotCrit
	folded.Components = append(folded.Components, dnd5eEvents.DamageComponent{
		Source: dnd5eEvents.DamageSourceSpell,
		Roll:   dnd5eEvents.RollComponent{Source: dnd5eEvents.RollSource{Name: "late arrival"}},
	})

	weapon := struck.DamageComponents[0]
	s.Equal([]int{5}, weapon.Roll.Dice.OriginalRolls)
	s.Equal([]int{5}, weapon.Roll.Dice.FinalRolls)
	s.Equal(5, weapon.Roll.Dice.Subtotal)
	s.Empty(weapon.Roll.Dice.Rerolls)
	s.Equal("Ooze Strike", weapon.Roll.Source.Name)
	s.Equal(refs.MonsterActions.WolfBite(), weapon.Roll.Source.Ref,
		"the outcome's identity ref is an owned copy, not the publisher's")
	s.Equal(2, *weapon.Roll.Modifier)
	s.Equal([]damage.Property{damage.AddsAttackAbilityModifier}, weapon.Properties)

	ability := struck.DamageComponents[1]
	s.Equal("Strength", ability.Roll.Source.Name)
	s.Equal(refs.Abilities.Strength(), ability.Roll.Source.Ref)
	s.NotNil(ability.Roll.Modifier)
	s.Equal(3, *ability.Roll.Modifier)

	s.Len(struck.DamageComponents, 2, "a component appended after capture does not leak in")
	s.Equal(10, struck.Damage, "5 pool + 2 flat + 3 Strength, settled during the fold")
}

// THE HEADLINE FOR DAMAGE: the strike preserves the root chain's complete
// trace — original faces, the ordered sourced reroll, final faces, subtotal,
// and the Strength modifier — instead of summing the pool into a scalar. The
// Great Weapon Fighting chain attachment rerolls INSIDE the fold; resolution
// carries the history it settled and rebuilds nothing.
//
// The GWF condition is constructed with its own roller because that constructor
// is the injection point the root exposes: a condition attached from a
// persisted sheet is built by the factory with a nil roller and falls back to
// the crypto-backed default. The strike's own roller scripts the attack roll
// and the weapon pool.
func TestGreatWeaponFightingTraceSurvivesTheStrike(t *testing.T) {
	bus := events.NewEventBus()
	gwf := conditions.NewFightingStyleGreatWeaponFightingCondition(
		heroID, &sequenceRoller{singles: []int{4}},
	)
	require.NoError(t, gwf.Apply(context.Background(), bus))

	definition := combatActions.Definition{
		Ref:  *refs.Weapons.Greatsword(),
		Name: "Greatsword",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 4,
			Ability:     &combatActions.AbilityContribution{Ability: abilities.STR, Modifier: 3},
			Weapon:      &combatActions.WeaponContext{TwoHanded: true},
			Damage: []damage.Damage{{
				Dice:       "2d6",
				Type:       damage.Slashing,
				Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			}},
		},
	}

	out, err := resolveOn(context.Background(), &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        actionWorld(t, 2),
		Participants: []Participant{{Character: actionHero()}, {Monster: monsters.NewWolf(wolfID).ToData()}},
		Machine: NewStrike(&StrikeInput{
			AttackerID: heroID,
			TargetID:   wolfID,
			Definition: definition,
			Roller:     &sequenceRoller{singles: []int{hitRoll}, pair: []int{1, 5}},
		}),
	}, newSurface(bus))
	require.NoError(t, err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	require.True(t, ok)
	require.True(t, outcome.Hit)
	require.False(t, outcome.Critical)
	require.Equal(t, 12, outcome.Damage, "final [4,5] subtotal 9 plus Strength 3")

	require.Len(t, outcome.DamageComponents, 2)
	weapon := outcome.DamageComponents[0]
	require.Equal(t, dnd5eEvents.DamageSourceWeapon, weapon.Source)
	require.Equal(t, refs.Weapons.Greatsword(), weapon.Roll.Source.Ref)
	require.Equal(t, "Greatsword", weapon.Roll.Source.Name,
		"the weapon component's provider name comes from the compiled definition")
	require.Equal(t, damage.Slashing, weapon.DamageType)
	require.Equal(t, []damage.Property{damage.AddsAttackAbilityModifier}, weapon.Properties)
	require.False(t, weapon.IsCritical)
	require.Nil(t, weapon.Roll.Modifier, "the pool declares no flat bonus of its own")
	require.NotNil(t, weapon.Roll.Dice)
	trace := weapon.Roll.Dice
	require.Equal(t, "2d6", trace.Notation)
	require.Equal(t, 6, trace.DieSize)
	require.Equal(t, []int{1, 5}, trace.OriginalRolls, "originals are never rewritten")
	require.Equal(t, []dnd5eEvents.DiceReroll{{
		DieIndex: 0, Before: 1, After: 4,
		Source: dnd5eEvents.RollSource{
			Ref:  refs.Conditions.FightingStyleGreatWeaponFighting(),
			Name: "Great Weapon Fighting",
		},
	}}, trace.Rerolls, "the reroll is ordered and sourced to the condition")
	require.Equal(t, []int{4, 5}, trace.FinalRolls)
	require.Equal(t, 9, trace.Subtotal)
	require.Empty(t, trace.KeptIndices)

	ability := outcome.DamageComponents[1]
	require.Equal(t, dnd5eEvents.DamageSourceAbility, ability.Source)
	require.Equal(t, refs.Abilities.Strength(), ability.Roll.Source.Ref,
		"the ability component's identity comes from the canonical ref")
	require.Equal(t, "Strength", ability.Roll.Source.Name,
		"the ability component's name comes from the ability's display authority")
	require.Nil(t, ability.Roll.Dice)
	require.NotNil(t, ability.Roll.Modifier)
	require.Equal(t, 3, *ability.Roll.Modifier)
	require.Equal(t, damage.Slashing, ability.DamageType)

	require.Equal(t, []damage.Instance{{Amount: 12, Type: damage.Slashing}}, outcome.DamageInstances)
}

func oozeProfile(pools ...damage.Damage) combatActions.Definition {
	return combatActions.Definition{
		Ref:  *refs.MonsterActions.WolfBite(),
		Name: "Ooze Strike",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 4,
			Damage:      pools,
		},
	}
}

// ADR-0041's explicit boundary guard: multiple declared pools still produce
// one DamageChain fold and one call through Strike's application seam.
func (s *DamageCustodyTestSuite) TestTwoPoolsUseOneFoldAndOneApplication() {
	bus := events.NewEventBus()
	damageFolds := 0
	_, err := dnd5eEvents.DamageChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, _ *dnd5eEvents.DamageChainEvent,
			c chain.Chain[*dnd5eEvents.DamageChainEvent],
		) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
			damageFolds++
			return c, nil
		})
	s.Require().NoError(err)

	definition := oozeProfile(
		damage.Damage{Dice: "1d8", Type: damage.Bludgeoning, FlatBonus: 2},
		damage.Damage{Dice: "1d6", Type: damage.Acid},
	)
	target := monsters.NewWolf(secondWolfID).ToData()
	startingHitPoints := target.HitPoints
	machine := NewStrike(&StrikeInput{
		AttackerID: wolfID,
		TargetID:   target.ID,
		Definition: definition,
		Roller:     &sequenceRoller{singles: []int{15}, pair: []int{4, 5}},
	}).(*strikeMachine)
	applyDamageCalls := 0
	machine.applyDamage = func(
		ctx context.Context, combatant combat.Combatant, input *combat.ApplyDamageInput,
	) *combat.ApplyDamageResult {
		applyDamageCalls++
		return combatant.ApplyDamage(ctx, input)
	}

	out, err := resolveOn(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World: s.roomWith(encounter.MemberID(wolfID), encounter.MemberID(target.ID)),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()},
			{Monster: target},
		},
		Machine: machine,
	}, newSurface(bus))
	s.Require().NoError(err)

	struck, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Equal(11, struck.Damage)
	s.Len(struck.DamageInstances, 2)
	s.Len(struck.DamageComponents, 2)
	s.Equal(1, damageFolds, "all pools travel through one DamageChain fold")
	s.Equal(1, applyDamageCalls, "all typed instances enter one ApplyDamage call")
	s.Require().Len(out.DirtyMonsters, 1)
	s.Equal(startingHitPoints-struck.Damage, out.DirtyMonsters[0].HitPoints,
		"the folded instances land together in one application")
}

func (s *DamageCustodyTestSuite) TestTypedOutcomePreservesMixedVulnerabilityAndImmunity() {
	bus := events.NewEventBus()
	s.multiplyOnBus(bus, damage.Bludgeoning, 2)
	s.multiplyOnBus(bus, damage.Acid, 0)

	profile := oozeProfile(
		damage.Damage{Dice: "1d8", Type: damage.Bludgeoning, FlatBonus: 2},
		damage.Damage{Dice: "1d6", Type: damage.Acid},
	)
	target := monsters.NewWolf(secondWolfID).ToData()
	out, err := resolveOn(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        s.roomWith(encounter.MemberID(wolfID), encounter.MemberID(target.ID)),
		Participants: []Participant{{Monster: monsters.NewWolf(wolfID).ToData()}, {Monster: target}},
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   target.ID,
			Definition: profile,
			Roller:     &sequenceRoller{singles: []int{15}, pair: []int{4, 5}},
		}),
	}, newSurface(bus))
	s.Require().NoError(err)

	struck, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Equal(12, struck.Damage, "bludgeoning 6 doubles while acid 5 is immune")
	s.Equal([]damage.Instance{{Amount: 12, Type: damage.Bludgeoning}}, struck.DamageInstances)
	s.Require().Len(struck.DamageComponents, 4,
		"the outcome retains both rolled pools and both typed defense components")
}

// The event's own DamageType is load-bearing, not decoration: Rage's
// resistance predicates on it twice — event.DamageType.IsPhysical() decides
// whether to resist at all, and the multiplier component it appends carries
// event.DamageType so the grouping puts it with the right damage.
//
// Real content, and the case a synthetic subscriber cannot pin: a raging
// barbarian takes half from the wolf's piercing bite.
func (s *DamageCustodyTestSuite) TestARagingTargetsResistanceReadsTheEventsDamageType() {
	world, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
			{ID: wolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 6}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	data := monsters.NewWolf(wolfID).ToData()
	attack := data.Actions[0]

	raging, err := (&conditions.RagingCondition{
		CharacterID: heroID,
		DamageBonus: 2,
		Level:       1,
		Source:      "rage",
	}).ToJSON()
	s.Require().NoError(err)

	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World: world.ToData(),
		Participants: []Participant{
			{Character: s.ragingHero(raging)},
			{Monster: data},
		},
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   heroID,
			Definition: attack,
			Roller:     &sequenceRoller{singles: []int{hitRoll}, pair: []int{3, 4, 18, 2}},
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
			Source: dnd5eEvents.DamageSourceCondition,
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{
					Ref:  &core.Ref{Module: "test", Type: "conditions", ID: "multiplier"},
					Name: "Test Multiplier",
				},
			},
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
		bonus := amount
		e.Components = append(e.Components, dnd5eEvents.DamageComponent{
			Source: dnd5eEvents.DamageSourceCondition,
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{
					Ref:  refs.Conditions.Raging(),
					Name: "Raging",
				},
				Modifier: &bonus,
			},
			DamageType: t,
		})
	})
}

// roomWith puts two monsters a cell apart. Monster-versus-monster on purpose:
// it keeps a character's AC chain out of the arithmetic under test, and the
// damage fold is the same fold whoever is swinging.
func (s *DamageCustodyTestSuite) roomWith(attacker, target encounter.MemberID) encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: attacker, Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 5}},
			{ID: target, Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 6}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}
