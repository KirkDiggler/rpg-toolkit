package encounter_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// TurnStateSuite proves Beat-1 slices 1 + 2b on the REAL hydrated path
// (LoadFromData → held *character.Character, no stub):
//   - SetMode→TurnBased seeds the active player's economy from the engine
//     (slice 1: the rpg-api ActionEconomyData{1,1,1} injection is now the
//     engine's job, North-Star Invariant 2).
//   - ActorTurnState exposes the character's two-level menu + economy as toolkit
//     domain types, with the EconomySlot + TargetKind fields populated
//     (slice 2b: the menu-as-data read surface, Invariant 11).
type TurnStateSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestTurnStateSuite(t *testing.T) {
	suite.Run(t, new(TurnStateSuite))
}

func (s *TurnStateSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *TurnStateSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

const (
	monkPlayerID = core.PlayerID("mira")
	monkEntityID = core.EntityID("char-mira")
)

// monkCharJSON builds a level-1 Monk character.Data blob WITHOUT a seeded
// ActionEconomy — proving the encounter seeds it at turn start, not the host.
func (s *TurnStateSuite) monkCharJSON() json.RawMessage {
	s.T().Helper()
	charData := &dnd5eCharacter.Data{
		ID:       string(monkEntityID),
		PlayerID: string(monkPlayerID),
		Name:     "Mira the Monk",
		Level:    1,
		ClassID:  classes.Monk,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12,
			abilities.DEX: 16,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 15,
			abilities.CHA: 8,
		},
		HitPoints:        9,
		MaxHitPoints:     9,
		ArmorClass:       15,
		ProficiencyBonus: 2,
		// NOTE: ActionEconomy intentionally nil — the encounter must seed it.
	}
	raw, err := json.Marshal(charData)
	s.Require().NoError(err)
	return raw
}

// loadedMonkEncounter builds a TURN_BASED encounter with one hydrated Monk seat
// via LoadFromData (the production create→persist→load path), so the encounter
// holds a real *character.Character on its bus.
func (s *TurnStateSuite) loadedMonkEncounter() *encounter.Encounter {
	s.T().Helper()
	view := perception.NewView(monkPlayerID, core.Hex{}, 10)
	view.ApplyReveal(perception.VisibleHexesAt(core.Hex{}, 10))
	data := encounter.NewData("enc-ts")
	data.Players[monkPlayerID] = &encounter.PlayerData{
		ID:         monkPlayerID,
		EntityID:   monkEntityID,
		View:       view,
		HP:         9,
		MaxHP:      9,
		AC:         15,
		DamageDice: "1d4",
		DamageType: "bludgeoning",
		DataJSON:   s.monkCharJSON(),
	}
	enc, err := encounter.LoadFromData(s.ctx, data, s.broker)
	s.Require().NoError(err)
	return enc
}

// TestSetMode_SeedsActiveActorEconomyAndExposesMenu is the slice-1 + slice-2b
// acceptance test on the real hydrated path.
func (s *TurnStateSuite) TestSetMode_SeedsActiveActorEconomyAndExposesMenu() {
	enc := s.loadedMonkEncounter()

	// Before TURN_BASED there is no economy to seed yet.
	pre := enc.ActorTurnState(monkEntityID)
	s.Require().Nil(pre.Economy, "economy is unseeded before TURN_BASED")

	// Flip to TURN_BASED — the only seat, so the Monk is the first actor.
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	s.Require().Equal(monkEntityID, enc.ActiveActor())

	ts := enc.ActorTurnState(monkEntityID)

	// Slice 1: the engine seeded the economy at turn start (Inv 2).
	s.Require().NotNil(ts.Economy, "encounter must seed the held char economy at turn start")
	s.Equal(1, ts.Economy.ActionsRemaining)
	s.Equal(1, ts.Economy.BonusActionsRemaining)
	s.Equal(1, ts.Economy.ReactionsRemaining)
	s.Positive(ts.Economy.MovementRemaining, "movement seeded from character speed")

	// Slice 2b: the menu is exposed as data. Every character is seeded with the
	// four universal abilities (Attack, Dash, Dodge, Disengage); buildAvailableActions
	// always lists Move.
	abilityByID := map[string]dnd5eCharacter.AvailableAbility{}
	for _, a := range ts.Abilities {
		abilityByID[a.Ref.ID] = a
	}
	s.Contains(abilityByID, refs.CombatAbilities.Attack().ID)
	s.Contains(abilityByID, refs.CombatAbilities.Dodge().ID)
	s.Contains(abilityByID, refs.CombatAbilities.Dash().ID)
	s.Contains(abilityByID, refs.CombatAbilities.Disengage().ID)

	// Target kind + economy slot are toolkit-authored on each entry.
	s.Equal(dnd5eCharacter.TargetKindSingleEntity, abilityByID[refs.CombatAbilities.Attack().ID].TargetKind)
	s.Equal(dnd5eCharacter.EconomySlotAction, abilityByID[refs.CombatAbilities.Attack().ID].EconomySlot)
	s.Equal(dnd5eCharacter.TargetKindSelf, abilityByID[refs.CombatAbilities.Dodge().ID].TargetKind)
	s.Equal(dnd5eCharacter.TargetKindNone, abilityByID[refs.CombatAbilities.Dash().ID].TargetKind)

	// Attack is available at turn start (one action in hand).
	s.True(abilityByID[refs.CombatAbilities.Attack().ID].CanUse)

	// Move is present in the actions list, with the movement slot + position target.
	var move *dnd5eCharacter.AvailableAction
	for i := range ts.Actions {
		if ts.Actions[i].Ref.ID == refs.Actions.Move().ID {
			move = &ts.Actions[i]
		}
	}
	s.Require().NotNil(move, "Move is always listed in the action menu")
	s.Equal(dnd5eCharacter.EconomySlotMovement, move.EconomySlot)
	s.Equal(dnd5eCharacter.TargetKindPosition, move.TargetKind)
	// D17: the character menu reports Move usable (a L1 char CAN move), but the
	// encounter composes effective takeability and marks it unavailable this
	// beat (movement lands in Beat 2) with an honest reason. The menu↔verb stay
	// consistent — TestTakeAction_RejectsDeferredMove proves the verb agrees.
	s.False(move.CanUse, "encounter defers Move this beat (D17)")
	s.NotEmpty(move.Reason, "deferred Move carries an unavailable reason")
}

// TestActorTurnState_NonPlayerActorIsEmpty proves the query is a clean no-op for
// a seat with no hydrated character (returns the zero view with the id set).
func (s *TurnStateSuite) TestActorTurnState_NonPlayerActorIsEmpty() {
	enc := s.loadedMonkEncounter()
	ts := enc.ActorTurnState(core.EntityID("nobody"))
	s.Equal(core.EntityID("nobody"), ts.ActorID)
	s.Nil(ts.Economy)
	s.Empty(ts.Abilities)
	s.Empty(ts.Actions)
}

// TestTakeAction_DodgeFlowsThroughGeneralDelegation proves the general
// dispatch (slice 3): a NON-attack ref (Dodge) reaches the held character's
// rules engine via ActivateAbility — no attack gate, no per-ref special-casing
// in the encounter — decrements the economy, and emits a first-class
// ActionResolvedEvent (Invariant 9) carrying the dodge ref + economy consumed.
//
// Note: activating Dodge today only spends the action + publishes
// DodgeActivatedEvent; wiring the DodgingCondition's mechanical effect is a
// separate gap (the toolkit has DodgingCondition but nothing applies it on
// DodgeActivated). Beat-1 proves the DISPATCH generality — the economy spend +
// resolved-action record — not Dodge's full mechanical effect.
func (s *TurnStateSuite) TestTakeAction_DodgeFlowsThroughGeneralDelegation() {
	enc := s.loadedMonkEncounter()
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	s.Require().Equal(monkEntityID, enc.ActiveActor())

	sub, err := s.broker.Subscribe("enc-ts", monkPlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	// Take Dodge — a non-attack ability — through the unified TakeAction verb.
	err = enc.TakeAction(monkPlayerID,
		encounter.ActionRef{
			Module: "dnd5e",
			Type:   refs.CombatAbilities.Dodge().Type,
			ID:     refs.CombatAbilities.Dodge().ID,
		},
		encounter.ActionTarget{EntityID: monkEntityID},
	)
	s.Require().NoError(err, "Dodge must flow through the general delegation, not hit an attack gate")

	// The economy decremented by one action (the engine deducted it).
	ts := enc.ActorTurnState(monkEntityID)
	s.Require().NotNil(ts.Economy)
	s.Equal(0, ts.Economy.ActionsRemaining, "Dodge spends the standard action")

	// A first-class ActionResolvedEvent fired for the non-attack action.
	var action *events.ActionResolvedEvent
	for _, e := range collectEventsTyped(sub, 500*time.Millisecond) {
		if ev, ok := e.(*events.ActionResolvedEvent); ok {
			action = ev
		}
	}
	s.Require().NotNil(action, "every action emits an ActionResolvedEvent, incl. non-attacks (Inv 9)")
	s.Equal(monkEntityID, action.ActorID)
	s.Equal(refs.CombatAbilities.Dodge().String(), action.ActionRef,
		"the resolved-action event carries the actor's real submitted ref")
	s.Equal(1, action.EconomyConsumed.Actions, "Dodge consumed one standard action")
}

// combatMonkEncounter builds a TURN_BASED encounter with a hydrated Monk seat
// AND a goblin target + an always-hit resolver, so the Monk can take a real
// attack and exercise the two-level economy.
func (s *TurnStateSuite) combatMonkEncounter() *encounter.Encounter {
	s.T().Helper()
	view := perception.NewView(monkPlayerID, core.Hex{}, 10)
	view.ApplyReveal(perception.VisibleHexesAt(core.Hex{}, 10))
	data := encounter.NewData("enc-ts")
	data.Players[monkPlayerID] = &encounter.PlayerData{
		ID:          monkPlayerID,
		EntityID:    monkEntityID,
		View:        view,
		HP:          9,
		MaxHP:       9,
		AC:          15,
		AttackBonus: 4,
		DamageDice:  "1d4",
		DamageType:  "bludgeoning",
		DataJSON:    s.monkCharJSON(),
	}
	data.Monsters["goblin-1"] = &encounter.MonsterData{
		ID:       "goblin-1",
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       7,
		MaxHP:    7,
		AC:       12,
	}
	enc, err := encounter.LoadFromData(s.ctx, data, s.broker,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 3, damageType: "bludgeoning"}),
	)
	s.Require().NoError(err)
	return enc
}

// TestGoalBehavior_MonkTakesActionThenBonusAction is the Beat-1 DONE-BAR proof:
// a Monk takes its Attack (a standard action) and then a Martial Arts bonus
// unarmed strike (a bonus action), with the economy enforced server-side —
// BOTH slots decrement. This exercises the two-level economy end-to-end:
// Attack grants + a strike consumes, the strike grants the Monk bonus, and the
// bonus strike (taken through the SAME unified TakeAction verb) consumes the
// bonus action.
func (s *TurnStateSuite) TestGoalBehavior_MonkTakesActionThenBonusAction() {
	enc := s.combatMonkEncounter()
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != monkEntityID {
		_, _, err := enc.EndTurn(s.ctx, enc.ActiveActor())
		s.Require().NoError(err)
	}

	s.Require().Equal(1, enc.ActorTurnState(monkEntityID).Economy.ActionsRemaining)
	s.Require().Equal(1, enc.ActorTurnState(monkEntityID).Economy.BonusActionsRemaining)

	// 1) Take the Attack action against the goblin.
	err := enc.TakeAction(monkPlayerID,
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: "goblin-1"},
	)
	s.Require().NoError(err)

	afterAttack := enc.ActorTurnState(monkEntityID)
	s.Equal(0, afterAttack.Economy.ActionsRemaining, "Attack consumed the standard action")

	// The Monk's main-hand strike granted the Martial Arts bonus strike, which
	// now appears in the action menu as a bonus-action option.
	var bonusStrike *dnd5eCharacter.AvailableAction
	for i := range afterAttack.Actions {
		if afterAttack.Actions[i].Ref.ID == refs.Actions.UnarmedStrike().ID {
			bonusStrike = &afterAttack.Actions[i]
		}
	}
	s.Require().NotNil(bonusStrike, "Monk Attack grants a Martial Arts bonus unarmed strike")
	s.Equal(dnd5eCharacter.EconomySlotBonusAction, bonusStrike.EconomySlot)

	// 2) Take the bonus unarmed strike through the SAME unified verb.
	err = enc.TakeAction(monkPlayerID,
		encounter.ActionRef{Module: "dnd5e", Type: bonusStrike.Ref.Type, ID: bonusStrike.Ref.ID},
		encounter.ActionTarget{EntityID: "goblin-1"},
	)
	s.Require().NoError(err)

	afterBonus := enc.ActorTurnState(monkEntityID)
	s.Equal(0, afterBonus.Economy.BonusActionsRemaining, "the bonus unarmed strike consumed the bonus action")
}

// TestTakeAction_RejectsDeferredMove proves the menu↔verb consistency for a
// beat-deferred ref (D17): the menu surfaces Move as available=false, and the
// verb rejects an attempt to take it with ErrActionDeferred — it does not
// no-op-succeed. Beat 2 removes Move from the deferred set and wires real
// movement; until then the server never promises what it won't do.
func (s *TurnStateSuite) TestTakeAction_RejectsDeferredMove() {
	enc := s.loadedMonkEncounter()
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))

	err := enc.TakeAction(monkPlayerID,
		encounter.ActionRef{
			Module: "dnd5e",
			Type:   refs.Actions.Move().Type,
			ID:     refs.Actions.Move().ID,
		},
		encounter.ActionTarget{},
	)
	s.ErrorIs(err, encounter.ErrActionDeferred)
}

// TestTakeAction_AttackRejectedWhenActionSpent proves the attack verb is gated
// on the economy server-side (the done-bar: economy enforced). After the Monk
// spends its standard action on Dodge, an Attack (which costs a standard action)
// is rejected with ErrActionUnaffordable and deals NO damage — the resolver
// never runs. This is the structural backstop behind the menu's available=false
// (Inv 11/12): the verb never resolves an unaffordable action.
func (s *TurnStateSuite) TestTakeAction_AttackRejectedWhenActionSpent() {
	enc := s.combatMonkEncounter()
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != monkEntityID {
		_, _, err := enc.EndTurn(s.ctx, enc.ActiveActor())
		s.Require().NoError(err)
	}

	// Spend the standard action on Dodge.
	s.Require().NoError(enc.TakeAction(monkPlayerID,
		encounter.ActionRef{Module: "dnd5e", Type: refs.CombatAbilities.Dodge().Type, ID: refs.CombatAbilities.Dodge().ID},
		encounter.ActionTarget{EntityID: monkEntityID},
	))
	s.Require().Equal(0, enc.ActorTurnState(monkEntityID).Economy.ActionsRemaining)

	goblinHPBefore := enc.ToData().Monsters["goblin-1"].HP

	// Attempt an Attack with no standard action left — must be rejected, no damage.
	err := enc.TakeAction(monkPlayerID,
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: "goblin-1"},
	)
	s.ErrorIs(err, encounter.ErrActionUnaffordable,
		"attack with no action left must be rejected by the economy gate")

	goblinHPAfter := enc.ToData().Monsters["goblin-1"].HP
	s.Equal(goblinHPBefore, goblinHPAfter,
		"a rejected attack must not resolve damage — the resolver never ran")
}

// TestTakeAction_RejectsUnknownRefOnHydratedCharacter proves that an unknown
// ref on a HYDRATED character (one with a real menu) is rejected with
// ErrUnsupportedAction — the character's engine reports it has no such
// ability/action. This is the unknown-ref path the old hard gate used to cover;
// the general delegation preserves it for refs the menu doesn't contain.
func (s *TurnStateSuite) TestTakeAction_RejectsUnknownRefOnHydratedCharacter() {
	enc := s.loadedMonkEncounter()
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))

	err := enc.TakeAction(monkPlayerID,
		encounter.ActionRef{Module: "dnd5e", Type: "actions", ID: "no-such-action"},
		encounter.ActionTarget{EntityID: monkEntityID},
	)
	s.ErrorIs(err, encounter.ErrUnsupportedAction)
}

// TestTakeAction_HelpAndHideFlowThroughDelegation proves the Help and Hide
// combat abilities (the remaining two of the universal catalog, landed in the
// dnd5e half rpg-toolkit#702 / v0.61.0) reach the held character's rules engine
// through the SAME unified verb — no per-ref code in the encounter — with the
// right target-kind exposed on the menu and the standard action enforced.
// Like Dodge, Beat-1 proves dispatch generality + economy spend, not the full
// mechanical effect (Help's advantage / Hide's Stealth check are later beats).
func (s *TurnStateSuite) TestTakeAction_HelpAndHideFlowThroughDelegation() {
	enc := s.loadedMonkEncounter()
	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	s.Require().Equal(monkEntityID, enc.ActiveActor())

	// The menu carries Help (single-entity) and Hide (self) with the action slot.
	ts := enc.ActorTurnState(monkEntityID)
	byID := map[string]dnd5eCharacter.AvailableAbility{}
	for _, a := range ts.Abilities {
		byID[a.Ref.ID] = a
	}
	s.Require().Contains(byID, refs.CombatAbilities.Help().ID, "Help must be in the menu")
	s.Require().Contains(byID, refs.CombatAbilities.Hide().ID, "Hide must be in the menu")
	s.Equal(dnd5eCharacter.TargetKindSingleEntity, byID[refs.CombatAbilities.Help().ID].TargetKind)
	s.Equal(dnd5eCharacter.EconomySlotAction, byID[refs.CombatAbilities.Help().ID].EconomySlot)
	s.Equal(dnd5eCharacter.TargetKindSelf, byID[refs.CombatAbilities.Hide().ID].TargetKind)
	s.Equal(dnd5eCharacter.EconomySlotAction, byID[refs.CombatAbilities.Hide().ID].EconomySlot)

	// Take Hide through the unified verb — spends the standard action.
	err := enc.TakeAction(monkPlayerID,
		encounter.ActionRef{Module: "dnd5e", Type: refs.CombatAbilities.Hide().Type, ID: refs.CombatAbilities.Hide().ID},
		encounter.ActionTarget{EntityID: monkEntityID},
	)
	s.Require().NoError(err, "Hide must flow through the general delegation")
	s.Equal(0, enc.ActorTurnState(monkEntityID).Economy.ActionsRemaining, "Hide spends the standard action")

	// With the action spent, Help is still LISTED in the menu but unavailable
	// (no action left) — the menu pre-empts the illegal action rather than
	// dropping the entry.
	afterByID := map[string]dnd5eCharacter.AvailableAbility{}
	for _, a := range enc.ActorTurnState(monkEntityID).Abilities {
		afterByID[a.Ref.ID] = a
	}
	s.Require().Contains(afterByID, refs.CombatAbilities.Help().ID,
		"Help stays listed after the action is spent (pre-empted, not dropped)")
	help := afterByID[refs.CombatAbilities.Help().ID]
	s.False(help.CanUse, "Help is unavailable once the action is spent")
	s.NotEmpty(help.Reason, "unavailable Help carries a reason")
}

// silence unused import if coreCombat ends up unreferenced in future edits.
var _ = coreCombat.ActionStandard
