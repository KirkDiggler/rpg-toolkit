// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/rolls"
)

type contributionSelectionIntegrationSuite struct{ suite.Suite }

func TestContributionSelectionIntegrationSuite(t *testing.T) {
	suite.Run(t, new(contributionSelectionIntegrationSuite))
}

func (s *contributionSelectionIntegrationSuite) gift(source, notation, params string) dndevents.ConditionBehavior {
	cfg := linkedRecipientConfig()
	cfg.ApplicationID = "application-" + source
	cfg.AddressSourceID = source
	cfg.Bindings.Owner.MemberID = source
	cfg.Bindings.Owner.SourceID = cfg.ApplicationID
	cfg.Bindings.Name = "Gift " + source
	cfg.Bindings.Parts = []dndfx.PartDeclaration{{Key: "roll", Kind: "add_to_roll", Amount: &generic.Amount{Dice: notation}, Params: json.RawMessage(params), OnTerminal: generic.EndPart}}
	raw, err := json.Marshal(cfg)
	s.Require().NoError(err)
	made, err := CreateFromRef(&CreateFromRefInput{Ref: refs.Conditions.LinkedEffects().String(), MemberID: "recipient", Origin: &dndevents.ConditionOrigin{Ref: *refs.Spells.Bless(), EntityID: source}, Config: raw})
	s.Require().NoError(err)
	return made.Condition
}

func (s *contributionSelectionIntegrationSuite) overlapping() []dndevents.ConditionBehavior {
	old, err := namedFixture(namedFixtureInput{Ref: refs.Conditions.Blessed(), MemberID: "recipient", SourceID: "old", SourceRef: refs.Spells.Bless()})
	s.Require().NoError(err)
	bane, err := namedFixture(namedFixtureInput{Ref: refs.Conditions.Baned(), MemberID: "recipient", SourceID: "bane", SourceRef: refs.Spells.Bane()})
	s.Require().NoError(err)
	newer := s.gift("new", "1d6", `{"polarity":"positive","rolls":["attack"],"selection":{"mode":"oldest_applicable","group":"bless"}}`)
	return []dndevents.ConditionBehavior{old, newer, bane}
}

func (s *contributionSelectionIntegrationSuite) TestBlessBaneAndAllApplicable() {
	held := s.overlapping()
	held = append(held, s.gift("gift-a", "1d8", `{"polarity":"positive","rolls":["attack"],"selection":{"mode":"all_applicable"}}`), s.gift("gift-b", "1d10", `{"polarity":"positive","rolls":["attack"],"selection":{"mode":"all_applicable"}}`))
	out, err := DescribeSelectedRollContributions(&DescribeSelectedRollContributionsInput{Conditions: held, Request: &dndevents.DescribeRollContributionsInput{Kind: dndevents.RollKindAttack}})
	s.Require().NoError(err)
	s.Require().Len(out.Contributions, 4)
	var sources, pools []string
	for _, term := range out.Contributions {
		sources = append(sources, term.Source.SourceID)
		pools = append(pools, term.Dice)
	}
	s.Equal([]string{"old", "bane", "gift-a", "gift-b"}, sources)
	s.Equal([]string{"1d4", "1d4", "1d8", "1d10"}, pools)
	s.False(out.Contributions[0].Subtract)
	s.True(out.Contributions[1].Subtract)
	s.Len(held, 5, "selection does not remove the newer held instance")
}

func (s *contributionSelectionIntegrationSuite) TestInformationMatchesExecution() {
	held := s.overlapping()
	frame := contributions.Frame{Actor: "recipient", Action: contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack)}}
	rows, err := AssessActionEffects(&AssessActionEffectsInput{Conditions: held, Frame: frame})
	s.Require().NoError(err)
	s.Require().Len(rows.Effects, 3)
	var infoSources []string
	for _, row := range rows.Effects {
		if row.State == contributions.StateApplies {
			infoSources = append(infoSources, row.Source.SourceID)
		}
	}
	s.Equal([]string{"old", "bane"}, infoSources)
	s.Equal("new", rows.Effects[1].Source.SourceID)
	s.Equal(contributions.StateDoesNotApply, rows.Effects[1].State)
	s.Contains(rows.Effects[1].Reason, "old", "the losing row must identify the selected source, not merely say another condition exists")
	s.Empty(rows.Effects[1].Benefit)
	actual, err := DescribeSelectedRollContributions(&DescribeSelectedRollContributionsInput{Conditions: held, Request: &dndevents.DescribeRollContributionsInput{Kind: dndevents.RollKindAttack}})
	s.Require().NoError(err)
	var actualSources []string
	for _, term := range actual.Contributions {
		actualSources = append(actualSources, term.Source.SourceID)
	}
	s.Equal([]string{"old", "bane"}, actualSources)
}

type selectedDiceRequest struct{ Count, Size int }
type selectedDiceRoller struct{ requests []selectedDiceRequest }

func (*selectedDiceRoller) Roll(context.Context, int) (int, error) {
	return 0, fmt.Errorf("unexpected single-die call")
}
func (r *selectedDiceRoller) RollN(_ context.Context, count, size int) ([]int, error) {
	r.requests = append(r.requests, selectedDiceRequest{Count: count, Size: size})
	faces := make([]int, count)
	for i := range faces {
		faces[i] = 1
	}
	return faces, nil
}
func (s *contributionSelectionIntegrationSuite) TestOnlySelectedDiceReachResolver() {
	held := s.overlapping()
	roller := &selectedDiceRoller{}
	selected, err := DescribeSelectedRollContributions(&DescribeSelectedRollContributionsInput{Conditions: held, Request: &dndevents.DescribeRollContributionsInput{Kind: dndevents.RollKindAttack}})
	s.Require().NoError(err)
	resolved, err := rolls.ResolveContributions(context.Background(), &rolls.ResolveContributionsInput{Roller: roller, Contributions: selected.Contributions})
	s.Require().NoError(err)
	s.Equal([]selectedDiceRequest{{Count: 1, Size: 4}, {Count: 1, Size: 4}}, roller.requests, "the excluded 1d6 is not rolled")
	s.Require().Len(resolved.Components, 2)
	s.Equal("old", resolved.Components[0].Source.SourceID)
	s.Equal("bane", resolved.Components[1].Source.SourceID)
	s.Equal([]int{1}, resolved.Components[1].Dice.FinalRolls, "physical faces remain positive")
	s.True(resolved.Components[1].SubtractDice)
}
