package features

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

type RageTestSuite struct {
	suite.Suite
	bus  events.EventBus
	rage *Rage
	ctx  context.Context
}

// StubEntity implements core.Entity and coreResources.ResourceAccessor for testing
type StubEntity struct {
	id         string
	resources  map[coreResources.ResourceKey]int
	conditions []dnd5eEvents.ConditionBehavior
	// levels answers the named class-level question; a class it does not
	// name holds zero levels.
	levels map[classes.Class]int
}

// ClassLevel answers from the stub's level record.
func (m *StubEntity) ClassLevel(class classes.Class) int { return m.levels[class] }

// GetConditions reports the stub's own held conditions.
func (m *StubEntity) GetConditions() []dnd5eEvents.ConditionBehavior { return m.conditions }

func (m *StubEntity) GetID() string            { return m.id }
func (m *StubEntity) GetType() core.EntityType { return "character" }

// IsResourceAvailable implements coreResources.ResourceAccessor
func (m *StubEntity) IsResourceAvailable(key coreResources.ResourceKey) bool {
	if m.resources == nil {
		return false
	}
	current, ok := m.resources[key]
	return ok && current > 0
}

// UseResource implements coreResources.ResourceAccessor
func (m *StubEntity) UseResource(key coreResources.ResourceKey, amount int) error {
	if m.resources == nil {
		return nil
	}
	if current, ok := m.resources[key]; ok {
		m.resources[key] = current - amount
	}
	return nil
}

// newStubEntityWithRage creates a barbarian stub of the given level holding
// that level's rage charges (PHB p.48).
func newStubEntityWithRage(id string, level int) *StubEntity {
	maxUses := map[int]int{1: 2, 2: 2, 3: 3, 5: 3, 6: 4, 12: 5, 17: 6, 20: 0}[level]
	return &StubEntity{
		id: id,
		resources: map[coreResources.ResourceKey]int{
			resources.RageCharges: maxUses,
		},
		levels: map[classes.Class]int{classes.Barbarian: level},
	}
}

// newRageForTest creates a rage feature for testing. It holds no level: the
// owner answers it.
func newRageForTest(id string) *Rage {
	return &Rage{
		id:   id,
		name: "Rage",
	}
}

func (s *RageTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.rage = newRageForTest("rage-feature")
	s.ctx = context.Background()
}

func (s *RageTestSuite) TestCanActivate() {
	owner := newStubEntityWithRage("barbarian-1", 3) // Level 3 = 3 uses

	// Should be able to activate with uses available
	err := s.rage.CanActivate(s.ctx, owner, FeatureInput{})
	s.NoError(err)

	// Use up all rages, each from its description
	for i := 0; i < 3; i++ {
		_, err = useRageFromItsDescription(s.ctx, s.T(), s.rage, owner, s.bus)
		s.NoError(err)
	}

	// Should not be able to activate with no uses
	err = s.rage.CanActivate(s.ctx, owner, FeatureInput{})
	s.Error(err)
	s.Contains(err.Error(), "no rage uses remaining")
}

// TestRageRefusesAnOwnerWithNoBarbarianLevels: whether rages are unlimited
// is asked of the owner, so an owner that cannot answer its barbarian level,
// or holds none, is refused rather than read as level one.
func (s *RageTestSuite) TestRageRefusesAnOwnerWithNoBarbarianLevels() {
	for name, levels := range map[string]map[classes.Class]int{
		"no barbarian levels": {classes.Fighter: 3},
		"no levels at all":    nil,
	} {
		owner := newStubEntityWithRage("barbarian-1", 3)
		owner.levels = levels
		s.Error(s.rage.CanActivate(s.ctx, owner, FeatureInput{}), name)
		_, err := useRageFromItsDescription(s.ctx, s.T(), s.rage, owner, s.bus)
		s.Error(err, name)
		s.Equal(3, owner.resources[resources.RageCharges], "%s: no charge spent", name)
	}
}

// TestUnlimitedRagesAtLevel20 asks the row, never the description's cost: a
// level 20 barbarian with no pool at all is always offered Rage. What the cost
// says at 20, and that she rages through it, live in RageDescribedSuite.
func (s *RageTestSuite) TestUnlimitedRagesAtLevel20() {
	owner := &StubEntity{id: "barbarian-1", levels: map[classes.Class]int{classes.Barbarian: 20}}

	for i := 0; i < 10; i++ {
		s.NoError(s.rage.CanActivate(s.ctx, owner, FeatureInput{}), "Level 20 barbarian should have unlimited rages")
		_, err := useRageFromItsDescription(s.ctx, s.T(), s.rage, owner, s.bus)
		s.NoError(err)
	}
	s.Nil(owner.resources, "no resource was needed")
}

func (s *RageTestSuite) TestLoadJSON() {
	// Note: Resource state (uses/max) is owned by Character, not the feature
	jsonData := []byte(`{
		"ref": {"value": "rage"},
		"id": "loaded-rage",
		"name": "Rage",
		"level": 5
	}`)

	rage := &Rage{}
	err := rage.loadJSON(jsonData)
	s.NoError(err)

	s.Equal("loaded-rage", rage.id)
	s.Equal("Rage", rage.name)

	resaved, err := rage.ToJSON()
	s.Require().NoError(err)
	s.NotContains(string(resaved), "level", "an old saved level is ignored, never re-written")
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
}

func (s *RageTestSuite) TestRageRefusedWhileAlreadyRaging() {
	for _, level := range []int{3, 20} {
		owner := newStubEntityWithRage("barbarian-1", level)
		existing, createErr := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.Raging().String(), MemberID: "barbarian-1", Origin: &dnd5eEvents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: "barbarian-1"}, Config: json.RawMessage(`{"damage_bonus":{"barbarian_level":true}}`)})
		s.Require().NoError(createErr)
		owner.conditions = []dnd5eEvents.ConditionBehavior{existing.Condition}
		rage := newRageForTest("rage-feature")
		chargesBefore := owner.resources[resources.RageCharges]

		err := rage.CanActivate(s.ctx, owner, FeatureInput{})
		s.Require().Error(err, "level %d", level)
		s.Equal(rpgerr.CodeConflictingState, rpgerr.GetCode(err), "level %d", level)
		s.Contains(err.Error(), "already raging")

		published := 0
		_, subErr := dnd5eEvents.ConditionAppliedTopic.On(s.bus).Subscribe(s.ctx,
			func(context.Context, dnd5eEvents.ConditionAppliedEvent) error { published++; return nil })
		s.Require().NoError(subErr)
		_, useErr := useRageFromItsDescription(s.ctx, s.T(), rage, owner, s.bus)
		s.Require().Error(useErr)
		s.Contains(useErr.Error(), "already raging")
		s.Equal(chargesBefore, owner.resources[resources.RageCharges], "a refused rage spends no charge")
		s.Zero(published, "a refused rage applies no condition")
	}
}

func (s *RageTestSuite) TestRageRefusesAnOwnerWhoseConditionsCannotBeRead() {
	owner := &resourceOnlyOwner{StubEntity: newStubEntityWithRage("barbarian-1", 3)}

	err := s.rage.CanActivate(s.ctx, owner, FeatureInput{})

	s.Require().Error(err, "unknown is not 'not raging'")
}

// resourceOnlyOwner carries resources but cannot report its conditions.
type resourceOnlyOwner struct{ *StubEntity }

func (o *resourceOnlyOwner) GetConditions() {}

func TestRageTestSuite(t *testing.T) {
	suite.Run(t, new(RageTestSuite))
}
