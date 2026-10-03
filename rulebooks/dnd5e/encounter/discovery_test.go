// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
	"testing"
)

type discoveryRoller struct {
	calls   int
	success bool
}

func (r *discoveryRoller) ResolveCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	r.calls++
	return &encounter.ResolveCheckOutput{Beaten: r.success, Applied: in.Approaches[0], Total: 1}, nil
}

func (r *discoveryRoller) ResolveDiscoveryCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return r.ResolveCheck(in)
}

type AutomaticDiscoverySuite struct{ suite.Suite }

func TestAutomaticDiscoverySuite(t *testing.T) { suite.Run(t, new(AutomaticDiscoverySuite)) }
func (s *AutomaticDiscoverySuite) world(private bool, max int, roller *discoveryRoller) *encounter.Encounter {
	prop := holdableProp("secret", "dnd5e:props:idol", spatial.Position{X: 4, Y: 0})
	prop.Holdable = false
	blocked := true
	prop.BlocksMovement = &blocked
	prop.BlocksLineOfSight = &blocked
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}, CheckResolver: roller, Witness: nobodyPerceives{},
		Field:   encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 10, 3)}, Props: []encounter.PropInput{prop}, Concealments: []encounter.ConcealmentInput{{ID: "secret", Checks: []encounter.CheckApproach{{Ability: "religion", DC: 15}}, Props: []string{"secret"}, Attempts: &encounter.DiscoveryPolicyInput{MaxAttempts: &max}}}},
		Members: []encounter.MemberInput{{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{}, PrivateDiscoveries: private}, {ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}}},
		Endings: []encounter.EndingInput{{Key: "exit", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}
func (s *AutomaticDiscoverySuite) step(enc *encounter.Encounter, x int) {
	_, err := enc.Step(&encounter.StepInput{Member: "alice", To: spatial.Position{X: float64(x)}})
	s.Require().NoError(err)
}
func (s *AutomaticDiscoverySuite) results(enc *encounter.Encounter, member string) []map[string]any {
	out, err := enc.Story(&encounter.StoryInput{Audience: encounter.MemberID(member)})
	s.Require().NoError(err)
	var results []map[string]any
	for _, entry := range out {
		var body map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &body))
		if body["beat"] == encounter.BeatDiscoveryChecked {
			results = append(results, body)
		}
	}
	return results
}
func (s *AutomaticDiscoverySuite) TestDefaultOneTryAndFailureAudience() {
	r := &discoveryRoller{}
	enc := s.world(false, 1, r)
	s.step(enc, 1)
	s.step(enc, 2)
	s.Zero(r.calls)
	s.step(enc, 3)
	s.Equal(1, r.calls)
	s.step(enc, 1)
	s.step(enc, 3)
	s.Equal(1, r.calls)
	s.Len(s.results(enc, "alice"), 1)
	s.Len(s.results(enc, "bob"), 1)
	result := s.results(enc, "alice")[0]
	s.Equal("religion", result["ability"])
	s.Equal(false, result["beaten"])
	for _, key := range []string{"subject", "concealment", "dc", "position"} {
		s.NotContains(result, key)
	}
	_, err := enc.Search(&encounter.SearchInput{Member: "alice", Region: "hall"})
	s.Error(err)
	s.Equal(1, r.calls)
}
func (s *AutomaticDiscoverySuite) TestThreeHexRearmAndReload() {
	r := &discoveryRoller{}
	enc := s.world(true, 2, r)
	s.step(enc, 3)
	s.step(enc, 2)
	s.step(enc, 3)
	s.Equal(1, r.calls)
	s.step(enc, 1)
	data := enc.ToData()
	s.True(data.Discovery["alice"].Attempts["secret"].Armed)
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}, CheckResolver: r, Witness: nobodyPerceives{}})
	s.Require().NoError(err)
	s.Equal(1, r.calls, "load does not roll")
	s.step(loaded, 3)
	s.Equal(2, r.calls)
	s.step(loaded, 1)
	s.step(loaded, 3)
	s.Equal(2, r.calls)
	s.Empty(s.results(loaded, "bob"))
}
func (s *AutomaticDiscoverySuite) TestSharingToggleDoesNotEraseOrBackfill() {
	r := &discoveryRoller{success: true}
	enc := s.world(true, 1, r)
	s.step(enc, 3)
	s.Len(s.results(enc, "alice"), 1)
	s.Empty(s.results(enc, "bob"))
	memory, err := enc.DiscoveryMemory(&encounter.DiscoveryMemoryInput{Member: "alice"})
	s.Require().NoError(err)
	s.True(memory.Checks["secret"].Learned)
	_, err = enc.SetDiscoverySharing(&encounter.SetDiscoverySharingInput{Member: "alice", Sharing: true})
	s.Require().NoError(err)
	s.Empty(s.results(enc, "bob"))
	memory, err = enc.DiscoveryMemory(&encounter.DiscoveryMemoryInput{Member: "bob"})
	s.Require().NoError(err)
	s.Empty(memory.Checks)
	r2 := &discoveryRoller{success: true}
	shared := s.world(false, 1, r2)
	s.step(shared, 3)
	memory, err = shared.DiscoveryMemory(&encounter.DiscoveryMemoryInput{Member: "bob"})
	s.Require().NoError(err)
	s.True(memory.Checks["secret"].Learned)
	_, err = shared.SetDiscoverySharing(&encounter.SetDiscoverySharingInput{Member: "alice", Sharing: false})
	s.Require().NoError(err)
	memory, err = shared.DiscoveryMemory(&encounter.DiscoveryMemoryInput{Member: "bob"})
	s.Require().NoError(err)
	s.True(memory.Checks["secret"].Learned)
}
