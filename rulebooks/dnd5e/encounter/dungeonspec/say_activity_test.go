package dungeonspec

import (
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"
)

func (s *SingleRoomSourceSuite) TestSayActivitySurvivesCompileAndSourceRoundTrip() {
	const binding = `    monsterBindings:
      skeleton-a:
        options:
          - id: ask
            say: What happened?
            check: [{ability: persuasion, dc: 5}]
            success:
              - say: Let me think.
                activity: {ref: 'dnd5e:activities:thoughtful', cycles: 2}
    monsterDeclarations:`
	source := []byte(strings.Replace(string(s.raw), "    monsterDeclarations:", binding, 1))
	s.Require().NotEqual(s.raw, source)
	decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: source})
	s.Require().NoError(err)
	encoded, err := yaml.Marshal(decoded.Spec)
	s.Require().NoError(err)
	var document map[string]any
	s.Require().NoError(yaml.Unmarshal(source, &document))
	jsonSource, err := json.Marshal(document)
	s.Require().NoError(err)
	for _, raw := range [][]byte{source, encoded, jsonSource} {
		compiled, loadErr := Load(raw)
		s.Require().NoError(loadErr)
		s.Require().NotEmpty(compiled.Monsters)
		s.Require().Len(compiled.Monsters[0].Options, 1)
		answer := compiled.Monsters[0].Options[0].Success[0]
		s.Equal("Let me think.", answer.Say)
		s.Require().NotNil(answer.Activity)
		s.Equal("dnd5e:activities:thoughtful", answer.Activity.Ref)
		s.Equal(uint32(2), answer.Activity.Cycles)
	}
}

func (s *SingleRoomSourceSuite) TestSayActivityRejectsMalformedContent() {
	for _, body := range []string{
		"null", "[]", "{}", "{ref: thoughtful}", "{cycles: 2}",
		"{ref: '', cycles: 2}", "{ref: 42, cycles: 2}",
		"{ref: thoughtful, cycles: 0}", "{ref: thoughtful, cycles: -1}",
		"{ref: thoughtful, cycles: 1.5}", "{ref: thoughtful, cycles: '2'}",
		"{ref: thoughtful, cycles: 4294967296}", "{ref: thoughtful, cycles: null}",
		"{ref: thoughtful, cycles: 2, delay: 1}",
	} {
		s.Run(body, func() {
			var row AnswerSpec
			err := yaml.Unmarshal([]byte("say: Hmm.\nactivity: "+body+"\n"), &row)
			s.Require().Error(err)
			s.Contains(err.Error(), "activity")
		})
	}
}
