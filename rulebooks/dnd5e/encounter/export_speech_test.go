// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// export_speech_test.go is the test-only door into one utterance
// (rpg-project#933) for the external package's keep benchmark. In a _test.go
// file, so nothing outside this package reaches it at build time.

// ExportedUtter has speaker say words at a loudness word to nobody in
// particular, as a creature's own-time line is said, and returns how many
// caught the words.
func (e *Encounter) ExportedUtter(speaker MemberID, words, loudness string) (int, error) {
	out, err := e.utter(utterance{
		speaker: speaker, words: words, loudness: loudnessWord(loudness),
		at: uint64(e.clock.ToData().HighWater),
	})
	if err != nil {
		return 0, err
	}
	return len(out.catchers), nil
}
