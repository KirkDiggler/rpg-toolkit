// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"maps"
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
// each carrier's expanded list at the carrier's paths; [lineRewrite.say]
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

// withOwnLines is a copy of s whose carriers' lists may be rewritten
// without touching s: the factions, the creature bindings and the prop
// bindings are copied one level down, everything else is shared. The
// validation and the compile expand the copy, so a caller's spec keeps its
// `use:` entries and a second compile after an edit reads the edit.
func withOwnLines(s *SingleRoomSpec) *SingleRoomSpec {
	c := *s
	c.Factions = slices.Clone(s.Factions)
	c.Room.Gameplay.MonsterBindings = maps.Clone(s.Room.Gameplay.MonsterBindings)
	c.Room.Gameplay.PropBindings = maps.Clone(s.Room.Gameplay.PropBindings)
	return &c
}

// lineRewrite is how a refusal said at a carrier's expanded line is said at
// the author's: each expanded `<carrier>.options[i]` to where that line was
// written, and how many carriers use each conversation.
type lineRewrite struct {
	sources map[string]LineSource
	uses    map[string]int
}

// resolveConversations expands every `use:` entry in a single-room document
// into its conversation's lines, IN PLACE: validation and compile hand it a
// copy ([withOwnLines]), never the caller's spec. It judges each
// conversation once at its own path, and returns the rewrite every later
// refusal is said through ([lineRewrite.say]).
func resolveConversations(s *SingleRoomSpec, add errSink) *lineRewrite {
	rw := &lineRewrite{sources: map[string]LineSource{}, uses: map[string]int{}}
	for _, id := range sortedKeys(s.Conversations) {
		at := "conversations." + id
		lines := s.Conversations[id].Lines
		if len(lines) == 0 {
			add(at+".lines", errConversationNoLines)
			continue
		}
		written := map[string]int{}
		for k, line := range lines {
			if line.useWritten {
				add(fmt.Sprintf("%s.lines[%d].use", at, k), errConversationNests)
				continue
			}
			// AN ID WRITTEN TWICE IN ONE CONVERSATION is the conversation's
			// fault, said once here, whoever uses it.
			if prev, dup := written[line.ID]; dup && line.ID != "" {
				add(fmt.Sprintf("%s.lines[%d].id", at, k), fmt.Sprintf(
					"line %q is already written at %s.lines[%d]; a line's id is unique within its conversation", line.ID, at, prev))
				continue
			}
			written[line.ID] = k
		}
	}
	for _, c := range lineCarriers(s) {
		authored := c.get()
		lines, sources, users := expandLines(s.Conversations, c.at, authored, add)
		for i, src := range sources {
			rw.sources[fmt.Sprintf("%s.options[%d]", c.at, i)] = src
		}
		for _, id := range users {
			rw.uses[id]++
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
		if rw.uses[id] == 0 && len(s.Conversations[id].Lines) > 0 {
			add("conversations."+id, fmt.Sprintf(
				"conversation %q is used by nobody: write `- use: %s` in a creature's, a prop's or a faction's options, or remove it",
				id, id))
		}
	}
	return rw
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
	// A LIST THAT USES A CONVERSATION has its ids judged here, every one at
	// an authored path: past a `use:` the grammar's indices are expanded
	// ones and would send the author to the wrong line. A list that uses
	// none is the grammar's, in its own words, as it always was.
	uses := slices.ContainsFunc(authored, func(o OptionSpec) bool { return o.useWritten })
	usedAt := map[string]int{}
	// seen is each line id already said on this carrier, to where it was
	// written and the conversation that wrote it (empty for its own).
	type origin struct{ path, conv string }
	seen := map[string]origin{}
	twice := func(id string, now origin, at string) bool {
		prev, dup := seen[id]
		switch {
		case id == "" || !uses:
			return false
		case !dup:
			seen[id] = now
			return false
		case now.conv != "" && prev.conv == now.conv:
			return true // refused once, inside the conversation
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
			if twice(line.ID, origin{path: src, conv: name}, p+".use") {
				continue
			}
			out = append(out, line)
			sources = append(sources, LineSource{Path: src, Conversation: name, Use: p})
		}
	}
	return out, sources, users
}

// sourceOf is the line a path is under, when it is under a carrier's
// expanded line, and the rest of the path below it.
func (rw *lineRewrite) sourceOf(path string) (LineSource, string, bool) {
	i := strings.Index(path, ".options[")
	if i < 0 {
		return LineSource{}, "", false
	}
	j := strings.IndexByte(path[i:], ']')
	if j < 0 {
		return LineSource{}, "", false
	}
	src, ok := rw.sources[path[:i+j+1]]
	return src, path[i+j+1:], ok
}

// say tells sink every refusal, each at the author's path and once: a
// carrier's own line at its authored index, a conversation's line at
// `conversations.<id>.lines[k]` however many carriers met the fault. A
// fault met on SOME of a conversation's carriers and not all is the
// carrier's as much as the line's: the refusal names the `use:` entries
// it was met through.
func (rw *lineRewrite) say(sink errSink, refusals []FieldError) {
	type said struct {
		path, message, conv string
		uses                []string
	}
	var order []*said
	byKey := map[[2]string]*said{}
	for _, e := range refusals {
		path, conv, use := e.Path, "", ""
		if src, rest, ok := rw.sourceOf(e.Path); ok {
			path, conv, use = src.Path+rest, src.Conversation, src.Use
		}
		key := [2]string{path, e.Message}
		at, ok := byKey[key]
		if !ok {
			at = &said{path: path, message: e.Message, conv: conv}
			byKey[key] = at
			order = append(order, at)
		}
		if use != "" && !slices.Contains(at.uses, use) {
			at.uses = append(at.uses, use)
		}
	}
	for _, at := range order {
		message := at.message
		if at.conv != "" && len(at.uses) < rw.uses[at.conv] {
			message += fmt.Sprintf(" (said through the conversation %q at %s)", at.conv, strings.Join(at.uses, ", "))
		}
		sink(at.path, message)
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
