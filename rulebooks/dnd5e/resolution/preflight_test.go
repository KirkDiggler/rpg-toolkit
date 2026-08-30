// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type PreflightTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *PreflightTestSuite) SetupTest() { s.ctx = context.Background() }

func TestPreflightSuite(t *testing.T) { suite.Run(t, new(PreflightTestSuite)) }

func (s *PreflightTestSuite) hero(id string, conds ...json.RawMessage) *character.Data {
	return &character.Data{
		ID: id, PlayerID: "player-1", Name: "Standre", Level: 1,
		ClassID: classes.Barbarian, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 16,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 14, MaxHitPoints: 14, ArmorClass: 10,
		ProficiencyBonus: 2, Conditions: conds,
	}
}

func unreadable() json.RawMessage { return json.RawMessage(`{"ref":"nonsense","x":`) }

// A clean cast reports nothing, which is the answer that lets a caller proceed.
func (s *PreflightTestSuite) TestACleanCastRefusesNobody() {
	out, err := Preflight(s.ctx, &PreflightInput{
		Participants: []Participant{{Character: s.hero("hero-1")}, {Character: s.hero("hero-2")}},
		Roller:       refusingRoller{},
	})
	s.Require().NoError(err)

	s.Empty(out.Unreadable, "every participant attached, so there is nothing to report")
}

// EVERY refusal is collected, not just the first — the reason this entry exists
// rather than the caller calling Resolve and reading its error.
//
// Two broken participants with a good one BETWEEN them, on purpose: an
// implementation that stopped at the first failure reports one row, and an
// implementation that stopped at the first SUCCESS reports one row too. Only a
// pass that runs the whole cast reports both, and the good one in the middle
// means neither shortcut can fake it.
func (s *PreflightTestSuite) TestEveryRefusalIsCollected() {
	out, err := Preflight(s.ctx, &PreflightInput{
		Participants: []Participant{
			{Character: s.hero("aaa", unreadable())},
			{Character: s.hero("mmm")},
			{Character: s.hero("zzz", unreadable())},
		},
		Roller: refusingRoller{},
	})
	s.Require().NoError(err)

	s.Require().Len(out.Unreadable, 2, "both broken participants, not just the one that failed first")
	s.Equal("aaa", out.Unreadable[0].Member)
	s.Equal("zzz", out.Unreadable[1].Member)
	s.Require().Error(out.Unreadable[0].Reason, "each row carries why, so a caller can say which row is dead and why")
}

// The report is in cast order whatever order the caller supplied, so two
// preflights over the same data produce comparable reports.
func (s *PreflightTestSuite) TestTheReportIsInCastOrder() {
	out, err := Preflight(s.ctx, &PreflightInput{
		Participants: []Participant{
			{Character: s.hero("zzz", unreadable())},
			{Character: s.hero("aaa", unreadable())},
		},
		Roller: refusingRoller{},
	})
	s.Require().NoError(err)

	s.Require().Len(out.Unreadable, 2)
	s.Equal([]string{"aaa", "zzz"},
		[]string{out.Unreadable[0].Member, out.Unreadable[1].Member})
}

// What refuses here refuses in Resolve, which is the whole claim: this predicts
// an interaction rather than approximating one.
//
// Asserted by running BOTH over the same record — a preflight that used its own
// looser rules would report clean and then Resolve would refuse, which is worse
// than no preflight at all.
func (s *PreflightTestSuite) TestItRefusesWhatResolveWouldRefuse() {
	record := s.hero("hero-1", unreadable())

	out, err := Preflight(s.ctx, &PreflightInput{
		Participants: []Participant{{Character: record}},
		Roller:       refusingRoller{},
	})
	s.Require().NoError(err)
	s.Require().Len(out.Unreadable, 1, "the preflight says this cast cannot be stood up")

	_, attachErr := attachAll(s.ctx, newSurface(events.NewEventBus()), &attachAllInput{
		Participants: []Participant{{Character: s.hero("hero-1", unreadable())}},
		Roller:       refusingRoller{},
	})
	s.Require().Error(attachErr, "and the strict attach Resolve uses agrees")
}

// Nothing survives the pass, including the participants that attached fine on
// the way to finding the ones that did not.
func (s *PreflightTestSuite) TestNothingIsLeftOnTheBus() {
	inner := events.NewEventBus()

	raw, err := (&conditions.UnarmoredDefenseCondition{
		MemberID: "hero-1", Type: conditions.UnarmoredDefenseBarbarian,
	}).ToJSON()
	s.Require().NoError(err)

	out, err := preflightOn(s.ctx, &PreflightInput{
		Participants: []Participant{
			{Character: s.hero("hero-1", raw)},
			{Character: s.hero("hero-2", unreadable())},
		},
		Roller: refusingRoller{},
	}, newSurface(inner))
	s.Require().NoError(err)
	s.Require().Len(out.Unreadable, 1, "it reported before it tore down")

	event := &combat.ACChainEvent{
		CharacterID: "hero-1",
		Breakdown:   &combat.ACBreakdown{Total: 0, Components: []combat.ACComponent{}},
	}
	chain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)

	modified, err := combat.ACChain.On(inner).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)

	s.Require().Empty(folded.Breakdown.Components,
		"the participant that DID attach was revoked too — a preflight leaves no subscribers behind")
}

// Registrations come out in participant order, which is R4: two preflights over
// identical data must grant identical registration lists, or the report cannot
// be compared against the next one.
//
// # This does NOT discriminate the implementation it replaced, and saying so is
// the point
//
// Two earlier versions of this test claimed it did. The first asserted every
// participant landed on the same surface; the second asserted sorted
// registration order. Review restored the exact previous implementation
// underneath both and everything passed — because that implementation ALSO
// shared one surface and ALSO sorted, just in its own loop.
//
// So this is a property of the ENTRY, held equally by the code before and after,
// and it is worth pinning for that reason rather than as evidence of a change.
// What actually changed is pinned below.
func (s *PreflightTestSuite) TestRegistrationsComeOutInParticipantOrder() {
	surf := newSurface(events.NewEventBus())

	_, err := preflightOn(s.ctx, &PreflightInput{
		Participants: []Participant{
			{Character: s.hero("zzz")}, {Character: s.hero("mmm")}, {Character: s.hero("aaa")},
		},
		Roller: refusingRoller{},
	}, surf)
	s.Require().NoError(err)

	var order []string
	for _, registration := range surf.registrations() {
		if len(order) == 0 || order[len(order)-1] != registration.Participant {
			order = append(order, registration.Participant)
		}
	}

	s.Equal([]string{"aaa", "mmm", "zzz"}, order,
		"however the caller stacked them, the attach grants registrations in participant order")
}

// TestOrderingIsDecidedInOnePlace pins what this entry actually changed.
//
// The previous implementation ran its own sort.Slice and then called the attach
// once per participant. The attach sorts too — so the ordering rule lived in TWO
// places that had to agree, and if attachAll's rule ever changed, this entry
// would have kept its own quietly. Nothing observable differed between the two
// while they agreed, which is exactly why no behavioural test could tell them
// apart and why this one is structural.
//
// The entry hands the whole cast to the attach and lets it order them. So this
// file names no sorter, and that absence is the change.
func TestOrderingIsDecidedInOnePlace(t *testing.T) {
	source, err := os.ReadFile("preflight.go")
	require.NoError(t, err)

	require.False(t, strings.Contains(string(source), "sort."),
		"preflight.go sorts participants itself. Ordering is attachAll's rule (R4), and a "+
			"second copy of it here is a copy that can drift: two sorts that agree today are "+
			"two sorts somebody must remember to change together. Hand it the whole cast")
}

// A missing roller is refused rather than defaulted// A missing roller is refused rather than defaulted: rolling on a real roller
// here would spend randomness the interaction being predicted has not spent.
func (s *PreflightTestSuite) TestAMissingRollerIsRefused() {
	_, err := Preflight(s.ctx, &PreflightInput{
		Participants: []Participant{{Character: s.hero("hero-1")}},
	})
	s.Require().ErrorIs(err, ErrNoRoller)
}

func (s *PreflightTestSuite) TestNilInputRefuses() {
	_, err := Preflight(s.ctx, nil)
	s.Require().ErrorIs(err, ErrNilInput)
}
