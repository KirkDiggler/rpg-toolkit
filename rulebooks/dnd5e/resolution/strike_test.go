// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
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
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
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
//
// The 14 is EARNED rather than declared: hide armor's 12 plus a capped +2 from
// DEX 14. It used to be a bare ArmorClass field on an unarmored sheet, which
// the strike honored only because it read that field directly — an unarmored
// character with DEX 14 is really AC 12, so the number was a fiction the flat
// read kept alive. Now that the strike folds the AC chain (#1018), the fixture
// has to be a character who could actually have this AC, and every assertion
// about 14 in this file stays true because the arithmetic agrees.
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
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeArmor, ID: string(armor.Hide), Quantity: 1},
		},
		EquipmentSlots: character.EquipmentSlots{
			character.SlotArmor: string(armor.Hide),
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
		Attack:     s.wolfAttack(),
		Roller:     roller,
	})
}

func (s *StrikeTestSuite) resolve(
	world encounter.EncounterData, hero *character.Data, machine Machine,
) (*Output, error) {
	return Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
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
		Attack:     s.wolfAttack(),
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

	forward, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World: world,
		Participants: []Participant{
			{Character: s.hero(s.raging())},
			{Monster: s.wolf(wolfID)},
			{Monster: s.wolf(secondWolfID)},
		},
		Machine: s.strike(wolfID, roller()),
	})
	s.Require().NoError(err)

	reversed, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
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

	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: world, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{},
	})
	s.Require().NoError(err)

	room, err := enc.Canvas()
	s.Require().NoError(err)
	s.Require().NotNil(room, "there is one world, and it is installed every time")

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
			s.Require().Equal([]string{"attack chain", "post attack roll", "damage chain"}, names,
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

func (s *StrikeTestSuite) TestStrikePublishesPostAttackRollForSubscribers() {
	bus := events.NewEventBus()
	var got *dnd5eEvents.PostAttackRollEvent
	_, err := dnd5eEvents.PostAttackRollChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, event *dnd5eEvents.PostAttackRollEvent,
			c chain.Chain[*dnd5eEvents.PostAttackRollEvent],
		) (chain.Chain[*dnd5eEvents.PostAttackRollEvent], error) {
			copy := *event
			got = &copy
			return c, nil
		})
	s.Require().NoError(err)

	_, err = s.resolveWith(bus, s.world(spatial.Position{X: 8, Y: 5}), s.hero(),
		NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   heroID,
			Attack:     s.wolfAttack(),
			Roller:     &sequenceRoller{singles: []int{hitRoll}, fallback: 2},
		}))
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Require().Equal(wolfID, got.AttackerID)
	s.Require().Equal(heroID, got.TargetID)
	s.Require().Equal(hitRoll, got.AttackRoll)
	s.Require().Equal(14, got.OriginalAC)
	s.Require().True(got.WouldHit)
}

// Damage lands exactly once. It is applied to the sheet directly, and NOT
// announced on DamageReceivedTopic — because a monster's own sheet keeper
// subscribes to that topic and applies the damage again, which took this wolf
// from 11 to 3 on a 4-damage bite before the publish came out. See
// strikeMachine.afterDamage: the topic is an instruction to one listener and a
// notification to another, which is slice 2's classification to make.
func (s *StrikeTestSuite) TestAMonsterTargetTakesItsDamageOnce() {
	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World: s.world(spatial.Position{X: 5, Y: 4}),
		Participants: []Participant{
			{Character: s.hero()},
			{Monster: s.wolf(wolfID)},
			{Monster: s.wolf(secondWolfID)},
		},
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   secondWolfID,
			Attack:     s.wolfAttack(),
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

	s.Run("an action this build cannot read is refused at compilation", func() {
		_, err := AttackFromMonsterAction(monster.ActionData{Ref: *refs.MonsterActions.Melee()})
		s.Require().ErrorIs(err, ErrBadAttack)
	})

	s.Run("a hand-built profile with no dice is refused by the machine", func() {
		_, err := s.resolve(world, s.hero(), NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   heroID,
			Attack:     AttackProfile{Ref: refs.MonsterActions.Bite()},
		}))
		s.Require().ErrorIs(err, ErrBadAttack)
	})

	s.Run("a target who is not a participant", func() {
		_, err := s.resolve(world, s.hero(), NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   "nobody",
			Attack:     s.wolfAttack(),
		}))
		s.Require().ErrorIs(err, ErrNoCombatant)
	})

	s.Run("no attacker or target named", func() {
		_, err := s.resolve(world, s.hero(), NewStrike(&StrikeInput{Attack: s.wolfAttack()}))
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
//
// secondWolf replaces the stock second wolf when a case needs a specific one —
// a stat block with an armor class no character could wear, say.
func (s *StrikeTestSuite) resolveWith(
	bus events.EventBus, world encounter.EncounterData, hero *character.Data, machine Machine,
	secondWolf ...*monster.Data,
) (*Output, error) {
	second := s.wolf(secondWolfID)
	if len(secondWolf) > 0 {
		second = secondWolf[0]
	}

	return resolveOn(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World: world,
		Participants: []Participant{
			{Character: hero},
			{Monster: s.wolf(wolfID)},
			{Monster: second},
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

	// A MONSTER wearing an absurd armor class, not a character.
	//
	// The rule under test needs a target the attack cannot reach, and 25 is
	// past anything 5e's armor can produce — plate and a shield stop at 20. A
	// character's AC is now computed from what they wear (#1018), so a flat 25
	// on a character sheet is simply ignored and the test would prove nothing.
	// A stat block's AC is a declared number by design: monsters have no AC
	// chain, GetEffectiveAC falls through to AC(), and 25 stands.
	armored := s.wolf(secondWolfID)
	armored.ArmorClass = 25

	out, err := s.resolveWith(bus, s.world(spatial.Position{X: 8, Y: 5}), s.hero(),
		NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   secondWolfID,
			Attack:     s.wolfAttack(),
			Roller:     &sequenceRoller{singles: []int{19}, fallback: 2},
		}), armored)
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
			Attack:     s.wolfAttack(),
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
			Attack:     s.wolfAttack(),
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
			Attack:     s.wolfAttack(),
			Roller:     &sequenceRoller{singles: []int{20, 11}, pair: []int{3, 4, 1, 2}, fallback: 2},
		}))
	s.Require().NoError(err)

	outcome := s.strikeOutcome(out)
	s.Require().True(outcome.Hit, "a natural 20 hits AC 25 or any other")
	s.Require().True(outcome.Critical)
	s.Require().Equal(3+4+1+2+2, outcome.Damage,
		"four dice for the crit, the modifier exactly once")
}

func (s *StrikeTestSuite) resolveProfile(
	profile AttackProfile, roller dice.Roller,
) (*Output, error) {
	return s.resolve(s.world(spatial.Position{X: 8, Y: 5}), s.hero(), NewStrike(&StrikeInput{
		AttackerID: wolfID,
		TargetID:   heroID,
		Attack:     profile,
		Roller:     roller,
	}))
}

func oozeProfile(pools ...damage.Damage) AttackProfile {
	return AttackProfile{
		Ref:         refs.MonsterActions.Melee(),
		AttackBonus: 4,
		Damage:      pools,
	}
}

func (s *StrikeTestSuite) TestTwoPoolsUseOneFoldAndOneApplication() {
	bus := events.NewEventBus()
	damageGathers := 0
	_, err := dnd5eEvents.DamageChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, _ *dnd5eEvents.DamageChainEvent,
			c chain.Chain[*dnd5eEvents.DamageChainEvent],
		) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
			damageGathers++
			return c, nil
		})
	s.Require().NoError(err)

	profile := oozeProfile(
		damage.Damage{Dice: "1d8", Type: damage.Bludgeoning, FlatBonus: 2},
		damage.Damage{Dice: "1d6", Type: damage.Acid},
	)
	machine := NewStrike(&StrikeInput{
		AttackerID: wolfID,
		TargetID:   heroID,
		Attack:     profile,
		Roller:     scripted(15, 4, 5),
	}).(*strikeMachine)
	applyDamageCalls := 0
	machine.applyDamage = func(
		ctx context.Context, target combat.Combatant, input *combat.ApplyDamageInput,
	) *combat.ApplyDamageResult {
		applyDamageCalls++
		return target.ApplyDamage(ctx, input)
	}

	out, err := s.resolveWith(bus, s.world(spatial.Position{X: 8, Y: 5}), s.hero(), machine)
	s.Require().NoError(err)

	struck := s.strikeOutcome(out)
	s.Equal(11, struck.Damage)
	s.Len(struck.DamageInstances, 2)
	s.Len(struck.DamageComponents, 2)
	s.Equal(1, damageGathers, "all pools travel through one damage fold")
	s.Equal(1, applyDamageCalls, "all typed instances enter one target application")
	s.Require().Len(out.DirtyCharacters, 1)
	s.Equal(14-11, out.DirtyCharacters[0].HitPoints,
		"the folded instances land together in one application")
}

func (s *StrikeTestSuite) TestSameTypePoolsExposeMarkedPrimaryMetadata() {
	bus := events.NewEventBus()
	var captured dnd5eEvents.DamageChainEvent
	damageGathers := 0
	_, err := dnd5eEvents.DamageChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, event *dnd5eEvents.DamageChainEvent,
			c chain.Chain[*dnd5eEvents.DamageChainEvent],
		) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
			damageGathers++
			captured = *event
			return c, nil
		})
	s.Require().NoError(err)

	profile := AttackProfile{
		Ref:             refs.Weapons.Shortsword(),
		AttackBonus:     5,
		AbilityUsed:     abilities.DEX,
		AbilityModifier: 3,
		Damage: []damage.Damage{
			{Dice: "1d4", Type: damage.Piercing},
			{
				Dice:       "1d6",
				Type:       damage.Piercing,
				Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			},
		},
	}
	out, err := s.resolveWith(bus, s.world(spatial.Position{X: 8, Y: 5}), s.hero(), NewStrike(&StrikeInput{
		AttackerID: wolfID,
		TargetID:   heroID,
		Attack:     profile,
		Roller:     scripted(15, 2, 4),
	}))
	s.Require().NoError(err)
	s.Require().NotNil(out)

	s.Equal(1, damageGathers)
	s.Require().Len(captured.Components, 3)
	s.Equal("1d6", captured.Components[1].Dice,
		"primary notation remains on the marked component, not the event envelope")
	s.Equal(damage.Piercing, captured.Components[1].DamageType)
}

func (s *StrikeTestSuite) TestCriticalDoublesEveryEligiblePoolButNoFlatBonus() {
	profile := oozeProfile(
		damage.Damage{Dice: "1d8", Type: damage.Bludgeoning, FlatBonus: 2},
		damage.Damage{Dice: "1d6", Type: damage.Acid},
	)
	out, err := s.resolveProfile(profile, scripted(20, 4, 5, 6, 3))
	s.Require().NoError(err)

	struck := s.strikeOutcome(out)
	s.Equal(20, struck.Damage)
	s.Require().Len(struck.DamageComponents, 2)
	s.Equal([]int{4, 5}, struck.DamageComponents[0].FinalDiceRolls)
	s.Equal([]int{6, 3}, struck.DamageComponents[1].FinalDiceRolls)
	s.True(struck.DamageComponents[0].IsCritical)
	s.True(struck.DamageComponents[1].IsCritical)
}

func (s *StrikeTestSuite) TestDoesNotCritPoolRollsOnlyOnce() {
	profile := oozeProfile(
		damage.Damage{Dice: "1d8", Type: damage.Bludgeoning},
		damage.Damage{Dice: "1d6", Type: damage.Acid, Properties: []damage.Property{damage.DoesNotCrit}},
	)
	roller := scripted(20, 4, 5, 6)
	out, err := s.resolveProfile(profile, roller)
	s.Require().NoError(err)

	struck := s.strikeOutcome(out)
	s.Equal(15, struck.Damage)
	s.Require().Len(struck.DamageComponents, 2)
	s.Equal([]int{4, 5}, struck.DamageComponents[0].FinalDiceRolls)
	s.True(struck.DamageComponents[0].IsCritical)
	s.Equal([]int{6}, struck.DamageComponents[1].FinalDiceRolls)
	s.False(struck.DamageComponents[1].IsCritical)
	s.Empty(roller.values, "the non-critical pool consumed no second roll")
}

func (s *StrikeTestSuite) TestTypedOutcomeMarksTheExactAbilityPool() {
	profile := AttackProfile{
		Ref:             refs.Weapons.Shortsword(),
		AttackBonus:     5,
		AbilityUsed:     abilities.DEX,
		AbilityModifier: 3,
		Damage: []damage.Damage{
			{Dice: "1d4", Type: damage.Piercing},
			{
				Dice:       "1d6",
				Type:       damage.Piercing,
				Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			},
		},
	}
	out, err := s.resolveProfile(profile, scripted(15, 2, 4))
	s.Require().NoError(err)

	struck := s.strikeOutcome(out)
	s.Equal(9, struck.Damage)
	s.Equal([]damage.Instance{{Amount: 9, Type: damage.Piercing}}, struck.DamageInstances)
	s.Require().Len(struck.DamageComponents, 3)
	s.Equal("1d4", struck.DamageComponents[0].Dice)
	s.Empty(struck.DamageComponents[0].Properties)
	s.Equal(0, struck.DamageComponents[0].FlatBonus)
	s.Equal("1d6", struck.DamageComponents[1].Dice)
	s.Equal([]damage.Property{damage.AddsAttackAbilityModifier}, struck.DamageComponents[1].Properties)
	s.Equal(0, struck.DamageComponents[1].FlatBonus)
	s.Equal(dnd5eEvents.DamageSourceAbility, struck.DamageComponents[2].Source)
	s.Equal(refs.Abilities.Dexterity(), struck.DamageComponents[2].SourceRef)
	s.Equal(damage.Piercing, struck.DamageComponents[2].DamageType)
	s.Equal(3, struck.DamageComponents[2].FlatBonus)
	s.False(struck.DamageComponents[2].IsCritical)
}

func (s *StrikeTestSuite) TestInvalidProfileConsumesNoRandomness() {
	roller := scripted(15, 4)
	_, err := s.resolveProfile(oozeProfile(
		damage.Damage{Dice: "1d8", Type: damage.Bludgeoning},
		damage.Damage{Dice: "1d6+2", Type: damage.Acid},
	), roller)

	s.Require().ErrorIs(err, ErrBadAttack)
	s.Zero(roller.rolls)
}

func (s *StrikeTestSuite) TestCanceledAdvantageRollsStraightAndDoesNotGrantSneakAttack() {
	sneak, err := conditions.NewSneakAttackCondition(conditions.SneakAttackInput{
		CharacterID: heroID,
		Level:       1,
	}).ToJSON()
	s.Require().NoError(err)

	bus := events.NewEventBus()
	_, err = dnd5eEvents.AttackChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, _ dnd5eEvents.AttackChainEvent,
			c chain.Chain[dnd5eEvents.AttackChainEvent],
		) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
			err := c.Add(combat.StageConditions, "canceled_advantage",
				func(_ context.Context, event dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
					event.AdvantageSources = append(event.AdvantageSources, dnd5eEvents.AttackModifierSource{
						SourceRef: refs.Conditions.Helped(),
					})
					event.DisadvantageSources = append(event.DisadvantageSources, dnd5eEvents.AttackModifierSource{
						SourceRef: refs.Conditions.Dodging(),
					})
					return event, nil
				})
			return c, err
		})
	s.Require().NoError(err)

	profile := AttackProfile{
		Ref:             refs.Weapons.Shortsword(),
		AttackBonus:     5,
		AbilityUsed:     abilities.DEX,
		AbilityModifier: 3,
		Damage: []damage.Damage{{
			Dice:       "1d6",
			Type:       damage.Piercing,
			Properties: []damage.Property{damage.AddsAttackAbilityModifier},
		}},
	}
	roller := scripted(15, 4)
	out, err := resolveOn(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World: s.world(spatial.Position{X: 8, Y: 5}),
		Participants: []Participant{
			{Character: s.hero(sneak)},
			{Monster: s.wolf(wolfID)},
			{Monster: s.wolf(secondWolfID)},
		},
		Machine: NewStrike(&StrikeInput{
			AttackerID: heroID,
			TargetID:   wolfID,
			Attack:     profile,
			Roller:     roller,
		}),
	}, newSurface(bus))
	s.Require().NoError(err)

	struck := s.strikeOutcome(out)
	s.Equal(15, struck.Roll, "advantage and disadvantage cancel to one d20")
	s.Require().Len(struck.Folded.AdvantageSources, 1)
	s.Require().Len(struck.Folded.DisadvantageSources, 1)
	s.Equal(7, struck.Damage)
	for _, component := range struck.DamageComponents {
		s.NotEqual(refs.Features.SneakAttack(), component.SourceRef,
			"canceled advantage alone cannot satisfy Sneak Attack")
	}
}

// wolfAttack compiles the catalog wolf's bite into the neutral profile the
// strike consumes — the monster half of the StrikeInput.Attack seam.
func (s *StrikeTestSuite) wolfAttack() AttackProfile {
	profile, err := AttackFromMonsterAction(s.wolfBite())
	s.Require().NoError(err)
	s.Require().NotNil(profile.Gate, "the catalog wolf declares its knockdown")

	return profile
}

// The compiler's second source: a stat-block weapon. The skeleton's shortsword
// is a generic MeleeAction — proof the profile seam is attack-kind-neutral on
// the monster side too, and that a weapon with no gate just hits.
func (s *StrikeTestSuite) TestASkeletonSwingsItsShortsword() {
	skeleton := monsters.NewSkeleton("skeleton").ToData()

	var sword monster.ActionData
	found := false
	for _, action := range skeleton.Actions {
		if action.Ref.ID == refs.MonsterActions.Melee().ID {
			sword, found = action, true
			break
		}
	}
	s.Require().True(found, "the catalog skeleton carries a melee weapon")

	attack, err := AttackFromMonsterAction(sword)
	s.Require().NoError(err)
	s.Require().Nil(attack.Gate, "a plain weapon declares no rider")

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 5, Y: 5}},
			{ID: "skeleton", Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 5, Y: 6}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World: enc.ToData(),
		Participants: []Participant{
			{Character: s.hero()},
			{Monster: skeleton},
		},
		Machine: NewStrike(&StrikeInput{
			AttackerID: "skeleton",
			TargetID:   heroID,
			Attack:     attack,
			Roller:     &sequenceRoller{singles: []int{hitRoll}, pair: []int{3}, fallback: 2},
		}),
	})
	s.Require().NoError(err)

	outcome := s.strikeOutcome(out)
	s.Require().True(outcome.Hit, "15 + 4 beats AC 14")
	s.Require().Equal(3+2, outcome.Damage, "1d6 rolled [3], plus the +2 — exact, not merely positive")
	s.Require().Nil(outcome.Contest, "no gate, no save, nobody falls over")

	s.Require().Len(out.DirtyCharacters, 1)
	s.Require().Equal(14-5, out.DirtyCharacters[0].HitPoints)
	s.Require().Empty(out.DirtyCharacters[0].Conditions)
}

// scoutID is the party member who is somewhere else. She never swings and is
// never swung at; her whole job is to be a participant standing in another
// region, which is the reference tomb's normal state and was enough to switch
// off every range predicate in the game (rpg-toolkit#1090).
const scoutID = "scout-1"

// spreadWorld is the tomb's normal state in miniature: the fight is in one
// chamber and a third party member is exploring another.
//
// Two rooms, disjoint (W2), the second anchored twelve cells east — so the
// scout's cell is nowhere near the fight and every question the fight asks is
// about room-1. The hero, both wolves and the scout are one roster, because
// that is what a cast is: session.castFor passes the WHOLE encounter roster,
// deliberately, since "deciding they are irrelevant would be this package
// deciding a rule" (ADR-0038).
func (s *StrikeTestSuite) spreadWorld(secondWolfRoom string, secondWolfAt spatial.Position) encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "room-1", Width: 10, Height: 10},
				{ID: "room-2", Width: 10, Height: 10, Origin: spatial.Position{X: 12}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 5, Y: 5}},
			{ID: wolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 5, Y: 6}},
			{ID: secondWolfID, Kind: encounter.KindMonster, Room: secondWolfRoom, Position: secondWolfAt},
			{ID: scoutID, Kind: encounter.KindPlayer, Room: "room-2", Position: spatial.Position{X: 5, Y: 5}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

// scout is the sheet of the party member in the other room. Deliberately plain:
// nothing about her is meant to influence the fight, and the point of the test
// is that her mere presence in the cast used to.
func (s *StrikeTestSuite) scout() *character.Data {
	return &character.Data{
		ID:       scoutID,
		PlayerID: "player-2",
		Name:     "Nyx",
		Level:    1,
		ClassID:  classes.Barbarian,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12,
			abilities.DEX: 14,
			abilities.CON: 12,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 10,
		},
		HitPoints:        11,
		MaxHitPoints:     11,
		ProficiencyBonus: 2,
	}
}

// resolveSpread is resolve with the scout in the cast — the whole roster, the
// way a session hands one over.
func (s *StrikeTestSuite) resolveSpread(
	world encounter.EncounterData, hero *character.Data, machine Machine,
) (*Output, error) {
	return Resolve(s.ctx, &Input{
		Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight:  everyoneSeesTheWholeMap{},
		Roller: dice.NewRoller(),
		World:  world,
		Participants: []Participant{
			{Character: hero},
			{Character: s.scout()},
			{Monster: s.wolf(wolfID)},
			{Monster: s.wolf(secondWolfID)},
		},
		Machine: machine,
	})
}

// #1090, THE HEADLINE. A prone hero with a wolf in the next cell, and one party
// member exploring the room next door.
//
// This is the same scene as TestASecondBiteFromAdjacentHasAdvantage — same
// prone hero, same wolf one cell away, same roller — with one thing changed
// that nothing in it is about: a fourth member of the cast standing somewhere
// else entirely. The rule must not notice her.
//
// It used to. The cast spanned two rooms, so no room was installed at all
// ("no single room describes this interaction"), prone's within-five-feet
// predicate had no positions to read, and the bite rolled flat. In the
// reference tomb that is not an edge case — a party spread across a dungeon is
// its NORMAL state, so every range predicate in the game was off whenever
// anybody wandered off.
func (s *StrikeTestSuite) TestProneConfersAdvantageWhileTheRestOfThePartyIsElsewhere() {
	out, err := s.resolveSpread(
		s.spreadWorld("room-1", spatial.Position{X: 5, Y: 4}), // one cell from the hero at (5,5)
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

// The far half of the same split, with the party equally spread — because a
// world that is installed but measured wrong answers "within five feet"
// everywhere, and a test that only pinned the near half would call that a pass.
func (s *StrikeTestSuite) TestProneStillCostsAtRangeWhileThePartyIsSpread() {
	out, err := s.resolveSpread(
		s.spreadWorld("room-1", spatial.Position{X: 8, Y: 5}), // three cells from the hero
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

// The distance the predicate reads is DUNGEON-ABSOLUTE, and this is the case
// that tells the two frames apart.
//
// The second wolf stands at room-2's local (5,4). The hero lies at room-1's
// local (5,5). Read ROOM-LOCALLY those are adjacent cells — one square, five
// feet, squarely inside prone's advantage half. Read absolutely the wolf is at
// (17,4) and the hero at (5,5): twelve cells, sixty feet, a different chamber.
//
// So a world built from unprojected coordinates does not merely measure
// oddly — it hands a wolf in the next room the advantage it would have if it
// were standing over him. Disadvantage is the right answer, and only the
// absolute frame produces it.
func (s *StrikeTestSuite) TestAWolfInTheNextChamberIsSixtyFeetAwayNotOneCell() {
	out, err := s.resolveSpread(
		s.spreadWorld("room-2", spatial.Position{X: 5, Y: 4}),
		s.hero(s.proneBlob()),
		s.strike(secondWolfID, &scriptedRoller{single: hitRoll, pair: []int{missRoll, hitRoll}}),
	)
	s.Require().NoError(err)

	outcome := s.strikeOutcome(out)
	s.Require().Equal(missRoll, outcome.Roll, "only a rolled-twice-take-lower can produce this die")
	s.Require().Len(outcome.Folded.DisadvantageSources, 1,
		"a chamber away is sixty feet, which is beyond prone's five")
	s.Require().Equal(refs.Conditions.Prone(), outcome.Folded.DisadvantageSources[0].SourceRef)
	s.Require().Empty(outcome.Folded.AdvantageSources)
}

// hexWorld is the same fight on a HEX field, and the two members' cells are
// chosen so that the two families disagree about them.
//
// The hero lies at axial (0,0) and the second wolf stands at axial (1,1). On a
// hex grid that is a distance of TWO — cube (0,0,0) to (1,1,-2), so
// (|1|+|1|+|2|)/2 — ten feet, outside prone's five. Read as a square grid the
// same two cells are Chebyshev ONE, adjacent, five feet, inside it.
//
// So the field's grid family is not decoration here: it decides the rule.
//
// The first wolf stands at axial (-5,-5) for a second reason. A hex grid is
// origin-CENTERED — a span of ten reaches [-5,4] on each axis — where a square
// grid of the same width reaches [0,9]. So the two families need different span
// arithmetic to hold the same field, and a member at the hex field's own corner
// is the only thing that can tell a correct hex span from a square one applied
// to a hex grid. (Mutation M8; nobody stood out there until it survived.)
func (s *StrikeTestSuite) hexWorld() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10, Grid: spatial.GridShapeHex}},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 0, Y: 0}},
			{ID: wolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: -5, Y: -5}},
			{ID: secondWolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

// A HEX FIELD IS MEASURED IN HEXES. The wolf two hexes from a prone hero rolls
// at disadvantage — and it is the same pair of coordinates that would be
// adjacent, and would confer advantage, if the world this package installs had
// quietly built a square grid instead.
//
// The whole suite was square until this test, so nothing anywhere in it could
// tell a hex field from a square one. That is exactly the shape of defect the
// family read exists to prevent, and it went unnoticed until a mutation that
// deleted the hex branch outright survived (M6).
func (s *StrikeTestSuite) TestAHexFieldIsMeasuredInHexes() {
	out, err := s.resolve(
		s.hexWorld(),
		s.hero(s.proneBlob()),
		s.strike(secondWolfID, &scriptedRoller{single: hitRoll, pair: []int{missRoll, hitRoll}}),
	)
	s.Require().NoError(err)

	outcome := s.strikeOutcome(out)
	s.Require().Equal(missRoll, outcome.Roll, "only a rolled-twice-take-lower can produce this die")
	s.Require().Len(outcome.Folded.DisadvantageSources, 1,
		"axial (0,0) to (1,1) is two hexes, which is ten feet")
	s.Require().Equal(refs.Conditions.Prone(), outcome.Folded.DisadvantageSources[0].SourceRef)
	s.Require().Empty(outcome.Folded.AdvantageSources, "and ten feet is beyond prone's five")
}
