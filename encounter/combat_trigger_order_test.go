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
//
// rpg-toolkit#864 rebase: OpenDoor now requires adjacency, so the player
// first walks to lineHex(2) (one hex from the door at lineHex(3)) before
// opening it — the original single Move from lineHex(0) through the doorway
// is split into that approach plus the same walk-through, unchanged in
// substance (still ends at lineHex(5), still the call that triggers combat).
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

	// rpg-toolkit#864: OpenDoor requires adjacency -- walk up to lineHex(2)
	// (one hex from the door) first. The door is still closed, so this
	// approach must not reveal or trigger anything.
	s.Require().NoError(enc.Move(triggerHeroPlayerID, []core.Hex{lineHex(1), lineHex(2)}))
	s.Require().NoError(enc.OpenDoor(triggerHeroPlayerID, "door-1"))
	s.Require().Equal(core.ModeFreeRoam, enc.Mode(),
		"opening the door alone must not start combat -- encounter.OpenDoor never calls "+
			"checkCombatEntry; only the player's subsequent move through the doorway does")

	s.Require().NoError(enc.Move(triggerHeroPlayerID,
		[]core.Hex{lineHex(3), lineHex(4), lineHex(5)}))

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

// --- PR #867 review follow-up: trigger MISATTRIBUTION (gate finding 1) ---
//
// checkCombatEntry evaluates a GLOBAL predicate (any player x monster LoS
// pair), but the caller only ever supplies ITS OWN candidate entity (Move's
// mover). Before this fix, checkCombatEntry blindly forwarded that candidate
// as the trigger the moment ANY pair matched -- even when the matching pair
// had nothing to do with the caller. AddPlayer never itself calls
// checkCombatEntry (only AddMonster/Move do), so a player seat added already
// within an existing monster's LoS leaves the encounter sitting in FREE_ROAM,
// unclaimed, until some OTHER player's action happens to re-evaluate the
// check -- and that unrelated player would wrongly seize initiative slot 0
// (and have its own unrelated movement wrongly docked) instead of the
// player whose LoS actually fired the transition.

const (
	attribScoutPlayerID    = core.PlayerID("scout")
	attribScoutEntityID    = core.EntityID("char-scout")
	attribWandererPlayerID = core.PlayerID("wanderer")
	attribWandererEntityID = core.EntityID("char-wanderer")
	attribMonsterID        = core.EntityID("goblin-attrib")
)

// TestCheckCombatEntry_TriggerAttributedToActualLoSMatch_NotUnrelatedMover
// reproduces the review's exact shape: the goblin is added first (zero
// players -- no combat-entry check possible), then scout is added ALREADY
// within its LoS (AddPlayer never checks combat entry, so mode stays
// FREE_ROAM even though the pair is already formed), then wanderer -- who
// can never see the goblin -- is added far away. wanderer's own unrelated
// move (40ft, over its own 30ft speed) is what re-fires checkCombatEntry.
// scout, not wanderer, must be forced into slot 0 with a genuinely full
// budget; wanderer's own budget must not be docked for a walk that never
// triggered anything on its behalf.
func (s *CombatTriggerOrderSuite) TestCheckCombatEntry_TriggerAttributedToActualLoSMatch_NotUnrelatedMover() {
	enc := encounter.New(s.ctx, "enc-attrib-1", s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
	)
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: attribMonsterID, Position: core.Hex{Q: 0, R: 0, S: 0}, HP: 7, MaxHP: 7,
	}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: attribScoutPlayerID, EntityID: attribScoutEntityID,
		// Already within the goblin's LoS the moment this seat is added.
		Position: core.Hex{Q: 1, R: 0, S: -1}, SightRange: 5,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: dice1d6, DamageType: damageSlashing,
		DataJSON: hydratedFighterJSON(s.T(), attribScoutEntityID, attribScoutPlayerID),
	}))
	s.Require().Equal(core.ModeFreeRoam, enc.Mode(),
		"test premise: AddPlayer never re-checks combat entry, so an already-visible "+
			"scout leaves the encounter in FREE_ROAM until something else re-evaluates it")

	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: attribWandererPlayerID, EntityID: attribWandererEntityID,
		// Far from the goblin -- never has LoS to it, in this move or any other.
		Position: core.Hex{Q: -20, R: 20, S: 0}, SightRange: 5,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: dice1d6, DamageType: damageSlashing,
		DataJSON: hydratedFighterJSON(s.T(), attribWandererEntityID, attribWandererPlayerID),
	}))
	s.Require().Equal(core.ModeFreeRoam, enc.Mode(), "test premise: still nobody has re-checked")

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker, encounter.WithRoller(fixedMaxRoller{}))
	s.Require().NoError(err)

	// wanderer's own unrelated move (8 contiguous hexes = 40ft, over its own
	// 30ft speed) is what re-fires checkCombatEntry; wanderer itself never
	// gains LoS to the goblin.
	s.Require().NoError(loaded.Move(attribWandererPlayerID, []core.Hex{
		{Q: -19, R: 19, S: 0}, {Q: -18, R: 18, S: 0}, {Q: -17, R: 17, S: 0}, {Q: -16, R: 16, S: 0},
		{Q: -15, R: 15, S: 0}, {Q: -14, R: 14, S: 0}, {Q: -13, R: 13, S: 0}, {Q: -12, R: 12, S: 0},
	}))
	s.Require().Equal(core.ModeTurnBased, loaded.Mode(),
		"wanderer's move re-evaluates the stale LoS pair and starts combat")

	s.Require().Equal(attribScoutEntityID, loaded.ActiveActor(),
		"scout, whose own LoS actually makes checkCombatEntry fire, must be the forced trigger -- "+
			"not wanderer, who merely happened to be the caller")

	scoutState := loaded.ActorTurnState(attribScoutEntityID)
	s.Require().NotNil(scoutState.Economy)
	s.Equal(30, scoutState.Economy.MovementRemaining,
		"scout never moved this call -- its forced-first turn must seed a genuinely full budget, "+
			"not a deduction that belongs to wanderer's own unrelated walk")

	// Cycle forward to wanderer's own turn and confirm its budget was never
	// docked for a walk that didn't actually trigger anything on its behalf.
	_, _, err = loaded.EndTurn(s.ctx, attribScoutEntityID)
	s.Require().NoError(err)
	for i := 0; loaded.ActiveActor() != attribWandererEntityID && i < 8; i++ {
		_, _, err = loaded.EndTurn(s.ctx, loaded.ActiveActor())
		s.Require().NoError(err)
	}
	s.Require().Equal(attribWandererEntityID, loaded.ActiveActor(), "setup must reach wanderer's turn")
	wandererState := loaded.ActorTurnState(attribWandererEntityID)
	s.Require().NotNil(wandererState.Economy)
	s.Equal(30, wandererState.Economy.MovementRemaining,
		"wanderer's own unrelated 40ft walk must not be docked from its budget -- it was never the trigger")
}

// --- PR #867 review follow-up: pending pre-spent survives an unhydrated
// transition (gate finding 2) ---

// TestSeedActiveActorIfUnseeded_HonorsPendingTriggerPreSpent covers
// rpg-toolkit#757's LoadFromData catch-up path (seedActiveActorIfUnseeded):
// a Move-triggered transition can fire on an encounter where the trigger has
// no held character yet (production's New()+AddPlayer+AddMonster flow --
// nothing is hydrated until a LoadFromData round-trip). seedActorTurn's
// immediate call inside setMode no-ops in that case (no character to seed),
// so the pre-spent deduction must be PERSISTED on e.data (not just a Go-only
// local) for the later LoadFromData catch-up to still apply it -- otherwise
// the catch-up hands the trigger a full, un-deducted budget.
func (s *CombatTriggerOrderSuite) TestSeedActiveActorIfUnseeded_HonorsPendingTriggerPreSpent() {
	enc := encounter.New(s.ctx, "enc-attrib-2", s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: budgetPlayerID, EntityID: budgetEntityID,
		Position: core.Hex{Q: -10, R: 10, S: 0}, SightRange: 8,
		HP: 12, MaxHP: 12, AC: 16, AttackBonus: 4,
		DamageDice: dice1d6, DamageType: damageSlashing,
		DataJSON: hydratedFighterJSON(s.T(), budgetEntityID, budgetPlayerID),
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: budgetMonster, Position: core.Hex{Q: 0, R: 0, S: 0}, HP: 7, MaxHP: 7,
	}))
	s.Require().Equal(core.ModeFreeRoam, enc.Mode())

	// enc is New()-only -- NOT yet hydrated (only LoadFromData hydrates), so
	// this Move-triggered transition fires with no held character for the
	// trigger: seedActorTurn's immediate call no-ops, and the 20ft pre-spent
	// deduction can only be honored later, by the LoadFromData catch-up.
	s.Require().NoError(enc.Move(budgetPlayerID, []core.Hex{
		{Q: -9, R: 9, S: 0}, {Q: -8, R: 8, S: 0}, {Q: -7, R: 7, S: 0}, {Q: -6, R: 6, S: 0},
	}))
	s.Require().Equal(core.ModeTurnBased, enc.Mode())
	s.Require().Equal(budgetEntityID, enc.ActiveActor())

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker, encounter.WithRoller(fixedMaxRoller{}))
	s.Require().NoError(err)

	state := loaded.ActorTurnState(budgetEntityID)
	s.Require().NotNil(state.Economy)
	s.Equal(10, state.Economy.MovementRemaining,
		"the LoadFromData catch-up must honor the trigger's pre-spent 20ft, not reseed a full 30ft")
}

// --- PR #867 gate re-check: PendingTriggerSeed staleness ---

// TestSeedActiveActorIfUnseeded_DoesNotApplyStalePendingFromEarlierTransition
// covers the gate's re-check finding: Data.PendingTriggerSeed was written
// only when preSpent > 0 (setMode, combat.go), and NOTHING cleared it --
// not the ModeFreeRoam branch, not a later TurnBased transition that has
// nothing pending for the same actor. Reproduced sequence: scout
// (unhydrated) walks 20ft into the goblin's LoS, triggering combat and
// stashing {scout, 20ft} (transition 1 -- scout has no held character yet).
// The encounter exits back to FREE_ROAM (scout's position, and so its LoS,
// is untouched). A wanderer who never gains LoS to anything then takes an
// unrelated step; finding 1's fallback correctly attributes scout as the
// trigger again (transition 2) -- but this time with a genuine pre-spent of
// 0, since scout itself didn't move on this call. The stale {scout, 20ft}
// stash from transition 1 must not survive to be misapplied to transition
// 2's forced-first turn: scout must seed a full 30ft, not 30-20=10.
func (s *CombatTriggerOrderSuite) TestSeedActiveActorIfUnseeded_DoesNotApplyStalePendingFromEarlierTransition() {
	enc := encounter.New(s.ctx, "enc-attrib-3", s.broker,
		encounter.WithRoller(fixedMaxRoller{}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: budgetPlayerID, EntityID: budgetEntityID,
		Position: core.Hex{Q: -10, R: 10, S: 0}, SightRange: 8,
		HP: 12, MaxHP: 12, AC: 16, AttackBonus: 4,
		DamageDice: dice1d6, DamageType: damageSlashing,
		DataJSON: hydratedFighterJSON(s.T(), budgetEntityID, budgetPlayerID),
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: budgetMonster, Position: core.Hex{Q: 0, R: 0, S: 0}, HP: 7, MaxHP: 7,
	}))
	s.Require().Equal(core.ModeFreeRoam, enc.Mode())

	// Transition 1: scout (still unhydrated) walks 20ft into the goblin's
	// LoS. Stashes {budgetEntityID, 20ft} since heldCharacter is nil.
	s.Require().NoError(enc.Move(budgetPlayerID, []core.Hex{
		{Q: -9, R: 9, S: 0}, {Q: -8, R: 8, S: 0}, {Q: -7, R: 7, S: 0}, {Q: -6, R: 6, S: 0},
	}))
	s.Require().Equal(core.ModeTurnBased, enc.Mode())
	s.Require().Equal(budgetEntityID, enc.ActiveActor())

	// Exit back to FREE_ROAM (simulating a cleared pocket) -- scout's
	// position, and therefore its LoS to the goblin, is untouched.
	s.Require().NoError(enc.SetMode(core.ModeFreeRoam))
	s.Require().Equal(core.ModeFreeRoam, enc.Mode())

	// A wanderer with zero LoS to anything takes an unrelated step. Scout
	// still has LoS to the goblin (never moved away), so this re-fires
	// checkCombatEntry and (correctly, per finding 1) attributes SCOUT as
	// the trigger again -- transition 2 -- but with a genuine pre-spent of
	// 0, since scout didn't move on this call.
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: attribWandererPlayerID, EntityID: attribWandererEntityID,
		Position: core.Hex{Q: -20, R: 20, S: 0}, SightRange: 5,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: dice1d6, DamageType: damageSlashing,
		DataJSON: hydratedFighterJSON(s.T(), attribWandererEntityID, attribWandererPlayerID),
	}))
	s.Require().NoError(enc.Move(attribWandererPlayerID, []core.Hex{
		{Q: -19, R: 19, S: 0},
	}))
	s.Require().Equal(core.ModeTurnBased, enc.Mode())
	s.Require().Equal(budgetEntityID, enc.ActiveActor(),
		"scout, still the only entity with LoS to the goblin, must be the trigger again")

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker, encounter.WithRoller(fixedMaxRoller{}))
	s.Require().NoError(err)

	state := loaded.ActorTurnState(budgetEntityID)
	s.Require().NotNil(state.Economy)
	s.Equal(30, state.Economy.MovementRemaining,
		"transition 2's genuine pre-spent is 0 (scout didn't move this call) -- the LoadFromData "+
			"catch-up must not misapply the stale 20ft stash left over from transition 1")
}
