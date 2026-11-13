// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// SharingSuite is the party's sharing dial as a setting (rpg-project#935):
// carried on the field input, persisted as the run's, read back by the host,
// refused when built or loaded only for a word the dial does not have, and
// refused for a launch by [encounter.LaunchableSharing] when it is shared
// knowledge (rpg-project#937). Nothing here asks an
// audience: the dial decides none.
type SharingSuite struct {
	suite.Suite
}

func TestSharingSuite(t *testing.T) { suite.Run(t, new(SharingSuite)) }

// launch is a yard with two players and a goblin, the dial set to sharing.
func (s *SharingSuite) launch(sharing encounter.Sharing) (*encounter.Encounter, error) {
	return encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 8, 8)},
			Sharing: sharing,
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 4, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 3}},
		},
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: (&OptionTestSuite{}).capabilities(),
	})
}

// load loads data, as a host does from its store.
func (s *SharingSuite) load(data encounter.EncounterData) (*encounter.Encounter, error) {
	return encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Capabilities: (&OptionTestSuite{}).capabilities(),
	})
}

// saved is enc saved and decoded again, as a store hands it back.
func (s *SharingSuite) saved(enc *encounter.Encounter) (encounter.EncounterData, map[string]json.RawMessage) {
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(raw, &data))
	var keys map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(raw, &keys))
	return data, keys
}

// TestUnmarkedIsSharedViewAndWritesNoKey: an encounter that says nothing
// runs as shared view and writes no `sharing` key, the bytes every earlier
// blob has.
func (s *SharingSuite) TestUnmarkedIsSharedViewAndWritesNoKey() {
	enc, err := s.launch("")
	s.Require().NoError(err)
	s.Equal(encounter.SharingView, enc.Sharing())
	s.Equal(encounter.DefaultSharing, enc.Sharing())

	_, keys := s.saved(enc)
	s.NotContains(keys, "sharing", "an unmarked run writes no sharing key")
}

// TestEachBuiltStopLaunchesAndSurvivesAReload: own and view launch, are read
// back as set, and come back as set from a save and from the pieces a store
// keeps.
func (s *SharingSuite) TestEachBuiltStopLaunchesAndSurvivesAReload() {
	for _, stop := range []encounter.Sharing{encounter.SharingOwn, encounter.SharingView} {
		s.Run(string(stop), func() {
			enc, err := s.launch(stop)
			s.Require().NoError(err)
			s.Equal(stop, enc.Sharing())

			data, keys := s.saved(enc)
			s.Equal(`"`+string(stop)+`"`, string(keys["sharing"]), "an authored stop is written as authored")
			reloaded, err := s.load(data)
			s.Require().NoError(err)
			s.Equal(stop, reloaded.Sharing(), "a reload keeps the stop")

			ck, err := enc.ToCheckpoint(&encounter.ToCheckpointInput{})
			s.Require().NoError(err)
			split, err := encounter.SplitPieces(&encounter.SplitPiecesInput{Data: &ck.Rest})
			s.Require().NoError(err)
			held := map[encounter.PieceKey][]byte{}
			for key, p := range split.Pieces {
				held[key] = p.Bytes
			}
			joined, err := encounter.JoinPieces(&encounter.JoinPiecesInput{Pieces: held})
			s.Require().NoError(err)
			s.Equal(stop, joined.Sharing, "the pieces carry the stop")
		})
	}
}

// TestARecordFromBeforeTheDialLoadsAsSharedView: a saved run with no
// sharing key — every run saved before the dial existed — loads as shared
// view.
func (s *SharingSuite) TestARecordFromBeforeTheDialLoadsAsSharedView() {
	enc, err := s.launch("")
	s.Require().NoError(err)
	data, keys := s.saved(enc)
	s.Require().NotContains(keys, "sharing")
	reloaded, err := s.load(data)
	s.Require().NoError(err)
	s.Equal(encounter.SharingView, reloaded.Sharing())
}

// TestSharedKnowledgeIsBuiltAndLoadedForAPreview: building and loading an
// encounter is also how a site is previewed, projected and saved, so a site
// authored with shared knowledge builds, projects its atlas, saves and loads
// like any other, and reads its setting back as authored.
func (s *SharingSuite) TestSharedKnowledgeIsBuiltAndLoadedForAPreview() {
	enc, err := s.launch(encounter.SharingKnowledge)
	s.Require().NoError(err, "built, as a preview builds it")
	s.Equal(encounter.SharingKnowledge, enc.Sharing())
	atlas, err := enc.AtlasFor(alice)
	s.Require().NoError(err, "its atlas projects")
	s.NotEmpty(atlas.Cells)

	data, keys := s.saved(enc)
	s.Equal(`"knowledge"`, string(keys["sharing"]))
	reloaded, err := s.load(data)
	s.Require().NoError(err, "and it loads")
	s.Equal(encounter.SharingKnowledge, reloaded.Sharing())
}

// TestOnlyALaunchRefusesSharedKnowledge: the launch rule, the one a host
// asks before it starts a run for players — shared knowledge refused, loudly,
// with the sentence that says it is not built; the built stops and the
// unmarked default pass; a word the dial does not have is refused too.
func (s *SharingSuite) TestOnlyALaunchRefusesSharedKnowledge() {
	err := encounter.LaunchableSharing(encounter.SharingKnowledge)
	s.Require().ErrorIs(err, encounter.ErrSharedKnowledgeNotBuilt)
	s.Equal("the party's sharing is set to shared knowledge, and shared knowledge is not built yet (rpg-project#937): "+
		"encounter: shared knowledge is not built yet", err.Error())
	for _, ok := range []encounter.Sharing{"", encounter.SharingOwn, encounter.SharingView} {
		s.NoError(encounter.LaunchableSharing(ok), "%q launches", ok)
	}
	s.ErrorIs(encounter.LaunchableSharing("everything"), encounter.ErrUnknownSharing)
}

// TestAnUnknownWordIsRefusedNamingTheWords: a word the dial does not have is
// refused when the encounter is built, naming the words it does, from the
// one list.
func (s *SharingSuite) TestAnUnknownWordIsRefusedNamingTheWords() {
	_, err := s.launch("everything")
	s.Require().ErrorIs(err, encounter.ErrUnknownSharing)
	s.Contains(err.Error(), `the party's sharing is "everything", and sharing is own, view or knowledge`)
}

// TestLoadAsksTheSameRule: a saved run carrying a word the dial does not
// have is refused at load as when built; every word of the dial loads.
func (s *SharingSuite) TestLoadAsksTheSameRule() {
	enc, err := s.launch(encounter.SharingOwn)
	s.Require().NoError(err)
	data, _ := s.saved(enc)

	data.Sharing = "everything"
	_, err = s.load(data)
	s.ErrorIs(err, encounter.ErrUnknownSharing)
}

// TestTheWordsAreTheDialInOrder: the one list, narrowest first, and the
// refusal's phrasing of it.
func (s *SharingSuite) TestTheWordsAreTheDialInOrder() {
	s.Equal([]string{"own", "view", "knowledge"}, encounter.SharingWords())
	s.Equal("own, view or knowledge", encounter.SharingList())
	for _, w := range encounter.Sharings {
		s.True(encounter.IsSharing(w))
	}
	s.False(encounter.IsSharing(""), "unmarked is not a word; it is the default")
	s.False(encounter.IsSharing("everything"))
}
