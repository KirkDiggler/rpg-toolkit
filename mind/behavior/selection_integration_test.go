package behavior_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/selection"
)

type SelectionIntegrationSuite struct{ suite.Suite }

func TestSelectionIntegrationSuite(t *testing.T) { suite.Run(t, new(SelectionIntegrationSuite)) }

type selectionDie struct {
	faces []int
	sizes []int
}

func (d *selectionDie) Roll(_ context.Context, size int) (int, error) {
	d.sizes = append(d.sizes, size)
	if len(d.faces) == 0 {
		return 0, nil
	}
	face := d.faces[0]
	d.faces = d.faces[1:]
	return face, nil
}

func (s *SelectionIntegrationSuite) TestBadFacesAreRefused() {
	for _, face := range []int{0, -1, 101} {
		die := &selectionDie{faces: []int{face}}
		out, err := behavior.Pick(context.Background(), &behavior.PickInput{
			Key: behavior.KeyTime, Table: behavior.Table{behavior.KeyTime: {{Weight: 1, Hold: true}}}, Die: die})
		s.Nil(out)
		s.ErrorIs(err, behavior.ErrBadTable)
		s.ErrorIs(err, selection.ErrBadRoll)
		s.Equal([]int{100}, die.sizes, "a bad face is not rerolled")
	}
}

func (s *SelectionIntegrationSuite) TestOverflowDoesNotRoll() {
	die := &selectionDie{faces: []int{1}}
	out, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: behavior.Table{behavior.KeyTime: {{Weight: int(^uint(0) >> 1), Hold: true}}}, Die: die})
	s.Nil(out)
	s.ErrorIs(err, behavior.ErrBadTable)
	s.ErrorIs(err, selection.ErrOverflow)
	s.Empty(die.sizes, "bad weight arithmetic is refused before consuming entropy")
}

func (s *SelectionIntegrationSuite) TestNestedChoicePreservesEvidence() {
	die := &selectionDie{faces: []int{200, 101}}
	out, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: behavior.Table{behavior.KeyTime: {guesser(2), guesser(1)}}, Facts: lostAndHearing, Die: die})
	s.Require().NoError(err)
	s.Equal([]int{300, 200}, die.sizes)
	s.Equal([]int{0, 1}, out.Path)
	s.Equal(0, out.Entry)
	s.Equal(behavior.HeadingRight, out.Answer.Search.Heading)
	s.Equal(200, out.Roll)
	s.Equal(300, out.Of)
	s.Equal([]behavior.Candidate{{Entry: 0, Weight: 2, Percent: 100, Loaded: 200}, {Entry: 1, Weight: 1, Percent: 100, Loaded: 100}}, out.Candidates)
	children := []behavior.RowVerdict{{Entry: 0, Held: true, Affordable: true, Weight: 1, Percent: 100, Loaded: 100}, {Entry: 1, Held: true, Affordable: true, Weight: 1, Percent: 100, Loaded: 100}}
	s.Equal([]behavior.RowVerdict{
		{Entry: 0, Held: true, Affordable: true, Weight: 2, Percent: 100, Loaded: 200, Rows: children, Roll: 101, Of: 200},
		{Entry: 1, Held: true, Affordable: true, Weight: 1, Percent: 100, Loaded: 100, Rows: children},
	}, out.Rows, "unselected group's eligibility is explained but it never rolls")
}

func (s *SelectionIntegrationSuite) TestFactsAreJudgedOnce() {
	var reads []string
	facts := behavior.Facts{Threat: func(row behavior.Answer) behavior.ThreatWord {
		reads = append(reads, row.Say)
		if row.Say == "Challenge" {
			return behavior.ThreatNone
		}
		return behavior.ThreatDash
	}}
	die := &selectionDie{faces: []int{100}}
	out, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: behavior.KeyTime, Table: behavior.Table{behavior.KeyTime: {
			{Weight: 1, Say: "Challenge", When: []behavior.When{{Threat: behavior.ThreatNone}}},
			{Weight: 1, Say: "Call for help", When: []behavior.When{{Threat: behavior.ThreatWalk}}},
		}}, Facts: facts, Die: die})
	s.Require().NoError(err)
	s.Equal([]string{"Challenge", "Call for help"}, reads)
	s.Equal([]int{100}, die.sizes)
	s.Equal("Challenge", out.Answer.Say)
	s.Equal(behavior.ThreatNone, out.Rows[0].Threat)
	s.Equal(behavior.ThreatDash, out.Rows[1].Threat)
	s.False(out.Rows[1].Held)
	s.True(out.Rows[1].Affordable)
	s.Zero(out.Rows[1].Loaded)
}

func (s *SelectionIntegrationSuite) TestEmptyAndZeroKeepCallerMeaning() {
	die := &selectionDie{faces: []int{1}}
	for _, key := range []behavior.AnswerKey{behavior.KeyTime, "reply"} {
		// A nonzero profile with no Hold share differs from the default profile.
		out, err := behavior.Pick(context.Background(), &behavior.PickInput{Key: key,
			Table:  behavior.Table{key: {{Weight: 1, Hold: true}}},
			Temper: behavior.Temper{Profile: behavior.TemperProfile{Attack: 100}}, Die: die})
		if key == behavior.KeyTime {
			s.Require().NoError(err)
			s.True(out.Answer.Hold)
			s.Equal(-1, out.Entry)
			s.True(out.Rows[0].Held)
			s.True(out.Rows[0].Affordable)
			s.Zero(out.Rows[0].Loaded)
		} else {
			s.Nil(out)
			s.ErrorIs(err, behavior.ErrBadTable)
		}
	}
	out, err := behavior.Pick(context.Background(), &behavior.PickInput{Key: "reply", Die: die})
	s.Nil(out)
	s.ErrorIs(err, behavior.ErrSilent)
	s.Empty(die.sizes, "empty or zero shares do not draw")
}

func (s *SelectionIntegrationSuite) TestProfileDealPreservesOrder() {
	profile := behavior.TemperProfile{Away: 300, Hold: 50}
	temper := behavior.Temper{Mix: map[string]int{"soldier": 1, "coward": 2}, Profiles: map[string]behavior.TemperProfile{"soldier": {}, "coward": profile}}
	for _, tc := range []struct {
		face int
		word string
	}{{1, "coward"}, {2, "coward"}, {3, "soldier"}} {
		die := &selectionDie{faces: []int{tc.face}}
		out, err := behavior.Deal(context.Background(), &behavior.DealInput{Temper: temper, Die: die})
		s.Require().NoError(err)
		s.Equal(tc.word, out.Temper.Word)
		s.Equal(temper.Profiles[tc.word], out.Temper.Profile)
		s.Nil(out.Temper.Mix)
		s.Equal(tc.face, out.Roll)
		s.Equal(3, out.Of)
		s.Equal([]int{3}, die.sizes)
	}
}

func (s *SelectionIntegrationSuite) TestMalformedProfileDealIsRefused() {
	for _, tc := range []struct {
		name  string
		mix   map[string]int
		face  int
		cause error
		sizes []int
	}{
		{"empty", map[string]int{}, 1, selection.ErrNoCandidates, nil},
		{"overflow", map[string]int{"a": int(^uint(0) >> 1), "b": 1}, 1, selection.ErrOverflow, nil},
		{"bad face", map[string]int{"a": 2}, 3, selection.ErrBadRoll, []int{2}},
	} {
		s.Run(tc.name, func() {
			die := &selectionDie{faces: []int{tc.face}}
			out, err := behavior.Deal(context.Background(), &behavior.DealInput{Temper: behavior.Temper{Mix: tc.mix, Profiles: map[string]behavior.TemperProfile{"a": {}, "b": {}}}, Die: die})
			s.Nil(out)
			s.ErrorIs(err, behavior.ErrBadMix)
			s.ErrorIs(err, tc.cause)
			s.Equal(tc.sizes, die.sizes)
		})
	}
}

func (s *SelectionIntegrationSuite) TestSpokenAnswerUsesTheSameChoice() {
	key := behavior.AnswerKey("reply")
	die := &selectionDie{faces: []int{301}}
	out, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: key, Table: behavior.Table{key: {{Weight: 3, Say: "Through to the hall"}, {Weight: 1, Say: "Wait here"}}},
		Temper: behavior.Temper{Word: "custom", Profile: behavior.TemperProfile{Hold: 50, Attack: 300}}, Die: die})
	s.Require().NoError(err)
	s.Equal("Wait here", out.Answer.Say)
	s.Equal([]int{400}, die.sizes)
	s.Equal([]int{1}, out.Path)
	s.Equal(100, out.Rows[0].Percent, "bare Say keeps the existing neutral factor")
	s.Equal(300, out.Candidates[0].Loaded)
	s.Equal(100, out.Candidates[1].Loaded)
}
