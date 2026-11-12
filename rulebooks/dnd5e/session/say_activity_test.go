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
		in.Behaviors = ordinaryBehavior(encounter.Table{encounter.AnswerTime: {{Weight: 1, Hold: true}}})
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
	// THE CUE RIDES THE PICK (rpg-project#934): the goblin's table answer
	// under the line's speak act, beside his words in his own Speech act.
	found, said := false, false
	for _, act := range actsOf(s.T(), mgr, "alice", "").Acts {
		switch b := act.Body.(type) {
		case session.TableAnswer:
			if b.Who.ID == "goblin" && b.Activity != nil {
				found = true
				s.Equal("dnd5e:activities:thoughtful", b.Activity.Ref)
				s.Equal(uint32(2), b.Activity.Cycles)
			}
		case session.SpeechAct:
			if b.Speaker.ID == "goblin" && b.Words != nil {
				said = true
				s.Equal("Let me think.", *b.Words)
			}
		}
	}
	s.True(found, "the real Speak result must reach the act read")
	s.True(said, "and his words their Speech act")
}
