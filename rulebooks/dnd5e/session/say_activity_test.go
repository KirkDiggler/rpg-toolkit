package session_test

import (
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

func (s *SpeakWorldSuite) TestSayActivityReachesTheActFromCapturedRecordOnly() {
	s.driver = session.Driver()
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		line := appealLine()
		line.Success = []encounter.Answer{{Weight: 1, Say: "Let me think.",
			Activity: &behavior.ActivityCue{Ref: "dnd5e:activities:thoughtful", Cycles: 2}}}
		in.Options = []encounter.Option{line}
		in.Table = encounter.Table{encounter.AnswerTime: {{Weight: 1, Hold: true}}}
	}
	mgr := s.front([]int{10, 1, 1})
	out, err := s.appeal(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)
	// Mutate the current authored table after the utterance. Its old act must
	// still describe the selected response captured in the record.
	for _, data := range s.encounters.byID {
		for i := range data.Members {
			if data.Members[i].ID == "goblin" {
				data.Members[i].Options[0].Success[0].Activity.Ref = "future:activity:different"
				data.Members[i].Options[0].Success[0].Activity.Cycles = 9
			}
		}
	}
	found := false
	for _, act := range actsOf(s.T(), mgr, "alice", "").Acts {
		if line, ok := act.Body.(session.SpeakAct); ok {
			found = true
			s.Require().NotNil(line.Answer)
			s.Equal("Let me think.", line.Answer.Say)
			s.Equal("goblin", line.Answer.Who.ID)
			s.Require().NotNil(line.Answer.Activity)
			s.Equal("dnd5e:activities:thoughtful", line.Answer.Activity.Ref)
			s.Equal(uint32(2), line.Answer.Activity.Cycles)
		}
	}
	s.True(found, "the real Speak result must reach the act read")
}
