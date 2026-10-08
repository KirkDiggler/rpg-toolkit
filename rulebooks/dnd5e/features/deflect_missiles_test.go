package features_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// monkOwner is a monk who answers its monk level and ability scores from its
// own sheet.
type monkOwner struct {
	id     string
	levels map[classes.Class]int
	scores shared.AbilityScores
}

func (m *monkOwner) GetID() string                       { return m.id }
func (m *monkOwner) GetType() core.EntityType            { return "character" }
func (m *monkOwner) ClassLevel(class classes.Class) int  { return m.levels[class] }
func (m *monkOwner) AbilityScores() shared.AbilityScores { return m.scores }

// newMonkOwner is a monk of the given level with Dexterity score dex.
func newMonkOwner(level, dex int) *monkOwner {
	return &monkOwner{
		id:     "test-monk",
		levels: map[classes.Class]int{classes.Monk: level},
		scores: shared.AbilityScores{abilities.DEX: dex},
	}
}

// fixedD10 rolls a fixed face.
type fixedD10 struct{ face int }

func (r fixedD10) Roll(context.Context, int) (int, error) { return r.face, nil }
func (r fixedD10) RollN(_ context.Context, count, _ int) ([]int, error) {
	faces := make([]int, count)
	for i := range faces {
		faces[i] = r.face
	}
	return faces, nil
}

type DeflectMissilesTestSuite struct {
	suite.Suite
	ctx     context.Context
	bus     events.EventBus
	feature features.Feature
}

func TestDeflectMissilesTestSuite(t *testing.T) {
	suite.Run(t, new(DeflectMissilesTestSuite))
}

func (s *DeflectMissilesTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.DeflectMissiles().String(),
		CharacterID: "test-monk",
	})
	s.Require().NoError(err)
	s.feature = output.Feature
}

// trigger subscribes to the deflection event and returns what it heard.
func (s *DeflectMissilesTestSuite) trigger() *[]dnd5eEvents.DeflectMissilesTriggerEvent {
	heard := &[]dnd5eEvents.DeflectMissilesTriggerEvent{}
	_, err := dnd5eEvents.DeflectMissilesTriggerTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.DeflectMissilesTriggerEvent) error {
			*heard = append(*heard, event)
			return nil
		})
	s.Require().NoError(err)
	return heard
}

// An old config carrying a monk level and Dexterity modifier is accepted and
// ignored: nothing is stored.
func (s *DeflectMissilesTestSuite) TestCreateFromRefIgnoresAConfiguredLevel() {
	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.DeflectMissiles().String(),
		Config:      json.RawMessage(`{"monk_level": 5, "dex_modifier": 4}`),
		CharacterID: "test-char",
	})
	s.Require().NoError(err)
	s.Equal(refs.Features.DeflectMissiles().ID, output.Feature.GetID())

	raw, err := output.Feature.ToJSON()
	s.Require().NoError(err)
	s.NotContains(string(raw), "monk_level")
	s.NotContains(string(raw), "dex_modifier")
}

// The deflection reduces by 1d10 + Dexterity modifier + monk level, asked of
// the owner when the reaction is taken: the same feature reads a different
// level and Dexterity with nothing rewritten.
func (s *DeflectMissilesTestSuite) TestDeflectionAsksTheOwnerAtActivation() {
	heard := s.trigger()

	s.Require().NoError(s.feature.Activate(s.ctx, newMonkOwner(3, 16),
		features.FeatureInput{Bus: s.bus, Roller: fixedD10{face: 4}}))
	s.Require().NoError(s.feature.Activate(s.ctx, newMonkOwner(7, 18),
		features.FeatureInput{Bus: s.bus, Roller: fixedD10{face: 4}}))

	s.Require().Len(*heard, 2)
	s.Equal(4+3+3, (*heard)[0].Reduction, "1d10 (4) + DEX 16 (+3) + monk 3")
	s.Equal(4+4+7, (*heard)[1].Reduction, "1d10 (4) + DEX 18 (+4) + monk 7")
	s.Equal("test-monk", (*heard)[0].CharacterID)
	s.Equal(refs.Features.DeflectMissiles().ID, (*heard)[0].Source)
}

// An owner with no monk levels, or one that cannot answer its level, is
// refused: the reduction cannot be computed and nothing is published.
func (s *DeflectMissilesTestSuite) TestDeflectionRefusesAnOwnerWithNoMonkLevels() {
	heard := s.trigger()
	noMonk := newMonkOwner(0, 16)
	noMonk.levels = map[classes.Class]int{classes.Fighter: 3}

	s.Error(s.feature.CanActivate(s.ctx, noMonk, features.FeatureInput{}))
	s.Error(s.feature.Activate(s.ctx, noMonk, features.FeatureInput{Bus: s.bus}))
	s.Error(s.feature.Activate(s.ctx, &mockResourceAccessor{id: "test-monk"}, features.FeatureInput{Bus: s.bus}),
		"an owner that cannot answer the class-level question")
	s.Empty(*heard)
}

func (s *DeflectMissilesTestSuite) TestThrowPublishesTheThrowEvent() {
	var received *dnd5eEvents.DeflectMissilesThrowEvent
	_, err := dnd5eEvents.DeflectMissilesThrowTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.DeflectMissilesThrowEvent) error {
			received = &event
			return nil
		})
	s.Require().NoError(err)

	err = s.feature.Activate(s.ctx, newMonkOwner(3, 16),
		features.FeatureInput{Bus: s.bus, Action: features.DeflectMissilesThrow})

	s.Require().NoError(err)
	s.Require().NotNil(received)
	s.Equal("test-monk", received.CharacterID)
	s.Equal(refs.Features.DeflectMissiles().ID, received.Source)
}

func (s *DeflectMissilesTestSuite) TestApplyAndRemove() {
	busEffect, ok := s.feature.(events.BusEffect)
	s.Require().True(ok, "DeflectMissiles should implement events.BusEffect")

	s.Require().NoError(busEffect.Apply(s.ctx, s.bus))
	s.True(busEffect.IsApplied())
	s.Error(busEffect.Apply(s.ctx, s.bus), "applying twice is refused")

	s.Require().NoError(busEffect.Remove(s.ctx, s.bus))
	s.False(busEffect.IsApplied())
}

// A hit does not deflect on its own: the reduction is the monk's reaction,
// taken through Activate.
func (s *DeflectMissilesTestSuite) TestAHitAloneDoesNotDeflect() {
	heard := s.trigger()
	busEffect, ok := s.feature.(events.BusEffect)
	s.Require().True(ok)
	s.Require().NoError(busEffect.Apply(s.ctx, s.bus))

	s.Require().NoError(dnd5eEvents.DamageReceivedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID: "test-monk", Amount: 8,
	}))

	s.Empty(*heard)
}

func (s *DeflectMissilesTestSuite) TestToJSON() {
	jsonData, err := s.feature.ToJSON()
	s.Require().NoError(err)

	var data map[string]interface{}
	s.Require().NoError(json.Unmarshal(jsonData, &data))
	s.Contains(data, "ref")
	s.Equal("test-monk", data["character_id"])
	s.NotContains(data, "monk_level", "the monk level is the sheet's, never stored")
	s.NotContains(data, "dex_modifier", "the Dexterity modifier is the sheet's, never stored")
}

func (s *DeflectMissilesTestSuite) TestRoundTripIgnoresAnOldSavedCopy() {
	loaded, err := features.LoadJSON(json.RawMessage(
		`{"ref":"dnd5e:features:deflect_missiles","id":"deflect_missiles","name":"Deflect Missiles",` +
			`"character_id":"test-monk","monk_level":3,"dex_modifier":3}`))
	s.Require().NoError(err)
	s.Equal(s.feature.GetID(), loaded.GetID())

	resaved, err := loaded.ToJSON()
	s.Require().NoError(err)
	s.NotContains(string(resaved), "monk_level")
	s.NotContains(string(resaved), "dex_modifier")
}
