package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// testBardicInspiration is the offer a roll window carries in these scenes.
var testBardicInspiration = encounter.ReactionIdentity{
	Ref:  "dnd5e:conditions:inspired",
	Name: "Bardic Inspiration",
}

type RollWindowTestSuite struct {
	suite.Suite
}

// scene is a fight with alice and the goblin in it, and nothing walking.
func (s *RollWindowTestSuite) scene() *encounter.Encounter {
	inner := &PauseTestSuite{}
	inner.SetT(s.T())
	return inner.walkingScene(&pausingMover{}, &downList{})
}

// rollWindowBeat returns the decoded payload of the one roll-window beat.
func (s *RollWindowTestSuite) rollWindowBeat(
	enc *encounter.Encounter, audience encounter.MemberID,
) map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == encounter.BeatRollWindowOpened {
			return beat
		}
	}
	s.Require().Fail("no roll-window beat in the story")
	return nil
}

func (s *RollWindowTestSuite) TestItPreservesTheSamePresentationIDForEveryRecipient() {
	enc := s.scene()
	_, err := enc.RecordRollWindow(&encounter.RollWindowInput{
		Audience: encounter.MemberID(alice), Offer: testBardicInspiration,
		Roll: 8, Total: 12, PresentationID: "roll~opaque:not-a-sequence",
	})
	s.Require().NoError(err)

	for _, recipient := range []encounter.MemberID{encounter.MemberID(alice), encounter.MemberID(goblin)} {
		beat := s.rollWindowBeat(enc, recipient)
		s.Equal("roll~opaque:not-a-sequence", beat["presentation_id"])
		s.Equal(float64(8), beat["roll"])
		s.Equal(float64(12), beat["total"])
		s.Equal(string(alice), beat["audience"], "the owner is not the event recipient")
	}
}

// TestItNarratesTheNumbersTheChoiceIsMadeWith — the beat exists so the player
// can see what they are deciding about.
func (s *RollWindowTestSuite) TestItNarratesTheNumbersTheChoiceIsMadeWith() {
	enc := s.scene()

	out, err := enc.RecordRollWindow(&encounter.RollWindowInput{
		Audience: encounter.MemberID(alice), Offer: testBardicInspiration, Roll: 8, Total: 12,
	})

	s.Require().NoError(err)
	s.NotZero(out.Seq)

	beat := s.rollWindowBeat(enc, encounter.MemberID(alice))
	s.Equal(string(alice), beat["audience"])
	s.Equal(float64(8), beat["roll"])
	s.Equal(float64(12), beat["total"])
	offer, ok := beat["offer"].(map[string]any)
	s.Require().True(ok)
	s.Equal(testBardicInspiration.Ref, offer["ref"])
	s.Equal(testBardicInspiration.Name, offer["name"])
	s.NotContains(beat, "presentation_id", "legacy input does not invent a roll identity")
}

// TestTheACIsNotOnIt is the one number deliberately left off. A beat that
// leaked it would tell the player whether the swing lands before they choose,
// which is the whole decision.
func (s *RollWindowTestSuite) TestTheACIsNotOnIt() {
	enc := s.scene()
	_, err := enc.RecordRollWindow(&encounter.RollWindowInput{
		Audience: encounter.MemberID(alice), Offer: testBardicInspiration, Roll: 8, Total: 12,
	})
	s.Require().NoError(err)

	beat := s.rollWindowBeat(enc, encounter.MemberID(alice))
	for _, key := range []string{"against", "ac", "target_ac", "targetAC"} {
		_, present := beat[key]
		s.False(present, "the beat must not carry %q", key)
	}
}

// TestEverybodyReadsIt is the pre-v1 full-data rule this composition applies
// to every beat: the client decides what to show.
func (s *RollWindowTestSuite) TestEverybodyReadsIt() {
	enc := s.scene()
	_, err := enc.RecordRollWindow(&encounter.RollWindowInput{
		Audience: encounter.MemberID(alice), Offer: testBardicInspiration, Roll: 8, Total: 12,
	})
	s.Require().NoError(err)

	s.NotNil(s.rollWindowBeat(enc, encounter.MemberID(alice)))
	s.NotNil(s.rollWindowBeat(enc, encounter.MemberID(goblin)))
}

// TestItNarratesAndPausesNothing is the boundary between the two pauses. A
// post-roll window lives in the session's ledger; the encounter's own paused
// turn is a driven-turn remainder and is not involved.
func (s *RollWindowTestSuite) TestItNarratesAndPausesNothing() {
	enc := s.scene()
	_, err := enc.RecordRollWindow(&encounter.RollWindowInput{
		Audience: encounter.MemberID(alice), Offer: testBardicInspiration, Roll: 8, Total: 12,
	})
	s.Require().NoError(err)

	s.False(enc.Paused(), "the encounter's own pause is untouched")
	s.Empty(string(enc.PausedMember()))
}

// TestItRefusesWhatItCannotNarrate — fail closed, every way in.
func (s *RollWindowTestSuite) TestItRefusesWhatItCannotNarrate() {
	enc := s.scene()

	_, err := enc.RecordRollWindow(nil)
	s.Require().ErrorIs(err, encounter.ErrNilInput)

	_, err = enc.RecordRollWindow(&encounter.RollWindowInput{
		Offer: testBardicInspiration, Roll: 8, Total: 12,
	})
	s.Require().ErrorIs(err, encounter.ErrNoMember, "no audience")

	_, err = enc.RecordRollWindow(&encounter.RollWindowInput{
		Audience: "nobody", Offer: testBardicInspiration, Roll: 8, Total: 12,
	})
	s.Require().ErrorIs(err, encounter.ErrNoMember, "an audience outside the fight")

	_, err = enc.RecordRollWindow(&encounter.RollWindowInput{
		Audience: encounter.MemberID(alice),
		Offer:    encounter.ReactionIdentity{Ref: "dnd5e:conditions:inspired"}, Roll: 8, Total: 12,
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData, "an offer with no name is a line with nothing to say")

	_, err = enc.RecordRollWindow(&encounter.RollWindowInput{
		Audience: encounter.MemberID(alice),
		Offer:    encounter.ReactionIdentity{Name: "Bardic Inspiration"}, Roll: 8, Total: 12,
	})
	s.Require().ErrorIs(err, encounter.ErrInvalidData, "an offer with no ref")

	for _, roll := range []int{0, 21, -3} {
		_, err = enc.RecordRollWindow(&encounter.RollWindowInput{
			Audience: encounter.MemberID(alice), Offer: testBardicInspiration, Roll: roll, Total: 12,
		})
		s.Require().ErrorIs(err, encounter.ErrInvalidData, "a d20 does not read %d", roll)
	}
}

func TestRollWindowSuite(t *testing.T) {
	suite.Run(t, new(RollWindowTestSuite))
}

// windowCalculation is the arithmetic an untrained check offers a die on: two
// d20 faces with the lower kept, and the record naming the rule.
func windowCalculation(kept, total int) *encounter.RollCalculation {
	modifier := total - kept
	return &encounter.RollCalculation{
		Components: []encounter.RollComponent{
			{
				Source: encounter.RollSource{
					Ref: "dnd5e:skills:persuasion", Name: "Persuasion", SourceID: string(alice),
				},
				Dice: &encounter.DiceTrace{
					Notation: "2d20", DieSize: 20,
					OriginalRolls: []int{kept, 18}, FinalRolls: []int{kept, 18},
					KeptIndices: []int{0}, Subtotal: kept,
					Keep: &encounter.DiceKeep{
						Rule: encounter.KeepDisadvantage,
						Imposed: []encounter.RollSource{{
							Ref: "dnd5e:rules:untrained", Name: "Untrained",
							Label: "rule", SourceID: string(alice),
						}},
					},
				},
			},
			{
				Source:   encounter.RollSource{Ref: "dnd5e:abilities:charisma", Name: "Charisma"},
				Modifier: &modifier,
			},
		},
		Total: total,
	}
}

// TestTheWindowCarriesTheRollBehindTheQuestion is R5 at this seam. The window
// is where an untrained roll is FIRST seen — before the verdict, before any
// beat about the outcome — and with only Roll and Total it could show one face
// and no rule while asking the player whether to spend a die on it.
func (s *RollWindowTestSuite) TestTheWindowCarriesTheRollBehindTheQuestion() {
	enc := s.scene()

	_, err := enc.RecordRollWindow(&encounter.RollWindowInput{
		Audience: encounter.MemberID(alice), Offer: testBardicInspiration,
		Roll: 8, Total: 12, Calculation: windowCalculation(8, 12),
	})
	s.Require().NoError(err)

	beat := s.rollWindowBeat(enc, encounter.MemberID(alice))
	raw, err := json.Marshal(beat["calculation"])
	s.Require().NoError(err)
	var got encounter.RollCalculation
	s.Require().NoError(json.Unmarshal(raw, &got))

	s.Require().NoError(encounter.ValidateRollCalculation(&got), "it round-trips as valid arithmetic")
	s.Equal(12, got.Total)
	die := got.Components[0].Dice
	s.Require().NotNil(die)
	s.Equal([]int{8, 18}, die.FinalRolls, "both faces survive persistence")
	s.Require().NotNil(die.Keep)
	s.Equal(encounter.KeepDisadvantage, die.Keep.Rule)
	s.Require().Len(die.Keep.Imposed, 1)
	s.Equal("Untrained", die.Keep.Imposed[0].Name)
}

// A window with no arithmetic writes no key: absent means absent, never a
// zero-valued calculation a reader would have to tell apart from a real one.
func (s *RollWindowTestSuite) TestAWindowWithoutArithmeticSaysSo() {
	enc := s.scene()
	_, err := enc.RecordRollWindow(&encounter.RollWindowInput{
		Audience: encounter.MemberID(alice), Offer: testBardicInspiration, Roll: 8, Total: 12,
	})
	s.Require().NoError(err)

	beat := s.rollWindowBeat(enc, encounter.MemberID(alice))
	_, present := beat["calculation"]
	s.False(present)
}

// TestArithmeticThatDisagreesWithTheQuestionIsRefused: the two scalars are what
// the player is shown. A calculation that could not have produced them would
// render a different roll beside the same question, so the window is refused
// rather than written.
func (s *RollWindowTestSuite) TestArithmeticThatDisagreesWithTheQuestionIsRefused() {
	tests := []struct {
		name   string
		change func(*encounter.RollCalculation)
	}{
		{
			name:   "the total disagrees",
			change: func(calc *encounter.RollCalculation) { calc.Total = 99 },
		},
		{
			name: "the d20 subtotal disagrees with the roll shown",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.KeptIndices = []int{1}
				calc.Components[0].Dice.Subtotal = 18
				calc.Components[0].Dice.Keep = nil
				calc.Total = 22
			},
		},
		{
			name: "the roll did not open with a d20",
			change: func(calc *encounter.RollCalculation) {
				die := calc.Components[0].Dice
				die.DieSize, die.Notation = 6, "2d6"
				die.OriginalRolls, die.FinalRolls = []int{1, 6}, []int{1, 6}
				die.Subtotal = 1
				die.Keep = nil
				die.KeptIndices = []int{0}
			},
		},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			enc := s.scene()
			calculation := windowCalculation(8, 12)
			test.change(calculation)

			_, err := enc.RecordRollWindow(&encounter.RollWindowInput{
				Audience: encounter.MemberID(alice), Offer: testBardicInspiration,
				Roll: 8, Total: 12, Calculation: calculation,
			})

			s.Require().Error(err)
			s.Contains(err.Error(), "calculation")
		})
	}
}
