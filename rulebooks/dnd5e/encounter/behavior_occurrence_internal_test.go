// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type behaviorOccurrenceSuite struct{ suite.Suite }

func TestBehaviorOccurrenceSuite(t *testing.T) { suite.Run(t, new(behaviorOccurrenceSuite)) }

func (s *behaviorOccurrenceSuite) TestQueuedSoundsRetainTheirOwnKindsAcrossReload() {
	enc, driver := earsHall(s.T(), listener("guard", 4, 1, postedRecipe(s.T())))
	s.Require().NoError(enc.makeSound(noiseAttack, "alice", 0))
	s.Require().NoError(enc.makeSound(noiseSay, "alice", 0))
	data := enc.ToData()
	s.Require().Len(data.Landed, 2)
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	var decoded EncounterData
	s.Require().NoError(json.Unmarshal(raw, &decoded))
	caps := wakeCapabilities(driver)
	caps.Sheets = earsSheets{}
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: decoded, Capabilities: caps})
	s.Require().NoError(err)
	for i, kind := range []string{SoundClang, SoundVoice} {
		s.Run(kind, func() {
			encoded, merr := json.Marshal(loaded.ToData().Landed[i])
			s.Require().NoError(merr)
			var saved map[string]any
			s.Require().NoError(json.Unmarshal(encoded, &saved))
			s.Equal(kind, saved["kind"], "the queue must carry this occurrence, not infer the latest held sound")
			ev, eerr := loaded.evFor("guard", loaded.members["guard"], loaded.landed[i])
			s.Require().NoError(eerr)
			encoded, merr = json.Marshal(ev)
			s.Require().NoError(merr)
			var delivered map[string]any
			s.Require().NoError(json.Unmarshal(encoded, &delivered))
			s.Equal(kind, delivered["Kind"])
			s.NotContains(delivered, "Subject", "a delivered hearing event must not disclose an unseen maker")
		})
	}
	s.Empty(driver.asked, "saving/loading queued observations grants no opportunity")

	// An unknown occurrence remains unknown even though the actor now holds
	// both known kinds. The loader must not infer it from those holdings.
	decoded.Landed[0].Kind = ""
	loaded, err = LoadEncounter(&LoadEncounterInput{Data: decoded, Capabilities: caps})
	s.Require().NoError(err)
	ev, err := loaded.evFor("guard", loaded.members["guard"], loaded.landed[0])
	s.Require().NoError(err)
	s.Empty(ev.Kind)
}
