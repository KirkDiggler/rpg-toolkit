// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
)

// stockAuthoredMonsters records a catalog goblin's stat block in the session
// for every monster the stored world authored onto its roster without one.
//
// WHY A SCENE NEEDS IT. A world played by the session asks each member's sheet
// for its speed, attacks and sight at the moment of use (rpg-project#538), and
// a member the verb holds no sheet for is refused rather than answered with a
// default. Production never authors a sheetless monster — a host Spawns every
// one, recording its stat block — but these fixtures place monsters straight
// onto the map as scenery. This gives each one the sheet a Spawn would have
// recorded, so the scene is about what it was always about.
//
// A goblin because every catalog stat block states no sight range (so each
// sees the stated default, as these authored members always did) and because
// nothing these scenes assert depends on which creature stands there. A scene
// that cares what its monster is spawns it instead.
func stockAuthoredMonsters(t fataler, sessions *fakeSessions, encounters *fakeEncounters, sessionID string) {
	data, ok := sessions.byID[sessionID]
	if !ok {
		t.Fatalf("stocking monsters: no session %q", sessionID)
	}
	world, ok := encounters.byID[data.Encounter]
	if !ok {
		t.Fatalf("stocking monsters: session %q has no stored world %q", sessionID, data.Encounter)
	}

	held := make(map[string]bool, len(data.NPCs))
	for _, npc := range data.NPCs {
		held[npc.ID] = true
	}
	for _, member := range world.Members {
		if member.Kind != encounter.KindMonster || held[string(member.ID)] {
			continue
		}
		data.NPCs = append(data.NPCs, *monsters.NewGoblin(string(member.ID)).ToData())
	}
}

// stockAuthoredPlayers records a plain sheet — dullEyed's level-one human
// fighter — for every player the stored world authored onto its roster that
// the character store does not hold, for [stockAuthoredMonsters]'s reason: a
// player the verb holds no sheet for is refused the first time the world asks
// how far it sees or how fast it walks. A scene whose cast is the point names
// its players' sheets itself; this only fills the ones it left as scenery.
func stockAuthoredPlayers(world *encounter.EncounterData, characters *fakeCharacters) {
	for _, member := range world.Members {
		if member.Kind != encounter.KindPlayer {
			continue
		}
		if _, held := characters.byID[string(member.ID)]; held {
			continue
		}
		characters.byID[string(member.ID)] = dullEyed(string(member.ID))
	}
}
