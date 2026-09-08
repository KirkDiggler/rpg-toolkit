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
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ContestDamageTestSuite drives the second thing a failed save can cost: a
// contest that delivers damage beside its condition, which is the whole of what
// Vicious Mockery needs and is not a machine of its own.
//
// The lane is deliberately not the wolf's knockdown. That one is condition-only
// and stays the regression; these scenes declare damage, and one of them
// declares nothing else.
type ContestDamageTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestContestDamageSuite(t *testing.T) {
	suite.Run(t, new(ContestDamageTestSuite))
}

func (s *ContestDamageTestSuite) SetupTest() {
	s.ctx = context.Background()
}

const (
	// spellSaveDC is a level-1 bard's: 8 + proficiency 2 + CHA +3.
	spellSaveDC = 13

	// psychicFace is the face every d4 shows in this suite, so 1d4 psychic is
	// a number the assertions can name rather than a range.
	psychicFace = 3

	// mockeryTurn is the turn the priced scenes are declared in.
	mockeryTurn = 1

	// mockerySpeed is the speed the door hands the refresh. It is not a race's
	// walking speed, for the same reason cost_test's is not.
	mockerySpeed = 25
)

// facedRoller answers a fixed face per die SIZE, so one roller drives both the
// d20 the save rolls and the d4 the damage rolls without either standing in
// for the other.
type facedRoller struct {
	d20   int
	other int
}

func (r facedRoller) face(size int) int {
	if size == 20 {
		return r.d20
	}

	return r.other
}

func (r facedRoller) Roll(_ context.Context, size int) (int, error) { return r.face(size), nil }

func (r facedRoller) RollN(_ context.Context, count, size int) ([]int, error) {
	faces := make([]int, count)
	for i := range faces {
		faces[i] = r.face(size)
	}

	return faces, nil
}

// mockery is the save gate Vicious Mockery declares: one Wisdom save against
// the caster's spell save DC, negating on success, no recurrence.
func mockeryGate() *saves.SaveGate {
	return &saves.SaveGate{
		Abilities:  []abilities.Ability{abilities.WIS},
		DC:         saves.DCStatic(spellSaveDC),
		OnSuccess:  saves.Negated,
		Recurrence: saves.RecurrenceNone,
	}
}

// psychic is the cantrip's declared damage: 1d4, one pool, no bonus.
func psychic() []damage.Damage {
	return []damage.Damage{{Dice: "1d4", Type: damage.Psychic}}
}

// mockedRef stands in for the spell that raised the save. The refs package has
// no spell ref this module may name yet, so the cause carries the condition's
// module ref and the assertions read provenance off it rather than asserting a
// spell id this slice does not own.
func mockedCause() dnd5eEvents.SaveCause {
	return dnd5eEvents.SaveCause{
		Trigger:      dnd5eEvents.SaveTriggerSpell,
		EffectRef:    refs.Conditions.Prone(),
		InstigatorID: bardID,
	}
}

const bardID = "bard"

// saver is the hero this suite mocks: WIS 12 (+1) and not proficient in Wisdom
// saves, so DC 13 is beaten by an 18 and missed by a 3.
func (s *ContestDamageTestSuite) saver(hp int, conds ...json.RawMessage) *character.Data {
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
		HitPoints:        hp,
		MaxHitPoints:     14,
		ArmorClass:       13,
		ProficiencyBonus: 2,
		SavingThrows: map[abilities.Ability]shared.ProficiencyLevel{
			abilities.STR: shared.Proficient,
		},
		Conditions: conds,
	}
}

// bard is the caster who pays for the cantrip. Its whole role in this suite is
// to hold an action economy the door can charge.
func (s *ContestDamageTestSuite) bard(actions int) *character.Data {
	return &character.Data{
		ID:       bardID,
		PlayerID: "player-2",
		Name:     "Scanlan",
		Level:    1,
		ClassID:  classes.Bard,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 14,
			abilities.CON: 12,
			abilities.INT: 10,
			abilities.WIS: 10,
			abilities.CHA: 16,
		},
		HitPoints:        9,
		MaxHitPoints:     9,
		ArmorClass:       12,
		ProficiencyBonus: 2,
		ActionEconomy: &character.ActionEconomyData{
			TurnNumber:            mockeryTurn,
			ActionsRemaining:      actions,
			BonusActionsRemaining: 1,
			ReactionsRemaining:    1,
			MovementRemaining:     mockerySpeed,
		},
	}
}

func (s *ContestDamageTestSuite) world() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{},
		Mover: encounter.RefusingMover{}, Announcer: quietAnnouncer{},
		Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: bardID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 1}},
			{ID: wolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 2, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

func (s *ContestDamageTestSuite) wolfData() *monster.Data {
	return &monster.Data{
		ID:            wolfID,
		Name:          "Wolf",
		Ref:           refs.Monsters.Wolf(),
		HitPoints:     11,
		MaxHitPoints:  11,
		ArmorClass:    13,
		AbilityScores: shared.AbilityScores{},
	}
}

// mock builds the contest Vicious Mockery would raise, with whichever halves
// the scene declares.
func mock(pools []damage.Damage, application combatActions.ConditionApplication, roll int) Machine {
	return NewContest(&ContestInput{
		Gate:        mockeryGate(),
		SaverID:     heroID,
		Application: application,
		Damage:      pools,
		Cause:       mockedCause(),
		Roller:      facedRoller{d20: roll, other: psychicFace},
	})
}

func prone() combatActions.ConditionApplication {
	return combatActions.ConditionApplication{Ref: *refs.Conditions.Prone()}
}

func (s *ContestDamageTestSuite) resolve(
	saver *character.Data, machine Machine, cost *Cost, payer *character.Data,
) (*Output, error) {
	participants := []Participant{{Character: saver}, {Monster: s.wolfData()}}
	if payer != nil {
		participants = append(participants, Participant{Character: payer})
	}

	return Resolve(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World: s.world(), Participants: participants, Machine: machine, Cost: cost,
	})
}

func (s *ContestDamageTestSuite) outcome(out *Output) ContestOutcome {
	outcome, ok := out.Outcome.(ContestOutcome)
	s.Require().True(ok, "a contest produces a ContestOutcome")

	return outcome
}

func (s *ContestDamageTestSuite) sheet(out *Output, id string) *character.Data {
	for _, data := range out.DirtyCharacters {
		if data.ID == id {
			return data
		}
	}
	s.Require().Failf("no dirty sheet", "%q did not come back to be saved", id)

	return nil
}

func (s *ContestDamageTestSuite) conditionRefs(data *character.Data) []string {
	out := make([]string, 0, len(data.Conditions))
	for _, raw := range data.Conditions {
		var peek struct {
			Ref core.Ref `json:"ref"`
		}
		s.Require().NoError(json.Unmarshal(raw, &peek))
		out = append(out, peek.Ref.String())
	}

	return out
}

// THE HEADLINE. One machine, one save, two consequences: the psychic damage
// lands and the rider goes on, in that order, and Imposed says which is which.
func (s *ContestDamageTestSuite) TestAFailedSaveDeliversDamageAndThenTheCondition() {
	out, err := s.resolve(s.saver(14), mock(psychic(), prone(), straightRoll), nil, nil)
	s.Require().NoError(err)

	outcome := s.outcome(out)
	s.Require().False(outcome.Succeeded, "+1 on a 3 is 4 against DC 13")
	s.Require().Equal(spellSaveDC, outcome.DC)
	s.Require().Equal(abilities.WIS, outcome.Ability)

	s.Require().Len(outcome.Imposed, 2, "one delivery per declared kind, and no more")
	s.Require().Equal(ImposedDamage, outcome.Imposed[0].Kind, "damage first")
	s.Require().Equal(psychicFace, outcome.Imposed[0].Amount)
	s.Require().Equal("1d4 psychic damage", outcome.Imposed[0].Description)
	s.Require().Len(outcome.Imposed[0].Components, 1, "one component per declared pool")
	s.Require().Equal(damage.Psychic, outcome.Imposed[0].Components[0].DamageType)
	s.Require().NotNil(outcome.Imposed[0].Components[0].Roll.Dice,
		"the faces travel, so a record can show the 1d4 rather than only its total")
	s.Require().Equal(psychicFace, outcome.Imposed[0].Components[0].Roll.Dice.Subtotal)

	s.Require().Equal(ImposedCondition, outcome.Imposed[1].Kind, "then the condition")
	s.Require().Equal(refs.Conditions.Prone().String(), outcome.Imposed[1].Ref.String())

	sheet := s.sheet(out, heroID)
	s.Require().Equal(11, sheet.HitPoints, "14 - 3, applied exactly once")
	s.Require().Equal([]string{refs.Conditions.Prone().String()}, s.conditionRefs(sheet))
}

// The control. Same declaration, a better die, and a made save negates both
// halves — while the outcome still says what was rolled and what it was against.
func (s *ContestDamageTestSuite) TestAMadeSaveDeliversNeitherAndStillRecordsTheRoll() {
	out, err := s.resolve(s.saver(14), mock(psychic(), prone(), advantageRoll), nil, nil)
	s.Require().NoError(err)

	outcome := s.outcome(out)
	s.Require().True(outcome.Succeeded)
	s.Require().Empty(outcome.Imposed, "a made save costs nothing at all")

	s.Require().NotNil(outcome.Save.Result)
	s.Require().Equal(advantageRoll, outcome.Save.Result.Roll)
	s.Require().Equal(advantageRoll+1, outcome.Save.Result.Total, "WIS 12 is +1")
	s.Require().Equal(spellSaveDC, outcome.DC)
	s.Require().Equal(refs.Conditions.Prone().String(), outcome.AtStake.Ref.String(),
		"and it still says what was resisted")

	s.Require().Empty(out.DirtyCharacters, "no damage, no condition, nothing to save")
}

// Damage with no rider: the other half of the fan-out, and the one that proves
// the condition is no longer required for a contest to mean something.
func (s *ContestDamageTestSuite) TestADamageOnlyContestLandsDamageAndNothingElse() {
	out, err := s.resolve(s.saver(14), mock(psychic(), combatActions.ConditionApplication{}, straightRoll), nil, nil)
	s.Require().NoError(err)

	outcome := s.outcome(out)
	s.Require().False(outcome.Succeeded)
	s.Require().Equal(ImposedDamage, outcome.AtStake.Kind,
		"with no condition declared, the damage is what the save was against")
	s.Require().Equal("1d4 psychic damage", outcome.AtStake.Description)
	s.Require().Zero(outcome.AtStake.Amount, "at stake is before the dice, so it claims no amount")

	s.Require().Len(outcome.Imposed, 1)
	s.Require().Equal(ImposedDamage, outcome.Imposed[0].Kind)
	s.Require().Equal(psychicFace, outcome.Imposed[0].Amount)

	sheet := s.sheet(out, heroID)
	s.Require().Equal(11, sheet.HitPoints)
	s.Require().Empty(sheet.Conditions, "nothing was declared, so nothing went on")
}

// THE REGRESSION THAT MATTERS. A contest that declares no damage is what it was
// before this slice: one condition, no arithmetic, an untouched hit point total.
func (s *ContestDamageTestSuite) TestAConditionOnlyContestIsUnchanged() {
	out, err := s.resolve(s.saver(14), mock(nil, prone(), straightRoll), nil, nil)
	s.Require().NoError(err)

	outcome := s.outcome(out)
	s.Require().False(outcome.Succeeded)
	s.Require().Len(outcome.Imposed, 1)
	s.Require().Equal(ImposedCondition, outcome.Imposed[0].Kind)
	s.Require().Equal(refs.Conditions.Prone().String(), outcome.Imposed[0].Ref.String())
	s.Require().Zero(outcome.Imposed[0].Amount, "a condition has no amount to report")
	s.Require().Nil(outcome.Imposed[0].Components)

	sheet := s.sheet(out, heroID)
	s.Require().Equal(14, sheet.HitPoints, "no damage was declared, so none was dealt")
	s.Require().Equal([]string{refs.Conditions.Prone().String()}, s.conditionRefs(sheet))
}

// The damage goes through the sheet's own ApplyDamage, so a cantrip can drop
// somebody: 3 psychic on 3 hit points leaves the saver down, and the condition
// still lands on the sheet that comes back.
func (s *ContestDamageTestSuite) TestDamageThatDropsTheSaverLeavesThemDown() {
	out, err := s.resolve(s.saver(psychicFace), mock(psychic(), prone(), straightRoll), nil, nil)
	s.Require().NoError(err)

	s.Require().Equal(psychicFace, s.outcome(out).Imposed[0].Amount)

	sheet := s.sheet(out, heroID)
	s.Require().Zero(sheet.HitPoints, "3 psychic on 3 hit points")
}

// And the transition that call OWNS comes with it: damage taken at zero is a
// death-save failure, applied by Character.ApplyDamage on the direct path
// because nothing here publishes DamageReceivedEvent. The contest did not
// reimplement it and did not have to.
func (s *ContestDamageTestSuite) TestDamageTakenAtZeroIsADeathSaveFailure() {
	down := s.saver(0)
	down.DeathSaveState = &saves.DeathSaveState{Successes: 1, Failures: 1}

	out, err := s.resolve(down, mock(psychic(), prone(), straightRoll), nil, nil)
	s.Require().NoError(err)

	sheet := s.sheet(out, heroID)
	s.Require().NotNil(sheet.DeathSaveState)
	s.Require().Equal(2, sheet.DeathSaveState.Failures, "one more failure, exactly as a strike's damage records")
	s.Require().Equal(1, sheet.DeathSaveState.Successes)
}

// A contest entered from the door: the price is charged before anything moves,
// and the machine is never told there was one.
func (s *ContestDamageTestSuite) TestACastPricedAtTheDoorIsChargedAndStillResolves() {
	out, err := s.resolve(s.saver(14), mock(psychic(), prone(), straightRoll), &Cost{
		PayerID: bardID,
		Profile: &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}},
		Turn:    &Turn{Number: mockeryTurn, Speed: mockerySpeed},
	}, s.bard(1))
	s.Require().NoError(err)

	outcome := s.outcome(out)
	s.Require().Len(outcome.Imposed, 2, "the contest ran exactly as it does for free")

	payer := s.sheet(out, bardID)
	s.Require().NotNil(payer.ActionEconomy)
	s.Require().Zero(payer.ActionEconomy.ActionsRemaining, "the action was spent at the door")
}

// And the refusal that makes the charge mean something: an empty budget is
// refused before the save is rolled or anything is delivered.
func (s *ContestDamageTestSuite) TestACastWithNoActionLeftIsRefusedAndChargesNothing() {
	out, err := s.resolve(s.saver(14), mock(psychic(), prone(), straightRoll), &Cost{
		PayerID: bardID,
		Profile: &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}},
		Turn:    &Turn{Number: mockeryTurn, Speed: mockerySpeed},
	}, s.bard(0))

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Require().Nil(out, "a refused resolution hands back nothing to store")
	s.Require().Contains(err.Error(), bardID, "and the refusal names who could not pay")
}

// A contest that would deliver nothing is refused rather than run: a save the
// player rolls for no consequence would look exactly like one that worked.
func (s *ContestDamageTestSuite) TestAContestDeclaringNeitherHalfIsRefused() {
	_, err := s.resolve(s.saver(14), NewContest(&ContestInput{
		Gate:    mockeryGate(),
		SaverID: heroID,
		Roller:  facedRoller{d20: straightRoll, other: psychicFace},
	}), nil, nil)

	s.Require().ErrorIs(err, ErrBadAction)
	s.Require().Contains(err.Error(), "condition, damage, or both")
}

// Malformed damage is refused during pure preflight, so a priced cast whose
// content is broken charges nobody.
func (s *ContestDamageTestSuite) TestMalformedDamageIsRefusedBeforeThePriceIsCharged() {
	out, err := s.resolve(s.saver(14), mock(
		[]damage.Damage{{Dice: "not-dice", Type: damage.Psychic}}, prone(), straightRoll,
	), &Cost{
		PayerID: bardID,
		Profile: &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}},
		Turn:    &Turn{Number: mockeryTurn, Speed: mockerySpeed},
	}, s.bard(1))

	s.Require().ErrorIs(err, ErrBadAction)
	s.Require().Nil(out)
}

// One Gather per effect kind, and each says what it does — so a reader of the
// step log sees the two deliveries rather than one step doing both.
func TestTheContestsDeliveryStepsSayWhatTheyDo(t *testing.T) {
	deal := applyPreparedDamage(
		[]damage.Damage{{Dice: "1d4", Type: damage.Psychic}}, nil, dnd5eEvents.SaveCause{}, nil, heroID,
		func(ImposedEffect) (Step, error) { return nil, nil },
	)
	require.Equal(t, "deal 1d4 psychic damage", deal.Name())

	require.Equal(t, "1d4 psychic and 1d6 fire damage", describeDamage([]damage.Damage{
		{Dice: "1d4", Type: damage.Psychic},
		{Dice: "1d6", Type: damage.Fire},
	}))
}
