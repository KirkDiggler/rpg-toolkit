// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// words.go turns the engine's compiled vocabulary into plain clauses. Every
// function returns a clause, never a whole sentence, so a statement can add
// one more facet as one more clause.

// quote marks an authored id as a word the file uses.
func quote(id string) string { return "`" + id + "`" }

// joinAnd joins words as "a, b and c".
func joinAnd(words []string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	}
	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}

// joinOr joins words as "a, b or c".
func joinOr(words []string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	}
	return strings.Join(words[:len(words)-1], ", ") + " or " + words[len(words)-1]
}

// quoteAll quotes every id.
func quoteAll(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = quote(id)
	}
	return out
}

// capitalize upper-cases a clause's first letter.
func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 || !unicode.IsLower(r) {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// said renders authored speech in typographic quotes.
func said(s string) string { return "“" + s + "”" }

// cellWords is an authored axial cell.
func cellWords(q, r int) string { return fmt.Sprintf("q %d, r %d", q, r) }

// posWords is a compiled position carried in the author's axial frame (a
// selector's `at`, a route stop, a post cell).
func posWords(p spatial.Position) string { return cellWords(int(p.X), int(p.Y)) }

// checkWords says what a check rolls: "Persuasion against DC 13, or
// Deception against DC 15".
func checkWords(approaches []encounter.CheckApproach) string {
	parts := make([]string, 0, len(approaches))
	for _, a := range approaches {
		p := fmt.Sprintf("%s against DC %d", abilityWords(a.Ability), a.DC)
		if a.Tool != "" {
			p = fmt.Sprintf("%s with %s against DC %d", abilityWords(a.Ability), quote(a.Tool), a.DC)
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, ", or ")
}

// abilityWords is a skill or ability word as a person reads it.
func abilityWords(a string) string {
	switch a {
	case "str":
		return "Strength"
	case "dex":
		return "Dexterity"
	case "con":
		return "Constitution"
	case "int":
		return "Intelligence"
	case "wis":
		return "Wisdom"
	case "cha":
		return "Charisma"
	}
	words := strings.Split(a, "-")
	for i, w := range words {
		if w != "of" && w != "and" {
			words[i] = capitalize(w)
		}
	}
	return strings.Join(words, " ")
}

// lineContext is who "me" and "the other party" are when a requirement is
// read: on a line, the speaker and the creature spoken to; on a post's pass,
// the walker and the post's holder.
type lineContext struct {
	me    string // "the speaker", "the walker"
	other string // the display name of the one spoken to or holding the post
}

// triggerWords says one compiled requirement or predicate as a clause about
// who knows, holds or recognises what. It also returns the connections the
// clause reads.
func (b *builder) triggerWords(t encounter.Trigger, ctx lineContext) (string, []Connection) {
	switch v := t.(type) {
	case encounter.TriggerFact:
		if ctx.me == "" {
			return fmt.Sprintf("the fact %s is known", quote(string(v.Fact))),
				[]Connection{{Kind: "fact", ID: string(v.Fact), Role: "asks"}}
		}
		return fmt.Sprintf("%s knows the fact %s", ctx.me, quote(string(v.Fact))),
			[]Connection{{Kind: "fact", ID: string(v.Fact), Role: "asks"}}
	case encounter.TriggerHolds:
		me := ctx.me
		if me == "" {
			me = "someone"
		}
		return fmt.Sprintf("%s carries the prop %s", me, quote(b.unqualify(string(v.Prop)))),
			[]Connection{{Kind: "prop", ID: b.unqualify(string(v.Prop)), Role: "asks"}}
	case encounter.TriggerRecognized:
		return fmt.Sprintf("%s recognises %s as %s (%s holds %s about %s)",
				ctx.other, ctx.me, quote(string(v.Topic)), ctx.other, quote(string(v.Topic)), ctx.me),
			[]Connection{{Kind: "topic", ID: string(v.Topic), Role: "asks"}}
	case encounter.TriggerInteraction:
		in := quoteAll(v.In)
		state := "in the state " + joinOr(in)
		if len(in) > 1 {
			state = "in one of the states " + joinOr(in)
		}
		return fmt.Sprintf("the conversation %s is %s (one state for the whole run, not per speaker)",
				quote(string(v.Interaction)), state),
			[]Connection{{Kind: "conversation", ID: string(v.Interaction), Role: "asks"}}
	case encounter.TriggerNot:
		me := ctx.me
		if me == "" {
			me = "someone"
		}
		switch in := v.Inner.(type) {
		case encounter.TriggerRecognized:
			return fmt.Sprintf("%s does NOT recognise %s as %s (%s does not hold %s about %s)",
					ctx.other, ctx.me, quote(string(in.Topic)), ctx.other, quote(string(in.Topic)), ctx.me),
				[]Connection{{Kind: "topic", ID: string(in.Topic), Role: "asks"}}
		case encounter.TriggerFact:
			return fmt.Sprintf("%s does NOT know the fact %s", me, quote(string(in.Fact))),
				[]Connection{{Kind: "fact", ID: string(in.Fact), Role: "asks"}}
		case encounter.TriggerHolds:
			return fmt.Sprintf("%s does NOT carry the prop %s", me, quote(b.unqualify(string(in.Prop)))),
				[]Connection{{Kind: "prop", ID: b.unqualify(string(in.Prop)), Role: "asks"}}
		}
		inner, conns := b.triggerWords(v.Inner, ctx)
		return "NOT (" + inner + ")", conns
	case encounter.TriggerMemberDown:
		return fmt.Sprintf("%s is down", b.creature(string(v.Member))),
			[]Connection{{Kind: "creature", ID: string(v.Member), Role: "asks"}}
	case encounter.TriggerLeft:
		return fmt.Sprintf("the last standing member of the party leaves by the exit %s", quote(string(v.Exit))),
			[]Connection{{Kind: "exit", ID: string(v.Exit), Role: "asks"}}
	case encounter.TriggerExitedHolding:
		return fmt.Sprintf("a party member leaves by the exit %s carrying the prop %s",
				quote(string(v.Exit)), quote(b.unqualify(string(v.Item)))),
			[]Connection{{Kind: "exit", ID: string(v.Exit), Role: "asks"}}
	case encounter.TriggerRound:
		return fmt.Sprintf("round %d is reached", v.Round), nil
	case encounter.TriggerReachedPosition:
		return fmt.Sprintf("a party member reaches %s", posWords(v.Position)), nil
	case encounter.TriggerStance:
		return fmt.Sprintf("the factions %s and %s stand %s", quote(string(v.Between[0])),
			quote(string(v.Between[1])), v.Stance), nil
	case encounter.TriggerExternal:
		return "the host says so", nil
	}
	return fmt.Sprintf("a requirement this build cannot say (%T)", t), nil
}

// subjectsWords says whom a recognition is about.
func subjectsWords(about encounter.RecognitionSubjects, holder string) string {
	switch about {
	case encounter.RecognitionPartyInSight:
		return fmt.Sprintf("the speaker and each member of the speaker's party that %s sees at that moment "+
			"(a member out of %s's sight is not recognised)", holder, holder)
	case encounter.RecognitionSpeaker:
		return fmt.Sprintf("the speaker alone, if %s sees the speaker (not whoever stands beside the speaker)", holder)
	}
	return fmt.Sprintf("the subjects %q (a word this build cannot say)", about)
}

// period ends a sentence, unless it already ends in a quotation that does.
func period(s string) string {
	for _, end := range []string{".”", "!”", "?”", "…”"} {
		if strings.HasSuffix(s, end) {
			return s
		}
	}
	return s + "."
}

// count says a number of things: "1 beat", "3 beats".
func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
