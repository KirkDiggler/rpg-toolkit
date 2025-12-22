package features

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type RageTestSuite struct {
	suite.Suite
	bus  events.EventBus
	rage *Rage
	ctx  context.Context
}

// StubEntity implements core.Entity for testing
type StubEntity struct {
	id string
}

func (m *StubEntity) GetID() string            { return m.id }
func (m *StubEntity) GetType() core.EntityType { return "character" }

// newRageForTest creates a rage feature for testing
func newRageForTest(id string, level int, characterID string) *Rage {
	maxUses := calculateRageUses(level)
	return &Rage{
		id:          id,
		name:        "Rage",
		level:       level,
		characterID: characterID,
		resource: combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          id,
			Maximum:     maxUses,
			CharacterID: characterID,
			ResetType:   coreResources.ResetLongRest,
		}),
	}
}

func (s *RageTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.rage = newRageForTest("rage-feature", 3, "barbarian-1") // Level 3 barbarian
	s.ctx = context.Background()
}

func (s *RageTestSuite) TestCanActivate() {
	owner := &StubEntity{id: "barbarian-1"}

	// Should be able to activate with uses available
	err := s.rage.CanActivate(s.ctx, owner, FeatureInput{})
	s.NoError(err)

	// Use up all rages
	for i := 0; i < 3; i++ {
		err = s.rage.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})
		s.NoError(err)
	}

	// Should not be able to activate with no uses
	err = s.rage.CanActivate(s.ctx, owner, FeatureInput{})
	s.Error(err)
	s.Contains(err.Error(), "no rage uses remaining")
}

func (s *RageTestSuite) TestActivatePublishesCondition() {
	owner := &StubEntity{id: "barbarian-1"}

	// Track if condition was published
	var receivedEvent *dnd5eEvents.ConditionAppliedEvent
	topic := dnd5eEvents.ConditionAppliedTopic.On(s.bus)
	_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
		receivedEvent = &event
		return nil
	})
	s.NoError(err)

	// Activate rage
	err = s.rage.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})
	s.NoError(err)

	// Check that event was published
	s.NotNil(receivedEvent)
	s.Equal(owner, receivedEvent.Target)
	s.Equal(dnd5eEvents.ConditionRaging, receivedEvent.Type)
	s.Equal(dnd5eEvents.ConditionSourceFeature, receivedEvent.Source)

	// Check condition was created properly
	ragingCond, ok := receivedEvent.Condition.(*conditions.RagingCondition)
	s.True(ok, "Event condition should be *RagingCondition")
	s.NotNil(ragingCond)
	s.Equal("barbarian-1", ragingCond.CharacterID)
	s.Equal(2, ragingCond.DamageBonus) // Level 3 = +2 damage
	s.Equal(3, ragingCond.Level)
	s.Equal("rage-feature", ragingCond.Source)
}

func (s *RageTestSuite) TestRageUsesPerLevel() {
	testCases := []struct {
		level    int
		expected int
	}{
		{1, 2},
		{2, 2},
		{3, 3},
		{5, 3},
		{6, 4},
		{11, 4},
		{12, 5},
		{16, 5},
		{17, 6},
		{19, 6},
		{20, -1}, // Unlimited
	}

	for _, tc := range testCases {
		actual := calculateRageUses(tc.level)
		s.Equal(tc.expected, actual, "Level %d should have %d rage uses", tc.level, tc.expected)
	}
}

func (s *RageTestSuite) TestRageDamagePerLevel() {
	testCases := []struct {
		level    int
		expected int
	}{
		{1, 2},
		{8, 2},
		{9, 3},
		{15, 3},
		{16, 4},
		{20, 4},
	}

	for _, tc := range testCases {
		actual := calculateRageDamage(tc.level)
		s.Equal(tc.expected, actual, "Level %d should have +%d rage damage", tc.level, tc.expected)
	}
}

func (s *RageTestSuite) TestUnlimitedRagesAtLevel20() {
	owner := &StubEntity{id: "barbarian-1"}
	rage20 := newRageForTest("epic-rage", 20, "barbarian-1")

	// Should be able to activate many times
	for i := 0; i < 10; i++ {
		err := rage20.CanActivate(s.ctx, owner, FeatureInput{})
		s.NoError(err, "Level 20 barbarian should have unlimited rages")

		err = rage20.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})
		s.NoError(err)
	}
}

func (s *RageTestSuite) TestLoadJSON() {
	jsonData := []byte(`{
		"ref": {"value": "rage"},
		"id": "loaded-rage",
		"name": "Rage",
		"level": 5,
		"character_id": "barbarian-1",
		"uses": 1,
		"max_uses": 3
	}`)

	rage := &Rage{}
	err := rage.loadJSON(jsonData)
	s.NoError(err)

	s.Equal("loaded-rage", rage.id)
	s.Equal("Rage", rage.name)
	s.Equal(5, rage.level)
	s.Equal("barbarian-1", rage.characterID)
	s.Equal(1, rage.resource.Current())
	s.Equal(3, rage.resource.Maximum())
}

func (s *RageTestSuite) TestToJSON() {
	jsonData, err := s.rage.ToJSON()
	s.NoError(err)

	// Load it back
	loaded := &Rage{}
	err = loaded.loadJSON(jsonData)
	s.NoError(err)

	s.Equal(s.rage.id, loaded.id)
	s.Equal(s.rage.name, loaded.name)
	s.Equal(s.rage.level, loaded.level)
}

func (s *RageTestSuite) TestLongRestRecovery() {
	owner := &StubEntity{id: "barbarian-1"}

	// Apply rage to subscribe to rest events
	err := s.rage.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Use all rages (level 3 = 3 uses)
	for i := 0; i < 3; i++ {
		err = s.rage.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})
		s.Require().NoError(err)
	}

	// Verify rages are exhausted
	s.Equal(0, s.rage.resource.Current())

	// Publish a long rest event
	restTopic := dnd5eEvents.RestTopic.On(s.bus)
	err = restTopic.Publish(s.ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetLongRest,
		CharacterID: "barbarian-1",
	})
	s.Require().NoError(err)

	// Verify rages are restored
	s.Equal(3, s.rage.resource.Current(), "Rage uses should be restored on long rest")
}

func (s *RageTestSuite) TestShortRestDoesNotRecoverRage() {
	owner := &StubEntity{id: "barbarian-1"}

	// Apply rage to subscribe to rest events
	err := s.rage.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Use one rage
	err = s.rage.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})
	s.Require().NoError(err)

	// Verify one rage is used
	s.Equal(2, s.rage.resource.Current())

	// Publish a short rest event
	restTopic := dnd5eEvents.RestTopic.On(s.bus)
	err = restTopic.Publish(s.ctx, dnd5eEvents.RestEvent{
		RestType:    coreResources.ResetShortRest,
		CharacterID: "barbarian-1",
	})
	s.Require().NoError(err)

	// Verify rages are NOT restored (rage only restores on long rest)
	s.Equal(2, s.rage.resource.Current(), "Rage uses should NOT be restored on short rest")
}

func TestRageTestSuite(t *testing.T) {
	suite.Run(t, new(RageTestSuite))
}
