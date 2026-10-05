package contributions_test

import (
	"math"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/stretchr/testify/suite"
)

type frameSuite struct{ suite.Suite }

func TestFrameSuite(t *testing.T) { suite.Run(t, new(frameSuite)) }

func validFrame() contributions.Frame {
	return contributions.Frame{
		Actor:  "rogue",
		Target: contributions.Known("goblin"),
		Action: contributions.ActionFacts{
			Roll:    contributions.Known(contributions.RollKindAttack),
			Ability: contributions.Known(abilities.DEX),
		},
		Pairs: []contributions.PairFacts{{
			From: "goblin", To: "fighter",
			DistanceCells: contributions.Known(1.0),
			Stance:        contributions.Known(contributions.StanceHostile),
		}},
	}
}

func (s *frameSuite) TestFrameValidateRejectsZeroFrameDuplicatePairsAndNaN() {
	s.Require().NoError(validFrame().Validate())

	s.Error(contributions.Frame{}.Validate(), "the zero frame is invalid")

	for name, mutate := range map[string]func(*contributions.Frame){
		"no actor":     func(f *contributions.Frame) { f.Actor = "" },
		"unknown roll": func(f *contributions.Frame) { f.Action.Roll = contributions.Unknown[contributions.RollKind]() },
		"known empty target": func(f *contributions.Frame) {
			f.Target = contributions.Known("")
		},
		"duplicate pair": func(f *contributions.Frame) { f.Pairs = append(f.Pairs, f.Pairs[0]) },
		"self pair":      func(f *contributions.Frame) { f.Pairs[0].To = f.Pairs[0].From },
		"empty from":     func(f *contributions.Frame) { f.Pairs[0].From = "" },
		"empty to":       func(f *contributions.Frame) { f.Pairs[0].To = "" },
		"NaN distance": func(f *contributions.Frame) {
			f.Pairs[0].DistanceCells = contributions.Known(math.NaN())
		},
		"infinite distance": func(f *contributions.Frame) {
			f.Pairs[0].DistanceCells = contributions.Known(math.Inf(1))
		},
		"negative distance": func(f *contributions.Frame) {
			f.Pairs[0].DistanceCells = contributions.Known(-1.0)
		},
		"unrecognised stance": func(f *contributions.Frame) {
			f.Pairs[0].Stance = contributions.Known(contributions.Stance("friendly-ish"))
		},
	} {
		frame := validFrame()
		mutate(&frame)
		s.Error(frame.Validate(), name)
	}

	reverse := validFrame()
	reverse.Pairs = append(reverse.Pairs, contributions.PairFacts{From: "fighter", To: "goblin"})
	s.NoError(reverse.Validate(), "the reverse direction is a distinct pair")

	for _, stance := range []contributions.Stance{
		contributions.StanceHostile, contributions.StanceNeutral,
		contributions.StanceAllied, contributions.StanceNone,
	} {
		frame := validFrame()
		frame.Pairs[0].Stance = contributions.Known(stance)
		s.NoError(frame.Validate(), "known stance %q is valid", stance)
	}
	emptyStance := validFrame()
	emptyStance.Pairs[0].Stance = contributions.Known(contributions.Stance(""))
	s.Error(emptyStance.Validate(), "a known empty stance is not one of the four")

	for name, roll := range map[string]contributions.RollKind{"known empty": "", "misspelled": "atack"} {
		frame := validFrame()
		frame.Action.Roll = contributions.Known(roll)
		s.Error(frame.Validate(), "a %s roll kind is not a roll kind", name)
	}
	savingThrow := validFrame()
	savingThrow.Action.Roll = contributions.Known(contributions.RollKindSavingThrow)
	s.NoError(savingThrow.Validate())

	noAbility := validFrame()
	noAbility.Action.Ability = contributions.Known(abilities.Ability(""))
	s.NoError(noAbility.Validate(), "Known(\"\") is an attack that declares no governing ability")
	for _, ability := range abilities.AllAbilities() {
		frame := validFrame()
		frame.Action.Ability = contributions.Known(ability)
		s.NoError(frame.Validate(), string(ability))
	}
	badAbility := validFrame()
	badAbility.Action.Ability = contributions.Known(abilities.Ability("strength"))
	s.Error(badAbility.Validate(), "a known ability must be one of the six or the declared none")

	unknownTarget := validFrame()
	unknownTarget.Target = contributions.Unknown[string]()
	s.NoError(unknownTarget.Validate(), "an unknown target is a valid frame")
}

func (s *frameSuite) TestFramePairMissingIsUnknown() {
	frame := validFrame()

	present := frame.Pair("goblin", "fighter")
	distance, known := present.DistanceCells.Get()
	s.True(known)
	s.Equal(1.0, distance)

	for _, missing := range []contributions.PairFacts{
		frame.Pair("fighter", "goblin"),
		frame.Pair("goblin", "nobody"),
	} {
		_, distanceKnown := missing.DistanceCells.Get()
		_, stanceKnown := missing.Stance.Get()
		s.False(distanceKnown, "a missing pair's distance is unknown, not zero")
		s.False(stanceKnown, "a missing pair's stance is unknown, not neutral")
	}
	s.Equal("fighter", frame.Pair("fighter", "goblin").From)
	s.Equal("goblin", frame.Pair("fighter", "goblin").To)
}

func (s *frameSuite) TestCloneDetachesPairs() {
	frame := validFrame()
	cloned := frame.Clone()
	s.Equal(frame, cloned)
	cloned.Pairs[0].To = "changed"
	s.Equal("fighter", frame.Pairs[0].To)
}
