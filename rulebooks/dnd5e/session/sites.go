// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// sites.go is THE SITE IS CONTENT THE RECORD REFERENCES at the session seam
// (rpg-project#879 S5, design §Where the instance lives).
//
// A stored world ([encounter.EncounterData]) names its compiled site by
// [encounter.EncounterData.SiteHash] and never carries the field. Every load
// this package makes resolves that hash through Config.Sites (nil is
// [encounter.DefaultSites], the in-process default). A provider that does not
// hold the hash is not the end: the host's [SiteRepository], when its
// encounter repository has one, is asked for the stored piece by hash, and the
// piece is handed to the load, which compiles it into the provider once. A hash
// neither knows fails the verb closed with [ErrUnknownSite].
//
// THE TWO TIERS. The site tier is shared across games: one compiled site per
// content per process, whatever number of sessions play on it. The instance
// tier is per game: the session's locker guards it, and nothing in a site is
// written by a verb.

// ErrUnknownSite means a stored world names a site that neither Config.Sites
// nor the host's [SiteRepository] holds. The world cannot be loaded on this
// process until one of them does; nothing is guessed. It arrives wrapped
// beside [ErrInvalidWorld], so a host matching on that still catches it.
var ErrUnknownSite = errors.New("session world names an unknown site")

// SiteRepository is an OPTIONAL capability of the host's [EncounterRepository],
// found by type assertion on Config.Encounters, exactly as
// [FencedEncounterRepository] is and for the same reason: a new required
// method is a silent runtime break for a host that has adopted this package.
// It stores the site PIECE, the compiled site's persistent form, keyed by its
// content hash, apart from the checkpoints that name it.
//
// A host without it keeps every site in process: a world launched here loads
// here, and a cold process cannot load a world whose site its provider does
// not already hold. A host whose provider resolves every hash from its own
// content (rpg-api's authored keeps) needs neither.
//
// Pieces are content-addressed and immutable: the same hash is always the
// same bytes, so PutSite is idempotent and a piece is never rewritten.
//
// A host that wraps its repository must forward these methods, or the
// wrapper hides the capability.
type SiteRepository interface {
	// GetSite returns the stored piece for in.Hash, or an ErrNotFound error
	// when none is stored. Success with no data violates the contract.
	GetSite(ctx context.Context, in *GetSiteInput) (*GetSiteOutput, error)

	// PutSite stores in.Data under in.Hash. Storing a hash already held is
	// a no-op success.
	PutSite(ctx context.Context, in *PutSiteInput) (*PutSiteOutput, error)
}

// GetSiteInput names the piece to read.
type GetSiteInput struct {
	// Hash is the site's content hash, as a stored world names it.
	Hash encounter.SiteHash
}

// GetSiteOutput is one stored site piece.
type GetSiteOutput struct {
	// Data is the piece. Never nil on success.
	Data *encounter.SiteData
}

// PutSiteInput is one piece to store.
type PutSiteInput struct {
	// Hash is the piece's content hash, the key a stored world names it by.
	Hash encounter.SiteHash

	// Data is the piece. Never nil.
	Data *encounter.SiteData
}

// PutSiteOutput is empty: a stored piece has nothing to report.
type PutSiteOutput struct{}

// siteSource is what a load hands [encounter.LoadEncounterInput.Site]: the
// stored piece, or nil when the provider already holds the site.
type siteSource struct {
	piece *encounter.SiteData
}

// sitePiece is the site source for a world naming hash: no piece when the
// provider already holds it (a warm load compiles nothing), the repository's
// stored piece when the provider does not, and ErrUnknownSite when neither has
// it. An empty hash is the encounter's to refuse by name, so it passes through
// with no piece.
func (m *Manager) sitePiece(ctx context.Context, hash encounter.SiteHash) (siteSource, error) {
	if hash == "" {
		return siteSource{}, nil
	}
	_, err := m.sites.Site(ctx, &encounter.SiteInput{Hash: hash})
	if err == nil {
		return siteSource{}, nil
	}
	if !errors.Is(err, encounter.ErrUnknownSite) {
		return siteSource{}, fmt.Errorf("site %s: %w: %v", hash, ErrInvalidWorld, err)
	}
	if m.siteStore == nil {
		return siteSource{}, fmt.Errorf("site %s, and no site repository to ask: %w: %w", hash, ErrInvalidWorld, ErrUnknownSite)
	}
	out, err := m.siteStore.GetSite(ctx, &GetSiteInput{Hash: hash})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return siteSource{}, fmt.Errorf("site %s: %w: %w", hash, ErrInvalidWorld, ErrUnknownSite)
		}
		return siteSource{}, fmt.Errorf("reading site %s: %w", hash, err)
	}
	if out == nil || out.Data == nil {
		return siteSource{}, fmt.Errorf("site %s: GetSite reported success with no data: %w", hash, ErrBadRepository)
	}
	return siteSource{piece: out.Data}, nil
}

// storeSite writes a freshly launched world's site piece to the host's
// [SiteRepository], once per Launch and before the checkpoint that names it,
// so no stored world ever names a piece the store does not hold. A host
// without the capability keeps the site in process.
func (m *Manager) storeSite(ctx context.Context, site *encounter.Site) error {
	if m.siteStore == nil || site == nil {
		return nil
	}
	data := site.Data()
	if _, err := m.siteStore.PutSite(ctx, &PutSiteInput{Hash: site.Hash(), Data: &data}); err != nil {
		return fmt.Errorf("storing site %s: %w", site.Hash(), err)
	}
	return nil
}
