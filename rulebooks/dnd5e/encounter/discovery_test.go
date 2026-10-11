// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
	"strings"
	"testing"
)

type discoveryConsult struct {
	member encounter.MemberID
	dc     int
}

type discoveryRoller struct {
	calls      int
	success    bool
	err        error
	nilVerdict bool
	order      []discoveryConsult
}

func (r *discoveryRoller) ResolveCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	r.calls++
	r.order = append(r.order, discoveryConsult{member: in.Member, dc: in.Approaches[0].DC})
	if r.err != nil {
		return nil, r.err
	}
	if r.nilVerdict {
		return nil, nil
	} // deliberate provider-contract violation
	return &encounter.ResolveCheckOutput{Beaten: r.success, Applied: in.Approaches[0], Total: 1}, nil
}

func (r *discoveryRoller) ResolveDiscoveryCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return r.ResolveCheck(in)
}

type AutomaticDiscoverySuite struct{ suite.Suite }

func TestAutomaticDiscoverySuite(t *testing.T) { suite.Run(t, new(AutomaticDiscoverySuite)) }
func (s *AutomaticDiscoverySuite) world(private bool, max int, roller *discoveryRoller) *encounter.Encounter {
	in := discoveryFixture(roller)
	in.Members[0].PrivateDiscoveries = private
	in.Field.Concealments[0].Attempts = &encounter.DiscoveryPolicyInput{MaxAttempts: &max}
	enc, err := encounter.NewEncounter(in)
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
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: encounter.Capabilities{Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{}, Conditions: encounter.UnobservedEquipment{}, Sheets: zeroSheets{}, Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: passDriver{}, CheckResolver: r, Witness: nobodyPerceives{}, Actors: encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}}}})
	s.Require().NoError(err)
	s.Equal(1, r.calls, "load does not roll")
	s.step(loaded, 3)
	s.Equal(2, r.calls)
	s.step(loaded, 1)
	s.step(loaded, 3)
	s.Equal(2, r.calls)
	s.Empty(s.results(loaded, "bob"))
}

// learned reports whether the saved journal records member as knowing a
// concealment: the only place what a member found is written down.
func (s *AutomaticDiscoverySuite) learned(enc *encounter.Encounter, member encounter.MemberID) bool {
	journal := enc.ToData().Journal
	if journal == nil {
		return false
	}
	for _, fact := range journal.Facts {
		if fact.Actor == string(member) && strings.HasPrefix(fact.Kind, "known:concealment:") {
			return true
		}
	}
	return false
}

func (s *AutomaticDiscoverySuite) TestSharingToggleDoesNotEraseOrBackfill() {
	r := &discoveryRoller{success: true}
	enc := s.world(true, 1, r)
	s.step(enc, 3)
	s.Len(s.results(enc, "alice"), 1)
	s.Empty(s.results(enc, "bob"))
	s.True(s.learned(enc, "alice"))
	_, err := enc.SetDiscoverySharing(&encounter.SetDiscoverySharingInput{Member: "alice", Sharing: true})
	s.Require().NoError(err)
	s.Empty(s.results(enc, "bob"))
	s.False(s.learned(enc, "bob"), "turning sharing on does not backfill what was found privately")
	r2 := &discoveryRoller{success: true}
	shared := s.world(false, 1, r2)
	s.step(shared, 3)
	s.True(s.learned(shared, "bob"))
	_, err = shared.SetDiscoverySharing(&encounter.SetDiscoverySharingInput{Member: "alice", Sharing: false})
	s.Require().NoError(err)
	s.True(s.learned(shared, "bob"), "turning sharing off does not erase what was found")
}
