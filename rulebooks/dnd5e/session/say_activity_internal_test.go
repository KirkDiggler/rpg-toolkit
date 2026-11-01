package session

import "fmt"

func (s *ActFoldSuite) TestSayActivityOnATableAnswerAndLegacyAbsence() {
	pieces, err := answeredPiece("time:one", []byte(`{"creature":"goblin","say":"Hmm.","activity":{"ref":"dnd5e:activities:thoughtful","cycles":3}}`))
	s.Require().NoError(err)
	s.Require().Len(pieces, 1)
	answer := pieces[0].body.(TableAnswer)
	s.Equal("goblin", answer.Who.ID)
	s.Require().NotNil(answer.Activity)
	s.Equal("dnd5e:activities:thoughtful", answer.Activity.Ref)
	s.Equal(uint32(3), answer.Activity.Cycles)
	legacy, err := answeredPiece("time:old", []byte(`{"creature":"goblin","say":"Hello."}`))
	s.Require().NoError(err)
	s.Nil(legacy[0].body.(TableAnswer).Activity)
}

func (s *ActFoldSuite) TestSayActivityMalformedPresentRecordsAreNotSilentlyDropped() {
	for _, cue := range []string{
		`null`, `{}`, `[]`, `{"ref":"x"}`, `{"cycles":2}`, `{"ref":"","cycles":2}`,
		`{"ref":"x","cycles":0}`, `{"ref":"x","cycles":-1}`, `{"ref":"x","cycles":1.5}`,
		`{"ref":"x","cycles":4294967296}`, `{"ref":"x","cycles":null}`,
		`{"ref":"x","cycles":2,"delay":1}`, `{"ref":"x","cycles":2,"cycles":3}`,
	} {
		s.Run(cue, func() {
			_, err := answeredPiece("time:one", []byte(fmt.Sprintf(`{"creature":"goblin","say":"Hmm.","activity":%s}`, cue)))
			s.Require().Error(err)
		})
	}
	_, err := answeredPiece("time:one", []byte(`{"creature":"goblin","activity":{"ref":"x","cycles":2}}`))
	s.Require().Error(err, "a cue must accompany the spoken line")
}
