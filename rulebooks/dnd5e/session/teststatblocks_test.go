// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// stockAuthoredPlayers records a plain sheet — dullEyed's level-one human
// fighter — for every party seat the scene names that the character store does
// not hold: a launch refuses a party member it holds no sheet for. A scene
// whose cast is the point names its players' sheets itself; this only fills
// the ones it left as scenery.
func stockAuthoredPlayers(sc scene, characters *fakeCharacters) {
	for _, seat := range sc.Party {
		if _, held := characters.byID[seat.ID]; held {
			continue
		}
		characters.byID[seat.ID] = dullEyed(seat.ID)
	}
}
