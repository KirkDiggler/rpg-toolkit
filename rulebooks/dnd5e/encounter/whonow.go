// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "fmt"

// whonow.go is WHO A WORD MEANS WHEN A ROW LANDS (rpg-project#990): the one
// resolver `give` ([Encounter.grantRecognition]) and a status grant
// ([StatusGrant.To]) both ask, so the sight rule a word carries is written
// once. A word means what its place in the who-word table says
// ([WhoWordAt]): `speaker` in `give.about` is the speaker only if the holder
// sees them; in `apply_status.to` it is the speaker, seen or not.

// landing is one consequence landing on a line: the line, and the live
// sight reach read for it at most once, and only when a word needs sight.
type landing struct {
	e     *Encounter
	line  recognitionLine
	reach sightReach
	read  bool
}

// landingOf is a landing of one consequence on the line.
func (e *Encounter) landingOf(line recognitionLine) *landing {
	return &landing{e: e, line: line}
}

// sight is the live reach the line is judged by ([Encounter.placedSightReach]),
// read on first use and kept for the rest of this landing.
func (l *landing) sight() (sightReach, error) {
	if l.read {
		return l.reach, nil
	}
	reach, err := l.e.placedSightReach()
	if err != nil {
		return sightReach{}, err
	}
	l.reach, l.read = reach, true
	return reach, nil
}

// whoNow is the members word means, written at place, as this line lands, in
// id order and none twice. Nobody is an answer: a line said to a thing has
// nobody spoken to, and a creature that sees none of the party sees nobody.
//
//   - `spoken_to`: the creature the line was said to.
//   - `party_in_sight`: the speaker's party the creature spoken to sees, live
//     — or, for a line judged as spoken, as it saw them when the words were
//     said ([recognitionLine.sees]) — the speaker included only if seen.
//   - `party_in_sight` at `state.give.about`: the party of the one the beat
//     names (that one included) the creature sees now.
//   - `the_gone` at `state.give.about`: the members of that party it does not
//     see now and holds a sight record about: it saw them earlier.
//   - `speaker` at `give.about`: the speaker, if the one spoken to sees them.
//   - `speaker` at `apply_status.to`: the speaker, unconditionally.
//
// A word its place does not allow is refused: the content checks refuse it
// first, so reaching here is a defect.
func (l *landing) whoNow(place, word string) ([]MemberID, error) {
	if _, ok := WhoWordAt(place, word); !ok {
		return nil, fmt.Errorf("who %q at %s: not a word written there", word, place)
	}
	line := l.line
	switch word {
	case WhoItself:
		return []MemberID{line.spokenTo}, nil
	case WhoTheOneNamed:
		if line.named == "" {
			return nil, nil
		}
		return []MemberID{line.named}, nil
	case string(RecognitionSpokenTo):
		if line.spokenTo == "" {
			return nil, nil
		}
		return []MemberID{line.spokenTo}, nil
	case string(RecognitionSpeaker):
		if place == WhoInApplyStatusTo {
			return []MemberID{line.speaker}, nil
		}
		holder := line.spokenTo
		if holder == "" || line.speaker == holder {
			return nil, nil
		}
		reach, err := l.sight()
		if err != nil {
			return nil, err
		}
		if !line.sees(reach, holder, line.speaker) {
			return nil, nil
		}
		return []MemberID{line.speaker}, nil
	case string(RecognitionPartyInSight):
		holder := line.spokenTo
		if holder == "" {
			return nil, nil
		}
		reach, err := l.sight()
		if err != nil {
			return nil, err
		}
		if place == WhoInStateGiveAbout {
			seen, _ := l.e.namedPartySeen(reach, holder, line)
			return seen, nil
		}
		return l.e.partyInSight(reach, holder, line), nil
	case WhoTheGone:
		if place != WhoInStateGiveAbout {
			break
		}
		holder := line.spokenTo
		if holder == "" {
			return nil, nil
		}
		reach, err := l.sight()
		if err != nil {
			return nil, err
		}
		_, gone := l.e.namedPartySeen(reach, holder, line)
		return gone, nil
	}
	return nil, fmt.Errorf("who %q at %s: no resolution written for it", word, place)
}

// namedPartySeen is the party of the one the line names, that one included
// and holder never, split in id order: those holder sees now on the reach
// given, and those it does not see now but holds a sight record about (it saw
// them earlier: the test an arrest uses). A line that names nobody gives
// nobody for both.
func (e *Encounter) namedPartySeen(reach sightReach, holder MemberID, line recognitionLine) (seen, gone []MemberID) {
	if line.named == "" {
		return nil, nil
	}
	for _, id := range e.partyOf(line.named) {
		if id == holder {
			continue
		}
		if line.sees(reach, holder, id) {
			seen = append(seen, id)
			continue
		}
		if _, held := e.intelLog.On(sightMember(holder), sightMember(id)); held == nil {
			gone = append(gone, id)
		}
	}
	return seen, gone
}
