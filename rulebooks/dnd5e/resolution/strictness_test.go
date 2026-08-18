// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// errRefused is what the refusing bus answers with.
var errRefused = errors.New("bus refused the subscription")

// corruptID sorts after heroID, so a resolution carrying both attaches the hero
// first and fails on this one — which is what makes "nothing survived the
// failure" a claim about teardown rather than about never having attached.
const corruptID = "villain"

// unroutable is a condition blob shaped like every other one and naming a ref
// no loader in this build recognises. It is the realistic corruption: not
// garbage bytes, but a persisted effect from a build that knew something this
// one does not.
var unroutable = json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"nope"}}`)

// StrictnessTestSuite pins the behaviour change that rides in with the pure-load
// API (rpg-toolkit#985): resolution refuses a sheet it cannot fully reconstitute
// instead of quietly resolving without part of it.
//
// This is not fussiness. Resolve hands back sheets to be persisted, so an effect
// dropped on the way in is an effect deleted on the way out, and the deletion
// looks exactly like the effect never having been there — rpg-toolkit#948, whose
// whole difficulty was that nothing anywhere said it had happened.
type StrictnessTestSuite struct {
	suite.Suite

	ctx    context.Context
	roller *scriptedRoller
}

func (s *StrictnessTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.roller = &scriptedRoller{single: straightRoll, pair: []int{straightRoll, advantageRoll}}
}

// world is a two-member encounter: the hero, and a second character whose sheet
// is the corrupt one.
func (s *StrictnessTestSuite) world() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 1, Y: 1}},
			{ID: corruptID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 5, Y: 5}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

func (s *StrictnessTestSuite) sheet(id string, conds ...json.RawMessage) *character.Data {
	return &character.Data{
		ID:       id,
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
		},
		Conditions: conds,
	}
}

func (s *StrictnessTestSuite) raging(id string) json.RawMessage {
	raw, err := (&conditions.RagingCondition{
		CharacterID: id,
		DamageBonus: 2,
		Level:       1,
		Source:      "rage",
	}).ToJSON()
	s.Require().NoError(err)

	return raw
}

func (s *StrictnessTestSuite) save() Machine {
	return NewSave(&SaveInput{
		SaverID: heroID,
		Ability: abilities.STR,
		DC:      saveDifficulty,
		Roller:  s.roller,
	})
}

// A condition blob this build cannot route fails the whole resolution, and the
// error says which participant and which blob. The legacy path resolved it
// happily, one effect short, and returned a sheet to persist in that state.
func (s *StrictnessTestSuite) TestAnUnroutableConditionFailsTheResolution() {
	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: s.sheet(heroID, unroutable)}},
		Machine:      s.save(),
	})

	s.Require().Error(err)
	s.Require().Nil(out, "a resolution that failed produces no output to act on")
	s.Require().Contains(err.Error(), heroID, "the error names the participant")
	s.Require().Contains(err.Error(), `"id":"nope"`, "and the blob that could not be read")
}

// The control that makes the refusal mean something: the same sheet, the same
// machine, a condition this build does know — and the resolution runs.
func (s *StrictnessTestSuite) TestTheSameSheetWithAReadableConditionResolves() {
	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: s.sheet(heroID, s.raging(heroID))}},
		Machine:      s.save(),
	})

	s.Require().NoError(err)
	s.Require().NotNil(out)
}

// Nothing is half-attached. The hero attaches first and its Raging subscribes to
// the saving-throw chain; the corrupt sheet then fails the resolution — and the
// chain the hero's condition had joined answers nobody afterwards.
//
// Two independent guarantees have to hold for this: the entity-side contract
// that a failed attach leaves nothing behind, and resolution's own teardown on
// the error path. Asserting on the bus rather than on either mechanism is what
// makes the test survive a change to which one does the work.
func (s *StrictnessTestSuite) TestAFailedResolutionLeavesNothingOnTheBus() {
	inner := events.NewEventBus()

	out, err := resolveOn(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World: s.world(),
		Participants: []Participant{
			{Character: s.sheet(heroID, s.raging(heroID))},
			{Character: s.sheet(corruptID, unroutable)},
		},
		Machine: s.save(),
	}, newSurface(inner))

	s.Require().Error(err)
	s.Require().Nil(out)

	event := &dnd5eEvents.SavingThrowChainEvent{SaverID: heroID, Ability: abilities.STR, DC: saveDifficulty}
	chain := events.NewStagedChain[*dnd5eEvents.SavingThrowChainEvent](combat.ModifierStages)

	modified, err := dnd5eEvents.SavingThrowChain.On(inner).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)

	s.Require().False(folded.HasAdvantage(),
		"the hero's Raging attached before the failure and does not answer this chain")
}

// A monster's traits get the same treatment: an unroutable trait ref fails the
// resolution rather than producing a monster missing an immunity nobody removed.
func (s *StrictnessTestSuite) TestAnUnroutableMonsterTraitFailsTheResolution() {
	skeleton := &monster.Data{
		ID:            skeletonID,
		Name:          "Skeleton",
		HitPoints:     13,
		MaxHitPoints:  13,
		ArmorClass:    13,
		AbilityScores: shared.AbilityScores{},
		Conditions: []json.RawMessage{
			json.RawMessage(`{"ref":{"module":"dnd5e","type":"monster_traits","id":"nope"}}`),
		},
	}

	world, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, Standing: everyoneStanding{},
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

	out, resolveErr := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World: world.ToData(),
		Participants: []Participant{
			{Character: s.sheet(heroID)},
			{Monster: skeleton},
		},
		Machine: s.save(),
	})

	s.Require().Error(resolveErr)
	s.Require().Nil(out)
	s.Require().Contains(resolveErr.Error(), skeletonID)
	s.Require().Contains(resolveErr.Error(), `"id":"nope"`)
}

// refusingBus is a real bus that refuses the nth Subscribe. A sheet whose data
// is perfectly readable can still fail to attach — a bus that will not take a
// subscription is the general case — and that failure has to reach the caller
// too, not only the failures that happen while parsing.
type refusingBus struct {
	inner  events.EventBus
	failOn int // 1-based
	calls  int
}

func (b *refusingBus) Subscribe(ctx context.Context, topic events.Topic, handler any) (string, error) {
	b.calls++
	if b.calls == b.failOn {
		return "", errRefused
	}

	return b.inner.Subscribe(ctx, topic, handler)
}

func (b *refusingBus) Unsubscribe(ctx context.Context, id string) error {
	return b.inner.Unsubscribe(ctx, id)
}

func (b *refusingBus) Publish(ctx context.Context, topic events.Topic, event any) error {
	return b.inner.Publish(ctx, topic, event)
}

// An attach that fails for a reason other than an unreadable blob fails the
// resolution just the same. Without this, resolution could swallow
// character.Attach's error and hand back a sheet that quietly attached nothing —
// the same silent loss as a dropped blob, arriving through a different door.
func (s *StrictnessTestSuite) TestAnAttachThatFailsFailsTheResolution() {
	bus := &refusingBus{inner: events.NewEventBus(), failOn: 1}

	out, err := resolveOn(s.ctx, &Input{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: s.sheet(heroID, s.raging(heroID))}},
		Machine:      s.save(),
	}, newSurface(bus))

	s.Require().Error(err)
	s.Require().Nil(out)
	s.Require().ErrorIs(err, errRefused)
	s.Require().Contains(err.Error(), heroID, "the error names the participant that would not attach")
}

func TestStrictnessSuite(t *testing.T) {
	suite.Run(t, new(StrictnessTestSuite))
}
