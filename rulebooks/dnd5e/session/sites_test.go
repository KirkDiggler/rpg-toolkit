// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// THE SITE IS CONTENT THE RECORD REFERENCES (rpg-project#879 S5, sites.go).
// These pin the session seam: Config.Sites resolves every load, the host's
// SiteRepository holds the piece a cold process compiles from, Launch stores
// it once, and a hash nobody holds fails closed by name.

// siteStore is the plain fake encounter repository with the optional site
// capability: pieces by hash, and counters for what was asked.
type siteStore struct {
	*fakeEncounters
	mu     sync.Mutex
	pieces map[encounter.SiteHash]*encounter.SiteData
	puts   []encounter.SiteHash
	gets   int
	// writes is every write in the order it landed: "site" for a piece,
	// "encounter" for a checkpoint.
	writes []string
}

func (f *siteStore) SaveEncounter(ctx context.Context, id string, data *encounter.EncounterData) error {
	f.mu.Lock()
	f.writes = append(f.writes, "encounter")
	f.mu.Unlock()
	return f.fakeEncounters.SaveEncounter(ctx, id, data)
}

func newSiteStore() *siteStore {
	return &siteStore{fakeEncounters: newFakeEncounters(), pieces: map[encounter.SiteHash]*encounter.SiteData{}}
}

func (f *siteStore) GetSite(_ context.Context, in *session.GetSiteInput) (*session.GetSiteOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets++
	piece, ok := f.pieces[in.Hash]
	if !ok {
		return nil, session.ErrNotFound
	}
	return &session.GetSiteOutput{Data: piece}, nil
}

func (f *siteStore) PutSite(_ context.Context, in *session.PutSiteInput) (*session.PutSiteOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.puts = append(f.puts, in.Hash)
	f.writes = append(f.writes, "site")
	f.pieces[in.Hash] = in.Data
	return &session.PutSiteOutput{}, nil
}

// countingSites is a site provider that counts the asks that carry a source
// to compile: a load that hands it none compiles nothing.
type countingSites struct {
	inner   encounter.SiteProvider
	mu      sync.Mutex
	sourced int
	asked   int
}

func newCountingSites() *countingSites {
	return &countingSites{inner: encounter.NewMemorySites(nil)}
}

func (c *countingSites) Site(ctx context.Context, in *encounter.SiteInput) (*encounter.SiteOutput, error) {
	c.mu.Lock()
	c.asked++
	if in.Field != nil || in.Data != nil {
		c.sourced++
	}
	c.mu.Unlock()
	return c.inner.Site(ctx, in)
}

func (c *countingSites) counts() (asked, sourced int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.asked, c.sourced
}

type SiteSuite struct {
	suite.Suite
	ctx        context.Context
	sessions   *fakeSessions
	characters *fakeCharacters
	seats      *fakeSeats
}

func TestSiteSuite(t *testing.T) { suite.Run(t, new(SiteSuite)) }

func (s *SiteSuite) SetupTest() {
	s.ctx = context.Background()
	s.sessions = newFakeSessions()
	s.characters = testCharacters()
	s.seats = newFakeSeats()
}

// manager is one process over the suite's shared stores, with its own sites.
func (s *SiteSuite) manager(encounters session.EncounterRepository, sites encounter.SiteProvider) *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		Record: session.DiscardRecord{}, Acts: &fakeStream{}, Seats: s.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: encounters, Characters: s.characters,
		Sites: sites,
	})
	s.Require().NoError(err)
	return mgr
}

// sight is what alice sees and maps, as bytes: the answers a load serves.
func (s *SiteSuite) sight(mgr *session.Manager) string {
	view, err := mgr.View(s.ctx, &session.ViewInput{Session: testSession, Member: "alice"})
	s.Require().NoError(err)
	atlas, err := mgr.Atlas(s.ctx, &session.AtlasInput{Session: testSession, Member: "alice"})
	s.Require().NoError(err)
	raw, err := json.Marshal(struct {
		View  *session.ViewOutput
		Atlas *session.Atlas
	}{view, atlas})
	s.Require().NoError(err)
	return string(raw)
}

func (s *SiteSuite) TestLaunchStoresThePieceOnceUnderTheHashTheWorldNames() {
	store := newSiteStore()
	_, err := s.manager(store, newCountingSites()).Launch(s.ctx, sceneInput(authoredWorld()))
	s.Require().NoError(err)

	world := store.byID[testSession]
	s.Require().NotEmpty(world.SiteHash)
	s.Equal([]encounter.SiteHash{world.SiteHash}, store.puts, "one piece, under the checkpoint's hash")
	compiled, err := encounter.CompileSite(&encounter.CompileSiteInput{Data: store.pieces[world.SiteHash]})
	s.Require().NoError(err)
	s.Equal(world.SiteHash, compiled.Site.Hash(), "the piece is the content the hash names")
}

func (s *SiteSuite) TestAWarmLoadCompilesNothing() {
	sites := newCountingSites()
	mgr := s.manager(newSiteStore(), sites)
	_, err := mgr.Launch(s.ctx, sceneInput(authoredWorld()))
	s.Require().NoError(err)
	askedBefore, sourcedBefore := sites.counts()

	_, err = mgr.Move(s.ctx, &session.MoveInput{Session: testSession, Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}}})
	s.Require().NoError(err)
	s.sight(mgr)

	asked, sourced := sites.counts()
	s.Greater(asked, askedBefore, "every load asked the injected provider")
	s.Equal(sourcedBefore, sourced, "no load handed it anything to compile")
}

func (s *SiteSuite) TestAColdProcessLoadsFromTheStoredPieceAndAnswersTheSame() {
	store := newSiteStore()
	warm := s.manager(store, newCountingSites())
	_, err := warm.Launch(s.ctx, sceneInput(authoredWorld()))
	s.Require().NoError(err)
	want := s.sight(warm)

	coldSites := newCountingSites()
	cold := s.manager(store, coldSites)
	s.Equal(want, s.sight(cold), "the cold process answers byte for byte")
	s.Equal(1, store.gets, "the provider missed once and the repository was asked once")
	_, sourced := coldSites.counts()
	s.Equal(1, sourced, "the piece compiled once into the cold provider")

	s.sight(cold)
	_, err = cold.Move(s.ctx, &session.MoveInput{Session: testSession, Member: "alice", Path: []spatial.Position{{X: 2, Y: 1}}})
	s.Require().NoError(err)
	_, sourced = coldSites.counts()
	s.Equal(1, store.gets, "warm now: the repository is not asked again")
	s.Equal(1, sourced, "and nothing compiles again")
}

func (s *SiteSuite) TestAnUnknownSiteFailsClosedByName() {
	store := newSiteStore()
	_, err := s.manager(store, newCountingSites()).Launch(s.ctx, sceneInput(authoredWorld()))
	s.Require().NoError(err)

	s.Run("the repository does not hold the piece", func() {
		empty := newSiteStore()
		empty.byID = store.byID
		_, err := s.manager(empty, newCountingSites()).View(s.ctx, &session.ViewInput{Session: testSession, Member: "alice"})
		s.Require().ErrorIs(err, session.ErrUnknownSite)
		s.ErrorIs(err, session.ErrInvalidWorld)
	})
	s.Run("the host keeps no pieces", func() {
		plain := newFakeEncounters()
		plain.byID = store.byID
		_, err := s.manager(plain, newCountingSites()).View(s.ctx, &session.ViewInput{Session: testSession, Member: "alice"})
		s.Require().ErrorIs(err, session.ErrUnknownSite)
		s.ErrorIs(err, session.ErrInvalidWorld)
	})
}

func (s *SiteSuite) TestTwoSessionsOnOneSiteShareIt() {
	sites := newCountingSites()
	store := newSiteStore()
	mgr := s.manager(store, sites)
	first := authoredWorld()
	second := authoredWorld()
	second.Session = "sess-2"
	second.Party = []sceneSeat{seatAt("bob", 1, 1)}
	_, err := mgr.Launch(s.ctx, sceneInput(first))
	s.Require().NoError(err)
	_, err = mgr.Launch(s.ctx, sceneInput(second))
	s.Require().NoError(err)

	a, err := mgr.LoadWorldForTest(s.ctx, testSession)
	s.Require().NoError(err)
	b, err := mgr.LoadWorldForTest(s.ctx, "sess-2")
	s.Require().NoError(err)
	s.Same(a.Site(), b.Site(), "one compiled site serves both games")
	s.Equal(store.byID[testSession].SiteHash, store.byID["sess-2"].SiteHash)
	s.Equal([]encounter.SiteHash{a.Site().Hash(), a.Site().Hash()}, store.puts,
		"each launch stores its piece once; the store keeps one under the hash")
	s.Len(store.pieces, 1)
}

// forgetfulSites compiles what it is handed and keeps nothing, so every ask
// by hash alone is unknown to it.
type forgetfulSites struct{}

func (forgetfulSites) Site(_ context.Context, in *encounter.SiteInput) (*encounter.SiteOutput, error) {
	if in.Field == nil && in.Data == nil {
		return nil, fmt.Errorf("site %s: %w", in.Hash, encounter.ErrUnknownSite)
	}
	out, err := encounter.CompileSite(&encounter.CompileSiteInput{Field: in.Field, Data: in.Data})
	if err != nil {
		return nil, err
	}
	return &encounter.SiteOutput{Site: out.Site}, nil
}

// evictingSites holds its sites until armed; armed, it answers the next ask
// by hash alone and then forgets that site, as a bounded provider evicting
// between a load's two asks would.
type evictingSites struct {
	inner *encounter.MemorySites
	armed bool
	hits  int
}

func (e *evictingSites) Site(ctx context.Context, in *encounter.SiteInput) (*encounter.SiteOutput, error) {
	if e.armed && in.Field == nil && in.Data == nil {
		e.hits++
		if e.hits > 1 {
			e.armed = false
			e.inner = encounter.NewMemorySites(nil)
			return nil, fmt.Errorf("site %s: %w", in.Hash, encounter.ErrUnknownSite)
		}
	}
	return e.inner.Site(ctx, in)
}

func (s *SiteSuite) TestLaunchStoresThePieceBeforeTheCheckpointThatNamesIt() {
	store := newSiteStore()
	_, err := s.manager(store, newCountingSites()).Launch(s.ctx, sceneInput(authoredWorld()))
	s.Require().NoError(err)
	s.Require().Contains(store.writes, "encounter")
	s.Equal("site", store.writes[0], "the piece lands before any checkpoint naming it: %v", store.writes)
}

// previewWorld is authoredWorld on content no other test compiles, so the
// process default never holds its site.
func previewWorld() scene {
	world := authoredWorld()
	world.Field.Regions = []encounter.RegionInput{rectRegion("s5-preview-hall", 0, 0, 5, 5)}
	return world
}

func (s *SiteSuite) TestTheMapPreviewLoadsThroughTheInjectedProvider() {
	dungeon := sceneInput(previewWorld()).Dungeon

	s.Run("a provider that holds the site serves the preview", func() {
		sites := newCountingSites()
		atlas, err := s.manager(newFakeEncounters(), sites).AtlasOf(s.ctx, &session.AtlasOfInput{Dungeon: dungeon})
		s.Require().NoError(err)
		s.NotNil(atlas)
		asked, _ := sites.counts()
		s.Greater(asked, 1, "compiled into the provider and loaded back through it")
	})
	s.Run("a provider that refuses the hash fails the preview closed", func() {
		_, err := s.manager(newFakeEncounters(), forgetfulSites{}).AtlasOf(s.ctx, &session.AtlasOfInput{Dungeon: dungeon})
		s.Require().ErrorIs(err, session.ErrUnknownSite)
		s.ErrorIs(err, session.ErrInvalidWorld)
	})
}

func (s *SiteSuite) TestASiteEvictedBetweenTheTwoAsksFallsThroughToTheStoredPiece() {
	s.Run("the stored piece serves the load", func() {
		s.SetupTest()
		sites := &evictingSites{inner: encounter.NewMemorySites(nil)}
		store := newSiteStore()
		mgr := s.manager(store, sites)
		_, err := mgr.Launch(s.ctx, sceneInput(authoredWorld()))
		s.Require().NoError(err)
		sites.armed = true
		_, err = mgr.View(s.ctx, &session.ViewInput{Session: testSession, Member: "alice"})
		s.Require().NoError(err)
		s.Equal(1, store.gets, "the second miss read the piece")
	})
	s.Run("no stored piece fails closed by name", func() {
		s.SetupTest()
		sites := &evictingSites{inner: encounter.NewMemorySites(nil)}
		mgr := s.manager(newFakeEncounters(), sites)
		_, err := mgr.Launch(s.ctx, sceneInput(authoredWorld()))
		s.Require().NoError(err)
		sites.armed = true
		_, err = mgr.View(s.ctx, &session.ViewInput{Session: testSession, Member: "alice"})
		s.Require().ErrorIs(err, session.ErrUnknownSite)
		s.ErrorIs(err, session.ErrInvalidWorld)
	})
}
