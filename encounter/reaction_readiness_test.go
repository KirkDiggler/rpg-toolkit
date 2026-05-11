package encounter_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/stretchr/testify/suite"
)

// ReactionReadinessSuite exercises the ReactionReadiness data field and
// the SetReactionReady / IsReactionReady verbs introduced in Wave 2.11c.
type ReactionReadinessSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	enc       *encounter.Encounter
}

func TestReactionReadinessSuite(t *testing.T) {
	suite.Run(t, new(ReactionReadinessSuite))
}

func (s *ReactionReadinessSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.enc = encounter.New("enc-readiness", s.broker)
}

func (s *ReactionReadinessSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// addCombatPlayer adds the standard fixture combatant (alice / char-alice)
// with full combat stats so OA default seeding fires. All tests in this
// suite assert against the hardcoded entity ID "char-alice".
func (s *ReactionReadinessSuite) addCombatPlayer() {
	s.T().Helper()
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID:   "alice",
		EntityID:   "char-alice",
		Position:   core.Hex{},
		SightRange: 4,
		HP:         20,
		MaxHP:      20,
		AC:         14,
		DamageDice: "1d8",
		DamageType: "slashing",
	}))
}

// addCombatMonster is a helper that adds a monster with combat stats so
// OA default seeding fires.
func (s *ReactionReadinessSuite) addCombatMonster(id string) {
	s.T().Helper()
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID:         core.EntityID(id),
		Position:   core.Hex{Q: 1},
		HP:         10,
		MaxHP:      10,
		AC:         12,
		DamageDice: "1d6",
		DamageType: "piercing",
	}))
}

// --- Default OA Seeding ---

func (s *ReactionReadinessSuite) TestAddPlayer_WithCombatStats_SeedsOAReadiness() {
	s.addCombatPlayer()
	s.True(s.enc.IsReactionReady("char-alice", encounter.OAReactionRef),
		"melee combatant should default-on for OA")
}

func (s *ReactionReadinessSuite) TestAddPlayer_WithoutCombatStats_DoesNotSeedOA() {
	// Observer player — no DamageDice
	s.Require().NoError(s.enc.AddPlayer(encounter.PlayerInput{
		PlayerID:   "observer",
		EntityID:   "char-observer",
		Position:   core.Hex{},
		SightRange: 4,
	}))
	s.False(s.enc.IsReactionReady("char-observer", encounter.OAReactionRef),
		"non-combatant should not have OA seeded")
}

func (s *ReactionReadinessSuite) TestAddMonster_WithCombatStats_SeedsOAReadiness() {
	s.addCombatMonster("goblin-1")
	s.True(s.enc.IsReactionReady("goblin-1", encounter.OAReactionRef),
		"combatant monster should default-on for OA")
}

func (s *ReactionReadinessSuite) TestAddMonster_WithoutCombatStats_DoesNotSeedOA() {
	s.Require().NoError(s.enc.AddMonster(encounter.MonsterInput{
		ID:    "prop-1",
		HP:    5,
		MaxHP: 5,
		AC:    10,
		// No DamageDice — this is a passive prop, not a combatant
	}))
	s.False(s.enc.IsReactionReady("prop-1", encounter.OAReactionRef),
		"non-combatant monster should not have OA seeded")
}

// --- SetReactionReady ---

func (s *ReactionReadinessSuite) TestSetReactionReady_SetsTrue() {
	s.addCombatPlayer()
	// Initially OA is true; verify we can also set an arbitrary reaction.
	const shieldRef = "dnd5e:conditions:shield"
	s.Require().NoError(s.enc.SetReactionReady("char-alice", shieldRef, true))
	s.True(s.enc.IsReactionReady("char-alice", shieldRef))
}

func (s *ReactionReadinessSuite) TestSetReactionReady_SetsFalse() {
	s.addCombatPlayer()
	// OA starts true; player can opt out.
	s.Require().NoError(s.enc.SetReactionReady("char-alice", encounter.OAReactionRef, false))
	s.False(s.enc.IsReactionReady("char-alice", encounter.OAReactionRef))
}

func (s *ReactionReadinessSuite) TestSetReactionReady_Idempotent() {
	s.addCombatPlayer()
	s.Require().NoError(s.enc.SetReactionReady("char-alice", encounter.OAReactionRef, true))
	s.Require().NoError(s.enc.SetReactionReady("char-alice", encounter.OAReactionRef, true))
	s.True(s.enc.IsReactionReady("char-alice", encounter.OAReactionRef))
}

func (s *ReactionReadinessSuite) TestSetReactionReady_UnknownEntity_ReturnsError() {
	err := s.enc.SetReactionReady("char-unknown", encounter.OAReactionRef, true)
	s.Error(err, "setting readiness for unknown entity must return error")
	s.Contains(err.Error(), "char-unknown")
}

func (s *ReactionReadinessSuite) TestSetReactionReady_EmptyCharID_ReturnsError() {
	err := s.enc.SetReactionReady("", encounter.OAReactionRef, true)
	s.Error(err)
}

func (s *ReactionReadinessSuite) TestSetReactionReady_EmptyRef_ReturnsError() {
	s.addCombatPlayer()
	err := s.enc.SetReactionReady("char-alice", "", true)
	s.Error(err)
}

// --- IsReactionReady ---

func (s *ReactionReadinessSuite) TestIsReactionReady_UnknownEntity_ReturnsFalse() {
	s.False(s.enc.IsReactionReady("char-nobody", encounter.OAReactionRef),
		"unknown entity should return false (safe default)")
}

func (s *ReactionReadinessSuite) TestIsReactionReady_UnknownRef_ReturnsFalse() {
	s.addCombatPlayer()
	s.False(s.enc.IsReactionReady("char-alice", "dnd5e:conditions:shield"),
		"reaction not yet set should return false (safe default)")
}

// --- Monster entity readiness ---

func (s *ReactionReadinessSuite) TestSetReactionReady_MonsterEntity() {
	s.addCombatMonster("goblin-1")
	s.Require().NoError(s.enc.SetReactionReady("goblin-1", encounter.OAReactionRef, false))
	s.False(s.enc.IsReactionReady("goblin-1", encounter.OAReactionRef))
}

// --- ToData / LoadFromData round-trip ---

func (s *ReactionReadinessSuite) TestReactionReadiness_RoundTrip() {
	s.addCombatPlayer()
	s.addCombatMonster("goblin-1")
	const shieldRef = "dnd5e:conditions:shield"
	s.Require().NoError(s.enc.SetReactionReady("char-alice", shieldRef, true))
	s.Require().NoError(s.enc.SetReactionReady("goblin-1", encounter.OAReactionRef, false))

	// Serialize
	data := s.enc.ToData()
	raw, err := json.Marshal(data)
	s.Require().NoError(err)

	// Deserialize
	var restored encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &restored))

	// Rehydrate
	enc2, err := encounter.LoadFromData(&restored, s.broker)
	s.Require().NoError(err)

	// Assertions
	s.True(enc2.IsReactionReady("char-alice", encounter.OAReactionRef),
		"alice OA default should survive round-trip")
	s.True(enc2.IsReactionReady("char-alice", shieldRef),
		"alice shield readiness should survive round-trip")
	s.False(enc2.IsReactionReady("goblin-1", encounter.OAReactionRef),
		"goblin OA cleared to false should survive round-trip")
}

func (s *ReactionReadinessSuite) TestReactionReadiness_EmptyData_RoundTrip() {
	// An encounter with no readiness entries should round-trip without error.
	data := s.enc.ToData()
	raw, err := json.Marshal(data)
	s.Require().NoError(err)

	var restored encounter.Data
	s.Require().NoError(json.Unmarshal(raw, &restored))

	enc2, err := encounter.LoadFromData(&restored, s.broker)
	s.Require().NoError(err)

	// No entries — unknown entities return false.
	s.False(enc2.IsReactionReady("char-alice", encounter.OAReactionRef))
}

// --- EventBus accessor ---

func (s *ReactionReadinessSuite) TestEventBus_NotNil() {
	s.NotNil(s.enc.EventBus(), "encounter must provide a non-nil event bus")
}

func (s *ReactionReadinessSuite) TestEventBus_SameInstanceAfterVerbs() {
	// The bus must be the same object between calls (encounter-scoped, not
	// per-verb). Conditions subscribe once and stay subscribed.
	// Use pointer identity — DeepEqual on an interface could match two
	// structurally identical but distinct bus objects.
	bus1 := s.enc.EventBus()
	s.addCombatPlayer()
	bus2 := s.enc.EventBus()
	s.Equal(
		reflect.ValueOf(bus1).Pointer(),
		reflect.ValueOf(bus2).Pointer(),
		"EventBus must return the same instance throughout the encounter's lifetime",
	)
}

func (s *ReactionReadinessSuite) TestEventBus_RestoredAfterLoadFromData() {
	data := s.enc.ToData()
	enc2, err := encounter.LoadFromData(data, s.broker)
	s.Require().NoError(err)
	s.NotNil(enc2.EventBus(), "rehydrated encounter must have a non-nil event bus")
}
