package encounter_test

// combat_trigger_order_test.go is the regression gate for rpg-toolkit#865:
// a monster taking a full turn before the player whose action triggered
// combat (found in a 2026-07-30 QA walk — a player moved into a boss's
// detection range, and the transition's rolled initiative put the boss
// FIRST because it simply won the d20 roll).
//
// Kirk's creative-director ruling (issue body): the entity that triggers
// combat (opens the door / walks into sight) is ALWAYS initiative slot 1
// (0-indexed: slot 0) -- its own roll is unused for placement -- and no
// monster may act before that entity's turn completes. The transition must
// also preserve whatever movement budget the trigger already spent getting
// there (free-roam movement is unmetered, so nothing gated it, but it still
// happened) rather than granting a full fresh turn on top of it.
//
// checkCombatEntry (combat.go) is the ONLY place a FreeRoam->TurnBased
// transition can fire from a player's own action -- Move is the sole call
// site that has a real acting entity (a mover); AddMonster/SeedMonsters have
// no acting player (a monster simply becomes visible) and are deliberately
// left at plain roll order (documented at their own call sites, not
// re-tested here). The "door-interact-reveals-monsters" vector the issue
// also calls out turns out NOT to be a separate code path: encounter.OpenDoor
// never calls checkCombatEntry itself (confirmed by reading it and rpg-api's
// Interact orchestrator, which dispatches to OpenDoor and nothing else) --
// opening a door only rebuilds LoS-blocking geometry. Combat only actually
// starts once the player's NEXT Move call re-evaluates checkCombatEntry
// through the newly-opened doorway (pocket_clear_move_teardown_test.go and
// combat_pockets_test.go's TestOpenDoorAndSightB_StartsFreshPocket already
// exercise this shape). So the door vector's "the triggering player DID act
// before the monster" QA observation was luck of the roll, not
// correct-by-design behavior -- it funnels through the exact same Move-
// triggered checkCombatEntry path movement-into-detection does, and
// TestOpenDoorThenMove_TriggerForcedFirstRegardlessOfRoll below proves the
// fix covers it too.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

// sequenceRoller returns rolls from a fixed sequence in call order (wrapping
// if exhausted), giving tests deterministic control over each individual
// rollInitiative seed rather than fixedMaxRoller's uniform-tie behavior.
// rollInitiative's own call order is: sorted playerIDs ascending, then
// sorted engaged monster IDs ascending (combat.go's rollInitiative doc) --
// tests using this roller pick entity IDs whose sort order matches the
// sequence they hand it.
type sequenceRoller struct {
	rolls []int
	i     int
}

func (r *sequenceRoller) Roll(_ context.Context, _ int) (int, error) {
	v := r.rolls[r.i%len(r.rolls)]
	r.i++
	return v, nil
}

func (r *sequenceRoller) RollN(ctx context.Context, count, _ int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		v, err := r.Roll(ctx, 20)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

type CombatTriggerOrderSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestCombatTriggerOrderSuite(t *testing.T) {
	suite.Run(t, new(CombatTriggerOrderSuite))
}

func (s *CombatTriggerOrderSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *CombatTriggerOrderSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

const (
	triggerHeroPlayerID = core.PlayerID("hero")
	triggerHeroEntityID = core.EntityID("char-hero")
)

// TestMove_TriggerForcedFirst_RegardlessOfRoll reproduces the exact QA
// incident shape: a monster ID ("boss-boss") that sorts alphabetically
// before the player's entity ID ("char-hero"), with fixedMaxRoller tying
// every roll at 20 -- the OLD rollInitiative's tiebreak (ascending id) would
// have put the monster first, exactly matching the QA-observed
// `initiative: ['boss-boss', 'char_...']`. The player starts out of the
// monster's LoS and moves into it; that Move is the trigger.
func (s *CombatTriggerOrderSuite) TestMove_TriggerForcedFirst_RegardlessOfRoll() {
	enc := encounter.New(s.ctx, "enc-trigger-order-1", s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: triggerHeroPlayerID, EntityID: triggerHeroEntityID,
		Position: core.Hex{Q: -10, R: 10, S: 0}, SightRange: 5,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: dice1d6, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: "boss-boss", Position: core.Hex{Q: 0, R: 0, S: 0}, HP: 30, MaxHP: 30,
	}))
	s.Require().Equal(core.ModeFreeRoam, enc.Mode(),
		"test premise: the boss must be out of LoS until the move below")

	s.Require().NoError(enc.Move(triggerHeroPlayerID, []core.Hex{
		{Q: -5, R: 5, S: 0}, {Q: 0, R: 0, S: 0},
	}))

	s.Require().Equal(core.ModeTurnBased, enc.Mode())
	s.Require().Equal([]core.EntityID{triggerHeroEntityID, "boss-boss"}, enc.ToData().Initiative,
		"the moving player triggered combat and must go first, "+
			"even though the monster's id wins the old ascending-id tiebreak (rpg-toolkit#865)")
	s.Equal(triggerHeroEntityID, enc.ActiveActor())
}

// TestMove_OthersStillOrderedByRoll_TriggerRollDiscarded proves the fix is
// scoped to the trigger's OWN placement only: a second player and a monster
// not involved in triggering the transition must still sort by their own
// rolls, descending, exactly as before this issue. The trigger's own roll
// (5, the lowest of the three) is deliberately made low, "won" by nobody
// looking, and discarded for its placement -- but it must not desync the
// rolls assigned to the other two seeds.
func (s *CombatTriggerOrderSuite) TestMove_OthersStillOrderedByRoll_TriggerRollDiscarded() {
	// rollInitiative's call order: sorted playerIDs ascending, then sorted
	// engaged-monster IDs ascending. "char-a-trigger" < "char-b-other"
	// alphabetically, so the sequence below is [trigger, other, monster].
	enc := encounter.New(s.ctx, "enc-trigger-order-2", s.broker,
		encounter.WithRoller(&sequenceRoller{rolls: []int{5, 18, 10}}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "trigger", EntityID: "char-a-trigger",
		Position: core.Hex{Q: -10, R: 10, S: 0}, SightRange: 5,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: dice1d6, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "other", EntityID: "char-b-other",
		// Nowhere near the monster -- must not itself have LoS, or AddMonster
		// below would trigger combat entry before the Move under test does.
		Position: core.Hex{Q: 10, R: -10, S: 0}, SightRange: 5,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: dice1d6, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: "goblin-m", Position: core.Hex{Q: 0, R: 0, S: 0}, HP: 7, MaxHP: 7,
	}))
	s.Require().Equal(core.ModeFreeRoam, enc.Mode(), "test premise: nobody has LoS to the goblin yet")

	s.Require().NoError(enc.Move("trigger", []core.Hex{
		{Q: -5, R: 5, S: 0}, {Q: 0, R: 0, S: 0},
	}))

	s.Require().Equal(core.ModeTurnBased, enc.Mode())
	s.Equal([]core.EntityID{"char-a-trigger", "char-b-other", "goblin-m"}, enc.ToData().Initiative,
		"trigger first despite its own low roll (5); the other two still sort by "+
			"their own rolls descending (18 before 10), untouched by the fix")
}

// TestOpenDoorThenMove_TriggerForcedFirstRegardlessOfRoll covers the issue's
// second vector. OpenDoor alone must NOT start combat (asserted below) --
// the player must still walk through the doorway, and THAT Move is the
// trigger, exactly like the movement-into-detection vector. The monster id
// ("aaa-ambush") again sorts before the player's id, so the old ascending-id
// tiebreak would have put it first.
func (s *CombatTriggerOrderSuite) TestOpenDoorThenMove_TriggerForcedFirstRegardlessOfRoll() {
	enc := encounter.New(s.ctx, "enc-trigger-order-door", s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
	)
	s.Require().NoError(enc.InitRoom(20, 20, environments.PatternEmpty))
	s.Require().NoError(enc.AddDoor("door-1", lineHex(3), false))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: triggerHeroPlayerID, EntityID: triggerHeroEntityID,
		Position: lineHex(0), SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: dice1d6, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: "aaa-ambush", Position: lineHex(6), HP: 7, MaxHP: 7,
	}))
	s.Require().Equal(core.ModeFreeRoam, enc.Mode(), "test premise: monster behind the closed door")

	s.Require().NoError(enc.OpenDoor(triggerHeroPlayerID, "door-1"))
	s.Require().Equal(core.ModeFreeRoam, enc.Mode(),
		"opening the door alone must not start combat -- encounter.OpenDoor never calls "+
			"checkCombatEntry; only the player's subsequent move through the doorway does")

	s.Require().NoError(enc.Move(triggerHeroPlayerID,
		[]core.Hex{lineHex(1), lineHex(2), lineHex(3), lineHex(4), lineHex(5)}))

	s.Require().Equal(core.ModeTurnBased, enc.Mode(), "sighting the monster through the open door must start combat")
	s.Equal([]core.EntityID{triggerHeroEntityID, "aaa-ambush"}, enc.ToData().Initiative,
		"the player who walked through the door triggered combat and must go first, "+
			"even though the monster's id wins the old ascending-id tiebreak (rpg-toolkit#865)")
}

// hydratedFighterJSON builds a level-1 Human Fighter (30ft speed) character
// blob for the budget-preservation tests below -- mirrors move_economy_test.go's
// charliCharJSON / pocket_clear_move_teardown_test.go's erinCharJSON.
func hydratedFighterJSON(t *testing.T, entityID core.EntityID, playerID core.PlayerID) json.RawMessage {
	t.Helper()
	charData := &dnd5eCharacter.Data{
		ID: string(entityID), PlayerID: string(playerID),
		Name: "Budget Test Fighter", Level: 1,
		ClassID: classes.Fighter, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 12, MaxHitPoints: 12, ArmorClass: 16, ProficiencyBonus: 2,
	}
	raw, err := json.Marshal(charData)
	if err != nil {
		t.Fatalf("marshal fighter char data: %v", err)
	}
	return raw
}

const (
	budgetPlayerID = core.PlayerID("budget-hero")
	budgetEntityID = core.EntityID("char-budget-hero")
	budgetMonster  = core.EntityID("goblin-budget")
)

// loadHydratedTriggerFixture builds a FREE_ROAM encounter with one hydrated
// (LoadFromData-round-tripped, so heldCharacter/ActorTurnState see a real
// *character.Character) Human Fighter far from a monster, so the caller can
// Move the fighter into the monster's LoS and inspect the seeded economy.
func (s *CombatTriggerOrderSuite) loadHydratedTriggerFixture() *encounter.Encounter {
	s.T().Helper()
	enc := encounter.New(s.ctx, "enc-trigger-budget", s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: budgetPlayerID, EntityID: budgetEntityID,
		// 10 hexes from the goblin at origin, SightRange 8: nothing shorter
		// than a 2-hex approach brings it into view, giving both budget tests
		// below room for a clean, exact hex count.
		Position: core.Hex{Q: -10, R: 10, S: 0}, SightRange: 8,
		HP: 12, MaxHP: 12, AC: 16, AttackBonus: 4,
		DamageDice: dice1d6, DamageType: damageSlashing,
		DataJSON: hydratedFighterJSON(s.T(), budgetEntityID, budgetPlayerID),
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: budgetMonster, Position: core.Hex{Q: 0, R: 0, S: 0}, HP: 7, MaxHP: 7,
	}))
	s.Require().Equal(core.ModeFreeRoam, enc.Mode(), "test premise: monster out of LoS")

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
	)
	s.Require().NoError(err)
	return loaded
}

// TestMove_PreCombatMovementBudget_PartiallyPreserved: the fighter (30ft
// speed) walks 20ft (4 hexes) in free-roam to spot the goblin. Free-roam
// movement is unmetered (tracksMovement is only true once already
// InCombat()), so nothing gated that 20ft -- but the transition it triggers
// must still account for it: the forced-first turn's seeded budget must be
// 30-20=10ft, not a fresh 30ft on top of the 20ft already covered.
func (s *CombatTriggerOrderSuite) TestMove_PreCombatMovementBudget_PartiallyPreserved() {
	enc := s.loadHydratedTriggerFixture()

	// 4 contiguous single-hex steps from (-10,10,0) toward the origin = 4
	// hexes = 20ft, ending at (-6,6,0) -- hex-distance 6 from the goblin at
	// the origin, inside the fixture's SightRange 8.
	s.Require().NoError(enc.Move(budgetPlayerID, []core.Hex{
		{Q: -9, R: 9, S: 0}, {Q: -8, R: 8, S: 0}, {Q: -7, R: 7, S: 0}, {Q: -6, R: 6, S: 0},
	}))
	s.Require().Equal(core.ModeTurnBased, enc.Mode())
	s.Require().Equal(budgetEntityID, enc.ActiveActor(), "the mover triggered combat and must go first")

	state := enc.ActorTurnState(budgetEntityID)
	s.Require().NotNil(state.Economy)
	s.Equal(10, state.Economy.MovementRemaining,
		"forced-first turn must seed 30ft speed minus the 20ft already spent reaching the goblin")
}

// TestMove_PreCombatMovementBudget_ClampedAtZero_NotNegative: the fighter
// walks 40ft (8 hexes) -- MORE than its 30ft speed -- to spot the goblin
// (legal because free-roam movement is unmetered). The forced-first turn's
// seeded budget must floor at zero, never go negative.
func (s *CombatTriggerOrderSuite) TestMove_PreCombatMovementBudget_ClampedAtZero_NotNegative() {
	enc := s.loadHydratedTriggerFixture()

	// 8 contiguous single-hex steps from (-10,10,0) toward the origin = 8
	// hexes = 40ft, ending at (-2,2,0) -- hex-distance 2 from the goblin at
	// the origin, comfortably inside the fixture's SightRange 8.
	s.Require().NoError(enc.Move(budgetPlayerID, []core.Hex{
		{Q: -9, R: 9, S: 0}, {Q: -8, R: 8, S: 0}, {Q: -7, R: 7, S: 0}, {Q: -6, R: 6, S: 0},
		{Q: -5, R: 5, S: 0}, {Q: -4, R: 4, S: 0}, {Q: -3, R: 3, S: 0}, {Q: -2, R: 2, S: 0},
	}))
	s.Require().Equal(core.ModeTurnBased, enc.Mode())
	s.Require().Equal(budgetEntityID, enc.ActiveActor(), "the mover triggered combat and must go first")

	state := enc.ActorTurnState(budgetEntityID)
	s.Require().NotNil(state.Economy)
	s.Equal(0, state.Economy.MovementRemaining,
		"an over-speed free-roam approach must clamp the forced-first turn's budget at zero, not go negative")
}
