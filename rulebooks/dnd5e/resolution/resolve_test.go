// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// scriptedRoller makes advantage detectable from the result alone: a straight
// roll yields 3, a rolled-twice-take-higher yields 18. The assertion is on a
// value that only one path can produce, rather than on a flag the code under
// test also sets.
type scriptedRoller struct {
	single int
	pair   []int
}

func (r *scriptedRoller) Roll(_ context.Context, _ int) (int, error) { return r.single, nil }

func (r *scriptedRoller) RollN(_ context.Context, _, _ int) ([]int, error) { return r.pair, nil }

const (
	straightRoll   = 3
	advantageRoll  = 18
	heroSaveBonus  = 5 // STR 16 (+3), proficient, proficiency bonus 2
	heroID         = "hero"
	skeletonID     = "skeleton"
	wolfID         = "wolf"
	saveDifficulty = 12
)

type ResolveTestSuite struct {
	suite.Suite

	ctx    context.Context
	roller *scriptedRoller
}

func (s *ResolveTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.roller = &scriptedRoller{single: straightRoll, pair: []int{straightRoll, advantageRoll}}
}

// world is a two-member encounter, already normalised by a load/save cycle so
// that "unchanged" can be asserted literally.
func (s *ResolveTestSuite) world() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: skeletonID, Kind: encounter.KindMonster, Position: spatial.Position{X: 5, Y: 5}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Actors: encounter.Actors{
				Striker:   noAttacksExpected{},
				Mover:     encounter.RefusingMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

func (s *ResolveTestSuite) barbarian(conds ...json.RawMessage) *character.Data {
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
		ProficiencyBonus: 2,
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
			abilities.CON: shared.Proficient,
		},
		Conditions: conds,
	}
}

func (s *ResolveTestSuite) raging() json.RawMessage {
	raw, err := (&conditions.RagingCondition{
		CharacterID: heroID,
		Source:      "rage",
	}).ToJSON()
	s.Require().NoError(err)

	return raw
}

func (s *ResolveTestSuite) dodging() json.RawMessage {
	raw, err := conditions.NewDodgingCondition(heroID).ToJSON()
	s.Require().NoError(err)

	return raw
}

func (s *ResolveTestSuite) shortsword() combatActions.Definition {
	return combatActions.Definition{
		Ref:  *refs.Weapons.Shortsword(),
		Name: "shortsword",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 4,
			Damage:      []damage.Damage{{Dice: "1d6", Type: damage.Piercing, FlatBonus: 2}},
		},
	}
}

// skeleton carries a trait that has nothing to do with saving throws, which is
// what makes it useful: it proves an irrelevant participant attaches and then
// contributes nothing. It also carries an action, because monster.ToData
// serializes actions — see TestAMonstersActionsSurviveResolution.
func (s *ResolveTestSuite) skeleton() *monster.Data {
	raw, err := json.Marshal(monstertraits.ImmunityData{
		Ref:        refs.MonsterTraits.Immunity(),
		OwnerID:    skeletonID,
		DamageType: "piercing",
	})
	s.Require().NoError(err)

	return &monster.Data{
		ID:            skeletonID,
		Name:          "Skeleton",
		Ref:           refs.Monsters.Skeleton(),
		HitPoints:     13,
		MaxHitPoints:  13,
		ArmorClass:    13,
		AbilityScores: shared.AbilityScores{},
		Actions:       []combatActions.Definition{s.shortsword()},
		Conditions:    []json.RawMessage{raw},
	}
}

func (s *ResolveTestSuite) wolf() *monster.Data {
	return monsters.NewWolf(wolfID).ToData()
}

// captureOutcome is what captureMachine produces. The Outcome set is sealed, so
// a machine written for a test still has to declare one.
type captureOutcome struct{}

func (captureOutcome) isOutcome() {}

// captureMachine folds nothing and holds on to the cast, so a test can assert on
// the sheets resolution actually loaded and attached rather than on the sheets it
// was handed.
type captureMachine struct {
	cast *Participants
}

func (m *captureMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	m.cast = cast

	return Done{Outcome: captureOutcome{}}, nil
}

func (s *ResolveTestSuite) save(ability abilities.Ability) Machine {
	return NewSave(&SaveInput{
		SaverID: heroID,
		Ability: ability,
		DC:      saveDifficulty,
		D20Source: dnd5eEvents.RollSource{
			Ref: refs.Spells.ViciousMockery(), Name: "Test Save",
		},
		Roller: s.roller,
	})
}

func (s *ResolveTestSuite) outcomeOf(out *Output) SaveOutcome {
	s.Require().NotNil(out)
	outcome, ok := out.Outcome.(SaveOutcome)
	s.Require().True(ok, "a save machine produces a SaveOutcome")

	return outcome
}

// THE HEADLINE. Nobody attached anything. The caller passed data — a world, a
// sheet with a persisted Raging condition, and "roll a STR save" — and Raging's
// own predicate decided it applied. This single assertion is ADR-0038 end to
// end.
func (s *ResolveTestSuite) TestRagingBarbarianGetsAdvantageOnAStrengthSave() {
	out, err := Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Character: s.barbarian(s.raging())}},
		Machine:      s.save(abilities.STR),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})
	s.Require().NoError(err)

	got := s.outcomeOf(out)
	s.Require().Equal(advantageRoll, got.Result.Roll,
		"only a rolled-twice-take-higher can produce this die")
	s.Require().Equal(advantageRoll+heroSaveBonus, got.Result.Total)
	s.Require().True(got.Result.Success)

	keep := keepOf(s.T(), got.Result.Calculation)
	s.Require().NotNil(keep)
	s.Require().Len(keep.Granted, 1)
	s.Equal(refs.Conditions.Raging().String(), keep.Granted[0].Ref.String(),
		"and it is Raging that says so, not the wiring")
}

// The control that makes the headline mean something: the same barbarian, the
// same save, no condition — a straight roll.
func (s *ResolveTestSuite) TestTheSameBarbarianWithoutRageRollsStraight() {
	out, err := Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Character: s.barbarian()}},
		Machine:      s.save(abilities.STR),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})
	s.Require().NoError(err)

	got := s.outcomeOf(out)
	s.Require().Equal(straightRoll, got.Result.Roll)
	s.Require().Nil(keepOf(s.T(), got.Result.Calculation), "nobody touched the pool")
}

// The second effect, on a different chain, through the same machinery.
func (s *ResolveTestSuite) TestDodgingGrantsAdvantageOnADexteritySave() {
	out, err := Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Character: s.barbarian(s.dodging())}},
		Machine:      s.save(abilities.DEX),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})
	s.Require().NoError(err)

	got := s.outcomeOf(out)
	s.Require().Equal(advantageRoll, got.Result.Roll)
	s.Require().Equal(refs.Conditions.Dodging().String(),
		keepOf(s.T(), got.Result.Calculation).Granted[0].Ref.String())
}

// Applicability is the effect's own predicate, never resolution's. Raging is
// attached for a DEX save exactly as it is for a STR save, and declines.
func (s *ResolveTestSuite) TestRagingDeclinesADexteritySaveOnItsOwn() {
	out, err := Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Character: s.barbarian(s.raging())}},
		Machine:      s.save(abilities.DEX),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})
	s.Require().NoError(err)

	got := s.outcomeOf(out)
	s.Require().Equal(straightRoll, got.Result.Roll)
	s.Require().Nil(keepOf(s.T(), got.Result.Calculation))

	s.Require().NotEmpty(hooksFor(out.Hooks, *refs.Conditions.Raging()),
		"it was attached — it simply decided the save was not its business")
}

// R3. A participant nobody expected to matter is passed in, attaches, and folds
// nothing. Pass-everyone-in costs correctness nothing.
func (s *ResolveTestSuite) TestAnIrrelevantParticipantAttachesAndFoldsNothing() {
	alone, err := Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Character: s.barbarian(s.raging())}},
		Machine:      s.save(abilities.STR),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})
	s.Require().NoError(err)

	s.SetupTest()
	together, err := Resolve(s.ctx, &Input{
		World: s.world(),
		Participants: []Participant{
			{Character: s.barbarian(s.raging())},
			{Monster: s.skeleton()},
		},
		Machine: s.save(abilities.STR),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})
	s.Require().NoError(err)

	s.Require().Equal(s.outcomeOf(alone).Result, s.outcomeOf(together).Result,
		"the skeleton changed nothing about the barbarian's save")

	immunity := hooksFor(together.Hooks, *refs.MonsterTraits.Immunity())
	s.Require().NotEmpty(immunity, "and it really was attached, not skipped")
	s.Require().Equal(skeletonID, immunity[0].Participant)
}

// R4/C8. The registration list is a function of the data, not of the order the
// caller happened to list participants in. Without this, a resumed suspension
// could attach into a differently-ordered world.
func (s *ResolveTestSuite) TestRegistrationsDoNotDependOnInputOrder() {
	forward, err := Resolve(s.ctx, &Input{
		World: s.world(),
		Participants: []Participant{
			{Character: s.barbarian(s.raging())},
			{Monster: s.skeleton()},
		},
		Machine: s.save(abilities.STR),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})
	s.Require().NoError(err)

	s.SetupTest()
	reversed, err := Resolve(s.ctx, &Input{
		World: s.world(),
		Participants: []Participant{
			{Monster: s.skeleton()},
			{Character: s.barbarian(s.raging())},
		},
		Machine: s.save(abilities.STR),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})
	s.Require().NoError(err)

	s.Require().Equal(forward.Hooks, reversed.Hooks)
	s.Require().NotEmpty(forward.Hooks)
	s.Require().Equal(heroID, forward.Hooks[0].Participant,
		"sorted by ID, so the hero attaches before the skeleton")
}

// R5, end to end. After Resolve returns, the bus it was given is inert: the
// same chain that Raging answered during the interaction now reaches nobody.
func (s *ResolveTestSuite) TestNothingSurvivesTheCall() {
	inner := events.NewEventBus()

	out, err := resolveOn(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Character: s.barbarian(s.raging())}},
		Machine:      s.save(abilities.STR),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	}, newSurface(inner))
	s.Require().NoError(err)
	s.Require().NotEmpty(out.Hooks, "there was something to tear down")

	event := &dnd5eEvents.SavingThrowChainEvent{SaverID: heroID, Ability: abilities.STR, DC: saveDifficulty}
	chain := events.NewStagedChain[*dnd5eEvents.SavingThrowChainEvent](combat.ModifierStages)

	modified, err := dnd5eEvents.SavingThrowChain.On(inner).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)

	s.Require().False(folded.HasAdvantage(),
		"Raging answered this exact chain during the interaction and does not answer it now")
}

// The world round-trips even though a saving throw reads nothing from it.
// Without this, Resolve is MakeSavingThrow with extra steps.
func (s *ResolveTestSuite) TestTheWorldRoundTripsUnchanged() {
	world := s.world()

	// Snapshot before the call, so this also says Resolve did not edit the
	// caller's world on its way through.
	before, err := json.Marshal(world)
	s.Require().NoError(err)

	out, err := Resolve(s.ctx, &Input{
		World:        world,
		Participants: []Participant{{Character: s.barbarian(s.raging())}},
		Machine:      s.save(abilities.STR),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})
	s.Require().NoError(err)

	after, err := json.Marshal(out.World)
	s.Require().NoError(err)

	s.Require().JSONEq(string(before), string(after))

	// Equal is not enough on its own: handing the input straight back would
	// satisfy it while never loading the world at all, which is precisely the
	// "extra steps" failure above. The output has to be the encounter's own
	// serialization — data the host owns outright — so writing through the
	// input the caller still holds must not reach it.
	s.Require().NotEmpty(world.Members)
	world.Members[0].ID = "scribbled"

	for _, m := range out.World.Members {
		s.Require().NotEqual(encounter.MemberID("scribbled"), m.ID,
			"the returned world shares memory with the input: it was never round-tripped")
	}
}

// The direct-definition round trip is pinned: a monster's authored action
// survives pure load, attach, and ToData without a behavior loader.
func (s *ResolveTestSuite) TestAMonstersActionsSurviveResolution() {
	machine := &captureMachine{}

	out, err := Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Monster: s.skeleton()}},
		Machine:      machine,
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(out)

	loaded, ok := machine.cast.Monster(skeletonID)
	s.Require().True(ok, "the skeleton was attached")

	// Reconstituted as behaviour, not merely carried along as bytes.
	s.Require().Len(loaded.Actions(), 1, "the shortsword definition survived")
	s.Require().Equal(refs.Weapons.Shortsword(), &loaded.Actions()[0].Ref)

	// And it round-trips back out to exactly the data that went in.
	s.Require().Equal([]combatActions.Definition{s.shortsword()}, loaded.ToData().Actions)
}

// A save changes nobody, so nobody comes back dirty. The point is that dirty
// means dirty rather than "was present".
func (s *ResolveTestSuite) TestASaveLeavesNobodyDirty() {
	out, err := Resolve(s.ctx, &Input{
		World: s.world(),
		Participants: []Participant{
			{Character: s.barbarian(s.raging())},
			{Monster: s.skeleton()},
		},
		Machine: s.save(abilities.STR),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})
	s.Require().NoError(err)

	s.Require().Empty(out.DirtyCharacters)
	s.Require().Empty(out.DirtyMonsters)
}

func (s *ResolveTestSuite) TestNilInputRejected() {
	s.Require().NotPanics(func() {
		out, err := Resolve(s.ctx, nil)
		s.Require().ErrorIs(err, ErrNilInput)
		s.Require().Nil(out)
	})
}

func (s *ResolveTestSuite) TestMissingMachineRejected() {
	_, err := Resolve(s.ctx, &Input{World: s.world(), Capabilities: encounter.Capabilities{Initiative: orderAsGiven{}, Driver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{}, Roller: dice.NewRoller(), Actors: Actors}})
	s.Require().ErrorIs(err, ErrNoMachine)
}

func (s *ResolveTestSuite) TestBadParticipantsRejected() {
	s.Run("empty", func() {
		_, err := Resolve(s.ctx, &Input{
			World:        s.world(),
			Participants: []Participant{{}},
			Machine:      s.save(abilities.STR),
			Capabilities: encounter.Capabilities{
				Initiative: orderAsGiven{},
				Driver:     passDriver{},
				Standing:   everyoneStanding{},
				Sight:      everyoneSeesTheWholeMap{},
				Equipment:  noHandsAreObserved{},
				Sheets:     noSheetsAsked{},
				Roller:     dice.NewRoller(),
				Actors:     Actors,
			},
		})
		s.Require().ErrorIs(err, ErrBadParticipant)
	})

	s.Run("both a character and a monster", func() {
		_, err := Resolve(s.ctx, &Input{
			World:        s.world(),
			Participants: []Participant{{Character: s.barbarian(), Monster: s.skeleton()}},
			Machine:      s.save(abilities.STR),
			Capabilities: encounter.Capabilities{
				Initiative: orderAsGiven{},
				Driver:     passDriver{},
				Standing:   everyoneStanding{},
				Sight:      everyoneSeesTheWholeMap{},
				Equipment:  noHandsAreObserved{},
				Sheets:     noSheetsAsked{},
				Roller:     dice.NewRoller(),
				Actors:     Actors,
			},
		})
		s.Require().ErrorIs(err, ErrBadParticipant)
	})

	s.Run("the same id twice", func() {
		_, err := Resolve(s.ctx, &Input{
			World:        s.world(),
			Participants: []Participant{{Character: s.barbarian()}, {Character: s.barbarian(s.raging())}},
			Machine:      s.save(abilities.STR),
			Capabilities: encounter.Capabilities{
				Initiative: orderAsGiven{},
				Driver:     passDriver{},
				Standing:   everyoneStanding{},
				Sight:      everyoneSeesTheWholeMap{},
				Equipment:  noHandsAreObserved{},
				Sheets:     noSheetsAsked{},
				Roller:     dice.NewRoller(),
				Actors:     Actors,
			},
		})
		s.Require().ErrorIs(err, ErrBadParticipant)
	})
}

// Rolling a save for someone who was not passed in would silently drop their
// modifier and every effect they carry, and still return a plausible number.
func (s *ResolveTestSuite) TestASaverWhoIsNotAParticipantIsRefused() {
	_, err := Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Monster: s.skeleton()}},
		Machine: NewSave(&SaveInput{
			SaverID: "nobody", Ability: abilities.STR, DC: saveDifficulty,
			D20Source: dnd5eEvents.RollSource{Ref: refs.Spells.ViciousMockery(), Name: "Test Save"},
			Roller:    s.roller,
		}),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})
	s.Require().ErrorIs(err, ErrNoSaver)
}

func (s *ResolveTestSuite) TestAMonsterCanSucceedOnASavingThrow() {
	s.roller.single = 11

	out, err := Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Monster: s.wolf()}},
		Machine: NewSave(&SaveInput{
			SaverID: wolfID,
			Ability: abilities.STR,
			DC:      saveDifficulty,
			D20Source: dnd5eEvents.RollSource{
				Ref: refs.Spells.ViciousMockery(), Name: "Test Save",
			},
			Roller: s.roller,
		}),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})
	s.Require().NoError(err)

	got := s.outcomeOf(out)
	s.Require().Equal(11, got.Result.Roll)
	s.Require().Equal(12, got.Result.Total, "the wolf adds its +1 STR modifier")
	s.Require().True(got.Result.Success)
}

func (s *ResolveTestSuite) TestAMonsterCanFailASavingThrowWithANegativeModifier() {
	s.roller.single = 10

	out, err := Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Monster: s.wolf()}},
		Machine: NewSave(&SaveInput{
			SaverID: wolfID,
			Ability: abilities.INT,
			DC:      7,
			D20Source: dnd5eEvents.RollSource{
				Ref: refs.Spells.ViciousMockery(), Name: "Test Save",
			},
			Roller: s.roller,
		}),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})
	s.Require().NoError(err)

	got := s.outcomeOf(out)
	s.Require().Equal(10, got.Result.Roll)
	s.Require().Equal(6, got.Result.Total, "the wolf adds its -4 INT modifier")
	s.Require().False(got.Result.Success)
}

func TestResolveSuite(t *testing.T) {
	suite.Run(t, new(ResolveTestSuite))
}

// TestCapabilitiesAreSuppliedNeverDefaulted pins both halves of one principle.
//
// Initiative was absent from this input entirely — the composition grew a
// required roller (rpg-toolkit#964) and this package kept calling LoadEncounter
// without one, so against encounter v0.9.0 every Resolve failed at the door.
// That was found by the session seam adopting this module for the first time
// (rpg-toolkit#966), which is the only place it could have been found: nothing
// here loads a world that requires one.
//
// Roller was worse, because it looked fine. A nil roller quietly became real
// randomness, so a caller who forgot to wire dice got a result that passed
// every assertion and could not be reproduced. Kirk's ruling on the
// composition's own roller covers both: a missing capability is an error
// returned way upstream, not a default.
//
// Exactly ONE literal in this suite was relying on that default when it was
// removed — the surface it was protecting was almost entirely imaginary.
//
// Standing joins them for the first reason rather than the second. The
// composition grew a required standing capability (rpg-toolkit#1077) and this
// package kept calling LoadEncounter without one, so against encounter v0.15.0
// every Resolve failed at the door — found, again, by the session seam adopting
// the new pin (rpg-toolkit#1079), and again because nothing here loads a world
// that requires one. The default this refuses is the tempting one: answering
// "nobody is down" on the caller's behalf, from a package holding no hit points.
func TestCapabilitiesAreSuppliedNeverDefaulted(t *testing.T) {
	machine := NewSave(&SaveInput{
		SaverID: "x", Ability: abilities.CON, DC: 10,
		D20Source: dnd5eEvents.RollSource{Ref: refs.Spells.ViciousMockery(), Name: "Test Save"},
		Roller:    dice.NewRoller(),
	})

	t.Run("no initiative", func(t *testing.T) {
		err := (&Input{Machine: machine, Capabilities: encounter.Capabilities{Roller: dice.NewRoller(), Actors: Actors}}).Validate()
		require.ErrorIs(t, err, encounter.ErrNoInitiative)
	})

	t.Run("no standing", func(t *testing.T) {
		err := (&Input{Machine: machine, Capabilities: encounter.Capabilities{Initiative: orderAsGiven{}, Driver: passDriver{}, Roller: dice.NewRoller(), Actors: Actors}}).Validate()
		require.ErrorIs(t, err, encounter.ErrNoStanding)
	})

	t.Run("no sight", func(t *testing.T) {
		err := (&Input{
			Machine: machine,
			Capabilities: encounter.Capabilities{
				Initiative: orderAsGiven{},
				Driver:     passDriver{},
				Standing:   everyoneStanding{},
				Roller:     dice.NewRoller(),
				Actors:     Actors,
			},
		}).Validate()
		require.ErrorIs(t, err, encounter.ErrNoSight)
	})

	// An absent equipment answer cannot be replaced with "everybody is
	// empty-handed": the sight seam turns this into per-observer testimony, and
	// that would be testimony nobody gave (rpg-toolkit#1615).
	t.Run("no equipment", func(t *testing.T) {
		err := (&Input{
			Machine: machine,
			Capabilities: encounter.Capabilities{
				Initiative: orderAsGiven{},
				Driver:     passDriver{},
				Standing:   everyoneStanding{},
				Sight:      everyoneSeesTheWholeMap{},
				Roller:     dice.NewRoller(),
				Actors:     Actors,
			},
		}).Validate()
		require.ErrorIs(t, err, encounter.ErrNoEquipment)
	})

	// The composition no longer stores a speed or a reach (rpg-project#538), so
	// an absent Sheets answer could only be replaced with one this package
	// made up — and zero is a real speed, not a missing one.
	t.Run("no sheets", func(t *testing.T) {
		err := (&Input{
			Machine: machine,
			Capabilities: encounter.Capabilities{
				Initiative: orderAsGiven{},
				Driver:     passDriver{},
				Standing:   everyoneStanding{},
				Sight:      everyoneSeesTheWholeMap{},
				Equipment:  noHandsAreObserved{},
				Roller:     dice.NewRoller(),
				Actors:     Actors,
			},
		}).Validate()
		require.ErrorIs(t, err, encounter.ErrNoSheets)
	})

	t.Run("no turn driver", func(t *testing.T) {
		err := (&Input{
			Machine: machine,
			Capabilities: encounter.Capabilities{
				Initiative: orderAsGiven{},
				Standing:   everyoneStanding{},
				Sight:      everyoneSeesTheWholeMap{},
				Roller:     dice.NewRoller(),
				Equipment:  noHandsAreObserved{},
				Sheets:     noSheetsAsked{},
				Actors:     Actors,
			},
		}).Validate()
		require.ErrorIs(t, err, encounter.ErrNoTurnDriver)
	})

	// Actors are capabilities like any other, and the encounter's own sentinels
	// name them, in the order its Validate checks them.
	t.Run("no striker", func(t *testing.T) {
		in := fullInput(machine)
		in.Striker = nil
		require.ErrorIs(t, in.Validate(), encounter.ErrNoStriker)
	})

	t.Run("no mover", func(t *testing.T) {
		in := fullInput(machine)
		in.Mover = nil
		require.ErrorIs(t, in.Validate(), encounter.ErrNoMover)
	})

	t.Run("no announcer", func(t *testing.T) {
		in := fullInput(machine)
		in.Announcer = nil
		require.ErrorIs(t, in.Validate(), encounter.ErrNoAnnouncer)
	})

	// The capabilities are checked before the roller, so a bare input is
	// refused by the encounter's own first sentinel and not by ours.
	t.Run("capabilities before roller", func(t *testing.T) {
		err := (&Input{Machine: machine}).Validate()
		require.ErrorIs(t, err, encounter.ErrNoInitiative)
		require.NotErrorIs(t, err, ErrNoRoller)
	})

	// The encounter's sentinel is wrapped with context and still reachable.
	t.Run("the sentinel is wrapped and reachable", func(t *testing.T) {
		in := fullInput(machine)
		in.Sight = nil
		err := in.Validate()
		require.ErrorIs(t, err, encounter.ErrNoSight)
		require.NotEqual(t, encounter.ErrNoSight, err)
	})

	t.Run("no roller", func(t *testing.T) {
		err := (&Input{
			Machine: machine,
			Capabilities: encounter.Capabilities{
				Initiative: orderAsGiven{},
				Driver:     passDriver{},
				Standing:   everyoneStanding{},
				Sight:      everyoneSeesTheWholeMap{},
				Equipment:  noHandsAreObserved{},
				Sheets:     noSheetsAsked{},
				Actors:     Actors,
			},
		}).Validate()
		require.ErrorIs(t, err, ErrNoRoller)
	})

	t.Run("all supplied", func(t *testing.T) {
		err := (&Input{
			Machine: machine,
			Capabilities: encounter.Capabilities{
				Initiative: orderAsGiven{},
				Driver:     passDriver{},
				Standing:   everyoneStanding{},
				Sight:      everyoneSeesTheWholeMap{},
				Equipment:  noHandsAreObserved{},
				Sheets:     noSheetsAsked{},
				Roller:     dice.NewRoller(),
				Actors:     Actors,
			},
		}).Validate()
		require.NoError(t, err)
	})
}

// countingStanding answers like everyoneStanding and remembers being asked
// through its Standing method.
type countingStanding struct{ asks int }

func (c *countingStanding) Standing(_ []encounter.MemberID) ([]encounter.MemberID, error) {
	c.asks++

	return nil, nil
}

// Assess carries the participation answer the load door asks for since #1453.
// It is deliberately NOT counted: the encounter asks participation to load;
// this package's contract is that IT never consults the Standing capability,
// and the uncounted Assess leaves the existing pin free to assert that.
func (c *countingStanding) Assess(
	members []encounter.MemberID,
) (*encounter.ParticipationAssessment, error) {
	assessment := &encounter.ParticipationAssessment{}
	for _, id := range members {
		assessment.Members = append(assessment.Members, encounter.MemberParticipation{
			Member: id, Contact: true, Conscious: true, Turn: encounter.TurnParticipationWait,
		})
	}
	return assessment, nil
}

var _ encounter.StandingWithParticipation = (*countingStanding)(nil)

// TestTheStandingCapabilityIsCarriedAndNeverAsked pins both halves of what this
// field is for, and they pull in opposite directions.
//
// CARRIED: the world underneath refuses to load without one, so a Resolve that
// succeeds at all is proof the capability reached LoadEncounter. Drop the
// pass-through and this fails at the door rather than in an assertion.
//
// NEVER ASKED: nothing here refreshes sight, so the count must stay zero. That
// is the claim the field's doc makes, and it is the one that would rot silently
// — a later change that started consulting it would be this package answering a
// question about hit points, which is the thing it must not do. Asserting the
// zero is how that stays a decision rather than a coincidence.
func TestTheStandingCapabilityIsCarriedAndNeverAsked(t *testing.T) {
	world, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Actors: encounter.Actors{
				Striker:   noAttacksExpected{},
				Mover:     encounter.RefusingMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	counter := &countingStanding{}
	out, err := Resolve(context.Background(), &Input{
		World: world.ToData(),
		Participants: []Participant{{Character: &character.Data{
			ID: heroID, PlayerID: "player-1", Name: "Grog", Level: 1,
			ClassID: classes.Barbarian, RaceID: races.Human,
			AbilityScores: shared.AbilityScores{
				abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
				abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
			},
			HitPoints: 14, MaxHitPoints: 14, ProficiencyBonus: 2,
		}}},
		Machine: NewSave(&SaveInput{
			SaverID: heroID, Ability: abilities.CON, DC: 10,
			D20Source: dnd5eEvents.RollSource{Ref: refs.Spells.ViciousMockery(), Name: "Test Save"},
			Roller:    dice.NewRoller(),
		}),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   counter,
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})

	require.NoError(t, err, "the world loads, which it cannot do without the capability")
	require.NotNil(t, out)
	require.Zero(t, counter.asks, "and this package never asks who is standing")
}

// countingSight answers like everyoneSeesTheWholeMap and remembers being asked.
type countingSight struct{ asks int }

func (c *countingSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	c.asks++

	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = unlimitedSight
	}

	return out, nil
}

// TestTheSightCapabilityIsCarriedAndNeverAsked is TestTheStandingCapability's
// twin, for the capability that arrived with encounter v0.20.0, and it pins the
// same two halves.
//
// CARRIED: the composition refuses to load without one (rpg-toolkit#1111), so a
// Resolve that succeeds at all is proof this reached LoadEncounter.
//
// NEVER ASKED: the composition asks how far somebody can see at exactly one
// place — where it rebuilds percepts — and the only two things that reach it are
// its own Setup and its refreshSight. This package calls neither: it loads a
// world and reads it back out. So the count stays zero, and the zero is the
// assertion rather than an accident, because a later change that started
// consulting it would be this package answering a question about light and
// darkvision it holds nothing of.
func TestTheSightCapabilityIsCarriedAndNeverAsked(t *testing.T) {
	world, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Actors: encounter.Actors{
				Striker:   noAttacksExpected{},
				Mover:     encounter.RefusingMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)

	counter := &countingSight{}
	out, err := Resolve(context.Background(), &Input{
		World: world.ToData(),
		Participants: []Participant{{Character: &character.Data{
			ID: heroID, PlayerID: "player-1", Name: "Grog", Level: 1,
			ClassID: classes.Barbarian, RaceID: races.Human,
			AbilityScores: shared.AbilityScores{
				abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
				abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
			},
			HitPoints: 14, MaxHitPoints: 14, ProficiencyBonus: 2,
		}}},
		Machine: NewSave(&SaveInput{
			SaverID: heroID, Ability: abilities.CON, DC: 10,
			D20Source: dnd5eEvents.RollSource{Ref: refs.Spells.ViciousMockery(), Name: "Test Save"},
			Roller:    dice.NewRoller(),
		}),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      counter,
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	})

	require.NoError(t, err, "the world loads, which it cannot do without the capability")
	require.NotNil(t, out)
	require.Zero(t, counter.asks, "and this package never asks how far anybody can see")
}

// fullInput is an input every presence check passes.
func fullInput(machine Machine) *Input {
	return &Input{
		Machine: machine,
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Roller:     dice.NewRoller(),
			Actors:     Actors,
		},
	}
}

// TestResolveRefusesAWorldWithoutActors pins both halves of resolution.Actors:
// with them the world loads, and without them Resolve refuses by the
// encounter's own sentinel before any participant attaches.
func TestResolveRefusesAWorldWithoutActors(t *testing.T) {
	machine := NewSave(&SaveInput{
		SaverID: "x", Ability: abilities.CON, DC: 10,
		D20Source: dnd5eEvents.RollSource{Ref: refs.Spells.ViciousMockery(), Name: "Test Save"},
		Roller:    dice.NewRoller(),
	})

	t.Run("with Actors the world loads", func(t *testing.T) {
		in := fullInput(machine)
		in.World = actionWorld(t, 2)
		in.Participants = []Participant{{Monster: monsters.NewWolf("wolf").ToData()}}
		_, err := Resolve(context.Background(), in)
		// ErrNoSaver is the error the machine raises once the world has
		// loaded and the participants attached, so it proves the load worked.
		require.ErrorIs(t, err, ErrNoSaver)
	})

	t.Run("without Actors it refuses before any participant attaches", func(t *testing.T) {
		in := fullInput(machine)
		in.Actors = encounter.Actors{}
		in.World = actionWorld(t, 2)
		// A participant that would fail to attach proves the order: the actor
		// refusal comes first.
		in.Participants = []Participant{{}}
		_, err := Resolve(context.Background(), in)
		require.ErrorIs(t, err, encounter.ErrNoStriker)
		require.NotErrorIs(t, err, ErrBadParticipant)
	})
}

// TestResolveLoadsWithTheCapabilitiesItWasHanded pins the law that resolution
// swaps no member of the capabilities it was handed: the one
// encounter.LoadEncounterInput literal in the module's non-test files has
// exactly the keys Data and Capabilities, and Capabilities is in.Capabilities.
func TestResolveLoadsWithTheCapabilitiesItWasHanded(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	require.NoError(t, err)

	var lits []*ast.CompositeLit
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				cl, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				sel, ok := cl.Type.(*ast.SelectorExpr)
				if ok && sel.Sel.Name == "LoadEncounterInput" {
					lits = append(lits, cl)
				}
				return true
			})
		}
	}
	require.Len(t, lits, 1, "exactly one LoadEncounterInput literal in non-test files")

	keys := map[string]string{}
	for _, e := range lits[0].Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		require.True(t, ok, "keyed elements only")
		var sb strings.Builder
		require.NoError(t, format.Node(&sb, fset, kv.Value))
		keys[kv.Key.(*ast.Ident).Name] = sb.String()
	}
	require.Len(t, keys, 2)
	require.Equal(t, "in.World", keys["Data"])
	require.Equal(t, "in.Capabilities", keys["Capabilities"])

	// Refusing stand-ins live in actors.go and nowhere else.
	for _, name := range []string{"RefusingStriker", "RefusingMover", "RefusingAnnouncer"} {
		for _, pkg := range pkgs {
			for fname, f := range pkg.Files {
				ast.Inspect(f, func(n ast.Node) bool {
					if id, ok := n.(*ast.Ident); ok && id.Name == name && !strings.HasSuffix(fname, "actors.go") {
						t.Errorf("%s names %s outside actors.go", fname, name)
					}
					return true
				})
			}
		}
	}
}
