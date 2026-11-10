// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
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
