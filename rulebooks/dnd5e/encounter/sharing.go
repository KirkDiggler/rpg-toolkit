// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"errors"
	"fmt"
	"strings"
)

// Sharing is THE PARTY'S SHARING DIAL (rpg-project#930, ruling R10;
// rpg-project#935): how much of what one party member perceives the others
// are shown. One setting per encounter, authored in the content as
// `party: { sharing: own | view | knowledge }`.
//
//   - [SharingOwn]: each player is shown only what their own character
//     perceives.
//   - [SharingView]: each player is also shown what any party friend sees and
//     hears, as it happens. THE DEFAULT: an encounter that says nothing is
//     shared view.
//   - [SharingKnowledge]: the friends' knowledge is shared too. Designed, not
//     built (rpg-project#937): it compiles, an encounter carrying it is built
//     and loaded like any other — a preview, an atlas, a save of the site —
//     and a host refuses to LAUNCH a run for players with it, by asking
//     [LaunchableSharing] ([ErrSharedKnowledgeNotBuilt]).
//
// INERT HERE: the encounter carries the setting, persists it and hands it to
// its host ([Encounter.Sharing]), and decides no audience by it. Every beat
// goes to exactly whom it went to before the dial existed; what a player is
// shown of a friend's perception is the session's merge (R11), which never
// widens an audience the rules decided.
type Sharing string

const (
	// SharingOwn is "each their own": a player is shown what their own
	// character perceives and nothing a friend does.
	SharingOwn Sharing = "own"
	// SharingView is "shared view", the default: a player is also shown what
	// any party friend sees and hears.
	SharingView Sharing = "view"
	// SharingKnowledge is "shared knowledge": designed, refused at launch
	// until rpg-project#937 builds it.
	SharingKnowledge Sharing = "knowledge"
)

// Sharings is every word the dial takes, in dial order: narrowest first.
// THE ONE LIST the content's refusal names its words from ([SharingWords]).
var Sharings = []Sharing{SharingOwn, SharingView, SharingKnowledge}

// DefaultSharing is the stop an encounter that says nothing is set to.
const DefaultSharing = SharingView

// ErrSharedKnowledgeNotBuilt is returned by [LaunchableSharing] for
// [SharingKnowledge]: the stop is designed and not yet built
// (rpg-project#937), and a run that silently played it as shared view would
// tell its players they share what they do not.
var ErrSharedKnowledgeNotBuilt = errors.New("encounter: shared knowledge is not built yet")

// ErrUnknownSharing is returned when an encounter carries a sharing word the
// dial does not have.
var ErrUnknownSharing = errors.New("encounter: no such sharing")

// SharingWords is [Sharings] as words, in dial order. A fresh slice each
// call, so no caller can change it for another.
func SharingWords() []string {
	out := make([]string, len(Sharings))
	for i, s := range Sharings {
		out[i] = string(s)
	}
	return out
}

// SharingList is [SharingWords] as a refusal says them: "own, view or
// knowledge".
func SharingList() string {
	words := SharingWords()
	return strings.Join(words[:len(words)-1], ", ") + " or " + words[len(words)-1]
}

// IsSharing reports whether s is a word the dial takes ([Sharings]).
func IsSharing(s Sharing) bool {
	for _, w := range Sharings {
		if s == w {
			return true
		}
	}
	return false
}

// validateSharing is the dial's rule for an encounter being built or loaded,
// asked by Setup and by Load alike: unmarked and every word of [Sharings] is
// a setting the encounter carries — shared knowledge too, because building
// is also how a site is previewed, projected and saved; any other word is
// refused naming the words there are. Whether a run may be LAUNCHED for
// players with it is [LaunchableSharing]'s.
func validateSharing(s Sharing) error {
	if s == "" || IsSharing(s) {
		return nil
	}
	return fmt.Errorf("the party's sharing is %q, and sharing is %s: %w", s, SharingList(), ErrUnknownSharing)
}

// LaunchableSharing is THE LAUNCH RULE for the dial, owned here so its words
// are said in one place: the check a host makes before it starts a run for
// players. Unmarked, each their own and shared view launch; shared knowledge
// is refused, wrapping [ErrSharedKnowledgeNotBuilt], with the sentence "the
// party's sharing is set to shared knowledge, and shared knowledge is not
// built yet (rpg-project#937)"; a word the dial does not have is refused
// with [ErrUnknownSharing]. It is not asked when an encounter is merely
// built or loaded — a preview, an atlas, a restore of a run that launched.
func LaunchableSharing(s Sharing) error {
	if s == SharingKnowledge {
		return fmt.Errorf("the party's sharing is set to shared knowledge, and shared knowledge is not built yet "+
			"(rpg-project#937): %w", ErrSharedKnowledgeNotBuilt)
	}
	return validateSharing(s)
}

// Sharing is the party's sharing dial this encounter runs with
// ([FieldInput.Sharing]): the authored stop, or [DefaultSharing] when the
// content said nothing. [SharingKnowledge] on an encounter built for a
// preview or projection; never on a launched run, which a host refuses by
// [LaunchableSharing]. The host reads it to decide what a player is shown of
// a friend's perception; the encounter decides nothing by it.
func (e *Encounter) Sharing() Sharing {
	if e.sharing == "" {
		return DefaultSharing
	}
	return e.sharing
}
