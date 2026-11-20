// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"slices"
	"strings"
)

// conversations.go is A CONVERSATION'S LINES WRITTEN ONCE (rpg-project#1025,
// design R20): a root `conversations:` entry holds lines, and any carrier's
// `options:` lists `- use: <id>` to say them.
//
//	conversations:
//	  gate-claims:
//	    lines:
//	      - id: troupe
//	        say: We're the troupe for tonight's feast.
//	        …
//	monsterBindings:
//	  gate-1:
//	    options:
//	      - use: gate-claims
//	  captain:
//	    options:
//	      - id: guest
//	        say: We're invited guests of His Majesty, here for the feast.
//	      - use: gate-claims
//
// EXPANDED AT LOAD, AS A BUNDLE IS ([resolveBundles]): every `use:` entry is
// replaced by the conversation's lines, in order, at that place, before the
// grammar or the compiler reads the list, so the engine compiles exactly
// what the same lines copied onto each carrier compile to. The runtime, the
// session and the wire never see a conversation's name.
//
// A FAULT INSIDE A LINE IS SAID ONCE, AT THE CONVERSATION. The grammar judges
// each carrier's expanded list at the carrier's paths; [conversationPaths]
// rewrites a path under an expanded line to the line's own path,
// `conversations.<id>.lines[k]…`, and the sink says each (path, message)
// once, however many carry it.
//
// A CONVERSATION DOES NOT NEST: a line in one may not `use:` another. A
// carrier lists each conversation it says beside its own lines instead.

// The sentences a conversation and a `use:` entry are refused with, for the
// form.
const (
	errUseNoConversation   = "`use` names no conversation"
	errConversationNoLines = "a conversation holds no lines: write the lines it offers, or remove it"
	errConversationNests   = "a conversation's line cannot use another conversation; conversations do not nest. " +
		"Write the line here, or list both conversations on each carrier"
	errUseOutsideSite = "`use` names a conversation, and only a version 5 site declares `conversations:`"
)

// LineSource is where one line a carrier says was written
// (rpg-project#1025): its own path, and, for a line a `use:` entry brought,
// the conversation and the entry.
type LineSource struct {
	// Path is the line's authored path: `<carrier>.options[i]` for the
	// carrier's own line, `conversations.<id>.lines[k]` for a conversation's.
	Path string

	// Conversation is the conversation the line was written in; empty for
	// the carrier's own line.
	Conversation string

	// Use is the path of the `use:` entry that brought the line,
	// `<carrier>.options[i]`; empty for the carrier's own line.
	Use string
}

// LineSources lists, in the order the carrier says them after expansion,
// where each of a carrier's lines was written. at is the carrier's path
// (`room.room.monsterBindings.gate-1`, `factions[0]`) and options its
// `options:` AS AUTHORED. On a site that compiles, the i-th source is the
// i-th line the carrier's compiled options hold. A `use:` entry the
// expansion refuses contributes nothing.
func LineSources(s *SingleRoomSpec, at string, options []OptionSpec) []LineSource {
	_, sources, _ := expandLines(s.Conversations, at, options, nil)
	return sources
}

// lineCarrier is one list a `use:` entry may sit in: its path, and how to
// read and write it.
type lineCarrier struct {
	at  string
	get func() []OptionSpec
	set func([]OptionSpec)
}

// lineCarriers is every list of lines in a single-room document, in a fixed
// order: the factions' in authored order, then the creatures' and the
// props' by id.
func lineCarriers(s *SingleRoomSpec) []lineCarrier {
	var out []lineCarrier
	for i := range s.Factions {
		out = append(out, lineCarrier{
			at:  fmt.Sprintf("factions[%d]", i),
			get: func() []OptionSpec { return s.Factions[i].Options },
			set: func(o []OptionSpec) { s.Factions[i].Options = o },
		})
	}
	gp := &s.Room.Gameplay
	for _, id := range sortedBindingIDs(gp.MonsterBindings) {
		out = append(out, lineCarrier{
			at:  "room.room.monsterBindings." + id,
			get: func() []OptionSpec { return gp.MonsterBindings[id].Options },
			set: func(o []OptionSpec) {
				b := gp.MonsterBindings[id]
				b.Options = o
				gp.MonsterBindings[id] = b
			},
		})
	}
	for _, id := range sortedPropBindingIDs(gp.PropBindings) {
		out = append(out, lineCarrier{
			at:  "room.room.propBindings." + id,
			get: func() []OptionSpec { return gp.PropBindings[id].Options },
			set: func(o []OptionSpec) {
				b := gp.PropBindings[id]
				b.Options = o
				gp.PropBindings[id] = b
			},
		})
	}
	return out
}

// resolveConversations expands every `use:` entry in a single-room document
// into its conversation's lines, judges each conversation once at its own
// path, and returns the rewrite of an expanded line's path to the line's
// own ([conversationPaths]).
//
// IDEMPOTENT. Validation and compile both run it: the first run keeps each
// carrier's authored list, and every run expands that, never an expanded
// one.
func resolveConversations(s *SingleRoomSpec, add errSink) func(string) string {
	carriers := lineCarriers(s)
	if s.authoredLines == nil {
		s.authoredLines = map[string][]OptionSpec{}
		for _, c := range carriers {
			s.authoredLines[c.at] = slices.Clone(c.get())
		}
	}
	for _, id := range sortedKeys(s.Conversations) {
		at := "conversations." + id
		lines := s.Conversations[id].Lines
		if len(lines) == 0 {
			add(at+".lines", errConversationNoLines)
			continue
		}
		for k, line := range lines {
			if line.useWritten {
				add(fmt.Sprintf("%s.lines[%d].use", at, k), errConversationNests)
			}
		}
	}
	paths := map[string]string{}
	used := map[string]bool{}
	for _, c := range carriers {
		authored := s.authoredLines[c.at]
		lines, sources, users := expandLines(s.Conversations, c.at, authored, add)
		for i, src := range sources {
			paths[fmt.Sprintf("%s.options[%d]", c.at, i)] = src.Path
		}
		for _, id := range users {
			used[id] = true
		}
		// A LIST WHOSE EVERY ENTRY WAS REFUSED is left absent, so the one
		// refusal is the entry's and not a second "lists nothing" beside it.
		// A LIST WRITTEN EMPTY stays empty, for the grammar's own refusal.
		switch {
		case len(lines) == 0 && len(authored) > 0:
			lines = nil
		case lines == nil && authored != nil:
			lines = []OptionSpec{}
		}
		c.set(lines)
	}
	for _, id := range sortedKeys(s.Conversations) {
		if !used[id] && len(s.Conversations[id].Lines) > 0 {
			add("conversations."+id, fmt.Sprintf(
				"conversation %q is used by nobody: write `- use: %s` in a creature's, a prop's or a faction's options, or remove it",
				id, id))
		}
	}
	return conversationPaths(paths)
}

// restoreAuthoredLines puts back each carrier's `options:` as it was
// written, undoing [resolveConversations]' expansion, so a decoded spec
// preserves exactly what was authored.
func restoreAuthoredLines(s *SingleRoomSpec) {
	if s.authoredLines == nil {
		return
	}
	for _, c := range lineCarriers(s) {
		c.set(s.authoredLines[c.at])
	}
	s.authoredLines = nil
}

// expandLines is one carrier's authored list with every `use:` entry
// replaced by its conversation's lines: the lines, where each was written,
// and the conversations it used. add, when not nil, is told every refusal;
// a refused entry or line is left out, so the expansion of a site that
// compiles is exactly what [LineSources] reports.
func expandLines(
	conversations map[string]ConversationSpec, at string, authored []OptionSpec, add errSink,
) ([]OptionSpec, []LineSource, []string) {
	refuse := func(path, message string) {
		if add != nil {
			add(path, message)
		}
	}
	var (
		out     []OptionSpec
		sources []LineSource
		users   []string
	)
	usedAt := map[string]int{}
	// seen is each line id already said on this carrier, to where it was
	// written and whether a conversation wrote it.
	type origin struct {
		path string
		conv bool
	}
	seen := map[string]origin{}
	// twice refuses a line id met a second time when a conversation wrote
	// either one (two own lines with one id are the grammar's to refuse, at
	// its own path and in its own words).
	twice := func(id string, now origin, at string) bool {
		prev, dup := seen[id]
		if id == "" || !dup || (!prev.conv && !now.conv) {
			if id != "" && !dup {
				seen[id] = now
			}
			return false
		}
		refuse(at, fmt.Sprintf("line %q is said twice on this carrier: at %s and at %s; a line's id is unique among "+
			"what one carrier says, so rename one or drop it", id, prev.path, now.path))
		return true
	}
	for i, o := range authored {
		p := fmt.Sprintf("%s.options[%d]", at, i)
		if !o.useWritten {
			if twice(o.ID, origin{path: p}, p+".id") {
				continue
			}
			out = append(out, o)
			sources = append(sources, LineSource{Path: p})
			continue
		}
		name := o.Use
		conv, declared := conversations[name]
		switch prev, again := usedAt[name]; {
		case len(o.beside) > 0:
			refuse(p, fmt.Sprintf("a `use:` entry stands for a conversation's lines and says nothing else: "+
				"drop %s here, or write it as a line of its own beside the use", quoteKeys(o.beside)))
			continue
		case name == "":
			refuse(p+".use", errUseNoConversation)
			continue
		case !declared:
			refuse(p+".use", fmt.Sprintf("`use: %s` names no conversation in this file: %s", name, declaredConversations(conversations)))
			continue
		case again:
			refuse(p+".use", fmt.Sprintf("conversation %q is already used at options[%d]; a carrier says a conversation once", name, prev))
			continue
		}
		usedAt[name] = i
		users = append(users, name)
		for k, line := range conv.Lines {
			if line.useWritten {
				continue // refused once, at the conversation's own path
			}
			src := fmt.Sprintf("conversations.%s.lines[%d]", name, k)
			if twice(line.ID, origin{path: src, conv: true}, p+".use") {
				continue
			}
			line.Conversation = name
			out = append(out, line)
			sources = append(sources, LineSource{Path: src, Conversation: name, Use: p})
		}
	}
	return out, sources, users
}

// conversationPaths is the rewrite of a path under a carrier's expanded
// line, `<carrier>.options[i]…`, to the line's authored path: the
// conversation's line, or the carrier's own line at its authored index. A
// path under no expanded line is returned as it is.
func conversationPaths(paths map[string]string) func(string) string {
	return func(path string) string {
		i := strings.Index(path, ".options[")
		if i < 0 {
			return path
		}
		j := strings.IndexByte(path[i:], ']')
		if j < 0 {
			return path
		}
		if to, ok := paths[path[:i+j+1]]; ok {
			return to + path[i+j+1:]
		}
		return path
	}
}

// saidOnceAt is a sink that rewrites each path with at and says each
// (path, message) once: a conversation's fault, met on every carrier, is
// one refusal at the conversation.
func saidOnceAt(sink errSink, at func(string) string) errSink {
	seen := map[[2]string]bool{}
	return func(path, message string) {
		key := [2]string{at(path), message}
		if !seen[key] {
			seen[key] = true
			sink(key[0], message)
		}
	}
}

// declaredConversations names the conversations a refusal may point at.
func declaredConversations(conversations map[string]ConversationSpec) string {
	ids := sortedKeys(conversations)
	if len(ids) == 0 {
		return "it declares none"
	}
	return "it declares " + strings.Join(ids, ", ")
}

// quoteKeys is a list of keys as a refusal names them: `id`, `say`.
func quoteKeys(keys []string) string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = "`" + k + "`"
	}
	return strings.Join(out, ", ")
}
