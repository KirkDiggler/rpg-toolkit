// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// loadOverSite loads data over the site compiled from site, through a fresh
// provider (rpg-project#879 S5): the seam an edited site piece is refused at
// now that the checkpoint carries only the hash. A site that compiles names
// its own hash on the checkpoint, so an edit the compiler accepts loads over
// the edited site; one it refuses is refused by the load, as an edited
// field always was.
func loadOverSite(
	data encounter.EncounterData, site encounter.SiteData, in encounter.LoadEncounterInput,
) (*encounter.Encounter, error) {
	// A site that does not compile has no hash of its own; the checkpoint
	// names one anyway, so the load compiles the piece and reports why not.
	data.SiteHash = "edited"
	if out, err := encounter.CompileSite(&encounter.CompileSiteInput{Data: &site}); err == nil {
		data.SiteHash = out.Site.Hash()
	}
	in.Data = data
	in.Site = &site
	in.Sites = encounter.NewMemorySites(nil)
	return encounter.LoadEncounter(&in)
}

// siteOf is the site piece a hand-built checkpoint's field compiles from: the
// field, and the checkpoint's own doors as geometry, written closed.
func siteOf(field encounter.FieldData, data encounter.EncounterData) encounter.SiteData {
	doors := make([]encounter.DoorData, 0, len(data.Doors))
	for _, d := range data.Doors {
		d.State = "closed"
		d.Lock = nil
		doors = append(doors, d)
	}
	return encounter.SiteData{Version: encounter.SiteDataVersion, Field: field, Doors: doors}
}

// withDefaultSite is data naming the site compiled from field, kept in
// [encounter.DefaultSites] so a plain LoadEncounter resolves it.
func withDefaultSite(field encounter.FieldData, data encounter.EncounterData) encounter.EncounterData {
	site := siteOf(field, data)
	out, err := encounter.DefaultSites().Site(context.Background(), &encounter.SiteInput{Data: &site})
	if err != nil {
		panic(err)
	}
	data.SiteHash = out.Site.Hash()
	return data
}
