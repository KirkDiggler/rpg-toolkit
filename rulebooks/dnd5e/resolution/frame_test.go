// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// FrameTestSuite holds the frame contract: the action facts both frames share,
// the information frame's mapping from what the actor knows, and the
// execution frame a strike builds once and hands to every rule it asks.
type FrameTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestFrameTestSuite(t *testing.T) {
	suite.Run(t, new(FrameTestSuite))
}

func (s *FrameTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// holdOut borrows the hold-out camp: a rogue with Sneak Attack, a fighter
// ally beside the raider scout, and the raider chief — a real run whose
// graph answers who is whose enemy.
func (s *FrameTestSuite) holdOut() *HoldOutSuite {
	fixtures := &HoldOutSuite{}
	fixtures.SetT(s.T())
	return fixtures
}

// resolveCamp runs one strike on the hold-out camp on the given bus.
func (s *FrameTestSuite) resolveCamp(bus events.EventBus, strike *StrikeInput) (*Output, error) {
	camp := s.holdOut()
	return resolveOn(s.ctx, &Input{
		World: camp.camp().ToData(),
		Participants: []Participant{
			{Character: camp.rogue()}, {Character: camp.ally()},
			{Monster: camp.raider(holdOutScout)}, {Monster: camp.raider(holdOutChief)},
		},
		Machine:    NewStrike(strike),
		Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		TurnDriver: passDriver{}, Roller: strike.Roller,
	}, newSurface(bus))
}

// monkSheet is a level-1 monk with Dexterity above Strength, empty-handed,
// carrying its persisted Martial Arts.
func (s *FrameTestSuite) monkSheet() *character.Data {
	martialArts, err := conditions.NewMartialArtsCondition(conditions.MartialArtsInput{
		MemberID: heroID, MonkLevel: 1,
	}).ToJSON()
	s.Require().NoError(err)

	return &character.Data{
		ID: heroID, PlayerID: "player-monk", Name: "Mira", Level: 1, ClassID: classes.Monk, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10, abilities.DEX: 16, abilities.CON: 12,
			abilities.INT: 10, abilities.WIS: 14, abilities.CHA: 8,
		},
		HitPoints: 9, MaxHitPoints: 9, ArmorClass: 15, ProficiencyBonus: 2,
		Conditions: []json.RawMessage{martialArts},
	}
}

// unarmedStrike assembles the monk's unarmed strike the way the host does:
// from the loaded sheet, with every override settled at assembly.
func (s *FrameTestSuite) unarmedStrike(data *character.Data) combatActions.Definition {
	monk, err := character.Load(s.ctx, data)
	s.Require().NoError(err)
	definition, err := character.AssembleAttack(monk, &character.AssembleAttackInput{Slot: character.SlotMainHand})
	s.Require().NoError(err)
	s.Require().NotNil(definition.Attack)
	return definition
}

func (s *FrameTestSuite) TestAttackActionFactsWeaponFinesse() {
	facts := attackActionFacts(dagger().Attack, false)

	s.Equal(contributions.Known(contributions.RollKindAttack), facts.Roll)
	s.Equal(contributions.Known(abilities.DEX), facts.Ability)
	s.Equal(contributions.Known(true), facts.Melee)
	s.Equal(contributions.Known(true), facts.WeaponPool)
	s.Equal(contributions.Unknown[bool](), facts.Advantage, "advantage is a fold result, not an assembly fact")
}

func (s *FrameTestSuite) TestAttackActionFactsSpellAttack() {
	facts := attackActionFacts(&combatActions.AttackProfile{
		Category: combatActions.AttackCategorySpell,
		Delivery: combatActions.AttackDelivery{Ranged: &combatActions.RangedDelivery{NormalFeet: 120}},
		Ability:  &combatActions.AbilityContribution{Ability: abilities.WIS, Modifier: 3},
		Damage: []damage.Damage{{
			Dice: "4d6", Type: damage.Radiant, Properties: []damage.Property{damage.AddsAttackAbilityModifier},
		}},
	}, false)

	s.Equal(contributions.Known(false), facts.WeaponPool, "a spell attack has no weapon pool, even one adding its ability")
	s.Equal(contributions.Known(false), facts.Melee)
	s.Equal(contributions.Known(abilities.WIS), facts.Ability)
}

func (s *FrameTestSuite) TestAttackActionFactsNoAbilityIsKnownNone() {
	facts := attackActionFacts(bite().Attack, false)

	ability, known := facts.Ability.Get()
	s.True(known, "a stat block that names no ability has answered")
	s.Equal(abilities.Ability(""), ability)
	s.Equal(contributions.Known(false), facts.WeaponPool, "no pool adds an attack ability modifier")
}

// TestAttackActionFactsForMonkUnarmedIsDexterity is R10 seen from the frame:
// Martial Arts settled the swing's ability at assembly, so the facts every
// rule reads already say Dexterity.
func (s *FrameTestSuite) TestAttackActionFactsForMonkUnarmedIsDexterity() {
	definition := s.unarmedStrike(s.monkSheet())

	facts := attackActionFacts(definition.Attack, false)

	s.Equal(contributions.Known(abilities.DEX), facts.Ability)
	s.Equal(contributions.Known(true), facts.Melee)
}

// TestMonkUnarmedHitRollsItsDamageDieOnce: the Martial Arts die is chosen
// before anything is rolled, so an unarmed hit throws the d20 and one d4 and
// nothing else — no die is rolled and then discarded.
func (s *FrameTestSuite) TestMonkUnarmedHitRollsItsDamageDieOnce() {
	monk := s.monkSheet()
	definition := s.unarmedStrike(monk)
	roller := &interactionRoller{script: []interactionRoll{
		{count: 1, sides: 20, faces: []int{15}},
		{count: 1, sides: 4, faces: []int{3}},
	}}

	out, err := Resolve(s.ctx, &Input{
		World:        actionWorld(s.T(), 2),
		Participants: []Participant{{Monster: monsters.NewWolf(wolfID).ToData()}, {Character: monk}},
		Machine:      NewStrike(&StrikeInput{AttackerID: heroID, TargetID: wolfID, Definition: definition, Roller: roller}),
		Initiative:   orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		TurnDriver: passDriver{}, Roller: roller,
	})
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Require().True(outcome.Hit, "precondition: the scripted swing lands")
	s.Equal([]string{"Roll(d20)", "RollN(1,d4)"}, roller.calls, "the d20 and the Martial Arts die, each once")
	s.Empty(roller.script)
	s.Equal(3+3, outcome.Damage, "1d4=3 plus Dexterity 3")
}

// TestInformationFrameMapsNilStanceToUnknown: an observer holding no belief
// about a pair has an unknown stance, never "no side"; the frame never claims
// to be complete, and advantage is not a fact information has.
func (s *FrameTestSuite) TestInformationFrameMapsNilStanceToUnknown() {
	hostile := encounter.StanceHostile
	out, err := informationFrame(&informationFrameInput{
		Observed: &encounter.ObservedContextOutput{
			Observer: holdOutRogue,
			Pairs: []encounter.ObservedContextPair{
				{From: holdOutRogue, To: holdOutScout, DistanceCells: 1, Stance: &hostile},
				{From: holdOutScout, To: holdOutLetter, DistanceCells: 2},
			},
		},
		Attack: dagger().Attack,
		Target: holdOutScout,
	})
	s.Require().NoError(err)

	frame := out.Frame
	s.Equal(holdOutRogue, frame.Actor)
	s.Equal(contributions.Known(holdOutScout), frame.Target)
	s.False(frame.Complete, "sightings never prove nobody else is there")
	s.Equal(contributions.Unknown[bool](), frame.Action.Advantage)
	s.Equal(contributions.Known(contributions.StanceHostile), frame.Pair(holdOutRogue, holdOutScout).Stance)
	unbelieved := frame.Pair(holdOutScout, holdOutLetter)
	s.Equal(contributions.Unknown[contributions.Stance](), unbelieved.Stance, "nil observed stance is unknown, not none")
	s.Equal(contributions.Known(2.0), unbelieved.DistanceCells)
}

// frameSpy records the frames the strike hands its rules.
type frameSpy struct {
	offered []contributions.Frame
	damaged []contributions.Frame
}

func (s *FrameTestSuite) watchFrames(bus events.EventBus) *frameSpy {
	spy := &frameSpy{}
	_, err := dnd5eEvents.PostRollOfferChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, e *dnd5eEvents.PostRollOfferEvent,
			c chain.Chain[*dnd5eEvents.PostRollOfferEvent],
		) (chain.Chain[*dnd5eEvents.PostRollOfferEvent], error) {
			spy.offered = append(spy.offered, e.Frame.Clone())
			return c, nil
		})
	s.Require().NoError(err)
	_, err = dnd5eEvents.DamageChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, e *dnd5eEvents.DamageChainEvent,
			c chain.Chain[*dnd5eEvents.DamageChainEvent],
		) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
			spy.damaged = append(spy.damaged, e.Frame.Clone())
			return c, nil
		})
	s.Require().NoError(err)
	return spy
}

// TestStrikeBuildsOneExecutionFrameForOfferAndDamage is O5: the offers and the
// damage fold read the same frame, built from authoritative state.
func (s *FrameTestSuite) TestStrikeBuildsOneExecutionFrameForOfferAndDamage() {
	bus := events.NewEventBus()
	spy := s.watchFrames(bus)
	roller := &interactionRoller{script: []interactionRoll{
		{count: 1, sides: 20, faces: []int{15}},
		{count: 1, sides: 4, faces: []int{3}},
		{count: 1, sides: 6, faces: []int{5}},
	}}

	_, err := s.resolveCamp(bus, &StrikeInput{AttackerID: holdOutRogue, TargetID: holdOutScout, Definition: dagger(), Roller: roller})
	s.Require().NoError(err)

	s.Require().Len(spy.offered, 1)
	s.Require().Len(spy.damaged, 1)
	s.Equal(spy.offered[0], spy.damaged[0], "one frame for the offer and the damage")

	frame := spy.damaged[0]
	s.Require().NoError(frame.Validate())
	s.Equal(holdOutRogue, frame.Actor)
	s.Equal(contributions.Known(holdOutScout), frame.Target)
	s.True(frame.Complete)
	s.Equal(contributions.Known(false), frame.Action.Advantage, "known: the fold granted nothing")
	s.Equal(contributions.Known(abilities.DEX), frame.Action.Ability)

	beside := frame.Pair(holdOutScout, holdOutAlly)
	s.Equal(contributions.Known(1.0), beside.DistanceCells)
	s.Equal(contributions.Known(contributions.StanceHostile), beside.Stance)
	s.Equal(contributions.Known(contributions.StanceAllied), frame.Pair(holdOutRogue, holdOutAlly).Stance)
	s.Len(frame.Pairs, 4*3, "every ordered pair of the four placed participants")
}

// TestStrikeSneakAttackFromExecutionFrame: the rogue's hit on the scout with
// the fighter beside it rolls Sneak Attack, decided from the frame alone.
func (s *FrameTestSuite) TestStrikeSneakAttackFromExecutionFrame() {
	roller := &interactionRoller{script: []interactionRoll{
		{count: 1, sides: 20, faces: []int{15}},
		{count: 1, sides: 4, faces: []int{3}},
		{count: 1, sides: 6, faces: []int{5}},
	}}

	out, err := s.resolveCamp(events.NewEventBus(),
		&StrikeInput{AttackerID: holdOutRogue, TargetID: holdOutScout, Definition: dagger(), Roller: roller})
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.True(sneakAttackFired(outcome), "components: %+v", outcome.DamageComponents)
	s.Equal(11, outcome.Damage, "1d4=3, Dexterity 3, Sneak Attack 1d6=5")
}

// TestStrikeFailsWhenARuleCannotAnswer is R13 at the interaction: a rule that
// cannot answer fails the action, so Resolve returns no world to save and the
// target's hit points never move.
func (s *FrameTestSuite) TestStrikeFailsWhenARuleCannotAnswer() {
	bus := events.NewEventBus()
	_, err := dnd5eEvents.DamageChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, _ *dnd5eEvents.DamageChainEvent,
			c chain.Chain[*dnd5eEvents.DamageChainEvent],
		) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
			return c, fmt.Errorf("spy rule: %w", contributions.ErrRuleCannotAnswer)
		})
	s.Require().NoError(err)
	target := monsters.NewWolf(wolfID).ToData()
	before := target.HitPoints

	out, err := resolveOn(s.ctx, &Input{
		World:        actionWorld(s.T(), 2),
		Participants: []Participant{{Monster: target}, {Character: actionHero()}},
		Machine: NewStrike(&StrikeInput{AttackerID: heroID, TargetID: wolfID, Definition: validMeleeDefinition(),
			Roller: &actionRoller{singles: []int{15}, damage: [][]int{{3}}}}),
		Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		TurnDriver: passDriver{}, Roller: &actionRoller{},
	}, newSurface(bus))

	s.Require().Error(err)
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer), "the rule's refusal reaches the caller: %v", err)
	s.Nil(out, "nothing comes back to be saved")
	s.Equal(before, target.HitPoints, "the target's record is untouched")
}

// TestResumedStrikeRebuildsFrameFromTruth is S3: a strike resumed after the
// post-roll offer was taken builds its damage frame afresh from current truth
// and the frozen fold — complete, never reconstructed from anything a client
// was shown.
func (s *FrameTestSuite) TestResumedStrikeRebuildsFrameFromTruth() {
	posed, err := heroSwings(s.T(), inspiredHero(s.T()), &actionRoller{singles: []int{8}})
	s.Require().NoError(err)
	s.Require().NotNil(posed.Posed)

	machine, err := NewStrikeResumed(&StrikeResumeInput{
		Frozen: posed.Posed.Frozen, Answer: OfferSpend,
		Roller: &actionRoller{singles: []int{4}, damage: [][]int{{3}}},
	})
	s.Require().NoError(err)
	bus := events.NewEventBus()
	spy := s.watchFrames(bus)

	out, err := resolveHeroStrikeOn(s.T(), inspiredHero(s.T()), machine, newSurface(bus))
	s.Require().NoError(err)
	s.Require().Nil(out.Posed)

	s.Empty(spy.offered, "the resumed half asks no offer again")
	s.Require().Len(spy.damaged, 1)
	frame := spy.damaged[0]
	s.Require().NoError(frame.Validate())
	s.True(frame.Complete)
	s.Equal(heroID, frame.Actor)
	s.Equal(contributions.Known(wolfID), frame.Target)
	s.Equal(contributions.Known(false), frame.Action.Advantage, "the frozen fold granted nothing")
	s.Equal(contributions.Known(1.0), frame.Pair(heroID, wolfID).DistanceCells)
}

// grantAdvantage has the fold grant the rogue advantage, as a condition would,
// so the execution frame's advantage is read from a fold that granted it.
func (s *FrameTestSuite) grantAdvantage(bus events.EventBus) {
	_, err := dnd5eEvents.AttackChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, e dnd5eEvents.AttackChainEvent,
			c chain.Chain[dnd5eEvents.AttackChainEvent],
		) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
			if e.AttackerID != holdOutRogue {
				return c, nil
			}
			return c, c.Add(combat.StageConditions, "granted-advantage",
				func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
					e.AdvantageSources = append(e.AdvantageSources, dnd5eEvents.AttackModifierSource{
						SourceRef: refs.Conditions.Dodging(), SourceID: holdOutAlly, Reason: "granted for the test",
					})
					return e, nil
				})
		})
	s.Require().NoError(err)
}

// TestGrantedAdvantageReachesTheExecutionFrame: the fold grants advantage and
// nobody of the party stands beside the chief, so only advantage can qualify
// Sneak Attack — the frame says known true and the sneak dice roll.
func (s *FrameTestSuite) TestGrantedAdvantageReachesTheExecutionFrame() {
	bus := events.NewEventBus()
	s.grantAdvantage(bus)
	spy := s.watchFrames(bus)
	roller := &interactionRoller{script: []interactionRoll{
		{count: 2, sides: 20, faces: []int{15, 15}},
		{count: 1, sides: 4, faces: []int{3}},
		{count: 1, sides: 6, faces: []int{5}},
	}}

	out, err := s.resolveCamp(bus, &StrikeInput{AttackerID: holdOutRogue, TargetID: holdOutChief, Definition: dagger(), Roller: roller})
	s.Require().NoError(err)

	s.Require().Len(spy.damaged, 1)
	s.Equal(contributions.Known(true), spy.damaged[0].Action.Advantage)
	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.True(sneakAttackFired(outcome), "components: %+v", outcome.DamageComponents)
	s.Empty(roller.script, "%v", roller.calls)
}

// TestCancelledAdvantageIsKnownFalseInTheExecutionFrame: advantage granted and
// disadvantage imposed cancel, so the frame says known false and, with no
// enemy of the chief beside it, Sneak Attack does not roll.
func (s *FrameTestSuite) TestCancelledAdvantageIsKnownFalseInTheExecutionFrame() {
	bus := events.NewEventBus()
	s.grantAdvantage(bus)
	spy := s.watchFrames(bus)
	roller := &interactionRoller{script: []interactionRoll{
		{count: 1, sides: 20, faces: []int{15}},
		{count: 1, sides: 4, faces: []int{3}},
	}}

	out, err := s.resolveCamp(bus, &StrikeInput{
		AttackerID: holdOutRogue, TargetID: holdOutChief, Definition: dagger(), Roller: roller,
		Imposed: []dnd5eEvents.AttackModifierSource{{
			SourceRef: refs.Conditions.Prone(), SourceID: holdOutRogue, Reason: "imposed for the test",
		}},
	})
	s.Require().NoError(err)

	s.Require().Len(spy.damaged, 1)
	s.Equal(contributions.Known(false), spy.damaged[0].Action.Advantage)
	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok)
	s.Require().True(outcome.Hit, "precondition: the scripted swing lands")
	s.NotEmpty(outcome.Folded.AdvantageSources, "precondition: advantage was granted")
	s.False(sneakAttackFired(outcome), "components: %+v", outcome.DamageComponents)
	s.Empty(roller.script, "%v", roller.calls)
}

// TestStrikeFailsWhenAnOfferRuleCannotAnswer is R13 on the offer side: a rule
// that cannot answer during the post-roll offer fails the action, so Resolve
// returns no world to save and no damage is applied.
func (s *FrameTestSuite) TestStrikeFailsWhenAnOfferRuleCannotAnswer() {
	bus := events.NewEventBus()
	_, err := dnd5eEvents.PostRollOfferChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, _ *dnd5eEvents.PostRollOfferEvent,
			c chain.Chain[*dnd5eEvents.PostRollOfferEvent],
		) (chain.Chain[*dnd5eEvents.PostRollOfferEvent], error) {
			return c, fmt.Errorf("spy offer rule: %w", contributions.ErrRuleCannotAnswer)
		})
	s.Require().NoError(err)
	damaged := s.watchFrames(bus)
	target := monsters.NewWolf(wolfID).ToData()
	before := target.HitPoints

	out, err := resolveOn(s.ctx, &Input{
		World:        actionWorld(s.T(), 2),
		Participants: []Participant{{Monster: target}, {Character: actionHero()}},
		Machine: NewStrike(&StrikeInput{AttackerID: heroID, TargetID: wolfID, Definition: validMeleeDefinition(),
			Roller: &actionRoller{singles: []int{15}, damage: [][]int{{3}}}}),
		Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		TurnDriver: passDriver{}, Roller: &actionRoller{},
	}, newSurface(bus))

	s.Require().Error(err)
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer), "the rule's refusal reaches the caller: %v", err)
	s.Nil(out, "nothing comes back to be saved")
	s.Empty(damaged.damaged, "no damage was folded")
	s.Equal(before, target.HitPoints, "the target's record is untouched")
}

// TestAPlacedMemberWithNoFactionIsKnownNoSide: an execution frame never
// leaves a placed pair's stance unknown. A world NPC belongs to no faction, so
// its pairs carry the known no side — never neutral, never unknown.
func (s *FrameTestSuite) TestAPlacedMemberWithNoFactionIsKnownNoSide() {
	const shopkeeperID = "shopkeeper"
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Mover: encounter.RefusingMover{},
		Announcer: quietAnnouncer{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		Field: encounter.FieldInput{Canvas: hexCanvas(), Regions: []encounter.RegionInput{rectRegion("room", 0, 0, 10, 4)}},
		Members: []encounter.MemberInput{
			{ID: wolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 1, Y: 1}},
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
			{ID: shopkeeperID, Kind: encounter.KindWorld, Position: spatial.Position{X: 0, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	shopkeeper := monsters.NewWolf(shopkeeperID).ToData()
	bus := events.NewEventBus()
	spy := s.watchFrames(bus)

	_, err = resolveOn(s.ctx, &Input{
		World: enc.ToData(),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()}, {Character: actionHero()}, {Monster: shopkeeper},
		},
		Machine: NewStrike(&StrikeInput{AttackerID: heroID, TargetID: wolfID, Definition: validMeleeDefinition(),
			Roller: &actionRoller{singles: []int{15}, damage: [][]int{{3}}}}),
		Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		TurnDriver: passDriver{}, Roller: &actionRoller{},
	}, newSurface(bus))
	s.Require().NoError(err)

	s.Require().Len(spy.damaged, 1)
	frame := spy.damaged[0]
	s.Equal(contributions.Known(contributions.StanceNone), frame.Pair(wolfID, shopkeeperID).Stance)
	s.Equal(contributions.Known(contributions.StanceNone), frame.Pair(shopkeeperID, heroID).Stance)
	s.Equal(contributions.Known(contributions.StanceHostile), frame.Pair(wolfID, heroID).Stance)
	s.Equal(contributions.Known(1.0), frame.Pair(wolfID, shopkeeperID).DistanceCells)
}
