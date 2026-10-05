// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
)

type DiscoveryPolicyAuthoringSuite struct{ suite.Suite }

func TestDiscoveryPolicyAuthoringSuite(t *testing.T) {
	suite.Run(t, new(DiscoveryPolicyAuthoringSuite))
}

func discoveryDocument(policy string) string {
	return v4With(fmt.Sprintf("concealments:\n  vault:\n    checks: [{ability: religion, dc: 15}]\n    cells: [{q: 1, r: 0}]\n    attempts: %s\n", policy), "")
}

func (s *DiscoveryPolicyAuthoringSuite) TestAuthoredPolicyLowersAndRoundTrips() {
	source := discoveryDocument("{max: 2, reset_hexes: 4, lifetime: run}")
	compiled := loadSource(s.T(), source)
	s.Require().Len(compiled.Concealments, 1)
	policy, err := encounter.ResolveDiscoveryPolicy(compiled.Concealments[0].Attempts)
	s.Require().NoError(err)
	s.Equal(encounter.DiscoveryPolicy{MaxAttempts: 2, ResetHexes: 4, Lifetime: encounter.DiscoveryLifetimeRun}, *policy)
	s.Equal("religion", compiled.Concealments[0].Checks[0].Ability)

	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: []byte(source)})
	s.Require().NoError(err)
	raw, err := yaml.Marshal(decoded.Spec)
	s.Require().NoError(err)
	reloaded := loadSource(s.T(), string(raw))
	s.Equal(compiled.Concealments, reloaded.Concealments)
}

func (s *DiscoveryPolicyAuthoringSuite) TestOmittedFieldsUseTheCanonicalDefaults() {
	compiled := loadSource(s.T(), discoveryDocument("{reset_hexes: 4}"))
	policy, err := encounter.ResolveDiscoveryPolicy(compiled.Concealments[0].Attempts)
	s.Require().NoError(err)
	s.Equal(1, policy.MaxAttempts)
	s.Equal(4, policy.ResetHexes)
	s.Equal(encounter.DiscoveryLifetimeCharacter, policy.Lifetime)
}

func (s *DiscoveryPolicyAuthoringSuite) TestMalformedPolicyRefusesAtItsAuthorPath() {
	for _, policy := range []string{
		"null", "[]", "true", "{max: null}", "{max: 0}", "{max: -1}", "{max: 1.5}",
		"{reset_hexes: null}", "{reset_hexes: 1}", "{reset_hexes: 3.0}",
		"{lifetime: null}", "{lifetime: typo}", "{lifetime: 12}", "{lifetime: ''}", "{maximum: 2}",
	} {
		s.Run(policy, func() {
			_, err := dungeonspec.Load([]byte(discoveryDocument(policy)))
			s.Require().Error(err)
			s.Contains(err.Error(), "concealments.vault.attempts")
		})
	}
}
