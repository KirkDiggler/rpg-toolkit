package assessment_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/assessment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/stretchr/testify/suite"
)

type damageContractSuite struct{ suite.Suite }

func TestDamageContractSuite(t *testing.T) { suite.Run(t, new(damageContractSuite)) }

func (s *damageContractSuite) TestKnownZeroAndUnresolvedDice() {
	zero := 0
	fixed := assessment.DamageChange{PoolID: assessment.PrimaryWeaponPool,
		Source: contributions.Source{Name: "Rule"}, Fixed: &zero}
	dice := assessment.DamageChange{PoolID: assessment.PrimaryWeaponPool,
		Source: contributions.Source{Name: "Rule"}, Dice: "2d6", DoubleDiceOnCritical: true}
	for _, change := range []assessment.DamageChange{fixed, dice} {
		s.Require().NoError((assessment.AssessDamageOutput{
			Decision: contributions.Decision{Applicability: contributions.Applies, Reason: "Known eligible"},
			Changes:  []assessment.DamageChange{change},
		}).Validate())
	}
}

func (s *damageContractSuite) TestContradictoryAndEmptyChangesFail() {
	zero := 0
	for _, changes := range [][]assessment.DamageChange{
		nil,
		{{PoolID: assessment.PrimaryWeaponPool, Source: contributions.Source{Name: "Rule"}}},
		{{PoolID: assessment.PrimaryWeaponPool, Source: contributions.Source{Name: "Rule"}, Fixed: &zero, Dice: "1d6"}},
		{{Source: contributions.Source{Name: "Rule"}, Fixed: &zero}},
		{{PoolID: assessment.PrimaryWeaponPool, Fixed: &zero}},
		{{PoolID: assessment.PrimaryWeaponPool, Source: contributions.Source{Name: "Rule"},
			Fixed: &zero, DoubleDiceOnCritical: true}},
	} {
		s.Error((assessment.AssessDamageOutput{
			Decision: contributions.Decision{Applicability: contributions.Applies, Reason: "Must have a real change"},
			Changes:  changes,
		}).Validate())
	}
	for _, decision := range []contributions.Decision{
		{Applicability: contributions.DoesNotApply, Reason: "Known ineligible"},
		{Applicability: contributions.NeedsContext, Reason: "Unknown",
			Needs: []contributions.Need{{Kind: contributions.NeedTarget}}},
	} {
		s.Error((assessment.AssessDamageOutput{Decision: decision,
			Changes: []assessment.DamageChange{{PoolID: assessment.PrimaryWeaponPool,
				Source: contributions.Source{Name: "Rule"}, Fixed: &zero}},
		}).Validate())
	}
}

func (s *damageContractSuite) TestDetachedChange() {
	amount := 2
	original := assessment.DamageChange{PoolID: assessment.PrimaryWeaponPool,
		Source: contributions.Source{Name: "Rule", Ref: &core.Ref{Module: "dnd5e", Type: "conditions", ID: "raging"}},
		Fixed:  &amount}
	cloned := original.Clone()
	*cloned.Fixed = 5
	cloned.Source.Ref.ID = "changed"
	s.Equal(2, *original.Fixed)
	s.Equal("raging", original.Source.Ref.ID)
}
