package encounter_test

import (
	"encoding/json"
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func (s *AnswerTestSuite) TestSayActivitySelectedRecordSurvivesReloadAndKeepsAudience() {
	for _, nested := range []bool{false, true} {
		s.Run(map[bool]string{false: "direct", true: "nested"}[nested], func() {
			rows := []encounter.Answer{
				{Weight: 1, Say: "Let me think.", Activity: &behavior.ActivityCue{Ref: "dnd5e:activities:thoughtful", Cycles: 2}},
			}
			if nested {
				rows = []encounter.Answer{{Weight: 1, Rows: rows}}
			}
			enc := s.reload(s.front(nil, threat(rows, []encounter.Answer{{Weight: 1, Say: "No."}})))
			_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
				OptionID: "threat", Skill: "intimidation", Actor: alice, Target: goblin,
				Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
			})
			s.Require().NoError(err)
			for _, viewer := range []core.EntityID{alice, goblin} {
				beats := s.answeredBeats(enc, viewer)
				s.Require().Len(beats, 1)
				s.Equal("Let me think.", beats[0]["say"])
				s.Equal(map[string]any{"ref": "dnd5e:activities:thoughtful", "cycles": float64(2)}, beats[0]["activity"])
			}
			for _, entry := range enc.ToData().Log.Entries {
				var beat map[string]any
				s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
				if beat["beat"] == encounter.BeatAnswered {
					s.ElementsMatch([]core.EntityID{alice, goblin}, entry.Audience, "the cue retains the answer audience")
				}
			}
			after := s.reload(enc)
			s.Equal(s.answeredBeats(enc, alice), s.answeredBeats(after, alice), "record replay keeps captured intent")
		})
	}
}

func (s *AnswerTestSuite) TestSayActivityDoesNotBorrowFromAnUnselectedResponse() {
	enc := s.reload(s.front(nil, threat([]encounter.Answer{
		{Weight: 1, Say: "Think.", Activity: &behavior.ActivityCue{Ref: "dnd5e:activities:thoughtful", Cycles: 2}},
	}, []encounter.Answer{{Weight: 1, Say: "No."}})))
	_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
		OptionID: "threat", Skill: "intimidation", Actor: alice, Target: goblin,
		Beaten: false, DC: 9, Total: 1, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	beats := s.answeredBeats(enc, alice)
	s.Require().Len(beats, 1)
	s.Equal("No.", beats[0]["say"])
	s.NotContains(beats[0], "activity")
}
