package encounter_test

// rpg-toolkit#733 — design intent (rpg-project#75 Beat 2 mechanical-effects
// design doc, "Turn-start is dead code"): "ActionEconomy for an unconscious
// actor should not offer actions." seedActorTurn (turn_economy.go) used to
// unconditionally call char.StartTurn on any player whose turn began,
// including a downed one — reseeding a fresh 1/1/1/30 economy for a player
// who cannot act. This file proves the fix: a downed player's own turn-start
// zeroes the economy (via char.EndTurn) instead of reseeding it, and the
// pushed TurnStateChangedEvent reflects that.

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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

const (
	tedDownedPlayerID = core.PlayerID("ted-downed")
	tedDownedEntityID = core.EntityID("char-ted-downed")
	tedBuddyPlayerID  = core.PlayerID("ted-buddy")
	tedBuddyEntityID  = core.EntityID("char-ted-buddy")
)

// TurnEconomyDownedSuite proves the seedActorTurn fix.
type TurnEconomyDownedSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
}

func TestTurnEconomyDownedSuite(t *testing.T) {
	suite.Run(t, new(TurnEconomyDownedSuite))
}

func (s *TurnEconomyDownedSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *TurnEconomyDownedSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// charDataJSONWithSeededEconomy builds a hydratable dnd5e character.Data blob
// at 1 HP (alive, about to be knocked down by real combat) carrying a
// PRE-SEEDED ActionEconomy (1/1/1/30, TurnNumber 1) — simulating a character
// already mid-combat (a real StartTurn already ran). This makes the
// regression meaningful: pre-fix, seedActorTurn would call char.StartTurn and
// RESEED 1/1/1/30 (visibly nonzero) at the downed player's next turn start;
// post-fix, char.EndTurn zeroes it instead.
//
// rpg-toolkit#785 (arcade recovery): this used to synthesize HitPoints:0
// directly into the seat's DataJSON at AddPlayer time. Encounter.AddPlayer
// now restores any 0-HP-or-less incoming DataJSON to full HP (a brand-new
// seat IS a brand-new encounter, per #785's decision) — a synthesized-dead
// fixture would be silently revived before the test even ran, which is
// correct production behavior but breaks the "already downed" premise this
// suite needs. The caller must knock this character down via a REAL combat
// hit (NPCAct) within the loaded encounter, matching
// TestDownedPlayerViaRealCombat_StaysZeroedAcrossMultipleTurns' established
// pattern, not synthesize the down state here.
func (s *TurnEconomyDownedSuite) charDataJSONWithSeededEconomy(id, playerID string) json.RawMessage {
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
		MaxHitPoints: 20,
		ArmorClass:   15,
		ActionEconomy: &dnd5eCharacter.ActionEconomyData{
			TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1,
			ReactionsRemaining: 1, MovementRemaining: 30,
		},
	}
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	return raw
}

// buddyCharDataJSON builds a plain hydratable Fighter with no special state —
// just needs to be a valid seat so the encounter has >1 combatant to cycle
// through.
func (s *TurnEconomyDownedSuite) buddyCharDataJSON(id, playerID string) json.RawMessage {
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
		HitPoints:    20,
		MaxHitPoints: 20,
		ArmorClass:   15,
	}
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	return raw
}

// TestDownedPlayerTurnStart_ZeroesEconomy_NotReseeds is the goal-behavior
// proof: at the downed player's own turn start, the pushed
// TurnStateChangedEvent shows ZERO actions/bonus/reactions/movement — not
// the normal freshly-seeded 1/1/1/30 values.
//
// rpg-toolkit#785: ted is seated ALIVE (1 HP) and knocked down by a REAL
// goblin hit within this encounter, not synthesized at 0 HP via AddPlayer —
// see charDataJSONWithSeededEconomy's doc for why AddPlayer can no longer be
// used to fabricate an already-dead seat.
func (s *TurnEconomyDownedSuite) TestDownedPlayerTurnStart_ZeroesEconomy_NotReseeds() {
	downedData := s.charDataJSONWithSeededEconomy(string(tedDownedEntityID), string(tedDownedPlayerID))

	encID := core.EncounterID("enc-ted-1")
	roller := encounter.WithRoller(alwaysFailDeathSaveRoller{})
	resolver := encounter.WithStrikeResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing})

	enc := encounter.New(s.ctx, encID, s.broker, roller, resolver)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: tedDownedPlayerID, EntityID: tedDownedEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 1, MaxHP: 20, AC: 15, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
		DataJSON: downedData,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
		DataJSON: testGoblinDataJSON(s.T(), gobEntityID),
	}))

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker, roller, resolver)
	s.Require().NoError(err)

	sub, err := s.broker.Subscribe(encID, tedDownedPlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	// ted and the goblin are in mutual LoS, so AddMonster (above, on enc)
	// already auto-transitioned to TURN_BASED before the Data round-trip;
	// loaded inherits that mode. ted's DataJSON carries a pre-seeded
	// ActionEconomy, so this doesn't depend on SetMode's seedActorTurn call.
	for loaded.ActiveActor() != gobEntityID {
		_, _, endErr := loaded.EndTurn(s.ctx, loaded.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(sub, 100*time.Millisecond)

	// Goblin's attack drops ted (1 HP) to 0 via REAL combat damage —
	// applyUnconsciousOnZeroHP is what syncs char.hitPoints and applies
	// Unconscious, exactly the production path.
	s.Require().NoError(loaded.NPCAct(s.ctx, gobEntityID))
	drainSub(sub, 100*time.Millisecond)

	// Cycle to ted's own next turn start — the CORE #733 assertion: it must
	// zero the stale pre-seeded economy (via char.EndTurn), not reseed a
	// fresh one (via char.StartTurn).
	active := loaded.ActiveActor()
	for active != tedDownedEntityID {
		next, _, endErr := loaded.EndTurn(s.ctx, active)
		s.Require().NoError(endErr)
		active = next
	}

	seen := collectEventsTyped(sub, 500*time.Millisecond)
	var ts *events.TurnStateChangedEvent
	for _, e := range seen {
		if t, ok := e.(*events.TurnStateChangedEvent); ok {
			ts = t
		}
	}
	s.Require().NotNil(ts, "the downed player's turn start must still push a TurnStateChangedEvent")
	s.Equal(tedDownedEntityID, ts.ActorID)
	s.Equal(0, ts.State.ActionsRemaining, "a downed player's turn start must NOT reseed the standard action")
	s.Equal(0, ts.State.BonusActionsRemaining, "a downed player's turn start must NOT reseed the bonus action")
	s.Equal(0, ts.State.ReactionsRemaining, "a downed player's turn start must NOT reseed the reaction")
	s.Equal(0, ts.State.MovementRemaining, "a downed player's turn start must NOT reseed movement")
}

// aliceAliveSeededCharDataJSON builds a hydratable Fighter at 1 HP (alive,
// about to be knocked down) with a pre-seeded ActionEconomy — same shape as
// unconscious_zero_hp_test.go's charDataJSON, duplicated here so this file
// stays self-contained (no cross-suite coupling to UnconsciousZeroHPSuite).
func (s *TurnEconomyDownedSuite) aliceAliveSeededCharDataJSON() json.RawMessage {
	s.T().Helper()
	data := &dnd5eCharacter.Data{
		ID:               string(aliceEntityID),
		PlayerID:         string(alicePlayerID),
		Name:             string(aliceEntityID),
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

// TestDownedPlayerRevivedByNat20MidTurnStart_ReseedsEconomy is the regression
// proof for the Copilot review on PR #739 (turn_economy.go): seedActorTurn
// used to gate the downed/unconscious check on the encounter's PlayerData.HP
// snapshot. That snapshot is written once — when a hit first drops a player
// to 0 (combat_phased.go / npc.go) — and is never refreshed afterward; it does
// NOT track the held character's live HP. But the TurnStartTopic publish
// immediately above the check is synchronous, and can itself revive the
// actor: a natural-20 death save (UnconsciousCondition.onTurnStart) publishes
// HealingReceivedEvent, which character.onHealingReceived applies straight to
// the held character's live hitPoints. Gating on the stale PlayerData.HP
// snapshot would still read 0 and zero the just-revived actor's action
// economy for a turn they can now fully act in. This proves the fix — reading
// char.GetHitPoints() instead — reseeds a full economy for the revived actor.
func (s *TurnEconomyDownedSuite) TestDownedPlayerRevivedByNat20MidTurnStart_ReseedsEconomy() {
	encID := core.EncounterID("enc-ted-nat20")
	roller := encounter.WithRoller(fixedMaxRoller{})
	resolver := encounter.WithStrikeResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing})

	enc := encounter.New(s.ctx, encID, s.broker, roller, resolver)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 1, MaxHP: 12, AC: 10, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
		DataJSON: s.aliceAliveSeededCharDataJSON(),
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
		DataJSON: testGoblinDataJSON(s.T(), gobEntityID),
	}))

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker, roller, resolver)
	s.Require().NoError(err)

	sub, err := s.broker.Subscribe(encID, alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	// alice and the goblin are in mutual LoS, so AddMonster (above, on enc)
	// already auto-transitioned to TURN_BASED before the Data round-trip;
	// loaded inherits that mode. An explicit SetMode here would be
	// redundant and error. alice's DataJSON carries a pre-seeded
	// ActionEconomy (aliceAliveSeededCharDataJSON), so this fixture doesn't
	// depend on SetMode's seedActorTurn call to populate her economy.
	for loaded.ActiveActor() != gobEntityID {
		_, _, endErr := loaded.EndTurn(s.ctx, loaded.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(sub, 100*time.Millisecond)

	// Goblin's attack drops alice (1 HP) to 0 — applies UnconsciousCondition
	// (constructed with e.roller == fixedMaxRoller, per applyUnconsciousOnZeroHP).
	s.Require().NoError(loaded.NPCAct(s.ctx, gobEntityID))
	drainSub(sub, 100*time.Millisecond)

	// Cycle back to alice's own turn start. Her turn-start publish drives
	// UnconsciousCondition's auto-death-save roll; fixedMaxRoller guarantees a
	// nat 20, reviving her to 1 HP via HealingReceivedEvent synchronously,
	// before seedActorTurn's downed check runs.
	active := loaded.ActiveActor()
	for active != aliceEntityID {
		next, _, endErr := loaded.EndTurn(s.ctx, active)
		s.Require().NoError(endErr)
		active = next
	}

	seen := collectEventsTyped(sub, 500*time.Millisecond)
	var ts *events.TurnStateChangedEvent
	for _, e := range seen {
		if t, ok := e.(*events.TurnStateChangedEvent); ok && t.ActorID == aliceEntityID {
			ts = t
		}
	}
	s.Require().NotNil(ts, "alice's revived turn-start must still push a TurnStateChangedEvent")
	s.Equal(1, ts.State.ActionsRemaining,
		"a nat-20-revived player must get a fresh action, not a zeroed economy carried over from the stale HP snapshot")
	s.Equal(1, ts.State.BonusActionsRemaining)
	s.Equal(1, ts.State.ReactionsRemaining)
	s.Positive(ts.State.MovementRemaining)
}

// TestAlivePlayerTurnStart_StillSeedsNormalEconomy is the negative control:
// an ALIVE player's turn start is completely unaffected by this fix — still
// gets the normal freshly-seeded 1/1/1/30 economy.
func (s *TurnEconomyDownedSuite) TestAlivePlayerTurnStart_StillSeedsNormalEconomy() {
	aliveData := s.buddyCharDataJSON(string(tedBuddyEntityID), string(tedBuddyPlayerID))

	encID := core.EncounterID("enc-ted-2")
	enc := encounter.New(s.ctx, encID, s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: tedBuddyPlayerID, EntityID: tedBuddyEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 20, MaxHP: 20, AC: 15, DataJSON: aliveData,
	}))

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker)
	s.Require().NoError(err)

	sub, err := s.broker.Subscribe(encID, tedBuddyPlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	s.Require().NoError(loaded.SetMode(core.ModeTurnBased))
	s.Require().Equal(tedBuddyEntityID, loaded.ActiveActor())

	seen := collectEventsTyped(sub, 500*time.Millisecond)
	var ts *events.TurnStateChangedEvent
	for _, e := range seen {
		if t, ok := e.(*events.TurnStateChangedEvent); ok {
			ts = t
		}
	}
	s.Require().NotNil(ts)
	s.Equal(1, ts.State.ActionsRemaining)
	s.Equal(1, ts.State.BonusActionsRemaining)
	s.Equal(1, ts.State.ReactionsRemaining)
	s.Positive(ts.State.MovementRemaining)
}

// TestDownedPlayerViaRealCombat_StaysZeroedAcrossMultipleTurns is the
// rpg-toolkit#781/#784 goal-behavior proof the gate named directly: every
// OTHER test in this file knocks a player down by SYNTHESIZING
// HitPoints:0 straight into DataJSON — combat never produces that shape.
// The real production path (NPCAct, or Move-triggered opportunity
// attacks) mutates PlayerData.HP only; neither the legacy nor phased
// attack-resolution path in rulebooks/dnd5e/combat ever calls
// char.ApplyDamage on a hydrated defender, so char.GetHitPoints() stayed
// stale-full — and seedActorTurn's downed-actor gate reads
// char.GetHitPoints(), not PlayerData.HP. Pre-fix, that meant a player
// downed by REAL combat re-seeded a full 1/1/1/30 economy on every turn
// after the knockdown (not just the first), letting them keep attacking
// indefinitely — the actual shape of the reported bug ("landed a killing
// blow while unconscious"), which the file's other synthesized-HP:0
// fixtures structurally cannot reproduce.
//
// alice is knocked down by a REAL NPCAct hit (no synthesized 0 HP
// anywhere in her fixture), then checked across TWO subsequent turn
// starts (deliberately stopping at 2 death-save failures, not the 3rd/
// fatal one — a solo player's 3rd failure ends the encounter via TPK,
// rpg-toolkit#772/#782, which is a different test's concern) to prove the
// zero-economy state holds continuously, not just once.
func (s *TurnEconomyDownedSuite) TestDownedPlayerViaRealCombat_StaysZeroedAcrossMultipleTurns() {
	encID := core.EncounterID("enc-ted-realcombat")
	roller := encounter.WithRoller(alwaysFailDeathSaveRoller{})
	resolver := encounter.WithStrikeResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing})

	enc := encounter.New(s.ctx, encID, s.broker, roller, resolver)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 1, MaxHP: 12, AC: 10, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
		DataJSON: s.aliceAliveSeededCharDataJSON(),
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
		DataJSON: testGoblinDataJSON(s.T(), gobEntityID),
	}))

	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker, roller, resolver)
	s.Require().NoError(err)

	sub, err := s.broker.Subscribe(encID, alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	// alice and the goblin are in mutual LoS, so AddMonster (above, on enc)
	// already auto-transitioned to TURN_BASED before the Data round-trip;
	// loaded inherits that mode. alice's DataJSON carries a pre-seeded
	// ActionEconomy, so this doesn't depend on SetMode's seedActorTurn call.
	for loaded.ActiveActor() != gobEntityID {
		_, _, endErr := loaded.EndTurn(s.ctx, loaded.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(sub, 100*time.Millisecond)

	// Goblin's attack drops alice (1 HP) to 0 via REAL combat damage —
	// applyAndPublishNPCOutcome mutates PlayerData.HP; applyUnconsciousOnZeroHP
	// is what must sync char.hitPoints (rpg-toolkit#781 fix under test).
	s.Require().NoError(loaded.NPCAct(s.ctx, gobEntityID))
	drainSub(sub, 100*time.Millisecond)

	// checkTurnZeroedAndCantAct advances AT LEAST one full turn (do-while,
	// not a pre-check loop) until alice's own turn starts again, asserts
	// the pushed TurnStateChangedEvent shows a fully zeroed economy, and
	// confirms TakeAction is rejected as unaffordable — not silently
	// resolved. Do-while matters on the second call: after the first
	// check, alice IS already the active actor (her failed TakeAction
	// attempt didn't end her turn), so a plain "loop while not alice"
	// would do zero iterations and re-inspect the SAME turn-start instead
	// of advancing through goblin's turn to alice's NEXT one.
	checkTurnZeroedAndCantAct := func(label string) {
		s.T().Helper()
		for {
			_, _, endErr := loaded.EndTurn(s.ctx, loaded.ActiveActor())
			s.Require().NoError(endErr)
			if loaded.ActiveActor() == aliceEntityID {
				break
			}
		}
		seen := collectEventsTyped(sub, 500*time.Millisecond)
		var ts *events.TurnStateChangedEvent
		for _, e := range seen {
			if t, ok := e.(*events.TurnStateChangedEvent); ok && t.ActorID == aliceEntityID {
				ts = t
			}
		}
		s.Require().NotNil(ts, "%s: alice's turn-start must still push a TurnStateChangedEvent", label)
		s.Equal(0, ts.State.ActionsRemaining, "%s: downed player must not get a reseeded action", label)
		s.Equal(0, ts.State.BonusActionsRemaining, "%s: downed player must not get a reseeded bonus action", label)
		s.Equal(0, ts.State.ReactionsRemaining, "%s: downed player must not get a reseeded reaction", label)
		s.Equal(0, ts.State.MovementRemaining, "%s: downed player must not get reseeded movement", label)

		err := loaded.TakeAction(alicePlayerID,
			encounter.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
			encounter.ActionTarget{EntityID: gobEntityID},
		)
		s.Require().Error(err, "%s: alice must not be able to act while unconscious", label)
		s.ErrorIs(err, encounter.ErrActionUnaffordable, "%s: rejection must be economy-based", label)
	}

	checkTurnZeroedAndCantAct("next turn (1st death-save failure)")
	checkTurnZeroedAndCantAct("turn after (2nd death-save failure)")
}
