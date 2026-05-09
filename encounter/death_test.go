package encounter_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
)

// Damage notations reused across the death-path fixtures (extracted so
// the goconst linter is happy — these are inert string literals).
const (
	damage1d8plus2 = "1d8+2"
	damage1d6plus2 = "1d6+2"
)

// fixedMaxRoller always returns the maximum face — guarantees nat-20s
// (auto-hit + crit) on attack rolls and max damage. Lets the death-path
// tests force a kill in a single attack without depending on the random
// roller's output.
type fixedMaxRoller struct{}

func (fixedMaxRoller) Roll(_ context.Context, size int) (int, error) {
	return size, nil
}

func (fixedMaxRoller) RollN(_ context.Context, count, size int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		out[i] = size
	}
	return out, nil
}

// DeathSuite covers the Wave 2.10 death + entity-removal + encounter-end
// chain hung off TakeAction's post-damage path.
type DeathSuite struct {
	suite.Suite
	ctx       context.Context
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	aliceSub  *encounter.Subscription
	bobSub    *encounter.Subscription
}

func TestDeathSuite(t *testing.T) {
	suite.Run(t, new(DeathSuite))
}

func (s *DeathSuite) SetupTest() {
	s.ctx = context.Background()
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
}

func (s *DeathSuite) TearDownTest() {
	if s.aliceSub != nil {
		_ = s.aliceSub.Close()
	}
	if s.bobSub != nil {
		_ = s.bobSub.Close()
	}
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// newSingleMonsterEnc builds a 2-player + 1-monster encounter wired with
// the fixed-max roller and pre-flipped to ModeTurnBased on alice's turn.
// Returns the encounter; subscriptions are written into the suite fields.
func (s *DeathSuite) newSingleMonsterEnc(encID core.EncounterID) *encounter.Encounter {
	enc := encounter.New(encID, s.broker, encounter.WithRoller(fixedMaxRoller{}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: bobEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1}, SightRange: 10,
		HP: 10, MaxHP: 10, AC: 13, AttackBonus: 3,
		DamageDice: "1d6+1", DamageType: "piercing",
	}))
	// Goblin starts at 1 HP so a single max-roll hit is guaranteed-lethal
	// regardless of crit math.
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID:       gobEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1},
		HP:       1, MaxHP: 7, AC: 15, Speed: 6,
		MonsterRef:  "dnd5e:monsters:goblin",
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	var err error
	s.aliceSub, err = s.broker.Subscribe(encID, "alice")
	s.Require().NoError(err)
	s.bobSub, err = s.broker.Subscribe(encID, "bob")
	s.Require().NoError(err)

	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != aliceEntityID {
		_, _, endErr := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(s.aliceSub, 100*time.Millisecond)
	drainSub(s.bobSub, 100*time.Millisecond)
	return enc
}

// Killing the lone monster fires EntityDied + EntityRemoved + EncounterEnded
// (in that order) and flips the encounter to ModeEnded with monsters cleared.
func (s *DeathSuite) TestSlice_MonsterDies_EncounterEnds() {
	enc := s.newSingleMonsterEnc("enc-death-1")

	err := enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.Require().NoError(err)

	seen := collectTypes(s.aliceSub, 500*time.Millisecond)
	s.Contains(seen, "*events.AttackResolvedEvent")
	s.Contains(seen, "*events.DamageDealtEvent")
	s.Contains(seen, "*events.EntityDiedEvent")
	s.Contains(seen, "*events.EntityRemovedEvent")
	s.Contains(seen, "*events.EncounterEndedEvent")

	// Order check: died precedes removed precedes ended.
	diedIdx := indexOf(seen, "*events.EntityDiedEvent")
	removedIdx := indexOf(seen, "*events.EntityRemovedEvent")
	endedIdx := indexOf(seen, "*events.EncounterEndedEvent")
	s.Less(diedIdx, removedIdx, "EntityDied should precede EntityRemoved")
	s.Less(removedIdx, endedIdx, "EntityRemoved should precede EncounterEnded")

	// State: monster gone, mode flipped, initiative cleared.
	persisted := enc.ToData()
	s.NotContains(persisted.Monsters, core.EntityID(gobEntityID))
	s.Equal(core.ModeEnded, enc.Mode())
	s.Empty(persisted.Initiative)
	s.Equal(0, persisted.ActiveIdx)
	s.Equal(0, persisted.Round)
}

// After the encounter ends, subsequent TakeAction / EndTurn / NPCAct
// return ErrEncounterEnded.
func (s *DeathSuite) TestSlice_PostEnd_ErrEncounterEnded() {
	enc := s.newSingleMonsterEnc("enc-death-2")

	s.Require().NoError(enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	))
	s.Require().Equal(core.ModeEnded, enc.Mode())

	err := enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.ErrorIs(err, encounter.ErrEncounterEnded)

	_, _, err = enc.EndTurn(aliceEntityID)
	s.ErrorIs(err, encounter.ErrEncounterEnded)

	err = enc.NPCAct(s.ctx, gobEntityID)
	s.ErrorIs(err, encounter.ErrEncounterEnded)
}

// Killing one of two monsters fires EntityDied + EntityRemoved for the
// dead goblin only, leaves the other monster + initiative intact, and
// does NOT publish EncounterEndedEvent.
func (s *DeathSuite) TestSlice_OneOfTwoMonstersDies_EncounterContinues() {
	encID := core.EncounterID("enc-death-3")
	enc := encounter.New(encID, s.broker, encounter.WithRoller(fixedMaxRoller{}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: bobEntityID,
		Position: core.Hex{Q: 1, R: 0, S: -1}, SightRange: 10,
		HP: 10, MaxHP: 10, AC: 13,
	}))
	// Two goblins; goblin-1 has 1 HP (instakill target), goblin-2 has 7.
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 1, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gob2EntityID, Position: core.Hex{Q: 2, R: 0, S: -2},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	var err error
	s.aliceSub, err = s.broker.Subscribe(encID, "alice")
	s.Require().NoError(err)
	s.bobSub, err = s.broker.Subscribe(encID, "bob")
	s.Require().NoError(err)

	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != aliceEntityID {
		_, _, endErr := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(s.aliceSub, 100*time.Millisecond)
	drainSub(s.bobSub, 100*time.Millisecond)

	s.Require().NoError(enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	))

	seen := collectTypes(s.aliceSub, 500*time.Millisecond)
	s.Contains(seen, "*events.EntityDiedEvent")
	s.Contains(seen, "*events.EntityRemovedEvent")
	s.NotContains(seen, "*events.EncounterEndedEvent",
		"second monster still alive — encounter should NOT have ended")

	persisted := enc.ToData()
	s.NotContains(persisted.Monsters, core.EntityID(gobEntityID))
	s.Contains(persisted.Monsters, core.EntityID(gob2EntityID))
	s.Equal(core.ModeTurnBased, enc.Mode())
	s.NotEmpty(persisted.Initiative)
	s.NotContains(persisted.Initiative, core.EntityID(gobEntityID),
		"dead goblin should be spliced out of initiative")
	s.Contains(persisted.Initiative, core.EntityID(gob2EntityID),
		"surviving goblin should remain in initiative")
}

// EndTurn after a monster kill correctly skips the dead actor — the
// next active actor is whoever was after the dead goblin in initiative
// (NOT the dead goblin). Regression for ActiveIdx splice math.
func (s *DeathSuite) TestSlice_EndTurnSkipsDeadActor() {
	encID := core.EncounterID("enc-death-4")
	enc := encounter.New(encID, s.broker, encounter.WithRoller(fixedMaxRoller{}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 1, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gob2EntityID, Position: core.Hex{Q: 2, R: 0, S: -2},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	var err error
	s.aliceSub, err = s.broker.Subscribe(encID, "alice")
	s.Require().NoError(err)

	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != aliceEntityID {
		_, _, endErr := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(endErr)
	}

	s.Require().NoError(enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	))

	// Initiative must no longer contain the dead goblin.
	persisted := enc.ToData()
	s.NotContains(persisted.Initiative, core.EntityID(gobEntityID))

	// alice ends her turn; the next active actor must be a live entity
	// (alice or goblin-2), never the dead goblin.
	next, _, err := enc.EndTurn(aliceEntityID)
	s.Require().NoError(err)
	s.NotEqual(core.EntityID(gobEntityID), next,
		"EndTurn must not land on the dead goblin")
	s.Contains([]core.EntityID{aliceEntityID, gob2EntityID}, next)
}

// When the encounter ends, EncounterEndedEvent is broadcast to every
// player (PerPlayer covers all of them, regardless of LoS to the kill).
func (s *DeathSuite) TestSlice_EncounterEndedBroadcastsToAll() {
	encID := core.EncounterID("enc-death-5")
	enc := encounter.New(encID, s.broker, encounter.WithRoller(fixedMaxRoller{}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 12, MaxHP: 12, AC: 14, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	// Bob is far out of LoS of the kill — but encounter-end is broadcast.
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "bob", EntityID: bobEntityID,
		Position: core.Hex{Q: 50, R: -25, S: -25}, SightRange: 5,
		HP: 10, MaxHP: 10, AC: 13,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 1, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	var err error
	s.aliceSub, err = s.broker.Subscribe(encID, "alice")
	s.Require().NoError(err)
	s.bobSub, err = s.broker.Subscribe(encID, "bob")
	s.Require().NoError(err)

	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != aliceEntityID {
		_, _, endErr := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(s.aliceSub, 100*time.Millisecond)
	drainSub(s.bobSub, 100*time.Millisecond)

	s.Require().NoError(enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	))

	aliceSeen := collectTypes(s.aliceSub, 500*time.Millisecond)
	bobSeen := collectTypes(s.bobSub, 500*time.Millisecond)
	s.Contains(aliceSeen, "*events.EncounterEndedEvent")
	s.Contains(bobSeen, "*events.EncounterEndedEvent",
		"bob is out of LoS but EncounterEndedEvent must broadcast to him")
	s.Contains(bobSeen, "*events.EntityRemovedEvent",
		"bob is out of LoS but EntityRemovedEvent must broadcast to him")
}

// NPCAct that drops a player to 0 HP fires EntityDiedEvent for the
// player (with the NPC as killer) but does NOT publish EntityRemovedEvent
// for them, does NOT remove them from initiative, and does NOT end the
// encounter (TPK is Wave 2.11+).
func (s *DeathSuite) TestSlice_PlayerDies_PartialOnly() {
	encID := core.EncounterID("enc-death-6")
	enc := encounter.New(encID, s.broker, encounter.WithRoller(fixedMaxRoller{}))
	// alice has 1 HP — guaranteed kill on first NPC hit.
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 1, MaxHP: 12, AC: 10, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	var err error
	s.aliceSub, err = s.broker.Subscribe(encID, "alice")
	s.Require().NoError(err)

	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	// Cycle to the goblin's turn.
	for enc.ActiveActor() != gobEntityID {
		_, _, endErr := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(endErr)
	}
	drainSub(s.aliceSub, 100*time.Millisecond)

	s.Require().NoError(enc.NPCAct(s.ctx, gobEntityID))

	seen := collectTypes(s.aliceSub, 500*time.Millisecond)
	s.Contains(seen, "*events.AttackResolvedEvent")
	s.Contains(seen, "*events.DamageDealtEvent")
	s.Contains(seen, "*events.EntityDiedEvent",
		"player kill must publish EntityDiedEvent")
	s.NotContains(seen, "*events.EntityRemovedEvent",
		"Wave 2.10: player death is partial — no EntityRemovedEvent")
	s.NotContains(seen, "*events.EncounterEndedEvent",
		"Wave 2.10: TPK does not auto-end — no EncounterEndedEvent")

	// Player still seated.
	persisted := enc.ToData()
	s.Contains(persisted.Players, core.PlayerID("alice"))
	s.Contains(persisted.Initiative, core.EntityID(aliceEntityID),
		"player must remain in initiative even at HP=0")
	s.Equal(core.ModeTurnBased, enc.Mode())
}

// Post-end Data round-trips through ToData / LoadFromData with mode
// preserved.
func (s *DeathSuite) TestSlice_PostEndRoundTrips() {
	enc := s.newSingleMonsterEnc("enc-death-7")
	s.Require().NoError(enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	))

	persisted := enc.ToData()
	s.Equal(core.ModeEnded, persisted.Mode)

	rehydrated, err := encounter.LoadFromData(persisted, s.broker)
	s.Require().NoError(err)
	s.Equal(core.ModeEnded, rehydrated.Mode())

	// Verbs against the rehydrated end-state still reject.
	err = rehydrated.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	)
	s.ErrorIs(err, encounter.ErrEncounterEnded)
}

// EntityDiedEvent for a monster kill carries killer = attacker entity id.
func (s *DeathSuite) TestSlice_EntityDiedCarriesKillerID() {
	enc := s.newSingleMonsterEnc("enc-death-8")
	s.Require().NoError(enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	))
	died := waitForEntityDied(s.T(), s.aliceSub, time.Second)
	s.Require().NotNil(died)
	s.Equal(core.EntityID(gobEntityID), died.EntityID)
	s.Equal(core.EntityID(aliceEntityID), died.KillerID)
}

// EntityRemovedEvent carries Reason = "destroyed" for HP-zero kills.
func (s *DeathSuite) TestSlice_EntityRemovedReasonDestroyed() {
	enc := s.newSingleMonsterEnc("enc-death-9")
	s.Require().NoError(enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	))
	removed := waitForEntityRemoved(s.T(), s.aliceSub, time.Second)
	s.Require().NotNil(removed)
	s.Equal(core.EntityID(gobEntityID), removed.EntityID)
	s.Equal(encounter.EntityRemovedReasonDestroyed, removed.Reason)
}

// EncounterEndedEvent carries Reason = "all_hostiles_defeated" for the
// Wave 2.10 end condition.
func (s *DeathSuite) TestSlice_EncounterEndedReasonAllHostilesDefeated() {
	enc := s.newSingleMonsterEnc("enc-death-10")
	s.Require().NoError(enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	))
	ended := waitForEncounterEnded(s.T(), s.aliceSub, time.Second)
	s.Require().NotNil(ended)
	s.Equal(encounter.EncounterEndedReasonAllHostilesDefeated, ended.Reason)
}

// Re-attacking a player who is already at HP=0 must not re-fire
// EntityDiedEvent. Regression for the "multi-attack NPC could double up
// the death event" risk Copilot raised on the first review pass.
func (s *DeathSuite) TestSlice_PlayerDeath_NotRePublishedOnReHit() {
	encID := core.EncounterID("enc-death-rehit-player")
	enc := encounter.New(encID, s.broker, encounter.WithRoller(fixedMaxRoller{}))
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
		HP: 1, MaxHP: 12, AC: 10, AttackBonus: 4,
		DamageDice: damage1d8plus2, DamageType: damageSlashing,
	}))
	s.Require().NoError(enc.AddMonster(encounter.MonsterInput{
		ID: gobEntityID, Position: core.Hex{Q: 1, R: 0, S: -1},
		HP: 7, MaxHP: 7, AC: 15, Speed: 6,
		AttackBonus: 4, DamageDice: damage1d6plus2, DamageType: damageSlashing,
	}))

	var err error
	s.aliceSub, err = s.broker.Subscribe(encID, "alice")
	s.Require().NoError(err)

	s.Require().NoError(enc.SetMode(core.ModeTurnBased))
	for enc.ActiveActor() != gobEntityID {
		_, _, endErr := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(endErr)
	}

	// Goblin's first turn: kills alice. EntityDiedEvent fires.
	s.Require().NoError(enc.NPCAct(s.ctx, gobEntityID))
	firstSeen := collectTypes(s.aliceSub, 500*time.Millisecond)
	s.Equal(1, countOf(firstSeen, "*events.EntityDiedEvent"),
		"first NPC kill must fire exactly one EntityDiedEvent")

	// Cycle back to the goblin's turn (alice is at 0 HP but still in
	// initiative per Wave 2.10 partial player-death). NPCAct again —
	// goblin re-hits the downed player. NO new EntityDiedEvent.
	for enc.ActiveActor() != gobEntityID {
		_, _, endErr := enc.EndTurn(enc.ActiveActor())
		s.Require().NoError(endErr)
	}
	s.Require().NoError(enc.NPCAct(s.ctx, gobEntityID))
	secondSeen := collectTypes(s.aliceSub, 500*time.Millisecond)
	s.Equal(0, countOf(secondSeen, "*events.EntityDiedEvent"),
		"re-hitting a downed player must NOT re-fire EntityDiedEvent")
}

// SetMode rejects ModeEnded — terminal state is internal-only, set by
// checkEncounterEnd, never via the public SetMode verb.
func (s *DeathSuite) TestSetMode_RejectsModeEnded() {
	enc := encounter.New("enc-setmode-end", s.broker)
	s.Require().NoError(enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 10,
	}))
	err := enc.SetMode(core.ModeEnded)
	s.Error(err, "SetMode(ModeEnded) must reject")
	s.NotEqual(core.ModeEnded, enc.Mode())
}

// SetMode against an already-ended encounter returns ErrEncounterEnded.
func (s *DeathSuite) TestSetMode_RejectsAfterEnd() {
	enc := s.newSingleMonsterEnc("enc-setmode-postend")
	s.Require().NoError(enc.TakeAction("alice",
		encounter.ActionRef{Module: "dnd5e", Type: "action", ID: "attack"},
		encounter.ActionTarget{EntityID: gobEntityID},
	))
	s.Require().Equal(core.ModeEnded, enc.Mode())

	err := enc.SetMode(core.ModeFreeRoam)
	s.ErrorIs(err, encounter.ErrEncounterEnded)
}

// --- helpers ---

func countOf(haystack []string, needle string) int {
	n := 0
	for _, h := range haystack {
		if h == needle {
			n++
		}
	}
	return n
}

func indexOf(haystack []string, needle string) int {
	for i, h := range haystack {
		if h == needle {
			return i
		}
	}
	return -1
}

func waitForEntityDied(
	t *testing.T, sub *encounter.Subscription, timeout time.Duration,
) *events.EntityDiedEvent {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				return nil
			}
			if d, ok := evt.(*events.EntityDiedEvent); ok {
				return d
			}
		case <-deadline:
			return nil
		}
	}
}

func waitForEntityRemoved(
	t *testing.T, sub *encounter.Subscription, timeout time.Duration,
) *events.EntityRemovedEvent {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				return nil
			}
			if r, ok := evt.(*events.EntityRemovedEvent); ok {
				return r
			}
		case <-deadline:
			return nil
		}
	}
}

func waitForEncounterEnded(
	t *testing.T, sub *encounter.Subscription, timeout time.Duration,
) *events.EncounterEndedEvent {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-sub.Events():
			if !ok {
				return nil
			}
			if e, ok := evt.(*events.EncounterEndedEvent); ok {
				return e
			}
		case <-deadline:
			return nil
		}
	}
}
