// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

type DiscoveryPolicySuite struct{ suite.Suite }

func TestDiscoveryPolicySuite(t *testing.T) { suite.Run(t, new(DiscoveryPolicySuite)) }

func discoveryInt(n int) *int { return &n }

func discoveryLifetime(v encounter.DiscoveryLifetime) *encounter.DiscoveryLifetime { return &v }

func (s *DiscoveryPolicySuite) TestDefaultsAreOneTryThreeHexesAndRetained() {
	policy, err := encounter.ResolveDiscoveryPolicy(&encounter.DiscoveryPolicyInput{})
	s.Require().NoError(err)
	s.Equal(1, policy.MaxAttempts)
	s.Equal(3, policy.ResetHexes)
	s.Equal(encounter.DiscoveryLifetimeCharacter, policy.Lifetime)
	_, err = encounter.ResolveDiscoveryPolicy(nil)
	s.ErrorIs(err, encounter.ErrNilInput)
}

func (s *DiscoveryPolicySuite) TestAuthoredOverridesDoNotAliasTheInput() {
	in := &encounter.DiscoveryPolicyInput{
		MaxAttempts: discoveryInt(2), ResetHexes: discoveryInt(4), Lifetime: discoveryLifetime(encounter.DiscoveryLifetimeRun),
	}
	policy, err := encounter.ResolveDiscoveryPolicy(in)
	s.Require().NoError(err)
	*in.MaxAttempts, *in.ResetHexes = 99, 99
	s.Equal(2, policy.MaxAttempts)
	s.Equal(4, policy.ResetHexes)
	s.Equal(encounter.DiscoveryLifetimeRun, policy.Lifetime)
}

func (s *DiscoveryPolicySuite) TestInvalidValuesAreNotDefaulted() {
	for _, in := range []encounter.DiscoveryPolicyInput{
		{MaxAttempts: discoveryInt(0)}, {MaxAttempts: discoveryInt(-1)},
		{ResetHexes: discoveryInt(0)}, {ResetHexes: discoveryInt(1)}, {ResetHexes: discoveryInt(-1)},
		{Lifetime: discoveryLifetime("typo")}, {Lifetime: discoveryLifetime("")},
	} {
		_, err := encounter.ResolveDiscoveryPolicy(&in)
		s.ErrorIs(err, encounter.ErrBadConcealment)
	}
}

func (s *DiscoveryPolicySuite) world(policy *encounter.DiscoveryPolicyInput) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{}, CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 3)},
			Concealments: []encounter.ConcealmentInput{{
				ID: "secret", Checks: vaultCheck(), Attempts: policy,
				Cells: []spatial.Position{{X: 6, Y: 0}},
			}},
		},
		Members: []encounter.MemberInput{{ID: "walker", Kind: encounter.KindPlayer, Position: spatial.Position{}}},
		Endings: []encounter.EndingInput{{Key: "exit", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

func (s *DiscoveryPolicySuite) load(data encounter.EncounterData) (*encounter.Encounter, error) {
	return encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Mover: quietMover{}, Announcer: quietAnnouncer{}, CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
	})
}

func (s *DiscoveryPolicySuite) TestEffectivePolicyIsFrozenAndRoundTrips() {
	input := &encounter.DiscoveryPolicyInput{MaxAttempts: discoveryInt(2)}
	enc := s.world(input)
	*input.MaxAttempts = 100
	data := enc.ToData()
	s.Require().Len(data.Field.Concealments, 1)
	policy := data.Field.Concealments[0].Attempts
	s.Require().NotNil(policy)
	s.Equal(2, policy.MaxAttempts)
	s.Equal(3, policy.ResetHexes)
	s.Equal(encounter.DiscoveryLifetimeCharacter, policy.Lifetime)

	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	var restored encounter.EncounterData
	s.Require().NoError(json.Unmarshal(raw, &restored))
	loaded, err := s.load(restored)
	s.Require().NoError(err)
	s.Equal(data, loaded.ToData())
	policy.MaxAttempts = 50
	s.Equal(2, enc.ToData().Field.Concealments[0].Attempts.MaxAttempts, "snapshot must not alias live state")
	s.Equal(2, loaded.ToData().Field.Concealments[0].Attempts.MaxAttempts)
}

func (s *DiscoveryPolicySuite) TestOmittedPolicyDefaultsButMalformedSavedPolicyRefuses() {
	data := s.world(nil).ToData()
	data.Field.Concealments[0].Attempts = nil
	loaded, err := s.load(data)
	s.Require().NoError(err)
	s.Equal(1, loaded.ToData().Field.Concealments[0].Attempts.MaxAttempts)
	for _, invalid := range []encounter.DiscoveryPolicy{
		{MaxAttempts: 0, ResetHexes: 3, Lifetime: encounter.DiscoveryLifetimeCharacter},
		{MaxAttempts: 1, ResetHexes: 1, Lifetime: encounter.DiscoveryLifetimeCharacter},
		{MaxAttempts: 1, ResetHexes: 3, Lifetime: ""},
		{MaxAttempts: 1, ResetHexes: 3, Lifetime: "typo"},
	} {
		data.Field.Concealments[0].Attempts = &invalid
		_, err = s.load(data)
		s.Require().Error(err, "an explicit malformed snapshot is not an omitted policy")
	}
}
