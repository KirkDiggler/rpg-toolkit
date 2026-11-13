// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// PartySpec is the root `party` block: the party's settings for one
// encounter (rpg-project#935).
//
//	party:
//	  sharing: own | view | knowledge
//
// Its one key today is the sharing dial ([encounter.Sharing], rpg-project#930
// R10). Any other key is refused by the decoder at its own path, naming the
// keys the block takes.
type PartySpec struct {
	// Sharing is the party's sharing dial: how much of a party friend's
	// perception a player is shown. One of [encounter.Sharings]; omitted is
	// shared view ([encounter.DefaultSharing]) and is never written for the
	// author. `knowledge` compiles, and the encounter refuses it at launch
	// until shared knowledge is built (rpg-project#937).
	Sharing string `yaml:"sharing,omitempty" json:"sharing,omitempty"`
}

// sharingOf is the authored dial as the field carries it: empty when the
// document said nothing, so an unmarked document compiles to exactly the
// picture it compiled to before the dial existed.
func sharingOf(spec *SingleRoomSpec) encounter.Sharing {
	if spec.Party == nil {
		return ""
	}
	return encounter.Sharing(spec.Party.Sharing)
}

// partySharing refuses a `party.sharing` word the dial does not have, at its
// path, naming the words it does ([encounter.SharingList]). Omitted is
// shared view, and is not written for the author.
func (g *grammar) partySharing(p *PartySpec) {
	if p == nil || p.Sharing == "" {
		return
	}
	if encounter.IsSharing(encounter.Sharing(p.Sharing)) {
		return
	}
	g.fail("party.sharing", "%q is not a sharing: write %s, or omit it for view", p.Sharing, encounter.SharingList())
}
