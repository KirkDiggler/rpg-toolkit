// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const secondWolfID = "wolf-2"

// hitRoll beats the hero's AC 14 with the wolf's +4; missRoll does not.
const (
	hitRoll  = 15
	missRoll = 4
)

// sequenceRoller answers each Roll from a queue, so one interaction can script
// several different dice — an attack that hits and then a save that fails,
// which a single fixed value cannot express. RollN takes the fallback unless a
// pair is queued, which is what the damage dice consume.
type sequenceRoller struct {
	singles  []int
	pair     []int
	fallback int
}

func (r *sequenceRoller) Roll(_ context.Context, _ int) (int, error) {
	if len(r.singles) > 0 {
		next := r.singles[0]
		r.singles = r.singles[1:]

		return next, nil
	}

	return r.fallback, nil
}

func (r *sequenceRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	if len(r.pair) >= count {
		next := r.pair[:count]
		r.pair = r.pair[count:]

		return next, nil
	}

	out := make([]int, count)
	for i := range out {
		out[i] = r.fallback
	}

	return out, nil
}

// StrikeTestSuite drives the whole lane against catalog content: the wolf's own
// bite, the wolf's own gate, the prone condition's own range predicate.
type StrikeTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestStrikeSuite(t *testing.T) {
	suite.Run(t, new(StrikeTestSuite))
}

func (s *StrikeTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// wolfBite is the catalog wolf's bite as content persists it — the action data
// the strike machine reads its numbers out of.
func (s *StrikeTestSuite) wolfBite() monster.ActionData {
	data := monsters.NewWolf(wolfID).ToData()
	s.Require().Len(data.Actions, 1)

	return data.Actions[0]
}

// world places the hero and both wolves. adjacent puts wolf-2 one cell from the
// hero; otherwise it stands three cells away — the two sides of prone's range
// split.
func (s *StrikeTestSuite) world(secondWolfAt spatial.Position) encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 5, Y: 5}},
			{ID: wolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 5, Y: 6}},
			{ID: secondWolfID, Kind: encounter.KindMonster, Room: "room-1", Position: secondWolfAt},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

// hero has AC 14 and a +5 STR save: the wolf's +4 bite needs an 10 to hit, and
// its DC 11 knockdown is a real contest.
func (s *StrikeTestSuite) hero(conds ...json.RawMessage) *character.Data {
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

func (s *StrikeTestSuite) wolf(id string) *monster.Data {
	data := monsters.NewWolf(id).ToData()

	return data
}

func (s *StrikeTestSuite) raging() json.RawMessage {
	raw, err := (&conditions.RagingCondition{
		CharacterID: heroID,
		DamageBonus: 2,
		Level:       1,
		Source:      "rage",
	}).ToJSON()
	s.Require().NoError(err)

	return raw
}

// strike builds the interaction one wolf's bite raises against the hero.
func (s *StrikeTestSuite) strike(attacker string, roller *scriptedRoller) Machine {
	return NewStrike(&StrikeInput{
		AttackerID: attacker,
		TargetID:   heroID,
		Action:     s.wolfBite(),
		Roller:     roller,
	})
}

func (s *StrikeTestSuite) resolve(
	world encounter.EncounterData, hero *character.Data, machine Machine,
) (*Output, error) {
	return Resolve(s.ctx, &Input{
		World: world,
		Participants: []Participant{
			{Character: hero},
			{Monster: s.wolf(wolfID)},
			{Monster: s.wolf(secondWolfID)},
		},
		Machine: machine,
	})
}

func (s *StrikeTestSuite) strikeOutcome(out *Output) StrikeOutcome {
	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok, "a strike produces a StrikeOutcome")

	return outcome
}

// THE HEADLINE. The wolf's stat block is the whole input: its bite's attack
// bonus, its damage dice, and its declared knockdown. One Resolve later the
// hero has taken damage and is on the floor, in the data that comes back to be
// persisted.
func (s *StrikeTestSuite) TestAWolfBitesTheHeroAndKnocksHimDown() {
	// The attack rolls 15 (+4 beats AC 14); the damage dice take the fallback;
	// the hero's STR save then rolls 4, and 4 + 5 is under the gate's DC 11.
	out, err := s.resolve(s.world(spatial.Position{X: 8, Y: 5}), s.hero(), NewStrike(&StrikeInput{
		AttackerID: wolfID,
		TargetID:   heroID,
		Action:     s.wolfBite(),
		Roller:     &sequenceRoller{singles: []int{hitRoll, missRoll}, fallback: 2},
	}))
	s.Require().NoError(err)

	outcome := s.strikeOutcome(out)
	s.Require().True(outcome.Hit, "15 + 4 beats AC 14")
	s.Require().Equal(14, outcome.TargetAC)
	s.Require().Positive(outcome.Damage, "a bite that lands does damage")

	s.Require().NotNil(outcome.Contest, "the bite declares a knockdown, so it was contested")
	s.Require().False(outcome.Contest.Succeeded, "the save rolled 4, and 4 + 5 is under DC 11")
	s.Require().Equal(11, outcome.Contest.DC)

	s.Require().Len(out.DirtyCharacters, 1)
	hero := out.DirtyCharacters[0]
	s.Require().Less(hero.HitPoints, 14, "the damage is on the sheet")
	s.Require().Len(hero.Conditions, 1)
	s.Require().Contains(string(hero.Conditions[0]), refs.Conditions.Prone().ID)
}

// #962's residual, and the reason it could not be pinned there: a bite that
// misses rolls no save. Nothing here decides not to — the contest is simply
// never requested, because the strike ends at the miss.
func (s *StrikeTestSuite) TestABiteThatMissesRollsNoSave() {
	out, err := s.resolve(s.world(spatial.Position{X: 8, Y: 5}), s.hero(),
		s.strike(wolfID, &scriptedRoller{single: missRoll, pair: []int{missRoll, missRoll}}))
	s.Require().NoError(err)

	outcome := s.strikeOutcome(out)
	s.Require().False(outcome.Hit, "4 + 4 does not reach AC 14")
	s.Require().Zero(outcome.Damage, "and a miss does no damage")
	s.Require().Nil(outcome.Contest, "and rolls no save")

	s.Require().Empty(out.DirtyCharacters, "nothing about the hero changed")
}

// Prone's range predicate, near half — and the reason WithRoom lands in this
// PR. The hero is already prone; the second wolf is one cell away, so its bite
// has advantage.
func (s *StrikeTestSuite) TestASecondBiteFromAdjacentHasAdvantage() {
	out, err := s.resolve(
		s.world(spatial.Position{X: 5, Y: 4}), // one cell from the hero at (5,5)
		s.hero(s.proneBlob()),
		s.strike(secondWolfID, &scriptedRoller{single: missRoll, pair: []int{missRoll, hitRoll}}),
	)
	s.Require().NoError(err)

	outcome := s.strikeOutcome(out)
	s.Require().Equal(hitRoll, outcome.Roll, "only a rolled-twice-take-higher can produce this die")
	s.Require().Len(outcome.Folded.AdvantageSources, 1)
	s.Require().Equal(refs.Conditions.Prone(), outcome.Folded.AdvantageSources[0].SourceRef)
	s.Require().Empty(outcome.Folded.DisadvantageSources)
}

// Prone's range predicate, far half. Same prone hero, same roller — the wolf
// three cells away rolls at disadvantage instead, and takes the lower die.
func (s *StrikeTestSuite) TestASecondBiteFromRangeHasDisadvantage() {
	out, err := s.resolve(
		s.world(spatial.Position{X: 8, Y: 5}), // three cells from the hero
		s.hero(s.proneBlob()),
		s.strike(secondWolfID, &scriptedRoller{single: hitRoll, pair: []int{missRoll, hitRoll}}),
	)
	s.Require().NoError(err)

	outcome := s.strikeOutcome(out)
	s.Require().Equal(missRoll, outcome.Roll, "only a rolled-twice-take-lower can produce this die")
	s.Require().Len(outcome.Folded.DisadvantageSources, 1)
	s.Require().Equal(refs.Conditions.Prone(), outcome.Folded.DisadvantageSources[0].SourceRef)
	s.Require().Empty(outcome.Folded.AdvantageSources)
}

// proneBlob is the persisted prone condition, so the hero starts the
// interaction already on the floor.
func (s *StrikeTestSuite) proneBlob() json.RawMessage {
	raw, err := conditions.NewProneCondition(heroID).ToJSON()
	s.Require().NoError(err)

	return raw
}

// An effect on the target folds into the attack chain, attributed to its own
// ref — the same seam prone uses, proving the fold is general rather than
// prone-shaped. Dodging imposes disadvantage on attacks against its owner.
func (s *StrikeTestSuite) TestDodgingFoldsIntoTheAttackChain() {
	dodging, err := conditions.NewDodgingCondition(heroID).ToJSON()
	s.Require().NoError(err)

	out, resolveErr := s.resolve(s.world(spatial.Position{X: 8, Y: 5}), s.hero(dodging),
		s.strike(wolfID, &scriptedRoller{single: hitRoll, pair: []int{missRoll, hitRoll}}))
	s.Require().NoError(resolveErr)

	outcome := s.strikeOutcome(out)
	s.Require().Equal(missRoll, outcome.Roll, "disadvantage took the lower die")
	s.Require().Len(outcome.Folded.DisadvantageSources, 1)
	s.Require().Equal(refs.Conditions.Dodging(), outcome.Folded.DisadvantageSources[0].SourceRef)
}

// Two machines and four phases, one registration list: the ledger is the
// participants' and does not depend on the order they were passed in (R4).
func (s *StrikeTestSuite) TestRegistrationsDoNotDependOnInputOrder() {
	world := s.world(spatial.Position{X: 8, Y: 5})
	roller := func() *scriptedRoller {
		return &scriptedRoller{single: hitRoll, pair: []int{hitRoll, hitRoll}}
	}

	forward, err := Resolve(s.ctx, &Input{
		World: world,
		Participants: []Participant{
			{Character: s.hero(s.raging())},
			{Monster: s.wolf(wolfID)},
			{Monster: s.wolf(secondWolfID)},
		},
		Machine: s.strike(wolfID, roller()),
	})
	s.Require().NoError(err)

	reversed, err := Resolve(s.ctx, &Input{
		World: world,
		Participants: []Participant{
			{Monster: s.wolf(secondWolfID)},
			{Monster: s.wolf(wolfID)},
			{Character: s.hero(s.raging())},
		},
		Machine: s.strike(wolfID, roller()),
	})
	s.Require().NoError(err)

	s.Require().NotEmpty(forward.Hooks)
	s.Require().Equal(forward.Hooks, reversed.Hooks)
}

// RE-ENTERABILITY, DEMONSTRATED. The machine is driven by hand, one step at a
// time, with the loop this test writes rather than the package's — which is
// only possible because every phase boundary is a value the machine hands back
// and the machine's own fields are the only state. A resolution that kept
// anything on the Go stack between phases could not be walked like this, and
// could not be suspended at one of them either.
func (s *StrikeTestSuite) TestTheStrikeRunsInPieces() {
	world := s.world(spatial.Position{X: 8, Y: 5})
	surf := newSurface(events.NewEventBus())

	room, err := interactionRoom(world, []Participant{{Character: s.hero()}, {Monster: s.wolf(wolfID)}})
	s.Require().NoError(err)
	s.Require().NotNil(room, "the participants share a room, so one is installed")

	ctx := gamectx.WithRoom(s.ctx, room)
	cast, err := attachAll(ctx, surf, []Participant{
		{Character: s.hero()},
		{Monster: s.wolf(wolfID)},
	}, nil)
	s.Require().NoError(err)

	machine := s.strike(wolfID, &scriptedRoller{single: hitRoll, pair: []int{hitRoll, hitRoll}})

	// Step one: the machine yields, and has done nothing else.
	step, err := machine.Start(ctx, cast)
	s.Require().NoError(err)

	names := []string{}
	for i := 0; i < 10; i++ {
		switch typed := step.(type) {
		case Done:
			s.Require().Equal([]string{"attack chain", "damage chain"}, names,
				"every phase boundary was a value this test drove by hand")

			outcome, ok := typed.Outcome.(StrikeOutcome)
			s.Require().True(ok)
			s.Require().True(outcome.Hit)
			s.Require().Positive(outcome.Damage)

			return

		case Gather:
			names = append(names, typed.Name())
			step, err = typed.run(ctx, surf)
			s.Require().NoError(err)

		case Request:
			// The contest is a machine of its own; driving it is the same loop
			// one level down, which is the point of the case existing.
			out, runErr := drive(ctx, surf, typed.machine, cast)
			s.Require().NoError(runErr)
			step, err = typed.next(ctx, out)
			s.Require().NoError(err)

		default:
			s.Require().Failf("unexpected step", "%T", step)
		}
	}

	s.Require().Fail("the machine did not finish")
}

// Damage lands exactly once. It is applied to the sheet directly, and NOT
// announced on DamageReceivedTopic — because a monster's own sheet keeper
// subscribes to that topic and applies the damage again, which took this wolf
// from 11 to 3 on a 4-damage bite before the publish came out. See
// strikeMachine.afterDamage: the topic is an instruction to one listener and a
// notification to another, which is slice 2's classification to make.
func (s *StrikeTestSuite) TestAMonsterTargetTakesItsDamageOnce() {
	out, err := Resolve(s.ctx, &Input{
		World: s.world(spatial.Position{X: 5, Y: 4}),
		Participants: []Participant{
			{Character: s.hero()},
			{Monster: s.wolf(wolfID)},
			{Monster: s.wolf(secondWolfID)},
		},
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   secondWolfID,
			Action:     s.wolfBite(),
			Roller:     &sequenceRoller{singles: []int{hitRoll, missRoll}, fallback: 2},
		}),
	})
	s.Require().NoError(err)

	outcome := s.strikeOutcome(out)
	s.Require().True(outcome.Hit)
	s.Require().Positive(outcome.Damage)

	s.Require().Len(out.DirtyMonsters, 1)
	s.Require().Equal(secondWolfID, out.DirtyMonsters[0].ID)
	s.Require().Equal(11-outcome.Damage, out.DirtyMonsters[0].HitPoints,
		"exactly the damage the strike resolved, applied exactly once")
}

func (s *StrikeTestSuite) TestRefusesAStrikeItCannotRun() {
	world := s.world(spatial.Position{X: 8, Y: 5})

	s.Run("an action this build cannot read", func() {
		_, err := s.resolve(world, s.hero(), NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   heroID,
			Action:     monster.ActionData{Ref: *refs.MonsterActions.Scimitar()},
		}))
		s.Require().ErrorIs(err, ErrBadAttack)
	})

	s.Run("a target who is not a participant", func() {
		_, err := s.resolve(world, s.hero(), NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   "nobody",
			Action:     s.wolfBite(),
		}))
		s.Require().ErrorIs(err, ErrNoCombatant)
	})

	s.Run("no attacker or target named", func() {
		_, err := s.resolve(world, s.hero(), NewStrike(&StrikeInput{Action: s.wolfBite()}))
		s.Require().ErrorIs(err, ErrNilInput)
	})
}

// widenCritTo subscribes an attack-chain modifier that widens the critical
// threshold, the way a Champion-style effect would — the shape that made the
// crit-range-is-not-a-hit-range distinction matter (Copilot review, #1002).
func (s *StrikeTestSuite) widenCritTo(bus events.EventBus, threshold int) {
	_, err := dnd5eEvents.AttackChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, _ dnd5eEvents.AttackChainEvent,
			c chain.Chain[dnd5eEvents.AttackChainEvent],
		) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
			err := c.Add(combat.StageConditions, "widen_crit",
				func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
					e.CriticalThreshold = threshold
					return e, nil
				})

			return c, err
		})
	s.Require().NoError(err)
}

// resolveWith is resolve on a bus the test holds, so a test can subscribe its
// own chain modifiers before the machine folds.
func (s *StrikeTestSuite) resolveWith(
	bus events.EventBus, world encounter.EncounterData, hero *character.Data, machine Machine,
) (*Output, error) {
	return resolveOn(s.ctx, &Input{
		World: world,
		Participants: []Participant{
			{Character: hero},
			{Monster: s.wolf(wolfID)},
			{Monster: s.wolf(secondWolfID)},
		},
		Machine: machine,
	}, newSurface(bus))
}

// The crit range is not a hit range: with the threshold widened to 19, a 19
// against armor it cannot reach is still a miss. Before the fix, any roll in
// the crit range auto-hit.
func (s *StrikeTestSuite) TestAWidenedCritRangeDoesNotWidenTheHitRange() {
	bus := events.NewEventBus()
	s.widenCritTo(bus, 19)

	armored := s.hero()
	armored.ArmorClass = 25

	out, err := s.resolveWith(bus, s.world(spatial.Position{X: 8, Y: 5}), armored,
		NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   heroID,
			Action:     s.wolfBite(),
			Roller:     &sequenceRoller{singles: []int{19}, fallback: 2},
		}))
	s.Require().NoError(err)

	outcome := s.strikeOutcome(out)
	s.Require().False(outcome.Hit, "19 + 4 cannot reach AC 25, widened crit range or not")
	s.Require().False(outcome.Critical, "a roll that misses crits nothing")
	s.Require().Zero(outcome.Damage)
	s.Require().Nil(outcome.Contest)
}

// The companion direction: the same widened 19 against reachable armor both
// hits and crits — the range widened which hits crit, and only that.
func (s *StrikeTestSuite) TestAWidenedCritRangeCritsOnAHit() {
	bus := events.NewEventBus()
	s.widenCritTo(bus, 19)

	out, err := s.resolveWith(bus, s.world(spatial.Position{X: 8, Y: 5}), s.hero(),
		NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   heroID,
			Action:     s.wolfBite(),
			Roller:     &sequenceRoller{singles: []int{19, 11}, pair: []int{3, 4, 1, 2}, fallback: 2},
		}))
	s.Require().NoError(err)

	outcome := s.strikeOutcome(out)
	s.Require().True(outcome.Hit, "19 + 4 beats AC 14")
	s.Require().True(outcome.Critical, "and 19 is in the widened range")
	s.Require().Equal(3+4+1+2+2, outcome.Damage,
		"2d4 rolled twice — [3 4] then [1 2] — plus the +2 modifier exactly once")
}

// The notation's +2 is not a die: it lands once, on top of the dice as rolled.
// Before the fix it was dropped entirely.
func (s *StrikeTestSuite) TestDamageKeepsItsFlatModifierExactlyOnce() {
	out, err := s.resolve(s.world(spatial.Position{X: 8, Y: 5}), s.hero(),
		NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   heroID,
			Action:     s.wolfBite(),
			Roller:     &sequenceRoller{singles: []int{hitRoll, 11}, pair: []int{3, 4}, fallback: 2},
		}))
	s.Require().NoError(err)

	outcome := s.strikeOutcome(out)
	s.Require().True(outcome.Hit)
	s.Require().Equal(3+4+2, outcome.Damage, "2d4 rolled [3 4], plus the +2 — derived, not echoed")
}

// A natural twenty doubles the DICE and nothing else: two extra d4s arrive,
// the +2 modifier does not double, and the roll auto-hits at any armor.
func (s *StrikeTestSuite) TestANaturalTwentyDoublesTheDiceNotTheModifier() {
	armored := s.hero()
	armored.ArmorClass = 25

	out, err := s.resolve(s.world(spatial.Position{X: 8, Y: 5}), armored,
		NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   heroID,
			Action:     s.wolfBite(),
			Roller:     &sequenceRoller{singles: []int{20, 11}, pair: []int{3, 4, 1, 2}, fallback: 2},
		}))
	s.Require().NoError(err)

	outcome := s.strikeOutcome(out)
	s.Require().True(outcome.Hit, "a natural 20 hits AC 25 or any other")
	s.Require().True(outcome.Critical)
	s.Require().Equal(3+4+1+2+2, outcome.Damage,
		"four dice for the crit, the modifier exactly once")
}
