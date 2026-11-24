package features

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

type SecondWindTestSuite struct {
	suite.Suite
	bus        events.EventBus
	secondWind *SecondWind
	ctx        context.Context
}

// newSecondWindForTest creates a Second Wind feature for testing. It holds no
// level: the owner answers it at activation.
func newSecondWindForTest(id string, characterID string) *SecondWind {
	return &SecondWind{
		id:          id,
		name:        "Second Wind",
		characterID: characterID,
	}
}

func (s *SecondWindTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.secondWind = newSecondWindForTest("second-wind-feature", "fighter-1")
	s.ctx = context.Background()
}

type deadSecondWindOwner struct {
	id             string
	hitPoints      int
	deathSaveState saves.DeathSaveState
	dirty          bool
}

func (o *deadSecondWindOwner) GetID() string            { return o.id }
func (o *deadSecondWindOwner) GetType() core.EntityType { return "character" }
func (o *deadSecondWindOwner) LifeState() combat.LifeState {
	return combat.ClassifyLifeState(combat.LifeStateInput{
		Kind:       combat.CombatantKindCharacter,
		Down:       o.hitPoints <= 0,
		Stabilized: o.deathSaveState.Stabilized,
		Dead:       o.deathSaveState.Dead,
	})
}

type countingSecondWindRoller struct {
	calls int
}

func (r *countingSecondWindRoller) Roll(_ context.Context, _ int) (int, error) {
	r.calls++
	return 10, nil
}

func (r *countingSecondWindRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	r.calls++
	return make([]int, count), nil
}

var _ dice.Roller = (*countingSecondWindRoller)(nil)

// fixedSecondWindRoller always rolls the same face, keeping Second Wind's
// published calculation exact without touching process-global randomness.
type fixedSecondWindRoller struct {
	face int
}

func (r *fixedSecondWindRoller) Roll(_ context.Context, _ int) (int, error) {
	return r.face, nil
}

func (r *fixedSecondWindRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	faces := make([]int, count)
	for i := range faces {
		faces[i] = r.face
	}
	return faces, nil
}

var _ dice.Roller = (*fixedSecondWindRoller)(nil)

func (s *SecondWindTestSuite) TestDeadOwnerIsRejectedBeforeRollOrSpend() {
	owner := &deadSecondWindOwner{
		id:             "fighter-1",
		deathSaveState: saves.DeathSaveState{Failures: 3, Dead: true},
	}
	roller := &countingSecondWindRoller{}
	published := 0
	_, err := dnd5eEvents.HealingReceivedTopic.On(s.bus).Subscribe(
		s.ctx,
		func(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
			published++
			owner.hitPoints += event.Amount
			owner.deathSaveState = saves.DeathSaveState{}
			owner.dirty = true
			return nil
		},
	)
	s.Require().NoError(err)

	err = s.secondWind.CanActivate(s.ctx, owner, FeatureInput{Bus: s.bus, Roller: roller})

	s.Require().Error(err)
	s.Equal(rpgerr.CodeInvalidState, rpgerr.GetCode(err))
	s.Zero(roller.calls)
	s.Zero(owner.hitPoints)
	s.Equal(saves.DeathSaveState{Failures: 3, Dead: true}, owner.deathSaveState)
	s.False(owner.dirty)
	s.Zero(published)
}

func (s *SecondWindTestSuite) TestCanActivate() {
	owner := fighterOwner(3)

	// Should be able to activate with uses available
	err := s.secondWind.CanActivate(s.ctx, owner, FeatureInput{})
	s.NoError(err)

	// Use second wind from its description
	_, err = useSecondWindFromItsDescription(s.ctx, s.T(), s.secondWind, owner, &fixedSecondWindRoller{face: 6})
	s.NoError(err)

	// Should not be able to activate with no uses
	err = s.secondWind.CanActivate(s.ctx, owner, FeatureInput{})
	s.Error(err)
	s.Contains(err.Error(), "no second wind uses remaining")
}

func (s *SecondWindTestSuite) TestLoadJSON() {
	jsonData := []byte(`{
		"ref": {"value": "second_wind"},
		"id": "loaded-second-wind",
		"name": "Second Wind",
		"level": 5,
		"character_id": "fighter-99",
		"uses": 0,
		"max_uses": 1
	}`)

	sw := &SecondWind{}
	err := sw.loadJSON(jsonData)
	s.NoError(err)

	s.Equal("loaded-second-wind", sw.id)
	s.Equal("Second Wind", sw.name)
	s.Equal("fighter-99", sw.characterID)
}

func (s *SecondWindTestSuite) TestToJSON() {
	jsonData, err := s.secondWind.ToJSON()
	s.NoError(err)

	// Load it back
	loaded := &SecondWind{}
	err = loaded.loadJSON(jsonData)
	s.NoError(err)

	s.Equal(s.secondWind.id, loaded.id)
	s.Equal(s.secondWind.name, loaded.name)
	s.NotContains(string(jsonData), "level", "Second Wind stores no fighter level")
}

func TestSecondWindTestSuite(t *testing.T) {
	suite.Run(t, new(SecondWindTestSuite))
}

// fighterOwner is a fighter of the given level who answers the named
// class-level question from its level record.
func fighterOwner(level int) *StubEntity {
	return &StubEntity{
		id:        "fighter-1",
		levels:    map[classes.Class]int{classes.Fighter: level},
		resources: map[coreResources.ResourceKey]int{resources.SecondWind: 1},
	}
}

// TestSecondWindRefusesAnOwnerWithNoFighterLevels: an owner that cannot answer
// its fighter level, or holds none, is refused with the use intact — zero is
// never read as level one.
func (s *SecondWindTestSuite) TestSecondWindRefusesAnOwnerWithNoFighterLevels() {
	for name, owner := range map[string]*StubEntity{
		"no fighter levels": {
			id: "fighter-1", levels: map[classes.Class]int{classes.Rogue: 2},
			resources: map[coreResources.ResourceKey]int{resources.SecondWind: 1},
		},
		"no level record": {id: "fighter-1", resources: map[coreResources.ResourceKey]int{resources.SecondWind: 1}},
	} {
		sw := newSecondWindForTest("second-wind-feature", "fighter-1")
		s.Error(sw.CanActivate(s.ctx, owner, FeatureInput{}),
			"%s: the Afford row reads unavailable, not a use Activate would refuse", name)
		s.Error(sw.Activate(s.ctx, owner, FeatureInput{Bus: s.bus}), name)
		s.Equal(1, owner.resources[resources.SecondWind], "%s: the use is not spent", name)
	}
}
