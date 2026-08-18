// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
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
	monsterActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
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
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 1, Y: 1}},
			{ID: skeletonID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 5, Y: 5}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
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
		DamageBonus: 2,
		Level:       1,
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

// shortsword is the skeleton's melee attack in the form a host stores it. Built
// through the action's own ToData so the fixture is the real serialized shape
// rather than a hand-copied guess at it.
func (s *ResolveTestSuite) shortsword() monster.ActionData {
	return monsterActions.NewMeleeAction(monsterActions.MeleeConfig{
		Name:        "shortsword",
		AttackBonus: 4,
		DamageDice:  "1d6+2",
		Reach:       5,
		DamageType:  damage.Piercing,
	}).ToData()
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
		Actions:       []monster.ActionData{s.shortsword()},
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
		Roller:  s.roller,
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
	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: s.barbarian(s.raging())}},
		Machine:      s.save(abilities.STR),
	})
	s.Require().NoError(err)

	got := s.outcomeOf(out)
	s.Require().Equal(advantageRoll, got.Result.Roll,
		"only a rolled-twice-take-higher can produce this die")
	s.Require().Equal(advantageRoll+heroSaveBonus, got.Result.Total)
	s.Require().True(got.Result.Success)

	s.Require().True(got.Folded.HasAdvantage())
	s.Require().Len(got.Folded.AdvantageSources, 1)
	s.Require().Equal(refs.Conditions.Raging(), got.Folded.AdvantageSources[0].SourceRef,
		"and it is Raging that says so, not the wiring")
}

// The control that makes the headline mean something: the same barbarian, the
// same save, no condition — a straight roll.
func (s *ResolveTestSuite) TestTheSameBarbarianWithoutRageRollsStraight() {
	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: s.barbarian()}},
		Machine:      s.save(abilities.STR),
	})
	s.Require().NoError(err)

	got := s.outcomeOf(out)
	s.Require().Equal(straightRoll, got.Result.Roll)
	s.Require().False(got.Folded.HasAdvantage())
	s.Require().Empty(got.Folded.AdvantageSources)
}

// The second effect, on a different chain, through the same machinery.
func (s *ResolveTestSuite) TestDodgingGrantsAdvantageOnADexteritySave() {
	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: s.barbarian(s.dodging())}},
		Machine:      s.save(abilities.DEX),
	})
	s.Require().NoError(err)

	got := s.outcomeOf(out)
	s.Require().Equal(advantageRoll, got.Result.Roll)
	s.Require().Equal(refs.Conditions.Dodging(), got.Folded.AdvantageSources[0].SourceRef)
}

// Applicability is the effect's own predicate, never resolution's. Raging is
// attached for a DEX save exactly as it is for a STR save, and declines.
func (s *ResolveTestSuite) TestRagingDeclinesADexteritySaveOnItsOwn() {
	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: s.barbarian(s.raging())}},
		Machine:      s.save(abilities.DEX),
	})
	s.Require().NoError(err)

	got := s.outcomeOf(out)
	s.Require().Equal(straightRoll, got.Result.Roll)
	s.Require().False(got.Folded.HasAdvantage())

	s.Require().NotEmpty(hooksFor(out.Hooks, *refs.Conditions.Raging()),
		"it was attached — it simply decided the save was not its business")
}

// R3. A participant nobody expected to matter is passed in, attaches, and folds
// nothing. Pass-everyone-in costs correctness nothing.
func (s *ResolveTestSuite) TestAnIrrelevantParticipantAttachesAndFoldsNothing() {
	alone, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: s.barbarian(s.raging())}},
		Machine:      s.save(abilities.STR),
	})
	s.Require().NoError(err)

	s.SetupTest()
	together, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World: s.world(),
		Participants: []Participant{
			{Character: s.barbarian(s.raging())},
			{Monster: s.skeleton()},
		},
		Machine: s.save(abilities.STR),
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
	forward, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World: s.world(),
		Participants: []Participant{
			{Character: s.barbarian(s.raging())},
			{Monster: s.skeleton()},
		},
		Machine: s.save(abilities.STR),
	})
	s.Require().NoError(err)

	s.SetupTest()
	reversed, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World: s.world(),
		Participants: []Participant{
			{Monster: s.skeleton()},
			{Character: s.barbarian(s.raging())},
		},
		Machine: s.save(abilities.STR),
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

	out, err := resolveOn(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: s.barbarian(s.raging())}},
		Machine:      s.save(abilities.STR),
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

	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World:        world,
		Participants: []Participant{{Character: s.barbarian(s.raging())}},
		Machine:      s.save(abilities.STR),
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

// The three-call assembly, pinned. monster.LoadFromData loads neither actions
// nor conditions, and monster.ToData serializes both — so a resolution that
// skips LoadMonsterActions does not merely leave the skeleton unable to swing,
// it writes back a skeleton that has silently lost its shortsword. Asserting the
// action reconstitutes AND survives ToData is what makes that class of loss a
// test failure instead of a data-loss bug in a host's repository.
func (s *ResolveTestSuite) TestAMonstersActionsSurviveResolution() {
	machine := &captureMachine{}

	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Monster: s.skeleton()}},
		Machine:      machine,
	})
	s.Require().NoError(err)
	s.Require().NotNil(out)

	loaded, ok := machine.cast.Monster(skeletonID)
	s.Require().True(ok, "the skeleton was attached")

	// Reconstituted as behaviour, not merely carried along as bytes.
	s.Require().Len(loaded.Actions(), 1, "the shortsword became a real action")
	s.Require().Equal(monster.TypeMeleeAttack, loaded.Actions()[0].ActionType())

	// And it round-trips back out to exactly the data that went in.
	s.Require().Equal([]monster.ActionData{s.shortsword()}, loaded.ToData().Actions)
}

// A save changes nobody, so nobody comes back dirty. The point is that dirty
// means dirty rather than "was present".
func (s *ResolveTestSuite) TestASaveLeavesNobodyDirty() {
	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World: s.world(),
		Participants: []Participant{
			{Character: s.barbarian(s.raging())},
			{Monster: s.skeleton()},
		},
		Machine: s.save(abilities.STR),
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
	_, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(), World: s.world()})
	s.Require().ErrorIs(err, ErrNoMachine)
}

func (s *ResolveTestSuite) TestBadParticipantsRejected() {
	s.Run("empty", func() {
		_, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
			World:        s.world(),
			Participants: []Participant{{}},
			Machine:      s.save(abilities.STR),
		})
		s.Require().ErrorIs(err, ErrBadParticipant)
	})

	s.Run("both a character and a monster", func() {
		_, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
			World:        s.world(),
			Participants: []Participant{{Character: s.barbarian(), Monster: s.skeleton()}},
			Machine:      s.save(abilities.STR),
		})
		s.Require().ErrorIs(err, ErrBadParticipant)
	})

	s.Run("the same id twice", func() {
		_, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
			World:        s.world(),
			Participants: []Participant{{Character: s.barbarian()}, {Character: s.barbarian(s.raging())}},
			Machine:      s.save(abilities.STR),
		})
		s.Require().ErrorIs(err, ErrBadParticipant)
	})
}

// Rolling a save for someone who was not passed in would silently drop their
// modifier and every effect they carry, and still return a plausible number.
func (s *ResolveTestSuite) TestASaverWhoIsNotAParticipantIsRefused() {
	_, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Monster: s.skeleton()}},
		Machine:      NewSave(&SaveInput{SaverID: "nobody", Ability: abilities.STR, DC: saveDifficulty}),
	})
	s.Require().ErrorIs(err, ErrNoSaver)
}

func (s *ResolveTestSuite) TestAMonsterCanSucceedOnASavingThrow() {
	s.roller.single = 11

	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Monster: s.wolf()}},
		Machine: NewSave(&SaveInput{
			SaverID: wolfID,
			Ability: abilities.STR,
			DC:      saveDifficulty,
			Roller:  s.roller,
		}),
	})
	s.Require().NoError(err)

	got := s.outcomeOf(out)
	s.Require().Equal(11, got.Result.Roll)
	s.Require().Equal(12, got.Result.Total, "the wolf adds its +1 STR modifier")
	s.Require().True(got.Result.Success)
}

func (s *ResolveTestSuite) TestAMonsterCanFailASavingThrowWithANegativeModifier() {
	s.roller.single = 10

	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Monster: s.wolf()}},
		Machine: NewSave(&SaveInput{
			SaverID: wolfID,
			Ability: abilities.INT,
			DC:      7,
			Roller:  s.roller,
		}),
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
		SaverID: "x", Ability: abilities.CON, DC: 10, Roller: dice.NewRoller(),
	})

	t.Run("no initiative", func(t *testing.T) {
		err := (&Input{Machine: machine, Roller: dice.NewRoller()}).Validate()
		require.ErrorIs(t, err, ErrNoInitiative)
	})

	t.Run("no standing", func(t *testing.T) {
		err := (&Input{Machine: machine, Initiative: orderAsGiven{}, Roller: dice.NewRoller()}).Validate()
		require.ErrorIs(t, err, ErrNoStanding)
	})

	t.Run("no roller", func(t *testing.T) {
		err := (&Input{Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{}}).Validate()
		require.ErrorIs(t, err, ErrNoRoller)
	})

	t.Run("all supplied", func(t *testing.T) {
		err := (&Input{
			Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		}).Validate()
		require.NoError(t, err)
	})
}

// countingStanding answers like everyoneStanding and remembers being asked.
type countingStanding struct{ asks int }

func (c *countingStanding) Standing(_ []encounter.MemberID) ([]encounter.MemberID, error) {
	c.asks++

	return nil, nil
}

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
		Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)

	counter := &countingStanding{}
	out, err := Resolve(context.Background(), &Input{
		Initiative: orderAsGiven{}, Standing: counter, Roller: dice.NewRoller(),
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
			SaverID: heroID, Ability: abilities.CON, DC: 10, Roller: dice.NewRoller(),
		}),
	})

	require.NoError(t, err, "the world loads, which it cannot do without the capability")
	require.NotNil(t, out)
	require.Zero(t, counter.asks, "and this package never asks who is standing")
}
