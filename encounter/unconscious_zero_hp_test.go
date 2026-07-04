package encounter_test

// rpg-toolkit#733 — players die outright at 0 HP instead of going
// unconscious and rolling death saves. This file covers the two encounter-
// level behaviors from the fix:
//
//  1. A player whose HP transitions >0 -> 0 now has UnconsciousCondition
//     applied (broker ConditionAppliedEvent, type "unconscious") instead of
//     firing EntityDiedEvent immediately — death saves gate death now.
//  2. Three failed death saves (the rulebook's own CharacterDiedTopic) are
//     bridged to the broker EntityDiedEvent via the new PERMANENT
//     subscription installed in New/LoadFromData (subscribeCharacterDiedBridge,
//     death.go), with an empty KillerID (no specific final blow).
//
// The pre-existing DeathSuite tests (death_test.go) already cover the
// fallback path unchanged: those fixtures use encounter.New with no
// PlayerInput.DataJSON, so e.heldCharacter returns nil and
// applyUnconsciousOnZeroHP falls back to the old publishPlayerDied behavior
// verbatim — proven by those tests passing unmodified against this change.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// alwaysFailDeathSaveRoller always rolls a 2 on a d20 (a plain death-save
// failure — not a natural 1 or 20) so a downed character's death saves fail
// deterministically, three turn-starts in a row, without depending on real
// randomness. Non-d20 rolls (RollN, or Roll for any other size) pass through
// at max face — irrelevant here since nothing else in these fixtures rolls
// dice (alwaysHitResolver is a canned CombatResolver stub).
type alwaysFailDeathSaveRoller struct{}

func (alwaysFailDeathSaveRoller) Roll(_ context.Context, size int) (int, error) {
	if size == 20 {
		return 2, nil
	}
	return size, nil
}

func (alwaysFailDeathSaveRoller) RollN(_ context.Context, count, size int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		out[i] = size
	}
	return out, nil
}

// UnconsciousZeroHPSuite drives the real cascade-hydrated encounter path
// (LoadFromData -> held *character.Character), same house style as
// turn_start_revival_test.go.
type UnconsciousZeroHPSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestUnconsciousZeroHPSuite(t *testing.T) {
	suite.Run(t, new(UnconsciousZeroHPSuite))
}

func (s *UnconsciousZeroHPSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *UnconsciousZeroHPSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// charDataJSON builds a minimal hydratable dnd5e character.Data blob for a
// combat-ready Fighter, mirroring turn_start_revival_test.go's helper.
func (s *UnconsciousZeroHPSuite) charDataJSON(id, playerID string) json.RawMessage {
	s.T().Helper()
	data := &dnd5eCharacter.Data{
		ID:               id,
		PlayerID:         playerID,
		Name:             id,
		Level:            3,
		ProficiencyBonus: 2,
		ClassID:          classes.Fighter,
		RaceID:           races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 14, abilities.DEX: 12, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 10,
		},
		HitPoints:    1,
		MaxHitPoints: 12,
		ArmorClass:   10,
		ActionEconomy: &dnd5eCharacter.ActionEconomyData{
			TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
			ReactionsRemaining: 1, MovementRemaining: 30,
		},
	}
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	return raw
}

// buildEncounter constructs a 1-hydrated-player (alice, HP=aliceHP) +
// 1-scripted-monster (goblin, no DataJSON) encounter wired with roller, then
// round-trips through ToData/LoadFromData so the cascade hydrates alice's
// character (mirrors turn_start_revival_test.go's loadEncounter).
func (s *UnconsciousZeroHPSuite) buildEncounter(
	encID core.EncounterID, aliceHP int, roller encounter.Option,
) *encounter.Encounter {
	s.T().Helper()
	enc := encounter.New(s.ctx, encID, s.broker, roller,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: aliceHP, MaxHP: 12, AC: 10, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
		DataJSON: s.charDataJSON(string(aliceEntityID), string(alicePlayerID)),
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker, roller,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}),
	)
	s.Require().NoError(err)
	return loaded
}

// TestPlayerHitsZeroHP_AppliesUnconscious_NotImmediateEntityDied is the goal-
// behavior proof for the fix's core change: a player hit to 0 HP by an NPC
// gets the Unconscious condition applied (broker ConditionAppliedEvent,
// ConditionRef="unconscious", labeled with the KILLER as source) instead of
// an immediate EntityDiedEvent. The player stays seated and in initiative —
// same partial-death shape Wave 2.10 established, just gated behind death
// saves now instead of firing instantly.
func (s *UnconsciousZeroHPSuite) TestPlayerHitsZeroHP_AppliesUnconscious_NotImmediateEntityDied() {
	enc := s.buildEncounter("enc-uzh-1", 1, encounter.WithRoller(fixedMaxRoller{}))

	sub, err := s.broker.Subscribe("enc-uzh-1", alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != gobEntityID {
		_, _, endErr := enc.EndTurn(s.ctx, enc.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(sub, 100*time.Millisecond)

	// Goblin's attack drops alice (1 HP) to 0 — guaranteed lethal via
	// alwaysHitResolver's 999 damage.
	s.Require().NoError(enc.NPCAct(s.ctx, gobEntityID))

	seen := collectEventsTyped(sub, 500*time.Millisecond)
	var (
		condApplied *events.ConditionAppliedEvent
		died        *events.EntityDiedEvent
	)
	for _, e := range seen {
		switch ev := e.(type) {
		case *events.ConditionAppliedEvent:
			condApplied = ev
		case *events.EntityDiedEvent:
			died = ev
		}
	}
	s.Require().NotNil(condApplied, "hitting 0 HP must apply the Unconscious condition")
	s.Equal(string(dnd5eEvents.ConditionUnconscious), condApplied.ConditionRef)
	s.Equal(core.EntityID(aliceEntityID), condApplied.TargetID,
		"condition must be labeled with the downed player, not the killer")
	s.Equal(core.EntityID(gobEntityID), condApplied.SourceID, "narration source is the killer")
	s.Nil(died, "player hitting 0 HP must NOT immediately fire EntityDiedEvent — death saves gate it now")

	persisted := enc.ToData()
	s.Contains(persisted.Players, core.PlayerID(alicePlayerID),
		"player must remain seated (Wave 2.10 partial-death shape)")
	s.Contains(persisted.Initiative, core.EntityID(aliceEntityID), "player must remain in initiative even at HP=0")
}

// TestThreeFailedDeathSaves_BridgesToEntityDied proves the missing half of
// the death-save loop: once UnconsciousCondition's own death-save state
// reaches Dead (3 failures), the rulebook's CharacterDiedTopic — which
// nothing in encounter/*.go subscribed to before this fix — now bridges to
// the broker EntityDiedEvent via the permanent subscription installed in
// New/LoadFromData (subscribeCharacterDiedBridge). KillerID is empty: there
// is no specific final blow at 3-failed-saves death.
func (s *UnconsciousZeroHPSuite) TestThreeFailedDeathSaves_BridgesToEntityDied() {
	enc := s.buildEncounter("enc-uzh-2", 1, encounter.WithRoller(alwaysFailDeathSaveRoller{}))

	aliceSub, err := s.broker.Subscribe("enc-uzh-2", alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = aliceSub.Close() }()

	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != gobEntityID {
		_, _, endErr := enc.EndTurn(s.ctx, enc.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(aliceSub, 100*time.Millisecond)

	// Goblin's attack drops alice to 0 HP — Unconscious applied, not dead.
	s.Require().NoError(enc.NPCAct(s.ctx, gobEntityID))
	drainSub(aliceSub, 100*time.Millisecond)

	bus := enc.EventBus()
	s.Require().NotNil(bus)
	var diedRulebook bool
	_, err = dnd5eEvents.CharacterDiedTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, e dnd5eEvents.CharacterDiedEvent) error {
			if e.CharacterID == string(aliceEntityID) {
				diedRulebook = true
			}
			return nil
		})
	s.Require().NoError(err)

	// Cycle turns (goblin <-> alice) until alice's own turn-start has fired
	// exactly 3 times — 3 consecutive plain death-save failures
	// (alwaysFailDeathSaveRoller never rolls a success/crit).
	aliceTurnStarts := 0
	active := enc.ActiveActor()
	for i := 0; i < 20 && aliceTurnStarts < 3; i++ {
		next, _, endErr := enc.EndTurn(s.ctx, active)
		s.Require().NoError(endErr)
		active = next
		if active == aliceEntityID {
			aliceTurnStarts++
		}
	}
	s.Require().Equal(3, aliceTurnStarts, "must reach alice's own turn 3 times to accumulate 3 failures")
	s.Require().True(diedRulebook, "3 failed death saves must publish the rulebook CharacterDiedEvent")

	seen := collectEventsTyped(aliceSub, 500*time.Millisecond)
	var died *events.EntityDiedEvent
	for _, e := range seen {
		if d, ok := e.(*events.EntityDiedEvent); ok {
			died = d
		}
	}
	s.Require().NotNil(died, "3 failed death saves must bridge to a broker EntityDiedEvent")
	s.Equal(core.EntityID(aliceEntityID), died.EntityID)
	s.Empty(died.KillerID, "final death-save death has no specific killer")

	// Wave 2.10's "player is NOT removed from initiative" call still stands —
	// this fix only wires the missing event, not removal/TPK semantics.
	persisted := enc.ToData()
	s.Contains(persisted.Players, core.PlayerID(alicePlayerID))
	s.NotEmpty(persisted.Initiative)
}
