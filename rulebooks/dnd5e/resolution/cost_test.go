// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// CostTestSuite drives the door: what an action costs is paid before the
// machine runs, and a resolution that cannot be paid for never starts one.
type CostTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestCostSuite(t *testing.T) {
	suite.Run(t, new(CostTestSuite))
}

func (s *CostTestSuite) SetupTest() {
	s.ctx = context.Background()
}

const (
	// bankedAttacks is what the Attack action left behind for a swing to spend.
	bankedAttacks = 1

	// heroBaseSpeed is the Human walking speed this hero's race gives them, and
	// therefore the number a door that read the SHEET instead of the caller
	// would seed movement with.
	heroBaseSpeed = 30

	// suppliedSpeed is deliberately not heroBaseSpeed. Every refresh in this
	// suite states it, so an assertion on movement tells the two sources apart.
	suppliedSpeed = 25

	firstTurn = 1
	nextTurn  = 2
)

// economy is a turn's worth of action economy, filed under a turn number, with
// whatever the Attack action has already banked.
func (s *CostTestSuite) economy(turn, actions, banked int) *character.ActionEconomyData {
	granted := map[character.GrantedActionKey]int{}
	if banked > 0 {
		granted[character.GrantedAttacks] = banked
	}

	return &character.ActionEconomyData{
		TurnNumber:            turn,
		ActionsRemaining:      actions,
		BonusActionsRemaining: 1,
		ReactionsRemaining:    1,
		MovementRemaining:     heroBaseSpeed,
		Granted:               granted,
	}
}

// hero is a level-1 human fighter with a longsword they know how to use. A nil
// economy is a character who is not in a fight.
func (s *CostTestSuite) hero(econ *character.ActionEconomyData) *character.Data {
	return &character.Data{
		ID:       heroID,
		PlayerID: "player-1",
		Name:     "Grog",
		Level:    1,
		ClassID:  classes.Fighter,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:           14,
		MaxHitPoints:        14,
		ArmorClass:          14,
		ProficiencyBonus:    2,
		WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponMartial},
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		},
		EquipmentSlots: character.EquipmentSlots{character.SlotMainHand: string(weapons.Longsword)},
		ActionEconomy:  econ,
	}
}

func (s *CostTestSuite) load(data *character.Data) *character.Character {
	c, err := character.Load(s.ctx, data)
	s.Require().NoError(err)

	return c
}

// attackCost and strikeCost come from the REAL compilers rather than a
// hand-built profile, so the two halves of the economy meet here as they will
// in the session: the rulebook prices the action, the door charges it.
func (s *CostTestSuite) attackCost(data *character.Data) *combat.SpendProfile {
	profile, err := character.CostOfAttack(s.load(data))
	s.Require().NoError(err)

	return profile
}

func (s *CostTestSuite) strikeCost(data *character.Data) *combat.SpendProfile {
	profile, err := character.CostOfStrike(s.load(data))
	s.Require().NoError(err)

	return profile
}

func (s *CostTestSuite) world() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms:  []encounter.RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []encounter.MemberInput{
			{ID: heroID, Kind: encounter.KindPlayer, Room: "room-1", Position: spatial.Position{X: 5, Y: 5}},
			{ID: wolfID, Kind: encounter.KindMonster, Room: "room-1", Position: spatial.Position{X: 5, Y: 6}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

// witnessMachine records that it was driven, then gets out of the way. The
// evidence a refused resolution has to leave is negative — nothing ran — so the
// pin needs something that would have said otherwise.
type witnessMachine struct {
	inner   Machine
	started bool
}

func (m *witnessMachine) Start(ctx context.Context, cast *Participants) (Step, error) {
	m.started = true

	return m.inner.Start(ctx, cast)
}

// countingRoller is the other half of that evidence: a strike that starts rolls
// its attack, so a count of zero says the machine never got that far. Every
// answer is a miss, which keeps a resolution that DOES run away from the damage
// phase and its sheet writes.
const missRoll = 1

type countingRoller struct {
	rolls int
}

func (r *countingRoller) Roll(_ context.Context, _ int) (int, error) {
	r.rolls++

	return missRoll, nil
}

func (r *countingRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	r.rolls += count

	out := make([]int, count)
	for i := range out {
		out[i] = missRoll
	}

	return out, nil
}

// swing drives the real strike machine — this hero's own longsword against the
// catalog wolf — with the door's cost in front of it.
//
// Input.Roller is a separate, ordinary roller: it reconstitutes effects at
// attach time, and letting it share the counting one would put rolls nobody
// asked about into the evidence.
func (s *CostTestSuite) swing(
	hero *character.Data, cost *Cost, roller *countingRoller,
) (*Output, *witnessMachine, error) {
	definition, err := character.AssembleAttack(s.load(hero), &character.AssembleAttackInput{Slot: character.SlotMainHand})
	s.Require().NoError(err)

	machine := &witnessMachine{inner: NewStrike(&StrikeInput{
		AttackerID: heroID,
		TargetID:   wolfID,
		Definition: definition,
		Roller:     roller,
	})}

	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: hero}, {Monster: monsters.NewWolf(wolfID).ToData()}},
		Machine:      machine,
		Cost:         cost,
	})

	return out, machine, err
}

// declare runs a cost against a machine that starts and is immediately Done —
// the shape the ruling gives an action with no interaction behind it. The
// Attack action is exactly that: it banks swings rather than making one, and
// ErrNoMachine's own doc blesses the form ("distinct from a machine that
// finishes immediately, which is legal").
func (s *CostTestSuite) declare(hero *character.Data, cost *Cost) (*Output, error) {
	return Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: hero}, {Monster: monsters.NewWolf(wolfID).ToData()}},
		Machine:      &captureMachine{},
		Cost:         cost,
	})
}

func (s *CostTestSuite) thisTurn() *Turn {
	return &Turn{Number: firstTurn, Speed: suppliedSpeed}
}

var errPreflight = errors.New("preflight refused")

type failingPreflightMachine struct {
	payer *character.Character
}

func (m *failingPreflightMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	m.payer, _ = cast.Character(heroID)
	return nil, errPreflight
}

func (s *CostTestSuite) TestStartFailurePaysNothing() {
	hero := s.hero(s.economy(firstTurn, 1, bankedAttacks))
	machine := &failingPreflightMachine{}

	out, err := Resolve(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World: s.world(), Participants: []Participant{{Character: hero}, {Monster: monsters.NewWolf(wolfID).ToData()}},
		Machine: machine,
		Cost:    &Cost{PayerID: heroID, Profile: s.strikeCost(hero), Turn: s.thisTurn()},
	})

	s.Require().ErrorIs(err, errPreflight)
	s.Require().Nil(out)
	s.Require().NotNil(machine.payer)
	s.Equal(1, machine.payer.CapacityLeft(combat.CapacityAttack), "invalid machine start pays nothing")
}

// ---------------------------------------------------------------------------
// THE HEADLINE: a swing nobody can pay for preflights but never executes.
// ---------------------------------------------------------------------------

// The hero is in combat with an action in hand, but the Attack action was never
// taken — so there is no banked swing for this strike to spend. The refusal has
// to arrive after pure preflight but before dice or mutation.
func (s *CostTestSuite) TestAnActorWhoCannotPayPreflightsButNeverExecutes() {
	hero := s.hero(s.economy(firstTurn, 1, 0))
	roller := &countingRoller{}

	out, machine, err := s.swing(hero, &Cost{
		PayerID: heroID,
		Profile: s.strikeCost(hero),
		Turn:    s.thisTurn(),
	}, roller)

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Require().Nil(out, "a refused resolution hands back nothing to store")
	s.Require().True(machine.started, "pure preflight runs before payment")
	s.Require().Zero(roller.rolls, "a refused strike rolls no attack")

	// The sentinel is what E3 matches on; the gate's own message is what says
	// WHICH currency ran out, and it has to survive being wrapped or the
	// translation on the other side has nothing to say.
	s.Require().Contains(err.Error(), heroID, "the refusal names who could not pay")
	s.Require().Contains(err.Error(), string(combat.CapacityAttack), "and what they were short of")
}

// Effective AC is an event-backed read for characters, so it belongs to
// execution rather than pure preflight. A strike the actor cannot afford must
// therefore reach neither that chain nor its dice.
func (s *CostTestSuite) TestAnUnaffordableStrikePublishesNoACChain() {
	bus := events.NewEventBus()
	acFolds := 0
	_, err := combat.ACChain.On(bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, _ *combat.ACChainEvent,
			c chain.Chain[*combat.ACChainEvent],
		) (chain.Chain[*combat.ACChainEvent], error) {
			acFolds++
			return c, nil
		})
	s.Require().NoError(err)

	hero := s.hero(s.economy(firstTurn, 1, 0))
	wolf := monsters.NewWolf(wolfID).ToData()
	roller := &countingRoller{}
	out, err := resolveOn(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World: s.world(), Participants: []Participant{{Character: hero}, {Monster: wolf}},
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID, TargetID: heroID, Definition: wolf.Actions[0], Roller: roller,
		}),
		Cost: &Cost{PayerID: heroID, Profile: s.strikeCost(hero), Turn: s.thisTurn()},
	}, newSurface(bus))

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Require().Nil(out)
	s.Zero(acFolds, "payment refusal happens before the character publishes its AC chain")
	s.Zero(roller.rolls, "payment refusal also happens before attack dice")
}

// A recurring on-hit gate is structurally valid shared data, but resolution
// cannot execute it yet. Strike must reject that limitation during its own
// Start, before the door spends or any attack phase can mutate a participant.
func (s *CostTestSuite) TestARecurringOnHitGateFailsBeforePaymentDiceOrMutation() {
	hero := s.hero(s.economy(firstTurn, 1, bankedAttacks))
	definition, err := character.AssembleAttack(s.load(hero), &character.AssembleAttackInput{
		Slot: character.SlotMainHand,
	})
	s.Require().NoError(err)

	gate := saves.NewSaveGate(abilities.STR, 11)
	gate.Recurrence = saves.RecurrenceEndOfTurn
	definition.Attack.OnHit = append(definition.Attack.OnHit, combatActions.ConditionApplication{
		Ref:  *refs.Conditions.Prone(),
		Save: gate,
	})

	roller := &actionRoller{singles: []int{15}, damage: [][]int{{4}}}
	machine := NewStrike(&StrikeInput{
		AttackerID: heroID,
		TargetID:   wolfID,
		Definition: definition,
		Roller:     roller,
	}).(*strikeMachine)
	applyDamageCalls := 0
	machine.applyDamage = func(
		ctx context.Context, target combat.Combatant, input *combat.ApplyDamageInput,
	) *combat.ApplyDamageResult {
		applyDamageCalls++
		return target.ApplyDamage(ctx, input)
	}

	wolf := monsters.NewWolf(wolfID).ToData()
	out, err := Resolve(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World: s.world(), Participants: []Participant{{Character: hero}, {Monster: wolf}},
		Machine: machine,
		Cost:    &Cost{PayerID: heroID, Profile: s.strikeCost(hero), Turn: s.thisTurn()},
	})

	s.Require().ErrorIs(err, ErrRecurrenceUnsupported)
	s.Require().Nil(out)
	s.Zero(roller.calls, "outer strike preflight refuses before attack or damage dice")
	s.Zero(applyDamageCalls, "outer strike preflight refuses before target application")

	s.Require().NotNil(machine.cast)
	loadedHero, ok := machine.cast.Character(heroID)
	s.Require().True(ok)
	s.Equal(bankedAttacks, loadedHero.CapacityLeft(combat.CapacityAttack),
		"outer strike preflight refuses before payment")
	loadedWolf, ok := machine.cast.Monster(wolfID)
	s.Require().True(ok)
	s.Equal(wolf.HitPoints, loadedWolf.ToData().HitPoints,
		"outer strike preflight leaves the target unchanged")
}

// A paid swing that MISSES still costs what it cost. The attacker's sheet comes
// back dirty with nothing on it but the spend, which is the whole reason the
// economy had to mark (#1087): a miss changes nothing else, so without the mark
// the debit would serialize perfectly and then be dropped by the write-back.
func (s *CostTestSuite) TestAPaidStrikeSpendsTheBankEvenWhenItMisses() {
	hero := s.hero(s.economy(firstTurn, 1, bankedAttacks))
	roller := &countingRoller{}

	out, machine, err := s.swing(hero, &Cost{
		PayerID: heroID,
		Profile: s.strikeCost(hero),
		Turn:    s.thisTurn(),
	}, roller)

	s.Require().NoError(err)
	s.Require().True(machine.started)

	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok, "a strike produces a StrikeOutcome")
	s.Require().False(outcome.Hit, "a 4 plus the hero's +5 does not reach the wolf's AC 13")

	s.Require().Len(out.DirtyCharacters, 1, "the attacker is dirty on a miss, and dirty for the economy alone")
	spent := out.DirtyCharacters[0]
	s.Require().Equal(heroID, spent.ID)
	s.Require().Equal(bankedAttacks-1, spent.ActionEconomy.Granted[character.GrantedAttacks],
		"exactly one swing off the bank")
	s.Require().Equal(hero.HitPoints, spent.HitPoints, "and nothing else about the attacker moved")
	s.Require().Empty(out.DirtyMonsters, "a miss takes no hit points")
}

// A nil cost is a free action: the door looks nobody up, refreshes nothing and
// charges nothing, and the sheet comes back exactly as clean as it went in.
// Every other suite in this package is the wider version of this pin — none of
// them passes a cost, and none of them changed.
func (s *CostTestSuite) TestANilCostChargesNothing() {
	hero := s.hero(s.economy(firstTurn, 1, bankedAttacks))

	out, machine, err := s.swing(hero, nil, &countingRoller{})

	s.Require().NoError(err)
	s.Require().True(machine.started, "a free action still runs")
	s.Require().Empty(out.DirtyCharacters, "nothing was spent, so there is nothing to save")
}

// ---------------------------------------------------------------------------
// One turn, one action — and the turn after.
// ---------------------------------------------------------------------------

// The end-to-end shape of the economy through the door: a fighter's Attack
// action costs the action and banks a swing, a second one in the same turn is
// refused, and a new turn refills the bank on the way in.
//
// The sheet travels the way a host would carry it — what came back dirty is
// what goes in next — so the refusal is against state that was actually written
// rather than against a fixture edited between calls.
func (s *CostTestSuite) TestTwoActionsInOneTurnAndTheSecondIsRefused() {
	cost := func(sheet *character.Data, turn int) *Cost {
		return &Cost{
			PayerID: heroID,
			Profile: s.attackCost(sheet),
			Turn:    &Turn{Number: turn, Speed: suppliedSpeed},
		}
	}

	hero := s.hero(s.economy(firstTurn, 1, 0))

	first, err := s.declare(hero, cost(hero, firstTurn))
	s.Require().NoError(err)
	s.Require().Len(first.DirtyCharacters, 1)

	spent := first.DirtyCharacters[0]
	s.Require().Zero(spent.ActionEconomy.ActionsRemaining, "the action is gone")
	s.Require().Equal(1, spent.ActionEconomy.Granted[character.GrantedAttacks],
		"and a level-1 fighter's Attack action banks exactly one swing")

	_, err = s.declare(spent, cost(spent, firstTurn))
	s.Require().ErrorIs(err, ErrCannotPay, "a second Attack action in the same turn has no action to pay with")

	third, err := s.declare(spent, cost(spent, nextTurn))
	s.Require().NoError(err, "a new turn refills the bank at the door")

	refreshed := third.DirtyCharacters[0].ActionEconomy
	s.Require().Equal(nextTurn, refreshed.TurnNumber)
	s.Require().Zero(refreshed.ActionsRemaining, "one seeded by the refresh, one spent by the door")
	s.Require().Equal(1, refreshed.Granted[character.GrantedAttacks],
		"the reseed empties the bank and the grant fills it again")
}

// A refused payment leaves the caller's sheet byte-identical. The turn stated
// here is the one the economy is already filed under, so the refresh is on its
// write-nothing path and the refusal is the only thing that could have written.
func (s *CostTestSuite) TestARefusedPaymentWritesNothingBack() {
	hero := s.hero(s.economy(firstTurn, 1, 0))

	before, err := json.Marshal(hero)
	s.Require().NoError(err)

	out, _, err := s.swing(hero, &Cost{
		PayerID: heroID,
		Profile: s.strikeCost(hero),
		Turn:    s.thisTurn(),
	}, &countingRoller{})
	s.Require().ErrorIs(err, ErrCannotPay)
	s.Require().Nil(out, "nothing comes back, so nothing reaches storage")

	after, err := json.Marshal(hero)
	s.Require().NoError(err)
	s.Require().JSONEq(string(before), string(after), "the sheet the caller handed over is untouched")
}

// ---------------------------------------------------------------------------
// Who can be charged.
// ---------------------------------------------------------------------------

// A monster in the cast is refused BY NAME rather than nil-dereferenced or
// silently let through. monster.Monster keeps no economy — it is handed one for
// a turn and it is thrown away after — so there is nothing on that sheet to
// debit, and the message says that rather than pretending the wolf was never
// passed in.
func (s *CostTestSuite) TestACostNamingAMonsterIsRefusedByName() {
	hero := s.hero(s.economy(firstTurn, 1, bankedAttacks))

	out, machine, err := s.swing(hero, &Cost{
		PayerID: wolfID,
		Profile: s.strikeCost(hero),
		Turn:    s.thisTurn(),
	}, &countingRoller{})

	s.Require().ErrorIs(err, ErrNoPayer)
	s.Require().Contains(err.Error(), wolfID)
	s.Require().Contains(err.Error(), "monster", "the refusal says which of the two ways it failed")
	s.Require().Nil(out)
	s.Require().True(machine.started, "pure preflight runs before payment")
}

func (s *CostTestSuite) TestACostNamingSomebodyNotPassedInIsRefused() {
	hero := s.hero(s.economy(firstTurn, 1, bankedAttacks))

	out, machine, err := s.swing(hero, &Cost{
		PayerID: "ghost",
		Profile: s.strikeCost(hero),
		Turn:    s.thisTurn(),
	}, &countingRoller{})

	s.Require().ErrorIs(err, ErrNoPayer)
	s.Require().Contains(err.Error(), "ghost")
	s.Require().Nil(out)
	s.Require().True(machine.started, "pure preflight runs before payment")
}

// Out of combat is a refusal, not a free swing. The refresh does not rescue it:
// a sheet with no economy has no turn to be stale, and inventing one here would
// put a character in a fight nobody put them in.
func (s *CostTestSuite) TestAnActorWithNoEconomyIsRefusedRatherThanChargedNothing() {
	hero := s.hero(nil)

	out, machine, err := s.swing(hero, &Cost{
		PayerID: heroID,
		Profile: s.strikeCost(hero),
		Turn:    s.thisTurn(),
	}, &countingRoller{})

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Require().Nil(out)
	s.Require().True(machine.started, "pure preflight runs before payment")
}

// ---------------------------------------------------------------------------
// A cost that is not a price.
// ---------------------------------------------------------------------------

func (s *CostTestSuite) TestACostThatNamesNoPayerIsRefused() {
	hero := s.hero(s.economy(firstTurn, 1, bankedAttacks))

	out, machine, err := s.swing(hero, &Cost{Profile: s.strikeCost(hero)}, &countingRoller{})

	s.Require().ErrorIs(err, ErrBadCost)
	s.Require().Nil(out)
	s.Require().False(machine.started)
}

// A price keyed to a currency no ledger holds is refused as a MALFORMED cost,
// not as an actor who ran out. E3 translates these to a client, and a caller
// told "you are out of actions" about a typo would go looking at the wrong
// sheet.
func (s *CostTestSuite) TestACostKeyedToACurrencyNoLedgerHoldsIsRefused() {
	hero := s.hero(s.economy(firstTurn, 1, bankedAttacks))

	out, machine, err := s.swing(hero, &Cost{
		PayerID: heroID,
		Profile: &combat.SpendProfile{
			Capacity: map[combat.CapacityType]int{combat.CapacityType("sword-swings"): 1},
		},
	}, &countingRoller{})

	s.Require().ErrorIs(err, ErrBadCost)
	s.Require().NotErrorIs(err, ErrCannotPay)
	s.Require().Contains(err.Error(), "sword-swings", "and it names the key nobody holds")
	s.Require().Nil(out)
	s.Require().False(machine.started)
}

// The refusal comes out of Validate, so it lands before the world is loaded.
// The world here is one this package cannot make sense of, and the malformed
// cost has to beat it to the answer.
func (s *CostTestSuite) TestAMalformedCostIsRefusedBeforeTheWorldIsLoaded() {
	hero := s.hero(s.economy(firstTurn, 1, bankedAttacks))

	out, err := Resolve(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        encounter.EncounterData{},
		Participants: []Participant{{Character: hero}},
		Machine:      &captureMachine{},
		Cost:         &Cost{Profile: s.strikeCost(hero)},
	})

	s.Require().ErrorIs(err, ErrBadCost)
	s.Require().Nil(out)
}

// ---------------------------------------------------------------------------
// The turn marker.
// ---------------------------------------------------------------------------

// A bank left over from last turn is refilled on the way in, which is what makes
// materialise-at-first-ask work with no turn-start signal to listen for.
//
// The movement assertion is the pin that matters: the speed seeded is the one
// the CALLER stated, and it is deliberately not this hero's base 30 — a door
// that read the sheet's own accessor would answer 30 here and be wrong for
// anyone hasted, slowed or unarmored-moving.
func (s *CostTestSuite) TestTheDoorRefillsAStaleBankFromTheSuppliedTurn() {
	hero := s.hero(s.economy(firstTurn, 0, 0))

	out, err := s.declare(hero, &Cost{
		PayerID: heroID,
		Profile: s.attackCost(hero),
		Turn:    &Turn{Number: nextTurn, Speed: suppliedSpeed},
	})
	s.Require().NoError(err, "the spent bank was refilled before it was charged")

	refreshed := out.DirtyCharacters[0].ActionEconomy
	s.Require().Equal(nextTurn, refreshed.TurnNumber)
	s.Require().Zero(refreshed.ActionsRemaining, "one seeded, one spent")
	s.Require().Equal(suppliedSpeed, refreshed.MovementRemaining,
		"the supplied speed, not the sheet's base speed")
}

// No turn stated, no turn invented. The bank is charged exactly as it was
// stored, and an actor who spent their action last turn is refused rather than
// quietly handed a fresh one.
func (s *CostTestSuite) TestWithoutATurnTheBankIsChargedExactlyAsStored() {
	hero := s.hero(s.economy(firstTurn, 0, 0))

	_, err := s.declare(hero, &Cost{PayerID: heroID, Profile: s.attackCost(hero)})

	s.Require().ErrorIs(err, ErrCannotPay)
}

// A free action does not go through the door at all, so a stale bank stays
// stale: the refresh is something a cost brings with it, not something every
// resolution does to whoever happens to be in the cast.
func (s *CostTestSuite) TestAFreeActionNeverRefreshesTheBank() {
	hero := s.hero(s.economy(firstTurn, 0, 0))

	out, err := s.declare(hero, nil)

	s.Require().NoError(err)
	s.Require().Empty(out.DirtyCharacters, "no cost, no door")
}

// A cost with a payer and a turn but no price refreshes and charges nothing.
// The door's trigger is the cost being present, not the profile being non-nil —
// which is what lets a caller say "this actor is acting now, and it is free"
// without the two statements having to travel separately.
func (s *CostTestSuite) TestACostWithNoProfileRefreshesAndChargesNothing() {
	hero := s.hero(s.economy(firstTurn, 0, 0))

	out, err := s.declare(hero, &Cost{
		PayerID: heroID,
		Turn:    &Turn{Number: nextTurn, Speed: suppliedSpeed},
	})
	s.Require().NoError(err)

	refreshed := out.DirtyCharacters[0].ActionEconomy
	s.Require().Equal(nextTurn, refreshed.TurnNumber)
	s.Require().Equal(1, refreshed.ActionsRemaining, "refilled, and nothing taken back out")
}

// raging is a condition that subscribes on attach and can be heard afterwards,
// which is what makes it a probe for whether anything survived.
func (s *CostTestSuite) raging() json.RawMessage {
	raw, err := (&conditions.RagingCondition{
		CharacterID: heroID,
		DamageBonus: 2,
		Level:       1,
		Source:      "rage",
	}).ToJSON()
	s.Require().NoError(err)

	return raw
}

// R5 holds on the refusal path too: the payment is refused AFTER every sheet
// attached, so every subscription those attachments granted has to come back
// off. The hero's Raging joins the saving-throw chain on the way in, and after
// the refusal that chain answers nobody.
//
// Asserted on the bus rather than on the teardown call, so the pin survives a
// change to which mechanism does the work — the same shape
// TestAFailedResolutionLeavesNothingOnTheBus uses for the attach path.
func (s *CostTestSuite) TestARefusedPaymentLeavesNothingOnTheBus() {
	inner := events.NewEventBus()

	hero := s.hero(s.economy(firstTurn, 1, 0))
	hero.Conditions = []json.RawMessage{s.raging()}

	out, err := resolveOn(s.ctx, &Input{Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        s.world(),
		Participants: []Participant{{Character: hero}, {Monster: monsters.NewWolf(wolfID).ToData()}},
		Machine:      &captureMachine{},
		Cost:         &Cost{PayerID: heroID, Profile: s.strikeCost(hero), Turn: s.thisTurn()},
	}, newSurface(inner))

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Require().Nil(out)

	event := &dnd5eEvents.SavingThrowChainEvent{SaverID: heroID, Ability: abilities.STR, DC: saveDifficulty}
	chain := events.NewStagedChain[*dnd5eEvents.SavingThrowChainEvent](combat.ModifierStages)

	modified, err := dnd5eEvents.SavingThrowChain.On(inner).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)

	s.Require().False(folded.HasAdvantage(),
		"the hero's Raging attached before the refusal and does not answer this chain")
}
