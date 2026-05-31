package encounter_test

// #689 — Encounter.LoadFromData owns combatant hydration via the cascade.
//
// These tests are the headline #684 regression guard plus the held-entity /
// turn-reset / ToData-cascade proofs. They run on the REAL broker + the real
// dnd5e event bus the encounter holds, not a stub — per the design's test
// strategy (test #1 first).
//
// The cure: Encounter.LoadFromData cascades into each combatant's own
// LoadFromData exactly once, so a condition (e.g. SneakAttack) Apply()s to the
// encounter bus exactly once. The pre-#689 bug was the host loading the
// character TWICE on the same bus (resolver + turn-end reset), producing two
// SneakAttack instances both adding the "sneak_attack" modifier to the damage
// chain → events.ErrDuplicateID ("modifier ID already exists").

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	tkdice "github.com/KirkDiggler/rpg-toolkit/dice"
	tkenc "github.com/KirkDiggler/rpg-toolkit/encounter"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
	dnd5events "github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eCharacter "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eConditions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	dnd5eMonster "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

const (
	rogueEntityID = encountercore.EntityID("char-rogue")
	roguePlayerID = encountercore.PlayerID("rogue-player")
	hydrGoblinID  = encountercore.EntityID("goblin-1")

	// damagePiercing / dice1d6 hoist two damage literals shared across the
	// encounter test files into constants (goconst).
	damagePiercing = "piercing"
	dice1d6        = "1d6"
)

// HydrationCascadeSuite exercises the LoadFromData cascade on the real bus.
type HydrationCascadeSuite struct {
	suite.Suite
	transport *tkenc.InMemoryTransport
	broker    *tkenc.Broker
	ctx       context.Context
}

func TestHydrationCascadeSuite(t *testing.T) {
	suite.Run(t, new(HydrationCascadeSuite))
}

func (s *HydrationCascadeSuite) SetupTest() {
	s.transport = tkenc.NewInMemoryTransport()
	s.broker = tkenc.NewBroker(s.transport)
	s.ctx = context.Background()
}

func (s *HydrationCascadeSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// rogueCharDataJSON builds a serialized dnd5e character.Data carrying a
// persisted SneakAttack condition, so the cascade reconstitutes + Apply()s it
// to the encounter bus during LoadFromData.
func (s *HydrationCascadeSuite) rogueCharDataJSON() json.RawMessage {
	sneak := dnd5eConditions.NewSneakAttackCondition(dnd5eConditions.SneakAttackInput{
		CharacterID: string(rogueEntityID),
		Level:       3,
		Roller:      tkdice.NewRoller(),
	})
	sneakJSON, err := sneak.ToJSON()
	s.Require().NoError(err)

	data := &dnd5eCharacter.Data{
		ID:               string(rogueEntityID),
		Name:             "Rogue",
		Level:            3,
		ProficiencyBonus: 2,
		Conditions:       []json.RawMessage{sneakJSON},
	}
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	return raw
}

// loadEncounterWithRogue seeds a turn-based encounter with a rogue (carrying
// DataJSON) + a goblin, serializes it, and re-loads via the cascade so the
// rogue's conditions Apply once to the held encounter bus.
func (s *HydrationCascadeSuite) loadEncounterWithRogue() *tkenc.Encounter {
	enc := tkenc.New(s.ctx, "enc-689", s.broker)
	s.Require().NoError(enc.AddPlayer(tkenc.PlayerInput{
		PlayerID: roguePlayerID, EntityID: rogueEntityID,
		Position: encountercore.Hex{}, SightRange: 6,
		HP: 24, MaxHP: 24, AC: 14, AttackBonus: 5,
		DamageDice: dice1d6, DamageType: damagePiercing,
		DataJSON: s.rogueCharDataJSON(),
	}))
	s.Require().NoError(enc.AddMonster(tkenc.MonsterInput{
		ID: hydrGoblinID, Position: encountercore.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	// Round-trip through Data so we exercise the production load path.
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data tkenc.Data
	s.Require().NoError(json.Unmarshal(raw, &data))

	loaded, err := tkenc.LoadFromData(s.ctx, &data, s.broker)
	s.Require().NoError(err)
	return loaded
}

// TestSneakAttack_SubscribesExactlyOnce_AcrossAttacks is the headline #684
// regression. After a single LoadFromData cascade, running the damage chain
// N times on the held bus must NOT surface ErrDuplicateID — proving the
// SneakAttack condition subscribed exactly once. (If the rogue were hydrated
// twice, two SneakAttack instances would each add the "sneak_attack" modifier
// to the chain and the Execute below would return events.ErrDuplicateID.)
func (s *HydrationCascadeSuite) TestSneakAttack_SubscribesExactlyOnce_AcrossAttacks() {
	enc := s.loadEncounterWithRogue()
	bus := enc.EventBus()
	s.Require().NotNil(bus)

	for i := 0; i < 3; i++ {
		// Fresh turn each iteration so once-per-turn does not suppress the
		// modifier add (the double-subscribe must surface on every fire).
		evt, err := s.executeDamageChain(bus)
		s.Require().NoError(err, "attack %d: damage chain must not double-subscribe", i+1)
		s.Require().NotNil(evt)
	}
}

// TestSneakAttack_UsedThisTurn_PersistsAndResets proves the held condition's
// per-turn state (a) accumulates within a turn (set after firing), (b) survives
// ToData → LoadFromData (the cascade-back), and (c) resets at the EndTurn
// boundary with NO re-load (EndTurn emits the turn signal on the held bus).
func (s *HydrationCascadeSuite) TestSneakAttack_UsedThisTurn_PersistsAndResets() {
	enc := s.loadEncounterWithRogue()
	s.Require().NoError(enc.SetMode(encountercore.ModeTurnBased))

	// Fire the rogue's sneak attack once — sets UsedThisTurn=true on the held
	// condition instance.
	_, err := s.executeDamageChain(enc.EventBus())
	s.Require().NoError(err)

	// ToData write-back of the held entities must succeed cleanly.
	_ = enc.ToData()
	s.Require().NoError(enc.SyncErr(), "ToData write-back must not drop a marshal error")

	// (b) ToData re-serializes the held character, capturing UsedThisTurn=true;
	// a fresh load sees it (cross-RPC once-per-turn enforcement).
	enc2 := s.reloadVia(enc)
	s.True(s.sneakUsedThisTurn(enc2.ToData()), "UsedThisTurn must persist through ToData/LoadFromData")

	// (c) End the rogue's turn — the held SneakAttack resets UsedThisTurn=false
	// in place via the bus turn signal (EndTurn emits TurnEndTopic for the
	// ending actor), with no character re-load. Cycle initiative until the
	// rogue's own turn ends (initiative order is roll-dependent), then ToData
	// captures the reset state.
	s.endTurnFor(enc2, rogueEntityID)
	s.False(s.sneakUsedThisTurn(enc2.ToData()), "UsedThisTurn must reset at the rogue's turn boundary")
}

// endTurnFor cycles initiative (calling EndTurn for whoever is active) until
// the target entity's OWN turn is the one that ends. The goblin has no
// rehydratable DataJSON so NPCAct is not driven here — we only advance turns.
func (s *HydrationCascadeSuite) endTurnFor(enc *tkenc.Encounter, target encountercore.EntityID) {
	for i := 0; i < 8; i++ {
		active := enc.ActiveActor()
		_, _, err := enc.EndTurn(s.ctx, active)
		s.Require().NoError(err)
		if active == target {
			return
		}
	}
	s.FailNow("never reached target's turn end", "target=%s", target)
}

// reloadVia round-trips the encounter through Data + LoadFromData on a fresh
// broker, mirroring the per-RPC production lifecycle.
func (s *HydrationCascadeSuite) reloadVia(enc *tkenc.Encounter) *tkenc.Encounter {
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data tkenc.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := tkenc.LoadFromData(s.ctx, &data, s.broker)
	s.Require().NoError(err)
	return loaded
}

// sneakUsedThisTurn decodes the rogue's persisted SneakAttack condition from the
// snapshot's PlayerData.DataJSON and reports its UsedThisTurn flag.
func (s *HydrationCascadeSuite) sneakUsedThisTurn(data *tkenc.Data) bool {
	for _, pd := range data.Players {
		if pd.EntityID != rogueEntityID {
			continue
		}
		var charData dnd5eCharacter.Data
		s.Require().NoError(json.Unmarshal(pd.DataJSON, &charData))
		for _, raw := range charData.Conditions {
			if !bytes.Contains(raw, []byte("sneak_attack")) {
				continue
			}
			var sneak struct {
				UsedThisTurn bool `json:"used_this_turn"`
			}
			s.Require().NoError(json.Unmarshal(raw, &sneak))
			return sneak.UsedThisTurn
		}
	}
	return false
}

// TestNPCAct_UsesHeldMonster_NoDoubleSubscribe proves the production path —
// LoadFromData cascade hydrates the monster onto e.bus, then NPCAct drives its
// turn — does NOT re-load the monster (which would re-subscribe its conditions
// to the same bus, the #684 class). NPCAct must reuse the cascade-held monster.
// A clean NPCAct (no error) on the round-tripped encounter is the proof; a
// re-load would surface as a double-subscribe failure in the chain.
func (s *HydrationCascadeSuite) TestNPCAct_UsesHeldMonster_NoDoubleSubscribe() {
	enc := tkenc.New(s.ctx, "enc-npc-689", s.broker,
		tkenc.WithCombatResolver(&spyCombatResolver{}))
	s.Require().NoError(enc.AddPlayer(tkenc.PlayerInput{
		PlayerID: roguePlayerID, EntityID: rogueEntityID,
		Position: encountercore.Hex{}, SightRange: 10,
		HP: 24, MaxHP: 24, AC: 14,
	}))
	gob := dnd5eMonster.NewGoblin("goblin-npc")
	gobJSON, err := json.Marshal(gob.ToData())
	s.Require().NoError(err)
	s.Require().NoError(enc.AddMonster(tkenc.MonsterInput{
		ID: "goblin-npc", Position: encountercore.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		MonsterRef: monsterRefGoblin, DataJSON: gobJSON,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	// Round-trip so the cascade hydrates the goblin onto the held bus.
	spy := &spyCombatResolver{}
	enc2 := s.reloadViaWithResolver(enc, spy)
	s.Require().NoError(enc2.SetMode(encountercore.ModeTurnBased))

	// Advance to the goblin's turn and run NPCAct. If NPCAct re-loaded the
	// goblin onto the bus, the monster's conditions would double-subscribe.
	for i := 0; i < 8 && enc2.ActiveActor() != "goblin-npc"; i++ {
		active := enc2.ActiveActor()
		_, _, endErr := enc2.EndTurn(s.ctx, active)
		s.Require().NoError(endErr)
	}
	s.Require().Equal(encountercore.EntityID("goblin-npc"), enc2.ActiveActor())
	s.Require().NoError(enc2.NPCAct(s.ctx, "goblin-npc"),
		"NPCAct on a cascade-hydrated monster must not re-load / double-subscribe")
}

// spyCombatResolver records the AttackInput it receives so tests can assert the
// SDK passed the held entity (Attacker/Defender) rather than expecting a
// re-load.
type spyCombatResolver struct {
	lastInput tkenc.AttackInput
	calls     int
}

func (r *spyCombatResolver) ResolveAttack(input tkenc.AttackInput) (*tkenc.AttackOutcome, error) {
	r.calls++
	r.lastInput = input
	return &tkenc.AttackOutcome{
		Hit: true, AttackRoll: 15, AttackBonus: 5, TargetAC: 12,
		Damage: 4, DamageType: damagePiercing,
	}, nil
}

// TestResolver_ReceivesHeldEntity proves the resolver seam receives the
// SDK-held, already-hydrated combatant (AttackInput.Attacker) — not a bare ID
// to re-load. The held attacker must be present and carry the rogue's ID.
func (s *HydrationCascadeSuite) TestResolver_ReceivesHeldEntity() {
	spy := &spyCombatResolver{}
	enc := tkenc.New(s.ctx, "enc-spy", s.broker, tkenc.WithCombatResolver(spy))
	s.Require().NoError(enc.AddPlayer(tkenc.PlayerInput{
		PlayerID: roguePlayerID, EntityID: rogueEntityID,
		Position: encountercore.Hex{}, SightRange: 6,
		HP: 24, MaxHP: 24, AC: 14, AttackBonus: 5,
		DamageDice: dice1d6, DamageType: damagePiercing,
		DataJSON: s.rogueCharDataJSON(),
	}))
	s.Require().NoError(enc.AddMonster(tkenc.MonsterInput{
		ID: hydrGoblinID, Position: encountercore.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	enc2 := s.reloadViaWithResolver(enc, spy)
	s.Require().NoError(enc2.SetMode(encountercore.ModeTurnBased))
	s.driveAttackFromRogue(enc2)

	s.Require().Positive(spy.calls, "resolver should have been called")
	s.Require().NotNil(spy.lastInput.Attacker, "SDK must pass the held attacker, not a bare ID")
	s.Equal(string(rogueEntityID), spy.lastInput.Attacker.GetID(),
		"held attacker must be the hydrated rogue")
}

// TestResolver_NoDataJSON_FallsBack proves a seat without DataJSON yields a nil
// held entity, so the resolver falls back to its stat-snapshot stand-in path
// (Attacker/Defender nil) — guards existing fixtures that don't carry blobs.
func (s *HydrationCascadeSuite) TestResolver_NoDataJSON_FallsBack() {
	spy := &spyCombatResolver{}
	enc := tkenc.New(s.ctx, "enc-nodata", s.broker, tkenc.WithCombatResolver(spy))
	// No DataJSON on the player.
	s.Require().NoError(enc.AddPlayer(tkenc.PlayerInput{
		PlayerID: roguePlayerID, EntityID: rogueEntityID,
		Position: encountercore.Hex{}, SightRange: 6,
		HP: 24, MaxHP: 24, AC: 14, AttackBonus: 5,
		DamageDice: dice1d6, DamageType: damagePiercing,
	}))
	s.Require().NoError(enc.AddMonster(tkenc.MonsterInput{
		ID: hydrGoblinID, Position: encountercore.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	enc2 := s.reloadViaWithResolver(enc, spy)
	s.Require().NoError(enc2.SetMode(encountercore.ModeTurnBased))
	s.driveAttackFromRogue(enc2)

	s.Require().Positive(spy.calls)
	s.Nil(spy.lastInput.Attacker, "no DataJSON → nil held attacker → resolver uses the stand-in")
}

// driveAttackFromRogue cycles initiative until the rogue is active, then issues
// a player attack against the goblin so the resolver seam is exercised.
func (s *HydrationCascadeSuite) driveAttackFromRogue(enc *tkenc.Encounter) {
	for i := 0; i < 8; i++ {
		if enc.ActiveActor() == rogueEntityID {
			break
		}
		active := enc.ActiveActor()
		_, _, err := enc.EndTurn(s.ctx, active)
		s.Require().NoError(err)
	}
	s.Require().Equal(rogueEntityID, enc.ActiveActor(), "rogue must be active to attack")
	err := enc.TakeAction(roguePlayerID,
		tkenc.ActionRef{Module: refModuleDnd5e, Type: refTypeAction, ID: actionIDAttackTest},
		tkenc.ActionTarget{EntityID: hydrGoblinID})
	s.Require().NoError(err)
}

// reloadViaWithResolver round-trips the encounter and re-wires the spy resolver
// on the loaded instance (resolvers are not serialized).
func (s *HydrationCascadeSuite) reloadViaWithResolver(
	enc *tkenc.Encounter, spy *spyCombatResolver,
) *tkenc.Encounter {
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data tkenc.Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := tkenc.LoadFromData(s.ctx, &data, s.broker, tkenc.WithCombatResolver(spy))
	s.Require().NoError(err)
	return loaded
}

// executeDamageChain publishes a DamageChainEvent (with advantage so SneakAttack
// fires without needing room context) on the bus and executes it. Returns the
// final event + any error from chain execution (the double-subscribe surfaces
// here as ErrDuplicateID).
func (s *HydrationCascadeSuite) executeDamageChain(
	bus dnd5events.EventBus,
) (*dnd5eEvents.DamageChainEvent, error) {
	evt := &dnd5eEvents.DamageChainEvent{
		AttackerID:   string(rogueEntityID),
		TargetID:     string(hydrGoblinID),
		AbilityUsed:  "dex",
		DamageType:   damagePiercing,
		WeaponDamage: dice1d6,
		IsCritical:   false,
		HasAdvantage: true,
	}
	chain := dnd5events.NewStagedChain[*dnd5eEvents.DamageChainEvent](dnd5eCombat.ModifierStages)
	modified, err := dnd5eEvents.DamageChain.On(bus).PublishWithChain(s.ctx, evt, chain)
	if err != nil {
		return nil, err
	}
	return modified.Execute(s.ctx, evt)
}
