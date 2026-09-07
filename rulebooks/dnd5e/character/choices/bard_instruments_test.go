package choices_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// BardInstrumentsSuite pins the bard's "three musical instruments of your
// choice" end of the choice pipeline: the requirement exists, it names all ten
// instruments, and the validator holds the player to exactly three distinct
// ones from that list.
//
// It is written because nothing pinned it. A requirement whose options a
// consumer cannot map is dropped rather than refused — the choice simply does
// not appear, and no test and no error says why — so the ten being present and
// spelled like the proficiency constants is the thing worth locking.
type BardInstrumentsSuite struct {
	suite.Suite
	req *choices.ToolRequirement
}

func TestBardInstrumentsSuite(t *testing.T) {
	suite.Run(t, new(BardInstrumentsSuite))
}

func (s *BardInstrumentsSuite) SetupTest() {
	requirements := choices.GetClassRequirements(classes.Bard)
	s.Require().NotNil(requirements, "the bard has requirements")
	s.Require().NotNil(requirements.Tools, "the bard has an instrument requirement")
	s.req = requirements.Tools
}

// TestItOffersAllTenInstruments — the list, exactly, and spelled the way the
// proficiency constants spell them so a consumer mapping option strings to its
// own vocabulary finds every one.
func (s *BardInstrumentsSuite) TestItOffersAllTenInstruments() {
	s.Equal(choices.BardInstruments, s.req.ID)
	s.Equal(3, s.req.Count)
	s.Equal([]shared.SelectionID{
		shared.SelectionID(proficiencies.ToolBagpipes),
		shared.SelectionID(proficiencies.ToolDrum),
		shared.SelectionID(proficiencies.ToolDulcimer),
		shared.SelectionID(proficiencies.ToolFlute),
		shared.SelectionID(proficiencies.ToolLute),
		shared.SelectionID(proficiencies.ToolLyre),
		shared.SelectionID(proficiencies.ToolHorn),
		shared.SelectionID(proficiencies.ToolPanFlute),
		shared.SelectionID(proficiencies.ToolShawm),
		shared.SelectionID(proficiencies.ToolViol),
	}, s.req.Options)
}

// chose builds the submissions for one instrument selection.
func (s *BardInstrumentsSuite) chose(instruments ...shared.SelectionID) *choices.Submissions {
	subs := choices.NewSubmissions()
	subs.Add(choices.Submission{
		Category: shared.ChoiceToolProficiency,
		Source:   shared.SourceClass,
		ChoiceID: choices.BardInstruments,
		Values:   instruments,
	})
	return subs
}

// instrumentError runs the validator over one selection and returns the
// instrument requirement's own complaint, or "" when it had none.
func (s *BardInstrumentsSuite) instrumentError(subs *choices.Submissions) string {
	result := choices.NewValidator().Validate(choices.GetClassRequirements(classes.Bard), subs)
	for _, err := range result.Errors {
		if err.ChoiceID == choices.BardInstruments {
			return err.Message
		}
	}
	return ""
}

// TestThreeInstrumentsAreAccepted is the reported case: a bard picks three and
// the requirement is satisfied.
func (s *BardInstrumentsSuite) TestThreeInstrumentsAreAccepted() {
	s.Empty(s.instrumentError(s.chose("lute", "flute", "drum")))
}

// TestEveryOfferedInstrumentIsChoosable — the whole list, one at a time,
// because an option that is offered and then refused is worse than one that
// was never offered.
func (s *BardInstrumentsSuite) TestEveryOfferedInstrumentIsChoosable() {
	for i := range s.req.Options {
		instrument := s.req.Options[i]
		other := s.req.Options[(i+1)%len(s.req.Options)]
		third := s.req.Options[(i+2)%len(s.req.Options)]
		s.Empty(s.instrumentError(s.chose(instrument, other, third)),
			"a bard may choose %q", instrument)
	}
}

// TestFewerThanThreeIsRefused — and the message says how many were missing.
func (s *BardInstrumentsSuite) TestFewerThanThreeIsRefused() {
	s.Contains(s.instrumentError(s.chose("lute", "flute")), "exactly 3")
	s.Contains(s.instrumentError(s.chose("lute")), "exactly 3")
}

// TestMoreThanThreeIsRefused, for the same reason from the other side.
func (s *BardInstrumentsSuite) TestMoreThanThreeIsRefused() {
	s.Contains(s.instrumentError(s.chose("lute", "flute", "drum", "viol")), "exactly 3")
}

// TestChoosingNothingIsRefused — an absent submission is not a satisfied one.
func (s *BardInstrumentsSuite) TestChoosingNothingIsRefused() {
	s.NotEmpty(s.instrumentError(choices.NewSubmissions()))
}

// TestDuplicatesAreRefused. The lute three times is one instrument, not three,
// and counting it as three would hand out a proficiency the player never
// picked.
func (s *BardInstrumentsSuite) TestDuplicatesAreRefused() {
	s.Contains(s.instrumentError(s.chose("lute", "lute", "flute")), "unique")
}

// TestSomethingThatIsNotAnInstrumentIsRefused — the option list is a rule, not
// a suggestion.
func (s *BardInstrumentsSuite) TestSomethingThatIsNotAnInstrumentIsRefused() {
	s.Contains(s.instrumentError(s.chose("lute", "flute", "thieves-tools")), "not in the allowed options")
}
