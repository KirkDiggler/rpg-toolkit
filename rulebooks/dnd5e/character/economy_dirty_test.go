// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// EconomyDirtyTestSuite pins the half of the round-trip that serialization
// alone does not buy: a sheet whose action economy or resource pool moved has
// to SAY so, or the write-back never happens.
//
// ToData has always serialized the economy (Data.ActionEconomy) and the pools
// (Data.Resources), but resolution.dirtyCharacters keeps only the sheets that
// report IsDirty() and session.saveDirty writes only what that returns. Before
// #1087 the flag was set at four sites, all hit points or conditions — so a
// spend performed on a loaded sheet inside a resolution was serialized
// perfectly and then dropped on the floor. Every test here fails for a sheet
// that mutates quietly.
//
// The other half is no less load-bearing: a sheet that was loaded and NOT
// touched must stay clean, or the write-back clobbers stored state with a
// re-serialization of what it just read.
type EconomyDirtyTestSuite struct {
	suite.Suite

	ctx context.Context
	bus events.EventBus
}

func TestEconomyDirtySuite(t *testing.T) {
	suite.Run(t, new(EconomyDirtyTestSuite))
}

func (s *EconomyDirtyTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// sheet is a level-3 monk mid-encounter: ki to spend, hit dice to spend, and
// no action economy yet (the encounter seeds that at the turn boundary).
func (s *EconomyDirtyTestSuite) sheet() *Data {
	return &Data{
		ID:               "economy-monk",
		PlayerID:         "economy-player",
		Name:             "Spender",
		Level:            3,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Monk,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12,
			abilities.DEX: 16,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 16,
			abilities.CHA: 8,
		},
		HitPoints:    18,
		MaxHitPoints: 24,
		ArmorClass:   16,
		Resources: map[coreResources.ResourceKey]RecoverableResourceData{
			resources.Ki:      {Current: 3, Maximum: 3, ResetType: coreResources.ResetShortRest},
			resources.HitDice: {Current: 3, Maximum: 3, ResetType: coreResources.ResetLongRest},
		},
	}
}

// loaded returns the sheet as it comes off storage: clean, with the combat
// abilities a monk brings to a turn. Load does not rebuild those (they come
// from the class, not the save), so the fixture adds them the way a caller
// would — before marking clean, so the sheet under test starts where a freshly
// loaded one does.
func (s *EconomyDirtyTestSuite) loaded() *Character {
	char, err := Load(s.ctx, s.sheet())
	s.Require().NoError(err)
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewAttack("attack-1")))
	s.Require().NoError(char.AddCombatAbility(combatabilities.NewDash("dash-1")))
	char.MarkClean()

	return char
}

// inCombat is a loaded sheet that has been given a turn, then marked clean —
// so that whatever the test does next is the only thing that could have
// dirtied it.
func (s *EconomyDirtyTestSuite) inCombat() *Character {
	char := s.loaded()
	_, err := char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	char.MarkClean()

	return char
}

// --- The no-clobber half ---

func (s *EconomyDirtyTestSuite) TestALoadedSheetIsClean() {
	char, err := Load(s.ctx, s.sheet())
	s.Require().NoError(err)

	s.False(char.IsDirty(), "a sheet that was only read back has nothing to write")
	s.Equal(3, char.GetResource(resources.Ki).Current(), "and it came back at its stored ki")
}

func (s *EconomyDirtyTestSuite) TestAttachingASheetLeavesItClean() {
	char := s.loaded()
	s.Require().NoError(Attach(s.ctx, char, s.bus))

	s.False(char.IsDirty(), "putting a sheet on a bus changes nothing about the sheet")
}

func (s *EconomyDirtyTestSuite) TestReadingTheEconomyLeavesTheSheetClean() {
	char := s.inCombat()

	s.True(char.InCombat())
	s.NotNil(char.GetActionEconomy())
	s.False(char.HasGranted(GrantedAttacks))
	s.NotEmpty(char.AvailableAbilities())
	s.NotEmpty(char.AvailableActions())

	s.False(char.IsDirty(), "asking a sheet what it can do is not a spend")
}

// --- Action economy: every write-site marks ---

func (s *EconomyDirtyTestSuite) TestStartTurnMarks() {
	char := s.loaded()

	_, err := char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)

	s.True(char.IsDirty(), "a seeded turn is economy state that has to be written down")
}

// The bridge is the site the issue names: activating a combat ability spends
// and grants inside a detached toolkit economy, and fromToolkitActionEconomy
// syncs it back onto the sheet. A mutation that lands through there must not
// escape marking.
func (s *EconomyDirtyTestSuite) TestActivatingAnAbilityMarksThroughTheBridge() {
	char := s.inCombat()
	hitPoints := char.GetHitPoints()

	out, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.CombatAbilities.Attack(),
	})
	s.Require().NoError(err)
	s.Require().True(out.Success)

	s.Equal(0, char.GetActionEconomy().ActionsRemaining, "the action slot was spent")
	s.Equal(hitPoints, char.GetHitPoints(), "and no hit point moved")
	s.True(char.IsDirty(), "an economy-only change still needs saving")
}

func (s *EconomyDirtyTestSuite) TestStrikingMarks() {
	char := s.inCombat()
	_, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: refs.CombatAbilities.Attack()})
	s.Require().NoError(err)
	char.MarkClean()

	out, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{ActionRef: refs.Actions.Strike()})
	s.Require().NoError(err)
	s.Require().True(out.Success)

	s.True(char.IsDirty(), "a spent attack is a spend")
}

func (s *EconomyDirtyTestSuite) TestARefusedStrikeLeavesTheSheetClean() {
	char := s.inCombat()

	out, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{ActionRef: refs.Actions.Strike()})
	s.Require().NoError(err)
	s.Require().False(out.Success, "no attack was granted, so there is nothing to spend")

	s.False(char.IsDirty(), "a refused action wrote nothing, so there is nothing to save")
}

func (s *EconomyDirtyTestSuite) TestTheUnarmedStrikeMarks() {
	char := s.inCombat()
	char.GrantCapacity(GrantedMartialArtsBonus, 1)
	char.MarkClean()

	out, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{ActionRef: refs.Actions.UnarmedStrike()})
	s.Require().NoError(err)
	s.Require().True(out.Success)

	s.True(char.IsDirty(), "the granted strike and the bonus action that paid for it both moved")
}

func (s *EconomyDirtyTestSuite) TestARefusedUnarmedStrikeLeavesTheSheetClean() {
	char := s.inCombat()
	char.GrantCapacity(GrantedMartialArtsBonus, 1)
	char.actionEconomy.BonusActionsRemaining = 0
	char.MarkClean()

	out, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{ActionRef: refs.Actions.UnarmedStrike()})
	s.Require().NoError(err)
	s.Require().False(out.Success)

	s.False(char.IsDirty())
}

func (s *EconomyDirtyTestSuite) TestTheOffHandStrikeMarks() {
	char := s.inCombat()
	char.GrantCapacity(GrantedOffHandStrikes, 1)
	char.MarkClean()

	out, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{ActionRef: refs.Actions.OffHandStrike()})
	s.Require().NoError(err)
	s.Require().True(out.Success)

	s.True(char.IsDirty())
}

func (s *EconomyDirtyTestSuite) TestTheFlurryStrikeMarks() {
	char := s.inCombat()
	char.GrantCapacity(GrantedFlurryStrikes, 1)
	char.MarkClean()

	out, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{ActionRef: refs.Actions.FlurryStrike()})
	s.Require().NoError(err)
	s.Require().True(out.Success)

	s.True(char.IsDirty())
}

func (s *EconomyDirtyTestSuite) TestMovingMarks() {
	char := s.inCombat()

	out, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{
		ActionRef: refs.Actions.Move(),
		Distance:  10,
	})
	s.Require().NoError(err)
	s.Require().True(out.Success)

	s.Equal(20, char.GetActionEconomy().MovementRemaining)
	s.True(char.IsDirty(), "movement spent off a persisted budget has to be written down")
}

func (s *EconomyDirtyTestSuite) TestARefusedMoveLeavesTheSheetClean() {
	char := s.inCombat()

	out, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{
		ActionRef: refs.Actions.Move(),
		Distance:  100,
	})
	s.Require().NoError(err)
	s.Require().False(out.Success, "100ft is over a 30ft budget")

	s.False(char.IsDirty())
}

func (s *EconomyDirtyTestSuite) TestGrantingCapacityMarks() {
	char := s.inCombat()

	char.GrantCapacity(GrantedAttacks, 2)

	s.True(char.IsDirty(), "granted capacity is persisted on the sheet")
}

func (s *EconomyDirtyTestSuite) TestGrantingCapacityOutOfCombatLeavesTheSheetClean() {
	char := s.loaded()

	char.GrantCapacity(GrantedAttacks, 2)

	s.False(char.IsDirty(), "there is no economy to grant into, so nothing was written")
}

func (s *EconomyDirtyTestSuite) TestEndTurnMarks() {
	char := s.inCombat()

	_, err := char.EndTurn(s.ctx, &EndTurnInput{})
	s.Require().NoError(err)

	s.True(char.IsDirty(), "a turn that ended zeroed four persisted counters")
}

func (s *EconomyDirtyTestSuite) TestExitCombatMarks() {
	char := s.inCombat()

	_, err := char.ExitCombat(s.ctx, &ExitCombatInput{})
	s.Require().NoError(err)

	s.True(char.IsDirty(), "the economy went from stored to absent, which is a change")
}

func (s *EconomyDirtyTestSuite) TestExitCombatOutOfCombatLeavesTheSheetClean() {
	char := s.loaded()

	_, err := char.ExitCombat(s.ctx, &ExitCombatInput{})
	s.Require().NoError(err)

	s.False(char.IsDirty(), "nil to nil is not a change")
}

// activateFeature is the other economy write path: it spends the slot itself
// rather than going through the bridge. The stub feature costs an action and
// touches nothing else, so the mark can only have come from that spend.
func (s *EconomyDirtyTestSuite) TestActivatingAFeatureMarksTheSlotItSpends() {
	char := s.inCombat()
	char.features = append(char.features, &stubFeature{actionType: coreCombat.ActionStandard})

	out, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: stubFeatureRef})
	s.Require().NoError(err)
	s.Require().True(out.Success)

	s.Equal(0, char.GetActionEconomy().ActionsRemaining)
	s.True(char.IsDirty())
}

// A feature whose Activate fails is rolled back to the same counters it
// started with — and the sheet stays dirty on purpose. A failed Activate may
// have moved feature state before it errored, and one redundant write is
// cheaper than one lost spend.
func (s *EconomyDirtyTestSuite) TestAFailedFeatureActivationLeavesTheSheetDirty() {
	char := s.inCombat()
	char.features = append(char.features, &stubFeature{actionType: coreCombat.ActionStandard, failActivate: true})

	out, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: stubFeatureRef})
	s.Require().NoError(err)
	s.Require().False(out.Success)

	s.Equal(1, char.GetActionEconomy().ActionsRemaining, "the slot was given back")
	s.True(char.IsDirty(), "but the sheet is not assumed to be untouched")
}

// A free action costs no slot, so activating a feature that takes one writes
// nothing to the economy — and a sheet with nothing written has nothing to
// save. (The free features that ship do change the sheet, but through other
// sites: Reckless Attack applies a condition, which marks.)
func (s *EconomyDirtyTestSuite) TestActivatingAFreeFeatureLeavesTheSheetClean() {
	char := s.inCombat()
	char.features = append(char.features, &stubFeature{actionType: coreCombat.ActionFree})
	before := *char.GetActionEconomy()

	out, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: stubFeatureRef})
	s.Require().NoError(err)
	s.Require().True(out.Success)

	s.Equal(before.ActionsRemaining, char.GetActionEconomy().ActionsRemaining)
	s.Equal(before.BonusActionsRemaining, char.GetActionEconomy().BonusActionsRemaining)
	s.Equal(before.ReactionsRemaining, char.GetActionEconomy().ReactionsRemaining)
	s.False(char.IsDirty(), "no slot moved, so there is nothing to write")
}

func (s *EconomyDirtyTestSuite) TestAnUnknownAbilityLeavesTheSheetClean() {
	char := s.inCombat()

	out, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: refs.Features.Rage()})
	s.Require().NoError(err)
	s.Require().False(out.Success)

	s.False(char.IsDirty())
}

// --- Resource pools: every write-site marks ---

func (s *EconomyDirtyTestSuite) TestSpendingAPoolMarksWithoutTouchingHitPoints() {
	char := s.loaded()
	hitPoints := char.GetHitPoints()

	s.Require().NoError(char.UseResource(resources.Ki, 1))

	s.Equal(2, char.GetResource(resources.Ki).Current())
	s.Equal(hitPoints, char.GetHitPoints(), "no hit point moved")
	s.True(char.IsDirty(), "a ki-shaped spend is the whole reason Data.Resources exists")
}

func (s *EconomyDirtyTestSuite) TestSpendingAPoolThatIsNotThereLeavesTheSheetClean() {
	char := s.loaded()

	s.Require().Error(char.UseResource(resources.RageCharges, 1))

	s.False(char.IsDirty(), "nothing was spent, so nothing needs saving")
}

func (s *EconomyDirtyTestSuite) TestSpendingMorePoolThanRemainsLeavesTheSheetClean() {
	char := s.loaded()

	s.Require().Error(char.UseResource(resources.Ki, 99))

	s.Equal(3, char.GetResource(resources.Ki).Current())
	s.False(char.IsDirty())
}

func (s *EconomyDirtyTestSuite) TestAddingAPoolMarks() {
	char := s.loaded()

	char.AddResource(resources.RageCharges, combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:      string(resources.RageCharges),
		Maximum: 2,
	}))

	s.True(char.IsDirty(), "a pool the sheet did not have is a pool ToData now writes")
}

// The sheet holds the bus but its keeper was never applied, so nothing is
// listening for the healing this publishes. What is left is the hit-dice pool
// moving — which has to mark on its own, not by leaning on the healing
// handler that happens to mark alongside it inside an encounter.
func (s *EconomyDirtyTestSuite) TestSpendingHitDiceMarksOnItsOwn() {
	char := s.loaded()
	char.bus = s.bus

	out, err := char.SpendHitDice(s.ctx, &SpendHitDiceInput{Count: 1})
	s.Require().NoError(err)
	s.Require().Equal(2, out.Remaining)

	s.True(char.IsDirty(), "the hit-dice pool moved")
}

func (s *EconomyDirtyTestSuite) TestAShortRestMarks() {
	char := s.loaded()
	char.bus = s.bus
	s.Require().NoError(char.UseResource(resources.Ki, 2))
	char.MarkClean()

	s.Require().NoError(char.ShortRest(s.ctx))

	s.Equal(3, char.GetResource(resources.Ki).Current())
	s.True(char.IsDirty(), "restored pools are as persisted as spent ones")
}

func (s *EconomyDirtyTestSuite) TestALongRestMarks() {
	char := s.loaded()
	char.bus = s.bus

	s.Require().NoError(char.LongRest(s.ctx))

	s.Equal(char.GetMaxHitPoints(), char.GetHitPoints())
	s.True(char.IsDirty())
}

// --- The four sites that already marked still mark ---

func (s *EconomyDirtyTestSuite) TestDamageStillMarks() {
	char := s.loaded()

	char.ApplyDamage(s.ctx, &combat.ApplyDamageInput{
		Instances: []combat.DamageInstance{{Amount: 3}},
	})

	s.True(char.IsDirty(), "the hit-point site is unchanged by #1087")
}

// The condition and healing sites keep their own pins in sheet_keeper_test.go.

// --- End to end ---

// The point of the whole slice: a spend on a loaded sheet is both reported as
// needing a save and present in what gets saved.
func (s *EconomyDirtyTestSuite) TestASpendOnALoadedSheetSurvivesToData() {
	char := s.inCombat()
	s.Require().NoError(char.UseResource(resources.Ki, 1))

	out, err := char.ExecuteAction(s.ctx, &ExecuteActionInput{ActionRef: refs.Actions.Move(), Distance: 15})
	s.Require().NoError(err)
	s.Require().True(out.Success)

	s.Require().True(char.IsDirty(), "so resolution keeps it in DirtyCharacters")

	data := char.ToData()
	s.Require().NotNil(data.ActionEconomy)
	s.Equal(15, data.ActionEconomy.MovementRemaining, "and this is what gets written")
	s.Equal(2, data.Resources[resources.Ki].Current)
}

// --- Stub feature ---

var stubFeatureRef = &core.Ref{Module: "dnd5e", Type: "features", ID: "stub-economy-feature"}

// stubFeature costs an action economy slot and does nothing else, so a test
// using it can attribute a dirty sheet to the slot and to nothing else.
type stubFeature struct {
	actionType   coreCombat.ActionType
	failActivate bool
}

func (f *stubFeature) GetID() string            { return stubFeatureRef.ID }
func (f *stubFeature) GetType() core.EntityType { return features.EntityTypeFeature }
func (f *stubFeature) Ref() *core.Ref           { return stubFeatureRef }
func (f *stubFeature) Name() string             { return "Stub" }

func (f *stubFeature) ActionType() coreCombat.ActionType { return f.actionType }

func (f *stubFeature) CanActivate(_ context.Context, _ core.Entity, _ features.FeatureInput) error {
	return nil
}

func (f *stubFeature) Activate(_ context.Context, _ core.Entity, _ features.FeatureInput) error {
	if f.failActivate {
		return rpgerr.New(rpgerr.CodeInternal, "stub feature refuses to activate")
	}

	return nil
}

func (f *stubFeature) ToJSON() (json.RawMessage, error) {
	return json.Marshal(map[string]any{"ref": stubFeatureRef})
}
