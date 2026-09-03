package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// countingBridgeRoller both pins the face and proves the injected roller is the
// one Second Wind rolls through: if activateFeature drops `Roller: input.Roller`,
// this roller is never called and the bridge test fails deterministically.
type countingBridgeRoller struct {
	face  int
	calls int
}

func (r *countingBridgeRoller) Roll(_ context.Context, _ int) (int, error) {
	r.calls++
	return r.face, nil
}

func (r *countingBridgeRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	r.calls++
	faces := make([]int, count)
	for i := range faces {
		faces[i] = r.face
	}
	return faces, nil
}

var _ dice.Roller = (*countingBridgeRoller)(nil)

// SecondWindBridgeTestSuite covers the full character-level bridge: a real
// Second Wind feature installed on a combat-ready Character, activated through
// ActivateAbility with an interaction-scoped roller.
type SecondWindBridgeTestSuite struct {
	suite.Suite

	ctx  context.Context
	bus  events.EventBus
	char *Character
}

func (s *SecondWindBridgeTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()

	char, err := Load(s.ctx, keeperSheet())
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, char, s.bus))
	s.char = char
}

func (s *SecondWindBridgeTestSuite) TearDownTest() {
	if s.char != nil {
		s.Require().NoError(s.char.Cleanup(s.ctx))
	}
}

// TestActivateAbilityCarriesRollerIntoSecondWindCalculation is the bridge
// regression: the roller handed to ActivateAbilityInput must reach the feature
// (`Roller: input.Roller` in activateFeature) so the published received and
// applied calculations record the interaction's faces. If that wiring is
// dropped, the feature falls back to the process-global default roller and the
// roller.calls assertion fails deterministically.
func (s *SecondWindBridgeTestSuite) TestActivateAbilityCarriesRollerIntoSecondWind() {
	// A combat-ready character on its own turn, hurt so the heal lands.
	s.char.hitPoints = 8
	s.char.maxHitPoints = 10
	markSaved(s.char)
	_, err := s.char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err, "starting the turn should succeed")
	s.Require().True(s.char.InCombat(), "the bridge requires a combat-ready character")

	// Install a real Second Wind feature through the factory.
	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.SecondWind().String(),
		Config:      []byte(`{"level": 1}`),
		CharacterID: s.char.GetID(),
	})
	s.Require().NoError(err)
	secondWind, ok := output.Feature.(*features.SecondWind)
	s.Require().True(ok, "expected a SecondWind feature")
	s.char.features = append(s.char.features, secondWind)
	s.Require().NoError(secondWind.Apply(s.ctx, s.bus))
	defer func() { _ = secondWind.Remove(s.ctx, s.bus) }()

	var received *dnd5eEvents.HealingReceivedEvent
	_, err = dnd5eEvents.HealingReceivedTopic.On(s.bus).Subscribe(
		s.ctx, func(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
			received = &event
			return nil
		})
	s.Require().NoError(err)

	var applied *dnd5eEvents.HealingAppliedEvent
	_, err = dnd5eEvents.HealingAppliedTopic.On(s.bus).Subscribe(
		s.ctx, func(_ context.Context, event dnd5eEvents.HealingAppliedEvent) error {
			applied = &event
			return nil
		})
	s.Require().NoError(err)

	roller := &countingBridgeRoller{face: 6}
	result, err := s.char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.Features.SecondWind(),
		Roller:     roller,
	})
	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Require().True(result.Success, "second wind activates on the character: %s", result.Error)

	// The injected roller is the one the feature rolled: face 6 everywhere and
	// exactly one roller call. Dropping `Roller: input.Roller` from
	// activateFeature leaves calls at zero and the faces random.
	s.Require().Equal(1, roller.calls,
		"the interaction-scoped roller must be the roller Second Wind rolls")
	s.Require().NotNil(received, "activation publishes a received healing event")
	s.Require().NotNil(received.Calculation)
	s.Require().Len(received.Calculation.Components, 2)
	s.Equal([]int{6}, received.Calculation.Components[0].Dice.OriginalRolls)
	s.Equal([]int{6}, received.Calculation.Components[0].Dice.FinalRolls)
	s.Equal(6, received.Calculation.Components[0].Dice.Subtotal)
	s.Require().NotNil(received.Calculation.Components[1].Modifier)
	s.Equal(1, *received.Calculation.Components[1].Modifier, "Fighter level modifier")
	s.Equal(7, received.Calculation.Total)
	s.Equal(7, received.Amount)

	// The applied fact carries the same calculation, post-clamp, with the
	// legacy scalars left zero: the calculation is the only roll record.
	s.Require().NotNil(applied, "the HP owner publishes an applied healing fact")
	s.Require().NotNil(applied.Calculation)
	s.Equal(7, applied.Requested)
	s.Equal(2, applied.Applied)
	s.Equal(8, applied.HPBefore)
	s.Equal(10, applied.HPAfter)
	s.Equal([]int{6}, applied.Calculation.Components[0].Dice.OriginalRolls)
	s.Equal(6, applied.Calculation.Components[0].Dice.Subtotal)
	s.Require().NotNil(applied.Calculation.Components[1].Modifier)
	s.Equal(1, *applied.Calculation.Components[1].Modifier)
	s.Equal(7, applied.Calculation.Total)
	s.Zero(applied.Roll, "calculation-bearing healing carries no legacy roll scalar")
	s.Zero(applied.Modifier, "calculation-bearing healing carries no legacy modifier scalar")

	// Published refs are fresh copies: scribbling on the applied fact cannot
	// corrupt the shared identity singletons.
	applied.SourceRef.ID = "corrupted"
	applied.Calculation.Components[0].Source.Ref.ID = "corrupted"
	applied.Calculation.Components[1].Source.Ref.ID = "corrupted"
	s.Equal("second_wind", refs.Features.SecondWind().ID)
	s.Equal("fighter", refs.Classes.Fighter().ID)
}

func TestSecondWindBridgeSuite(t *testing.T) {
	suite.Run(t, new(SecondWindBridgeTestSuite))
}
