// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// THE SITE IS CONTENT THE RECORD REFERENCES (rpg-project#879 S5). A stored
// world names its site by hash; the load resolves it through the host's
// provider, and the piece the host stored beside the checkpoint compiles on a
// provider miss. These pin that resolution's one load takes both as handed.

// countingSites is a provider that counts what it is asked.
type countingSites struct {
	inner encounter.SiteProvider
	asked atomic.Int32
}

func (c *countingSites) Site(ctx context.Context, in *encounter.SiteInput) (*encounter.SiteOutput, error) {
	c.asked.Add(1)
	return c.inner.Site(ctx, in)
}

func (s *ResolveTestSuite) siteCaps() encounter.Capabilities {
	return encounter.Capabilities{
		Initiative:    orderAsGiven{},
		Driver:        passDriver{},
		Participation: everyoneParticipates{},
		Sight:         everyoneSeesTheWholeMap{},
		Equipment:     noHandsAreObserved{},
		Conditions:    noHandsAreObserved{},
		Sheets:        noSheetsAsked{},
		Roller:        dice.NewRoller(),
		Actors:        Actors,
	}
}

func (s *ResolveTestSuite) resolveOn(world encounter.EncounterData, sites encounter.SiteProvider, piece *encounter.SiteData) (*Output, error) {
	return Resolve(s.ctx, &Input{
		World:        world,
		Sites:        sites,
		Site:         piece,
		Participants: []Participant{{Character: s.barbarian()}},
		Machine:      s.save(abilities.STR),
		Capabilities: s.siteCaps(),
	})
}

func (s *ResolveTestSuite) TestALoadResolvesItsSiteThroughTheHostsProvider() {
	sites := &countingSites{inner: encounter.NewMemorySites(nil)}
	enc := s.encounterOn(sites)
	asked := sites.asked.Load()

	out, err := s.resolveOn(enc.ToData(), sites, nil)
	s.Require().NoError(err)
	s.Greater(sites.asked.Load(), asked, "the load asked the injected provider")
	s.Equal(enc.Site().Hash(), out.World.SiteHash, "the stored world names the same site")
}

func (s *ResolveTestSuite) TestAProviderThatDoesNotHoldTheSiteFailsClosed() {
	// The world's site is in the process default; the host injected a
	// provider that has never seen it. The load must not fall back.
	world := s.world()
	out, err := s.resolveOn(world, encounter.NewMemorySites(nil), nil)
	s.Require().Error(err)
	s.True(errors.Is(err, encounter.ErrUnknownSite), "an unknown site is refused by name: %v", err)
	s.Nil(out)
}

func (s *ResolveTestSuite) TestTheStoredSitePieceCompilesOnAProviderMiss() {
	enc := s.encounterOn(encounter.NewMemorySites(nil))
	piece := enc.Site().Data()
	cold := encounter.NewMemorySites(nil)

	_, err := s.resolveOn(enc.ToData(), cold, &piece)
	s.Require().NoError(err)

	held, err := cold.Site(s.ctx, &encounter.SiteInput{Hash: enc.Site().Hash()})
	s.Require().NoError(err, "the piece compiled into the host's provider")
	s.Equal(enc.Site().Hash(), held.Site.Hash())
}

func (s *ResolveTestSuite) TestTheProcessDefaultServesAHostThatInjectsNothing() {
	out, err := s.resolveOn(s.world(), nil, nil)
	s.Require().NoError(err)
	s.NotEmpty(out.World.SiteHash)
}

func (s *ResolveTestSuite) TestASitePieceBesideALentWorldIsRefused() {
	enc := s.encounterOn(nil)
	piece := enc.Site().Data()
	_, err := Resolve(s.ctx, &Input{
		Live:         enc,
		Site:         &piece,
		Participants: []Participant{{Character: s.barbarian()}},
		Machine:      s.save(abilities.STR),
		Capabilities: s.siteCaps(),
	})
	s.Require().ErrorIs(err, ErrBadWorld)
}
