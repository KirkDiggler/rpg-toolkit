// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// hail_site_test.go plays the hail site (rpg-project#923, Gate S3 of #769):
// a visitor walks up to the porter, who carries the gate's one claim; inside
// the gate's approach of two cells he hails her and the walk stops there,
// backing out of reach ends the address with the gate's progress untouched,
// and coming back is hailed again.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const hailSiteFixture = "testdata/hail-site.yaml"

const (
	hailSiteVisitor = encounter.MemberID("visitor")
	hailSitePorter  = encounter.MemberID("porter")
	hailSiteLine    = "Halt there. What's your business?"
	hailSiteReach   = 3
)

// hailSite is one run of the site.
type hailSite struct {
	t   *testing.T
	enc *encounter.Encounter
}

// openTheHailSite seats the visitor at the party start and the porter where
// the file posts him, everyone seeing twelve cells.
func openTheHailSite(t *testing.T) *hailSite {
	t.Helper()
	compiled := loadContent(t, hailSiteFixture)
	members := []encounter.MemberInput{{ID: hailSiteVisitor, Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At}}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Behaviors: m.Behaviors, Facing: m.FacingDegrees, Options: m.Options,
		})
	}
	stealth := &sneakSiteStealth{}
	site := &hailSite{t: t}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:     compiled.Field,
		Members:   members,
		Endings:   append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Retention: encounter.RetentionUnbounded,
		Capabilities: encounter.Capabilities{
			Sight:         sneakSiteSight{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sneakSiteSheets{enc: &site.enc, speeds: &[]int{}},
			Participation: &castleStanding{down: map[encounter.MemberID]bool{}},
			Initiative:    orderAsGiven{},
			Driver:        encounter.BehaviorDriver{},
			Roller:        rollsOne{},
			CheckResolver: stealth,
			Witness:       nobodyPerceivesAnything{},
			Actors: encounter.Actors{
				Striker:   cellsStriker{calls: &[]cellsStrike{}},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)
	site.enc = enc

	return site
}

// walkToward steps the visitor along the route to a cell, as a host walks
// her: it drops the remaining legs when a step reports an address, and
// returns that address, nil when the walk arrived unhailed.
func (s *hailSite) walkToward(to spatial.Position) *encounter.Addressed {
	s.t.Helper()
	route, err := s.enc.Route(encounter.RouteInput{Mover: hailSiteVisitor, Policy: encounter.MoveToward, Anchor: to, Budget: 60})
	require.NoError(s.t, err)
	for i, cell := range route.Path {
		out, err := s.enc.Step(&encounter.StepInput{Member: hailSiteVisitor, To: cell, EndWalk: i == len(route.Path)-1, Reach: hailSiteReach})
		require.NoError(s.t, err)
		if len(out.Addressed) > 0 {
			return &out.Addressed[0]
		}
	}

	return nil
}

// distance is how far the visitor stands from the porter.
func (s *hailSite) distance() float64 {
	s.t.Helper()
	return s.enc.Distance(positionOf(s.t, s.enc, hailSiteVisitor), positionOf(s.t, s.enc, hailSitePorter))
}

// standing is the visitor's standing address.
func (s *hailSite) standing() *encounter.Addressed {
	s.t.Helper()
	out, err := s.enc.AddressFor(&encounter.AddressForInput{Member: hailSiteVisitor, Reach: hailSiteReach})
	require.NoError(s.t, err)
	return out.Address
}

func TestTheHailSitePorterHailsTheVisitor(t *testing.T) {
	s := openTheHailSite(t)
	gate := s.enc.ToData().Interactions[0].Blob
	besidePorter := castleCell(3, 1)

	hailed := s.walkToward(besidePorter)
	require.NotNil(t, hailed, "the porter hails her on the way")
	require.Equal(t, encounter.InteractionID("gate"), hailed.Interaction)
	require.Equal(t, hailSitePorter, hailed.Guard)
	require.Equal(t, 2.0, s.distance(), "the walk stopped where the hail landed, two cells off")
	require.NotNil(t, s.standing())

	story, err := s.enc.Story(&encounter.StoryInput{Audience: hailSiteVisitor})
	require.NoError(t, err)
	said := false
	for _, entry := range story {
		said = said || answeredWith(entry.Payload, hailSiteLine)
	}
	require.True(t, said, "his hail line, through the answer path")

	// Out past Speak reach: the address ends, the gate is where it was.
	away := s.walkToward(compiledPartyStart(t))
	require.Nil(t, away, "nobody hails her walking away")
	require.Greater(t, s.distance(), float64(hailSiteReach), "precondition: beyond reach")
	require.Nil(t, s.standing(), "the address ended")
	require.Equal(t, gate, s.enc.ToData().Interactions[0].Blob, "progress untouched")

	again := s.walkToward(besidePorter)
	require.NotNil(t, again, "coming back is hailed again")
	require.Equal(t, hailSitePorter, again.Guard)
}

// compiledPartyStart is the site's party start.
func compiledPartyStart(t *testing.T) spatial.Position {
	t.Helper()
	return loadContent(t, hailSiteFixture).PartyStart[0].At
}

// answeredWith reports whether a beat is the porter's answer saying line.
func answeredWith(payload []byte, line string) bool {
	var b struct {
		Beat     string `json:"beat"`
		Creature string `json:"creature"`
		Say      string `json:"say"`
	}
	return json.Unmarshal(payload, &b) == nil && b.Beat == encounter.BeatAnswered &&
		b.Creature == string(hailSitePorter) && b.Say == line
}
