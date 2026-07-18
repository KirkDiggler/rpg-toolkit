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
	dnd5eConditions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
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

// alwaysSuccessDeathSaveRoller always rolls a 15 on a d20 (a plain death-save
// success — not a natural 20, which would instead regain consciousness) so a
// downed character's death saves succeed deterministically, three turn-starts
// in a row, stabilizing rather than reviving or dying. Non-d20 rolls (RollN,
// or Roll for any other size) pass through at max face — irrelevant here
// since nothing else in these fixtures rolls dice via RollN (same
// convention as alwaysFailDeathSaveRoller above).
type alwaysSuccessDeathSaveRoller struct{}

func (alwaysSuccessDeathSaveRoller) Roll(_ context.Context, size int) (int, error) {
	if size == 20 {
		return 15, nil
	}
	return size, nil
}

func (alwaysSuccessDeathSaveRoller) RollN(_ context.Context, count, size int) ([]int, error) {
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

// buildEncounter constructs a 1-hydrated-player (alice, HP=1 — always
// guaranteed lethal via alwaysHitResolver's 999 damage) + 1-scripted-monster
// (goblin, no DataJSON) encounter wired with roller, then round-trips
// through ToData/LoadFromData so the cascade hydrates alice's character
// (mirrors turn_start_revival_test.go's loadEncounter). alice's starting HP
// is not a caller-varying concern across this file's tests — every one of
// them needs the same guaranteed-knockdown setup.
func (s *UnconsciousZeroHPSuite) buildEncounter(
	encID core.EncounterID, roller encounter.Option,
) *encounter.Encounter {
	s.T().Helper()
	enc := encounter.New(s.ctx, encID, s.broker, roller,
		encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing}),
	)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: alicePlayerID, EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 1, MaxHP: 12, AC: 10, AttackBonus: 4,
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
	enc := s.buildEncounter("enc-uzh-1", encounter.WithRoller(fixedMaxRoller{}))

	sub, err := s.broker.Subscribe("enc-uzh-1", alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = sub.Close() }()

	// alice and the goblin are in mutual LoS, so buildEncounter's AddMonster
	// already auto-transitioned to TURN_BASED (alice's DataJSON carries a
	// pre-seeded ActionEconomy, so this doesn't depend on SetMode's
	// seedActorTurn call); an explicit SetMode here would be redundant and
	// error.
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
	enc := s.buildEncounter("enc-uzh-2", encounter.WithRoller(alwaysFailDeathSaveRoller{}))

	aliceSub, err := s.broker.Subscribe("enc-uzh-2", alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = aliceSub.Close() }()

	// alice and the goblin are in mutual LoS, so buildEncounter's AddMonster
	// already auto-transitioned to TURN_BASED (alice's DataJSON carries a
	// pre-seeded ActionEconomy, so this doesn't depend on SetMode's
	// seedActorTurn call); an explicit SetMode here would be redundant and
	// error.
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

	// rpg-toolkit#772/#782: alice was the only player, and this death is
	// CONFIRMED (3 failed saves), not merely unconscious — the TPK predicate
	// must fire. This is the direct fix for #772 ("solo PC died, encounter
	// looped the corpse's turns forever"): Initiative must now be cleared,
	// not left non-empty (which is what this test asserted before this
	// wave, encoding the bug as expected behavior).
	var ended *events.EncounterEndedEvent
	for _, e := range seen {
		if e2, ok := e.(*events.EncounterEndedEvent); ok {
			ended = e2
		}
	}
	s.Require().NotNil(ended, "the last living player's confirmed death must publish EncounterEndedEvent")
	s.Equal(encounter.EncounterEndedReasonTPK, ended.Reason)

	s.Equal(core.ModeEnded, enc.Mode())
	persisted := enc.ToData()
	s.Contains(persisted.Players, core.PlayerID(alicePlayerID))
	s.True(persisted.Players[alicePlayerID].Dead)
	s.Empty(persisted.Initiative, "TPK must clear initiative, same as a victory ending")

	// The encounter is over — further turn dispatch must reject.
	_, _, endErr := enc.EndTurn(s.ctx, enc.ActiveActor())
	s.Require().ErrorIs(endErr, encounter.ErrEncounterEnded)
}

// TestDeathSaveRoll_SurvivesPerRPCReload_ViaRealEndTurn is the rpg-api-shape
// regression the death-wave gate asked for after a live playtest raised a
// question: does the turn-start death-save auto-roll (UnconsciousCondition.
// onTurnStart) actually fire when driven through the REAL per-RPC lifecycle
// — reload before AND after every verb call, exactly like rpg-api's
// orchestrator (load, call the verb, persist, discard; never holds an
// Encounter across RPCs) — rather than TestThreeFailedDeathSaves_
// AcrossPerRPCReloads_BridgesToEntityDied's approach of publishing raw
// DamageReceivedEvents directly on the bus to sidestep the turn-start
// roller not surviving reload (onTurnStart's own comment: reload restores
// CharacterID + death-save counters via loadJSON but not Roller, falling
// back to dice.NewRoller() — real, non-deterministic — every time).
//
// That non-determinism is exactly why NO existing test drove EndTurn
// through a reload cycle and asserted the roll itself happens — this one
// does, accepting non-determinism over the OUTCOME while asserting the
// PROCESS. Deliberately watches the broker DeathSaveRolledEvent stream
// rather than inspecting the persisted Unconscious condition's counters:
// a roll's OUTCOME can remove the condition entirely (a nat-20 regains
// consciousness) or freeze its counters permanently at a valid non-zero
// value (3 successes = stabilized, no more rolls by RAW) — inspecting
// condition state after the fact can't distinguish "never rolled" from
// "rolled and resolved," but DeathSaveRolledEvent fires unconditionally,
// before any outcome branching (unconscious.go's onTurnStart), for EVERY
// roll regardless of what it resolves to. Counting that event directly is
// what actually answers the live playtest's question.
//
// Live playtest context (rpg-toolkit#772/#781/#782): a gate review flagged
// that toolkit suites "never ride rpg-api's per-RPC reload + dirty-gated
// persist cycle" as a possible explanation for zero observed
// DeathSaveRolledEvents across seven live rounds. This test closes that
// specific coverage gap. It could not reproduce a failure here, nor across
// three independent live grpcurl+Redis reproductions against the actual
// rpg-api per-RPC lifecycle (see the PR discussion) — this test is the
// automated, CI-run backstop for what those live checks already showed.
func (s *UnconsciousZeroHPSuite) TestDeathSaveRoll_SurvivesPerRPCReload_ViaRealEndTurn() {
	// fixedMaxRoller guarantees the goblin's knockdown hit; irrelevant to
	// onTurnStart's own death-save roll (uncontrollable across reload, by
	// design here — see the doc comment above).
	roller := encounter.WithRoller(fixedMaxRoller{})
	resolver := encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing})
	opts := []encounter.Option{roller, resolver}

	enc := s.buildEncounter("enc-uzh-realendturn", roller)
	enc = s.reloadEncounter(enc, opts...)

	aliceSub, err := s.broker.Subscribe("enc-uzh-realendturn", alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = aliceSub.Close() }()

	// Cycle to the goblin's turn, reloading before AND after every EndTurn
	// call — this is the exact "load, call the verb, persist, discard"
	// shape rpg-api's orchestrator uses.
	for enc.ActiveActor() != gobEntityID {
		_, _, endErr := enc.EndTurn(s.ctx, enc.ActiveActor())
		s.Require().NoError(endErr)
		enc = s.reloadEncounter(enc, opts...)
	}

	// Goblin's attack drops alice (1 HP) to 0 — Unconscious applied via a
	// REAL NPCAct call (not a synthesized fixture), then reload. This is
	// the reload boundary between "condition applied" and "condition's
	// own subscriptions must still be live" that the live playtest
	// questioned.
	s.Require().NoError(enc.NPCAct(s.ctx, gobEntityID))
	enc = s.reloadEncounter(enc, opts...)
	drainSub(aliceSub, 100*time.Millisecond)

	// Cycle turns (goblin's own act, then alice's turn-start; repeat),
	// reloading before AND after every EndTurn call, counting
	// DeathSaveRolledEvents on the broker stream as they arrive — drained
	// incrementally each iteration rather than buffered to the end, so a
	// long run can't overflow the subscription channel. Stops as soon as
	// alice reaches ANY terminal outcome the encounter itself reports
	// (ModeEnded == death/TPK; HP > 0 == nat-20 revival), so the loop
	// never asserts past a state where RAW says no more rolls should
	// happen. Bounded at 6 of alice's own turns — generous (P(neither 3
	// successes nor 3 failures nor a nat-20 within 6 independent rolls) is
	// small) without being so large a genuinely-stuck mechanism hangs the
	// suite.
	rolledCount := 0
	aliceTurnsSeen := 0
	stabilized := false
	for aliceTurnsSeen < 6 {
		_, _, endErr := enc.EndTurn(s.ctx, enc.ActiveActor())
		s.Require().NoError(endErr)
		enc = s.reloadEncounter(enc, opts...)

		// enc.ActiveActor() here is the actor whose turn just STARTED —
		// seedActorTurn (and thus onTurnStart, and thus any death-save
		// roll) already ran, inside the EndTurn call above, for THIS
		// actor before it returned.
		newActive := enc.ActiveActor()
		// 500ms, matching this suite's established drain-window convention
		// elsewhere (not 200ms): Broker.Publish only places the payload on
		// the transport's channel synchronously — actual delivery to
		// sub.events happens on Broker.listen's separate goroutine
		// (broker.go), a real async hop even though it's normally
		// microseconds. 200ms proved marginal under load (many sequential
		// go test invocations in a tight shell loop create real scheduler
		// pressure); it is not a signal about production event delivery.
		for _, e := range collectEventsTyped(aliceSub, 500*time.Millisecond) {
			if dsr, ok := e.(*events.DeathSaveRolledEvent); ok {
				rolledCount++
				if dsr.Stabilized {
					stabilized = true
				}
			}
		}

		if newActive == aliceEntityID {
			aliceTurnsSeen++
		}
		if enc.Mode() == core.ModeEnded {
			break // death/TPK — confirmed dead, no more rolls possible by design.
		}
		if aliceData := enc.ToData().Players[alicePlayerID]; aliceData != nil && aliceData.HP > 0 {
			break // nat-20 revival — no longer unconscious, nothing left to roll for.
		}
		if stabilized {
			break // 3 successes — RAW correctly stops rolling once stable; no more turns should add to rolledCount.
		}
	}

	s.Positive(aliceTurnsSeen, "the turn cycle must have reached alice's own turn at least once")
	s.GreaterOrEqual(rolledCount, aliceTurnsSeen,
		"expected >=1 DeathSaveRolledEvent per turn-start alice remained unconscious for (saw %d rolls / %d turns) — "+
			"fewer rolls than turns means a turn-start silently failed to roll, the live-playtest symptom",
		rolledCount, aliceTurnsSeen)
}

// reloadEncounter round-trips enc through ToData/LoadFromData with the given
// opts, mirroring the real per-RPC production lifecycle: hydration.go's
// package doc states the hydration cascade is "the only place conditions
// Apply() to e.bus" and runs from a FRESH bus on every LoadFromData call —
// which is what rpg-api actually does once per verb RPC (load, call the
// verb, persist, discard). encounter.Option values (roller, resolver) are
// NOT persisted, so callers must re-supply the same opts on every reload,
// exactly as rpg-api's orchestrator does with its fixed construction.
func (s *UnconsciousZeroHPSuite) reloadEncounter(
	enc *encounter.Encounter, opts ...encounter.Option,
) *encounter.Encounter {
	s.T().Helper()
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadFromData(s.ctx, &data, s.broker, opts...)
	s.Require().NoError(err)
	return loaded
}

// TestThreeFailedDeathSaves_AcrossPerRPCReloads_BridgesToEntityDied is the
// rpg-toolkit#741 root-cause repro. TestThreeFailedDeathSaves_BridgesToEntityDied
// above proves the CharacterDied bridge fires when the entire 3-failure
// sequence runs against ONE long-lived in-memory *Encounter — but production
// never holds an Encounter across RPCs; every verb call is its own
// LoadFromData round-trip (see hydration.go's package doc and
// hydration_test.go's reloadVia, which mirrors the same "per-RPC" lifecycle
// for its own regression).
//
// Failures are driven via repeated damage while unconscious
// (UnconsciousCondition.onDamageReceived — 1 automatic death-save failure
// per hit, no dice roll involved: rpg-toolkit/rulebooks/dnd5e/saves.
// TakeDamageWhileUnconscious), published directly on the encounter-held bus
// rather than routed through NPCAct/monster AI. Two independent reasons a
// turn-start-roll-driven, NPCAct-driven repro cannot give a deterministic
// signal here: (1) buildPerception (encounter/npc.go) deliberately excludes
// HP<=0 players from a monster's target list (rpg-toolkit#733's
// closestPlayer fix — "don't keep attacking a downed player"), so NPCAct
// against an already-downed alice is a silent no-op, not a failure source;
// (2) onTurnStart's Roller is NOT restored by conditions.LoadJSON (documented
// in unconscious.go's onTurnStart comment — reload survives CharacterID +
// counters but not Roller, falling back to a fresh, un-injectable
// dice.NewRoller() every time), so even a turn-start roll can't be forced
// deterministic across reloads. Publishing DamageReceivedEvent directly
// exercises the identical onDamageReceived -> CharacterDiedTopic code path
// death saves ultimately use, without depending on either gap.
func (s *UnconsciousZeroHPSuite) TestThreeFailedDeathSaves_AcrossPerRPCReloads_BridgesToEntityDied() {
	roller := encounter.WithRoller(fixedMaxRoller{})
	resolver := encounter.WithCombatResolver(alwaysHitResolver{damage: 999, damageType: damageSlashing})
	opts := []encounter.Option{roller, resolver}

	enc := s.buildEncounter("enc-uzh-3", roller)

	aliceSub, err := s.broker.Subscribe("enc-uzh-3", alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = aliceSub.Close() }()

	// alice and the goblin are in mutual LoS, so buildEncounter's AddMonster
	// already auto-transitioned to TURN_BASED (alice's DataJSON carries a
	// pre-seeded ActionEconomy, so this doesn't depend on SetMode's
	// seedActorTurn call); an explicit SetMode here would be redundant and
	// error.
	enc = s.reloadEncounter(enc, opts...)
	for enc.ActiveActor() != gobEntityID {
		_, _, endErr := enc.EndTurn(s.ctx, enc.ActiveActor())
		s.Require().NoError(endErr)
		enc = s.reloadEncounter(enc, opts...)
	}
	drainSub(aliceSub, 100*time.Millisecond)

	// Knockdown: 1 HP -> 0 applies UnconsciousCondition, 0 failures yet.
	// Reload before the next verb call, matching one RPC per verb.
	s.Require().NoError(enc.NPCAct(s.ctx, gobEntityID))
	enc = s.reloadEncounter(enc, opts...)
	drainSub(aliceSub, 100*time.Millisecond)

	// 3 more automatic death-save failures via direct DamageReceivedEvent
	// publishes on the freshly-reloaded bus, reloading again after each one.
	// The 3rd pushes Failures to 3 -> Dead -> rulebook CharacterDiedTopic ->
	// subscribeCharacterDiedBridge -> broker EntityDiedEvent. Reloading fresh
	// before/after each publish exercises the exact per-RPC
	// bridge-resubscription path #741 reports as broken.
	for i := 0; i < 3; i++ {
		bus := enc.EventBus()
		s.Require().NotNil(bus)

		var diedRulebook bool
		_, subErr := dnd5eEvents.CharacterDiedTopic.On(bus).Subscribe(s.ctx,
			func(_ context.Context, e dnd5eEvents.CharacterDiedEvent) error {
				if e.CharacterID == string(aliceEntityID) {
					diedRulebook = true
				}
				return nil
			})
		s.Require().NoError(subErr)

		pubErr := dnd5eEvents.DamageReceivedTopic.On(bus).Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
			TargetID: string(aliceEntityID), SourceID: string(gobEntityID),
			Amount: 5, DamageType: damageSlashing,
		})
		s.Require().NoError(pubErr)

		var successes, failures int
		var dead, stabilized bool
		for _, pd := range enc.ToData().Players {
			if pd.EntityID != aliceEntityID {
				continue
			}
			var charData dnd5eCharacter.Data
			s.Require().NoError(json.Unmarshal(pd.DataJSON, &charData))
			for _, raw := range charData.Conditions {
				var uc dnd5eConditions.UnconsciousData
				if jerr := json.Unmarshal(raw, &uc); jerr == nil && uc.Ref != nil && uc.Ref.ID == refs.Conditions.Unconscious().ID {
					successes, failures, dead, stabilized = uc.Successes, uc.Failures, uc.Dead, uc.Stabilized
				}
			}
		}
		s.T().Logf(
			"hit=%d diedRulebook=%v successes=%d failures=%d dead=%v stabilized=%v",
			i, diedRulebook, successes, failures, dead, stabilized,
		)

		enc = s.reloadEncounter(enc, opts...)
	}

	seen := collectEventsTyped(aliceSub, 500*time.Millisecond)
	var died *events.EntityDiedEvent
	for _, e := range seen {
		if d, ok := e.(*events.EntityDiedEvent); ok {
			died = d
		}
	}
	s.Require().NotNil(died,
		"3 failed death saves across per-RPC reloads must bridge to a broker EntityDiedEvent")
	s.Equal(core.EntityID(aliceEntityID), died.EntityID)
	s.Empty(died.KillerID, "final death-save death has no specific killer")

	// rpg-toolkit#772/#782: this must also hold across the per-RPC reload
	// cycle, not just the single-long-lived-Encounter shape
	// TestThreeFailedDeathSaves_BridgesToEntityDied covers — the TPK
	// predicate is re-derived from persisted PlayerData.Dead on every
	// LoadFromData, not cached in memory, so it must survive the reload
	// that happens inside the very same call that confirms alice's death.
	var ended *events.EncounterEndedEvent
	for _, e := range seen {
		if e2, ok := e.(*events.EncounterEndedEvent); ok {
			ended = e2
		}
	}
	s.Require().NotNil(ended, "the last living player's confirmed death must publish EncounterEndedEvent")
	s.Equal(encounter.EncounterEndedReasonTPK, ended.Reason)
	s.Equal(core.ModeEnded, enc.Mode())

	persisted := enc.ToData()
	s.True(persisted.Players[alicePlayerID].Dead)
	s.Empty(persisted.Initiative, "TPK must clear initiative, same as a victory ending")
	for _, pd := range persisted.Players {
		if pd.EntityID != aliceEntityID {
			continue
		}
		var charData dnd5eCharacter.Data
		s.Require().NoError(json.Unmarshal(pd.DataJSON, &charData))
		var found bool
		for _, raw := range charData.Conditions {
			var uc dnd5eConditions.UnconsciousData
			if jerr := json.Unmarshal(raw, &uc); jerr == nil && uc.Ref != nil && uc.Ref.ID == refs.Conditions.Unconscious().ID {
				found = true
				s.GreaterOrEqual(uc.Failures, 3)
				s.True(uc.Dead)
			}
		}
		s.True(found, "persisted state must carry the dead unconscious condition")
	}
}

// TestDeathSaveRolledBridge_EveryRollBridgesToEvent is rpg-toolkit#741 part 3
// regression coverage: before this fix, dnd5eEvents.DeathSaveRolledTopic had
// zero production subscribers anywhere in encounter/*.go — every death save
// roll (not just the terminal Dead/Stabilized outcome) was wire-invisible.
// One turn-start roll is enough to prove the bridge: alice is knocked to 0 HP,
// then her own next turn start rolls a single plain failure (roll=2, not a
// crit) via alwaysFailDeathSaveRoller, and the broker DeathSaveRolledEvent
// must carry the roll detail.
func (s *UnconsciousZeroHPSuite) TestDeathSaveRolledBridge_EveryRollBridgesToEvent() {
	enc := s.buildEncounter("enc-dsr-1", encounter.WithRoller(alwaysFailDeathSaveRoller{}))

	aliceSub, err := s.broker.Subscribe("enc-dsr-1", alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = aliceSub.Close() }()

	// alice and the goblin are in mutual LoS, so buildEncounter's AddMonster
	// already auto-transitioned to TURN_BASED (alice's DataJSON carries a
	// pre-seeded ActionEconomy, so this doesn't depend on SetMode's
	// seedActorTurn call); an explicit SetMode here would be redundant and
	// error.
	for enc.ActiveActor() != gobEntityID {
		_, _, endErr := enc.EndTurn(s.ctx, enc.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(aliceSub, 100*time.Millisecond)

	// Goblin's attack drops alice to 0 HP — Unconscious applied, not dead.
	s.Require().NoError(enc.NPCAct(s.ctx, gobEntityID))
	drainSub(aliceSub, 100*time.Millisecond)

	// Cycle to alice's own next turn start — her one auto-rolled death save.
	active := enc.ActiveActor()
	for active != aliceEntityID {
		next, _, endErr := enc.EndTurn(s.ctx, active)
		s.Require().NoError(endErr)
		active = next
	}

	seen := collectEventsTyped(aliceSub, 500*time.Millisecond)
	var rolled *events.DeathSaveRolledEvent
	for _, e := range seen {
		if d, ok := e.(*events.DeathSaveRolledEvent); ok {
			rolled = d
		}
	}
	s.Require().NotNil(rolled, "a death save roll must bridge to a broker DeathSaveRolledEvent")
	s.Equal(core.EntityID(aliceEntityID), rolled.EntityID)
	s.Equal(2, rolled.Roll)
	s.Equal(0, rolled.Successes)
	s.Equal(1, rolled.Failures)
	s.False(rolled.IsCriticalFail)
	s.False(rolled.IsCriticalSuccess)
	s.False(rolled.Stabilized)
	s.False(rolled.Dead)
}

// TestCharacterStabilizedBridge_ThreeSuccessfulDeathSaves_BridgesToEntityStabilized
// is rpg-toolkit#741 part 2 regression coverage: before this fix,
// dnd5eEvents.CharacterStabilizedTopic had zero production subscribers
// anywhere in encounter/*.go — a character who stabilized (3 successful
// death saves) never told any client. Mirrors
// TestThreeFailedDeathSaves_BridgesToEntityDied's shape exactly but with
// alwaysSuccessDeathSaveRoller instead of alwaysFailDeathSaveRoller, so all
// three of alice's own turn-start rolls succeed instead of fail. Also
// asserts a DeathSaveRolledEvent appeared for each of the three rolls (part
// 3 coverage, complementing the single-roll proof above with the
// multi-roll/no-double-count shape).
func (s *UnconsciousZeroHPSuite) TestCharacterStabilizedBridge_ThreeSuccessfulDeathSaves_BridgesToEntityStabilized() {
	enc := s.buildEncounter("enc-csb-1", encounter.WithRoller(alwaysSuccessDeathSaveRoller{}))

	aliceSub, err := s.broker.Subscribe("enc-csb-1", alicePlayerID)
	s.Require().NoError(err)
	defer func() { _ = aliceSub.Close() }()

	// alice and the goblin are in mutual LoS, so buildEncounter's AddMonster
	// already auto-transitioned to TURN_BASED (alice's DataJSON carries a
	// pre-seeded ActionEconomy, so this doesn't depend on SetMode's
	// seedActorTurn call); an explicit SetMode here would be redundant and
	// error.
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
	var stabilizedRulebook bool
	_, err = dnd5eEvents.CharacterStabilizedTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, e dnd5eEvents.CharacterStabilizedEvent) error {
			if e.CharacterID == string(aliceEntityID) {
				stabilizedRulebook = true
			}
			return nil
		})
	s.Require().NoError(err)

	// Cycle turns (goblin <-> alice) until alice's own turn-start has fired
	// exactly 3 times — 3 consecutive plain death-save successes
	// (alwaysSuccessDeathSaveRoller never rolls a failure/crit).
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
	s.Require().Equal(3, aliceTurnStarts, "must reach alice's own turn 3 times to accumulate 3 successes")
	s.Require().True(stabilizedRulebook, "3 successful death saves must publish the rulebook CharacterStabilizedEvent")

	seen := collectEventsTyped(aliceSub, 500*time.Millisecond)
	var stabilized *events.EntityStabilizedEvent
	rolledCount := 0
	for _, e := range seen {
		switch ev := e.(type) {
		case *events.EntityStabilizedEvent:
			stabilized = ev
		case *events.DeathSaveRolledEvent:
			if ev.EntityID == core.EntityID(aliceEntityID) {
				rolledCount++
			}
		}
	}
	s.Require().NotNil(stabilized,
		"3 successful death saves must bridge to a broker EntityStabilizedEvent")
	s.Equal(core.EntityID(aliceEntityID), stabilized.EntityID)
	s.Equal(3, rolledCount, "all 3 rolls must each bridge their own DeathSaveRolledEvent")

	persisted := enc.ToData()
	for _, pd := range persisted.Players {
		if pd.EntityID != aliceEntityID {
			continue
		}
		var charData dnd5eCharacter.Data
		s.Require().NoError(json.Unmarshal(pd.DataJSON, &charData))
		var found bool
		for _, raw := range charData.Conditions {
			var uc dnd5eConditions.UnconsciousData
			if jerr := json.Unmarshal(raw, &uc); jerr == nil && uc.Ref != nil && uc.Ref.ID == refs.Conditions.Unconscious().ID {
				found = true
				s.GreaterOrEqual(uc.Successes, 3)
				s.True(uc.Stabilized)
			}
		}
		s.True(found, "persisted state must carry the stabilized unconscious condition")
	}
}
